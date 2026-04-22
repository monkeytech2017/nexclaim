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

// ── in-memory fake HospitalRepo so handler tests don't need a real DB ──

type memHospitalRepo struct {
	mu   sync.Mutex
	data map[string]store.Hospital
}

func newMemHospitalRepo() *memHospitalRepo {
	return &memHospitalRepo{data: make(map[string]store.Hospital)}
}

func (m *memHospitalRepo) List(_ context.Context) ([]store.Hospital, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.Hospital, 0, len(m.data))
	for _, h := range m.data {
		out = append(out, h)
	}
	return out, nil
}

func (m *memHospitalRepo) Get(_ context.Context, hcode string) (*store.Hospital, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.data[hcode]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &h, nil
}

func (m *memHospitalRepo) Upsert(_ context.Context, h store.Hospital) (*store.Hospital, error) {
	if len(h.HCode) != 5 {
		return nil, errors.New("hcode must be 5 chars")
	}
	if h.NameTH == "" {
		return nil, errors.New("name_th required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[h.HCode] = h
	cp := h
	return &cp, nil
}

func (m *memHospitalRepo) Delete(_ context.Context, hcode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[hcode]; !ok {
		return store.ErrNotFound
	}
	delete(m.data, hcode)
	return nil
}

// ── handler tests (no DB required) ──

func TestHospitalAPI_UpsertListGetDelete(t *testing.T) {
	repo := newMemHospitalRepo()
	h := server.New(server.Deps{HospitalRepo: repo})

	body := map[string]any{
		"hcode": "12345", "name_th": "รพ.ตัวอย่าง",
		"changwat": "10", "amphur": "1001", "is_active": true,
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/hospitals", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var created store.Hospital
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.HCode != "12345" || created.NameTH != "รพ.ตัวอย่าง" {
		t.Errorf("created = %+v", created)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/hospitals", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET list want 200, got %d", w.Code)
	}
	var list struct{ Hospitals []store.Hospital }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Hospitals) != 1 {
		t.Errorf("list size = %d", len(list.Hospitals))
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/hospitals/12345", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET one want 200, got %d", w.Code)
	}

	// PATCH via path param
	patch, _ := json.Marshal(map[string]any{"name_th": "รพ.ใหม่", "is_active": false})
	preq := httptest.NewRequest(http.MethodPatch, "/api/v1/master/hospitals/12345", bytes.NewReader(patch))
	preq.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, preq)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var patched store.Hospital
	_ = json.Unmarshal(w.Body.Bytes(), &patched)
	if patched.NameTH != "รพ.ใหม่" || patched.IsActive {
		t.Errorf("patch mismatch: %+v", patched)
	}

	// DELETE
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/master/hospitals/12345", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE want 204, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/hospitals/12345", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("GET after delete want 404, got %d", w.Code)
	}
}

func TestHospitalAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/hospitals", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503 when repo nil, got %d", w.Code)
	}
}

func TestHospitalAPI_ValidationErrors(t *testing.T) {
	repo := newMemHospitalRepo()
	h := server.New(server.Deps{HospitalRepo: repo})

	// Bad hcode length
	body, _ := json.Marshal(map[string]any{"hcode": "ABC", "name_th": "X"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/hospitals", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid hcode, got %d", w.Code)
	}

	// Missing name_th
	body, _ = json.Marshal(map[string]any{"hcode": "12345"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/master/hospitals", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing name_th, got %d", w.Code)
	}
}

// ── PgHospitalRepo integration test (skip without DSN) ──

func TestHospitalRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping Pg hospital repo test")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`TRUNCATE m_hospital CASCADE`); err != nil {
		t.Fatalf("truncate: %v — is migration 001 applied?", err)
	}

	repo := store.NewPgHospitalRepo(conn)
	ctx := context.Background()

	h, err := repo.Upsert(ctx, store.Hospital{
		HCode: "11065", NameTH: "รพ.นครปฐม",
		Changwat: "73", Amphur: "7301", IsActive: true,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if h.NameTH != "รพ.นครปฐม" {
		t.Errorf("thai roundtrip: %q", h.NameTH)
	}

	h2, err := repo.Upsert(ctx, store.Hospital{
		HCode: "11065", NameTH: "รพ.นครปฐม สาขา 2", IsActive: false,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if h2.IsActive {
		t.Error("is_active should be false after update")
	}

	rows, _ := repo.List(ctx)
	if len(rows) != 1 {
		t.Errorf("want 1 row, got %d", len(rows))
	}

	if err := repo.Delete(ctx, "11065"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, "11065"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
