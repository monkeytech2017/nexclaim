package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/nexclaim/nexclaim/internal/audit"
)

// AuditEntry = row ใน audit_log — read-side DTO (mirrors the wire format).
// Admin UI reads these; every field is optional except Action + CreatedAt.
type AuditEntry struct {
	ID         string         `db:"id"          json:"id"`
	ActorID    *string        `db:"-"           json:"actor_id,omitempty"`
	ActorIDRaw sql.NullString `db:"actor_id"    json:"-"`
	ActorRole  string         `db:"actor_role"  json:"actor_role,omitempty"`
	ActorName  string         `db:"actor_name"  json:"actor_name,omitempty"`
	Action     string         `db:"action"      json:"action"`
	TargetKind string         `db:"target_kind" json:"target_kind,omitempty"`
	TargetID   string         `db:"target_id"   json:"target_id,omitempty"`
	HCode      string         `db:"hcode"       json:"hcode,omitempty"`
	Payload    map[string]any `db:"-"           json:"payload,omitempty"`
	PayloadRaw []byte         `db:"payload"     json:"-"`
	CreatedAt  time.Time      `db:"created_at"  json:"created_at"`
}

// AuditFilter — all fields optional, AND-combined.
type AuditFilter struct {
	Action     string
	ActorID    string
	HCode      string
	TargetKind string
	TargetID   string
	From       time.Time // zero = unfiltered
	To         time.Time
	Limit      int // 0 = default 200
}

// AuditRepo = read + append contract. Implementations must NOT expose
// UPDATE/DELETE — prod audit_log grant is SELECT + INSERT only.
// It also satisfies audit.Writer so server callers can use a single value
// for both the read handler and the write side.
type AuditRepo interface {
	audit.Writer
	List(ctx context.Context, f AuditFilter) ([]AuditEntry, error)
}

// PgAuditRepo is the Postgres-backed impl.
type PgAuditRepo struct{ db *sqlx.DB }

func NewPgAuditRepo(db *sqlx.DB) *PgAuditRepo { return &PgAuditRepo{db: db} }

// Write inserts one row. Payload is marshalled to JSONB; nil → SQL NULL.
// Returns the error unchanged — the server wrapper is responsible for
// logging + swallowing so the originating action continues.
func (r *PgAuditRepo) Write(ctx context.Context, e audit.Entry) error {
	if e.Action == "" {
		return fmt.Errorf("audit: action required")
	}
	var payload []byte
	if len(e.Payload) > 0 {
		b, err := json.Marshal(e.Payload)
		if err != nil {
			return fmt.Errorf("audit: marshal payload: %w", err)
		}
		payload = b
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_log
		  (actor_id, actor_role, actor_name, action,
		   target_kind, target_id, hcode, payload)
		VALUES
		  (NULLIF($1,'')::uuid, NULLIF($2,''), NULLIF($3,''), $4,
		   NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), $8::jsonb)
	`,
		e.ActorID, e.ActorRole, e.ActorName, e.Action,
		e.TargetKind, e.TargetID, e.HCode, nullableJSON(payload),
	)
	if err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	return nil
}

// nullableJSON returns nil (→ SQL NULL) when bytes are empty, else the
// raw JSON payload so pq sends it as jsonb.
func nullableJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// List returns rows newest-first, filtered per AuditFilter.
// Mirrors the `add := func(...)` condition-builder style used by
// claim_batch_repo.go so the query stays easy to extend.
func (r *PgAuditRepo) List(ctx context.Context, f AuditFilter) ([]AuditEntry, error) {
	conds := []string{}
	args := []interface{}{}
	add := func(cond string, v interface{}) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Action != "" {
		add("action = $%d", f.Action)
	}
	if f.ActorID != "" {
		add("actor_id = $%d::uuid", f.ActorID)
	}
	if f.HCode != "" {
		add("hcode = $%d", f.HCode)
	}
	if f.TargetKind != "" {
		add("target_kind = $%d", f.TargetKind)
	}
	if f.TargetID != "" {
		add("target_id = $%d", f.TargetID)
	}
	if !f.From.IsZero() {
		add("created_at >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("created_at <= $%d", f.To)
	}
	q := `
		SELECT id::text AS id,
		       actor_id::text AS actor_id,
		       COALESCE(actor_role,'') AS actor_role,
		       COALESCE(actor_name,'') AS actor_name,
		       action,
		       COALESCE(target_kind,'') AS target_kind,
		       COALESCE(target_id,'')   AS target_id,
		       COALESCE(hcode,'')       AS hcode,
		       payload,
		       created_at
		FROM audit_log
	`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY created_at DESC"
	lim := f.Limit
	if lim <= 0 {
		lim = 200
	}
	q += fmt.Sprintf(" LIMIT %d", lim)

	var rows []AuditEntry
	if err := r.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	// Project nullable scan fields into JSON-friendly shape.
	for i := range rows {
		if rows[i].ActorIDRaw.Valid && rows[i].ActorIDRaw.String != "" {
			s := rows[i].ActorIDRaw.String
			rows[i].ActorID = &s
		}
		if len(rows[i].PayloadRaw) > 0 {
			var m map[string]any
			if err := json.Unmarshal(rows[i].PayloadRaw, &m); err == nil {
				rows[i].Payload = m
			}
		}
	}
	return rows, nil
}

// Compile-time contract check.
var _ AuditRepo = (*PgAuditRepo)(nil)
