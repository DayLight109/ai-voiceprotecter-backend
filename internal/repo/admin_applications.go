package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func scanAdminApp(r interface {
	Scan(dest ...any) error
}) (domain.AdminApplication, error) {
	var a domain.AdminApplication
	err := r.Scan(&a.ID, &a.UserID, &a.Scope, &a.Reason, &a.Contact, &a.Status, &a.CreatedAt)
	return a, err
}

const adminAppCols = `id, user_id, scope, reason, contact, status, created_at`

func (r *Repo) ListAdminApplications(ctx context.Context, status string, p Page) ([]domain.AdminApplication, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM admin_applications WHERE ($1::text='' OR status=$1)`,
		status).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+adminAppCols+` FROM admin_applications
		WHERE ($1::text='' OR status=$1)
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		status, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.AdminApplication, 0, limit)
	for rows.Next() {
		a, err := scanAdminApp(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, a)
	}
	return out, total, nil
}

func (r *Repo) GetAdminApplicationByID(ctx context.Context, id string) (domain.AdminApplication, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+adminAppCols+` FROM admin_applications WHERE id = $1`, id)
	a, err := scanAdminApp(row)
	return a, translateErr(err)
}

func (r *Repo) GetLatestAdminApplicationByUser(ctx context.Context, userID string) (domain.AdminApplication, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+adminAppCols+` FROM admin_applications
		WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID)
	a, err := scanAdminApp(row)
	return a, translateErr(err)
}

type CreateAdminApplicationParams struct {
	ID, UserID, Scope, Reason, Contact string
}

func (r *Repo) CreateAdminApplication(ctx context.Context, p CreateAdminApplicationParams) (domain.AdminApplication, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO admin_applications (id, user_id, scope, reason, contact, status)
		VALUES ($1,$2,$3,$4,$5,'pending')
		RETURNING `+adminAppCols,
		p.ID, p.UserID, p.Scope, p.Reason, p.Contact,
	)
	a, err := scanAdminApp(row)
	return a, translateErr(err)
}

func (r *Repo) UpdateAdminApplicationStatus(ctx context.Context, id, status, reviewedBy string) (domain.AdminApplication, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE admin_applications SET status=$2, reviewed_at=now(), reviewed_by=NULLIF($3,'')
		WHERE id=$1
		RETURNING `+adminAppCols,
		id, status, reviewedBy,
	)
	a, err := scanAdminApp(row)
	return a, translateErr(err)
}

// DeletePendingAdminApplicationsByUser 撤回当前用户所有待审核申请，返回删除条数。
func (r *Repo) DeletePendingAdminApplicationsByUser(ctx context.Context, userID string) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM admin_applications WHERE user_id = $1 AND status = 'pending'`, userID)
	if err != nil {
		return 0, translateErr(err)
	}
	return tag.RowsAffected(), nil
}
