package repo

import (
	"context"
	"time"
)

type VoiceSampleRow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SizeBytes int64     `json:"size"`
	Tag       string    `json:"tag"`
	ObjectKey string    `json:"objectKey,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (r *Repo) ListVoiceSamples(ctx context.Context, p Page) ([]VoiceSampleRow, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM voice_samples`).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, size_bytes, tag, object_key, created_at FROM voice_samples
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]VoiceSampleRow, 0, limit)
	for rows.Next() {
		var v VoiceSampleRow
		if err := rows.Scan(&v.ID, &v.Name, &v.SizeBytes, &v.Tag, &v.ObjectKey, &v.CreatedAt); err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, v)
	}
	return out, total, nil
}

type CreateVoiceSampleParams struct {
	ID, Name, Tag, ObjectKey string
	SizeBytes                int64
}

func (r *Repo) CreateVoiceSample(ctx context.Context, p CreateVoiceSampleParams) (VoiceSampleRow, error) {
	var v VoiceSampleRow
	err := r.pool.QueryRow(ctx, `
		INSERT INTO voice_samples (id, name, size_bytes, tag, object_key)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, name, size_bytes, tag, object_key, created_at`,
		p.ID, p.Name, p.SizeBytes, p.Tag, p.ObjectKey,
	).Scan(&v.ID, &v.Name, &v.SizeBytes, &v.Tag, &v.ObjectKey, &v.CreatedAt)
	return v, translateErr(err)
}

func (r *Repo) GetVoiceSampleByID(ctx context.Context, id string) (VoiceSampleRow, error) {
	var v VoiceSampleRow
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, size_bytes, tag, object_key, created_at FROM voice_samples WHERE id = $1`, id,
	).Scan(&v.ID, &v.Name, &v.SizeBytes, &v.Tag, &v.ObjectKey, &v.CreatedAt)
	return v, translateErr(err)
}

func (r *Repo) DeleteVoiceSample(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM voice_samples WHERE id = $1`, id)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
