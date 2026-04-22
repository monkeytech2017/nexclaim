package tests

import (
	"context"
	"testing"
	"time"

	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/sender"
)

type fakeFDH struct {
	calls16, callsCIPN, callsCSOP int
	lastCIPNZip, lastCSOPZip      []byte
}

type fakeCHI struct {
	callsAIPN, callsSSOP int
	lastAIPNZip          []byte
	lastSSOPZip          []byte
}

func (c *fakeCHI) SendAIPN(z []byte, period string) (*sender.SubmitResult, error) {
	c.callsAIPN++
	c.lastAIPNZip = z
	return &sender.SubmitResult{TxnID: "TXN-AIPN-" + period, Status: "ACCEPTED"}, nil
}

func (c *fakeCHI) SendSSOP(z []byte, period string) (*sender.SubmitResult, error) {
	c.callsSSOP++
	c.lastSSOPZip = z
	return &sender.SubmitResult{TxnID: "TXN-SSOP-" + period, Status: "ACCEPTED"}, nil
}

func (f *fakeFDH) Send16Files(z []byte, period string) (*sender.SubmitResult, error) {
	f.calls16++
	return &sender.SubmitResult{TxnID: "TXN-16-" + period, Status: "ACCEPTED"}, nil
}

func (f *fakeFDH) SendCIPN(z []byte, period string) (*sender.SubmitResult, error) {
	f.callsCIPN++
	f.lastCIPNZip = z
	return &sender.SubmitResult{TxnID: "TXN-CIPN-" + period, Status: "ACCEPTED"}, nil
}

func (f *fakeFDH) SendCSOP(z []byte, period string) (*sender.SubmitResult, error) {
	f.callsCSOP++
	f.lastCSOPZip = z
	return &sender.SubmitResult{TxnID: "TXN-CSOP-" + period, Status: "ACCEPTED"}, nil
}

func fxDOB() time.Time {
	t, _ := time.Parse("2006-01-02", "1990-04-21")
	return t
}
func fxDate() time.Time {
	t, _ := time.Parse("2006-01-02", "2025-04-21")
	return t
}

func sampleUCSVisit() model.OPDVisit {
	return model.OPDVisit{
		Patient: model.Patient{
			HN: "H1", PersonID: makeThaiID("110010100127"),
			INSCL: model.INSCL_UCS, DOB: fxDOB(), Sex: "1", Nation: "099",
		},
		SEQ: "S1", DateOPD: fxDate(), UUC: "1",
		Diagnoses: []model.Diagnosis{{Code: "A000", Type: "1"}},
		Total:     100,
	}
}

func sampleCSMBSOPD() model.OPDVisit {
	return model.OPDVisit{
		Patient: model.Patient{
			HN: "H2", PersonID: makeThaiID("110010100127"),
			INSCL: model.INSCL_CSMBS, PermitNo: "P1",
		},
		SEQ: "S2", DateOPD: fxDate(), UUC: "1",
		Diagnoses: []model.Diagnosis{{Code: "I10", Type: "1"}},
		Charges:   []model.ChargeItem{{Code: "01", Amount: 500}},
		Total:     500,
	}
}

func sampleCSMBSIPD() model.IPDAdmit {
	return model.IPDAdmit{
		Patient: model.Patient{
			HN: "H3", PersonID: makeThaiID("110010100127"),
			INSCL: model.INSCL_CSMBS, PermitNo: "P1",
		},
		AN: "A100001", DateAdm: fxDate(), DateDsc: fxDate(),
		LOS: 1, Dischs: "1", Discht: "1", UUC: "1",
		Diagnoses: []model.Diagnosis{{Code: "I10", Type: "1"}},
		Charges:   []model.ChargeItem{{Code: "01", Amount: 1500}},
		DRG:       &model.DRGInfo{Code: "05130", Version: "6.3", RW: 1.23},
		Total:     1500,
	}
}

// ── UCS: 16-file ──

func TestPipeline_UCS_DryRun(t *testing.T) {
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_UCS, extractor.Result{
		OPD: []model.OPDVisit{sampleUCSVisit()},
	})
	out, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_UCS,
		DryRun: true, Extr: extr,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(out.Submissions) != 1 {
		t.Fatalf("want 1 submission, got %d", len(out.Submissions))
	}
	s := out.Submissions[0]
	if s.Format != model.Format16Files {
		t.Errorf("want 16FILES, got %s", s.Format)
	}
	if len(s.Files) == 0 || len(s.ZipBytes) == 0 {
		t.Errorf("expected files and zip; got files=%d zip=%d", len(s.Files), len(s.ZipBytes))
	}
	if s.TxnID != "" {
		t.Error("dry-run should not submit")
	}
}

func TestPipeline_UCS_Submit(t *testing.T) {
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_UCS, extractor.Result{
		OPD: []model.OPDVisit{sampleUCSVisit()},
	})
	fdh := &fakeFDH{}
	out, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_UCS,
		Extr: extr, FDH: fdh,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if fdh.calls16 != 1 {
		t.Errorf("want 1 Send16Files call, got %d", fdh.calls16)
	}
	if len(out.Submissions) != 1 || out.Submissions[0].TxnID == "" {
		t.Errorf("want submitted TxnID, got %+v", out.Submissions)
	}
}

// ── CSMBS: OPD → CSOP, IPD → CIPN ──

func TestPipeline_CSMBS_BothFormats(t *testing.T) {
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_CSMBS, extractor.Result{
		OPD: []model.OPDVisit{sampleCSMBSOPD()},
		IPD: []model.IPDAdmit{sampleCSMBSIPD()},
	})
	fdh := &fakeFDH{}
	out, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_CSMBS,
		Extr: extr, FDH: fdh,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(out.Submissions) != 2 {
		t.Fatalf("want 2 submissions (CSOP + CIPN), got %d", len(out.Submissions))
	}
	formats := map[model.ClaimFormat]bool{}
	for _, s := range out.Submissions {
		formats[s.Format] = true
		if len(s.XML) == 0 {
			t.Errorf("%s: expected XML bytes", s.Format)
		}
		if len(s.ZipBytes) == 0 {
			t.Errorf("%s: expected zip bytes", s.Format)
		}
	}
	if !formats[model.FormatCIPN] || !formats[model.FormatCSOP] {
		t.Errorf("expected both CIPN and CSOP, got %v", formats)
	}
	if fdh.callsCIPN != 1 || fdh.callsCSOP != 1 {
		t.Errorf("expected CIPN=1 CSOP=1, got CIPN=%d CSOP=%d", fdh.callsCIPN, fdh.callsCSOP)
	}
}

func TestPipeline_CSMBS_OPDOnly(t *testing.T) {
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_CSMBS, extractor.Result{
		OPD: []model.OPDVisit{sampleCSMBSOPD()},
	})
	out, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_CSMBS,
		DryRun: true, Extr: extr,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(out.Submissions) != 1 || out.Submissions[0].Format != model.FormatCSOP {
		t.Errorf("want only CSOP when no IPD data, got %+v", out.Submissions)
	}
}

func TestPipeline_CSMBS_BlocksSubmitOnValidationError(t *testing.T) {
	// CSMBS IPD without Charges or DRG → validator errors
	bad := sampleCSMBSIPD()
	bad.Charges = nil
	bad.DRG = nil
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_CSMBS, extractor.Result{
		IPD: []model.IPDAdmit{bad},
	})
	fdh := &fakeFDH{}
	_, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_CSMBS,
		Extr: extr, FDH: fdh,
	})
	if err == nil {
		t.Fatal("expected validation error to block submit")
	}
	if fdh.callsCIPN != 0 {
		t.Error("FDH should not be called when validation fails")
	}
}

// ── SSO: OPD → SSOP, IPD → AIPN via CHI ──

func sampleSSOOPD() model.OPDVisit {
	return model.OPDVisit{
		Patient: model.Patient{
			HN: "H4", PersonID: makeThaiID("110010100127"),
			INSCL: model.INSCL_SSS,
		},
		SEQ: "S4", DateOPD: fxDate(), UUC: "1",
		Diagnoses: []model.Diagnosis{{Code: "J00", Type: "1", DoctorID: "DR-SSO-1"}},
		Total:     300,
	}
}

func sampleSSOIPD() model.IPDAdmit {
	return model.IPDAdmit{
		Patient: model.Patient{
			HN: "H5", PersonID: makeThaiID("110010100127"),
			INSCL: model.INSCL_SSS,
		},
		AN: "A200001", DateAdm: fxDate(), DateDsc: fxDate(),
		LOS: 2, Dischs: "1", Discht: "1", UUC: "1",
		Diagnoses: []model.Diagnosis{{Code: "J00", Type: "1", DoctorID: "DR-SSO-2"}},
		Total:     2000,
	}
}

func TestPipeline_SSS_SubmitBoth(t *testing.T) {
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_SSS, extractor.Result{
		OPD: []model.OPDVisit{sampleSSOOPD()},
		IPD: []model.IPDAdmit{sampleSSOIPD()},
	})
	chi := &fakeCHI{}
	out, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_SSS,
		Extr: extr, CHI: chi,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if chi.callsAIPN != 1 || chi.callsSSOP != 1 {
		t.Errorf("want AIPN=1 SSOP=1, got AIPN=%d SSOP=%d", chi.callsAIPN, chi.callsSSOP)
	}
	formats := map[model.ClaimFormat]string{}
	for _, s := range out.Submissions {
		formats[s.Format] = s.TxnID
	}
	if formats[model.FormatAIPN] == "" || formats[model.FormatSSOP] == "" {
		t.Errorf("expected TxnIDs for both AIPN and SSOP, got %+v", formats)
	}
}

func TestPipeline_SSS_NoCHIConfigured(t *testing.T) {
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_SSS, extractor.Result{
		OPD: []model.OPDVisit{sampleSSOOPD()},
	})
	// FDH set but CHI missing → per-submission error, no exception
	out, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_SSS,
		Extr: extr, FDH: &fakeFDH{},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(out.Submissions) != 1 || out.Submissions[0].Err == nil {
		t.Errorf("want per-submission error; got %+v", out.Submissions)
	}
}

// ── OFC: agency override ──

func TestPipeline_OFC_AgencyFromPatient(t *testing.T) {
	v := sampleCSMBSOPD()
	v.Patient.INSCL = model.INSCL_OFC
	v.Patient.AgencyCode = "NBTC"
	extr := extractor.NewMemoryExtractor()
	extr.Put("202504", model.INSCL_OFC, extractor.Result{OPD: []model.OPDVisit{v}})

	out, err := pipeline.Run(context.Background(), pipeline.Options{
		HCode: "12345", Period: "202504", INSCL: model.INSCL_OFC,
		DryRun: true, Extr: extr,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(out.Submissions) != 1 {
		t.Fatalf("want 1 submission, got %d", len(out.Submissions))
	}
	// agency should end up in the XML header
	xml := string(out.Submissions[0].XML)
	if !contains(xml, "<AGENCY>NBTC</AGENCY>") {
		t.Errorf("expected AGENCY=NBTC in XML, got: %s", xml)
	}
}

// ── helper ──

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
