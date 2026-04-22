// Package batch เก็บ VisitSummary ที่ HIS push เข้ามาก่อนที่ NexClaim
// จะไปดึง detail + ประมวลผล pipeline.
//
// Implementation ตอนนี้ = in-memory (map ใน process). รองรับ single-instance
// server. สำหรับ multi-instance ให้สลับเป็น Redis/Postgres store ที่มี
// interface เดียวกัน.
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

// Batch หนึ่ง batch = หนึ่ง push จาก HIS (hospital_code + period + visits).
type Batch struct {
	ID           string                   `json:"batch_id"`
	HospitalCode string                   `json:"hospital_code"`
	Period       string                   `json:"period"`
	ExportedBy   string                   `json:"exported_by,omitempty"`
	State        State                    `json:"state"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
	Visits       []hisclient.VisitSummary `json:"visits"`
	// Optional: ผลลัพธ์หลัง process (เก็บไว้ให้ client poll status)
	LastError string `json:"last_error,omitempty"`
}

// Store = in-memory batch repository.
type Store struct {
	mu      sync.RWMutex
	batches map[string]*Batch
}

func New() *Store {
	return &Store{batches: make(map[string]*Batch)}
}

// Put เก็บ batch ใหม่ + generate batchId. ส่งกลับ ID.
func (s *Store) Put(req hisclient.VisitListRequest) *Batch {
	b := &Batch{
		ID:           "BATCH-" + time.Now().Format("20060102") + "-" + shortUUID(),
		HospitalCode: req.HospitalCode,
		Period:       req.Period,
		ExportedBy:   req.ExportedBy,
		State:        StateReceived,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Visits:       req.Visits,
	}
	s.mu.Lock()
	s.batches[b.ID] = b
	s.mu.Unlock()
	return b
}

// Get อ่าน batch ตาม ID.
func (s *Store) Get(id string) (*Batch, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.batches[id]
	return b, ok
}

// SetState update state + optional error message.
func (s *Store) SetState(id string, state State, errMsg string) error {
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

// List คืน batch ทั้งหมด (sorted by CreatedAt asc). ใช้สำหรับ admin/dashboard.
func (s *Store) List() []*Batch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Batch, 0, len(s.batches))
	for _, b := range s.batches {
		out = append(out, b)
	}
	return out
}

func shortUUID() string {
	return uuid.New().String()[:8]
}
