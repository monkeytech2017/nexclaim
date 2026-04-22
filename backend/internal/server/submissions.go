package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

func listClaimBatchesHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.ClaimBatchRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "claim-batch repo not configured"})
			return
		}
		rows, err := d.ClaimBatchRepo.List(c.Request.Context(), store.ClaimBatchFilter{
			HCode:  c.Query("hcode"),
			Period: c.Query("period"),
			INSCL:  c.Query("inscl"),
			Format: c.Query("format"),
			Status: c.Query("status"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func getClaimBatchHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.ClaimBatchRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "claim-batch repo not configured"})
			return
		}
		row, err := d.ClaimBatchRepo.Get(c.Request.Context(), c.Param("batchId"))
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, row)
	}
}
