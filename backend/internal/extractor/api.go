package extractor

import (
	"context"
	"fmt"

	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/model"
)

// APIExtractor ดึงข้อมูลผ่าน HIS API โดยอ้างอิง batch ที่เก็บไว้ใน Store.
//
// Flow:
//   1. Lookup batch ตาม BatchID
//   2. สำหรับทุก VN ใน batch: GET /api/nexclaim/opd/visit/{vn}
//   3. Map response → model.OPDVisit (carry over HN จาก summary)
//   4. คืน Result{OPD: [...]}
//
// IPD ยังไม่รองรับผ่าน API (ใช้ share-file แทน — Slice B).
type APIExtractor struct {
	Store   batch.Store
	Client  *hisclient.Client
	BatchID string
	// TMTFactory (optional) builds a HIS-drug-code → TMT resolver once the
	// hospital code is known. Nil = legacy fallback only.
	TMTFactory TMTResolverFactory
}

func NewAPIExtractor(st batch.Store, client *hisclient.Client, batchID string) *APIExtractor {
	return &APIExtractor{Store: st, Client: client, BatchID: batchID}
}

// WithTMTFactory returns the extractor with a his_drug_map-backed resolver
// factory wired for TMT translation. Safe to call with a nil factory (no-op).
func (e *APIExtractor) WithTMTFactory(f TMTResolverFactory) *APIExtractor {
	e.TMTFactory = f
	return e
}

func (e *APIExtractor) Extract(ctx context.Context, _ Request) (Result, error) {
	if e.Store == nil || e.Client == nil {
		return Result{}, fmt.Errorf("apiextractor: store + client required")
	}
	b, ok := e.Store.Get(e.BatchID)
	if !ok {
		return Result{}, fmt.Errorf("apiextractor: batch %s not found", e.BatchID)
	}

	_ = e.Store.SetState(e.BatchID, batch.StateFetching, "")

	resolver := resolverFor(e.TMTFactory, ctx, b.HospitalCode)

	out := Result{}
	for _, sum := range b.Visits {
		if err := ctx.Err(); err != nil {
			_ = e.Store.SetState(e.BatchID, batch.StateFailed, err.Error())
			return out, fmt.Errorf("apiextractor: canceled: %w", err)
		}
		detail, err := e.Client.GetVisit(sum.VN)
		if err != nil {
			msg := fmt.Sprintf("fetch vn=%s: %v", sum.VN, err)
			_ = e.Store.SetState(e.BatchID, batch.StateFailed, msg)
			return out, fmt.Errorf("apiextractor: %s", msg)
		}
		visit := hisclient.ToOPDVisit(detail, sum.HN, resolver)
		// ถ้า INSCL ใน detail ว่าง ใช้ของ summary เป็น fallback
		if visit.Patient.INSCL == "" {
			visit.Patient.INSCL = model.INSCL(sum.INSCL)
		}
		out.OPD = append(out.OPD, visit)
	}

	_ = e.Store.SetState(e.BatchID, batch.StateCompleted, "")
	return out, nil
}
