package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/store"
)

// TestRetryRepo_Pg exercises SelectRetryable + RecordRetry against a real DB.
// Skips cleanly when POSTGRES_TEST_DSN is unset (same pattern as other *_pg tests).
func TestRetryRepo_Pg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Cleanup child→parent (RDS role has no TRUNCATE).
	for _, q := range []string{
		`DELETE FROM send_log     WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203096')`,
		`DELETE FROM c_code_log   WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203096')`,
		`DELETE FROM claim_record WHERE batch_id IN (SELECT batch_id FROM claim_batch WHERE period='203096')`,
		`DELETE FROM claim_batch  WHERE period='203096'`,
	} {
		if _, err := conn.Exec(q); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	if _, err := conn.Exec(`
		INSERT INTO m_hospital (hcode, name_th, is_active, created_at)
		VALUES ('12345','Test Hospital',true,now())
		ON CONFLICT (hcode) DO NOTHING`); err != nil {
		t.Fatalf("seed hospital: %v", err)
	}

	// Seed two batches directly: one error (retryable), one sent (not).
	var errorBatchID, sentBatchID string
	if err := conn.Get(&errorBatchID, `
		INSERT INTO claim_batch
		  (hcode, period, inscl, format, sender, status,
		   total_records, valid_records, error_records,
		   zip_filename, zip_bytes, attempt_no, next_retry_at, error_msg)
		VALUES ('12345','203096','UCS','16FILES','FDH','error',
		        1,1,0,'12345UCS203096.ZIP',$1,1,now() - interval '1 minute',
		        'network timeout')
		RETURNING batch_id::text
	`, []byte("fake-zip-payload")); err != nil {
		t.Fatalf("seed error batch: %v", err)
	}
	if err := conn.Get(&sentBatchID, `
		INSERT INTO claim_batch
		  (hcode, period, inscl, format, sender, status,
		   total_records, valid_records, error_records,
		   zip_filename, zip_bytes, attempt_no, sent_at, fdh_txn_id)
		VALUES ('12345','203096','UCS','16FILES','FDH','sent',
		        1,1,0,'12345UCS203096-sent.ZIP',$1,1,now(),'TXN-SENT')
		RETURNING batch_id::text
	`, []byte("fake-zip-sent")); err != nil {
		t.Fatalf("seed sent batch: %v", err)
	}

	repo := store.NewPgRetryRepo(conn)

	// SelectRetryable should return ONLY the error row.
	rows, err := repo.SelectRetryable(ctx, 10, 5)
	if err != nil {
		t.Fatalf("SelectRetryable: %v", err)
	}
	gotErrorRow := false
	for _, r := range rows {
		if r.BatchID == sentBatchID {
			t.Errorf("sent batch returned as retryable: %s", r.BatchID)
		}
		if r.BatchID == errorBatchID {
			gotErrorRow = true
			if string(r.ZipBytes) != "fake-zip-payload" {
				t.Errorf("zip_bytes not loaded: %q", string(r.ZipBytes))
			}
			if r.AttemptNo != 1 {
				t.Errorf("attempt_no = %d, want 1", r.AttemptNo)
			}
		}
	}
	if !gotErrorRow {
		t.Fatalf("error batch not in retryable set: %+v", rows)
	}

	// RecordRetry(success=true) should flip status → sent + insert send_log.
	err = repo.RecordRetry(ctx, store.RecordRetryArgs{
		BatchID:      errorBatchID,
		NewAttemptNo: 2,
		MaxAttempts:  5,
		Success:      true,
		Duration:     123 * time.Millisecond,
		Endpoint:     "/api/claim/16files",
		TxnID:        "TXN-RETRY-OK",
	})
	if err != nil {
		t.Fatalf("RecordRetry: %v", err)
	}

	// Verify status + attempt_no + txn_id on claim_batch.
	var status, txn string
	var attempt int
	if err := conn.QueryRow(`
		SELECT status, attempt_no, COALESCE(fdh_txn_id,'')
		  FROM claim_batch WHERE batch_id=$1::uuid
	`, errorBatchID).Scan(&status, &attempt, &txn); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "sent" {
		t.Errorf("status = %q, want sent", status)
	}
	if attempt != 2 {
		t.Errorf("attempt_no = %d, want 2", attempt)
	}
	if txn != "TXN-RETRY-OK" {
		t.Errorf("txn = %q", txn)
	}

	// Verify a send_log row appeared with attempt_no=2.
	var logCount int
	if err := conn.Get(&logCount, `
		SELECT COUNT(*) FROM send_log WHERE batch_id=$1::uuid AND attempt_no=2 AND success=true
	`, errorBatchID); err != nil {
		t.Fatalf("count send_log: %v", err)
	}
	if logCount != 1 {
		t.Errorf("send_log attempt=2 rows = %d, want 1", logCount)
	}

	// Now SelectRetryable should skip this batch (status='sent').
	rows2, err := repo.SelectRetryable(ctx, 10, 5)
	if err != nil {
		t.Fatalf("SelectRetryable 2: %v", err)
	}
	for _, r := range rows2 {
		if r.BatchID == errorBatchID {
			t.Errorf("sent batch still picked as retryable")
		}
	}

	// Cleanup
	for _, q := range []string{
		`DELETE FROM send_log     WHERE batch_id IN ($1::uuid, $2::uuid)`,
		`DELETE FROM claim_record WHERE batch_id IN ($1::uuid, $2::uuid)`,
		`DELETE FROM claim_batch  WHERE batch_id IN ($1::uuid, $2::uuid)`,
	} {
		if _, err := conn.Exec(q, errorBatchID, sentBatchID); err != nil {
			t.Errorf("cleanup %s: %v", q, err)
		}
	}
}
