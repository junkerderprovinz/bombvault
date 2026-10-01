package api

import (
	"testing"
)

func TestRestoringAtATargetThatNoLongerHoldsTheIDSaysSnapshotMissing(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.vm("web", "")
	set := f.fileSet("Photos", "")
	src := map[string]string{}
	for _, domain := range []string{"containers", "vms", "files", "flash", "config"} {
		src[domain] = "offsite:" + f.target(domain, "B2", "b2:bucket:"+domain).ID
	}
	f.hold("b2:bucket:flash", copied("c1c1c1c1", "d1d1d1d1", 1_758_000_000, "flash"))
	f.hold("b2:bucket:config", copied("c2c2c2c2", "d2d2d2d2", 1_758_000_000, "config"))
	const gone = "0badc0de"
	restoreTo := map[string]any{"snapshotId": gone, "targetPath": "restore", "confirm": true}
	someFiles := map[string]any{"snapshotId": gone, "paths": []string{"/data"}, "targetPath": "restore", "confirm": true}

	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/containers/nginx/restore?source=" + src["containers"], map[string]any{"snapshotId": gone, "confirm": true}},
		{"GET", "/api/containers/nginx/files?snapshot=" + gone + "&source=" + src["containers"], nil},
		{"POST", "/api/containers/nginx/restore-files?source=" + src["containers"], someFiles},
		{"POST", "/api/containers/nginx/restore-to?source=" + src["containers"], map[string]any{"snapshotId": gone, "targetPath": "restore"}},
		{"POST", "/api/vms/web/restore?source=" + src["vms"], map[string]any{"snapshotId": gone, "confirm": true}},
		{"POST", "/api/files/sets/" + set.ID + "/restore?source=" + src["files"], restoreTo},
		{"GET", "/api/files/sets/" + set.ID + "/files?snapshot=" + gone + "&source=" + src["files"], nil},
		{"POST", "/api/files/sets/" + set.ID + "/restore-files?source=" + src["files"], someFiles},
		{"POST", "/api/config/restore", map[string]any{"source": src["config"], "snapshot": gone}},
		{"GET", "/api/flash/download?snapshot=" + gone + "&source=" + src["flash"], nil},
	} {
		if m := f.do(c.method, c.path, c.body); m["ok"] != false || m["code"] != "snapshot-missing" {
			t.Errorf("%s %s = %v, want snapshot-missing", c.method, c.path, m)
		}
	}

	m := f.do("POST", "/api/containers/nginx/restore", map[string]any{"snapshotId": gone, "confirm": true})
	if m["ok"] != false || m["code"] != nil {
		t.Fatalf("local restore = %v, want the plain refusal without a code", m)
	}
}
