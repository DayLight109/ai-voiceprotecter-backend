package repo

import (
	"context"
	"time"

	"github.com/sentinel/gateway/internal/domain"
)

// UserRow 是 users 表完整投影（含 password_hash）。
// 业务层一般用 domain.User（去掉敏感字段）。
type UserRow struct {
	ID           string
	TenantID     string
	Name         string
	Phone        string
	Email        string
	IDCardHash   string
	PasswordHash string
	Role         string
	Status       string
	Dept         string
	LastLoginAt  *time.Time
	CreatedAt    time.Time
}

func (u UserRow) ToDomain() domain.User {
	return domain.User{
		ID: u.ID, TenantID: u.TenantID, Name: u.Name,
		Phone: u.Phone, Email: u.Email,
		Role: domain.Role(u.Role), Status: u.Status, Dept: u.Dept,
		LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt,
	}
}

func scanUserRow(r interface {
	Scan(dest ...any) error
}) (UserRow, error) {
	var u UserRow
	var phone, email, idHash, dept *string
	err := r.Scan(
		&u.ID, &u.TenantID, &u.Name, &phone, &idHash, &email,
		&u.PasswordHash, &u.Role, &u.Status, &dept,
		&u.LastLoginAt, &u.CreatedAt,
	)
	if err != nil {
		return UserRow{}, err
	}
	if phone != nil {
		u.Phone = *phone
	}
	if email != nil {
		u.Email = *email
	}
	if idHash != nil {
		u.IDCardHash = *idHash
	}
	if dept != nil {
		u.Dept = *dept
	}
	return u, nil
}

const userColumns = `id, tenant_id, name, phone, id_card_hash, email, password_hash, role, status, dept, last_login_at, created_at`

func (r *Repo) GetUserByID(ctx context.Context, id string) (UserRow, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	u, err := scanUserRow(row)
	return u, translateErr(err)
}

func (r *Repo) GetUserByPhone(ctx context.Context, phone string) (UserRow, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE phone = $1`, phone)
	u, err := scanUserRow(row)
	return u, translateErr(err)
}

func (r *Repo) GetUserByAccount(ctx context.Context, account string) (UserRow, error) {
	// 登录支持 phone / email 任一
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE phone = $1 OR email = $1 LIMIT 1`, account)
	u, err := scanUserRow(row)
	return u, translateErr(err)
}

func (r *Repo) ListUsersByTenant(ctx context.Context, tenantID string, p Page) ([]domain.User, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND status <> 'suspended'`,
		tenantID).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id = $1 AND status <> 'suspended'
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.User, 0, limit)
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, u.ToDomain())
	}
	return out, total, nil
}

type CreateUserParams struct {
	ID, TenantID, Name, Phone, Email, PasswordHash, Role, Status, Dept string
}

func (r *Repo) CreateUser(ctx context.Context, p CreateUserParams) (UserRow, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (id, tenant_id, name, phone, email, password_hash, role, status, dept)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,$7,$8,NULLIF($9,''))
		RETURNING `+userColumns,
		p.ID, p.TenantID, p.Name, p.Phone, p.Email, p.PasswordHash, p.Role, defaultStr(p.Status, "active"), p.Dept,
	)
	u, err := scanUserRow(row)
	return u, translateErr(err)
}

type UpdateUserParams struct {
	ID, Name, Dept, Role, Status string
}

func (r *Repo) UpdateUser(ctx context.Context, p UpdateUserParams) (UserRow, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE users SET name=$2, dept=NULLIF($3,''), role=$4, status=$5
		WHERE id=$1 RETURNING `+userColumns,
		p.ID, p.Name, p.Dept, p.Role, p.Status,
	)
	u, err := scanUserRow(row)
	return u, translateErr(err)
}

// UpdateUserProfile 只更新用户可自助修改的资料字段（不动 role/status）。
// 空串走 NULLIF 落库为 NULL；email 唯一性冲突翻成 ErrConflict。
func (r *Repo) UpdateUserProfile(ctx context.Context, id, name, phone, email, dept string) (UserRow, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE users
		   SET name=$2, phone=NULLIF($3,''), email=NULLIF($4,''), dept=NULLIF($5,''), updated_at=now()
		 WHERE id=$1 RETURNING `+userColumns,
		id, name, phone, email, dept,
	)
	u, err := scanUserRow(row)
	return u, translateErr(err)
}

// SetUserPassword 改密码（已 hash）。
func (r *Repo) SetUserPassword(ctx context.Context, id, passwordHash string) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash=$2, updated_at=now() WHERE id=$1`, id, passwordHash)
	if err != nil {
		return translateErr(err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UserHasAvatar 当前用户是否已上传头像。
func (r *Repo) UserHasAvatar(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_avatars WHERE user_id=$1)`, id).Scan(&exists)
	return exists, translateErr(err)
}

func (r *Repo) TouchUserLogin(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return translateErr(err)
}

func (r *Repo) DeleteUser(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return translateErr(err)
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
