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

func (r *Repo) RevokeAllSessionsByUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE sessions SET revoked = true WHERE user_id = $1`, userID)
	return translateErr(err)
}

func (r *Repo) PurgeExpiredSessions(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now() - interval '30 days'`)
	return translateErr(err)
}
