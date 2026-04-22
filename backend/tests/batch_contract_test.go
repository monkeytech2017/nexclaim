package tests

import (
	"os"
	"testing"

	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/hisclient"
)

// contractTest exercises the batch.Store interface so the same tests validate
// both MemoryStore and PgStore. PgStore test runs only if POSTGRES_TEST_DSN
// is set (pointing at a throw-away database where the 004 migration is applied).
func contractTest(t *testing.T, s batch.Store) {
	t.Helper()

	req := hisclient.VisitListRequest{
		HospitalCode: "12345",
		Period:       "202504",
		ExportedBy:   "contract-test",
		Visits: []hisclient.VisitSummary{
			{VN: "VN-A", HN: "HN1", PID: "1100101001271", INSCL: "UCS", VisitDate: "20250418"},
			{VN: "VN-B", HN: "HN2", PID: "1100101001271", INSCL: "011", VisitDate: "20250418", TotalCharge: 1250},
		},
	}
	b := s.Put(req)
	if b.ID == "" {
		t.Fatal("Put returned empty batch ID")
	}
	if b.State != batch.StateReceived {
		t.Errorf("new batch state = %s, want RECEIVED", b.State)
	}
	if len(b.Visits) != 2 {
		t.Errorf("new batch visits = %d, want 2", len(b.Visits))
	}

	got, ok := s.Get(b.ID)
	if !ok {
		t.Fatal("Get(newly-put batch) returned ok=false")
	}
	if got.HospitalCode != "12345" || got.Period != "202504" {
		t.Errorf("Get mismatch: %+v", got)
	}
	if len(got.Visits) != 2 {
		t.Errorf("Get visits = %d, want 2", len(got.Visits))
	}
	// visit ordering must be preserved
	if got.Visits[0].VN != "VN-A" || got.Visits[1].VN != "VN-B" {
		t.Errorf("visit order mismatch: %+v", got.Visits)
	}

	if err := s.SetState(b.ID, batch.StateCompleted, ""); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	got, _ = s.Get(b.ID)
	if got.State != batch.StateCompleted {
		t.Errorf("state after SetState = %s", got.State)
	}

	if err := s.SetState("MISSING", batch.StateCompleted, ""); err == nil {
		t.Error("SetState on missing id should error")
	}

	list := s.List()
	found := false
	for _, x := range list {
		if x.ID == b.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("List did not include the put batch")
	}
}

func TestBatchStore_Memory(t *testing.T) {
	contractTest(t, batch.NewMemory())
}

func TestBatchStore_Postgres(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping PgStore contract test")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Start clean so repeated runs don't collide.
	if _, err := conn.Exec(`TRUNCATE opd_ingest_visit, opd_ingest_batch CASCADE`); err != nil {
		t.Fatalf("truncate: %v — is migration 004 applied?", err)
	}

	contractTest(t, batch.NewPostgres(conn))
}
