package tests

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/ipdimport"
	"github.com/nexclaim/nexclaim/internal/watcher"
)

// processor reused directly: no server deps required beyond FDH/CHI fake
// (fakeFDH satisfies FDHSubmitter).

func TestProcessor_LiveSubmit_MovesToProcessed(t *testing.T) {
	root := setupShareRoot(t)
	fixtureExport(t, filepath.Join(root, "incoming"), "EXP-PROC-OK")

	fdh := &fakeFDH{}
	proc := &ipdimport.Processor{Root: root, FDH: fdh}

	res, err := proc.Process(context.Background(), "EXP-PROC-OK", false)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if res.MoveError != "" {
		t.Errorf("moveError = %q", res.MoveError)
	}
	// fixture has UCS + CSMBS admits → FDH should see both 16-file + CIPN
	if fdh.calls16 < 1 || fdh.callsCIPN < 1 {
		t.Errorf("FDH calls 16=%d CIPN=%d (want >=1 each)", fdh.calls16, fdh.callsCIPN)
	}
	// folder moved to processed/
	if _, err := os.Stat(filepath.Join(root, "processed", "EXP-PROC-OK", "MANIFEST.json")); err != nil {
		t.Errorf("folder should be under processed/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "incoming", "EXP-PROC-OK")); err == nil {
		t.Error("folder should no longer be in incoming/")
	}
}

func TestProcessor_DryRunLeavesFolder(t *testing.T) {
	root := setupShareRoot(t)
	fixtureExport(t, filepath.Join(root, "incoming"), "EXP-PROC-DR")

	fdh := &fakeFDH{}
	proc := &ipdimport.Processor{Root: root, FDH: fdh}
	_, err := proc.Process(context.Background(), "EXP-PROC-DR", true)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if fdh.calls16+fdh.callsCIPN != 0 {
		t.Error("dry-run should not submit to FDH")
	}
	if _, err := os.Stat(filepath.Join(root, "incoming", "EXP-PROC-DR", "MANIFEST.json")); err != nil {
		t.Error("dry-run should leave folder in incoming/")
	}
}

func TestIPDWatcher_PicksUpFolder(t *testing.T) {
	root := setupShareRoot(t)

	fdh := &fakeFDH{}
	proc := &ipdimport.Processor{Root: root, FDH: fdh}
	var logBuf bytes.Buffer
	w := &watcher.IPD{Proc: proc, Interval: 50 * time.Millisecond, Out: &logBuf}

	// Drop the folder BEFORE starting the watcher — first tick should pick it up immediately.
	fixtureExport(t, filepath.Join(root, "incoming"), "EXP-WATCH")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Wait for the folder to move to processed/ (watcher processed it).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(root, "processed", "EXP-WATCH", "MANIFEST.json")); err == nil {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	cancel()
	<-done

	if _, err := os.Stat(filepath.Join(root, "processed", "EXP-WATCH", "MANIFEST.json")); err != nil {
		t.Errorf("watcher should have moved folder to processed/: %v\nlog:\n%s", err, logBuf.String())
	}
	if fdh.calls16 < 1 {
		t.Errorf("watcher should have triggered FDH submission: %s", logBuf.String())
	}

	// Log should mention the processed folder
	if !bytes.Contains(logBuf.Bytes(), []byte("EXP-WATCH")) {
		t.Errorf("log did not mention folder: %s", logBuf.String())
	}
}

func TestIPDWatcher_Disabled(t *testing.T) {
	var logBuf bytes.Buffer
	w := &watcher.IPD{Proc: &ipdimport.Processor{Root: "/tmp/nonexistent"}, Interval: 0, Out: &logBuf}
	w.Run(context.Background())
	if !bytes.Contains(logBuf.Bytes(), []byte("disabled")) {
		t.Errorf("expected 'disabled' log, got: %s", logBuf.String())
	}
}
