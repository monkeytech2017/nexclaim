package batch

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/nexclaim/nexclaim/internal/hisclient"
)

// PgStore persists ingest batches in Postgres (tables: opd_ingest_batch,
// opd_ingest_visit — see migrations/004_ingest_batch.sql).
type PgStore struct {
	db *sqlx.DB
}

// NewPostgres wraps a *sqlx.DB; caller owns lifecycle (db.Close()).
func NewPostgres(db *sqlx.DB) *PgStore { return &PgStore{db: db} }

func (s *PgStore) Put(req hisclient.VisitListRequest) *Batch {
	b := newBatch(req)

	tx, err := s.db.Beginx()
	if err != nil {
		b.LastError = "tx begin: " + err.Error()
		return b
	}
	defer func() { _ = tx.Rollback() }() // noop if Commit succeeded

	_, err = tx.Exec(`
		INSERT INTO opd_ingest_batch
		  (batch_id, hcode, period, exported_by, state, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,''), $5, $6, $7)
	`, b.ID, b.HospitalCode, b.Period, b.ExportedBy, string(b.State), b.CreatedAt, b.UpdatedAt)
	if err != nil {
		b.LastError = "insert batch: " + err.Error()
		return b
	}

	if len(b.Visits) > 0 {
		stmt, err := tx.Preparex(`
			INSERT INTO opd_ingest_visit
			  (batch_id, vn, hn, pid, patient_name, visit_date, visit_time,
			   inscl, inscl_name, clinic_code, clinic_name, doctor_code,
			   total_charge, ordinal)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		`)
		if err != nil {
			b.LastError = "prepare visit: " + err.Error()
			return b
		}
		for i, v := range b.Visits {
			if _, err := stmt.Exec(
				b.ID, v.VN, nullable(v.HN), nullable(v.PID),
				nullable(v.PatientName), nullable(v.VisitDate), nullable(v.VisitTime),
				nullable(v.INSCL), nullable(v.INSCLName),
				nullable(v.ClinicCode), nullable(v.ClinicName),
				nullable(v.DoctorCode),
				nullableFloat(v.TotalCharge), i,
			); err != nil {
				b.LastError = fmt.Sprintf("insert visit %s: %v", v.VN, err)
				_ = stmt.Close()
				return b
			}
		}
		_ = stmt.Close()
	}

	if err := tx.Commit(); err != nil {
		b.LastError = "tx commit: " + err.Error()
		return b
	}
	return b
}

func (s *PgStore) Get(id string) (*Batch, bool) {
	var row batchRow
	err := s.db.Get(&row, `
		SELECT batch_id, hcode, period, COALESCE(exported_by,'') AS exported_by,
		       state, created_at, updated_at, COALESCE(last_error,'') AS last_error
		FROM opd_ingest_batch WHERE batch_id = $1
	`, id)
	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	b := row.toBatch()
	visits, err := s.visitsFor(id)
	if err == nil {
		b.Visits = visits
	}
	return b, true
}

func (s *PgStore) SetState(id string, state State, errMsg string) error {
	res, err := s.db.Exec(`
		UPDATE opd_ingest_batch
		SET state = $1, last_error = NULLIF($2,''), updated_at = $3
		WHERE batch_id = $4
	`, string(state), errMsg, time.Now(), id)
	if err != nil {
		return fmt.Errorf("update state: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("batch %s not found", id)
	}
	return nil
}

func (s *PgStore) List() []*Batch {
	var rows []batchRow
	if err := s.db.Select(&rows, `
		SELECT batch_id, hcode, period, COALESCE(exported_by,'') AS exported_by,
		       state, created_at, updated_at, COALESCE(last_error,'') AS last_error
		FROM opd_ingest_batch ORDER BY created_at DESC
	`); err != nil {
		return nil
	}
	out := make([]*Batch, 0, len(rows))
	for _, r := range rows {
		b := r.toBatch()
		if v, err := s.visitsFor(b.ID); err == nil {
			b.Visits = v
		}
		out = append(out, b)
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────

type batchRow struct {
	BatchID    string    `db:"batch_id"`
	HCode      string    `db:"hcode"`
	Period     string    `db:"period"`
	ExportedBy string    `db:"exported_by"`
	State      string    `db:"state"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
	LastError  string    `db:"last_error"`
}

func (r batchRow) toBatch() *Batch {
	return &Batch{
		ID: r.BatchID, HospitalCode: r.HCode, Period: r.Period,
		ExportedBy: r.ExportedBy, State: State(r.State),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, LastError: r.LastError,
	}
}

type visitRow struct {
	VN          string          `db:"vn"`
	HN          sql.NullString  `db:"hn"`
	PID         sql.NullString  `db:"pid"`
	PatientName sql.NullString  `db:"patient_name"`
	VisitDate   sql.NullString  `db:"visit_date"`
	VisitTime   sql.NullString  `db:"visit_time"`
	INSCL       sql.NullString  `db:"inscl"`
	INSCLName   sql.NullString  `db:"inscl_name"`
	ClinicCode  sql.NullString  `db:"clinic_code"`
	ClinicName  sql.NullString  `db:"clinic_name"`
	DoctorCode  sql.NullString  `db:"doctor_code"`
	TotalCharge sql.NullFloat64 `db:"total_charge"`
	Ordinal     int             `db:"ordinal"`
}

func (r visitRow) toVisit() hisclient.VisitSummary {
	return hisclient.VisitSummary{
		VN: r.VN,
		HN: r.HN.String, PID: r.PID.String,
		PatientName: r.PatientName.String,
		VisitDate:   r.VisitDate.String, VisitTime: r.VisitTime.String,
		INSCL: r.INSCL.String, INSCLName: r.INSCLName.String,
		ClinicCode: r.ClinicCode.String, ClinicName: r.ClinicName.String,
		DoctorCode:  r.DoctorCode.String,
		TotalCharge: r.TotalCharge.Float64,
	}
}

func (s *PgStore) visitsFor(batchID string) ([]hisclient.VisitSummary, error) {
	var rows []visitRow
	err := s.db.Select(&rows, `
		SELECT vn, hn, pid, patient_name, visit_date, visit_time,
		       inscl, inscl_name, clinic_code, clinic_name, doctor_code,
		       total_charge, ordinal
		FROM opd_ingest_visit WHERE batch_id = $1
		ORDER BY ordinal ASC
	`, batchID)
	if err != nil {
		return nil, err
	}
	out := make([]hisclient.VisitSummary, len(rows))
	for i, r := range rows {
		out[i] = r.toVisit()
	}
	return out, nil
}

// nullable returns a sql.NullString that is NULL if s is empty.
func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullableFloat(f float64) interface{} {
	if f == 0 {
		return nil
	}
	return f
}

// Assertion: PgStore implements Store.
var _ Store = (*PgStore)(nil)
var _ Store = (*MemoryStore)(nil)
