package api

import (
	"context"
	"io"
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
		s.notifyOverBudget(context.Background(), "containers", 100, 50)
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
