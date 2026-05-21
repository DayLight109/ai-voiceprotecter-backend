// Package repo 数据访问层（DAL）。
//
// 设计：
//   · 一个 Repo struct，持有 *pgxpool.Pool；按表分文件挂方法。
//   · 等价于 sqlc 手写版本——sqlc 工具链不在本仓库时也能编译运行。
//   · 不暴露 pgx 类型给上层；所有方法返回 internal/domain 中的实体。
//   · 错误用 fmt.Errorf 包一层语义（CodeNotFound / CodeConflict / Internal）
//     由 service 层翻译成 HTTP 错误码。
//
// 命名：
//   ListXxx / CountXxx / GetXxxByY / CreateXxx / UpdateXxx / DeleteXxx
package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo 数据访问层根。
type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

// ── 错误语义 ─────────────────────────────────────────────────

var (
	ErrNotFound = errors.New("repo: not found")
	ErrConflict = errors.New("repo: conflict")
)

// translateErr 把 pgx ErrNoRows 翻成 domain ErrNotFound；
// 把 23505 unique_violation 翻成 ErrConflict。
func translateErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	// pgx PgError 解析 unique_violation
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return fmt.Errorf("repo: %w", err)
}

// ── 辅助：分页 ───────────────────────────────────────────────

// Page 计算 limit / offset 并 clamp（1 起页码，pageSize 1..100）。
type Page struct {
	Page     int
	PageSize int
}

func (p Page) Clamp() (limit, offset int) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
	limit = p.PageSize
	offset = (p.Page - 1) * p.PageSize
	return
}

// ── 事务封装 ────────────────────────────────────────────────

// WithTx 在事务中执行 fn；任一返回错误自动回滚。
func (r *Repo) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return translateErr(err)
	}
	defer tx.Rollback(ctx) // commit 后无害
	if err := fn(tx); err != nil {
		return err
	}
	return translateErr(tx.Commit(ctx))
}
