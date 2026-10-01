package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/relay"
)

func TestAMemberReadsWhatRunsHereOverTheGroup(t *testing.T) {
	h, _, _, _ := newMCPGateHandler(t)
	h.svc.peerActivity = h.activity
	h.progress = progress.NewStore()
	h.progress.Publish(progress.Event{Key: "container:nextcloud", Phase: "backup", Percent: 40, Active: true})

	status, body := h.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodGet, Path: "/api/group/peer/activity"})
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	for _, want := range []string{`"runs":[]`, `"key":"container:nextcloud"`, `"next":[]`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("answer %s lacks %s", body, want)
		}
	}
}
