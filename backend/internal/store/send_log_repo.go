package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// SendLogRow = one row in send_log joined with its claim_batch for context.
// Read-only — inserts happen via PgClaimRepo.SaveRun in the same tx as the batch.
type SendLogRow struct {
	ID           string         `db:"id"             json:"id"`
	BatchID      string         `db:"batch_id"       json:"batch_id"`
	AttemptNo    int            `db:"attempt_no"     json:"attempt_no"`
	Endpoint     string         `db:"endpoint"       json:"endpoint"`
	HTTPStatus   sql.NullInt64  `db:"http_status"    json:"-"`
	HTTPStatusJS *int           `db:"-"              json:"http_status,omitempty"`
	FDHTxnID     sql.NullString `db:"fdh_txn_id"     json:"fdh_txn_id,omitempty"`
	ResponseBody sql.NullString `db:"response_body"  json:"response_body,omitempty"`
	DurationMs   sql.NullInt64  `db:"duration_ms"    json:"duration_ms,omitempty"`
	Success      sql.NullBool   `db:"success"        json:"-"`
	SuccessJS    *bool          `db:"-"              json:"success,omitempty"`
	ErrorMsg     sql.NullString `db:"error_msg"      json:"error_msg,omitempty"`
	SentAt       time.Time      `db:"sent_at"        json:"sent_at"`
	// Joined from claim_batch for convenience in UI.
	HCode  string `db:"hcode"  json:"hcode"`
	Period string `db:"period" json:"period"`
	INSCL  string `db:"inscl"  json:"inscl"`
	Format string `db:"format" json:"format"`
}

// SendLogFilter — all fields optional, AND-combined.
// Success is a string ("", "true", "false") so handlers can distinguish
// "filter not set" from "filter set to false".
type SendLogFilter struct {
	BatchID string
	HCode   string
	Period  string
	Success string
	Limit   int
}

type SendLogRepo interface {
	List(ctx context.Context, f SendLogFilter) ([]SendLogRow, error)
}

type PgSendLogRepo struct{ db *sqlx.DB }

func NewPgSendLogRepo(db *sqlx.DB) *PgSendLogRepo { return &PgSendLogRepo{db: db} }

const sendLogBaseSelect = `
	SELECT
		sl.id::text            AS id,
		sl.batch_id::text      AS batch_id,
		sl.attempt_no, sl.endpoint,
		sl.http_status, sl.fdh_txn_id, sl.response_body,
		sl.duration_ms, sl.success, sl.error_msg, sl.sent_at,
		cb.hcode, cb.period, cb.inscl, cb.format
	FROM send_log sl
	JOIN claim_batch cb ON cb.batch_id = sl.batch_id
`

func (r *PgSendLogRepo) List(ctx context.Context, f SendLogFilter) ([]SendLogRow, error) {
	conds := []string{}
	args := []interface{}{}
	add := func(cond string, v interface{}) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.BatchID != "" {
		add("sl.batch_id = $%d::uuid", f.BatchID)
	}
	if f.HCode != "" {
		add("cb.hcode = $%d", f.HCode)
	}
	if f.Period != "" {
		add("cb.period = $%d", f.Period)
	}
	switch f.Success {
	case "true":
		conds = append(conds, "sl.success = true")
	case "false":
		conds = append(conds, "sl.success = false")
	}
	q := sendLogBaseSelect
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY sl.sent_at DESC"
	lim := f.Limit
	if lim <= 0 {
		lim = 200
	}
	q += fmt.Sprintf(" LIMIT %d", lim)

	var rows []SendLogRow
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].HTTPStatus.Valid {
			v := int(rows[i].HTTPStatus.Int64)
			rows[i].HTTPStatusJS = &v
		}
		if rows[i].Success.Valid {
			v := rows[i].Success.Bool
			rows[i].SuccessJS = &v
		}
	}
	return rows, nil
}

var _ SendLogRepo = (*PgSendLogRepo)(nil)
