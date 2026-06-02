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

// In-memory SendLogRepo for handler tests.
type memSendLogRepo struct {
	mu   sync.Mutex
	rows []store.SendLogRow
}

func (m *memSendLogRepo) List(_ context.Context, f store.SendLogFilter) ([]store.SendLogRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []store.SendLogRow{}
	for _, r := range m.rows {
		if f.BatchID != "" && r.BatchID != f.BatchID {
			continue
		}
		if f.HCode != "" && r.HCode != f.HCode {
			continue
		}
		if f.Period != "" && r.Period != f.Period {
			continue
		}
		if f.Success == "true" && (r.SuccessJS == nil || !*r.SuccessJS) {
			continue
		}
		if f.Success == "false" && (r.SuccessJS == nil || *r.SuccessJS) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func TestSendLogAPI_ListFilter(t *testing.T) {
	trueV, falseV := true, false
	repo := &memSendLogRepo{rows: []store.SendLogRow{
		{ID: "1", BatchID: "B1", HCode: "12345", Period: "202504", Endpoint: "/api/claim/16files", SuccessJS: &trueV},
		{ID: "2", BatchID: "B2", HCode: "12345", Period: "202504", Endpoint: "/api/claim/cipn", SuccessJS: &falseV},
		{ID: "3", BatchID: "B3", HCode: "99999", Period: "202504", Endpoint: "/aipnupload/", SuccessJS: &trueV},
	}}
	h := server.New(server.Deps{SendLogRepo: repo})

	get := func(url string) []store.SendLogRow {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var body struct{ Items []store.SendLogRow }
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return body.Items
	}

	if got := get("/api/v1/send-logs"); len(got) != 3 {
		t.Errorf("all = %d", len(got))
	}
	if got := get("/api/v1/send-logs?hcode=12345"); len(got) != 2 {
		t.Errorf("by hcode = %d", len(got))
	}
	if got := get("/api/v1/send-logs?success=false"); len(got) != 1 {
		t.Errorf("by success=false = %d", len(got))
	}
	if got := get("/api/v1/send-logs?batch_id=B2"); len(got) != 1 {
		t.Errorf("by batch_id = %d", len(got))
	}
}

func TestSendLogAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/send-logs", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

// Pg integration — SaveRun writes send_log when Attempt is populated.
func TestSendLogRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Reset the period we'll use.
	for _, q := range []string{
		`DELETE FROM send_log     WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203097')`,
		`DELETE FROM c_code_log   WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203097')`,
		`DELETE FROM claim_record WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203097')`,
		`DELETE FROM claim_batch  WHERE period='203097'`,
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

	saver := store.NewPg(conn)
	outcome := &pipeline.Outcome{
		INSCL:    model.INSCL_UCS,
		OPDCount: 1,
		OPD: []model.OPDVisit{{
			Patient: model.Patient{HN: "HN-SL-1", PersonID: makeThaiID("110010100127"), INSCL: model.INSCL_UCS},
			SEQ: "SEQ-SL-1", DateOPD: fxDate(), UUC: "1",
		}},
		Submissions: []pipeline.Submission{
			{
				Format: model.Format16Files, ZipName: "12345UCS203097.ZIP",
				ZipBytes: []byte("x"), TxnID: "TXN-SL-1",
				Attempt: &pipeline.Attempt{
					Endpoint: "/api/claim/16files", DurationMs: 123,
					Success: true, Response: "txnId=TXN-SL-1", SentAt: time.Now(),
				},
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := saver.SaveRun(ctx, store.SaveRequest{HCode: "12345", Period: "203097", Outcome: outcome}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	// Find the batch we just inserted.
	var batchID string
	if err := conn.Get(&batchID, `SELECT batch_id::text FROM claim_batch WHERE period='203097' LIMIT 1`); err != nil {
		t.Fatalf("find batch: %v", err)
	}

	repo := store.NewPgSendLogRepo(conn)
	rows, err := repo.List(ctx, store.SendLogFilter{BatchID: batchID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 send_log row, got %d", len(rows))
	}
	r := rows[0]
	if r.Endpoint != "/api/claim/16files" {
		t.Errorf("endpoint = %q", r.Endpoint)
	}
	if r.SuccessJS == nil || !*r.SuccessJS {
		t.Errorf("success = %v", r.SuccessJS)
	}
	if !r.FDHTxnID.Valid || r.FDHTxnID.String != "TXN-SL-1" {
		t.Errorf("txn = %+v", r.FDHTxnID)
	}
	if !r.DurationMs.Valid || r.DurationMs.Int64 != 123 {
		t.Errorf("duration = %+v", r.DurationMs)
	}
	if r.HCode != "12345" || r.Period != "203097" {
		t.Errorf("joined batch fields wrong: %+v", r)
	}

	// Filter by success=true should include this row.
	rowsT, _ := repo.List(ctx, store.SendLogFilter{BatchID: batchID, Success: "true"})
	if len(rowsT) != 1 {
		t.Errorf("success=true filter: want 1, got %d", len(rowsT))
	}
	rowsF, _ := repo.List(ctx, store.SendLogFilter{BatchID: batchID, Success: "false"})
	if len(rowsF) != 0 {
		t.Errorf("success=false filter: want 0, got %d", len(rowsF))
	}

	// Cleanup
	_, _ = conn.Exec(`DELETE FROM send_log     WHERE batch_id = $1::uuid`, batchID)
	_, _ = conn.Exec(`DELETE FROM claim_record WHERE batch_id = $1::uuid`, batchID)
	_, _ = conn.Exec(`DELETE FROM claim_batch  WHERE batch_id = $1::uuid`, batchID)
}
