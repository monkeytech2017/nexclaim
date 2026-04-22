package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

func listFieldMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.FieldMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "field-map repo not configured"})
			return
		}
		rows, err := d.FieldMapRepo.List(c.Request.Context(), store.FieldMapFilter{
			HCode:      c.Query("hcode"),
			HISTable:   c.Query("his_table"),
			TargetFile: c.Query("target_file"),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func upsertFieldMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.FieldMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "field-map repo not configured"})
			return
		}
		var body store.FieldMap
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		out, err := d.FieldMapRepo.Upsert(c.Request.Context(), body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func deleteFieldMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.FieldMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "field-map repo not configured"})
			return
		}
		err := d.FieldMapRepo.Delete(c.Request.Context(), c.Param("id"))
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func bulkFieldMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.FieldMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "field-map repo not configured"})
			return
		}
		var body struct {
			Items []store.FieldMap `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		res := bulkImport(c.Request.Context(), body.Items,
			func(m store.FieldMap) string {
				return m.HCode + "/" + m.HISTable + "." + m.HISColumn +
					" → " + m.TargetFile + "." + m.TargetField
			},
			func(ctx context.Context, m store.FieldMap) error {
				_, err := d.FieldMapRepo.Upsert(ctx, m)
				return err
			},
		)
		c.JSON(http.StatusOK, res)
	}
}
