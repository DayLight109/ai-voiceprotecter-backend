package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func scanAppeal(r interface {
	Scan(dest ...any) error
}) (domain.Appeal, error) {
	var a domain.Appeal
	err := r.Scan(&a.ID, &a.UserID, &a.TenantID, &a.Type, &a.Number, &a.Reason, &a.Status, &a.CreatedAt, &a.ResolvedAt)
	return a, err
}

const appealCols = `id, user_id, tenant_id, type, number, reason, status, created_at, resolved_at`

func (r *Repo) ListAppeals(ctx context.Context, tenantID, status string, p Page) ([]domain.Appeal, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM appeals
		WHERE ($1::text='' OR tenant_id=$1) AND ($2::text='' OR status=$2)`,
		tenantID, status).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+appealCols+` FROM appeals
		WHERE ($1::text='' OR tenant_id=$1) AND ($2::text='' OR status=$2)
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
		tenantID, status, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.Appeal, 0, limit)
	for rows.Next() {
		a, err := scanAppeal(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, a)
	}
	return out, total, nil
}

func (r *Repo) GetAppealByID(ctx context.Context, id string) (domain.Appeal, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+appealCols+` FROM appeals WHERE id = $1`, id)
	a, err := scanAppeal(row)
	return a, translateErr(err)
}

type CreateAppealParams struct {
	ID, UserID, TenantID, Type, Number, Reason string
}

func (r *Repo) CreateAppeal(ctx context.Context, p CreateAppealParams) (domain.Appeal, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO appeals (id, user_id, tenant_id, type, number, reason, status)
		VALUES ($1,$2,$3,$4,$5,$6,'处理中')
		RETURNING `+appealCols,
		p.ID, p.UserID, p.TenantID, p.Type, p.Number, p.Reason,
	)
	a, err := scanAppeal(row)
	return a, translateErr(err)
}

func (r *Repo) UpdateAppealStatus(ctx context.Context, id, status, resolvedBy string) (domain.Appeal, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE appeals SET status=$2, resolved_at=now(), resolved_by=NULLIF($3,'')
		WHERE id=$1
		RETURNING `+appealCols,
		id, status, resolvedBy,
	)
	a, err := scanAppeal(row)
	return a, translateErr(err)
}
