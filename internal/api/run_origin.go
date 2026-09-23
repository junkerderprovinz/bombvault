package api

import (
	"context"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// runOriginKey marks a context whose runs were asked for by something the audit
// trail can name, such as the MCP endpoint. Its value survives
// context.WithoutCancel, which is how the origin reaches the detached
// goroutines the backup starters hand their work to.
type runOriginKey struct{}

// RunOrigin names who asked for a run. The zero value covers the web interface
// and the scheduler, which the run history does not single out.
type RunOrigin struct {
	Via   string // "mcp"
	KeyID string // mcp_keys.id
}

// WithRunOrigin marks ctx as started by o, so every run row this package
// records under it carries the origin.
func WithRunOrigin(ctx context.Context, o RunOrigin) context.Context {
	return context.WithValue(ctx, runOriginKey{}, o)
}

// runOriginFromContext reports who asked for this context's runs, or the zero
// value for a context without an origin and for a nil one (a zero-value
// runsAdapter carries no context at all).
func runOriginFromContext(ctx context.Context) RunOrigin {
	if ctx == nil {
		return RunOrigin{}
	}
	o, _ := ctx.Value(runOriginKey{}).(RunOrigin)
	return o
}

// startRunWith opens a run row with the "Backup Everything" group and the run
// origin in the same INSERT, so no run can exist without the audit trail that
// explains it. It takes the store rather than a Service because runsAdapter is
// built with a nil one at the bookkeeping-only call sites.
func startRunWith(ctx context.Context, st *store.Repo, targetID, kind string) (string, error) {
	o := runOriginFromContext(ctx)
	return st.StartRunWith(targetID, kind, store.RunMeta{
		GroupID:       runGroupFromContext(ctx),
		StartedVia:    o.Via,
		StartedViaKey: o.KeyID,
	})
}

func (s *Service) startRun(ctx context.Context, targetID, kind string) (string, error) {
	return startRunWith(ctx, s.store, targetID, kind)
}
