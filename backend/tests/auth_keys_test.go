package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

// apiKeyDTO mirrors server.APIKeyDTO without importing it so the test
// doesn't bind tightly to unexported helpers. Field names must match the
// JSON contract (`id`, `name`, `role`, `hcode`, `is_active`, `created_at`,
// `last_used_at`, `raw_key`).
type apiKeyDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	HCode      *string `json:"hcode,omitempty"`
	IsActive   bool    `json:"is_active"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	RawKey     string  `json:"raw_key,omitempty"`
}

// ── POST /api/v1/auth/keys — auth disabled (pass-through) ──

func TestAPIKeys_CreateAdmin_OK(t *testing.T) {
	repo := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: repo}) // AuthEnabled=false → pass-through

	body, _ := json.Marshal(map[string]any{
		"role": "admin", "name": "ops-laptop",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var dto apiKeyDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.Role != "admin" || dto.Name != "ops-laptop" {
		t.Errorf("identity mismatch: %+v", dto)
	}
	if dto.HCode != nil {
		t.Errorf("admin should not have hcode, got %v", *dto.HCode)
	}
	if !strings.HasPrefix(dto.RawKey, "nck_") {
		t.Errorf("raw_key missing or malformed: %q", dto.RawKey)
	}
	if !dto.IsActive {
		t.Errorf("new key should be active")
	}
}

func TestAPIKeys_CreateHospital_OK(t *testing.T) {
	repo := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: repo})

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
	var dto apiKeyDTO
	_ = json.Unmarshal(w.Body.Bytes(), &dto)
	if dto.Role != "hospital" || dto.HCode == nil || *dto.HCode != "12345" {
		t.Errorf("hospital identity mismatch: %+v", dto)
	}
	if dto.RawKey == "" {
		t.Errorf("raw_key missing on create")
	}
}

func TestAPIKeys_CreateHospital_MissingHcode_400(t *testing.T) {
	repo := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: repo})

	body, _ := json.Marshal(map[string]any{
		"role": "hospital", "name": "rpt-BKK",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hcode") {
		t.Errorf("expected hcode in error, got %s", w.Body.String())
	}
}

func TestAPIKeys_CreateAdmin_WithHcode_400(t *testing.T) {
	repo := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: repo})

	body, _ := json.Marshal(map[string]any{
		"role": "admin", "hcode": "12345", "name": "ops",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAPIKeys_CreateEmptyName_400(t *testing.T) {
	repo := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: repo})

	body, _ := json.Marshal(map[string]any{"role": "admin", "name": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for empty name, got %d", w.Code)
	}
}

func TestAPIKeys_CreateInvalidRole_400(t *testing.T) {
	repo := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: repo})

	body, _ := json.Marshal(map[string]any{"role": "superuser", "name": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid role, got %d", w.Code)
	}
}

// ── GET /api/v1/auth/keys ──

func TestAPIKeys_List_NoRawKey(t *testing.T) {
	repo := newMemAuthRepo()
	_ = repo.mint(auth.RoleAdmin, "", "admin-1")
	_ = repo.mint(auth.RoleHospital, "12345", "hosp-1")
	h := server.New(server.Deps{AuthRepo: repo})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/keys", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []apiKeyDTO `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(resp.Items))
	}
	for _, it := range resp.Items {
		if it.RawKey != "" {
			t.Errorf("raw_key must never appear in list: %+v", it)
		}
	}
	// Also confirm via raw-string check — catches any JSON-renaming regression.
	if strings.Contains(w.Body.String(), "nck_") {
		t.Errorf("list body must not contain any raw nck_ keys: %s", w.Body.String())
	}
}

func TestAPIKeys_List_IncludesInactive(t *testing.T) {
	repo := newMemAuthRepo()
	_ = repo.mint(auth.RoleAdmin, "", "admin-1")
	raw2 := repo.mint(auth.RoleAdmin, "", "admin-2")
	_ = raw2
	// Flip one inactive by looking up its id via Count-aware walk.
	items, _ := repo.List(context.Background())
	var targetID string
	for _, it := range items {
		if it.Name == "admin-2" {
			targetID = it.ID
		}
	}
	if _, err := repo.SetActive(context.Background(), targetID, false); err != nil {
		t.Fatalf("set inactive: %v", err)
	}

	h := server.New(server.Deps{AuthRepo: repo})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/keys", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp struct {
		Items []apiKeyDTO `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Items) != 2 {
		t.Fatalf("list should include inactive, got %d", len(resp.Items))
	}
	var sawInactive bool
	for _, it := range resp.Items {
		if !it.IsActive {
			sawInactive = true
		}
	}
	if !sawInactive {
		t.Errorf("expected at least one inactive row in list")
	}
}

// ── PATCH /api/v1/auth/keys/:id ──

func TestAPIKeys_Patch_Deactivate_200(t *testing.T) {
	repo := newMemAuthRepo()
	_ = repo.mint(auth.RoleAdmin, "", "to-deactivate")
	items, _ := repo.List(context.Background())
	id := items[0].ID

	h := server.New(server.Deps{AuthRepo: repo})

	body, _ := json.Marshal(map[string]any{"is_active": false})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/keys/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var dto apiKeyDTO
	_ = json.Unmarshal(w.Body.Bytes(), &dto)
	if dto.IsActive {
		t.Errorf("expected is_active=false after patch, got %+v", dto)
	}
	if dto.RawKey != "" {
		t.Errorf("PATCH response must not carry raw_key")
	}
}

func TestAPIKeys_Patch_UnknownID_404(t *testing.T) {
	repo := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: repo})

	body, _ := json.Marshal(map[string]any{"is_active": false})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/keys/id-nobody", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAPIKeys_Patch_MissingIsActive_400(t *testing.T) {
	repo := newMemAuthRepo()
	_ = repo.mint(auth.RoleAdmin, "", "x")
	items, _ := repo.List(context.Background())
	id := items[0].ID
	h := server.New(server.Deps{AuthRepo: repo})

	body, _ := json.Marshal(map[string]any{}) // no is_active key
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/keys/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// ── RequireRole("admin") enforcement ──

func TestAPIKeys_HospitalCallerForbidden(t *testing.T) {
	repo := newMemAuthRepo()
	hospKey := repo.mint(auth.RoleHospital, "12345", "hosp")
	h := server.New(server.Deps{AuthRepo: repo, AuthEnabled: true})

	// LIST
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/keys", nil)
	req.Header.Set("Authorization", "Bearer "+hospKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("GET want 403, got %d", w.Code)
	}

	// POST
	body, _ := json.Marshal(map[string]any{"role": "admin", "name": "bogus"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+hospKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("POST want 403, got %d", w.Code)
	}

	// PATCH
	patch, _ := json.Marshal(map[string]any{"is_active": false})
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/auth/keys/any-id", bytes.NewReader(patch))
	req.Header.Set("Authorization", "Bearer "+hospKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("PATCH want 403, got %d", w.Code)
	}
}

func TestAPIKeys_AdminCallerAllowed(t *testing.T) {
	repo := newMemAuthRepo()
	adminKey := repo.mint(auth.RoleAdmin, "", "admin-caller")
	h := server.New(server.Deps{AuthRepo: repo, AuthEnabled: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/keys", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin GET want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// ── Pg integration — e2e HTTP through real PgAPIKeyRepo ──

func TestAPIKeyHandlers_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping Pg api-key handler test")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()

	// Clean prior test rows (no TRUNCATE — RDS CRUD-only).
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM api_key WHERE name LIKE 'test-apikey-%'`); err != nil {
		t.Fatalf("cleanup api_key: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}
	// Cleanup on exit.
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(),
			`DELETE FROM api_key WHERE name LIKE 'test-apikey-%'`)
	})

	repo := store.NewPgAPIKeyRepo(conn)
	h := server.New(server.Deps{AuthRepo: repo})

	// POST admin key.
	body, _ := json.Marshal(map[string]any{
		"role": "admin", "name": "test-apikey-admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST admin want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var admin apiKeyDTO
	_ = json.Unmarshal(w.Body.Bytes(), &admin)
	if !strings.HasPrefix(admin.RawKey, "nck_") {
		t.Errorf("raw admin key malformed: %q", admin.RawKey)
	}
	adminID := admin.ID

	// POST hospital key.
	body, _ = json.Marshal(map[string]any{
		"role": "hospital", "hcode": "12345", "name": "test-apikey-hosp",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST hospital want 200, got %d body=%s", w.Code, w.Body.String())
	}

	// POST hospital w/ unknown hcode → 400.
	body, _ = json.Marshal(map[string]any{
		"role": "hospital", "hcode": "99999", "name": "test-apikey-bad",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown hcode want 400, got %d body=%s", w.Code, w.Body.String())
	}

	// GET list → 2 rows, newest-first, no raw_key.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/keys", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET list want 200, got %d", w.Code)
	}
	var resp struct {
		Items []apiKeyDTO `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	// Filter to our test rows so other seed data doesn't flake the count.
	testRows := 0
	for _, it := range resp.Items {
		if strings.HasPrefix(it.Name, "test-apikey-") {
			testRows++
			if it.RawKey != "" {
				t.Errorf("raw_key leaked in list: %+v", it)
			}
		}
	}
	if testRows != 2 {
		t.Errorf("want 2 test rows, got %d (all items: %+v)", testRows, resp.Items)
	}

	// PATCH deactivate admin.
	patch, _ := json.Marshal(map[string]any{"is_active": false})
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/auth/keys/"+adminID, bytes.NewReader(patch))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var patched apiKeyDTO
	_ = json.Unmarshal(w.Body.Bytes(), &patched)
	if patched.IsActive {
		t.Errorf("expected inactive after patch")
	}

	// GET again — inactive row still listed.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/keys", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp2 struct {
		Items []apiKeyDTO `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp2)
	var sawInactive bool
	for _, it := range resp2.Items {
		if it.ID == adminID && !it.IsActive {
			sawInactive = true
		}
	}
	if !sawInactive {
		t.Errorf("inactive admin should still appear in list")
	}

	// PATCH unknown id → 404 (uuid-shaped to avoid pg cast error).
	req = httptest.NewRequest(http.MethodPatch,
		"/api/v1/auth/keys/00000000-0000-0000-0000-000000000000",
		bytes.NewReader(patch))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("PATCH unknown want 404, got %d body=%s", w.Code, w.Body.String())
	}
}
