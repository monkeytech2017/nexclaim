-- ============================================================
-- NexClaim Migration 008: Send Retry Loop
-- ============================================================
--
-- Adds the state required for a background worker to retry failed
-- outbound submissions (pipeline.submitOne). The retry worker picks
-- claim_batch rows in status='error' whose attempt_no < max, rebuilds
-- the ZIP from zip_bytes, calls the sender, writes a new send_log row,
-- then flips status to 'sent' or 'failed' (terminal).
--
-- Schema choices:
--   zip_bytes       BYTEA   — stored at insert time so the worker never
--                              re-runs the pipeline. Typical size 100KB–5MB.
--                              Nullable because historical rows don't have it.
--   attempt_no      INT     — count of send attempts written so far (1 = the
--                              initial pipeline run, 2+ = retries).
--   next_retry_at   TIMESTAMPTZ — earliest time the worker should pick this
--                                  row again. Exponential backoff capped at 30min.
--
-- claim_batch.status is plain TEXT (no CHECK constraint) — the new
-- terminal state 'failed' is added by convention, enforced in Go.

ALTER TABLE claim_batch ADD COLUMN zip_bytes     BYTEA          NULL;
ALTER TABLE claim_batch ADD COLUMN attempt_no    INT            NOT NULL DEFAULT 1;
ALTER TABLE claim_batch ADD COLUMN next_retry_at TIMESTAMPTZ    NULL;

-- Worker index: partial covers only rows still eligible for retry.
CREATE INDEX claim_batch_retry_idx
    ON claim_batch (next_retry_at, attempt_no)
    WHERE status = 'error';
