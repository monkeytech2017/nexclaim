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

type memDrugMapRepo struct {
	mu   sync.Mutex
	data map[string]store.DrugMap
}

func newMemDrugMapRepo() *memDrugMapRepo { return &memDrugMapRepo{data: make(map[string]store.DrugMap)} }

func (m *memDrugMapRepo) List(_ context.Context, hcode string) ([]store.DrugMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.DrugMap, 0, len(m.data))
	for _, v := range m.data {
		if hcode == "" || v.HCode == hcode {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *memDrugMapRepo) Get(_ context.Context, hc, c string) (*store.DrugMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[hc+"|"+c]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &v, nil
}

func (m *memDrugMapRepo) Upsert(_ context.Context, v store.DrugMap) (*store.DrugMap, error) {
	if len(v.HCode) != 5 || v.HISDrugCode == "" {
		return nil, errors.New("hcode + his_drug_code required")
	}
	if v.TMTCode != "" && len(v.TMTCode) != 24 {
		return nil, errors.New("tmt must be 24 chars")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[v.HCode+"|"+v.HISDrugCode] = v
	cp := v
	return &cp, nil
}

func (m *memDrugMapRepo) Delete(_ context.Context, hc, c string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := hc + "|" + c
	if _, ok := m.data[k]; !ok {
		return store.ErrNotFound
	}
	delete(m.data, k)
	return nil
}

type memDoctorMapRepo struct {
	mu   sync.Mutex
	data map[string]store.DoctorMap
}

func newMemDoctorMapRepo() *memDoctorMapRepo {
	return &memDoctorMapRepo{data: make(map[string]store.DoctorMap)}
}

func (m *memDoctorMapRepo) List(_ context.Context, hcode string) ([]store.DoctorMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.DoctorMap, 0, len(m.data))
	for _, v := range m.data {
		if hcode == "" || v.HCode == hcode {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *memDoctorMapRepo) Get(_ context.Context, hc, c string) (*store.DoctorMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[hc+"|"+c]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &v, nil
}

func (m *memDoctorMapRepo) Upsert(_ context.Context, v store.DoctorMap) (*store.DoctorMap, error) {
	if len(v.HCode) != 5 || v.HISDoctorCode == "" {
		return nil, errors.New("hcode + his_doctor_code required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[v.HCode+"|"+v.HISDoctorCode] = v
	cp := v
	return &cp, nil
}

func (m *memDoctorMapRepo) Delete(_ context.Context, hc, c string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := hc + "|" + c
	if _, ok := m.data[k]; !ok {
		return store.ErrNotFound
	}
	delete(m.data, k)
	return nil
}

// ── handler tests ──

func TestDrugMapAPI_UpsertListDelete(t *testing.T) {
	h := server.New(server.Deps{DrugMapRepo: newMemDrugMapRepo()})

	body, _ := json.Marshal(map[string]any{
		"hcode": "12345", "his_drug_code": "DRG000124",
		"tmt_code": "100452100101010100000612",
		"his_drug_name": "AMLODIPINE 5MG TAB", "is_active": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/drug-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Bad TMT (not 24 chars) rejected
	body, _ = json.Marshal(map[string]any{
		"hcode": "12345", "his_drug_code": "DRG002",
		"tmt_code": "TOOSHORT",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/master/drug-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("short TMT should return 400, got %d", w.Code)
	}

	// DELETE
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/master/drug-maps/12345/DRG000124", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE want 204, got %d", w.Code)
	}
}

func TestDoctorMapAPI_UpsertListDelete(t *testing.T) {
	h := server.New(server.Deps{DoctorMapRepo: newMemDoctorMapRepo()})

	body, _ := json.Marshal(map[string]any{
		"hcode": "12345", "his_doctor_code": "D0045", "doctor_id": "INTERNAL-45",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/doctor-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// List
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/doctor-maps?hcode=12345", nil))
	var list struct{ Items []store.DoctorMap }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Errorf("list size = %d", len(list.Items))
	}

	// DELETE
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/master/doctor-maps/12345/D0045", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE want 204, got %d", w.Code)
	}
}

// ── Pg integration ──

func TestDrugMapRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`DELETE FROM his_drug_map WHERE hcode = '12345'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}
	// tmt_code FK to m_tmt_drug — seed one
	if _, err := conn.Exec(`
		INSERT INTO m_tmt_drug (tmt_code, name_th, is_active, updated_at)
		VALUES ('100452100101010100000612','Test Drug',true,now())
		ON CONFLICT (tmt_code) DO NOTHING`); err != nil {
		t.Fatalf("seed tmt: %v", err)
	}

	repo := store.NewPgDrugMapRepo(conn)
	ctx := context.Background()

	m, err := repo.Upsert(ctx, store.DrugMap{
		HCode: "12345", HISDrugCode: "DRG000124",
		TMTCode: "100452100101010100000612",
		HISDrugName: "AMLODIPINE 5MG TAB", Note: "ทดสอบ", IsActive: true,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if m.Note != "ทดสอบ" {
		t.Errorf("thai roundtrip: %q", m.Note)
	}

	// Upsert same key — should update not duplicate
	_, err = repo.Upsert(ctx, store.DrugMap{
		HCode: "12345", HISDrugCode: "DRG000124",
		HISDrugName: "AMLODIPINE 5MG TAB (updated)", IsActive: false,
	})
	if err != nil {
		t.Fatalf("upsert 2nd: %v", err)
	}
	rows, _ := repo.List(ctx, "12345")
	if len(rows) != 1 {
		t.Errorf("upsert should not duplicate — got %d rows", len(rows))
	}

	_ = repo.Delete(ctx, "12345", "DRG000124")
	if _, err := repo.Get(ctx, "12345", "DRG000124"); err != store.ErrNotFound {
		t.Errorf("want not found, got %v", err)
	}
}

func TestDoctorMapRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`DELETE FROM his_doctor_map WHERE hcode = '12345'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}
	if _, err := conn.Exec(`DELETE FROM m_doctor WHERE doctor_id = 'D-TEST-1'`); err != nil {
		t.Fatalf("cleanup doctor: %v", err)
	}
	if _, err := conn.Exec(`
		INSERT INTO m_doctor (doctor_id, hcode, license_no, is_active, updated_at)
		VALUES ('D-TEST-1','12345','V000001',true,now())`); err != nil {
		t.Fatalf("seed doctor: %v", err)
	}

	repo := store.NewPgDoctorMapRepo(conn)
	ctx := context.Background()

	m, err := repo.Upsert(ctx, store.DoctorMap{
		HCode: "12345", HISDoctorCode: "HIS-DOC-99", DoctorID: "D-TEST-1",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if m.DoctorID != "D-TEST-1" {
		t.Errorf("link doctor_id: %q", m.DoctorID)
	}

	rows, _ := repo.List(ctx, "12345")
	if len(rows) != 1 {
		t.Errorf("want 1 row, got %d", len(rows))
	}

	_ = repo.Delete(ctx, "12345", "HIS-DOC-99")
}
