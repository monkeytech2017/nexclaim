// Package file16 สร้างชุด 16 แฟ้ม (สปสช.) จาก OPD visits + IPD admits
//
// Output: map[filename][]byte โดย filename เป็นตัวพิมพ์ใหญ่เช่น "INS.txt", "PAT.txt", ...
// ทุกไฟล์ใช้ pipe-delimited, one record per line, ไม่มี header row.
// วันที่ทุก field = YYYYMMDD ค.ศ. และ UUC = "1" เสมอ.
package file16

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/util"
)

// Input bundles everything needed to render 16 files for one submission period.
type Input struct {
	HCode    string // รหัส รพ. 5 หลัก
	HCodeSub string // รหัสสถานพยาบาลย่อย (ปกติ = HCode)
	Period   string // YYYYMM (metadata; not embedded in records)
	OPD      []model.OPDVisit
	IPD      []model.IPDAdmit
}

// Bundle is the rendered result.
// Files keyed by canonical filename ("INS.txt" ...) → raw bytes.
// Empty files are omitted from the map.
type Bundle struct {
	Files map[string][]byte
}

// Filenames เรียงตามลำดับ 1..16 ตาม spec
var Filenames = []string{
	"INS.txt", "PAT.txt", "OPD.txt", "ORF.txt", "ODX.txt", "OOP.txt",
	"IPD.txt", "IRF.txt", "IDX.txt", "IOP.txt",
	"CHT.txt", "CHA.txt", "AER.txt", "ADP.txt", "LVD.txt", "DRU.txt",
}

// Build renders the 16-file bundle.
func Build(in Input) (Bundle, error) {
	if in.HCode == "" {
		return Bundle{}, fmt.Errorf("file16: HCode is required")
	}
	if in.HCodeSub == "" {
		in.HCodeSub = in.HCode
	}

	g := &builder{in: in}
	for _, v := range in.OPD {
		g.addOPD(v)
	}
	for _, a := range in.IPD {
		g.addIPD(a)
	}
	return g.render(), nil
}

type builder struct {
	in  Input
	ins []INSRecord
	pat []PATRecord
	opd []OPDRecord
	orf []ORFRecord
	odx []ODXRecord
	oop []OOPRecord
	ipd []IPDRecord
	irf []IRFRecord
	idx []IDXRecord
	iop []IOPRecord
	cht []CHTRecord
	cha []CHARecord
	aer []AERRecord
	adp []ADPRecord
	lvd []LVDRecord
	dru []DRURecord

	seenINS map[string]bool
	seenPAT map[string]bool
}

func (b *builder) addOPD(v model.OPDVisit) {
	dateServ := opdDateServ(v)
	b.addInsPat(v.Patient, dateServ)

	b.opd = append(b.opd, OPDRecord{
		HN: v.Patient.HN, Clinic: v.Clinic,
		DateOPD: dateServ, TimeOPD: v.TimeOPD,
		SEQ: v.SEQ, UUC: must1(v.UUC),
		TypeIn: "", TypeOut: v.TypeOut,
		ServPrice: f2(v.Total), ProjCode: "",
		DateServ: dateServ,
	})

	for _, dx := range v.Diagnoses {
		b.odx = append(b.odx, ODXRecord{
			HN: v.Patient.HN, SEQ: v.SEQ,
			Code: dx.Code, DxType: dx.Type, DoctorID: dx.DoctorID,
			DateServ: dateServ,
		})
	}
	for _, op := range v.Ops {
		b.oop = append(b.oop, OOPRecord{
			HN: v.Patient.HN, SEQ: v.SEQ,
			Code: op.Code, DoctorID: op.DoctorID,
			Date: util.FormatDate(op.Date), DateServ: dateServ,
		})
	}

	b.cht = append(b.cht, CHTRecord{
		HN: v.Patient.HN, SEQOrAN: v.SEQ,
		Total: v.Total, Paid: v.Paid, DateServ: dateServ,
	})
	for _, ch := range v.Charges {
		b.cha = append(b.cha, CHARecord{
			HN: v.Patient.HN, SEQOrAN: v.SEQ,
			ChrgItem: ch.Code, Amount: ch.Amount, DateServ: dateServ,
		})
	}
	for _, d := range v.Drugs {
		b.dru = append(b.dru, DRURecord{
			HN: v.Patient.HN, SEQOrAN: v.SEQ,
			TMTID: d.TMTID, Amount: d.Amount, Unit: d.Unit,
			Price: d.Price, Cost: d.Cost, DateServ: dateServ,
		})
	}
	if v.IsUCEP {
		b.aer = append(b.aer, AERRecord{
			HN: v.Patient.HN, SEQ: v.SEQ,
			DateAE: dateServ, TimeAE: v.TimeOPD,
			Cause: "1", PermitNo: v.Patient.PermitNo, DateServ: dateServ,
		})
	}
}

func (b *builder) addIPD(a model.IPDAdmit) {
	dateServ := ipdDateServ(a)
	b.addInsPat(a.Patient, dateServ)

	var drgCode, rw string
	if a.DRG != nil {
		drgCode = a.DRG.Code
		if a.DRG.AdjRW > 0 {
			rw = f2(a.DRG.AdjRW)
		} else {
			rw = f2(a.DRG.RW)
		}
	}

	b.ipd = append(b.ipd, IPDRecord{
		HN: a.Patient.HN, AN: a.AN,
		DateAdm: util.FormatDate(a.DateAdm), TimeAdm: a.TimeAdm,
		DateDsc: util.FormatDate(a.DateDsc), TimeDsc: a.TimeDsc,
		Dischs: a.Dischs, Discht: a.Discht,
		UUC: must1(a.UUC), AdmType: "",
		DRGCode: drgCode, RW: rw,
		LOS: fmt.Sprintf("%d", a.LOS), DateServ: dateServ,
	})

	for _, dx := range a.Diagnoses {
		b.idx = append(b.idx, IDXRecord{
			HN: a.Patient.HN, AN: a.AN,
			Code: dx.Code, DxType: dx.Type, DoctorID: dx.DoctorID,
			DateServ: dateServ,
		})
	}
	for _, op := range a.Ops {
		b.iop = append(b.iop, IOPRecord{
			HN: a.Patient.HN, AN: a.AN,
			Code: op.Code, DoctorID: op.DoctorID,
			Date: util.FormatDate(op.Date), DateServ: dateServ,
		})
	}
	b.cht = append(b.cht, CHTRecord{
		HN: a.Patient.HN, SEQOrAN: a.AN,
		Total: a.Total, Paid: a.Paid, DateServ: dateServ,
	})
	for _, ch := range a.Charges {
		b.cha = append(b.cha, CHARecord{
			HN: a.Patient.HN, SEQOrAN: a.AN,
			ChrgItem: ch.Code, Amount: ch.Amount, DateServ: dateServ,
		})
	}
	for _, d := range a.Drugs {
		b.dru = append(b.dru, DRURecord{
			HN: a.Patient.HN, SEQOrAN: a.AN,
			TMTID: d.TMTID, Amount: d.Amount, Unit: d.Unit,
			Price: d.Price, Cost: d.Cost, DateServ: dateServ,
		})
	}
	if a.IsUCEP {
		b.aer = append(b.aer, AERRecord{
			HN: a.Patient.HN, SEQ: a.AN,
			DateAE: util.FormatDate(a.DateAdm), TimeAE: a.TimeAdm,
			Cause: "1", PermitNo: a.Patient.PermitNo, DateServ: dateServ,
		})
	}
}

func (b *builder) addInsPat(p model.Patient, dateServ string) {
	if b.seenINS == nil {
		b.seenINS = make(map[string]bool)
		b.seenPAT = make(map[string]bool)
	}
	insKey := p.HN + "|" + dateServ
	if !b.seenINS[insKey] {
		b.seenINS[insKey] = true
		b.ins = append(b.ins, INSRecord{
			HospMain: b.in.HCode, HospSub: b.in.HCodeSub,
			PID: p.PersonID, INSCL: string(p.INSCL), SubType: p.PTTYPE,
			DateIn: "", DateExp: "",
			HN: p.HN, DateServ: dateServ,
		})
	}
	if !b.seenPAT[p.HN] {
		b.seenPAT[p.HN] = true
		b.pat = append(b.pat, PATRecord{
			HospMain: b.in.HCode, PID: p.PersonID, HN: p.HN,
			PreName: "", Name: p.Name,
			DOB: util.FormatDate(p.DOB), Sex: p.Sex,
			Marriage: "", Occupa: "", Nation: p.Nation,
			PersonID: p.PersonID,
			Changwat: p.Changwat, Amphur: p.Amphur,
			DateServ: dateServ,
		})
	}
}

func (b *builder) render() Bundle {
	out := make(map[string][]byte, 16)
	appendFile(out, "INS.txt", linesOf(b.ins, func(r INSRecord) string { return r.ToLine() }))
	appendFile(out, "PAT.txt", linesOf(b.pat, func(r PATRecord) string { return r.ToLine() }))
	appendFile(out, "OPD.txt", linesOf(b.opd, func(r OPDRecord) string { return r.ToLine() }))
	appendFile(out, "ORF.txt", linesOf(b.orf, func(r ORFRecord) string { return r.ToLine() }))
	appendFile(out, "ODX.txt", linesOf(b.odx, func(r ODXRecord) string { return r.ToLine() }))
	appendFile(out, "OOP.txt", linesOf(b.oop, func(r OOPRecord) string { return r.ToLine() }))
	appendFile(out, "IPD.txt", linesOf(b.ipd, func(r IPDRecord) string { return r.ToLine() }))
	appendFile(out, "IRF.txt", linesOf(b.irf, func(r IRFRecord) string { return r.ToLine() }))
	appendFile(out, "IDX.txt", linesOf(b.idx, func(r IDXRecord) string { return r.ToLine() }))
	appendFile(out, "IOP.txt", linesOf(b.iop, func(r IOPRecord) string { return r.ToLine() }))
	appendFile(out, "CHT.txt", linesOf(b.cht, func(r CHTRecord) string { return r.ToLine() }))
	appendFile(out, "CHA.txt", linesOf(b.cha, func(r CHARecord) string { return r.ToLine() }))
	appendFile(out, "AER.txt", linesOf(b.aer, func(r AERRecord) string { return r.ToLine() }))
	appendFile(out, "ADP.txt", linesOf(b.adp, func(r ADPRecord) string { return r.ToLine() }))
	appendFile(out, "LVD.txt", linesOf(b.lvd, func(r LVDRecord) string { return r.ToLine() }))
	appendFile(out, "DRU.txt", linesOf(b.dru, func(r DRURecord) string { return r.ToLine() }))
	return Bundle{Files: out}
}

func appendFile(m map[string][]byte, name string, lines []string) {
	if len(lines) == 0 {
		return
	}
	sort.Strings(lines) // deterministic output → stable MD5
	var buf bytes.Buffer
	for i, l := range lines {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(l)
	}
	m[name] = buf.Bytes()
}

func linesOf[T any](rows []T, toLine func(T) string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = toLine(r)
	}
	return out
}

func must1(uuc string) string {
	if uuc == "" {
		return "1"
	}
	return uuc
}
