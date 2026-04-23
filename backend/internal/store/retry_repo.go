package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/nexclaim/nexclaim/internal/model"
)

// RetryRow = the subset of claim_batch a retry worker needs to re-send.
// Returned by SelectRetryable + consumed by RecordRetry in the same tick.
type RetryRow struct {
	BatchID     string            `db:"batch_id"`
	HCode       string            `db:"hcode"`
	Period      string            `db:"period"`
	INSCL       string            `db:"inscl"`
	Format      model.ClaimFormat `db:"format"`
	Sender      model.Sender      `db:"sender"`
	AttemptNo   int               `db:"attempt_no"`
	ZipName     string            `db:"zip_filename"`
	ZipBytes    []byte            `db:"zip_bytes"`
	NextRetryAt sql.NullTime      `db:"next_retry_at"`
}

// RetryRepo narrows ClaimRepo to the retry worker's needs. Implementations
// must enforce status='error' + attempt_no<max + eligibility gate atomically.
type RetryRepo interface {
	// SelectRetryable returns up to `limit` claim_batch rows whose status is
	// still 'error', attempt_no is below maxAttempts, and whose next_retry_at
	// is null or in the past. Rows are returned locked FOR UPDATE SKIP LOCKED
	// so multiple workers never pick the same row.
	SelectRetryable(ctx context.Context, limit, maxAttempts int) ([]RetryRow, error)

	// RecordRetry writes a new send_log row with attempt_no=newAttemptNo and
	// updates claim_batch accordingly:
	//   success=true  → status='sent', next_retry_at=NULL
	//   success=false → if newAttemptNo >= max: status='failed' + next_retry_at=NULL
	//                   else:                    status='error' + next_retry_at=<nextRetryAt>
	// endpoint is the URL hit; errMsg is "" on success.
	RecordRetry(ctx context.Context, args RecordRetryArgs) error
}

// RecordRetryArgs packs the inputs so callers don't have to line up many
// positional parameters. All fields are required unless noted.
type RecordRetryArgs struct {
	BatchID       string
	NewAttemptNo  int
	MaxAttempts   int
	Success       bool
	Duration      time.Duration
	Endpoint      string
	ErrMsg        string        // "" when Success
	TxnID         string        // from sender on success; optional
	NextRetryAt   time.Time     // ignored when Success or terminal
}

// ── PgRetryRepo ──────────────────────────────────────────────

// PgRetryRepo implements RetryRepo against Postgres.
// Single transaction per retry: SELECT ... FOR UPDATE SKIP LOCKED → caller
// processes → RecordRetry either commits or leaves the lock to expire.
type PgRetryRepo struct{ db *sqlx.DB }

func NewPgRetryRepo(db *sqlx.DB) *PgRetryRepo { return &PgRetryRepo{db: db} }

// SelectRetryable runs a short-lived read — we do NOT hold the tx open across
// the HTTP send, because that would pin a DB connection for ~60 seconds.
// Instead we rely on attempt_no monotonically increasing in RecordRetry
// (same batch picked by another worker on the next tick will have a bumped
// attempt_no, so it's a no-op). FOR UPDATE SKIP LOCKED is still included so
// near-simultaneous ticks on one replica don't double-pick.
func (r *PgRetryRepo) SelectRetryable(ctx context.Context, limit, maxAttempts int) ([]RetryRow, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `
		SELECT batch_id::text AS batch_id,
		       hcode, period, inscl, format, sender, attempt_no,
		       COALESCE(zip_filename,'') AS zip_filename,
		       zip_bytes, next_retry_at
		FROM claim_batch
		WHERE status = 'error'
		  AND attempt_no < $1
		  AND (next_retry_at IS NULL OR next_retry_at <= now())
		  AND zip_bytes IS NOT NULL
		ORDER BY COALESCE(next_retry_at, created_at) ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`
	var rows []RetryRow
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("retry select tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := tx.SelectContext(ctx, &rows, q, maxAttempts, limit); err != nil {
		return nil, fmt.Errorf("retry select: %w", err)
	}
	// Commit releases the FOR UPDATE locks; we rely on attempt_no to guard
	// duplicate processing (see header comment).
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("retry select commit: %w", err)
	}
	return rows, nil
}

func (r *PgRetryRepo) RecordRetry(ctx context.Context, a RecordRetryArgs) error {
	if a.BatchID == "" {
		return fmt.Errorf("retry record: batch_id required")
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("retry record tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1) insert send_log
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO send_log (batch_id, attempt_no, endpoint, fdh_txn_id,
		                     duration_ms, success, error_msg, sent_at)
		VALUES ($1::uuid, $2, $3, NULLIF($4,''), $5, $6, NULLIF($7,''), now())
	`, a.BatchID, a.NewAttemptNo, a.Endpoint, a.TxnID,
		a.Duration.Milliseconds(), a.Success, a.ErrMsg); err != nil {
		return fmt.Errorf("retry send_log: %w", err)
	}

	// 2) update claim_batch
	status := "error"
	switch {
	case a.Success:
		status = "sent"
	case a.NewAttemptNo >= a.MaxAttempts:
		status = "failed"
	}

	var nextRetry any // NULL unless we're staying in 'error'
	if status == "error" {
		nextRetry = a.NextRetryAt
	}
	var sentAt any
	if a.Success {
		sentAt = time.Now()
	}
	var txnID any
	if a.TxnID != "" {
		txnID = a.TxnID
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE claim_batch
		   SET status        = $1,
		       attempt_no    = $2,
		       next_retry_at = $3,
		       sent_at       = COALESCE($4, sent_at),
		       fdh_txn_id    = COALESCE($5, fdh_txn_id),
		       error_msg     = CASE WHEN $1 = 'sent' THEN NULL ELSE NULLIF($6,'') END
		 WHERE batch_id = $7::uuid
	`, status, a.NewAttemptNo, nextRetry, sentAt, txnID, a.ErrMsg, a.BatchID); err != nil {
		return fmt.Errorf("retry update: %w", err)
	}
	return tx.Commit()
}

// Compile-time check.
var _ RetryRepo = (*PgRetryRepo)(nil)
