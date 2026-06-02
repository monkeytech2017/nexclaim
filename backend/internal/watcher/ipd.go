// Package watcher ตรวจหา folder ใหม่ใน IPD share incoming/ เป็นระยะ
// แล้ว trigger ipdimport.Processor.Process ให้อัตโนมัติ.
//
// ไม่ใช้ fsnotify เพราะ share folder มักเป็น NFS/SMB ที่ inotify ไม่ทำงาน
// — polling เป็นทางที่ portable กว่า. ปกติ hospital export รายวันหรือรายเดือน,
// interval 30–60 วินาทีเหลือเฟือ.
package watcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/nexclaim/nexclaim/internal/ipdimport"
)

// IPD periodic poller. Safe to run as a goroutine; stops when ctx is done.
type IPD struct {
	Proc     *ipdimport.Processor
	Interval time.Duration
	// Out receives one log line per significant event (processed folder,
	// per-folder error). Defaults to os.Stderr if nil.
	Out io.Writer
}

// Run blocks until ctx is done. Each tick lists incoming/, picks folders
// that have MANIFEST.json, and calls Proc.Process on them. Processor moves
// each folder to processed/ or error/ on success/failure so the next tick
// naturally skips already-handled exports.
func (w *IPD) Run(ctx context.Context) {
	out := w.Out
	if out == nil {
		out = os.Stderr
	}
	if w.Interval <= 0 {
		fmt.Fprintln(out, "[watcher] interval <= 0, disabled")
		return
	}
	if w.Proc == nil || w.Proc.Root == "" {
		fmt.Fprintln(out, "[watcher] processor/root missing, disabled")
		return
	}
	fmt.Fprintf(out, "[watcher] IPD watcher started (root=%s interval=%s)\n",
		w.Proc.Root, w.Interval)

	// Tick immediately once so a folder dropped while the server was
	// starting up gets picked up without waiting for the first interval.
	w.tick(ctx, out)

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(out, "[watcher] IPD watcher stopped")
			return
		case <-ticker.C:
			w.tick(ctx, out)
		}
	}
}

func (w *IPD) tick(ctx context.Context, out io.Writer) {
	incoming := filepath.Join(w.Proc.Root, "incoming")
	entries, err := os.ReadDir(incoming)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(out, "[watcher] read incoming: %v\n", err)
		}
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		manifest := filepath.Join(incoming, id, "MANIFEST.json")
		if _, err := os.Stat(manifest); err != nil {
			continue // folder exists but not ready
		}

		fmt.Fprintf(out, "[watcher] processing %s\n", id)
		res, err := w.Proc.Process(ctx, id, false)
		switch {
		case err != nil:
			fmt.Fprintf(out, "[watcher] %s: %v\n", id, err)
		case res == nil:
			fmt.Fprintf(out, "[watcher] %s: empty result\n", id)
		default:
			nErr := 0
			for _, r := range res.Runs {
				if r.Err != nil {
					nErr++
				}
			}
			fmt.Fprintf(out, "[watcher] %s: %d runs (%d errors); moveErr=%q\n",
				id, len(res.Runs), nErr, res.MoveError)
		}
	}
}
