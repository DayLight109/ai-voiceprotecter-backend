package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sentinel/gateway/internal/api/middleware"
)

// DashboardRouter 风险大屏聚合数据
//
//	risk-index = 近 1h 阻断率 + AI 合成占比加权 → 1..100
//	regions    = GROUP BY region ORDER BY count DESC LIMIT 7
//	events     = SELECT * FROM call_logs ORDER BY created_at DESC LIMIT ?limit
func DashboardRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/risk-index", riskIndex(d))
	r.Get("/regions", regions(d))
	r.Get("/events", events(d))
	return r
}

func riskIndex(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		filter := tenantID
		if role == "sysadmin" {
			filter = ""
		}
		var total, blocked, aiClones int
		row := d.DB.QueryRow(r.Context(), `
			SELECT COUNT(*),
			       COUNT(*) FILTER (WHERE verdict = '拦截'),
			       COUNT(*) FILTER (WHERE voiceprint_json ? 'verdict' AND voiceprint_json->>'verdict' = 'SYNTH')
			FROM call_logs
			WHERE created_at > now() - interval '1 hour'
			  AND ($1::text = '' OR tenant_id = $1)`, filter)
		if err := row.Scan(&total, &blocked, &aiClones); err != nil {
			internalErr(w)
			return
		}
		// 简化算法：阻断率 0-70 + AI 合成占比 0-30
		idx := 1
		if total > 0 {
			idx = (blocked*70)/total + (aiClones*30)/total
			if idx < 1 {
				idx = 1
			}
			if idx > 100 {
				idx = 100
			}
		}
		ok(w, map[string]any{
			"index":       idx,
			"sampleSize":  total,
			"blocked":     blocked,
			"aiClones":    aiClones,
			"windowHours": 1,
		})
	}
}

func regions(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		filter := tenantID
		if role == "sysadmin" {
			filter = ""
		}
		rows, err := d.DB.Query(r.Context(), `
			SELECT COALESCE(region,'未知') AS region, COUNT(*) AS n
			FROM call_logs
			WHERE created_at > now() - interval '24 hours'
			  AND ($1::text = '' OR tenant_id = $1)
			GROUP BY region ORDER BY n DESC LIMIT 7`, filter)
		if err != nil {
			internalErr(w)
			return
		}
		defer rows.Close()
		type rg struct {
			Region string `json:"region"`
			N      int    `json:"count"`
		}
		out := []rg{}
		for rows.Next() {
			var x rg
			if err := rows.Scan(&x.Region, &x.N); err != nil {
				internalErr(w)
				return
			}
			out = append(out, x)
		}
		ok(w, out)
	}
}

func events(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		filter := tenantID
		if role == "sysadmin" {
			filter = ""
		}
		rows, err := d.DB.Query(r.Context(), `
			SELECT id, phone, COALESCE(region,''), verdict, COALESCE(reason,''),
			       COALESCE(risk_score,0), created_at
			FROM call_logs
			WHERE ($1::text = '' OR tenant_id = $1)
			ORDER BY created_at DESC LIMIT $2`, filter, limit)
		if err != nil {
			internalErr(w)
			return
		}
		defer rows.Close()
		type ev struct {
			ID        string `json:"id"`
			Phone     string `json:"phone"`
			Region    string `json:"region"`
			Verdict   string `json:"verdict"`
			Reason    string `json:"reason"`
			RiskScore int    `json:"riskScore"`
			CreatedAt string `json:"createdAt"`
		}
		out := []ev{}
		for rows.Next() {
			var e ev
			var ts time.Time
			if err := rows.Scan(&e.ID, &e.Phone, &e.Region, &e.Verdict, &e.Reason, &e.RiskScore, &ts); err != nil {
				internalErr(w)
				return
			}
			e.CreatedAt = ts.Format(time.RFC3339)
			out = append(out, e)
		}
		ok(w, out)
	}
}
