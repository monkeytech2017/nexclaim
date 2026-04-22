package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Hospital = row ใน m_hospital.
// HCode เป็น PK 5 หลัก; his_db_key ใช้เลือก credential connection ของ HIS
// ต่อ รพ. (ตอนนี้ยังไม่ต่อ DB-extractor แต่ schema ready).
type Hospital struct {
	HCode     string    `db:"hcode"      json:"hcode"`
	NameTH    string    `db:"name_th"    json:"name_th"`
	Changwat  string    `db:"changwat"   json:"changwat,omitempty"`
	Amphur    string    `db:"amphur"     json:"amphur,omitempty"`
	HISDBKey  string    `db:"his_db_key" json:"his_db_key,omitempty"`
	IsActive  bool      `db:"is_active"  json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ErrNotFound is returned when a requested row doesn't exist.
var ErrNotFound = errors.New("not found")

// HospitalRepo persists m_hospital rows.
type HospitalRepo interface {
	List(ctx context.Context) ([]Hospital, error)
	Get(ctx context.Context, hcode string) (*Hospital, error)
	Upsert(ctx context.Context, h Hospital) (*Hospital, error)
	Delete(ctx context.Context, hcode string) error
}

type PgHospitalRepo struct{ db *sqlx.DB }

func NewPgHospitalRepo(db *sqlx.DB) *PgHospitalRepo { return &PgHospitalRepo{db: db} }

func (r *PgHospitalRepo) List(ctx context.Context) ([]Hospital, error) {
	var out []Hospital
	err := r.db.SelectContext(ctx, &out, `
		SELECT hcode, name_th, COALESCE(changwat,'') AS changwat,
		       COALESCE(amphur,'') AS amphur,
		       COALESCE(his_db_key,'') AS his_db_key,
		       is_active, created_at
		FROM m_hospital
		ORDER BY hcode
	`)
	return out, err
}

func (r *PgHospitalRepo) Get(ctx context.Context, hcode string) (*Hospital, error) {
	var h Hospital
	err := r.db.GetContext(ctx, &h, `
		SELECT hcode, name_th, COALESCE(changwat,'') AS changwat,
		       COALESCE(amphur,'') AS amphur,
		       COALESCE(his_db_key,'') AS his_db_key,
		       is_active, created_at
		FROM m_hospital WHERE hcode = $1
	`, hcode)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// Upsert: insert new, or update name/location/is_active if hcode exists.
// created_at is preserved on update.
func (r *PgHospitalRepo) Upsert(ctx context.Context, h Hospital) (*Hospital, error) {
	if h.HCode == "" || len(h.HCode) != 5 {
		return nil, fmt.Errorf("hcode must be exactly 5 chars")
	}
	if h.NameTH == "" {
		return nil, fmt.Errorf("name_th required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO m_hospital
		  (hcode, name_th, changwat, amphur, his_db_key, is_active, created_at)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6, now())
		ON CONFLICT (hcode) DO UPDATE SET
			name_th    = EXCLUDED.name_th,
			changwat   = EXCLUDED.changwat,
			amphur     = EXCLUDED.amphur,
			his_db_key = EXCLUDED.his_db_key,
			is_active  = EXCLUDED.is_active
	`, h.HCode, h.NameTH, h.Changwat, h.Amphur, h.HISDBKey, h.IsActive)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, h.HCode)
}

func (r *PgHospitalRepo) Delete(ctx context.Context, hcode string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM m_hospital WHERE hcode = $1`, hcode)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Assertion
var _ HospitalRepo = (*PgHospitalRepo)(nil)
