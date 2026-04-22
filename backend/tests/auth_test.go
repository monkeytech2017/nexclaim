package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
)

// ── in-memory auth.Repo for handler tests ──

type memAuthRepo struct {
	mu    sync.Mutex
	byID  map[string]*auth.Identity
	hash  map[string]string // key_hash → id
}

func newMemAuthRepo() *memAuthRepo {
	return &memAuthRepo{
		byID: map[string]*auth.Identity{},
		hash: map[string]string{},
	}
}

func (m *memAuthRepo) GetByHash(_ context.Context, h string) (*auth.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.hash[h]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *m.byID[id]
	return &cp, nil
}

func (m *memAuthRepo) UpdateLastUsed(_ context.Context, _ string) error { return nil }

func (m *memAuthRepo) Insert(_ context.Context, in auth.Insert) (*auth.Identity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := "id-" + in.Name
	ident := &auth.Identity{ID: id, Role: in.Role, HCode: in.HCode, Name: in.Name}
	m.byID[id] = ident
	m.hash[in.KeyHash] = id
	return ident, nil
}

func (m *memAuthRepo) Count(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.byID), nil
}

// mint creates+inserts a key the same way the CLI would, and returns the raw
// token the caller must send. Convenient for setting up test fixtures.
func (m *memAuthRepo) mint(role, hcode, name string) string {
	raw, hash, _ := auth.GenerateKey()
	_, _ = m.Insert(context.Background(), auth.Insert{
		KeyHash: hash, Role: role, HCode: hcode, Name: name,
	})
	return raw
}

// ── middleware behaviour ──

func TestAuthMiddleware_Disabled_PassThrough(t *testing.T) {
	repo := newMemClaimBatchRepo() // fake from submissions_test.go
	h := server.New(server.Deps{
		ClaimBatchRepo: repo,
		// AuthEnabled: false (zero value) → pass-through
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disabled auth should pass through, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_NoAuth_401(t *testing.T) {
	ar := newMemAuthRepo()
	h := server.New(server.Deps{
		AuthRepo: ar, AuthEnabled: true,
		ClaimBatchRepo: newMemClaimBatchRepo(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_InvalidKey_401(t *testing.T) {
	ar := newMemAuthRepo()
	h := server.New(server.Deps{
		AuthRepo: ar, AuthEnabled: true,
		ClaimBatchRepo: newMemClaimBatchRepo(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches", nil)
	req.Header.Set("Authorization", "Bearer nck_deadbeefnotreal")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidKey_200(t *testing.T) {
	ar := newMemAuthRepo()
	adminKey := ar.mint(auth.RoleAdmin, "", "test-admin")
	h := server.New(server.Deps{
		AuthRepo: ar, AuthEnabled: true,
		ClaimBatchRepo: newMemClaimBatchRepo(
			store.ClaimBatchRow{BatchID: "B1", HCode: "12345", Period: "202504", INSCL: "UCS", Format: "16FILES", Status: "sent"},
		),
	})

	// Check whoami returns the identity from context.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("whoami: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var who auth.Identity
	if err := json.Unmarshal(w.Body.Bytes(), &who); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if who.Role != auth.RoleAdmin || who.Name != "test-admin" {
		t.Errorf("identity mismatch: %+v", who)
	}

	// And the authed list endpoint actually returns data.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin list: want 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ── scope enforcement ──

func TestAuthScope_HospitalWrongHcode_403(t *testing.T) {
	ar := newMemAuthRepo()
	hospKey := ar.mint(auth.RoleHospital, "12345", "test-hospital")
	h := server.New(server.Deps{
		AuthRepo: ar, AuthEnabled: true,
		ClaimBatchRepo: newMemClaimBatchRepo(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches?hcode=99999", nil)
	req.Header.Set("Authorization", "Bearer "+hospKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthScope_HospitalOwnHcode_200(t *testing.T) {
	ar := newMemAuthRepo()
	hospKey := ar.mint(auth.RoleHospital, "12345", "test-hospital")
	h := server.New(server.Deps{
		AuthRepo: ar, AuthEnabled: true,
		ClaimBatchRepo: newMemClaimBatchRepo(
			store.ClaimBatchRow{BatchID: "B1", HCode: "12345", Period: "202504", INSCL: "UCS", Format: "16FILES", Status: "sent"},
		),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches?hcode=12345", nil)
	req.Header.Set("Authorization", "Bearer "+hospKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("own hcode should pass, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthScope_AdminAnyHcode_200(t *testing.T) {
	ar := newMemAuthRepo()
	adminKey := ar.mint(auth.RoleAdmin, "", "admin")
	h := server.New(server.Deps{
		AuthRepo: ar, AuthEnabled: true,
		ClaimBatchRepo: newMemClaimBatchRepo(
			store.ClaimBatchRow{BatchID: "B1", HCode: "99999", Period: "202504", INSCL: "UCS", Format: "16FILES", Status: "sent"},
		),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/claim/batches?hcode=99999", nil)
	req.Header.Set("Authorization", "Bearer "+adminKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin should hit any hcode, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthScope_HospitalCannotHitMaster(t *testing.T) {
	ar := newMemAuthRepo()
	hospKey := ar.mint(auth.RoleHospital, "12345", "hosp")
	h := server.New(server.Deps{
		AuthRepo: ar, AuthEnabled: true,
		HospitalRepo: newMemHospitalRepo(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/master/hospitals", nil)
	req.Header.Set("Authorization", "Bearer "+hospKey)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("hospital shouldn't reach /master/*, got %d", w.Code)
	}
}

// ── bootstrap ──

func TestAuthBootstrap_EmptyTable_Mints(t *testing.T) {
	ar := newMemAuthRepo()
	t.Setenv("AUTH_BOOTSTRAP_TOKEN", "super-secret")
	h := server.New(server.Deps{AuthRepo: ar, AuthEnabled: true})

	body := strings.NewReader(`{"name":"first-admin"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/bootstrap", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "super-secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID, Name, Role, Key string
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Role != auth.RoleAdmin || resp.Name != "first-admin" {
		t.Errorf("bad identity: %+v", resp)
	}
	if !strings.HasPrefix(resp.Key, "nck_") || len(resp.Key) < 40 {
		t.Errorf("key looks wrong: %q", resp.Key)
	}

	// And now the key actually works end-to-end.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+resp.Key)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("whoami with fresh key: want 200, got %d", w.Code)
	}
}

func TestAuthBootstrap_Populated_Rejects(t *testing.T) {
	ar := newMemAuthRepo()
	_ = ar.mint(auth.RoleAdmin, "", "already-here")
	t.Setenv("AUTH_BOOTSTRAP_TOKEN", "super-secret")
	h := server.New(server.Deps{AuthRepo: ar, AuthEnabled: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/bootstrap",
		bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", "super-secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("populated table should 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthBootstrap_WrongToken_401(t *testing.T) {
	ar := newMemAuthRepo()
	t.Setenv("AUTH_BOOTSTRAP_TOKEN", "super-secret")
	h := server.New(server.Deps{AuthRepo: ar, AuthEnabled: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/bootstrap",
		bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Bootstrap-Token", "wrong")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong bootstrap token: want 401, got %d", w.Code)
	}
}

// Healthz must always work, even with auth on.
func TestAuth_HealthzAlwaysPublic(t *testing.T) {
	ar := newMemAuthRepo()
	h := server.New(server.Deps{AuthRepo: ar, AuthEnabled: true})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz should be public, got %d", w.Code)
	}
}

// ── hash/generate helpers ──

func TestHashKey_Deterministic(t *testing.T) {
	a := auth.HashKey("nck_abc")
	b := auth.HashKey("nck_abc")
	if a != b {
		t.Errorf("hash not stable: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Errorf("hash length = %d, want 64", len(a))
	}
}

func TestGenerateKey_UniqueAndPrefixed(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 16; i++ {
		raw, hash, err := auth.GenerateKey()
		if err != nil {
			t.Fatalf("gen: %v", err)
		}
		if !strings.HasPrefix(raw, "nck_") {
			t.Errorf("prefix missing: %q", raw)
		}
		if len(hash) != 64 {
			t.Errorf("hash len %d", len(hash))
		}
		if seen[raw] {
			t.Errorf("collision: %s", raw)
		}
		seen[raw] = true
	}
}

// ── Pg integration ──

func TestAPIKeyRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()

	// Clean prior test rows (no TRUNCATE — DELETE by name pattern).
	if _, err := conn.ExecContext(ctx,
		`DELETE FROM api_key WHERE name LIKE 'test-%'`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	// Need a hospital for the FK on hospital role.
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	repo := store.NewPgAPIKeyRepo(conn)

	// Mint an admin + a hospital key through the normal path.
	rawA, hashA, _ := auth.GenerateKey()
	identA, err := repo.Insert(ctx, auth.Insert{
		KeyHash: hashA, Role: auth.RoleAdmin, Name: "test-admin-pg",
	})
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	if identA.HCode != "" || identA.Role != auth.RoleAdmin {
		t.Errorf("admin identity mismatch: %+v", identA)
	}

	rawH, hashH, _ := auth.GenerateKey()
	identH, err := repo.Insert(ctx, auth.Insert{
		KeyHash: hashH, Role: auth.RoleHospital, HCode: "12345", Name: "test-hosp-pg",
	})
	if err != nil {
		t.Fatalf("insert hospital: %v", err)
	}
	if identH.HCode != "12345" {
		t.Errorf("hospital hcode mismatch: %+v", identH)
	}

	// GetByHash round-trip.
	got, err := repo.GetByHash(ctx, auth.HashKey(rawA))
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if got.Name != "test-admin-pg" || got.Role != auth.RoleAdmin {
		t.Errorf("got admin: %+v", got)
	}

	got, err = repo.GetByHash(ctx, auth.HashKey(rawH))
	if err != nil {
		t.Fatalf("get hospital: %v", err)
	}
	if got.HCode != "12345" {
		t.Errorf("got hospital: %+v", got)
	}

	// UpdateLastUsed doesn't error.
	if err := repo.UpdateLastUsed(ctx, identA.ID); err != nil {
		t.Errorf("touch: %v", err)
	}

	// Count sees our new rows.
	n, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n < 2 {
		t.Errorf("count = %d, want >= 2", n)
	}

	// Validation: admin cannot have hcode.
	if _, err := repo.Insert(ctx, auth.Insert{
		KeyHash: auth.HashKey("nck_bogus"), Role: auth.RoleAdmin, HCode: "12345", Name: "test-bogus",
	}); err == nil {
		t.Error("admin+hcode should fail")
	}
	// Validation: hospital requires hcode.
	if _, err := repo.Insert(ctx, auth.Insert{
		KeyHash: auth.HashKey("nck_bogus2"), Role: auth.RoleHospital, Name: "test-bogus2",
	}); err == nil {
		t.Error("hospital without hcode should fail")
	}

	// Cleanup.
	_, _ = conn.ExecContext(ctx, `DELETE FROM api_key WHERE name LIKE 'test-%'`)
}
