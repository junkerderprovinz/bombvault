package api

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

func TestRecoverZFSRestartsStartsPendingAndClears(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	dock.containers["db"].running = false
	dock.containers["web"].running = false
	s, st, _, d := zfsStopFixture(t, dock, "db", "web")
	messages := zfsCaptureNotifications(t, s)
	if err := st.SetZFSRestartPending(d.ID, []string{"db", "web"}); err != nil {
		t.Fatal(err)
	}

	s.RecoverZFSRestarts(context.Background())

	calls := dock.recorded()
	if callIndex(calls, "start:db") < 0 || callIndex(calls, "start:web") < 0 {
		t.Fatalf("calls = %v, want both containers started", calls)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.RestartPending) != 0 {
		t.Fatalf("restart marker = %v, want it cleared", row.RestartPending)
	}
	if !zfsAnyMessageContains(messages(), "2 containers") {
		t.Fatalf("notifications = %v, want one naming how many came back", messages())
	}
}

func TestRecoverZFSRestartsKeepsNamesThatFailToStart(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	dock.containers["db"].running = false
	dock.containers["web"].running = false
	dock.containers["web"].startLeavesDown = true
	s, st, _, d := zfsStopFixture(t, dock, "db", "web")
	if err := st.SetZFSRestartPending(d.ID, []string{"db", "web"}); err != nil {
		t.Fatal(err)
	}

	s.RecoverZFSRestarts(context.Background())

	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.RestartPending) != 1 || row.RestartPending[0] != "web" {
		t.Fatalf("restart marker = %v, want only the container still down", row.RestartPending)
	}
}

func TestStartupRecoversRestartsSynchronously(t *testing.T) {
	dock := newZFSFakeDocker("db")
	dock.containers["db"].running = false
	s, st, host, d := zfsStopFixture(t, dock, "db")
	if err := st.SetZFSRestartPending(d.ID, []string{"db"}); err != nil {
		t.Fatal(err)
	}

	s.RecoverZFSRestarts(context.Background())

	if callIndex(dock.recorded(), "start:db") < 0 {
		t.Fatalf("calls = %v, want the container started before the call returned", dock.recorded())
	}
	// The recovery runs before the background sweep takes the domain, so an
	// unreachable pool host can never keep a user's applications down.
	if len(host.recorded()) != 0 {
		t.Fatalf("host calls = %v, want the recovery to wait for nothing over SSH", host.recorded())
	}
}

func TestStartupSweepHoldsLockButDoesNotBlock(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	zfsSeedItem(t, st, zfsRoot)
	listing := make(chan struct{})
	release := make(chan struct{})
	host.onTree = func() {
		close(listing)
		<-release
	}

	unlock := s.LockDomainForStartupSweep()
	swept := make(chan struct{})
	go func() {
		defer close(swept)
		defer unlock()
		s.SweepZFSLeftoversOnStartup(context.Background())
	}()

	<-listing
	if _, ok := s.tryLockDomain(zfsDomain); ok {
		t.Fatal("a scheduled job could start while the startup sweep runs")
	}
	close(release)
	<-swept

	got, ok := s.tryLockDomain(zfsDomain)
	if !ok {
		t.Fatal("the sweep did not release the domain")
	}
	got()
}

func TestStartupSweepRemovesDestroyKilledAtShutdown(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	d := zfsSeedItem(t, st, zfsRoot)
	host.destroyErr = context.DeadlineExceeded
	s.shuttingDown.Store(true)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	stamp := host.snapshotNames()
	if len(stamp) != 1 {
		t.Fatalf("snapshots left on the host = %v, want the one the destroy could not remove", stamp)
	}

	s.shuttingDown.Store(false)
	host.destroyErr = nil
	host.snaps = []zfs.SnapshotEntry{{Dataset: zfsRoot, Name: stamp[0]}}
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	s.SweepZFSLeftoversOnStartup(context.Background())

	if len(host.snapshotNames()) != 0 {
		t.Fatalf("snapshots left = %v, want the stamp destroyed", host.snapshotNames())
	}
	if !strings.Contains(logged.String(), stamp[0]) {
		t.Fatalf("log = %q, want the stamp named", logged.String())
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.LeftoverCount != 0 {
		t.Fatalf("leftover count = %d, want the row cleared", row.LeftoverCount)
	}
}
