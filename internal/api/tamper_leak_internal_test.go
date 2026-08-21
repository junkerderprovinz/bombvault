package api

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestRunTamperTestScrubsCredentialsFromURLParseFailure is the fix for the
// most serious finding of this review round: tamperProbe builds an
// *http.Request from a probe URL derived directly from the repo's configured
// location via http.NewRequestWithContext. When that URL is malformed, the
// underlying url.Parse failure embeds the FULL, UNPARSED input URL verbatim
// in its error text — credentials included, since Go's url.Error.Error()
// quotes the raw string it failed to parse, not a redacted form of it.
//
// That error used to reach runs.error via truncateRunErr with NO scrubbing:
// truncateRunErr's old doc comment claimed "the restic adapter already
// scrubs secrets/paths from its own errors", which is true for
// restic-originated errors but not for this tamperProbe code path (it never
// goes anywhere near the restic adapter). From runs.error the raw password
// reached three surfaces verbatim: the UI (handleRuns embeds store.Run
// directly, no scrubbing), the weekly digest (digest.go reads run.Error and
// forwards it to every notification channel), and the embeddable widget
// (widget.go's truncateWidgetError only limits length). The same leak exists
// via primary_remote.go's RunPrimaryTamperTest, which hits the identical
// tamperProbe code path for a domain's remote PRIMARY instead of an off-site
// destination.
//
// A raw space in the hostname is used here purely as a RELIABLE,
// deterministic way to make url.Parse fail in a test (no reliance on network
// behavior) while the credential immediately before it survives untouched in
// url.Error's message — any other malformed repo URL that fails
// http.NewRequestWithContext the same way reproduces the identical leak.
func TestRunTamperTestScrubsCredentialsFromURLParseFailure(t *testing.T) {
	const (
		user     = "backupuser"
		password = "Tr0ub4dor&3" //nolint:gosec // G101: test fixture credential, not a real secret
	)
	// The space between "stor" and "age.example.com" is what makes
	// http.NewRequestWithContext's url.Parse fail ("invalid character \" \"
	// in host name") — deterministically, without a real network call.
	badRepo := "rest:https://" + user + ":" + password + "@stor age.example.com:8000/containers"

	svc, st := tamperService(t, badRepo, &fakeHostSSH{})
	_, err := svc.RunTamperTest(context.Background(), "containers")
	if err == nil {
		t.Fatal("a malformed probe URL must return a non-nil error (inconclusive probe)")
	}
	// Confirm this test actually exercises the leak: tamperProbe's raw,
	// returned error must itself still contain the password — if it didn't,
	// the assertions below would trivially pass for the wrong reason.
	if !strings.Contains(err.Error(), password) {
		t.Fatalf("test setup did not reproduce a raw url.Parse failure containing the password (got %v) — adjust badRepo so it does", err)
	}

	// This is the fix under test: the PERSISTED run row (runs.error — read
	// verbatim by the UI, the digest/notification path, and the widget feed)
	// must not carry the credential that the raw error contained.
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

// TestRunPrimaryTamperTestScrubsCredentialsFromURLParseFailure is the same
// fix, exercised through RunPrimaryTamperTest — the domain's remote-PRIMARY
// counterpart of RunTamperTest (see primary_remote.go), which reaches the
// exact same tamperProbe code path and therefore carried the identical leak.
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
	// A saved remote-primary safety row is required before RunPrimaryTamperTest
	// will probe at all (see its own doc comment).
	if _, err := svc.SetPrimaryRemoteConfig(domain, store.OffsiteTarget{Immutable: true}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.RunPrimaryTamperTest(context.Background(), domain)
	if err == nil {
		t.Fatal("a malformed probe URL must return a non-nil error (inconclusive probe)")
	}
	if !strings.Contains(err.Error(), password) {
		t.Fatalf("test setup did not reproduce a raw url.Parse failure containing the password (got %v) — adjust badRepo so it does", err)
	}

	run := latestTamperRun(t, st)
	if strings.Contains(run.Error, password) {
		t.Fatalf("runs.error leaked the primary repo password from a URL-parse failure, got %q", run.Error)
	}
	if strings.Contains(run.Error, user) {
		t.Fatalf("runs.error leaked the primary repo username from a URL-parse failure, got %q", run.Error)
	}
}
