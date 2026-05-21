-- name: ListAdminApplications :many
SELECT * FROM admin_applications
WHERE ($1::text = '' OR status = $1)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAdminApplications :one
SELECT COUNT(*) FROM admin_applications WHERE ($1::text = '' OR status = $1);

-- name: GetAdminApplicationByID :one
SELECT * FROM admin_applications WHERE id = $1;

-- name: GetAdminApplicationByUser :one
SELECT * FROM admin_applications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1;

-- name: CreateAdminApplication :one
INSERT INTO admin_applications (id, user_id, scope, reason, contact, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
RETURNING *;

-- name: UpdateAdminApplicationStatus :one
UPDATE admin_applications
SET status = $2, reviewed_at = now(), reviewed_by = $3
WHERE id = $1
RETURNING *;
