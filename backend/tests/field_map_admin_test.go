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

// ── in-memory fake ──

type memFieldMapRepo struct {
	mu   sync.Mutex
	data map[string]store.FieldMap // key = natural 5-part
	next int
}

func newMemFieldMapRepo() *memFieldMapRepo {
	return &memFieldMapRepo{data: make(map[string]store.FieldMap)}
}

func fmKey(m store.FieldMap) string {
	return m.HCode + "|" + m.HISTable + "|" + m.HISColumn + "|" + m.TargetFile + "|" + m.TargetField
}

func (m *memFieldMapRepo) List(_ context.Context, f store.FieldMapFilter) ([]store.FieldMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.FieldMap, 0, len(m.data))
	for _, v := range m.data {
		if f.HCode != "" && v.HCode != f.HCode {
			continue
		}
		if f.HISTable != "" && v.HISTable != f.HISTable {
			continue
		}
		if f.TargetFile != "" && v.TargetFile != f.TargetFile {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (m *memFieldMapRepo) Get(_ context.Context, id string) (*store.FieldMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.data {
		if v.ID == id {
			return &v, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *memFieldMapRepo) Upsert(_ context.Context, v store.FieldMap) (*store.FieldMap, error) {
	if len(v.HCode) != 5 {
		return nil, errors.New("hcode must be 5 chars")
	}
	if v.HISTable == "" || v.HISColumn == "" || v.TargetFile == "" || v.TargetField == "" {
		return nil, errors.New("5-part key required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.data[fmKey(v)]
	if ok {
		v.ID = existing.ID
	} else {
		m.next++
		v.ID = "fm-" + string(rune('A'+m.next))
	}
	if v.Transform == "" {
		v.Transform = "none"
	}
	m.data[fmKey(v)] = v
	cp := v
	return &cp, nil
}

func (m *memFieldMapRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.data {
		if v.ID == id {
			delete(m.data, k)
			return nil
		}
	}
	return store.ErrNotFound
}

// ── handler tests ──

func TestFieldMapAPI_UpsertListDelete(t *testing.T) {
	h := server.New(server.Deps{FieldMapRepo: newMemFieldMapRepo()})

	body, _ := json.Marshal(map[string]any{
		"hcode": "12345", "his_table": "opd_visit", "his_column": "vstdate",
		"target_file": "OPD", "target_field": "DATEOPD",
		"transform": "to_ad", "is_required": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/field-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var created store.FieldMap
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID == "" {
		t.Error("expected ID assigned")
	}

	// Upsert same 5-part key → same ID (no duplicate)
	body, _ = json.Marshal(map[string]any{
		"hcode": "12345", "his_table": "opd_visit", "his_column": "vstdate",
		"target_file": "OPD", "target_field": "DATEOPD",
		"transform": "none", "is_required": false,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/master/field-maps", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var updated store.FieldMap
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.ID != created.ID {
		t.Errorf("upsert should preserve ID: created=%s updated=%s", created.ID, updated.ID)
	}

	// List filtered by target_file
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/master/field-maps?target_file=OPD", nil))
	var list struct{ Items []store.FieldMap }
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 {
		t.Errorf("filter OPD list size = %d", len(list.Items))
	}

	// DELETE by id
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/master/field-maps/"+created.ID, nil))
	if w.Code != http.StatusNoContent {
		t.Errorf("DELETE want 204, got %d", w.Code)
	}
}

func TestFieldMapAPI_BulkPartialErrors(t *testing.T) {
	h := server.New(server.Deps{FieldMapRepo: newMemFieldMapRepo()})
	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"hcode": "12345", "his_table": "opd_visit", "his_column": "vstdate",
				"target_file": "OPD", "target_field": "DATEOPD", "is_required": true},
			{"hcode": "BAD", "his_table": "t", "his_column": "c", "target_file": "OPD", "target_field": "F"}, // hcode != 5
			{"hcode": "12345", "his_table": "", "his_column": "c", "target_file": "OPD", "target_field": "F"}, // empty table
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/master/field-maps/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var res server.BulkResult
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Total != 3 || res.Imported != 1 || len(res.Errors) != 2 {
		t.Errorf("got %+v", res)
	}
}

// ── Pg integration ──

func TestFieldMapRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`DELETE FROM his_field_map WHERE hcode = '12345'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	repo := store.NewPgFieldMapRepo(conn)
	ctx := context.Background()

	// Insert
	m, err := repo.Upsert(ctx, store.FieldMap{
		HCode: "12345", HISTable: "opd_visit", HISColumn: "vstdate",
		TargetFile: "OPD", TargetField: "DATEOPD",
		Transform: "to_ad", IsRequired: true, Note: "แปลง พ.ศ. → ค.ศ.",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if m.ID == "" {
		t.Error("id empty")
	}
	if m.Note != "แปลง พ.ศ. → ค.ศ." {
		t.Errorf("thai roundtrip: %q", m.Note)
	}
	firstID := m.ID

	// Upsert same 5-part key — same id, transform updated
	m2, err := repo.Upsert(ctx, store.FieldMap{
		HCode: "12345", HISTable: "opd_visit", HISColumn: "vstdate",
		TargetFile: "OPD", TargetField: "DATEOPD",
		Transform: "none", IsRequired: false,
	})
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	if m2.ID != firstID {
		t.Errorf("upsert should preserve id: %s vs %s", firstID, m2.ID)
	}
	if m2.Transform != "none" {
		t.Errorf("transform not updated: %q", m2.Transform)
	}

	// List filter
	rows, _ := repo.List(ctx, store.FieldMapFilter{HCode: "12345", TargetFile: "OPD"})
	if len(rows) != 1 {
		t.Errorf("list OPD rows = %d", len(rows))
	}

	// Delete by id
	if err := repo.Delete(ctx, firstID); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, firstID); err != store.ErrNotFound {
		t.Errorf("want not found, got %v", err)
	}
}
