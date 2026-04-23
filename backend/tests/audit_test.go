package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/audit"
	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

// ── in-memory AuditRepo fake ──────────────────────────────────────────────
//
// memAuditRepo doubles as an audit.Writer and a store.AuditRepo so the same
// value can be handed to server.Deps.AuditRepo + .AuditWriter. Kept small
// on purpose — the server exercises it via public routes, no internal hooks.

type memAuditRepo struct {
	mu      sync.Mutex
	entries []memAuditRow
	// writeErr, if non-nil, is returned from every Write call so tests can
	// verify that instrumented handlers don't propagate audit failures.
	writeErr error
}

// memAuditRow mirrors store.AuditEntry for the in-memory path. We keep the
// exact fields the List handler returns on the wire so assertions stay
// readable.
type memAuditRow struct {
	ID         string
	ActorID    string
	ActorRole  string
	ActorName  string
	Action     string
	TargetKind string
	TargetID   string
	HCode      string
	Payload    map[string]any
	CreatedAt  time.Time
}

func newMemAuditRepo() *memAuditRepo { return &memAuditRepo{} }

func (m *memAuditRepo) Write(_ context.Context, e audit.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.entries = append(m.entries, memAuditRow{
		ID:         fmt.Sprintf("audit-%d", len(m.entries)+1),
		ActorID:    e.ActorID,
		ActorRole:  e.ActorRole,
		ActorName:  e.ActorName,
		Action:     e.Action,
		TargetKind: e.TargetKind,
		TargetID:   e.TargetID,
		HCode:      e.HCode,
		Payload:    e.Payload,
		CreatedAt:  time.Now(),
	})
	return nil
}

func (m *memAuditRepo) List(_ context.Context, f store.AuditFilter) ([]store.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.AuditEntry, 0, len(m.entries))
	for _, r := range m.entries {
		if f.Action != "" && r.Action != f.Action {
			continue
		}
		if f.ActorID != "" && r.ActorID != f.ActorID {
			continue
		}
		if f.HCode != "" && r.HCode != f.HCode {
			continue
		}
		if f.TargetKind != "" && r.TargetKind != f.TargetKind {
			continue
		}
		if f.TargetID != "" && r.TargetID != f.TargetID {
			continue
		}
		entry := store.AuditEntry{
			ID:         r.ID,
			ActorRole:  r.ActorRole,
			ActorName:  r.ActorName,
			Action:     r.Action,
			TargetKind: r.TargetKind,
			TargetID:   r.TargetID,
			HCode:      r.HCode,
			Payload:    r.Payload,
			CreatedAt:  r.CreatedAt,
		}
		if r.ActorID != "" {
			id := r.ActorID
			entry.ActorID = &id
		}
		out = append(out, entry)
	}
	return out, nil
}

func (m *memAuditRepo) snapshot() []memAuditRow {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]memAuditRow, len(m.entries))
	copy(cp, m.entries)
	return cp
}

// auditListItem mirrors the JSON the handler returns — kept loose (map[string]any
// for payload) so we don't tie the test to the exact struct tags.
type auditListItem struct {
	ID         string         `json:"id"`
	ActorID    *string        `json:"actor_id,omitempty"`
	ActorRole  string         `json:"actor_role,omitempty"`
	ActorName  string         `json:"actor_name,omitempty"`
	Action     string         `json:"action"`
	TargetKind string         `json:"target_kind,omitempty"`
	TargetID   string         `json:"target_id,omitempty"`
	HCode      string         `json:"hcode,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ── Handler-level tests ───────────────────────────────────────────────────

func TestAuditAPI_ListFilter(t *testing.T) {
	ar := newMemAuthRepo()
	adminKey := ar.mint(auth.RoleAdmin, "", "admin-caller")
	hospKey := ar.mint(auth.RoleHospital, "12345", "hosp")
	audit0 := newMemAuditRepo()
	// Seed 3 entries with different shapes.
	_ = audit0.Write(context.Background(), audit.Entry{
		Action: "api_key.create", TargetKind: "api_key", TargetID: "K1",
		HCode: "12345", ActorRole: "admin", ActorName: "admin-caller",
	})
	_ = audit0.Write(context.Background(), audit.Entry{
		Action: "ccode.resolve", TargetKind: "ccode", TargetID: "CC-1",
		HCode: "12345",
	})
	_ = audit0.Write(context.Background(), audit.Entry{
		Action: "rep.fetch", TargetKind: "rep", TargetID: "99999/202504",
		HCode: "99999",
	})

	h := server.New(server.Deps{
		AuthRepo:    ar,
		AuthEnabled: true,
		AuditRepo:   audit0,
		AuditWriter: audit0,
	})

	// admin, no filter → 3 rows
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin list: want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct{ Items []auditListItem }
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 3 {
		t.Errorf("want 3 items, got %d", len(resp.Items))
	}

	// filter by action
	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?action=api_key.create", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Action != "api_key.create" {
		t.Errorf("action filter mismatch: %+v", resp.Items)
	}

	// filter by hcode
	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?hcode=99999", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].Action != "rep.fetch" {
		t.Errorf("hcode filter mismatch: %+v", resp.Items)
	}

	// filter by target_kind
	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit-log?target_kind=ccode", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 1 || resp.Items[0].TargetKind != "ccode" {
		t.Errorf("target_kind filter mismatch: %+v", resp.Items)
	}

	// hospital caller → 403 (admin-only)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit-log", nil)
	req.Header.Set("Authorization", "Bearer "+hospKey)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("hospital should 403 on audit-log, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuditAPI_NoRepo503(t *testing.T) {
	h := server.New(server.Deps{}) // AuditRepo=nil, auth off → pass-through
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit-log", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAuditAPI_CreateKeyWritesAuditRow(t *testing.T) {
	authRepo := newMemAuthRepo()
	au := newMemAuditRepo()
	h := server.New(server.Deps{
		AuthRepo:    authRepo,
		AuditRepo:   au,
		AuditWriter: au,
	})

	body, _ := json.Marshal(map[string]any{
		"role": "hospital", "hcode": "12345", "name": "rpt-BKK",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	// Raw key must appear in the response once.
	if !bytes.Contains(w.Body.Bytes(), []byte(`"raw_key":"nck_`)) {
		t.Fatalf("raw_key missing from response")
	}

	entries := au.snapshot()
	if len(entries) != 1 {
		t.Fatalf("want 1 audit row, got %d", len(entries))
	}
	e := entries[0]
	if e.Action != "api_key.create" {
		t.Errorf("action = %q, want api_key.create", e.Action)
	}
	if e.TargetKind != "api_key" || e.TargetID == "" {
		t.Errorf("target mismatch: kind=%q id=%q", e.TargetKind, e.TargetID)
	}
	if e.HCode != "12345" {
		t.Errorf("hcode = %q, want 12345", e.HCode)
	}
	// Payload must carry role+hcode+name, NEVER raw_key or hash.
	if r, _ := e.Payload["role"].(string); r != "hospital" {
		t.Errorf("payload role missing/wrong: %+v", e.Payload)
	}
	if h, _ := e.Payload["hcode"].(string); h != "12345" {
		t.Errorf("payload hcode missing: %+v", e.Payload)
	}
	if n, _ := e.Payload["name"].(string); n != "rpt-BKK" {
		t.Errorf("payload name missing: %+v", e.Payload)
	}
	if _, ok := e.Payload["raw_key"]; ok {
		t.Errorf("raw_key leaked into audit payload: %+v", e.Payload)
	}
	if _, ok := e.Payload["key_hash"]; ok {
		t.Errorf("key_hash leaked into audit payload: %+v", e.Payload)
	}
}

func TestAuditAPI_DeactivateWritesAuditRow(t *testing.T) {
	authRepo := newMemAuthRepo()
	_ = authRepo.mint(auth.RoleAdmin, "", "to-deactivate")
	items, _ := authRepo.List(context.Background())
	id := items[0].ID

	au := newMemAuditRepo()
	h := server.New(server.Deps{
		AuthRepo:    authRepo,
		AuditRepo:   au,
		AuditWriter: au,
	})

	body, _ := json.Marshal(map[string]any{"is_active": false})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/keys/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	entries := au.snapshot()
	if len(entries) != 1 {
		t.Fatalf("want 1 audit row, got %d", len(entries))
	}
	e := entries[0]
	if e.Action != "api_key.deactivate" {
		t.Errorf("action = %q, want api_key.deactivate", e.Action)
	}
	if e.TargetKind != "api_key" || e.TargetID != id {
		t.Errorf("target mismatch: kind=%q id=%q want id=%q", e.TargetKind, e.TargetID, id)
	}
	prev, prevOK := e.Payload["prev_is_active"].(bool)
	next, nextOK := e.Payload["next_is_active"].(bool)
	if !prevOK || !nextOK || prev != true || next != false {
		t.Errorf("payload prev/next mismatch: %+v", e.Payload)
	}
}

func TestAuditAPI_CCodeResolveWritesAuditRow(t *testing.T) {
	cc := newMemCCodeRepo()
	seededID, _ := cc.Insert(context.Background(), store.CCodeInsert{
		BatchID: "B1", HN: "HN001", ANOrSEQ: "S001",
		CCode: "C104", CDesc: "PERSON_ID ผิด",
	})
	cc.hcodeByID[seededID] = "12345"

	au := newMemAuditRepo()
	h := server.New(server.Deps{
		CCodeRepo:   cc,
		AuditRepo:   au,
		AuditWriter: au,
	})

	body, _ := json.Marshal(map[string]string{"resolved_by": "operator@nexclaim"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/ccodes/"+seededID+"/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d body=%s", w.Code, w.Body.String())
	}

	entries := au.snapshot()
	if len(entries) != 1 {
		t.Fatalf("want 1 audit row, got %d", len(entries))
	}
	e := entries[0]
	if e.Action != "ccode.resolve" {
		t.Errorf("action = %q, want ccode.resolve", e.Action)
	}
	if e.TargetKind != "ccode" || e.TargetID != seededID {
		t.Errorf("target mismatch: kind=%q id=%q", e.TargetKind, e.TargetID)
	}
	if e.HCode != "12345" {
		t.Errorf("hcode expected 12345, got %q", e.HCode)
	}
	if rb, _ := e.Payload["resolved_by"].(string); rb != "operator@nexclaim" {
		t.Errorf("payload resolved_by mismatch: %+v", e.Payload)
	}
}

// Audit-write failure must NOT bubble up into the originating action.
func TestAuditWriteFailure_DoesNotBreakAction(t *testing.T) {
	authRepo := newMemAuthRepo()
	au := newMemAuditRepo()
	au.writeErr = fmt.Errorf("simulated audit backend down")

	h := server.New(server.Deps{
		AuthRepo:    authRepo,
		AuditRepo:   au,
		AuditWriter: au,
	})

	body, _ := json.Marshal(map[string]any{"role": "admin", "name": "ops-laptop"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create-key must succeed even with audit down, got %d body=%s", w.Code, w.Body.String())
	}
	if len(au.snapshot()) != 0 {
		t.Errorf("writeErr fake should have stored zero rows")
	}
}

// ── Pg integration ─────────────────────────────────────────────────────────

func TestAuditRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping Pg audit test")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	// Clean prior test rows (no TRUNCATE — RDS CRUD-only + audit is append-only;
	// we only use DELETE in tests for cleanup).
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM audit_log WHERE action LIKE 'test.%'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(),
			`DELETE FROM audit_log WHERE action LIKE 'test.%'`)
	})

	repo := store.NewPgAuditRepo(conn)

	// Insert 2 rows.
	if err := repo.Write(ctx, audit.Entry{
		ActorRole: "admin", ActorName: "pg-test-admin",
		Action: "test.one", TargetKind: "api_key", TargetID: "K1",
		HCode:   "12345",
		Payload: map[string]any{"k": "v", "n": 42},
	}); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	if err := repo.Write(ctx, audit.Entry{
		ActorRole: "system", ActorName: "bootstrap",
		Action: "test.two", TargetKind: "rep", TargetID: "12345/202504",
		HCode: "12345",
	}); err != nil {
		t.Fatalf("write 2: %v", err)
	}

	// List with filter on action prefix — we can't do LIKE here, so use
	// exact match on the second test action to exercise filtering.
	rows, err := repo.List(ctx, store.AuditFilter{Action: "test.two"})
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if len(rows) != 1 || rows[0].Action != "test.two" {
		t.Errorf("filtered list mismatch: %+v", rows)
	}

	// List all then filter in-memory to confirm both rows made it in.
	all, err := repo.List(ctx, store.AuditFilter{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	found := map[string]bool{}
	for _, r := range all {
		if r.Action == "test.one" || r.Action == "test.two" {
			found[r.Action] = true
		}
	}
	if !found["test.one"] || !found["test.two"] {
		t.Errorf("expected both seeded rows present, got %+v", found)
	}

	// Spot-check payload decoding on the first row (JSONB → map).
	rows, err = repo.List(ctx, store.AuditFilter{Action: "test.one"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("list test.one: %v, rows=%+v", err, rows)
	}
	if v, _ := rows[0].Payload["k"].(string); v != "v" {
		t.Errorf("payload k mismatch: %+v", rows[0].Payload)
	}
}
