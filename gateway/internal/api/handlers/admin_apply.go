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

func AdminApplyRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.With(middleware.RequireRole("sysadmin")).Get("/", listAdminApply(d))
	r.Get("/status", getMyAdminApply(d))
	r.Post("/", submitAdminApply(d))
	r.Delete("/mine", withdrawAdminApply(d))
	r.With(middleware.RequireRole("sysadmin")).Put("/{id}/review", reviewAdminApply(d))
	return r
}

func listAdminApply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		p := parsePage(r)
		rows, total, err := d.Repo.ListAdminApplications(r.Context(), status, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

func getMyAdminApply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		a, err := d.Repo.GetLatestAdminApplicationByUser(r.Context(), uid)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				ok(w, map[string]any{"status": "none"})
				return
			}
			internalErr(w)
			return
		}
		ok(w, a)
	}
}

type adminApplyInput struct {
	Scope   string `json:"scope"`   // family | biz
	Reason  string `json:"reason"`
	Contact string `json:"contact"`
}

func submitAdminApply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req adminApplyInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Scope != "family" && req.Scope != "biz" {
			badRequest(w, "VALIDATION_FAILED", "scope 仅允许 family / biz")
			return
		}
		if req.Reason == "" || req.Contact == "" {
			badRequest(w, "VALIDATION_FAILED", "reason / contact 必填")
			return
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		// 已有待审核申请时拒绝重复提交（前端按 ADMIN_APPLY_PENDING 提示）
		if latest, err := d.Repo.GetLatestAdminApplicationByUser(r.Context(), uid); err == nil && latest.Status == "pending" {
			writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
				Code: "ADMIN_APPLY_PENDING", Message: "已存在审核中的申请，请等待结果或先撤回",
			}})
			return
		}
		a, err := d.Repo.CreateAdminApplication(r.Context(), repo.CreateAdminApplicationParams{
			ID: "aa_" + uuid.NewString(), UserID: uid,
			Scope: req.Scope, Reason: req.Reason, Contact: req.Contact,
		})
		if err != nil {
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: a})
	}
}

// withdrawAdminApply 撤回本人待审核中的申请（已审结的不可撤回）。
func withdrawAdminApply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		n, err := d.Repo.DeletePendingAdminApplicationsByUser(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		if n == 0 {
			notFoundErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type adminApplyReview struct {
	Status string `json:"status"` // approved | rejected
}

func reviewAdminApply(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req adminApplyReview
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Status != "approved" && req.Status != "rejected" {
			badRequest(w, "VALIDATION_FAILED", "status 仅允许 approved / rejected")
			return
		}
		reviewerUID, _ := r.Context().Value(middleware.CtxUserID).(string)

		// 先取申请记录，做合法性校验，再落盘 —— 避免审批失败后状态已被改成
		// approved 但实际并未升权。
		existing, err := d.Repo.GetAdminApplicationByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if existing.UserID == reviewerUID {
			writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
				Code: "RBAC_FORBIDDEN", Message: "不可审批自己的申请",
			}})
			return
		}
		if existing.Status != "pending" {
			writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
				Code: "ADMIN_APPLY_ALREADY_REVIEWED", Message: "该申请已被审核",
			}})
			return
		}

		// approved 时：必须校验申请人 tenant kind 与 scope 一致，
		// 防止 family 用户填 scope=biz 直接升 admin。
		if req.Status == "approved" {
			applicant, err := d.Repo.GetUserByID(r.Context(), existing.UserID)
			if err != nil {
				if errors.Is(err, repo.ErrNotFound) {
					writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
						Code: "APPLICANT_NOT_FOUND", Message: "申请人已不存在",
					}})
					return
				}
				internalErr(w)
				return
			}
			tenant, err := d.Repo.GetTenantByID(r.Context(), applicant.TenantID)
			if err != nil {
				internalErr(w)
				return
			}
			scopeOK := (existing.Scope == "family" && tenant.Kind == "family") ||
				(existing.Scope == "biz" && tenant.Kind == "enterprise")
			if !scopeOK {
				writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
					Code: "ADMIN_APPLY_SCOPE_MISMATCH",
					Message: "申请 scope 与申请人所属租户类型不一致",
				}})
				return
			}
		}

		a, err := d.Repo.UpdateAdminApplicationStatus(r.Context(), id, req.Status, reviewerUID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}

		// approved 时升级申请人 role
		if req.Status == "approved" {
			newRole := "family_admin"
			if a.Scope == "biz" {
				newRole = "admin"
			}
			cur, err := d.Repo.GetUserByID(r.Context(), a.UserID)
			if err == nil {
				_, _ = d.Repo.UpdateUser(r.Context(), repo.UpdateUserParams{
					ID: cur.ID, Name: cur.Name, Dept: cur.Dept, Role: newRole, Status: cur.Status,
				})
			}
		}
		ok(w, a)
	}
}
