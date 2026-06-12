package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/sentinel/gateway/internal/store"
)

// RateLimit 装饰器：优先 Redis 固定窗口，失败 / 未注入则回退到本地内存桶。
//
// 内存版按 client IP 维度；Redis 版同样按 IP，scope="ip"。
// 已登录请求可在 service 层再追加按 user/tenant 的细粒度限流。
func RateLimit(perMinute int, rds *store.Redis) func(http.Handler) http.Handler {
	mem := newMemBuckets(perMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			allow := true
			if rds != nil && rds.Available() {
				allow = rds.RateAllow(r.Context(), "ip", ip, perMinute)
			} else {
				allow = mem.allow(ip)
			}

			if !allow {
				w.Header().Set("Retry-After", "30")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"请稍后重试"}}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// 取第一个
		for i := 0; i < len(ip); i++ {
			if ip[i] == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	return r.RemoteAddr
}

// ── 本地降级实现 ─────────────────────────────────────────────

type memBucket struct {
	tokens   int
	lastFill time.Time
}

type memBuckets struct {
	mu        sync.Mutex
	perMinute int
	m         map[string]*memBucket
}

func newMemBuckets(perMinute int) *memBuckets {
	return &memBuckets{perMinute: perMinute, m: map[string]*memBucket{}}
}

func (b *memBuckets) allow(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	bk, ok := b.m[key]
	if !ok {
		bk = &memBucket{tokens: b.perMinute, lastFill: time.Now()}
		b.m[key] = bk
	}
	elapsed := time.Since(bk.lastFill)
	add := int(elapsed.Seconds() * float64(b.perMinute) / 60.0)
	if add > 0 {
		bk.tokens += add
		if bk.tokens > b.perMinute {
			bk.tokens = b.perMinute
		}
		bk.lastFill = time.Now()
	}
	if bk.tokens <= 0 {
		return false
	}
	bk.tokens--
	return true
}
