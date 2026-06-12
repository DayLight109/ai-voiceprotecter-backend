package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/sentinel/gateway/internal/api/middleware"
)

// sessionView 对应前端 lib/api.ts 的 SessionView。
// token 即会话 jti（前端撤销时回传）。
type sessionView struct {
	Token       string `json:"token"`
	DeviceLabel string `json:"deviceLabel"`
	IP          string `json:"ip"`
	UserAgent   string `json:"userAgent"`
	CreatedAt   string `json:"createdAt"`
	LastSeenAt  string `json:"lastSeenAt"`
	ExpiresAt   string `json:"expiresAt"`
	Current     bool   `json:"current"`
}

func ListSessions(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		// 当前会话：access claims 里带了所属 refresh 会话 jti（sid），可精确匹配。
		// sid 为空（改版前签发的旧 token）时退回 UA + 最近活跃启发式。
		sid, _ := r.Context().Value(middleware.CtxSID).(string)
		curUA := r.Header.Get("User-Agent")
		rows, err := d.Repo.ListSessionsByUser(r.Context(), uid)
		if err != nil {
			d.Logger.Error("listSessions", "err", err)
			internalErr(w)
			return
		}
		out := make([]sessionView, 0, len(rows))
		currentMarked := false
		for _, s := range rows {
			isCur := false
			if sid != "" {
				isCur = s.JTI == sid
			} else if !currentMarked && s.UserAgent == curUA && curUA != "" {
				isCur = true
			}
			if isCur {
				currentMarked = true
			}
			out = append(out, sessionView{
				Token:       s.JTI,
				DeviceLabel: deviceLabelFromUA(s.UserAgent),
				IP:          s.IP,
				UserAgent:   s.UserAgent,
				CreatedAt:   s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				LastSeenAt:  s.LastSeenAt.Format("2006-01-02T15:04:05Z07:00"),
				ExpiresAt:   s.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
				Current:     isCur,
			})
		}
		// 全未命中时（sid 对应会话刚被撤销等），把最新一条标 current，避免前端「无当前设备」
		if !currentMarked && len(out) > 0 {
			out[0].Current = true
		}
		ok(w, out)
	}
}

func RevokeSession(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		if token == "" {
			badRequest(w, "VALIDATION_FAILED", "token 必填")
			return
		}
		if err := d.Repo.RevokeSession(r.Context(), token); err != nil {
			internalErr(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func RevokeOtherSessions(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(middleware.CtxUserID).(string)
		if uid == "" {
			badRequest(w, "AUTH_TOKEN_MISSING", "未登录")
			return
		}
		// 保留当前会话：优先 access claims 里的 sid（精确）；
		// 旧 token 无 sid 时退回 UA 启发式，最后兜底保留最新一条。
		keep, _ := r.Context().Value(middleware.CtxSID).(string)
		if keep == "" {
			curUA := r.Header.Get("User-Agent")
			rows, _ := d.Repo.ListSessionsByUser(r.Context(), uid)
			for _, s := range rows {
				if s.UserAgent == curUA && curUA != "" {
					keep = s.JTI
					break
				}
			}
			if keep == "" && len(rows) > 0 {
				keep = rows[0].JTI
			}
		}
		n, err := d.Repo.RevokeSessionsExcept(r.Context(), uid, keep)
		if err != nil {
			internalErr(w)
			return
		}
		ok(w, map[string]int{"revoked": int(n)})
	}
}

// deviceLabelFromUA 从 User-Agent 粗提一个人类可读标签。
func deviceLabelFromUA(ua string) string {
	if ua == "" {
		return "未知设备"
	}
	type pair struct{ kw, label string }
	os := []pair{
		{"Windows", "Windows"}, {"Mac OS", "macOS"}, {"Macintosh", "macOS"},
		{"Android", "Android"}, {"iPhone", "iPhone"}, {"iPad", "iPad"}, {"Linux", "Linux"},
	}
	br := []pair{
		{"Edg", "Edge"}, {"Chrome", "Chrome"}, {"Firefox", "Firefox"}, {"Safari", "Safari"},
	}
	osName, brName := "", ""
	for _, p := range os {
		if strings.Contains(ua, p.kw) {
			osName = p.label
			break
		}
	}
	for _, p := range br {
		if strings.Contains(ua, p.kw) {
			brName = p.label
			break
		}
	}
	switch {
	case osName != "" && brName != "":
		return brName + " · " + osName
	case osName != "":
		return osName
	case brName != "":
		return brName
	default:
		return "未知设备"
	}
}
