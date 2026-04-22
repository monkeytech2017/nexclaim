-- ============================================================
-- NexClaim Migration 005: API Key (auth)
--
-- Gates every non-public endpoint behind a bearer API key.
-- Two roles:
--   admin    — unrestricted
--   hospital — bound to exactly one hcode (m_hospital.hcode)
--
-- Keys are stored as SHA-256 hex of the raw token; the raw key
-- is returned ONCE at creation time and never again.
-- ============================================================

-- pgcrypto is needed for gen_random_uuid(). Safe to re-run.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE api_key (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash     VARCHAR(64)  NOT NULL UNIQUE,          -- SHA-256 hex of the raw key (64 chars)
    role         VARCHAR(20)  NOT NULL CHECK (role IN ('admin','hospital')),
    hcode        VARCHAR(5)   REFERENCES m_hospital(hcode) ON DELETE RESTRICT,
    name         TEXT         NOT NULL,                 -- human label, e.g. "ops-laptop-2026"
    is_active    BOOLEAN      NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    -- Enforce role ↔ hcode invariant at the storage layer so bugs in
    -- handler code cannot mint an "admin with hcode" or a hospital key
    -- without a bound hospital.
    CONSTRAINT api_key_role_hcode_ck CHECK (
        (role = 'admin'    AND hcode IS NULL)
     OR (role = 'hospital' AND hcode IS NOT NULL)
    )
);

-- Hot lookup path: middleware does GetByHash on every request.
CREATE INDEX idx_api_key_active_hash ON api_key (key_hash) WHERE is_active = true;
CREATE INDEX idx_api_key_hcode       ON api_key (hcode) WHERE hcode IS NOT NULL;

-- CRUD grant to the nexclaim app role. TRUNCATE intentionally omitted.
-- scripts/create_user.sql grants ALL TABLES in public (incl. future tables
-- via ALTER DEFAULT PRIVILEGES), so this is additive — but we only run it
-- if the role actually exists so the migration is safe on a fresh DB where
-- create_user.sh has not been invoked yet.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'nexclaim') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON api_key TO nexclaim';
    END IF;
END $$;
