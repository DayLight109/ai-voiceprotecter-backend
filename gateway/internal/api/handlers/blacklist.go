package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/repo"
)

// BlacklistRouter CRUD + import/export
func BlacklistRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Get("/", listBlacklist(d))
	r.Post("/", createBlacklist(d))
	r.Get("/export", exportBlacklist(d))
	r.Post("/import", importBlacklist(d))
	r.Route("/{id}", func(r chi.Router) {
		r.Put("/", updateBlacklist(d))
		r.Delete("/", deleteBlacklist(d))
	})
	return r
}

func parsePage(r *http.Request) repo.Page {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	return repo.Page{Page: page, PageSize: size}
}

func listBlacklist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		p := parsePage(r)
		q := strings.TrimSpace(r.URL.Query().Get("q"))

		var (
			items []any
			total int64
			err   error
		)
		if q != "" {
			rows, t, e := d.Repo.SearchBlacklist(r.Context(), tenantID, q, p)
			items = toAny(rows)
			total = t
			err = e
		} else {
			rows, t, e := d.Repo.ListBlacklist(r.Context(), tenantID, p)
			items = toAny(rows)
			total = t
			err = e
		}
		if err != nil {
			d.Logger.Error("blacklist list", "err", err)
			internalErr(w)
			return
		}
		okMeta(w, items, &Meta{Page: p.Page, PageSize: p.PageSize, Total: int(total)})
	}
}

type blacklistInput struct {
	Number   string `json:"number"`
	Reason   string `json:"reason"`
	Category string `json:"category"`
	Risk     int    `json:"risk"`
	Source   string `json:"source"`
	Global   bool   `json:"global"` // sysadmin 可建全局
}

func createBlacklist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req blacklistInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		if req.Number == "" || req.Category == "" || req.Source == "" {
			badRequest(w, "VALIDATION_FAILED", "number / category / source 必填")
			return
		}
		if req.Risk < 0 || req.Risk > 100 {
			badRequest(w, "VALIDATION_FAILED", "risk 须 0-100")
			return
		}
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if req.Global && role != "sysadmin" {
			writeJSON(w, http.StatusForbidden, ErrEnvelope{Error: ErrBody{
				Code: "RBAC_FORBIDDEN", Message: "仅 sysadmin 可创建全局黑名单",
			}})
			return
		}
		if req.Global {
			tenantID = ""
		}
		entry, err := d.Repo.CreateBlacklist(r.Context(), repo.CreateBlacklistParams{
			ID: "bl_" + uuid.NewString(), TenantID: tenantID,
			Number: req.Number, Reason: req.Reason, Category: req.Category,
			Risk: req.Risk, Source: req.Source, CreatedBy: uid,
		})
		if err != nil {
			d.Logger.Error("blacklist create", "err", err)
			internalErr(w)
			return
		}
		writeJSON(w, http.StatusCreated, Envelope{Data: entry})
	}
}

func updateBlacklist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req blacklistInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			badRequest(w, "VALIDATION_FAILED", "请求体无法解析")
			return
		}
		entry, err := d.Repo.UpdateBlacklist(r.Context(), id, req.Number, req.Reason, req.Category, req.Risk)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			d.Logger.Error("blacklist update", "err", err)
			internalErr(w)
			return
		}
		ok(w, entry)
	}
}

func deleteBlacklist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		if err := d.Repo.DeleteBlacklist(r.Context(), id, tenantID, role); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				notFoundErr(w)
				return
			}
			d.Logger.Error("blacklist delete", "err", err)
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── CSV import / export ─────────────────────────────────────

func exportBlacklist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		// 拉一页 1000 条简化实现；前端如需更大量改成流式
		rows, _, err := d.Repo.ListBlacklist(r.Context(), tenantID, repo.Page{Page: 1, PageSize: 100})
		if err != nil {
			internalErr(w)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="blacklist.csv"`)
		_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM for Excel
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"号码", "类别", "风险", "原因", "来源"})
		for _, e := range rows {
			_ = cw.Write([]string{
				csvSafe(e.Number), csvSafe(e.Category), strconv.Itoa(e.Risk),
				csvSafe(e.Reason), csvSafe(e.Source),
			})
		}
		cw.Flush()
	}
}

// csvSafe 防 Excel/WPS 公式注入：以 = + - @ \t \r 起头的单元格会被识别为公式
// （可触发 HYPERLINK 钓鱼、DDE 命令执行），前补单引号让其按文本处理。
func csvSafe(s string) string {
	if len(s) == 0 {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func importBlacklist(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 接受 CSV 或 JSON 数组
		ct := r.Header.Get("Content-Type")
		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		var imported, skipped int

		insert := func(req blacklistInput) {
			if req.Number == "" || req.Category == "" || req.Source == "" || req.Risk < 0 || req.Risk > 100 {
				skipped++
				return
			}
			_, err := d.Repo.CreateBlacklist(r.Context(), repo.CreateBlacklistParams{
				ID: "bl_" + uuid.NewString(), TenantID: tenantID,
				Number: req.Number, Reason: req.Reason, Category: req.Category,
				Risk: req.Risk, Source: req.Source, CreatedBy: uid,
			})
			if err != nil {
				skipped++
				return
			}
			imported++
		}

		switch {
		case strings.HasPrefix(ct, "text/csv"), strings.HasPrefix(ct, "application/csv"):
			cr := csv.NewReader(r.Body)
			cr.FieldsPerRecord = -1
			rows, err := cr.ReadAll()
			if err != nil {
				badRequest(w, "VALIDATION_FAILED", "CSV 解析失败："+err.Error())
				return
			}
			for i, row := range rows {
				if i == 0 || len(row) < 3 {
					continue
				}
				risk, _ := strconv.Atoi(row[2])
				req := blacklistInput{
					Number: strings.TrimSpace(row[0]), Category: row[1], Risk: risk,
					Source: "本地",
				}
				if len(row) > 3 {
					req.Reason = row[3]
				}
				if len(row) > 4 {
					req.Source = row[4]
				}
				insert(req)
			}
		default:
			var batch []blacklistInput
			if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
				badRequest(w, "VALIDATION_FAILED", "JSON 解析失败")
				return
			}
			for _, req := range batch {
				insert(req)
			}
		}
		ok(w, map[string]int{"imported": imported, "skipped": skipped})
	}
}

// toAny 把 []T 折回 []any
func toAny[T any](xs []T) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}
