// Package api 是 HTTP 层的总装。
//
// router.go 把所有 handler 挂载到 chi。一个 handler 包含一个 New 函数返回 chi.Router。
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sentinel/gateway/internal/aiclient"
	"github.com/sentinel/gateway/internal/api/handlers"
	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/config"
	"github.com/sentinel/gateway/internal/feed"
	"github.com/sentinel/gateway/internal/ops"
	"github.com/sentinel/gateway/internal/repo"
	"github.com/sentinel/gateway/internal/storage"
	"github.com/sentinel/gateway/internal/store"
)

// Deps 把要注入给 handler 的依赖打包传递。
type Deps struct {
	Cfg     *config.Config
	Logger  *slog.Logger
	DB      *pgxpool.Pool
	Redis   *store.Redis
	Repo    *repo.Repo
	Hub     *feed.Hub
	Store   *store.Store
	AI      *aiclient.Client
	Storage *storage.MinIO
	Ops     *ops.Sampler
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// ── 全局中间件 ────────────────────────────────────────
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logging(d.Logger))
	r.Use(chimw.Recoverer)
	r.Use(middleware.CORS(d.Cfg.CORS.AllowedOrigins))
	r.Use(middleware.RateLimit(d.Cfg.Rate.PerMinute, d.Redis))

	// ── 健康检查 (无鉴权) ────────────────────────────────
	r.Get("/api/v1/health", handlers.Health(handlers.Deps{
		Cfg: d.Cfg, Logger: d.Logger, DB: d.DB, Redis: d.Redis, Storage: d.Storage, AI: d.AI,
	}))

	// ── 业务 API ────────────────────────────────────────
	hd := handlers.Deps{
		Cfg:     d.Cfg,
		Logger:  d.Logger,
		DB:      d.DB,
		Redis:   d.Redis,
		Repo:    d.Repo,
		Hub:     d.Hub,
		Store:   d.Store,
		AI:      d.AI,
		Storage: d.Storage,
		Ops:     d.Ops,
	}

	// ── 运维监测 (无鉴权；仅聚合指标，不含业务数据) ──────
	r.Mount("/api/v1/ops", handlers.OpsRouter(hd))

	r.Route("/api/v1", func(r chi.Router) {
		// 公开端点（登录 / 注册 / refresh / logout —— 各自内部按需挂限流）
		r.Mount("/auth", handlers.AuthRouter(hd))

		// 需要鉴权 ─────────────────────────────────────
		r.Group(func(r chi.Router) {
			// 会话活跃打点：Auth 验签成功后异步落 sessions.last_seen_at（中间件内已节流）
			touch := func(sid string) {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					if err := d.Repo.TouchSessionLastSeen(ctx, sid); err != nil {
						d.Logger.Warn("touch session failed", "err", err)
					}
				}()
			}
			r.Use(middleware.Auth(d.Cfg.JWT, d.Redis, touch))
			r.Use(middleware.Tenant)
			r.Use(middleware.Audit(d.Logger, d.DB))

			r.Get("/me", handlers.Me(hd))
			r.Get("/me/", handlers.Me(hd)) // 前端 me() 调带尾斜杠的 /me/
			r.Put("/me", handlers.UpdateMe(hd))
			r.Put("/me/", handlers.UpdateMe(hd))
			r.Put("/me/password", handlers.ChangePassword(hd))
			r.Get("/me/sessions", handlers.ListSessions(hd))
			r.Delete("/me/sessions/others", handlers.RevokeOtherSessions(hd))
			r.Delete("/me/sessions/{token}", handlers.RevokeSession(hd))
			r.Put("/me/avatar", handlers.UploadAvatar(hd))
			r.Get("/me/avatar", handlers.GetAvatar(hd))
			r.Delete("/me/avatar", handlers.DeleteAvatar(hd))
			r.Mount("/me/credentials", handlers.IdentityRouter(hd))
			r.Mount("/me/identity-modes", handlers.IdentityModesRouter(hd))
			r.Mount("/me/emergency-contacts", handlers.EmergencyContactsRouter(hd))

			r.Post("/analyze", handlers.Analyze(hd))

			// SSE & warroom 兼容（鉴权后按 tenant 过滤；sysadmin 看全部）。
			// DEFCON 设置敏感，仅 sysadmin 可改。
			r.Get("/stats", handlers.Stats(hd))
			r.Get("/defcon", handlers.DefconGet(hd))
			r.With(middleware.RequireRole("sysadmin")).Post("/defcon", handlers.DefconSet(hd))
			r.Get("/feed", handlers.FeedRecent(hd))
			r.Get("/feed/stream", handlers.FeedStream(hd))
			r.Get("/threats", handlers.Threats(hd))
			r.Get("/warroom/overview", handlers.WarroomOverview(hd))
			r.Get("/warroom/voiceprint/latest", handlers.WarroomLatestVoiceprint(hd))

			r.Mount("/blacklist", handlers.BlacklistRouter(hd))
			r.Mount("/whitelist", handlers.WhitelistRouter(hd))
			r.Mount("/knowledge", handlers.KnowledgeRouter(hd))
			r.Mount("/scam-rules", handlers.RulesRouter(hd))
			r.Mount("/risk-level", handlers.RiskLevelRouter(hd))
			r.Mount("/samples", handlers.SamplesRouter(hd))
			r.Mount("/voice-models", handlers.VoiceModelsRouter(hd))
			r.Mount("/voice-samples", handlers.VoiceSamplesRouter(hd))
			r.Mount("/agents", handlers.AgentsRouter(hd))
			r.Mount("/recordings", handlers.RecordingsRouter(hd))
			r.Mount("/calls", handlers.CallsRouter(hd))
			r.Mount("/users", handlers.UsersRouter(hd))
			r.Mount("/appeals", handlers.AppealsRouter(hd))
			r.Mount("/admin-apply", handlers.AdminApplyRouter(hd))
			r.Mount("/permissions", handlers.PermissionsRouter(hd))
			r.Mount("/devices", handlers.DevicesRouter(hd))
			r.Mount("/audit", handlers.AuditRouter(hd))
			r.Mount("/dashboard", handlers.DashboardRouter(hd))
		})
	})

	r.NotFound(handlers.NotFound)

	return r
}
