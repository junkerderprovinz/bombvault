package api

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// heartbeatFakeEngine implements only the methods copyToOffsite reaches; the
// embedded ResticEngine stays nil. Copy blocks until blockCopy is closed.
type heartbeatFakeEngine struct {
	ResticEngine
	blockCopy chan struct{}
}

func (f *heartbeatFakeEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (f *heartbeatFakeEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

// Snapshots is called by the background stats sampling that copyToOffsiteTarget
// starts when no growth budget is set.
func (f *heartbeatFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, nil
}

func (f *heartbeatFakeEngine) Copy(_ context.Context, _, _ string, _ []string, _ restic.Limits, _ restic.Mode) error {
	if f.blockCopy != nil {
		<-f.blockCopy
	}
	return nil
}

// A long copy has to keep republishing its progress event, or the frontend's
// staleness check (STALE_MS in web/src/lib/progress.ts) hides it from the
// dashboard.
func TestCopyToOffsiteHeartbeatsWhileCopying(t *testing.T) {
	orig := offsiteProgressHeartbeat
	offsiteProgressHeartbeat = 3 * time.Millisecond
	t.Cleanup(func() { offsiteProgressHeartbeat = orig })

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashOffsite = "rest:http://192.168.1.2:8000/flash"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	fake := &heartbeatFakeEngine{blockCopy: make(chan struct{})}
	prog := progress.NewStore()
	svc := &Service{store: st, engine: fake, progress: prog}

	ch, cancel := prog.Subscribe()
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- svc.copyToOffsite(context.Background(), "flash", settings, "", []domainRepoRef{ownRef("/local/flash")}, nil)
	}()

	// Three active events while Copy blocks cannot all be the start event.
	activeCount := 0
	deadline := time.After(2 * time.Second)
loop:
	for {
		select {
		case e := <-ch:
			if e.Key == "offsite:flash" && e.Active {
				activeCount++
				if activeCount >= 3 {
					break loop
				}
			}
		case <-deadline:
			break loop
		}
	}
	close(fake.blockCopy)

	select {
	case cerr := <-done:
		if cerr != nil {
			t.Fatalf("copyToOffsite: %v", cerr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("copyToOffsite did not return after unblocking Copy")
	}

	if activeCount < 3 {
		t.Fatalf("expected at least 3 active heartbeat events while Copy was in flight, got %d", activeCount)
	}
}

// heartbeatRealProgressFakeEngine's Copy reports one real percentage through
// the CopySink in ctx, as restic.Copy does, and then blocks. Every later event
// comes from the heartbeat.
type heartbeatRealProgressFakeEngine struct {
	ResticEngine
	proceed chan struct{}
}

func (f *heartbeatRealProgressFakeEngine) RepoOpens(context.Context, string, restic.Mode) bool {
	return true
}

func (f *heartbeatRealProgressFakeEngine) Unlock(context.Context, string, bool, restic.Mode) error {
	return nil
}

func (f *heartbeatRealProgressFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, nil
}

func (f *heartbeatRealProgressFakeEngine) Copy(ctx context.Context, _, _ string, _ []string, _ restic.Limits, _ restic.Mode) error {
	if sink := progress.CopySinkFrom(ctx); sink != nil {
		sink(progress.CopyProgress{SnapshotIndex: 2, Percent: 63})
	}
	<-f.proceed
	return nil
}

// Publish replaces an event wholesale, so a heartbeat carrying a blank frame
// would erase the real percentage the copy last reported. Every tick must
// republish the last real values.
func TestCopyToOffsiteHeartbeatPreservesRealPercentage(t *testing.T) {
	orig := offsiteProgressHeartbeat
	offsiteProgressHeartbeat = 3 * time.Millisecond
	t.Cleanup(func() { offsiteProgressHeartbeat = orig })

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashOffsite = "rest:http://192.168.1.2:8000/flash"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	fake := &heartbeatRealProgressFakeEngine{proceed: make(chan struct{})}
	prog := progress.NewStore()
	svc := &Service{store: st, engine: fake, progress: prog}

	ch, cancel := prog.Subscribe()
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- svc.copyToOffsite(context.Background(), "flash", settings, "", []domainRepoRef{ownRef("/local/flash")}, nil)
	}()

	sawReal := false
	verifiedAfter := 0
	deadline := time.After(2 * time.Second)
loop:
	for {
		select {
		case e := <-ch:
			if e.Key != "offsite:flash" || !e.Active {
				continue
			}
			if !sawReal {
				if e.SnapshotIndex == 2 && e.Percent == 63 {
					sawReal = true
				}
				continue
			}
			// SnapshotTotal stays 0: the fake's Snapshots returns nothing, so
			// there is no estimate, and progBeginCopySink publishes that as
			// unknown (see TestProgBeginCopySinkTotal).
			if e.Percent != 63 || e.SnapshotIndex != 2 || e.SnapshotTotal != 0 {
				t.Fatalf("event after the real percentage lost it: %+v", e)
			}
			verifiedAfter++
			if verifiedAfter >= 3 {
				break loop
			}
		case <-deadline:
			t.Fatal("timed out waiting for heartbeat ticks after the real percentage")
		}
	}
	close(fake.proceed)

	select {
	case cerr := <-done:
		if cerr != nil {
			t.Fatalf("copyToOffsite: %v", cerr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("copyToOffsite did not return after unblocking Copy")
	}

	if !sawReal {
		t.Fatal("never observed the real percentage event")
	}
}

// The heartbeat stops before the final progEnd event, so a late tick cannot
// mark the finished copy active again.
func TestCopyToOffsiteHeartbeatStopsAfterFinish(t *testing.T) {
	orig := offsiteProgressHeartbeat
	offsiteProgressHeartbeat = 3 * time.Millisecond
	t.Cleanup(func() { offsiteProgressHeartbeat = orig })

	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.FlashOffsite = "rest:http://192.168.1.2:8000/flash"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	fake := &heartbeatFakeEngine{}
	prog := progress.NewStore()
	svc := &Service{store: st, engine: fake, progress: prog}

	if err := svc.copyToOffsite(context.Background(), "flash", settings, "", []domainRepoRef{ownRef("/local/flash")}, nil); err != nil {
		t.Fatalf("copyToOffsite: %v", err)
	}

	// Several heartbeat intervals pass after the call returns; a running
	// heartbeat would publish into this window.
	ch, cancel := prog.Subscribe()
	defer cancel()
	select {
	case e := <-ch:
		t.Fatalf("unexpected event after copyToOffsite returned: %+v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

// SnapshotTotal is what offsiteRunProgress in web/src/lib/progress.ts divides
// by. Without an estimate it stays 0, meaning unknown; widening it to the live
// index would show "snapshot 7 of 7" for a run that has barely started. An
// estimate the live index has overtaken is widened, since "snapshot 3 of 2"
// would be worse.
func TestProgBeginCopySinkTotal(t *testing.T) {
	cases := []struct {
		name      string
		estimate  int
		index     int
		wantTotal int
	}{
		{"no estimate stays unknown", 0, 7, 0},
		{"real estimate is published as-is", 4, 2, 4},
		{"undercounting estimate is widened to the live index", 2, 3, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := progress.NewStore()
			svc := &Service{progress: prog}
			ch, cancel := prog.Subscribe()
			defer cancel()

			ctx := svc.progBeginCopySink(context.Background(), "flash", 1700000000, tc.estimate, nil)
			sink := progress.CopySinkFrom(ctx)
			if sink == nil {
				t.Fatal("progBeginCopySink installed no CopySink on the context")
			}
			sink(progress.CopyProgress{SnapshotIndex: tc.index, Percent: 42})

			select {
			case e := <-ch:
				if e.Key != "offsite:flash" || e.SnapshotIndex != tc.index {
					t.Fatalf("unexpected event: %+v", e)
				}
				if e.SnapshotTotal != tc.wantTotal {
					t.Fatalf("SnapshotTotal = %d, want %d (estimate %d, live index %d)",
						e.SnapshotTotal, tc.wantTotal, tc.estimate, tc.index)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("no event published")
			}
		})
	}
}
