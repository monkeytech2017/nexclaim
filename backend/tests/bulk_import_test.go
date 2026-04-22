package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

func TestBulkDrugMaps(t *testing.T) {
	repo := newMemDrugMapRepo()
	h := server.New(server.Deps{DrugMapRepo: repo})

	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"hcode": "12345", "his_drug_code": "D001", "tmt_code": "100000000000000000000001", "is_active": true},
			{"hcode": "12345", "his_drug_code": "D002", "tmt_code": "100000000000000000000002", "is_active": true},
			{"hcode": "12345", "his_drug_code": "D003", "tmt_code": "TOOSHORT", "is_active": true}, // bad — 8 chars
			{"hcode": "XXX", "his_drug_code": "D004", "is_active": true},                          // bad — hcode != 5 chars
			{"hcode": "12345", "his_drug_code": "D005", "is_active": true},                        // ok — no TMT (mapping pending)
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/drug-maps/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	var res server.BulkResult
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Total != 5 {
		t.Errorf("total = %d, want 5", res.Total)
	}
	if res.Imported != 3 {
		t.Errorf("imported = %d, want 3 (D001+D002+D005)", res.Imported)
	}
	if len(res.Errors) != 2 {
		t.Errorf("errors = %d, want 2 (D003 + D004)", len(res.Errors))
	}
	// Check error rows carry row index + key
	for _, e := range res.Errors {
		if e.Key == "" || e.Reason == "" {
			t.Errorf("error row missing fields: %+v", e)
		}
	}

	// Confirm good rows actually stored
	list, _ := repo.List(nil, "12345")
	if len(list) != 3 {
		t.Errorf("stored rows = %d, want 3", len(list))
	}
}

func TestBulkIcdMaps_MixedTypes(t *testing.T) {
	repo := newMemIcdMapRepo()
	h := server.New(server.Deps{IcdMapRepo: repo})

	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"hcode": "12345", "his_icd_code": "A", "icd_type": "10", "std_code": "I10"},
			{"hcode": "12345", "his_icd_code": "A", "icd_type": "9C", "std_code": "99.04"},
			{"hcode": "12345", "his_icd_code": "B", "icd_type": "BAD", "std_code": "X"}, // invalid type
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/icd-maps/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var res server.BulkResult
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Imported != 2 || len(res.Errors) != 1 {
		t.Errorf("got %+v", res)
	}
	if res.Errors[0].Key != "12345/BAD/B" {
		t.Errorf("error key should encode composite PK, got %q", res.Errors[0].Key)
	}
}

func TestBulkEndpoint_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	body, _ := json.Marshal(map[string]any{"items": []store.DrugMap{{HCode: "12345"}}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/drug-maps/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}
