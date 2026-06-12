package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/auth"
	"github.com/sentinel/gateway/internal/repo"
)

// AuthRouter 暴露认证相关接口（无需鉴权）。
func AuthRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Post("/login", authLogin(d))
	r.Post("/register", authRegister(d))
	r.Post("/refresh", authRefresh(d))
	r.Post("/logout", authLogout(d))
	// 证件 / 活体 / 指纹 走骨架占位，由前端独立流程或第三方完成
	r.Post("/verify-id", func(w http.ResponseWriter, _ *http.Request) { notImplemented(w) })
	r.Post("/verify-liveness", func(w http.ResponseWriter, _ *http.Request) { notImplemented(w) })
	r.Post("/verify-fingerprint", func(w http.ResponseWriter, _ *http.Request) { notImplemented(w) })
	r.Get("/me", Me(d))
	return r
}

// ─── 登录 ────────────────────────────────────────────────────

type loginRequest struct {
	Account  string `json:"account"`  // phone 或 email
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken  string         `json:"accessToken"`
	RefreshToken string         `json:"refreshToken"`
	ExpiresAt    time.Time      `json:"expiresAt"`
	User         publicUser     `json:"user"`
}

type publicUser struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenantId"`
	Name      string `json:"name"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	Dept      string `json:"dept,omitempty"`
	HasAvatar bool   `json:"hasAvatar"`
}

func toPublicUser(u repo.UserRow) publicUser {
	return publicUser{
		ID: u.ID, TenantID: u.TenantID, Name: u.Name,
		Phone: u.Phone, Email: u.Email,
		Role: u.Role, Status: u.Status, Dept: u.Dept,
	}
}

// withAvatar 在 publicUser 上补 hasAvatar 标志（需查 user_avatars）。
func withAvatar(p publicUser, has bool) publicUser {
	p.HasAvatar = has
	return p
}

func authLogin(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		req.Account = strings.TrimSpace(req.Account)
		if req.Account == "" || req.Password == "" {
			badRequest(w, "VALIDATION_FAILED", "账号 / 密码必填")
			return
		}

		user, err := d.Repo.GetUserByAccount(r.Context(), req.Account)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeJSON(w, http.StatusUnauthorized, ErrEnvelope{Error: ErrBody{
					Code: "AUTH_INVALID_CREDENTIALS", Message: "账号或密码错误",
				}})
				return
			}
			d.Logger.Error("login query user", "err", err)
			internalErr(w)
			return
		}
		if user.Status == "suspended" {
			writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
				Code: "AUTH_ACCOUNT_SUSPENDED", Message: "账号已停用",
			}})
			return
		}
		if !auth.VerifyPassword(user.PasswordHash, req.Password) {
			writeJSON(w, http.StatusUnauthorized, ErrEnvelope{Error: ErrBody{
				Code: "AUTH_INVALID_CREDENTIALS", Message: "账号或密码错误",
			}})
			return
		}

		tokens, err := auth.Issue(d.Cfg.JWT, user.ID, user.Role, user.TenantID)
		if err != nil {
			d.Logger.Error("issue jwt", "err", err)
			internalErr(w)
			return
		}

		// 持久化 refresh session（access 过期前 jti 不入库；通过 Redis 黑名单按需撤销）
		hashHex := sha256Hex(tokens.RefreshToken)
		if err := d.Repo.CreateSession(r.Context(), repo.CreateSessionParams{
			JTI:              tokens.RefreshJTI,
			UserID:           user.ID,
			RefreshTokenHash: hashHex,
			ExpiresAt:        tokens.RefreshExp,
			UserAgent:        r.Header.Get("User-Agent"),
			IP:               clientIPHandler(r),
		}); err != nil {
			d.Logger.Error("create session", "err", err)
			internalErr(w)
			return
		}

		_ = d.Repo.TouchUserLogin(r.Context(), user.ID)

		ok(w, loginResponse{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresAt:    tokens.AccessExp,
			User:         toPublicUser(user),
		})
	}
}

// ─── 注册 ────────────────────────────────────────────────────

type registerRequest struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`     // family | biz；其余角色由审批升级
	TenantID string `json:"tenantId"` // 可选；空则按 role 选默认全局家庭/企业占位
}

func authRegister(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Phone = strings.TrimSpace(req.Phone)
		req.Email = strings.TrimSpace(req.Email)
		if req.Name == "" || req.Password == "" || (req.Phone == "" && req.Email == "") {
			badRequest(w, "VALIDATION_FAILED", "name / password / (phone 或 email) 必填")
			return
		}
		switch req.Role {
		case "family", "biz", "":
		default:
			badRequest(w, "VALIDATION_FAILED", "role 仅允许 family / biz")
			return
		}
		if req.Role == "" {
			req.Role = "family"
		}
		tenantID := req.TenantID
		if tenantID == "" {
			if req.Role == "biz" {
				tenantID = "default-enterprise"
			} else {
				tenantID = "default-family"
			}
		}
		// 确保租户存在
		if _, err := d.Repo.GetTenantByID(r.Context(), tenantID); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				kind := "family"
				if req.Role == "biz" {
					kind = "enterprise"
				}
				if _, e2 := d.Repo.CreateTenant(r.Context(), tenantID, kind, tenantID); e2 != nil {
					d.Logger.Error("create default tenant", "err", e2)
					internalErr(w)
					return
				}
			} else {
				d.Logger.Error("get tenant", "err", err)
				internalErr(w)
				return
			}
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			d.Logger.Error("hash password", "err", err)
			internalErr(w)
			return
		}
		uid := "u_" + uuid.NewString()
		user, err := d.Repo.CreateUser(r.Context(), repo.CreateUserParams{
			ID: uid, TenantID: tenantID, Name: req.Name,
			Phone: req.Phone, Email: req.Email, PasswordHash: hash,
			Role: req.Role, Status: "active",
		})
		if err != nil {
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "USER_DUPLICATE", Message: "phone 或 email 已注册",
				}})
				return
			}
			d.Logger.Error("create user", "err", err)
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: toPublicUser(user)})
	}
}

// ─── refresh / logout ────────────────────────────────────────

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func authRefresh(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
			badRequest(w, "VALIDATION_FAILED", "refreshToken 必填")
			return
		}
		claims, err := auth.VerifyRefresh(d.Cfg.JWT, req.RefreshToken)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, ErrEnvelope{Error: ErrBody{
				Code: "AUTH_INVALID_CREDENTIALS", Message: "refresh 无效或过期",
			}})
			return
		}
		sess, err := d.Repo.GetSession(r.Context(), claims.ID)
		if err != nil || sess.Revoked || sess.ExpiresAt.Before(time.Now()) {
			writeJSON(w, http.StatusUnauthorized, ErrEnvelope{Error: ErrBody{
				Code: "AUTH_TOKEN_REVOKED", Message: "refresh 已撤销",
			}})
			return
		}
		// 滚动颁新 access + 新 refresh，旧 refresh 撤销
		tokens, err := auth.Issue(d.Cfg.JWT, claims.UID, claims.Role, claims.TenantID)
		if err != nil {
			internalErr(w)
			return
		}
		_ = d.Repo.RevokeSession(r.Context(), claims.ID)
		_ = d.Repo.CreateSession(r.Context(), repo.CreateSessionParams{
			JTI: tokens.RefreshJTI, UserID: claims.UID,
			RefreshTokenHash: sha256Hex(tokens.RefreshToken),
			ExpiresAt:        tokens.RefreshExp,
			UserAgent:        r.Header.Get("User-Agent"),
			IP:               clientIPHandler(r),
		})
		ok(w, map[string]any{
			"accessToken":  tokens.AccessToken,
			"refreshToken": tokens.RefreshToken,
			"expiresAt":    tokens.AccessExp,
		})
	}
}

func authLogout(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 主动撤销当前 access (Redis JTI) + 当前用户所有 refresh
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		jti, _ := r.Context().Value(middleware.CtxJTI).(string)
		if uid == "" {
			ok(w, map[string]any{"ok": true}) // 未登录直接 200
			return
		}
		if d.Redis != nil && jti != "" {
			// access exp 由 jwt 自己控；这里用 30 分钟 ttl 兜住即可
			_ = d.Redis.RevokeJTI(r.Context(), jti, time.Now().Add(30*time.Minute))
		}
		_ = d.Repo.RevokeAllSessionsByUser(r.Context(), uid)
		ok(w, map[string]any{"ok": true})
	}
}

// Me 返回当前登录用户
func Me(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			writeJSON(w, http.StatusUnauthorized, ErrEnvelope{Error: ErrBody{
				Code: "AUTH_TOKEN_MISSING", Message: "未登录",
			}})
			return
		}
		u, err := d.Repo.GetUserByID(r.Context(), uid)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			d.Logger.Error("me query", "err", err)
			internalErr(w)
			return
		}
		has, _ := d.Repo.UserHasAvatar(r.Context(), uid)
		ok(w, withAvatar(toPublicUser(u), has))
	}
}

// ── helpers ──────────────────────────────────────────────────

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func clientIPHandler(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		for i := 0; i < len(ip); i++ {
			if ip[i] == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	return r.RemoteAddr
}
