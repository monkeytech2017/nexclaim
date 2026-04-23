-- ============================================================
-- NexClaim — Create application role (idempotent)
--
-- Expects two psql variables:
--   :'role'       — role name, e.g. 'nexclaim_app'
--   :'password'   — plaintext password (sent over protocol, store securely)
--
-- Run via scripts/create_user.sh; that wrapper prompts for the password
-- and passes it with -v so it never appears on disk in shell history.
-- ============================================================

-- CREATE if missing, ALTER password if exists.
SELECT CASE
    WHEN EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'role')
        THEN format('ALTER ROLE %I WITH LOGIN PASSWORD %L',  :'role', :'password')
        ELSE format('CREATE ROLE %I WITH LOGIN PASSWORD %L', :'role', :'password')
END \gexec

-- CRUD privileges on nexclaim.public (current + future tables)
GRANT CONNECT ON DATABASE nexclaim TO :"role";
GRANT USAGE   ON SCHEMA   public   TO :"role";

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA public TO :"role";
GRANT USAGE,  SELECT                 ON ALL SEQUENCES IN SCHEMA public TO :"role";

-- Explicit per-table grants (redundant with ALL TABLES above, but spelled out
-- so a future reader sees exactly which app-writable tables exist). TRUNCATE
-- is intentionally NOT granted — tests use DELETE FROM child → parent.
DO $grants$
DECLARE
    t text;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'm_hospital','m_doctor','m_inscl','m_agency','m_chrgitem','m_icd10','m_icd9cm','m_tmt',
        'his_inscl_map','his_drug_map','his_doctor_map','his_icd_map','his_field_map',
        'claim_batch','claim_record','c_code_log','send_log',
        'opd_ingest_batch','opd_ingest_visit',
        'api_key'
    ] LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename=t) THEN
            EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%I TO %I', t, :'role');
        END IF;
    END LOOP;
END
$grants$;

-- audit_log is append-only: SELECT + INSERT only. No UPDATE/DELETE even for
-- the app role — tamper-resistance for the audit trail.
DO $audit$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='audit_log') THEN
        EXECUTE format('GRANT SELECT, INSERT ON public.audit_log TO %I', :'role');
    END IF;
END
$audit$;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO :"role";
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE,  SELECT                 ON SEQUENCES TO :"role";

\echo Role :"role" ready — CRUD granted on database nexclaim (schema public).
