package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// FieldMap = row ใน his_field_map.
//
// จับคู่ HIS column (his_table.his_column) → target (target_file.target_field)
// + transform function + required/default. Natural key 5 คอลัมน์; id (UUID)
// ใช้เป็น internal surrogate สำหรับ DELETE (URL ไม่พก 5 ส่วน).
//
// Typical real-hospital set = 100–200 mappings. Bulk import จำเป็น.
type FieldMap struct {
	ID           string `db:"id"            json:"id,omitempty"`
	HCode        string `db:"hcode"         json:"hcode"`
	HISTable     string `db:"his_table"     json:"his_table"`
	HISColumn    string `db:"his_column"    json:"his_column"`
	TargetFile   string `db:"target_file"   json:"target_file"`
	TargetField  string `db:"target_field"  json:"target_field"`
	Transform    string `db:"transform"     json:"transform,omitempty"`
	IsRequired   bool   `db:"is_required"   json:"is_required"`
	DefaultValue string `db:"default_value" json:"default_value,omitempty"`
	Note         string `db:"note"          json:"note,omitempty"`
}

// FieldMapFilter for List — all fields optional, AND-combined.
type FieldMapFilter struct {
	HCode      string
	HISTable   string
	TargetFile string
}

type FieldMapRepo interface {
	List(ctx context.Context, f FieldMapFilter) ([]FieldMap, error)
	Get(ctx context.Context, id string) (*FieldMap, error)
	Upsert(ctx context.Context, m FieldMap) (*FieldMap, error)
	Delete(ctx context.Context, id string) error
}

type PgFieldMapRepo struct{ db *sqlx.DB }

func NewPgFieldMapRepo(db *sqlx.DB) *PgFieldMapRepo { return &PgFieldMapRepo{db: db} }

func (r *PgFieldMapRepo) List(ctx context.Context, f FieldMapFilter) ([]FieldMap, error) {
	conds := []string{}
	args := []interface{}{}
	add := func(cond string, v interface{}) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.HCode != "" {
		add("hcode = $%d", f.HCode)
	}
	if f.HISTable != "" {
		add("his_table = $%d", f.HISTable)
	}
	if f.TargetFile != "" {
		add("target_file = $%d", f.TargetFile)
	}
	q := `
		SELECT id::text AS id, hcode, his_table, his_column, target_file, target_field,
		       COALESCE(transform,'')     AS transform,
		       is_required,
		       COALESCE(default_value,'') AS default_value,
		       COALESCE(note,'')          AS note
		FROM his_field_map
	`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY hcode, target_file, target_field"

	var out []FieldMap
	err := r.db.SelectContext(ctx, &out, q, args...)
	return out, err
}

func (r *PgFieldMapRepo) Get(ctx context.Context, id string) (*FieldMap, error) {
	var m FieldMap
	err := r.db.GetContext(ctx, &m, `
		SELECT id::text AS id, hcode, his_table, his_column, target_file, target_field,
		       COALESCE(transform,'')     AS transform,
		       is_required,
		       COALESCE(default_value,'') AS default_value,
		       COALESCE(note,'')          AS note
		FROM his_field_map WHERE id = $1::uuid
	`, id)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PgFieldMapRepo) Upsert(ctx context.Context, m FieldMap) (*FieldMap, error) {
	if m.HCode == "" || len(m.HCode) != 5 {
		return nil, fmt.Errorf("hcode must be exactly 5 chars")
	}
	if m.HISTable == "" || m.HISColumn == "" {
		return nil, fmt.Errorf("his_table + his_column required")
	}
	if m.TargetFile == "" || m.TargetField == "" {
		return nil, fmt.Errorf("target_file + target_field required")
	}
	transform := m.Transform
	if transform == "" {
		transform = "none"
	}

	var id string
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO his_field_map
		  (hcode, his_table, his_column, target_file, target_field,
		   transform, is_required, default_value, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,''), NULLIF($9,''))
		ON CONFLICT (hcode, his_table, his_column, target_file, target_field) DO UPDATE SET
			transform     = EXCLUDED.transform,
			is_required   = EXCLUDED.is_required,
			default_value = EXCLUDED.default_value,
			note          = EXCLUDED.note
		RETURNING id::text
	`, m.HCode, m.HISTable, m.HISColumn, m.TargetFile, m.TargetField,
		transform, m.IsRequired, m.DefaultValue, m.Note,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *PgFieldMapRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM his_field_map WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

var _ FieldMapRepo = (*PgFieldMapRepo)(nil)
