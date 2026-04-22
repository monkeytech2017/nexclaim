package tests

import (
	"testing"

	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/hisclient"
)

func TestBatchStore_PutAndGet(t *testing.T) {
	s := batch.New()
	b := s.Put(hisclient.VisitListRequest{
		HospitalCode: "12345", Period: "202504",
		Visits: []hisclient.VisitSummary{{VN: "VN1", HN: "HN1"}},
	})
	if b.ID == "" {
		t.Fatal("empty batch id")
	}
	if b.State != batch.StateReceived {
		t.Errorf("state = %s", b.State)
	}
	got, ok := s.Get(b.ID)
	if !ok || got.HospitalCode != "12345" || len(got.Visits) != 1 {
		t.Errorf("get mismatch: %+v", got)
	}
}

func TestBatchStore_SetState(t *testing.T) {
	s := batch.New()
	b := s.Put(hisclient.VisitListRequest{
		HospitalCode: "12345", Period: "202504",
		Visits: []hisclient.VisitSummary{{VN: "V1", HN: "H1"}},
	})
	if err := s.SetState(b.ID, batch.StateCompleted, ""); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, _ := s.Get(b.ID)
	if got.State != batch.StateCompleted {
		t.Errorf("state not updated: %s", got.State)
	}
}

func TestBatchStore_SetState_NotFound(t *testing.T) {
	s := batch.New()
	if err := s.SetState("MISSING", batch.StateCompleted, ""); err == nil {
		t.Error("expected not found error")
	}
}
