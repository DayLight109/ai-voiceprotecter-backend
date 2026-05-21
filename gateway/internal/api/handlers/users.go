package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/auth"
	"github.com/sentinel/gateway/internal/repo"
)

// UsersRouter 家庭 / 企业 成员管理。family_admin / admin / sysadmin 才能写。
func UsersRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listUsers(d))
	r.With(middleware.RequireRole("family_admin", "admin", "sysadmin")).Post("/", createUser(d))
	r.Route("/{id}", func(r chi.Router) {
		r.With(middleware.RequireRole("family_admin", "admin", "sysadmin")).Put("/", updateUser(d))
		r.With(middleware.RequireRole("family_admin", "admin", "sysadmin")).Delete("/", deleteUser(d))
	})
	return r
}

// manageableRoles 定义"actor 可管理哪些 target.Role"。
// 同时也用于 createUser 限制可创建的初始 role —— 防止 family_admin / admin
// 自创 sysadmin 账户实现权限提升。
var manageableRoles = map[string]map[string]bool{
	"family_admin": {"family": true},
	"admin":        {"biz": true},
	"sysadmin":     {"family": true, "biz": true, "family_admin": true, "admin": true, "sysadmin": true},
}

func canManageRole(actor, target string) bool {
	if m, ok := manageableRoles[actor]; ok {
		return m[target]
	}
	return false
}

// defaultRoleFor 返回 actor 的默认下属角色（用于 createUser 未传 role 的兜底）。
func defaultRoleFor(actor string) string {
	switch actor {
	case "family_admin":
		return "family"
	case "admin":
		return "biz"
	default:
		return "family"
	}
}

func listUsers(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		p := parsePage(r)
		users, total, err := d.Repo.ListUsersByTenant(r.Context(), tenantID, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(users), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

type userInput struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Dept     string `json:"dept"`
	Status   string `json:"status"`
}

func forbid(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{Code: "RBAC_FORBIDDEN", Message: msg}})
}

func createUser(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req userInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Name == "" || req.Password == "" {
			badRequest(w, "VALIDATION_FAILED", "name / password 必填")
			return
		}
		actorRole, _ := r.Context().Value(middleware.CtxRole).(string)
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role := req.Role
		if role == "" {
			role = defaultRoleFor(actorRole)
		}
		if !canManageRole(actorRole, role) {
			forbid(w, "无权创建该角色的用户")
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			internalErr(w)
			return
		}
		u, err := d.Repo.CreateUser(r.Context(), repo.CreateUserParams{
			ID: "u_" + uuid.NewString(), TenantID: tenantID,
			Name: req.Name, Phone: req.Phone, Email: req.Email, PasswordHash: hash,
			Role: role, Status: req.Status, Dept: req.Dept,
		})
		if err != nil {
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "USER_DUPLICATE", Message: "phone / email 已被占用",
				}})
				return
			}
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: toPublicUser(u)})
	}
}

func updateUser(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req userInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		actorRole, _ := r.Context().Value(middleware.CtxRole).(string)
		actorTenant, _ := r.Context().Value(middleware.CtxTenantID).(string)

		// 取目标用户，校验跨租户 + 角色等级
		target, err := d.Repo.GetUserByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if target.TenantID != actorTenant && actorRole != "sysadmin" {
			forbid(w, "不可修改其它租户的成员")
			return
		}
		if !canManageRole(actorRole, target.Role) {
			forbid(w, "无权修改该角色的用户")
			return
		}
		// 若试图改 role，校验新 role 也在 actor 可管理范围内（防降权再提权）
		newRole := req.Role
		if newRole == "" {
			newRole = target.Role
		}
		if newRole != target.Role && !canManageRole(actorRole, newRole) {
			forbid(w, "无权赋予该角色")
			return
		}
		u, err := d.Repo.UpdateUser(r.Context(), repo.UpdateUserParams{
			ID: id, Name: req.Name, Dept: req.Dept, Role: newRole, Status: req.Status,
		})
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, toPublicUser(u))
	}
}

func deleteUser(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		actorRole, _ := r.Context().Value(middleware.CtxRole).(string)
		actorTenant, _ := r.Context().Value(middleware.CtxTenantID).(string)
		actorUid, _ := r.Context().Value(middleware.CtxUserID).(string)

		target, err := d.Repo.GetUserByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if target.ID == actorUid {
			forbid(w, "不可删除自己")
			return
		}
		if target.TenantID != actorTenant && actorRole != "sysadmin" {
			forbid(w, "不可删除其它租户的成员")
			return
		}
		if !canManageRole(actorRole, target.Role) {
			forbid(w, "无权删除该角色的用户")
			return
		}
		if err := d.Repo.DeleteUser(r.Context(), id); err != nil {
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
