package tests

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/nexclaim/nexclaim/internal/generator/aipn"
	"github.com/nexclaim/nexclaim/internal/generator/ssop"
	"github.com/nexclaim/nexclaim/internal/model"
)

func TestAIPN_Build_DRDXAndDROPID(t *testing.T) {
	out, err := aipn.Build(aipn.Input{
		HCode: "12345", Period: "202504",
		IPD: []model.IPDAdmit{sampleSSOIPD()},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `DRDX="DR-SSO-2"`) {
		t.Errorf("expected DRDX attribute, got: %s", s)
	}
	body := out
	if i := strings.Index(s, "?>"); i > 0 {
		body = out[i+2:]
	}
	var doc aipn.Document
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Admits) != 1 || doc.Admits[0].AN != "A200001" {
		t.Errorf("admit = %+v", doc.Admits)
	}
	// AIPN header should NOT contain AGENCY (unlike CIPN)
	if strings.Contains(s, "<AGENCY>") {
		t.Error("AIPN header should not include AGENCY element")
	}
}

func TestSSOP_Build_DRDXAttribute(t *testing.T) {
	out, err := ssop.Build(ssop.Input{
		HCode: "12345", Period: "202504",
		OPD: []model.OPDVisit{sampleSSOOPD()},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `DRDX="DR-SSO-1"`) {
		t.Errorf("expected DRDX attribute on diag, got: %s", s)
	}
	body := out
	if i := strings.Index(s, "?>"); i > 0 {
		body = out[i+2:]
	}
	var doc ssop.Document
	if err := xml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Visits) != 1 {
		t.Fatalf("want 1 visit, got %d", len(doc.Visits))
	}
}

func TestAIPN_Build_RequiresHCode(t *testing.T) {
	_, err := aipn.Build(aipn.Input{Period: "202504"})
	if err == nil {
		t.Error("expected HCode error")
	}
}

func TestSSOP_Build_RequiresHCode(t *testing.T) {
	_, err := ssop.Build(ssop.Input{Period: "202504"})
	if err == nil {
		t.Error("expected HCode error")
	}
}
