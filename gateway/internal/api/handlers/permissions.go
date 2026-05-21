package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// PermissionsRouter 家庭 / 企业各自一套位图。
//
// 存储：permissions(user_id, key, enabled)。前缀 family. / biz. 区分两套。
//
// 安全要点：
//   - GET 读自己的位图（任何已登录用户）。
//   - PUT 仅 family_admin/admin/sysadmin 可写；body 必传 targetUserId 指定写谁。
//     防止普通成员自助打开"管理员功能"位图绕过 admin-apply 审批流。
//   - 跨租户写入由 sysadmin 之外的角色拒绝。
func PermissionsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/family", famPermsGet(d))
	r.With(middleware.RequireRole("family_admin", "admin", "sysadmin")).Put("/family", famPermsPut(d))
	r.Get("/biz", bizPermsGet(d))
	r.With(middleware.RequireRole("family_admin", "admin", "sysadmin")).Put("/biz", bizPermsPut(d))
	return r
}

func filterByPrefix(rows []repo.Permission, prefix string) []repo.Permission {
	out := make([]repo.Permission, 0, len(rows))
	for _, r := range rows {
		if strings.HasPrefix(r.Key, prefix) {
			out = append(out, r)
		}
	}
	return out
}

func famPermsGet(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		rows, err := d.Repo.ListPermissions(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, filterByPrefix(rows, "family."))
	}
}

func bizPermsGet(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		rows, err := d.Repo.ListPermissions(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, filterByPrefix(rows, "biz."))
	}
}

type permsBody struct {
	TargetUserID string            `json:"targetUserId"` // 必传：要写权限的用户 id
	Items        []repo.Permission `json:"items"`
}

func famPermsPut(d Deps) http.HandlerFunc { return permsPut(d, "family.") }
func bizPermsPut(d Deps) http.HandlerFunc { return permsPut(d, "biz.") }

func permsPut(d Deps, prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req permsBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.TargetUserID == "" {
			badRequest(w, "VALIDATION_FAILED", "targetUserId 必填")
			return
		}
		for _, p := range req.Items {
			if !strings.HasPrefix(p.Key, prefix) {
				badRequest(w, "VALIDATION_FAILED", "key 必须以 "+prefix+" 开头")
				return
			}
		}
		actorRole, _ := r.Context().Value(middleware.CtxRole).(string)
		actorTenant, _ := r.Context().Value(middleware.CtxTenantID).(string)

		// 校验目标用户存在且同租户（sysadmin 例外）
		target, err := d.Repo.GetUserByID(r.Context(), req.TargetUserID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if target.TenantID != actorTenant && actorRole != "sysadmin" {
			forbid(w, "不可修改其它租户成员的权限位图")
			return
		}

		if err := d.Repo.UpsertPermissions(r.Context(), req.TargetUserID, req.Items); err != nil {
			internalErr(w)
			return
		}
		ok(w, req.Items)
	}
}
