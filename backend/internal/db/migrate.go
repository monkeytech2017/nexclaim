package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

// Migration describes one SQL file on disk.
type Migration struct {
	Name    string // e.g. "001_master_data.sql"
	Path    string
	Applied bool
	Error   string // reason we couldn't verify (e.g. connection issue)
}

// ScanMigrations reads migrations/*.sql and returns them sorted by filename.
// Files starting with "000_" (database bootstrap) are included for display but
// should be applied before the DB exists — we can't probe them via sql.
func ScanMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var out []Migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		out = append(out, Migration{
			Name: e.Name(),
			Path: filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// DetectApplied probes a live DB for tables owned by each migration. Fast —
// one SELECT to information_schema. We match migrations by the known table
// names they create (hardcoded map; there is no migration history table yet).
func DetectApplied(db *sqlx.DB, migrations []Migration) ([]Migration, error) {
	probe := map[string][]string{
		"001_master_data.sql":  {"m_inscl", "m_agency", "m_chrgitem", "m_hospital"},
		"002_his_mapping.sql":  {"his_field_map", "his_inscl_map"},
		"003_transactions.sql": {"claim_batch", "claim_record", "c_code_log"},
		"004_ingest_batch.sql": {"opd_ingest_batch", "opd_ingest_visit"},
	}
	existing, err := listTables(db)
	if err != nil {
		return migrations, fmt.Errorf("list tables: %w", err)
	}
	present := make(map[string]bool, len(existing))
	for _, t := range existing {
		present[t] = true
	}

	for i := range migrations {
		tables, ok := probe[migrations[i].Name]
		if !ok {
			continue // 000_create_database.sql has no tables to probe
		}
		allPresent := true
		for _, t := range tables {
			if !present[t] {
				allPresent = false
				break
			}
		}
		migrations[i].Applied = allPresent
	}
	return migrations, nil
}

func listTables(db *sqlx.DB) ([]string, error) {
	var tables []string
	err := db.Select(&tables, `
		SELECT tablename FROM pg_catalog.pg_tables
		WHERE schemaname = 'public'
	`)
	return tables, err
}
