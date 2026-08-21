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

// heartbeatRealProgressFakeEngine's Copy reports ONE real per-snapshot
// percentage through the CopySink installed in ctx (see progBeginCopySink),
// exactly like restic.Copy does when it parses a live "packs copied" line,
// then blocks — so every "offsite:flash" event observed afterwards can only
// have come from the heartbeat goroutine, never from another real update.
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

// TestCopyToOffsiteHeartbeatPreservesRealPercentage is the reviewer's exact
// repro for the code-review blocker on this fix: before it, the heartbeat
// unconditionally republished Percent:0 with no SnapshotIndex/SnapshotTotal
// on the SAME "offsite:<domain>" key every offsiteProgressHeartbeat tick,
// which — since Publish's map replaces the stored/streamed state wholesale —
// erased whatever real percentage progBeginCopySink's sink had just reported
// (e.g. "Replicating snapshot 2 of 4 (63%)" regressing to a bare
// "Replicating…" on every heartbeat tick). On a slow transfer, where a whole
// real percentage step can take many seconds, the heartbeat "wins" almost
// every render. This test drives copyToOffsite for real (not a synthetic
// event sequence): one real CopyProgress update lands, then several
// heartbeat ticks fire (the interval is shrunk via the package var) while
// Copy is still blocked — every one of them must still carry that same real
// percent/snapshotIndex/snapshotTotal, proving the heartbeat now republishes
// the last known real value instead of overwriting it.
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
		done <- svc.copyToOffsite(context.Background(), "flash", settings, restic.Mode{}, "/local/flash")
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
			// Every active event from here on — heartbeat-driven, since Copy is
			// still blocked and can publish no further real update — must still
			// carry the real values. Regressing to Percent:0/SnapshotIndex:0 here
			// is exactly the bug this test guards against.
			// SnapshotTotal is 0 here on purpose: this fake's Snapshots returns
			// nothing for either repo, so copyToOffsiteTarget's upfront candidate
			// count is "could not estimate" — and progBeginCopySink now publishes
			// that unknown honestly instead of widening it to the live index (see
			// TestProgBeginCopySinkTotal and issue #159's follow-up). What this
			// test guards is unchanged: every heartbeat tick must republish the
			// sink's LAST REAL values, not a blank frame.
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

// TestProgBeginCopySinkTotal pins what progBeginCopySink publishes as
// SnapshotTotal, which is the number web/src/lib/progress.ts's
// offsiteRunProgress divides by to derive the run-level percentage the
// dashboard and OffsiteIndicator now show (issue #159's follow-up).
//
// Three cases, and the first one is the fix:
//   - no estimate at all (0) stays 0, the documented "unknown" on the wire.
//     It used to be widened to the live index, which fabricated a plausible
//     "snapshot 7 of 7" out of nothing — indistinguishable from a genuine
//     final snapshot, and enough to make a derived run percentage claim ~99%
//     for a run that had barely started.
//   - a real estimate is published as-is.
//   - a real estimate the live index has overtaken IS widened, because there
//     the estimate is known to have undercounted and "snapshot 3 of 2" would
//     be worse than a slightly optimistic total.
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
