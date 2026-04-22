package tests

import (
	"testing"

	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/validator"
)

func TestIsValidPersonID_Valid(t *testing.T) {
	// 1-1001-01001-27-1 is a classic demo ID that satisfies the check digit.
	// Build one programmatically so the test documents the rule clearly.
	id := makeThaiID("110010100127")
	if !validator.IsValidPersonID(id) {
		t.Errorf("expected %q to be valid", id)
	}
}

func TestIsValidPersonID_WrongLength(t *testing.T) {
	if validator.IsValidPersonID("123") {
		t.Error("short id should be invalid")
	}
	if validator.IsValidPersonID("") {
		t.Error("empty id should be invalid")
	}
}

func TestIsValidPersonID_BadCheckDigit(t *testing.T) {
	if validator.IsValidPersonID("1234567890123") {
		t.Error("random 13 digits should fail check digit")
	}
}

func TestIsValidAN(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"123456789", true},  // 9 digits ok
		{"1234567890", false}, // 10 digits too long
		{"", false},
		{"A12/45", false}, // contains /
		{"A*1", false},    // contains *
	}
	for _, c := range cases {
		if got := validator.IsValidAN(c.in); got != c.want {
			t.Errorf("IsValidAN(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestValidateOPD_UCSOk(t *testing.T) {
	v := model.OPDVisit{
		Patient: model.Patient{
			INSCL:    model.INSCL_UCS,
			PersonID: makeThaiID("110010100127"),
		},
		UUC: "1",
	}
	if errs := validator.ValidateOPD(v, validator.NoopMaster{}); len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateOPD_CSMBSRequiresPermit(t *testing.T) {
	v := model.OPDVisit{
		Patient: model.Patient{
			INSCL:    model.INSCL_CSMBS,
			PersonID: makeThaiID("110010100127"),
			PermitNo: "",
		},
		UUC: "1",
	}
	errs := validator.ValidateOPD(v, validator.NoopMaster{})
	if len(errs) == 0 {
		t.Fatal("expected PERMITNO error for CSMBS OPD")
	}
	if errs[0].CCode != "C125" {
		t.Errorf("want C125, got %s", errs[0].CCode)
	}
}

func TestValidateIPD_CSMBSRequiresCHAandDRG(t *testing.T) {
	a := model.IPDAdmit{
		Patient: model.Patient{INSCL: model.INSCL_CSMBS},
		AN:      "123456789",
		UUC:     "1",
	}
	errs := validator.ValidateIPD(a, validator.NoopMaster{})
	hasCHA, hasDRG := false, false
	for _, e := range errs {
		if e.Field == "CHA" {
			hasCHA = true
		}
		if e.Field == "DRGCODE" {
			hasDRG = true
		}
	}
	if !hasCHA || !hasDRG {
		t.Errorf("want CHA+DRGCODE errors, got %v", errs)
	}
}

// makeThaiID takes 12 digits and appends the valid Thai national-id check digit.
func makeThaiID(prefix12 string) string {
	if len(prefix12) != 12 {
		return prefix12
	}
	sum := 0
	for i := 0; i < 12; i++ {
		sum += int(prefix12[i]-'0') * (13 - i)
	}
	check := (11 - (sum % 11)) % 10
	return prefix12 + string(rune('0'+check))
}
