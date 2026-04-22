package hisclient

import (
	"strings"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/util"
)

// ToOPDVisit แปลง HIS VisitDetail → domain model.OPDVisit.
//
// hn ส่งมาจาก VisitSummary ที่ HIS push ตอนแรก (detail response ไม่มี hn
// — หลัก spec). isUCEP ถูก derive จาก INSCL=TPBS หรือมี accident + ที่รพ.
// flag ว่าเป็น UCEP (ใน MVP = มี accident object).
func ToOPDVisit(d *VisitDetail, hn string) model.OPDVisit {
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
		// tmt24 มาก่อน, fallback tmt_tp (โรงพยาบาลส่วนใหญ่ยังใช้ TP)
		tmt := dr.TMT24
		if tmt == "" {
			tmt = dr.TMTTP
		}
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
