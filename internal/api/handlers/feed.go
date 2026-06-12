package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sentinel/gateway/internal/api/middleware"
)

// FeedStream 沿用现有 backend SSE 协议：
//   retry: 3000\n event: hello\n data: {...}\n\n
//   event: feed\n data: {...}\n\n   （每个事件）
//   : ping <ts>\n\n                  （15 s 心跳）
//
// 前端 useFeed hook 零改动即可消费。
//
// 鉴权由路由层 Auth + Tenant 中间件完成；这里从 ctx 取 tenantID + role 给 Hub
// 做事件过滤，sysadmin 看全部，其他角色只看本租户。
func FeedStream(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		tenantID, _ := r.Context().Value(middleware.CtxTenantID).(string)
		role, _ := r.Context().Value(middleware.CtxRole).(string)
		isAdmin := role == "sysadmin"

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache, no-transform")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		// hello
		_, _ = fmt.Fprintf(w, "retry: 3000\nevent: hello\ndata: {\"service\":\"sentinel-gateway\"}\n\n")
		flusher.Flush()

		ch, unsub := d.Hub.Subscribe(tenantID, isAdmin)
		defer unsub()

		// 回放最近 8 条，提供即时上下文（按租户过滤）
		for _, ev := range d.Hub.Recent(8, tenantID, isAdmin) {
			writeSSE(w, ev)
		}
		flusher.Flush()

		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, alive := <-ch:
				if !alive {
					return
				}
				writeSSE(w, ev)
				flusher.Flush()
			case <-heartbeat.C:
				_, _ = fmt.Fprintf(w, ": ping %d\n\n", time.Now().Unix())
				flusher.Flush()
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, payload any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: feed\ndata: %s\n\n", b)
}
