package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/model"
)

// visitDetailFixture used by both GetVisit test and mapper tests.
func visitDetailFixture() hisclient.VisitDetail {
	return hisclient.VisitDetail{
		Patient: hisclient.VisitPatient{
			PID: "1100101001271", Prefix: "นาย",
			FirstName: "สมชาย", LastName: "ใจดี",
			DOB: "19850115", Sex: "1", Nation: "099",
			Changwat: "10", Amphur: "1001",
		},
		Visit: hisclient.VisitInfo{
			VN: "VN6804180001", SEQ: "SEQ6804180001",
			Date: "20250418", Time: "0930",
			ClinicCode: "MED01", UUC: "1",
		},
		Insurance: hisclient.VisitInsurance{INSCL: "011", PermitNo: "EDC20250418"},
		Diagnosis: []hisclient.VisitDiagnose{
			{ICD10: "I10", DxType: "1", DoctorCode: "D0045"},
			{ICD10: "E11.9", DxType: "2", DoctorCode: "D0045"},
		},
		Procedure: []hisclient.VisitProc{
			{ICD9CM: "89.52", Date: "20250418", DoctorCode: "D0045", Charge: 200},
		},
		Drug: []hisclient.VisitDrug{
			{
				HISItemID: "DRG000124", HISItemName: "AMLODIPINE 5MG TAB",
				TMTTP: "1004521", TMT24: "100452100101010100000612",
				Quantity: 30, Unit: "TAB", UnitPrice: 2.5, TotalPrice: 75,
				DoctorCode: "D0045",
			},
		},
		Charge: []hisclient.VisitCharge{
			{ChrgItem: "02", Amount: 200},
			{ChrgItem: "03", Amount: 297},
			{ChrgItem: "12", Amount: 100},
		},
	}
}

// ── HIS client against httptest mock ──

func TestHISClient_GetVisit(t *testing.T) {
	fix := visitDetailFixture()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/nexclaim/opd/visit/VN6804180001" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header = %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fix)
	}))
	defer srv.Close()

	c := hisclient.New(srv.URL, hisclient.WithBearerToken("TEST-TOKEN"))
	d, err := c.GetVisit("VN6804180001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if d.Patient.PID != "1100101001271" || len(d.Diagnosis) != 2 {
		t.Errorf("got %+v", d)
	}
}

func TestHISClient_GetVisit_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"NOT_FOUND","message":"VN not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()
	c := hisclient.New(srv.URL)
	_, err := c.GetVisit("VN-MISSING")
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "NOT_FOUND") {
		t.Errorf("error should include upstream msg, got: %v", err)
	}
}

func TestHISClient_BatchVisits_PartialOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("vn") != "VN1,VN2" {
			t.Errorf("vn query = %s", r.URL.Query().Get("vn"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hisclient.BatchVisitsResponse{
			Status: "PARTIAL", Count: 1,
			Visits: []hisclient.VisitDetail{visitDetailFixture()},
			Errors: []hisclient.BatchError{{VN: "VN2", Error: "NOT_FOUND", Message: "ไม่พบ"}},
		})
	}))
	defer srv.Close()
	c := hisclient.New(srv.URL)
	out, err := c.GetVisitsBatch([]string{"VN1", "VN2"})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if out.Status != "PARTIAL" || len(out.Visits) != 1 || len(out.Errors) != 1 {
		t.Errorf("got %+v", out)
	}
}

func TestHISClient_BatchTooMany(t *testing.T) {
	c := hisclient.New("http://ignored")
	var vns []string
	for i := 0; i < 51; i++ {
		vns = append(vns, "X")
	}
	_, err := c.GetVisitsBatch(vns)
	if err == nil {
		t.Error("expected batch-size error")
	}
}

func TestHISClient_Health(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hisclient.HealthResponse{Status: "ok", Version: "1.0.0"})
	}))
	defer srv.Close()
	c := hisclient.New(srv.URL, hisclient.WithAPIKey("K123"))
	h, err := c.Health()
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if h.Status != "ok" {
		t.Errorf("status = %s", h.Status)
	}
}

// ── Mapper tests ──

func TestMapper_ToOPDVisit(t *testing.T) {
	fix := visitDetailFixture()
	v := hisclient.ToOPDVisit(&fix, "HN001234", nil)
	if v.Patient.HN != "HN001234" {
		t.Errorf("HN should carry from summary, got %q", v.Patient.HN)
	}
	if v.Patient.INSCL != model.INSCL_CSMBS {
		t.Errorf("INSCL should be 011, got %s", v.Patient.INSCL)
	}
	if v.Patient.PermitNo != "EDC20250418" {
		t.Errorf("permit missed: %s", v.Patient.PermitNo)
	}
	if v.Patient.Name != "นายสมชาย ใจดี" {
		t.Errorf("name = %q", v.Patient.Name)
	}
	if v.DateOPD.Year() != 2025 || v.DateOPD.Month() != 4 || v.DateOPD.Day() != 18 {
		t.Errorf("date = %v", v.DateOPD)
	}
	if len(v.Diagnoses) != 2 || v.Diagnoses[0].Code != "I10" || v.Diagnoses[0].DoctorID != "D0045" {
		t.Errorf("dx mapping off: %+v", v.Diagnoses)
	}
	if len(v.Drugs) != 1 || v.Drugs[0].TMTID != "100452100101010100000612" {
		t.Errorf("drug TMT24 should win over TMT-TP: %+v", v.Drugs)
	}
	if v.Total != 597 { // 200+297+100
		t.Errorf("total = %v (want 597)", v.Total)
	}
	if v.IsUCEP {
		t.Error("no accident → IsUCEP should be false")
	}
}

func TestMapper_UCEPFromAccident(t *testing.T) {
	fix := visitDetailFixture()
	fix.Insurance.INSCL = "UCS"
	fix.Accident = &hisclient.VisitAccident{AEDate: "20250418", AEType: "1", Cause: "1"}
	v := hisclient.ToOPDVisit(&fix, "HN1", nil)
	if !v.IsUCEP {
		t.Error("accident on non-TPBS INSCL should set IsUCEP=true")
	}
}

func TestMapper_TPBS_NotUCEP(t *testing.T) {
	fix := visitDetailFixture()
	fix.Insurance.INSCL = "TPBS"
	fix.Accident = &hisclient.VisitAccident{AEDate: "20250418"}
	v := hisclient.ToOPDVisit(&fix, "HN1", nil)
	if v.IsUCEP {
		t.Error("TPBS has its own path, IsUCEP should stay false")
	}
}

func TestMapper_DrugFallbackToTMTTP(t *testing.T) {
	fix := visitDetailFixture()
	fix.Drug[0].TMT24 = "" // only TMT-TP available
	v := hisclient.ToOPDVisit(&fix, "HN1", nil)
	if v.Drugs[0].TMTID != "1004521" {
		t.Errorf("should fallback to TMT-TP, got %q", v.Drugs[0].TMTID)
	}
}
