#!/usr/bin/env bash
# ============================================================
# NexClaim — Init Postgres database
#
# รัน script นี้ครั้งเดียวตอน setup — จะสร้าง DB + apply migrations
# 000..003 ลำดับให้ครบ.
#
# Prerequisites:
#   1. PostgreSQL 14+ ติดตั้งแล้ว (recommended — มี gen_random_uuid() ในตัว)
#   2. Locale th_TH.UTF-8 generate ใน OS:
#        macOS:  installed by default (ไม่ต้องทำอะไร)
#        Ubuntu: sudo locale-gen th_TH.UTF-8 && sudo update-locale
#                (แล้ว restart postgres container/service)
#   3. Superuser role ที่เข้าได้ (default: postgres)
#
# Usage:
#   PGHOST=localhost PGPORT=5432 PGUSER=postgres ./scripts/init_db.sh
#   # or minimum (uses PG defaults):
#   ./scripts/init_db.sh
#
# Environment variables:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD — standard libpq vars
#   DB_NAME                            — target database (default: nexclaim)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MIGRATIONS_DIR="$SCRIPT_DIR/../migrations"
DB_NAME="${DB_NAME:-nexclaim}"

echo "[init_db] target: PGHOST=${PGHOST:-localhost} PGPORT=${PGPORT:-5432} PGUSER=${PGUSER:-postgres} DB=$DB_NAME"

# Sanity: Postgres reachable?
if ! psql -d postgres -c "SELECT 1" >/dev/null 2>&1; then
  echo "[init_db] ERROR: cannot connect to Postgres. Check PGHOST/PGPORT/PGUSER + run pg_isready." >&2
  exit 1
fi

# Sanity: locale available?
if ! psql -d postgres -tAc "SELECT 1 FROM pg_collation WHERE collname IN ('th_TH.UTF-8','th_TH.utf8') LIMIT 1" | grep -q 1; then
  cat >&2 <<EOF
[init_db] ERROR: locale 'th_TH.UTF-8' not available in Postgres.
  macOS:    /usr/bin/locale -a | grep th_TH   (should list th_TH.UTF-8)
  Ubuntu:   sudo locale-gen th_TH.UTF-8 && sudo systemctl restart postgresql
  Docker:   rebuild image with  RUN sed -i '/th_TH.UTF-8/s/^# //' /etc/locale.gen && locale-gen
EOF
  exit 1
fi

# Step 1: create DB (skip if exists)
if psql -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1; then
  echo "[init_db] DB '$DB_NAME' already exists — skip CREATE DATABASE"
else
  echo "[init_db] creating DB '$DB_NAME' with th_TH.UTF-8 collation..."
  psql -d postgres -v ON_ERROR_STOP=1 -f "$MIGRATIONS_DIR/000_create_database.sql"
fi

# Step 2: apply schema migrations
for migration in 001_master_data.sql 002_his_mapping.sql 003_transactions.sql 004_ingest_batch.sql; do
  echo "[init_db] applying $migration..."
  psql -d "$DB_NAME" -v ON_ERROR_STOP=1 -f "$MIGRATIONS_DIR/$migration"
done

echo "[init_db] done."
psql -d "$DB_NAME" -c "\dt"
