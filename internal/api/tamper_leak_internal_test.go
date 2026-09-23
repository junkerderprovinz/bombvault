package api

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// url.Error quotes the raw URL it failed to parse, credentials included. The run
// row feeds the UI, the digest and the widget, so the error stored there has to
// be scrubbed. tamperProbe does not go through the restic adapter, which scrubs
// its own errors.
func TestRunTamperTestScrubsCredentialsFromURLParseFailure(t *testing.T) {
	const (
		user     = "backupuser"
		password = "Tr0ub4dor&3" //nolint:gosec // G101: test fixture credential, not a real secret
	)
	// The space in the host name makes url.Parse fail without a network call.
	badRepo := "rest:https://" + user + ":" + password + "@stor age.example.com:8000/containers"

	svc, st := tamperService(t, badRepo, &fakeHostSSH{})
	_, err := svc.RunTamperTest(context.Background(), "containers")
	if err == nil {
		t.Fatal("a malformed probe URL must return a non-nil error (inconclusive probe)")
	}
	// The raw error has to contain the password, or the checks below would pass
	// trivially.
	if !strings.Contains(err.Error(), password) {
		t.Fatalf("test setup did not reproduce a raw url.Parse failure containing the password (got %v); adjust badRepo so it does", err)
	}

	run := latestTamperRun(t, st)
	if strings.Contains(run.Error, password) {
		t.Fatalf("runs.error leaked the repo password from a tamper-test URL-parse failure, got %q", run.Error)
	}
	if strings.Contains(run.Error, user) {
		t.Fatalf("runs.error leaked the repo username from a tamper-test URL-parse failure, got %q", run.Error)
	}
	if run.Status != "skipped" {
		t.Fatalf("an inconclusive (unparseable) probe should settle a skipped run, got status=%q", run.Status)
	}
}

// RunPrimaryTamperTest reaches the same tamperProbe path for a domain's remote
// primary.
func TestRunPrimaryTamperTestScrubsCredentialsFromURLParseFailure(t *testing.T) {
	const (
		domain   = "containers"
		user     = "backupuser"
		password = "Tr0ub4dor&3" //nolint:gosec // G101: test fixture credential, not a real secret
	)
	badRepo := "rest:https://" + user + ":" + password + "@stor age.example.com:8000/containers"

	svc, st := tamperService(t, "", &fakeHostSSH{})
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = badRepo
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	// RunPrimaryTamperTest probes only once a remote-primary row is saved.
	if _, err := svc.SetPrimaryRemoteConfig(domain, store.OffsiteTarget{Immutable: true}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.RunPrimaryTamperTest(context.Background(), domain)
	if err == nil {
		t.Fatal("a malformed probe URL must return a non-nil error (inconclusive probe)")
	}
	if !strings.Contains(err.Error(), password) {
		t.Fatalf("test setup did not reproduce a raw url.Parse failure containing the password (got %v); adjust badRepo so it does", err)
	}

	run := latestTamperRun(t, st)
	if strings.Contains(run.Error, password) {
		t.Fatalf("runs.error leaked the primary repo password from a URL-parse failure, got %q", run.Error)
	}
	if strings.Contains(run.Error, user) {
		t.Fatalf("runs.error leaked the primary repo username from a URL-parse failure, got %q", run.Error)
	}
}
