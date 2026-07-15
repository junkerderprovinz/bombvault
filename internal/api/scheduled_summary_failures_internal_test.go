package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
)

// TestFormatItemFailuresEnumeratesAndCaps pins the summary body builder (#64): it
// renders one "- name: reason" line per failure, scrubs absolute host paths out of
// each reason (matching the per-item notifyBackup treatment), and caps a large run
// with a "+N more" tail so a 45-container night with 35 failures stays legible.
func TestFormatItemFailuresEnumeratesAndCaps(t *testing.T) {
	// A short list is enumerated in full, with the reason path-scrubbed.
	short := []schedule.ItemFailure{
		{Name: "plex", Reason: "init repo: no space left on device"},
		{Name: "sonarr", Reason: "open /mnt/user/backups/containers/config: input/output error"},
	}
	got := formatItemFailures(short)
	if !strings.Contains(got, "- plex: init repo: no space left on device") {
		t.Fatalf("expected the plex failure enumerated with its reason, got:\n%s", got)
	}
	if !strings.Contains(got, "- sonarr:") {
		t.Fatalf("expected the sonarr failure enumerated, got:\n%s", got)
	}
	if strings.Contains(got, "/mnt/user") {
		t.Fatalf("absolute host paths must be scrubbed from the reason, got:\n%s", got)
	}
	if strings.Contains(got, "+") {
		t.Fatalf("a short list must not be capped, got:\n%s", got)
	}

	// A large list caps at maxListedFailures with a "+N more" tail and omits the rest.
	many := make([]schedule.ItemFailure, 0, maxListedFailures+5)
	for i := 0; i < maxListedFailures+5; i++ {
		many = append(many, schedule.ItemFailure{Name: "c" + string(rune('A'+i)), Reason: "restic repo error"})
	}
	got = formatItemFailures(many)
	if lines := strings.Count(got, "\n") + 1; lines != maxListedFailures+1 {
		t.Fatalf("a capped list should have %d lines (%d shown + 1 tail), got %d:\n%s",
			maxListedFailures+1, maxListedFailures, lines, got)
	}
	if !strings.Contains(got, "+5 more") {
		t.Fatalf("expected a '+5 more' tail for the 5 omitted failures, got:\n%s", got)
	}
	if strings.Contains(got, many[maxListedFailures].Name) {
		t.Fatalf("the %dth+ failure must be omitted (folded into the tail), got:\n%s", maxListedFailures+1, got)
	}
}

// TestScheduledNotifyResultEnumeratesFailedContainers pins the end-to-end summary
// (#64): a scheduled run that failed sends ONE webhook message that names the failed
// containers and their reasons — not just a bare "N of M failed" count — so the user
// knows WHICH containers to look at without digging. A domain-wide fault that trips
// the pre-flight guards for many containers is exactly the case this surfaces.
func TestScheduledNotifyResultEnumeratesFailedContainers(t *testing.T) {
	var body string
	wh := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer wh.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "always", WebhookURL: wh.URL, WebhookFormat: "generic", ScheduledSummary: true,
	}); err != nil {
		t.Fatal(err)
	}

	failures := []schedule.ItemFailure{
		{Name: "plex", Reason: "init repo: no space left on device"},
		{Name: "sonarr", Reason: "inspect container: cannot connect to the Docker daemon"},
	}
	s.ScheduledNotifyResult(context.Background(), "containers", 45, 2, failures)

	if body == "" {
		t.Fatal("a failed scheduled run must send a summary webhook")
	}
	if !strings.Contains(body, "2 of 45 items failed") {
		t.Fatalf("summary should carry the failure count, got: %s", body)
	}
	for _, want := range []string{"plex", "sonarr", "no space left on device", "cannot connect to the Docker daemon"} {
		if !strings.Contains(body, want) {
			t.Fatalf("summary should enumerate %q, got: %s", want, body)
		}
	}
}

// TestScheduledNotifyResultAllSuccessNoList pins that an all-success scheduled run
// still sends only the clean "no failures" summary — the failure enumeration is added
// solely on the failure path and never leaks an empty list into a green run.
func TestScheduledNotifyResultAllSuccessNoList(t *testing.T) {
	var body string
	wh := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer wh.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "always", WebhookURL: wh.URL, WebhookFormat: "generic", ScheduledSummary: true,
	}); err != nil {
		t.Fatal(err)
	}

	s.ScheduledNotifyResult(context.Background(), "containers", 45, 0, nil)

	if !strings.Contains(body, "45 items, no failures") {
		t.Fatalf("an all-success run should send the clean no-failures summary, got: %s", body)
	}
	if strings.Contains(body, "- ") {
		t.Fatalf("an all-success run must not enumerate any failures, got: %s", body)
	}
}
