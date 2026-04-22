#!/usr/bin/env bash
# ============================================================
# NexClaim — Create application database role
#
# สร้าง (หรือ reset password ให้) role ที่ใช้ connect จาก backend.
# Run as a Postgres superuser — ต้องเข้า DB nexclaim ได้ก่อน.
#
# Usage:
#   ./scripts/create_user.sh                          # prompt password
#   ./scripts/create_user.sh nexclaim_rw              # custom role name
#   DB_USER_PASSWORD='…' ./scripts/create_user.sh     # non-interactive
#
# Environment:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD   — standard libpq vars
#   DB_NAME                               — default: nexclaim
#   DB_USER_PASSWORD                      — skip prompt (CI/scripted)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROLE="${1:-nexclaim_app}"
DB_NAME="${DB_NAME:-nexclaim}"

# Sanity: Postgres reachable?
if ! psql -d "$DB_NAME" -c "SELECT 1" >/dev/null 2>&1; then
  echo "[create_user] ERROR: cannot connect to Postgres db=$DB_NAME" >&2
  echo "  check PGHOST/PGPORT/PGUSER + that init_db.sh has been run" >&2
  exit 1
fi

# Prompt only if password not provided via env.
if [ -z "${DB_USER_PASSWORD:-}" ]; then
  read -rs -p "Password for role '$ROLE' (input hidden): " DB_USER_PASSWORD
  echo
  if [ -z "${DB_USER_PASSWORD}" ]; then
    echo "[create_user] ERROR: password cannot be empty" >&2
    exit 1
  fi
fi

psql -d "$DB_NAME" \
     -v ON_ERROR_STOP=1 \
     -v role="$ROLE" \
     -v password="$DB_USER_PASSWORD" \
     -f "$SCRIPT_DIR/create_user.sql"

cat <<EOF

[create_user] done.

ถ้ายังไม่ได้ตั้ง backend/.env ให้เพิ่ม/แก้บรรทัดเหล่านี้
(copy จาก ../.env.example ก่อนถ้าไฟล์ยังไม่มี):

  DB_HOST=${PGHOST:-localhost}
  DB_PORT=${PGPORT:-5432}
  DB_NAME=$DB_NAME
  DB_USER=$ROLE
  DB_PASS=<password ที่ตั้งไว้>

ทดสอบ connection:
  cd backend && ./nexclaim migrate status
EOF
