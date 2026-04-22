// Package ipdimport runs one IPD share-folder export through the pipeline.
// Shared between:
//   - HTTP handler POST /api/v1/his/ipd/imports/:exportId
//   - Auto watcher (internal/watcher) that periodically scans incoming/
//
// Caller provides the folder root and all pipeline deps; Process does the
// parse → bucket-by-INSCL → per-bucket pipeline.Run → persist → move archive.
package ipdimport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/store"
	"github.com/nexclaim/nexclaim/internal/validator"
)

// Processor = reusable runner. Zero-value not ready for use — must set Root
// and at least one of FDH/CHI for live submission.
type Processor struct {
	Root      string                    // $IPD_SHARE_ROOT (contains incoming/, processed/, error/)
	FDH       pipeline.FDHSubmitter     // required for UC/CSMBS/LGO/OFC
	CHI       pipeline.CHISubmitter     // required for SSS/SS4
	ClaimRepo store.ClaimRepo           // nil → runs are not persisted
	Master    validator.MasterValidator // nil → NoopMaster in pipeline
}

// RunResult = outcome of one INSCL bucket within a single export.
type RunResult struct {
	INSCL      model.INSCL
	AdmitCount int
	Outcome    *pipeline.Outcome // nil on pre-pipeline error
	Err        error             // nil on success (use out.Submissions for per-format status)
}

// Result summarizes Process() for a single export folder.
type Result struct {
	ExportID  string
	DryRun    bool
	Runs      []RunResult
	MoveError string // empty on success
}

// Process reads the export folder, groups by INSCL, runs pipeline.Run per
// bucket, persists outcomes (if !dryRun) and moves the folder to processed/
// or error/. Returns Result even on partial failure so callers can surface
// per-bucket status.
func (p *Processor) Process(ctx context.Context, exportID string, dryRun bool) (*Result, error) {
	if p.Root == "" {
		return nil, fmt.Errorf("ipdimport: Root is required")
	}
	incoming := filepath.Join(p.Root, "incoming", exportID)
	if _, err := os.Stat(filepath.Join(incoming, "MANIFEST.json")); err != nil {
		return nil, fmt.Errorf("MANIFEST.json not found at %s", incoming)
	}

	fx := extractor.NewFileExtractor(incoming)
	admits, err := fx.Admits()
	if err != nil {
		// archive the broken folder so watcher doesn't retry forever
		mvErr := moveToError(p.Root, exportID, "parse: "+err.Error())
		return &Result{ExportID: exportID, DryRun: dryRun, MoveError: mvErr}, err
	}
	meta := fx.Manifest()

	// Bucket admits by INSCL — one pipeline.Run per bucket.
	buckets := make(map[model.INSCL][]model.IPDAdmit)
	for _, a := range admits {
		buckets[a.Patient.INSCL] = append(buckets[a.Patient.INSCL], a)
	}

	result := &Result{ExportID: exportID, DryRun: dryRun, Runs: make([]RunResult, 0, len(buckets))}

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	for inscl, as := range buckets {
		mem := extractor.NewMemoryExtractor()
		mem.Put(meta.Period, inscl, extractor.Result{IPD: as})
		out, err := pipeline.Run(runCtx, pipeline.Options{
			HCode:  meta.HospitalCode,
			Period: meta.Period,
			INSCL:  inscl,
			DryRun: dryRun,
			Extr:   mem,
			FDH:    p.FDH,
			CHI:    p.CHI,
			Master: p.Master,
		})
		if !dryRun && p.ClaimRepo != nil && out != nil {
			// Swallowed — persistence failure must not block the archive move.
			_ = p.ClaimRepo.SaveRun(runCtx, store.SaveRequest{
				HCode: meta.HospitalCode, Period: meta.Period, Outcome: out,
			})
		}
		result.Runs = append(result.Runs, RunResult{
			INSCL: inscl, AdmitCount: len(as), Outcome: out, Err: err,
		})
	}

	// Archive only when live; dry-run leaves the folder in place.
	if !dryRun {
		anyErr := false
		var reasons []string
		for _, r := range result.Runs {
			if r.Err != nil {
				anyErr = true
				reasons = append(reasons, string(r.INSCL)+": "+r.Err.Error())
			}
		}
		if anyErr {
			result.MoveError = moveToError(p.Root, exportID,
				"pipeline errors: "+strings.Join(reasons, "; "))
		} else {
			result.MoveError = moveToProcessed(p.Root, exportID)
		}
	}
	return result, nil
}

// ── archive helpers ────────────────────────────────────────────

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

// moveToError archives a failed import and drops a text file describing why.
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
