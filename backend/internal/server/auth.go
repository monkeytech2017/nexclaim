package server

import (
	"context"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/audit"
	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/store"
)

// bootstrapHandler mints the FIRST admin key if and only if:
//
//  1. The AUTH_BOOTSTRAP_TOKEN env var is set (non-empty).
//  2. The inbound X-Bootstrap-Token header matches it exactly.
//  3. The api_key table is empty (Count == 0).
//
// After the first successful call, future calls fail at step 3 until someone
// manually empties the table. This is the only path a raw admin key ever
// leaves the server over HTTP — every subsequent key is minted via CLI.
func bootstrapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.AuthRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth repo not configured"})
			return
		}
		expected := os.Getenv("AUTH_BOOTSTRAP_TOKEN")
		if expected == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "bootstrap disabled (AUTH_BOOTSTRAP_TOKEN unset)"})
			return
		}
		if c.GetHeader("X-Bootstrap-Token") != expected {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "bootstrap token mismatch"})
			return
		}
		n, err := d.AuthRepo.Count(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if n > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "api_key table is not empty; bootstrap already performed"})
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Name == "" {
			body.Name = "bootstrap-admin"
		}

		raw, hash, err := auth.GenerateKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ident, err := d.AuthRepo.Insert(c.Request.Context(), auth.Insert{
			KeyHash: hash, Role: auth.RoleAdmin, Name: body.Name,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Audit: api_key.bootstrap — the only API-key event that has no
		// authenticated actor, so we stamp it as actor_role=system.
		writeAudit(c.Request.Context(), d, audit.Entry{
			ActorRole:  "system",
			ActorName:  "bootstrap",
			Action:     "api_key.bootstrap",
			TargetKind: "api_key",
			TargetID:   ident.ID,
			Payload: map[string]any{
				"role": ident.Role,
				"name": ident.Name,
			},
		})

		c.JSON(http.StatusCreated, gin.H{
			"id":   ident.ID,
			"name": ident.Name,
			"role": ident.Role,
			"key":  raw, // ← SHOWN ONCE. Caller must save it now.
			"note": "save this key — it will never be shown again",
		})
	}
}

// whoamiHandler echoes the caller's Identity (minus any secret). Useful for
// frontends that need to know "am I admin or hospital, and which hcode?".
// If auth is disabled, returns 204 — the client should treat that as
// "no identity available, treat as open".
func whoamiHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ident, ok := auth.FromContext(c)
		if !ok {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusOK, ident)
	}
}

// claimBatchHcodeAdapter adapts store.ClaimBatchRepo to auth.BatchHcodeResolver
// so the auth package can stay store-free. The wrapper only reads the HCode
// field from the row and drops the rest.
type claimBatchHcodeAdapter struct{ repo store.ClaimBatchRepo }

func (a claimBatchHcodeAdapter) Get(ctx context.Context, batchID string) (auth.BatchHcodeRow, error) {
	row, err := a.repo.Get(ctx, batchID)
	if err != nil {
		return auth.BatchHcodeRow{}, err
	}
	if row == nil {
		return auth.BatchHcodeRow{}, store.ErrNotFound
	}
	return auth.BatchHcodeRow{BatchID: row.BatchID, HCode: row.HCode}, nil
}

// newBatchResolver returns nil if the repo is unset (middleware becomes a
// pass-through — the handler's own 503 path will fire instead).
func newBatchResolver(repo store.ClaimBatchRepo) auth.BatchHcodeResolver {
	if repo == nil {
		return nil
	}
	return claimBatchHcodeAdapter{repo: repo}
}

// ccodeHcodeAdapter adapts store.CCodeRepo to auth.CCodeHcodeResolver. Same
// store-free pattern as the batch adapter.
type ccodeHcodeAdapter struct{ repo store.CCodeRepo }

func (a ccodeHcodeAdapter) GetHcode(ctx context.Context, id string) (string, error) {
	return a.repo.GetHcode(ctx, id)
}

func newCCodeResolver(repo store.CCodeRepo) auth.CCodeHcodeResolver {
	if repo == nil {
		return nil
	}
	return ccodeHcodeAdapter{repo: repo}
}
