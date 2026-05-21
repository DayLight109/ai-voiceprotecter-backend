package repo

import (
	"context"
	"time"

	"github.com/sentinel/gateway/internal/domain"
)

type CredentialRow struct {
	ID         string
	UserID     string
	Kind       string
	ValueHash  string
	Verified   bool
	VerifiedAt *time.Time
}

func (c CredentialRow) ToDomain() domain.IdentityCredential {
	return domain.IdentityCredential{
		ID: c.ID, UserID: c.UserID, Kind: c.Kind,
		Verified: c.Verified, VerifiedAt: c.VerifiedAt,
	}
}

func (r *Repo) ListCredentialsByUser(ctx context.Context, userID string) ([]domain.IdentityCredential, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, kind, COALESCE(value_hash,''), verified, verified_at
		FROM identity_credentials WHERE user_id = $1 ORDER BY kind`, userID)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := []domain.IdentityCredential{}
	for rows.Next() {
		var c CredentialRow
		if err := rows.Scan(&c.ID, &c.UserID, &c.Kind, &c.ValueHash, &c.Verified, &c.VerifiedAt); err != nil {
			return nil, translateErr(err)
		}
		out = append(out, c.ToDomain())
	}
	return out, nil
}

func (r *Repo) GetCredentialByKind(ctx context.Context, userID, kind string) (CredentialRow, error) {
	var c CredentialRow
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, kind, COALESCE(value_hash,''), verified, verified_at
		FROM identity_credentials WHERE user_id = $1 AND kind = $2`, userID, kind).
		Scan(&c.ID, &c.UserID, &c.Kind, &c.ValueHash, &c.Verified, &c.VerifiedAt)
	return c, translateErr(err)
}

type UpsertCredentialParams struct {
	ID, UserID, Kind, ValueHash string
	Verified                    bool
}

func (r *Repo) UpsertCredential(ctx context.Context, p UpsertCredentialParams) (domain.IdentityCredential, error) {
	var c CredentialRow
	err := r.pool.QueryRow(ctx, `
		INSERT INTO identity_credentials (id, user_id, kind, value_hash, verified, verified_at)
		VALUES ($1, $2, $3, NULLIF($4,''), $5, CASE WHEN $5 THEN now() ELSE NULL END)
		ON CONFLICT (user_id, kind) DO UPDATE
		SET value_hash = EXCLUDED.value_hash,
		    verified = EXCLUDED.verified,
		    verified_at = CASE WHEN EXCLUDED.verified THEN now() ELSE identity_credentials.verified_at END
		RETURNING id, user_id, kind, COALESCE(value_hash,''), verified, verified_at`,
		p.ID, p.UserID, p.Kind, p.ValueHash, p.Verified,
	).Scan(&c.ID, &c.UserID, &c.Kind, &c.ValueHash, &c.Verified, &c.VerifiedAt)
	return c.ToDomain(), translateErr(err)
}

func (r *Repo) DeleteCredential(ctx context.Context, userID, kind string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM identity_credentials WHERE user_id = $1 AND kind = $2`, userID, kind)
	return translateErr(err)
}
