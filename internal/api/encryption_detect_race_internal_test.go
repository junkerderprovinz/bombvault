package api

// One dead off-site host keeps a detection probing for a minute, and the
// Recovery page starts one on mount, so a settings save during the probe is
// the normal case. Detection determines only EncryptionEnabled, and these
// tests check that it leaves everything else the user saved meanwhile alone.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// gatedEngine signals the start of the first probe and holds it, like a dead
// backend, until the test releases it.
type gatedEngine struct {
	modeStubEngine
	started chan struct{}
	release chan struct{}
	probes  int
}

func (e *gatedEngine) RepoOpensErr(ctx context.Context, repo string, m restic.Mode) error {
	e.probes++
	if e.probes == 1 {
		close(e.started)
		<-e.release
	}
	return e.modeStubEngine.RepoOpensErr(ctx, repo, m)
}

func TestDetectEncryptionKeepsConcurrentSettingsSave(t *testing.T) {
	eng := &gatedEngine{
		modeStubEngine: modeStubEngine{encrypted: map[string]bool{}},
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = true

	// The stored mode is wrong, so the detection has something to apply.
	settings := setPaths(t, st, map[string]string{"containers": "backups/containers"})
	settings.EncryptionEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	type result struct {
		det EncryptionDetection
		err error
	}
	done := make(chan result, 1)
	go func() {
		det, err := s.DetectEncryption(context.Background())
		done <- result{det, err}
	}()

	// The probe is in flight and stuck, like a dead sftp host.
	select {
	case <-eng.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never started")
	}

	// The user saves new paths in the wizard and another tab sets a login
	// password while the probe is still running.
	if _, err := st.MutateSettings(func(cur *store.Settings) error {
		cur.VMsPath = "user/backups/vms-CHOSEN-BY-THE-USER"
		cur.FilesPath = "user/backups/files-CHOSEN-BY-THE-USER"
		cur.RestoreFolder = "user/restores"
		cur.AuthPasswordHash = "set-while-the-probe-was-running"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	close(eng.release)

	var got result
	select {
	case got = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("DetectEncryption never returned")
	}
	if got.err != nil {
		t.Fatalf("detect: %v", got.err)
	}
	if got.det.Verdict != VerdictEncrypted || !got.det.Applied {
		t.Fatalf("verdict = %q applied = %v, want encrypted+applied", got.det.Verdict, got.det.Applied)
	}

	after, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !after.EncryptionEnabled {
		t.Fatal("the detected mode was not applied")
	}
	if after.VMsPath != "user/backups/vms-CHOSEN-BY-THE-USER" {
		t.Fatalf("VMsPath = %q; the save made during the probe was reverted", after.VMsPath)
	}
	if after.FilesPath != "user/backups/files-CHOSEN-BY-THE-USER" {
		t.Fatalf("FilesPath = %q; the save made during the probe was reverted", after.FilesPath)
	}
	if after.RestoreFolder != "user/restores" {
		t.Fatalf("RestoreFolder = %q; the save made during the probe was reverted", after.RestoreFolder)
	}
	if after.AuthPasswordHash != "set-while-the-probe-was-running" {
		t.Fatalf("AuthPasswordHash = %q; a password set during the probe was reverted and auth turned back off", after.AuthPasswordHash)
	}
	// The detection reports the row as it stands after the save, not the
	// snapshot it read before the probe.
	if got.det.EncryptionEnabled != after.EncryptionEnabled {
		t.Fatalf("reported encryptionEnabled = %v but the row holds %v", got.det.EncryptionEnabled, after.EncryptionEnabled)
	}
}

// An undecided detection must not write the row it read either, or it would
// still revert a concurrent save.
func TestDetectEncryptionUndecidedWritesNothing(t *testing.T) {
	eng := &gatedEngine{
		modeStubEngine: modeStubEngine{
			encrypted: map[string]bool{},
			failErr: map[string]error{
				"rest:http://offsite:8000/repo": errors.New("server response unexpected: 401 Unauthorized"),
			},
		},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	s, st, _ := newDetectSvc(t, eng)
	setPaths(t, st, map[string]string{"containersOffsite": "rest:http://offsite:8000/repo"})

	done := make(chan EncryptionDetection, 1)
	go func() {
		det, err := s.DetectEncryption(context.Background())
		if err != nil {
			t.Errorf("detect: %v", err)
		}
		done <- det
	}()

	select {
	case <-eng.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never started")
	}
	if _, err := st.MutateSettings(func(cur *store.Settings) error {
		cur.InstanceName = "renamed-during-an-undecidable-probe"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	close(eng.release)

	select {
	case det := <-done:
		if det.Applied {
			t.Fatal("an undecidable probe must never write the setting")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("DetectEncryption never returned")
	}

	after, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.InstanceName != "renamed-during-an-undecidable-probe" {
		t.Fatalf("InstanceName = %q; an undecided detection still clobbered a concurrent save", after.InstanceName)
	}
}

// Two callers at once, such as two tabs or a reload mid-probe, share one pass.
func TestDetectEncryptionSharesOneProbePass(t *testing.T) {
	eng := &gatedEngine{
		modeStubEngine: modeStubEngine{encrypted: map[string]bool{}},
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = true
	setPaths(t, st, map[string]string{"containers": "backups/containers"})

	first := make(chan EncryptionDetection, 1)
	go func() {
		det, err := s.DetectEncryption(context.Background())
		if err != nil {
			t.Errorf("leader detect: %v", err)
		}
		first <- det
	}()
	select {
	case <-eng.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never started")
	}

	second := make(chan EncryptionDetection, 1)
	go func() {
		det, err := s.DetectEncryption(context.Background())
		if err != nil {
			t.Errorf("follower detect: %v", err)
		}
		second <- det
	}()
	// Give the follower time to join or start its own pass; a pass of its own
	// would show up in the probe count below.
	time.Sleep(50 * time.Millisecond)
	close(eng.release)

	var a, b EncryptionDetection
	select {
	case a = <-first:
	case <-time.After(10 * time.Second):
		t.Fatal("the leading DetectEncryption never returned")
	}
	select {
	case b = <-second:
	case <-time.After(10 * time.Second):
		t.Fatal("the following DetectEncryption never returned")
	}

	if a.Verdict != b.Verdict || a.EncryptionEnabled != b.EncryptionEnabled {
		t.Fatalf("the two callers disagree: %+v vs %+v", a, b)
	}
	// The repository opens under the first (encrypted) probe, so one pass
	// probes once.
	if eng.probes != 1 {
		t.Fatalf("%d probes; two concurrent callers must share one pass, not run one each", eng.probes)
	}
}

// detectResult is what one DetectEncryption caller got back.
type detectResult struct {
	det EncryptionDetection
	err error
}

// cancelAwareEngine holds the first probe like a slow backend, then answers as
// restic does when the probe's context died: the wrapped context error matches
// no absence wording, so the repository reads as unreachable.
type cancelAwareEngine struct {
	modeStubEngine
	started chan struct{}
	release chan struct{}
	gate    sync.Once
}

func (e *cancelAwareEngine) RepoOpensErr(ctx context.Context, repo string, m restic.Mode) error {
	e.gate.Do(func() {
		close(e.started)
		<-e.release
	})
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("restic cat cancelled: %w", err)
	}
	return e.modeStubEngine.RepoOpensErr(ctx, repo, m)
}

// The leader of a shared pass is whoever arrived first. When its tab closes
// mid-probe the pass carries on, or a live follower would be told "unknown"
// for a repository that opens fine.
func TestDetectEncryptionSurvivesTheLeadersCancelledRequest(t *testing.T) {
	eng := &cancelAwareEngine{
		modeStubEngine: modeStubEngine{encrypted: map[string]bool{}},
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = true

	settings := setPaths(t, st, map[string]string{"containers": "backups/containers"})
	settings.EncryptionEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()
	leader := make(chan detectResult, 1)
	go func() {
		det, err := s.DetectEncryption(leaderCtx)
		leader <- detectResult{det, err}
	}()
	select {
	case <-eng.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never started")
	}

	// A second tab joins the pass already running.
	follower := make(chan detectResult, 1)
	go func() {
		det, err := s.DetectEncryption(context.Background())
		follower <- detectResult{det, err}
	}()
	time.Sleep(50 * time.Millisecond)

	// The tab that started it goes away first, and only then does the backend answer.
	cancelLeader()
	close(eng.release)

	var got detectResult
	select {
	case got = <-follower:
	case <-time.After(10 * time.Second):
		t.Fatal("the following DetectEncryption never returned")
	}
	if got.err != nil {
		t.Fatalf("follower detect: %v", got.err)
	}
	if got.det.Verdict != VerdictEncrypted {
		t.Fatalf("follower verdict = %q (repos: %+v); the leader's disconnect cancelled the shared probes, so a live caller was told the repositories are unreachable", got.det.Verdict, got.det.Repos)
	}
	if !got.det.EncryptionEnabled {
		t.Fatal("the detected mode reached no one: a cancelled leader must not cost the followers their answer")
	}

	select {
	case <-leader:
	case <-time.After(10 * time.Second):
		t.Fatal("the leading DetectEncryption never returned")
	}

	after, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !after.EncryptionEnabled {
		t.Fatal("the detected mode was never applied; the pass died with its leader's request")
	}
}

// A panicking leader must still clear the single-flight slot and release its
// followers with an error. Otherwise detection stays dead for the life of the
// process and the followers wait forever.
func TestDetectEncryptionPanicOnTheLeaderReleasesTheFlight(t *testing.T) {
	eng := &gatedEngine{
		modeStubEngine: modeStubEngine{encrypted: map[string]bool{}},
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = true
	setPaths(t, st, map[string]string{"containers": "backups/containers"})

	panicked := make(chan struct{})
	go func() {
		defer func() {
			if recover() == nil {
				t.Error("the leading pass was supposed to panic")
			}
			close(panicked)
		}()
		_, _ = s.DetectEncryption(context.Background())
	}()
	select {
	case <-eng.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never started")
	}

	// A second tab joins the pass that is about to die.
	follower := make(chan error, 1)
	go func() {
		_, err := s.DetectEncryption(context.Background())
		follower <- err
	}()
	time.Sleep(50 * time.Millisecond)

	// Make the leader panic on its own stack after the probes, when it writes
	// the mode back through a nil store.
	s.store = nil
	close(eng.release)

	select {
	case <-panicked:
	case <-time.After(10 * time.Second):
		t.Fatal("the leading DetectEncryption never returned")
	}

	select {
	case err := <-follower:
		if err == nil {
			t.Fatal("a follower of a pass that died mid-flight was told it succeeded")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the follower is still parked on a flight nobody closed")
	}

	// The next caller starts a fresh pass.
	s.store = st
	done := make(chan EncryptionDetection, 1)
	go func() {
		det, err := s.DetectEncryption(context.Background())
		if err != nil {
			t.Errorf("detect after the panic: %v", err)
		}
		done <- det
	}()
	select {
	case det := <-done:
		if det.Verdict != VerdictEncrypted {
			t.Fatalf("verdict = %q, want encrypted (repos: %+v)", det.Verdict, det.Repos)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("encryption detection stayed wedged after the leader panicked")
	}
}
