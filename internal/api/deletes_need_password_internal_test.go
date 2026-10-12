package api

import (
	"net/http"
	"testing"
)

func TestDeletingBackupsForGoodNeedsALoginPassword(t *testing.T) {
	f, b2 := nginxAtHomeAndB2(t)
	timeline := map[string]any{"places": []map[string]any{{"place": "offsite:" + b2.ID, "label": "B2", "snapshotIds": []string{"b1b1b1b1"}}}}
	routes := []struct {
		method, path string
		body         any
	}{
		{http.MethodDelete, "/api/items/containers/nginx/timeline/a1a1a1a1", timeline},
		{http.MethodDelete, "/api/snapshots/containers/b1b1b1b1?source=offsite:" + b2.ID, nil},
		{http.MethodPost, "/api/prune/containers", nil},
	}
	for _, r := range routes {
		code, out := f.call(r.method, r.path, r.body)
		if code != http.StatusForbidden || out["code"] != codeNoPassword {
			t.Errorf("%s %s without a password: %d %v", r.method, r.path, code, out)
		}
	}
	if got := forgottenAt(f, "b2:bucket:containers"); len(got) != 0 {
		t.Fatalf("forgotten without a password: %v", got)
	}

	f.signIn()
	if m := f.do(http.MethodDelete, routes[0].path, timeline); m["ok"] != true {
		t.Fatalf("delete signed in = %v", m)
	}
}
