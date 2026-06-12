package handlers

import (
	"errors"
	"html"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sentinel/gateway/internal/repo"
)

func SamplesRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listSamples(d))
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", getSample(d))
		r.Post("/analyze", analyzeSample(d))
		r.Post("/reject", rejectSample(d))
		r.Get("/export-doc", exportSampleDoc(d))
	})
	return r
}

func listSamples(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		p := parsePage(r)
		rows, total, err := d.Repo.ListSamples(r.Context(), status, p)
		if err != nil {
			internalErr(w)
			return
		}
		okMeta(w, toAny(rows), &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

func getSample(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		s, err := d.Repo.GetSampleByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, s)
	}
}

// analyzeSample 拉 transcript → 调 AI /classify → 把分类结果写回 sample.classification + 状态置已审核
func analyzeSample(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		s, err := d.Repo.GetSampleByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		if s.Transcript == "" {
			badRequest(w, "VALIDATION_FAILED", "sample 无 transcript，无法分类")
			return
		}
		hits, err := d.AI.Classify(r.Context(), s.Transcript, qwenOptions(r.Context(), d))
		if err != nil {
			d.Logger.Error("ai classify failed", "err", err, "sampleId", id)
			upstreamErr(w, "AI_UPSTREAM_ERROR", "AI 分类服务调用失败")
			return
		}
		classification := ""
		if len(hits) > 0 {
			classification = hits[0].Category
		}
		if err := d.Repo.UpdateSampleStatus(r.Context(), id, "已审核", classification); err != nil {
			internalErr(w)
			return
		}
		ok(w, map[string]any{
			"id":             id,
			"status":         "已审核",
			"classification": classification,
			"hits":           hits,
		})
	}
}

func rejectSample(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := d.Repo.UpdateSampleStatus(r.Context(), id, "已驳回", ""); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		ok(w, map[string]any{"id": id, "status": "已驳回"})
	}
}

// exportSampleDoc 返回 Microsoft Word 兼容 HTML（用 .doc 后缀，Office 可打开）
// 所有用户字段（含 Transcript）必须 HTML 转义，防止 .doc 在 Word/邮件预览/浏览器中触发 XSS。
func exportSampleDoc(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		s, err := d.Repo.GetSampleByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			internalErr(w)
			return
		}
		safeID := html.EscapeString(id)
		w.Header().Set("Content-Type", "application/msword; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="sample-`+safeID+`.doc"`)
		_, _ = w.Write([]byte(`<html xmlns:o='urn:schemas-microsoft-com:office:office' xmlns:w='urn:schemas-microsoft-com:office:word'>
<head><meta charset="utf-8"><title>样本 ` + safeID + `</title></head><body>`))
		_, _ = w.Write([]byte(`<h2>样本 ` + safeID + `</h2>`))
		_, _ = w.Write([]byte(`<p><b>来源：</b>` + html.EscapeString(s.Origin) + `</p>`))
		_, _ = w.Write([]byte(`<p><b>时长：</b>` + html.EscapeString(s.Duration) + `</p>`))
		_, _ = w.Write([]byte(`<p><b>分类：</b>` + html.EscapeString(s.Classification) + `</p>`))
		_, _ = w.Write([]byte(`<p><b>状态：</b>` + html.EscapeString(s.Status) + `</p>`))
		_, _ = w.Write([]byte(`<h3>转写</h3><pre style="white-space:pre-wrap">` + html.EscapeString(s.Transcript) + `</pre>`))
		_, _ = w.Write([]byte(`</body></html>`))
	}
}
