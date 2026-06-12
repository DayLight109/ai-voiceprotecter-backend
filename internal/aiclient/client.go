// Package aiclient 调用 Python AI 子服务的 HTTP 客户端。
//
// 调用关系：
//   gateway/handlers/analyze.go → aiclient.Analyze → POST {AI_BASE_URL}/v1/analyze
package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
	log     *slog.Logger
}

func New(baseURL string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: timeout},
		log:     log,
	}
}

// ── DTO ──────────────────────────────────────────────────

type AnalyzeRequest struct {
	CallID         string  `json:"callId"`
	ShownNumber    string  `json:"shownNumber"`
	SignalOriginCC string  `json:"signalOriginCC"`
	AudioSeconds   float64 `json:"audioSeconds"`
	TranscriptHint string  `json:"transcriptHint"`
	// LLMOptions 透传 sysadmin 在「智能体管理」页保存的千问参数
	// （agent_config key=qwen 的 value 原样转发；AI 侧按需覆盖默认配置）。
	LLMOptions json.RawMessage `json:"llmOptions,omitempty"`
}

type AnalyzeResponse struct {
	CallID        string         `json:"callId"`
	Timestamp     time.Time      `json:"ts"`
	Trace         TraceVerdict   `json:"trace"`
	Voiceprint    *VoiceVerdict  `json:"voiceprint"`  // 降级时为 nil
	Script        *ScriptVerdict `json:"script"`      // 降级时为 nil
	RiskScore     int            `json:"riskScore"`
	RiskLevel     string         `json:"riskLevel"`
	Action        string         `json:"action"`
	LatencyMillis int64          `json:"latencyMillis"`
	// 非空表示 AI 侧缺层降级判决（缺失层及原因）
	DegradedLayers []DegradedLayer `json:"degradedLayers,omitempty"`
}

type DegradedLayer struct {
	Layer  string `json:"layer"`
	Reason string `json:"reason"`
}

type TraceVerdict struct {
	ShownRegistry string `json:"shownRegistry"`
	ActualOrigin  string `json:"actualOrigin"`
	Mismatch      bool   `json:"mismatch"`
	HopCount      int    `json:"hopCount"`
	Risk          int    `json:"risk"`
	Note          string `json:"note"`
}

type VoiceVerdict struct {
	SynthProbability float64 `json:"synthProbability"`
	F0Jitter         float64 `json:"f0Jitter"`
	BreathScore      float64 `json:"breathScore"`
	Regularity       float64 `json:"regularity"`
	Risk             int     `json:"risk"`
	Verdict          string  `json:"verdict"`
}

type ScriptVerdict struct {
	Hits []ScriptHit `json:"hits"`
	Risk int         `json:"risk"`
}

type ScriptHit struct {
	Category string `json:"category"`
	Phrase   string `json:"phrase"`
	Weight   int    `json:"weight"`
}

// ── 方法 ─────────────────────────────────────────────────

func (c *Client) Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResponse, error) {
	var out AnalyzeResponse
	if err := c.postJSON(ctx, "/v1/analyze", req, &out); err != nil {
		return AnalyzeResponse{}, err
	}
	return out, nil
}

// 也可单独调用某一层（后续可用）
func (c *Client) Transcribe(ctx context.Context, audioKey string) (string, error) {
	var out struct {
		Text string `json:"text"`
	}
	body := map[string]string{"audioKey": audioKey}
	if err := c.postJSON(ctx, "/v1/transcribe", body, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

// Classify 调 AI /v1/classify，返回命中数组。llmOptions 可为 nil（用 AI 默认配置）。
func (c *Client) Classify(ctx context.Context, transcript string, llmOptions json.RawMessage) ([]ScriptHit, error) {
	var out ScriptVerdict
	body := map[string]any{"transcript": transcript}
	if len(llmOptions) > 0 {
		body["llmOptions"] = llmOptions
	}
	if err := c.postJSON(ctx, "/v1/classify", body, &out); err != nil {
		return nil, err
	}
	return out.Hits, nil
}

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ai upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return errors.New("ai upstream " + resp.Status + ": " + string(raw))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
