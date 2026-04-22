package validator

import (
	"github.com/nexclaim/nexclaim/internal/model"
)

// ValidateOPD ตรวจสอบ OPD visit ตาม business rules + master lookups.
// master = nil → bypass ICD/TMT checks (useful ใน tests / bootstrap ก่อน seed).
func ValidateOPD(visit model.OPDVisit, master MasterValidator) []ValidationError {
	if master == nil {
		master = NoopMaster{}
	}
	var errs []ValidationError

	// กฎทั่วไป
	if !IsValidUUC(visit.UUC) {
		errs = append(errs, ValidationError{Field: "UUC", Value: visit.UUC, CCode: "C115", Reason: "UUC ต้องเป็น '1'"})
	}
	if !IsValidPersonID(visit.Patient.PersonID) {
		switch visit.Patient.INSCL {
		case model.INSCL_WP1, model.INSCL_WP2, model.INSCL_NON:
			// แรงงานต่างด้าว/ไร้สัญชาติ — ใช้ format แยก ไม่ validate check digit
		default:
			errs = append(errs, ValidationError{Field: "PERSON_ID", Value: visit.Patient.PersonID, CCode: "C104", Reason: "PERSON_ID ไม่ถูกต้อง"})
		}
	}

	// กฎเฉพาะ CSMBS/LGO/OFC
	switch visit.Patient.INSCL {
	case model.INSCL_CSMBS, model.INSCL_WEL, model.INSCL_LGO, model.INSCL_OFC:
		if visit.Patient.PermitNo == "" {
			errs = append(errs, ValidationError{Field: "PERMITNO", Value: "", CCode: "C125", Reason: "PERMITNO บังคับสำหรับ CSMBS/LGO/OFC OPD"})
		}
	}

	// กฎ SSO — DRDX/DROPID บังคับทุก record
	if visit.Patient.INSCL == model.INSCL_SSS || visit.Patient.INSCL == model.INSCL_SS4 {
		for _, dx := range visit.Diagnoses {
			if dx.DoctorID == "" {
				errs = append(errs, ValidationError{Field: "DRDX", Value: "", Reason: "SSO: DRDX บังคับทุก diagnosis record", CCode: "C130"})
			}
		}
	}

	// Master lookups — ตรวจทุก code ที่ยังไม่ว่าง
	for _, dx := range visit.Diagnoses {
		if dx.Code != "" && !master.IsValidICD10(dx.Code) {
			errs = append(errs, ValidationError{Field: "ICD10", Value: dx.Code, CCode: "C101", Reason: "ICD-10 ไม่อยู่ใน master"})
		}
	}
	for _, op := range visit.Ops {
		if op.Code != "" && !master.IsValidICD9CM(op.Code) {
			errs = append(errs, ValidationError{Field: "ICD9CM", Value: op.Code, Reason: "ICD-9CM ไม่อยู่ใน master"})
		}
	}
	for _, d := range visit.Drugs {
		if d.TMTID != "" && !master.IsValidTMT(d.TMTID) {
			errs = append(errs, ValidationError{Field: "TMT", Value: d.TMTID, Reason: "TMT code ไม่อยู่ใน master"})
		}
	}

	return errs
}

// ValidateIPD ตรวจสอบ IPD admit ตาม business rules + master lookups.
func ValidateIPD(admit model.IPDAdmit, master MasterValidator) []ValidationError {
	if master == nil {
		master = NoopMaster{}
	}
	var errs []ValidationError

	if !IsValidAN(admit.AN) {
		errs = append(errs, ValidationError{Field: "AN", Value: admit.AN, Reason: "AN ไม่ถูกต้อง (ต้อง ≤9 หลัก)"})
	}
	if !IsValidUUC(admit.UUC) {
		errs = append(errs, ValidationError{Field: "UUC", Value: admit.UUC, CCode: "C115", Reason: "UUC ต้องเป็น '1'"})
	}

	// CSMBS/LGO ต้องมี CHA (CHRGITEM) และ DRG
	switch admit.Patient.INSCL {
	case model.INSCL_CSMBS, model.INSCL_WEL, model.INSCL_LGO, model.INSCL_OFC:
		if len(admit.Charges) == 0 {
			errs = append(errs, ValidationError{Field: "CHA", Value: "", Reason: "CSMBS/LGO: CHA (CHRGITEM 01-16) บังคับ"})
		}
		if admit.DRG == nil || admit.DRG.Code == "" {
			errs = append(errs, ValidationError{Field: "DRGCODE", Value: "", Reason: "CSMBS/LGO: DRGCODE บังคับ"})
		}
	}

	// Master lookups — ICD-10 (primary + comorbidity + complication + external)
	for _, dx := range admit.Diagnoses {
		if dx.Code != "" && !master.IsValidICD10(dx.Code) {
			errs = append(errs, ValidationError{Field: "ICD10", Value: dx.Code, CCode: "C101", Reason: "ICD-10 ไม่อยู่ใน master"})
		}
	}
	if admit.AdmDx != "" && !master.IsValidICD10(admit.AdmDx) {
		errs = append(errs, ValidationError{Field: "ADMDX", Value: admit.AdmDx, CCode: "C101", Reason: "ADMDX ICD-10 ไม่อยู่ใน master"})
	}
	for _, op := range admit.Ops {
		if op.Code != "" && !master.IsValidICD9CM(op.Code) {
			errs = append(errs, ValidationError{Field: "ICD9CM", Value: op.Code, Reason: "ICD-9CM ไม่อยู่ใน master"})
		}
	}
	for _, d := range admit.Drugs {
		if d.TMTID != "" && !master.IsValidTMT(d.TMTID) {
			errs = append(errs, ValidationError{Field: "TMT", Value: d.TMTID, Reason: "TMT code ไม่อยู่ใน master"})
		}
	}

	return errs
}
