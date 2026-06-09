package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
)

// Canonical filenames accepted by the master loader. The loader upserts —
// calling Load() repeatedly is safe.
const (
	FileICD10  = "icd10.json"
	FileICD9CM = "icd9cm.json"
	FileTMT    = "tmt.json"
)

// Development/test fixtures — smaller files with the same schema.
const (
	SampleICD10  = "icd10.sample.json"
	SampleICD9CM = "icd9cm.sample.json"
	SampleTMT    = "tmt.sample.json"
)

type icd10Row struct {
	Code    string `json:"code"`
	NameEN  string `json:"name_en"`
	NameTH  string `json:"name_th"`
	Chapter string `json:"chapter"`
}

type icd9cmRow struct {
	Code   string `json:"code"`
	NameEN string `json:"name_en"`
	NameTH string `json:"name_th"`
}

type tmtRow struct {
	TMTCode     string `json:"tmt_code"`
	NameTH      string `json:"name_th"`
	GenericName string `json:"generic_name"`
	Strength    string `json:"strength"`
	DosageForm  string `json:"dosage_form"`
	Unit        string `json:"unit"`
}

// LoadResult counts rows upserted per table; useful for CLI output.
type LoadResult struct {
	ICD10    int
	ICD9CM   int
	TMT      int
	Warnings []string
}

// LoadMaster reads ICD-10/ICD-9CM/TMT JSON files from dir (missing files are
// reported as warnings, not errors) and upserts each row into the matching
// m_* table. dir typically == backend/data/.
//
// Loader prefers FileICD10 over SampleICD10 but falls back to the sample if
// the full file is absent — lets tests and fresh checkouts seed a handful of
// rows without shipping the real master.
func LoadMaster(ctx context.Context, db *sqlx.DB, dir string) (*LoadResult, error) {
	res := &LoadResult{}

	if rows, warn, err := readICD10(dir); err != nil {
		return res, err
	} else {
		if warn != "" {
			res.Warnings = append(res.Warnings, warn)
		}
		n, err := upsertICD10(ctx, db, rows)
		if err != nil {
			return res, fmt.Errorf("icd10 upsert: %w", err)
		}
		res.ICD10 = n
	}

	if rows, warn, err := readICD9CM(dir); err != nil {
		return res, err
	} else {
		if warn != "" {
			res.Warnings = append(res.Warnings, warn)
		}
		n, err := upsertICD9CM(ctx, db, rows)
		if err != nil {
			return res, fmt.Errorf("icd9cm upsert: %w", err)
		}
		res.ICD9CM = n
	}

	if rows, warn, err := readTMT(dir); err != nil {
		return res, err
	} else {
		if warn != "" {
			res.Warnings = append(res.Warnings, warn)
		}
		n, err := upsertTMT(ctx, db, rows)
		if err != nil {
			return res, fmt.Errorf("tmt upsert: %w", err)
		}
		res.TMT = n
	}

	return res, nil
}

// ── readers ────────────────────────────────────────────────────

func readICD10(dir string) ([]icd10Row, string, error) {
	b, src, err := readFirstExisting(dir, FileICD10, SampleICD10)
	if err != nil {
		return nil, "no icd10.json or icd10.sample.json found", nil
	}
	var rows []icd10Row
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", src, err)
	}
	warn := ""
	if src == SampleICD10 {
		warn = "using icd10.sample.json (dev fixture — replace with full สปสช. master for prod)"
	}
	return rows, warn, nil
}

func readICD9CM(dir string) ([]icd9cmRow, string, error) {
	b, src, err := readFirstExisting(dir, FileICD9CM, SampleICD9CM)
	if err != nil {
		return nil, "no icd9cm.json or icd9cm.sample.json found", nil
	}
	var rows []icd9cmRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", src, err)
	}
	warn := ""
	if src == SampleICD9CM {
		warn = "using icd9cm.sample.json (dev fixture — replace with full ICD-9CM master for prod)"
	}
	return rows, warn, nil
}

func readTMT(dir string) ([]tmtRow, string, error) {
	b, src, err := readFirstExisting(dir, FileTMT, SampleTMT)
	if err != nil {
		return nil, "no tmt.json or tmt.sample.json found", nil
	}
	var rows []tmtRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, "", fmt.Errorf("parse %s: %w", src, err)
	}
	warn := ""
	if src == SampleTMT {
		warn = "using tmt.sample.json (dev fixture — replace with full tmt.this.or.th master for prod)"
	}
	return rows, warn, nil
}

// readFirstExisting tries each filename in order; returns file bytes + matched name.
func readFirstExisting(dir string, names ...string) ([]byte, string, error) {
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err == nil {
			return b, n, nil
		}
	}
	return nil, "", fmt.Errorf("none of %v found in %s", names, dir)
}

// ── upserts ────────────────────────────────────────────────────

func upsertICD10(ctx context.Context, db *sqlx.DB, rows []icd10Row) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PreparexContext(ctx, `
		INSERT INTO m_icd10 (code, name_th, name_en, chapter, is_active)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), true)
		ON CONFLICT (code) DO UPDATE SET
			name_th = EXCLUDED.name_th,
			name_en = EXCLUDED.name_en,
			chapter = EXCLUDED.chapter,
			is_active = true
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.Code, r.NameTH, r.NameEN, r.Chapter); err != nil {
			return 0, fmt.Errorf("code %s: %w", r.Code, err)
		}
	}
	return len(rows), tx.Commit()
}

func upsertICD9CM(ctx context.Context, db *sqlx.DB, rows []icd9cmRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PreparexContext(ctx, `
		INSERT INTO m_icd9cm (code, name_th, name_en, is_active)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''), true)
		ON CONFLICT (code) DO UPDATE SET
			name_th = EXCLUDED.name_th,
			name_en = EXCLUDED.name_en,
			is_active = true
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.Code, r.NameTH, r.NameEN); err != nil {
			return 0, fmt.Errorf("code %s: %w", r.Code, err)
		}
	}
	return len(rows), tx.Commit()
}

func upsertTMT(ctx context.Context, db *sqlx.DB, rows []tmtRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PreparexContext(ctx, `
		INSERT INTO m_tmt_drug (tmt_code, name_th, generic_name, strength, dosage_form, unit, is_active, updated_at)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), true, now())
		ON CONFLICT (tmt_code) DO UPDATE SET
			name_th = EXCLUDED.name_th,
			generic_name = EXCLUDED.generic_name,
			strength = EXCLUDED.strength,
			dosage_form = EXCLUDED.dosage_form,
			unit = EXCLUDED.unit,
			is_active = true,
			updated_at = now()
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, r := range rows {
		// TMTID จริงเป็น running number 6–7 หลัก (คอลัมน์ VARCHAR(24))
		if r.TMTCode == "" || len(r.TMTCode) > 24 {
			return 0, fmt.Errorf("tmt_code %q: want 1-24 chars, got %d", r.TMTCode, len(r.TMTCode))
		}
		if _, err := stmt.ExecContext(ctx, r.TMTCode, r.NameTH, r.GenericName, r.Strength, r.DosageForm, r.Unit); err != nil {
			return 0, fmt.Errorf("tmt %s: %w", r.TMTCode, err)
		}
	}
	return len(rows), tx.Commit()
}
