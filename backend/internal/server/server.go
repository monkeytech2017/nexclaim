// Package server เปิด REST API ด้วย gin — ใช้ pipeline + FDH/CHI ภายใน.
//
// Endpoints:
//   GET  /healthz           — liveness
//   POST /api/submit        — trigger pipeline.Run
//   GET  /api/status/:txnId — forward to FDH
//
// Server ถือ state น้อยที่สุด; business logic อยู่ที่ pipeline.
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/validator"
)

// Deps รวม dependencies ที่ server ต้องใช้. ไว้ให้ test inject fake ได้ตรง ๆ.
type Deps struct {
	HCode     string
	Extractor extractor.Extractor // fallback extractor สำหรับ POST /api/submit
	FDH       pipeline.FDHSubmitter
	CHI       pipeline.CHISubmitter
	// HIS client + batch store เปิดใช้ endpoint /api/v1/his/opd/*
	// ถ้า nil → endpoint คืน 503
	HISClient *hisclient.Client
	Batches   batch.Store
	// IPDShareRoot = root ของ /shared/nexclaim/ipd/ (ต้องมี subdirs:
	// incoming/, processed/, error/). ถ้าว่าง → endpoint /api/v1/his/ipd/*
	// คืน 503.
	IPDShareRoot string
	// StatusLookup reads status by txnId. Usually a *sender.FDHClient.
	StatusLookup interface {
		GetStatus(txnID string) (*sender.SubmitResult, error)
	}
}

// New สร้าง gin engine โดยไม่ start HTTP listener.
// Caller responsibility: run http.ListenAndServe(addr, engine).
func New(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "service": "nexclaim"})
	})

	api := r.Group("/api")
	api.POST("/submit", submitHandler(d))
	api.GET("/status/:txnId", statusHandler(d))

	// OPD 2-Way endpoints (HIS → NexClaim → HIS)
	his := r.Group("/api/v1/his")
	his.POST("/opd/visits", receiveVisitsHandler(d))
	his.GET("/opd/batches", listBatchesHandler(d))
	his.GET("/opd/batches/:batchId", getBatchHandler(d))
	his.POST("/opd/batches/:batchId/process", processBatchHandler(d))

	// IPD Share Folder ingestion
	his.GET("/ipd/imports", listImportsHandler(d))
	his.POST("/ipd/imports/:exportId", runImportHandler(d))

	return r
}

// ── /api/submit ──

type SubmitRequest struct {
	INSCL  string `json:"inscl"  binding:"required"`
	Period string `json:"period" binding:"required"` // YYYYMM
	HCode  string `json:"hcode"  binding:"omitempty"`
	Agency string `json:"agency" binding:"omitempty"`
	DryRun bool   `json:"dryRun" binding:"omitempty"`
}

type SubmitResponse struct {
	INSCL            string              `json:"inscl"`
	OPDCount         int                 `json:"opdCount"`
	IPDCount         int                 `json:"ipdCount"`
	ValidationErrors []validationErrorDTO `json:"validationErrors,omitempty"`
	Submissions      []submissionDTO     `json:"submissions"`
}

type validationErrorDTO struct {
	Field  string `json:"field"`
	Value  string `json:"value,omitempty"`
	CCode  string `json:"cCode,omitempty"`
	Reason string `json:"reason"`
}

type submissionDTO struct {
	Format   string `json:"format"`
	ZipName  string `json:"zipName,omitempty"`
	ZipBytes int    `json:"zipBytes"`
	XMLBytes int    `json:"xmlBytes"`
	FilesN   int    `json:"filesN"`
	TxnID    string `json:"txnId,omitempty"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

func submitHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SubmitRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		hcode := req.HCode
		if hcode == "" {
			hcode = d.HCode
		}
		if hcode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hcode required (pass in body or configure server default)"})
			return
		}
		if d.Extractor == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "extractor not configured"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()

		out, err := pipeline.Run(ctx, pipeline.Options{
			HCode:  hcode,
			Period: req.Period,
			INSCL:  model.INSCL(req.INSCL),
			Agency: model.Agency(req.Agency),
			DryRun: req.DryRun,
			Extr:   d.Extractor,
			FDH:    d.FDH,
			CHI:    d.CHI,
		})
		// We still return 200 + body when err != nil, because out can carry
		// useful partial info (validation errors, per-submission errors).
		// Hard failures (no Outcome) → 500.
		if out == nil {
			code := http.StatusInternalServerError
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}
		resp := outcomeToDTO(out)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":    err.Error(),
				"outcome":  resp,
			})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func outcomeToDTO(out *pipeline.Outcome) SubmitResponse {
	resp := SubmitResponse{
		INSCL:    string(out.INSCL),
		OPDCount: out.OPDCount,
		IPDCount: out.IPDCount,
	}
	for _, ve := range out.ValidationErrors {
		resp.ValidationErrors = append(resp.ValidationErrors, validationErrorDTO{
			Field: ve.Field, Value: ve.Value, CCode: ve.CCode, Reason: ve.Reason,
		})
	}
	for _, s := range out.Submissions {
		dto := submissionDTO{
			Format:   string(s.Format),
			ZipName:  s.ZipName,
			ZipBytes: len(s.ZipBytes),
			XMLBytes: len(s.XML),
			FilesN:   len(s.Files),
			TxnID:    s.TxnID,
			Status:   s.Status,
			Message:  s.Message,
		}
		if s.Err != nil {
			dto.Error = s.Err.Error()
		}
		resp.Submissions = append(resp.Submissions, dto)
	}
	return resp
}

// ── /api/v1/his/opd/visits (HIS → NexClaim push) ──

func receiveVisitsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Batches == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "batch store not configured"})
			return
		}
		var req hisclient.VisitListRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, hisclient.VisitListError{
				Status: "ERROR", Error: "BAD_REQUEST", Message: err.Error(),
			})
			return
		}
		if req.HospitalCode == "" || req.Period == "" || len(req.Visits) == 0 {
			c.JSON(http.StatusBadRequest, hisclient.VisitListError{
				Status: "ERROR", Error: "BAD_REQUEST",
				Message: "hospital_code, period, and non-empty visits are required",
			})
			return
		}

		// Per-visit validation: PID + required fields. สะสม error ก่อนตัดสิน accept/reject.
		var verrs []hisclient.VisitErrorDetail
		for _, v := range req.Visits {
			if v.VN == "" {
				verrs = append(verrs, hisclient.VisitErrorDetail{VN: v.VN, Field: "vn", Message: "vn required"})
			}
			if v.HN == "" {
				verrs = append(verrs, hisclient.VisitErrorDetail{VN: v.VN, Field: "hn", Message: "hn required"})
			}
			if v.PID != "" && !validator.IsValidPersonID(v.PID) {
				verrs = append(verrs, hisclient.VisitErrorDetail{
					VN: v.VN, Field: "pid", Message: "PID check digit invalid",
				})
			}
		}
		if len(verrs) > 0 {
			c.JSON(http.StatusBadRequest, hisclient.VisitListError{
				Status: "ERROR", Error: "VALIDATION_ERROR",
				Message: fmt.Sprintf("%d visit(s) failed validation", len(verrs)),
				Errors:  verrs,
			})
			return
		}

		b := d.Batches.Put(req)
		c.JSON(http.StatusOK, hisclient.VisitListResponse{
			Status:        "OK",
			BatchID:       b.ID,
			ReceivedCount: len(b.Visits),
			Message:       fmt.Sprintf("รับข้อมูล %d visits สำเร็จ", len(b.Visits)),
			CreatedAt:     b.CreatedAt.Format(time.RFC3339),
		})
	}
}

// ── /api/v1/his/opd/batches (list all) ──

func listBatchesHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Batches == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "batch store not configured"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"batches": d.Batches.List()})
	}
}

// ── /api/v1/his/opd/batches/:batchId ──

func getBatchHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Batches == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "batch store not configured"})
			return
		}
		b, ok := d.Batches.Get(c.Param("batchId"))
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "batch not found"})
			return
		}
		c.JSON(http.StatusOK, b)
	}
}

// ── /api/v1/his/opd/batches/:batchId/process ──
//
// Pulls visit details from HIS, runs pipeline, returns Outcome.
// Query param `dry_run=true` skips FDH/CHI submission.

func processBatchHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.Batches == nil || d.HISClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "batch store + HIS client required"})
			return
		}
		batchID := c.Param("batchId")
		b, ok := d.Batches.Get(batchID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "batch not found"})
			return
		}
		dryRun := c.Query("dry_run") == "true"

		// Group visits by INSCL → run pipeline once per INSCL bucket
		// (router maps INSCL → format, so mixing is wrong).
		buckets := make(map[string][]string) // inscl → []vn
		for _, v := range b.Visits {
			buckets[v.INSCL] = append(buckets[v.INSCL], v.VN)
		}

		type batchRunOutcome struct {
			INSCL   string              `json:"inscl"`
			VNCount int                 `json:"vn_count"`
			Outcome *SubmitResponse     `json:"outcome,omitempty"`
			Error   string              `json:"error,omitempty"`
		}
		results := make([]batchRunOutcome, 0, len(buckets))

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()

		for inscl, vns := range buckets {
			// build an extractor preconfigured to return only these VNs' details
			sub := subBatch(d.Batches, batchID, vns)
			if sub == "" {
				results = append(results, batchRunOutcome{
					INSCL: inscl, VNCount: len(vns),
					Error: "failed to create sub-batch",
				})
				continue
			}
			apiExtr := extractor.NewAPIExtractor(d.Batches, d.HISClient, sub)
			out, err := pipeline.Run(ctx, pipeline.Options{
				HCode:  b.HospitalCode,
				Period: b.Period,
				INSCL:  model.INSCL(inscl),
				DryRun: dryRun,
				Extr:   apiExtr,
				FDH:    d.FDH,
				CHI:    d.CHI,
			})
			dto := outcomeToDTO(out)
			item := batchRunOutcome{INSCL: inscl, VNCount: len(vns), Outcome: &dto}
			if err != nil {
				item.Error = err.Error()
			}
			results = append(results, item)
		}

		_ = d.Batches.SetState(batchID, batch.StateCompleted, "")
		c.JSON(http.StatusOK, gin.H{
			"batchId": batchID,
			"dryRun":  dryRun,
			"runs":    results,
		})
	}
}

// subBatch creates a view over an existing batch that contains only a subset
// of its visits, under a new deterministic ID. Returns the sub-batch ID.
// Used so we can run pipeline.Run per INSCL bucket without mutating the parent batch.
func subBatch(store batch.Store, parentID string, vns []string) string {
	parent, ok := store.Get(parentID)
	if !ok {
		return ""
	}
	keep := make(map[string]bool, len(vns))
	for _, vn := range vns {
		keep[vn] = true
	}
	req := hisclient.VisitListRequest{
		HospitalCode: parent.HospitalCode,
		Period:       parent.Period,
		ExportedBy:   parent.ExportedBy,
	}
	for _, v := range parent.Visits {
		if keep[v.VN] {
			req.Visits = append(req.Visits, v)
		}
	}
	sub := store.Put(req)
	return sub.ID
}

// ── IPD Share Folder: GET list imports ──

func listImportsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.IPDShareRoot == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "IPD share root not configured"})
			return
		}
		incoming := filepath.Join(d.IPDShareRoot, "incoming")
		entries, err := os.ReadDir(incoming)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		type item struct {
			ExportID string `json:"export_id"`
			Ready    bool   `json:"ready"` // MANIFEST.json present
			Path     string `json:"path"`
		}
		var out []item
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(incoming, e.Name())
			_, err := os.Stat(filepath.Join(dir, "MANIFEST.json"))
			out = append(out, item{ExportID: e.Name(), Ready: err == nil, Path: dir})
		}
		c.JSON(http.StatusOK, gin.H{"imports": out})
	}
}

// ── IPD Share Folder: POST run import ──

func runImportHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.IPDShareRoot == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "IPD share root not configured"})
			return
		}
		exportID := c.Param("exportId")
		incoming := filepath.Join(d.IPDShareRoot, "incoming", exportID)
		if _, err := os.Stat(filepath.Join(incoming, "MANIFEST.json")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "MANIFEST.json not found at " + incoming})
			return
		}
		dryRun := c.Query("dry_run") == "true"

		fx := extractor.NewFileExtractor(incoming)
		admits, err := fx.Admits()
		if err != nil {
			if mvErr := moveToError(d.IPDShareRoot, exportID, "parse: "+err.Error()); mvErr != "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "moveError": mvErr})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		meta := fx.Manifest()

		// Bucket admits by INSCL → pipeline per bucket
		buckets := make(map[model.INSCL][]model.IPDAdmit)
		for _, a := range admits {
			buckets[a.Patient.INSCL] = append(buckets[a.Patient.INSCL], a)
		}

		type runOut struct {
			INSCL      string          `json:"inscl"`
			AdmitCount int             `json:"admit_count"`
			Outcome    *SubmitResponse `json:"outcome,omitempty"`
			Error      string          `json:"error,omitempty"`
		}
		results := make([]runOut, 0, len(buckets))

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
		defer cancel()

		for inscl, as := range buckets {
			mem := extractor.NewMemoryExtractor()
			mem.Put(meta.Period, inscl, extractor.Result{IPD: as})
			out, err := pipeline.Run(ctx, pipeline.Options{
				HCode:  meta.HospitalCode,
				Period: meta.Period,
				INSCL:  inscl,
				DryRun: dryRun,
				Extr:   mem,
				FDH:    d.FDH,
				CHI:    d.CHI,
			})
			dto := outcomeToDTO(out)
			item := runOut{INSCL: string(inscl), AdmitCount: len(as), Outcome: &dto}
			if err != nil {
				item.Error = err.Error()
			}
			results = append(results, item)
		}

		// Move to processed/ only if no per-bucket errors AND not dry-run.
		resp := gin.H{"export_id": exportID, "dryRun": dryRun, "runs": results}
		if !dryRun {
			anyErr := false
			var reasons []string
			for _, r := range results {
				if r.Error != "" {
					anyErr = true
					reasons = append(reasons, r.INSCL+": "+r.Error)
				}
			}
			var moveErrMsg string
			if anyErr {
				moveErrMsg = moveToError(d.IPDShareRoot, exportID,
					"pipeline errors: "+strings.Join(reasons, "; "))
			} else {
				moveErrMsg = moveToProcessed(d.IPDShareRoot, exportID)
			}
			if moveErrMsg != "" {
				resp["moveError"] = moveErrMsg
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

// moveToProcessed archives a successfully-imported export folder.
// Returns an error message (empty on success).
func moveToProcessed(root, exportID string) string {
	src := filepath.Join(root, "incoming", exportID)
	dst := filepath.Join(root, "processed", exportID)
	if err := os.MkdirAll(filepath.Join(root, "processed"), 0o755); err != nil {
		return "mkdir processed: " + err.Error()
	}
	if err := os.Rename(src, dst); err != nil {
		return err.Error()
	}
	return ""
}

// moveToError archives a failed import and drops a text file describing why,
// so the HIS team can inspect the folder later and diagnose what broke.
// Returns an error message (empty on success).
func moveToError(root, exportID, reason string) string {
	src := filepath.Join(root, "incoming", exportID)
	dst := filepath.Join(root, "error", exportID)
	if err := os.MkdirAll(filepath.Join(root, "error"), 0o755); err != nil {
		return "mkdir error: " + err.Error()
	}
	if err := os.Rename(src, dst); err != nil {
		return err.Error()
	}
	payload := fmt.Sprintf("export_id: %s\ntime: %s\nreason: %s\n",
		exportID, time.Now().Format(time.RFC3339), reason)
	if err := os.WriteFile(filepath.Join(dst, "ERROR.txt"), []byte(payload), 0o644); err != nil {
		return "write ERROR.txt: " + err.Error()
	}
	return ""
}

// ── /api/status/:txnId ──

func statusHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.StatusLookup == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "status lookup not configured"})
			return
		}
		txnID := c.Param("txnId")
		if txnID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "txnId required"})
			return
		}
		res, err := d.StatusLookup.GetStatus(txnID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"txnId":   res.TxnID,
			"status":  res.Status,
			"message": res.Message,
		})
	}
}

