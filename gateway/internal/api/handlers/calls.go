package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

func CallsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listCalls(d))
	r.Get("/{id}", getCall(d))
	return r
}

func listCalls(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		p := parsePage(r)

		if role == "sysadmin" {
			rows, total, err := d.Repo.ListAllCallLogs(r.Context(), p)
			if err != nil {
				internalErr(w)
				return
			}
			okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
			return
		}
		rows, total, err := d.Repo.ListCallLogs(r.Context(), tenantID, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

func getCall(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		d2, err := d.Repo.GetCallLogByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		if d2.TenantID != tenantID && role != "sysadmin" {
			writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
				Code: "RBAC_FORBIDDEN", Message: "不可查看其它租户的通话记录",
			}})
			return
		}
		ok(w, d2)
	}
}
