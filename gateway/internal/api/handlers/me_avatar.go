package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

func UploadAvatar(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		// 限制请求体大小，防止超大上传打爆内存
		r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+1024)
		if err := r.ParseMultipartForm(maxAvatarBytes + 1024); err != nil {
			badRequest(w, "VALIDATION_FAILED", "解析上传失败或文件过大（上限 2MB）")
			return
		}
		f, fh, err := r.FormFile("file")
		if err != nil {
			badRequest(w, "VALIDATION_FAILED", "缺少 file 字段")
			return
		}
		defer f.Close()
		if fh.Size > maxAvatarBytes {
			badRequest(w, "VALIDATION_FAILED", "头像不能超过 2MB")
			return
		}
		ct := fh.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "image/") {
			badRequest(w, "VALIDATION_FAILED", "仅支持图片格式")
			return
		}
		data, err := io.ReadAll(f)
		if err != nil {
			internalErr(w)
			return
		}
		if err := d.Repo.UpsertAvatar(r.Context(), uid, ct, data); err != nil {
			d.Logger.Error("uploadAvatar", "err", err)
			internalErr(w)
			return
		}
		u, err := d.Repo.GetUserByID(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, withAvatar(toPublicUser(u), true))
	}
}

func GetAvatar(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		a, err := d.Repo.GetAvatar(r.Context(), uid)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		w.Header().Set("Content-Type", a.ContentType)
		w.Header().Set("Cache-Control", "private, max-age=0, must-revalidate")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(a.Bytes)
	}
}

func DeleteAvatar(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		if err := d.Repo.DeleteAvatar(r.Context(), uid); err != nil {
			internalErr(w)
			return
		}
		u, err := d.Repo.GetUserByID(r.Context(), uid)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, withAvatar(toPublicUser(u), false))
	}
}
