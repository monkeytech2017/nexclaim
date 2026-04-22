package file16

import (
	"fmt"
	"strings"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/util"
)

// Pipe-delimited 16-file records. Column ordering follows สปสช. master spec
// (eClaim / FDH). Any blank field is emitted as empty between pipes.
// All dates must be YYYYMMDD in ค.ศ.; UUC is always "1".

const delim = "|"

func join(fields ...string) string { return strings.Join(fields, delim) }

func f2(v float64) string { return fmt.Sprintf("%.2f", v) }

// ── 1. INS — สิทธิผู้ป่วย ─────────────────────────────────────────
type INSRecord struct {
	HospMain   string // รหัสสถานพยาบาลหลัก (HCODE 5 หลัก)
	HospSub    string // รหัสสถานพยาบาลรอง
	PID        string // PERSON_ID (13 หลัก)
	INSCL      string
	SubType    string // ประเภทย่อยสิทธิ
	DateIn     string // YYYYMMDD
	DateExp    string // YYYYMMDD หรือว่างถ้าไม่หมดอายุ
	HN         string
	DateServ   string // YYYYMMDD
}

func (r INSRecord) ToLine() string {
	return join(r.HospMain, r.HospSub, r.PID, r.INSCL, r.SubType,
		r.DateIn, r.DateExp, r.HN, r.DateServ)
}

// ── 2. PAT — ข้อมูลประชากร ─────────────────────────────────────
type PATRecord struct {
	HospMain string
	PID      string
	HN       string
	PreName  string
	Name     string
	DOB      string
	Sex      string
	Marriage string
	Occupa   string
	Nation   string
	PersonID string
	Changwat string
	Amphur   string
	DateServ string
}

func (r PATRecord) ToLine() string {
	return join(r.HospMain, r.PID, r.HN, r.PreName, r.Name, r.DOB, r.Sex,
		r.Marriage, r.Occupa, r.Nation, r.PersonID, r.Changwat, r.Amphur, r.DateServ)
}

// ── 3. OPD — ผู้ป่วยนอก ─────────────────────────────────────────
type OPDRecord struct {
	HN       string
	Clinic   string
	DateOPD  string
	TimeOPD  string // HHMM
	SEQ      string
	UUC      string // "1"
	TypeIn   string
	TypeOut  string
	ServPrice string
	ProjCode string
	DateServ string
}

func (r OPDRecord) ToLine() string {
	return join(r.HN, r.Clinic, r.DateOPD, r.TimeOPD, r.SEQ, r.UUC,
		r.TypeIn, r.TypeOut, r.ServPrice, r.ProjCode, r.DateServ)
}

// ── 4. ORF — ส่งต่อ OPD ───────────────────────────────────────
type ORFRecord struct {
	HN       string
	SEQ      string
	ReferIn  string // hcode ที่ส่งมา
	ReferOut string // hcode ที่ส่งไป
	Reason   string
	DateServ string
}

func (r ORFRecord) ToLine() string {
	return join(r.HN, r.SEQ, r.ReferIn, r.ReferOut, r.Reason, r.DateServ)
}

// ── 5. ODX — วินิจฉัย OPD ──────────────────────────────────────
type ODXRecord struct {
	HN       string
	SEQ      string
	Code     string // ICD-10
	DxType   string // "1"..."5"
	DoctorID string // DRDX (บังคับ SSO)
	DateServ string
}

func (r ODXRecord) ToLine() string {
	return join(r.HN, r.SEQ, r.Code, r.DxType, r.DoctorID, r.DateServ)
}

// ── 6. OOP — หัตถการ OPD ───────────────────────────────────────
type OOPRecord struct {
	HN       string
	SEQ      string
	Code     string // ICD-9CM
	DoctorID string
	Date     string
	DateServ string
}

func (r OOPRecord) ToLine() string {
	return join(r.HN, r.SEQ, r.Code, r.DoctorID, r.Date, r.DateServ)
}

// ── 7. IPD — ผู้ป่วยใน ─────────────────────────────────────────
type IPDRecord struct {
	HN       string
	AN       string
	DateAdm  string
	TimeAdm  string
	DateDsc  string
	TimeDsc  string
	Dischs   string
	Discht   string
	UUC      string
	AdmType  string
	DRGCode  string
	RW       string
	LOS      string
	DateServ string
}

func (r IPDRecord) ToLine() string {
	return join(r.HN, r.AN, r.DateAdm, r.TimeAdm, r.DateDsc, r.TimeDsc,
		r.Dischs, r.Discht, r.UUC, r.AdmType, r.DRGCode, r.RW, r.LOS, r.DateServ)
}

// ── 8. IRF — ส่งต่อ IPD ────────────────────────────────────────
type IRFRecord struct {
	HN       string
	AN       string
	ReferIn  string
	ReferOut string
	Reason   string
	DateServ string
}

func (r IRFRecord) ToLine() string {
	return join(r.HN, r.AN, r.ReferIn, r.ReferOut, r.Reason, r.DateServ)
}

// ── 9. IDX — วินิจฉัย IPD ──────────────────────────────────────
type IDXRecord struct {
	HN       string
	AN       string
	Code     string
	DxType   string
	DoctorID string
	DateServ string
}

func (r IDXRecord) ToLine() string {
	return join(r.HN, r.AN, r.Code, r.DxType, r.DoctorID, r.DateServ)
}

// ── 10. IOP — หัตถการ IPD ──────────────────────────────────────
type IOPRecord struct {
	HN       string
	AN       string
	Code     string
	DoctorID string
	Date     string
	DateServ string
}

func (r IOPRecord) ToLine() string {
	return join(r.HN, r.AN, r.Code, r.DoctorID, r.Date, r.DateServ)
}

// ── 11. CHT — ค่าใช้จ่ายรวม ────────────────────────────────────
type CHTRecord struct {
	HN       string
	SEQOrAN  string // SEQ for OPD, AN for IPD (spec uses separate position;
	// here we emit it as one column — map to SEQ or AN at build time)
	Total    float64
	Paid     float64
	DateServ string
}

func (r CHTRecord) ToLine() string {
	return join(r.HN, r.SEQOrAN, f2(r.Total), f2(r.Paid), r.DateServ)
}

// ── 12. CHA — ค่าใช้จ่ายแยกหมวด 01–16 (บังคับ CSMBS/LGO) ───────
type CHARecord struct {
	HN       string
	SEQOrAN  string
	ChrgItem string // "01"–"16"
	Amount   float64
	DateServ string
}

func (r CHARecord) ToLine() string {
	return join(r.HN, r.SEQOrAN, r.ChrgItem, f2(r.Amount), r.DateServ)
}

// ── 13. AER — อุบัติเหตุ/ฉุกเฉิน / UCEP ─────────────────────────
type AERRecord struct {
	HN       string
	SEQ      string
	DateAE   string
	TimeAE   string
	Cause    string // "1"=auto "2"=work ...
	PermitNo string
	DateServ string
}

func (r AERRecord) ToLine() string {
	return join(r.HN, r.SEQ, r.DateAE, r.TimeAE, r.Cause, r.PermitNo, r.DateServ)
}

// ── 14. ADP — ค่าใช้จ่ายอื่น / Project code ────────────────────
type ADPRecord struct {
	HN       string
	SEQOrAN  string
	ProjCode string
	Amount   float64
	DateServ string
}

func (r ADPRecord) ToLine() string {
	return join(r.HN, r.SEQOrAN, r.ProjCode, f2(r.Amount), r.DateServ)
}

// ── 15. LVD — Leave day ───────────────────────────────────────
type LVDRecord struct {
	HN       string
	AN       string
	DateLeave string
	DateBack string
	DateServ string
}

func (r LVDRecord) ToLine() string {
	return join(r.HN, r.AN, r.DateLeave, r.DateBack, r.DateServ)
}

// ── 16. DRU — ยา (TMT 24 หลัก) ─────────────────────────────────
type DRURecord struct {
	HN       string
	SEQOrAN  string
	TMTID    string
	Amount   float64 // ปริมาณ
	Unit     string
	Price    float64 // ราคาขาย
	Cost     float64 // ต้นทุน
	DateServ string
}

func (r DRURecord) ToLine() string {
	return join(r.HN, r.SEQOrAN, r.TMTID, f2(r.Amount), r.Unit,
		f2(r.Price), f2(r.Cost), r.DateServ)
}

// ── Helper conversions ────────────────────────────────────────

// patientDateServ returns the canonical DATE_SERV (YYYYMMDD ค.ศ.)
// used across every 16-file row for a given OPD/IPD visit.
func opdDateServ(v model.OPDVisit) string {
	return util.FormatDate(v.DateOPD)
}

func ipdDateServ(a model.IPDAdmit) string {
	// ใช้ DATEADM เป็น DATE_SERV ของทุก row ใน IPD
	return util.FormatDate(a.DateAdm)
}
