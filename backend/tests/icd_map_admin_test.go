package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

type memIcdMapRepo struct {
	mu   sync.Mutex
	data map[string]store.IcdMap
}

func newMemIcdMapRepo() *memIcdMapRepo { return &memIcdMapRepo{data: make(map[string]store.IcdMap)} }

func icdKey(hc, code, typ string) string { return hc + "|" + typ + "|" + code }

func (m *memIcdMapRepo) List(_ context.Context, hcode, icdType string) ([]store.IcdMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.IcdMap, 0, len(m.data))
	for _, v := range m.data {
		if (hcode == "" || v.HCode == hcode) && (icdType == "" || v.IcdType == icdType) {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *memIcdMapRepo) Get(_ context.Context, hc, code, typ string) (*store.IcdMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[icdKey(hc, code, typ)]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &v, nil
}

func (m *memIcdMapRepo) Upsert(_ context.Context, v store.IcdMap) (*store.IcdMap, error) {
	if len(v.HCode) != 5 {
		return nil, errors.New("hcode must be 5 chars")
	}
	if v.IcdType != store.IcdType10 && v.IcdType != store.IcdType9C {
		return nil, errors.New("icd_type must be '10' or '9C'")
	}
	if v.HISIcdCode == "" || v.StdCode == "" {
		return nil, errors.New("his_icd_code + std_code required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[icdKey(v.HCode, v.HISIcdCode, v.IcdType)] = v
	cp := v
	return &cp, nil
}

func (m *memIcdMapRepo) Delete(_ context.Context, hc, code, typ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := icdKey(hc, code, typ)
	if _, ok := m.data[k]; !ok {
		return store.ErrNotFound
	}
	delete(m.data, k)
	return nil
}

func TestIcdMapAPI_UpsertListDelete(t *testing.T) {
	h := server.New(server.Deps{IcdMapRepo: newMemIcdMapRepo()})

	// Create ICD-10 entry
	body, _ := json.Marshal(map[string]any{
		"hcode": "12345", "his_icd_code": "LOCAL-J06", "icd_type": "10", "std_code": "J06.9",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/icd-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST ICD-10 want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Same HIS code but different icd_type — should NOT conflict (part of PK)
	body, _ = json.Marshal(map[string]any{
		"hcode": "12345", "his_icd_code": "LOCAL-J06", "icd_type": "9C", "std_code": "88.71",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/master/icd-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST ICD-9CM same code want 200, got %d", w.Code)
	}

	// Reject invalid icd_type
	body, _ = json.Marshal(map[string]any{
		"hcode": "12345", "his_icd_code": "X", "icd_type": "BOGUS", "std_code": "X",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/master/icd-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad icd_type should return 400, got %d", w.Code)
	}

	// List with type filter
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/icd-maps?type=10", nil))
	var list struct{ Items []store.IcdMap }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Errorf("list filter by type=10 should return 1, got %d", len(list.Items))
	}

	// Delete — URL uses hcode/type/code order
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/master/icd-maps/12345/10/LOCAL-J06", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE want 204, got %d", w.Code)
	}
	// The 9C variant should still exist
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/icd-maps?hcode=12345", nil))
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Errorf("9C variant should survive deletion of 10 variant, got %d rows", len(list.Items))
	}
}

func TestIcdMapRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`DELETE FROM his_icd_map WHERE hcode = '12345'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	repo := store.NewPgIcdMapRepo(conn)
	ctx := context.Background()

	// Insert one of each type — share the same HIS code
	if _, err := repo.Upsert(ctx, store.IcdMap{
		HCode: "12345", HISIcdCode: "LOCAL-A1", IcdType: "10", StdCode: "I10",
	}); err != nil {
		t.Fatalf("upsert 10: %v", err)
	}
	if _, err := repo.Upsert(ctx, store.IcdMap{
		HCode: "12345", HISIcdCode: "LOCAL-A1", IcdType: "9C", StdCode: "99.04",
	}); err != nil {
		t.Fatalf("upsert 9C: %v", err)
	}
	all, _ := repo.List(ctx, "12345", "")
	if len(all) != 2 {
		t.Errorf("want 2 rows (one per icd_type), got %d", len(all))
	}
	tenOnly, _ := repo.List(ctx, "12345", "10")
	if len(tenOnly) != 1 {
		t.Errorf("type filter should return 1, got %d", len(tenOnly))
	}

	// Upsert same composite key — should update std_code, not duplicate
	if _, err := repo.Upsert(ctx, store.IcdMap{
		HCode: "12345", HISIcdCode: "LOCAL-A1", IcdType: "10", StdCode: "I11",
	}); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	again, _ := repo.List(ctx, "12345", "")
	if len(again) != 2 {
		t.Errorf("upsert-twice shouldn't duplicate, got %d", len(again))
	}
	got, _ := repo.Get(ctx, "12345", "LOCAL-A1", "10")
	if got.StdCode != "I11" {
		t.Errorf("std_code not updated: %q", got.StdCode)
	}

	_ = repo.Delete(ctx, "12345", "LOCAL-A1", "10")
	_ = repo.Delete(ctx, "12345", "LOCAL-A1", "9C")
}
