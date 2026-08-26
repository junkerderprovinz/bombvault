package api_test

// Tests for Task 4 of the "Backup Everything" plan (docs/superpowers/plans/
// 2026-08-20-backup-everything.md): BackupEverything/StartBackupEverything
// (internal/api/everything.go), the core sequential orchestration over
// containers → vms → flash → files → config plus the global pre/post hooks.
//
// SCOPE NOTE: sequencing/failure-survival/hook-timing (tests 1-3 below) are
// exercised over containers + flash + config only — the cheapest domains to
// fake, per the plan's own explicit allowance when wiring ALL FIVE real
// domains (VM/libvirt fakes especially) into one test is impractical. The
// production BackupEverything code is NOT special-cased for this: it always
// attempts all five domains unconditionally. vms/files are exercised for
// real, at ZERO eligible items (no VM targets or file sets are ever
// registered by everythingTestService below), which is itself a genuine,
// common production path (an operator who hasn't set up VMs/file sets yet)
// and is asserted to count as that domain's own "ok" outcome (design spec,
// decision 3), not skipped from the pass. Group-stamping (test 5) and
// StartBackupEverything re-entrancy (test 6) are unaffected by this scoping
// and are fully exercised.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// everythingOrderLog is a shared, ordered event trail appended to by both
// orderedEngine (below) and everythingFakeHostShell — since BackupEverything
// runs entirely on the calling goroutine with no concurrency of its own, the
// order entries land in is exactly the pass's real execution order, letting a
// single assertion cover both "domains run in the right order" and "the
// pre-hook fires before any domain step, the post-hook fires after all of
// them".
type everythingOrderLog struct {
	entries []string
}

func (o *everythingOrderLog) add(s string) { o.entries = append(o.entries, s) }

// orderedEngine wraps a *fakeResticEngine (service_test.go's shared restic
// fake) to also append a domain label to a shared everythingOrderLog on every
// Backup call, identifying which repo (containers/flash/config) the call was
// for by substring match on the repo path built from ContainersPath/
// FlashPath/ConfigPath in everythingTestService below. Every other
// ResticEngine method is promoted from the embedded *fakeResticEngine
// unchanged (Go method promotion), so this satisfies api.ResticEngine in
// full without reimplementing it.
type orderedEngine struct {
	*fakeResticEngine
	log *everythingOrderLog
}

func (e *orderedEngine) Backup(ctx context.Context, repo string, paths, tags []string, mode restic.Mode, excludes ...string) (restic.Summary, error) {
	switch {
	case strings.Contains(repo, "containers"):
		e.log.add("containers")
	case strings.Contains(repo, "flash"):
		e.log.add("flash")
	case strings.Contains(repo, "config"):
		e.log.add("config")
	default:
		e.log.add("backup:" + repo)
	}
	return e.fakeResticEngine.Backup(ctx, repo, paths, tags, mode, excludes...)
}

// everythingFakeHostShell is a minimal HostShell fake (see hostshell.go's
// interface) for this file: it records every command it was asked to run, in
// order, and — when wired with a shared everythingOrderLog — appends a
// "hook:<cmd>" entry too, so hook timing can be asserted against domain
// timing in one combined trail.
type everythingFakeHostShell struct {
	log   *everythingOrderLog
	calls []string
}

var _ api.HostShell = (*everythingFakeHostShell)(nil)

func (f *everythingFakeHostShell) Run(_ context.Context, cmd string) error {
	f.calls = append(f.calls, cmd)
	if f.log != nil {
		f.log.add("hook:" + cmd)
	}
	return nil
}

// everythingTestService builds a Service wired for containers + flash +
// config (see the file-level scope note): one container target ("primary")
// with an explicit SelectedPaths folder that actually exists on disk — a real
// (not "stateless"/definition-only) backup, so the containers domain step
// genuinely reaches Restic.Backup and can be observed/blocked via eng — a
// real /boot mount for the flash domain, and ConfigPath/ConfigEnabled for the
// config (self) domain. No VM targets or file sets are ever registered, so
// those two domains always run for real at zero eligible items (see the
// file-level scope note).
func everythingTestService(t *testing.T, eng api.ResticEngine) (svc *api.Service, st *store.Repo, docker *fakeServiceDocker, tg store.Target) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	flashDir := root + "/boot"
	if err := os.MkdirAll(flashDir, 0o750); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(dir, "data", "primary")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     root,
		FlashDir:          flashDir,
		FlashTemplatesDir: filepath.Join(dir, "flashtpl"),
	}
	st = newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.FlashPath = "backups/flash"
	s.ConfigPath = "backups/config"
	s.ConfigEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// Seed the containers repo so EnsureRepo passes cleanly.
	repo := filepath.Join(dir, "backups", "containers")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	docker = &fakeServiceDocker{}
	svc = api.NewService(cfg, st, docker, fakeVirsh{}, eng)

	var err error
	tg, err = st.UpsertTarget(store.Target{
		ContainerName:     "primary",
		IncludeInSchedule: true,
		SelectedPaths:     []string{dataDir},
	})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
	return svc, st, docker, tg
}

// waitForEverythingDone blocks until StartBackupEverything's background
// goroutine has released the everythingActive guard, mirroring
// waitForBackupDone (handlers_test.go) for batchActive.
func waitForEverythingDone(t *testing.T, svc *api.Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !svc.EverythingInProgress() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the Backup Everything pass to finish")
}

// TestBackupEverythingOrder pins requirement 1: containers before vms before
// flash before files before config. vms/files have zero eligible items in
// this harness (see the file-level scope note), so the observable order log
// is exactly [containers, flash, config].
func TestBackupEverythingOrder(t *testing.T) {
	log := &everythingOrderLog{}
	oe := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, _, _, _ := everythingTestService(t, oe)

	if _, err := svc.BackupEverything(context.Background()); err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	want := []string{"containers", "flash", "config"}
	if len(log.entries) != len(want) {
		t.Fatalf("order log = %v, want %v", log.entries, want)
	}
	for i, w := range want {
		if log.entries[i] != w {
			t.Fatalf("order log[%d] = %q, want %q (full log: %v)", i, log.entries[i], w, log.entries)
		}
	}
}

// TestBackupEverythingHooksFireExactlyOnce pins requirement 2: the pre-hook
// fires at most once, before any domain step; the post-hook fires exactly
// once, after every domain step. Both are asserted against the SAME shared
// order log the domain calls append to, so the ordering claim is verified,
// not just the call count.
func TestBackupEverythingHooksFireExactlyOnce(t *testing.T) {
	log := &everythingOrderLog{}
	oe := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, st, _, _ := everythingTestService(t, oe)

	shell := &everythingFakeHostShell{log: log}
	svc.SetHostShell(shell)

	s := mustSettings(t, st)
	s.EverythingPreHook = "echo pre"
	s.EverythingPostHook = "echo post"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.BackupEverything(context.Background()); err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	if len(shell.calls) != 2 || shell.calls[0] != "echo pre" || shell.calls[1] != "echo post" {
		t.Fatalf("hostShell calls = %v, want exactly [echo pre, echo post]", shell.calls)
	}

	want := []string{"hook:echo pre", "containers", "flash", "config", "hook:echo post"}
	if len(log.entries) != len(want) {
		t.Fatalf("combined order log = %v, want %v", log.entries, want)
	}
	for i, w := range want {
		if log.entries[i] != w {
			t.Fatalf("combined order log[%d] = %q, want %q (full log: %v)", i, log.entries[i], w, log.entries)
		}
	}
}

// TestBackupEverythingSurvivesOneDomainFailing pins requirement 3: a domain
// that fails entirely (here, containers — every Docker Inspect call errors)
// does not abort the remaining domains (flash and config both still run,
// observed via the shared order log), the post-hook still fires exactly
// once, and the parent run's status is "failed" with a breakdown naming the
// failing domain and its reason.
func TestBackupEverythingSurvivesOneDomainFailing(t *testing.T) {
	log := &everythingOrderLog{}
	oe := &orderedEngine{fakeResticEngine: &fakeResticEngine{}, log: log}
	svc, st, docker, _ := everythingTestService(t, oe)

	shell := &everythingFakeHostShell{log: log}
	svc.SetHostShell(shell)

	s := mustSettings(t, st)
	s.EverythingPostHook = "echo post"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	docker.inspectErr = errors.New("simulated docker inspect failure")

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	if sum.Status != "failed" {
		t.Fatalf("Status = %q, want %q (domains: %+v)", sum.Status, "failed", sum.Domains)
	}
	if !strings.Contains(sum.Error, "containers") || !strings.Contains(sum.Error, "simulated docker inspect failure") {
		t.Fatalf("breakdown %q does not name the failing domain/reason", sum.Error)
	}

	// The REMAINING domains still ran, in order, despite containers failing.
	want := []string{"flash", "config", "hook:echo post"}
	if len(log.entries) != len(want) {
		t.Fatalf("order log = %v, want %v (flash/config must still run, post-hook must still fire)", log.entries, want)
	}
	for i, w := range want {
		if log.entries[i] != w {
			t.Fatalf("order log[%d] = %q, want %q (full log: %v)", i, log.entries[i], w, log.entries)
		}
	}
	if len(shell.calls) != 1 || shell.calls[0] != "echo post" {
		t.Fatalf("post-hook calls = %v, want exactly one [echo post]", shell.calls)
	}
}

// TestBackupEverythingAllCleanPass pins requirement 4 (clean half): every
// domain succeeding (including vms/files at zero eligible items) yields
// parent run status "success" and an empty Error.
func TestBackupEverythingAllCleanPass(t *testing.T) {
	svc, _, _, _ := everythingTestService(t, &fakeResticEngine{})

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	if sum.Status != "success" {
		t.Fatalf("Status = %q, want %q (domains: %+v)", sum.Status, "success", sum.Domains)
	}
	if sum.Error != "" {
		t.Fatalf("Error = %q, want empty on a clean pass", sum.Error)
	}
}

// TestBackupEverythingGroupStamping pins requirement 5: the child run the
// containers domain step produces carries GroupID == the parent run's id
// (EverythingSummary.RunID), tying Task 3's group-stamp mechanism to the real
// BackupEverything call path.
func TestBackupEverythingGroupStamping(t *testing.T) {
	svc, st, _, tg := everythingTestService(t, &fakeResticEngine{})

	sum, err := svc.BackupEverything(context.Background())
	if err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}
	if sum.RunID == "" {
		t.Fatal("expected a non-empty parent run id")
	}

	run, err := st.LastRunForTarget(tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run == nil {
		t.Fatal("expected a recorded container run")
	}
	if run.GroupID != sum.RunID {
		t.Fatalf("child run GroupID = %q, want the parent run id %q", run.GroupID, sum.RunID)
	}
}

// TestStartBackupEverythingRefusesConcurrent pins requirement 6: a second
// StartBackupEverything call while one is already in flight returns
// (false, nil), mirroring StartBackupAll's busy-refusal contract. The fake
// engine's block channel holds the first pass inside the containers domain's
// real Restic.Backup call (see everythingTestService: the "primary" target
// has a genuine, existing SelectedPaths folder) so the pass is deterministicly
// still in flight when the second call is made.
func TestStartBackupEverythingRefusesConcurrent(t *testing.T) {
	eng := &fakeResticEngine{block: make(chan struct{})}
	svc, _, _, _ := everythingTestService(t, eng)

	started1, err1 := svc.StartBackupEverything(context.Background())
	if err1 != nil || !started1 {
		t.Fatalf("first StartBackupEverything should start: started=%v err=%v", started1, err1)
	}

	started2, err2 := svc.StartBackupEverything(context.Background())
	if err2 != nil || started2 {
		t.Fatalf("second StartBackupEverything while one is in flight should be refused: started=%v err=%v", started2, err2)
	}

	close(eng.block)
	waitForEverythingDone(t, svc)
}
