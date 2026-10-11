package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestARequestActsOnlyOnTheEmergencyItNames(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	_, first := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	_, second := f.emergency("POST", "/api/emergency/read", map[string]any{"key": strings.Repeat("e", 64)})
	if first.Session.ID == "" || second.Session.ID == "" || first.Session.ID == second.Session.ID {
		t.Fatalf("the two emergencies are named %q and %q", first.Session.ID, second.Session.ID)
	}

	for name, body := range map[string]map[string]any{
		"the emergency that was replaced": {"session": first.Session.ID},
		"no emergency":                    {},
	} {
		for _, path := range []string{"/api/emergency/adopt", "/api/emergency/sign-ins", "/api/emergency/finish"} {
			if res, _ := f.emergency("POST", path, body); res["code"] != "emergency-ended" {
				t.Errorf("%s naming %s: %v", path, name, res)
			}
		}
	}
	if _, found := f.adoptedRow(); found {
		t.Fatal("a key was taken over by a request that named another emergency")
	}
	if _, view := f.emergency("GET", "/api/emergency", nil); view.Session == nil || view.Session.ID != second.Session.ID {
		t.Fatalf("the emergency in progress after the refusals: %+v", view.Session)
	}
}

func TestCancellingAnEmergencyLeavesAnEarlierAdoptionAlone(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("adopt: %v", res)
	}
	f.emergency("POST", "/api/emergency/finish", nil)

	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"key": strings.Repeat("e", 64)})
	if res, _ := f.emergency("DELETE", "/api/emergency", nil); res["code"] != "emergency-ended" {
		t.Fatalf("a cancel that names no emergency: %v", res)
	}
	res, after := f.emergency("DELETE", "/api/emergency?session="+view.Session.ID, nil)
	if res["ok"] != true || after.Session != nil || after.BackupKey != backupKeyAdopted {
		t.Fatalf("cancel: %v, %+v", res, after)
	}
	if key, err := f.svc.backupKey(); err != nil || key != oldKey {
		t.Fatalf("cancelling a fresh emergency put the key back that an earlier one took over: %v", err)
	}

	res, after = f.emergency("POST", "/api/emergency/undo", nil)
	if res["ok"] != true || after.BackupKey != backupKeyOwn {
		t.Fatalf("undo: %v, %+v", res, after)
	}
}

func TestAKeyIsNotTakenOverWhileABackupRunsOrOverARepositoryOfThisServer(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})

	f.svc.batchActive.Store(true)
	if _, busy := f.emergency("GET", "/api/emergency", nil); busy.Session.Refusal != "adopt-busy" {
		t.Fatalf("while a backup runs the emergency says %q", busy.Session.Refusal)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["code"] != "adopt-busy" {
		t.Fatalf("adopt while a backup runs: %v", res)
	}
	f.svc.batchActive.Store(false)

	// A repository the kit names is the old install's, whoever opened it.
	for _, p := range view.Session.Places {
		if p.Folder == "" {
			continue
		}
		repo, err := f.svc.resolveRepo(p.Folder)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.st.MarkRepoEstablished(repo); err != nil {
			t.Fatal(err)
		}
	}
	if _, named := f.emergency("GET", "/api/emergency", nil); named.Session.Refusal != "" {
		t.Fatalf("with the kit's own repositories opened the emergency says %q", named.Session.Refusal)
	}

	// One it does not name is this server's, also after the entry that was
	// backed up into it is gone with its runs.
	own, err := f.svc.resolveRepo("backups/elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.MarkRepoEstablished(own); err != nil {
		t.Fatal(err)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["code"] != "adopt-own-backups" {
		t.Fatalf("adopt over a repository of this server's own: %v", res)
	}
	if _, found := f.adoptedRow(); found {
		t.Fatal("the key was taken over")
	}
}

func TestSignInsOfAKitReplaceTheServersOwnOnlyWhenToldAndAnUndoBringsThemBack(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	if err := f.svc.SetCloudCreds(CloudCreds{RESTUser: "mine", RESTPassword: "my-secret"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetRcloneConf("[mine]\ntype = sftp"); err != nil {
		t.Fatal(err)
	}
	own := func() (CloudCreds, string) {
		t.Helper()
		settings, err := f.st.GetSettings()
		if err != nil {
			t.Fatal(err)
		}
		creds, _ := f.svc.decodeCloud(settings)
		conf, _ := f.svc.decodeRcloneConf(settings)
		return creds, conf
	}

	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if !view.Session.Replaces.CloudSignIn || view.Session.Replaces.RcloneConfig {
		t.Fatalf("the emergency says it would replace %+v", view.Session.Replaces)
	}
	if res, _ := f.emergency("POST", "/api/emergency/sign-ins", nil); res["ok"] != false || res["code"] != "sign-ins-exist" {
		t.Fatalf("sign-ins over the server's own without a word: %v", res)
	}
	if creds, _ := own(); creds.RESTUser != "mine" || creds.RESTPassword != "my-secret" {
		t.Fatalf("the refused request changed the sign-in to %+v", creds)
	}

	if res, _ := f.emergency("POST", "/api/emergency/sign-ins", map[string]any{"session": f.sessionID(), "replace": true}); res["ok"] != true {
		t.Fatalf("sign-ins with replace: %v", res)
	}
	if creds, conf := own(); creds.RESTUser != "bv" || creds.RESTPassword != "rest-secret" || !strings.Contains(conf, "[mine]") {
		t.Fatalf("after the replace the server has %+v and %q", creds, conf)
	}

	if res, _ := f.emergency("POST", "/api/emergency/undo", nil); res["ok"] != true {
		t.Fatalf("undo: %v", res)
	}
	if creds, conf := own(); creds.RESTUser != "mine" || creds.RESTPassword != "my-secret" || !strings.Contains(conf, "[mine]") {
		t.Fatalf("after the undo the server has %+v and %q", creds, conf)
	}
	if _, found, _ := f.st.ReplacedSignIns(); found {
		t.Fatal("the undo left the record of what was replaced")
	}
}

func TestAfterTheSignInsAreStoredThePlacesBehindThemAreLookedIntoAgain(t *testing.T) {
	old := newPlacementFixture(t)
	old.svc.cfg.AppKey = oldKey
	old.svc.forgetBackupKey()
	if err := old.svc.SetRcloneConf("[box]\ntype = sftp"); err != nil {
		t.Fatal(err)
	}
	if _, err := old.st.MutateSettings(func(s *store.Settings) error {
		s.EncryptionEnabled, s.FilesOffsite = true, "rclone:box:files"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	kit, err := old.svc.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}

	f := newPlacementFixture(t)
	f.eng.opens["rclone:box:files"] = true
	_, view := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if got := placeStates(view)["offsite/files"]; got != placeNeedsSignIn {
		t.Fatalf("before the rclone config is this server's the place behind it reads %q", got)
	}
	res, view := f.emergency("POST", "/api/emergency/sign-ins", nil)
	if res["ok"] != true {
		t.Fatalf("sign-ins: %v", res)
	}
	if got := placeStates(view)["offsite/files"]; got != placeReadable {
		t.Fatalf("after the sign-ins were stored the place reads %q", got)
	}
}

func TestASettingsRestoreBelongsToAnEmergencyOnlyWhenItNamesIt(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	old := settingsOfTheOldInstall(t, oldKey)
	f := newPlacementFixture(t)
	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("adopt: %v", res)
	}

	f.stage(old)
	if err := f.svc.takeOverStagedSettings(t.Context()); !errors.Is(err, errSettingsOtherKey) {
		t.Fatalf("a restore that names no emergency, of settings under another key: %v", err)
	}
	f.stage(old)
	if err := f.svc.takeOverStagedSettings(forEmergency(t.Context(), strings.Repeat("0", 32))); !errors.Is(err, errNoEmergency) {
		t.Fatalf("a restore that names an emergency that is not in progress: %v", err)
	}
	f.stage(old)
	if err := f.svc.takeOverStagedSettings(f.inEmergency()); err != nil {
		t.Fatalf("a restore that names the emergency: %v", err)
	}
}
