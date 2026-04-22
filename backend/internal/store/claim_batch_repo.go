package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// ClaimBatchRow = denormalized row จาก claim_batch (+ optional c-code count).
// อ่าน-อย่างเดียว สำหรับ submission history UI.
type ClaimBatchRow struct {
	BatchID       string       `db:"batch_id"       json:"batch_id"`
	HCode         string       `db:"hcode"          json:"hcode"`
	Period        string       `db:"period"         json:"period"`
	INSCL         string       `db:"inscl"          json:"inscl"`
	Format        string       `db:"format"         json:"format"`
	Sender        string       `db:"sender"         json:"sender"`
	Status        string       `db:"status"         json:"status"`
	TotalRecords  int          `db:"total_records"  json:"total_records"`
	ValidRecords  int          `db:"valid_records"  json:"valid_records"`
	ErrorRecords  int          `db:"error_records"  json:"error_records"`
	FDHTxnID      string       `db:"fdh_txn_id"     json:"fdh_txn_id,omitempty"`
	ZipFilename   string       `db:"zip_filename"   json:"zip_filename,omitempty"`
	ZipMD5        string       `db:"zip_md5"        json:"zip_md5,omitempty"`
	CreatedAt     time.Time    `db:"created_at"     json:"created_at"`
	SentAt        sql.NullTime `db:"sent_at"        json:"-"`
	SentAtJSON    *time.Time   `db:"-"              json:"sent_at,omitempty"`
	ErrorMsg      string       `db:"error_msg"      json:"error_msg,omitempty"`
	CCodeCount    int          `db:"c_code_count"   json:"c_code_count"` // 0 until REP ingested
	CCodeOpen     int          `db:"c_code_open"    json:"c_code_open"`  // unresolved only
}

// ClaimBatchFilter — all fields optional, AND-combined.
type ClaimBatchFilter struct {
	HCode  string
	Period string
	INSCL  string
	Format string
	Status string
	Limit  int // 0 = default 200
}

type ClaimBatchRepo interface {
	List(ctx context.Context, f ClaimBatchFilter) ([]ClaimBatchRow, error)
	Get(ctx context.Context, batchID string) (*ClaimBatchRow, error)
}

type PgClaimBatchRepo struct{ db *sqlx.DB }

func NewPgClaimBatchRepo(db *sqlx.DB) *PgClaimBatchRepo { return &PgClaimBatchRepo{db: db} }

// baseSelect joins claim_batch with c_code_log counts in one round trip.
const claimBatchBaseSelect = `
	SELECT
		cb.batch_id::text       AS batch_id,
		cb.hcode, cb.period, cb.inscl, cb.format, cb.sender, cb.status,
		cb.total_records, cb.valid_records, cb.error_records,
		COALESCE(cb.fdh_txn_id,'')   AS fdh_txn_id,
		COALESCE(cb.zip_filename,'') AS zip_filename,
		COALESCE(cb.zip_md5,'')      AS zip_md5,
		cb.created_at, cb.sent_at,
		COALESCE(cb.error_msg,'')    AS error_msg,
		COALESCE(cc.total, 0)        AS c_code_count,
		COALESCE(cc.open,  0)        AS c_code_open
	FROM claim_batch cb
	LEFT JOIN (
		SELECT batch_id,
		       COUNT(*)                              AS total,
		       COUNT(*) FILTER (WHERE NOT resolved)  AS open
		FROM c_code_log
		GROUP BY batch_id
	) cc ON cc.batch_id = cb.batch_id
`

func (r *PgClaimBatchRepo) List(ctx context.Context, f ClaimBatchFilter) ([]ClaimBatchRow, error) {
	conds := []string{}
	args := []interface{}{}
	add := func(cond string, v interface{}) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.HCode != "" {
		add("cb.hcode = $%d", f.HCode)
	}
	if f.Period != "" {
		add("cb.period = $%d", f.Period)
	}
	if f.INSCL != "" {
		add("cb.inscl = $%d", f.INSCL)
	}
	if f.Format != "" {
		add("cb.format = $%d", f.Format)
	}
	if f.Status != "" {
		add("cb.status = $%d", f.Status)
	}
	q := claimBatchBaseSelect
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY cb.created_at DESC"
	lim := f.Limit
	if lim <= 0 {
		lim = 200
	}
	q += fmt.Sprintf(" LIMIT %d", lim)

	var rows []ClaimBatchRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].SentAt.Valid {
			t := rows[i].SentAt.Time
			rows[i].SentAtJSON = &t
		}
	}
	return rows, nil
}

func (r *PgClaimBatchRepo) Get(ctx context.Context, batchID string) (*ClaimBatchRow, error) {
	var row ClaimBatchRow
	err := r.db.GetContext(ctx, &row, claimBatchBaseSelect+` WHERE cb.batch_id = $1::uuid`, batchID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.SentAt.Valid {
		t := row.SentAt.Time
		row.SentAtJSON = &t
	}
	return &row, nil
}

var _ ClaimBatchRepo = (*PgClaimBatchRepo)(nil)
