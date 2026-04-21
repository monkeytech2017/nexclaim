package validator

import (
	"github.com/nexclaim/nexclaim/internal/model"
)

// Validate ตรวจสอบ OPD visit ตาม business rules แยกตามสิทธิ
func ValidateOPD(visit model.OPDVisit) []ValidationError {
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
		for i, dx := range visit.Diagnoses {
			if dx.DoctorID == "" {
				errs = append(errs, ValidationError{Field: "DRDX", Value: "", Reason: "SSO: DRDX บังคับทุก diagnosis record", CCode: "C130"})
				_ = i
			}
		}
	}

	return errs
}

// ValidateIPD ตรวจสอบ IPD admit
func ValidateIPD(admit model.IPDAdmit) []ValidationError {
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

	return errs
}
