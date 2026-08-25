package api

// Tests for the encryption-mode auto-detection (DetectEncryption).
//
// The interesting behaviour is not "an encrypted repo reports encrypted" — it
// is the ambiguity handling: a probe failure must never be read as "plain", an
// unreachable repo alongside an empty one must report "unknown" rather than
// "absent", and only a definite verdict may write the setting.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// modeStubEngine opens a repo only under the mode recorded for it: true =
// opens ONLY encrypted, false = opens ONLY plain. A repo absent from the map
// fails both probes with failErr (default: restic's not-a-repo message).
type modeStubEngine struct {
	ResticEngine
	encrypted map[string]bool
	failErr   map[string]error
}

func (e *modeStubEngine) RepoOpensErr(_ context.Context, repo string, m restic.Mode) error {
	if want, ok := e.encrypted[repo]; ok {
		if m.Encrypted == want {
			return nil
		}
		return errors.New("Fatal: wrong password or no key found")
	}
	if err, ok := e.failErr[repo]; ok {
		return err
	}
	return errors.New("Fatal: unable to open config file: repository does not exist")
}

func (e *modeStubEngine) RepoOpens(ctx context.Context, repo string, m restic.Mode) bool {
	return e.RepoOpensErr(ctx, repo, m) == nil
}

// newDetectSvc builds a Service whose containers/vms/flash/files/config paths
// all live under a real temp mount root, so resolveRepo succeeds and the local
// on-disk checks (localRepoMissing) see a real filesystem.
func newDetectSvc(t *testing.T, eng ResticEngine) (*Service, *store.Repo, string) {
	t.Helper()
	s, st := newSyncTestService(t)
	root := t.TempDir()
	s.cfg = config.Config{AppKey: strings.Repeat("a", 64), HostMountRoot: root, HostSourceRoot: "/mnt"}
	s.engine = eng
	return s, st, root
}

// setPaths configures only the named domains' LOCAL paths, blanking the rest so
// exactly the intended repositories are probed.
func setPaths(t *testing.T, st *store.Repo, paths map[string]string) store.Settings {
	t.Helper()
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = paths["containers"]
	settings.VMsPath = paths["vms"]
	settings.FlashPath = paths["flash"]
	settings.FilesPath = paths["files"]
	settings.ConfigPath = paths["config"]
	settings.ContainersOffsite = paths["containersOffsite"]
	settings.VMsOffsite = ""
	settings.FlashOffsite = ""
	settings.FilesOffsite = ""
	settings.ConfigOffsite = ""
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

// mkrepo creates the location sub resolves to and drops a `config` file in it,
// so localRepoMissing is false — a repository really is present on disk there.
// It returns the path in the SAME form resolveRepo produces (paths.Resolve is
// slash-based, so it does not match filepath.Join on Windows), which is the key
// the engine stub is looked up under.
func mkrepo(t *testing.T, s *Service, sub string) string {
	t.Helper()
	dir, err := s.resolveRepo(sub)
	if err != nil {
		t.Fatalf("resolve %q: %v", sub, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDetectEncryptedRepoAppliesSetting is the common path: the repository
// opens with the derived password, so the verdict is "encrypted" and the stored
// setting FOLLOWS it — the user asserted nothing.
func TestDetectEncryptedRepoAppliesSetting(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = true

	// Start from the WRONG stored value, so "applied" is observable.
	settings := setPaths(t, st, map[string]string{"containers": "backups/containers"})
	settings.EncryptionEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	det, err := s.DetectEncryption(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if det.Verdict != VerdictEncrypted {
		t.Fatalf("verdict = %q, want encrypted (repos: %+v)", det.Verdict, det.Repos)
	}
	if !det.Applied || !det.EncryptionEnabled {
		t.Fatalf("expected the detected mode to be applied, got applied=%v enabled=%v", det.Applied, det.EncryptionEnabled)
	}
	got, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.EncryptionEnabled {
		t.Fatal("stored EncryptionEnabled must follow the detected mode")
	}
}

// TestDetectPlainRepoAppliesSetting is the mirror case: a password-less repo
// flips a wrongly-enabled setting back off.
func TestDetectPlainRepoAppliesSetting(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = false

	settings := setPaths(t, st, map[string]string{"containers": "backups/containers"})
	settings.EncryptionEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	det, err := s.DetectEncryption(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if det.Verdict != VerdictPlain {
		t.Fatalf("verdict = %q, want plain (repos: %+v)", det.Verdict, det.Repos)
	}
	if !det.Applied || det.EncryptionEnabled {
		t.Fatalf("expected plain to be applied, got applied=%v enabled=%v", det.Applied, det.EncryptionEnabled)
	}
}

// TestDetectUnreachableIsNotPlain is THE safety property. A repository that
// fails to open for an unrelated reason (dead backend, wrong credentials) must
// report "unknown" and must NOT touch the setting — reading a probe failure as
// "no password needed" is the one wrong answer that would quietly create an
// empty repo beside the real backups.
func TestDetectUnreachableIsNotPlain(t *testing.T) {
	eng := &modeStubEngine{
		encrypted: map[string]bool{},
		failErr: map[string]error{
			"rest:http://offsite:8000/repo": errors.New("server response unexpected: 401 Unauthorized"),
		},
	}
	s, st, _ := newDetectSvc(t, eng)

	settings := setPaths(t, st, map[string]string{"containersOffsite": "rest:http://offsite:8000/repo"})
	settings.EncryptionEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	det, err := s.DetectEncryption(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if det.Verdict != VerdictUnknown {
		t.Fatalf("verdict = %q, want unknown (repos: %+v)", det.Verdict, det.Repos)
	}
	if det.Applied {
		t.Fatal("an undecidable probe must never write the setting")
	}
	if !det.EncryptionEnabled {
		t.Fatal("the stored setting must be left exactly as it was")
	}
	if len(det.Repos) != 1 || det.Repos[0].State != RepoUnreachable {
		t.Fatalf("expected one unreachable repo, got %+v", det.Repos)
	}
	if det.Repos[0].Err == "" {
		t.Fatal("an unreachable repo must carry the reason it could not be read")
	}
}

// TestDetectAbsentRepoIsFirstTimeSetup: a reachable REMOTE location that simply
// holds no repository yet is "absent" — nothing to detect, the user's choice
// genuinely decides how it gets created — and the setting is left alone.
func TestDetectAbsentRepoIsFirstTimeSetup(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, st, _ := newDetectSvc(t, eng)
	setPaths(t, st, map[string]string{"containersOffsite": "rest:http://offsite:8000/repo"})

	det, err := s.DetectEncryption(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if det.Verdict != VerdictAbsent {
		t.Fatalf("verdict = %q, want absent (repos: %+v)", det.Verdict, det.Repos)
	}
	if det.Applied {
		t.Fatal("absent must not write the setting")
	}
}

// TestDetectUnconfiguredWhenNoPathsSet: nothing configured at all is its own
// verdict, distinct from "configured but empty".
func TestDetectUnconfiguredWhenNoPathsSet(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, st, _ := newDetectSvc(t, eng)
	setPaths(t, st, map[string]string{})

	det, err := s.DetectEncryption(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if det.Verdict != VerdictUnconfigured {
		t.Fatalf("verdict = %q, want unconfigured (repos: %+v)", det.Verdict, det.Repos)
	}
	if len(det.Repos) != 0 {
		t.Fatalf("nothing configured must probe nothing, got %+v", det.Repos)
	}
}

// TestDetectConflictWhenReposDisagree: two real repositories in DIFFERENT modes.
// One global flag cannot open both, so there is no correct value to apply and
// the verdict says so instead of picking a side.
func TestDetectConflictWhenReposDisagree(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, st, _ := newDetectSvc(t, eng)
	encRepo := mkrepo(t, s, "backups/containers")
	plainRepo := mkrepo(t, s, "backups/vms")
	eng.encrypted[encRepo] = true
	eng.encrypted[plainRepo] = false

	settings := setPaths(t, st, map[string]string{
		"containers": "backups/containers",
		"vms":        "backups/vms",
	})
	settings.EncryptionEnabled = true
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	det, err := s.DetectEncryption(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if det.Verdict != VerdictConflict {
		t.Fatalf("verdict = %q, want conflict (repos: %+v)", det.Verdict, det.Repos)
	}
	if det.Applied {
		t.Fatal("a conflict must never write the setting")
	}
	// The UI needs to name the odd one out, so both states must be reported.
	var sawEnc, sawPlain bool
	for _, r := range det.Repos {
		switch r.State {
		case RepoEncrypted:
			sawEnc = true
		case RepoPlain:
			sawPlain = true
		}
	}
	if !sawEnc || !sawPlain {
		t.Fatalf("conflict must report both sides, got %+v", det.Repos)
	}
}

// TestDetectDetectionWinsOverAbsentSibling: a definite detection from ONE repo
// settles the (global) setting even when another configured location is still
// empty. Otherwise attaching a half-configured box could never auto-detect.
func TestDetectDetectionWinsOverAbsentSibling(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, st, _ := newDetectSvc(t, eng)
	encRepo := mkrepo(t, s, "backups/containers")
	eng.encrypted[encRepo] = true

	settings := setPaths(t, st, map[string]string{
		"containers":        "backups/containers",
		"containersOffsite": "rest:http://offsite:8000/repo", // absent
	})
	settings.EncryptionEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}

	det, err := s.DetectEncryption(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if det.Verdict != VerdictEncrypted || !det.Applied {
		t.Fatalf("verdict = %q applied=%v, want encrypted+applied (repos: %+v)", det.Verdict, det.Applied, det.Repos)
	}
}

// TestFoldEncryptionUnknownBeatsAbsent pins the ordering rule directly: an
// unreachable repository alongside an empty one is "cannot tell", NOT "fresh
// install". The unreachable one may be the encrypted repo being restored.
func TestFoldEncryptionUnknownBeatsAbsent(t *testing.T) {
	got := foldEncryption([]RepoEncryption{
		{Domain: "containers", Source: "local", State: RepoAbsent},
		{Domain: "vms", Source: "offsite", State: RepoUnreachable, Err: "401"},
	})
	if got != VerdictUnknown {
		t.Fatalf("fold = %q, want unknown — an unreachable repo must not be read as a fresh install", got)
	}
}

// TestFoldEncryptionTable covers the remaining fold combinations in one place.
func TestFoldEncryptionTable(t *testing.T) {
	cases := []struct {
		name string
		in   []RepoEncryption
		want EncryptionVerdict
	}{
		{"nothing configured", nil, VerdictUnconfigured},
		{"single encrypted", []RepoEncryption{{State: RepoEncrypted}}, VerdictEncrypted},
		{"single plain", []RepoEncryption{{State: RepoPlain}}, VerdictPlain},
		{"all absent", []RepoEncryption{{State: RepoAbsent}, {State: RepoAbsent}}, VerdictAbsent},
		{"all unreachable", []RepoEncryption{{State: RepoUnreachable}}, VerdictUnknown},
		{"encrypted + unreachable", []RepoEncryption{{State: RepoEncrypted}, {State: RepoUnreachable}}, VerdictEncrypted},
		{"plain + absent", []RepoEncryption{{State: RepoPlain}, {State: RepoAbsent}}, VerdictPlain},
		{"both modes", []RepoEncryption{{State: RepoEncrypted}, {State: RepoPlain}}, VerdictConflict},
		{"both modes + noise", []RepoEncryption{{State: RepoPlain}, {State: RepoUnreachable}, {State: RepoEncrypted}}, VerdictConflict},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := foldEncryption(c.in); got != c.want {
				t.Fatalf("fold = %q, want %q", got, c.want)
			}
		})
	}
}

// TestClassifyClosedRepoLocalWithConfigIsUnreachable: a local location that DOES
// have a `config` file but opened under neither mode is not a BombVault repo, is
// corrupt, or is permission-denied. Never "absent" — there is clearly something
// there.
func TestClassifyClosedRepoLocalWithConfigIsUnreachable(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, _, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")

	state, msg := s.classifyClosedRepo(repo, errors.New("Fatal: wrong password or no key found"))
	if state != RepoUnreachable {
		t.Fatalf("state = %q, want unreachable — a present config that opens under neither mode is not an empty location", state)
	}
	if msg == "" {
		t.Fatal("expected the probe failure to be reported")
	}
}

// TestClassifyClosedRepoLocalMissingIsAbsent: a local location with no `config`
// and no established marker is a genuine fresh location.
func TestClassifyClosedRepoLocalMissingIsAbsent(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, _, root := newDetectSvc(t, eng)
	repo := filepath.Join(root, "backups", "nothing-here")

	state, _ := s.classifyClosedRepo(repo, errors.New("Fatal: unable to open config file"))
	if state != RepoAbsent {
		t.Fatalf("state = %q, want absent", state)
	}
}

// TestClassifyClosedRepoVanishedMountIsUnreachable is the #55 case: BombVault
// established a repo here before, its `config` is gone, and the backing store is
// not in the mount table. That is a real repository we cannot see — reporting it
// as "absent" would be exactly the wrong guess.
func TestClassifyClosedRepoVanishedMountIsUnreachable(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, st, root := newDetectSvc(t, eng)
	repo := filepath.Join(root, "remotes", "nas", "containers")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkRepoEstablished(repo); err != nil {
		t.Fatal(err)
	}
	// destinationMounted reads /proc/self/mountinfo; the temp path is not a
	// mount point, so it reports false — the vanished-backing-store case.
	state, msg := s.classifyClosedRepo(repo, errors.New("Fatal: unable to open config file"))
	if state != RepoUnreachable {
		t.Fatalf("state = %q, want unreachable for an established-but-unmounted repo", state)
	}
	if msg == "" {
		t.Fatal("expected the not-mounted reason to be reported")
	}
}

// TestDeadRemoteHostIsUnreachableNotAbsent is a LIVE-CAUGHT regression. Pointed
// at a host with nothing listening, restic answers with a message that contains
// BOTH "unable to open config file" and the dial failure — so isRepoUninitialized
// matches and a naive classification called a dead network "absent". That is the
// one wrong answer this feature must never give: "absent" means "nothing exists,
// your choice decides", which would license creating an empty repository beside
// backups that were there all along.
//
// The message below is the verbatim text observed against the test container.
func TestDeadRemoteHostIsUnreachableNotAbsent(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, _, _ := newDetectSvc(t, eng)

	dead := errors.New(`Fatal: unable to open config file: Head "http://:***@192.168.20.199:8000/norepo/config": dial tcp 192.168.20.199:8000: connect: no route to host
Is there a repository at the following location?
rest:http://192.168.20.199:8000/norepo/`)

	if isRepoUninitialized(dead) != true {
		t.Fatal("precondition: the loose helper is expected to match this message — that is the trap being guarded")
	}
	if isRepoDefinitelyAbsent(dead) {
		t.Fatal("a dial failure must never count as a definitely-absent repository")
	}
	state, msg := s.classifyClosedRepo("rest:http://192.168.20.199:8000/norepo", dead)
	if state != RepoUnreachable {
		t.Fatalf("state = %q, want unreachable for a dead remote host", state)
	}
	if msg == "" {
		t.Fatal("expected the transport failure to be reported")
	}
}

// TestReachableRemoteWithNoRepoIsAbsent is the other half: a REACHABLE backend
// that simply holds no repository yet must still classify as absent, so the
// stricter check above did not just turn every remote into "unknown".
func TestReachableRemoteWithNoRepoIsAbsent(t *testing.T) {
	eng := &modeStubEngine{encrypted: map[string]bool{}}
	s, _, _ := newDetectSvc(t, eng)

	empty := errors.New("Fatal: unable to open config file: repository does not exist\nIs there a repository at the following location?")
	if !isRepoDefinitelyAbsent(empty) {
		t.Fatal("a clean not-a-repository answer must still count as absent")
	}
	state, _ := s.classifyClosedRepo("rest:http://offsite:8000/repo", empty)
	if state != RepoAbsent {
		t.Fatalf("state = %q, want absent", state)
	}
}

// TestTransportMarkersRejectAbsent walks the failure shapes a remote backend
// realistically produces, each of which must read as "cannot tell".
func TestTransportMarkersRejectAbsent(t *testing.T) {
	cases := map[string]string{
		"refused":     "Fatal: unable to open config file: dial tcp 10.0.0.5:8000: connect: connection refused",
		"dns":         "Fatal: unable to open config file: dial tcp: lookup backup.example: no such host",
		"timeout":     "Fatal: unable to open config file: Head \"https://x/config\": context deadline exceeded",
		"auth":        "Fatal: unable to open config file: server response unexpected: 401 Unauthorized",
		"forbidden":   "Fatal: unable to open config file: server response unexpected: 403 Forbidden",
		"tls":         "Fatal: unable to open config file: x509: certificate signed by unknown authority",
		"permissions": "Fatal: unable to open config file: open /mnt/x/config: permission denied",
	}
	for name, msg := range cases {
		t.Run(name, func(t *testing.T) {
			if isRepoDefinitelyAbsent(errors.New(msg)) {
				t.Fatalf("%q must not be read as an absent repository", msg)
			}
		})
	}
}

// TestDetectProbeNeverLocks proves the probe stays read-only in the one way that
// can damage someone else's repository: every probe carries NoLock, so asking
// what mode a repo is in never writes a lock file into it.
func TestDetectProbeNeverLocks(t *testing.T) {
	eng := &lockRecordingEngine{modeStubEngine: modeStubEngine{encrypted: map[string]bool{}}}
	s, st, _ := newDetectSvc(t, eng)
	repo := mkrepo(t, s, "backups/containers")
	eng.encrypted[repo] = true
	setPaths(t, st, map[string]string{"containers": "backups/containers"})

	if _, err := s.DetectEncryption(context.Background()); err != nil {
		t.Fatalf("detect: %v", err)
	}
	if eng.probes == 0 {
		t.Fatal("expected at least one probe")
	}
	if eng.locking > 0 {
		t.Fatalf("%d probe(s) ran without NoLock — detection must never lock a repository", eng.locking)
	}
}

type lockRecordingEngine struct {
	modeStubEngine
	probes  int
	locking int
}

func (e *lockRecordingEngine) RepoOpensErr(ctx context.Context, repo string, m restic.Mode) error {
	e.probes++
	if !m.NoLock {
		e.locking++
	}
	return e.modeStubEngine.RepoOpensErr(ctx, repo, m)
}
