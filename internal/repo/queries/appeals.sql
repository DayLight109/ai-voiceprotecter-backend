-- name: ListAppeals :many
SELECT * FROM appeals
WHERE ($1::text = '' OR tenant_id = $1)
  AND ($2::text = '' OR status = $2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAppeals :one
SELECT COUNT(*) FROM appeals
WHERE ($1::text = '' OR tenant_id = $1)
  AND ($2::text = '' OR status = $2);

-- name: GetAppealByID :one
SELECT * FROM appeals WHERE id = $1;

-- name: CreateAppeal :one
INSERT INTO appeals (id, user_id, tenant_id, type, number, reason, status)
VALUES ($1, $2, $3, $4, $5, $6, '处理中')
RETURNING *;

-- name: UpdateAppealStatus :one
UPDATE appeals
SET status = $2, resolved_at = now(), resolved_by = $3
WHERE id = $1
RETURNING *;
