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

// formatItemFailures writes one "- name: reason" line per failure with host
// paths scrubbed, and caps a long list with a "+N more" line.
func TestFormatItemFailuresEnumeratesAndCaps(t *testing.T) {
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

// A failed scheduled run sends one webhook message that names each failed
// container and its reason, not just the count.
func TestScheduledNotifyResultEnumeratesFailedContainers(t *testing.T) {
	var body string
	wh := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer wh.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "always", WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic", ScheduledSummary: true,
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

// A run without failures sends the plain summary with no list.
func TestScheduledNotifyResultAllSuccessNoList(t *testing.T) {
	var body string
	wh := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer wh.Close()

	s := unraidNotifyService(t, nil)
	if err := s.SetNotifyConfig(notify.Config{
		On: "always", WebhookEnabled: true, WebhookURL: wh.URL, WebhookFormat: "generic", ScheduledSummary: true,
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
