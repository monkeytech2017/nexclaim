package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nexclaim/nexclaim/internal/store"
)

// listICD10Handler serves GET /api/v1/master/icd10?q=&limit=50
func listICD10Handler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.MasterDataRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "master data repo not configured"})
			return
		}
		q := c.Query("q")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		rows, err := d.MasterDataRepo.ListICD10(c.Request.Context(), q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []store.ICD10Row{}
		}
		c.JSON(http.StatusOK, gin.H{"items": rows, "count": len(rows)})
	}
}

// listICD9CMHandler serves GET /api/v1/master/icd9cm?q=&limit=50
func listICD9CMHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.MasterDataRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "master data repo not configured"})
			return
		}
		q := c.Query("q")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		rows, err := d.MasterDataRepo.ListICD9CM(c.Request.Context(), q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []store.ICD9CMRow{}
		}
		c.JSON(http.StatusOK, gin.H{"items": rows, "count": len(rows)})
	}
}

// listTMTHandler serves GET /api/v1/master/tmt?q=&limit=50
func listTMTHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.MasterDataRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "master data repo not configured"})
			return
		}
		q := c.Query("q")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		rows, err := d.MasterDataRepo.ListTMT(c.Request.Context(), q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rows == nil {
			rows = []store.TMTRow{}
		}
		c.JSON(http.StatusOK, gin.H{"items": rows, "count": len(rows)})
	}
}
