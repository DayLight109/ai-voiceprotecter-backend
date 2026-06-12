-- name: ListKnowledge :many
SELECT id, title, category, summary, views, updated_at
FROM knowledge_articles
WHERE status = 'published'
  AND ($1::text = '' OR category = $1)
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountKnowledge :one
SELECT COUNT(*) FROM knowledge_articles
WHERE status = 'published' AND ($1::text = '' OR category = $1);

-- name: GetKnowledgeByID :one
SELECT * FROM knowledge_articles WHERE id = $1;

-- name: IncrementKnowledgeView :exec
UPDATE knowledge_articles SET views = views + 1 WHERE id = $1;

-- name: CreateKnowledge :one
INSERT INTO knowledge_articles (id, title, category, summary, body, status, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateKnowledge :one
UPDATE knowledge_articles
SET title=$2, category=$3, summary=$4, body=$5, status=$6, updated_by=$7, updated_at=now()
WHERE id=$1
RETURNING *;

-- name: DeleteKnowledge :exec
DELETE FROM knowledge_articles WHERE id = $1;
