// Package pipeline wires extractor → validator → generator → sender
// สำหรับ one (INSCL, period) submission.
//
// สำหรับบาง INSCL (CSMBS/LGO/OFC, SSS/SS4) record ประเภท OPD กับ IPD route
// ไปต่างรูปแบบ — pipeline จะสร้าง Submission แยกแต่ละรูปแบบที่มีข้อมูล.
package pipeline

import (
	"context"
	"fmt"

	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/generator/aipn"
	"github.com/nexclaim/nexclaim/internal/generator/cipn"
	"github.com/nexclaim/nexclaim/internal/generator/csop"
	"github.com/nexclaim/nexclaim/internal/generator/file16"
	"github.com/nexclaim/nexclaim/internal/generator/ssop"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/router"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/validator"
)

// FDHSubmitter = ช่องทางส่ง FDH (UC, CSMBS, LGO, OFC, TPBS, MON)
type FDHSubmitter interface {
	Send16Files(zipData []byte, period string) (*sender.SubmitResult, error)
	SendCIPN(zipData []byte, period string) (*sender.SubmitResult, error)
	SendCSOP(zipData []byte, period string) (*sender.SubmitResult, error)
}

// CHISubmitter = ช่องทางส่งประกันสังคม (SSS/SS4)
type CHISubmitter interface {
	SendAIPN(zipData []byte, period string) (*sender.SubmitResult, error)
	SendSSOP(zipData []byte, period string) (*sender.SubmitResult, error)
}

// Submitter รวม FDH + CHI — implementers อาจ embed ทั้งคู่ (หรือ one-of).
type Submitter interface {
	FDHSubmitter
	CHISubmitter
}

// Options ควบคุม pipeline.
type Options struct {
	HCode  string
	Period string       // YYYYMM
	INSCL  model.INSCL
	Agency model.Agency // override สำหรับ OFC (ถ้าไม่ระบุจะดึงจาก Patient.AgencyCode)
	DryRun bool
	Extr   extractor.Extractor
	FDH    FDHSubmitter // required when route → FDH + !DryRun
	CHI    CHISubmitter // required when route → CHI (SSS/SS4) + !DryRun
}

// Submission = หนึ่ง format output หนึ่งรอบส่ง
type Submission struct {
	Format   model.ClaimFormat
	ZipName  string
	ZipBytes []byte
	Files    map[string][]byte // populated for 16-file only
	XML      []byte            // populated for CIPN/CSOP (XML formats)
	TxnID    string
	Status   string
	Message  string
	Err      error // per-submission error; does not stop the rest of the run
}

// Outcome สรุปผลรวมของ pipeline run.
type Outcome struct {
	INSCL            model.INSCL
	OPDCount         int
	IPDCount         int
	ValidationErrors []validator.ValidationError
	Submissions      []Submission
}

// Run pipeline สำหรับ (INSCL, period).
func Run(ctx context.Context, opt Options) (*Outcome, error) {
	if opt.HCode == "" {
		return nil, fmt.Errorf("pipeline: HCode required")
	}
	if opt.Period == "" {
		return nil, fmt.Errorf("pipeline: Period required")
	}
	if opt.Extr == nil {
		return nil, fmt.Errorf("pipeline: Extractor required")
	}

	res, err := opt.Extr.Extract(ctx, extractor.Request{INSCL: opt.INSCL, Period: opt.Period})
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}

	out := &Outcome{
		INSCL:    opt.INSCL,
		OPDCount: len(res.OPD),
		IPDCount: len(res.IPD),
	}
	for _, v := range res.OPD {
		out.ValidationErrors = append(out.ValidationErrors, validator.ValidateOPD(v)...)
	}
	for _, a := range res.IPD {
		out.ValidationErrors = append(out.ValidationErrors, validator.ValidateIPD(a)...)
	}

	dispatchFormats(opt, res, out)

	if !opt.DryRun && len(out.ValidationErrors) > 0 {
		return out, fmt.Errorf("pipeline: %d validation error(s); refusing to submit (use --dry-run to inspect)", len(out.ValidationErrors))
	}

	if !opt.DryRun {
		for i := range out.Submissions {
			submitOne(opt.FDH, opt.CHI, &out.Submissions[i], opt.Period)
		}
	}

	return out, nil
}

// dispatchFormats decides ว่าจะสร้าง submission ใดบ้างจาก INSCL + data ที่มี.
func dispatchFormats(opt Options, res extractor.Result, out *Outcome) {
	// ใช้ router เป็น single source of truth — query ต่อ record type.
	opdRoute, _ := router.Route(opt.INSCL, false)
	ipdRoute, _ := router.Route(opt.INSCL, true)

	switch opdRoute.Format {
	case model.Format16Files:
		// 16-file: OPD + IPD ถูก bundle อยู่ใน ZIP เดียว
		if len(res.OPD) == 0 && len(res.IPD) == 0 {
			return
		}
		out.Submissions = append(out.Submissions, build16Files(opt, res))

	case model.FormatCSOP:
		if len(res.OPD) > 0 {
			agency := resolveAgency(opt, res.OPD[0].Patient, opdRoute)
			out.Submissions = append(out.Submissions, buildCSOP(opt, res.OPD, agency))
		}
	}

	switch ipdRoute.Format {
	case model.FormatCIPN:
		if len(res.IPD) > 0 {
			agency := resolveAgency(opt, res.IPD[0].Patient, ipdRoute)
			out.Submissions = append(out.Submissions, buildCIPN(opt, res.IPD, agency))
		}
	case model.FormatAIPN:
		if len(res.IPD) > 0 {
			out.Submissions = append(out.Submissions, buildAIPN(opt, res.IPD))
		}
	}

	// SSO OPD → SSOP (separate from CSOP path above because of the sender).
	if opdRoute.Format == model.FormatSSOP && len(res.OPD) > 0 {
		out.Submissions = append(out.Submissions, buildSSOP(opt, res.OPD))
	}
}

// resolveAgency สำหรับ OFC/LGO ที่ต้องการ agency code ที่ถูกต้องใน header.
// ลำดับ: opt.Agency → patient.AgencyCode → route.Agency (default จาก router).
func resolveAgency(opt Options, p model.Patient, r model.RouteResult) model.Agency {
	if opt.Agency != "" {
		return opt.Agency
	}
	if p.AgencyCode != "" {
		return router.AgencyFromCode(p.AgencyCode)
	}
	return r.Agency
}

// ── Per-format builders ────────────────────────────────────────

func build16Files(opt Options, res extractor.Result) Submission {
	sub := Submission{Format: model.Format16Files}
	b, err := file16.Build(file16.Input{
		HCode: opt.HCode, Period: opt.Period,
		OPD: res.OPD, IPD: res.IPD,
	})
	if err != nil {
		sub.Err = fmt.Errorf("file16 build: %w", err)
		return sub
	}
	sub.Files = b.Files
	if len(b.Files) == 0 {
		return sub
	}
	zipBytes, err := sender.BuildMultiFileZip(b.Files)
	if err != nil {
		sub.Err = fmt.Errorf("file16 zip: %w", err)
		return sub
	}
	sub.ZipBytes = zipBytes
	sub.ZipName = sender.ZipFilename(opt.HCode, string(opt.INSCL), opt.Period)
	return sub
}

func buildCIPN(opt Options, ipd []model.IPDAdmit, agency model.Agency) Submission {
	sub := Submission{Format: model.FormatCIPN}
	xmlBytes, err := cipn.Build(cipn.Input{
		HCode: opt.HCode, Agency: agency, Period: opt.Period, IPD: ipd,
	})
	if err != nil {
		sub.Err = fmt.Errorf("cipn build: %w", err)
		return sub
	}
	sub.XML = xmlBytes
	zipName := sender.ZipFilename(opt.HCode, "CIPN", opt.Period)
	xmlName := opt.HCode + "CIPN" + opt.Period + ".xml"
	zipBytes, err := sender.BuildZipWithMD5(xmlBytes, xmlName)
	if err != nil {
		sub.Err = fmt.Errorf("cipn zip: %w", err)
		return sub
	}
	sub.ZipBytes = zipBytes
	sub.ZipName = zipName
	return sub
}

func buildCSOP(opt Options, opd []model.OPDVisit, agency model.Agency) Submission {
	sub := Submission{Format: model.FormatCSOP}
	xmlBytes, err := csop.Build(csop.Input{
		HCode: opt.HCode, Agency: agency, Period: opt.Period, OPD: opd,
	})
	if err != nil {
		sub.Err = fmt.Errorf("csop build: %w", err)
		return sub
	}
	sub.XML = xmlBytes
	zipName := sender.ZipFilename(opt.HCode, "CSOP", opt.Period)
	xmlName := opt.HCode + "CSOP" + opt.Period + ".xml"
	zipBytes, err := sender.BuildZipWithMD5(xmlBytes, xmlName)
	if err != nil {
		sub.Err = fmt.Errorf("csop zip: %w", err)
		return sub
	}
	sub.ZipBytes = zipBytes
	sub.ZipName = zipName
	return sub
}

func submitOne(fdh FDHSubmitter, chi CHISubmitter, sub *Submission, period string) {
	if sub.Err != nil || len(sub.ZipBytes) == 0 {
		return
	}
	var (
		res *sender.SubmitResult
		err error
	)
	switch sub.Format {
	case model.Format16Files:
		if fdh == nil {
			sub.Err = fmt.Errorf("submit %s: FDH submitter not configured", sub.Format)
			return
		}
		res, err = fdh.Send16Files(sub.ZipBytes, period)
	case model.FormatCIPN:
		if fdh == nil {
			sub.Err = fmt.Errorf("submit %s: FDH submitter not configured", sub.Format)
			return
		}
		res, err = fdh.SendCIPN(sub.ZipBytes, period)
	case model.FormatCSOP:
		if fdh == nil {
			sub.Err = fmt.Errorf("submit %s: FDH submitter not configured", sub.Format)
			return
		}
		res, err = fdh.SendCSOP(sub.ZipBytes, period)
	case model.FormatAIPN:
		if chi == nil {
			sub.Err = fmt.Errorf("submit %s: CHI submitter not configured", sub.Format)
			return
		}
		res, err = chi.SendAIPN(sub.ZipBytes, period)
	case model.FormatSSOP:
		if chi == nil {
			sub.Err = fmt.Errorf("submit %s: CHI submitter not configured", sub.Format)
			return
		}
		res, err = chi.SendSSOP(sub.ZipBytes, period)
	default:
		sub.Err = fmt.Errorf("submit: format %s not supported", sub.Format)
		return
	}
	if err != nil {
		sub.Err = fmt.Errorf("submit %s: %w", sub.Format, err)
		return
	}
	sub.TxnID = res.TxnID
	sub.Status = res.Status
	sub.Message = res.Message
}

func buildAIPN(opt Options, ipd []model.IPDAdmit) Submission {
	sub := Submission{Format: model.FormatAIPN}
	xmlBytes, err := aipn.Build(aipn.Input{
		HCode: opt.HCode, Period: opt.Period, IPD: ipd,
	})
	if err != nil {
		sub.Err = fmt.Errorf("aipn build: %w", err)
		return sub
	}
	sub.XML = xmlBytes
	zipName := sender.ZipFilename(opt.HCode, "AIPN", opt.Period)
	xmlName := opt.HCode + "AIPN" + opt.Period + ".xml"
	zipBytes, err := sender.BuildZipWithMD5(xmlBytes, xmlName)
	if err != nil {
		sub.Err = fmt.Errorf("aipn zip: %w", err)
		return sub
	}
	sub.ZipBytes = zipBytes
	sub.ZipName = zipName
	return sub
}

func buildSSOP(opt Options, opd []model.OPDVisit) Submission {
	sub := Submission{Format: model.FormatSSOP}
	xmlBytes, err := ssop.Build(ssop.Input{
		HCode: opt.HCode, Period: opt.Period, OPD: opd,
	})
	if err != nil {
		sub.Err = fmt.Errorf("ssop build: %w", err)
		return sub
	}
	sub.XML = xmlBytes
	zipName := sender.ZipFilename(opt.HCode, "SSOP", opt.Period)
	xmlName := opt.HCode + "SSOP" + opt.Period + ".xml"
	zipBytes, err := sender.BuildZipWithMD5(xmlBytes, xmlName)
	if err != nil {
		sub.Err = fmt.Errorf("ssop zip: %w", err)
		return sub
	}
	sub.ZipBytes = zipBytes
	sub.ZipName = zipName
	return sub
}
