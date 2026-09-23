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

// failIfCalledSSH fails the test on any call that would reach the host, so an
// Unraid-only step that is attempted on another platform and then logged away
// still fails.
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

// Each call site runs with Unraid notifications on and SSH configured, the
// state in which an Unraid host would send.
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

// The Test button waits for an answer, so unlike the background call sites it
// has to return an error instead of reporting success without sending.
func TestTestNotifyUnraidChannelSkippedOnNonUnraidPlatform(t *testing.T) {
	s := unraidNotifyService(t, failIfCalledSSH{t})
	s.SetPlatform(platform.Generic{})
	if err := s.TestNotify(context.Background(), notify.Config{Unraid: true}); err == nil {
		t.Fatal("expected an error: the Unraid test-notify channel must not be attempted on a non-Unraid platform")
	}
}

// The dashboard plugin is managed with Unraid's plugin CLI, so on another
// platform install and remove refuse before opening SSH.
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

// Detect recognises Unraid only by the dockerMan marker under /host/boot, so an
// Unraid host without that mount runs with Unraid notifications on, SSH
// configured and a Generic platform. The gate stays closed there, but logs the
// mismatch once for the background paths.
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

// A correctly detected Unraid host logs nothing.
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

// With the Unraid toggle off there is nothing to diagnose.
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

// Without SSH the platform is not what blocks, so a mismatch line would
// mislead. sendUnraidNotify reports the missing SSH itself.
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

// The Test button's refusal names the detected platform and the /host/boot
// fix, because the user reads the response, not the container log.
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

// The plugin refusal names the platform and the /host/boot fix as well. "Only
// available on Unraid hosts" would read as a limitation to an operator who is
// on Unraid.
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

// Scrubbed, the platform-mismatch hint would read "verify the host's [path] is
// bind-mounted to [path] inside the container".
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
	// TestNotify wraps it as "unraid: %w", which has to survive as well.
	wrapped := fmt.Errorf("unraid: %w", err)
	if !errors.Is(wrapped, errUnraidPlatformMismatch) {
		t.Fatal(`fmt.Errorf("unraid: %w", ...)-wrapped platform-mismatch error must still satisfy errors.Is`)
	}
	if gotWrapped := scrubError(wrapped); strings.Contains(gotWrapped, "[path]") {
		t.Fatalf("wrapped platform-mismatch error must not be path-scrubbed, got %q", gotWrapped)
	}
	// Other errors still lose their absolute paths.
	if other := scrubError(errors.New("open /config/bombvault.db: permission denied")); !strings.Contains(other, "[path]") {
		t.Fatalf("ordinary errors must still be path-scrubbed, got %q", other)
	}
}
