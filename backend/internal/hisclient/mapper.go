package hisclient

import (
	"strings"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/util"
)

// TMTResolver แปลง HIS internal drug code → TMT 24 หลัก โดยอ้างอิง
// his_drug_map ของโรงพยาบาล. คืน ok=false เมื่อไม่พบ mapping ที่ active.
// nil resolver = ไม่มี DB mapping (in-memory/dry-run) → fallback เดิม.
type TMTResolver func(hisCode string) (tmtCode string, ok bool)

// ToOPDVisit แปลง HIS VisitDetail → domain model.OPDVisit.
//
// hn ส่งมาจาก VisitSummary ที่ HIS push ตอนแรก (detail response ไม่มี hn
// — หลัก spec). isUCEP ถูก derive จาก INSCL=TPBS หรือมี accident + ที่รพ.
// flag ว่าเป็น UCEP (ใน MVP = มี accident object).
//
// resolver (อาจเป็น nil) ใช้แปลง HIS internal drug code → TMT เมื่อ HIS
// ไม่ได้ส่ง TMT มาเอง.
func ToOPDVisit(d *VisitDetail, hn string, resolver TMTResolver) model.OPDVisit {
	p := d.Patient
	name := strings.TrimSpace(p.Prefix + p.FirstName + " " + p.LastName)
	dob, _ := util.ParseHISDate(p.DOB)
	dateOPD, _ := util.ParseHISDate(d.Visit.Date)

	v := model.OPDVisit{
		Patient: model.Patient{
			HN:         hn,
			PersonID:   p.PID,
			INSCL:      model.INSCL(d.Insurance.INSCL),
			AgencyCode: d.Insurance.AgencyCode,
			Name:       name,
			DOB:        dob,
			Sex:        p.Sex,
			Nation:     p.Nation,
			Changwat:   p.Changwat,
			Amphur:     p.Amphur,
			PermitNo:   d.Insurance.PermitNo,
		},
		SEQ:     d.Visit.SEQ,
		DateOPD: dateOPD,
		TimeOPD: d.Visit.Time,
		Clinic:  d.Visit.ClinicCode,
		UUC:     d.Visit.UUC,
	}

	for _, dx := range d.Diagnosis {
		v.Diagnoses = append(v.Diagnoses, model.Diagnosis{
			Code: dx.ICD10, Type: dx.DxType, DoctorID: dx.DoctorCode,
		})
	}
	for _, op := range d.Procedure {
		opDate, _ := util.ParseHISDate(op.Date)
		v.Ops = append(v.Ops, model.Operation{
			Code: op.ICD9CM, DoctorID: op.DoctorCode, Date: opDate,
		})
	}
	for _, dr := range d.Drug {
		tmt := resolveTMT(dr.TMT24, dr.TMTTP, dr.HISItemID, resolver)
		cost := 0.0
		if dr.Quantity > 0 {
			cost = dr.TotalPrice / dr.Quantity
		}
		v.Drugs = append(v.Drugs, model.Drug{
			TMTID:  tmt,
			Amount: dr.Quantity,
			Price:  dr.UnitPrice,
			Cost:   cost,
			Unit:   dr.Unit,
		})
	}
	for _, ch := range d.Charge {
		v.Charges = append(v.Charges, model.ChargeItem{
			Code: ch.ChrgItem, Amount: ch.Amount,
		})
		v.Total += ch.Amount
	}

	// UCEP: accident object ที่ไม่ใช่ TPBS (TPBS มี path ของตัวเอง)
	if d.Accident != nil && v.Patient.INSCL != model.INSCL_TPBS {
		v.IsUCEP = true
	}
	return v
}

// resolveTMT decides the TMT id for one drug line.
//
// Resolution order:
//  1. HIS-provided TMT wins: tmt24, else tmt_tp (non-empty).
//  2. Otherwise consult the his_drug_map resolver (if wired) by HIS internal
//     drug code; use its result only when non-empty.
//  3. Otherwise keep the original fallback (tmtTP, which may be empty).
func resolveTMT(tmt24, tmtTP, hisCode string, resolver TMTResolver) string {
	if tmt24 != "" {
		return tmt24
	}
	if tmtTP != "" {
		return tmtTP
	}
	if resolver != nil && hisCode != "" {
		if mapped, ok := resolver(hisCode); ok && mapped != "" {
			return mapped
		}
	}
	return tmtTP
}
