package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func scanScamRule(r interface {
	Scan(dest ...any) error
}) (domain.ScamRule, error) {
	var s domain.ScamRule
	err := r.Scan(&s.ID, &s.Category, &s.Keyword, &s.Weight, &s.Enabled)
	return s, err
}

func (r *Repo) ListScamRules(ctx context.Context, category string, p Page) ([]domain.ScamRule, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scam_rules WHERE ($1::text='' OR category=$1)`,
		category).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, category, keyword, weight, enabled FROM scam_rules
		WHERE ($1::text='' OR category=$1)
		ORDER BY weight DESC, category LIMIT $2 OFFSET $3`,
		category, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.ScamRule, 0, limit)
	for rows.Next() {
		s, err := scanScamRule(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, s)
	}
	return out, total, nil
}

func (r *Repo) ListEnabledScamRules(ctx context.Context) ([]domain.ScamRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, category, keyword, weight, enabled FROM scam_rules
		 WHERE enabled = true ORDER BY weight DESC`)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := []domain.ScamRule{}
	for rows.Next() {
		s, err := scanScamRule(rows)
		if err != nil {
			return nil, translateErr(err)
		}
		out = append(out, s)
	}
	return out, nil
}

type CreateScamRuleParams struct {
	ID, Category, Keyword string
	Weight                int
	Enabled               bool
}

func (r *Repo) CreateScamRule(ctx context.Context, p CreateScamRuleParams) (domain.ScamRule, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO scam_rules (id, category, keyword, weight, enabled)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, category, keyword, weight, enabled`,
		p.ID, p.Category, p.Keyword, p.Weight, p.Enabled,
	)
	return scanScamRule(row)
}

func (r *Repo) UpdateScamRule(ctx context.Context, p CreateScamRuleParams) (domain.ScamRule, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE scam_rules SET category=$2, keyword=$3, weight=$4, enabled=$5, updated_at=now()
		WHERE id=$1
		RETURNING id, category, keyword, weight, enabled`,
		p.ID, p.Category, p.Keyword, p.Weight, p.Enabled,
	)
	s, err := scanScamRule(row)
	return s, translateErr(err)
}

func (r *Repo) DeleteScamRule(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM scam_rules WHERE id = $1`, id)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
