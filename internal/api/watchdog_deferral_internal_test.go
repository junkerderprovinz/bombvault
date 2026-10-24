package api

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestTheOverdueAlertWaitsForABackupWithinItsDeferral(t *testing.T) {
	svc, st, hits, seeded := watchdogTestService(t)
	const day = int64(86400)
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	// Deferred two hours before the check, with the default twelve hours to go.
	since := seeded + 3*day - 2*3600
	if err := st.SaveDeferral(store.Deferral{TargetID: tg.ID, Container: "plex", Since: since, NextTry: since + 1800, Tries: 1}); err != nil {
		t.Fatal(err)
	}
	if err := svc.runWatchdogAt(context.Background(), seeded+3*day); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Fatalf("an overdue alert went out while the backup waited within its deadline (%d)", n)
	}

	// Past its deadline the wait holds nothing back.
	if err := svc.runWatchdogAt(context.Background(), since+13*3600); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(hits); n != 1 {
		t.Fatalf("no overdue alert once the deadline had passed (%d)", n)
	}
}
