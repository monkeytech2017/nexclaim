package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

// listDashboardStatsHandler — GET /api/v1/dashboard/stats
// Query: ?hcode=&period_from=&period_to=
// Returns a single DashboardStats payload mixing batch/record/c_code/send
// aggregates. 503 if DashboardRepo isn't wired (no DB).
func listDashboardStatsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DashboardRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "dashboard repo not configured"})
			return
		}
		stats, err := d.DashboardRepo.Stats(c.Request.Context(), store.DashboardFilter{
			HCode:      c.Query("hcode"),
			PeriodFrom: c.Query("period_from"),
			PeriodTo:   c.Query("period_to"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}
