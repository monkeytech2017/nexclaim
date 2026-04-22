package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// IcdMap = row ใน his_icd_map. Per-รพ. translation ของ ICD code ที่ HIS ใช้
// (อาจเป็น code ภายใน) ไปเป็นรหัสมาตรฐาน (ICD-10 WHO หรือ ICD-9CM).
// Composite PK (hcode, his_icd_code, icd_type) — รหัสเดียวกันอาจถูก map
// ต่างกันระหว่าง '10' (ICD-10) และ '9C' (ICD-9CM).
type IcdMap struct {
	HCode      string `db:"hcode"        json:"hcode"`
	HISIcdCode string `db:"his_icd_code" json:"his_icd_code"`
	IcdType    string `db:"icd_type"     json:"icd_type"` // "10" | "9C"
	StdCode    string `db:"std_code"     json:"std_code"`
}

// IcdType constants — match CHECK constraint in migration 002.
const (
	IcdType10 = "10"
	IcdType9C = "9C"
)

type IcdMapRepo interface {
	List(ctx context.Context, hcode, icdType string) ([]IcdMap, error)
	Get(ctx context.Context, hcode, hisIcdCode, icdType string) (*IcdMap, error)
	Upsert(ctx context.Context, m IcdMap) (*IcdMap, error)
	Delete(ctx context.Context, hcode, hisIcdCode, icdType string) error
}

type PgIcdMapRepo struct{ db *sqlx.DB }

func NewPgIcdMapRepo(db *sqlx.DB) *PgIcdMapRepo { return &PgIcdMapRepo{db: db} }

func (r *PgIcdMapRepo) List(ctx context.Context, hcode, icdType string) ([]IcdMap, error) {
	var out []IcdMap
	q := `SELECT hcode, his_icd_code, icd_type, std_code FROM his_icd_map`
	args := []interface{}{}
	conds := []string{}
	if hcode != "" {
		args = append(args, hcode)
		conds = append(conds, fmt.Sprintf("hcode = $%d", len(args)))
	}
	if icdType != "" {
		args = append(args, icdType)
		conds = append(conds, fmt.Sprintf("icd_type = $%d", len(args)))
	}
	if len(conds) > 0 {
		q += " WHERE " + joinAnd(conds)
	}
	q += " ORDER BY hcode, icd_type, his_icd_code"
	err := r.db.SelectContext(ctx, &out, q, args...)
	return out, err
}

func (r *PgIcdMapRepo) Get(ctx context.Context, hcode, hisIcdCode, icdType string) (*IcdMap, error) {
	var m IcdMap
	err := r.db.GetContext(ctx, &m, `
		SELECT hcode, his_icd_code, icd_type, std_code
		FROM his_icd_map
		WHERE hcode = $1 AND his_icd_code = $2 AND icd_type = $3
	`, hcode, hisIcdCode, icdType)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PgIcdMapRepo) Upsert(ctx context.Context, m IcdMap) (*IcdMap, error) {
	if m.HCode == "" || len(m.HCode) != 5 {
		return nil, fmt.Errorf("hcode must be exactly 5 chars")
	}
	if m.HISIcdCode == "" {
		return nil, fmt.Errorf("his_icd_code required")
	}
	if m.IcdType != IcdType10 && m.IcdType != IcdType9C {
		return nil, fmt.Errorf("icd_type must be '10' or '9C'")
	}
	if m.StdCode == "" {
		return nil, fmt.Errorf("std_code required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO his_icd_map (hcode, his_icd_code, icd_type, std_code)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (hcode, his_icd_code, icd_type) DO UPDATE SET
			std_code = EXCLUDED.std_code
	`, m.HCode, m.HISIcdCode, m.IcdType, m.StdCode)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, m.HCode, m.HISIcdCode, m.IcdType)
}

func (r *PgIcdMapRepo) Delete(ctx context.Context, hcode, hisIcdCode, icdType string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM his_icd_map WHERE hcode = $1 AND his_icd_code = $2 AND icd_type = $3
	`, hcode, hisIcdCode, icdType)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func joinAnd(conds []string) string {
	out := ""
	for i, c := range conds {
		if i > 0 {
			out += " AND "
		}
		out += c
	}
	return out
}

var _ IcdMapRepo = (*PgIcdMapRepo)(nil)
