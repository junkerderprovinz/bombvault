package api

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsStopFixture is the run fixture with a Docker fake wired in and an item
// that holds the named containers for the snapshot instant.
func zfsStopFixture(t *testing.T, dock *zfsFakeDocker, stop ...string) (*Service, *store.Repo, *fakeZFSHost, store.ZFSDataset) {
	t.Helper()
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	s.docker = dock
	d := zfsSeedItem(t, st, zfsRoot)
	if err := st.SetZFSDatasetStopContainers(d.ID, stop); err != nil {
		t.Fatalf("set stop list: %v", err)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, st, host, row
}

// callIndex is where a call sits in the recorded order, or -1.
func callIndex(calls []string, want string) int {
	for i, c := range calls {
		if c == want {
			return i
		}
	}
	return -1
}

func TestZFSConsistencyStopsOnlyRunningAndRestartsInOrder(t *testing.T) {
	dock := newZFSFakeDocker("db", "web", "idle")
	dock.containers["web"].dependsOn = "db"
	dock.containers["idle"].running = false
	s, st, _, d := zfsStopFixture(t, dock, "db", "web", "idle")

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}

	calls := dock.recorded()
	stopWeb, stopDB := callIndex(calls, "stop:web"), callIndex(calls, "stop:db")
	startDB, startWeb := callIndex(calls, "start:db"), callIndex(calls, "start:web")
	if stopWeb < 0 || stopDB < 0 || stopWeb > stopDB {
		t.Fatalf("calls = %v, want web stopped before the database it depends on", calls)
	}
	if startDB < 0 || startWeb < 0 || startDB > startWeb {
		t.Fatalf("calls = %v, want the database started before web", calls)
	}
	if callIndex(calls, "stop:idle") >= 0 || callIndex(calls, "start:idle") >= 0 {
		t.Fatalf("calls = %v, want a container the user stopped left alone", calls)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.RestartPending) != 0 {
		t.Fatalf("restart marker = %v, want it cleared", row.RestartPending)
	}
	run := zfsLastRun(t, st, d.ID)
	detail, err := st.GetZFSRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.WindowSeconds < 0 {
		t.Fatalf("window = %d, want the stop window measured", detail.WindowSeconds)
	}
}

func TestZFSConsistencyWritesMarkerBeforeFirstStop(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	s, st, _, d := zfsStopFixture(t, dock, "db", "web")

	var atFirstStop []string
	var once sync.Once
	dock.onStop = func(string) {
		once.Do(func() {
			row, err := st.GetZFSDataset(d.ID)
			if err != nil {
				t.Errorf("read the row during the stop: %v", err)
				return
			}
			atFirstStop = row.RestartPending
		})
	}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if len(atFirstStop) != 2 {
		t.Fatalf("marker at the first stop = %v, want both containers", atFirstStop)
	}
}

func TestZFSConsistencyStopFailureRestartsAndSkipsSnapshot(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	dock.containers["web"].dependsOn = "db"
	dock.containers["db"].stopErr = errors.New("device or resource busy")
	s, st, host, d := zfsStopFixture(t, dock, "db", "web")

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a stop that fails must fail the run")
	}
	if hostDid(host, "snapshot -r") {
		t.Fatalf("host calls = %v, want no snapshot after a failed stop", host.recorded())
	}
	if callIndex(dock.recorded(), "start:web") < 0 {
		t.Fatalf("calls = %v, want the container that went down started again", dock.recorded())
	}
	run := zfsLastRun(t, st, d.ID)
	if run.Status != "failed" || !strings.Contains(run.Error, "consistency-stop-failed") {
		t.Fatalf("run = %q / %q, want a failure coded consistency-stop-failed", run.Status, run.Error)
	}
	if !strings.Contains(run.Error, "db") {
		t.Fatalf("run error = %q, want the container named", run.Error)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.RestartPending) != 0 {
		t.Fatalf("restart marker = %v, want it cleared", row.RestartPending)
	}
}

func TestZFSConsistencyStopThatOutlivesItsCallStillCountsAsStopped(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	dock.containers["db"].stopErr = context.DeadlineExceeded
	dock.containers["db"].stopsAnyway = true
	s, st, _, d := zfsStopFixture(t, dock, "db", "web")

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("a container that did stop must not fail the run: %v", err)
	}
	if callIndex(dock.recorded(), "start:db") < 0 {
		t.Fatalf("calls = %v, want the container that went down started again", dock.recorded())
	}
	if !dock.containers["db"].running {
		t.Fatal("db was left stopped")
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.RestartPending) != 0 {
		t.Fatalf("restart marker = %v, want it cleared once db runs again", row.RestartPending)
	}
}

func TestZFSConsistencyStopOutlastsTheGraceAndTheCancel(t *testing.T) {
	dock := newZFSFakeDocker("db")
	s, _, _, d := zfsStopFixture(t, dock, "db")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dock.onStop = func(string) { cancel() }

	_, _ = s.BackupZFSDataset(ctx, d.ID)

	if len(dock.stopCtxErrs) != 1 || dock.stopCtxErrs[0] != nil {
		t.Fatalf("stop context errors = %v, want a stop a cancel cannot cut short", dock.stopCtxErrs)
	}
	if len(dock.stopBudgets) != 1 || dock.stopBudgets[0] <= zfsStopTimeout {
		t.Fatalf("stop budgets = %v, want more than the %v grace the daemon waits before it kills", dock.stopBudgets, zfsStopTimeout)
	}
	if callIndex(dock.recorded(), "start:db") < 0 {
		t.Fatalf("calls = %v, want db started again after the cancelled run", dock.recorded())
	}
}

func TestZFSConsistencyTimesOutOnBusyContainersLock(t *testing.T) {
	dock := newZFSFakeDocker("db")
	s, st, host, d := zfsStopFixture(t, dock, "db")

	previous := zfsManualLockWait
	t.Cleanup(func() { zfsManualLockWait = previous })
	zfsManualLockWait = 20 * time.Millisecond

	release := s.lockDomainFor("containers", "backup")
	defer release()

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a busy containers domain must fail the run")
	}
	if callIndex(dock.recorded(), "stop:db") >= 0 {
		t.Fatalf("calls = %v, want nothing stopped while container backups run", dock.recorded())
	}
	if hostDid(host, "snapshot -r") {
		t.Fatalf("host calls = %v, want no snapshot", host.recorded())
	}
	run := zfsLastRun(t, st, d.ID)
	if !strings.Contains(run.Error, "containers-busy") {
		t.Fatalf("run error = %q, want it coded containers-busy", run.Error)
	}
}

func TestZFSConsistencyGetsLockBetweenRelockingContainerBackups(t *testing.T) {
	s, _, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())

	// A container batch takes and releases the domain lock once per container.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			s.lockDomainFor("containers", "backup")()
		}
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
	})

	unlock, ok := s.lockWithin(context.Background(), "containers", "zfs-consistency", 5*time.Second)
	if !ok {
		t.Fatal("the window never got the lock between two container backups")
	}
	unlock()
}

func TestZFSConsistencyWaitForContainersEndsOnCancel(t *testing.T) {
	s, _, _, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	release := s.lockDomainFor("containers", "backup")

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	done := make(chan bool, 1)
	go func() {
		_, ok := s.lockWithin(ctx, "containers", "zfs-consistency", time.Hour)
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("the wait reported the lock although the holder never let go")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled run kept waiting for the containers domain")
	}

	release()
	unlock, ok := s.lockWithin(context.Background(), "containers", "backup", 5*time.Second)
	if !ok {
		t.Fatal("the abandoned wait kept the containers domain locked")
	}
	unlock()
}

func TestZFSConsistencyStopsLevelInParallel(t *testing.T) {
	dock := newZFSFakeDocker("one", "two")
	s, _, _, d := zfsStopFixture(t, dock, "one", "two")

	var arrived atomic.Int32
	var serial atomic.Bool
	both := make(chan struct{})
	dock.onStop = func(string) {
		if arrived.Add(1) == 2 {
			close(both)
		}
		select {
		case <-both:
		case <-time.After(2 * time.Second):
			serial.Store(true)
		}
	}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if serial.Load() {
		t.Fatal("the containers of one dependency level were stopped one after another")
	}
}

func TestZFSConsistencyRefusesSelf(t *testing.T) {
	dock := newZFSFakeDocker("bombvault")
	dock.self = "bombvault"
	s, st, host, d := zfsStopFixture(t, dock, "bombvault")

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("stopping BombVault itself must fail the run")
	}
	if callIndex(dock.recorded(), "stop:bombvault") >= 0 {
		t.Fatalf("calls = %v, want BombVault left running", dock.recorded())
	}
	if hostDid(host, "snapshot -r") {
		t.Fatalf("host calls = %v, want no snapshot", host.recorded())
	}
	run := zfsLastRun(t, st, d.ID)
	if !strings.Contains(run.Error, "container-is-self") {
		t.Fatalf("run error = %q, want it coded container-is-self", run.Error)
	}
}

func TestZFSConsistencyClearsMarkerOnlyWhenAllRunning(t *testing.T) {
	dock := newZFSFakeDocker("db", "web")
	dock.containers["web"].startLeavesDown = true
	s, st, _, d := zfsStopFixture(t, dock, "db", "web")
	messages := zfsCaptureNotifications(t, s)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	row, err := st.GetZFSDataset(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.RestartPending) != 1 || row.RestartPending[0] != "web" {
		t.Fatalf("restart marker = %v, want only the container still down", row.RestartPending)
	}
	if !zfsAnyMessageContains(messages(), "web") {
		t.Fatalf("notifications = %v, want the container that stayed down named", messages())
	}
}

func TestZFSHooksPreFailureFailsRun(t *testing.T) {
	dock := newZFSFakeDocker("db")
	dock.execErr = errors.New("exit status 1: mysqldump: access denied")
	s, st, host, d := zfsStopFixture(t, dock)
	if err := st.SetZFSHooks(d.ID, "db", "mysqldump -A > /data/dump.sql", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err == nil {
		t.Fatal("a failing pre-snapshot command must fail the run")
	}
	if hostDid(host, "snapshot -r") {
		t.Fatalf("host calls = %v, want no snapshot after a failed pre-snapshot command", host.recorded())
	}
	run := zfsLastRun(t, st, d.ID)
	if run.Status != "failed" || !strings.Contains(run.Error, "pre-snapshot-failed") {
		t.Fatalf("run = %q / %q, want a failure coded pre-snapshot-failed", run.Status, run.Error)
	}
}

func TestZFSHooksPostFailureIsRecordedOnTheRun(t *testing.T) {
	dock := newZFSFakeDocker("db")
	dock.execErr = errors.New("exit status 2: no such file")
	s, st, _, d := zfsStopFixture(t, dock)
	if err := st.SetZFSHooks(d.ID, "db", "", "rm /data/dump.sql"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("a failing post-snapshot command must leave the backup standing: %v", err)
	}
	run := zfsLastRun(t, st, d.ID)
	detail, err := st.GetZFSRun(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(detail.HookDetail, "no such file") {
		t.Fatalf("hook detail = %q, want the command's error kept on the run", detail.HookDetail)
	}
}
