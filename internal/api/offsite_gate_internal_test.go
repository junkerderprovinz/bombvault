package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The predicate must agree with the scheduler, which registers a cron only for
// an enabled cadence. Treating "off" as a schedule of its own would leave the
// domain replicated by nobody.
func TestOffsiteReplicatesOnOwnSchedule(t *testing.T) {
	s := &Service{}
	cases := []struct {
		name     string
		schedule string
		wantOwn  bool // false: replicated by the backup run
	}{
		{"blank couples", "", false},
		{"off couples", "off", false},
		{"whitespace couples", "   ", false},
		{"invalid cadence defaults to coupled (safe direction)", "not-a-cadence", false},
		{"daily is its own schedule", "daily 02:00", true},
		{"weekly is its own schedule", "weekly Sun 03:00", true},
		{"everyN is its own schedule", "everyN 3 04:00", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settings := store.Settings{ContainersOffsiteSchedule: c.schedule}
			if got := s.offsiteReplicatesOnOwnSchedule("containers", settings); got != c.wantOwn {
				t.Fatalf("offsiteReplicatesOnOwnSchedule(%q) = %v, want %v", c.schedule, got, c.wantOwn)
			}
		})
	}
}

type otherCtxKey struct{}

// The batch loops wrap the suppressed context in timeouts and progress
// contexts before replicateOffsite reads the flag, so it has to survive
// derivation.
func TestBulkReplicateSuppressedRoundTrip(t *testing.T) {
	ctx := context.Background()
	if bulkReplicateSuppressed(ctx) {
		t.Fatal("a plain context must NOT report bulk-suppressed")
	}
	sctx := WithBulkReplicateSuppressed(ctx)
	if !bulkReplicateSuppressed(sctx) {
		t.Fatal("WithBulkReplicateSuppressed(ctx) must report bulk-suppressed")
	}
	child := context.WithValue(sctx, otherCtxKey{}, "x")
	if !bulkReplicateSuppressed(child) {
		t.Fatal("a child context derived from a suppressed one must STAY suppressed")
	}
}
