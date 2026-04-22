package tests

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/nexclaim/nexclaim/internal/generator/cipn"
	"github.com/nexclaim/nexclaim/internal/model"
)

func TestCIPN_Build_ValidXML(t *testing.T) {
	out, err := cipn.Build(cipn.Input{
		HCode: "12345", Agency: model.AgencyCGD, Period: "202504",
		IPD: []model.IPDAdmit{sampleCSMBSIPD()},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// should parse as well-formed XML
	var doc cipn.Document
	body := out
	// strip XML declaration before unmarshal (stdlib xml.Unmarshal tolerates it but safe)
	if i := strings.Index(string(body), "?>"); i > 0 {
		body = body[i+2:]
	}
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("unmarshal: %v\nXML was:\n%s", err, out)
	}
	if doc.Header.Agency != "CGD" {
		t.Errorf("want AGENCY=CGD, got %s", doc.Header.Agency)
	}
	if doc.Header.Period != "202504" {
		t.Errorf("want PERIOD=202504, got %s", doc.Header.Period)
	}
	if len(doc.Admits) != 1 {
		t.Fatalf("want 1 admit, got %d", len(doc.Admits))
	}
	a := doc.Admits[0]
	if a.AN != "A100001" {
		t.Errorf("AN = %s, want A100001", a.AN)
	}
	if a.DateAdm != "20250421" {
		t.Errorf("DATEADM should be 20250421, got %s", a.DateAdm)
	}
	if a.UUC != "1" {
		t.Errorf("UUC should be 1, got %s", a.UUC)
	}
	if len(a.Diagnoses) != 1 || a.Diagnoses[0].DxType != "1" || a.Diagnoses[0].Code != "I10" {
		t.Errorf("diag = %+v", a.Diagnoses)
	}
	if a.DRG == nil || a.DRG.Code != "05130" {
		t.Errorf("DRG = %+v", a.DRG)
	}
}

func TestCIPN_Build_RequiresHCode(t *testing.T) {
	_, err := cipn.Build(cipn.Input{Agency: model.AgencyCGD, Period: "202504"})
	if err == nil {
		t.Error("expected HCode error")
	}
}

func TestCIPN_Build_RequiresAgency(t *testing.T) {
	_, err := cipn.Build(cipn.Input{HCode: "12345", Period: "202504"})
	if err == nil {
		t.Error("expected Agency error")
	}
}

func TestCIPN_Build_XMLDeclaration(t *testing.T) {
	out, err := cipn.Build(cipn.Input{
		HCode: "12345", Agency: model.AgencyCGD, Period: "202504",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.HasPrefix(string(out), "<?xml") {
		t.Errorf("output should start with XML declaration, got %q", string(out[:20]))
	}
}
