package api

import (
	"context"
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

func TestTheProgressStreamMarksTheEndOfItsReplay(t *testing.T) {
	h := &Handler{progress: progress.NewStore()}
	h.progress.Publish(progress.Event{Key: "container:plex", Phase: "backup", Percent: 40, Active: true})
	// A reader already gone still gets the replay the stream opens with.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	h.handleProgress(w, httptest.NewRequest(http.MethodGet, "/api/progress", nil).WithContext(ctx))
	body := w.Body.String()
	plex, synced := strings.Index(body, `"key":"container:plex"`), strings.Index(body, `data: {"synced":true}`)
	if plex < 0 || synced < plex {
		t.Fatalf("body %q, want the replay of plex and then the synced message", body)
	}
}
