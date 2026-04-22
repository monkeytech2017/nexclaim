package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Doctor = row ใน m_doctor.
// license_no (เลขใบประกอบวิชาชีพ 6 หลัก) ใช้เป็น DRDX/DROPID ใน claim XML —
// บังคับ SSO/CHI. unique ต่อ (hcode, license_no).
type Doctor struct {
	DoctorID  string    `db:"doctor_id"  json:"doctor_id"`
	HCode     string    `db:"hcode"      json:"hcode"`
	LicenseNo string    `db:"license_no" json:"license_no"`
	NameTH    string    `db:"name_th"    json:"name_th,omitempty"`
	Specialty string    `db:"specialty"  json:"specialty,omitempty"`
	IsActive  bool      `db:"is_active"  json:"is_active"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// DoctorRepo CRUD สำหรับ m_doctor. Query list ฟิลเตอร์ by hcode เพราะ รพ.
// ไหนก็ดึงเฉพาะของตัวเอง (ยังไม่มี multi-tenancy middleware).
type DoctorRepo interface {
	List(ctx context.Context, hcode string) ([]Doctor, error)
	Get(ctx context.Context, doctorID string) (*Doctor, error)
	Upsert(ctx context.Context, d Doctor) (*Doctor, error)
	Delete(ctx context.Context, doctorID string) error
}

type PgDoctorRepo struct{ db *sqlx.DB }

func NewPgDoctorRepo(db *sqlx.DB) *PgDoctorRepo { return &PgDoctorRepo{db: db} }

func (r *PgDoctorRepo) List(ctx context.Context, hcode string) ([]Doctor, error) {
	var out []Doctor
	q := `
		SELECT doctor_id, hcode, license_no,
		       COALESCE(name_th,'') AS name_th,
		       COALESCE(specialty,'') AS specialty,
		       is_active, updated_at
		FROM m_doctor
	`
	args := []interface{}{}
	if hcode != "" {
		q += ` WHERE hcode = $1`
		args = append(args, hcode)
	}
	q += ` ORDER BY hcode, license_no`
	err := r.db.SelectContext(ctx, &out, q, args...)
	return out, err
}

func (r *PgDoctorRepo) Get(ctx context.Context, doctorID string) (*Doctor, error) {
	var d Doctor
	err := r.db.GetContext(ctx, &d, `
		SELECT doctor_id, hcode, license_no,
		       COALESCE(name_th,'') AS name_th,
		       COALESCE(specialty,'') AS specialty,
		       is_active, updated_at
		FROM m_doctor WHERE doctor_id = $1
	`, doctorID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *PgDoctorRepo) Upsert(ctx context.Context, d Doctor) (*Doctor, error) {
	if d.DoctorID == "" {
		return nil, fmt.Errorf("doctor_id required")
	}
	if d.HCode == "" || len(d.HCode) != 5 {
		return nil, fmt.Errorf("hcode must be exactly 5 chars")
	}
	if d.LicenseNo == "" {
		return nil, fmt.Errorf("license_no required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO m_doctor (doctor_id, hcode, license_no, name_th, specialty, is_active, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6, now())
		ON CONFLICT (doctor_id) DO UPDATE SET
			hcode      = EXCLUDED.hcode,
			license_no = EXCLUDED.license_no,
			name_th    = EXCLUDED.name_th,
			specialty  = EXCLUDED.specialty,
			is_active  = EXCLUDED.is_active,
			updated_at = now()
	`, d.DoctorID, d.HCode, d.LicenseNo, d.NameTH, d.Specialty, d.IsActive)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, d.DoctorID)
}

func (r *PgDoctorRepo) Delete(ctx context.Context, doctorID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM m_doctor WHERE doctor_id = $1`, doctorID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

var _ DoctorRepo = (*PgDoctorRepo)(nil)
