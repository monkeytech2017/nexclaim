package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/server"
)

// These tests exercise auth.RateLimiter via the real server so we catch
// both the per-IP semantics and the "mounted only on hot paths" wiring.
// /api/v1/auth/whoami is the simplest target because it doesn't need a
// key to reach — auth middleware is a pass-through when AuthEnabled=false.

func TestRateLimit_Disabled(t *testing.T) {
	h := server.New(server.Deps{RateLimitPerMin: 0})

	// With perMin=0, NewRateLimiter returns nil → middleware is a pass-through.
	// 1000 requests from the same IP should all see 204 (whoami with no ident).
	for i := 0; i < 1000; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
		req.RemoteAddr = "1.2.3.4:1111"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("disabled limiter blocked at i=%d", i)
		}
	}
}

func TestRateLimit_Bucket(t *testing.T) {
	h := server.New(server.Deps{RateLimitPerMin: 5})

	// Burst = 5 → first 5 pass, 6th → 429.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
		req.RemoteAddr = "10.0.0.1:1111"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d unexpectedly throttled", i+1)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
	req.RemoteAddr = "10.0.0.1:1111"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("6th request: want 429, got %d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != "60" {
		t.Errorf("Retry-After header want 60, got %q", got)
	}
}

func TestRateLimit_PerIP(t *testing.T) {
	h := server.New(server.Deps{RateLimitPerMin: 2})

	hit := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
		req.RemoteAddr = ip + ":1111"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}

	// IP A: burn budget.
	if code := hit("10.0.0.1"); code == http.StatusTooManyRequests {
		t.Fatalf("A req1 throttled: %d", code)
	}
	if code := hit("10.0.0.1"); code == http.StatusTooManyRequests {
		t.Fatalf("A req2 throttled: %d", code)
	}
	if code := hit("10.0.0.1"); code != http.StatusTooManyRequests {
		t.Fatalf("A req3: want 429, got %d", code)
	}

	// IP B: fresh bucket — still allowed.
	if code := hit("10.0.0.2"); code == http.StatusTooManyRequests {
		t.Fatalf("B req1 unexpectedly throttled: %d — per-IP bucket leaked", code)
	}
	if code := hit("10.0.0.2"); code == http.StatusTooManyRequests {
		t.Fatalf("B req2 unexpectedly throttled: %d", code)
	}
}

// Sanity: rate-limiter is NOT applied to /healthz or hospital-scoped paths.
// Otherwise a burst of dashboard polls would DoS legit users. We probe a
// different endpoint with the same IP that just exhausted the keys-POST budget.
func TestRateLimit_OnlyHotPaths(t *testing.T) {
	h := server.New(server.Deps{RateLimitPerMin: 1})

	// Burn the whoami budget.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
	req.RemoteAddr = "10.0.0.5:1111"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/whoami", nil)
	req.RemoteAddr = "10.0.0.5:1111"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected whoami 429 after burn, got %d", w.Code)
	}

	// /healthz from same IP must still be free.
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "10.0.0.5:1111"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz got 429 — limiter leaked onto non-hot path: %d", w.Code)
	}
}

// Direct sanity for NewRateLimiter(0) returning a sentinel nil.
func TestNewRateLimiter_DisabledNil(t *testing.T) {
	if rl := auth.NewRateLimiter(0); rl != nil {
		t.Errorf("perMin=0 should return nil, got %+v", rl)
	}
	if rl := auth.NewRateLimiter(-5); rl != nil {
		t.Errorf("perMin=-5 should return nil, got %+v", rl)
	}
	if rl := auth.NewRateLimiter(10); rl == nil {
		t.Errorf("perMin=10 should return non-nil")
	}
}
