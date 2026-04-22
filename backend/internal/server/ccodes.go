package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

// POST /api/v1/claim/rep/:hcode/:period — download REP from FDH + ingest.
func fetchREPHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.REPIngester == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "REP ingester not configured"})
			return
		}
		hcode := c.Param("hcode")
		period := c.Param("period")
		res, err := d.REPIngester.Ingest(c.Request.Context(), hcode, period)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// GET /api/v1/ccodes?batch_id=&hcode=&period=&resolved=&c_code=
func listCCodesHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.CCodeRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "c-code repo not configured"})
			return
		}
		f := store.CCodeFilter{
			BatchID: c.Query("batch_id"),
			HCode:   c.Query("hcode"),
			Period:  c.Query("period"),
			CCode:   c.Query("c_code"),
		}
		switch c.Query("resolved") {
		case "true":
			t := true
			f.Resolved = &t
		case "false":
			t := false
			f.Resolved = &t
		}
		rows, err := d.CCodeRepo.List(c.Request.Context(), f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

// PATCH /api/v1/ccodes/:id/resolve body: {resolved_by}
func resolveCCodeHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.CCodeRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "c-code repo not configured"})
			return
		}
		var body struct {
			ResolvedBy string `json:"resolved_by"`
		}
		_ = c.ShouldBindJSON(&body)
		err := d.CCodeRepo.Resolve(c.Request.Context(), c.Param("id"), body.ResolvedBy)
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found or already resolved"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
