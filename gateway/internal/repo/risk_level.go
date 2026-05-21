package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func (r *Repo) GetRiskLevelState(ctx context.Context, tenantID string) (domain.RiskLevelState, error) {
	var s domain.RiskLevelState
	err := r.pool.QueryRow(ctx,
		`SELECT tenant_id, active_level FROM risk_level_state WHERE tenant_id = $1`,
		tenantID).Scan(&s.TenantID, &s.ActiveLevel)
	return s, translateErr(err)
}

func (r *Repo) UpsertRiskLevelState(ctx context.Context, tenantID string, level int) (domain.RiskLevelState, error) {
	var s domain.RiskLevelState
	err := r.pool.QueryRow(ctx, `
		INSERT INTO risk_level_state (tenant_id, active_level)
		VALUES ($1, $2)
		ON CONFLICT (tenant_id) DO UPDATE
		SET active_level = EXCLUDED.active_level, updated_at = now()
		RETURNING tenant_id, active_level`,
		tenantID, level).Scan(&s.TenantID, &s.ActiveLevel)
	return s, translateErr(err)
}

func scanRiskRule(r interface {
	Scan(dest ...any) error
}) (domain.RiskLevelRule, error) {
	var x domain.RiskLevelRule
	err := r.Scan(&x.ID, &x.TenantID, &x.Level, &x.Keyword, &x.Weight, &x.Enabled)
	return x, err
}

func (r *Repo) ListRiskLevelRules(ctx context.Context, tenantID string, level int, p Page) ([]domain.RiskLevelRule, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM risk_level_rules
		WHERE tenant_id=$1 AND ($2::int = 0 OR level = $2)`,
		tenantID, level).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, level, keyword, weight, enabled
		FROM risk_level_rules
		WHERE tenant_id=$1 AND ($2::int = 0 OR level = $2)
		ORDER BY level, weight DESC LIMIT $3 OFFSET $4`,
		tenantID, level, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.RiskLevelRule, 0, limit)
	for rows.Next() {
		x, err := scanRiskRule(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, x)
	}
	return out, total, nil
}

type CreateRiskLevelRuleParams struct {
	ID, TenantID, Keyword string
	Level, Weight         int
	Enabled               bool
}

func (r *Repo) CreateRiskLevelRule(ctx context.Context, p CreateRiskLevelRuleParams) (domain.RiskLevelRule, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO risk_level_rules (id, tenant_id, level, keyword, weight, enabled)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, tenant_id, level, keyword, weight, enabled`,
		p.ID, p.TenantID, p.Level, p.Keyword, p.Weight, p.Enabled,
	)
	x, err := scanRiskRule(row)
	return x, translateErr(err)
}

func (r *Repo) UpdateRiskLevelRule(ctx context.Context, p CreateRiskLevelRuleParams) (domain.RiskLevelRule, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE risk_level_rules
		SET level=$3, keyword=$4, weight=$5, enabled=$6, updated_at=now()
		WHERE id=$1 AND tenant_id=$2
		RETURNING id, tenant_id, level, keyword, weight, enabled`,
		p.ID, p.TenantID, p.Level, p.Keyword, p.Weight, p.Enabled,
	)
	x, err := scanRiskRule(row)
	return x, translateErr(err)
}

func (r *Repo) DeleteRiskLevelRule(ctx context.Context, id, tenantID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM risk_level_rules WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
