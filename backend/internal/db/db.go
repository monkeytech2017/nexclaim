// Package db opens and pools the Postgres connection for NexClaim.
//
// Usage:
//   d, err := db.Open(cfg.DSN())
//   defer d.Close()
//
// Callers get a *sqlx.DB — standard database/sql under the hood with helpers.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Open connects, pings, and returns a pooled *sqlx.DB.
// Panics are avoided; error includes the DSN host for quick triage.
func Open(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return db, nil
}
