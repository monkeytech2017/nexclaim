package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"

	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/config"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/store"
)

const authCLITimeout = 10 * time.Second

// runAuth dispatches the `nexclaim auth ...` subcommand tree:
//
//	nexclaim auth create-admin    --name "ops-laptop"
//	nexclaim auth create-hospital --hcode 12345 --name "rpt-BKK-10700"
//	nexclaim auth list
//
// A key is printed ONCE to stdout on creation; the hash is stored in the DB.
// Never again — rotate by creating a new key and deactivating the old row.
func runAuth(args []string) {
	if len(args) == 0 {
		printAuthUsage()
		os.Exit(0)
	}
	switch args[0] {
	case "create-admin":
		runAuthCreateAdmin(args[1:])
	case "create-hospital":
		runAuthCreateHospital(args[1:])
	case "list":
		runAuthList(args[1:])
	case "-h", "--help", "help":
		printAuthUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown auth subcommand: %s\n", args[0])
		printAuthUsage()
		os.Exit(1)
	}
}

func printAuthUsage() {
	fmt.Println("Usage:")
	fmt.Println("  nexclaim auth create-admin    --name <label>")
	fmt.Println("  nexclaim auth create-hospital --hcode <5 digits> --name <label>")
	fmt.Println("  nexclaim auth list")
	fmt.Println()
	fmt.Println("The raw key is printed ONCE on creation. Save it immediately.")
}

// openAuthDB is a small helper shared by all auth subcommands. It loads .env,
// validates config, opens the DB, and returns a *sqlx.DB the caller must
// close. Fatal-exits on any setup failure — auth CLI without a DB is useless.
func openAuthDB() *sqlx.DB {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if cfg.DBUser == "" {
		fmt.Fprintln(os.Stderr, "auth: DB_USER not configured — auth CLI requires a database")
		os.Exit(1)
	}
	conn, err := db.Open(cfg.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "db open: %v\n", err)
		os.Exit(1)
	}
	return conn
}

func runAuthCreateAdmin(args []string) {
	fs := flag.NewFlagSet("create-admin", flag.ExitOnError)
	name := fs.String("name", "", "human label for this key (e.g. 'ops-laptop-2026')")
	_ = fs.Parse(args)
	if *name == "" {
		fmt.Fprintln(os.Stderr, "ต้องระบุ --name")
		os.Exit(2)
	}

	conn := openAuthDB()
	defer conn.Close()
	repo := newAuthRepoPg(conn)

	raw, hash, err := auth.GenerateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), authCLITimeout)
	defer cancel()
	ident, err := repo.Insert(ctx, auth.Insert{
		KeyHash: hash, Role: auth.RoleAdmin, Name: *name,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert: %v\n", err)
		os.Exit(1)
	}
	printNewKey(ident, raw)
}

func runAuthCreateHospital(args []string) {
	fs := flag.NewFlagSet("create-hospital", flag.ExitOnError)
	hcode := fs.String("hcode", "", "5-digit hospital code (must exist in m_hospital)")
	name := fs.String("name", "", "human label")
	_ = fs.Parse(args)
	if *hcode == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "ต้องระบุ --hcode และ --name")
		os.Exit(2)
	}
	if len(*hcode) != 5 {
		fmt.Fprintln(os.Stderr, "hcode ต้อง 5 หลัก")
		os.Exit(2)
	}

	conn := openAuthDB()
	defer conn.Close()
	repo := newAuthRepoPg(conn)

	raw, hash, err := auth.GenerateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), authCLITimeout)
	defer cancel()
	ident, err := repo.Insert(ctx, auth.Insert{
		KeyHash: hash, Role: auth.RoleHospital, HCode: *hcode, Name: *name,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert: %v\n", err)
		os.Exit(1)
	}
	printNewKey(ident, raw)
}

func runAuthList(_ []string) {
	conn := openAuthDB()
	defer conn.Close()

	// Query directly — listing is a CLI-only operation, we don't expose it
	// as a Repo method (avoids bloating the auth.Repo interface for one
	// debug tool).
	type row struct {
		ID         string  `db:"id"`
		Name       string  `db:"name"`
		Role       string  `db:"role"`
		HCode      string  `db:"hcode"`
		IsActive   bool    `db:"is_active"`
		LastUsedAt *string `db:"last_used_at"`
		CreatedAt  string  `db:"created_at"`
	}
	var rows []row
	err := conn.Select(&rows, `
		SELECT id::text          AS id,
		       name, role,
		       COALESCE(hcode,'') AS hcode,
		       is_active,
		       to_char(last_used_at, 'YYYY-MM-DD HH24:MI') AS last_used_at,
		       to_char(created_at,   'YYYY-MM-DD HH24:MI') AS created_at
		FROM api_key
		ORDER BY created_at DESC
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		fmt.Println("(no api keys yet — create one with `nexclaim auth create-admin` or bootstrap via HTTP)")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tROLE\tHCODE\tACTIVE\tLAST_USED\tCREATED")
	for _, r := range rows {
		last := "-"
		if r.LastUsedAt != nil && *r.LastUsedAt != "" {
			last = *r.LastUsedAt
		}
		active := "no"
		if r.IsActive {
			active = "yes"
		}
		hcode := r.HCode
		if hcode == "" {
			hcode = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ID, r.Name, r.Role, hcode, active, last, r.CreatedAt)
	}
	_ = w.Flush()
}

// ── helpers ──

func newAuthRepoPg(conn *sqlx.DB) auth.Repo {
	return store.NewPgAPIKeyRepo(conn)
}

func printNewKey(ident *auth.Identity, raw string) {
	fmt.Println("── NEW API KEY ────────────────────────────────────────")
	fmt.Printf("id:    %s\n", ident.ID)
	fmt.Printf("name:  %s\n", ident.Name)
	fmt.Printf("role:  %s\n", ident.Role)
	if ident.HCode != "" {
		fmt.Printf("hcode: %s\n", ident.HCode)
	}
	fmt.Println()
	fmt.Println("KEY (save this NOW — it will never be shown again):")
	fmt.Println()
	fmt.Printf("  %s\n", raw)
	fmt.Println()
	fmt.Println("Use as:  Authorization: Bearer", raw)
	fmt.Println("────────────────────────────────────────────────────────")
}
