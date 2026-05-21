package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sentinel/gateway/internal/api/middleware"
)

// 多租户隔离：Hub.Recent 按 tenantID 过滤，sysadmin 看全部。
// Snapshot / Defcon 是平台级状态，已登录任意角色均可读。

func tenantFromCtx(r *http.Request) (string, bool) {
	tid, _ := r.Context().Value(middleware.CtxTenantID).(string)
	role, _ := r.Context().Value(middleware.CtxRole).(string)
	return tid, role == "sysadmin"
}

// Stats /api/v1/stats — 全局计数器，warroom 兼容
func Stats(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, d.Store.Snapshot())
	}
}

// Threats /api/v1/threats — 最近 16 条 danger 级事件（按租户过滤）
func Threats(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tid, isAdmin := tenantFromCtx(r)
		all := d.Hub.Recent(256, tid, isAdmin)
		out := make([]any, 0, 16)
		for i := len(all) - 1; i >= 0 && len(out) < 16; i-- {
			if all[i].Level == "danger" {
				out = append(out, all[i])
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// FeedRecent 最近 N 条事件（按租户过滤）
func FeedRecent(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := 32
		if q := r.URL.Query().Get("n"); q != "" {
			if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 256 {
				n = v
			}
		}
		tid, isAdmin := tenantFromCtx(r)
		writeJSON(w, http.StatusOK, d.Hub.Recent(n, tid, isAdmin))
	}
}

// DefconGet / DefconSet
func DefconGet(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"level": d.Store.Defcon()})
	}
}

func DefconSet(d Deps) http.HandlerFunc {
	type req struct{ Level int `json:"level"` }
	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			badRequest(w, "VALIDATION_FAILED", "invalid json")
			return
		}
		if body.Level < 1 || body.Level > 5 {
			badRequest(w, "VALIDATION_FAILED", "level must be 1..5")
			return
		}
		d.Store.SetDefcon(body.Level)
		writeJSON(w, http.StatusOK, map[string]any{"level": d.Store.Defcon()})
	}
}
