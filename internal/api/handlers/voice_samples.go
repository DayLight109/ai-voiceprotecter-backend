package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// VoiceSamplesRouter 训练样本库。写操作仅 sysadmin。
func VoiceSamplesRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listVoiceSamples(d))
	r.With(middleware.RequireRole("sysadmin")).Post("/", uploadVoiceSample(d))
	r.With(middleware.RequireRole("sysadmin")).Delete("/{id}", deleteVoiceSample(d))
	return r
}

func listVoiceSamples(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parsePage(r)
		rows, total, err := d.Repo.ListVoiceSamples(r.Context(), p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

func uploadVoiceSample(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Storage == nil || !d.Storage.Available() {
			upstreamErr(w, "STORAGE_UNAVAILABLE", "对象存储未就绪")
			return
		}
		if err := r.ParseMultipartForm(50 << 20); err != nil { // 50 MB 上限
			badRequest(w, "VALIDATION_FAILED", "multipart 解析失败："+err.Error())
			return
		}
		tag := r.FormValue("tag")
		if tag != "synth" && tag != "human" {
			badRequest(w, "VALIDATION_FAILED", "tag 仅允许 synth / human")
			return
		}
		f, fh, err := r.FormFile("file")
		if err != nil {
			badRequest(w, "VALIDATION_FAILED", "file 必填")
			return
		}
		defer f.Close()

		id := "vs_" + uuid.NewString()
		// 路径安全：key 仅用服务端 id，不拼客户端 fh.Filename（避免 ../../ 注入）。
		key := "voice-samples/" + id
		if _, err := d.Storage.Put(r.Context(), d.Storage.ModelsBucket(), key, f, fh.Size, fh.Header.Get("Content-Type")); err != nil {
			d.Logger.Error("storage put failed", "bucket", "models", "key", key, "err", err)
			upstreamErr(w, "STORAGE_PUT_FAILED", "样本文件存储失败")
			return
		}
		s, err := d.Repo.CreateVoiceSample(r.Context(), repo.CreateVoiceSampleParams{
			ID: id, Name: fh.Filename, SizeBytes: fh.Size, Tag: tag, ObjectKey: key,
		})
		if err != nil {
			_ = d.Storage.Delete(r.Context(), d.Storage.ModelsBucket(), key)
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: s})
	}
}

func deleteVoiceSample(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		v, err := d.Repo.GetVoiceSampleByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if d.Storage != nil && d.Storage.Available() && v.ObjectKey != "" {
			if err := d.Storage.Delete(r.Context(), d.Storage.ModelsBucket(), v.ObjectKey); err != nil {
				d.Logger.Warn("storage delete failed", "err", err, "key", v.ObjectKey)
			}
		}
		if err := d.Repo.DeleteVoiceSample(r.Context(), id); err != nil {
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
