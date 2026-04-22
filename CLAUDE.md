# CLAUDE.md — NexClaim

> **NexClaim** — Healthcare Claim Middleware
> *Every claim, every fund — connected.*
> อัปเดตล่าสุด: เมษายน 2569 (monorepo: `backend/` + `frontend/` · RDS Postgres th_TH.UTF-8 · Master CRUD + REP feedback loop + IPD watcher)

---

## App Identity

| | |
|---|---|
| App name | NexClaim |
| Tagline | *Every claim, every fund — connected.* |
| Module | `github.com/nexclaim/nexclaim` |
| CLI | `nexclaim submit --inscl UCS --period 202504` |
| Primary color | `#185FA5` |
| Accent color | `#378ADD` |

---

## 1. Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | **Go 1.22+** |
| Web framework | `github.com/gin-gonic/gin` |
| XML | `encoding/xml` stdlib — struct tag map ตรง spec |
| ZIP + MD5 | `archive/zip`, `crypto/md5` stdlib |
| HTTP client | `github.com/go-resty/resty/v2` |
| Database | PostgreSQL — `github.com/jmoiron/sqlx` + `github.com/lib/pq` |
| Config | `github.com/joho/godotenv` |
| Logger | `github.com/rs/zerolog` |
| CLI | `github.com/spf13/cobra` |
| Test | `github.com/stretchr/testify` |

**ทำไมถึงเลือก Go:**
- `encoding/xml` + struct tag → field ผิดชื่อ = **build error ทันที** ไม่รอ runtime
- compile เป็น single binary → deploy แค่ copy ไฟล์เดียว ไม่ต้องติดตั้ง runtime
- goroutine → รับ concurrent claim หลาย รพ. พร้อมกันได้ดี
- stdlib ครอบคลุมเกือบทุกอย่าง (XML, ZIP, MD5, HTTP)

---

## 2. โครงสร้าง Project

```
nexconnect/
├── CLAUDE.md
├── README.md
├── .env.example
├── NexClaim_HIS_Integration_Spec.xlsx   ← HIS team reference (OPD 2-way + IPD share-file)
├── backend/                              ← Go service
│   ├── go.mod / go.sum
│   ├── main.go                           ← calls cmd.Execute()
│   ├── cmd/
│   │   ├── root.go                       ← flag-stdlib dispatcher (submit/status/server)
│   │   ├── submit.go                     ← CLI: submit --inscl --period [--dry-run]
│   │   └── server.go                     ← CLI: server (gin HTTP)
│   ├── internal/
│   │   ├── model/model.go                ← domain types: Patient, OPDVisit, IPDAdmit, INSCL, ...
│   │   ├── config/config.go              ← load .env via godotenv + validate
│   │   ├── router/router.go              ← Route(inscl, isIPD) → RouteResult
│   │   ├── pipeline/pipeline.go          ← extractor → validator → generator → sender
│   │   ├── extractor/
│   │   │   ├── his.go                    ← Extractor interface + MemoryExtractor
│   │   │   ├── api.go                    ← APIExtractor (OPD 2-way via HIS client)
│   │   │   └── file.go                   ← FileExtractor (IPD share folder)
│   │   ├── hisclient/                    ← HIS API client (OPD 2-way)
│   │   │   ├── dto.go                    ← request/response JSON structs
│   │   │   ├── client.go                 ← resty: GetVisit, GetVisitsBatch, Health, GetDrugList
│   │   │   └── mapper.go                 ← VisitDetail → model.OPDVisit
│   │   ├── sharefile/                    ← IPD share folder (CSV)
│   │   │   ├── manifest.go               ← MANIFEST.json parse + validate
│   │   │   ├── csv.go                    ← pipe-delimited UTF-8 reader
│   │   │   ├── parse.go                  ← 10 row types + per-file parsers
│   │   │   └── assemble.go               ← join rows → []model.IPDAdmit
│   │   ├── batch/                        ← OPD ingest batch (interface + Memory/Pg impls)
│   │   │   ├── store.go                  ← Store interface + MemoryStore + New helpers
│   │   │   └── pg_store.go               ← Postgres-backed impl (opd_ingest_batch/_visit)
│   │   ├── ipdimport/processor.go        ← shared IPD pipeline (HTTP handler + watcher)
│   │   ├── watcher/ipd.go                ← polling watcher (NFS/SMB-friendly, not fsnotify)
│   │   ├── db/                           ← db.go (sqlx pool) + migrate.go (migration status)
│   │   ├── generator/
│   │   │   ├── file16/ (file16.go + records.go)  ← 16 แฟ้ม pipe-delimited
│   │   │   ├── cipn/cipn.go              ← XML CIPN (IPD ข้าราชการ/อปท./OFC)
│   │   │   ├── csop/csop.go              ← XML CSOP (OPD ข้าราชการ/อปท./OFC)
│   │   │   ├── aipn/aipn.go              ← XML AIPN (IPD ประกันสังคม)
│   │   │   └── ssop/ssop.go              ← XML SSOP (OPD ประกันสังคม)
│   │   ├── validator/                    ← field.go, icd.go, rules.go, masters.go
│   │   │                                    (MasterValidator iface + NoopMaster/StaticMaster + LoadFromDB)
│   │   ├── sender/                       ← fdh.go, chi.go, zip.go
│   │   ├── response/                     ← ack.go, ccode.go (ParseREP)
│   │   ├── store/                        ← Postgres repositories (sqlx)
│   │   │   ├── claim_repo.go + claim_repo_pg.go       ← SaveRun (claim_batch + claim_record)
│   │   │   ├── claim_batch_repo.go                    ← list/get submissions (joins c_code_log counts)
│   │   │   ├── hospital_repo.go / doctor_repo.go      ← m_hospital / m_doctor CRUD
│   │   │   ├── inscl_map_repo.go / drug_map_repo.go   ← his_inscl_map / his_drug_map
│   │   │   ├── doctor_map_repo.go / icd_map_repo.go   ← his_doctor_map / his_icd_map
│   │   │   ├── field_map_repo.go                      ← his_field_map
│   │   │   ├── ccode_repo.go                          ← c_code_log CRUD + resolve
│   │   │   ├── rep_ingest.go                          ← fetch + parse FDH REP → c_code_log
│   │   │   └── master_loader.go                       ← load masters → validator
│   │   ├── server/                       ← gin HTTP (split by concern)
│   │   │   ├── server.go                 ← Deps struct + router
│   │   │   ├── submissions.go            ← claim_batch list/get
│   │   │   ├── ccodes.go                 ← c-code list/resolve + fetch REP
│   │   │   ├── field_maps.go             ← his_field_map handlers
│   │   │   └── bulk.go                   ← bulk CSV import (ICD/TMT/doctor)
│   │   └── util/                         ← date.go, str.go (StrOr, etc.)
│   ├── migrations/                       ← 000_create_database (th_TH.UTF-8), 001_master_data,
│   │                                        002_his_mapping, 003_transactions, 004_ingest_batch
│   ├── scripts/
│   │   ├── init_db.sh                    ← check locale + create DB + apply migrations
│   │   ├── create_user.sh + .sql         ← create role + GRANT CRUD (no TRUNCATE) — idempotent
│   │   └── seed_master.sh                ← bulk-import ICD10/ICD9CM/TMT via HTTP
│   ├── data/                             ← icd10.json, icd9cm.json, tmt.json, chrgitem.json
│   └── tests/                            ← *_test.go (111 tests: router, pipeline, file16, cipn, csop,
│                                             aipn, ssop, hisclient, sharefile, server, submissions,
│                                             ccodes, ipd_import, *_pg integration [skip w/o DSN], ...)
└── frontend/                             ← Next.js 14 App Router
    ├── package.json / tsconfig.json / tailwind.config.ts
    └── src/
        ├── app/(dashboard)/              ← dashboard, opd-batches, ipd-imports, claims,
        │                                    history, submissions, c-codes,
        │                                    admin/{hospitals,doctors,inscl-maps,drug-maps,
        │                                           doctor-maps,icd-maps,field-maps}
        ├── components/                   ← layout/{Sidebar,Header}, BulkImportModal,
        │                                    SubmissionOutcome, ui/{feedback,badges}
        └── lib/                          ← api.ts (typed client), csv.ts, providers.tsx
```

---

## 3. Domain Model (`internal/model/model.go`)

```go
// INSCL รหัสสิทธิการรักษา
type INSCL string

const (
    INSCL_CSMBS INSCL = "011"  // ข้าราชการพลเรือน/ทหาร/ตำรวจ
    INSCL_WEL   INSCL = "WEL"  // ลูกจ้างประจำ/บำนาญ
    INSCL_LGO   INSCL = "LGO"  // อปท. (กทม./เทศบาล/อบต./พัทยา)
    INSCL_OFC   INSCL = "OFC"  // หน่วยงานอิสระ — Agency แยกใน CIPN/CSOP header
    INSCL_UCS   INSCL = "UCS"  // บัตรทอง
    INSCL_NON   INSCL = "NON"  // ไร้สัญชาติ/ปัญหาสถานะ
    INSCL_WP1   INSCL = "WP1"  // แรงงานต่างด้าว MOU
    INSCL_WP2   INSCL = "WP2"  // แรงงานต่างด้าวขึ้นทะเบียน
    INSCL_SSS   INSCL = "SSS"  // ประกันสังคม ม.33/39
    INSCL_SS4   INSCL = "SS4"  // ประกันสังคม ม.40
    INSCL_TPBS  INSCL = "TPBS" // พ.ร.บ.ผู้ประสบภัยรถ
    INSCL_WK    INSCL = "WK"   // กองทุนทดแทน
    INSCL_MON   INSCL = "MON"  // พระภิกษุสงฆ์
    INSCL_PRS   INSCL = "PRS"  // ผู้ต้องขัง
)

// Agency สำหรับ CIPN/CSOP header <AGENCY>
type Agency string

const (
    AgencyCGD  Agency = "CGD"  // กรมบัญชีกลาง
    AgencyLGO  Agency = "LGO"  // อปท.
    AgencyNBTC Agency = "NBTC" // กสทช.
    AgencyBAAC Agency = "BAAC" // กฟก. (ธ.ก.ส.)
    AgencyECT  Agency = "ECT"  // กกต.
    AgencyPEA  Agency = "PEA"  // สผผ. (กฟภ.)
    AgencyMEA  Agency = "MEA"  // กฟน.
    AgencyMWA  Agency = "MWA"  // ประปานครหลวง
    AgencySRT  Agency = "SRT"  // การรถไฟ
)

// RouteResult ผลจาก Rights Router
type RouteResult struct {
    INSCL  INSCL
    IsIPD  bool
    Format ClaimFormat // "16FILES" | "CIPN" | "CSOP" | "AIPN" | "SSOP"
    Agency Agency      // สำหรับ CIPN/CSOP เท่านั้น
    Sender Sender      // "FDH" | "CHI" | "WCF" | "DOC"
}
```

---

## 4. Rights Router (`internal/router/router.go`)

```go
func Route(inscl model.INSCL, isIPD bool) (model.RouteResult, error) {
    switch inscl {
    case model.INSCL_CSMBS, model.INSCL_WEL:
        if isIPD {
            return model.RouteResult{Format: model.FormatCIPN, Agency: model.AgencyCGD, Sender: model.SenderFDH}, nil
        }
        return model.RouteResult{Format: model.FormatCSOP, Agency: model.AgencyCGD, Sender: model.SenderFDH}, nil

    case model.INSCL_LGO:
        if isIPD {
            return model.RouteResult{Format: model.FormatCIPN, Agency: model.AgencyLGO, Sender: model.SenderFDH}, nil
        }
        return model.RouteResult{Format: model.FormatCSOP, Agency: model.AgencyLGO, Sender: model.SenderFDH}, nil

    case model.INSCL_OFC:
        // Agency ดึงแยกจาก patient.AgencyCode (query จาก HIS)
        if isIPD {
            return model.RouteResult{Format: model.FormatCIPN, Sender: model.SenderFDH}, nil
        }
        return model.RouteResult{Format: model.FormatCSOP, Sender: model.SenderFDH}, nil

    case model.INSCL_UCS, model.INSCL_NON, model.INSCL_WP1, model.INSCL_WP2, model.INSCL_MON:
        return model.RouteResult{Format: model.Format16Files, Sender: model.SenderFDH}, nil

    case model.INSCL_SSS, model.INSCL_SS4:
        if isIPD {
            return model.RouteResult{Format: model.FormatAIPN, Sender: model.SenderCHI}, nil
        }
        return model.RouteResult{Format: model.FormatSSOP, Sender: model.SenderCHI}, nil

    case model.INSCL_TPBS:
        return model.RouteResult{Format: model.Format16Files, Sender: model.SenderFDH}, nil
    case model.INSCL_WK:
        return model.RouteResult{Format: model.Format16Files, Sender: model.SenderWCF}, nil
    case model.INSCL_PRS:
        return model.RouteResult{Format: model.Format16Files, Sender: model.SenderDOC}, nil

    default:
        return model.RouteResult{Format: model.Format16Files, Sender: model.SenderFDH},
            fmt.Errorf("unknown INSCL %q: routed to 16files/FDH as fallback", inscl)
    }
}
```

---

## 5. XML Struct Pattern (`encoding/xml`)

```go
// struct tag map ตรงกับ spec — ผิดชื่อ = compile error
type CIPNAdmit struct {
    XMLName xml.Name `xml:"ADMIT"`
    AN      string   `xml:"AN"`      // ≤9 หลัก unique
    DateAdm string   `xml:"DATEADM"` // YYYYMMDD (ค.ศ.)
    TimeAdm string   `xml:"TIMEADM"` // HHMM
    DateDsc string   `xml:"DATEDSC"`
    TimeDsc string   `xml:"TIMEDSC"`
    WardDsc string   `xml:"WARDDSC"`
    LOS     int      `xml:"LOS"`
    Dischs  string   `xml:"DISCHS"` // 1=หายดี 2=ดีขึ้น 3=ไม่ดีขึ้น 4=ส่งต่อ 5=ตาย
    Discht  string   `xml:"DISCHT"` // 1=กลับบ้าน 2=ส่งต่อ 3=ตาย
}

// attribute + chardata
type CIPNDiag struct {
    XMLName xml.Name `xml:"DIAG"`
    DxType  string   `xml:"DXTYPE,attr"` // <DIAG DXTYPE="1">
    Code    string   `xml:",chardata"`   // A000
}

// Build + ZIP + MD5
func BuildZipWithMD5(xmlContent []byte, filename string) ([]byte, error) {
    hash := fmt.Sprintf("%x", md5.Sum(xmlContent))
    withHash := append(xmlContent, []byte("\n"+hash)...)
    var buf bytes.Buffer
    w := zip.NewWriter(&buf)
    f, _ := w.Create(filename)
    f.Write(withHash)
    w.Close()
    return buf.Bytes(), nil
}
```

**ชื่อไฟล์ ZIP:**
- CIPN → `{HCODE}CIPN{YYYYMM}.ZIP`
- CSOP → `{HCODE}CSOP{YYYYMM}.ZIP`
- AIPN → `{HCODE}AIPN{YYYYMM}.ZIP`
- SSOP → `{HCODE}SSOP{YYYYMM}.ZIP`

---

## 6. สิทธิที่รองรับ — INSCL Routing

### CSMBS — กรมบัญชีกลาง
| INSCL | กลุ่ม | IPD | OPD | ปลายทาง |
|-------|------|-----|-----|---------|
| `011` | ข้าราชการ/ทหาร/ตำรวจ/ครอบครัว | CIPN | CSOP | FDH → CGD |
| `WEL` | ลูกจ้างประจำ/บำนาญ | CIPN | CSOP | FDH → CGD |

### อปท./LGO
| INSCL | กลุ่ม | IPD | OPD | ปลายทาง |
|-------|------|-----|-----|---------|
| `LGO` | อบจ./เทศบาล/อบต./กทม./พัทยา/บำนาญ | CIPN | CSOP | FDH → CGD |

### หน่วยงานอิสระ (OFC) — Agency Code ใน XML Header
| Agency | หน่วยงาน | IPD | OPD |
|--------|---------|-----|-----|
| `NBTC` | กสทช. | CIPN | CSOP |
| `BAAC` | กฟก. (ธ.ก.ส.) | CIPN | CSOP |
| `ECT`  | กกต. | CIPN | CSOP |
| `PEA`  | สผผ. (กฟภ.) | CIPN | CSOP |
| `MEA`  | กฟน. | CIPN | CSOP |
| `MWA`  | ประปานครหลวง | CIPN | CSOP |
| `SRT`  | การรถไฟ | CIPN | CSOP |

### ประกันสังคม (SSO)
| INSCL | กลุ่ม | IPD | OPD | ปลายทาง |
|-------|------|-----|-----|---------|
| `SSS` | ม.33/39 | AIPN | SSOP | cs8.chi.or.th |
| `SS4` | ม.40 | AIPN | SSOP | cs8.chi.or.th |

### สปสช. (UC)
| INSCL | กลุ่ม | Format | ปลายทาง |
|-------|------|--------|---------|
| `UCS` | บัตรทอง + UCEP | 16 แฟ้ม | FDH → สปสช. |
| `NON` | ไร้สัญชาติ | 16 แฟ้ม | FDH → สปสช. |
| `WP1`/`WP2` | แรงงานต่างด้าว | 16 แฟ้ม | FDH → สปสช. |

### สิทธิพิเศษ
| INSCL | กลุ่ม | Format | ปลายทาง |
|-------|------|--------|---------|
| `TPBS` | พ.ร.บ.รถ | 16 แฟ้ม | FDH |
| `WK`   | กองทุนทดแทน | 16 แฟ้ม | WCF |
| `MON`  | พระภิกษุ | 16 แฟ้ม | FDH |
| `PRS`  | ราชทัณฑ์ | 16 แฟ้ม | DOC |

---

## 7. Format 16 แฟ้ม

> Date ทุก field → `YYYYMMDD` (ค.ศ.) เท่านั้น
> `UUC = "1"` ทุก record เสมอ
> `SEQ` unique ต่อ OPD visit | `AN` unique ต่อ IPD ≤9 หลัก

| # | แฟ้ม | เนื้อหา | UC | CSMBS | SSO |
|---|------|---------|----|----|-----|
| 1 | INS | สิทธิผู้ป่วย | ✓ | ✓ | ✓ |
| 2 | PAT | ข้อมูลประชากร | ✓ | ✓ | ✓ |
| 3 | OPD | ผู้ป่วยนอก | ✓ | ✓ | ✓ |
| 4 | ORF | ส่งต่อ OPD | ถ้ามี | ถ้ามี | ถ้ามี |
| 5 | ODX | วินิจฉัย OPD (ICD-10) | ✓ | ✓ | ✓ |
| 6 | OOP | หัตถการ OPD (ICD-9CM) | ถ้ามี | ถ้ามี | ถ้ามี |
| 7 | IPD | ผู้ป่วยใน | ✓ | ✓ | ✓ |
| 8 | IRF | ส่งต่อ IPD | ถ้ามี | ถ้ามี | ถ้ามี |
| 9 | IDX | วินิจฉัย IPD (ICD-10) | ✓ | ✓ | ✓ |
| 10 | IOP | หัตถการ IPD (ICD-9CM) | ถ้ามี | ถ้ามี | ถ้ามี |
| 11 | CHT | ค่าใช้จ่ายรวม | ✓ | ✓ | ✓ |
| 12 | CHA | ค่าใช้จ่ายแยกหมวด 01–16 | ✗ | **บังคับ** | ✗ |
| 13 | AER | อุบัติเหตุ/ฉุกเฉิน | ถ้ามี | ถ้ามี | ถ้ามี |
| 14 | ADP | ค่าใช้จ่ายอื่น / Project code | ถ้ามี | ✓ | ถ้ามี |
| 15 | LVD | Leave day | ถ้ามี | ถ้ามี | ถ้ามี |
| 16 | DRU | ยา (TMT 24 หลัก) | ✓ | ✓ | ✓ |

---

## 8. Validation Rules

```go
// Date: YYYYMMDD ค.ศ.
func IsValidDate(s string) bool {
    _, err := time.Parse("20060102", s)
    return err == nil
}

// PERSON_ID: 13 หลัก + check digit (ยกเว้น WP1/WP2/NON)
func IsValidPersonID(id string) bool {
    if len(id) != 13 { return false }
    return verifyThaiIDCheckDigit(id)
}
// AN: ≤9 หลัก, ห้าม \ / : * ? " < > | (AIPN ใช้ = แทน)
// UUC: "1" เสมอ
// ICD-10: validate กับ data/icd10.json
// ICD-9CM: validate กับ data/icd9cm.json
// TMT: 24 หลัก
```

**กฎเฉพาะสิทธิ:**
```
CSMBS/LGO/OFC OPD  → PERMITNO บังคับ
CSMBS/LGO/OFC IPD  → CHA (CHRGITEM 01–16) บังคับ + DRGCODE บังคับ
SSO ทุก record      → DRDX/DROPID บังคับ (ODX/OOP/IDX/IOP)
UCEP ทุกสิทธิ       → AER บังคับ + PERMITNO + CAUSE ระบุ
TPBS               → AER บังคับ, CAUSE="1", วงเงิน 30,000 บาท
WP1/WP2/NON        → PERSON_ID validate format แยก
```

### C-code ที่พบบ่อย
| Code | ความหมาย | วิธีแก้ |
|------|---------|--------|
| C101 | ICD-10 ผิด | ตรวจ master list |
| C102 | วันเกิดผิด | แก้ DOB |
| C103 | เพศไม่ตรง | แก้ SEX |
| C104 | PERSON_ID ผิด | ตรวจ 13 หลัก + check digit |
| C115 | UUC ≠ 1 | ตั้ง UUC="1" |
| C125 | ไม่มี PERMITNO | เพิ่ม PERMITNO |
| C126 | LOS สั้นเกิน | ตรวจ DATEADM/DATEDSC |

---

## 9. Integration Architecture (HIS ↔ NexClaim ↔ Fund Agencies)

```
┌──────────────┐   OPD 2-Way API        ┌───────────────┐   CIPN/CSOP/16-files   ┌────────┐
│  HIS (รพ.)   │ ───── POST visits ───▶│               │ ────────────────────▶│  FDH   │
│              │ ◀──── GET visit/{vn} ─│   NexClaim    │                       └────────┘
│              │                        │   Pipeline     │   AIPN/SSOP            ┌────────┐
│              │   IPD Share Folder     │  (routing →    │ ────────────────────▶│cs8.chi │
│              │ ── export CSV + ─────▶│   generate →   │                       └────────┘
│              │    MANIFEST.json       │   zip+md5 →    │
└──────────────┘                        │   submit)      │
                                        └───────────────┘
```

### 9.1 OPD — 2-Way API

| # | ทิศทาง | Endpoint | Purpose |
|---|--------|----------|---------|
| 1 | HIS → NexClaim | `POST /api/v1/his/opd/visits` | HIS push visit summary → รับ `batchId` |
| 2 | NexClaim → HIS | `GET /api/nexclaim/opd/visit/{vn}` | ดึง detail (patient+dx+dr+charge+ref+acc) |
| 3 | NexClaim → HIS | `GET /api/nexclaim/opd/visits?vn=...` | batch ≤50 |
| 4 | NexClaim → HIS | `GET /api/nexclaim/drug/list?page&limit` | TMT mapping (drug master) |
| 5 | NexClaim → HIS | `GET /api/nexclaim/health` | liveness |

Auth: Bearer/API-Key (ENV `HIS_API_BASE_URL` + `HIS_API_TOKEN`).

### 9.2 IPD — Shared Folder (CSV)

```
/shared/nexclaim/ipd/
├── incoming/{export_id}/            ← HIS export CSV 10 ไฟล์ + MANIFEST.json (last)
├── processed/{export_id}/           ← NexClaim archive หลังสำเร็จ
└── error/{export_id}/               ← ถ้า parse/pipeline fail + ERROR.txt
```

ไฟล์ CSV (pipe-delimited `|`, UTF-8, header row, empty = null):

| File | บังคับ | เนื้อหา |
|------|-------|---------|
| PAT.csv | ✓ | pid, prefix, first/last name, dob, sex, marriage, nation, changwat, amphur, address |
| IPD.csv | ✓ | an, pid, hn, inscl, permit_no, agency_code, date_adm/dsc, time_adm/dsc, ward, los, dischs, discht, drg_code, adj_rw, doctor_code, uuc |
| IDX.csv | ✓ | an, icd10, dx_type, doctor_code |
| IOP.csv | ถ้ามี | an, icd9cm, op_date, op_time, doctor_code, charge |
| DRU.csv | ✓ | an, his_item_id, his_item_name, tmt_tp, tmt24, quantity, unit, unit_price, total_price, drug_date_start/end, usage, doctor_code |
| CHT.csv | ✓ | an, total_charge, total_claim, total_copay |
| CHA.csv | บังคับ CSMBS/LGO/OFC | an, chrgitem (01-16), amount |
| AER.csv | ถ้ามี | an, ae_date, ae_time, ae_type, cause, place |
| IRF.csv | ถ้ามี | an, refer_from, refer_to, refer_date, refer_cause |
| LVD.csv | ถ้ามี | an, leave_date, leave_days |

MANIFEST.json = signal ตัวสุดท้าย: `{export_id, export_date, period, hospital_code, total_admissions, files, exported_by}`.

### 9.3 Fund Agencies (outbound)

```
FDH (MOPH Financial Data Hub)
  POST /api/auth/token               → Bearer JWT (1 ชั่วโมง)
  POST /api/claim/16files            → UC 16 แฟ้ม (multipart ZIP)
  POST /api/claim/cipn               → IPD CSMBS/LGO/OFC
  POST /api/claim/csop               → OPD CSMBS/LGO/OFC
  GET  /api/claim/status/{txnId}     → สถานะ
  GET  /api/claim/rep/{YYYYMM}       → REP (C-code)

CHI (cs8.chi.or.th) — SSO Basic auth
  POST /ssopupload/                  → SSOP (OPD ม.33/39/40)
  POST /aipnupload/                  → AIPN (IPD ม.33/39/40)
```

ส่งใน 24 ชม. → สปสช.จ่ายใน 72 ชม. | ช้ากว่า → OPD 15 วัน, IPD 30 วัน

---

## 9A. NexClaim HTTP API (gin)

```
# Core
GET  /healthz
POST /api/submit                                    ← pipeline.Run ตรง (dev/admin)
GET  /api/status/:txnId                             ← forward ไป FDH

# HIS — OPD 2-Way
POST /api/v1/his/opd/visits                         ← HIS push visit list → batchId
GET  /api/v1/his/opd/batches                        ← list (filter: hcode, period, status)
GET  /api/v1/his/opd/batches/:batchId               ← status ของ batch
POST /api/v1/his/opd/batches/:batchId/process       ← fetch detail → pipeline
  ?dry_run=true                                       (skip FDH/CHI submit)

# HIS — IPD Share Folder
GET  /api/v1/his/ipd/imports                        ← list folders ใน incoming/
POST /api/v1/his/ipd/imports/:exportId              ← parse → pipeline per INSCL
  ?dry_run=true                                       + move ไป processed/ หรือ error/

# Submissions (claim_batch — what we sent to FDH/CHI)
GET  /api/v1/claim/batches                          ← filter: hcode, period, inscl, format, status
GET  /api/v1/claim/batches/:batchId                 ← detail + c_code counts

# C-code feedback (from FDH REP)
GET  /api/v1/ccodes                                 ← filter: batch_id, hcode, period, resolved, c_code
POST /api/v1/ccodes/:id/resolve                     ← { by } → mark resolved
POST /api/v1/rep/fetch                              ← { hcode, period } → download REP, parse, upsert

# Master data — CRUD
GET/POST/PATCH/DELETE  /api/v1/admin/hospitals[/:hcode]
GET/POST/PATCH/DELETE  /api/v1/admin/doctors[/:code]
GET/POST/PATCH/DELETE  /api/v1/admin/inscl-maps[/:id]
GET/POST/PATCH/DELETE  /api/v1/admin/drug-maps[/:id]
GET/POST/PATCH/DELETE  /api/v1/admin/doctor-maps[/:id]
GET/POST/PATCH/DELETE  /api/v1/admin/icd-maps[/:id]
GET/POST/PATCH/DELETE  /api/v1/admin/field-maps[/:id]

# Bulk import (CSV) — per-row error isolation → BulkResult{total, imported, errors[]}
POST /api/v1/admin/bulk/icd10      ← code, name_th, name_en
POST /api/v1/admin/bulk/icd9cm     ← code, name_th, name_en
POST /api/v1/admin/bulk/tmt        ← tmt24 (24 chars), name_th, name_en, strength, form
POST /api/v1/admin/bulk/doctors    ← code, name_th, license_no, specialty
```

Pipeline ทุกรอบทำ: **extract → validate (inc. MasterValidator from DB) → bucket by INSCL → generate (16-file/CIPN/CSOP/AIPN/SSOP) → zip+md5 → submit → SaveRun (claim_batch+claim_record)**. REP fetch แล้ว `ParseREP` → upsert `c_code_log` (fallback: attach to latest batch ใน period ถ้า match ไม่เจอ).

---

## 10. Date Utility (`internal/util/date.go`)

```go
// ToAD แปลง พ.ศ. → ค.ศ. (HIS ไทยมักเก็บ พ.ศ.)
func ToAD(s string) string {
    if len(s) < 4 { return s }
    year, _ := strconv.Atoi(s[:4])
    if year > 2400 {
        return strconv.Itoa(year-543) + s[4:]
    }
    return s
}

func FormatDate(t time.Time) string     { return t.Format("20060102") }
func FormatDateTime(t time.Time) string { return t.Format("20060102150405") }
```

---

## 11. Coding Conventions

- ทุก package มี `_test.go` — ห้าม merge โดยไม่มี test (tests อยู่ที่ `backend/tests/`)
- error wrapping: `fmt.Errorf("context: %w", err)` ทุกที่
- log ทุก claim: `{ txnId, hcode, inscl, format, period, status }`
- XML struct field ตั้งชื่อตาม spec (**UPPERCASE**): `AN`, `DATEADM`, `DRDX`
- แต่ละ format อยู่ใน package แยก — ห้าม CIPN logic รั่วเข้า CSOP
- Date ทุก field ต้องผ่าน `util.ToAD()` หรือ `util.ParseHISDate()` ก่อนใส่ struct
- Default ("" → "1" สำหรับ UUC) ใช้ `util.StrOr` — ห้าม copy `orDefault` helper ในแต่ละ package
- ห้าม send โดยไม่ผ่าน validator ก่อน (pipeline block submit ถ้ามี validation errors + ไม่ใช่ dry-run)
- **Repo pattern**: ทุก DB access ผ่าน interface ใน `store/` — handler รับ interface, `cmd/server.go` wire Pg impl. ทำให้ handler tests ใช้ in-memory fake ได้ (ดู `memClaimBatchRepo` ใน `tests/submissions_test.go`).
- **ห้าม TRUNCATE ใน tests** — RDS role `nexclaim` มีแค่ CRUD; ใช้ `DELETE FROM` ไล่จาก child → parent (c_code_log → claim_record → claim_batch).
- **Integration tests** skip ถ้า `POSTGRES_TEST_DSN` ว่าง (เพื่อให้ `go test ./...` บน dev laptop ผ่านโดยไม่ต้อง DB).
- `go build ./...` และ `go test ./...` ต้องผ่านก่อน commit (รันจาก `backend/`)

---

## 12. Environment Variables (`backend/.env`)

```env
# Database (NexClaim Postgres — ถ้ามีค่า server เปิดใช้ store/ + master CRUD + submissions + c-codes)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=nexclaim
DB_USER=nexclaim
DB_PASS=
# เช่น: postgres://nexclaim:...@<rds-host>:5432/nexclaim?sslmode=require
POSTGRES_TEST_DSN=        # tests/*_pg tests skip ถ้าว่าง

# HIS API (OPD 2-Way) — ถ้ามีค่า server เปิดใช้ /api/v1/his/opd/batches/:id/process
HIS_API_BASE_URL=
HIS_API_TOKEN=            # Bearer token (หรือใช้ hisclient.WithAPIKey แทน)

# IPD share folder — ถ้ามีค่า server เปิดใช้ /api/v1/his/ipd/* + auto-watcher
IPD_SHARE_ROOT=           # เช่น /shared/nexclaim/ipd (ต้องมี incoming/ processed/ error/)
IPD_WATCH_INTERVAL=30s    # polling interval (0 = ปิด watcher, ยังใช้ HTTP endpoint ได้)

# FDH (ส่ง 16 แฟ้ม/CIPN/CSOP)
FDH_BASE_URL=https://fdh.moph.go.th
FDH_USERNAME=
FDH_PASSWORD=
FDH_HCODE=XXXXX

# SSO / CHI (ส่ง AIPN/SSOP)
CHI_BASE_URL=https://cs8.chi.or.th
CHI_USERNAME=
CHI_PASSWORD=

# Hospital
HOSPITAL_NAME=
HOSPITAL_HCODE=
HOSPITAL_CHANGWAT=

# App
PORT=8080
ENV=production
LOG_LEVEL=info
```

---

## 13. Quick Start

```bash
git clone https://github.com/monkeytech2017/nexclaim   # repo = monorepo (backend + frontend)
cd nexclaim

# ── Database (ครั้งแรกเท่านั้น) ──
# สร้าง DB nexclaim พร้อม th_TH.UTF-8 collation + apply migrations 000..004
cd backend
PGHOST=localhost PGUSER=postgres ./scripts/init_db.sh

# สร้าง role สำหรับ app (idempotent — CRUD เท่านั้น, ไม่มี TRUNCATE)
PGHOST=<rds> PGUSER=<admin> PGPASSWORD=<pw> ./scripts/create_user.sh nexclaim

# ── Backend ──
cp ../.env.example .env     # แก้ credentials
go mod tidy
go build -o nexclaim .
go test ./...               # ต้องผ่านก่อน commit — ปัจจุบัน 111/111 (Pg integration skip ถ้าไม่ตั้ง DSN)
POSTGRES_TEST_DSN="postgres://nexclaim:...@<rds>/nexclaim?sslmode=require" go test ./tests/... # รวม Pg contract

# Migrations + seed
./nexclaim migrate status   # list migrations ที่ apply แล้ว / pending
./nexclaim seed master      # seed m_inscl / m_agency / m_chrgitem / m_hospital(dev) จาก data/*.json

# CLI
./nexclaim submit --inscl UCS --period 202504 --dry-run --hcode 12345
./nexclaim submit --inscl 011 --period 202504 --dry-run --hcode 12345
./nexclaim status --txn-id <id>

# HTTP server (ใช้งานจริงจาก HIS หรือ frontend — เปิด watcher + master CRUD + REP feedback)
./nexclaim server --addr :8080
curl http://localhost:8080/healthz

# Bulk import master (หลัง server รัน) — ICD/TMT/doctor CSV
./scripts/seed_master.sh data/icd10.csv data/icd9cm.csv data/tmt.csv

# ── Frontend (Next.js 14) ──
cd ../frontend
npm install
npm run dev
```

---

## 13A. Agent Team (`.claude/agents/`)

Three specialized subagents cover the natural seams in this codebase. Delegate via the Agent tool — each is self-contained and briefed from its own file, so there's no need to re-explain conventions.

| Agent | When to use | Scope |
|-------|-------------|-------|
| `nexclaim-backend`   | Anything under `backend/` — Go code, migrations, handlers, tests | Repo pattern, Pg conventions, pipeline flow, XML structs |
| `nexclaim-frontend`  | Anything under `frontend/` — pages, components, `lib/api.ts`, Tailwind | App Router, React Query, BulkImportModal, Suspense rule |
| `nexclaim-domain`    | Domain questions BEFORE implementation — INSCL routing, format selection, C-codes, CHRGITEM, spec xlsx | Read-only advisor, does not edit code |

**Workflow pattern.** For cross-layer work, ask `nexclaim-domain` first (if there's any spec judgment), then run `nexclaim-backend` and `nexclaim-frontend` in **parallel** when their work is independent (e.g. a new endpoint + its UI page can be done at the same time — the frontend agent stubs against the typed API contract while the backend agent implements it). Use a single Agent tool message with multiple blocks for parallel spawn.

**What the main agent still owns.** Cross-cutting decisions (adding a new slice to backlog, editing `CLAUDE.md`, coordinating PRs, confirming risky actions). Never delegate *understanding* — if a fix needs thought, do it yourself, don't shop it around.

---

## 14. Frontend (Next.js 14 App Router)

Sidebar แบ่ง 2 กลุ่ม:

**การส่งเบิก** (workflow) — Dashboard · OPD Batches · IPD Imports · Claims (submit) · History (ingest) · Submissions (outbound claim_batch) · C-codes (REP feedback)

**Master Data** (admin CRUD) — Hospitals · Doctors · INSCL Maps · Drug Maps (→TMT24) · Doctor Maps (→DRDX) · ICD Maps · Field Maps (HIS column → target spec)

- `lib/api.ts` — typed client (ทุก endpoint); every hook ใช้ React Query → invalidate keys หลัง mutation
- Admin pages share `BulkImportModal` (CSV auto-detect delimiter) + per-row error display (`BulkResult.errors`)
- `SubmissionOutcome` — inline banner หลัง `fetchRep` (fetched/inserted/skipped/errors)
- `/c-codes?batch=<id>` deep-links จาก Submissions table → filter C-code ตาม batch

---

## 15. Remaining Backlog

1. **Real HIS API end-to-end** — ทดสอบ live `HIS_API_BASE_URL` + batch flow (spec xlsx มี endpoint แล้ว, client code พร้อม, ต้อง staging env จาก HIS team)
2. **CHI REP parser** — mirror FDH REP ingest สำหรับ SSO (AIPN/SSOP)
3. **Dashboard enrichment** — Recharts cards จาก `claim_batch` + `c_code_log` (รายสิทธิ, pass/fail, trend)
4. **send_log audit trail** — insert ทุกครั้งที่ sender ส่ง (success/fail) + UI view
5. **Auth middleware** — JWT หรือ API-key per hospital (ตอนนี้ server open; ต้อง gate ก่อน production)

---

## 16. References

| เอกสาร | URL |
|--------|-----|
| FDH Portal | https://fdh.moph.go.th/hospital/ |
| FDH คู่มือ | https://www.sshos.go.th/financial-data-hub/ |
| AIPN upload | https://cs8.chi.or.th/aipnupload/ |
| SSOP/CHI | https://cs8.chi.or.th |
| e-Claim สปสช. | https://eclaim.nhso.go.th |
| CGD สวัสดิการ | https://www.cgd.go.th |
| ตรวจสอบสิทธิ CGD | https://mbdb.cgd.go.th/wel |
| ตรวจสอบสิทธิ อปท. | https://www.nhso.go.th/lgo/ |
| ICD-10 WHO | https://icd.who.int/browse10/ |
| TMT Drug | https://tmt.this.or.th |
