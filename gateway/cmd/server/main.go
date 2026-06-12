// Sentinel Gateway · 主入口
//
// 启动顺序:
//   1. 加载 env 配置
//   2. 初始化 DB(必需) / Redis(可降级) / MinIO(可降级)
//   3. 初始化 AI client / Feed Hub / Store
//   4. 装配 chi 路由
//   5. 监听信号 → 优雅关停（先关 HTTP，再关 DB / Redis）
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sentinel/gateway/internal/aiclient"
	"github.com/sentinel/gateway/internal/api"
	"github.com/sentinel/gateway/internal/config"
	"github.com/sentinel/gateway/internal/db"
	"github.com/sentinel/gateway/internal/feed"
	"github.com/sentinel/gateway/internal/ops"
	"github.com/sentinel/gateway/internal/repo"
	"github.com/sentinel/gateway/internal/storage"
	"github.com/sentinel/gateway/internal/store"
)

func main() {
	addrFlag := flag.String("addr", "", "listen address (overrides env)")
	flag.Parse()

	// ── 日志 ──────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// ── 配置 ──────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}
	if *addrFlag != "" {
		cfg.Addr = *addrFlag
	}
	logger.Info("config loaded", "env", cfg.Env, "addr", cfg.Addr)

	// ── 基础设施初始化 ────────────────────────────────────
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer bootCancel()

	// DB · 必需，连不上直接退出
	pool, err := db.New(bootCtx, cfg.DB.URL, logger)
	if err != nil {
		logger.Error("db init failed (fatal)", "err", err)
		os.Exit(1)
	}
	defer db.Close(pool, logger)

	// Redis · 可降级
	rds := store.NewRedis(bootCtx, cfg.Redis, logger)
	defer rds.Close()

	// MinIO · 可降级；启动时建桶
	obj := storage.New(bootCtx, cfg.MinIO, logger)
	if obj.Available() {
		if err := obj.EnsureBuckets(bootCtx); err != nil {
			logger.Warn("ensure buckets failed", "err", err)
		}
	}

	// SSE / 计数器 / AI client
	hub := feed.NewHub(logger)
	st := store.New()
	ai := aiclient.New(cfg.AI.BaseURL, cfg.AI.Timeout, logger)

	// 数据访问层
	dataRepo := repo.New(pool)

	// 计数器播种：从 call_logs 重建历史口径，重启不清零
	if stats, err := dataRepo.CountCallLogStats(bootCtx); err != nil {
		logger.Warn("seed counters failed", "err", err)
	} else {
		st.Seed(stats.Total, stats.Blocked, stats.AIClones, stats.ScriptHits)
		logger.Info("counters seeded from call_logs",
			"total", stats.Total, "blocked", stats.Blocked,
			"aiClones", stats.AIClones, "scriptHits", stats.ScriptHits)
	}

	// 运维指标采样器（/ops/series、/warroom/overview 数据源）：5s × 360 = 30 分钟窗口
	sampler := ops.NewSampler(5*time.Second, 360)
	sampler.Start()
	defer sampler.Stop()

	// ── HTTP 路由装配 ────────────────────────────────────
	router := api.NewRouter(api.Deps{
		Cfg:     cfg,
		Logger:  logger,
		DB:      pool,
		Redis:   rds,
		Repo:    dataRepo,
		Hub:     hub,
		Store:   st,
		AI:      ai,
		Storage: obj,
		Ops:     sampler,
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // SSE 不能整体超时
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("gateway listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// ── 后台任务：超过 90s 没心跳的设备置为 offline ───────
	bgStop := startDeviceSweeper(dataRepo, logger, 90)
	defer bgStop()

	// ── 优雅关停 ─────────────────────────────────────────
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
	logger.Info("bye")
}

// startDeviceSweeper 每 30s 把超过 ttl 没心跳的设备置为 offline。
// 返回 stop 函数，调用即停止。
func startDeviceSweeper(r *repo.Repo, log *slog.Logger, ttlSeconds int) func() {
	ticker := time.NewTicker(30 * time.Second)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if n, err := r.MarkStaleOffline(ctx, ttlSeconds); err != nil {
					log.Warn("device sweeper failed", "err", err)
				} else if n > 0 {
					log.Info("device sweeper marked offline", "count", n)
				}
				cancel()
			}
		}
	}()
	return func() { close(done) }
}
