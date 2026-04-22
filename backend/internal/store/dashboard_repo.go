// Package store dashboard aggregate repository.
//
// PgDashboardRepo.Stats runs one SQL per sub-aggregate (summary, by_format,
// by_status, by_day, top_ccodes) and stitches them into a single
// DashboardStats response for the main dashboard page.
//
// One method = one query design: keeps each query readable + easy to reason
// about with EXPLAIN, at the cost of a handful of round-trips. Given this
// endpoint is a dashboard refresh (few hits/minute), that's fine.
package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// DashboardStats — shape bound by frontend. Field names are part of the API
// contract; do not rename without updating the frontend in lockstep.
type DashboardStats struct {
	Summary   Summary       `json:"summary"`
	ByFormat  []FormatCount `json:"by_format"`
	ByStatus  []StatusCount `json:"by_status"`
	ByDay     []DayBucket   `json:"by_day"`
	TopCCodes []CCodeCount  `json:"top_ccodes"`
}

// Summary — totals across claim_batch, claim_record, c_code_log, and send_log.
type Summary struct {
	BatchesTotal    int     `json:"batches_total"`
	RecordsTotal    int     `json:"records_total"`
	RecordsErrors   int     `json:"records_errors"`
	CCodesOpen      int     `json:"ccodes_open"`
	AvgSendMs       int     `json:"avg_send_ms"`       // over send_log where success=true
	SendSuccessRate float64 `json:"send_success_rate"` // 0..1 over send_log
}

type FormatCount struct {
	Format  string `json:"format"`
	Count   int    `json:"count"`
	Records int    `json:"records"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// DayBucket uses a map so the frontend can stack an arbitrary subset of
// formats without a predetermined column list.
type DayBucket struct {
	Day     string         `json:"day"`
	Formats map[string]int `json:"formats"`
}

type CCodeCount struct {
	CCode      string `json:"c_code"`
	Count      int    `json:"count"`
	DescSample string `json:"desc_sample"`
}

// DashboardFilter — all fields optional; AND-combined. Period range is
// inclusive; empty means unbounded on that side.
type DashboardFilter struct {
	HCode      string
	PeriodFrom string // YYYYMM inclusive
	PeriodTo   string // YYYYMM inclusive
}

// DashboardRepo is a read-only aggregate over claim_batch / send_log /
// c_code_log / claim_record.
type DashboardRepo interface {
	Stats(ctx context.Context, f DashboardFilter) (*DashboardStats, error)
}

type PgDashboardRepo struct{ db *sqlx.DB }

func NewPgDashboardRepo(db *sqlx.DB) *PgDashboardRepo { return &PgDashboardRepo{db: db} }

// Stats runs each sub-aggregate independently and stitches them into one
// DashboardStats. Empty result sets → zero-valued (not nil) slices/maps so
// the JSON always has the expected shape.
func (r *PgDashboardRepo) Stats(ctx context.Context, f DashboardFilter) (*DashboardStats, error) {
	out := &DashboardStats{
		ByFormat:  []FormatCount{},
		ByStatus:  []StatusCount{},
		ByDay:     []DayBucket{},
		TopCCodes: []CCodeCount{},
	}

	sum, err := r.summary(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	out.Summary = sum

	if out.ByFormat, err = r.byFormat(ctx, f); err != nil {
		return nil, fmt.Errorf("by_format: %w", err)
	}
	if out.ByStatus, err = r.byStatus(ctx, f); err != nil {
		return nil, fmt.Errorf("by_status: %w", err)
	}
	if out.ByDay, err = r.byDay(ctx, f); err != nil {
		return nil, fmt.Errorf("by_day: %w", err)
	}
	if out.TopCCodes, err = r.topCCodes(ctx, f); err != nil {
		return nil, fmt.Errorf("top_ccodes: %w", err)
	}
	return out, nil
}

// batchFilterClause builds a reusable WHERE fragment aliased on table `cb`
// (claim_batch). Returns "" if no filter is active.
func batchFilterClause(f DashboardFilter) (string, []interface{}) {
	conds := []string{}
	args := []interface{}{}
	idx := 1
	if f.HCode != "" {
		conds = append(conds, fmt.Sprintf("cb.hcode = $%d", idx))
		args = append(args, f.HCode)
		idx++
	}
	if f.PeriodFrom != "" {
		conds = append(conds, fmt.Sprintf("cb.period >= $%d", idx))
		args = append(args, f.PeriodFrom)
		idx++
	}
	if f.PeriodTo != "" {
		conds = append(conds, fmt.Sprintf("cb.period <= $%d", idx))
		args = append(args, f.PeriodTo)
		idx++
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// ── sub-aggregates ─────────────────────────────────────────────

// summary — four scalars from claim_batch, one count from c_code_log joined
// through claim_batch, and two scalars from send_log joined through
// claim_batch. Kept as three small queries; COALESCE everywhere so JSON has
// zeroes instead of null.
func (r *PgDashboardRepo) summary(ctx context.Context, f DashboardFilter) (Summary, error) {
	var s Summary
	where, args := batchFilterClause(f)

	// claim_batch totals (batches + records + errors).
	q1 := `
		SELECT
			COALESCE(COUNT(*), 0)                AS batches_total,
			COALESCE(SUM(cb.total_records), 0)   AS records_total,
			COALESCE(SUM(cb.error_records), 0)   AS records_errors
		FROM claim_batch cb` + where
	totals := struct {
		BatchesTotal  int `db:"batches_total"`
		RecordsTotal  int `db:"records_total"`
		RecordsErrors int `db:"records_errors"`
	}{}
	if err := r.db.GetContext(ctx, &totals, q1, args...); err != nil {
		return s, err
	}
	s.BatchesTotal = totals.BatchesTotal
	s.RecordsTotal = totals.RecordsTotal
	s.RecordsErrors = totals.RecordsErrors

	// c_code_log open count, filtered via claim_batch.
	q2 := `
		SELECT COALESCE(COUNT(*), 0) AS ccodes_open
		FROM c_code_log cc
		JOIN claim_batch cb ON cb.batch_id = cc.batch_id
		WHERE cc.resolved = false`
	if where != "" {
		q2 += " AND " + strings.TrimPrefix(where, " WHERE ")
	}
	var openRow struct {
		CCodesOpen int `db:"ccodes_open"`
	}
	if err := r.db.GetContext(ctx, &openRow, q2, args...); err != nil {
		return s, err
	}
	s.CCodesOpen = openRow.CCodesOpen

	// send_log avg duration (success=true) + overall success rate.
	q3 := `
		SELECT
			COALESCE(AVG(sl.duration_ms) FILTER (WHERE sl.success = true), 0)::int AS avg_ms,
			CASE WHEN COUNT(*) = 0 THEN 0::float
			     ELSE COUNT(*) FILTER (WHERE sl.success = true)::float / COUNT(*)::float
			END                                                                    AS success_rate
		FROM send_log sl
		JOIN claim_batch cb ON cb.batch_id = sl.batch_id` + where
	send := struct {
		AvgMs       int     `db:"avg_ms"`
		SuccessRate float64 `db:"success_rate"`
	}{}
	if err := r.db.GetContext(ctx, &send, q3, args...); err != nil {
		return s, err
	}
	s.AvgSendMs = send.AvgMs
	s.SendSuccessRate = send.SuccessRate
	return s, nil
}

// byFormat counts batches + sums records per format.
func (r *PgDashboardRepo) byFormat(ctx context.Context, f DashboardFilter) ([]FormatCount, error) {
	where, args := batchFilterClause(f)
	q := `
		SELECT
			cb.format                            AS format,
			COALESCE(COUNT(*), 0)                AS count,
			COALESCE(SUM(cb.total_records), 0)   AS records
		FROM claim_batch cb` + where + `
		GROUP BY cb.format
		ORDER BY count DESC
	`
	var rows []struct {
		Format  string `db:"format"`
		Count   int    `db:"count"`
		Records int    `db:"records"`
	}
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	out := make([]FormatCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, FormatCount{Format: r.Format, Count: r.Count, Records: r.Records})
	}
	return out, nil
}

// byStatus counts batches per status (pending/validating/sending/sent/error/c_code).
func (r *PgDashboardRepo) byStatus(ctx context.Context, f DashboardFilter) ([]StatusCount, error) {
	where, args := batchFilterClause(f)
	q := `
		SELECT cb.status AS status, COALESCE(COUNT(*), 0) AS count
		FROM claim_batch cb` + where + `
		GROUP BY cb.status
		ORDER BY count DESC
	`
	var rows []struct {
		Status string `db:"status"`
		Count  int    `db:"count"`
	}
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	out := make([]StatusCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, StatusCount{Status: r.Status, Count: r.Count})
	}
	return out, nil
}

// byDay — last 30 days, grouped by DATE(created_at) in the server timezone.
// Returned as []DayBucket with Formats map so the frontend can stack an
// arbitrary subset of formats.
func (r *PgDashboardRepo) byDay(ctx context.Context, f DashboardFilter) ([]DayBucket, error) {
	where, args := batchFilterClause(f)
	// Append the "last 30 days" constraint.
	if where == "" {
		where = " WHERE cb.created_at >= now() - INTERVAL '30 days'"
	} else {
		where += " AND cb.created_at >= now() - INTERVAL '30 days'"
	}
	q := `
		SELECT
			to_char(DATE(cb.created_at), 'YYYY-MM-DD') AS day,
			cb.format                                  AS format,
			COALESCE(COUNT(*), 0)                      AS count
		FROM claim_batch cb` + where + `
		GROUP BY DATE(cb.created_at), cb.format
		ORDER BY DATE(cb.created_at) ASC
	`
	var rows []struct {
		Day    string `db:"day"`
		Format string `db:"format"`
		Count  int    `db:"count"`
	}
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	// Bucket into map[day]DayBucket while preserving ordered day list.
	order := []string{}
	byDay := map[string]*DayBucket{}
	for _, r := range rows {
		if _, ok := byDay[r.Day]; !ok {
			byDay[r.Day] = &DayBucket{Day: r.Day, Formats: map[string]int{}}
			order = append(order, r.Day)
		}
		byDay[r.Day].Formats[r.Format] += r.Count
	}
	out := make([]DayBucket, 0, len(order))
	for _, d := range order {
		out = append(out, *byDay[d])
	}
	return out, nil
}

// topCCodes — top-10 codes across the filter window, with one sample desc
// (via MIN) so the UI can show a hint.
func (r *PgDashboardRepo) topCCodes(ctx context.Context, f DashboardFilter) ([]CCodeCount, error) {
	where, args := batchFilterClause(f)
	// c_code_log has no period/hcode itself — join claim_batch.
	q := `
		SELECT
			cc.c_code                                   AS c_code,
			COALESCE(COUNT(*), 0)                       AS count,
			COALESCE(MIN(cc.c_desc), '')                AS desc_sample
		FROM c_code_log cc
		JOIN claim_batch cb ON cb.batch_id = cc.batch_id` + where + `
		GROUP BY cc.c_code
		ORDER BY count DESC, cc.c_code ASC
		LIMIT 10
	`
	var rows []struct {
		CCode      string `db:"c_code"`
		Count      int    `db:"count"`
		DescSample string `db:"desc_sample"`
	}
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, err
	}
	out := make([]CCodeCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, CCodeCount{CCode: r.CCode, Count: r.Count, DescSample: r.DescSample})
	}
	return out, nil
}

var _ DashboardRepo = (*PgDashboardRepo)(nil)
