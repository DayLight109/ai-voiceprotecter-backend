package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

const whitelistColumns = `id, tenant_id, number, COALESCE(name,''), COALESCE(relation,''), created_at`

func scanWhitelist(r interface {
	Scan(dest ...any) error
}) (domain.WhitelistEntry, error) {
	var e domain.WhitelistEntry
	err := r.Scan(&e.ID, &e.TenantID, &e.Number, &e.Name, &e.Relation, &e.CreatedAt)
	return e, err
}

func (r *Repo) ListWhitelist(ctx context.Context, tenantID string, p Page) ([]domain.WhitelistEntry, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM whitelist WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+whitelistColumns+` FROM whitelist WHERE tenant_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.WhitelistEntry, 0, limit)
	for rows.Next() {
		e, err := scanWhitelist(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, e)
	}
	return out, total, nil
}

func (r *Repo) GetWhitelistByNumber(ctx context.Context, tenantID, number string) (domain.WhitelistEntry, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+whitelistColumns+` FROM whitelist WHERE tenant_id = $1 AND number = $2 LIMIT 1`,
		tenantID, number)
	e, err := scanWhitelist(row)
	return e, translateErr(err)
}

type CreateWhitelistParams struct {
	ID, TenantID, Number, Name, Relation string
}

func (r *Repo) CreateWhitelist(ctx context.Context, p CreateWhitelistParams) (domain.WhitelistEntry, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO whitelist (id, tenant_id, number, name, relation)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''))
		RETURNING `+whitelistColumns,
		p.ID, p.TenantID, p.Number, p.Name, p.Relation,
	)
	e, err := scanWhitelist(row)
	return e, translateErr(err)
}

func (r *Repo) UpdateWhitelist(ctx context.Context, id, tenantID, number, name, relation string) (domain.WhitelistEntry, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE whitelist SET number=$3, name=NULLIF($4,''), relation=NULLIF($5,'')
		WHERE id=$1 AND tenant_id=$2 RETURNING `+whitelistColumns,
		id, tenantID, number, name, relation,
	)
	e, err := scanWhitelist(row)
	return e, translateErr(err)
}

func (r *Repo) DeleteWhitelist(ctx context.Context, id, tenantID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM whitelist WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
