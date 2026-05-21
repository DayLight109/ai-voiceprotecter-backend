package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// IdentityRouter 处理 5 种证件认证 (/me/credentials)
func IdentityRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listCredentials(d))
	r.Post("/{kind}", submitCredential(d))
	r.Delete("/{kind}", deleteCredential(d))
	return r
}

// IdentityModesRouter 三种认证模式开关 (位图保存在 permissions 表，前缀 identity.)
func IdentityModesRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getIdentityModes(d))
	r.Patch("/", updateIdentityModes(d))
	return r
}

var allowedCredentialKinds = map[string]struct{}{
	"phone": {}, "id_card": {}, "passport": {}, "military": {}, "hk_mo": {},
}

func listCredentials(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		creds, err := d.Repo.ListCredentialsByUser(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, creds)
	}
}

type credentialInput struct {
	Value    string `json:"value"`    // 明文，仅做 hash 后入库
	Verified bool   `json:"verified"` // 真实环境必须通过外部接口核验后才置 true
}

func submitCredential(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := strings.ToLower(chi.URLParam(r, "kind"))
		if _, ok := allowedCredentialKinds[kind]; !ok {
			badRequest(w, "VALIDATION_FAILED", "kind 仅允许 phone/id_card/passport/military/hk_mo")
			return
		}
		var req credentialInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Value == "" {
			badRequest(w, "VALIDATION_FAILED", "value 必填")
			return
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		c, err := d.Repo.UpsertCredential(r.Context(), repo.UpsertCredentialParams{
			ID: "ic_" + uuid.NewString(), UserID: uid, Kind: kind,
			ValueHash: sha256Hex(req.Value), Verified: req.Verified,
		})
		if err != nil {
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: c})
	}
}

func deleteCredential(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kind := strings.ToLower(chi.URLParam(r, "kind"))
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if err := d.Repo.DeleteCredential(r.Context(), uid, kind); err != nil {
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── identity-modes (permissions 前缀 identity.*) ─────────────

func getIdentityModes(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		all, err := d.Repo.ListPermissions(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, filterByPrefix(all, "identity."))
	}
}

func updateIdentityModes(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req permsBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		for _, p := range req.Items {
			if !strings.HasPrefix(p.Key, "identity.") {
				badRequest(w, "VALIDATION_FAILED", "key 必须以 identity. 开头")
				return
			}
		}
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if err := d.Repo.UpsertPermissions(r.Context(), uid, req.Items); err != nil {
			internalErr(w)
			return
		}
		ok(w, req.Items)
	}
}
