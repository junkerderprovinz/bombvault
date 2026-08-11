package api

import (
	"context"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// heartbeatFakeEngine embeds ResticEngine (left nil) and overrides only the
// three methods copyToOffsite's path exercises — the api_test fakeResticEngine
// isn't visible here (external test package), and the other interface methods
// are never reached on this path. Same pattern as self_restart_internal_test.go.
type heartbeatFakeEngine struct {
	ResticEngine
	blockCopy chan struct{}
}

func (f *heartbeatFakeEngine) RepoOpens(context.Context, string, restic.Mode) bool { return true }

func (f *heartbeatFakeEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

// Snapshots is reached by CollectStatsAsync's background sampling goroutine
// (fired whenever no off-site growth budget is set — see copyToOffsiteTarget),
// which races with the rest of this test; stub it so that background goroutine
// doesn't panic on the nil-embedded interface. Its result is irrelevant here.
func (f *heartbeatFakeEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, nil
}

func (f *heartbeatFakeEngine) Copy(_ context.Context, _, _ string, _ []string, _ restic.Limits, _ restic.Mode) error {
	if f.blockCopy != nil {
		<-f.blockCopy
	}
	return nil
}

// TestCopyToOffsiteHeartbeatsWhileCopying pins #134: a long-running off-site
// copy must keep re-publishing its progress event so the frontend's 15s
// staleness check (web/src/lib/progress.ts STALE_MS) never hides the
// dashboard's "running" line. Before this fix the event was only published
// once at start (progBegin) and once at finish (progEnd), so any replication
// slower than 15s looked like it had silently vanished from the dashboard —
// reported by a real user against a real off-site upload (bombvault#134).
// The heartbeat interval is shrunk via the package var so the test itself
// runs in milliseconds, not real minutes.
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
		done <- svc.copyToOffsite(context.Background(), "flash", settings, restic.Mode{}, "/local/flash")
	}()

	// Drain events while Copy is still blocked, counting active "offsite:flash"
	// heartbeats. 3 within the deadline proves the heartbeat is genuinely
	// periodic, not just the single progBegin event.
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

// TestCopyToOffsiteHeartbeatStopsAfterFinish pins the shutdown-ordering half
// of #134: once copyToOffsite returns, no further heartbeat can fire — the
// heartbeat goroutine is stopped (via defer, registered after progEnd so it
// unwinds FIRST) before the terminal progEnd event, so a late tick can never
// race past it and resurrect Active:true after the operation is already done.
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

	fake := &heartbeatFakeEngine{} // blockCopy nil: Copy returns immediately
	prog := progress.NewStore()
	svc := &Service{store: st, engine: fake, progress: prog}

	if err := svc.copyToOffsite(context.Background(), "flash", settings, restic.Mode{}, "/local/flash"); err != nil {
		t.Fatalf("copyToOffsite: %v", err)
	}

	// Subscribe AFTER the call returns and wait several heartbeat intervals:
	// a still-running goroutine would publish an Active:true tick into this
	// window; a properly stopped one publishes nothing.
	ch, cancel := prog.Subscribe()
	defer cancel()
	select {
	case e := <-ch:
		t.Fatalf("unexpected event after copyToOffsite returned: %+v", e)
	case <-time.After(50 * time.Millisecond):
		// no event — heartbeat goroutine correctly stopped
	}
}
