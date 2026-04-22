package sharefile

import (
	"fmt"
	"strings"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/util"
)

// Assemble stitches the parsed row sets into []model.IPDAdmit.
//
// Joins:
//   IPD.pid    → PAT.pid    (patient info)
//   IDX.an     → IPD.an     (diagnoses)
//   IOP.an     → IPD.an     (procedures)
//   DRU.an     → IPD.an     (drugs)
//   CHT.an     → IPD.an     (total charge / paid)
//   CHA.an     → IPD.an     (charge by CHRGITEM — CSMBS/LGO/OFC)
//   AER.an     → IPD.an     (accident → triggers IsUCEP if not TPBS)
//
// Missing PAT row for a given pid is treated as a hard error — spec says
// PAT.csv must contain every patient referenced in IPD.csv.
func Assemble(b *Bundle) ([]model.IPDAdmit, error) {
	if b == nil {
		return nil, fmt.Errorf("assemble: nil bundle")
	}

	pats := make(map[string]PATRow, len(b.PAT))
	for _, p := range b.PAT {
		pats[p.PID] = p
	}

	idxByAN := groupByAN(b.IDX, func(r IDXRow) string { return r.AN })
	iopByAN := groupByAN(b.IOP, func(r IOPRow) string { return r.AN })
	druByAN := groupByAN(b.DRU, func(r DRURow) string { return r.AN })
	chaByAN := groupByAN(b.CHA, func(r CHARow) string { return r.AN })
	chtByAN := singleByAN(b.CHT, func(r CHTRow) string { return r.AN })
	aerByAN := singleByAN(b.AER, func(r AERRow) string { return r.AN })

	out := make([]model.IPDAdmit, 0, len(b.IPD))
	for i, ipd := range b.IPD {
		pat, ok := pats[ipd.PID]
		if !ok {
			return nil, fmt.Errorf("assemble: IPD.csv row %d (an=%s): PAT.csv has no row for pid=%s",
				i+1, ipd.AN, ipd.PID)
		}

		admit := model.IPDAdmit{
			Patient: patientFromRow(pat, ipd),
			AN:      ipd.AN,
			DateAdm: util.MustParseHISDate(ipd.DateAdm),
			TimeAdm: ipd.TimeAdm,
			DateDsc: util.MustParseHISDate(ipd.DateDsc),
			TimeDsc: ipd.TimeDsc,
			WardDsc: ipd.WardDsc,
			LOS:     ipd.LOS,
			Dischs:  ipd.Dischs,
			Discht:  ipd.Discht,
			UUC:     util.StrOr(ipd.UUC, "1"),
		}
		if ipd.DRGCode != "" || ipd.AdjRW > 0 {
			admit.DRG = &model.DRGInfo{Code: ipd.DRGCode, AdjRW: ipd.AdjRW}
		}

		for _, dx := range idxByAN[ipd.AN] {
			admit.Diagnoses = append(admit.Diagnoses, model.Diagnosis{
				Code: dx.ICD10, Type: dx.DxType, DoctorID: dx.DoctorCode,
			})
			if admit.AdmDx == "" && dx.DxType == "1" {
				admit.AdmDx = dx.ICD10
			}
		}
		for _, op := range iopByAN[ipd.AN] {
			admit.Ops = append(admit.Ops, model.Operation{
				Code: op.ICD9CM, DoctorID: op.DoctorCode,
				Date: util.MustParseHISDate(op.OpDate),
			})
		}
		for _, dr := range druByAN[ipd.AN] {
			tmt := dr.TMT24
			if tmt == "" {
				tmt = dr.TMTTP
			}
			cost := 0.0
			if dr.Quantity > 0 {
				cost = dr.TotalPrice / dr.Quantity
			}
			admit.Drugs = append(admit.Drugs, model.Drug{
				TMTID: tmt, Amount: dr.Quantity,
				Price: dr.UnitPrice, Cost: cost, Unit: dr.Unit,
			})
		}
		for _, ch := range chaByAN[ipd.AN] {
			admit.Charges = append(admit.Charges, model.ChargeItem{
				Code: ch.ChrgItem, Amount: ch.Amount,
			})
		}
		if cht, ok := chtByAN[ipd.AN]; ok {
			admit.Total = cht.TotalCharge
			admit.Paid = cht.TotalClaim // amount to be reimbursed
		}
		if _, ok := aerByAN[ipd.AN]; ok && admit.Patient.INSCL != model.INSCL_TPBS {
			admit.IsUCEP = true
		}

		out = append(out, admit)
	}
	return out, nil
}

func patientFromRow(p PATRow, ipd IPDRow) model.Patient {
	name := strings.TrimSpace(p.Prefix + p.FirstName + " " + p.LastName)
	return model.Patient{
		HN:         ipd.HN,
		PersonID:   p.PID,
		INSCL:      model.INSCL(ipd.INSCL),
		AgencyCode: ipd.AgencyCode,
		Name:       name,
		DOB:        util.MustParseHISDate(p.DOB),
		Sex:        p.Sex,
		Nation:     p.Nation,
		Changwat:   p.Changwat,
		Amphur:     p.Amphur,
		PermitNo:   ipd.PermitNo,
	}
}

func groupByAN[T any](rows []T, key func(T) string) map[string][]T {
	m := make(map[string][]T)
	for _, r := range rows {
		m[key(r)] = append(m[key(r)], r)
	}
	return m
}

func singleByAN[T any](rows []T, key func(T) string) map[string]T {
	m := make(map[string]T, len(rows))
	for _, r := range rows {
		m[key(r)] = r
	}
	return m
}
