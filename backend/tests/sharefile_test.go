package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/sharefile"
)

// fixtureExport writes a complete valid IPD export to a new tmp dir
// (at the path {root}/{exportID}/...) and returns the path. Cleans up on Cleanup.
func fixtureExport(t *testing.T, root, exportID string) string {
	t.Helper()
	dir := filepath.Join(root, exportID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"PAT.csv": `pid|prefix|first_name|last_name|dob|sex|marriage|nation|changwat|amphur|address
1100101001271|นาย|สมชาย|ใจดี|19850115|1|2|099|10|1001|123 ถ.สุขุมวิท
1100101001271|นาง|สมหญิง|รักดี|19900320|2|1|099|10|1024|789 ถ.พหลโยธิน`,

		"IPD.csv": `an|pid|hn|inscl|permit_no|agency_code|date_adm|time_adm|date_dsc|time_dsc|ward_admit|ward_dsc|room_type|los|dischs|discht|drg_code|adj_rw|doctor_code|uuc
AN001234|1100101001271|HN001234|011|EDC2025001||20250410|1430|20250418|1000|MED1|MED1|1|8|1|1|MDC14|1.2345|D0045|1
AN001235|1100101001271|HN005678|UCS|||20250412|0900|20250415|1100|SUR1|SUR1|1|3|1|1||0.8500|D0078|1`,

		"IDX.csv": `an|icd10|dx_type|doctor_code
AN001234|K35.8|1|D0045
AN001234|K65.0|3|D0045
AN001235|O80.0|1|D0078`,

		"IOP.csv": `an|icd9cm|op_date|op_time|doctor_code|charge
AN001234|47.0|20250411|0830|D0045|15000.00`,

		"DRU.csv": `an|his_item_id|his_item_name|tmt_tp|tmt24|quantity|unit|unit_price|total_price|drug_date_start|drug_date_end|usage|doctor_code
AN001234|DRG000567|CEFTRIAXONE 1G INJ|1005678||14|VIAL|285.0000|3990.00|20250410|20250416|IV drip OD|D0045
AN001235|DRG001234|OXYTOCIN 10U INJ|1007890||2|AMP|45.0000|90.00|20250412|20250412|IV drip|D0078`,

		"CHT.csv": `an|total_charge|total_claim|total_copay
AN001234|45800.00|42300.00|3500.00
AN001235|18500.00|18500.00|0.00`,

		"CHA.csv": `an|chrgitem|amount
AN001234|01|4800.00
AN001234|03|5010.00
AN001234|10|15000.00`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	manifest := sharefile.Manifest{
		ExportID: exportID, ExportDate: "2025-04-21T18:30:00+07:00",
		Period: "202504", HospitalCode: "12345",
		TotalAdmissions: 2,
		Files:           []string{"PAT.csv", "IPD.csv", "IDX.csv", "IOP.csv", "DRU.csv", "CHT.csv", "CHA.csv"},
		ExportedBy:      "unit-test",
	}
	mb, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "MANIFEST.json"), mb, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ── manifest + csv tests ──

func TestSharefile_LoadManifest_OK(t *testing.T) {
	dir := fixtureExport(t, t.TempDir(), "EXP-OK")
	m, err := sharefile.LoadManifest(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Period != "202504" || m.HospitalCode != "12345" {
		t.Errorf("manifest = %+v", m)
	}
}

func TestSharefile_LoadManifest_MissingRequired(t *testing.T) {
	dir := t.TempDir()
	// MANIFEST says PAT+IPD required but we don't create them
	mb, _ := json.Marshal(sharefile.Manifest{
		ExportID: "X", Period: "202504", HospitalCode: "12345",
		Files: []string{"ONLY.csv"},
	})
	_ = os.WriteFile(filepath.Join(dir, "MANIFEST.json"), mb, 0o644)
	_, err := sharefile.LoadManifest(dir)
	if err == nil {
		t.Error("expected error when PAT/IPD missing")
	}
}

// ── parse + assemble tests ──

func TestSharefile_ParseAndAssemble(t *testing.T) {
	dir := fixtureExport(t, t.TempDir(), "EXP-1")
	m, err := sharefile.LoadManifest(dir)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	bundle, err := sharefile.Parse(dir, m)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(bundle.PAT) != 2 || len(bundle.IPD) != 2 {
		t.Errorf("counts PAT=%d IPD=%d", len(bundle.PAT), len(bundle.IPD))
	}

	admits, err := sharefile.Assemble(bundle)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(admits) != 2 {
		t.Fatalf("want 2 admits, got %d", len(admits))
	}

	first := admits[0]
	if first.AN != "AN001234" {
		t.Errorf("an = %s", first.AN)
	}
	if first.Patient.INSCL != model.INSCL_CSMBS {
		t.Errorf("INSCL = %s", first.Patient.INSCL)
	}
	if first.Patient.PermitNo != "EDC2025001" {
		t.Errorf("permit = %s", first.Patient.PermitNo)
	}
	if first.LOS != 8 {
		t.Errorf("los = %d", first.LOS)
	}
	if first.DRG == nil || first.DRG.Code != "MDC14" || first.DRG.AdjRW != 1.2345 {
		t.Errorf("DRG = %+v", first.DRG)
	}
	if len(first.Diagnoses) != 2 || first.AdmDx != "K35.8" {
		t.Errorf("diag = %+v / admdx=%s", first.Diagnoses, first.AdmDx)
	}
	if len(first.Ops) != 1 || first.Ops[0].Code != "47.0" {
		t.Errorf("ops = %+v", first.Ops)
	}
	if len(first.Drugs) != 1 {
		t.Errorf("drugs = %d", len(first.Drugs))
	}
	if len(first.Charges) != 3 {
		t.Errorf("charges = %d", len(first.Charges))
	}
	if first.Total != 45800 || first.Paid != 42300 {
		t.Errorf("total=%v paid=%v", first.Total, first.Paid)
	}
}

func TestSharefile_Assemble_MissingPAT(t *testing.T) {
	dir := t.TempDir()
	// IPD references pid that PAT doesn't have
	files := map[string]string{
		"PAT.csv": "pid|prefix|first_name|last_name|dob|sex|marriage|nation|changwat|amphur|address\n" +
			"9999999999999|นาย|X|Y|19850115|1|2|099|10|1001|-",
		"IPD.csv": "an|pid|hn|inscl|permit_no|agency_code|date_adm|time_adm|date_dsc|time_dsc|ward_admit|ward_dsc|room_type|los|dischs|discht|drg_code|adj_rw|doctor_code|uuc\n" +
			"A1|1100101001271|H1|UCS|||20250410|1000|20250411|1000||||1|1|1|||1",
	}
	for name, content := range files {
		_ = os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	mb, _ := json.Marshal(sharefile.Manifest{
		ExportID: "X", Period: "202504", HospitalCode: "12345",
		Files: []string{"PAT.csv", "IPD.csv"},
	})
	_ = os.WriteFile(filepath.Join(dir, "MANIFEST.json"), mb, 0o644)

	m, err := sharefile.LoadManifest(dir)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	bundle, _ := sharefile.Parse(dir, m)
	_, err = sharefile.Assemble(bundle)
	if err == nil {
		t.Error("expected error when PAT missing for referenced pid")
	}
}

func TestSharefile_Assemble_UCEPFromAER(t *testing.T) {
	root := t.TempDir()
	dir := fixtureExport(t, root, "EXP-UCEP")
	// Change INSCL on AN001235 to UCS and add AER record
	_ = os.WriteFile(filepath.Join(dir, "AER.csv"), []byte("an|ae_date|ae_time|ae_type|cause|place\nAN001234|20250410|1200|1|1|ถ.XX"), 0o644)
	mb, _ := json.Marshal(sharefile.Manifest{
		ExportID: "EXP-UCEP", Period: "202504", HospitalCode: "12345",
		Files: []string{"PAT.csv", "IPD.csv", "IDX.csv", "IOP.csv", "DRU.csv", "CHT.csv", "CHA.csv", "AER.csv"},
	})
	_ = os.WriteFile(filepath.Join(dir, "MANIFEST.json"), mb, 0o644)

	m, _ := sharefile.LoadManifest(dir)
	b, _ := sharefile.Parse(dir, m)
	admits, err := sharefile.Assemble(b)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	foundUCEP := false
	for _, a := range admits {
		if a.AN == "AN001234" && a.IsUCEP {
			foundUCEP = true
		}
	}
	if !foundUCEP {
		t.Error("AN001234 should have IsUCEP=true from AER row (non-TPBS)")
	}
}

// ── FileExtractor integration ──

func TestFileExtractor_FiltersByINSCL(t *testing.T) {
	dir := fixtureExport(t, t.TempDir(), "EXP-FILT")
	ex := extractor.NewFileExtractor(dir)

	uc, err := ex.Extract(context.Background(), extractor.Request{INSCL: model.INSCL_UCS, Period: "202504"})
	if err != nil {
		t.Fatalf("UCS extract: %v", err)
	}
	if len(uc.IPD) != 1 || uc.IPD[0].AN != "AN001235" {
		t.Errorf("UCS → want AN001235, got %+v", uc.IPD)
	}

	csmbs, err := ex.Extract(context.Background(), extractor.Request{INSCL: model.INSCL_CSMBS, Period: "202504"})
	if err != nil {
		t.Fatalf("CSMBS extract: %v", err)
	}
	if len(csmbs.IPD) != 1 || csmbs.IPD[0].AN != "AN001234" {
		t.Errorf("CSMBS → want AN001234, got %+v", csmbs.IPD)
	}
}

func TestFileExtractor_AdmitsAll(t *testing.T) {
	dir := fixtureExport(t, t.TempDir(), "EXP-ALL")
	ex := extractor.NewFileExtractor(dir)
	admits, err := ex.Admits()
	if err != nil {
		t.Fatalf("admits: %v", err)
	}
	if len(admits) != 2 {
		t.Errorf("want 2 admits, got %d", len(admits))
	}
}
