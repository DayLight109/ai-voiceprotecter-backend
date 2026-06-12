package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/auth"
	"github.com/sentinel/gateway/internal/repo"
)

// me.go — 个人中心 /me/* 端点（settings 页驱动）：
//   PUT    /me, /me/            更新资料
//   PUT    /me/password         改密码
//   GET    /me/sessions         登录设备列表
//   DELETE /me/sessions/{token} 撤销指定会话
//   DELETE /me/sessions/others  撤销其余会话
//   PUT    /me/avatar           上传头像（multipart file）
//   GET    /me/avatar           读取头像（bytes）
//   DELETE /me/avatar           删除头像

const maxAvatarBytes = 2 << 20 // 2 MiB

// ── 更新资料 ──────────────────────────────────────────────────

type updateMeRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
	Dept  string `json:"dept"`
}

func UpdateMe(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		// 读现有资料：前端是部分字段 patch，缺省字段沿用旧值
		cur, err := d.Repo.GetUserByID(r.Context(), uid)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		var req updateMeRequest
		req.Name, req.Phone, req.Email, req.Dept = cur.Name, cur.Phone, cur.Email, cur.Dept
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Phone = strings.TrimSpace(req.Phone)
		req.Email = strings.TrimSpace(req.Email)
		req.Dept = strings.TrimSpace(req.Dept)
		if req.Name == "" {
			badRequest(w, "VALIDATION_FAILED", "name 不能为空")
			return
		}
		u, err := d.Repo.UpdateUserProfile(r.Context(), uid, req.Name, req.Phone, req.Email, req.Dept)
		if err != nil {
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "USER_DUPLICATE", Message: "phone 或 email 已被占用",
				}})
				return
			}
			d.Logger.Error("updateMe", "err", err)
			internalErr(w)
			return
		}
		has, _ := d.Repo.UserHasAvatar(r.Context(), uid)
		ok(w, withAvatar(toPublicUser(u), has))
	}
}

// ── 改密码 ────────────────────────────────────────────────────

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func ChangePassword(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		var req changePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.OldPassword == "" || req.NewPassword == "" {
			badRequest(w, "VALIDATION_FAILED", "oldPassword / newPassword 必填")
			return
		}
		if len(req.NewPassword) < 8 {
			badRequest(w, "VALIDATION_FAILED", "新密码至少 8 位")
			return
		}
		u, err := d.Repo.GetUserByID(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		if !auth.VerifyPassword(u.PasswordHash, req.OldPassword) {
			writeJSON(w, http.StatusUnauthorized, ErrEnvelope{Error: ErrBody{
				Code: "AUTH_INVALID_CREDENTIALS", Message: "原密码错误",
			}})
			return
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			internalErr(w)
			return
		}
		if err := d.Repo.SetUserPassword(r.Context(), uid, hash); err != nil {
			d.Logger.Error("changePassword", "err", err)
			internalErr(w)
			return
		}
		// 改密后撤销其余会话，仅保留当前（sid = 所属 refresh 会话 jti；
		// 旧 token 无 sid 时传空串 → 全部撤销，下次请求重新登录，安全侧倾斜）
		sid, _ := r.Context().Value(middleware.CtxSID).(string)
		_, _ = d.Repo.RevokeSessionsExcept(r.Context(), uid, sid)
		ok(w, map[string]bool{"ok": true})
	}
}
