package api

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func (f *placementFixture) storedRcloneConf() string {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	conf, err := f.svc.decodeRcloneConf(settings)
	if err != nil {
		f.t.Fatal(err)
	}
	return conf
}

func TestAPastedRemoteJoinsTheOnesTheWizardSaved(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf("[gdrive]\ntype = drive\ntoken = {\"access_token\":\"t\"}\n\n[b2]\ntype = b2\naccount = old\n")

	res := f.do("POST", "/api/rclone", map[string]any{"conf": "[mybox]\ntype = sftp\nhost = box.example.com\n\n[b2]\ntype = b2\naccount = new\n"})
	if res["ok"] != true {
		t.Fatalf("paste = %v", res)
	}

	conf := f.storedRcloneConf()
	if got := parseRcloneRemotes(conf); strings.Join(slices.Sorted(slices.Values(got)), ",") != "b2,gdrive,mybox" {
		t.Fatalf("remotes = %v, want gdrive, b2 and mybox", got)
	}
	if !strings.Contains(conf, "account = new") || strings.Contains(conf, "account = old") {
		t.Errorf("b2 was not replaced by the pasted one:\n%s", conf)
	}
	onDisk, err := os.ReadFile(f.svc.rcloneConfPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != conf {
		t.Errorf("rclone.conf = %q, want the stored config %q", onDisk, conf)
	}
}

func TestAPasteWithoutARemoteChangesNothing(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf("[gdrive]\ntype = drive\n")

	for _, pasted := range []string{"", "type = sftp\nhost = box\n"} {
		if res := f.do("POST", "/api/rclone", map[string]any{"conf": pasted}); res["ok"] != false {
			t.Errorf("paste %q = %v, want a refusal", pasted, res)
		}
	}
	if got := f.storedRcloneConf(); got != "[gdrive]\ntype = drive\n" {
		t.Fatalf("stored config = %q, want it unchanged", got)
	}
}
