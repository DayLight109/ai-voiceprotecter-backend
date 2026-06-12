package repo

import (
	"context"

	"github.com/sentinel/gateway/internal/domain"
)

const emergencyContactColumns = `id, user_id, name, phone, COALESCE(relation,''), created_at`

func scanEmergencyContact(r interface {
	Scan(dest ...any) error
}) (domain.EmergencyContact, error) {
	var e domain.EmergencyContact
	err := r.Scan(&e.ID, &e.UserID, &e.Name, &e.Phone, &e.Relation, &e.CreatedAt)
	return e, err
}

func (r *Repo) ListEmergencyContacts(ctx context.Context, userID string) ([]domain.EmergencyContact, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+emergencyContactColumns+` FROM emergency_contacts
		 WHERE user_id = $1 ORDER BY created_at ASC`,
		userID)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()
	out := make([]domain.EmergencyContact, 0, 4)
	for rows.Next() {
		e, err := scanEmergencyContact(rows)
		if err != nil {
			return nil, translateErr(err)
		}
		out = append(out, e)
	}
	return out, nil
}

type CreateEmergencyContactParams struct {
	ID, UserID, Name, Phone, Relation string
}

func (r *Repo) CreateEmergencyContact(ctx context.Context, p CreateEmergencyContactParams) (domain.EmergencyContact, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO emergency_contacts (id, user_id, name, phone, relation)
		VALUES ($1, $2, $3, $4, COALESCE(NULLIF($5,''),''))
		RETURNING `+emergencyContactColumns,
		p.ID, p.UserID, p.Name, p.Phone, p.Relation,
	)
	e, err := scanEmergencyContact(row)
	return e, translateErr(err)
}

func (r *Repo) UpdateEmergencyContact(ctx context.Context, id, userID, name, phone, relation string) (domain.EmergencyContact, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE emergency_contacts
		   SET name = $3, phone = $4, relation = COALESCE(NULLIF($5,''),'')
		 WHERE id = $1 AND user_id = $2
		 RETURNING `+emergencyContactColumns,
		id, userID, name, phone, relation,
	)
	e, err := scanEmergencyContact(row)
	return e, translateErr(err)
}

func (r *Repo) DeleteEmergencyContact(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM emergency_contacts WHERE id = $1 AND user_id = $2`,
		id, userID)
	if err != nil {
		return translateErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
