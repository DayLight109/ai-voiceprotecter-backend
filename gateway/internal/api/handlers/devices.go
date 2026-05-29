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

func DevicesRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listDevices(d))
	r.Post("/", createDevice(d))
	r.Get("/audit", devicesAudit(d))
	r.Route("/{id}", func(r chi.Router) {
		r.Put("/", updateDevice(d))
		r.Delete("/", deleteDevice(d))
		r.Post("/heartbeat", heartbeatDevice(d))
	})
	return r
}

func listDevices(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceType := r.URL.Query().Get("type") // enterprise | family
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		// sysadmin 可看全部；其余只看本租户
		filterTenant := tenantID
		if role == "sysadmin" {
			filterTenant = ""
		}
		p := parsePage(r)
		rows, total, err := d.Repo.ListDevices(r.Context(), deviceType, filterTenant, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

type deviceInput struct {
	Name     string `json:"name"`
	TenantID string `json:"tenantId"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Version  string `json:"version"`
	Contact  string `json:"contact"`
}

func createDevice(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req deviceInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Name == "" || req.Type == "" || req.Version == "" {
			badRequest(w, "VALIDATION_FAILED", "name / type / version 必填")
			return
		}
		if req.Type != "enterprise" && req.Type != "family" {
			badRequest(w, "VALIDATION_FAILED", "type 仅允许 enterprise / family")
			return
		}
		tenantID := req.TenantID
		if tenantID == "" {
			tenantID, _ = r.Context().Value(middleware.CtxTenantID).(string)
		}
		dev, err := d.Repo.CreateDevice(r.Context(), repo.CreateDeviceParams{
			ID: "dev_" + uuid.NewString(), Name: req.Name, TenantID: tenantID,
			Type: req.Type, Status: req.Status, Version: req.Version, Contact: req.Contact,
		})
		if err != nil {
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: dev})
	}
}

func updateDevice(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req deviceInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		dev, err := d.Repo.UpdateDevice(r.Context(), id, req.Name, req.Status, req.Version, req.Contact)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, dev)
	}
}

func deleteDevice(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := d.Repo.DeleteDevice(r.Context(), id); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// heartbeatDevice 端侧定时心跳
func heartbeatDevice(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := d.Repo.HeartbeatDevice(r.Context(), id); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, map[string]string{"id": id, "status": "online"})
	}
}

// devicesAudit DeviceManager 的「行为日志审计」tab — 拉 audit_logs。
func devicesAudit(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parsePage(r)
		rows, total, err := d.Repo.ListAuditLogs(r.Context(), repo.AuditFilter{}, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}
