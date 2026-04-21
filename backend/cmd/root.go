package cmd

import (
	"flag"
	"fmt"
	"os"
)

// Execute entry point สำหรับ CLI
// ใช้ flag stdlib — เพิ่ม cobra หลัง go mod tidy บนเครื่องจริง
func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}
	switch os.Args[1] {
	case "submit":
		runSubmit(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "server":
		runServer(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("NexClaim — Every claim, every fund — connected.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  nexclaim submit  --inscl <INSCL> --period <YYYYMM>")
	fmt.Println("  nexclaim status  --txn-id <id>")
	fmt.Println("  nexclaim server")
}

func runSubmit(args []string) {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	inscl := fs.String("inscl", "", "รหัสสิทธิ เช่น UCS, 011, SSS")
	period := fs.String("period", "", "รอบส่ง YYYYMM เช่น 202504")
	dryRun := fs.Bool("dry-run", false, "ทดสอบโดยไม่ส่งจริง")
	_ = fs.Parse(args)
	if *inscl == "" || *period == "" {
		fmt.Fprintln(os.Stderr, "ต้องระบุ --inscl และ --period")
		os.Exit(1)
	}
	fmt.Printf("[NexClaim] submit INSCL=%s PERIOD=%s dry-run=%v\n", *inscl, *period, *dryRun)
	// TODO: เรียก pipeline
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	txnID := fs.String("txn-id", "", "Transaction ID จาก FDH")
	_ = fs.Parse(args)
	if *txnID == "" {
		fmt.Fprintln(os.Stderr, "ต้องระบุ --txn-id")
		os.Exit(1)
	}
	fmt.Printf("[NexClaim] checking status txnId=%s\n", *txnID)
	// TODO: query FDH
}

func runServer(args []string) {
	fmt.Println("[NexClaim] starting HTTP server on :8080")
	// TODO: start gin server
}
