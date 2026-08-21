package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// failIfCalledSSH implements HostSSH but fails the test immediately if ANY
// method is invoked. Task 6 requires the Unraid-only sendUnraidNotify/
// dashplugin steps to be skipped ENTIRELY on a non-Unraid platform — not
// attempted and then swallowed. Wiring this fake in place of the usual
// fakeHostSSH proves "entirely": if the guard degrades to "attempt anyway,
// let the error get logged away", this fake fails the test the instant the
// attempt happens, rather than requiring an assertion on call counts.
type failIfCalledSSH struct{ t *testing.T }

var _ HostSSH = failIfCalledSSH{}

func (f failIfCalledSSH) fail(method string) {
	f.t.Helper()
	f.t.Fatalf("unexpected HostSSH.%s call: an Unraid-only step ran on a non-Unraid platform", method)
}

func (f failIfCalledSSH) ReadFile(context.Context, string) ([]byte, error) {
	f.fail("ReadFile")
	return nil, nil
}
func (f failIfCalledSSH) WriteFile(context.Context, string, []byte) error {
	f.fail("WriteFile")
	return nil
}
func (f failIfCalledSSH) PublicKey() (string, error) { return "", nil }
func (f failIfCalledSSH) Test(context.Context) error { return nil }
func (f failIfCalledSSH) Run(context.Context, ...string) (string, error) {
	f.fail("Run")
	return "", nil
}
func (f failIfCalledSSH) EnsureKnownHost(context.Context) error { return nil }
func (f failIfCalledSSH) StreamCommand(context.Context, ...string) (io.ReadCloser, func() error, error) {
	f.fail("StreamCommand")
	return nil, nil, nil
}
func (f failIfCalledSSH) RunWithStdin(context.Context, io.Reader, ...string) error {
	f.fail("RunWithStdin")
	return nil
}

// TestSendUnraidNotifyCallSitesSkipOnNonUnraidPlatform pins Task 6's contract
// for every background/best-effort sendUnraidNotify call site across
// service.go, digest.go, tamper.go, watchdog.go and receiver_watch.go: with
// notify.Config.Unraid=true and host SSH configured — exactly the state that
// makes today's Unraid host attempt the call — a Generic platform must skip
// the SSH round-trip ENTIRELY instead of attempting it and swallowing the
// (predictable, noisy) failure. Each subtest wires the SAME notify.Config
// shape a real Unraid host would use to actually reach the guarded block, so
// the ONLY variable is the platform.
func TestSendUnraidNotifyCallSitesSkipOnNonUnraidPlatform(t *testing.T) {
	t.Run("notifyRetentionFailed", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		s.notifyRetentionFailed(context.Background(), "containers", "boom")
	})

	t.Run("notifyOverBudget", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		s.notifyOverBudget(context.Background(), "containers", 100, 50, "off-site")
	})

	t.Run("notifyReplicationFailed", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		s.notifyReplicationFailed(context.Background(), "containers", "boom")
	})

	t.Run("notifyDrillFailure", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		s.notifyDrillFailure(context.Background(), "containers", "plex", "boom")
	})

	t.Run("notifyProtectionLost", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		s.notifyProtectionLost(context.Background(), "containers", "boom")
	})

	t.Run("notifyBackup", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "always", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		s.notifyBackup(context.Background(), "container", "plex", true, backup.Summary{SnapshotID: "deadbeef"}, nil)
	})

	t.Run("recordAndNotifyContainerSkip", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if _, err := s.store.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
			t.Fatal(err)
		}
		if err := s.SetNotifyConfig(notify.Config{On: "always", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		s.recordAndNotifyContainerSkip(context.Background(), "plex")
	})

	t.Run("ScheduledNotifyResult", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "always", Unraid: true, ScheduledSummary: true}); err != nil {
			t.Fatal(err)
		}
		s.ScheduledNotifyResult(context.Background(), "containers", 1, 0, nil)
	})

	t.Run("SendDigest", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		if err := s.SetNotifyConfig(notify.Config{On: "always", Unraid: true}); err != nil {
			t.Fatal(err)
		}
		if err := s.SendDigest(context.Background()); err != nil {
			t.Fatalf("SendDigest: %v", err)
		}
	})

	t.Run("notifyBackupOverdue", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		s.notifyBackupOverdue(context.Background(), notify.Config{Unraid: true}, "containers", 0, 3600, time.Now().Unix())
	})

	t.Run("notifyReceiverDeadMan", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		s.notifyReceiverDeadMan(context.Background(), notify.Config{Unraid: true}, "repo", "host", 24)
	})

	t.Run("notifyReceiverIntegrity", func(t *testing.T) {
		s := unraidNotifyService(t, failIfCalledSSH{t})
		s.SetPlatform(platform.Generic{})
		s.notifyReceiverIntegrity(context.Background(), notify.Config{Unraid: true}, "repo", "boom")
	})

	t.Run("updateContainerAfterBackup", func(t *testing.T) {
		svc, st := newUpdateTestSvc(t)
		svc.ssh = failIfCalledSSH{t}
		svc.SetPlatform(platform.Generic{})
		if err := svc.SetNotifyConfig(notify.Config{On: "always", Unraid: true, NotifyOnUpdate: true}); err != nil {
			t.Fatal(err)
		}
		tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
		if err != nil {
			t.Fatal(err)
		}
		svc.docker = &updateFakeDocker{imageID: "sha256:NEW"}
		in := model.Inspect{Name: "/plex", Image: "sha256:OLD", Config: model.Config{Image: "plex:latest"}}
		svc.updateContainerAfterBackup(context.Background(), "plex", in, tg.ID)
	})
}

// TestTestNotifyUnraidChannelSkippedOnNonUnraidPlatform: the explicit "Test"
// button path is a request-scoped action, not a background job — silently
// reporting success without ever contacting anything would be dishonest, so
// this call site (unlike the fire-and-forget ones above) must return a clear
// error instead of attempting the SSH command, rather than pretending the
// test passed.
func TestTestNotifyUnraidChannelSkippedOnNonUnraidPlatform(t *testing.T) {
	s := unraidNotifyService(t, failIfCalledSSH{t})
	s.SetPlatform(platform.Generic{})
	if err := s.TestNotify(context.Background(), notify.Config{Unraid: true}); err == nil {
		t.Fatal("expected an error: the Unraid test-notify channel must not be attempted on a non-Unraid platform")
	}
}

// TestDashboardPluginInstallRemoveSkipOnNonUnraidPlatform: the companion
// dashboard-tile plugin's install/remove are Unraid `plugin` CLI operations —
// meaningless anywhere else — so on a non-Unraid platform they must refuse
// before ever opening the SSH connection, not attempt a command that could
// only fail on the far end.
func TestDashboardPluginInstallRemoveSkipOnNonUnraidPlatform(t *testing.T) {
	for _, ep := range []struct {
		name string
		call func(*Service) (string, error)
	}{
		{"install", func(s *Service) (string, error) { return s.InstallDashboardPlugin(context.Background()) }},
		{"remove", func(s *Service) (string, error) { return s.RemoveDashboardPlugin(context.Background()) }},
	} {
		t.Run(ep.name, func(t *testing.T) {
			s := &Service{ssh: failIfCalledSSH{t}}
			s.SetPlatform(platform.Generic{})
			if _, err := ep.call(s); err == nil {
				t.Fatalf("%s must fail (not attempted) on a non-Unraid platform", ep.name)
			}
		})
	}
}

// --- code-review fix: loud, actionable diagnostics on a c.Unraid=true /
// Kind()!=KindUnraid mismatch, instead of the silent feature drop the above
// tests (Task 6, pre-existing) only proved the SAFE half of. ---
//
// Detect()'s only Unraid signal is the dockerMan marker under the
// container's /host/boot mount, which the shipped Unraid template only
// wired up months after the Kind() gate above was introduced. A genuinely
// Unraid host whose mount is missing hits exactly this state — Unraid=true,
// SSH configured, Kind()==KindGeneric — and used to have every Unraid-only
// feature go dark with no way to tell "the user turned this off" apart from
// "detection is wrong". The tests below pin the fix: the gate stays HARD
// (Option B — trusting the toggle blindly would reintroduce the exact
// wrong-platform SSH attempts Task 6 eliminated, see unraidGate's doc
// comment), but the mismatch is now loud and actionable everywhere it can
// be observed: a once-per-process log line for the best-effort background
// paths, and a named, hinted error for the two request-scoped refusals
// (TestNotify, the dashboard plugin).

// TestUnraidGateMismatchWarnsOncePerService pins the exact misdetection
// scenario from the finding: notify.Config.Unraid=true, SSH configured,
// Kind()==KindGeneric. unraidGate must still return false (SAFE — no
// Unraid-only SSH command runs, enforced by failIfCalledSSH), but unlike
// before the fix, the mismatch must now be OBSERVABLE: exactly one
// diagnostic log line naming the detected platform and the /host/boot fix,
// even across repeated calls on the same Service (platformMismatchOnce must
// not spam the log once per notification on a bad day with many failures).
func TestUnraidGateMismatchWarnsOncePerService(t *testing.T) {
	s := unraidNotifyService(t, failIfCalledSSH{t})
	s.SetPlatform(platform.Generic{})

	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	for i := 0; i < 3; i++ {
		if s.unraidGate(true) {
			t.Fatal("unraidGate must return false when the detected platform is not Unraid")
		}
	}

	out := buf.String()
	const marker = "notify.Config.Unraid is enabled but BombVault detected platform"
	if got := strings.Count(out, marker); got != 1 {
		t.Fatalf("expected exactly 1 mismatch diagnostic across 3 calls (once per process/Service), got %d; log=%s", got, out)
	}
	for _, want := range []string{`"generic"`, `"unraid"`, "/host/boot", "restart the container"} {
		if !strings.Contains(out, want) {
			t.Fatalf("mismatch diagnostic missing %q; log=%s", want, out)
		}
	}
}

// TestUnraidGateNoWarnWhenPlatformMatches: the common case (a real Unraid
// host, correctly detected) must stay completely silent — no diagnostic
// noise when nothing is wrong.
func TestUnraidGateNoWarnWhenPlatformMatches(t *testing.T) {
	s := unraidNotifyService(t, &fakeHostSSH{}) // no SetPlatform: platformFn() defaults to Unraid{}

	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	if !s.unraidGate(true) {
		t.Fatal("unraidGate must return true when SSH is configured and the platform is Unraid")
	}
	if out := buf.String(); out != "" {
		t.Fatalf("no mismatch diagnostic expected when the platform matches, got: %s", out)
	}
}

// TestUnraidGateNoWarnWhenToggleOff: a non-Unraid host with the Unraid
// toggle correctly left off has nothing to diagnose — no log line.
func TestUnraidGateNoWarnWhenToggleOff(t *testing.T) {
	s := unraidNotifyService(t, failIfCalledSSH{t})
	s.SetPlatform(platform.Generic{})

	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	if s.unraidGate(false) {
		t.Fatal("unraidGate must return false when the caller doesn't want Unraid features")
	}
	if out := buf.String(); out != "" {
		t.Fatalf("no mismatch diagnostic expected when the toggle itself is off (nothing to explain), got: %s", out)
	}
}

// TestUnraidGateNoWarnWhenSSHUnconfigured: without SSH configured at all,
// the platform-mismatch diagnostic would be misleading (SSH, not detection,
// is the actual blocker) — no log line; sendUnraidNotify's own nil-SSH
// message already covers that case where it's reachable (e.g. TestNotify).
func TestUnraidGateNoWarnWhenSSHUnconfigured(t *testing.T) {
	s := unraidNotifyService(t, nil)
	s.SetPlatform(platform.Generic{})

	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	if s.unraidGate(true) {
		t.Fatal("unraidGate must return false without SSH configured")
	}
	if out := buf.String(); out != "" {
		t.Fatalf("no platform-mismatch diagnostic expected when SSH isn't configured, got: %s", out)
	}
}

// TestTestNotifyPlatformMismatchErrorIsActionable extends
// TestTestNotifyUnraidChannelSkippedOnNonUnraidPlatform (which only pinned
// "must fail"): the refusal TestNotify's Settings "Test" button surfaces to
// the user must itself name the detected platform and the /host/boot fix —
// this is a synchronous, user-initiated action, so the diagnostic belongs in
// the response, not just the container log.
func TestTestNotifyPlatformMismatchErrorIsActionable(t *testing.T) {
	s := unraidNotifyService(t, failIfCalledSSH{t})
	s.SetPlatform(platform.Generic{})
	err := s.TestNotify(context.Background(), notify.Config{Unraid: true})
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{`"generic"`, "/host/boot", "BombVault Unraid template"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("TestNotify platform-mismatch error missing %q, got: %s", want, msg)
		}
	}
}

// TestDashboardPluginPlatformMismatchErrorIsActionable extends
// TestDashboardPluginInstallRemoveSkipOnNonUnraidPlatform the same way: the
// install/remove refusal must name the detected platform and the
// /host/boot fix, not just say "only available on Unraid hosts" (which
// reads as a deliberate limitation, not a possible detection bug, to an
// operator who is certain they ARE on Unraid).
func TestDashboardPluginPlatformMismatchErrorIsActionable(t *testing.T) {
	for _, ep := range []struct {
		name string
		call func(*Service) (string, error)
	}{
		{"install", func(s *Service) (string, error) { return s.InstallDashboardPlugin(context.Background()) }},
		{"remove", func(s *Service) (string, error) { return s.RemoveDashboardPlugin(context.Background()) }},
	} {
		t.Run(ep.name, func(t *testing.T) {
			s := &Service{ssh: failIfCalledSSH{t}}
			s.SetPlatform(platform.Generic{})
			_, err := ep.call(s)
			if err == nil {
				t.Fatal("expected an error")
			}
			msg := err.Error()
			for _, want := range []string{`"generic"`, "/host/boot", "dashboard plugin"} {
				if !strings.Contains(msg, want) {
					t.Fatalf("%s platform-mismatch error missing %q, got: %s", ep.name, want, msg)
				}
			}
		})
	}
}

// TestScrubErrorKeepsUnraidPlatformMismatchPaths pins the scrubber bypass
// unraidPlatformMismatchError relies on (found via live verification: without
// it, handlers.go's generic absolute-path scrubber reduced the whole
// actionable hint to "...verify the host's [path] is bind-mounted to
// [path] inside the container...", exactly as useless as the pre-fix
// errRestoreDestination/errRepoPathGuidance bugs that pattern already fixes
// elsewhere). Mirrors TestScrubErrorKeepsRestoreDestinationPath's shape.
func TestScrubErrorKeepsUnraidPlatformMismatchPaths(t *testing.T) {
	s := &Service{}
	s.SetPlatform(platform.Generic{})
	err := s.unraidPlatformMismatchError("the companion dashboard plugin")
	if !errors.Is(err, errUnraidPlatformMismatch) {
		t.Fatal("unraidPlatformMismatchError must satisfy errors.Is(err, errUnraidPlatformMismatch)")
	}
	got := scrubError(err)
	if strings.Contains(got, "[path]") {
		t.Fatalf("platform-mismatch error must not be path-scrubbed, got %q", got)
	}
	for _, want := range []string{"/boot", "/host/boot", `"generic"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("scrubbed platform-mismatch message missing %q, got %q", want, got)
		}
	}
	// The wrapped TestNotify shape (fmt.Errorf("unraid: %w", ...)) must survive
	// the same way: errors.Is unwraps through fmt.Errorf's %w to reach
	// platformMismatchErr's own Is method.
	wrapped := fmt.Errorf("unraid: %w", err)
	if !errors.Is(wrapped, errUnraidPlatformMismatch) {
		t.Fatal(`fmt.Errorf("unraid: %w", ...)-wrapped platform-mismatch error must still satisfy errors.Is`)
	}
	if gotWrapped := scrubError(wrapped); strings.Contains(gotWrapped, "[path]") {
		t.Fatalf("wrapped platform-mismatch error must not be path-scrubbed, got %q", gotWrapped)
	}
	// Unrelated errors still get their absolute paths stripped (the scrubber's
	// normal, unbypassed behavior must be unaffected).
	if other := scrubError(errors.New("open /config/bombvault.db: permission denied")); !strings.Contains(other, "[path]") {
		t.Fatalf("ordinary errors must still be path-scrubbed, got %q", other)
	}
}
