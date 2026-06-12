package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

func RecordingsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listRecordings(d))
	r.Post("/", uploadRecording(d))
	r.Get("/policy", getRecordingPolicy(d))
	r.With(middleware.RequireRole("family_admin", "admin", "sysadmin")).Put("/policy", putRecordingPolicy(d))
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/download", downloadRecording(d))
		r.Delete("/", deleteRecording(d))
	})
	return r
}

func listRecordings(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		p := parsePage(r)
		rows, total, err := d.Repo.ListRecordings(r.Context(), tenantID, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

// uploadRecording multipart：file + phone + duration + verdict（可选）。
// 上传前先检查租户 recording_policy.upload_enabled。
func uploadRecording(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Storage == nil || !d.Storage.Available() {
			upstreamErr(w, "STORAGE_UNAVAILABLE", "对象存储未就绪")
			return
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)

		// 策略检查
		pol, err := d.Repo.GetRecordingPolicy(r.Context(), tenantID)
		if err == nil && !pol.UploadEnabled {
			writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
				Code: "RECORDING_DISABLED", Message: "当前租户已关闭录音上传",
			}})
			return
		}

		if err := r.ParseMultipartForm(100 << 20); err != nil { // 100 MB
			badRequest(w, "VALIDATION_FAILED", "multipart 解析失败："+err.Error())
			return
		}
		phone := r.FormValue("phone")
		duration := r.FormValue("duration")
		verdict := r.FormValue("verdict")
		if verdict == "" {
			verdict = "通过"
		}
		f, fh, err := r.FormFile("file")
		if err != nil {
			badRequest(w, "VALIDATION_FAILED", "file 必填")
			return
		}
		defer f.Close()

		id := "rec_" + uuid.NewString()
		// 路径安全：key 仅用服务端生成的 id，不拼接客户端 fh.Filename
		// （后者可注入 ../../，污染或覆盖跨租户对象）。
		key := tenantID + "/" + id
		ct := fh.Header.Get("Content-Type")
		if ct == "" {
			ct = "audio/wav"
		}
		if _, err := d.Storage.Put(r.Context(), d.Storage.RecordingsBucket(), key, f, fh.Size, ct); err != nil {
			d.Logger.Error("storage put failed", "bucket", "recordings", "key", key, "err", err)
			upstreamErr(w, "STORAGE_PUT_FAILED", "录音文件存储失败")
			return
		}
		rec, err := d.Repo.CreateRecording(r.Context(), repo.CreateRecordingParams{
			ID: id, TenantID: tenantID, OwnerUserID: uid,
			Phone: phone, Duration: duration, SizeBytes: fh.Size,
			Verdict: verdict, ObjectKey: key,
		})
		if err != nil {
			_ = d.Storage.Delete(r.Context(), d.Storage.RecordingsBucket(), key)
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: rec})
	}
}

func downloadRecording(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		rec, err := d.Repo.GetRecordingByID(r.Context(), id)
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
		if rec.TenantID != tenantID && role != "sysadmin" {
			writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
				Code: "RBAC_FORBIDDEN", Message: "不可下载其它租户的录音",
			}})
			return
		}
		if d.Storage == nil || !d.Storage.Available() {
			upstreamErr(w, "STORAGE_UNAVAILABLE", "对象存储未就绪")
			return
		}
		url, err := d.Storage.PresignGet(r.Context(), d.Storage.RecordingsBucket(), rec.ObjectKey, 5*time.Minute)
		if err != nil {
			d.Logger.Error("storage presign failed", "bucket", "recordings", "key", rec.ObjectKey, "err", err)
			upstreamErr(w, "STORAGE_PRESIGN_FAILED", "录音下载链接生成失败")
			return
		}
		ok(w, map[string]any{
			"url":       url,
			"expiresIn": 300,
		})
	}
}

func deleteRecording(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		rec, err := d.Repo.GetRecordingByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if rec.TenantID != tenantID {
			role, _ := r.Context().Value(middleware.CtxRole).(string)
			if role != "sysadmin" {
				writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
					Code: "RBAC_FORBIDDEN", Message: "不可删除其它租户的录音",
				}})
				return
			}
		}
		if d.Storage != nil && d.Storage.Available() && rec.ObjectKey != "" {
			if err := d.Storage.Delete(r.Context(), d.Storage.RecordingsBucket(), rec.ObjectKey); err != nil {
				d.Logger.Warn("storage delete failed", "err", err, "key", rec.ObjectKey)
			}
		}
		if err := d.Repo.DeleteRecording(r.Context(), id, rec.TenantID); err != nil {
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getRecordingPolicy(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		p, err := d.Repo.GetRecordingPolicy(r.Context(), tenantID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				ok(w, map[string]any{"tenantId": tenantID, "uploadEnabled": true})
				return
			}
			internalErr(w)
			return
		}
		ok(w, p)
	}
}

type recordingPolicyInput struct {
	UploadEnabled bool `json:"uploadEnabled"`
}

func putRecordingPolicy(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req recordingPolicyInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		p, err := d.Repo.UpsertRecordingPolicy(r.Context(), tenantID, req.UploadEnabled)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, p)
	}
}
