package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

// KnowledgeSummary 列表用，省去 body
type KnowledgeSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Summary   string `json:"summary"`
	Views     int64  `json:"views"`
	UpdatedAt string `json:"updatedAt"`
}

func (r *Repo) ListKnowledge(ctx context.Context, category string, p Page) ([]domain.KnowledgeArticle, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM knowledge_articles
		 WHERE status='published' AND ($1::text='' OR category=$1)`,
		category).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, category, COALESCE(summary,''), '', views, status, updated_at
		FROM knowledge_articles
		WHERE status='published' AND ($1::text='' OR category=$1)
		ORDER BY updated_at DESC LIMIT $2 OFFSET $3`,
		category, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.KnowledgeArticle, 0, limit)
	for rows.Next() {
		var k domain.KnowledgeArticle
		if err := rows.Scan(&k.ID, &k.Title, &k.Category, &k.Summary, &k.Body, &k.Views, &k.Status, &k.UpdatedAt); err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, k)
	}
	return out, total, nil
}

func (r *Repo) GetKnowledgeByID(ctx context.Context, id string) (domain.KnowledgeArticle, error) {
	var k domain.KnowledgeArticle
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, category, COALESCE(summary,''), body, views, status, updated_at
		FROM knowledge_articles WHERE id=$1`, id).
		Scan(&k.ID, &k.Title, &k.Category, &k.Summary, &k.Body, &k.Views, &k.Status, &k.UpdatedAt)
	return k, translateErr(err)
}

func (r *Repo) IncrementKnowledgeView(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE knowledge_articles SET views = views + 1 WHERE id = $1`, id)
	return translateErr(err)
}

type CreateKnowledgeParams struct {
	ID, Title, Category, Summary, Body, Status, UpdatedBy string
}

func (r *Repo) CreateKnowledge(ctx context.Context, p CreateKnowledgeParams) (domain.KnowledgeArticle, error) {
	var k domain.KnowledgeArticle
	err := r.pool.QueryRow(ctx, `
		INSERT INTO knowledge_articles (id, title, category, summary, body, status, updated_by)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,NULLIF($7,''))
		RETURNING id, title, category, COALESCE(summary,''), body, views, status, updated_at`,
		p.ID, p.Title, p.Category, p.Summary, p.Body, defaultStr(p.Status, "published"), p.UpdatedBy,
	).Scan(&k.ID, &k.Title, &k.Category, &k.Summary, &k.Body, &k.Views, &k.Status, &k.UpdatedAt)
	return k, translateErr(err)
}

func (r *Repo) UpdateKnowledge(ctx context.Context, p CreateKnowledgeParams) (domain.KnowledgeArticle, error) {
	var k domain.KnowledgeArticle
	err := r.pool.QueryRow(ctx, `
		UPDATE knowledge_articles
		SET title=$2, category=$3, summary=NULLIF($4,''), body=$5, status=$6, updated_by=NULLIF($7,''), updated_at=now()
		WHERE id=$1
		RETURNING id, title, category, COALESCE(summary,''), body, views, status, updated_at`,
		p.ID, p.Title, p.Category, p.Summary, p.Body, p.Status, p.UpdatedBy,
	).Scan(&k.ID, &k.Title, &k.Category, &k.Summary, &k.Body, &k.Views, &k.Status, &k.UpdatedAt)
	return k, translateErr(err)
}

func (r *Repo) DeleteKnowledge(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM knowledge_articles WHERE id = $1`, id)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
