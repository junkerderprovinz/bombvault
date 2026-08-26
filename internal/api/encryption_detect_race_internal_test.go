package api

// DetectEncryption vs. a concurrent settings save.
//
// Detection reads the settings row, then probes every configured repository.
// That probe is the slow part by design: one dead off-site host costs
// 2 x encryptionProbeTimeout (a minute) while the local repo answers at once,
// and the Recovery page fires the whole thing on mount. So a settings save
// landing DURING the probe is not a corner case, it is the normal case for a
// user working through the recovery wizard.
//
// What detection actually determines is one bit: EncryptionEnabled. Writing
// back the whole row it read a minute ago instead reverts every column the user
// saved in between — paths, schedules, the login password — while the UI shows
// those saves as successful. These tests pin that a detection can only ever
// change the bit it determined.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// gatedEngine reports when a probe has started and blocks it until the test
// releases it — standing in for the minute a dead backend really costs.
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

// TestDetectEncryptionKeepsConcurrentSettingsSave is the regression proof for
// the lost-update bug: while the probe runs, the user saves unrelated settings.
// Both writes must survive — the detected mode AND everything the user saved.
func TestDetectEncryptionKeepsConcurrentSettingsSave(t *testing.T) {
	eng := &gatedEngine{
		modeStubEngine: modeStubEngine{encrypted: map[string]bool{}},
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = true

	// Stored mode is WRONG, so the detection has something to apply.
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

	// The probe is now in flight (and stuck, like a dead sftp host).
	select {
	case <-eng.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the probe never started")
	}

	// The user finishes the wizard's Step 2 and saves new paths; another tab
	// sets a login password. Both land while the probe is still running.
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
		t.Fatalf("VMsPath = %q — the save made during the probe was reverted", after.VMsPath)
	}
	if after.FilesPath != "user/backups/files-CHOSEN-BY-THE-USER" {
		t.Fatalf("FilesPath = %q — the save made during the probe was reverted", after.FilesPath)
	}
	if after.RestoreFolder != "user/restores" {
		t.Fatalf("RestoreFolder = %q — the save made during the probe was reverted", after.RestoreFolder)
	}
	if after.AuthPasswordHash != "set-while-the-probe-was-running" {
		t.Fatalf("AuthPasswordHash = %q — a password set during the probe was reverted, i.e. auth silently turned back off", after.AuthPasswordHash)
	}
	// The detection reports the row as it now stands, not the snapshot it read
	// before the probe.
	if got.det.EncryptionEnabled != after.EncryptionEnabled {
		t.Fatalf("reported encryptionEnabled = %v but the row holds %v", got.det.EncryptionEnabled, after.EncryptionEnabled)
	}
}

// TestDetectEncryptionUndecidedWritesNothing: an undecidable probe must not
// write at all — not even the row it read. Otherwise the "we changed nothing"
// path is still a full-row write and still reverts a concurrent save.
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
		t.Fatalf("InstanceName = %q — an undecided detection still clobbered a concurrent save", after.InstanceName)
	}
}

// TestDetectEncryptionSharesOneProbePass: two callers arriving together (two
// browser tabs on the Recovery page, or a reload mid-probe) must share ONE
// probe pass rather than forking a second set of restic processes at the same
// repositories.
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
	// Give the follower a moment to either join or start its own pass. It cannot
	// start one without blocking on the gate, so a second probe would show up in
	// the counter below.
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
	// One pass over one repository probes exactly once (it opens under the first,
	// encrypted, mode). Two passes would have probed twice.
	if eng.probes != 1 {
		t.Fatalf("%d probes — two concurrent callers must share one pass, not run one each", eng.probes)
	}
}
