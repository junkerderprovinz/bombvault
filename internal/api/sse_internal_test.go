package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

func TestAProgressSnapshotEndsAfterTheBarsInFlight(t *testing.T) {
	h := &Handler{progress: progress.NewStore()}
	h.progress.Publish(progress.Event{Key: "container:nextcloud", Phase: "backup", Percent: 40, Active: true})

	done := make(chan *httptest.ResponseRecorder)
	go func() {
		w := httptest.NewRecorder()
		h.handleProgress(w, httptest.NewRequest(http.MethodGet, "/api/progress?snapshot=1", nil))
		done <- w
	}()
	select {
	case w := <-done:
		if !strings.Contains(w.Body.String(), `"key":"container:nextcloud"`) {
			t.Fatalf("snapshot body %q lacks the running backup", w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the snapshot stream did not end")
	}
}
