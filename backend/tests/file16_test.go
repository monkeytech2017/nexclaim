package tests

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/generator/file16"
	"github.com/nexclaim/nexclaim/internal/model"
)

func sampleOPD() model.OPDVisit {
	dob, _ := time.Parse("2006-01-02", "1990-04-21")
	date, _ := time.Parse("2006-01-02", "2025-04-21")
	return model.OPDVisit{
		Patient: model.Patient{
			HN: "H0001", PersonID: "1234567890121",
			INSCL: model.INSCL_UCS, PTTYPE: "89",
			Name: "SOMCHAI T.", DOB: dob, Sex: "1", Nation: "099",
			Changwat: "10", Amphur: "1001",
		},
		SEQ: "S001", DateOPD: date, TimeOPD: "0830",
		Clinic: "0100", TypeOut: "1", UUC: "1",
		Diagnoses: []model.Diagnosis{
			{Code: "A000", Type: "1", DoctorID: "DR1"},
		},
		Drugs: []model.Drug{
			{TMTID: "100000000000000000000001", Amount: 1, Unit: "TAB", Price: 12.50, Cost: 10.00},
		},
		Charges: []model.ChargeItem{{Code: "01", Amount: 500}},
		Total:   500, Paid: 0,
	}
}

func TestFile16_Build_MinimalUCS(t *testing.T) {
	b, err := file16.Build(file16.Input{
		HCode:  "12345",
		Period: "202504",
		OPD:    []model.OPDVisit{sampleOPD()},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	want := []string{"INS.txt", "PAT.txt", "OPD.txt", "ODX.txt", "CHT.txt", "CHA.txt", "DRU.txt"}
	for _, name := range want {
		if _, ok := b.Files[name]; !ok {
			t.Errorf("missing file: %s", name)
		}
	}
	// files with no data should not appear
	for _, name := range []string{"IPD.txt", "IDX.txt", "IOP.txt", "IRF.txt", "ORF.txt", "AER.txt", "ADP.txt", "LVD.txt"} {
		if _, ok := b.Files[name]; ok {
			t.Errorf("unexpected file: %s", name)
		}
	}
}

func TestFile16_PipeDelimited_NoHeader(t *testing.T) {
	b, _ := file16.Build(file16.Input{HCode: "12345", OPD: []model.OPDVisit{sampleOPD()}})
	ins := string(b.Files["INS.txt"])
	if !strings.Contains(ins, "|") {
		t.Errorf("INS should be pipe-delimited, got %q", ins)
	}
	// 9 columns → 8 separators
	if got := strings.Count(ins, "|"); got != 8 {
		t.Errorf("INS: want 8 pipes (9 cols), got %d", got)
	}
	if strings.HasPrefix(ins, "HOSPMAIN") || strings.HasPrefix(ins, "HN") {
		t.Error("INS should not have a header row")
	}
}

func TestFile16_DateInAD_YYYYMMDD(t *testing.T) {
	b, _ := file16.Build(file16.Input{HCode: "12345", OPD: []model.OPDVisit{sampleOPD()}})
	opd := string(b.Files["OPD.txt"])
	if !strings.Contains(opd, "20250421") {
		t.Errorf("OPD date should be 20250421 (ค.ศ.), got %q", opd)
	}
}

func TestFile16_UUC_Always1(t *testing.T) {
	v := sampleOPD()
	v.UUC = "" // should be forced to "1"
	b, _ := file16.Build(file16.Input{HCode: "12345", OPD: []model.OPDVisit{v}})
	opd := string(b.Files["OPD.txt"])
	// UUC is column index 5 (0-based); just assert "1" appears between pipes after SEQ
	if !strings.Contains(opd, "S001|1|") {
		t.Errorf("UUC should normalize to 1; got line: %q", opd)
	}
}

func TestFile16_Deterministic(t *testing.T) {
	in := file16.Input{HCode: "12345", OPD: []model.OPDVisit{sampleOPD(), sampleOPD()}}
	b1, _ := file16.Build(in)
	b2, _ := file16.Build(in)
	for name, c1 := range b1.Files {
		c2 := b2.Files[name]
		if !bytes.Equal(c1, c2) {
			t.Errorf("%s: non-deterministic output", name)
		}
	}
}

func TestFile16_MissingHCode(t *testing.T) {
	_, err := file16.Build(file16.Input{HCode: ""})
	if err == nil {
		t.Error("expected error when HCode missing")
	}
}
