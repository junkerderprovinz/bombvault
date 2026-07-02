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

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// tamperService builds a Service whose containers off-site repo points at offsite
// (flagged immutable), with an optional recording SSH for the Unraid notify path.
// No docker/virsh/engine is needed — RunTamperTest speaks raw HTTP only.
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

// TestRunTamperTestProtected: a server that refuses deletes (403) yields a
// testable, protected verdict and probes both /data and /snapshots.
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
	// The verdict was recorded as protected.
	last, found, err := st.LatestTamperTest("containers")
	if err != nil || !found {
		t.Fatalf("expected a recorded tamper test, found=%v err=%v", found, err)
	}
	if !last.Protected {
		t.Fatalf("recorded verdict should be protected")
	}
}

// TestRunTamperTestUnprotectedFlipNotifies: a server that would delete (404)
// yields an unprotected verdict, records it, and — because the PREVIOUS verdict
// was protected — fires exactly one protection-loss notification.
func TestRunTamperTestUnprotectedFlipNotifies(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusNotFound, &seen))
	defer srv.Close()

	ssh := &fakeHostSSH{}
	svc, st := tamperService(t, "rest:"+srv.URL, ssh)
	// Notify on failure via the Unraid channel (recorded by fakeHostSSH.Run).
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	// Seed a previous PROTECTED verdict so the new unprotected one is a flip.
	if err := st.RecordTamperTest("containers", true, ""); err != nil {
		t.Fatal(err)
	}

	v, err := svc.RunTamperTest(context.Background(), "containers")
	if err != nil {
		t.Fatalf("RunTamperTest: %v", err)
	}
	if !v.Testable || v.Protected {
		t.Fatalf("404 must be testable + NOT protected, got %+v", v)
	}
	if !strings.Contains(v.Detail, "404") {
		t.Fatalf("detail should mention the 404 verdict, got %q", v.Detail)
	}
	// Recorded as unprotected.
	last, found, err := st.LatestTamperTest("containers")
	if err != nil || !found || last.Protected {
		t.Fatalf("expected a recorded UNprotected verdict, got found=%v protected=%v err=%v", found, last.Protected, err)
	}
	// Protection-loss notification fired exactly once over the Unraid channel.
	if len(ssh.runs) != 1 {
		t.Fatalf("expected exactly one protection-loss notify on the flip, got %d", len(ssh.runs))
	}
	joined := strings.Join(ssh.runs[0], " ")
	if !strings.Contains(joined, "protection LOST") {
		t.Fatalf("notification should announce the protection loss, got %v", ssh.runs[0])
	}
}

// TestRunTamperTestAccepted: a server that accepts the delete (200) is NOT
// protected, with a clear detail.
func TestRunTamperTestAccepted(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(deleteRecorder(http.StatusOK, &seen))
	defer srv.Close()

	svc, _ := tamperService(t, "rest:"+srv.URL, &fakeHostSSH{})
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
}

// TestRunTamperTestNonRestNotTestable: a non-REST off-site repo is honestly
// reported as not testable, with nothing recorded.
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
		t.Fatalf("a non-testable repo must record nothing")
	}
}

// TestRunTamperTestTransportErrorInconclusive: a server that refuses the
// connection makes the test INCONCLUSIVE — RunTamperTest returns an error and
// records NOTHING (never treats an unreachable server as protected OR unprotected).
func TestRunTamperTestTransportErrorInconclusive(t *testing.T) {
	srv := httptest.NewServer(deleteRecorder(http.StatusForbidden, new([]string)))
	url := srv.URL
	srv.Close() // now the address refuses connections

	svc, st := tamperService(t, "rest:"+url, &fakeHostSSH{})
	// Seed a previous verdict with a UNIQUE marker so we can prove the inconclusive
	// run inserted no new row (a new record would replace it as "latest").
	const seedMarker = "SEED-MARKER-DO-NOT-REPLACE"
	if err := st.RecordTamperTest("containers", true, seedMarker); err != nil {
		t.Fatal(err)
	}

	_, err := svc.RunTamperTest(context.Background(), "containers")
	if err == nil {
		t.Fatal("a transport error must return a non-nil error (inconclusive)")
	}
	// The latest record is STILL the seeded one, untouched — RecordTamperTest was
	// never called for the inconclusive run.
	last, found, lerr := st.LatestTamperTest("containers")
	if lerr != nil || !found {
		t.Fatalf("expected the seeded record to remain, found=%v err=%v", found, lerr)
	}
	if !last.Protected || last.Detail != seedMarker {
		t.Fatalf("inconclusive run must record nothing (seeded marker must stand), got protected=%v detail=%q", last.Protected, last.Detail)
	}
}

// TestRunTamperTestInconclusiveStatuses: a 401 (rotated creds) or 503 (far-side
// maintenance / proxy) is NOT a delete verdict — it is INCONCLUSIVE, exactly like
// a transport error: RunTamperTest returns an error, records NO row and fires NO
// notification (it must never flip a stored PROTECTED verdict to a false
// "protection LOST" on a non-decisive status).
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
			// Seed a previous PROTECTED verdict with a unique marker — a non-decisive
			// status must leave it untouched (no new row, no flip).
			const seed = "SEED-INCONCLUSIVE-DO-NOT-REPLACE"
			if err := st.RecordTamperTest("containers", true, seed); err != nil {
				t.Fatal(err)
			}

			_, err := svc.RunTamperTest(context.Background(), "containers")
			if err == nil {
				t.Fatalf("status %d must be inconclusive → a non-nil error", status)
			}
			// Nothing recorded — the seeded PROTECTED row still stands.
			last, found, lerr := st.LatestTamperTest("containers")
			if lerr != nil || !found || !last.Protected || last.Detail != seed {
				t.Fatalf("status %d must record nothing (seed must stand), got protected=%v detail=%q found=%v err=%v", status, last.Protected, last.Detail, found, lerr)
			}
			// And no protection-loss notification fired.
			if len(ssh.runs) != 0 {
				t.Fatalf("status %d must not notify, got %d notifications", status, len(ssh.runs))
			}
		})
	}
}

// TestRunTamperTestConcurrentFlipNotifiesOnce: two concurrent tamper tests that
// each observe a protected→unprotected flip must fire the protection-loss alert
// EXACTLY once. RunTamperTest serialises per domain, so read-prev → record →
// notify is atomic: the second run reads the verdict the first recorded and sees
// no flip. Without the per-domain lock both could read the old PROTECTED verdict
// and double-alarm.
func TestRunTamperTestConcurrentFlipNotifiesOnce(t *testing.T) {
	srv := httptest.NewServer(deleteRecorder(http.StatusNotFound, new([]string))) // 404 = would delete = unprotected
	defer srv.Close()

	ssh := &fakeHostSSH{}
	svc, st := tamperService(t, "rest:"+srv.URL, ssh)
	if err := svc.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	// Seed a previous PROTECTED verdict so BOTH concurrent runs would, without
	// serialisation, observe the same protected→unprotected flip.
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
