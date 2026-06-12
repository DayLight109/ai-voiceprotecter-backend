package repo

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation 检测 pgx 中的 unique_violation 错误码 23505。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
