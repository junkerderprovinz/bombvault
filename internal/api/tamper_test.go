package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// tamperService builds a Service whose containers off-site repo points at offsite
// (flagged immutable), with an optional recording SSH for the Unraid notify path.
// RunTamperTest only speaks HTTP, so no docker, virsh or engine is needed.
func tamperService(t *testing.T, offsite string, ssh HostSSH) (*Service, *store.Repo) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() }) // close before TempDir cleanup (Windows file lock)
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ContainersOffsite = offsite
	s.ContainersOffsiteImmutable = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		cfg:   config.Config{AppKey: strings.Repeat("a", 64)},
		store: st,
		ssh:   ssh,
	}
	return svc, st
}

// latestTamperRun returns the newest run of kind "tamper" and fails the test
// when there is none.
func latestTamperRun(t *testing.T, st *store.Repo) store.Run {
	t.Helper()
	runs, err := st.ListRuns(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Kind == "tamper" {
			return r
		}
	}
	t.Fatal("no tamper run row recorded")
	return store.Run{}
}

// deleteRecorder is an httptest handler that returns a fixed status to every
// DELETE and records the paths it saw, so a test can assert both probes ran.
func deleteRecorder(status int, seen *[]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			*seen = append(*seen, r.URL.Path)
		}
		w.WriteHeader(status)
	})
}

// A 403 yields a testable, protected verdict, and both /data and /snapshots are
// probed.
func TestRunTamperTestProtected(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer srv.Close()

	svc, st := tamperService(t, "rest:"+srv.URL, &fakeHostSSH{})
	v, err := svc.RunTamperTest(context.Background(), "containers")
	if err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if !v.Testable || !v.Protected {
		t.Fatalf("403 must be testable+protected, got %+v", v)
	}
	if len(seen) != 2 {
		t.Fatalf("expected both probes (data + snapshots), saw %v", seen)
	}
	hasData, hasSnap := false, false
	for _, p := range seen {
		if strings.Contains(p, "/data/") {
			hasData = true
		}
		if strings.Contains(p, "/snapshots/") {
			hasSnap = true
		}
	}
	if !hasData || !hasSnap {
		t.Fatalf("both /data and /snapshots must be probed, saw %v", seen)
	}
	last, found, err := st.LatestTamperTest("containers")
	if err != nil || !found {
		t.Fatalf("expected a recorded tamper test, found=%v err=%v", found, err)
	}
	if !last.Protected {
		t.Fatalf("recorded verdict should be protected")
	}
	run := latestTamperRun(t, st)
	if run.Status != "success" || run.TargetID != "containers" || run.FinishedAt == nil {
		t.Fatalf("tamper run = %+v, want Status=success TargetID=containers finished", run)
	}
}

// A target with its own named credential set is probed with it. The shared
// credentials would get a 401, which is inconclusive, and the test would stay
// skipped.
func TestRunTamperTestUsesTargetOwnCredentials(t *testing.T) {
	var probedAs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			user, _, _ := r.BasicAuth()
			probedAs = append(probedAs, user)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	svc, st := tamperService(t, "rest:"+srv.URL, &fakeHostSSH{})
	if err := svc.SetCloudCreds(CloudCreds{RESTUser: "shared", RESTPassword: "sharedpw"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCloudCredSets([]CloudCredSet{
		{ID: "garage", Name: "Garage", CloudCreds: CloudCreds{RESTUser: "named", RESTPassword: "namedpw"}},
	}); err != nil {
		t.Fatal(err)
	}
	// Only a stored target row carries a CredsRef.
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		Domain:    "containers",
		Name:      "Primary",
		Repo:      "rest:" + srv.URL,
		Immutable: true,
		Enabled:   true,
		CredsRef:  "garage",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RunTamperTest(context.Background(), "containers"); err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if len(probedAs) == 0 {
		t.Fatal("no DELETE probe reached the server")
	}
	for _, user := range probedAs {
		if user != "named" {
			t.Fatalf("tamper probe authenticated as %q, want %q: the target's own credential set was ignored", user, "named")
		}
	}
}

// drainTwoProgressEvents reads the begin and end events a synchronous call
// published. The Subscribe channel is buffered, so both are queued by the time
// the call returns; the deadline keeps a failure from hanging.
func drainTwoProgressEvents(t *testing.T, ch <-chan progress.Event) (begin, term progress.Event) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	select {
	case begin = <-ch:
	case <-deadline:
		t.Fatal("timed out waiting for the begin progress event")
	}
	select {
	case term = <-ch:
	case <-deadline:
		t.Fatal("timed out waiting for the terminal progress event")
	}
	return begin, term
}

// A running tamper test publishes a "maintenance" progress pair keyed
// "tamper:<domain>", so the activity log shows it while the far side is probed.
func TestRunTamperTestEmitsLiveProgress(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer srv.Close()

	svc, _ := tamperService(t, "rest:"+srv.URL, &fakeHostSSH{})
	prog := progress.NewStore()
	svc.SetProgress(prog)
	ch, cancel := prog.Subscribe()
	defer cancel()

	if _, err := svc.RunTamperTest(context.Background(), "containers"); err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}

	begin, term := drainTwoProgressEvents(t, ch)
	if begin.Key != "tamper:containers" || begin.Phase != "maintenance" || !begin.Active {
		t.Fatalf("begin event = %+v, want Key=tamper:containers Phase=maintenance Active=true", begin)
	}
	if term.Key != "tamper:containers" || term.Phase != "maintenance" || term.Active || term.Percent != 100 {
		t.Fatalf("terminal event = %+v, want Key=tamper:containers Phase=maintenance Active=false Percent=100", term)
	}
}

// A server that accepts deletes after a protected verdict sends exactly one
// protection-loss notification.
func TestRunTamperTestUnprotectedFlipNotifies(t *testing.T) {
	var seen []string
	// A rest-server without --append-only answers 200 for a missing 64-hex
	// object; a 404 would be inconclusive.
	srv := httptest.NewServer(deleteRecorder(http.StatusOK, &seen))
	defer srv.Close()

	ssh := &fakeHostSSH{}
	svc, st := tamperService(t, "rest:"+srv.URL, ssh)
	// Notify on failure via the Unraid channel (recorded by fakeHostSSH.Run).
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	// A previous protected verdict makes the new one a flip.
	if err := st.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatal(err)
	}

	v, err := svc.RunTamperTest(context.Background(), "containers")
	if err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if !v.Testable || v.Protected {
		t.Fatalf("an accepted delete must be testable + NOT protected, got %+v", v)
	}
	// The UI shows the detail, so it names the status the far side returned.
	if !strings.Contains(v.Detail, "200") {
		t.Fatalf("detail must name the status the far side returned, got %q", v.Detail)
	}
	last, found, err := st.LatestTamperTest("containers")
	if err != nil || !found || last.Protected {
		t.Fatalf("expected a recorded UNprotected verdict, got found=%v protected=%v err=%v", found, last.Protected, err)
	}
	if len(ssh.runs) != 1 {
		t.Fatalf("expected exactly one protection-loss notify on the flip, got %d", len(ssh.runs))
	}
	joined := strings.Join(ssh.runs[0], " ")
	if !strings.Contains(joined, "protection LOST") {
		t.Fatalf("notification should announce the protection loss, got %v", ssh.runs[0])
	}
}

// An accepted delete is not protected and records a failed run carrying the
// detail.
func TestRunTamperTestAccepted(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusOK, &seen))
	defer srv.Close()

	svc, st := tamperService(t, "rest:"+srv.URL, &fakeHostSSH{})
	v, err := svc.RunTamperTest(context.Background(), "containers")
	if err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if !v.Testable || v.Protected {
		t.Fatalf("200 must be testable + NOT protected, got %+v", v)
	}
	if !strings.Contains(strings.ToLower(v.Detail), "accepted") {
		t.Fatalf("detail should say the server accepted a delete, got %q", v.Detail)
	}
	run := latestTamperRun(t, st)
	if run.Status != "failed" || run.TargetID != "containers" {
		t.Fatalf("tamper run = %+v, want Status=failed TargetID=containers", run)
	}
	if !strings.Contains(strings.ToLower(run.Error), "accepted") {
		t.Fatalf("failed tamper run should carry the detail, got %q", run.Error)
	}
}

// A non-REST off-site repo is not testable and records no verdict, but still
// settles a skipped run with the reason, so a scheduled test shows up in the
// activity log.
func TestRunTamperTestNonRestNotTestable(t *testing.T) {
	svc, st := tamperService(t, "s3:s3.amazonaws.com/bucket/containers", &fakeHostSSH{})
	v, err := svc.RunTamperTest(context.Background(), "containers")
	if err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if v.Testable {
		t.Fatalf("an s3 repo must not be testable, got %+v", v)
	}
	if !strings.Contains(v.Detail, "REST") {
		t.Fatalf("detail should explain only REST repos are verifiable, got %q", v.Detail)
	}
	if _, found, _ := st.LatestTamperTest("containers"); found {
		t.Fatalf("a non-testable repo must record no verdict")
	}
	run := latestTamperRun(t, st)
	if run.Status != "skipped" || run.TargetID != "containers" || run.FinishedAt == nil {
		t.Fatalf("tamper run = %+v, want Status=skipped TargetID=containers finished", run)
	}
	if !strings.Contains(run.Error, "REST") {
		t.Fatalf("skipped tamper run should carry the not-a-REST-server reason, got %q", run.Error)
	}
}

// A refused connection is inconclusive: RunTamperTest returns an error and
// records no verdict, but still settles a skipped run.
func TestRunTamperTestTransportErrorInconclusive(t *testing.T) {
	srv := httptest.NewServer(deleteRecorder(http.StatusForbidden, new([]string)))
	url := srv.URL
	srv.Close() // now the address refuses connections

	svc, st := tamperService(t, "rest:"+url, &fakeHostSSH{})
	// A new record would replace this one as the latest.
	const seedMarker = "SEED-MARKER-DO-NOT-REPLACE"
	if err := st.RecordTamperTest("containers", true, seedMarker); err != nil {
		t.Fatal(err)
	}

	_, err := svc.RunTamperTest(context.Background(), "containers")
	if err == nil {
		t.Fatal("a transport error must return a non-nil error (inconclusive)")
	}
	last, found, lerr := st.LatestTamperTest("containers")
	if lerr != nil || !found {
		t.Fatalf("expected the seeded record to remain, found=%v err=%v", found, lerr)
	}
	if !last.Protected || last.Detail != seedMarker {
		t.Fatalf("inconclusive run must record nothing (seeded marker must stand), got protected=%v detail=%q", last.Protected, last.Detail)
	}
	run := latestTamperRun(t, st)
	if run.Status != "skipped" || run.TargetID != "containers" || run.Error == "" {
		t.Fatalf("tamper run = %+v, want Status=skipped TargetID=containers with an error text", run)
	}
}

// A 401 (rotated credentials) or 503 (far-side maintenance) is inconclusive like
// a transport error: no verdict, no notification, only a skipped run.
func TestRunTamperTestInconclusiveStatuses(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusServiceUnavailable} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			srv := httptest.NewServer(deleteRecorder(status, new([]string)))
			defer srv.Close()

			ssh := &fakeHostSSH{}
			svc, st := tamperService(t, "rest:"+srv.URL, ssh)
			if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
				t.Fatal(err)
			}
			// A protected verdict that has to stay the latest.
			const seed = "SEED-INCONCLUSIVE-DO-NOT-REPLACE"
			if err := st.RecordTamperTest("containers", true, seed); err != nil {
				t.Fatal(err)
			}

			_, err := svc.RunTamperTest(context.Background(), "containers")
			if err == nil {
				t.Fatalf("status %d must be inconclusive → a non-nil error", status)
			}
			last, found, lerr := st.LatestTamperTest("containers")
			if lerr != nil || !found || !last.Protected || last.Detail != seed {
				t.Fatalf("status %d must record nothing (seed must stand), got protected=%v detail=%q found=%v err=%v", status, last.Protected, last.Detail, found, lerr)
			}
			if len(ssh.runs) != 0 {
				t.Fatalf("status %d must not notify, got %d notifications", status, len(ssh.runs))
			}
			run := latestTamperRun(t, st)
			if run.Status != "skipped" || !strings.Contains(run.Error, "inconclusive") {
				t.Fatalf("status %d tamper run = %+v, want Status=skipped with the inconclusive reason", status, run)
			}
		})
	}
}

// RunTamperTest serialises per domain, so of two concurrent runs the second
// reads the verdict the first recorded and sees no flip.
func TestRunTamperTestConcurrentFlipNotifiesOnce(t *testing.T) {
	srv := httptest.NewServer(deleteRecorder(http.StatusOK, new([]string))) // accepted delete: unprotected
	defer srv.Close()

	ssh := &fakeHostSSH{}
	svc, st := tamperService(t, "rest:"+srv.URL, ssh)
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	// Without the lock both runs would see this protected verdict and alert.
	if err := st.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.RunTamperTest(context.Background(), "containers"); err != nil {
				t.Errorf("RunTamperTest: %v", err)
			}
		}()
	}
	wg.Wait()

	if len(ssh.runs) != 1 {
		t.Fatalf("a concurrent protected→unprotected flip must notify exactly once, got %d", len(ssh.runs))
	}
}

// rest-server names every object by its full 64-hex ID; any other length is not
// an object path. Measured with and without --append-only:
//
//	DELETE /repo/data/<64 hex>        403 (append-only)   200 (plain)
//	DELETE /repo/snapshots/<64 hex>   403 (append-only)   200 (plain)
//	DELETE /repo/snapshots/<8 hex>    404                 404
//
// A probe with the 8-hex short ID restic prints would get 404 from every server.
func TestTamperProbeIDsAreFullLength(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusForbidden, &seen))
	defer srv.Close()

	st := stage4Store(t)
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: "t1", Domain: "containers", Name: "t1", Repo: "rest:" + srv.URL, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st, ssh: &fakeHostSSH{}}
	if _, err := svc.RunTamperTest(context.Background(), "containers"); err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("both probes must run, saw %d: %v", len(seen), seen)
	}
	for _, p := range seen {
		id := p[strings.LastIndex(p, "/")+1:]
		if len(id) != 64 {
			t.Errorf("probe %q uses a %d-character id; rest-server only recognises 64-hex object names", p, len(id))
		}
	}
	// Deleting locks is allowed even under --append-only, so a locks probe would
	// call every protected server unprotected.
	for _, p := range seen {
		if strings.Contains(p, "/locks/") {
			t.Errorf("probe %q targets locks, which append-only deliberately still allows", p)
		}
	}
}

// rest-server checks append-only before it looks for the object, so a delete of
// a missing 64-hex object gets 200 or 403, never 404. A 404 means the URL is not
// an object path; reading it as unprotected would fire the protection-lost
// alert.
func TestTamperProbe404IsInconclusive(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusNotFound, &seen))
	defer srv.Close()

	st := stage4Store(t)
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{
		ID: "t404", Domain: "containers", Name: "t404", Repo: "rest:" + srv.URL, Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cfg: config.Config{AppKey: strings.Repeat("a", 64)}, store: st, ssh: &fakeHostSSH{}}

	if _, err := svc.RunTamperTest(context.Background(), "containers"); err == nil {
		t.Fatal("a 404 must be inconclusive (non-nil error), not a NOT-protected verdict")
	}
	if _, found, err := st.LatestTamperTestForTarget("containers", "t404"); err != nil || found {
		t.Fatalf("an inconclusive probe must record NO verdict, found=%v err=%v", found, err)
	}
}
