package api

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
)

// Without a binary to describe the backends there is nothing to tell a secret
// from a setting by, which is the case this host's tests can reach.
func TestTheRcloneAnswerNamesEachRemoteAndShowsNoValueRcloneDidNotClear(t *testing.T) {
	f := newPlacementFixture(t)
	f.svc.rcloneOnce.Do(func() {
		f.svc.rclone = &remotes.Rclone{Bin: filepath.Join(t.TempDir(), "no-rclone"), Config: f.svc.rcloneConfPath()}
	})
	const obscured, token, s3Secret = "b2JzY3VyZWQtcGFzcw", `{"access_token":"ya29.secret"}`, "wJalrXUtnFEMI-secret"
	conf := "[nas]\ntype = sftp\nhost = nas.example.org\nport = 2222\npass = " + obscured + "\n\n" +
		"[drive]\ntype = drive\ntoken = " + token + "\n\n" +
		"[bucket]\ntype = s3\nprovider = Wasabi\nsecret_access_key = " + s3Secret + "\n"
	if err := f.svc.SetRcloneConf(conf); err != nil {
		t.Fatal(err)
	}

	res := f.do("GET", "/api/rclone", nil)

	body, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{obscured, "ya29.secret", s3Secret, "nas.example.org", "2222", "Wasabi"} {
		if strings.Contains(string(body), secret) {
			t.Errorf("the answer carries %q: %s", secret, body)
		}
	}
	if got, want := res["remotes"], []any{"nas", "drive", "bucket"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("remotes = %v, want %v", got, want)
	}
	sections := res["sections"].([]any)
	if len(sections) != 3 {
		t.Fatalf("sections = %v, want one per remote", sections)
	}
	nas := sections[0].(map[string]any)
	want := []any{
		map[string]any{"name": "host", "hidden": true},
		map[string]any{"name": "port", "hidden": true},
		map[string]any{"name": "pass", "hidden": true},
	}
	if nas["name"] != "nas" || nas["type"] != "sftp" || !reflect.DeepEqual(nas["options"], want) {
		t.Fatalf("first section = %v, want nas of type sftp with three hidden options", nas)
	}
}

func TestTheRcloneAnswerOfAnEmptyConfigListsNothing(t *testing.T) {
	f := newPlacementFixture(t)
	f.svc.rcloneOnce.Do(func() {
		f.svc.rclone = &remotes.Rclone{Bin: filepath.Join(t.TempDir(), "no-rclone"), Config: f.svc.rcloneConfPath()}
	})

	res := f.do("GET", "/api/rclone", nil)

	if res["ok"] != true || len(res["remotes"].([]any)) != 0 || len(res["sections"].([]any)) != 0 {
		t.Fatalf("answer = %v, want two empty lists", res)
	}
}

func TestTheRcloneAnswerShowsTheSettingsRcloneClearsAndKeepsTheSecrets(t *testing.T) {
	f := newPlacementFixture(t)
	useFakeRclone(t, f.svc)
	const obscured, s3Secret = "b2JzY3VyZWQtcGFzcw", "wJalrXUtnFEMI-secret"
	conf := "[nas]\ntype = smb\nhost = nas.example.org\nport = 4450\npass = " + obscured + "\n\n" +
		"[bucket]\ntype = s3\nprovider = Wasabi\naccess_key_id = AKIDSECRET\nsecret_access_key = " + s3Secret + "\n"
	if err := f.svc.SetRcloneConf(conf); err != nil {
		t.Fatal(err)
	}

	res := f.do("GET", "/api/rclone", nil)

	body, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{obscured, s3Secret, "AKIDSECRET", "nas.example.org"} {
		if strings.Contains(string(body), secret) {
			t.Errorf("the answer carries %q: %s", secret, body)
		}
	}
	for _, shown := range []string{`{"name":"port","value":"4450"}`, `{"name":"provider","value":"Wasabi"}`} {
		if !strings.Contains(string(body), shown) {
			t.Errorf("the answer lacks %s: %s", shown, body)
		}
	}
}
