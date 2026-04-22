package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/store"
)

func TestMasterLoader_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping master loader Postgres test")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`TRUNCATE m_icd10, m_icd9cm, m_tmt_drug RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate: %v — is migration 001 applied?", err)
	}

	// Relative from tests/ dir, the data/ folder lives two levels up.
	wd, _ := os.Getwd()
	dataDir := filepath.Join(wd, "..", "data")

	res, err := store.LoadMaster(context.Background(), conn, dataDir)
	if err != nil {
		t.Fatalf("LoadMaster: %v", err)
	}
	if res.ICD10 == 0 || res.ICD9CM == 0 || res.TMT == 0 {
		t.Errorf("expected non-zero loads, got ICD10=%d ICD9CM=%d TMT=%d",
			res.ICD10, res.ICD9CM, res.TMT)
	}
	// Sample fixtures are expected to be used (warnings surfaced).
	if len(res.Warnings) == 0 {
		t.Log("no warnings — running with full master files?")
	}

	// Re-run must be idempotent.
	res2, err := store.LoadMaster(context.Background(), conn, dataDir)
	if err != nil {
		t.Fatalf("LoadMaster (2nd): %v", err)
	}
	if res2.ICD10 != res.ICD10 {
		t.Errorf("2nd run returned different count: %d vs %d", res2.ICD10, res.ICD10)
	}

	// Verify a row roundtripped with Thai text intact (collation check).
	var nameTH string
	if err := conn.Get(&nameTH, `SELECT name_th FROM m_icd10 WHERE code = 'I10'`); err != nil {
		t.Fatalf("select: %v", err)
	}
	if nameTH == "" {
		t.Error("name_th empty for I10 — Thai collation issue?")
	}
}
