package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

// memTmtRepo is an in-memory fake implementing store.TmtRepo. It captures the
// last filter it received so handler tests can assert pass-through, and applies
// the same [1,200] limit clamp the Pg impl does so paging behaves identically.
type memTmtRepo struct {
	mu     sync.Mutex
	rows   []store.TmtDrug
	lastF  store.TmtFilter
	called bool
}

func newMemTmtRepo(seeded ...store.TmtDrug) *memTmtRepo {
	return &memTmtRepo{rows: append([]store.TmtDrug{}, seeded...)}
}

func clampLimit(l int) int {
	if l <= 0 {
		return 50
	}
	if l > 200 {
		return 200
	}
	return l
}

func (m *memTmtRepo) Search(_ context.Context, f store.TmtFilter) ([]store.TmtDrug, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastF = f
	m.called = true

	matched := []store.TmtDrug{}
	for _, r := range m.rows {
		if f.Q == "" {
			matched = append(matched, r)
			continue
		}
		q := strings.ToLower(f.Q)
		if strings.Contains(strings.ToLower(r.TmtCode), q) ||
			strings.Contains(strings.ToLower(r.NameTh), q) ||
			strings.Contains(strings.ToLower(r.GenericName), q) {
			matched = append(matched, r)
		}
	}
	total := len(matched)

	off := f.Offset
	if off < 0 {
		off = 0
	}
	if off > len(matched) {
		off = len(matched)
	}
	lim := clampLimit(f.Limit)
	end := off + lim
	if end > len(matched) {
		end = len(matched)
	}
	return matched[off:end], total, nil
}

var _ store.TmtRepo = (*memTmtRepo)(nil)

func TestTmtAPI_SearchAndShape(t *testing.T) {
	repo := newMemTmtRepo(
		store.TmtDrug{TmtCode: "1000043", NameTh: "Paracetamol 500mg tablet", GenericName: "paracetamol"},
		store.TmtDrug{TmtCode: "2000099", NameTh: "Amoxicillin 500mg capsule", GenericName: "amoxicillin"},
		store.TmtDrug{TmtCode: "3000111", NameTh: "Paracetamol syrup", GenericName: "paracetamol"},
	)
	h := server.New(server.Deps{TmtRepo: repo})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/tmt?q=para&limit=10", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Items []store.TmtDrug `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
	if len(resp.Items) != 2 {
		t.Errorf("items = %d, want 2", len(resp.Items))
	}
	for _, it := range resp.Items {
		if !strings.Contains(strings.ToLower(it.NameTh), "para") &&
			!strings.Contains(strings.ToLower(it.GenericName), "para") {
			t.Errorf("unexpected non-matching row: %+v", it)
		}
	}

	// Handler must forward q/limit to the repo verbatim.
	if repo.lastF.Q != "para" || repo.lastF.Limit != 10 {
		t.Errorf("filter not forwarded: %+v", repo.lastF)
	}
}

func TestTmtAPI_LimitRespected(t *testing.T) {
	seeded := make([]store.TmtDrug, 0, 5)
	for _, c := range []string{"A1", "A2", "A3", "A4", "A5"} {
		seeded = append(seeded, store.TmtDrug{TmtCode: c, NameTh: "drug " + c})
	}
	repo := newMemTmtRepo(seeded...)
	h := server.New(server.Deps{TmtRepo: repo})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/tmt?limit=2", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp struct {
		Items []store.TmtDrug `json:"items"`
		Total int             `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 2 {
		t.Errorf("items = %d, want 2 (limit)", len(resp.Items))
	}
	if resp.Total != 5 {
		t.Errorf("total = %d, want 5 (all matching)", resp.Total)
	}
}

func TestTmtAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/tmt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

// TestTmtAPI_LimitClamp asserts a limit beyond the cap is forwarded raw by the
// handler (the repo applies the clamp). The fake mirrors the Pg clamp, so the
// returned page is bounded to 200 even though limit=9999 was requested.
func TestTmtAPI_LimitClamp(t *testing.T) {
	seeded := make([]store.TmtDrug, 0, 250)
	for i := 0; i < 250; i++ {
		seeded = append(seeded, store.TmtDrug{
			TmtCode: "C" + strings.Repeat("0", 4) + string(rune('A'+i%26)),
			NameTh:  "row",
		})
	}
	repo := newMemTmtRepo(seeded...)
	h := server.New(server.Deps{TmtRepo: repo})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/tmt?limit=9999", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Handler forwards the raw 9999; the clamp lives in the repo.
	if repo.lastF.Limit != 9999 {
		t.Errorf("handler should forward raw limit, got %d", repo.lastF.Limit)
	}
	if clampLimit(repo.lastF.Limit) != 200 {
		t.Errorf("clamp(9999) = %d, want 200", clampLimit(repo.lastF.Limit))
	}

	var resp struct {
		Items []store.TmtDrug `json:"items"`
		Total int             `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 200 {
		t.Errorf("page size = %d, want 200 (clamped)", len(resp.Items))
	}
	if resp.Total != 250 {
		t.Errorf("total = %d, want 250", resp.Total)
	}
}

// TestTmtRepo_Pg exercises the real Postgres impl. Read-only — no writes, no
// cleanup. Skips cleanly without POSTGRES_TEST_DSN. Relies on the seeded
// "Paracetamol 500mg tablet" row in m_tmt_drug.
func TestTmtRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	repo := store.NewPgTmtRepo(conn)
	items, total, err := repo.Search(context.Background(), store.TmtFilter{Q: "para", Limit: 5})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) > 5 {
		t.Errorf("returned %d items, want <=5", len(items))
	}
	if total < len(items) {
		t.Errorf("total %d < items %d", total, len(items))
	}
	found := false
	for _, it := range items {
		// Codes must be trimmed (no trailing padding).
		if it.TmtCode != strings.TrimSpace(it.TmtCode) {
			t.Errorf("tmt_code not trimmed: %q", it.TmtCode)
		}
		hay := strings.ToLower(it.NameTh + " " + it.GenericName)
		if strings.Contains(hay, "para") {
			found = true
		}
	}
	if !found {
		t.Errorf("no row matched 'para' case-insensitively in %+v", items)
	}
}
