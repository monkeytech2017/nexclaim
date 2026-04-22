package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/nexclaim/nexclaim/internal/batch"
	"github.com/nexclaim/nexclaim/internal/config"
	"github.com/nexclaim/nexclaim/internal/db"
	"github.com/nexclaim/nexclaim/internal/extractor"
	"github.com/nexclaim/nexclaim/internal/hisclient"
	"github.com/nexclaim/nexclaim/internal/sender"
	"github.com/nexclaim/nexclaim/internal/server"
	"github.com/nexclaim/nexclaim/internal/store"
	"github.com/nexclaim/nexclaim/internal/validator"
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
	var repIngester *store.REPIngester
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
			repIngester = store.NewREPIngester(pg, ccodeRepo, fdh)

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

	engine := server.New(server.Deps{
		HCode:        hcode,
		Extractor:    extr,
		FDH:          fdh,
		CHI:          chi,
		HISClient:    hisCli,
		Batches:      batches,
		IPDShareRoot: ipdShareRoot,
		ClaimRepo:    claimRepo,
		HospitalRepo:  hospitalRepo,
		DoctorRepo:    doctorRepo,
		InsclMapRepo:  insclMapRepo,
		DrugMapRepo:   drugMapRepo,
		DoctorMapRepo: doctorMapRepo,
		IcdMapRepo:    icdMapRepo,
		FieldMapRepo:  fieldMapRepo,
		CCodeRepo:     ccodeRepo,
		REPIngester:   repIngester,
		Master:        master,
		StatusLookup:  fdh,
	})

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
