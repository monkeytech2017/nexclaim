// Package store = persistent claim history (claim_batch, claim_record, ...).
//
// Pipeline ยังคง stateless — หลัง pipeline.Run เสร็จ, server handlers
// เรียก ClaimRepo.SaveRun(outcome) เพื่อ persist. Implementations:
//
//   NoopClaimRepo — dev/test ไม่มี DB
//   PgClaimRepo   — เขียนเข้า Postgres (migration 003)
package store

import (
	"context"

	"github.com/nexclaim/nexclaim/internal/pipeline"
)

// ClaimRepo persists the outcome of a pipeline run.
type ClaimRepo interface {
	SaveRun(ctx context.Context, req SaveRequest) error
}

// SaveRequest รวมทุกอย่างที่ ClaimRepo ต้องใช้. ถือ pipeline.Outcome ตรง ๆ
// เพื่อไม่ต้อง duplicate structs.
type SaveRequest struct {
	HCode   string
	Period  string
	Outcome *pipeline.Outcome
}

// NoopClaimRepo is a no-op implementation used when persistence isn't configured.
type NoopClaimRepo struct{}

func (NoopClaimRepo) SaveRun(_ context.Context, _ SaveRequest) error { return nil }

// Assertion
var _ ClaimRepo = NoopClaimRepo{}
