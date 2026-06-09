// Package server เปิด REST API ด้วย gin — ใช้ pipeline + FDH/CHI ภายใน.
//
// Endpoints:
//
//	GET  /healthz           — liveness
//	POST /api/submit        — trigger pipeline.Run
//	GET  /api/status/:txnId — forward to FDH
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

	"github.com/nexclaim/nexclaim/internal/audit"
	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/ipdimport"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/store"
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
	// ClaimRepo persists pipeline.Outcome → claim_batch/claim_record.
	// If nil, runs are not persisted (NoopClaimRepo is used).
	ClaimRepo store.ClaimRepo
	// AuthRepo gates non-public endpoints when AuthEnabled is true.
	// Nil = middleware becomes a pass-through (back-compat for dev + tests).
	AuthRepo    auth.Repo
	AuthEnabled bool
	// RateLimitPerMin caps bootstrap/whoami/keys-POST requests per client IP
	// per minute. 0 (default) = disabled. Single-instance only; see
	// auth/ratelimit.go header.
	RateLimitPerMin int
	// HospitalRepo backs the /api/v1/master/hospitals admin endpoints.
	// Nil = return 503 (admin CRUD requires a database).
	HospitalRepo   store.HospitalRepo
	DoctorRepo     store.DoctorRepo
	InsclMapRepo   store.InsclMapRepo
	DrugMapRepo    store.DrugMapRepo
	DoctorMapRepo  store.DoctorMapRepo
	IcdMapRepo     store.IcdMapRepo
	FieldMapRepo   store.FieldMapRepo
	CCodeRepo      store.CCodeRepo
	ClaimBatchRepo store.ClaimBatchRepo
	SendLogRepo    store.SendLogRepo
	DashboardRepo  store.DashboardRepo
	REPIngester    *store.REPIngester
	// AuditRepo backs GET /api/v1/audit-log (admin-only read).
	// Nil = endpoint returns 503; handlers still try AuditWriter best-effort.
	AuditRepo store.AuditRepo
	// AuditWriter is the sink every instrumented handler writes to AFTER the
	// action succeeds. Typically equal to AuditRepo (PgAuditRepo satisfies
	// both). Nil falls back to audit.NoopWriter so tests/dev don't panic.
	AuditWriter audit.Writer
	// MasterDataRepo backs the read-only master viewer endpoints
	// GET /api/v1/master/icd10, /icd9cm, /tmt. Nil = 503.
	MasterDataRepo store.MasterDataRepo
	// Master backs ICD/TMT lookup in pipeline validation. Nil = noop.
	Master validator.MasterValidator
	// StatusLookup reads status by txnId. Usually a *sender.FDHClient.
	StatusLookup interface {
		GetStatus(txnID string) (*sender.SubmitResult, error)
	}
}

// New สร้าง gin engine โดยไม่ start HTTP listener.
// Caller responsibility: run http.ListenAndServe(addr, engine).
//
// Route grouping (for auth gating):
//
//	Public (no auth):
//	  GET  /healthz
//	  POST /api/v1/auth/bootstrap          (only when AUTH_BOOTSTRAP_TOKEN set + table empty)
//
//	Hospital-scoped (hospital OR admin; hospital bound to its own hcode):
//	  /api/v1/auth/whoami                  — reports caller identity
//	  /api/v1/his/*                        — OPD 2-way + IPD share
//	  /api/v1/claim/*                      — submission history + REP
//	  /api/v1/ccodes*                      — REP feedback loop
//	  /api/v1/send-logs
//	  /api/v1/dashboard/stats
//
//	Admin-only:
//	  /api/v1/master/*                     — master CRUD + bulk imports
//	  POST /api/submit                     — dev/ops pipeline trigger
//	  GET  /api/status/:txnId              — forward to FDH
//
// When AuthEnabled = false (default) or AuthRepo = nil, every middleware
// becomes a pass-through — tests + dev laptops keep working without a DB.
func New(d Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Rate limiter for hot security endpoints only. A nil rl returns a
	// pass-through middleware, so we always wire it (no branching here).
	rl := auth.NewRateLimiter(d.RateLimitPerMin)
	rlMW := rl.Middleware()

	// ── public ──
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "service": "nexclaim"})
	})
	// Rate limit BEFORE the handler runs so bootstrap's DB round-trip is
	// shielded too (matches whoami + keys POST below).
	r.POST("/api/v1/auth/bootstrap", rlMW, bootstrapHandler(d))

	// ── authed root ──
	authed := r.Group("", auth.Middleware(d.AuthRepo, d.AuthEnabled))
	// whoami is cheap but a common probe target; rate-limit it too.
	authed.GET("/api/v1/auth/whoami", rlMW, whoamiHandler())

	// Admin-only API-key management (create / list / deactivate).
	// Rotation workflow: create new → deactivate old (no explicit delete).
	// Rate limit ONLY the POST (minting) path; list/patch are admin-only and
	// low-volume, so the limiter would just be noise.
	keys := authed.Group("/api/v1/auth/keys", auth.RequireRole(auth.RoleAdmin))
	keys.POST("", rlMW, createAPIKeyHandler(d))
	keys.GET("", listAPIKeysHandler(d))
	keys.PATCH("/:id", patchAPIKeyHandler(d))

	// Admin-only: pipeline trigger + FDH status forward.
	adminAPI := authed.Group("/api", auth.RequireRole(auth.RoleAdmin))
	adminAPI.POST("/submit", submitHandler(d))
	adminAPI.GET("/status/:txnId", statusHandler(d))

	// Admin-only: audit log read endpoint. Writers are the instrumented
	// handlers themselves (best-effort, via d.AuditWriter).
	authed.GET("/api/v1/audit-log", auth.RequireRole(auth.RoleAdmin), listAuditLogHandler(d))

	// Hospital-scoped group: HIS endpoints. Path :hcode matches via
	// RequireHcodeMatch; batchId-scoped routes resolve hcode from the repo.
	batchResolver := newBatchResolver(d.ClaimBatchRepo)
	ccodeResolver := newCCodeResolver(d.CCodeRepo)
	his := authed.Group("/api/v1/his")
	his.POST("/opd/visits", receiveVisitsHandler(d)) // body carries hospital_code; handler validates
	his.GET("/opd/batches", listBatchesHandler(d))
	his.GET("/opd/batches/:batchId", auth.RequireBatchHcodeMatch(batchResolver), getBatchHandler(d))
	his.POST("/opd/batches/:batchId/process", auth.RequireBatchHcodeMatch(batchResolver), processBatchHandler(d))
	his.GET("/ipd/imports", listImportsHandler(d))
	his.POST("/ipd/imports/:exportId", runImportHandler(d))

	// Hospital-scoped: claim history. Path/query :hcode enforced by middleware.
	claim := authed.Group("/api/v1/claim")
	claim.POST("/rep/:hcode/:period", auth.RequireHcodeMatch("hcode"), fetchREPHandler(d))
	claim.GET("/batches", auth.RequireHcodeMatch("hcode"), listClaimBatchesHandler(d))
	claim.GET("/batches/:batchId", auth.RequireBatchHcodeMatch(batchResolver), getClaimBatchHandler(d))

	// Hospital-scoped: c-code feedback.
	authed.GET("/api/v1/ccodes", auth.RequireHcodeMatch("hcode"), listCCodesHandler(d))
	authed.PATCH("/api/v1/ccodes/:id/resolve", auth.RequireCCodeHcodeMatch(ccodeResolver), resolveCCodeHandler(d))

	// Hospital-scoped: send-log audit + dashboard summary.
	authed.GET("/api/v1/send-logs", auth.RequireHcodeMatch("hcode"), listSendLogsHandler(d))
	authed.GET("/api/v1/dashboard/stats", auth.RequireHcodeMatch("hcode"), listDashboardStatsHandler(d))

	// ── admin-only: master data CRUD + bulk imports ──
	master := authed.Group("/api/v1/master", auth.RequireRole(auth.RoleAdmin))
	master.GET("/hospitals", listHospitalsHandler(d))
	master.POST("/hospitals", upsertHospitalHandler(d))
	master.GET("/hospitals/:hcode", getHospitalHandler(d))
	master.PATCH("/hospitals/:hcode", upsertHospitalHandler(d))
	master.DELETE("/hospitals/:hcode", deleteHospitalHandler(d))

	master.GET("/doctors", listDoctorsHandler(d))
	master.POST("/doctors", upsertDoctorHandler(d))
	master.GET("/doctors/:doctorId", getDoctorHandler(d))
	master.PATCH("/doctors/:doctorId", upsertDoctorHandler(d))
	master.DELETE("/doctors/:doctorId", deleteDoctorHandler(d))

	master.GET("/inscl-maps", listInsclMapsHandler(d))
	master.POST("/inscl-maps", upsertInsclMapHandler(d))
	master.DELETE("/inscl-maps/:hcode/:hisPttype", deleteInsclMapHandler(d))

	master.GET("/drug-maps", listDrugMapsHandler(d))
	master.POST("/drug-maps", upsertDrugMapHandler(d))
	master.POST("/drug-maps/bulk", bulkDrugMapsHandler(d))
	master.DELETE("/drug-maps/:hcode/:hisDrugCode", deleteDrugMapHandler(d))

	master.GET("/doctor-maps", listDoctorMapsHandler(d))
	master.POST("/doctor-maps", upsertDoctorMapHandler(d))
	master.POST("/doctor-maps/bulk", bulkDoctorMapsHandler(d))
	master.DELETE("/doctor-maps/:hcode/:hisDoctorCode", deleteDoctorMapHandler(d))

	master.GET("/icd-maps", listIcdMapsHandler(d))
	master.POST("/icd-maps", upsertIcdMapHandler(d))
	master.POST("/icd-maps/bulk", bulkIcdMapsHandler(d))
	master.DELETE("/icd-maps/:hcode/:icdType/:hisIcdCode", deleteIcdMapHandler(d))

	master.GET("/field-maps", listFieldMapsHandler(d))
	master.POST("/field-maps", upsertFieldMapHandler(d))
	master.POST("/field-maps/bulk", bulkFieldMapsHandler(d))
	master.DELETE("/field-maps/:id", deleteFieldMapHandler(d))

	// Read-only master data viewer (ICD-10 / ICD-9CM / TMT drug).
	master.GET("/icd10", listICD10Handler(d))
	master.GET("/icd9cm", listICD9CMHandler(d))
	master.GET("/tmt", listTMTHandler(d))

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
	INSCL            string               `json:"inscl"`
	OPDCount         int                  `json:"opdCount"`
	IPDCount         int                  `json:"ipdCount"`
	ValidationErrors []validationErrorDTO `json:"validationErrors,omitempty"`
	Submissions      []submissionDTO      `json:"submissions"`
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
			Master: d.Master,
		})
		// We still return 200 + body when err != nil, because out can carry
		// useful partial info (validation errors, per-submission errors).
		// Hard failures (no Outcome) → 500.
		if out == nil {
			code := http.StatusInternalServerError
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}
		if !req.DryRun {
			persistRun(ctx, d, hcode, req.Period, out)
		}
		resp := outcomeToDTO(out)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":   err.Error(),
				"outcome": resp,
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
		// Hospital-scoped callers may only push visits for their own hcode.
		// Body-level enforcement — RequireHcodeMatch middleware only inspects
		// path/query params and doesn't parse the JSON body.
		if ident, ok := auth.FromContext(c); ok && ident.Role == auth.RoleHospital {
			if req.HospitalCode != ident.HCode {
				c.JSON(http.StatusForbidden, hisclient.VisitListError{
					Status: "ERROR", Error: "FORBIDDEN",
					Message: fmt.Sprintf("hospital key bound to %q cannot push visits for %q", ident.HCode, req.HospitalCode),
				})
				return
			}
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
			INSCL   string          `json:"inscl"`
			VNCount int             `json:"vn_count"`
			Outcome *SubmitResponse `json:"outcome,omitempty"`
			Error   string          `json:"error,omitempty"`
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
			apiExtr := extractor.NewAPIExtractor(d.Batches, d.HISClient, sub).
				WithTMTFactory(DrugMapTMTFactory(d.DrugMapRepo))
			out, err := pipeline.Run(ctx, pipeline.Options{
				HCode:  b.HospitalCode,
				Period: b.Period,
				INSCL:  model.INSCL(inscl),
				DryRun: dryRun,
				Extr:   apiExtr,
				FDH:    d.FDH,
				CHI:    d.CHI,
				Master: d.Master,
			})
			if !dryRun {
				persistRun(ctx, d, b.HospitalCode, b.Period, out)
			}
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
		dryRun := c.Query("dry_run") == "true"

		proc := &ipdimport.Processor{
			Root:       d.IPDShareRoot,
			FDH:        d.FDH,
			CHI:        d.CHI,
			ClaimRepo:  d.ClaimRepo,
			Master:     d.Master,
			TMTFactory: DrugMapTMTFactory(d.DrugMapRepo),
		}
		res, err := proc.Process(c.Request.Context(), exportID, dryRun)
		if err != nil && res == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		runs := make([]map[string]any, 0, len(res.Runs))
		for _, r := range res.Runs {
			item := map[string]any{
				"inscl":       string(r.INSCL),
				"admit_count": r.AdmitCount,
			}
			if r.Outcome != nil {
				dto := outcomeToDTO(r.Outcome)
				item["outcome"] = dto
			}
			if r.Err != nil {
				item["error"] = r.Err.Error()
			}
			runs = append(runs, item)
		}
		resp := gin.H{"export_id": res.ExportID, "dryRun": res.DryRun, "runs": runs}
		if res.MoveError != "" {
			resp["moveError"] = res.MoveError
		}
		if err != nil {
			resp["error"] = err.Error()
		}
		c.JSON(http.StatusOK, resp)
	}
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

// ── /api/v1/master/hospitals ──

func listHospitalsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.HospitalRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hospital repo not configured"})
			return
		}
		rows, err := d.HospitalRepo.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"hospitals": rows})
	}
}

func getHospitalHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.HospitalRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hospital repo not configured"})
			return
		}
		h, err := d.HospitalRepo.Get(c.Request.Context(), c.Param("hcode"))
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, h)
	}
}

func upsertHospitalHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.HospitalRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hospital repo not configured"})
			return
		}
		var body store.Hospital
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// For PATCH /hospitals/:hcode, prefer path param over body.
		if param := c.Param("hcode"); param != "" {
			body.HCode = param
		}
		h, err := d.HospitalRepo.Upsert(c.Request.Context(), body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, h)
	}
}

func deleteHospitalHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.HospitalRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hospital repo not configured"})
			return
		}
		err := d.HospitalRepo.Delete(c.Request.Context(), c.Param("hcode"))
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

// ── /api/v1/master/doctors ──

func listDoctorsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor repo not configured"})
			return
		}
		rows, err := d.DoctorRepo.List(c.Request.Context(), c.Query("hcode"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"doctors": rows})
	}
}

func getDoctorHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor repo not configured"})
			return
		}
		doc, err := d.DoctorRepo.Get(c.Request.Context(), c.Param("doctorId"))
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, doc)
	}
}

func upsertDoctorHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor repo not configured"})
			return
		}
		var body store.Doctor
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if p := c.Param("doctorId"); p != "" {
			body.DoctorID = p
		}
		out, err := d.DoctorRepo.Upsert(c.Request.Context(), body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func deleteDoctorHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor repo not configured"})
			return
		}
		err := d.DoctorRepo.Delete(c.Request.Context(), c.Param("doctorId"))
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

// ── /api/v1/master/inscl-maps ──

func listInsclMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.InsclMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "inscl-map repo not configured"})
			return
		}
		rows, err := d.InsclMapRepo.List(c.Request.Context(), c.Query("hcode"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func upsertInsclMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.InsclMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "inscl-map repo not configured"})
			return
		}
		var body store.InsclMap
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		out, err := d.InsclMapRepo.Upsert(c.Request.Context(), body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func deleteInsclMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.InsclMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "inscl-map repo not configured"})
			return
		}
		err := d.InsclMapRepo.Delete(c.Request.Context(), c.Param("hcode"), c.Param("hisPttype"))
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

// ── /api/v1/master/drug-maps ──

func listDrugMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DrugMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "drug-map repo not configured"})
			return
		}
		rows, err := d.DrugMapRepo.List(c.Request.Context(), c.Query("hcode"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func upsertDrugMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DrugMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "drug-map repo not configured"})
			return
		}
		var body store.DrugMap
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		out, err := d.DrugMapRepo.Upsert(c.Request.Context(), body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func deleteDrugMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DrugMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "drug-map repo not configured"})
			return
		}
		err := d.DrugMapRepo.Delete(c.Request.Context(), c.Param("hcode"), c.Param("hisDrugCode"))
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

// ── /api/v1/master/doctor-maps ──

func listDoctorMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor-map repo not configured"})
			return
		}
		rows, err := d.DoctorMapRepo.List(c.Request.Context(), c.Query("hcode"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func upsertDoctorMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor-map repo not configured"})
			return
		}
		var body store.DoctorMap
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		out, err := d.DoctorMapRepo.Upsert(c.Request.Context(), body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func deleteDoctorMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.DoctorMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "doctor-map repo not configured"})
			return
		}
		err := d.DoctorMapRepo.Delete(c.Request.Context(), c.Param("hcode"), c.Param("hisDoctorCode"))
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

// ── /api/v1/master/icd-maps ──

func listIcdMapsHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.IcdMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "icd-map repo not configured"})
			return
		}
		rows, err := d.IcdMapRepo.List(c.Request.Context(), c.Query("hcode"), c.Query("type"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": rows})
	}
}

func upsertIcdMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.IcdMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "icd-map repo not configured"})
			return
		}
		var body store.IcdMap
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		out, err := d.IcdMapRepo.Upsert(c.Request.Context(), body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func deleteIcdMapHandler(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if d.IcdMapRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "icd-map repo not configured"})
			return
		}
		err := d.IcdMapRepo.Delete(c.Request.Context(),
			c.Param("hcode"), c.Param("hisIcdCode"), c.Param("icdType"))
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

// writeAudit is the best-effort audit sink used by instrumented handlers.
// Errors are logged to stderr + swallowed — mirrors persistRun below, so
// the originating action's HTTP response is never blocked by audit failure.
// Falls back to NoopWriter when no writer is wired (tests, dev-without-DB).
func writeAudit(ctx context.Context, d Deps, entry audit.Entry) {
	w := d.AuditWriter
	if w == nil {
		return
	}
	if err := w.Write(ctx, entry); err != nil {
		fmt.Fprintf(os.Stderr, "[NexClaim] audit write: action=%s target=%s/%s: %v\n",
			entry.Action, entry.TargetKind, entry.TargetID, err)
	}
}

// persistRun saves a live (non-dry-run) outcome via the configured ClaimRepo.
// Swallows errors — persistence failures must not fail the API response the
// HIS operator already saw. They are logged to stderr for later triage.
func persistRun(ctx context.Context, d Deps, hcode, period string, out *pipeline.Outcome) {
	if out == nil || d.ClaimRepo == nil {
		return
	}
	if err := d.ClaimRepo.SaveRun(ctx, store.SaveRequest{
		HCode: hcode, Period: period, Outcome: out,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[NexClaim] claim_repo save: %v\n", err)
	}
}

// DrugMapTMTFactory builds a his_drug_map-backed TMT resolver factory for the
// extractors. Returns nil when repo is nil (auth/in-memory mode) so the
// extractor falls back to byte-for-byte legacy behaviour. The mapping is loaded
// once per hospital (List(hcode)) and indexed; HIS internal drug code → TMT.
func DrugMapTMTFactory(repo store.DrugMapRepo) extractor.TMTResolverFactory {
	if repo == nil {
		return nil
	}
	return func(ctx context.Context, hcode string) func(string) (string, bool) {
		if hcode == "" {
			return nil
		}
		maps, err := repo.List(ctx, hcode)
		if err != nil {
			return nil
		}
		idx := make(map[string]string, len(maps))
		for _, m := range maps {
			if !m.IsActive || m.TMTCode == "" {
				continue
			}
			idx[strings.TrimSpace(m.HISDrugCode)] = m.TMTCode
		}
		if len(idx) == 0 {
			return nil
		}
		return func(hisCode string) (string, bool) {
			tmt, ok := idx[strings.TrimSpace(hisCode)]
			return tmt, ok
		}
	}
}
