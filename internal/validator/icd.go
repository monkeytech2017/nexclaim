package validator

// ICD10Validator validate รหัส ICD-10 กับ master list
// TODO: โหลดจาก DB หรือ data/icd10.json
type ICD10Validator struct {
	codes map[string]bool
}

func NewICD10Validator(codes []string) *ICD10Validator {
	m := make(map[string]bool, len(codes))
	for _, c := range codes {
		m[c] = true
	}
	return &ICD10Validator{codes: m}
}

func (v *ICD10Validator) IsValid(code string) bool {
	return v.codes[code]
}

// ICD9CMValidator validate รหัส ICD-9CM
type ICD9CMValidator struct {
	codes map[string]bool
}

func NewICD9CMValidator(codes []string) *ICD9CMValidator {
	m := make(map[string]bool, len(codes))
	for _, c := range codes {
		m[c] = true
	}
	return &ICD9CMValidator{codes: m}
}

func (v *ICD9CMValidator) IsValid(code string) bool {
	return v.codes[code]
}
