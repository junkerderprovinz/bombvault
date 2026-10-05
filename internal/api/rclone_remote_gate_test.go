package api_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestAddRcloneRemoteNeverEchoesThePassword(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	w, m := doJSON(t, h, http.MethodPost, "/api/offsite/rclone-remote",
		`{"name":"nas","type":"smb","host":"192.168.1.10","user":"backup","password":"hunter2"}`)
	if w.Code == http.StatusForbidden {
		t.Fatalf("the form was refused with auth off: %v", m)
	}
	if body := w.Body.String(); strings.Contains(body, "hunter2") {
		t.Fatalf("the answer echoed the password back: %s", body)
	}
}
