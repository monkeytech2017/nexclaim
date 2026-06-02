package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

// listTmtHandler serves GET /api/v1/master/tmt — read-only browse/search of the
// m_tmt_drug master. Response: { "items": []store.TmtDrug, "total": int }.
//
// Query params (all optional, parse errors → defaults):
//
//	q      case-insensitive substring on code/name_th/generic_name
//	limit  default 50, clamped to max 200 by the repo
//	offset default 0
func listTmtHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.TmtRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tmt repo not configured"})
			return
		}
		limit, _ := strconv.Atoi(c.Query("limit"))
		offset, _ := strconv.Atoi(c.Query("offset"))
		items, total, err := d.TmtRepo.Search(c.Request.Context(), store.TmtFilter{
			Q:      c.Query("q"),
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
	}
}
