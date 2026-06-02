package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

func listSendLogsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.SendLogRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "send-log repo not configured"})
			return
		}
		limit := 0
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		rows, err := d.SendLogRepo.List(c.Request.Context(), store.SendLogFilter{
			BatchID: c.Query("batch_id"),
			HCode:   c.Query("hcode"),
			Period:  c.Query("period"),
			Success: c.Query("success"),
			Limit:   limit,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}
