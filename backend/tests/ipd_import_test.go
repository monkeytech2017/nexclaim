package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexclaim/nexclaim/internal/server"
)

func setupShareRoot(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	for _, sub := range []string{"incoming", "processed", "error"} {
		_ = os.MkdirAll(filepath.Join(root, sub), 0o755)
	}
	return
}

func TestServer_ListImports(t *testing.T) {
	root := setupShareRoot(t)
	fixtureExport(t, filepath.Join(root, "incoming"), "EXP-A")
	// second dir w/o manifest
	_ = os.MkdirAll(filepath.Join(root, "incoming", "EXP-B"), 0o755)

	h := server.New(server.Deps{IPDShareRoot: root})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/his/ipd/imports", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Imports []struct {
			ExportID string `json:"export_id"`
			Ready    bool   `json:"ready"`
		} `json:"imports"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	ready := map[string]bool{}
	for _, i := range body.Imports {
		ready[i.ExportID] = i.Ready
	}
	if !ready["EXP-A"] || ready["EXP-B"] {
		t.Errorf("ready map = %v (want EXP-A=true, EXP-B=false)", ready)
	}
}

func TestServer_ListImports_NoRoot(t *testing.T) {
	h := server.New(server.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/his/ipd/imports", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", w.Code)
	}
}

func TestServer_RunImport_DryRun(t *testing.T) {
	root := setupShareRoot(t)
	fixtureExport(t, filepath.Join(root, "incoming"), "EXP-DR")

	fdh := &fakeFDH{}
	h := server.New(server.Deps{IPDShareRoot: root, FDH: fdh})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/ipd/imports/EXP-DR?dry_run=true", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	if fdh.calls16+fdh.callsCIPN+fdh.callsCSOP != 0 {
		t.Error("dry-run should not submit to FDH")
	}
	// folder should STILL be in incoming/ (dry-run does not move)
	if _, err := os.Stat(filepath.Join(root, "incoming", "EXP-DR", "MANIFEST.json")); err != nil {
		t.Error("dry-run should leave folder in incoming/")
	}
}

func TestServer_RunImport_LiveSubmitsAndMoves(t *testing.T) {
	root := setupShareRoot(t)
	fixtureExport(t, filepath.Join(root, "incoming"), "EXP-LIVE")

	fdh := &fakeFDH{}
	h := server.New(server.Deps{IPDShareRoot: root, FDH: fdh})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/ipd/imports/EXP-LIVE", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	// UCS bucket → 16-file; CSMBS bucket → CIPN (IPD). Expect ≥ 1 FDH call per bucket.
	if fdh.calls16 < 1 || fdh.callsCIPN < 1 {
		t.Errorf("expected both Send16Files and SendCIPN; got 16=%d CIPN=%d", fdh.calls16, fdh.callsCIPN)
	}
	// folder should have been moved to processed/
	if _, err := os.Stat(filepath.Join(root, "processed", "EXP-LIVE", "MANIFEST.json")); err != nil {
		t.Errorf("folder should be under processed/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "incoming", "EXP-LIVE")); err == nil {
		t.Error("folder should no longer be in incoming/")
	}
}

func TestServer_RunImport_NotFound(t *testing.T) {
	root := setupShareRoot(t)
	h := server.New(server.Deps{IPDShareRoot: root})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/his/ipd/imports/MISSING", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}
