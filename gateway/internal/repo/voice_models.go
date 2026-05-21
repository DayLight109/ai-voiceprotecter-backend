package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func scanVoiceModel(r interface {
	Scan(dest ...any) error
}) (domain.VoiceModel, error) {
	var m domain.VoiceModel
	err := r.Scan(&m.ID, &m.Version, &m.Accuracy, &m.SizeBytes, &m.ObjectKey, &m.Active, &m.UploadedAt)
	return m, err
}

const voiceModelCols = `id, version, accuracy::float8, size_bytes, object_key, active, uploaded_at`

func (r *Repo) ListVoiceModels(ctx context.Context, p Page) ([]domain.VoiceModel, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM voice_models`).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+voiceModelCols+` FROM voice_models ORDER BY uploaded_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.VoiceModel, 0, limit)
	for rows.Next() {
		m, err := scanVoiceModel(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, m)
	}
	return out, total, nil
}

func (r *Repo) GetVoiceModelByID(ctx context.Context, id string) (domain.VoiceModel, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+voiceModelCols+` FROM voice_models WHERE id = $1`, id)
	m, err := scanVoiceModel(row)
	return m, translateErr(err)
}

func (r *Repo) GetActiveVoiceModel(ctx context.Context) (domain.VoiceModel, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+voiceModelCols+` FROM voice_models WHERE active = true LIMIT 1`)
	m, err := scanVoiceModel(row)
	return m, translateErr(err)
}

type CreateVoiceModelParams struct {
	ID, Version, ObjectKey string
	Accuracy               float64
	SizeBytes              int64
}

func (r *Repo) CreateVoiceModel(ctx context.Context, p CreateVoiceModelParams) (domain.VoiceModel, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO voice_models (id, version, accuracy, size_bytes, object_key, active)
		VALUES ($1,$2,$3,$4,$5,false)
		RETURNING `+voiceModelCols,
		p.ID, p.Version, p.Accuracy, p.SizeBytes, p.ObjectKey,
	)
	m, err := scanVoiceModel(row)
	return m, translateErr(err)
}

// ActivateVoiceModel 同时把其余模型设为 inactive。
func (r *Repo) ActivateVoiceModel(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE voice_models SET active = (id = $1)`, id)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteVoiceModel(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM voice_models WHERE id = $1 AND active = false`, id)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
