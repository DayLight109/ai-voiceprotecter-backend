package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Audit 在所有写操作 (POST/PUT/PATCH/DELETE) 完成后落审计日志。
//
//	既 log 一行（运维场景）也异步写一行到 audit_logs（业务场景，未来 /audit API 查它）。
//	DB 为 nil 或写入失败不影响主流程，仅 log.Warn —— 审计是 best-effort。
func Audit(log *slog.Logger, db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)

			if !isMutation(r.Method) {
				return
			}
			result := "成功"
			if rw.status >= 400 {
				result = "失败"
			}
			actor, _ := r.Context().Value(CtxUserID).(string)
			tenant, _ := r.Context().Value(CtxTenantID).(string)
			ip := clientIP(r)
			action := r.Method + " " + r.URL.Path

			log.Info("audit",
				"actor", actor,
				"tenant", tenant,
				"action", action,
				"status", rw.status,
				"result", result,
				"ip", ip,
			)

			// 异步落库；与 HTTP 响应解耦。
			if db != nil {
				go writeAuditLog(db, log, actor, tenant, action, result, ip, rw.status)
			}
		})
	}
}

func writeAuditLog(db *pgxpool.Pool, log *slog.Logger, actor, tenant, action, result, ip string, status int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// audit_logs.result CHECK ('成功','失败')；status / tenant 进 payload jsonb
	payload := fmt.Sprintf(`{"status":%d,"tenant":%q}`, status, tenant)
	_, err := db.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target, result, ip, payload)
		VALUES (NULLIF($1,''), $2, $3, $4, NULLIF($5,'')::inet, $6::jsonb)
	`, actor, action, "", result, ip, payload)
	if err != nil {
		log.Warn("audit insert failed", "err", err, "action", action)
	}
}

func isMutation(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush 透传底层 ResponseWriter 的 Flush。
// 不实现它，SSE handler 的 w.(http.Flusher) 断言会失败 → /feed/stream 恒 500。
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap 暴露底层 writer，使 http.ResponseController 能穿透本包装器。
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
