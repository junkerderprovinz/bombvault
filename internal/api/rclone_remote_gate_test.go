package api_test

// The form that adds an SMB or WebDAV destination accepts a live storage
// password in its request body. authGate is a deliberate pass-through in
// trusted-LAN mode, so a route like this has to close itself the way the
// recovery kit and the credentialed export do.

import (
	"net/http"
	"strings"
	"testing"
)

func TestAddRcloneRemoteRefusesWhenAuthDisabled(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	w, m := doJSON(t, h, http.MethodPost, "/api/offsite/rclone-remote",
		`{"name":"nas","type":"smb","host":"192.168.1.10","user":"backup","password":"hunter2"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 with auth off: %v", w.Code, m)
	}
	if body := w.Body.String(); strings.Contains(body, "hunter2") {
		t.Fatalf("the refusal echoed the password back: %s", body)
	}
}
