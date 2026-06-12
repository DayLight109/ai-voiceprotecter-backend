package middleware

import "net/http"

// Tenant 简单注入：在已有 Auth 之后，tenant_id 已经在 ctx 中。
// 这里再加一道兜底，避免某些路径漏挂 Auth。
//
// 后续 sqlc 查询通过 ctx.Value(CtxTenantID) 自动隔离。
func Tenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v, _ := r.Context().Value(CtxTenantID).(string); v == "" {
			// 没有租户：要么是 sysadmin（允许），要么是配置错误
			role, _ := r.Context().Value(CtxRole).(string)
			if role != "sysadmin" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":{"code":"TENANT_MISSING","message":"租户信息缺失"}}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
