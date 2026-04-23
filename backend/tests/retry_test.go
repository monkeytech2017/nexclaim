package tests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/retry"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/store"
)

// ── in-memory RetryRepo fake ──────────────────────────────────

type memRetryRepo struct {
	mu       sync.Mutex
	rows     []store.RetryRow
	statuses map[string]string // batch_id → status
	records  []store.RecordRetryArgs
	// Optional hook: forceError returns a non-nil error on SelectRetryable
	// to simulate DB failure.
	forceSelectErr error
}

func newMemRetryRepo() *memRetryRepo {
	return &memRetryRepo{statuses: map[string]string{}}
}

// seed adds a row in 'error' state. Caller specifies the current attempt_no
// so tests can drive the "last attempt" edge case directly.
func (m *memRetryRepo) seed(batchID, hcode, period string, format model.ClaimFormat, attempt int, nextRetry time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, store.RetryRow{
		BatchID:   batchID,
		HCode:     hcode,
		Period:    period,
		INSCL:     "UCS",
		Format:    format,
		Sender:    senderFor(format),
		AttemptNo: attempt,
		ZipName:   batchID + ".ZIP",
		ZipBytes:  []byte("zip-" + batchID),
	})
	m.statuses[batchID] = "error"
	// Store nextRetry on the row so SelectRetryable can gate on it.
	if !nextRetry.IsZero() {
		i := len(m.rows) - 1
		m.rows[i].NextRetryAt.Valid = true
		m.rows[i].NextRetryAt.Time = nextRetry
	}
}

func (m *memRetryRepo) setStatus(batchID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[batchID] = status
}

func (m *memRetryRepo) SelectRetryable(_ context.Context, limit, maxAttempts int) ([]store.RetryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.forceSelectErr != nil {
		return nil, m.forceSelectErr
	}
	out := []store.RetryRow{}
	now := time.Now()
	for _, r := range m.rows {
		if m.statuses[r.BatchID] != "error" {
			continue
		}
		if r.AttemptNo >= maxAttempts {
			continue
		}
		if r.NextRetryAt.Valid && r.NextRetryAt.Time.After(now) {
			continue
		}
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *memRetryRepo) RecordRetry(_ context.Context, args store.RecordRetryArgs) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, args)
	// Mirror the Pg impl's state transitions.
	switch {
	case args.Success:
		m.statuses[args.BatchID] = "sent"
	case args.NewAttemptNo >= args.MaxAttempts:
		m.statuses[args.BatchID] = "failed"
	default:
		m.statuses[args.BatchID] = "error"
	}
	// Bump attempt_no on the stored row so the next SelectRetryable sees it.
	for i := range m.rows {
		if m.rows[i].BatchID != args.BatchID {
			continue
		}
		m.rows[i].AttemptNo = args.NewAttemptNo
		if args.Success || m.statuses[args.BatchID] == "failed" {
			m.rows[i].NextRetryAt.Valid = false
		} else {
			m.rows[i].NextRetryAt.Valid = true
			m.rows[i].NextRetryAt.Time = args.NextRetryAt
		}
	}
	return nil
}

func senderFor(f model.ClaimFormat) model.Sender {
	switch f {
	case model.FormatAIPN, model.FormatSSOP:
		return model.SenderCHI
	default:
		return model.SenderFDH
	}
}

// ── stub FDH/CHI ──────────────────────────────────────────────

type stubFDH struct {
	mu    sync.Mutex
	calls int
	err   error // set per-call
}

func (s *stubFDH) Send16Files(z []byte, period string) (*sender.SubmitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return &sender.SubmitResult{TxnID: fmt.Sprintf("RETRY-%d-%s", s.calls, period), Status: "ACCEPTED"}, nil
}
func (s *stubFDH) SendCIPN(z []byte, period string) (*sender.SubmitResult, error) {
	return s.Send16Files(z, period)
}
func (s *stubFDH) SendCSOP(z []byte, period string) (*sender.SubmitResult, error) {
	return s.Send16Files(z, period)
}

type stubCHI struct{ calls int }

func (s *stubCHI) SendAIPN(z []byte, period string) (*sender.SubmitResult, error) {
	s.calls++
	return &sender.SubmitResult{TxnID: "CHI-" + period, Status: "ACCEPTED"}, nil
}
func (s *stubCHI) SendSSOP(z []byte, period string) (*sender.SubmitResult, error) {
	return s.SendAIPN(z, period)
}

// ── tests ─────────────────────────────────────────────────────

func TestRetryWorker_SuccessTransitionsToSent(t *testing.T) {
	repo := newMemRetryRepo()
	// seed: attempt_no=1 (initial pipeline run failed), next_retry_at elapsed.
	repo.seed("B1", "12345", "202504", model.Format16Files, 1, time.Now().Add(-1*time.Minute))

	fdh := &stubFDH{}
	w := &retry.Worker{Repo: repo, FDH: fdh, MaxAttempts: 5}
	w.Tick(context.Background())

	if fdh.calls != 1 {
		t.Errorf("FDH calls = %d, want 1", fdh.calls)
	}
	if len(repo.records) != 1 {
		t.Fatalf("records = %d, want 1", len(repo.records))
	}
	r := repo.records[0]
	if r.NewAttemptNo != 2 || !r.Success {
		t.Errorf("record = %+v, want attempt=2 success=true", r)
	}
	if r.TxnID == "" {
		t.Errorf("record txn_id should be populated on success")
	}
	if repo.statuses["B1"] != "sent" {
		t.Errorf("status = %q, want sent", repo.statuses["B1"])
	}
}

func TestRetryWorker_FailureIncrementsAttempt(t *testing.T) {
	repo := newMemRetryRepo()
	repo.seed("B2", "12345", "202504", model.Format16Files, 2, time.Now().Add(-1*time.Minute))

	fdh := &stubFDH{err: errors.New("502 bad gateway")}
	w := &retry.Worker{Repo: repo, FDH: fdh, MaxAttempts: 5}

	before := time.Now()
	w.Tick(context.Background())
	if fdh.calls != 1 {
		t.Errorf("FDH calls = %d", fdh.calls)
	}
	if len(repo.records) != 1 {
		t.Fatalf("records = %d", len(repo.records))
	}
	r := repo.records[0]
	if r.NewAttemptNo != 3 || r.Success {
		t.Errorf("record = %+v, want attempt=3 success=false", r)
	}
	if r.ErrMsg == "" {
		t.Errorf("ErrMsg should be populated on failure")
	}
	if !r.NextRetryAt.After(before) {
		t.Errorf("NextRetryAt should be in the future")
	}
	// Exponential backoff: after attempt_no=3 we wait 4min. Allow ±30s slack.
	expected := 4 * time.Minute
	got := time.Until(r.NextRetryAt)
	if got < expected-30*time.Second || got > expected+30*time.Second {
		t.Errorf("backoff delay = %v, want ~%v", got, expected)
	}
	if repo.statuses["B2"] != "error" {
		t.Errorf("status = %q, want error", repo.statuses["B2"])
	}
}

func TestRetryWorker_MaxAttemptsTransitionsToFailed(t *testing.T) {
	repo := newMemRetryRepo()
	// attempt_no already at max-1 (4) — next failure flips to 'failed'.
	repo.seed("B3", "12345", "202504", model.Format16Files, 4, time.Now().Add(-1*time.Minute))

	fdh := &stubFDH{err: errors.New("network timeout")}
	w := &retry.Worker{Repo: repo, FDH: fdh, MaxAttempts: 5}
	w.Tick(context.Background())

	if repo.statuses["B3"] != "failed" {
		t.Errorf("status = %q, want failed", repo.statuses["B3"])
	}

	// Subsequent ticks should NOT pick the row up.
	fdh.calls = 0
	w.Tick(context.Background())
	if fdh.calls != 0 {
		t.Errorf("failed row was retried: %d calls", fdh.calls)
	}
	if len(repo.records) != 1 {
		t.Errorf("records after second tick = %d, want 1", len(repo.records))
	}
}

func TestRetryWorker_SkipsNonErrorRows(t *testing.T) {
	repo := newMemRetryRepo()
	repo.seed("B-sent", "12345", "202504", model.Format16Files, 1, time.Time{})
	repo.setStatus("B-sent", "sent")
	repo.seed("B-pending", "12345", "202504", model.Format16Files, 1, time.Time{})
	repo.setStatus("B-pending", "pending")
	repo.seed("B-failed", "12345", "202504", model.Format16Files, 5, time.Time{})
	repo.setStatus("B-failed", "failed")

	fdh := &stubFDH{}
	w := &retry.Worker{Repo: repo, FDH: fdh, MaxAttempts: 5}
	w.Tick(context.Background())

	if fdh.calls != 0 {
		t.Errorf("want zero FDH calls, got %d", fdh.calls)
	}
	if len(repo.records) != 0 {
		t.Errorf("want zero records, got %d", len(repo.records))
	}
}

func TestRetryWorker_SkipsWhenNextRetryFuture(t *testing.T) {
	repo := newMemRetryRepo()
	repo.seed("B-future", "12345", "202504", model.Format16Files, 1, time.Now().Add(5*time.Minute))

	fdh := &stubFDH{}
	w := &retry.Worker{Repo: repo, FDH: fdh, MaxAttempts: 5}
	w.Tick(context.Background())

	if fdh.calls != 0 {
		t.Errorf("future next_retry_at was picked: %d calls", fdh.calls)
	}
}

func TestRetryWorker_DisabledNoopWhenNotConfigured(t *testing.T) {
	// No repo, no submitters → Tick() must be a no-op.
	w := &retry.Worker{}
	w.Tick(context.Background()) // must not panic

	// Repo but no submitters → also no-op.
	repo := newMemRetryRepo()
	repo.seed("B-x", "12345", "202504", model.Format16Files, 1, time.Time{})
	w2 := &retry.Worker{Repo: repo}
	w2.Tick(context.Background())
	if len(repo.records) != 0 {
		t.Errorf("no submitters → want zero records, got %d", len(repo.records))
	}
}

func TestRetryWorker_ChiFormatRoutesToChi(t *testing.T) {
	repo := newMemRetryRepo()
	repo.seed("B-chi", "12345", "202504", model.FormatAIPN, 1, time.Now().Add(-1*time.Minute))

	fdh := &stubFDH{}
	chi := &stubCHI{}
	w := &retry.Worker{Repo: repo, FDH: fdh, CHI: chi, MaxAttempts: 5}
	w.Tick(context.Background())

	if chi.calls != 1 || fdh.calls != 0 {
		t.Errorf("AIPN should hit CHI only: fdh=%d chi=%d", fdh.calls, chi.calls)
	}
}
