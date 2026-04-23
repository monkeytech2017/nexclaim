// Package auth gates every non-public NexClaim endpoint behind an API key.
//
// Two roles exist:
//
//	admin    — unrestricted; may touch any hospital's data.
//	hospital — bound to exactly one hcode; requests that mention a different
//	           hcode (path param, query param, or the hcode of the batchId
//	           they reference) are rejected with 403.
//
// Callers send the raw key in the Authorization header:
//
//	Authorization: Bearer nck_<40 hex chars>
//
// We store only SHA-256(key); the raw key is shown ONCE at creation time and
// never logged. A short "nck_" prefix exists purely as a visual hint for
// operators reading logs — it is NOT treated as a secret.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Identity is the authenticated caller's profile. It is attached to the
// gin.Context via c.Set(IdentityKey, ...) once Middleware accepts a request.
//
// HCode is empty for admin keys and non-empty for hospital keys.
// IsActive/CreatedAt/LastUsedAt are populated for list/admin endpoints; the
// middleware path only reads ID/Role/HCode/Name so these extra fields are a
// harmless zero on the hot path.
type Identity struct {
	ID         string     `json:"id"`
	Role       string     `json:"role"`
	HCode      string     `json:"hcode,omitempty"`
	Name       string     `json:"name"`
	IsActive   bool       `json:"is_active,omitempty"`
	CreatedAt  time.Time  `json:"created_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// Role constants — string literals match the DB CHECK constraint.
const (
	RoleAdmin    = "admin"
	RoleHospital = "hospital"
)

// IdentityKey is the gin.Context key under which the Identity is stored.
const IdentityKey = "identity"

// keyPrefix is the human-facing tag prepended to every raw key. It has no
// security role — it exists so operators can tell a NexClaim key from a
// random string at a glance. SHA-256 is computed over the full raw string
// including this prefix.
const keyPrefix = "nck_"

// keyHexLen is the hex length of the random portion (40 hex = 20 bytes entropy).
const keyHexLen = 40

// ErrInvalidKey is returned when the bearer token doesn't match any active row.
var ErrInvalidKey = errors.New("invalid or inactive api key")

// HashKey returns the hex SHA-256 of the raw key. Safe to call on untrusted
// input — length is always 64 chars, matches the DB column width.
func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GenerateKey mints a new random key. Returns the raw key (to hand to the
// operator once) and the hash (to persist). Crypto-random; never log the raw.
func GenerateKey() (raw string, hash string, err error) {
	buf := make([]byte, keyHexLen/2) // 20 bytes → 40 hex
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("auth: generate key: %w", err)
	}
	raw = keyPrefix + hex.EncodeToString(buf)
	return raw, HashKey(raw), nil
}

// Insert is the payload for Repo.Insert. The caller is responsible for
// hashing the raw key before calling Insert — keep the raw key out of Repo.
type Insert struct {
	KeyHash string
	Role    string
	HCode   string // empty for admin
	Name    string
}

// Repo is the persistence contract for api_key. Implementations must:
//   - GetByHash only return active rows (is_active = true).
//   - UpdateLastUsed be safe to call best-effort; error is logged but
//     never blocks a request.
//   - Count return the total number of rows (active + inactive) — used to
//     gate the bootstrap endpoint.
type Repo interface {
	GetByHash(ctx context.Context, hash string) (*Identity, error)
	UpdateLastUsed(ctx context.Context, id string) error
	Insert(ctx context.Context, in Insert) (*Identity, error)
	Count(ctx context.Context) (int, error)
	// List returns every api_key row (active + inactive). Used by the admin
	// management UI; never exposes raw keys or hashes.
	List(ctx context.Context) ([]Identity, error)
	// SetActive flips is_active on a single row and returns the fresh
	// Identity. Implementations return store.ErrNotFound (or an equivalent)
	// when the id doesn't exist.
	SetActive(ctx context.Context, id string, active bool) (*Identity, error)
}

// ── middleware ──

// Middleware validates the bearer token and attaches Identity to the context.
// When enabled == false OR repo == nil it is a pass-through — this is the
// back-compat path for dev laptops without a DB. Callers that still want
// stricter posture can wrap with RequireRole/RequireHcodeMatch downstream
// (those also treat a missing Identity as "auth disabled → allow").
func Middleware(repo Repo, enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled || repo == nil {
			c.Next()
			return
		}
		raw := extractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			abort(c, http.StatusUnauthorized, "missing Authorization bearer token")
			return
		}
		ident, err := repo.GetByHash(c.Request.Context(), HashKey(raw))
		if err != nil || ident == nil {
			abort(c, http.StatusUnauthorized, "invalid or inactive api key")
			return
		}
		c.Set(IdentityKey, ident)
		// Best-effort last_used_at bump — never block the request on it.
		go func(id string) {
			_ = repo.UpdateLastUsed(context.Background(), id)
		}(ident.ID)
		c.Next()
	}
}

// RequireRole rejects callers whose role is not the one required.
// admin always passes hospital-required routes (admin is a superset).
// If no Identity is on the context (auth disabled), RequireRole is a no-op.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ident, ok := FromContext(c)
		if !ok {
			c.Next()
			return
		}
		if ident.Role == RoleAdmin {
			c.Next()
			return
		}
		if ident.Role != role {
			abort(c, http.StatusForbidden, fmt.Sprintf("requires role=%q", role))
			return
		}
		c.Next()
	}
}

// RequireHcodeMatch enforces that a hospital-scoped caller's bound hcode
// matches the hcode in the request. param is the gin path-param name to
// read first (typically "hcode"); if that param is empty, we fall back to
// the same-named query string value. admin passes through. No-op when auth
// is disabled (no Identity on the context).
func RequireHcodeMatch(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ident, ok := FromContext(c)
		if !ok {
			c.Next()
			return
		}
		if ident.Role == RoleAdmin {
			c.Next()
			return
		}
		want := c.Param(param)
		if want == "" {
			want = c.Query(param)
		}
		// No hcode on the request at all → hospital-scoped listings should
		// be implicitly scoped to the caller's hcode. We inject it as a
		// query override so downstream handlers see the correct filter.
		if want == "" {
			c.Request.URL.RawQuery = injectQuery(c.Request.URL.RawQuery, param, ident.HCode)
			c.Next()
			return
		}
		if want != ident.HCode {
			abort(c, http.StatusForbidden, fmt.Sprintf("hospital key bound to %q cannot access hcode %q", ident.HCode, want))
			return
		}
		c.Next()
	}
}

// BatchHcodeResolver is satisfied by store.ClaimBatchRepo (Get returns
// *store.ClaimBatchRow whose HCode field we read). Declared as an interface
// here with a structural-ish signature so tests can inject a fake without
// depending on the full store API.
type BatchHcodeResolver interface {
	Get(ctx context.Context, batchID string) (BatchHcodeRow, error)
}

// BatchHcodeRow is the minimal row shape RequireBatchHcodeMatch reads. The
// adapter in server/ wraps store.ClaimBatchRepo into this shape so the auth
// package stays free of a store dependency.
type BatchHcodeRow struct {
	BatchID string
	HCode   string
}

// RequireBatchHcodeMatch reads :batchId from the path, looks up its hcode
// via the supplied resolver, and rejects hospital callers whose bound
// hcode differs. admin passes. No-op if auth is disabled or resolver is nil.
func RequireBatchHcodeMatch(resolver BatchHcodeResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		ident, ok := FromContext(c)
		if !ok || resolver == nil {
			c.Next()
			return
		}
		if ident.Role == RoleAdmin {
			c.Next()
			return
		}
		batchID := c.Param("batchId")
		if batchID == "" {
			c.Next()
			return
		}
		row, err := resolver.Get(c.Request.Context(), batchID)
		if err != nil {
			// If the batch doesn't exist we let the handler return its
			// own 404 rather than leaking existence via a 403 here. But
			// if we know the hcode mismatches, we block.
			c.Next()
			return
		}
		if row.HCode != "" && row.HCode != ident.HCode {
			abort(c, http.StatusForbidden, fmt.Sprintf("batch %s belongs to a different hospital", batchID))
			return
		}
		c.Next()
	}
}

// ── helpers ──

// FromContext reads the Identity back out. ok=false means auth was disabled
// or the request slipped past the middleware (which should never happen on
// a gated route).
func FromContext(c *gin.Context) (*Identity, bool) {
	v, ok := c.Get(IdentityKey)
	if !ok {
		return nil, false
	}
	ident, ok := v.(*Identity)
	if !ok || ident == nil {
		return nil, false
	}
	return ident, true
}

func extractBearer(h string) string {
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func abort(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, gin.H{"error": msg})
}

// injectQuery sets key=value in a raw query string, replacing any existing
// occurrence. Simple string-mode manipulation — avoids pulling url.Values
// for a two-field rewrite.
func injectQuery(raw, key, value string) string {
	if value == "" {
		return raw
	}
	pair := key + "=" + value
	if raw == "" {
		return pair
	}
	// Remove any existing key=... occurrence.
	parts := strings.Split(raw, "&")
	out := parts[:0]
	prefix := key + "="
	for _, p := range parts {
		if strings.HasPrefix(p, prefix) {
			continue
		}
		out = append(out, p)
	}
	out = append(out, pair)
	return strings.Join(out, "&")
}

