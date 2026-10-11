package api

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// encrypted switches encryption on, so a mode carries a password to look at.
func (f *placementFixture) encrypted() store.Settings {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	settings.EncryptionEnabled = true
	if err := f.st.UpdateSettings(settings); err != nil {
		f.t.Fatal(err)
	}
	return settings
}

// adopt stores key as the adopted backup key, sealed under sealedWith.
func (f *placementFixture) adopt(key, sealedWith string) {
	f.t.Helper()
	enc, err := secret.Encrypt(sealedWith, []byte(key))
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.st.SetAdoptedKey(enc, store.KeyFromKit, true); err != nil {
		f.t.Fatal(err)
	}
	f.svc.forgetBackupKey()
}

func (f *placementFixture) statusBackupKey() any {
	f.t.Helper()
	return f.do("GET", "/api/status", nil)["backupKey"]
}

func TestAnInstallThatAdoptedNothingBacksUpUnderItsAppKey(t *testing.T) {
	f := newPlacementFixture(t)
	settings := f.encrypted()
	own := f.svc.cfg.AppKey

	if got, want := f.svc.ModeFor(settings), (restic.Mode{Encrypted: true, Password: restickey.Derive(own)}); got.Encrypted != want.Encrypted || got.Password != want.Password {
		t.Fatalf("mode = %+v, want the password derived from APP_KEY", got)
	}
	if got := f.svc.oppositeMode(restic.Mode{}).Password; got != restickey.Derive(own) {
		t.Fatalf("the probe of the other mode carries %q, want the password derived from APP_KEY", got)
	}
	if got := f.svc.resticPassword(); got != restickey.Derive(own) {
		t.Fatalf("the password a member is given = %q, want the one derived from APP_KEY", got)
	}
	sealed, err := f.svc.sealDefinition([]byte(`{"name":"plex"}`))
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := secret.Decrypt(own, sealed); err != nil || string(plain) != `{"name":"plex"}` {
		t.Fatalf("a definition sealed without an adopted key opens under APP_KEY as %q, %v", plain, err)
	}
	if got := f.statusBackupKey(); got != backupKeyOwn {
		t.Fatalf("status names the backup key %v, want %q", got, backupKeyOwn)
	}
	if _, found, err := f.st.AdoptedKey(); err != nil || found {
		t.Fatalf("reading the backup key stored one: found %v, %v", found, err)
	}
}

func TestAnAdoptedKeyTakesOverTheBackupsAndNothingElse(t *testing.T) {
	f := newPlacementFixture(t)
	settings := f.encrypted()
	own, old := f.svc.cfg.AppKey, strings.Repeat("d", 64)
	f.adopt(old, own)

	if got := f.svc.ModeFor(settings).Password; got != restickey.Derive(old) {
		t.Fatalf("the restic password = %q, want the one derived from the adopted key", got)
	}
	if got := f.svc.resticPassword(); got != restickey.Derive(old) {
		t.Fatalf("the password a member is given = %q, want the one derived from the adopted key", got)
	}
	sealed, err := f.svc.sealDefinition([]byte("definition"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secret.Decrypt(old, sealed); err != nil {
		t.Fatalf("a definition is not sealed under the adopted key: %v", err)
	}
	if plain, err := f.svc.openDefinition(sealed); err != nil || string(plain) != "definition" {
		t.Fatalf("openDefinition = %q, %v", plain, err)
	}
	if got := f.statusBackupKey(); got != backupKeyAdopted {
		t.Fatalf("status names the backup key %v, want %q", got, backupKeyAdopted)
	}

	// What the install seals for itself stays under its own key.
	conf, err := secret.Encrypt(own, []byte("[remote]"))
	if err != nil {
		t.Fatal(err)
	}
	settings.RcloneConf = base64.StdEncoding.EncodeToString(conf)
	if got, err := f.svc.decodeRcloneConf(settings); err != nil || got != "[remote]" {
		t.Fatalf("a secret sealed under APP_KEY reads %q, %v after the adoption", got, err)
	}
}

func TestAnAdoptedKeyThatNoLongerOpensLocksTheBackups(t *testing.T) {
	f := newPlacementFixture(t)
	settings := f.encrypted()
	f.adopt(strings.Repeat("d", 64), strings.Repeat("e", 64))

	if got := f.statusBackupKey(); got != backupKeyLocked {
		t.Fatalf("status names the backup key %v, want %q", got, backupKeyLocked)
	}
	mode := f.svc.ModeFor(settings)
	if mode.Password != lockedPassword {
		t.Fatalf("a locked key gives the restic password %q, which must open nothing", mode.Password)
	}
	fresh := f.root + "/backups/fresh"
	f.eng.opens[fresh] = false
	if err := f.svc.EnsureRepo(context.Background(), fresh, mode); !errors.Is(err, errBackupKeyLocked) {
		t.Fatalf("EnsureRepo under a locked key = %v, want %v", err, errBackupKeyLocked)
	}
	if len(f.eng.ensured) != 0 {
		t.Fatalf("a repository was created under a locked key: %v", f.eng.ensured)
	}
	if _, err := f.svc.sealDefinition([]byte("definition")); !errors.Is(err, errBackupKeyLocked) {
		t.Fatalf("sealing a definition under a locked key = %v, want %v", err, errBackupKeyLocked)
	}
	if got := f.svc.resticPassword(); got != "" {
		t.Fatalf("a member is given the password %q while the key is locked", got)
	}
	if code := placementCode(errBackupKeyLocked); code != "backup-key-locked" {
		t.Fatalf("the refusal carries the code %q", code)
	}
}
