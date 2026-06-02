// Package retry runs a background loop that re-sends claim_batch rows
// whose original submit (pipeline.submitOne) failed.
//
// Lifecycle of claim_batch.status:
//
//	pending  — record inserted, sender not called yet (dry_run, or build error)
//	sent     — last send_log row has success=true
//	error    — sender call failed, attempt_no < max_attempts, eligible
//	failed   — gave up after max_attempts (terminal; no more retries)
//
// The worker is OPT-IN: cmd/server.go only wires it when
// RETRY_WORKER_ENABLED=true. When disabled the existing behaviour is
// unchanged — operators bump attempt_no in SQL or re-trigger batches by
// hand.
package retry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nexclaim/nexclaim/internal/audit"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/store"
)

// Worker periodically scans claim_batch for eligible rows and re-sends them.
//
// Nil-safe: if Repo is nil or both submitters are nil, Run() returns
// immediately; Tick() is a no-op. This lets server wiring always call
// Run() on a goroutine without having to duplicate the env gate.
type Worker struct {
	// Repo persists retry bookkeeping. Usually *store.PgRetryRepo in prod,
	// an in-memory fake in tests.
	Repo store.RetryRepo
	// FDH handles 16FILES / CIPN / CSOP retries. May be nil if only CHI
	// formats are expected.
	FDH pipeline.FDHSubmitter
	// CHI handles AIPN / SSOP retries. May be nil.
	CHI pipeline.CHISubmitter
	// MaxAttempts caps the total number of attempts (initial + retries).
	// 0 → defaults to 5.
	MaxAttempts int
	// Interval between Tick() invocations. 0 → 30 seconds.
	Interval time.Duration
	// AuditWriter is best-effort (write failures are logged and swallowed).
	// Nil falls back to audit.NoopWriter so the worker is always safe to run.
	AuditWriter audit.Writer
	// BatchLimit caps how many rows one Tick processes. 0 → 50.
	BatchLimit int
}

const (
	defaultMaxAttempts = 5
	defaultInterval    = 30 * time.Second
	defaultBatchLimit  = 50
	// backoffCap bounds the exponential backoff so a stuck batch is still
	// retried at least twice an hour.
	backoffCap = 30 * time.Minute
	// backoffBase is the first retry delay. Subsequent retries double it.
	backoffBase = 1 * time.Minute
)

// Run blocks until ctx is done, invoking Tick every Interval.
// No-op when the worker is not fully wired.
func (w *Worker) Run(ctx context.Context) {
	if !w.enabled() {
		return
	}
	interval := w.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	// Run one tick immediately so ops don't wait the full interval on boot.
	w.Tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Tick(ctx)
		}
	}
}

// Tick selects eligible batches and retries each in series. Per-batch
// failures (sender error, DB error) are logged + swallowed so one bad row
// doesn't starve the others.
func (w *Worker) Tick(ctx context.Context) {
	if !w.enabled() {
		return
	}
	max := w.max()
	rows, err := w.Repo.SelectRetryable(ctx, w.limit(), max)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[NexClaim] retry select: %v\n", err)
		return
	}
	for _, row := range rows {
		w.process(ctx, row, max)
	}
}

func (w *Worker) process(ctx context.Context, row store.RetryRow, max int) {
	newAttempt := row.AttemptNo + 1
	sub := w.pickSubmitter(row.Format)
	if sub == nil {
		// No submitter configured for this format — leave the row in 'error'.
		// Log loudly so ops can spot the misconfiguration.
		fmt.Fprintf(os.Stderr,
			"[NexClaim] retry skip batch=%s format=%s: no submitter wired\n",
			row.BatchID, row.Format)
		return
	}

	start := time.Now()
	res, sendErr := sub(row.ZipBytes, row.Period)
	duration := time.Since(start)

	success := sendErr == nil
	errMsg := ""
	if sendErr != nil {
		errMsg = truncate(sendErr.Error(), 1024)
	}
	txnID := ""
	if success && res != nil {
		txnID = res.TxnID
	}

	next := nextRetryAt(newAttempt)
	args := store.RecordRetryArgs{
		BatchID:      row.BatchID,
		NewAttemptNo: newAttempt,
		MaxAttempts:  max,
		Success:      success,
		Duration:     duration,
		Endpoint:     endpointFor(row.Format),
		ErrMsg:       errMsg,
		TxnID:        txnID,
		NextRetryAt:  next,
	}
	if err := w.Repo.RecordRetry(ctx, args); err != nil {
		fmt.Fprintf(os.Stderr,
			"[NexClaim] retry record batch=%s attempt=%d: %v\n",
			row.BatchID, newAttempt, err)
		return
	}

	// Audit is best-effort (matches persistRun / writeAudit pattern).
	w.writeAudit(ctx, row, args)
}

// ── helpers ─────────────────────────────────────────────────

// submitFunc abstracts over FDH.Send16Files / CHI.SendAIPN / etc. so
// process() stays linear.
type submitFunc func(zip []byte, period string) (*sender.SubmitResult, error)

func (w *Worker) pickSubmitter(f model.ClaimFormat) submitFunc {
	switch f {
	case model.Format16Files:
		if w.FDH == nil {
			return nil
		}
		return w.FDH.Send16Files
	case model.FormatCIPN:
		if w.FDH == nil {
			return nil
		}
		return w.FDH.SendCIPN
	case model.FormatCSOP:
		if w.FDH == nil {
			return nil
		}
		return w.FDH.SendCSOP
	case model.FormatAIPN:
		if w.CHI == nil {
			return nil
		}
		return w.CHI.SendAIPN
	case model.FormatSSOP:
		if w.CHI == nil {
			return nil
		}
		return w.CHI.SendSSOP
	}
	return nil
}

// nextRetryAt computes the next eligible retry time using exponential
// backoff capped at backoffCap. newAttempt is the attempt number we JUST
// finished (failed). So after attempt 1 (initial run) we wait 1min, after
// attempt 2 we wait 2min, after 3 → 4min, 4 → 8min, 5 → 16min, 6+ → 30min.
func nextRetryAt(newAttempt int) time.Time {
	delay := backoffBase
	// attempt 2 (first retry just happened) → delay = base * 2^0 = 1min
	// attempt 3 → 2min, attempt 4 → 4min, etc.
	for i := 0; i < newAttempt-1; i++ {
		delay *= 2
		if delay >= backoffCap {
			delay = backoffCap
			break
		}
	}
	if delay > backoffCap {
		delay = backoffCap
	}
	return time.Now().Add(delay)
}

// endpointFor mirrors pipeline.endpointFor — kept private there so we
// re-implement the small switch. (Single source of truth isn't worth the
// exported symbol.)
func endpointFor(f model.ClaimFormat) string {
	switch f {
	case model.Format16Files:
		return "/api/claim/16files"
	case model.FormatCIPN:
		return "/api/claim/cipn"
	case model.FormatCSOP:
		return "/api/claim/csop"
	case model.FormatAIPN:
		return "/aipnupload/"
	case model.FormatSSOP:
		return "/ssopupload/"
	}
	return ""
}

func (w *Worker) writeAudit(ctx context.Context, row store.RetryRow, args store.RecordRetryArgs) {
	writer := w.AuditWriter
	if writer == nil {
		writer = audit.NoopWriter{}
	}
	payload := map[string]any{
		"attempt_no":  args.NewAttemptNo,
		"success":     args.Success,
		"format":      string(row.Format),
		"duration_ms": args.Duration.Milliseconds(),
	}
	if args.ErrMsg != "" {
		payload["error"] = args.ErrMsg
	}
	if args.TxnID != "" {
		payload["txn_id"] = args.TxnID
	}
	entry := audit.Entry{
		ActorRole:  "system",
		ActorName:  "retry-worker",
		Action:     "pipeline.retry",
		TargetKind: "claim_batch",
		TargetID:   row.BatchID,
		HCode:      row.HCode,
		Payload:    payload,
	}
	if err := writer.Write(ctx, entry); err != nil {
		fmt.Fprintf(os.Stderr, "[NexClaim] retry audit: %v\n", err)
	}
}

func (w *Worker) enabled() bool {
	if w == nil || w.Repo == nil {
		return false
	}
	return w.FDH != nil || w.CHI != nil
}

func (w *Worker) max() int {
	if w.MaxAttempts <= 0 {
		return defaultMaxAttempts
	}
	return w.MaxAttempts
}

func (w *Worker) limit() int {
	if w.BatchLimit <= 0 {
		return defaultBatchLimit
	}
	return w.BatchLimit
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
