package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/server"
)

type fakeStatus struct{ called int }

func (f *fakeStatus) GetStatus(txnID string) (*sender.SubmitResult, error) {
	f.called++
	return &sender.SubmitResult{TxnID: txnID, Status: "PROCESSED", Message: "ok"}, nil
}

func newTestServer(t *testing.T, withData bool) (http.Handler, *fakeFDH, *fakeCHI, *fakeStatus) {
	extr := extractor.NewMemoryExtractor()
	if withData {
		extr.Put("202504", model.INSCL_UCS, extractor.Result{
			OPD: []model.OPDVisit{sampleUCSVisit()},
		})
	}
	fdh := &fakeFDH{}
	chi := &fakeCHI{}
	st := &fakeStatus{}
	return server.New(server.Deps{
		HCode: "12345", Extractor: extr, FDH: fdh, CHI: chi, StatusLookup: st,
	}), fdh, chi, st
}

func TestServer_Healthz(t *testing.T) {
	h, _, _, _ := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["ok"] != true {
		t.Errorf("body = %v", body)
	}
}

func TestServer_Submit_UCS_DryRun(t *testing.T) {
	h, fdh, _, _ := newTestServer(t, true)

	payload := map[string]any{
		"inscl": "UCS", "period": "202504", "dryRun": true,
	}
	buf, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if fdh.calls16 != 0 {
		t.Error("dry-run should not call FDH")
	}
	var resp server.SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.INSCL != "UCS" || resp.OPDCount != 1 {
		t.Errorf("resp = %+v", resp)
	}
	if len(resp.Submissions) != 1 || resp.Submissions[0].Format != "16FILES" {
		t.Errorf("submissions = %+v", resp.Submissions)
	}
	if resp.Submissions[0].FilesN == 0 || resp.Submissions[0].ZipBytes == 0 {
		t.Errorf("expected files + zip bytes; got %+v", resp.Submissions[0])
	}
}

func TestServer_Submit_UCS_LiveCallsFDH(t *testing.T) {
	h, fdh, _, _ := newTestServer(t, true)

	buf, _ := json.Marshal(map[string]any{"inscl": "UCS", "period": "202504"})
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if fdh.calls16 != 1 {
		t.Errorf("want 1 Send16Files call, got %d", fdh.calls16)
	}
	var resp server.SubmitResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Submissions[0].TxnID == "" {
		t.Error("expected TxnID in response")
	}
}

func TestServer_Submit_InvalidBody(t *testing.T) {
	h, _, _, _ := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestServer_Status(t *testing.T) {
	h, _, _, st := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/status/TXN-123", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if st.called != 1 {
		t.Errorf("want 1 GetStatus call, got %d", st.called)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["txnId"] != "TXN-123" || body["status"] != "PROCESSED" {
		t.Errorf("body = %v", body)
	}
}

func TestServer_Submit_SSOViaCHI(t *testing.T) {
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_SSS, extractor.Result{
		OPD: []model.OPDVisit{sampleSSOOPD()},
	})
	fdh := &fakeFDH{}
	chi := &fakeCHI{}
	h := server.New(server.Deps{
		HCode: "12345", Extractor: extr, FDH: fdh, CHI: chi,
	})

	buf, _ := json.Marshal(map[string]any{"inscl": "SSS", "period": "202504"})
	req := httptest.NewRequest(http.MethodPost, "/api/submit", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if chi.callsSSOP != 1 {
		t.Errorf("want 1 SendSSOP call, got %d", chi.callsSSOP)
	}
	if fdh.calls16+fdh.callsCIPN+fdh.callsCSOP != 0 {
		t.Error("FDH should not be called for SSO")
	}
}
