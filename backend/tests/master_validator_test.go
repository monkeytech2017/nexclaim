package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/validator"
)

func TestStaticMaster_LookupSets(t *testing.T) {
	m := validator.NewStaticMaster(
		[]string{"I10", "E11.9"},
		[]string{"47.0", "89.52"},
		[]string{"100000000000000000000001"},
	)
	if !m.IsValidICD10("I10") {
		t.Error("I10 should be valid")
	}
	if m.IsValidICD10("Z99.99") {
		t.Error("unseeded ICD should be invalid")
	}
	if !m.IsValidICD9CM("47.0") {
		t.Error("47.0 should be valid")
	}
	if !m.IsValidTMT("100000000000000000000001") {
		t.Error("TMT should be valid")
	}
}

func TestValidateOPD_MasterChecks(t *testing.T) {
	master := validator.NewStaticMaster(
		[]string{"I10"},    // ICD-10 allowlist
		nil,                // ICD-9CM
		nil,                // TMT
	)

	v := model.OPDVisit{
		Patient: model.Patient{
			INSCL:    model.INSCL_UCS,
			PersonID: makeThaiID("110010100127"),
		},
		UUC: "1",
		Diagnoses: []model.Diagnosis{
			{Code: "I10", Type: "1"},   // valid
			{Code: "Z99.9", Type: "1"}, // NOT in master
		},
	}
	errs := validator.ValidateOPD(v, master)

	var foundICD10Error bool
	for _, e := range errs {
		if e.Field == "ICD10" && e.Value == "Z99.9" {
			foundICD10Error = true
		}
	}
	if !foundICD10Error {
		t.Errorf("expected ICD10 error for Z99.9, got errors: %+v", errs)
	}
}

func TestValidateIPD_MasterChecks(t *testing.T) {
	master := validator.NewStaticMaster(
		[]string{"I10"}, nil, nil,
	)

	a := model.IPDAdmit{
		Patient: model.Patient{INSCL: model.INSCL_UCS},
		AN:      "A12345",
		UUC:     "1",
		AdmDx:   "BOGUS.1",                              // invalid
		Diagnoses: []model.Diagnosis{{Code: "I10", Type: "1"}}, // valid
	}
	errs := validator.ValidateIPD(a, master)
	var foundAdmDxErr bool
	for _, e := range errs {
		if e.Field == "ADMDX" && e.Value == "BOGUS.1" {
			foundAdmDxErr = true
		}
	}
	if !foundAdmDxErr {
		t.Errorf("expected ADMDX error for BOGUS.1, got %+v", errs)
	}
}

func TestMasterValidator_LoadFromPg(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set — skipping LoadFromDB test")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// Ensure at least one row so the loader has something to return.
	if _, err := conn.Exec(`
		INSERT INTO m_icd10 (code, name_th, name_en, is_active)
		VALUES ('I10', 'ความดันโลหิตสูง', 'Essential hypertension', true)
		ON CONFLICT (code) DO NOTHING
	`); err != nil {
		t.Fatalf("seed icd10: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mv, counts, err := validator.LoadFromDB(ctx, conn)
	if err != nil {
		t.Fatalf("LoadFromDB: %v", err)
	}
	if counts.ICD10 == 0 {
		t.Errorf("icd10 count = 0 — seed not persisted?")
	}
	if !mv.IsValidICD10("I10") {
		t.Error("I10 should be in master after LoadFromDB")
	}
	if mv.IsValidICD10("ZZZZ") {
		t.Error("non-existent code should be invalid")
	}
}
