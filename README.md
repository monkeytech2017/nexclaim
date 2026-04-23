<div align="center">
  <h1>NexClaim</h1>
  <p><em>Every claim, every fund — connected.</em></p>
  <p>
    <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go">
    <img src="https://img.shields.io/badge/Next.js-14-000000?style=flat&logo=next.js" alt="Next.js 14">
    <img src="https://img.shields.io/badge/PostgreSQL-16+-4169E1?style=flat&logo=postgresql" alt="PostgreSQL">
    <img src="https://img.shields.io/badge/FDH-Ready-185FA5?style=flat" alt="FDH">
    <img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="MIT">
  </p>
</div>

Healthcare Claim Middleware สำหรับโรงพยาบาลไทย — รับข้อมูลจาก HIS (API 2-way หรือ share folder), สร้าง claim ตาม format ของแต่ละกองทุน (16 แฟ้ม / CIPN / CSOP / AIPN / SSOP), ส่งขึ้น FDH/CHI, และดึงผล REP กลับมาเป็น C-code เพื่อให้ รพ.แก้ไข.

## Monorepo

```
nexclaim/
├── backend/     — Go (gin + sqlx + stdlib encoding/xml) · CLI + HTTP server
├── frontend/    — Next.js 14 App Router · React Query · Tailwind · Recharts
└── .claude/     — agents/ (briefings for Claude Code subagents)
```

## กองทุนที่รองรับ

| สิทธิ (INSCL) | Format | ช่องทาง |
|---|---|---|
| บัตรทอง (UCS) / ไร้สัญชาติ (NON) / ต่างด้าว (WP1, WP2) | 16 แฟ้ม | FDH → สปสช. |
| ข้าราชการ (011, WEL) | CIPN (IPD) / CSOP (OPD) | FDH → CGD |
| อปท. (LGO) | CIPN / CSOP | FDH → CGD |
| หน่วยงานอิสระ (OFC: NBTC, BAAC, ECT, PEA, MEA, MWA, SRT) | CIPN / CSOP | FDH → Agency |
| ประกันสังคม (SSS ม.33/39, SS4 ม.40) | AIPN (IPD) / SSOP (OPD) | cs8.chi.or.th |
| พ.ร.บ.รถ (TPBS) · พระภิกษุ (MON) | 16 แฟ้ม | FDH |
| กองทุนทดแทน (WK) | 16 แฟ้ม | WCF |
| ราชทัณฑ์ (PRS) | 16 แฟ้ม | DOC |

เต็มๆ + routing code ดู [`backend/internal/router/router.go`](backend/internal/router/router.go) และ [CLAUDE.md §6](CLAUDE.md).

## Quick start

```bash
# Database (ครั้งแรก — th_TH.UTF-8 collation + migrations 000–005)
cd backend
PGHOST=localhost PGUSER=postgres ./scripts/init_db.sh
PGHOST=<rds> PGUSER=<admin> PGPASSWORD=<pw> ./scripts/create_user.sh nexclaim

# Backend
cp ../.env.example .env   # แก้ credentials
go mod tidy
go build -o nexclaim .
go test ./...             # 148 tests; Pg integration skips without POSTGRES_TEST_DSN

# Mint your first admin key (ต้องมีฐานข้อมูลพร้อมแล้ว)
./nexclaim auth create-admin --name "ops"   # แสดง raw key ครั้งเดียว — copy ให้ดี

# HTTP server
AUTH_ENABLED=true ./nexclaim server --addr :8080
curl http://localhost:8080/healthz

# Frontend
cd ../frontend
npm install
npm run dev               # เข้า http://localhost:3000/login → วาง API key
```

## ความสามารถ

- **OPD 2-way API** — HIS push visit list → NexClaim pulls detail → รัน pipeline (`POST /api/v1/his/opd/visits`)
- **IPD share folder** — HIS drop CSVs + `MANIFEST.json` ใน `incoming/` → watcher หรือ HTTP trigger รัน pipeline → archive ไป `processed/` หรือ `error/`
- **Pipeline** — extract → validate (MasterValidator from DB) → bucket by INSCL → generate → zip+md5 → submit → `SaveRun(claim_batch + claim_record + send_log)`
- **REP feedback loop** — ดึง REP จาก FDH → parse → upsert `c_code_log` → UI for review/resolve
- **Master data** — CRUD + bulk CSV import (Hospitals · Doctors · INSCL maps · Drug maps (TMT24) · Doctor maps (DRDX) · ICD maps · Field maps)
- **Observability** — `send_log` captures every outbound attempt (endpoint, duration, success/error); dashboard aggregates batches / records / c-codes / send latency
- **Auth** — API-key per identity, two roles (`admin`, `hospital`); hospital keys are scoped to a single hcode at the middleware layer; admin-only `/admin/api-keys` page to mint/list/deactivate

## เอกสารละเอียด

- **[CLAUDE.md](CLAUDE.md)** — full stack reference (sections 1–16). ที่มาของทุก convention ในรีโปนี้
- **[.claude/agents/](.claude/agents/)** — subagent briefings for Claude Code sessions (`nexclaim-backend`, `nexclaim-frontend`, `nexclaim-domain`)
- FDH portal: https://fdh.moph.go.th/hospital/ · คู่มือ: https://www.sshos.go.th/financial-data-hub/
- CHI (SSO): https://cs8.chi.or.th
- ICD-10 WHO: https://icd.who.int/browse10/ · TMT drug master: https://tmt.this.or.th

## License

MIT
