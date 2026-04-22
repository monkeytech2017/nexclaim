package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"github.com/nexclaim/nexclaim/internal/config"
	"github.com/nexclaim/nexclaim/internal/sender"
)

// Execute entry point สำหรับ CLI.
//
// ใช้ flag stdlib (ไม่ cobra) เพื่อให้ dependency cross-module น้อยที่สุด —
// เราจะย้ายไป cobra เมื่อมี subcommand tree ที่ซับซ้อนกว่านี้.
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
	case "migrate":
		runMigrate(os.Args[2:])
	case "seed":
		runSeed(os.Args[2:])
	case "auth":
		runAuth(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
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
	fmt.Println("  nexclaim submit  --inscl <INSCL> --period <YYYYMM> [--dry-run] [--hcode <code>] [--agency <code>]")
	fmt.Println("  nexclaim status  --txn-id <id>")
	fmt.Println("  nexclaim server  [--addr :8080]")
	fmt.Println("  nexclaim migrate status")
	fmt.Println("  nexclaim seed master [--dir data/]")
	fmt.Println("  nexclaim auth create-admin    --name <label>")
	fmt.Println("  nexclaim auth create-hospital --hcode <5digits> --name <label>")
	fmt.Println("  nexclaim auth list")
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	txnID := fs.String("txn-id", "", "Transaction ID จาก FDH")
	_ = fs.Parse(args)
	if *txnID == "" {
		fmt.Fprintln(os.Stderr, "ต้องระบุ --txn-id")
		os.Exit(2)
	}

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	c := sender.NewFDHClient(cfg.FDHBaseURL, cfg.FDHUsername, cfg.FDHPassword, cfg.FDHHCode)
	res, err := c.GetStatus(*txnID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("txnId=%s status=%s msg=%s\n", res.TxnID, res.Status, res.Message)
}

