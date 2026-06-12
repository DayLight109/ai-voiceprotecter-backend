// Package handlers 把每个资源拆成独立文件。
//
// 公共：Deps、JSON 响应封装、错误码、NotFound、Health、骨架占位帮助。
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sentinel/gateway/internal/aiclient"
	"github.com/sentinel/gateway/internal/config"
	"github.com/sentinel/gateway/internal/feed"
	"github.com/sentinel/gateway/internal/ops"
	"github.com/sentinel/gateway/internal/repo"
	"github.com/sentinel/gateway/internal/storage"
	"github.com/sentinel/gateway/internal/store"
)

// Deps 是注入给所有 handler 的依赖。
type Deps struct {
	Cfg     *config.Config
	Logger  *slog.Logger
	DB      *pgxpool.Pool
	Redis   *store.Redis
	Repo    *repo.Repo
	Hub     *feed.Hub
	Store   *store.Store
	AI      *aiclient.Client
	Storage *storage.MinIO
	Ops     *ops.Sampler
}

// 响应封装 ─────────────────────────────────────────────────

type Envelope struct {
	Data any   `json:"data,omitempty"`
	Meta *Meta `json:"meta,omitempty"`
}

type Meta struct {
	Page     int `json:"page,omitempty"`
	PageSize int `json:"pageSize,omitempty"`
	Total    int `json:"total"`
}

type ErrEnvelope struct {
	Error ErrBody `json:"error"`
}

type ErrBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, v any) {
	writeJSON(w, http.StatusOK, Envelope{Data: v})
}

func okMeta(w http.ResponseWriter, v any, m *Meta) {
	writeJSON(w, http.StatusOK, Envelope{Data: v, Meta: m})
}

func badRequest(w http.ResponseWriter, code, msg string) {
	writeJSON(w, http.StatusBadRequest, ErrEnvelope{Error: ErrBody{Code: code, Message: msg}})
}

func notFoundErr(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, ErrEnvelope{Error: ErrBody{Code: "RESOURCE_NOT_FOUND", Message: "未找到"}})
}

func internalErr(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, ErrEnvelope{Error: ErrBody{Code: "INTERNAL_ERROR", Message: "内部错误"}})
}

// emptyList 用于骨架阶段或确实没有数据：返回空数组 + Total=0
func emptyList(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, Envelope{
		Data: []any{},
		Meta: &Meta{Page: 1, PageSize: 20, Total: 0},
	})
}

// notImplemented 返回 501 NOT_IMPLEMENTED，明示等待业务层接入。
func notImplemented(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotImplemented, ErrEnvelope{Error: ErrBody{
		Code: "NOT_IMPLEMENTED", Message: "等待业务层实现（见 internal/service）",
	}})
}

// upstreamErr 用于调用上游失败（AI / MinIO / 第三方）。
func upstreamErr(w http.ResponseWriter, code, msg string) {
	writeJSON(w, http.StatusBadGateway, ErrEnvelope{Error: ErrBody{Code: code, Message: msg}})
}

// Health 深度健康检查：探测 DB / Redis / MinIO 可达性，返回各组件状态。
// 任一组件挂掉返回 503；DB 必须健康才算 OK。
func Health(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		components := map[string]string{}
		overall := http.StatusOK

		// DB（必需）
		if d.DB != nil {
			if err := d.DB.Ping(ctx); err != nil {
				components["db"] = "down: " + err.Error()
				overall = http.StatusServiceUnavailable
			} else {
				components["db"] = "ok"
			}
		} else {
			components["db"] = "not-configured"
			overall = http.StatusServiceUnavailable
		}

		// Redis（降级允许）
		if d.Redis != nil && d.Redis.Available() {
			components["redis"] = "ok"
		} else {
			components["redis"] = "degraded"
		}

		// MinIO（降级允许）
		if d.Storage != nil && d.Storage.Available() {
			components["minio"] = "ok"
		} else {
			components["minio"] = "degraded"
		}

		// AI（不在此处探测，避免对 AI 子服务造成压力）
		components["ai"] = "n/a"

		writeJSON(w, overall, Envelope{Data: map[string]any{
			"status":     statusFromCode(overall),
			"service":    "sentinel-gateway",
			"version":    "v0.1.0",
			"components": components,
		}})
	}
}

func statusFromCode(c int) string {
	if c == http.StatusOK {
		return "ok"
	}
	return "degraded"
}

// NotFound 404 兜底
func NotFound(w http.ResponseWriter, _ *http.Request) {
	notFoundErr(w)
}
