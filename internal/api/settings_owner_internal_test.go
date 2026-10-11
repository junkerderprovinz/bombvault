package api

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// bareSettings writes the settings database of an install under key that
// seals nothing and has a login. recorded says whether the install wrote the
// check value of its key, as every start does.
func bareSettings(t *testing.T, key, password string, recorded bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bombvault.sqlite")
	svc, st, closeDB := fileInstall(t, path, key)
	hash, err := secret.HashPassword(key, password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash, s.InstanceName = hash, "tower"; return nil }); err != nil {
		t.Fatal(err)
	}
	if recorded {
		if err := svc.RecordKeyCheck(); err != nil {
			t.Fatal(err)
		}
	}
	closeDB()
	return path
}

// withPassword gives the server of f a login.
func (f *placementFixture) withPassword(password string) {
	f.t.Helper()
	hash, err := secret.HashPassword(f.svc.cfg.AppKey, password)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = hash; return nil }); err != nil {
		f.t.Fatal(err)
	}
}

func restoredLogin(t *testing.T, staged, key string) string {
	t.Helper()
	_, st, _ := fileInstall(t, staged, key)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	return settings.AuthPasswordHash
}

func TestOwnSettingsRestoredWhileAnEmergencyLingersKeepTheServersKey(t *testing.T) {
	f := newPlacementFixture(t)
	own := f.svc.cfg.AppKey
	f.backupRun(f.container("nginx", "").ID, time.Now().Unix()-3600)
	typed := strings.Repeat("e", 64)
	if res, _ := f.emergency("POST", "/api/emergency/read", map[string]any{"key": typed}); res["ok"] != true {
		t.Fatalf("read: %v", res)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != false {
		t.Fatalf("a typed key was adopted over the server's own backups: %v", res)
	}

	f.withPassword("this server's password")
	staged := f.stage(settingsOfTheOldInstall(t, own))
	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatalf("takeOverStagedSettings: %v", err)
	}
	svc, _, _ := fileInstall(t, staged, own)
	if key, err := svc.backupKey(); err != nil || key != own || svc.currentBackupKey().state != backupKeyOwn {
		t.Fatalf("the restored settings back up under another key than the server's own (state %s, %v)", svc.currentBackupKey().state, err)
	}
	// Without the check value nothing shows the login in them is this server's.
	if hash := restoredLogin(t, staged, own); !secret.VerifyPassword(own, "this server's password", hash) {
		t.Fatal("settings without a check value came back with their own login")
	}
}

func TestSettingsOfAnotherKeyWithNothingSealedAreToldByTheirCheckValue(t *testing.T) {
	f := newPlacementFixture(t)
	f.withPassword("this server's password")
	f.stage(bareSettings(t, oldKey, "the old password", true))
	if err := f.svc.takeOverStagedSettings(context.Background()); !errors.Is(err, errSettingsOtherKey) {
		t.Fatalf("settings that carry another key's check value: %v, want %v", err, errSettingsOtherKey)
	}

	own := newPlacementFixture(t)
	staged := own.stage(bareSettings(t, own.svc.cfg.AppKey, "an older password", true))
	if err := own.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatalf("settings that carry this server's check value: %v", err)
	}
	if hash := restoredLogin(t, staged, own.svc.cfg.AppKey); !secret.VerifyPassword(own.svc.cfg.AppKey, "an older password", hash) {
		t.Fatal("this server's own settings lost their login")
	}
}

func TestSettingsThatDoNotShowWhoseTheyAreComeBackWithThisServersLogin(t *testing.T) {
	f := newPlacementFixture(t)
	own := f.svc.cfg.AppKey
	f.withPassword("this server's password")
	staged := f.stage(bareSettings(t, oldKey, "the old password", false))
	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatalf("takeOverStagedSettings: %v", err)
	}
	if hash := restoredLogin(t, staged, own); !secret.VerifyPassword(own, "this server's password", hash) {
		t.Fatal("the login is one no password on this server matches")
	}
}

func TestASecondFactorSealedWithAnotherKeyMarksTheSettingsAsAnothers(t *testing.T) {
	path := bareSettings(t, oldKey, "the old password", false)
	sealed, err := secret.Encrypt(oldKey, []byte("GEZDGNBVGY3TQOJQ"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenStaged(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE settings SET totp_secret = ?, totp_enabled = 1`, hex.EncodeToString(sealed)); err != nil {
		t.Fatal(err)
	}
	if err := store.CloseStaged(db); err != nil {
		t.Fatal(err)
	}

	f := newPlacementFixture(t)
	f.stage(path)
	if err := f.svc.takeOverStagedSettings(context.Background()); !errors.Is(err, errSettingsOtherKey) {
		t.Fatalf("settings whose second factor no key here opens: %v, want %v", err, errSettingsOtherKey)
	}
}

func TestASettingsRestoreStoresNoKeyAnAdoptionWouldRefuse(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	f.backupRun(f.container("nginx", "").ID, time.Now().Unix()-3600)
	res, _ := f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res["ok"] != true {
		t.Fatalf("read: %v", res)
	}
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["code"] != "adopt-own-backups" {
		t.Fatalf("adopt on a server with backups of its own: %v", res)
	}

	staged := f.stage(settingsOfTheOldInstall(t, oldKey))
	if err := f.svc.takeOverStagedSettings(f.inEmergency()); !errors.Is(err, errAdoptOwnBackups) {
		t.Fatalf("the settings of the old install on a server with backups of its own: %v, want %v", err, errAdoptOwnBackups)
	}
	db, err := store.OpenStaged(staged)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test database
	if has, err := store.HasAdoptedKey(db); err != nil || has {
		t.Fatalf("the refused restore left a key behind: %v, %v", has, err)
	}
}

func TestWhoseSettingsGoesByTheDatabaseBeforeTheEmergency(t *testing.T) {
	own, old, third := strings.Repeat("a", 64), strings.Repeat("d", 64), strings.Repeat("9", 64)
	for name, c := range map[string]struct {
		check, from  string
		opened, lost int
		want         stagedOwner
	}{
		"own check value, emergency in progress":       {secret.KeyCheck(own), old, 0, 0, stagedOwn},
		"the emergency's check value":                  {secret.KeyCheck(old), old, 0, 0, stagedForeign},
		"a third key's check value":                    {secret.KeyCheck(third), old, 0, 3, stagedOther},
		"another key's check value over own values":    {secret.KeyCheck(third), "", 2, 0, stagedOwn},
		"no check value, values open here":             {"", old, 1, 0, stagedOwn},
		"no check value, emergency in progress":        {"", old, 0, 0, stagedForeign},
		"no check value, nothing opens, no emergency":  {"", "", 0, 2, stagedOther},
		"no check value, nothing sealed, no emergency": {"", "", 0, 0, stagedUnknown},
	} {
		if got := whoseSettings(c.check, own, c.from, c.opened, c.lost); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
}
