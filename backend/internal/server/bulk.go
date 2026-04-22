package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/store"
)

// BulkResult aggregated response for any /bulk endpoint.
// Errors accumulate per-row so callers can show which rows failed.
// Imported + len(Errors) may be < Total if the request body was malformed
// (we still attempt to process what we can).
type BulkResult struct {
	Total    int          `json:"total"`
	Imported int          `json:"imported"`
	Errors   []BulkError  `json:"errors,omitempty"`
}

type BulkError struct {
	Row    int    `json:"row"`    // 1-based (matches spreadsheet row number minus header)
	Key    string `json:"key,omitempty"`
	Reason string `json:"reason"`
}

// bulkImport is the shared driver: iterate a slice, call upsertFn per row,
// accumulate errors. Empty input returns zero result, not an error.
func bulkImport[T any](
	ctx context.Context,
	items []T,
	keyFn func(T) string,
	upsertFn func(context.Context, T) error,
) BulkResult {
	res := BulkResult{Total: len(items)}
	for i, it := range items {
		if err := upsertFn(ctx, it); err != nil {
			res.Errors = append(res.Errors, BulkError{
				Row: i + 1, Key: keyFn(it), Reason: err.Error(),
			})
			continue
		}
		res.Imported++
	}
	return res
}

// ── handlers ──

func bulkDrugMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DrugMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "drug-map repo not configured"})
			return
		}
		var body struct {
			Items []store.DrugMap `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		res := bulkImport(c.Request.Context(), body.Items,
			func(m store.DrugMap) string { return m.HCode + "/" + m.HISDrugCode },
			func(ctx context.Context, m store.DrugMap) error {
				_, err := d.DrugMapRepo.Upsert(ctx, m)
				return err
			},
		)
		c.JSON(http.StatusOK, res)
	}
}

func bulkDoctorMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor-map repo not configured"})
			return
		}
		var body struct {
			Items []store.DoctorMap `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		res := bulkImport(c.Request.Context(), body.Items,
			func(m store.DoctorMap) string { return m.HCode + "/" + m.HISDoctorCode },
			func(ctx context.Context, m store.DoctorMap) error {
				_, err := d.DoctorMapRepo.Upsert(ctx, m)
				return err
			},
		)
		c.JSON(http.StatusOK, res)
	}
}

func bulkIcdMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.IcdMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "icd-map repo not configured"})
			return
		}
		var body struct {
			Items []store.IcdMap `json:"items"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		res := bulkImport(c.Request.Context(), body.Items,
			func(m store.IcdMap) string { return m.HCode + "/" + m.IcdType + "/" + m.HISIcdCode },
			func(ctx context.Context, m store.IcdMap) error {
				_, err := d.IcdMapRepo.Upsert(ctx, m)
				return err
			},
		)
		c.JSON(http.StatusOK, res)
	}
}
