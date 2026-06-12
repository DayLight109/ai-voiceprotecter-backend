package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

type CreateBlacklistParams struct {
	ID, Number, Reason, Category, Source, CreatedBy string
	TenantID                                        string // 空表示全局（NULL）
	Risk                                            int
	Dispatched                                      bool // false = 举报通过自动入库、待下发
}

const blacklistColumns = `id, tenant_id, number, reason, category, risk, source, dispatched, created_at`

func scanBlacklist(r interface {
	Scan(dest ...any) error
}) (domain.BlacklistEntry, error) {
	var e domain.BlacklistEntry
	var tenantID, reason *string
	err := r.Scan(&e.ID, &tenantID, &e.Number, &reason, &e.Category, &e.Risk, &e.Source, &e.Dispatched, &e.CreatedAt)
	if err != nil {
		return e, err
	}
	if tenantID != nil {
		t := *tenantID
		e.TenantID = &t
	}
	e.IsGlobal = tenantID == nil
	if reason != nil {
		e.Reason = *reason
	}
	return e, nil
}

// blacklistScopeCond 由 scope 决定可见范围：
//   ""       → 本租户 + 全局（默认，与历史行为一致）
//   "tenant" → 仅本租户
//   "global" → 仅全局（sysadmin 总库页）
// tenantID 为 $1、scope 为 $2 时的 WHERE 片段。
const blacklistScopeCond = `(
	   ($2 = ''       AND (tenant_id = $1 OR tenant_id IS NULL))
	OR ($2 = 'tenant' AND tenant_id = $1)
	OR ($2 = 'global' AND tenant_id IS NULL)
)`

func (r *Repo) ListBlacklist(ctx context.Context, tenantID, scope string, p Page) ([]domain.BlacklistEntry, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM blacklist WHERE `+blacklistScopeCond,
		tenantID, scope).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+blacklistColumns+` FROM blacklist
		 WHERE `+blacklistScopeCond+`
		 ORDER BY risk DESC, created_at DESC LIMIT $3 OFFSET $4`,
		tenantID, scope, limit, offset)
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

func (r *Repo) SearchBlacklist(ctx context.Context, tenantID, scope, q string, p Page) ([]domain.BlacklistEntry, int64, error) {
	limit, offset := p.Clamp()
	pat := "%" + q + "%"
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM blacklist
		WHERE `+blacklistScopeCond+`
		  AND (number ILIKE $3 OR reason ILIKE $3 OR category ILIKE $3)`,
		tenantID, scope, pat).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+blacklistColumns+` FROM blacklist
		 WHERE `+blacklistScopeCond+`
		   AND (number ILIKE $3 OR reason ILIKE $3 OR category ILIKE $3)
		 ORDER BY risk DESC LIMIT $4 OFFSET $5`,
		tenantID, scope, pat, limit, offset)
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
		INSERT INTO blacklist (id, tenant_id, number, reason, category, risk, source, dispatched, created_by)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,NULLIF($9,''))
		RETURNING `+blacklistColumns,
		p.ID, tenant, p.Number, p.Reason, p.Category, p.Risk, p.Source, p.Dispatched, p.CreatedBy,
	)
	e, err := scanBlacklist(row)
	return e, translateErr(err)
}

// DispatchBlacklist 把"待下发"条目正式生效（dispatched=false → true）。
//   - admin / family_admin：仅可下发本租户条目，就地生效；
//   - sysadmin：下发即提升为全局（tenant_id 置 NULL），全网生效。
//     若全局名单已有同号码条目，唯一索引会拒绝 → ErrConflict。
func (r *Repo) DispatchBlacklist(ctx context.Context, id, tenantID, role string) (domain.BlacklistEntry, error) {
	var row interface{ Scan(dest ...any) error }
	if role == "sysadmin" {
		row = r.pool.QueryRow(ctx, `
			UPDATE blacklist SET dispatched = true, tenant_id = NULL
			WHERE id = $1
			RETURNING `+blacklistColumns, id)
	} else {
		row = r.pool.QueryRow(ctx, `
			UPDATE blacklist SET dispatched = true
			WHERE id = $1 AND tenant_id = $2
			RETURNING `+blacklistColumns, id, tenantID)
	}
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
