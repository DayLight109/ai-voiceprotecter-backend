package handlers

import (
	"encoding/json"
	"errors"
	"io"
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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			badRequest(w, "VALIDATION_FAILED", "读取请求体失败")
			return
		}
		// 校验是合法 JSON
		var probe any
		if err := json.Unmarshal(body, &probe); err != nil {
			badRequest(w, "VALIDATION_FAILED", "value 必须是合法 JSON")
			return
		}
		a, err := d.Repo.UpsertAgentConfig(r.Context(), key, body)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, a)
	}
}
