package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// AuditRouter 审计日志查询。仅 sysadmin 可访问（包含所有租户的写操作记录，敏感）。
func AuditRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.With(middleware.RequireRole("sysadmin")).Get("/", listAudit(d))
	return r
}

func listAudit(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := r.URL.Query().Get("actor") // 可选过滤
		p := parsePage(r)
		rows, total, err := d.Repo.ListAuditLogs(r.Context(), repo.AuditFilter{ActorID: actor}, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}
