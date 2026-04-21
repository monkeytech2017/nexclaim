<div align="center">
  <h1>NexClaim</h1>
  <p><em>Every claim, every fund — connected.</em></p>
  <p>
    <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go">
    <img src="https://img.shields.io/badge/PostgreSQL-16+-4169E1?style=flat&logo=postgresql" alt="PostgreSQL">
    <img src="https://img.shields.io/badge/FDH-Ready-185FA5?style=flat" alt="FDH">
    <img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="MIT">
  </p>
</div>

Healthcare Claim Middleware สำหรับโรงพยาบาลไทย — รับข้อมูลจาก HIS, แปลงเป็น format มาตรฐาน (16 แฟ้ม / CIPN / CSOP / AIPN / SSOP) และส่งไปยังกองทุนสุขภาพภาครัฐทุกประเภทผ่าน MOPH Financial Data Hub

## กองทุนที่รองรับ

| สิทธิ | Format | ช่องทาง |
|-------|--------|---------|
| บัตรทอง (UCS) | 16 แฟ้ม | FDH → สปสช. |
| ข้าราชการ CSMBS (011, WEL) | CIPN/CSOP | FDH → กรมบัญชีกลาง |
| อปท./LGO | CIPN/CSOP | FDH → CGD |
| กสทช./กฟก./กกต./สผผ./กฟน./MWA/SRT | CIPN/CSOP | FDH → Agency |
| ประกันสังคม ม.33/39 (SSS) | AIPN/SSOP | cs8.chi.or.th |
| ประกันสังคม ม.40 (SS4) | AIPN/SSOP | cs8.chi.or.th |
| พ.ร.บ.รถ (TPBS) | 16 แฟ้ม | FDH |
| แรงงานต่างด้าว (WP1/WP2) | 16 แฟ้ม | FDH → สปสช. |

## Quick Start

```bash
git clone https://github.com/nexclaim/nexclaim
cd nexclaim
cp .env.example .env   # แก้ไข credentials
go mod tidy
go build -o nexclaim .

# CLI
./nexclaim submit --inscl UCS  --period 202504
./nexclaim submit --inscl 011  --period 202504
./nexclaim submit --inscl SSS  --period 202504 --dry-run
./nexclaim status  --txn-id <id>

# HTTP server
./nexclaim server
```

## Database Setup

```bash
createdb nexclaim
psql nexclaim < migrations/001_master_data.sql
psql nexclaim < migrations/002_his_mapping.sql
psql nexclaim < migrations/003_transactions.sql
```

## Project Structure

```
nexclaim/
├── cmd/                    CLI commands (submit, status, server)
├── internal/
│   ├── model/              Domain types (INSCL, Patient, OPDVisit, IPDAdmit)
│   ├── config/             Environment config loader
│   ├── router/             Rights router: INSCL → format + sender
│   ├── extractor/          ดึงข้อมูลจาก HIS DB
│   ├── generator/          สร้าง claim files (file16, cipn, csop, aipn, ssop)
│   ├── validator/          ตรวจสอบ field, ICD-10, ICD-9CM, business rules
│   ├── sender/             ส่งผ่าน FDH API และ cs8.chi.or.th
│   ├── response/           Parse ACK และ C-code จาก REP
│   └── util/               Date utilities (ToAD, FormatDate)
├── migrations/             SQL migration files
│   ├── 001_master_data.sql     Master data (INSCL, ICD-10, TMT, CHRGITEM)
│   ├── 002_his_mapping.sql     HIS field/drug/doctor mapping
│   └── 003_transactions.sql    Transaction + log tables
├── data/                   Master data JSON (icd10, icd9cm, tmt, chrgitem)
├── tests/                  Unit tests
└── docs/                   Documentation
```

## ลำดับการออกแบบ

```
Master Data (หน่วยงาน)
    ↓
DB Schema (NexClaim)
    ↓
HIS Field Mapping (ต่อ รพ.)
    ↓
Generator + Validator
    ↓
Sender → FDH / CHI
```

## ดูเพิ่มเติม

- [CLAUDE.md](./CLAUDE.md) — context สำหรับ Claude Code
- [FDH Portal](https://fdh.moph.go.th/hospital/)
- [คู่มือ FDH](https://www.sshos.go.th/financial-data-hub/)
- [AIPN Upload](https://cs8.chi.or.th/aipnupload/)
