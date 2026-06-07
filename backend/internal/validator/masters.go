package validator

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// MasterValidator = lookup ว่า code ที่ใช้ใน claim เป็นรหัสที่ถูกต้องตาม master
// (ICD-10 WHO, ICD-9CM, TMT). TMT ในที่นี้คือ TMTID (Thai Medicines
// Terminology โดย THIS/สวรส.) — running numeric id ปัจจุบัน 6–7 หลัก เก็บใน
// คอลัมน์ tmt_code ไม่ใช่ "รหัสยา 24 หลัก" ของ สปสช. ซึ่งเป็นระบบ legacy คนละชุด.
//
// Implementations:
//   NoopMaster   — always valid (tests / sample fixtures ที่ยังไม่ seed master)
//   StaticMaster — in-memory set (load-once-on-startup from Postgres)
type MasterValidator interface {
	IsValidICD10(code string) bool
	IsValidICD9CM(code string) bool
	IsValidTMT(code string) bool
}

// NoopMaster bypasses master validation. Use in tests or during bootstrap
// when the master tables are empty and the check would be noise.
type NoopMaster struct{}

func (NoopMaster) IsValidICD10(string) bool  { return true }
func (NoopMaster) IsValidICD9CM(string) bool { return true }
func (NoopMaster) IsValidTMT(string) bool    { return true }

// StaticMaster holds sets of active master codes in memory. Built once via
// LoadFromDB — server must restart (or hit a future /admin/reload-masters
// endpoint) to pick up newly-added codes.
type StaticMaster struct {
	icd10  map[string]struct{}
	icd9cm map[string]struct{}
	tmt    map[string]struct{}
}

func NewStaticMaster(icd10, icd9cm, tmt []string) *StaticMaster {
	m := &StaticMaster{
		icd10:  make(map[string]struct{}, len(icd10)),
		icd9cm: make(map[string]struct{}, len(icd9cm)),
		tmt:    make(map[string]struct{}, len(tmt)),
	}
	for _, c := range icd10 {
		m.icd10[c] = struct{}{}
	}
	for _, c := range icd9cm {
		m.icd9cm[c] = struct{}{}
	}
	for _, c := range tmt {
		m.tmt[c] = struct{}{}
	}
	return m
}

func (s *StaticMaster) IsValidICD10(c string) bool  { _, ok := s.icd10[c]; return ok }
func (s *StaticMaster) IsValidICD9CM(c string) bool { _, ok := s.icd9cm[c]; return ok }
func (s *StaticMaster) IsValidTMT(c string) bool    { _, ok := s.tmt[c]; return ok }

// MasterCounts summarizes how many codes each set holds (for logs).
type MasterCounts struct {
	ICD10  int
	ICD9CM int
	TMT    int
}

func (c MasterCounts) String() string {
	return fmt.Sprintf("icd10=%d icd9cm=%d tmt=%d", c.ICD10, c.ICD9CM, c.TMT)
}

// LoadFromDB reads active codes from m_icd10 / m_icd9cm / m_tmt_drug
// and returns a ready-to-use *StaticMaster + counts for logging.
// Returns NoopMaster as fallback on error so callers can still start.
func LoadFromDB(ctx context.Context, db *sqlx.DB) (MasterValidator, MasterCounts, error) {
	var icd10, icd9cm, tmt []string

	if err := db.SelectContext(ctx, &icd10,
		`SELECT code FROM m_icd10 WHERE is_active`); err != nil {
		return NoopMaster{}, MasterCounts{}, fmt.Errorf("load m_icd10: %w", err)
	}
	if err := db.SelectContext(ctx, &icd9cm,
		`SELECT code FROM m_icd9cm WHERE is_active`); err != nil {
		return NoopMaster{}, MasterCounts{}, fmt.Errorf("load m_icd9cm: %w", err)
	}
	if err := db.SelectContext(ctx, &tmt,
		`SELECT tmt_code FROM m_tmt_drug WHERE is_active`); err != nil {
		return NoopMaster{}, MasterCounts{}, fmt.Errorf("load m_tmt_drug: %w", err)
	}

	return NewStaticMaster(icd10, icd9cm, tmt),
		MasterCounts{ICD10: len(icd10), ICD9CM: len(icd9cm), TMT: len(tmt)},
		nil
}
