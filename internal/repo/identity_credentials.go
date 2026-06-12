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
	Masked     string
	UpdatedAt  *time.Time
}

func (c CredentialRow) ToDomain() domain.IdentityCredential {
	return domain.IdentityCredential{
		ID: c.ID, UserID: c.UserID, Kind: c.Kind,
		Verified: c.Verified, VerifiedAt: c.VerifiedAt,
		Masked: c.Masked, UpdatedAt: c.UpdatedAt,
	}
}

const credentialCols = `id, user_id, kind, COALESCE(value_hash,''), verified, verified_at, COALESCE(masked,''), updated_at`

func scanCredential(r interface {
	Scan(dest ...any) error
}) (CredentialRow, error) {
	var c CredentialRow
	err := r.Scan(&c.ID, &c.UserID, &c.Kind, &c.ValueHash, &c.Verified, &c.VerifiedAt, &c.Masked, &c.UpdatedAt)
	return c, err
}

func (r *Repo) ListCredentialsByUser(ctx context.Context, userID string) ([]domain.IdentityCredential, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+credentialCols+`
		FROM identity_credentials WHERE user_id = $1 ORDER BY kind`, userID)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := []domain.IdentityCredential{}
	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return nil, translateErr(err)
		}
		out = append(out, c.ToDomain())
	}
	return out, nil
}

func (r *Repo) GetCredentialByKind(ctx context.Context, userID, kind string) (CredentialRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+credentialCols+`
		FROM identity_credentials WHERE user_id = $1 AND kind = $2`, userID, kind)
	c, err := scanCredential(row)
	return c, translateErr(err)
}

type UpsertCredentialParams struct {
	ID, UserID, Kind, ValueHash, Masked string
	Verified                            bool
}

func (r *Repo) UpsertCredential(ctx context.Context, p UpsertCredentialParams) (domain.IdentityCredential, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO identity_credentials (id, user_id, kind, value_hash, masked, verified, verified_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6, CASE WHEN $6 THEN now() ELSE NULL END, now())
		ON CONFLICT (user_id, kind) DO UPDATE
		SET value_hash = EXCLUDED.value_hash,
		    masked = EXCLUDED.masked,
		    verified = EXCLUDED.verified,
		    verified_at = CASE WHEN EXCLUDED.verified THEN now() ELSE identity_credentials.verified_at END,
		    updated_at = now()
		RETURNING `+credentialCols,
		p.ID, p.UserID, p.Kind, p.ValueHash, p.Masked, p.Verified,
	)
	c, err := scanCredential(row)
	return c.ToDomain(), translateErr(err)
}

func (r *Repo) DeleteCredential(ctx context.Context, userID, kind string) error {
	// 证件记录与其照片一并删除
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM identity_photos WHERE user_id = $1 AND kind = $2`, userID, kind); err != nil {
		return translateErr(err)
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM identity_credentials WHERE user_id = $1 AND kind = $2`, userID, kind)
	return translateErr(err)
}

// ── 证件照片（identity_photos，bytea 存储） ─────────────────────

type CredentialPhotoRow struct {
	UserID      string
	Kind        string
	Slot        string
	Name        string
	ContentType string
	Bytes       []byte
	UpdatedAt   time.Time
}

type UpsertCredentialPhotoParams struct {
	UserID, Kind, Slot, Name, ContentType string
	Bytes                                 []byte
}

func (r *Repo) UpsertCredentialPhoto(ctx context.Context, p UpsertCredentialPhotoParams) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO identity_photos (user_id, kind, slot, name, content_type, bytes, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (user_id, kind, slot) DO UPDATE
		SET name = EXCLUDED.name,
		    content_type = EXCLUDED.content_type,
		    bytes = EXCLUDED.bytes,
		    updated_at = now()`,
		p.UserID, p.Kind, p.Slot, p.Name, p.ContentType, p.Bytes,
	)
	return translateErr(err)
}

func (r *Repo) DeleteCredentialPhoto(ctx context.Context, userID, kind, slot string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM identity_photos WHERE user_id = $1 AND kind = $2 AND slot = $3`,
		userID, kind, slot)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) ListCredentialPhotosByUser(ctx context.Context, userID string) ([]CredentialPhotoRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, kind, slot, name, content_type, bytes, updated_at
		FROM identity_photos WHERE user_id = $1 ORDER BY kind, slot`, userID)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := []CredentialPhotoRow{}
	for rows.Next() {
		var p CredentialPhotoRow
		if err := rows.Scan(&p.UserID, &p.Kind, &p.Slot, &p.Name, &p.ContentType, &p.Bytes, &p.UpdatedAt); err != nil {
			return nil, translateErr(err)
		}
		out = append(out, p)
	}
	return out, nil
}
