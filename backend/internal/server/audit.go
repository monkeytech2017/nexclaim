package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

// GET /api/v1/audit-log?action=&actor_id=&hcode=&target_kind=&target_id=&from=&to=&limit=
//
// Admin-only. Returns append-only audit rows (newest-first, default 200).
// Time filters `from` / `to` parse RFC 3339; bad values are silently ignored
// so a flaky clock on the caller side doesn't 400 the whole page.
func listAuditLogHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.AuditRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "audit repo not configured"})
			return
		}
		f := store.AuditFilter{
			Action:     c.Query("action"),
			ActorID:    c.Query("actor_id"),
			HCode:      c.Query("hcode"),
			TargetKind: c.Query("target_kind"),
			TargetID:   c.Query("target_id"),
		}
		if raw := c.Query("from"); raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				f.From = t
			}
		}
		if raw := c.Query("to"); raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				f.To = t
			}
		}
		if raw := c.Query("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				f.Limit = n
			}
		}
		items, err := d.AuditRepo.List(c.Request.Context(), f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}
