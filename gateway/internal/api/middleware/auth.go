// Package middleware 集中放 chi 中间件。
//
// 装载顺序见 router.go:
//   RequestID → RealIP → Logging → Recoverer → CORS → RateLimit
//   ↓ 公开端点止于此 ↓
//   Auth → Tenant → Audit → handler
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/sentinel/gateway/internal/config"
	"github.com/sentinel/gateway/internal/store"
)

// ctxKey 避免和别人的 context key 撞车
type ctxKey int

const (
	CtxUserID ctxKey = iota + 1
	CtxRole
	CtxTenantID
	CtxJTI
	CtxSID // 所属 refresh 会话的 jti（sessions.jti），用于精确定位"当前设备"
)

// Claims 是 JWT payload
type Claims struct {
	UID      string `json:"uid"`
	Role     string `json:"role"`
	TenantID string `json:"tid"`
	SID      string `json:"sid,omitempty"` // access 所属 refresh 会话 jti
	jwt.RegisteredClaims
}

// TouchSession 会话活跃回调：Auth 验签成功后以 sid 调用（已节流），
// 由 router 注入 repo.TouchSessionLastSeen。nil = 不记录。
type TouchSession func(sid string)

// touchThrottle 同一会话 60s 内只回调一次，避免每个请求都写 DB。
const touchThrottle = time.Minute

// Auth 验签 access token，把 claims 注入 context。
//
//	失败：401 + AUTH_TOKEN_EXPIRED / AUTH_INVALID_CREDENTIALS / AUTH_TOKEN_REVOKED。
//	rds 用来检查 jti 黑名单（登出 / 重置密码时主动撤销）；nil 或不可用时跳过检查。
//	touch 见 TouchSession。
func Auth(cfg config.JWTConfig, rds *store.Redis, touch TouchSession) func(http.Handler) http.Handler {
	var lastTouch sync.Map // sid → time.Time
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearer(r)
			if raw == "" {
				writeAuthErr(w, "AUTH_TOKEN_MISSING", "缺少 Authorization Bearer token")
				return
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(cfg.AccessSecret), nil
			})

			if err != nil || !token.Valid {
				if errors.Is(err, jwt.ErrTokenExpired) {
					writeAuthErr(w, "AUTH_TOKEN_EXPIRED", "access 已过期")
					return
				}
				writeAuthErr(w, "AUTH_INVALID_CREDENTIALS", "token 无效")
				return
			}

			// jti 黑名单检查（Redis 不可用时降级 = 跳过）
			if rds != nil && claims.ID != "" && rds.IsJTIRevoked(r.Context(), claims.ID) {
				writeAuthErr(w, "AUTH_TOKEN_REVOKED", "token 已被撤销")
				return
			}

			// 会话活跃打点（节流；旧 token 无 sid 时跳过）
			if touch != nil && claims.SID != "" {
				now := time.Now()
				if v, ok := lastTouch.Load(claims.SID); !ok || now.Sub(v.(time.Time)) > touchThrottle {
					lastTouch.Store(claims.SID, now)
					touch(claims.SID)
				}
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, CtxUserID, claims.UID)
			ctx = context.WithValue(ctx, CtxRole, claims.Role)
			ctx = context.WithValue(ctx, CtxTenantID, claims.TenantID)
			ctx = context.WithValue(ctx, CtxJTI, claims.ID)
			ctx = context.WithValue(ctx, CtxSID, claims.SID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole 装饰器：限制访问角色（任一匹配即通过）。
//
//	r.With(middleware.RequireRole("sysadmin")).Post("/rules", ...)
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(CtxRole).(string)
			for _, want := range roles {
				if role == want {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeAuthErr(w, "RBAC_FORBIDDEN", "当前角色无权访问")
		})
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeAuthErr(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + msg + `"}}`))
}
