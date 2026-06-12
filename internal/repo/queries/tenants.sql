-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = $1;

-- name: ListTenants :many
SELECT * FROM tenants WHERE kind = ANY($1::text[]) ORDER BY created_at DESC;

-- name: CreateTenant :one
INSERT INTO tenants (id, kind, name) VALUES ($1, $2, $3) RETURNING *;

-- name: DeleteTenant :exec
DELETE FROM tenants WHERE id = $1;
