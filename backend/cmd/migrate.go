package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/nexclaim/nexclaim/internal/config"
	"github.com/nexclaim/nexclaim/internal/db"
)

// runMigrate handles:
//
//	nexclaim migrate status   — list migrations + applied/missing against current DB
//
// Apply (run SQL) ใช้ scripts/init_db.sh — การ apply ผ่าน psql เป็นวิธีที่
// ชัดเจนกว่าและ support เรื่อง locale ใน script มากกว่าใน binary.
func runMigrate(args []string) {
	if len(args) == 0 {
		printMigrateUsage()
		os.Exit(0)
	}
	switch args[0] {
	case "status":
		runMigrateStatus(args[1:])
	case "-h", "--help", "help":
		printMigrateUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown migrate subcommand: %s\n", args[0])
		printMigrateUsage()
		os.Exit(1)
	}
}

func printMigrateUsage() {
	fmt.Println("Usage:")
	fmt.Println("  nexclaim migrate status [--dir PATH]")
	fmt.Println()
	fmt.Println("Apply migrations: scripts/init_db.sh (applies 000..004 in order)")
}

func runMigrateStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dir := fs.String("dir", "migrations", "path to migrations directory")
	_ = fs.Parse(args)

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve dir: %v\n", err)
		os.Exit(1)
	}

	ms, err := db.ScanMigrations(absDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	_ = godotenv.Load()
	cfg, cfgErr := config.Load()
	if cfgErr != nil || cfg.DBUser == "" {
		fmt.Println("# no DB credentials — listing migrations without probing applied state")
		for _, m := range ms {
			fmt.Printf("  ?  %s\n", m.Name)
		}
		return
	}

	conn, err := db.Open(cfg.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	ms, err = db.DetectApplied(conn, ms)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("# DB: %s/%s\n", cfg.DBHost, cfg.DBName)
	for _, m := range ms {
		mark := "?"
		switch {
		case m.Name == "000_create_database.sql":
			mark = "—" // N/A — DB exists, so this obviously ran
		case m.Applied:
			mark = "✓"
		default:
			mark = "✗"
		}
		fmt.Printf("  %s  %s\n", mark, m.Name)
	}
}
