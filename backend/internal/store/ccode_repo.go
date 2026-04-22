package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// CCodeRow = row ใน c_code_log (per-record error reported by FDH/CHI via REP).
type CCodeRow struct {
	ID         string       `db:"id"          json:"id"`
	BatchID    string       `db:"batch_id"    json:"batch_id"`
	RecordID   string       `db:"record_id"   json:"record_id,omitempty"`
	HN         string       `db:"hn"          json:"hn,omitempty"`
	ANOrSEQ    string       `db:"an_or_seq"   json:"an_or_seq,omitempty"`
	CCode      string       `db:"c_code"      json:"c_code"`
	CDesc      string       `db:"c_desc"      json:"c_desc,omitempty"`
	FieldName  string       `db:"field_name"  json:"field_name,omitempty"`
	FieldValue string       `db:"field_value" json:"field_value,omitempty"`
	Resolved   bool         `db:"resolved"    json:"resolved"`
	ResolvedBy string       `db:"resolved_by" json:"resolved_by,omitempty"`
	ResolvedAt sql.NullTime `db:"resolved_at" json:"-"`
	ReceivedAt time.Time    `db:"received_at" json:"received_at"`
	// JSON mirror of ResolvedAt so frontend can read it directly.
	ResolvedAtJSON *time.Time `db:"-" json:"resolved_at,omitempty"`
}

// CCodeInsert payload for Insert/Upsert.
type CCodeInsert struct {
	BatchID    string
	RecordID   string // optional
	HN         string
	ANOrSEQ    string
	CCode      string
	CDesc      string
	FieldName  string
	FieldValue string
}

// CCodeFilter for List.
type CCodeFilter struct {
	BatchID  string
	HCode    string // join claim_batch
	Period   string // join claim_batch
	Resolved *bool  // nil = all, true/false filter
	CCode    string // exact match (e.g. "C104")
	Limit    int    // 0 = default 500
}

type CCodeRepo interface {
	Insert(ctx context.Context, row CCodeInsert) (string, error)
	List(ctx context.Context, f CCodeFilter) ([]CCodeRow, error)
	Resolve(ctx context.Context, id, by string) error
	// LookupClaimRecord ช่วย REP ingest: หา batch_id + record_id ที่ match กับ
	// (hcode, period, hn, an/seq). ถ้าไม่เจอคืน "" ทั้งคู่ (ไม่ error).
	LookupClaimRecord(ctx context.Context, hcode, period, hn, an, seq string) (batchID, recordID string)
}

type PgCCodeRepo struct{ db *sqlx.DB }

func NewPgCCodeRepo(db *sqlx.DB) *PgCCodeRepo { return &PgCCodeRepo{db: db} }

func (r *PgCCodeRepo) Insert(ctx context.Context, row CCodeInsert) (string, error) {
	var id string
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO c_code_log
		  (batch_id, record_id, hn, an_or_seq, c_code, c_desc, field_name, field_value, received_at)
		VALUES
		  ($1, NULLIF($2,'')::uuid, NULLIF($3,''), NULLIF($4,''), $5, NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), now())
		RETURNING id
	`,
		row.BatchID, row.RecordID, row.HN, row.ANOrSEQ,
		row.CCode, row.CDesc, row.FieldName, row.FieldValue,
	).Scan(&id)
	return id, err
}

func (r *PgCCodeRepo) List(ctx context.Context, f CCodeFilter) ([]CCodeRow, error) {
	conds := []string{}
	args := []interface{}{}
	add := func(cond string, val interface{}) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.BatchID != "" {
		add("c.batch_id = $%d::uuid", f.BatchID)
	}
	if f.HCode != "" {
		add("cb.hcode = $%d", f.HCode)
	}
	if f.Period != "" {
		add("cb.period = $%d", f.Period)
	}
	if f.Resolved != nil {
		add("c.resolved = $%d", *f.Resolved)
	}
	if f.CCode != "" {
		add("c.c_code = $%d", f.CCode)
	}
	q := `
		SELECT c.id::text AS id, c.batch_id::text AS batch_id,
		       COALESCE(c.record_id::text,'') AS record_id,
		       COALESCE(c.hn,'') AS hn, COALESCE(c.an_or_seq,'') AS an_or_seq,
		       c.c_code, COALESCE(c.c_desc,'') AS c_desc,
		       COALESCE(c.field_name,'') AS field_name,
		       COALESCE(c.field_value,'') AS field_value,
		       c.resolved, COALESCE(c.resolved_by,'') AS resolved_by,
		       c.resolved_at, c.received_at
		FROM c_code_log c
		JOIN claim_batch cb ON cb.batch_id = c.batch_id
	`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY c.received_at DESC"
	lim := f.Limit
	if lim <= 0 {
		lim = 500
	}
	q += fmt.Sprintf(" LIMIT %d", lim)

	var rows []CCodeRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ResolvedAt.Valid {
			t := rows[i].ResolvedAt.Time
			rows[i].ResolvedAtJSON = &t
		}
	}
	return rows, nil
}

func (r *PgCCodeRepo) Resolve(ctx context.Context, id, by string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE c_code_log
		SET resolved = true, resolved_by = NULLIF($1,''), resolved_at = now()
		WHERE id = $2::uuid AND resolved = false
	`, by, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PgCCodeRepo) LookupClaimRecord(ctx context.Context, hcode, period, hn, an, seq string) (string, string) {
	// Try exact match on (hcode, period, hn, an or seq)
	var batchID, recordID string
	// AN wins if present (IPD), else SEQ (OPD).
	var key, col string
	switch {
	case an != "":
		key, col = an, "an"
	case seq != "":
		key, col = seq, "seq"
	default:
		return "", ""
	}
	q := fmt.Sprintf(`
		SELECT cr.batch_id::text, cr.record_id::text
		FROM claim_record cr
		JOIN claim_batch cb ON cb.batch_id = cr.batch_id
		WHERE cb.hcode = $1 AND cb.period = $2
		  AND cr.hn = $3 AND cr.%s = $4
		ORDER BY cr.created_at DESC
		LIMIT 1
	`, col)
	_ = r.db.QueryRowxContext(ctx, q, hcode, period, hn, key).Scan(&batchID, &recordID)
	return batchID, recordID
}

var _ CCodeRepo = (*PgCCodeRepo)(nil)
