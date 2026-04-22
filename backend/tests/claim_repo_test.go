package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/store"
)

func TestClaimRepo_Noop(t *testing.T) {
	// Must not error on empty/nil outcomes — server calls this for every run.
	repo := store.NoopClaimRepo{}
	if err := repo.SaveRun(context.Background(), store.SaveRequest{}); err != nil {
		t.Errorf("Noop.SaveRun: %v", err)
	}
	if err := repo.SaveRun(context.Background(), store.SaveRequest{
		Outcome: &pipeline.Outcome{},
	}); err != nil {
		t.Errorf("Noop.SaveRun with empty outcome: %v", err)
	}
}

func TestClaimRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping ClaimRepo Postgres test")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Fresh tables so we count rows deterministically. Child rows first —
	// migration 003 doesn't declare ON DELETE CASCADE on these FKs.
	for _, q := range []string{
		`DELETE FROM c_code_log`,
		`DELETE FROM send_log`,
		`DELETE FROM claim_record`,
		`DELETE FROM claim_batch`,
	} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatalf("cleanup %q: %v — is migration 003 applied?", q, err)
		}
	}
	// claim_batch.hcode → m_hospital.hcode FK — must have a hospital row.
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345', 'Test Hospital', true, now())
		ON CONFLICT (hcode) DO NOTHING
	`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	repo := store.NewPg(conn)

	outcome := &pipeline.Outcome{
		INSCL:    model.INSCL_CSMBS,
		OPDCount: 1, IPDCount: 1,
		OPD: []model.OPDVisit{sampleCSMBSOPD()},
		IPD: []model.IPDAdmit{sampleCSMBSIPD()},
		Submissions: []pipeline.Submission{
			{Format: model.FormatCSOP, ZipName: "12345CSOP202504.ZIP", ZipBytes: []byte("a"), TxnID: "TXN-CSOP-1"},
			{Format: model.FormatCIPN, ZipName: "12345CIPN202504.ZIP", ZipBytes: []byte("b"), TxnID: "TXN-CIPN-1"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := repo.SaveRun(ctx, store.SaveRequest{
		HCode: "12345", Period: "202504", Outcome: outcome,
	}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	var batchN, recordN int
	_ = conn.Get(&batchN, `SELECT COUNT(*) FROM claim_batch`)
	_ = conn.Get(&recordN, `SELECT COUNT(*) FROM claim_record`)
	if batchN != 2 {
		t.Errorf("want 2 claim_batch rows (CSOP+CIPN), got %d", batchN)
	}
	// CSOP → OPD record; CIPN → IPD record. Total 2.
	if recordN != 2 {
		t.Errorf("want 2 claim_record rows (1 per submission bucket), got %d", recordN)
	}

	// Verify bucket split: OPD record belongs to CSOP batch, IPD to CIPN.
	var opdFormat, ipdFormat string
	_ = conn.Get(&opdFormat, `
		SELECT cb.format FROM claim_batch cb
		JOIN claim_record cr ON cb.batch_id = cr.batch_id
		WHERE cr.is_ipd = false`)
	_ = conn.Get(&ipdFormat, `
		SELECT cb.format FROM claim_batch cb
		JOIN claim_record cr ON cb.batch_id = cr.batch_id
		WHERE cr.is_ipd = true`)
	if opdFormat != "CSOP" {
		t.Errorf("OPD record should link to CSOP batch, got %s", opdFormat)
	}
	if ipdFormat != "CIPN" {
		t.Errorf("IPD record should link to CIPN batch, got %s", ipdFormat)
	}
}
