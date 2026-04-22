package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/nexclaim/nexclaim/internal/response"
)

// REPFetcher ขั้นต่ำที่ IngestREP ต้องการ (FDHClient implements it).
type REPFetcher interface {
	GetREP(period string) ([]byte, error)
}

// REPIngester บอก: ดึง REP.xml จาก FDH (period) → parse → match claim_record
// → insert c_code_log per item. ถ้า match ไม่ได้ จะ fallback ใช้ claim_batch
// ใน (hcode, period) ตัวแรก — ไม่ drop เพราะ operator ต้องเห็น error code
// ไม่ว่ามาจาก record ไหนก็ตาม.
type REPIngester struct {
	db    *sqlx.DB
	Repo  CCodeRepo
	Fetch REPFetcher
}

func NewREPIngester(db *sqlx.DB, repo CCodeRepo, fetch REPFetcher) *REPIngester {
	return &REPIngester{db: db, Repo: repo, Fetch: fetch}
}

// IngestResult สรุปผลการดึง REP หนึ่งรอบ.
type IngestResult struct {
	Fetched  int `json:"fetched"`
	Inserted int `json:"inserted"`
	Skipped  int `json:"skipped"` // no matching claim_batch
	Errors   int `json:"errors"`  // insert failed
}

// Ingest ทำงานตามรายการข้างต้น. hcode + period เป็นสิ่งที่ caller รู้
// (เช่น จาก query string ของ /api/v1/claim/rep/:hcode/:period).
func (i *REPIngester) Ingest(ctx context.Context, hcode, period string) (*IngestResult, error) {
	if i.Fetch == nil {
		return nil, fmt.Errorf("rep ingest: fetcher not configured")
	}
	xml, err := i.Fetch.GetREP(period)
	if err != nil {
		return nil, fmt.Errorf("fetch rep: %w", err)
	}
	items, err := response.ParseREP(xml)
	if err != nil {
		return nil, fmt.Errorf("parse rep: %w", err)
	}
	res := &IngestResult{Fetched: len(items)}

	// Precompute a fallback batch_id for this (hcode, period) in case
	// per-record lookup misses — the c_code_log.batch_id FK is NOT NULL.
	var fallbackBatchID string
	_ = i.db.GetContext(ctx, &fallbackBatchID, `
		SELECT batch_id::text FROM claim_batch
		WHERE hcode = $1 AND period = $2
		ORDER BY created_at DESC LIMIT 1
	`, hcode, period)

	for _, it := range items {
		batchID, recordID := i.Repo.LookupClaimRecord(ctx, hcode, period, it.HN, it.AN, it.SEQ)
		if batchID == "" {
			batchID = fallbackBatchID
		}
		if batchID == "" {
			res.Skipped++
			continue
		}
		anOrSeq := it.AN
		if anOrSeq == "" {
			anOrSeq = it.SEQ
		}
		if _, err := i.Repo.Insert(ctx, CCodeInsert{
			BatchID:    batchID,
			RecordID:   recordID,
			HN:         it.HN,
			ANOrSEQ:    anOrSeq,
			CCode:      it.CCode,
			CDesc:      it.CDesc,
			FieldName:  it.FieldName,
			FieldValue: it.FieldValue,
		}); err != nil {
			res.Errors++
			continue
		}
		res.Inserted++
	}
	return res, nil
}
