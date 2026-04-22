package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/nexclaim/nexclaim/internal/config"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/store"
)

// runSeed dispatches `nexclaim seed ...` subcommands.
func runSeed(args []string) {
	if len(args) == 0 {
		printSeedUsage()
		os.Exit(0)
	}
	switch args[0] {
	case "master":
		runSeedMaster(args[1:])
	case "-h", "--help", "help":
		printSeedUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown seed subcommand: %s\n", args[0])
		printSeedUsage()
		os.Exit(1)
	}
}

func printSeedUsage() {
	fmt.Println("Usage:")
	fmt.Println("  nexclaim seed master [--dir data/]")
	fmt.Println()
	fmt.Println("Loads icd10.json, icd9cm.json, tmt.json into m_icd10 / m_icd9cm / m_tmt_drug.")
	fmt.Println("Falls back to *.sample.json if the full master file is absent.")
}

func runSeedMaster(args []string) {
	fs := flag.NewFlagSet("master", flag.ExitOnError)
	dir := fs.String("dir", "data", "path to data directory containing master JSON files")
	_ = fs.Parse(args)

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if cfg.DBUser == "" {
		fmt.Fprintln(os.Stderr, "seed master requires DB_USER (and other DB_* env vars)")
		os.Exit(1)
	}

	conn, err := db.Open(cfg.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	res, err := store.LoadMaster(ctx, conn, *dir)
	for _, w := range res.Warnings {
		fmt.Fprintf(os.Stderr, "warn: %s\n", w)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed master: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[seed master] m_icd10=%d m_icd9cm=%d m_tmt_drug=%d\n",
		res.ICD10, res.ICD9CM, res.TMT)
}
