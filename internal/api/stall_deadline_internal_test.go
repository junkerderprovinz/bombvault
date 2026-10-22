package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

func TestAMaintenanceRunGoesOnWhileResticReportsAndEndsWhenItFallsSilent(t *testing.T) {
	var forwarded int
	ctx := progress.WithCountSink(context.Background(), func(progress.CountProgress) { forwarded++ })
	ctx, cancel := stallDeadline(ctx, 100*time.Millisecond)
	defer cancel()
	sink := progress.CountSinkFrom(ctx)

	// Well past the limit in all, through one step that does not move, as a
	// check of one large snapshot reports.
	for i := range 8 {
		time.Sleep(30 * time.Millisecond)
		sink(progress.CountProgress{Done: 0, Total: 1, Unit: "snapshots"})
		if ctx.Err() != nil {
			t.Fatalf("cut off after %d counters while restic was still reporting", i+1)
		}
	}
	if forwarded != 8 {
		t.Fatalf("the progress sink got %d counters, want all 8", forwarded)
	}

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("a run that fell silent was not cut off")
	}
	cause := context.Cause(ctx)
	if !errors.Is(cause, context.DeadlineExceeded) || !strings.Contains(cause.Error(), "reported nothing") {
		t.Fatalf("cause = %v, want a deadline that says why", cause)
	}
}
