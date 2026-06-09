package extractor

import (
	"context"
	"fmt"
	"sync"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/sharefile"
)

// FileExtractor implements Extractor backed by an IPD share folder (one export).
//
// Loads MANIFEST.json + CSVs once (lazy on first Extract) and caches the full
// set of admits. Each Extract filters by Request.INSCL so one folder can feed
// multiple pipeline runs (one per INSCL) without re-parsing.
type FileExtractor struct {
	Dir string
	// TMTFactory (optional) builds a HIS-drug-code → TMT resolver once the
	// manifest's hospital code is known. Nil = legacy fallback.
	TMTFactory TMTResolverFactory

	once    sync.Once
	admits  []model.IPDAdmit
	loadErr error
	meta    *sharefile.Manifest
}

func NewFileExtractor(dir string) *FileExtractor {
	return &FileExtractor{Dir: dir}
}

// WithTMTFactory returns the extractor with a his_drug_map-backed resolver
// factory wired for TMT translation. Safe to call with a nil factory (no-op).
// Must be called before the first Extract/Admits since the parse result is
// cached.
func (e *FileExtractor) WithTMTFactory(f TMTResolverFactory) *FileExtractor {
	e.TMTFactory = f
	return e
}

func (e *FileExtractor) load() error {
	e.once.Do(func() {
		m, err := sharefile.LoadManifest(e.Dir)
		if err != nil {
			e.loadErr = err
			return
		}
		e.meta = m
		bundle, err := sharefile.Parse(e.Dir, m)
		if err != nil {
			e.loadErr = fmt.Errorf("parse: %w", err)
			return
		}
		resolver := resolverFor(e.TMTFactory, context.Background(), m.HospitalCode)
		admits, err := sharefile.AssembleWithResolver(bundle, resolver)
		if err != nil {
			e.loadErr = fmt.Errorf("assemble: %w", err)
			return
		}
		e.admits = admits
	})
	return e.loadErr
}

func (e *FileExtractor) Extract(_ context.Context, r Request) (Result, error) {
	if err := e.load(); err != nil {
		return Result{}, err
	}
	out := Result{}
	for _, a := range e.admits {
		if a.Patient.INSCL == r.INSCL {
			out.IPD = append(out.IPD, a)
		}
	}
	return out, nil
}

// Manifest returns the manifest (nil if load failed or not yet loaded).
func (e *FileExtractor) Manifest() *sharefile.Manifest { return e.meta }

// Admits returns all parsed admits (regardless of INSCL). Used by callers
// that need to enumerate INSCL buckets for per-INSCL pipeline runs.
func (e *FileExtractor) Admits() ([]model.IPDAdmit, error) {
	if err := e.load(); err != nil {
		return nil, err
	}
	return e.admits, nil
}
