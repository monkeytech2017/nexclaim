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

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO :"role";
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE,  SELECT                 ON SEQUENCES TO :"role";

\echo Role :"role" ready — CRUD granted on database nexclaim (schema public).
