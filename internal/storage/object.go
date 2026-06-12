// Package storage 封装 MinIO/S3 操作。
//
// 设计原则：
//   · 连不上 / 未配 endpoint → log.Warn + client=nil，方法返回错误（不静默成功）
//   · 真实 SDK：minio-go/v7
//   · 启动时 EnsureBuckets 自动建桶（dev 友好；生产 IaC 建议预建）
//
// 用法：
//   key, _ := s.Put(ctx, s.RecordingsBucket(), id+".wav", reader, size, "audio/wav")
//   url, _ := s.PresignGet(ctx, s.RecordingsBucket(), key, 5*time.Minute)
package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/sentinel/gateway/internal/config"
)

type MinIO struct {
	client *minio.Client
	cfg    config.MinIOConfig
	log    *slog.Logger
}

// New 建客户端并跑一次 ListBuckets 验证可达。
// 连不上不阻断 gateway 启动 —— 录音 / 模型 / 凭证类 API 在降级时返回 503。
func New(ctx context.Context, cfg config.MinIOConfig, log *slog.Logger) *MinIO {
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		log.Warn("minio not configured, storage methods will return errors")
		return &MinIO{cfg: cfg, log: log}
	}
	cli, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		log.Warn("minio init failed, degraded mode", "err", err)
		return &MinIO{cfg: cfg, log: log}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := cli.ListBuckets(probeCtx); err != nil {
		log.Warn("minio probe failed, degraded mode", "err", err, "endpoint", cfg.Endpoint)
		return &MinIO{cfg: cfg, log: log}
	}
	log.Info("minio connected", "endpoint", cfg.Endpoint, "ssl", cfg.UseSSL)
	return &MinIO{client: cli, cfg: cfg, log: log}
}

func (s *MinIO) Available() bool {
	return s != nil && s.client != nil
}

// EnsureBuckets 启动时调用，缺啥建啥。
func (s *MinIO) EnsureBuckets(ctx context.Context) error {
	if !s.Available() {
		return errors.New("minio not available")
	}
	for _, b := range []string{s.cfg.BucketRecordings, s.cfg.BucketModels, s.cfg.BucketCredentials} {
		if b == "" {
			continue
		}
		exists, err := s.client.BucketExists(ctx, b)
		if err != nil {
			return err
		}
		if !exists {
			if err := s.client.MakeBucket(ctx, b, minio.MakeBucketOptions{}); err != nil {
				return err
			}
			s.log.Info("minio bucket created", "name", b)
		}
	}
	return nil
}

// Put 上传对象，返回 key。
func (s *MinIO) Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) (string, error) {
	if !s.Available() {
		return "", errors.New("storage not initialized")
	}
	_, err := s.client.PutObject(ctx, bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

// Get 下载对象，调用方负责关闭 ReadCloser。
func (s *MinIO) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if !s.Available() {
		return nil, errors.New("storage not initialized")
	}
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// PresignGet 返回临时下载 URL。
func (s *MinIO) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	if !s.Available() {
		return "", errors.New("storage not initialized")
	}
	u, err := s.client.PresignedGetObject(ctx, bucket, key, ttl, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// PresignPut 返回临时上传 URL（前端直传大文件用）。
func (s *MinIO) PresignPut(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	if !s.Available() {
		return "", errors.New("storage not initialized")
	}
	u, err := s.client.PresignedPutObject(ctx, bucket, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Delete 删除对象。
func (s *MinIO) Delete(ctx context.Context, bucket, key string) error {
	if !s.Available() {
		return errors.New("storage not initialized")
	}
	return s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

// ── 桶名访问器（让 handler 不直接读 cfg） ────────────────────
func (s *MinIO) RecordingsBucket() string  { return s.cfg.BucketRecordings }
func (s *MinIO) ModelsBucket() string      { return s.cfg.BucketModels }
func (s *MinIO) CredentialsBucket() string { return s.cfg.BucketCredentials }
