package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// RulesRouter — scam_rules 系统全局规则库。仅 sysadmin 可写。
func RulesRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listRules(d))
	r.With(middleware.RequireRole("sysadmin")).Post("/", createRule(d))
	r.Route("/{id}", func(r chi.Router) {
		r.With(middleware.RequireRole("sysadmin")).Put("/", updateRule(d))
		r.With(middleware.RequireRole("sysadmin")).Delete("/", deleteRule(d))
	})
	return r
}

func listRules(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.URL.Query().Get("category")
		p := parsePage(r)
		rows, total, err := d.Repo.ListScamRules(r.Context(), category, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

type ruleInput struct {
	Category string `json:"category"`
	Keyword  string `json:"keyword"`
	Weight   int    `json:"weight"`
	Enabled  bool   `json:"enabled"`
}

func createRule(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ruleInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Category == "" || req.Keyword == "" {
			badRequest(w, "VALIDATION_FAILED", "category / keyword 必填")
			return
		}
		if req.Weight < 0 || req.Weight > 100 {
			badRequest(w, "VALIDATION_FAILED", "weight 须 0-100")
			return
		}
		s, err := d.Repo.CreateScamRule(r.Context(), repo.CreateScamRuleParams{
			ID: "sr_" + uuid.NewString(),
			Category: req.Category, Keyword: req.Keyword,
			Weight: req.Weight, Enabled: req.Enabled,
		})
		if err != nil {
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "RULE_DUPLICATE", Message: "同一 category+keyword 已存在",
				}})
				return
			}
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: s})
	}
}

func updateRule(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req ruleInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		s, err := d.Repo.UpdateScamRule(r.Context(), repo.CreateScamRuleParams{
			ID: id, Category: req.Category, Keyword: req.Keyword,
			Weight: req.Weight, Enabled: req.Enabled,
		})
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, s)
	}
}

func deleteRule(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := d.Repo.DeleteScamRule(r.Context(), id); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
