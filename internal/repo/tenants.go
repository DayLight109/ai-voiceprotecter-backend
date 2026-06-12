package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func (r *Repo) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	var t domain.Tenant
	err := r.pool.QueryRow(ctx,
		`SELECT id, kind, name, created_at FROM tenants WHERE id = $1`, id).
		Scan(&t.ID, &t.Kind, &t.Name, &t.CreatedAt)
	return t, translateErr(err)
}

func (r *Repo) ListTenantsByKind(ctx context.Context, kinds []string) ([]domain.Tenant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, kind, name, created_at FROM tenants
		 WHERE kind = ANY($1::text[]) ORDER BY created_at DESC`, kinds)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := []domain.Tenant{}
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Kind, &t.Name, &t.CreatedAt); err != nil {
			return nil, translateErr(err)
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *Repo) CreateTenant(ctx context.Context, id, kind, name string) (domain.Tenant, error) {
	var t domain.Tenant
	err := r.pool.QueryRow(ctx,
		`INSERT INTO tenants (id, kind, name) VALUES ($1, $2, $3)
		 RETURNING id, kind, name, created_at`,
		id, kind, name).Scan(&t.ID, &t.Kind, &t.Name, &t.CreatedAt)
	return t, translateErr(err)
}

func (r *Repo) DeleteTenant(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	return translateErr(err)
}
