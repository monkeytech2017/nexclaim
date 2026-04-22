package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// DoctorMap = row ใน his_doctor_map (per-รพ. mapping: HIS doctor code → m_doctor.doctor_id).
// Composite PK (hcode, his_doctor_code).
type DoctorMap struct {
	HCode         string `db:"hcode"           json:"hcode"`
	HISDoctorCode string `db:"his_doctor_code" json:"his_doctor_code"`
	DoctorID      string `db:"doctor_id"       json:"doctor_id,omitempty"`
}

type DoctorMapRepo interface {
	List(ctx context.Context, hcode string) ([]DoctorMap, error)
	Get(ctx context.Context, hcode, hisDoctorCode string) (*DoctorMap, error)
	Upsert(ctx context.Context, m DoctorMap) (*DoctorMap, error)
	Delete(ctx context.Context, hcode, hisDoctorCode string) error
}

type PgDoctorMapRepo struct{ db *sqlx.DB }

func NewPgDoctorMapRepo(db *sqlx.DB) *PgDoctorMapRepo { return &PgDoctorMapRepo{db: db} }

func (r *PgDoctorMapRepo) List(ctx context.Context, hcode string) ([]DoctorMap, error) {
	var out []DoctorMap
	q := `
		SELECT hcode, his_doctor_code, COALESCE(doctor_id,'') AS doctor_id
		FROM his_doctor_map
	`
	args := []interface{}{}
	if hcode != "" {
		q += ` WHERE hcode = $1`
		args = append(args, hcode)
	}
	q += ` ORDER BY hcode, his_doctor_code`
	err := r.db.SelectContext(ctx, &out, q, args...)
	return out, err
}

func (r *PgDoctorMapRepo) Get(ctx context.Context, hcode, hisDoctorCode string) (*DoctorMap, error) {
	var m DoctorMap
	err := r.db.GetContext(ctx, &m, `
		SELECT hcode, his_doctor_code, COALESCE(doctor_id,'') AS doctor_id
		FROM his_doctor_map WHERE hcode = $1 AND his_doctor_code = $2
	`, hcode, hisDoctorCode)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PgDoctorMapRepo) Upsert(ctx context.Context, m DoctorMap) (*DoctorMap, error) {
	if m.HCode == "" || len(m.HCode) != 5 {
		return nil, fmt.Errorf("hcode must be exactly 5 chars")
	}
	if m.HISDoctorCode == "" {
		return nil, fmt.Errorf("his_doctor_code required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO his_doctor_map (hcode, his_doctor_code, doctor_id)
		VALUES ($1, $2, NULLIF($3,''))
		ON CONFLICT (hcode, his_doctor_code) DO UPDATE SET
			doctor_id = EXCLUDED.doctor_id
	`, m.HCode, m.HISDoctorCode, m.DoctorID)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, m.HCode, m.HISDoctorCode)
}

func (r *PgDoctorMapRepo) Delete(ctx context.Context, hcode, hisDoctorCode string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM his_doctor_map WHERE hcode = $1 AND his_doctor_code = $2
	`, hcode, hisDoctorCode)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

var _ DoctorMapRepo = (*PgDoctorMapRepo)(nil)
