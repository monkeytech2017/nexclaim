#!/usr/bin/env bash
# โหลด master data จาก JSON files เข้า DB
# Usage: ./scripts/seed_master.sh
set -euo pipefail

DB_NAME="${DB_NAME:-nexclaim}"
echo "[seed] Loading master data into $DB_NAME..."

# TODO: implement seed script
# psql $DB_NAME -c "COPY m_icd10 FROM 'data/icd10.json' ..."
# psql $DB_NAME -c "COPY m_tmt_drug FROM 'data/tmt.json' ..."

echo "[seed] Done"
