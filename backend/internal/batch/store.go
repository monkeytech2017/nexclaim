// Package batch เก็บ OPD ingest batches ที่ HIS push เข้ามาก่อน pipeline ทำงาน.
//
// Store มีสอง implementation:
//   - MemoryStore  — in-process map (dev/test, หายเมื่อ restart)
//   - PgStore      — Postgres-backed (prod, survive restart)
//
// pipeline ใช้ interface Store — ไม่ต้องรู้ว่าอยู่ใน backend ไหน.
package batch

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/nexclaim/nexclaim/internal/hisclient"
)

// State ของ batch ในระบบ
type State string

const (
	StateReceived  State = "RECEIVED"
	StateFetching  State = "FETCHING"
	StateCompleted State = "COMPLETED"
	StateFailed    State = "FAILED"
)

// Batch = หนึ่ง push จาก HIS (hospital_code + period + visits).
type Batch struct {
	ID           string                   `json:"batch_id"           db:"batch_id"`
	HospitalCode string                   `json:"hospital_code"      db:"hcode"`
	Period       string                   `json:"period"             db:"period"`
	ExportedBy   string                   `json:"exported_by,omitempty" db:"exported_by"`
	State        State                    `json:"state"              db:"state"`
	CreatedAt    time.Time                `json:"created_at"         db:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"         db:"updated_at"`
	Visits       []hisclient.VisitSummary `json:"visits"`
	LastError    string                   `json:"last_error,omitempty" db:"last_error"`
}

// Store abstracts batch persistence. MemoryStore/PgStore implement it.
type Store interface {
	Put(req hisclient.VisitListRequest) *Batch
	Get(id string) (*Batch, bool)
	SetState(id string, state State, errMsg string) error
	List() []*Batch
}

// New returns a default MemoryStore. Prefer NewMemory/NewPostgres for clarity.
func New() Store { return NewMemory() }

// NewMemory constructs an in-process, thread-safe store.
func NewMemory() *MemoryStore {
	return &MemoryStore{batches: make(map[string]*Batch)}
}

// ── MemoryStore ───────────────────────────────────────────────

type MemoryStore struct {
	mu      sync.RWMutex
	batches map[string]*Batch
}

func (s *MemoryStore) Put(req hisclient.VisitListRequest) *Batch {
	b := newBatch(req)
	s.mu.Lock()
	s.batches[b.ID] = b
	s.mu.Unlock()
	return b
}

func (s *MemoryStore) Get(id string) (*Batch, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.batches[id]
	return b, ok
}

func (s *MemoryStore) SetState(id string, state State, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.batches[id]
	if !ok {
		return fmt.Errorf("batch %s not found", id)
	}
	b.State = state
	b.LastError = errMsg
	b.UpdatedAt = time.Now()
	return nil
}

func (s *MemoryStore) List() []*Batch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Batch, 0, len(s.batches))
	for _, b := range s.batches {
		out = append(out, b)
	}
	return out
}

// ── Shared helpers ────────────────────────────────────────────

func newBatch(req hisclient.VisitListRequest) *Batch {
	now := time.Now()
	return &Batch{
		ID:           "BATCH-" + now.Format("20060102") + "-" + shortUUID(),
		HospitalCode: req.HospitalCode,
		Period:       req.Period,
		ExportedBy:   req.ExportedBy,
		State:        StateReceived,
		CreatedAt:    now,
		UpdatedAt:    now,
		Visits:       req.Visits,
	}
}

func shortUUID() string {
	return uuid.New().String()[:8]
}
