package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

// ── in-memory CCodeRepo fake (for handler tests) ──

type memCCodeRepo struct {
	mu      sync.Mutex
	rows    map[string]store.CCodeRow
	nextIdx int
}

func newMemCCodeRepo() *memCCodeRepo {
	return &memCCodeRepo{rows: make(map[string]store.CCodeRow)}
}

func (m *memCCodeRepo) Insert(_ context.Context, row store.CCodeInsert) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextIdx++
	id := "ID-" + string(rune('A'+m.nextIdx))
	m.rows[id] = store.CCodeRow{
		ID: id, BatchID: row.BatchID, RecordID: row.RecordID,
		HN: row.HN, ANOrSEQ: row.ANOrSEQ, CCode: row.CCode,
		CDesc: row.CDesc, FieldName: row.FieldName, FieldValue: row.FieldValue,
		ReceivedAt: time.Now(),
	}
	return id, nil
}

func (m *memCCodeRepo) List(_ context.Context, f store.CCodeFilter) ([]store.CCodeRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.CCodeRow, 0, len(m.rows))
	for _, r := range m.rows {
		if f.BatchID != "" && r.BatchID != f.BatchID {
			continue
		}
		if f.Resolved != nil && r.Resolved != *f.Resolved {
			continue
		}
		if f.CCode != "" && r.CCode != f.CCode {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (m *memCCodeRepo) Resolve(_ context.Context, id, by string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.rows[id]
	if !ok || r.Resolved {
		return store.ErrNotFound
	}
	r.Resolved = true
	r.ResolvedBy = by
	t := time.Now()
	r.ResolvedAtJSON = &t
	m.rows[id] = r
	return nil
}

func (m *memCCodeRepo) LookupClaimRecord(_ context.Context, _, _, _, _, _ string) (string, string) {
	return "", "" // fake: never match, caller falls back
}

// ── handler tests ──

func TestCCodeAPI_ListAndResolve(t *testing.T) {
	repo := newMemCCodeRepo()
	// seed one row
	_, _ = repo.Insert(context.Background(), store.CCodeInsert{
		BatchID: "B1", HN: "HN001", ANOrSEQ: "S001",
		CCode: "C104", CDesc: "PERSON_ID ผิด",
		FieldName: "pid", FieldValue: "1234567890123",
	})

	h := server.New(server.Deps{CCodeRepo: repo})

	// List (default — all)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ccodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var list struct{ Items []store.CCodeRow }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(list.Items))
	}
	if list.Items[0].CCode != "C104" {
		t.Errorf("c_code = %s", list.Items[0].CCode)
	}

	// Filter by resolved=false — should find
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ccodes?resolved=false", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Errorf("resolved=false want 1, got %d", len(list.Items))
	}
	id := list.Items[0].ID

	// Resolve
	body, _ := json.Marshal(map[string]string{"resolved_by": "operator@nexclaim"})
	preq := httptest.NewRequest(http.MethodPatch, "/api/v1/ccodes/"+id+"/resolve", bytes.NewReader(body))
	preq.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, preq)
	if w.Code != http.StatusNoContent {
		t.Errorf("resolve want 204, got %d", w.Code)
	}

	// Filter resolved=true — now 1
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ccodes?resolved=true", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 || !list.Items[0].Resolved {
		t.Errorf("after resolve, want 1 resolved, got %+v", list.Items)
	}

	// Resolve again — should 404 (already resolved)
	preq = httptest.NewRequest(http.MethodPatch, "/api/v1/ccodes/"+id+"/resolve", bytes.NewReader(body))
	preq.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, preq)
	if w.Code != http.StatusNotFound {
		t.Errorf("double-resolve want 404, got %d", w.Code)
	}
}

func TestCCodeAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ccodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

// ── REP ingest (Pg integration) ──

func TestREPIngester_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Clean + seed a claim_batch + claim_record so lookup succeeds
	for _, q := range []string{
		`DELETE FROM c_code_log WHERE hn LIKE 'HN-REP-%'`,
		`DELETE FROM claim_record WHERE hn LIKE 'HN-REP-%'`,
		`DELETE FROM claim_batch  WHERE period = '203099' AND hcode = '12345'`,
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

	// Use pipeline to make a claim_batch + claim_record the normal way.
	repo := store.NewPg(conn)
	batchIDs := []string{}
	outcome := &pipeline.Outcome{
		INSCL:    model.INSCL_UCS,
		OPDCount: 1,
		OPD: []model.OPDVisit{{
			Patient: model.Patient{HN: "HN-REP-1", PersonID: makeThaiID("110010100127"), INSCL: model.INSCL_UCS},
			SEQ: "SEQ-REP-1", DateOPD: fxDate(), UUC: "1",
		}},
		Submissions: []pipeline.Submission{
			{Format: model.Format16Files, ZipName: "12345UCS203099.ZIP", ZipBytes: []byte("x"), TxnID: "TXN-REP-1"},
		},
	}
	if err := repo.SaveRun(context.Background(), store.SaveRequest{
		HCode: "12345", Period: "203099", Outcome: outcome,
	}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	_ = conn.Select(&batchIDs, `SELECT batch_id::text FROM claim_batch WHERE period='203099' AND hcode='12345'`)

	// Fake REP XML with one matching + one missing record
	repXML := `<REP>
		<ITEM><CCODE>C104</CCODE><CDESC>PERSON_ID ผิด</CDESC><HN>HN-REP-1</HN><SEQ>SEQ-REP-1</SEQ><FIELDNAME>pid</FIELDNAME><FIELDVALUE>bad</FIELDVALUE></ITEM>
		<ITEM><CCODE>C115</CCODE><CDESC>UUC ผิด</CDESC><HN>HN-UNKNOWN</HN><SEQ>SEQ-UNKNOWN</SEQ></ITEM>
	</REP>`
	ing := store.NewREPIngester(conn, store.NewPgCCodeRepo(conn), stubFetcher{data: repXML})

	res, err := ing.Ingest(context.Background(), "12345", "203099")
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if res.Fetched != 2 || res.Inserted != 2 {
		t.Errorf("fetched=%d inserted=%d (want 2/2 — unmatched row falls back to latest batch)", res.Fetched, res.Inserted)
	}

	// Verify they persist + the first row linked to the right batch via hn/seq
	var n int
	_ = conn.Get(&n, `SELECT count(*) FROM c_code_log WHERE hn LIKE 'HN-REP-%' OR hn = 'HN-UNKNOWN'`)
	if n != 2 {
		t.Errorf("c_code_log rows = %d, want 2", n)
	}

	var matchedRecordID string
	_ = conn.Get(&matchedRecordID, `
		SELECT COALESCE(record_id::text, '') FROM c_code_log WHERE hn = 'HN-REP-1'
	`)
	if matchedRecordID == "" {
		t.Error("HN-REP-1 c-code should link to claim_record via LookupClaimRecord")
	}

	// cleanup
	_, _ = conn.Exec(`DELETE FROM c_code_log WHERE hn LIKE 'HN-REP-%' OR hn = 'HN-UNKNOWN'`)
	_, _ = conn.Exec(`DELETE FROM claim_record WHERE hn LIKE 'HN-REP-%'`)
	_, _ = conn.Exec(`DELETE FROM claim_batch WHERE period = '203099' AND hcode = '12345'`)
}

type stubFetcher struct{ data string }

func (s stubFetcher) GetREP(_ string) ([]byte, error) {
	return []byte(strings.TrimSpace(s.data)), nil
}
