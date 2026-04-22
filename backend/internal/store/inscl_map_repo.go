package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// InsclMap = row ใน his_inscl_map (per-รพ. mapping of HIS pttype → INSCL
// มาตรฐาน + Agency สำหรับ OFC). Composite PK = (hcode, his_pttype).
type InsclMap struct {
	HCode      string `db:"hcode"       json:"hcode"`
	HISPttype  string `db:"his_pttype"  json:"his_pttype"`
	INSCL      string `db:"inscl"       json:"inscl"`
	AgencyCode string `db:"agency_code" json:"agency_code,omitempty"`
	Note       string `db:"note"        json:"note,omitempty"`
}

// InsclMapRepo: CRUD ต่อ (hcode, his_pttype). List รับ hcode filter.
type InsclMapRepo interface {
	List(ctx context.Context, hcode string) ([]InsclMap, error)
	Get(ctx context.Context, hcode, hisPttype string) (*InsclMap, error)
	Upsert(ctx context.Context, m InsclMap) (*InsclMap, error)
	Delete(ctx context.Context, hcode, hisPttype string) error
}

type PgInsclMapRepo struct{ db *sqlx.DB }

func NewPgInsclMapRepo(db *sqlx.DB) *PgInsclMapRepo { return &PgInsclMapRepo{db: db} }

func (r *PgInsclMapRepo) List(ctx context.Context, hcode string) ([]InsclMap, error) {
	var out []InsclMap
	q := `
		SELECT hcode, his_pttype, inscl,
		       COALESCE(agency_code,'') AS agency_code,
		       COALESCE(note,'') AS note
		FROM his_inscl_map
	`
	args := []interface{}{}
	if hcode != "" {
		q += ` WHERE hcode = $1`
		args = append(args, hcode)
	}
	q += ` ORDER BY hcode, his_pttype`
	err := r.db.SelectContext(ctx, &out, q, args...)
	return out, err
}

func (r *PgInsclMapRepo) Get(ctx context.Context, hcode, hisPttype string) (*InsclMap, error) {
	var m InsclMap
	err := r.db.GetContext(ctx, &m, `
		SELECT hcode, his_pttype, inscl,
		       COALESCE(agency_code,'') AS agency_code,
		       COALESCE(note,'') AS note
		FROM his_inscl_map WHERE hcode = $1 AND his_pttype = $2
	`, hcode, hisPttype)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PgInsclMapRepo) Upsert(ctx context.Context, m InsclMap) (*InsclMap, error) {
	if m.HCode == "" || len(m.HCode) != 5 {
		return nil, fmt.Errorf("hcode must be exactly 5 chars")
	}
	if m.HISPttype == "" {
		return nil, fmt.Errorf("his_pttype required")
	}
	if m.INSCL == "" {
		return nil, fmt.Errorf("inscl required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO his_inscl_map (hcode, his_pttype, inscl, agency_code, note)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''))
		ON CONFLICT (hcode, his_pttype) DO UPDATE SET
			inscl       = EXCLUDED.inscl,
			agency_code = EXCLUDED.agency_code,
			note        = EXCLUDED.note
	`, m.HCode, m.HISPttype, m.INSCL, m.AgencyCode, m.Note)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, m.HCode, m.HISPttype)
}

func (r *PgInsclMapRepo) Delete(ctx context.Context, hcode, hisPttype string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM his_inscl_map WHERE hcode = $1 AND his_pttype = $2
	`, hcode, hisPttype)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

var _ InsclMapRepo = (*PgInsclMapRepo)(nil)
