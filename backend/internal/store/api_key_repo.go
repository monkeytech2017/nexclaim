package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/nexclaim/nexclaim/internal/auth"
)

// PgAPIKeyRepo is the Postgres-backed implementation of auth.Repo.
// It only reads/writes api_key rows; it does NOT hold the raw key —
// the caller hashes first and hands the hash to Insert.
type PgAPIKeyRepo struct{ db *sqlx.DB }

func NewPgAPIKeyRepo(db *sqlx.DB) *PgAPIKeyRepo { return &PgAPIKeyRepo{db: db} }

// apiKeyRow is the internal scan target. We project COALESCE(hcode,'') so
// admin rows (hcode NULL) map cleanly to the empty string in auth.Identity.
type apiKeyRow struct {
	ID         string       `db:"id"`
	Role       string       `db:"role"`
	HCode      string       `db:"hcode"`
	Name       string       `db:"name"`
	IsActive   bool         `db:"is_active"`
	CreatedAt  time.Time    `db:"created_at"`
	LastUsedAt sql.NullTime `db:"last_used_at"`
	ExpiresAt  sql.NullTime `db:"expires_at"`
}

// toIdentity projects an apiKeyRow into the domain Identity. last_used_at +
// expires_at are NULL for never-used / never-expiring keys — expose as nil
// pointers so JSON omits the keys.
func (r apiKeyRow) toIdentity() *auth.Identity {
	ident := &auth.Identity{
		ID: r.ID, Role: r.Role, HCode: r.HCode, Name: r.Name,
		IsActive: r.IsActive, CreatedAt: r.CreatedAt,
	}
	if r.LastUsedAt.Valid {
		t := r.LastUsedAt.Time
		ident.LastUsedAt = &t
	}
	if r.ExpiresAt.Valid {
		t := r.ExpiresAt.Time
		ident.ExpiresAt = &t
	}
	return ident
}

func (r *PgAPIKeyRepo) GetByHash(ctx context.Context, hash string) (*auth.Identity, error) {
	var row apiKeyRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id::text        AS id,
		       role,
		       COALESCE(hcode,'') AS hcode,
		       name,
		       is_active,
		       created_at,
		       last_used_at,
		       expires_at
		FROM api_key
		WHERE key_hash = $1 AND is_active = true
	`, hash)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("api_key get: %w", err)
	}
	return row.toIdentity(), nil
}

func (r *PgAPIKeyRepo) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE api_key SET last_used_at = now() WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("api_key touch: %w", err)
	}
	return nil
}

func (r *PgAPIKeyRepo) Insert(ctx context.Context, in auth.Insert) (*auth.Identity, error) {
	if in.KeyHash == "" {
		return nil, fmt.Errorf("key_hash required")
	}
	if in.Name == "" {
		return nil, fmt.Errorf("name required")
	}
	switch in.Role {
	case auth.RoleAdmin:
		if in.HCode != "" {
			return nil, fmt.Errorf("admin key cannot have hcode")
		}
	case auth.RoleHospital:
		if in.HCode == "" {
			return nil, fmt.Errorf("hospital key requires hcode")
		}
	default:
		return nil, fmt.Errorf("role must be admin or hospital, got %q", in.Role)
	}

	// Insert with NULLIF so an empty hcode becomes NULL (matches the
	// role-admin case; the CHECK constraint would otherwise reject).
	// expires_at: nil → NULL (never expires); time → stored as TIMESTAMPTZ.
	var expires sql.NullTime
	if in.ExpiresAt != nil {
		expires = sql.NullTime{Time: *in.ExpiresAt, Valid: true}
	}
	var row apiKeyRow
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO api_key (key_hash, role, hcode, name, is_active, expires_at)
		VALUES ($1, $2, NULLIF($3,''), $4, true, $5)
		RETURNING id::text,
		          role,
		          COALESCE(hcode,'') AS hcode,
		          name,
		          is_active,
		          created_at,
		          last_used_at,
		          expires_at
	`, in.KeyHash, in.Role, in.HCode, in.Name, expires).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("api_key insert: %w", err)
	}
	return row.toIdentity(), nil
}

// List returns every api_key row (admin sees all — active + inactive),
// newest-first by created_at. Never includes key_hash or the raw key.
func (r *PgAPIKeyRepo) List(ctx context.Context) ([]auth.Identity, error) {
	var rows []apiKeyRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id::text          AS id,
		       role,
		       COALESCE(hcode,'') AS hcode,
		       name,
		       is_active,
		       created_at,
		       last_used_at,
		       expires_at
		FROM api_key
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("api_key list: %w", err)
	}
	out := make([]auth.Identity, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row.toIdentity())
	}
	return out, nil
}

// SetActive flips is_active on a single id and returns the fresh Identity.
// Uses a single UPDATE ... RETURNING (no separate SELECT needed); empty
// result → ErrNotFound so the HTTP layer can map to 404.
func (r *PgAPIKeyRepo) SetActive(ctx context.Context, id string, active bool) (*auth.Identity, error) {
	var row apiKeyRow
	err := r.db.QueryRowxContext(ctx, `
		UPDATE api_key
		   SET is_active = $2
		 WHERE id = $1::uuid
		RETURNING id::text,
		          role,
		          COALESCE(hcode,'') AS hcode,
		          name,
		          is_active,
		          created_at,
		          last_used_at,
		          expires_at
	`, id, active).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("api_key set_active: %w", err)
	}
	return row.toIdentity(), nil
}

func (r *PgAPIKeyRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM api_key`)
	if err != nil {
		return 0, fmt.Errorf("api_key count: %w", err)
	}
	return n, nil
}

// Compile-time check that PgAPIKeyRepo satisfies the interface.
var _ auth.Repo = (*PgAPIKeyRepo)(nil)
