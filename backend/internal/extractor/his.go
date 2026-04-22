// Package extractor ดึงข้อมูลจาก HIS Database
//
// Architecture: Extractor เป็น interface; implementation จริงจะ query HIS DB
// โดยใช้ his_field_map + his_inscl_map เพื่อแปลง column ของ HIS ไทย
// (HOSxP / JHCIS / SSB / SD) ให้เป็น domain model ของ NexClaim.
//
// สำหรับ dry-run และ unit test มี MemoryExtractor ที่ถือ fixture data
// ใน memory — พอสำหรับ test pipeline ทั้งสาย โดยไม่ต้องแตะ DB จริง.
package extractor

import (
	"context"

	"github.com/nexclaim/nexclaim/internal/model"
)

// Request ตัวกรองสิ่งที่ต้องการดึงออกจาก HIS
type Request struct {
	INSCL  model.INSCL
	Period string // YYYYMM
}

// Result ข้อมูลที่สกัดออกมาพร้อม route → generator
type Result struct {
	OPD []model.OPDVisit
	IPD []model.IPDAdmit
}

// Extractor เป็น interface ที่ layer อื่นใช้ — DB-backed หรือ in-memory
// ก็ได้เหมือนกัน.
type Extractor interface {
	Extract(ctx context.Context, r Request) (Result, error)
}

// ── MemoryExtractor ───────────────────────────────────────────

// MemoryExtractor เก็บ fixture ใน memory — ใช้สำหรับ test + dry-run
type MemoryExtractor struct {
	// Data keyed by period ("YYYYMM"); each entry grouped by INSCL.
	Data map[string]map[model.INSCL]Result
}

func NewMemoryExtractor() *MemoryExtractor {
	return &MemoryExtractor{Data: make(map[string]map[model.INSCL]Result)}
}

// Put เพิ่ม/ทับข้อมูลของ period + inscl
func (m *MemoryExtractor) Put(period string, inscl model.INSCL, r Result) {
	if m.Data[period] == nil {
		m.Data[period] = make(map[model.INSCL]Result)
	}
	m.Data[period][inscl] = r
}

func (m *MemoryExtractor) Extract(_ context.Context, r Request) (Result, error) {
	if p, ok := m.Data[r.Period]; ok {
		if res, ok := p[r.INSCL]; ok {
			return res, nil
		}
	}
	return Result{}, nil
}
