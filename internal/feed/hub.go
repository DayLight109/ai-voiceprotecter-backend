// Package feed 是 SSE 事件总线。
//
// 与现有 声纹捕手/backend/internal/feed/hub.go 协议完全一致：
//   GET /api/v1/feed/stream → event: feed\ndata: {...}\n\n
// 前端 useFeed 切换 NEXT_PUBLIC_API_URL 即可直接消费。
//
// 真实事件由 analyze handler 在判决完成后 Publish。
// 不再注入任何模拟数据 —— 没有真实流量时订阅者只会收到 hello + 心跳。
//
// 多租户隔离：
//   - Event.TenantID == ""  → 系统级事件，所有订阅者都收到（如 DEFCON 变更）。
//   - Event.TenantID != ""  → 仅同 tenantID 的订阅者收到；sysadmin 订阅者看所有。
//
// TenantID 字段用 json:"-" 隐藏，避免泄露到 SSE payload。
package feed

import (
	"log/slog"
	"sync"
	"time"
)

// Event 单条事件
//
// Level: "info" | "warn" | "danger"
// Side : "system" | "trace" | "voice" | "script"
type Event struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"ts"`
	Side      string    `json:"side"`
	Verb      string    `json:"verb"`
	Payload   string    `json:"payload"`
	Level     string    `json:"level"`
	TenantID  string    `json:"-"` // 路由用，不下发到 SSE
}

type subscriber struct {
	ch       chan Event
	tenantID string
	isAdmin  bool // sysadmin / 监控类账号 → 看所有 tenant
}

// Hub 扇出 + 滑动窗口
type Hub struct {
	mu          sync.RWMutex
	subs        map[*subscriber]struct{}
	recent      []Event
	cap         int
	lastEventAt time.Time
	log         *slog.Logger
}

func NewHub(log *slog.Logger) *Hub {
	return &Hub{subs: map[*subscriber]struct{}{}, cap: 128, log: log}
}

// canSee 判断订阅者是否应收到该事件。
func canSee(s *subscriber, ev Event) bool {
	if s.isAdmin {
		return true
	}
	if ev.TenantID == "" {
		return true // 系统级事件
	}
	return s.tenantID == ev.TenantID
}

func (h *Hub) Publish(ev Event) {
	h.mu.Lock()
	h.recent = append(h.recent, ev)
	if len(h.recent) > h.cap {
		h.recent = h.recent[len(h.recent)-h.cap:]
	}
	h.lastEventAt = ev.Timestamp
	targets := make([]*subscriber, 0, len(h.subs))
	for s := range h.subs {
		if canSee(s, ev) {
			targets = append(targets, s)
		}
	}
	h.mu.Unlock()

	for _, s := range targets {
		select {
		case s.ch <- ev:
		default: // 慢订阅者直接丢
		}
	}
}

// Stats 返回（在线订阅者数, 窗口内缓存事件数, 最近事件时间；零值表示尚无事件）。
func (h *Hub) Stats() (subscribers, buffered int, lastEventAt time.Time) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs), len(h.recent), h.lastEventAt
}

// Subscribe 订阅事件流。
//
//	tenantID — 订阅者所属租户；为 "" 表示无租户（如未登录调试，已被路由层拦截）。
//	isAdmin  — true 时跨租户全订阅，仅用于 sysadmin。
func (h *Hub) Subscribe(tenantID string, isAdmin bool) (<-chan Event, func()) {
	sub := &subscriber{ch: make(chan Event, 32), tenantID: tenantID, isAdmin: isAdmin}
	h.mu.Lock()
	h.subs[sub] = struct{}{}
	h.mu.Unlock()
	return sub.ch, func() {
		h.mu.Lock()
		delete(h.subs, sub)
		h.mu.Unlock()
		close(sub.ch)
	}
}

// Recent 返回最近 n 条订阅者可见的事件（按时间正序）。
func (h *Hub) Recent(n int, tenantID string, isAdmin bool) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n <= 0 {
		return nil
	}
	probe := &subscriber{tenantID: tenantID, isAdmin: isAdmin}
	// 倒序找最近 n 条匹配，再反转回正序
	matched := make([]Event, 0, n)
	for i := len(h.recent) - 1; i >= 0 && len(matched) < n; i-- {
		if canSee(probe, h.recent[i]) {
			matched = append(matched, h.recent[i])
		}
	}
	for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
		matched[i], matched[j] = matched[j], matched[i]
	}
	return matched
}
