package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// RiskLevelRouter L1-L5 当前等级 + 自定义规则
func RiskLevelRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Route("/state", func(r chi.Router) {
		r.Get("/", getRiskState(d))
		r.Put("/", updateRiskState(d))
	})
	r.Route("/rules", func(r chi.Router) {
		r.Get("/", listRiskRules(d))
		r.Post("/", createRiskRule(d))
		r.Route("/{id}", func(r chi.Router) {
			r.Put("/", updateRiskRule(d))
			r.Delete("/", deleteRiskRule(d))
		})
	})
	return r
}

func getRiskState(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		s, err := d.Repo.GetRiskLevelState(r.Context(), tenantID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				ok(w, map[string]any{"tenantId": tenantID, "activeLevel": 3})
				return
			}
			internalErr(w)
			return
		}
		ok(w, s)
	}
}

type riskStateInput struct {
	ActiveLevel int `json:"activeLevel"`
}

func updateRiskState(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req riskStateInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.ActiveLevel < 1 || req.ActiveLevel > 5 {
			badRequest(w, "VALIDATION_FAILED", "activeLevel 须 1-5")
			return
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		s, err := d.Repo.UpsertRiskLevelState(r.Context(), tenantID, req.ActiveLevel)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, s)
	}
}

func listRiskRules(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		level := 0
		if l := r.URL.Query().Get("level"); l != "" {
			if v, err := strconv.Atoi(l); err == nil {
				level = v
			}
		}
		p := parsePage(r)
		rows, total, err := d.Repo.ListRiskLevelRules(r.Context(), tenantID, level, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

type riskRuleInput struct {
	Level   int    `json:"level"`
	Keyword string `json:"keyword"`
	Weight  int    `json:"weight"`
	Enabled bool   `json:"enabled"`
}

func createRiskRule(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req riskRuleInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Level < 1 || req.Level > 5 || req.Keyword == "" {
			badRequest(w, "VALIDATION_FAILED", "level 1-5 + keyword 必填")
			return
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		s, err := d.Repo.CreateRiskLevelRule(r.Context(), repo.CreateRiskLevelRuleParams{
			ID: "rlr_" + uuid.NewString(), TenantID: tenantID,
			Level: req.Level, Keyword: req.Keyword, Weight: req.Weight, Enabled: req.Enabled,
		})
		if err != nil {
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: s})
	}
}

func updateRiskRule(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		var req riskRuleInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		s, err := d.Repo.UpdateRiskLevelRule(r.Context(), repo.CreateRiskLevelRuleParams{
			ID: id, TenantID: tenantID,
			Level: req.Level, Keyword: req.Keyword, Weight: req.Weight, Enabled: req.Enabled,
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

func deleteRiskRule(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		if err := d.Repo.DeleteRiskLevelRule(r.Context(), id, tenantID); err != nil {
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
