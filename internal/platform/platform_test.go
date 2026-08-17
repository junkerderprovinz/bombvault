package platform_test

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// Pinned literals: these are today's REAL pre-Platform-seam values (verified
// against internal/api/service.go before this package existed), not a
// paraphrase. A future accidental edit to Unraid{}'s literals must fail here.
const (
	testHostMountRoot = "/host/user"

	wantUnraidAppdataFallback          = "/mnt/user/appdata/myapp" // path.Join("/mnt/user/appdata", "myapp")
	wantUnraidForeignContainerDestBase = "/host/user/user/appdata" // path.Join(hostMountRoot, "user/appdata")
	wantUnraidForeignVMDestBase        = "/host/user/user/domains" // path.Join(hostMountRoot, "user/domains")
)

// --- Unraid{} ---

func TestUnraidKind(t *testing.T) {
	if got := (platform.Unraid{}).Kind(); got != platform.KindUnraid {
		t.Fatalf("Unraid{}.Kind() = %q, want %q", got, platform.KindUnraid)
	}
}

func TestUnraidAppdataFallback(t *testing.T) {
	got := platform.Unraid{}.AppdataFallback(testHostMountRoot, "myapp")
	if got != wantUnraidAppdataFallback {
		t.Fatalf("Unraid{}.AppdataFallback = %q, want %q", got, wantUnraidAppdataFallback)
	}
	// hostMountRoot must be ignored: Unraid's convention is a fixed HOST path.
	got2 := platform.Unraid{}.AppdataFallback("/totally/different/root", "myapp")
	if got2 != wantUnraidAppdataFallback {
		t.Fatalf("Unraid{}.AppdataFallback must ignore hostMountRoot: got %q, want %q", got2, wantUnraidAppdataFallback)
	}
}

func TestUnraidForeignContainerDestBase(t *testing.T) {
	got := platform.Unraid{}.ForeignContainerDestBase(testHostMountRoot)
	if got != wantUnraidForeignContainerDestBase {
		t.Fatalf("Unraid{}.ForeignContainerDestBase = %q, want %q", got, wantUnraidForeignContainerDestBase)
	}
}

func TestUnraidForeignVMDestBase(t *testing.T) {
	got := platform.Unraid{}.ForeignVMDestBase(testHostMountRoot)
	if got != wantUnraidForeignVMDestBase {
		t.Fatalf("Unraid{}.ForeignVMDestBase = %q, want %q", got, wantUnraidForeignVMDestBase)
	}
}

// fakeSSH is a minimal platform.SSHRunner test double that records the args
// it was called with and returns a canned error.
type fakeSSH struct {
	called bool
	args   []string
	err    error
}

func (f *fakeSSH) Run(_ context.Context, args ...string) (string, error) {
	f.called = true
	f.args = args
	return "", f.err
}

func TestUnraidReconcileContainerUpdateStatus_NilSSHIsNoop(t *testing.T) {
	if err := (platform.Unraid{}).ReconcileContainerUpdateStatus(context.Background(), nil, "plex:latest"); err != nil {
		t.Fatalf("nil ssh must be a silent no-op, got err: %v", err)
	}
}

func TestUnraidReconcileContainerUpdateStatus_EmptyRefIsNoop(t *testing.T) {
	ssh := &fakeSSH{}
	if err := (platform.Unraid{}).ReconcileContainerUpdateStatus(context.Background(), ssh, ""); err != nil {
		t.Fatalf("empty imageRef must be a silent no-op, got err: %v", err)
	}
	if ssh.called {
		t.Fatalf("empty imageRef must not run anything over SSH, args=%v", ssh.args)
	}
}

func TestUnraidReconcileContainerUpdateStatus_RunsPHPWithImageRefToken(t *testing.T) {
	ssh := &fakeSSH{}
	if err := (platform.Unraid{}).ReconcileContainerUpdateStatus(context.Background(), ssh, "plex:latest"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ssh.called {
		t.Fatal("expected an SSH command to run")
	}
	if len(ssh.args) < 5 || ssh.args[0] != "php" || ssh.args[1] != "-r" {
		t.Fatalf("expected a `php -r <script>` invocation, got args=%v", ssh.args)
	}
	if !strings.Contains(ssh.args[2], "reloadUpdateStatus") {
		t.Fatalf("PHP script must call reloadUpdateStatus, got %q", ssh.args[2])
	}
	if ssh.args[3] != "--" {
		t.Fatalf("expected the image ref passed as a separate argv token after --, got args=%v", ssh.args)
	}
	if ssh.args[4] != "plex:latest" {
		t.Fatalf("expected the image ref as its own token, got %q", ssh.args[4])
	}
}

func TestUnraidReconcileContainerUpdateStatus_PropagatesSSHError(t *testing.T) {
	wantErr := errors.New("ssh down")
	ssh := &fakeSSH{err: wantErr}
	err := (platform.Unraid{}).ReconcileContainerUpdateStatus(context.Background(), ssh, "plex:latest")
	if !errors.Is(err, wantErr) {
		t.Fatalf("ReconcileContainerUpdateStatus err = %v, want %v", err, wantErr)
	}
}

// --- Generic{} ---

func TestGenericKind(t *testing.T) {
	if got := (platform.Generic{}).Kind(); got != platform.KindGeneric {
		t.Fatalf("Generic{}.Kind() = %q, want %q", got, platform.KindGeneric)
	}
}

func TestGenericAppdataFallback(t *testing.T) {
	if got := (platform.Generic{}).AppdataFallback(testHostMountRoot, "myapp"); got != "" {
		t.Fatalf("Generic{}.AppdataFallback = %q, want \"\" (no convention)", got)
	}
}

func TestGenericForeignContainerDestBase(t *testing.T) {
	if got := (platform.Generic{}).ForeignContainerDestBase(testHostMountRoot); got != testHostMountRoot {
		t.Fatalf("Generic{}.ForeignContainerDestBase = %q, want %q (identity)", got, testHostMountRoot)
	}
}

func TestGenericForeignVMDestBase(t *testing.T) {
	if got := (platform.Generic{}).ForeignVMDestBase(testHostMountRoot); got != testHostMountRoot {
		t.Fatalf("Generic{}.ForeignVMDestBase = %q, want %q (identity)", got, testHostMountRoot)
	}
}

func TestGenericReconcileContainerUpdateStatusIsNoop(t *testing.T) {
	ssh := &fakeSSH{}
	if err := (platform.Generic{}).ReconcileContainerUpdateStatus(context.Background(), ssh, "plex:latest"); err != nil {
		t.Fatalf("Generic{}.ReconcileContainerUpdateStatus must always be a no-op, got err: %v", err)
	}
	if ssh.called {
		t.Fatalf("Generic{}.ReconcileContainerUpdateStatus must never touch SSH, args=%v", ssh.args)
	}
}

// --- Detect ---

func TestDetectFindsUnraidMarker(t *testing.T) {
	flashDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(flashDir, "config/plugins/dockerMan"), 0o750); err != nil {
		t.Fatal(err)
	}
	if got := platform.Detect(context.Background(), "", flashDir); got != platform.KindUnraid {
		t.Fatalf("Detect = %q, want %q (dockerMan marker present)", got, platform.KindUnraid)
	}
}

func TestDetectFallsBackToGenericWithoutMarker(t *testing.T) {
	flashDir := t.TempDir() // empty: no config/plugins/dockerMan
	if got := platform.Detect(context.Background(), "", flashDir); got != platform.KindGeneric {
		t.Fatalf("Detect = %q, want %q (no dockerMan marker)", got, platform.KindGeneric)
	}
}

func TestDetectOverrideWinsRegardlessOfMarker(t *testing.T) {
	// A flash dir that WOULD auto-detect as Unraid; the override must still win.
	flashDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(flashDir, "config/plugins/dockerMan"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		override string
		want     platform.Kind
	}{
		{"unraid", platform.KindUnraid},
		{"truenas", platform.KindTrueNAS},
		{"generic", platform.KindGeneric},
	} {
		if got := platform.Detect(context.Background(), tc.override, flashDir); got != tc.want {
			t.Fatalf("Detect(override=%q) = %q, want %q", tc.override, got, tc.want)
		}
	}
}

func TestDetectUnrecognizedOverrideFallsBackToGenericAndWarns(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	got := platform.Detect(context.Background(), "amiga-os", t.TempDir())
	if got != platform.KindGeneric {
		t.Fatalf("Detect(override=%q) = %q, want %q (unrecognized falls back to generic)", "amiga-os", got, platform.KindGeneric)
	}
	if !strings.Contains(buf.String(), "amiga-os") {
		t.Fatalf("unrecognized PLATFORM override must be logged, log=%q", buf.String())
	}
}
