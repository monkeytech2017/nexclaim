package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// ICD10Row is a row from m_icd10.
type ICD10Row struct {
	Code    string `db:"code"    json:"code"`
	NameTH  string `db:"name_th" json:"name_th"`
	NameEN  string `db:"name_en" json:"name_en"`
	Chapter string `db:"chapter" json:"chapter"`
}

// ICD9CMRow is a row from m_icd9cm.
type ICD9CMRow struct {
	Code   string `db:"code"    json:"code"`
	NameTH string `db:"name_th" json:"name_th"`
	NameEN string `db:"name_en" json:"name_en"`
}

// TMTRow is a row from m_tmt_drug.
type TMTRow struct {
	TMTCode     string `db:"tmt_code"    json:"tmt_code"`
	NameTH      string `db:"name_th"     json:"name_th"`
	GenericName string `db:"generic_name" json:"generic_name"`
	Strength    string `db:"strength"    json:"strength"`
	DosageForm  string `db:"dosage_form" json:"dosage_form"`
	Unit        string `db:"unit"        json:"unit"`
}

// MasterDataRepo provides read-only search over the master code tables.
type MasterDataRepo interface {
	ListICD10(ctx context.Context, q string, limit int) ([]ICD10Row, error)
	ListICD9CM(ctx context.Context, q string, limit int) ([]ICD9CMRow, error)
	ListTMT(ctx context.Context, q string, limit int) ([]TMTRow, error)
}

// clampLimit enforces 1 ≤ limit ≤ 200, defaulting to 50.
func clampLimit(n int) int {
	if n <= 0 {
		return 50
	}
	if n > 200 {
		return 200
	}
	return n
}

// PgMasterDataRepo is the Postgres-backed implementation of MasterDataRepo.
type PgMasterDataRepo struct{ db *sqlx.DB }

// NewPgMasterDataRepo returns a Postgres-backed MasterDataRepo.
func NewPgMasterDataRepo(db *sqlx.DB) *PgMasterDataRepo {
	return &PgMasterDataRepo{db: db}
}

// Compile-time assertion.
var _ MasterDataRepo = (*PgMasterDataRepo)(nil)

func (r *PgMasterDataRepo) ListICD10(ctx context.Context, q string, limit int) ([]ICD10Row, error) {
	out := []ICD10Row{}
	err := r.db.SelectContext(ctx, &out, `
		SELECT code,
		       COALESCE(name_th,'') AS name_th,
		       COALESCE(name_en,'') AS name_en,
		       COALESCE(chapter,'') AS chapter
		FROM m_icd10
		WHERE is_active
		  AND ($1 = ''
		       OR code          ILIKE '%' || $1 || '%'
		       OR COALESCE(name_th,'') ILIKE '%' || $1 || '%'
		       OR COALESCE(name_en,'') ILIKE '%' || $1 || '%')
		ORDER BY code
		LIMIT $2
	`, q, clampLimit(limit))
	return out, err
}

func (r *PgMasterDataRepo) ListICD9CM(ctx context.Context, q string, limit int) ([]ICD9CMRow, error) {
	out := []ICD9CMRow{}
	err := r.db.SelectContext(ctx, &out, `
		SELECT code,
		       COALESCE(name_th,'') AS name_th,
		       COALESCE(name_en,'') AS name_en
		FROM m_icd9cm
		WHERE is_active
		  AND ($1 = ''
		       OR code          ILIKE '%' || $1 || '%'
		       OR COALESCE(name_th,'') ILIKE '%' || $1 || '%'
		       OR COALESCE(name_en,'') ILIKE '%' || $1 || '%')
		ORDER BY code
		LIMIT $2
	`, q, clampLimit(limit))
	return out, err
}

func (r *PgMasterDataRepo) ListTMT(ctx context.Context, q string, limit int) ([]TMTRow, error) {
	out := []TMTRow{}
	err := r.db.SelectContext(ctx, &out, `
		SELECT tmt_code,
		       COALESCE(name_th,'')      AS name_th,
		       COALESCE(generic_name,'') AS generic_name,
		       COALESCE(strength,'')     AS strength,
		       COALESCE(dosage_form,'')  AS dosage_form,
		       COALESCE(unit,'')         AS unit
		FROM m_tmt_drug
		WHERE is_active
		  AND ($1 = ''
		       OR tmt_code                    ILIKE '%' || $1 || '%'
		       OR COALESCE(name_th,'')        ILIKE '%' || $1 || '%'
		       OR COALESCE(generic_name,'')   ILIKE '%' || $1 || '%')
		ORDER BY tmt_code
		LIMIT $2
	`, q, clampLimit(limit))
	return out, err
}
