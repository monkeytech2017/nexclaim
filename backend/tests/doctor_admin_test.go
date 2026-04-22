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

// ── in-memory fakes ──

type memDoctorRepo struct {
	mu   sync.Mutex
	data map[string]store.Doctor
}

func newMemDoctorRepo() *memDoctorRepo { return &memDoctorRepo{data: make(map[string]store.Doctor)} }

func (m *memDoctorRepo) List(_ context.Context, hcode string) ([]store.Doctor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.Doctor, 0, len(m.data))
	for _, d := range m.data {
		if hcode == "" || d.HCode == hcode {
			out = append(out, d)
		}
	}
	return out, nil
}

func (m *memDoctorRepo) Get(_ context.Context, id string) (*store.Doctor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.data[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &d, nil
}

func (m *memDoctorRepo) Upsert(_ context.Context, d store.Doctor) (*store.Doctor, error) {
	if d.DoctorID == "" {
		return nil, errors.New("doctor_id required")
	}
	if len(d.HCode) != 5 {
		return nil, errors.New("hcode must be 5 chars")
	}
	if d.LicenseNo == "" {
		return nil, errors.New("license_no required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[d.DoctorID] = d
	cp := d
	return &cp, nil
}

func (m *memDoctorRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

// ── handler tests ──

func TestDoctorAPI_FullCrud(t *testing.T) {
	h := server.New(server.Deps{DoctorRepo: newMemDoctorRepo()})

	// Create
	body, _ := json.Marshal(map[string]any{
		"doctor_id": "D0001", "hcode": "12345", "license_no": "V012345",
		"name_th": "นพ.สมชาย", "is_active": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/doctors", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// List filtered by hcode
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/doctors?hcode=12345", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET list want 200, got %d", w.Code)
	}
	var list struct{ Doctors []store.Doctor }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Doctors) != 1 {
		t.Errorf("list size = %d", len(list.Doctors))
	}

	// PATCH
	patch, _ := json.Marshal(map[string]any{"license_no": "V999999", "is_active": false, "hcode": "12345"})
	preq := httptest.NewRequest(http.MethodPatch, "/api/v1/master/doctors/D0001", bytes.NewReader(patch))
	preq.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, preq)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH want 200, got %d", w.Code)
	}
	var patched store.Doctor
	_ = json.Unmarshal(w.Body.Bytes(), &patched)
	if patched.LicenseNo != "V999999" || patched.IsActive {
		t.Errorf("patch mismatch: %+v", patched)
	}

	// DELETE
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/master/doctors/D0001", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE want 204, got %d", w.Code)
	}
}

func TestDoctorAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/doctors", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

// ── Pg integration (skip without DSN) ──

func TestDoctorRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Need an m_hospital row — doctor.hcode has FK to m_hospital.
	if _, err := conn.Exec(`DELETE FROM m_doctor`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	repo := store.NewPgDoctorRepo(conn)
	ctx := context.Background()

	d, err := repo.Upsert(ctx, store.Doctor{
		DoctorID: "D-TEST-1", HCode: "12345", LicenseNo: "V012345",
		NameTH: "นพ.ทดสอบ", Specialty: "อายุรกรรม", IsActive: true,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if d.NameTH != "นพ.ทดสอบ" {
		t.Errorf("thai roundtrip: %q", d.NameTH)
	}

	rows, _ := repo.List(ctx, "12345")
	if len(rows) != 1 {
		t.Errorf("want 1 row, got %d", len(rows))
	}

	_ = repo.Delete(ctx, "D-TEST-1")
	if _, err := repo.Get(ctx, "D-TEST-1"); err != store.ErrNotFound {
		t.Errorf("want not found, got %v", err)
	}
}
