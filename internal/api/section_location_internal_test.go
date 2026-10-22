package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const zfsNoLocationMessage = "ZFS datasets has no storage location yet. Open Settings, Storage locations and switch on a line for ZFS datasets."

// upgradedWithoutLocations leaves the store as an install that was upgraded
// with its sections off and no folder for them.
func upgradedWithoutLocations(t *testing.T, st *store.Repo) {
	t.Helper()
	if _, err := st.MutateSettings(func(cur *store.Settings) error {
		cur.FlashEnabled, cur.ConfigEnabled, cur.FilesEnabled, cur.ZFSEnabled = false, false, false, false
		cur.FlashPath, cur.ConfigPath, cur.FilesPath, cur.ZFSPath = "", "", "", ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSwitchingASectionOnGivesItTheFolderAFreshInstallHas(t *testing.T) {
	for _, tc := range []struct{ domain, switchKey, pathKey string }{
		{"flash", "flashEnabled", "flashPath"},
		{"config", "configEnabled", "configPath"},
		{"files", "filesEnabled", "filesPath"},
		{"zfs", "zfsEnabled", "zfsPath"},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			f := newPlacementFixture(t)
			upgradedWithoutLocations(t, f.st)

			_, got := f.call(http.MethodGet, "/api/settings", nil)
			view, _ := got["settings"].(map[string]any)
			view[tc.switchKey] = true
			if code, out := f.call(http.MethodPut, "/api/settings", view); code != http.StatusOK || out["ok"] != true {
				t.Fatalf("switching %s on = %d %v", tc.domain, code, out)
			}

			_, got = f.call(http.MethodGet, "/api/settings", nil)
			view, _ = got["settings"].(map[string]any)
			if want := store.DefaultSectionPath(tc.domain); view[tc.pathKey] != want {
				t.Fatalf("%s = %v after the switch, want %q", tc.pathKey, view[tc.pathKey], want)
			}
		})
	}
}

func TestASectionThatStaysOffKeepsItsEmptyLocation(t *testing.T) {
	f := newPlacementFixture(t)
	upgradedWithoutLocations(t, f.st)

	_, got := f.call(http.MethodGet, "/api/settings", nil)
	view, _ := got["settings"].(map[string]any)
	view["flashEnabled"] = true
	if code, out := f.call(http.MethodPut, "/api/settings", view); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("PUT = %d %v", code, out)
	}

	s, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ZFSPath != "" || s.FilesPath != "" {
		t.Fatalf("sections that stayed off got folders: zfs %q, files %q", s.ZFSPath, s.FilesPath)
	}
}

func TestStartupGivesSwitchedOnSectionsWithoutALocationTheirFolder(t *testing.T) {
	f := newPlacementFixture(t)
	upgradedWithoutLocations(t, f.st)
	if _, err := f.st.MutateSettings(func(cur *store.Settings) error {
		cur.ZFSEnabled, cur.FilesEnabled = true, true
		cur.FilesPath = "remotes/nas/files"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	filled, err := f.st.RepairSectionPaths()
	if err != nil {
		t.Fatal(err)
	}

	s, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ZFSPath != store.DefaultSectionPath("zfs") || len(filled) != 1 || filled[0] != "zfs" {
		t.Fatalf("zfs path %q, filled %v, want only zfs filled with its default", s.ZFSPath, filled)
	}
	if s.FilesPath != "remotes/nas/files" || s.FlashPath != "" {
		t.Fatalf("files %q, flash %q: a set location or a section that is off was touched", s.FilesPath, s.FlashPath)
	}
}

func TestAZFSBackupWithoutALocationIsRefusedBeforeRestic(t *testing.T) {
	s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	if _, err := st.MutateSettings(func(cur *store.Settings) error {
		cur.ZFSPath = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	d := zfsSeedItem(t, st, zfsRoot)

	_, err := s.BackupZFSDataset(context.Background(), d.ID)

	if !errors.Is(err, ErrNoLocation) || err.Error() != zfsNoLocationMessage {
		t.Fatalf("backup error = %v, want the no-location refusal %q", err, zfsNoLocationMessage)
	}
	if len(eng.backupRepos) != 0 {
		t.Fatalf("restic was reached with %v", eng.backupRepos)
	}
}

func TestAnOffsiteCopyOfASectionWithoutALocationIsRefusedWithTheSameSentence(t *testing.T) {
	s, st, _, eng := zfsRunFixture(t, zfsTwoDatasetTree())
	if _, err := st.MutateSettings(func(cur *store.Settings) error {
		cur.ZFSPath = ""
		cur.ZFSOffsite = "b2:bucket/offsite"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	err := s.ReplicateOffsite(context.Background(), "zfs")

	if !errors.Is(err, ErrNoLocation) {
		t.Fatalf("replication error = %v, want the no-location refusal", err)
	}
	if got := truncateRunErr(err); got != zfsNoLocationMessage {
		t.Fatalf("recorded as %q, want %q", got, zfsNoLocationMessage)
	}
	if len(eng.copies) != 0 {
		t.Fatalf("restic copied %v", eng.copies)
	}
}

func TestAPressedBackupOfASectionWithoutALocationCarriesACodeThePageCanTranslate(t *testing.T) {
	f := newPlacementFixture(t)
	f.svc.cfg.FlashDir = t.TempDir()
	if _, err := f.st.MutateSettings(func(cur *store.Settings) error {
		cur.FlashEnabled, cur.FlashPath = true, ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	code, out := f.call(http.MethodPost, "/api/flash/backup", nil)

	if code != http.StatusConflict || out["code"] != codeNoLocation {
		t.Fatalf("POST /api/flash/backup = %d %v, want 409 with code %q", code, out, codeNoLocation)
	}
}

func TestAScheduledFlashBackupWithoutALocationNeverReachesRestic(t *testing.T) {
	f := newPlacementFixture(t)
	f.svc.cfg.FlashDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(f.svc.cfg.FlashDir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.MutateSettings(func(cur *store.Settings) error {
		cur.FlashEnabled, cur.FlashPath = true, ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	c := countCalls(f)

	_, err := f.svc.BackupFlash(context.Background())

	if !errors.Is(err, ErrNoLocation) {
		t.Fatalf("flash backup error = %v, want the no-location refusal", err)
	}
	if got := c.at(""); len(got) != 0 {
		t.Fatalf("restic calls at the empty location: %v", got)
	}
}

func TestASkippedLocationKeepsTheMountFolderItNamesInTheRecordedError(t *testing.T) {
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user", HostSourceRoot: "/mnt"}}
	_, rerr := s.resolveRepo("../outside")
	skipped := []repoSkip{{Name: "the zfs repository", Reason: scrubError(rerr), Unreachable: true}}

	got := truncateRunErr(nothingCoveredError(skipped))

	if strings.Contains(got, "[path]") || !strings.Contains(got, "(/mnt)") {
		t.Fatalf("recorded error %q lost the folder it names", got)
	}
}

func TestNotificationsCarryNoDashes(t *testing.T) {
	const enDash, emDash = rune(0x2013), rune(0x2014)
	ssh := &fakeHostSSH{}
	s := unraidNotifyService(t, ssh)
	s.SetPlatform(platform.Unraid{})
	if err := s.SetNotifyConfig(notify.Config{On: "failure", Unraid: true}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	s.notifyReplicationFailed(ctx, "zfs", "boom")
	s.notifyRetentionFailed(ctx, "zfs", "zfs:tank/apps", "boom")
	s.notifyOverBudget(ctx, "zfs", 100, 50, "off-site")
	s.notifyDrillFailure(ctx, "zfs", "local", "boom")

	if len(ssh.runs) != 4 {
		t.Fatalf("sent %d notifications, want 4", len(ssh.runs))
	}
	for _, run := range ssh.runs {
		text := strings.Join(run, " ")
		if strings.ContainsRune(text, enDash) || strings.ContainsRune(text, emDash) {
			t.Errorf("notification %q has a dash", text)
		}
	}
}
