package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/domain"
	"github.com/sentinel/gateway/internal/repo"
)

// IdentityRouter 处理 5 种证件认证 (/me/credentials)
func IdentityRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listCredentials(d))
	r.Post("/{kind}", submitCredential(d))
	r.Delete("/{kind}", deleteCredential(d))
	r.Post("/{kind}/upload", uploadCredentialPhoto(d))
	r.Delete("/{kind}/photos/{slot}", deleteCredentialPhoto(d))
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

var allowedPhotoSlots = map[string]struct{}{
	"face": {}, "emblem": {}, "main": {},
}

const maxCredPhotoBytes = 5 << 20 // 5 MiB，与前端 identity 页限制一致

func listCredentials(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		creds, err := d.Repo.ListCredentialsByUser(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		// 附带证件照片：前端用 dataUrl 直接渲染 <img>（带 token 的 <img src> 做不到）
		photos, err := d.Repo.ListCredentialPhotosByUser(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		byKind := map[string][]domain.CredentialPhoto{}
		for _, p := range photos {
			byKind[p.Kind] = append(byKind[p.Kind], domain.CredentialPhoto{
				Slot: p.Slot, Name: p.Name, Size: int64(len(p.Bytes)), Mime: p.ContentType,
				DataURL:   "data:" + p.ContentType + ";base64," + base64.StdEncoding.EncodeToString(p.Bytes),
				UpdatedAt: p.UpdatedAt,
			})
		}
		for i := range creds {
			creds[i].Photos = byKind[creds[i].Kind]
		}
		ok(w, creds)
	}
}

type credentialInput struct {
	Value    string `json:"value"`    // 明文，仅做 hash 后入库
	Verified bool   `json:"verified"` // 真实环境必须通过外部接口核验后才置 true
}

// maskCredentialValue 生成脱敏展示值："13800134921" → "138 ···· 4921"。
// 仅保留首 3 尾 4（短值进一步缩短），明文不落库。
func maskCredentialValue(v string) string {
	rs := []rune(strings.Join(strings.Fields(v), "")) // 去掉所有空白
	n := len(rs)
	switch {
	case n >= 8:
		return string(rs[:3]) + " ···· " + string(rs[n-4:])
	case n >= 5:
		return string(rs[:1]) + " ···· " + string(rs[n-2:])
	default:
		return "····"
	}
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
			ValueHash: sha256Hex(req.Value), Masked: maskCredentialValue(req.Value),
			Verified: req.Verified,
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

// uploadCredentialPhoto 证件照片上传（multipart：slot + file）。
// 要求先提交过证件号（存在 credential 行），照片按 (user, kind, slot) 覆盖存储。
func uploadCredentialPhoto(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		kind := strings.ToLower(chi.URLParam(r, "kind"))
		if _, okKind := allowedCredentialKinds[kind]; !okKind {
			badRequest(w, "VALIDATION_FAILED", "kind 仅允许 phone/id_card/passport/military/hk_mo")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxCredPhotoBytes+64<<10)
		if err := r.ParseMultipartForm(maxCredPhotoBytes + 64<<10); err != nil {
			badRequest(w, "VALIDATION_FAILED", "解析上传失败或文件过大（上限 5MB）")
			return
		}
		slot := strings.ToLower(r.FormValue("slot"))
		if _, okSlot := allowedPhotoSlots[slot]; !okSlot {
			badRequest(w, "VALIDATION_FAILED", "slot 仅允许 face/emblem/main")
			return
		}
		f, fh, err := r.FormFile("file")
		if err != nil {
			badRequest(w, "VALIDATION_FAILED", "缺少 file 字段")
			return
		}
		defer f.Close()
		if fh.Size > maxCredPhotoBytes {
			badRequest(w, "VALIDATION_FAILED", "照片不能超过 5MB")
			return
		}
		ct := fh.Header.Get("Content-Type")
		if ct != "image/jpeg" && ct != "image/png" {
			badRequest(w, "VALIDATION_FAILED", "仅支持 JPG / PNG")
			return
		}
		// 必须先提交证件号，避免出现游离照片
		if _, err := d.Repo.GetCredentialByKind(r.Context(), uid, kind); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				badRequest(w, "VALIDATION_FAILED", "请先提交证件号再上传照片")
				return
			}
			internalErr(w)
			return
		}
		data, err := io.ReadAll(f)
		if err != nil {
			internalErr(w)
			return
		}
		if err := d.Repo.UpsertCredentialPhoto(r.Context(), repo.UpsertCredentialPhotoParams{
			UserID: uid, Kind: kind, Slot: slot,
			Name: fh.Filename, ContentType: ct, Bytes: data,
		}); err != nil {
			d.Logger.Error("uploadCredentialPhoto", "err", err)
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: domain.CredentialPhoto{
			Slot: slot, Name: fh.Filename, Size: fh.Size, Mime: ct, UpdatedAt: time.Now(),
		}})
	}
}

func deleteCredentialPhoto(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		kind := strings.ToLower(chi.URLParam(r, "kind"))
		slot := strings.ToLower(chi.URLParam(r, "slot"))
		if err := d.Repo.DeleteCredentialPhoto(r.Context(), uid, kind, slot); err != nil {
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
