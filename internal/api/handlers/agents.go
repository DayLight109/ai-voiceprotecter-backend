package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// AgentsRouter 智能体配置：display_words / whisper / qwen。
// GET 任意角色可看；PUT 仅 sysadmin / admin 可写（前端 LLM 等敏感参数）。
func AgentsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/display-words", getAgent(d, "display_words"))
	r.With(middleware.RequireRole("sysadmin", "admin")).Put("/display-words", putAgent(d, "display_words"))
	r.Get("/whisper", getAgent(d, "whisper"))
	r.With(middleware.RequireRole("sysadmin", "admin")).Put("/whisper", putAgent(d, "whisper"))
	r.Get("/qwen", getAgent(d, "qwen"))
	r.With(middleware.RequireRole("sysadmin", "admin")).Put("/qwen", putAgent(d, "qwen"))
	return r
}

func getAgent(d Deps, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := d.Repo.GetAgentConfig(r.Context(), key)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				ok(w, map[string]any{"key": key, "value": json.RawMessage("null")})
				return
			}
			internalErr(w)
			return
		}
		ok(w, a)
	}
}

func putAgent(d Deps, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 契约：请求体 {"value": <json>}，只存内层 value（历史版本误存整个
		// 信封导致 GET 双重嵌套，迁移 0013 已清洗存量数据）。
		var req struct {
			Value json.RawMessage `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体必须是 {\"value\": <json>}")
			return
		}
		if len(req.Value) == 0 || string(req.Value) == "null" {
			badRequest(w, "VALIDATION_FAILED", "value 必填")
			return
		}
		a, err := d.Repo.UpsertAgentConfig(r.Context(), key, req.Value)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, a)
	}
}
