package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

// In-memory fake for handler tests.
type memClaimBatchRepo struct {
	mu   sync.Mutex
	rows []store.ClaimBatchRow
}

func newMemClaimBatchRepo(seeded ...store.ClaimBatchRow) *memClaimBatchRepo {
	return &memClaimBatchRepo{rows: append([]store.ClaimBatchRow{}, seeded...)}
}

func (m *memClaimBatchRepo) List(_ context.Context, f store.ClaimBatchFilter) ([]store.ClaimBatchRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []store.ClaimBatchRow{}
	for _, r := range m.rows {
		if f.HCode != "" && r.HCode != f.HCode {
			continue
		}
		if f.Period != "" && r.Period != f.Period {
			continue
		}
		if f.INSCL != "" && r.INSCL != f.INSCL {
			continue
		}
		if f.Format != "" && r.Format != f.Format {
			continue
		}
		if f.Status != "" && r.Status != f.Status {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *memClaimBatchRepo) Get(_ context.Context, id string) (*store.ClaimBatchRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.BatchID == id {
			return &r, nil
		}
	}
	return nil, store.ErrNotFound
}

func TestClaimBatchAPI_ListFilter(t *testing.T) {
	repo := newMemClaimBatchRepo(
		store.ClaimBatchRow{BatchID: "B1", HCode: "12345", Period: "202504", INSCL: "UCS", Format: "16FILES", Status: "sent"},
		store.ClaimBatchRow{BatchID: "B2", HCode: "12345", Period: "202504", INSCL: "011", Format: "CIPN", Status: "pending"},
		store.ClaimBatchRow{BatchID: "B3", HCode: "99999", Period: "202504", INSCL: "UCS", Format: "16FILES", Status: "sent"},
	)
	h := server.New(server.Deps{ClaimBatchRepo: repo})

	// List all
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var list struct{ Items []store.ClaimBatchRow }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 3 {
		t.Errorf("all = %d", len(list.Items))
	}

	// Filter by hcode
	req = httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches?hcode=12345", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 2 {
		t.Errorf("by hcode = %d", len(list.Items))
	}

	// Filter by status=sent
	req = httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches?status=sent", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 2 {
		t.Errorf("by status = %d", len(list.Items))
	}

	// Get one
	req = httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches/B2", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var row store.ClaimBatchRow
	_ = json.Unmarshal(w.Body.Bytes(), &row)
	if row.INSCL != "011" {
		t.Errorf("get B2 = %+v", row)
	}

	// Not found
	req = httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches/NOPE", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("not found = %d", w.Code)
	}
}

func TestClaimBatchAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

// Pg integration — seed via repo.SaveRun (same path production uses).
func TestClaimBatchRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Reset the subset we'll insert
	for _, q := range []string{
		`DELETE FROM c_code_log   WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203098')`,
		`DELETE FROM claim_record WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203098')`,
		`DELETE FROM claim_batch  WHERE period='203098'`,
	} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	// Use ClaimRepo.SaveRun to create a row the normal way.
	saver := store.NewPg(conn)
	outcome := &pipeline.Outcome{
		INSCL:    model.INSCL_UCS,
		OPDCount: 1,
		OPD: []model.OPDVisit{{
			Patient: model.Patient{HN: "HN-CB-1", PersonID: makeThaiID("110010100127"), INSCL: model.INSCL_UCS},
			SEQ: "SEQ-CB-1", DateOPD: fxDate(), UUC: "1",
		}},
		Submissions: []pipeline.Submission{
			{Format: model.Format16Files, ZipName: "12345UCS203098.ZIP", ZipBytes: []byte("x"), TxnID: "TXN-CB-1"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := saver.SaveRun(ctx, store.SaveRequest{HCode: "12345", Period: "203098", Outcome: outcome}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	// Read via ClaimBatchRepo
	repo := store.NewPgClaimBatchRepo(conn)
	rows, err := repo.List(ctx, store.ClaimBatchFilter{HCode: "12345", Period: "203098"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.Format != "16FILES" || got.Sender != "FDH" || got.Status != "sent" {
		t.Errorf("row = %+v", got)
	}
	if got.FDHTxnID != "TXN-CB-1" || got.ZipFilename != "12345UCS203098.ZIP" {
		t.Errorf("txn/zip mismatch: %+v", got)
	}
	if got.SentAtJSON == nil {
		t.Error("sent_at should be populated for 'sent' status")
	}

	// Also Get works
	one, err := repo.Get(ctx, got.BatchID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if one.BatchID != got.BatchID {
		t.Errorf("get mismatch")
	}

	// Insert a c-code linked to that batch → counts reflect in list
	ccode := store.NewPgCCodeRepo(conn)
	if _, err := ccode.Insert(ctx, store.CCodeInsert{
		BatchID: got.BatchID, HN: "HN-CB-1", ANOrSEQ: "SEQ-CB-1",
		CCode: "C104", CDesc: "ทดสอบ",
	}); err != nil {
		t.Fatalf("insert ccode: %v", err)
	}
	rows2, _ := repo.List(ctx, store.ClaimBatchFilter{HCode: "12345", Period: "203098"})
	if rows2[0].CCodeCount != 1 || rows2[0].CCodeOpen != 1 {
		t.Errorf("c-code counts should reflect: %+v", rows2[0])
	}

	// Cleanup
	_, _ = conn.Exec(`DELETE FROM c_code_log   WHERE batch_id = $1::uuid`, got.BatchID)
	_, _ = conn.Exec(`DELETE FROM claim_record WHERE batch_id = $1::uuid`, got.BatchID)
	_, _ = conn.Exec(`DELETE FROM claim_batch  WHERE batch_id = $1::uuid`, got.BatchID)
}
