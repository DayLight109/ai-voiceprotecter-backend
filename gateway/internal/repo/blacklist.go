package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

type CreateBlacklistParams struct {
	ID, Number, Reason, Category, Source, CreatedBy string
	TenantID                                        string // 空表示全局（NULL）
	Risk                                            int
}

const blacklistColumns = `id, tenant_id, number, reason, category, risk, source, created_at`

func scanBlacklist(r interface {
	Scan(dest ...any) error
}) (domain.BlacklistEntry, error) {
	var e domain.BlacklistEntry
	var tenantID, reason *string
	err := r.Scan(&e.ID, &tenantID, &e.Number, &reason, &e.Category, &e.Risk, &e.Source, &e.CreatedAt)
	if err != nil {
		return e, err
	}
	if tenantID != nil {
		t := *tenantID
		e.TenantID = &t
	}
	if reason != nil {
		e.Reason = *reason
	}
	return e, nil
}

func (r *Repo) ListBlacklist(ctx context.Context, tenantID string, p Page) ([]domain.BlacklistEntry, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM blacklist WHERE (tenant_id = $1 OR tenant_id IS NULL)`,
		tenantID).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+blacklistColumns+` FROM blacklist
		 WHERE (tenant_id = $1 OR tenant_id IS NULL)
		 ORDER BY risk DESC, created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.BlacklistEntry, 0, limit)
	for rows.Next() {
		e, err := scanBlacklist(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, e)
	}
	return out, total, nil
}

func (r *Repo) SearchBlacklist(ctx context.Context, tenantID, q string, p Page) ([]domain.BlacklistEntry, int64, error) {
	limit, offset := p.Clamp()
	pat := "%" + q + "%"
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM blacklist
		WHERE (tenant_id = $1 OR tenant_id IS NULL)
		  AND (number ILIKE $2 OR reason ILIKE $2 OR category ILIKE $2)`,
		tenantID, pat).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+blacklistColumns+` FROM blacklist
		 WHERE (tenant_id = $1 OR tenant_id IS NULL)
		   AND (number ILIKE $2 OR reason ILIKE $2 OR category ILIKE $2)
		 ORDER BY risk DESC LIMIT $3 OFFSET $4`,
		tenantID, pat, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.BlacklistEntry, 0, limit)
	for rows.Next() {
		e, err := scanBlacklist(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, e)
	}
	return out, total, nil
}

func (r *Repo) GetBlacklistByNumber(ctx context.Context, tenantID, number string) (domain.BlacklistEntry, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+blacklistColumns+` FROM blacklist
		 WHERE (tenant_id = $1 OR tenant_id IS NULL) AND number = $2 LIMIT 1`,
		tenantID, number)
	e, err := scanBlacklist(row)
	return e, translateErr(err)
}

func (r *Repo) CreateBlacklist(ctx context.Context, p CreateBlacklistParams) (domain.BlacklistEntry, error) {
	var tenant any = p.TenantID
	if p.TenantID == "" {
		tenant = nil
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO blacklist (id, tenant_id, number, reason, category, risk, source, created_by)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,NULLIF($8,''))
		RETURNING `+blacklistColumns,
		p.ID, tenant, p.Number, p.Reason, p.Category, p.Risk, p.Source, p.CreatedBy,
	)
	e, err := scanBlacklist(row)
	return e, translateErr(err)
}

func (r *Repo) UpdateBlacklist(ctx context.Context, id, tenantID, role, number, reason, category string, risk int) (domain.BlacklistEntry, error) {
	// 授权收口：仅本租户条目可改；全局条目 (tenant_id IS NULL) 只命中 sysadmin 分支。
	// 越权 / 不存在统一走 ErrNoRows → ErrNotFound（404），不泄漏条目存在性。
	row := r.pool.QueryRow(ctx, `
		UPDATE blacklist SET number=$4, reason=NULLIF($5,''), category=$6, risk=$7
		WHERE id=$1 AND (tenant_id = $2 OR $3 = 'sysadmin')
		RETURNING `+blacklistColumns,
		id, tenantID, role, number, reason, category, risk,
	)
	e, err := scanBlacklist(row)
	return e, translateErr(err)
}

func (r *Repo) DeleteBlacklist(ctx context.Context, id, tenantID, role string) error {
	// 全局条目 (tenant_id IS NULL) 只能由 sysadmin 删除。
	// 不可写 `OR tenant_id IS NULL`，否则任意租户用户都能删除全局黑名单（与 queries/blacklist.sql 对齐）。
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM blacklist
		WHERE id = $1 AND (tenant_id = $2 OR $3 = 'sysadmin')`,
		id, tenantID, role,
	)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
