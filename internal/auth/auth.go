// Package auth 集中密码 hash 与 JWT 签发/验签。
//
// 签发：access (短 TTL，无状态) + refresh (长 TTL，hash 入库)。
// 撤销 access 走 Redis 黑名单（jti），refresh 走 DB sessions.revoked。
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/sentinel/gateway/internal/api/middleware"
	"github.com/sentinel/gateway/internal/config"
)

// HashPassword bcrypt cost=12（约 250 ms）
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	return string(b), err
}

func VerifyPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// Tokens 是登录成功后的返回
type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	AccessJTI    string `json:"-"`
	RefreshJTI   string `json:"-"`
	AccessExp    time.Time `json:"-"`
	RefreshExp   time.Time `json:"-"`
}

// Issue 签发一对 token
func Issue(cfg config.JWTConfig, uid, role, tenantID string) (*Tokens, error) {
	if cfg.AccessSecret == "" || cfg.RefreshSecret == "" {
		return nil, errors.New("JWT secrets not configured")
	}
	now := time.Now()
	accessJTI, _ := randomID(16)
	refreshJTI, _ := randomID(16)
	accessExp := now.Add(cfg.AccessTTL)
	refreshExp := now.Add(cfg.RefreshTTL)

	access := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{
		UID: uid, Role: role, TenantID: tenantID,
		SID: refreshJTI, // 关联所属 refresh 会话，用于"当前设备"定位与活跃打点
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        accessJTI,
			ExpiresAt: jwt.NewNumericDate(accessExp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	})
	at, err := access.SignedString([]byte(cfg.AccessSecret))
	if err != nil {
		return nil, err
	}

	refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{
		UID: uid, Role: role, TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        refreshJTI,
			ExpiresAt: jwt.NewNumericDate(refreshExp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	})
	rt, err := refresh.SignedString([]byte(cfg.RefreshSecret))
	if err != nil {
		return nil, err
	}

	return &Tokens{
		AccessToken:  at,
		RefreshToken: rt,
		AccessJTI:    accessJTI,
		RefreshJTI:   refreshJTI,
		AccessExp:    accessExp,
		RefreshExp:   refreshExp,
	}, nil
}

// VerifyRefresh 校验 refresh 并返回 claims
func VerifyRefresh(cfg config.JWTConfig, raw string) (*middleware.Claims, error) {
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(cfg.RefreshSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid refresh token")
	}
	return claims, nil
}

func randomID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
