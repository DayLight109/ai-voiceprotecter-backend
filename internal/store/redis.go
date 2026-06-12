// Package store 也承载 Redis 客户端。
//
// 设计原则：Redis 不可用时所有方法返回**安全默认值**，让 gateway 继续可用：
//   - RateAllow  → true（不拦）
//   - IsRevoked  → false（不拦）
//   - 缓存方法   → cache miss
//
// 这样开发态可以不起 Redis；生产严禁连不上就静默放行 —— 上线时配置
// 监控告警观察 redis.connected metric。
package store

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/sentinel/gateway/internal/config"
)

// Redis 实例。client == nil 表示降级模式。
type Redis struct {
	client *redis.Client
	log    *slog.Logger
}

// NewRedis 创建客户端并 ping 一次。连不上不返回 error，只记 warn —— 走降级。
func NewRedis(ctx context.Context, cfg config.RedisConfig, log *slog.Logger) *Redis {
	r := &Redis{log: log}
	if cfg.Addr == "" {
		log.Warn("redis not configured, running in degraded mode")
		return r
	}
	cli := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     20,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := cli.Ping(pingCtx).Err(); err != nil {
		log.Warn("redis ping failed, degraded mode", "err", err, "addr", cfg.Addr)
		_ = cli.Close()
		return r
	}
	log.Info("redis connected", "addr", cfg.Addr, "db", cfg.DB)
	r.client = cli
	return r
}

func (r *Redis) Close() {
	if r == nil || r.client == nil {
		return
	}
	_ = r.client.Close()
}

func (r *Redis) Available() bool {
	return r != nil && r.client != nil
}

// ── 限流：固定窗口 ─────────────────────────────────────────────
//
// 简洁稳定。key 设计：rl:{scope}:{id}:{epochMinute}
// 不可用时直接放行（true）。
func (r *Redis) RateAllow(ctx context.Context, scope, id string, perMinute int) bool {
	if !r.Available() {
		return true
	}
	minute := strconv.FormatInt(time.Now().Unix()/60, 10)
	key := "rl:" + scope + ":" + id + ":" + minute
	n, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		r.log.Warn("redis incr failed", "err", err)
		return true
	}
	if n == 1 {
		_ = r.client.Expire(ctx, key, 70*time.Second).Err()
	}
	return n <= int64(perMinute)
}

// ── JTI 黑名单（access 主动撤销） ───────────────────────────────
//
// key: jti:{id}, value: "1", TTL: 到 token 自然过期为止。
// 不可用 → 视为未撤销（false）。
func (r *Redis) RevokeJTI(ctx context.Context, jti string, exp time.Time) error {
	if !r.Available() {
		return errors.New("redis unavailable")
	}
	ttl := time.Until(exp)
	if ttl <= 0 {
		return nil
	}
	return r.client.Set(ctx, "jti:"+jti, "1", ttl).Err()
}

func (r *Redis) IsJTIRevoked(ctx context.Context, jti string) bool {
	if !r.Available() {
		return false
	}
	n, err := r.client.Exists(ctx, "jti:"+jti).Result()
	if err != nil {
		r.log.Warn("redis exists failed", "err", err)
		return false
	}
	return n > 0
}

// ── 通用 KV 缓存（黑名单热点、配置等） ─────────────────────────
//
// Get 返回 (val, ok)。ok=false 表示 miss 或降级。
func (r *Redis) Get(ctx context.Context, key string) (string, bool) {
	if !r.Available() {
		return "", false
	}
	v, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

func (r *Redis) Set(ctx context.Context, key, val string, ttl time.Duration) error {
	if !r.Available() {
		return errors.New("redis unavailable")
	}
	return r.client.Set(ctx, key, val, ttl).Err()
}

func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if !r.Available() {
		return errors.New("redis unavailable")
	}
	return r.client.Del(ctx, keys...).Err()
}
