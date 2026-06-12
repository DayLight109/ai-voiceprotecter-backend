package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// warroom.go — 指挥中心聚合端点（前端 api.warroom.* 消费，envelope 包装）。
//
//	GET /api/v1/warroom/overview          计数器 + hub + 引擎 + 运行时快照
//	GET /api/v1/warroom/voiceprint/latest 最近一次声纹判决（无记录返回 null）
//
// 鉴权由路由层 Auth + Tenant 完成；voiceprint/latest 按租户过滤，sysadmin 看全部。

func WarroomOverview(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		s := d.Store.Snapshot()

		hub := map[string]any{"subscribers": 0, "buffered": 0, "lastEventAt": ""}
		if d.Hub != nil {
			subs, buffered, lastAt := d.Hub.Stats()
			hub["subscribers"], hub["buffered"] = subs, buffered
			if !lastAt.IsZero() {
				hub["lastEventAt"] = lastAt.UTC().Format(time.RFC3339)
			}
		}

		analyzed, failed, lastAnalyzed := d.Store.EngineStats()
		engine := map[string]any{"analyzed": analyzed, "failed": failed, "lastAnalyzedAt": ""}
		if !lastAnalyzed.IsZero() {
			engine["lastAnalyzedAt"] = lastAnalyzed.UTC().Format(time.RFC3339)
		}

		rt := map[string]any{
			"cpuPct": 0.0, "memPct": 0.0, "netRxBps": 0.0, "netTxBps": 0.0,
			"goroutines": 0, "sampledAt": "",
		}
		if d.Ops != nil {
			if smp, okSample := d.Ops.Latest(); okSample {
				rt["cpuPct"], rt["memPct"] = smp.CPUPct, smp.MemPct
				rt["netRxBps"], rt["netTxBps"] = smp.NetRxBps, smp.NetTxBps
				rt["goroutines"] = smp.Goroutines
				rt["sampledAt"] = smp.T.UTC().Format(time.RFC3339)
			}
		}

		ok(w, map[string]any{
			"counters": map[string]any{
				"interceptedCalls": s.InterceptedCalls,
				"blockedCalls":     s.BlockedCalls,
				"aiCloneDetected":  s.AICloneDetected,
				"scriptHits":       s.ScriptHits,
				"smsBlocked":       s.SmsBlocked,
				"fundsHeldYuan":    s.FundsHeldYuan,
			},
			"defcon":  s.Defcon,
			"since":   s.Since.UTC().Format(time.RFC3339),
			"nowUtc":  s.NowUTC.Format(time.RFC3339),
			"hub":     hub,
			"engine":  engine,
			"runtime": rt,
		})
	}
}

// verdictToLevel call_logs 中文判定 → 风险分档（与 AI riskLevel 对齐的近似映射）。
func verdictToLevel(verdict string) string {
	switch verdict {
	case "拦截":
		return "BLOCK"
	case "预警":
		return "ALERT"
	default:
		return "SAFE"
	}
}

func WarroomLatestVoiceprint(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		filter := tenantID
		if role == "sysadmin" {
			filter = ""
		}
		v, err := d.Repo.GetLatestVoiceprintCallLog(r.Context(), filter)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				ok(w, nil) // 尚无声纹判决
				return
			}
			d.Logger.Error("warroom latest voiceprint", "err", err)
			internalErr(w)
			return
		}
		ok(w, map[string]any{
			"callId":     v.ID,
			"ts":         v.CreatedAt.UTC().Format(time.RFC3339),
			"riskScore":  v.RiskScore,
			"riskLevel":  verdictToLevel(v.Verdict),
			"voiceprint": json.RawMessage(v.VoiceprintJSON),
		})
	}
}
