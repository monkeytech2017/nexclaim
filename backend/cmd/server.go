package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/nexclaim/nexclaim/internal/audit"
	"github.com/nexclaim/nexclaim/internal/auth"
	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/config"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/ipdimport"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
	"github.com/nexclaim/nexclaim/internal/validator"
	"github.com/nexclaim/nexclaim/internal/watcher"
)

func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	addr := fs.String("addr", "", "listen address (default: :$PORT จาก .env หรือ :8080)")
	_ = fs.Parse(args)

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	hcode := cfg.HospitalHCode
	if hcode == "" {
		hcode = cfg.FDHHCode
	}

	fdh := sender.NewFDHClient(cfg.FDHBaseURL, cfg.FDHUsername, cfg.FDHPassword, hcode)
	chi := sender.NewCHIClient(cfg.CHIBaseURL, cfg.CHIUsername, cfg.CHIPassword).WithHCode(hcode)

	// Extractor: เริ่มต้นเป็น in-memory (empty). ต่อ DB-backed extractor
	// ใน step ถัดไปเมื่อ his_field_map migration พร้อม.
	extr := extractor.NewMemoryExtractor()

	// OPD 2-Way: HIS client + batch store
	hisURL := os.Getenv("HIS_API_BASE_URL")
	hisToken := os.Getenv("HIS_API_TOKEN")
	var hisCli *hisclient.Client
	if hisURL != "" {
		opts := []hisclient.Option{}
		if hisToken != "" {
			opts = append(opts, hisclient.WithBearerToken(hisToken))
		}
		hisCli = hisclient.New(hisURL, opts...)
	}
	// Batch store + ClaimRepo + HospitalRepo: DB-backed ถ้า config มี DB_USER,
	// ไม่เช่นนั้น batch = in-memory, claim = noop, hospital = nil (503).
	var batches batch.Store = batch.NewMemory()
	var claimRepo store.ClaimRepo = store.NoopClaimRepo{}
	var hospitalRepo store.HospitalRepo
	var doctorRepo store.DoctorRepo
	var insclMapRepo store.InsclMapRepo
	var drugMapRepo store.DrugMapRepo
	var doctorMapRepo store.DoctorMapRepo
	var icdMapRepo store.IcdMapRepo
	var fieldMapRepo store.FieldMapRepo
	var ccodeRepo store.CCodeRepo
	var claimBatchRepo store.ClaimBatchRepo
	var sendLogRepo store.SendLogRepo
	var dashboardRepo store.DashboardRepo
	var repIngester *store.REPIngester
	var authRepo auth.Repo
	var auditRepo store.AuditRepo
	var auditWriter audit.Writer = audit.NoopWriter{}
	var master validator.MasterValidator = validator.NoopMaster{}
	if cfg.DBUser != "" {
		if pg, err := db.Open(cfg.DSN()); err != nil {
			fmt.Fprintf(os.Stderr, "[NexClaim] DB connect failed, using in-memory store: %v\n", err)
		} else {
			batches = batch.NewPostgres(pg)
			claimRepo = store.NewPg(pg)
			hospitalRepo = store.NewPgHospitalRepo(pg)
			doctorRepo = store.NewPgDoctorRepo(pg)
			insclMapRepo = store.NewPgInsclMapRepo(pg)
			drugMapRepo = store.NewPgDrugMapRepo(pg)
			doctorMapRepo = store.NewPgDoctorMapRepo(pg)
			icdMapRepo = store.NewPgIcdMapRepo(pg)
			fieldMapRepo = store.NewPgFieldMapRepo(pg)
			ccodeRepo = store.NewPgCCodeRepo(pg)
			claimBatchRepo = store.NewPgClaimBatchRepo(pg)
			sendLogRepo = store.NewPgSendLogRepo(pg)
			dashboardRepo = store.NewPgDashboardRepo(pg)
			repIngester = store.NewREPIngester(pg, ccodeRepo, fdh)
			authRepo = store.NewPgAPIKeyRepo(pg)
			pgAudit := store.NewPgAuditRepo(pg)
			auditRepo = pgAudit
			auditWriter = pgAudit

			if mv, counts, err := validator.LoadFromDB(context.Background(), pg); err != nil {
				fmt.Fprintf(os.Stderr, "[NexClaim] master validator load failed, falling back to noop: %v\n", err)
			} else {
				master = mv
				fmt.Printf("[NexClaim] master validator loaded (%s)\n", counts)
			}
			fmt.Printf("[NexClaim] postgres store wired (%s/%s)\n", cfg.DBHost, cfg.DBName)
		}
	}

	ipdShareRoot := os.Getenv("IPD_SHARE_ROOT") // เช่น /shared/nexclaim/ipd

	authEnabled := os.Getenv("AUTH_ENABLED") == "true"
	if authEnabled && authRepo == nil {
		fmt.Fprintln(os.Stderr, "[NexClaim] AUTH_ENABLED=true but no DB — auth middleware will pass through")
	}

	// RATE_LIMIT_PER_MIN: 0 = disabled, default 60. Non-numeric → warn + default.
	rateLimitPerMin := 60
	if raw := os.Getenv("RATE_LIMIT_PER_MIN"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			rateLimitPerMin = n
		} else {
			fmt.Fprintf(os.Stderr, "[NexClaim] RATE_LIMIT_PER_MIN=%q invalid, using default 60\n", raw)
		}
	}

	engine := server.New(server.Deps{
		HCode:        hcode,
		Extractor:    extr,
		FDH:          fdh,
		CHI:          chi,
		HISClient:    hisCli,
		Batches:      batches,
		IPDShareRoot: ipdShareRoot,
		ClaimRepo:    claimRepo,
		AuthRepo:        authRepo,
		AuthEnabled:     authEnabled,
		RateLimitPerMin: rateLimitPerMin,
		HospitalRepo:  hospitalRepo,
		DoctorRepo:    doctorRepo,
		InsclMapRepo:  insclMapRepo,
		DrugMapRepo:   drugMapRepo,
		DoctorMapRepo: doctorMapRepo,
		IcdMapRepo:    icdMapRepo,
		FieldMapRepo:   fieldMapRepo,
		CCodeRepo:      ccodeRepo,
		ClaimBatchRepo: claimBatchRepo,
		SendLogRepo:    sendLogRepo,
		DashboardRepo:  dashboardRepo,
		REPIngester:    repIngester,
		AuditRepo:      auditRepo,
		AuditWriter:    auditWriter,
		Master:         master,
		StatusLookup:  fdh,
	})

	// IPD auto-watcher: start if IPD_WATCH_INTERVAL is set (e.g. "60s", "5m").
	// Zero / unset = disabled — admin still triggers imports manually.
	if raw := os.Getenv("IPD_WATCH_INTERVAL"); raw != "" && ipdShareRoot != "" {
		if interval, err := time.ParseDuration(raw); err != nil {
			fmt.Fprintf(os.Stderr, "[NexClaim] IPD_WATCH_INTERVAL %q invalid: %v\n", raw, err)
		} else if interval > 0 {
			proc := &ipdimport.Processor{
				Root:      ipdShareRoot,
				FDH:       fdh,
				CHI:       chi,
				ClaimRepo: claimRepo,
				Master:    master,
			}
			go (&watcher.IPD{Proc: proc, Interval: interval}).Run(context.Background())
		}
	}

	listen := *addr
	if listen == "" {
		listen = ":" + cfg.Port
	}
	srv := &http.Server{
		Addr:              listen,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Printf("[NexClaim] HTTP server listening on %s (hcode=%s)\n", listen, hcode)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}
