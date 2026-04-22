package tests

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/nexclaim/nexclaim/internal/generator/csop"
	"github.com/nexclaim/nexclaim/internal/model"
)

func TestCSOP_Build_ValidXML(t *testing.T) {
	out, err := csop.Build(csop.Input{
		HCode: "12345", Agency: model.AgencyCGD, Period: "202504",
		OPD: []model.OPDVisit{sampleCSMBSOPD()},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	body := out
	if i := strings.Index(string(body), "?>"); i > 0 {
		body = body[i+2:]
	}
	var doc csop.Document
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if doc.Header.Agency != "CGD" || doc.Header.Period != "202504" {
		t.Errorf("header = %+v", doc.Header)
	}
	if len(doc.Visits) != 1 {
		t.Fatalf("want 1 visit, got %d", len(doc.Visits))
	}
	v := doc.Visits[0]
	if v.SEQ != "S2" {
		t.Errorf("SEQ = %s, want S2", v.SEQ)
	}
	if v.DateOPD != "20250421" {
		t.Errorf("DATEOPD should be 20250421, got %s", v.DateOPD)
	}
	if v.PermitNo != "P1" {
		t.Errorf("PERMITNO should carry through, got %q", v.PermitNo)
	}
}

func TestCSOP_Build_RequiresAgency(t *testing.T) {
	_, err := csop.Build(csop.Input{HCode: "12345"})
	if err == nil {
		t.Error("expected Agency error")
	}
}

func TestCSOP_Build_XMLDeclaration(t *testing.T) {
	out, _ := csop.Build(csop.Input{
		HCode: "12345", Agency: model.AgencyCGD, Period: "202504",
	})
	if !strings.HasPrefix(string(out), "<?xml") {
		t.Errorf("expected XML declaration; got %q", string(out[:20]))
	}
}
