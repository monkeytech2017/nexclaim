package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/nexclaim/nexclaim/internal/config"
	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/model"
	"github.com/nexclaim/nexclaim/internal/pipeline"
	"github.com/nexclaim/nexclaim/internal/sender"
)

func runSubmit(args []string) {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	insclArg := fs.String("inscl", "", "รหัสสิทธิ เช่น UCS, 011, SSS")
	period := fs.String("period", "", "รอบส่ง YYYYMM เช่น 202504")
	dryRun := fs.Bool("dry-run", false, "ทดสอบโดยไม่ส่งจริง (validate + generate + สรุป)")
	hcodeArg := fs.String("hcode", "", "รหัส รพ. (override HOSPITAL_HCODE จาก .env)")
	agencyArg := fs.String("agency", "", "Agency code สำหรับ OFC เช่น NBTC/BAAC/ECT/PEA/MEA/MWA/SRT")
	_ = fs.Parse(args)

	if *insclArg == "" || *period == "" {
		fmt.Fprintln(os.Stderr, "ต้องระบุ --inscl และ --period")
		os.Exit(2)
	}

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil && !*dryRun {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	hcode := *hcodeArg
	if hcode == "" && cfg != nil {
		hcode = cfg.HospitalHCode
		if hcode == "" {
			hcode = cfg.FDHHCode
		}
	}
	if hcode == "" {
		hcode = "00000" // dry-run fallback without config
	}

	extr := extractor.NewMemoryExtractor()

	opt := pipeline.Options{
		HCode:  hcode,
		Period: *period,
		INSCL:  model.INSCL(*insclArg),
		Agency: model.Agency(*agencyArg),
		DryRun: *dryRun,
		Extr:   extr,
	}
	if !*dryRun && cfg != nil {
		opt.FDH = sender.NewFDHClient(cfg.FDHBaseURL, cfg.FDHUsername, cfg.FDHPassword, hcode)
		opt.CHI = sender.NewCHIClient(cfg.CHIBaseURL, cfg.CHIUsername, cfg.CHIPassword).WithHCode(hcode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	out, err := pipeline.Run(ctx, opt)
	printOutcome(out, err, *dryRun)
	if err != nil {
		os.Exit(1)
	}
}

func printOutcome(out *pipeline.Outcome, err error, dryRun bool) {
	if out == nil {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return
	}
	fmt.Printf("[NexClaim] INSCL=%s records: OPD=%d IPD=%d\n", out.INSCL, out.OPDCount, out.IPDCount)

	if len(out.ValidationErrors) > 0 {
		fmt.Printf("[NexClaim] validation: %d issue(s)\n", len(out.ValidationErrors))
		for i, ve := range out.ValidationErrors {
			if i >= 10 {
				fmt.Printf("  ... (+%d more)\n", len(out.ValidationErrors)-10)
				break
			}
			fmt.Printf("  - %s\n", ve.Error())
		}
	}

	if len(out.Submissions) == 0 {
		fmt.Println("[NexClaim] no data to submit")
	}

	for _, s := range out.Submissions {
		fmt.Printf("[NexClaim] submission → format=%s\n", s.Format)
		if len(s.Files) > 0 {
			fmt.Printf("  files: %d\n", len(s.Files))
			for _, name := range []string{
				"INS.txt", "PAT.txt", "OPD.txt", "ORF.txt", "ODX.txt", "OOP.txt",
				"IPD.txt", "IRF.txt", "IDX.txt", "IOP.txt",
				"CHT.txt", "CHA.txt", "AER.txt", "ADP.txt", "LVD.txt", "DRU.txt",
			} {
				if b, ok := s.Files[name]; ok {
					fmt.Printf("    %s (%d bytes)\n", name, len(b))
				}
			}
		}
		if len(s.XML) > 0 {
			fmt.Printf("  xml: %d bytes\n", len(s.XML))
		}
		if len(s.ZipBytes) > 0 {
			fmt.Printf("  zip: %s (%d bytes)\n", s.ZipName, len(s.ZipBytes))
		}
		switch {
		case s.Err != nil:
			fmt.Fprintf(os.Stderr, "  error: %v\n", s.Err)
		case s.TxnID != "":
			fmt.Printf("  submitted: txnId=%s status=%s %s\n", s.TxnID, s.Status, s.Message)
		}
	}

	if dryRun && len(out.Submissions) > 0 {
		fmt.Println("[NexClaim] dry-run → no submission")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}
