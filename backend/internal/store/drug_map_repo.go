package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// DrugMap = row ใน his_drug_map (per-รพ. mapping: HIS drug code → TMT 24).
// Natural key (hcode, his_drug_code) is UNIQUE — we upsert on it;
// id (UUID) is an internal surrogate used when linking C-code errors.
type DrugMap struct {
	ID          string `db:"id"            json:"id,omitempty"`
	HCode       string `db:"hcode"         json:"hcode"`
	HISDrugCode string `db:"his_drug_code" json:"his_drug_code"`
	TMTCode     string `db:"tmt_code"      json:"tmt_code,omitempty"`
	HISDrugName string `db:"his_drug_name" json:"his_drug_name,omitempty"`
	Note        string `db:"note"          json:"note,omitempty"`
	IsActive    bool   `db:"is_active"     json:"is_active"`
}

type DrugMapRepo interface {
	List(ctx context.Context, hcode string) ([]DrugMap, error)
	Get(ctx context.Context, hcode, hisDrugCode string) (*DrugMap, error)
	Upsert(ctx context.Context, m DrugMap) (*DrugMap, error)
	Delete(ctx context.Context, hcode, hisDrugCode string) error
}

type PgDrugMapRepo struct{ db *sqlx.DB }

func NewPgDrugMapRepo(db *sqlx.DB) *PgDrugMapRepo { return &PgDrugMapRepo{db: db} }

func (r *PgDrugMapRepo) List(ctx context.Context, hcode string) ([]DrugMap, error) {
	var out []DrugMap
	q := `
		SELECT id::text AS id, hcode, his_drug_code,
		       COALESCE(tmt_code,'')      AS tmt_code,
		       COALESCE(his_drug_name,'') AS his_drug_name,
		       COALESCE(note,'')          AS note,
		       is_active
		FROM his_drug_map
	`
	args := []interface{}{}
	if hcode != "" {
		q += ` WHERE hcode = $1`
		args = append(args, hcode)
	}
	q += ` ORDER BY hcode, his_drug_code`
	err := r.db.SelectContext(ctx, &out, q, args...)
	return out, err
}

func (r *PgDrugMapRepo) Get(ctx context.Context, hcode, hisDrugCode string) (*DrugMap, error) {
	var m DrugMap
	err := r.db.GetContext(ctx, &m, `
		SELECT id::text AS id, hcode, his_drug_code,
		       COALESCE(tmt_code,'')      AS tmt_code,
		       COALESCE(his_drug_name,'') AS his_drug_name,
		       COALESCE(note,'')          AS note,
		       is_active
		FROM his_drug_map WHERE hcode = $1 AND his_drug_code = $2
	`, hcode, hisDrugCode)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PgDrugMapRepo) Upsert(ctx context.Context, m DrugMap) (*DrugMap, error) {
	if m.HCode == "" || len(m.HCode) != 5 {
		return nil, fmt.Errorf("hcode must be exactly 5 chars")
	}
	if m.HISDrugCode == "" {
		return nil, fmt.Errorf("his_drug_code required")
	}
	if m.TMTCode != "" && len(m.TMTCode) != 24 {
		return nil, fmt.Errorf("tmt_code must be 24 chars when provided")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO his_drug_map (hcode, his_drug_code, tmt_code, his_drug_name, note, is_active)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6)
		ON CONFLICT (hcode, his_drug_code) DO UPDATE SET
			tmt_code      = EXCLUDED.tmt_code,
			his_drug_name = EXCLUDED.his_drug_name,
			note          = EXCLUDED.note,
			is_active     = EXCLUDED.is_active
	`, m.HCode, m.HISDrugCode, m.TMTCode, m.HISDrugName, m.Note, m.IsActive)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, m.HCode, m.HISDrugCode)
}

func (r *PgDrugMapRepo) Delete(ctx context.Context, hcode, hisDrugCode string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM his_drug_map WHERE hcode = $1 AND his_drug_code = $2
	`, hcode, hisDrugCode)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

var _ DrugMapRepo = (*PgDrugMapRepo)(nil)
