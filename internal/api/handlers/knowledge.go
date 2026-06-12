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

func KnowledgeRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listKnowledge(d))
	r.With(middleware.RequireRole("sysadmin", "admin")).Post("/", createKnowledge(d))
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", getKnowledge(d))
		r.With(middleware.RequireRole("sysadmin", "admin")).Put("/", updateKnowledge(d))
		r.With(middleware.RequireRole("sysadmin")).Delete("/", deleteKnowledge(d))
	})
	return r
}

func listKnowledge(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.URL.Query().Get("category")
		p := parsePage(r)
		rows, total, err := d.Repo.ListKnowledge(r.Context(), category, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

func getKnowledge(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		a, err := d.Repo.GetKnowledgeByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		_ = d.Repo.IncrementKnowledgeView(r.Context(), id)
		ok(w, a)
	}
}

type knowledgeInput struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	Body     string `json:"body"`
	Status   string `json:"status"`
}

func createKnowledge(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req knowledgeInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Title == "" || req.Category == "" || req.Body == "" {
			badRequest(w, "VALIDATION_FAILED", "title / category / body 必填")
			return
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		k, err := d.Repo.CreateKnowledge(r.Context(), repo.CreateKnowledgeParams{
			ID: "kb_" + uuid.NewString(),
			Title: req.Title, Category: req.Category, Summary: req.Summary, Body: req.Body,
			Status: req.Status, UpdatedBy: uid,
		})
		if err != nil {
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: k})
	}
}

func updateKnowledge(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req knowledgeInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		k, err := d.Repo.UpdateKnowledge(r.Context(), repo.CreateKnowledgeParams{
			ID: id, Title: req.Title, Category: req.Category, Summary: req.Summary,
			Body: req.Body, Status: req.Status, UpdatedBy: uid,
		})
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, k)
	}
}

func deleteKnowledge(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := d.Repo.DeleteKnowledge(r.Context(), id); err != nil {
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
