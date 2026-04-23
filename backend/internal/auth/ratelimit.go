// In-memory, single-instance rate limiter. Revisit when deploying multi-replica
// (Redis/external store). Deliberately tiny: a token bucket per client IP, no
// Redis dependency, bounded to 10_000 active buckets to cap memory.
//
// Applied ONLY to hot security endpoints (bootstrap, whoami, keys POST) —
// every other route is untouched so production traffic doesn't see the lock.
//
// Token-bucket math:
//   burst     = perMin                (capacity)
//   refill    = perMin / 60 tokens/s  (steady state)
// A request costs 1 token. When tokens fall below 1 we deny with 429 +
// Retry-After: 60 and move on. No queueing, no back-pressure signalling.
package auth

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// maxBuckets caps the sync.Map entry count so a burst of distinct IPs cannot
// OOM the process. When we exceed this, we evict the oldest (by insertion
// order) until we're back under the cap. Good enough for a single-instance
// MVP; a production-grade LRU + metrics upgrade is on the "revisit" list.
const maxBuckets = 10_000

// bucket is the per-IP state. lastRefill tracks the last time we computed
// accrued tokens; we lazy-refill on demand (no background goroutine).
type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// RateLimiter is a per-IP token bucket. A nil *RateLimiter is valid and its
// Middleware is a pass-through — constructor returns nil for perMin <= 0.
type RateLimiter struct {
	perMin  int
	buckets sync.Map // map[string]*bucket

	// mu guards the FIFO insertion-order list used for overflow eviction.
	// sync.Map is great for the hot get/put path but doesn't expose "oldest",
	// so we keep a parallel slice. Only touched on insert + overflow —
	// trivially cheap compared to the HTTP round-trip.
	mu    sync.Mutex
	order []string
}

// NewRateLimiter returns a limiter, or nil when perMin <= 0 (disabled).
// A nil limiter is safe — its Middleware short-circuits.
func NewRateLimiter(perMin int) *RateLimiter {
	if perMin <= 0 {
		return nil
	}
	return &RateLimiter{
		perMin: perMin,
		order:  make([]string, 0, 64),
	}
}

// allow consumes 1 token for the given key. Refills first. Returns true when
// the request should proceed, false when the caller is over budget.
func (rl *RateLimiter) allow(key string, now time.Time) bool {
	if rl == nil {
		return true
	}
	// Fast path: bucket exists → update + test.
	if v, ok := rl.buckets.Load(key); ok {
		b := v.(*bucket)
		return rl.refillAndTake(b, now)
	}
	// First time seeing this IP: mint a full bucket (burst = perMin).
	newB := &bucket{tokens: float64(rl.perMin), lastRefill: now}
	actual, loaded := rl.buckets.LoadOrStore(key, newB)
	if loaded {
		// Raced with another goroutine; use theirs.
		return rl.refillAndTake(actual.(*bucket), now)
	}
	// We won the race — track insertion order for eviction.
	rl.mu.Lock()
	rl.order = append(rl.order, key)
	// Overflow trim: drop oldest entries until we're back under the cap.
	for len(rl.order) > maxBuckets {
		old := rl.order[0]
		rl.order = rl.order[1:]
		rl.buckets.Delete(old)
	}
	rl.mu.Unlock()
	return rl.refillAndTake(newB, now)
}

// refillAndTake runs the steady-state refill + consume-one step. We guard
// with a bucket-local mutex-less approach: the hot path is a single worker
// per key in practice, and over-refilling by a tick is harmless.
func (rl *RateLimiter) refillAndTake(b *bucket, now time.Time) bool {
	// Serialize per-bucket math: multiple goroutines on the same IP is rare
	// but possible. sync.Mutex on a struct we already hand out is simpler
	// than sync.Mutex-per-bucket plus coordination.
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		refill := elapsed * float64(rl.perMin) / 60.0
		b.tokens += refill
		if b.tokens > float64(rl.perMin) {
			b.tokens = float64(rl.perMin)
		}
		b.lastRefill = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Middleware returns a gin handler that allows or rejects. A nil receiver is
// a pass-through — callers can always wire the middleware unconditionally.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl == nil {
			c.Next()
			return
		}
		// gin's ClientIP() already honors X-Forwarded-For when the engine
		// has ForwardedByClientIP + TrustedProxies configured; with default
		// settings it falls back to the socket's RemoteAddr. We accept that
		// default here — deployers who need XFF parsing should set
		// engine.SetTrustedProxies accordingly.
		ip := c.ClientIP()
		if ip == "" {
			ip = c.Request.RemoteAddr
		}
		if !rl.allow(ip, time.Now()) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, retry in 60s",
			})
			return
		}
		c.Next()
	}
}
