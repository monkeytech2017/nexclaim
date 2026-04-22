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

type memInsclRepo struct {
	mu   sync.Mutex
	data map[string]store.InsclMap // key = hcode + "|" + his_pttype
}

func newMemInsclRepo() *memInsclRepo { return &memInsclRepo{data: make(map[string]store.InsclMap)} }

func key(hc, pt string) string { return hc + "|" + pt }

func (m *memInsclRepo) List(_ context.Context, hcode string) ([]store.InsclMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.InsclMap, 0, len(m.data))
	for _, v := range m.data {
		if hcode == "" || v.HCode == hcode {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *memInsclRepo) Get(_ context.Context, hc, pt string) (*store.InsclMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key(hc, pt)]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &v, nil
}

func (m *memInsclRepo) Upsert(_ context.Context, v store.InsclMap) (*store.InsclMap, error) {
	if len(v.HCode) != 5 {
		return nil, errors.New("hcode must be 5 chars")
	}
	if v.HISPttype == "" {
		return nil, errors.New("his_pttype required")
	}
	if v.INSCL == "" {
		return nil, errors.New("inscl required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key(v.HCode, v.HISPttype)] = v
	cp := v
	return &cp, nil
}

func (m *memInsclRepo) Delete(_ context.Context, hc, pt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(hc, pt)
	if _, ok := m.data[k]; !ok {
		return store.ErrNotFound
	}
	delete(m.data, k)
	return nil
}

// ── handler tests ──

func TestInsclMapAPI_UpsertListDelete(t *testing.T) {
	h := server.New(server.Deps{InsclMapRepo: newMemInsclRepo()})

	body, _ := json.Marshal(map[string]any{
		"hcode": "12345", "his_pttype": "A01", "inscl": "011", "agency_code": "CGD",
		"note": "ข้าราชการพลเรือน",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/inscl-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Upsert same key — should update not duplicate
	body, _ = json.Marshal(map[string]any{
		"hcode": "12345", "his_pttype": "A01", "inscl": "WEL",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/master/inscl-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST (upsert) want 200, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/inscl-maps?hcode=12345", nil))
	var list struct{ Items []store.InsclMap }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Errorf("upsert should not duplicate — got %d rows", len(list.Items))
	}
	if list.Items[0].INSCL != "WEL" {
		t.Errorf("INSCL should be updated to WEL, got %s", list.Items[0].INSCL)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/master/inscl-maps/12345/A01", nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE want 204, got %d", w.Code)
	}
}

// ── Pg integration ──

func TestInsclMapRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	for _, q := range []string{`DELETE FROM his_inscl_map WHERE hcode = '12345'`} {
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

	repo := store.NewPgInsclMapRepo(conn)
	ctx := context.Background()

	m, err := repo.Upsert(ctx, store.InsclMap{
		HCode: "12345", HISPttype: "A01", INSCL: "011",
		AgencyCode: "CGD", Note: "ข้าราชการ",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if m.Note != "ข้าราชการ" {
		t.Errorf("thai roundtrip: %q", m.Note)
	}

	// Upsert same key
	m, err = repo.Upsert(ctx, store.InsclMap{HCode: "12345", HISPttype: "A01", INSCL: "WEL"})
	if err != nil {
		t.Fatalf("upsert 2nd: %v", err)
	}
	if m.INSCL != "WEL" {
		t.Errorf("INSCL not updated: %s", m.INSCL)
	}

	rows, _ := repo.List(ctx, "12345")
	if len(rows) != 1 {
		t.Errorf("should be 1 row after upsert-twice, got %d", len(rows))
	}

	_ = repo.Delete(ctx, "12345", "A01")
	if _, err := repo.Get(ctx, "12345", "A01"); err != store.ErrNotFound {
		t.Errorf("want not found, got %v", err)
	}
}
