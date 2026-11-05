//go:build flows

package flows

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestABackupBehindALinkOutOfTheMountStopsAndSaysWhy(t *testing.T) {
	f := typedFlow(t, "files")
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "inner", "kept-out.txt"), []byte("outside the mount\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := f.b.hostPath("user/data/parent")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	id := f.b.createSet("behind", "user/data/parent/inner")
	known := len(f.b.runsOf("backup"))

	f.b.call("POST", "/api/files/sets/"+id+"/backup", nil, nil)

	got := f.b.awaitRun("backup", known)
	if got.Status == "success" || !strings.Contains(got.Error, "link above") {
		f.errorf("the run through the link reads %+v", got)
	}
	f.finish()
}

func TestAHiddenLocationMovesUnderItsPassword(t *testing.T) {
	t.Parallel()
	f := newFlow(t)
	f.b.start(currentBinary)
	const (
		typed  = "rest:https://bob:inventedpw@127.0.0.1:9/repo"
		hidden = "rest:https://[redacted]@127.0.0.1:9/repo"
		moved  = "rest:https://[redacted]@127.0.0.1:9/moved"
	)
	var created struct {
		Target map[string]any `json:"target"`
	}
	f.b.call("POST", "/api/offsite/targets", map[string]any{"domain": "files", "name": "Box", "repo": typed, "enabled": true, "sortOrder": 1}, &created)
	if created.Target["repo"] != hidden {
		f.errorf("the answer to the save shows %v", created.Target["repo"])
	}
	listed := func() map[string]any {
		t.Helper()
		var got struct {
			Targets []map[string]any `json:"targets"`
		}
		f.b.call("GET", "/api/offsite/targets?domain=files", nil, &got)
		for _, row := range got.Targets {
			if row["id"] == created.Target["id"] {
				return row
			}
		}
		t.Fatalf("the target is not listed: %+v", got.Targets)
		return nil
	}
	row := listed()
	if row["repo"] != hidden {
		f.errorf("the list shows %v", row["repo"])
	}

	row["repo"] = moved
	f.b.call("PUT", "/api/offsite/targets/"+row["id"].(string), row, nil)

	if got := listed()["repo"]; got != moved {
		f.errorf("after the edit the list shows %v, want %s", got, moved)
	}
	f.b.signIn()
	_, stored := f.b.send("GET", "/api/settings/export?includeCredentials=true", nil)
	if !strings.Contains(string(stored), "rest:https://bob:inventedpw@127.0.0.1:9/moved") {
		f.errorf("the stored location lost its password or did not move: %s", stored)
	}
	f.finish()
}

func TestFlashZipAndKeyHoldingExportsAskForAPassword(t *testing.T) {
	t.Parallel()
	f := newFlow(t)
	f.b.start(currentBinary)
	export := map[string]any{"form": "tar", "snapshotId": "latest"}
	for _, r := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/flash/download", nil},
		{"POST", "/api/items/config/config/export", export},
		{"POST", "/api/items/flash/flash/export", export},
		{"POST", "/api/items/config/config/export/preview", export},
	} {
		status, raw := f.b.send(r.method, r.path, r.body)
		if status != http.StatusForbidden || !strings.Contains(string(raw), "password-not-set") {
			f.errorf("%s %s without a password = %d: %s", r.method, r.path, status, raw)
		}
	}

	f.b.signIn()
	if status, raw := f.b.send("GET", "/api/flash/download", nil); status == http.StatusForbidden {
		f.errorf("the download with a password and a sign-in = %d: %s", status, raw)
	}
	f.finish()
}
