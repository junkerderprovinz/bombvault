package api

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var shipLog = fortress.Caller{ID: "s1", App: "shiplog", Device: "tower"}

// requestRig is a BombVault whose container backups start as start says and
// leave a run row behind when they do.
type requestRig struct {
	svc    *Service
	st     *store.Repo
	target store.Target
	starts atomic.Int32
	// running is the run a backup holds open, empty between backups.
	running string
}

func newRequestRig(t *testing.T, start func(n int32) (bool, error)) *requestRig {
	t.Helper()
	svc, st := newFortressSvc(t)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "nextcloud", AppdataPaths: []string{"/data"}})
	if err != nil {
		t.Fatal(err)
	}
	rig := &requestRig{svc: svc, st: st, target: tg}
	svc.docker = namespaceDocker{live: map[string]model.Inspect{"nextcloud": {}}}
	svc.house.retry = 10 * time.Millisecond
	svc.house.start = func(_ context.Context, name string) (bool, error) {
		if name != "nextcloud" {
			t.Errorf("started %q", name)
		}
		return start(rig.starts.Add(1))
	}
	return rig
}

func (r *requestRig) ask(t *testing.T, body string) map[string]any {
	t.Helper()
	out, err := r.svc.backupRequest(context.Background(), shipLog, []byte(body))
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return decodeAnswer(t, out)
}

func (r *requestRig) state(t *testing.T, run string) map[string]any {
	t.Helper()
	return r.ask(t, `{"run":"`+run+`"}`)
}

// waitFor asks about run until its state is want.
func (r *requestRig) waitFor(t *testing.T, run, want string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := r.state(t, run)
		if got["state"] == want {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s is %v, want %s", run, got, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// finish moves the container's backup on to status, opening its run first.
func (r *requestRig) finish(t *testing.T, status, snapshot, errMsg string) {
	t.Helper()
	if r.running == "" {
		id, err := r.st.StartRun(r.target.ID, "backup")
		if err != nil {
			t.Fatal(err)
		}
		r.running = id
	}
	if status == "running" {
		return
	}
	if err := r.st.FinishRun(r.running, status, snapshot, 1, errMsg); err != nil {
		t.Fatal(err)
	}
	r.running = ""
}

func TestABackupRequestAnswersARunThatEndsWithItsSnapshotAndNoAddress(t *testing.T) {
	rig := newRequestRig(t, func(int32) (bool, error) { return true, nil })
	run, _ := rig.ask(t, `{"container":"nextcloud","reason":"update"}`)["run"].(string)
	if run == "" {
		t.Fatal("no run id")
	}
	rig.waitFor(t, run, "running")

	rig.finish(t, "running", "", "")
	if got := rig.state(t, run); got["state"] != "running" {
		t.Fatalf("while the backup runs: %v", got)
	}
	rig.finish(t, "success", "4f3a9c21", "")
	got := rig.waitFor(t, run, "done")
	if _, link := got["link"]; got["id"] != run || got["snapshot"] != "4f3a9c21" || link {
		t.Fatalf("a finished request: %v", got)
	}
}

func TestABackupRequestWaitsForTheBackupThatRuns(t *testing.T) {
	rig := newRequestRig(t, func(n int32) (bool, error) { return n > 2, nil })
	run, _ := rig.ask(t, `{"container":"nextcloud","reason":"maintenance"}`)["run"].(string)
	if got := rig.state(t, run); got["state"] != "queued" && got["state"] != "running" {
		t.Fatalf("while another backup runs: %v", got)
	}
	rig.waitFor(t, run, "running")
	if n := rig.starts.Load(); n != 3 {
		t.Fatalf("tried %d times, want 3", n)
	}
}

func TestABackupRequestThatFailsSaysWhy(t *testing.T) {
	rig := newRequestRig(t, func(int32) (bool, error) { return true, nil })
	run, _ := rig.ask(t, `{"container":"nextcloud","reason":"update"}`)["run"].(string)
	rig.waitFor(t, run, "running")
	rig.finish(t, "failed", "", "restic backup failed")
	if got := rig.waitFor(t, run, "failed"); got["error"] != "restic backup failed" {
		t.Fatalf("a failed backup: %v", got)
	}

	refused := newRequestRig(t, func(int32) (bool, error) { return false, errors.New("no storage location") })
	run, _ = refused.ask(t, `{"container":"nextcloud","reason":"update"}`)["run"].(string)
	if got := refused.waitFor(t, run, "failed"); got["error"] != "no storage location" {
		t.Fatalf("a backup that could not start: %v", got)
	}
}

func TestABackupRequestRefusesWhatItCannotDo(t *testing.T) {
	rig := newRequestRig(t, func(int32) (bool, error) { return true, nil })
	for body, want := range map[string][2]any{
		`{"container":"plex","reason":"update"}`:     {404, "unknown_container"},
		`{"container":"nextcloud","reason":"party"}`: {400, "unknown_reason"},
		`{"run":"0123456789abcdef"}`:                 {404, "unknown_run"},
		`{"container":`:                              {400, "bad_request"},
		`{"reason":"update"}`:                        {404, "unknown_container"},
	} {
		_, err := rig.svc.backupRequest(context.Background(), shipLog, []byte(body))
		refusedWith(t, err, want[0].(int), want[1].(string))
	}
	if n := rig.starts.Load(); n != 0 {
		t.Fatalf("a refused request started %d backups", n)
	}
}

// A container BombVault has not backed up yet is still on the host, so a
// request backs it up for the first time.
func TestABackupRequestBacksUpAContainerWithoutBackupsYet(t *testing.T) {
	svc, st := newFortressSvc(t)
	svc.docker = namespaceDocker{live: map[string]model.Inspect{"jellyfin": {}}}
	svc.house.retry = 10 * time.Millisecond
	var started atomic.Value
	svc.house.start = func(_ context.Context, name string) (bool, error) {
		started.Store(name)
		return true, nil
	}
	out, err := svc.backupRequest(context.Background(), shipLog, []byte(`{"container":"jellyfin","reason":"update"}`))
	if err != nil {
		t.Fatal(err)
	}
	run, _ := decodeAnswer(t, out)["run"].(string)
	rig := &requestRig{svc: svc, st: st}
	rig.waitFor(t, run, "running")
	if started.Load() != "jellyfin" {
		t.Fatalf("started %v", started.Load())
	}

	tg, err := st.UpsertTarget(store.Target{ContainerName: "jellyfin", AppdataPaths: []string{"/data"}})
	if err != nil {
		t.Fatal(err)
	}
	rig.target = tg
	rig.finish(t, "success", "9b1e44d0", "")
	if got := rig.waitFor(t, run, "done"); got["snapshot"] != "9b1e44d0" {
		t.Fatalf("the first backup of a container: %v", got)
	}
}
