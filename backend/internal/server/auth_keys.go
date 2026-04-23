package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/audit"
	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/store"
)

// APIKeyDTO is the wire format for /api/v1/auth/keys. RawKey is populated on
// POST only — every other response serializes it as the empty string (omitted
// via omitempty). Mirrors the shape the frontend consumes; do not reshape
// without coordinating with the frontend agent.
type APIKeyDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Role       string     `json:"role"`
	HCode      *string    `json:"hcode,omitempty"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RawKey     string     `json:"raw_key,omitempty"`
}

// toAPIKeyDTO projects an auth.Identity into the wire shape. hcode empty
// string → nil pointer so the JSON field is omitted for admin rows.
func toAPIKeyDTO(ident *auth.Identity) APIKeyDTO {
	dto := APIKeyDTO{
		ID:         ident.ID,
		Name:       ident.Name,
		Role:       ident.Role,
		IsActive:   ident.IsActive,
		CreatedAt:  ident.CreatedAt,
		LastUsedAt: ident.LastUsedAt,
		ExpiresAt:  ident.ExpiresAt,
	}
	if ident.HCode != "" {
		h := ident.HCode
		dto.HCode = &h
	}
	return dto
}

// createAPIKeyReq is the POST body. HCode is a plain string (not *string) so
// json binding + validation stays simple; empty string means "not supplied".
//
// TTLDays is optional: 0 / missing → no expiry; positive → expires_at set to
// now + TTLDays*24h; negative → 400. Audit payload carries ttl_days only when
// > 0 so the common "no expiry" case stays terse.
type createAPIKeyReq struct {
	Role    string `json:"role"`
	HCode   string `json:"hcode"`
	Name    string `json:"name"`
	TTLDays int    `json:"ttl_days,omitempty"`
}

// createAPIKeyHandler mints a new api_key row and returns the raw token.
// The raw key is shown here ONCE — every subsequent listing hides it.
// Validation rules are enforced here (not just in the repo) so we return
// clean 400s with targeted messages.
func createAPIKeyHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.AuthRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth repo not configured"})
			return
		}
		var req createAPIKeyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Role = strings.TrimSpace(req.Role)
		req.HCode = strings.TrimSpace(req.HCode)

		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
			return
		}
		if req.Role != auth.RoleAdmin && req.Role != auth.RoleHospital {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
		if req.Role == auth.RoleHospital && req.HCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hcode required for hospital role"})
			return
		}
		if req.Role == auth.RoleAdmin && req.HCode != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "admin role must not have hcode"})
			return
		}
		if req.TTLDays < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ttl_days must be >= 0"})
			return
		}

		// TTL > 0 → compute absolute expiry; 0 → nil (never expires).
		var expires *time.Time
		if req.TTLDays > 0 {
			t := time.Now().Add(time.Duration(req.TTLDays) * 24 * time.Hour)
			expires = &t
		}

		raw, hash, err := auth.GenerateKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ident, err := d.AuthRepo.Insert(c.Request.Context(), auth.Insert{
			KeyHash: hash, Role: req.Role, HCode: req.HCode, Name: req.Name,
			ExpiresAt: expires,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": translateAPIKeyInsertErr(err, req.HCode)})
			return
		}

		// Log mint event (no raw key, no hash). Stderr keeps this audit-
		// friendly for `journalctl`-style scraping without introducing a
		// dedicated logger dependency. Matches the persistRun pattern.
		actorID := ""
		if caller, ok := auth.FromContext(c); ok {
			actorID = caller.ID
		}
		fmt.Fprintf(os.Stderr,
			"[NexClaim] api_key mint: id=%s role=%s hcode=%q name=%q actor_id=%s\n",
			ident.ID, ident.Role, ident.HCode, ident.Name, actorID)

		// Audit: api_key.create — payload CAREFULLY excludes the raw key + hash.
		// ttl_days is included ONLY when > 0; zero/no-expiry is the common case,
		// omitting the field keeps the common payload tidy.
		actorRole, actorName, aid := audit.ActorFromContext(c)
		payload := map[string]any{
			"role":  ident.Role,
			"hcode": ident.HCode,
			"name":  ident.Name,
		}
		if req.TTLDays > 0 {
			payload["ttl_days"] = req.TTLDays
		}
		writeAudit(c.Request.Context(), d, audit.Entry{
			ActorID:    aid,
			ActorRole:  actorRole,
			ActorName:  actorName,
			Action:     "api_key.create",
			TargetKind: "api_key",
			TargetID:   ident.ID,
			HCode:      ident.HCode,
			Payload:    payload,
		})

		dto := toAPIKeyDTO(ident)
		dto.RawKey = raw
		c.JSON(http.StatusOK, dto)
	}
}

// translateAPIKeyInsertErr turns pg FK / constraint violations into a
// human-readable 400 message. We keep this as string-sniff instead of
// depending on pq.Error directly so fake repos can surface their own
// messages without importing pq.
func translateAPIKeyInsertErr(err error, hcode string) string {
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "foreign key") || strings.Contains(lower, "violates foreign key") {
		return fmt.Sprintf("hcode %q not found in m_hospital", hcode)
	}
	return msg
}

// listAPIKeysHandler returns all keys (active + inactive) sorted newest-first.
// RawKey is never populated; hash is never touched.
func listAPIKeysHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.AuthRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth repo not configured"})
			return
		}
		items, err := d.AuthRepo.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]APIKeyDTO, 0, len(items))
		for i := range items {
			out = append(out, toAPIKeyDTO(&items[i]))
		}
		c.JSON(http.StatusOK, gin.H{"items": out})
	}
}

// patchAPIKeyReq toggles the is_active flag. Pointer so we can distinguish
// "is_active omitted" (→ 400) from "is_active=false" (valid request).
type patchAPIKeyReq struct {
	IsActive *bool `json:"is_active"`
}

// patchAPIKeyHandler flips is_active on a single key. No other fields are
// mutable via this endpoint — key rotation is "create new + deactivate old".
func patchAPIKeyHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.AuthRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth repo not configured"})
			return
		}
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
			return
		}
		var req patchAPIKeyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.IsActive == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "is_active required"})
			return
		}
		// Best-effort pre-read so audit can record prev_is_active. List is
		// bounded + this path is admin-only + low-throughput, so the extra
		// round-trip is acceptable vs. extending the Repo interface.
		prevActive := !*req.IsActive // safe default if lookup misses
		if existing, err := d.AuthRepo.List(c.Request.Context()); err == nil {
			for i := range existing {
				if existing[i].ID == id {
					prevActive = existing[i].IsActive
					break
				}
			}
		}
		ident, err := d.AuthRepo.SetActive(c.Request.Context(), id, *req.IsActive)
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Audit: api_key.activate / api_key.deactivate.
		action := "api_key.activate"
		if !ident.IsActive {
			action = "api_key.deactivate"
		}
		actorRole, actorName, aid := audit.ActorFromContext(c)
		writeAudit(c.Request.Context(), d, audit.Entry{
			ActorID:    aid,
			ActorRole:  actorRole,
			ActorName:  actorName,
			Action:     action,
			TargetKind: "api_key",
			TargetID:   ident.ID,
			HCode:      ident.HCode,
			Payload: map[string]any{
				"prev_is_active": prevActive,
				"next_is_active": ident.IsActive,
			},
		})

		c.JSON(http.StatusOK, toAPIKeyDTO(ident))
	}
}
