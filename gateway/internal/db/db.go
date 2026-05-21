// Package db 集中管理 pgxpool 生命周期。
//
// 数据库是硬依赖：连接失败必须 fail-fast，不允许降级。
// 使用 pgxpool（连接池）而非单连接 —— 处理并发请求需要。
package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New 用 DATABASE_URL 初始化连接池并 Ping 一次确保可达。
//
// 参数：
//   url             postgres://user:pass@host:port/db?sslmode=disable
//   maxConns        池上限（推荐 20-50，看 PG max_connections）
//   connectTimeout  首次握手超时
func New(ctx context.Context, url string, log *slog.Logger) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("DATABASE_URL is empty")
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	// 合理默认；可后续从 config 覆盖
	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(pingCtx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	log.Info("db connected",
		"host", cfg.ConnConfig.Host,
		"db", cfg.ConnConfig.Database,
		"maxConns", cfg.MaxConns,
	)
	return pool, nil
}

// Close 优雅释放，super safe nil-friendly。
func Close(pool *pgxpool.Pool, log *slog.Logger) {
	if pool == nil {
		return
	}
	pool.Close()
	log.Info("db closed")
}
