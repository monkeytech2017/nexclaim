package store

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
)

// PgClaimRepo persists one claim_batch row per Submission + one claim_record
// row per OPD visit / IPD admit that went into it (per schema 003).
type PgClaimRepo struct {
	db *sqlx.DB
}

func NewPg(db *sqlx.DB) *PgClaimRepo { return &PgClaimRepo{db: db} }

func (r *PgClaimRepo) SaveRun(ctx context.Context, req SaveRequest) error {
	if req.Outcome == nil {
		return nil
	}
	if len(req.Outcome.Submissions) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("tx begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, sub := range req.Outcome.Submissions {
		records := pickRecords(sub.Format, req.Outcome)
		batchID, err := insertBatch(ctx, tx, req, sub, len(records))
		if err != nil {
			return fmt.Errorf("insert claim_batch: %w", err)
		}
		for _, rec := range records {
			if err := insertRecord(ctx, tx, batchID, req.HCode, rec); err != nil {
				return fmt.Errorf("insert claim_record: %w", err)
			}
		}
		if sub.Attempt != nil {
			if err := insertSendLog(ctx, tx, batchID, sub); err != nil {
				return fmt.Errorf("insert send_log: %w", err)
			}
		}
	}
	return tx.Commit()
}

// ── insert helpers ─────────────────────────────────────────────

func insertBatch(ctx context.Context, tx *sqlx.Tx, req SaveRequest, sub pipeline.Submission, total int) (string, error) {
	status, errMsg, sentAt := statusFrom(sub)
	var md5Hex *string
	if len(sub.ZipBytes) > 0 {
		h := fmt.Sprintf("%x", md5.Sum(sub.ZipBytes))
		md5Hex = &h
	}
	var zipName *string
	if sub.ZipName != "" {
		z := sub.ZipName
		zipName = &z
	}
	var txnID *string
	if sub.TxnID != "" {
		t := sub.TxnID
		txnID = &t
	}
	validationErrs := len(req.Outcome.ValidationErrors)
	valid := total - validationErrs
	if valid < 0 {
		valid = 0
	}

	// Persist zip_bytes so the retry worker can re-send without rebuilding
	// the pipeline. NULL when no bytes were produced (build failure, dry-run).
	var zipBytes any
	if len(sub.ZipBytes) > 0 {
		zipBytes = sub.ZipBytes
	}
	// Schedule the first retry ~1 minute out when the initial submit failed.
	// attempt_no defaults to 1 (the attempt we just recorded, whether the
	// attempt row is in send_log yet or not).
	var nextRetryAt any
	if status == "error" && len(sub.ZipBytes) > 0 {
		nextRetryAt = time.Now().Add(1 * time.Minute)
	}

	var id string
	err := tx.QueryRowxContext(ctx, `
		INSERT INTO claim_batch
		  (hcode, period, inscl, format, sender, status,
		   total_records, valid_records, error_records,
		   fdh_txn_id, zip_filename, zip_md5,
		   created_at, sent_at, error_msg,
		   zip_bytes, attempt_no, next_retry_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,1,$17)
		RETURNING batch_id
	`,
		req.HCode, req.Period, string(req.Outcome.INSCL),
		string(sub.Format), string(senderFor(sub.Format)),
		status, total, valid, validationErrs,
		txnID, zipName, md5Hex,
		time.Now(), sentAt, nullIfEmpty(errMsg),
		zipBytes, nextRetryAt,
	).Scan(&id)
	return id, err
}

func insertSendLog(ctx context.Context, tx *sqlx.Tx, batchID string, sub pipeline.Submission) error {
	att := sub.Attempt
	_, err := tx.ExecContext(ctx, `
		INSERT INTO send_log (batch_id, attempt_no, endpoint, fdh_txn_id,
		                     response_body, duration_ms, success, error_msg, sent_at)
		VALUES ($1, 1, $2, NULLIF($3,''), NULLIF($4,''), $5, $6, NULLIF($7,''), $8)
	`, batchID, att.Endpoint, sub.TxnID, att.Response, att.DurationMs, att.Success, att.ErrorMsg, att.SentAt)
	return err
}

func insertRecord(ctx context.Context, tx *sqlx.Tx, batchID, hcode string, rec record) error {
	raw, _ := json.Marshal(rec.raw)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO claim_record
		  (batch_id, hcode, hn, an, seq, is_ipd, service_date, inscl, pttype,
		   status, total_charge, paid_amount, has_error, raw_json)
		VALUES ($1,$2,$3, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, NULLIF($9,''),
		        'pending', $10, $11, false, $12)
	`,
		batchID, hcode, rec.hn, rec.an, rec.seq, rec.isIPD,
		rec.serviceDate, string(rec.inscl), rec.pttype,
		rec.totalCharge, rec.paidAmount, raw,
	)
	return err
}

// ── record normalization ─────────────────────────────────────

type record struct {
	hn          string
	an          string
	seq         string
	isIPD       bool
	serviceDate time.Time
	inscl       model.INSCL
	pttype      string
	totalCharge float64
	paidAmount  float64
	raw         any
}

func pickRecords(format model.ClaimFormat, out *pipeline.Outcome) []record {
	switch format {
	case model.Format16Files:
		rs := make([]record, 0, len(out.OPD)+len(out.IPD))
		for _, v := range out.OPD {
			rs = append(rs, fromOPD(v))
		}
		for _, a := range out.IPD {
			rs = append(rs, fromIPD(a))
		}
		return rs
	case model.FormatCSOP, model.FormatSSOP:
		rs := make([]record, 0, len(out.OPD))
		for _, v := range out.OPD {
			rs = append(rs, fromOPD(v))
		}
		return rs
	case model.FormatCIPN, model.FormatAIPN:
		rs := make([]record, 0, len(out.IPD))
		for _, a := range out.IPD {
			rs = append(rs, fromIPD(a))
		}
		return rs
	}
	return nil
}

func fromOPD(v model.OPDVisit) record {
	return record{
		hn: v.Patient.HN, seq: v.SEQ,
		isIPD: false, serviceDate: v.DateOPD,
		inscl: v.Patient.INSCL, pttype: v.Patient.PTTYPE,
		totalCharge: v.Total, paidAmount: v.Paid,
		raw: v,
	}
}

func fromIPD(a model.IPDAdmit) record {
	return record{
		hn: a.Patient.HN, an: a.AN,
		isIPD: true, serviceDate: a.DateAdm,
		inscl: a.Patient.INSCL, pttype: a.Patient.PTTYPE,
		totalCharge: a.Total, paidAmount: a.Paid,
		raw: a,
	}
}

// ── small helpers ────────────────────────────────────────────

func statusFrom(sub pipeline.Submission) (string, string, any) {
	if sub.Err != nil {
		return "error", sub.Err.Error(), nil
	}
	if sub.TxnID != "" {
		return "sent", "", time.Now()
	}
	return "pending", "", nil
}

func senderFor(f model.ClaimFormat) model.Sender {
	switch f {
	case model.FormatAIPN, model.FormatSSOP:
		return model.SenderCHI
	default:
		return model.SenderFDH
	}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Assertion
var _ ClaimRepo = (*PgClaimRepo)(nil)
