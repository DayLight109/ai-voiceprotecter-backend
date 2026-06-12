// Package config 集中读取 env 变量。
//
// 设计原则：每个子系统的配置独立成 struct，便于注入与测试。
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string
	Addr     string
	LogLevel string

	JWT     JWTConfig
	DB      DBConfig
	Redis   RedisConfig
	MinIO   MinIOConfig
	AI      AIConfig
	CORS    CORSConfig
	Rate    RateConfig
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type DBConfig struct {
	URL string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type MinIOConfig struct {
	Endpoint            string
	AccessKey           string
	SecretKey           string
	UseSSL              bool
	BucketRecordings    string
	BucketModels        string
	BucketCredentials   string
}

type AIConfig struct {
	BaseURL string
	Timeout time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

type RateConfig struct {
	PerMinute int
}

// 安全约束：所有环境（含 development）都生效，避免上线时把占位串带到生产。
const (
	minSecretBytes = 32
)

// placeholderSecrets 是 .env.example 中已知的占位串。
// 启动时若 secret 含其中任一片段即拒绝，强迫部署方真正生成随机密钥。
var placeholderSecrets = []string{
	"change-me",
	"please-change",
	"your-secret",
	"replace-me",
	"placeholder",
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:      getenv("GATEWAY_ENV", "development"),
		Addr:     getenv("GATEWAY_ADDR", ":8080"),
		LogLevel: getenv("GATEWAY_LOG_LEVEL", "info"),

		JWT: JWTConfig{
			AccessSecret:  getenv("JWT_ACCESS_SECRET", ""),
			RefreshSecret: getenv("JWT_REFRESH_SECRET", ""),
			AccessTTL:     getduration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL:    getduration("JWT_REFRESH_TTL", 168*time.Hour),
		},
		DB: DBConfig{
			URL: getenv("DATABASE_URL", ""),
		},
		Redis: RedisConfig{
			Addr:     getenv("REDIS_ADDR", "redis:6379"),
			Password: getenv("REDIS_PASSWORD", ""),
			DB:       getint("REDIS_DB", 0),
		},
		MinIO: MinIOConfig{
			Endpoint:          getenv("MINIO_ENDPOINT", "minio:9000"),
			AccessKey:         getenv("MINIO_ACCESS_KEY", ""),
			SecretKey:         getenv("MINIO_SECRET_KEY", ""),
			UseSSL:            getbool("MINIO_USE_SSL", false),
			BucketRecordings:  getenv("MINIO_BUCKET_RECORDINGS", "sentinel-recordings"),
			BucketModels:      getenv("MINIO_BUCKET_MODELS", "sentinel-models"),
			BucketCredentials: getenv("MINIO_BUCKET_CREDENTIALS", "sentinel-credentials"),
		},
		AI: AIConfig{
			BaseURL: getenv("AI_BASE_URL", "http://ai:8090"),
			Timeout: getduration("AI_TIMEOUT", 30*time.Second),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		},
		Rate: RateConfig{
			PerMinute: getint("RATE_LIMIT_PER_MINUTE", 120),
		},
	}

	// JWT secret 强校验（所有环境一致）：
	//   - 非空
	//   - 长度 ≥ 32 字节
	//   - 不包含 .env.example 中的占位片段
	//   - access ≠ refresh（防 token 复用攻击）
	if err := validateSecret("JWT_ACCESS_SECRET", cfg.JWT.AccessSecret); err != nil {
		return nil, err
	}
	if err := validateSecret("JWT_REFRESH_SECRET", cfg.JWT.RefreshSecret); err != nil {
		return nil, err
	}
	if cfg.JWT.AccessSecret == cfg.JWT.RefreshSecret {
		return nil, errors.New("JWT_ACCESS_SECRET 必须不同于 JWT_REFRESH_SECRET")
	}

	// CORS 防御性检查：禁止配 "*"（与 Allow-Credentials:true 同时存在是高危组合）。
	for _, o := range cfg.CORS.AllowedOrigins {
		if o == "*" {
			return nil, errors.New("CORS_ALLOWED_ORIGINS 禁止配 '*'，必须列出具体 origin")
		}
	}

	// AI_BASE_URL 校验：必须是合法 http(s) URL，且不能指向云元数据 IP。
	if err := validateAIBaseURL(cfg.AI.BaseURL); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateSecret(name, val string) error {
	if val == "" {
		return fmt.Errorf("%s 必填", name)
	}
	if len(val) < minSecretBytes {
		return fmt.Errorf("%s 至少 %d 字节（当前 %d）", name, minSecretBytes, len(val))
	}
	low := strings.ToLower(val)
	for _, p := range placeholderSecrets {
		if strings.Contains(low, p) {
			return fmt.Errorf("%s 含占位片段 %q，请用 `openssl rand -hex 32` 重新生成", name, p)
		}
	}
	return nil
}

// metadataHosts 是云供应商的元数据服务地址，访问可获取实例凭据 / IAM 角色，
// gateway 调 AI 服务时不能误指过去（SSRF）。
var metadataHosts = map[string]bool{
	"169.254.169.254": true, // AWS / Azure / GCP
	"169.254.170.2":   true, // AWS ECS task metadata
	"100.100.100.200": true, // 阿里云
	"metadata.google.internal": true, // GCP DNS
}

func validateAIBaseURL(raw string) error {
	if raw == "" {
		return errors.New("AI_BASE_URL 必填")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("AI_BASE_URL 解析失败：%w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("AI_BASE_URL scheme 必须是 http 或 https（当前 %q）", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("AI_BASE_URL 缺少 host")
	}
	if metadataHosts[host] {
		return fmt.Errorf("AI_BASE_URL host %q 是云元数据地址，禁止使用", host)
	}
	return nil
}

// ── helpers ─────────────────────────────────────────────

func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func getint(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getbool(k string, def bool) bool {
	if v := strings.ToLower(os.Getenv(k)); v != "" {
		return v == "true" || v == "1" || v == "yes"
	}
	return def
}

func getduration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
