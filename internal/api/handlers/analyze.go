package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/aiclient"
	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/feed"
	"github.com/sentinel/gateway/internal/repo"
)

// Analyze 三层引擎判决 — 全程依赖 AI 子服务，无 mock 兜底。
//
// 完成后副作用（按出现顺序，与 HTTP 响应解耦）：
//  1. Publish feed.Event 到 Hub（SSE 订阅者实时收到）
//  2. 计数器递增（Store.Inc*）
//  3. 异步落 call_logs（写失败仅 log）
func Analyze(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req aiclient.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.ShownNumber == "" {
			badRequest(w, "VALIDATION_FAILED", "shownNumber 必填")
			return
		}

		// 附带 sysadmin 配置的千问参数（agent_config key=qwen）；读不到就用 AI 默认
		req.LLMOptions = qwenOptions(r.Context(), d)

		verdict, err := d.AI.Analyze(r.Context(), req)
		if err != nil {
			if d.Store != nil {
				d.Store.IncAnalyzeFailed()
			}
			d.Logger.Error("ai analyze failed", "err", err, "callId", req.CallID)
			upstreamErr(w, "AI_UPSTREAM_ERROR", "AI 子服务调用失败")
			return
		}
		if d.Store != nil {
			d.Store.IncAnalyzed()
		}
		if len(verdict.DegradedLayers) > 0 {
			d.Logger.Warn("ai analyze degraded", "callId", req.CallID, "layers", verdict.DegradedLayers)
		}

		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		publishVerdictEvent(d, tenantID, req, verdict)
		bumpCounters(d, verdict)
		go persistCallLog(d, tenantID, uid, req, verdict)
		go persistSample(d, tenantID, req, verdict)

		ok(w, verdict)
	}
}

// ── helpers ──────────────────────────────────────────────────

// qwenOptions 读 agent_config(key=qwen) 作为 LLM 参数透传给 AI；
// 未配置 / 读失败返回 nil（AI 用自己的 env 默认值）。
func qwenOptions(ctx context.Context, d Deps) json.RawMessage {
	if d.Repo == nil {
		return nil
	}
	cfgCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	a, err := d.Repo.GetAgentConfig(cfgCtx, "qwen")
	if err != nil {
		return nil
	}
	return a.Value
}

// maskPhone 中间四位脱敏，避免 SSE 事件 payload 含完整手机号广播给跨租户订阅者。
// "13800138000" → "138***8000"；过短的串原样返回。
func maskPhone(p string) string {
	n := len(p)
	if n <= 7 {
		return p
	}
	return p[:3] + strings.Repeat("*", n-7) + p[n-4:]
}

func publishVerdictEvent(d Deps, tenantID string, req aiclient.AnalyzeRequest, v aiclient.AnalyzeResponse) {
	if d.Hub == nil {
		return
	}
	// 注意：v.Action 是小写设备指令（block/alert/pass），分档用 v.RiskLevel（BLOCK/ALERT/WATCH/SAFE）
	level := "info"
	switch v.RiskLevel {
	case "ALERT", "BLOCK":
		level = "danger"
	case "WATCH":
		level = "warn"
	}
	payload, _ := json.Marshal(map[string]any{
		"callId":      v.CallID,
		"shownNumber": maskPhone(req.ShownNumber),
		"riskScore":   v.RiskScore,
		"riskLevel":   v.RiskLevel,
		"verdict":     v.Action,
	})
	d.Hub.Publish(feed.Event{
		ID:        "ev_" + uuid.NewString(),
		Timestamp: time.Now(),
		Side:      "trace",
		Verb:      v.Action,
		Level:     level,
		Payload:   string(payload),
		TenantID:  tenantID, // SSE 订阅者按租户隔离
	})
}

func bumpCounters(d Deps, v aiclient.AnalyzeResponse) {
	if d.Store == nil {
		return
	}
	d.Store.IncIntercepted(1)
	if v.RiskLevel == "BLOCK" {
		d.Store.IncBlocked(1)
	}
	if v.Voiceprint != nil && v.Voiceprint.Verdict == "SYNTH" {
		d.Store.IncAIClones(1)
	}
	if v.Script != nil && len(v.Script.Hits) > 0 {
		d.Store.IncScriptHits(1)
	}
}

func persistCallLog(d Deps, tenantID, userID string, req aiclient.AnalyzeRequest, v aiclient.AnalyzeResponse) {
	if d.Repo == nil || tenantID == "" {
		return
	}
	ctx, cancel := makeBgCtx()
	defer cancel()
	trace, _ := json.Marshal(v.Trace)
	voice, _ := json.Marshal(v.Voiceprint)
	script, _ := json.Marshal(v.Script)
	id := "cl_" + uuid.NewString()
	_, err := d.Repo.CreateCallLog(ctx, repo.CreateCallLogParams{
		ID: id, TenantID: tenantID, UserID: userID,
		Phone:     req.ShownNumber,
		Region:    req.SignalOriginCC,
		Duration:  formatSeconds(req.AudioSeconds),
		Verdict:   riskLevelToCN(v.RiskLevel),
		Reason:    v.Trace.Note,
		RiskScore: v.RiskScore,
		TraceJSON: trace, VoiceprintJSON: voice, ScriptJSON: script,
	})
	if err != nil {
		d.Logger.Warn("persist call log failed", "err", err)
	}
}

// persistSample 可疑通话（ALERT/BLOCK 且有转写）自动进样本审核队列，
// 供 sysadmin 在「样本审核」页人工复核 → 派生规则/知识。写失败仅 log。
func persistSample(d Deps, tenantID string, req aiclient.AnalyzeRequest, v aiclient.AnalyzeResponse) {
	if d.Repo == nil || v.RiskScore < 65 || strings.TrimSpace(req.TranscriptHint) == "" {
		return
	}
	ctx, cancel := makeBgCtx()
	defer cancel()
	classification := ""
	if v.Script != nil && len(v.Script.Hits) > 0 {
		classification = v.Script.Hits[0].Category
	}
	callID := v.CallID
	if callID == "" {
		callID = req.CallID
	}
	_, err := d.Repo.CreateSample(ctx, repo.CreateSampleParams{
		ID:             "smp_" + uuid.NewString(),
		CallID:         callID,
		Transcript:     req.TranscriptHint,
		Duration:       formatSeconds(req.AudioSeconds),
		Origin:         req.SignalOriginCC,
		Classification: classification,
		Status:         "待审核",
		TenantID:       tenantID,
	})
	if err != nil {
		d.Logger.Warn("persist sample failed", "err", err)
	}
}

// riskLevelToCN 将 AI 返回的 riskLevel（BLOCK/ALERT/WATCH/SAFE）映射为 call_logs 中文判定。
func riskLevelToCN(level string) string {
	switch level {
	case "BLOCK":
		return "拦截"
	case "ALERT", "WATCH":
		return "预警"
	default:
		return "通过"
	}
}

func formatSeconds(s float64) string {
	if s <= 0 {
		return ""
	}
	mm := int(s) / 60
	ss := int(s) % 60
	if mm == 0 {
		return fmt.Sprintf("%02ds", ss)
	}
	return fmt.Sprintf("%02dm%02ds", mm, ss)
}

// makeBgCtx 用于异步副作用，独立于请求 ctx（请求 ctx 可能在响应后被取消）
func makeBgCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
