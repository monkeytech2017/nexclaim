-- ============================================================
-- NexClaim Migration 006: Audit Log (append-only)
--
-- Records admin-sensitive actions (who did what, when, where).
-- Written best-effort by server handlers AFTER the triggering
-- action succeeds — audit-write errors must NOT fail the action.
--
-- Access model:
--   - nexclaim role: SELECT + INSERT only (no UPDATE/DELETE in prod).
--     Tests clean up with DELETE — we grant DELETE there only if
--     you choose to extend in CI; production grant stays append-only.
-- ============================================================

-- pgcrypto is needed for gen_random_uuid(). Safe to re-run.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE audit_log (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id     UUID,                                         -- api_key.id (nullable: bootstrap, auth-disabled)
    actor_role   VARCHAR(20),                                  -- 'admin' | 'hospital' | 'system' | ''
    actor_name   TEXT,                                         -- denormalized — survives key deletion
    action       VARCHAR(60)   NOT NULL,                       -- e.g. 'api_key.create'
    target_kind  VARCHAR(40),                                  -- 'api_key' | 'ccode' | 'rep' | 'claim_batch' | ...
    target_id    VARCHAR(100),
    hcode        VARCHAR(5),                                   -- which hospital scope (nullable for admin-wide)
    payload      JSONB,                                        -- freeform extras — redacted (never raw keys/PII)
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX ON audit_log (created_at DESC);
CREATE INDEX ON audit_log (action);
CREATE INDEX ON audit_log (actor_id);
CREATE INDEX ON audit_log (hcode) WHERE hcode IS NOT NULL;
CREATE INDEX ON audit_log (target_kind, target_id);

-- nexclaim role needs INSERT + SELECT (admin reads); no UPDATE/DELETE (append-only audit).
-- Guard so migration is safe on a fresh DB where create_user.sh has not run yet.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'nexclaim') THEN
        EXECUTE 'GRANT SELECT, INSERT ON audit_log TO nexclaim';
    END IF;
END $$;
