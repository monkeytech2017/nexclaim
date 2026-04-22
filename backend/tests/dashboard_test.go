package tests

import (
	"context"
	"encoding/json"
	"math"
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

// In-memory DashboardRepo fake for handler-shape test.
type memDashboardRepo struct {
	mu    sync.Mutex
	stats store.DashboardStats
	last  store.DashboardFilter
	err   error
}

func (m *memDashboardRepo) Stats(_ context.Context, f store.DashboardFilter) (*store.DashboardStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = f
	if m.err != nil {
		return nil, m.err
	}
	s := m.stats
	return &s, nil
}

func TestDashboardAPI_Shape(t *testing.T) {
	repo := &memDashboardRepo{stats: store.DashboardStats{
		Summary: store.Summary{
			BatchesTotal: 5, RecordsTotal: 25, RecordsErrors: 2,
			CCodesOpen: 1, AvgSendMs: 180, SendSuccessRate: 0.8,
		},
		ByFormat: []store.FormatCount{
			{Format: "16FILES", Count: 3, Records: 15},
			{Format: "CIPN", Count: 2, Records: 10},
		},
		ByStatus: []store.StatusCount{
			{Status: "sent", Count: 4},
			{Status: "error", Count: 1},
		},
		ByDay: []store.DayBucket{
			{Day: "2026-04-22", Formats: map[string]int{"16FILES": 1, "CIPN": 2}},
			{Day: "2026-04-23", Formats: map[string]int{"16FILES": 2}},
		},
		TopCCodes: []store.CCodeCount{
			{CCode: "C104", Count: 3, DescSample: "PERSON_ID ผิด"},
		},
	}}

	h := server.New(server.Deps{DashboardRepo: repo})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/dashboard/stats?hcode=12345&period_from=202504&period_to=202604", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	// Assert the fake received the filter verbatim.
	if repo.last.HCode != "12345" || repo.last.PeriodFrom != "202504" || repo.last.PeriodTo != "202604" {
		t.Errorf("filter not forwarded: %+v", repo.last)
	}

	var got store.DashboardStats
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Summary.BatchesTotal != 5 || got.Summary.RecordsTotal != 25 {
		t.Errorf("summary = %+v", got.Summary)
	}
	if got.Summary.AvgSendMs != 180 || got.Summary.SendSuccessRate != 0.8 {
		t.Errorf("send summary = %+v", got.Summary)
	}
	if len(got.ByFormat) != 2 || got.ByFormat[0].Format != "16FILES" {
		t.Errorf("by_format = %+v", got.ByFormat)
	}
	if len(got.ByStatus) != 2 || got.ByStatus[0].Status != "sent" {
		t.Errorf("by_status = %+v", got.ByStatus)
	}
	if len(got.ByDay) != 2 {
		t.Errorf("by_day = %+v", got.ByDay)
	}
	if n := got.ByDay[0].Formats["CIPN"]; n != 2 {
		t.Errorf("by_day[0].CIPN = %d", n)
	}
	if len(got.TopCCodes) != 1 || got.TopCCodes[0].CCode != "C104" {
		t.Errorf("top_ccodes = %+v", got.TopCCodes)
	}

	// Raw-JSON key check — guards against struct-tag rot (snake_case contract).
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	for _, k := range []string{"summary", "by_format", "by_status", "by_day", "top_ccodes"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing top-level key %q in response", k)
		}
	}
	var summary map[string]json.RawMessage
	_ = json.Unmarshal(raw["summary"], &summary)
	for _, k := range []string{
		"batches_total", "records_total", "records_errors",
		"ccodes_open", "avg_send_ms", "send_success_rate",
	} {
		if _, ok := summary[k]; !ok {
			t.Errorf("missing summary key %q", k)
		}
	}
}

func TestDashboardAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/stats", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

// Pg integration — seed via ClaimRepo.SaveRun (the same write path prod uses)
// then read via DashboardRepo.Stats. Uses a unique period ('203096') so we
// don't interfere with other Pg tests and can DELETE our own rows cleanly.
func TestDashboardRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	const period = "203096"

	// Cleanup order: child → parent (role is CRUD-only, no TRUNCATE).
	cleanup := []string{
		`DELETE FROM send_log     WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period=$1)`,
		`DELETE FROM c_code_log   WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period=$1)`,
		`DELETE FROM claim_record WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period=$1)`,
		`DELETE FROM claim_batch  WHERE period=$1`,
	}
	for _, q := range cleanup {
		if _, err := conn.Exec(q, period); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	// Seed hospital (FK target) before claim_batch insert.
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Two claim_batch rows (different formats): one UCS 16FILES, one CSMBS CIPN.
	saver := store.NewPg(conn)
	out1 := &pipeline.Outcome{
		INSCL:    model.INSCL_UCS,
		OPDCount: 1,
		OPD: []model.OPDVisit{{
			Patient: model.Patient{HN: "HN-D-1", PersonID: makeThaiID("110010100127"), INSCL: model.INSCL_UCS},
			SEQ:     "SEQ-D-1", DateOPD: fxDate(), UUC: "1",
		}},
		Submissions: []pipeline.Submission{{
			Format: model.Format16Files, ZipName: "12345UCS" + period + ".ZIP",
			ZipBytes: []byte("x"), TxnID: "TXN-D-1",
			Attempt: &pipeline.Attempt{
				Endpoint: "/api/claim/16files", DurationMs: 100,
				Success: true, Response: "ok", SentAt: time.Now(),
			},
		}},
	}
	if err := saver.SaveRun(ctx, store.SaveRequest{HCode: "12345", Period: period, Outcome: out1}); err != nil {
		t.Fatalf("SaveRun #1: %v", err)
	}

	out2 := &pipeline.Outcome{
		INSCL:    model.INSCL_CSMBS,
		IPDCount: 1,
		IPD: []model.IPDAdmit{{
			Patient: model.Patient{HN: "HN-D-2", PersonID: makeThaiID("110010100127"), INSCL: model.INSCL_CSMBS},
			AN:      "A-D-2", DateAdm: fxDate(), DateDsc: fxDate(), UUC: "1",
		}},
		Submissions: []pipeline.Submission{{
			Format: model.FormatCIPN, ZipName: "12345CIPN" + period + ".ZIP",
			ZipBytes: []byte("x"), TxnID: "TXN-D-2",
			Attempt: &pipeline.Attempt{
				Endpoint: "/api/claim/cipn", DurationMs: 300,
				Success: true, Response: "ok", SentAt: time.Now(),
			},
		}},
	}
	if err := saver.SaveRun(ctx, store.SaveRequest{HCode: "12345", Period: period, Outcome: out2}); err != nil {
		t.Fatalf("SaveRun #2: %v", err)
	}

	// Pull out batch IDs for direct send_log and c_code inserts.
	type bRow struct {
		BatchID string `db:"batch_id"`
		Format  string `db:"format"`
	}
	var brows []bRow
	if err := conn.Select(&brows, `SELECT batch_id::text AS batch_id, format FROM claim_batch WHERE period=$1 ORDER BY format`, period); err != nil {
		t.Fatalf("list batches: %v", err)
	}
	if len(brows) != 2 {
		t.Fatalf("want 2 batches, got %d", len(brows))
	}
	// Add a failed send_log attempt to batch #1 so success rate = 2 successes / 3 total,
	// then override to match the 0.5 ask: add another failure. We want 2 success, 2 fail → 0.5.
	// SaveRun above inserted 2 successful send_log rows. Add 2 failures.
	for _, b := range brows {
		if _, err := conn.Exec(`
			INSERT INTO send_log (batch_id, attempt_no, endpoint, duration_ms, success, error_msg, sent_at)
			VALUES ($1::uuid, 2, $2, 500, false, 'retry later', now())`, b.BatchID, "/api/claim/retry"); err != nil {
			t.Fatalf("seed send_log fail: %v", err)
		}
	}

	// One c_code_log row on the first batch.
	ccode := store.NewPgCCodeRepo(conn)
	if _, err := ccode.Insert(ctx, store.CCodeInsert{
		BatchID: brows[0].BatchID, HN: "HN-D-1", ANOrSEQ: "SEQ-D-1",
		CCode: "C104", CDesc: "ทดสอบ PERSON_ID ผิด",
	}); err != nil {
		t.Fatalf("insert c_code: %v", err)
	}

	// Now query DashboardRepo, filtered to the unique period (isolates other seed data).
	dashRepo := store.NewPgDashboardRepo(conn)
	stats, err := dashRepo.Stats(ctx, store.DashboardFilter{
		HCode:      "12345",
		PeriodFrom: period,
		PeriodTo:   period,
	})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if stats.Summary.BatchesTotal != 2 {
		t.Errorf("batches_total = %d, want 2", stats.Summary.BatchesTotal)
	}
	if stats.Summary.CCodesOpen != 1 {
		t.Errorf("ccodes_open = %d, want 1", stats.Summary.CCodesOpen)
	}
	// 2 successful (from SaveRun, 100 + 300) + 2 failed (retry) = 4 total, 0.5 rate.
	if math.Abs(stats.Summary.SendSuccessRate-0.5) > 0.01 {
		t.Errorf("send_success_rate = %v, want ≈0.5", stats.Summary.SendSuccessRate)
	}
	// Avg of successful = (100+300)/2 = 200.
	if stats.Summary.AvgSendMs != 200 {
		t.Errorf("avg_send_ms = %d, want 200", stats.Summary.AvgSendMs)
	}

	if len(stats.ByFormat) != 2 {
		t.Errorf("by_format len = %d, want 2 (got %+v)", len(stats.ByFormat), stats.ByFormat)
	}
	seen := map[string]bool{}
	for _, fc := range stats.ByFormat {
		seen[fc.Format] = true
	}
	if !seen["16FILES"] || !seen["CIPN"] {
		t.Errorf("by_format missing formats: %+v", stats.ByFormat)
	}

	if len(stats.TopCCodes) != 1 || stats.TopCCodes[0].CCode != "C104" {
		t.Errorf("top_ccodes = %+v", stats.TopCCodes)
	}

	// Cleanup
	for _, q := range cleanup {
		if _, err := conn.Exec(q, period); err != nil {
			t.Fatalf("teardown: %v", err)
		}
	}
}
