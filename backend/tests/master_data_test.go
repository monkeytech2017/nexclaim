package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

// ── in-memory fake MasterDataRepo ──

type memMasterDataRepo struct {
	icd10  []store.ICD10Row
	icd9cm []store.ICD9CMRow
	tmt    []store.TMTRow
	err    error // if non-nil, every method returns this error

	// captured params for assertions
	lastQ     string
	lastLimit int
}

func (m *memMasterDataRepo) ListICD10(_ context.Context, q string, limit int) ([]store.ICD10Row, error) {
	m.lastQ, m.lastLimit = q, limit
	if m.err != nil {
		return nil, m.err
	}
	return m.icd10, nil
}

func (m *memMasterDataRepo) ListICD9CM(_ context.Context, q string, limit int) ([]store.ICD9CMRow, error) {
	m.lastQ, m.lastLimit = q, limit
	if m.err != nil {
		return nil, m.err
	}
	return m.icd9cm, nil
}

func (m *memMasterDataRepo) ListTMT(_ context.Context, q string, limit int) ([]store.TMTRow, error) {
	m.lastQ, m.lastLimit = q, limit
	if m.err != nil {
		return nil, m.err
	}
	return m.tmt, nil
}

// ── helpers ──

func newMasterEngine(repo store.MasterDataRepo) *server.Deps {
	d := &server.Deps{MasterDataRepo: repo}
	return d
}

func masterSrv(repo store.MasterDataRepo) http.Handler {
	return server.New(server.Deps{MasterDataRepo: repo})
}

// ── tests ──

// (a) returns items as JSON {items, count}
func TestMasterData_ICD10_ReturnsItems(t *testing.T) {
	repo := &memMasterDataRepo{
		icd10: []store.ICD10Row{
			{Code: "A000", NameTH: "อหิวาตกโรค", NameEN: "Cholera due to Vibrio cholerae 01", Chapter: "I"},
			{Code: "A001", NameTH: "อหิวาต 2", NameEN: "Cholera due to Vibrio cholerae 01, biovar eltor", Chapter: "I"},
		},
	}
	h := masterSrv(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/icd10", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []store.ICD10Row `json:"items"`
		Count int              `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 2 {
		t.Errorf("count want 2, got %d", resp.Count)
	}
	if len(resp.Items) != 2 {
		t.Errorf("items len want 2, got %d", len(resp.Items))
	}
	if resp.Items[0].Code != "A000" {
		t.Errorf("first code want A000, got %q", resp.Items[0].Code)
	}
}

func TestMasterData_ICD9CM_ReturnsItems(t *testing.T) {
	repo := &memMasterDataRepo{
		icd9cm: []store.ICD9CMRow{
			{Code: "0010", NameTH: "คอลเลอรา", NameEN: "Cholera due to Vibrio cholerae"},
		},
	}
	h := masterSrv(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/icd9cm", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []store.ICD9CMRow `json:"items"`
		Count int               `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 1 || len(resp.Items) != 1 {
		t.Errorf("want 1 item, got count=%d len=%d", resp.Count, len(resp.Items))
	}
}

func TestMasterData_TMT_ReturnsItems(t *testing.T) {
	repo := &memMasterDataRepo{
		tmt: []store.TMTRow{
			{
				TMTCode: "1000000000000000000000TM",
				NameTH:  "ยาตัวอย่าง",
				GenericName: "Paracetamol",
				Strength:   "500 mg",
				DosageForm: "Tablet",
				Unit:       "tablet",
			},
		},
	}
	h := masterSrv(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/tmt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []store.TMTRow `json:"items"`
		Count int            `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("count want 1, got %d", resp.Count)
	}
	if resp.Items[0].TMTCode != "1000000000000000000000TM" {
		t.Errorf("tmt_code mismatch: %q", resp.Items[0].TMTCode)
	}
}

// (b) q and limit query params reach the repo
func TestMasterData_QueryParams(t *testing.T) {
	repo := &memMasterDataRepo{icd10: []store.ICD10Row{}}
	h := masterSrv(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/icd10?q=fever&limit=25", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if repo.lastQ != "fever" {
		t.Errorf("q want 'fever', got %q", repo.lastQ)
	}
	if repo.lastLimit != 25 {
		t.Errorf("limit want 25, got %d", repo.lastLimit)
	}
}

func TestMasterData_ICD9CM_QueryParams(t *testing.T) {
	repo := &memMasterDataRepo{icd9cm: []store.ICD9CMRow{}}
	h := masterSrv(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/icd9cm?q=op&limit=10", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if repo.lastQ != "op" {
		t.Errorf("q want 'op', got %q", repo.lastQ)
	}
	if repo.lastLimit != 10 {
		t.Errorf("limit want 10, got %d", repo.lastLimit)
	}
}

func TestMasterData_TMT_QueryParams(t *testing.T) {
	repo := &memMasterDataRepo{tmt: []store.TMTRow{}}
	h := masterSrv(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/tmt?q=para&limit=100", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if repo.lastQ != "para" {
		t.Errorf("q want 'para', got %q", repo.lastQ)
	}
	if repo.lastLimit != 100 {
		t.Errorf("limit want 100, got %d", repo.lastLimit)
	}
}

// (c) nil repo → 503
func TestMasterData_NilRepo_503(t *testing.T) {
	h := server.New(server.Deps{})

	for _, path := range []string{"/api/v1/master/icd10", "/api/v1/master/icd9cm", "/api/v1/master/tmt"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("path %s want 503, got %d", path, w.Code)
		}
	}
}

// (d) empty result → items is [] not null
func TestMasterData_EmptyResult_NotNull(t *testing.T) {
	repo := &memMasterDataRepo{
		icd10:  []store.ICD10Row{},
		icd9cm: []store.ICD9CMRow{},
		tmt:    []store.TMTRow{},
	}
	h := masterSrv(repo)

	paths := []string{"/api/v1/master/icd10", "/api/v1/master/icd9cm", "/api/v1/master/tmt"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("path %s want 200, got %d body=%s", path, w.Code, w.Body.String())
			continue
		}
		// Unmarshal raw to check items is [] not null
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Errorf("path %s unmarshal: %v", path, err)
			continue
		}
		itemsRaw, ok := raw["items"]
		if !ok {
			t.Errorf("path %s: no 'items' key in response", path)
			continue
		}
		if string(itemsRaw) == "null" {
			t.Errorf("path %s: items is null, want []", path)
		}
	}
}

// (e) repo error → 500
func TestMasterData_RepoError_500(t *testing.T) {
	repo := &memMasterDataRepo{err: errors.New("db connection lost")}
	h := masterSrv(repo)

	for _, path := range []string{"/api/v1/master/icd10", "/api/v1/master/icd9cm", "/api/v1/master/tmt"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("path %s want 500, got %d", path, w.Code)
		}
	}
}

// Compile-time assertion: fake satisfies the interface.
var _ store.MasterDataRepo = (*memMasterDataRepo)(nil)
