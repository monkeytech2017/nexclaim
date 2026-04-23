-- ============================================================
-- NexClaim Migration 007: API Key TTL / expiry
--
-- Adds an optional expires_at column. NULL = never expires
-- (the default for back-compat with rows minted before this
-- migration). Middleware rejects rows whose expires_at has
-- passed with the same 401 body as unknown keys — no leak.
--
-- Partial index: we only care about rows that CAN expire, and
-- keeping the index skinny avoids bloating the hot GetByHash
-- lookup path (which already has idx_api_key_active_hash).
-- ============================================================

ALTER TABLE api_key
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_api_key_expires_at
    ON api_key (expires_at)
    WHERE expires_at IS NOT NULL;
