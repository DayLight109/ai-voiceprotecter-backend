package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func scanRecording(r interface {
	Scan(dest ...any) error
}) (domain.Recording, error) {
	var rec domain.Recording
	var ownerID, phone, duration, verdict, encKey *string
	err := r.Scan(&rec.ID, &rec.TenantID, &ownerID, &phone, &duration, &rec.SizeBytes, &verdict, &rec.ObjectKey, &encKey, &rec.CreatedAt)
	if err != nil {
		return rec, err
	}
	if ownerID != nil {
		rec.OwnerUserID = *ownerID
	}
	if phone != nil {
		rec.Phone = *phone
	}
	if duration != nil {
		rec.Duration = *duration
	}
	if verdict != nil {
		rec.Verdict = *verdict
	}
	if encKey != nil {
		rec.EncryptionKey = *encKey
	}
	return rec, nil
}

const recordingCols = `id, tenant_id, owner_user_id, phone, duration, COALESCE(size_bytes,0), verdict, object_key, encryption_key, created_at`

func (r *Repo) ListRecordings(ctx context.Context, tenantID string, p Page) ([]domain.Recording, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM recordings WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+recordingCols+` FROM recordings WHERE tenant_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.Recording, 0, limit)
	for rows.Next() {
		rec, err := scanRecording(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, rec)
	}
	return out, total, nil
}

func (r *Repo) GetRecordingByID(ctx context.Context, id string) (domain.Recording, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+recordingCols+` FROM recordings WHERE id = $1`, id)
	rec, err := scanRecording(row)
	return rec, translateErr(err)
}

type CreateRecordingParams struct {
	ID, TenantID, OwnerUserID, Phone, Duration, Verdict, ObjectKey, EncryptionKey string
	SizeBytes                                                                     int64
}

func (r *Repo) CreateRecording(ctx context.Context, p CreateRecordingParams) (domain.Recording, error) {
	var owner any = p.OwnerUserID
	if p.OwnerUserID == "" {
		owner = nil
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO recordings (id, tenant_id, owner_user_id, phone, duration, size_bytes, verdict, object_key, encryption_key)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NULLIF($7,''),$8,NULLIF($9,''))
		RETURNING `+recordingCols,
		p.ID, p.TenantID, owner, p.Phone, p.Duration, p.SizeBytes, p.Verdict, p.ObjectKey, p.EncryptionKey,
	)
	rec, err := scanRecording(row)
	return rec, translateErr(err)
}

func (r *Repo) DeleteRecording(ctx context.Context, id, tenantID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM recordings WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── 录音策略 ────────────────────────────────────────────────

type RecordingPolicy struct {
	TenantID      string `json:"tenantId"`
	UploadEnabled bool   `json:"uploadEnabled"`
}

func (r *Repo) GetRecordingPolicy(ctx context.Context, tenantID string) (RecordingPolicy, error) {
	var p RecordingPolicy
	err := r.pool.QueryRow(ctx,
		`SELECT tenant_id, upload_enabled FROM recording_policy WHERE tenant_id = $1`,
		tenantID).Scan(&p.TenantID, &p.UploadEnabled)
	return p, translateErr(err)
}

func (r *Repo) UpsertRecordingPolicy(ctx context.Context, tenantID string, enabled bool) (RecordingPolicy, error) {
	var p RecordingPolicy
	err := r.pool.QueryRow(ctx, `
		INSERT INTO recording_policy (tenant_id, upload_enabled)
		VALUES ($1, $2)
		ON CONFLICT (tenant_id) DO UPDATE
		SET upload_enabled = EXCLUDED.upload_enabled, updated_at = now()
		RETURNING tenant_id, upload_enabled`,
		tenantID, enabled).Scan(&p.TenantID, &p.UploadEnabled)
	return p, translateErr(err)
}
