package api

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func newDraftSvc(t *testing.T) *Service {
	t.Helper()
	svc, _, _ := newConnectionSvc(t, nil)
	useFakeRclone(t, svc)
	return svc
}

// savedRemote saves the draft and returns the settings of its remote.
func savedRemote(t *testing.T, svc *Service, req draftRequest) map[string]string {
	t.Helper()
	req.Dir = "bv"
	d, err := svc.SaveNewDestination(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	remote, _, _ := strings.Cut(strings.TrimPrefix(d.Repo, "rclone:"), ":")
	got, ok := storedRemote(t, svc, remote)
	if !ok {
		t.Fatalf("no remote %s in the config", remote)
	}
	return got
}

func TestAMegaSignInWithATwoFactorCodeIsSavedWithTheSessionItOpened(t *testing.T) {
	svc := newDraftSvc(t)
	req := draftRequest{Provider: "mega", Settings: map[string]string{"user": "me@example.com", "pass": "pw", "2fa": "123456"}}

	if _, err := svc.CheckDraft(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := savedRemote(t, svc, req)

	if got["session_id"] != "session-of-me@example.com" || got["master_key"] != "key" {
		t.Fatalf("remote = %v, want the session the test opened", got)
	}
	if _, kept := got["2fa"]; kept {
		t.Fatal("the used two-factor code was saved")
	}
	if strings.Contains(svc.rcloneFile(), "BVDRAFT") {
		t.Fatal("the draft's session landed in BombVault's own rclone config")
	}
}

func TestAMegaCodeNoTestUsedIsNotSaved(t *testing.T) {
	svc := newDraftSvc(t)

	_, err := svc.SaveNewDestination(context.Background(), draftRequest{
		Provider: "mega", Dir: "bv", Settings: map[string]string{"user": "me@example.com", "pass": "pw", "2fa": "123456"},
	})
	if err == nil || !strings.Contains(err.Error(), "two-factor code") {
		t.Fatalf("err = %v, want the refusal to keep a one-time code", err)
	}
}

func TestATokenRefreshedDuringTheTestIsTheOneSaved(t *testing.T) {
	svc := newDraftSvc(t)
	req := draftRequest{Provider: "box", Settings: map[string]string{"token": "rotate-1"}}

	if _, err := svc.CheckDraft(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	if got := savedRemote(t, svc, req); got["token"] != "rotated-1" {
		t.Fatalf("token = %q, want the one the test refreshed to", got["token"])
	}
}

func TestOneDriveAsksForTheDriveAndSavesTheOnePicked(t *testing.T) {
	svc := newDraftSvc(t)
	req := draftRequest{Provider: "onedrive", Settings: map[string]string{"token": `{"access_token":"a"}`}}

	got, err := svc.CheckDraft(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reachable || got.Question == nil || got.Question.Name != "config_driveid" || len(got.Question.Choices) != 2 {
		t.Fatalf("check = %+v, want the question which drive", got)
	}
	req.Settings["config_driveid"] = "team-drive"
	if got, err := svc.CheckDraft(context.Background(), req); err != nil || !got.Reachable {
		t.Fatalf("check = %+v, %v; want it reached", got, err)
	}

	saved := savedRemote(t, svc, req)
	if saved["drive_id"] != "team-drive" || saved["drive_type"] != "business" {
		t.Fatalf("remote = %v, want the drive picked", saved)
	}
	if saved["hard_delete"] != "true" {
		t.Fatalf("hard_delete = %q, want a business drive to delete outright", saved["hard_delete"])
	}
	if _, kept := saved["config_driveid"]; kept {
		t.Fatal("the answer to the setup's question was saved as a setting")
	}
}

func TestJottacloudTradesTheLoginTokenOnce(t *testing.T) {
	svc := newDraftSvc(t)
	req := draftRequest{Provider: "jottacloud", Settings: map[string]string{"token": "LOGIN"}}

	if got, err := svc.CheckDraft(context.Background(), req); err != nil || !got.Reachable {
		t.Fatalf("check = %+v, %v; want it reached", got, err)
	}

	saved := savedRemote(t, svc, req)
	if saved["token"] != `{"access_token":"for-LOGIN"}` || saved["configVersion"] != "1" || saved["client_id"] != "jottacli" {
		t.Fatalf("remote = %v, want what the setup traded the login token for", saved)
	}
}

func TestJottacloudIsNotSavedBeforeItsLoginTokenIsTraded(t *testing.T) {
	svc := newDraftSvc(t)

	_, err := svc.SaveNewDestination(context.Background(), draftRequest{Provider: "jottacloud", Dir: "bv", Settings: map[string]string{"token": "LOGIN"}})
	if err == nil {
		t.Fatal("a login token nothing traded was saved as the sign-in")
	}
}

func TestZohoTakesItsOnlyWorkspaceWithoutAsking(t *testing.T) {
	svc := newDraftSvc(t)
	req := draftRequest{Provider: "zoho", Settings: map[string]string{"token": `{"access_token":"a"}`, "region": "eu"}}

	if got, err := svc.CheckDraft(context.Background(), req); err != nil || !got.Reachable {
		t.Fatalf("check = %+v, %v; want it reached", got, err)
	}

	if saved := savedRemote(t, svc, req); saved["root_folder_id"] != "only-workspace" {
		t.Fatalf("remote = %v, want the workspace", saved)
	}
}

func TestAShareFileTokenFromAConfigBlobBringsItsEndpoint(t *testing.T) {
	svc := newDraftSvc(t)
	blob := base64.RawStdEncoding.EncodeToString([]byte(`{"token":"{\"access_token\":\"a\"}","endpoint":"https://acme.sf-api.com"}`))

	saved := savedRemote(t, svc, draftRequest{Provider: "sharefile", Settings: map[string]string{"token": blob}})

	if saved["endpoint"] != "https://acme.sf-api.com" || saved["token"] != `{"access_token":"a"}` {
		t.Fatalf("remote = %v, want the token and the endpoint the blob carries", saved)
	}
}

func TestGoogleDriveDeletesPrunedPacksOutright(t *testing.T) {
	svc := newDraftSvc(t)

	if saved := savedRemote(t, svc, draftRequest{Provider: "gdrive", Settings: map[string]string{"token": `{"access_token":"a"}`}}); saved["use_trash"] != "false" {
		t.Fatalf("use_trash = %q, want pruned packs kept out of the trash", saved["use_trash"])
	}
}
