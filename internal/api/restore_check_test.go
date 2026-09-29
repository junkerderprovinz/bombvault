package api_test

import (
	"net/http"
	"testing"
)

func TestRestoreCheckRouteAnswersInTheEnvelope(t *testing.T) {
	h, _ := newTestRouter(t, &fakeServiceDocker{}, &fakeResticEngine{})
	w, m := doJSON(t, h, http.MethodPost, "/api/restore/check", `{"kind":"stack","name":"media"}`)
	if w.Code != http.StatusOK || m["ok"] != false || m["error"] == "" {
		t.Fatalf("unknown kind: %d %v", w.Code, m)
	}

	// A container with no backup still gets a checklist rather than an error.
	w, m = doJSON(t, h, http.MethodPost, "/api/restore/check", `{"kind":"container","name":"plex","snapshotId":"aaaa1111"}`)
	if w.Code != http.StatusOK || m["ok"] != true || m["ready"] != false {
		t.Fatalf("container: %d %v", w.Code, m)
	}
	if checks, _ := m["checks"].([]any); len(checks) != 4 {
		t.Fatalf("checks = %v", m["checks"])
	}
}
