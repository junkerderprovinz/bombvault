package api

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pullRecorder records which location each engine call reaches.
// copyToOffsiteTarget, which the pull mirrors, calls EnsureRepo, unlockStale
// and applyRetention on its far end, and copying one of those lines would
// modify the source without any error, so the test checks locations rather
// than call counts.
type pullRecorder struct {
	ResticEngine
	// calls records "Method repo" for every engine call, in order.
	calls []string
	snaps map[string][]restic.Snapshot
	// copyMode is the mode the copy went out with, so the test can check the
	// passwords and not only that a source side exists.
	copyMode *restic.Mode
}

func (e *pullRecorder) note(method, repo string) { e.calls = append(e.calls, method+" "+repo) }

func (e *pullRecorder) RepoOpens(_ context.Context, repo string, m restic.Mode) bool {
	e.note("RepoOpens", repo)
	return m.Encrypted
}
func (e *pullRecorder) Init(_ context.Context, repo string, _ restic.Mode) error {
	e.note("Init", repo)
	return nil
}
func (e *pullRecorder) Unlock(_ context.Context, repo string, _ bool, _ restic.Mode) error {
	e.note("Unlock", repo)
	return nil
}
func (e *pullRecorder) Snapshots(_ context.Context, repo string, _ restic.Mode) ([]restic.Snapshot, error) {
	e.note("Snapshots", repo)
	return e.snaps[repo], nil
}
func (e *pullRecorder) Copy(_ context.Context, dest, src string, _ []string, _ restic.Limits, m restic.Mode) error {
	e.note("Copy dest="+dest+" src="+src, "")
	// The source's own password must be the one that travels, or restic tries the
	// destination's key on a repository derived from a different one.
	if m.From == nil {
		e.note("Copy WITHOUT a source side", "")
	}
	mode := m
	e.copyMode = &mode
	return nil
}

func TestPullNeverWritesToTheSource(t *testing.T) {
	const ourKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const theirKey = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	const srcLoc = "rest:http://192.168.1.9:8000/their-containers"

	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(ourKey, []byte(theirKey))
	if err != nil {
		t.Fatal(err)
	}

	eng := &pullRecorder{snaps: map[string][]restic.Snapshot{
		// A snapshot we do not have, so the copy actually runs.
		srcLoc: {{ID: "aaa"}},
	}}
	svc := &Service{
		cfg:    config.Config{AppKey: ourKey, DataDir: dir, HostMountRoot: dir},
		store:  st,
		engine: eng,
	}

	ps := store.PullSource{
		ID:        "p1",
		Name:      "Tower next door",
		Repo:      srcLoc,
		AppKeyEnc: enc,
		Domain:    "containers",
		Enabled:   true,
	}
	if _, err := svc.PullFromSource(context.Background(), ps); err != nil {
		t.Fatalf("the pull failed before it could prove anything: %v", err)
	}

	var touched []string
	for _, c := range eng.calls {
		if !strings.Contains(c, srcLoc) {
			continue
		}
		touched = append(touched, c)
		switch {
		case strings.HasPrefix(c, "RepoOpens "):
		case strings.HasPrefix(c, "Snapshots "):
			// Read-only by mode: listSnapshots does not clear a lock when NoLock
			// is set (see readonly_never_unlocks).
		case strings.Contains(c, "src="+srcLoc):
		default:
			t.Errorf("the pull reached the SOURCE with %q.\n"+
				"Only RepoOpens, the read-only listing and Copy's source argument may. Every other\n"+
				"engine call writes: Init creates, Unlock deletes lock files, Forget and Prune\n"+
				"delete data. This repository belongs to somebody else.", c)
		}
	}
	if len(touched) == 0 {
		t.Fatal("the source was never reached at all, so this test asserted nothing")
	}

	var copied bool
	for _, c := range eng.calls {
		if strings.HasPrefix(c, "Copy dest=") {
			copied = true
		}
		if c == "Copy WITHOUT a source side " {
			t.Error("the copy went out without Mode.From, so restic gets the DESTINATION's password\n" +
				"for the source. Two instances never share one, and the error restic answers with\n" +
				"reads as \"wrong APP_KEY\" for a key that was typed correctly.")
		}
	}
	if !copied {
		t.Fatal("no copy was attempted, so the source-side assertion above proved nothing")
	}

	// Their key has to open the source and ours the destination. The nil check
	// above would still pass with the two sides swapped, so compare the values.
	theirPw := restickey.Derive(theirKey)
	ourPw := restickey.Derive(ourKey)
	if theirPw == ourPw {
		t.Fatal("fixture is wrong: the two keys derive the same password, so nothing below can tell the sides apart")
	}
	if eng.copyMode == nil {
		t.Fatal("no copy mode was recorded, so the two assertions below prove nothing")
	}
	if eng.copyMode.From == nil {
		t.Fatal("the copy went out without a source side; the assertions below cannot run")
	}
	if got := eng.copyMode.From.Password; got != theirPw {
		which := "some third value"
		if got == ourPw {
			which = "OUR OWN password, so the two sides are swapped"
		}
		t.Errorf("the copy carried the wrong password for the SOURCE: %s.\n"+
			"It must be the password derived from the source instance's APP_KEY. restic passes it as\n"+
			"RESTIC_FROM_PASSWORD, so the wrong value here makes a correctly typed foreign key fail with\n"+
			"a decryption error that reads as \"wrong APP_KEY\".", which)
	}
	if eng.copyMode.Password != ourPw {
		t.Error("the copy did not carry OUR password for the destination.\n" +
			"The destination is this box's own repository; if the source's password reached it instead,\n" +
			"the pull would write nothing and blame the local repository for it.")
	}
}
