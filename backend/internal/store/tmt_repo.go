package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// TmtDrug = read-only row จาก m_tmt_drug (TMT drug master reference).
// tmt_code เป็น fixed-width char ในบาง row → SELECT TRIM ให้คืน code สะอาด.
type TmtDrug struct {
	TmtCode     string `db:"tmt_code"     json:"tmt_code"`
	NameTh      string `db:"name_th"      json:"name_th"`
	GenericName string `db:"generic_name" json:"generic_name,omitempty"`
	Strength    string `db:"strength"     json:"strength,omitempty"`
	DosageForm  string `db:"dosage_form"  json:"dosage_form,omitempty"`
	Unit        string `db:"unit"         json:"unit,omitempty"`
}

// TmtFilter — Q optional (ILIKE %Q% on code/name_th/generic_name).
// Limit clamped to [1,200] (default 50); Offset default 0.
type TmtFilter struct {
	Q      string
	Limit  int
	Offset int
}

// clampedLimit returns Limit bounded to [1,200], defaulting to 50.
func (f TmtFilter) clampedLimit() int {
	lim := f.Limit
	if lim <= 0 {
		return 50
	}
	if lim > 200 {
		return 200
	}
	return lim
}

type TmtRepo interface {
	// Search returns a page of active rows matching the filter plus the total
	// count matching the same filter (for pagination).
	Search(ctx context.Context, f TmtFilter) (items []TmtDrug, total int, err error)
}

type PgTmtRepo struct{ db *sqlx.DB }

func NewPgTmtRepo(db *sqlx.DB) *PgTmtRepo { return &PgTmtRepo{db: db} }

const tmtBaseSelect = `
	SELECT
		TRIM(tmt_code)                AS tmt_code,
		COALESCE(name_th,'')          AS name_th,
		COALESCE(generic_name,'')     AS generic_name,
		COALESCE(strength,'')         AS strength,
		COALESCE(dosage_form,'')      AS dosage_form,
		COALESCE(unit,'')             AS unit
	FROM m_tmt_drug
`

func (r *PgTmtRepo) Search(ctx context.Context, f TmtFilter) ([]TmtDrug, int, error) {
	conds := []string{"is_active = true"}
	args := []interface{}{}
	if f.Q != "" {
		args = append(args, "%"+f.Q+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf(
			"(TRIM(tmt_code) ILIKE $%d OR name_th ILIKE $%d OR generic_name ILIKE $%d)",
			n, n, n))
	}
	where := " WHERE " + joinAnd(conds)

	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM m_tmt_drug`+where, args...); err != nil {
		return nil, 0, fmt.Errorf("tmt count: %w", err)
	}

	lim := f.clampedLimit()
	off := f.Offset
	if off < 0 {
		off = 0
	}
	args = append(args, lim, off)
	q := tmtBaseSelect + where +
		fmt.Sprintf(" ORDER BY tmt_code LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	items := []TmtDrug{}
	if err := r.db.SelectContext(ctx, &items, q, args...); err != nil {
		return nil, 0, fmt.Errorf("tmt search: %w", err)
	}
	return items, total, nil
}

// joinAnd (shared helper in icd_map_repo.go) joins conditions with " AND ".

var _ TmtRepo = (*PgTmtRepo)(nil)
