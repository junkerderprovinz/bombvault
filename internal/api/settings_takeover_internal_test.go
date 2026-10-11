package api

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/selfrestore"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fileInstall is a Service over a settings database in a file, as a settings
// backup holds one. close has to run before the file is copied or read.
func fileInstall(t *testing.T, path, appKey string) (svc *Service, st *store.Repo, closeDB func()) {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st = store.New(db)
	dir := filepath.Dir(path)
	svc = NewService(config.Config{AppKey: appKey, DataDir: dir, HostMountRoot: filepath.ToSlash(dir)}, st, nil, nil, &placementEngine{})
	closed := false
	closeDB = func() {
		if !closed {
			closed = true
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(closeDB)
	return svc, st, closeDB
}

// settingsOfTheOldInstall writes the settings database of an install under
// key, with a secret of every sealed form and a login, and returns its path.
func settingsOfTheOldInstall(t *testing.T, key string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bombvault.sqlite")
	svc, st, closeDB := fileInstall(t, path, key)
	if err := svc.SetCloudCreds(CloudCreds{S3KeyID: "AKIA", S3Secret: "s3-secret"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRcloneConf("[box]\ntype = sftp"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetNotifyConfig(notify.Config{WebhookURL: "https://hooks.example/abc", On: "failure"}); err != nil {
		t.Fatal(err)
	}
	group, err := secret.Encrypt(key, []byte(strings.Repeat("g", 16)))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetGroupSecret(group, time.Now()); err != nil {
		t.Fatal(err)
	}
	hash, err := secret.HashPassword(key, "the old password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash, s.InstanceName = hash, "tower"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	closeDB()
	return path
}

// stage puts a settings database where a settings restore stages one. The
// staging folder repeats the data folder's own path below it, which a path
// with a drive letter cannot be, so the data folder becomes a relative one.
func (f *placementFixture) stage(from string) string {
	f.t.Helper()
	f.t.Chdir(f.t.TempDir())
	f.svc.cfg.DataDir = "data"
	dir := selfrestore.RestoredSnapshotDir(f.svc.cfg.DataDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		f.t.Fatal(err)
	}
	src, err := os.Open(from) //nolint:gosec // test file
	if err != nil {
		f.t.Fatal(err)
	}
	defer src.Close() //nolint:errcheck // test file
	staged := filepath.Join(dir, "bombvault.sqlite")
	dst, err := os.Create(staged) //nolint:gosec // test file
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		f.t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		f.t.Fatal(err)
	}
	return staged
}

func TestSettingsOfTheOldInstallComeBackUnderThisServersKey(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	old := settingsOfTheOldInstall(t, oldKey)
	f := newPlacementFixture(t)
	own := f.svc.cfg.AppKey
	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	if res, _ := f.emergency("POST", "/api/emergency/adopt", nil); res["ok"] != true {
		t.Fatalf("adopt: %v", res)
	}
	freshHash, err := secret.HashPassword(own, "the new password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.MutateSettings(func(s *store.Settings) error { s.AuthPasswordHash = freshHash; return nil }); err != nil {
		t.Fatal(err)
	}
	staged := f.stage(old)

	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatalf("takeOverStagedSettings: %v", err)
	}

	// The next boot opens the staged file as its settings, under this key.
	svc, st, _ := fileInstall(t, staged, own)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if creds, err := svc.decodeCloud(settings); err != nil || creds.S3Secret != "s3-secret" {
		t.Fatalf("the cloud sign-in reads %+v, %v", creds, err)
	}
	if conf, err := svc.decodeRcloneConf(settings); err != nil || !strings.Contains(conf, "type = sftp") {
		t.Fatalf("the rclone config reads %q, %v", conf, err)
	}
	if n, err := svc.NotifyConfig(); err != nil || n.WebhookURL != "https://hooks.example/abc" {
		t.Fatalf("the notification settings read %+v, %v", n, err)
	}
	g, err := st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	if sec, err := svc.groupSecret(g); err != nil || string(sec) != strings.Repeat("g", 16) {
		t.Fatalf("the group secret reads %q, %v", sec, err)
	}
	if key, err := svc.backupKey(); err != nil || key != oldKey || svc.currentBackupKey().state != backupKeyAdopted {
		t.Fatalf("the backup key after the restore is the old one: %v, %v", key == oldKey, err)
	}
	if settings.InstanceName != "tower" {
		t.Fatalf("the settings are not the old install's: instance name %q", settings.InstanceName)
	}
	if !secret.VerifyPassword(own, "the new password", settings.AuthPasswordHash) {
		t.Fatal("the login is not this server's own")
	}
	if got := f.keyRuns(oldKey); got[len(got)-1] != "keysettings success" {
		t.Fatalf("the activity log ends with %v", got)
	}
}

func TestNothingSealedUnderTheOldKeyIsLeftInTheRestoredSettings(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	f := newPlacementFixture(t)
	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	staged := f.stage(settingsOfTheOldInstall(t, oldKey))
	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatal(err)
	}

	db, err := store.OpenStaged(staged)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test database
	var seen, underOwn int
	if err := store.ResealDatabase(db, func(_ store.SealedColumn, sealed []byte) []byte {
		seen++
		if _, err := secret.Decrypt(oldKey, sealed); err == nil {
			t.Errorf("a value still opens under the old key")
		}
		if _, err := secret.Decrypt(f.svc.cfg.AppKey, sealed); err == nil {
			underOwn++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Cloud sign-in, rclone config, notifications, group secret and the
	// adopted backup key.
	if seen != 5 || underOwn != 5 {
		t.Fatalf("%d sealed values, %d of them under this server's key; want 5 and 5", seen, underOwn)
	}
}

func TestSettingsOfAnotherKeyAreRefusedOutsideAnEmergency(t *testing.T) {
	f := newPlacementFixture(t)
	f.stage(settingsOfTheOldInstall(t, oldKey))
	if err := f.svc.takeOverStagedSettings(context.Background()); !errors.Is(err, errSettingsOtherKey) {
		t.Fatalf("settings of another key without an emergency: %v, want %v", err, errSettingsOtherKey)
	}

	// The server's own settings go through as they are, login included.
	own := newPlacementFixture(t)
	staged := own.stage(settingsOfTheOldInstall(t, own.svc.cfg.AppKey))
	if err := own.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatalf("the server's own settings: %v", err)
	}
	_, st, _ := fileInstall(t, staged, own.svc.cfg.AppKey)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !secret.VerifyPassword(own.svc.cfg.AppKey, "the old password", settings.AuthPasswordHash) {
		t.Fatal("restoring the server's own settings replaced their login")
	}
	if _, found, _ := st.AdoptedKey(); found {
		t.Fatal("restoring the server's own settings stored an adopted key")
	}
}

func TestOwnSettingsFromBeforeAnAdoptionKeepTheAdoptedKey(t *testing.T) {
	f := newPlacementFixture(t)
	own := f.svc.cfg.AppKey
	f.adopt(oldKey, own)
	staged := f.stage(settingsOfTheOldInstall(t, own))

	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatal(err)
	}
	svc, _, _ := fileInstall(t, staged, own)
	if key, err := svc.backupKey(); err != nil || key != oldKey {
		t.Fatalf("the restored settings lost the adopted key: %v", err)
	}
}

// resticSnapshots lists a repository with the plain restic CLI and the
// password a recovery kit names.
func resticSnapshots(t *testing.T, repo, password string) []restic.Snapshot {
	t.Helper()
	cmd := exec.Command("restic", "-r", repo, "snapshots", "--json") //nolint:gosec // test command over a temp dir
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+password)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("restic snapshots: %v", err)
	}
	var snaps []restic.Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		t.Fatal(err)
	}
	return snaps
}

func TestBackupsMadeUnderAnotherKeyAreReadAndWrittenAfterTheAdoption(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("restic is not on PATH")
	}
	ctx := context.Background()
	engine := restic.Restic{Bin: "restic"}
	share := t.TempDir()
	repo := filepath.Join(share, "backups", "files")
	source := filepath.Join(t.TempDir(), "docs")
	if err := os.MkdirAll(source, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "letter.txt"), []byte("dear tower"), 0o600); err != nil {
		t.Fatal(err)
	}
	install := func(appKey string) (*Service, store.Settings) {
		svc, st, _ := fileInstall(t, filepath.Join(t.TempDir(), "bombvault.sqlite"), appKey)
		svc.cfg.HostMountRoot = filepath.ToSlash(share)
		svc.engine = engine
		settings, err := st.MutateSettings(func(s *store.Settings) error {
			s.EncryptionEnabled, s.FilesEnabled, s.FilesPath = true, true, "backups/files"
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return svc, settings
	}

	old, settings := install(oldKey)
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := old.EnsureRepo(ctx, repo, old.ModeFor(settings)); err != nil {
		t.Fatalf("create the old install's repository: %v", err)
	}
	if _, err := engine.Backup(ctx, repo, []string{source}, []string{"fileset:docs"}, old.ModeFor(settings)); err != nil {
		t.Fatalf("the old install's backup: %v", err)
	}
	kit, err := old.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}

	fresh, _ := install(strings.Repeat("b", 64))
	if err := fresh.ReadEmergency(ctx, emergencyRead{Key: strings.Repeat("e", 64), Places: []string{"backups/files"}}); err != nil {
		t.Fatal(err)
	}
	view, err := fresh.EmergencyState()
	if err != nil || view.Session.Places[0].State != placeWrongKey || view.Session.Refusal != "adopt-nothing-readable" {
		t.Fatalf("a wrong key on a real repository: %+v, %v", view.Session, err)
	}

	if err := fresh.ReadEmergency(ctx, emergencyRead{Kit: kit}); err != nil {
		t.Fatal(err)
	}
	if err := fresh.AdoptEmergencyKey(); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	now, err := fresh.store.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	mode := fresh.ModeFor(now)
	snaps, err := engine.Snapshots(ctx, repo, mode)
	if err != nil || len(snaps) != 1 {
		t.Fatalf("the adopted repository lists %d backups, %v", len(snaps), err)
	}
	entries, err := engine.Ls(ctx, repo, snaps[0].ID, mode)
	if err != nil {
		t.Fatalf("list the backup in the adopted repository: %v", err)
	}
	letter := ""
	for _, e := range entries {
		if strings.HasSuffix(e.Path, "/letter.txt") {
			letter = e.Path
		}
	}
	target := t.TempDir()
	if err := engine.RestoreInclude(ctx, repo, snaps[0].ID, letter, target, mode); err != nil {
		t.Fatalf("restore from the adopted repository: %v", err)
	}
	var restored []byte
	_ = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.Name() == "letter.txt" {
			restored, _ = os.ReadFile(path) //nolint:gosec // test file
		}
		return nil
	})
	if string(restored) != "dear tower" {
		t.Fatalf("the restored file reads %q", restored)
	}
	if err := fresh.EnsureRepo(ctx, repo, mode); err != nil {
		t.Fatalf("the adopted repository before a backup: %v", err)
	}
	if _, err := engine.Backup(ctx, repo, []string{source}, []string{"fileset:docs"}, mode); err != nil {
		t.Fatalf("a backup into the adopted repository: %v", err)
	}

	freshKit, err := fresh.RecoveryKit()
	if err != nil {
		t.Fatal(err)
	}
	read, err := readRecoveryKit(freshKit)
	if err != nil || read.appKey != oldKey || read.sealingKey != fresh.cfg.AppKey {
		t.Fatalf("the new server's kit: %v", err)
	}
	if got := resticSnapshots(t, repo, restickey.Derive(read.appKey)); len(got) != 2 {
		t.Fatalf("plain restic with the password of the new kit lists %d backups, want both", len(got))
	}
}

func TestARestoreNamesTheSealedValuesItCouldNotBringOver(t *testing.T) {
	kit := kitOfTheOldInstall(t)
	old := settingsOfTheOldInstall(t, oldKey)
	// Two values sealed under a key nobody has any more.
	stranger := strings.Repeat("9", 64)
	db, err := store.OpenStaged(old)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := secret.Encrypt(stranger, []byte("lost"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE settings SET registry_auths = '` + base64.StdEncoding.EncodeToString(sealed) + `'`,
		`INSERT INTO received_repos (id, name, repo, created_at, restic_password_enc) VALUES ('rr1', 'attic', '/host/user/attic', 1, x'` + hex.EncodeToString(sealed) + `')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := store.CloseStaged(db); err != nil {
		t.Fatal(err)
	}

	f := newPlacementFixture(t)
	f.emergency("POST", "/api/emergency/read", map[string]any{"kit": kit})
	f.emergency("POST", "/api/emergency/adopt", nil)
	staged := f.stage(old)
	if err := f.svc.takeOverStagedSettings(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := sealedLoss{Count: 2, Kinds: map[string]int{sealedRegistry: 1, sealedPeers: 1}}
	if got := f.svc.lastTakeoverLoss(); got == nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("the restore reports %+v, want %+v", got, want)
	}

	// After the restart the restored database says the same until somebody
	// has seen it.
	svc, st, _ := fileInstall(t, staged, f.svc.cfg.AppKey)
	view, err := svc.EmergencyState()
	if err != nil || view.Lost == nil || !reflect.DeepEqual(*view.Lost, want) {
		t.Fatalf("the state after the restore has lost = %+v, %v; want %+v", view.Lost, err, want)
	}
	if err := st.ClearAdoptedLoss(); err != nil {
		t.Fatal(err)
	}
	if view, err := svc.EmergencyState(); err != nil || view.Lost != nil || view.BackupKey != backupKeyAdopted {
		t.Fatalf("after the list was seen: %+v, %v", view, err)
	}
}
