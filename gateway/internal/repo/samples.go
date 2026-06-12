package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func scanSample(r interface {
	Scan(dest ...any) error
}) (domain.Sample, error) {
	var s domain.Sample
	var callID, transcript, duration, origin, classification, audioKey *string
	err := r.Scan(&s.ID, &callID, &transcript, &duration, &origin, &classification, &s.Status, &audioKey, &s.ReceivedAt)
	if err != nil {
		return s, err
	}
	if callID != nil {
		s.CallID = *callID
	}
	if transcript != nil {
		s.Transcript = *transcript
	}
	if duration != nil {
		s.Duration = *duration
	}
	if origin != nil {
		s.Origin = *origin
	}
	if classification != nil {
		s.Classification = *classification
	}
	if audioKey != nil {
		s.AudioKey = *audioKey
	}
	return s, nil
}

const sampleCols = `id, call_id, transcript, duration, origin, classification, status, audio_key, received_at`

func (r *Repo) ListSamples(ctx context.Context, status string, p Page) ([]domain.Sample, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM samples WHERE ($1::text='' OR status=$1)`,
		status).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+sampleCols+` FROM samples
		 WHERE ($1::text='' OR status=$1)
		 ORDER BY received_at DESC LIMIT $2 OFFSET $3`,
		status, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.Sample, 0, limit)
	for rows.Next() {
		s, err := scanSample(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, s)
	}
	return out, total, nil
}

func (r *Repo) GetSampleByID(ctx context.Context, id string) (domain.Sample, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+sampleCols+` FROM samples WHERE id = $1`, id)
	s, err := scanSample(row)
	return s, translateErr(err)
}

type CreateSampleParams struct {
	ID, CallID, Transcript, Duration, Origin, Classification, Status, AudioKey string
	TenantID                                                                   string // 样本来源租户；空 = 全局
}

func (r *Repo) CreateSample(ctx context.Context, p CreateSampleParams) (domain.Sample, error) {
	var tenant any = p.TenantID
	if p.TenantID == "" {
		tenant = nil
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO samples (id, call_id, transcript, duration, origin, classification, status, audio_key, tenant_id)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), $7, NULLIF($8,''), $9)
		RETURNING `+sampleCols,
		p.ID, p.CallID, p.Transcript, p.Duration, p.Origin, p.Classification, defaultStr(p.Status, "待审核"), p.AudioKey, tenant,
	)
	s, err := scanSample(row)
	return s, translateErr(err)
}

func (r *Repo) UpdateSampleStatus(ctx context.Context, id, status, classification string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE samples SET status=$2, classification=NULLIF($3,'') WHERE id=$1`,
		id, status, classification)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
