package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

func scanDevice(r interface {
	Scan(dest ...any) error
}) (domain.Device, error) {
	var d domain.Device
	var tenantID *string
	err := r.Scan(&d.ID, &d.Name, &tenantID, &d.Type, &d.Status, &d.Version, &d.LastSeenAt, &d.Contact)
	if err != nil {
		return d, err
	}
	if tenantID != nil {
		t := *tenantID
		d.TenantID = &t
	}
	return d, nil
}

const deviceColumns = `id, name, tenant_id, type, status, version, last_seen_at, COALESCE(contact,'')`

func (r *Repo) ListDevices(ctx context.Context, deviceType, tenantID string, p Page) ([]domain.Device, int64, error) {
	limit, offset := p.Clamp()
	var total int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM devices
		WHERE ($1::text='' OR type=$1) AND ($2::text='' OR tenant_id=$2)`,
		deviceType, tenantID).Scan(&total); err != nil {
		return nil, 0, translateErr(err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+deviceColumns+` FROM devices
		WHERE ($1::text='' OR type=$1) AND ($2::text='' OR tenant_id=$2)
		ORDER BY last_seen_at DESC NULLS LAST LIMIT $3 OFFSET $4`,
		deviceType, tenantID, limit, offset)
	if err != nil {
		return nil, 0, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.Device, 0, limit)
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, 0, translateErr(err)
		}
		out = append(out, d)
	}
	return out, total, nil
}

func (r *Repo) GetDeviceByID(ctx context.Context, id string) (domain.Device, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+deviceColumns+` FROM devices WHERE id = $1`, id)
	d, err := scanDevice(row)
	return d, translateErr(err)
}

type CreateDeviceParams struct {
	ID, Name, TenantID, Type, Status, Version, Contact string
}

func (r *Repo) CreateDevice(ctx context.Context, p CreateDeviceParams) (domain.Device, error) {
	var tenant any = p.TenantID
	if p.TenantID == "" {
		tenant = nil
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO devices (id, name, tenant_id, type, status, version, contact)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''))
		RETURNING `+deviceColumns,
		p.ID, p.Name, tenant, p.Type, defaultStr(p.Status, "offline"), p.Version, p.Contact,
	)
	d, err := scanDevice(row)
	return d, translateErr(err)
}

func (r *Repo) UpdateDevice(ctx context.Context, id, name, status, version, contact string) (domain.Device, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE devices SET name=$2, status=$3, version=$4, contact=NULLIF($5,'')
		WHERE id=$1 RETURNING `+deviceColumns,
		id, name, status, version, contact,
	)
	d, err := scanDevice(row)
	return d, translateErr(err)
}

func (r *Repo) HeartbeatDevice(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE devices SET last_seen_at = now(), status='online' WHERE id = $1`, id)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) DeleteDevice(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM devices WHERE id = $1`, id)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkStaleOffline 把超过 ttl 没心跳的设备置为 offline。由后台任务调用。
func (r *Repo) MarkStaleOffline(ctx context.Context, ttlSeconds int) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE devices SET status='offline'
		WHERE last_seen_at IS NOT NULL
		  AND last_seen_at < now() - ($1::int || ' seconds')::interval
		  AND status <> 'offline'`, ttlSeconds)
	if err != nil {
		return 0, translateErr(err)
	}
	return tag.RowsAffected(), nil
}
