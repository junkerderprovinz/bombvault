package backup

import (
	"context"
	"errors"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// StalledError is the cause a backup's context is cancelled with when the stall
// guard gives up on it. After is the configured time without progress.
type StalledError struct {
	After time.Duration
}

func (e *StalledError) Error() string { return store.StalledReason(e.After, "") }

// StalledBy returns the stall that cancelled ctx, or nil when ctx is live or
// something else ended it.
func StalledBy(ctx context.Context) *StalledError {
	var stall *StalledError
	if errors.As(context.Cause(ctx), &stall) {
		return stall
	}
	return nil
}
