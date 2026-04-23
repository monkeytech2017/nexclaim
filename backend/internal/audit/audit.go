// Package audit writes admin-sensitive events to an append-only audit log.
//
// Writers are invoked AFTER the triggering action has succeeded — audit
// failures must never block the original request (they are logged to
// stderr + swallowed, mirroring the persistRun pattern in server/server.go).
//
// Payload is a freeform map[string]any. Callers are responsible for
// redacting secrets (raw API keys, passwords, PII). There is no automatic
// redaction middleware in this slice — err on the side of omitting.
package audit

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/nexclaim/nexclaim/internal/auth"
)

// Entry is the wire-free shape the server hands to a Writer. The DB layer
// fills in id + created_at on insert; they are not in this struct to keep
// callers from accidentally forging values.
type Entry struct {
	ActorID    string         // api_key.id — "" means unknown (bootstrap, auth-disabled)
	ActorRole  string         // 'admin' | 'hospital' | 'system' | ''
	ActorName  string         // denormalized — survives key deletion
	Action     string         // required, e.g. 'api_key.create'
	TargetKind string         // 'api_key' | 'ccode' | 'rep' | 'claim_batch' | ...
	TargetID   string
	HCode      string         // hospital scope; "" for admin-wide events
	Payload    map[string]any // redacted extras; MUST NOT contain raw keys/PII
}

// Writer is the minimum contract an audit sink must satisfy. The server
// code holds an audit.Writer so tests can swap in fakes trivially.
// Implementations return an error on failure; callers log + swallow.
type Writer interface {
	Write(ctx context.Context, entry Entry) error
}

// NoopWriter satisfies Writer without persisting anything. Used as the
// safe default when DB is not configured.
type NoopWriter struct{}

// Write silently accepts every entry.
func (NoopWriter) Write(_ context.Context, _ Entry) error { return nil }

// Compile-time check.
var _ Writer = NoopWriter{}

// ActorFromContext extracts caller identity for audit logging. When no
// Identity is attached (auth disabled, bootstrap flow), it returns the
// "system" / "auth-disabled" pair so audit rows still capture intent.
//
// Returns (role, name, id). id is the api_key UUID when known.
func ActorFromContext(c *gin.Context) (role, name, id string) {
	ident, ok := auth.FromContext(c)
	if !ok || ident == nil {
		return "system", "auth-disabled", ""
	}
	return ident.Role, ident.Name, ident.ID
}
