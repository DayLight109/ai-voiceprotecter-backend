package repo

import (
	"context"
	"time"
)

type SessionRow struct {
	JTI              string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
	Revoked          bool
	UserAgent        string
	IP               string
	CreatedAt        time.Time
	LastSeenAt       time.Time
}

type CreateSessionParams struct {
	JTI, UserID, RefreshTokenHash string
	ExpiresAt                     time.Time
	UserAgent, IP                 string
}

func (r *Repo) CreateSession(ctx context.Context, p CreateSessionParams) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions (jti, user_id, refresh_token_hash, expires_at, user_agent, ip)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,'')::inet)`,
		p.JTI, p.UserID, p.RefreshTokenHash, p.ExpiresAt, p.UserAgent, p.IP,
	)
	return translateErr(err)
}

func (r *Repo) GetSession(ctx context.Context, jti string) (SessionRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT jti, user_id, refresh_token_hash, expires_at, revoked,
		       COALESCE(user_agent,''), COALESCE(host(ip),''), created_at
		FROM sessions WHERE jti = $1`, jti)
	var s SessionRow
	err := row.Scan(&s.JTI, &s.UserID, &s.RefreshTokenHash, &s.ExpiresAt, &s.Revoked, &s.UserAgent, &s.IP, &s.CreatedAt)
	return s, translateErr(err)
}

func (r *Repo) RevokeSession(ctx context.Context, jti string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked = true WHERE jti = $1`, jti)
	return translateErr(err)
}

// TouchSessionLastSeen 更新会话活跃时间（Auth 中间件节流后调用）。
func (r *Repo) TouchSessionLastSeen(ctx context.Context, jti string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now() WHERE jti = $1`, jti)
	return translateErr(err)
}

func (r *Repo) RevokeAllSessionsByUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked = true WHERE user_id = $1`, userID)
	return translateErr(err)
}

// ListSessionsByUser 返回某用户全部未撤销且未过期的会话，按最近活跃倒序。
// 用于 /me/sessions 登录设备列表。
func (r *Repo) ListSessionsByUser(ctx context.Context, userID string) ([]SessionRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT jti, user_id, refresh_token_hash, expires_at, revoked,
		       COALESCE(user_agent,''), COALESCE(host(ip),''), created_at,
		       COALESCE(last_seen_at, created_at)
		FROM sessions
		WHERE user_id = $1 AND revoked = false AND expires_at > now()
		ORDER BY COALESCE(last_seen_at, created_at) DESC`, userID)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := make([]SessionRow, 0, 8)
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.JTI, &s.UserID, &s.RefreshTokenHash, &s.ExpiresAt,
			&s.Revoked, &s.UserAgent, &s.IP, &s.CreatedAt, &s.LastSeenAt); err != nil {
			return nil, translateErr(err)
		}
		out = append(out, s)
	}
	return out, translateErr(rows.Err())
}

// RevokeSessionsExcept 撤销该用户除 keepJTI 外的所有会话，返回撤销条数。
func (r *Repo) RevokeSessionsExcept(ctx context.Context, userID, keepJTI string) (int64, error) {
	ct, err := r.pool.Exec(ctx,
		`UPDATE sessions SET revoked = true WHERE user_id = $1 AND jti <> $2 AND revoked = false`,
		userID, keepJTI)
	if err != nil {
		return 0, translateErr(err)
	}
	return ct.RowsAffected(), nil
}

func (r *Repo) PurgeExpiredSessions(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now() - interval '30 days'`)
	return translateErr(err)
}
