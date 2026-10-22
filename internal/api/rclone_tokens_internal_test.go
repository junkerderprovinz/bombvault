package api

import (
	"os"
	"strings"
	"testing"
)

const pastedBox = "[box]\ntype = box\ntoken = {\"refresh_token\":\"first\"}\n\n[nas]\ntype = smb\nhost = nas\n"

// rcloneRefreshes writes a new Box token into the file the way rclone does
// after a refresh.
func rcloneRefreshes(t *testing.T, f *placementFixture) {
	t.Helper()
	onDisk := strings.Replace(pastedBox, `"first"`, `"second"`, 1)
	if err := os.WriteFile(f.svc.rcloneConfPath(), []byte(onDisk), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestATokenRcloneRefreshedSurvivesARestart(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf(pastedBox)
	rcloneRefreshes(t, f)

	if err := f.svc.WriteRcloneConfFile(); err != nil {
		t.Fatal(err)
	}

	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	stored, err := f.svc.storedRcloneConf(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored, `"second"`) {
		t.Fatalf("stored config = %q, want the refreshed token", stored)
	}
	onDisk, err := os.ReadFile(f.svc.rcloneConfPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), `"second"`) {
		t.Fatalf("rclone.conf = %q, want the refreshed token back on disk", onDisk)
	}
}

func TestAddingARemoteKeepsATokenRcloneRefreshed(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf(pastedBox)
	rcloneRefreshes(t, f)

	if err := f.svc.editRcloneConf(func(conf string) (string, error) {
		return replaceRcloneSection(conf, "other", "[other]\ntype = sftp\nhost = box\n"), nil
	}); err != nil {
		t.Fatal(err)
	}

	onDisk, err := os.ReadFile(f.svc.rcloneConfPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), `"second"`) || !strings.Contains(string(onDisk), "[other]") {
		t.Fatalf("rclone.conf = %q, want the new remote beside the refreshed token", onDisk)
	}
}

func TestAnExportCarriesTheTokenRcloneRefreshed(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf(pastedBox)
	rcloneRefreshes(t, f)

	if conf := f.storedRcloneConf(); !strings.Contains(conf, `"second"`) {
		t.Fatalf("config handed out = %q, want the refreshed token", conf)
	}
}

func TestOnlyWhatRcloneRefreshesIsTakenFromItsFile(t *testing.T) {
	stored := "[box]\ntype = box\ntoken = old\n\n[cloud]\ntype = drive\ntoken = keep\n\n[mail]\ntype = iclouddrive\napple_id = me\n"
	disk := "[box]\ntype = box\ntoken = new\nroot_folder_id = 9\n\n[cloud]\ntype = onedrive\ntoken = other\n\n[mail]\ntype = iclouddrive\napple_id = changed\ncookies = c\n\n[stray]\ntype = box\ntoken = x\n"

	got := withRefreshedTokens(stored, disk)

	want := "[box]\ntype = box\ntoken = new\n\n[cloud]\ntype = drive\ntoken = keep\n\n[mail]\ntype = iclouddrive\napple_id = me\ncookies = c\n"
	if got != want {
		t.Fatalf("merged =\n%s\nwant\n%s", got, want)
	}
	if withRefreshedTokens(stored, "") != stored || withRefreshedTokens(stored, stored) != stored {
		t.Fatal("a file with nothing new changed the stored config")
	}
}
