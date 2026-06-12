package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// EmergencyContactsRouter 提供当前用户的紧急联系人 CRUD：
//
//	GET    /me/emergency-contacts
//	POST   /me/emergency-contacts
//	PUT    /me/emergency-contacts/{id}
//	DELETE /me/emergency-contacts/{id}
//
// 鉴权：仅作用于 ctx 中的 user_id；不依赖 tenant，不暴露他人记录。
func EmergencyContactsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listEmergencyContacts(d))
	r.Post("/", createEmergencyContact(d))
	r.Route("/{id}", func(r chi.Router) {
		r.Put("/", updateEmergencyContact(d))
		r.Delete("/", deleteEmergencyContact(d))
	})
	return r
}

type emergencyContactInput struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Relation string `json:"relation"`
}

func (in *emergencyContactInput) clean() {
	in.Name = strings.TrimSpace(in.Name)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Relation = strings.TrimSpace(in.Relation)
}

func listEmergencyContacts(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		rows, err := d.Repo.ListEmergencyContacts(r.Context(), uid)
		if err != nil {
			d.Logger.Error("emergency_contacts list", "err", err)
			internalErr(w)
			return
		}
		ok(w, rows)
	}
}

func createEmergencyContact(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req emergencyContactInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		req.clean()
		if req.Name == "" {
			badRequest(w, "VALIDATION_FAILED", "name 必填")
			return
		}
		if req.Phone == "" {
			badRequest(w, "VALIDATION_FAILED", "phone 必填")
			return
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		entry, err := d.Repo.CreateEmergencyContact(r.Context(), repo.CreateEmergencyContactParams{
			ID:       "ec_" + uuid.NewString(),
			UserID:   uid,
			Name:     req.Name,
			Phone:    req.Phone,
			Relation: req.Relation,
		})
		if err != nil {
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "EMERGENCY_CONTACT_DUPLICATE", Message: "该号码已在紧急联系人列表",
				}})
				return
			}
			d.Logger.Error("emergency_contacts create", "err", err)
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: entry})
	}
}

func updateEmergencyContact(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		var req emergencyContactInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		req.clean()
		if req.Name == "" {
			badRequest(w, "VALIDATION_FAILED", "name 必填")
			return
		}
		if req.Phone == "" {
			badRequest(w, "VALIDATION_FAILED", "phone 必填")
			return
		}
		entry, err := d.Repo.UpdateEmergencyContact(r.Context(), id, uid, req.Name, req.Phone, req.Relation)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "EMERGENCY_CONTACT_DUPLICATE", Message: "该号码已在紧急联系人列表",
				}})
				return
			}
			d.Logger.Error("emergency_contacts update", "err", err)
			internalErr(w)
			return
		}
		ok(w, entry)
	}
}

func deleteEmergencyContact(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if err := d.Repo.DeleteEmergencyContact(r.Context(), id, uid); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			d.Logger.Error("emergency_contacts delete", "err", err)
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
