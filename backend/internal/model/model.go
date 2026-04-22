// Package model กำหนด domain types ทั้งหมดของ NexClaim
package model

import "time"

// ─── สิทธิ & Routing ────────────────────────────────────────────

type INSCL string

const (
	INSCL_CSMBS INSCL = "011"
	INSCL_WEL   INSCL = "WEL"
	INSCL_LGO   INSCL = "LGO"
	INSCL_OFC   INSCL = "OFC"
	INSCL_UCS   INSCL = "UCS"
	INSCL_NON   INSCL = "NON"
	INSCL_WP1   INSCL = "WP1"
	INSCL_WP2   INSCL = "WP2"
	INSCL_SSS   INSCL = "SSS"
	INSCL_SS4   INSCL = "SS4"
	INSCL_TPBS  INSCL = "TPBS"
	INSCL_WK    INSCL = "WK"
	INSCL_MON   INSCL = "MON"
	INSCL_PRS   INSCL = "PRS"
)

type Agency string

const (
	AgencyCGD  Agency = "CGD"
	AgencyLGO  Agency = "LGO"
	AgencyNBTC Agency = "NBTC"
	AgencyBAAC Agency = "BAAC"
	AgencyECT  Agency = "ECT"
	AgencyPEA  Agency = "PEA"
	AgencyMEA  Agency = "MEA"
	AgencyMWA  Agency = "MWA"
	AgencySRT  Agency = "SRT"
)

type ClaimFormat string

const (
	Format16Files ClaimFormat = "16FILES"
	FormatCIPN    ClaimFormat = "CIPN"
	FormatCSOP    ClaimFormat = "CSOP"
	FormatAIPN    ClaimFormat = "AIPN"
	FormatSSOP    ClaimFormat = "SSOP"
)

type Sender string

const (
	SenderFDH Sender = "FDH"
	SenderCHI Sender = "CHI"
	SenderWCF Sender = "WCF"
	SenderDOC Sender = "DOC"
)

type RouteResult struct {
	INSCL   INSCL
	IsIPD   bool
	Format  ClaimFormat
	Agency  Agency
	Sender  Sender
}

// ─── Patient & Visit ────────────────────────────────────────────

type Patient struct {
	HN         string
	PersonID   string
	INSCL      INSCL
	PTTYPE     string
	AgencyCode string // สำหรับ OFC
	Name       string
	DOB        time.Time
	Sex        string // "1"=ชาย "2"=หญิง
	Nation     string // "099"=ไทย
	Changwat   string
	Amphur     string
	PermitNo   string
}

type Diagnosis struct {
	Code     string // ICD-10
	Type     string // "1"=Primary "2"=Comorbidity "3"=Complication "4"=Other "5"=External
	DoctorID string // เลขใบประกอบวิชาชีพ (บังคับ SSO)
}

type Operation struct {
	Code     string // ICD-9CM
	DoctorID string
	Date     time.Time
}

type Drug struct {
	TMTID  string  // TMT 24 หลัก
	Amount float64
	Price  float64
	Cost   float64
	Unit   string
}

// CHRGITEM หมวดค่าบริการ 01–16 (บังคับ CSMBS/LGO CHA)
type ChargeItem struct {
	Code   string  // "01"–"16"
	Amount float64
}

type DRGInfo struct {
	Code    string
	Version string
	RW      float64
	AdjRW   float64
	LostDay int
}

type OPDVisit struct {
	Patient   Patient
	SEQ       string
	DateOPD   time.Time
	TimeOPD   string // HHMM
	Clinic    string
	TypeOut   string // "1"=กลับบ้าน "2"=รับไว้ "9"=อื่น
	UUC       string // = "1" เสมอ
	Diagnoses []Diagnosis
	Ops       []Operation
	Drugs     []Drug
	Charges   []ChargeItem
	Total     float64
	Paid      float64
	IsUCEP    bool
}

type IPDAdmit struct {
	Patient   Patient
	AN        string // unique ≤9 หลัก
	DateAdm   time.Time
	TimeAdm   string
	DateDsc   time.Time
	TimeDsc   string
	WardDsc   string
	LOS       int
	Dischs    string // "1"–"5"
	Discht    string // "1"–"3"
	AdmDx     string // ICD-10 แรกรับ
	UUC       string // = "1" เสมอ
	Diagnoses []Diagnosis
	Ops       []Operation
	Drugs     []Drug
	Charges   []ChargeItem
	Total     float64
	Paid      float64
	DRG       *DRGInfo
	IsUCEP    bool
}
