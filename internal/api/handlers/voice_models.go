package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// VoiceModelsRouter ONNX 模型版本管理。写操作仅 sysadmin。
func VoiceModelsRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listVoiceModels(d))
	r.With(middleware.RequireRole("sysadmin")).Post("/", uploadVoiceModel(d))
	r.Route("/{id}", func(r chi.Router) {
		r.With(middleware.RequireRole("sysadmin")).Post("/activate", activateVoiceModel(d))
		r.With(middleware.RequireRole("sysadmin")).Delete("/", deleteVoiceModel(d))
	})
	return r
}

func listVoiceModels(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parsePage(r)
		rows, total, err := d.Repo.ListVoiceModels(r.Context(), p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

// uploadVoiceModel multipart：
//
//	form 字段:
//	  file       (required, binary)
//	  version    (required, e.g. "v2.6.1")
//	  accuracy   (optional, default 0)
func uploadVoiceModel(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Storage == nil || !d.Storage.Available() {
			upstreamErr(w, "STORAGE_UNAVAILABLE", "对象存储未就绪")
			return
		}
		// 限 200 MB
		if err := r.ParseMultipartForm(200 << 20); err != nil {
			badRequest(w, "VALIDATION_FAILED", "multipart 解析失败："+err.Error())
			return
		}
		version := r.FormValue("version")
		if version == "" {
			badRequest(w, "VALIDATION_FAILED", "version 必填")
			return
		}
		accuracy, _ := strconv.ParseFloat(r.FormValue("accuracy"), 64)
		f, fh, err := r.FormFile("file")
		if err != nil {
			badRequest(w, "VALIDATION_FAILED", "file 必填")
			return
		}
		defer f.Close()

		id := "vm_" + uuid.NewString()
		// 路径安全：key 仅用服务端 id，不拼客户端 fh.Filename（避免 ../../ 注入）。
		key := "voice-models/" + id
		if _, err := d.Storage.Put(r.Context(), d.Storage.ModelsBucket(), key, f, fh.Size, "application/octet-stream"); err != nil {
			d.Logger.Error("storage put failed", "bucket", "models", "key", key, "err", err)
			upstreamErr(w, "STORAGE_PUT_FAILED", "模型文件存储失败")
			return
		}
		m, err := d.Repo.CreateVoiceModel(r.Context(), repo.CreateVoiceModelParams{
			ID: id, Version: version, Accuracy: accuracy, SizeBytes: fh.Size, ObjectKey: key,
		})
		if err != nil {
			// 上传成功但入库失败，尝试回滚对象
			_ = d.Storage.Delete(r.Context(), d.Storage.ModelsBucket(), key)
			if errors.Is(err, repo.ErrConflict) {
				writeJSON(w, http.StatusConflict, ErrEnvelope{Error: ErrBody{
					Code: "MODEL_VERSION_DUPLICATE", Message: "version 已存在",
				}})
				return
			}
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: m})
	}
}

func activateVoiceModel(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := d.Repo.ActivateVoiceModel(r.Context(), id); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, map[string]any{"id": id, "active": true})
	}
}

func deleteVoiceModel(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		// 先取一次 record 以拿 object_key
		m, err := d.Repo.GetVoiceModelByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if err := d.Repo.DeleteVoiceModel(r.Context(), id); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if d.Storage != nil && d.Storage.Available() && m.ObjectKey != "" {
			_ = d.Storage.Delete(r.Context(), d.Storage.ModelsBucket(), m.ObjectKey)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
