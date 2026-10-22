package api

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	boxSignedIn  = "[box]\ntype = box\ntoken = {\"refresh_token\":\"first\"}\n"
	boxRefreshed = "[box]\ntype = box\ntoken = {\"refresh_token\":\"second\"}\n"
)

// rcloneRefreshed stands in for rclone saving a refreshed sign-in into its
// file during a run.
func rcloneRefreshed(t *testing.T, svc *Service, conf string) {
	t.Helper()
	if err := os.WriteFile(svc.rcloneConfPath(), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestATokenRcloneRefreshedOutlivesARestart(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf(boxSignedIn)
	rcloneRefreshed(t, f.svc, boxRefreshed)

	if err := f.svc.WriteRcloneConfFile(); err != nil {
		t.Fatal(err)
	}

	if got := f.storedRcloneConf(); !strings.Contains(got, `"second"`) || strings.Contains(got, `"first"`) {
		t.Fatalf("stored config = %q, want the refreshed token", got)
	}
	if got := f.svc.rcloneFile(); !strings.Contains(got, `"second"`) {
		t.Fatalf("rclone.conf = %q, want the refreshed token back", got)
	}
}

func TestATokenRcloneRefreshedOutlivesAnotherRemoteBeingAdded(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf(boxSignedIn)
	rcloneRefreshed(t, f.svc, boxRefreshed)

	if res := f.do("POST", "/api/rclone", map[string]any{"conf": "[b2]\ntype = b2\naccount = a\n"}); res["ok"] != true {
		t.Fatalf("paste = %v", res)
	}

	for name, conf := range map[string]string{"stored config": f.storedRcloneConf(), "rclone.conf": f.svc.rcloneFile()} {
		if !strings.Contains(conf, `"second"`) || !strings.Contains(conf, "[b2]") {
			t.Errorf("%s = %q, want the refreshed token and the new remote", name, conf)
		}
	}
}

func TestTheRecoveryKitCarriesTheTokenRcloneRefreshed(t *testing.T) {
	f := newPlacementFixture(t)
	f.rcloneConf(boxSignedIn)
	rcloneRefreshed(t, f.svc, boxRefreshed)

	kit, err := f.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(kit, `"second"`) || strings.Contains(kit, `"first"`) {
		t.Fatal("the kit holds the token rclone had already replaced")
	}
}

func TestOnlyTheSignInOfTheSameRemoteIsTakenFromTheFile(t *testing.T) {
	stored := "[box]\ntype = box\ntoken = old\n\n[nas]\ntype = smb\nhost = nas\n"
	onDisk := "[box]\ntype = box\ntoken = new\nchunk_size = 1Gi\n\n[nas]\ntype = sftp\nsession_id = s\n\n[BVDRAFT]\ntype = mega\nsession_id = d\n"

	got := withRcloneRefreshes(stored, onDisk)

	want := "[box]\ntype = box\ntoken = new\n\n[nas]\ntype = smb\nhost = nas\n"
	if got != want {
		t.Fatalf("config =\n%s\nwant\n%s", got, want)
	}
}

// boxDestination saves a Box destination whose token rclone replaces on the
// first listing.
func boxDestination(t *testing.T, svc *Service) (store.OffsiteTarget, string) {
	t.Helper()
	useFakeRclone(t, svc)
	d, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "box", Name: "Box", Dir: "bv", Settings: map[string]string{"token": "rotate-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	remote, _, _ := strings.Cut(strings.TrimPrefix(d.Repo, "rclone:"), ":")
	return d, remote
}

func TestAConnectionCheckThatRefreshedTheTokenSavesTheNewOne(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	d, remote := boxDestination(t, svc)

	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Dir: folder("bv2")}); err != nil {
		t.Fatal(err)
	}

	if got, _ := storedRemote(t, svc, remote); got["token"] != "rotated-1" {
		t.Fatalf("token = %q, want the one the check refreshed to", got["token"])
	}
}

func TestARefusedConnectionChangeKeepsTheTokenItsCheckRefreshed(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	d, remote := boxDestination(t, svc)

	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, connectionEdit{Dir: folder("missing")}); err == nil {
		t.Fatal("a folder that is not there was saved")
	}

	if got, _ := storedRemote(t, svc, remote); got["token"] != "rotated-1" {
		t.Fatalf("token = %q, want the one the check refreshed to", got["token"])
	}
	if _, left := storedRemote(t, svc, remote+"-check"); left {
		t.Fatal("the remote it was tried under is still in the config")
	}
}

func TestARefusedChangeOfAnotherOptionKeepsTheTokenItsCheckRefreshed(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	d, remote := boxDestination(t, svc)

	edit := connectionEdit{Dir: folder("missing"), Settings: map[string]string{"token": "", "root_folder_id": "42"}}
	if _, _, err := svc.EditDestinationConnection(context.Background(), d.ID, edit); err == nil {
		t.Fatal("a folder that is not there was saved")
	}
	if got, _ := storedRemote(t, svc, remote); got["token"] != "rotated-1" {
		t.Fatalf("token = %q, want the one the check refreshed to", got["token"])
	}
}

func TestAFolderWalkOfASavedRemoteWhoseTokenRcloneRefreshesGoesThrough(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	d, _ := boxDestination(t, svc)

	if _, _, err := svc.DraftFolders(context.Background(), draftRequest{Provider: "box", Destination: d.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestAFolderWalkOfASavedRemoteKeepsTheTokenRcloneRefreshed(t *testing.T) {
	svc, _, _ := newConnectionSvc(t, nil)
	d, remote := boxDestination(t, svc)

	if _, _, err := svc.DraftFolders(context.Background(), draftRequest{Provider: "box", Destination: d.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _ := storedRemote(t, svc, remote); got["token"] != "rotated-1" {
		t.Fatalf("token after the walk = %q, want the one rclone refreshed to", got["token"])
	}
}
