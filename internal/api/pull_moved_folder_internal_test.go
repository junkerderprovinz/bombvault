package api

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// movedFolderRig is a real source repository with one snapshot and a source
// pulling it into the section's own folder on a real disk.
func movedFolderRig(t *testing.T) (*Service, *store.Repo, store.PullSource, string) {
	t.Helper()
	bin, err := exec.LookPath("restic")
	if err != nil {
		t.Skip("no restic")
	}
	mount := t.TempDir()
	eng := restic.Restic{Bin: bin, CacheDir: t.TempDir()}
	theirPassword := restickey.Derive(pullTheirKey)
	srcMode := restic.Mode{Encrypted: true, Password: theirPassword}
	srcRepo := filepath.Join(mount, "theirs")
	if err := eng.Init(context.Background(), srcRepo, srcMode); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "a.txt"), []byte("invented"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Backup(context.Background(), srcRepo, []string{data}, []string{"fileset:docs"}, srcMode); err != nil {
		t.Fatal(err)
	}

	svc, st, ps := pullFixture(t, eng)
	svc.cfg = config.Config{AppKey: pullOurKey, DataDir: t.TempDir(), HostMountRoot: mount}
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = "first/containers"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	enc, err := secret.Encrypt(pullOurKey, []byte(theirPassword))
	if err != nil {
		t.Fatal(err)
	}
	ps.Repo, ps.ResticPasswordEnc = "theirs", enc
	if err := st.UpdatePullSource(ps); err != nil {
		t.Fatal(err)
	}
	return svc, st, ps, mount
}

func moveContainersFolder(t *testing.T, st *store.Repo, to string) {
	t.Helper()
	if _, err := st.MutateSettings(func(s *store.Settings) error {
		s.ContainersPath = to
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMovingTheOwnFolderDoesNotCopyAPulledSourceAgain(t *testing.T) {
	svc, st, ps, mount := movedFolderRig(t)
	if n, err := svc.PullFromSource(context.Background(), ps); err != nil || n != 1 {
		t.Fatalf("first pull: %d, %v", n, err)
	}
	ps, _, _ = st.GetPullSource(ps.ID)

	moveContainersFolder(t, st, "second/containers")
	n, err := svc.PullFromSource(context.Background(), ps)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("the pull after the move carried %d snapshots, want none", n)
	}
	if _, err := os.Stat(filepath.Join(mount, "second", "containers", "pulled", ps.ID)); !os.IsNotExist(err) {
		t.Fatalf("a second copy started in the new folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mount, "first", "containers", "pulled", ps.ID, "config")); err != nil {
		t.Fatalf("the copy left its place: %v", err)
	}
}

func TestAPullFollowsTheOwnFolderOnceTheOldCopyIsGone(t *testing.T) {
	svc, st, ps, mount := movedFolderRig(t)
	if _, err := svc.PullFromSource(context.Background(), ps); err != nil {
		t.Fatal(err)
	}
	ps, _, _ = st.GetPullSource(ps.ID)
	moveContainersFolder(t, st, "second/containers")
	if err := os.RemoveAll(filepath.Join(mount, "first")); err != nil {
		t.Fatal(err)
	}

	if n, err := svc.PullFromSource(context.Background(), ps); err != nil || n != 1 {
		t.Fatalf("pull after the old copy went: %d, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(mount, "second", "containers", "pulled", ps.ID, "config")); err != nil {
		t.Fatalf("no copy in the new folder: %v", err)
	}
	got, _, _ := st.GetPullSource(ps.ID)
	if got.Landing != filepath.Join(mount, "second", "containers", "pulled", ps.ID) && filepath.ToSlash(got.Landing) != filepath.ToSlash(mount)+"/second/containers/pulled/"+ps.ID {
		t.Fatalf("landing = %q", got.Landing)
	}
}

func TestARemoteLandingSignsInWithTheFolderItLiesIn(t *testing.T) {
	eng := &openRecorder{}
	svc, st, ps := pullFixture(t, eng)
	if err := svc.SetCloudCredSets([]CloudCredSet{{ID: "old", Name: "old", CloudCreds: CloudCreds{S3KeyID: "OLDKEY", S3Secret: "s"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertPrimaryRemoteTarget("containers", store.OffsiteTarget{Domain: "containers", Repo: pullDomain, CredsRef: "old", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	moveContainersFolder(t, st, "s3:https://s3.example.com/new/containers")
	settings, _ := st.GetSettings()
	ps.Landing = pullDomain + "/pulled/" + ps.ID
	dest, mode := svc.pullLanding(context.Background(), ps, settings, "containers", settings.ContainersPath)
	if dest != ps.Landing {
		t.Fatalf("dest = %q", dest)
	}
	found := false
	for _, kv := range mode.Env {
		found = found || kv == "AWS_ACCESS_KEY_ID=OLDKEY"
	}
	if !found {
		t.Fatalf("env %v does not sign in with the folder's own sign-in", mode.Env)
	}
}
