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

func AppealsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listAppeals(d))
	r.Post("/", createAppeal(d))
	r.With(middleware.RequireRole("family_admin", "admin", "sysadmin")).
		Put("/{id}/status", reviewAppeal(d))
	return r
}

func listAppeals(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		filterTenant := tenantID
		if role == "sysadmin" {
			filterTenant = "" // sysadmin 看全部
		}
		status := r.URL.Query().Get("status")
		p := parsePage(r)
		rows, total, err := d.Repo.ListAppeals(r.Context(), filterTenant, status, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

type appealInput struct {
	Type   string `json:"type"`
	Number string `json:"number"`
	Reason string `json:"reason"`
}

func createAppeal(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req appealInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Type == "" || req.Number == "" || req.Reason == "" {
			badRequest(w, "VALIDATION_FAILED", "type / number / reason 必填")
			return
		}
		if req.Type != "误判申诉" && req.Type != "号码举报" {
			badRequest(w, "VALIDATION_FAILED", "type 仅允许 误判申诉 / 号码举报")
			return
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		a, err := d.Repo.CreateAppeal(r.Context(), repo.CreateAppealParams{
			ID: "ap_" + uuid.NewString(),
			UserID: uid, TenantID: tenantID,
			Type: req.Type, Number: req.Number, Reason: req.Reason,
		})
		if err != nil {
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: a})
	}
}

type appealReview struct {
	Status string `json:"status"` // 已通过 | 已驳回
}

func reviewAppeal(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req appealReview
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Status != "已通过" && req.Status != "已驳回" {
			badRequest(w, "VALIDATION_FAILED", "status 仅允许 已通过 / 已驳回")
			return
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		a, err := d.Repo.UpdateAppealStatus(r.Context(), id, req.Status, uid)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, a)
	}
}
