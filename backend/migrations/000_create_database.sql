-- ============================================================
-- NexClaim Migration 000: Create Database
--   Run as a Postgres superuser BEFORE the other migrations.
--   Usage:   psql -U postgres -f migrations/000_create_database.sql
-- ============================================================
--
-- Locale th_TH.UTF-8 ต้องถูก generate ใน OS ก่อน (ดู README ของ script)
-- — ถ้ายังไม่มี PostgreSQL จะ error "invalid locale name: th_TH.UTF-8".
-- ============================================================

-- Drop ถ้ามี (comment ออกถ้าไม่อยากให้ทำลายของเดิม)
-- DROP DATABASE IF EXISTS nexclaim;

-- CREATE DATABASE ต้องใช้ TEMPLATE template0 เมื่อเปลี่ยน collation
CREATE DATABASE nexclaim
    WITH
    OWNER      = CURRENT_USER
    ENCODING   = 'UTF8'
    LC_COLLATE = 'th_TH.UTF-8'
    LC_CTYPE   = 'th_TH.UTF-8'
    TEMPLATE   = template0
    CONNECTION LIMIT = -1;

COMMENT ON DATABASE nexclaim IS 'NexClaim — Healthcare Claim Middleware (th_TH.UTF-8)';

-- Switch เข้า DB ใหม่แล้วเปิด extensions ที่ migrations ใช้
\connect nexclaim

CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid() for PG < 13
CREATE EXTENSION IF NOT EXISTS "uuid-ossp"; -- uuid_generate_v4() fallback

-- (Optional) สร้าง application role แยกจาก superuser — uncomment ถ้าต้องการ
-- CREATE ROLE nexclaim_app WITH LOGIN PASSWORD 'CHANGE_ME';
-- GRANT CONNECT ON DATABASE nexclaim TO nexclaim_app;
-- GRANT USAGE ON SCHEMA public TO nexclaim_app;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public
--     GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO nexclaim_app;

-- Show effective settings for verification
SELECT datname, pg_encoding_to_char(encoding) AS encoding,
       datcollate, datctype
FROM pg_database WHERE datname = 'nexclaim';
