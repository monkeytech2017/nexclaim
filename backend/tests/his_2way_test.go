package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/server"
)

// hisTestServer spins up a fake HIS that returns visitDetailFixture for any
// visit id, overriding INSCL if a mapping is provided.
func hisTestServer(t *testing.T, inscls map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/nexclaim/opd/visit/") {
			http.NotFound(w, r)
			return
		}
		vn := strings.TrimPrefix(r.URL.Path, "/api/nexclaim/opd/visit/")
		d := visitDetailFixture()
		d.Visit.VN = vn
		d.Visit.SEQ = "SEQ-" + vn
		if code, ok := inscls[vn]; ok {
			d.Insurance.INSCL = code
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(d)
	}))
}

func TestServer_ReceiveVisits_OK(t *testing.T) {
	batches := batch.New()
	h := server.New(server.Deps{HCode: "12345", Batches: batches})

	body := hisclient.VisitListRequest{
		HospitalCode: "12345",
		Period:       "202504",
		ExportedBy:   "unit-test",
		Visits: []hisclient.VisitSummary{
			{VN: "VN1", HN: "HN1", PID: makeThaiID("110010100127"), VisitDate: "20250418", INSCL: "UCS"},
			{VN: "VN2", HN: "HN2", PID: makeThaiID("110010100127"), VisitDate: "20250418", INSCL: "011"},
		},
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/opd/visits", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp hisclient.VisitListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "OK" || resp.BatchID == "" || resp.ReceivedCount != 2 {
		t.Errorf("resp = %+v", resp)
	}
	// batch must be in store
	if _, ok := batches.Get(resp.BatchID); !ok {
		t.Error("batch not persisted")
	}
}

func TestServer_ReceiveVisits_BadPID(t *testing.T) {
	batches := batch.New()
	h := server.New(server.Deps{HCode: "12345", Batches: batches})

	body := map[string]any{
		"hospital_code": "12345", "period": "202504",
		"visits": []map[string]any{
			{"vn": "VN1", "hn": "HN1", "pid": "1234567890123", "visit_date": "20250418", "inscl": "UCS"},
		},
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/opd/visits", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	var ve hisclient.VisitListError
	_ = json.Unmarshal(w.Body.Bytes(), &ve)
	if ve.Error != "VALIDATION_ERROR" || len(ve.Errors) == 0 {
		t.Errorf("expected VALIDATION_ERROR, got %+v", ve)
	}
}

func TestServer_ReceiveVisits_MissingFields(t *testing.T) {
	batches := batch.New()
	h := server.New(server.Deps{HCode: "12345", Batches: batches})

	body := map[string]any{
		"hospital_code": "12345", "period": "202504",
		"visits": []map[string]any{
			{"vn": "VN1", "pid": makeThaiID("110010100127"), "visit_date": "20250418", "inscl": "UCS"},
		},
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/opd/visits", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing HN, got %d", w.Code)
	}
}

func TestServer_GetBatch(t *testing.T) {
	batches := batch.New()
	b := batches.Put(hisclient.VisitListRequest{
		HospitalCode: "12345", Period: "202504",
		Visits: []hisclient.VisitSummary{{VN: "V1", HN: "H1"}},
	})
	h := server.New(server.Deps{Batches: batches})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/his/opd/batches/"+b.ID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got batch.Batch
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.ID != b.ID || got.State != batch.StateReceived {
		t.Errorf("got %+v", got)
	}
}

func TestServer_GetBatch_NotFound(t *testing.T) {
	h := server.New(server.Deps{Batches: batch.New()})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/his/opd/batches/MISSING", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestServer_ProcessBatch_DryRun(t *testing.T) {
	// fake HIS returning CSMBS (011) for VN1, UCS for VN2
	his := hisTestServer(t, map[string]string{"VN1": "011", "VN2": "UCS"})
	defer his.Close()

	batches := batch.New()
	b := batches.Put(hisclient.VisitListRequest{
		HospitalCode: "12345", Period: "202504",
		Visits: []hisclient.VisitSummary{
			{VN: "VN1", HN: "HN1", PID: makeThaiID("110010100127"), INSCL: "011"},
			{VN: "VN2", HN: "HN2", PID: makeThaiID("110010100127"), INSCL: "UCS"},
		},
	})
	fdh := &fakeFDH{}
	h := server.New(server.Deps{
		HCode: "12345", Batches: batches,
		HISClient: hisclient.New(his.URL),
		FDH:       fdh, Extractor: extractor.NewMemoryExtractor(),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/opd/batches/"+b.ID+"/process?dry_run=true", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if fdh.calls16+fdh.callsCIPN+fdh.callsCSOP != 0 {
		t.Error("dry-run should not submit")
	}

	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	runs, _ := resp["runs"].([]any)
	if len(runs) != 2 {
		t.Errorf("want 2 runs (011 + UCS), got %d", len(runs))
	}
}

func TestServer_ProcessBatch_LiveSubmits(t *testing.T) {
	his := hisTestServer(t, map[string]string{"VN1": "UCS"})
	defer his.Close()

	batches := batch.New()
	b := batches.Put(hisclient.VisitListRequest{
		HospitalCode: "12345", Period: "202504",
		Visits: []hisclient.VisitSummary{
			{VN: "VN1", HN: "HN1", PID: makeThaiID("110010100127"), INSCL: "UCS"},
		},
	})
	fdh := &fakeFDH{}
	h := server.New(server.Deps{
		HCode: "12345", Batches: batches,
		HISClient: hisclient.New(his.URL),
		FDH:       fdh,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/opd/batches/"+b.ID+"/process", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if fdh.calls16 != 1 {
		t.Errorf("want 1 Send16Files call, got %d", fdh.calls16)
	}
}

func TestServer_ProcessBatch_HISMissing(t *testing.T) {
	batches := batch.New()
	h := server.New(server.Deps{Batches: batches})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/opd/batches/BATCH-X/process", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 when HISClient missing, got %d", w.Code)
	}
}
