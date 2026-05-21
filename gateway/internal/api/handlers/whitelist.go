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

func WhitelistRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listWhitelist(d))
	r.Post("/", createWhitelist(d))
	r.Route("/{id}", func(r chi.Router) {
		r.Put("/", updateWhitelist(d))
		r.Delete("/", deleteWhitelist(d))
	})
	return r
}

func listWhitelist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		p := parsePage(r)
		rows, total, err := d.Repo.ListWhitelist(r.Context(), tenantID, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

type whitelistInput struct {
	Number   string `json:"number"`
	Name     string `json:"name"`
	Relation string `json:"relation"`
}

func createWhitelist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req whitelistInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Number == "" {
			badRequest(w, "VALIDATION_FAILED", "number 必填")
			return
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		entry, err := d.Repo.CreateWhitelist(r.Context(), repo.CreateWhitelistParams{
			ID: "wl_" + uuid.NewString(), TenantID: tenantID,
			Number: req.Number, Name: req.Name, Relation: req.Relation,
		})
		if err != nil {
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "WHITELIST_DUPLICATE", Message: "该号码已在白名单",
				}})
				return
			}
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: entry})
	}
}

func updateWhitelist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		var req whitelistInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		entry, err := d.Repo.UpdateWhitelist(r.Context(), id, tenantID, req.Number, req.Name, req.Relation)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, entry)
	}
}

func deleteWhitelist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		if err := d.Repo.DeleteWhitelist(r.Context(), id, tenantID); err != nil {
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
