package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// firstBackupRig is a service whose Containers and Folders domains have their
// own repositories on disk, with nginx installed and its appdata present.
type firstBackupRig struct {
	svc  *api.Service
	st   *store.Repo
	eng  *fakeResticEngine
	dock *fakeServiceDocker
	root string
}

func newFirstBackupRig(t *testing.T) firstBackupRig {
	t.Helper()
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.FilesPath = "backups/files"
	s.RetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	markRepo(t, root+"/backups/containers")
	markRepo(t, root+"/backups/files")
	for _, src := range []string{"appdata/nginx", "docs"} {
		if err := os.MkdirAll(root+"/"+src, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	rig := firstBackupRig{
		st:  st,
		eng: &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{}, snapsErrFor: map[string]error{}},
		dock: &fakeServiceDocker{inspect: model.Inspect{
			Name:   "/nginx",
			Image:  "nginx:latest",
			Mounts: []model.Mount{{Type: "bind", Source: root + "/appdata/nginx", Destination: "/config"}},
		}},
		root: root,
	}
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	rig.svc = api.NewService(cfg, st, rig.dock, fakeVirsh{}, rig.eng)
	return rig
}

// markRepo leaves a local repository that restic would open.
func markRepo(t *testing.T, loc string) {
	t.Helper()
	if err := os.MkdirAll(loc, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(loc+"/config", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (r firstBackupRig) named(t *testing.T, name, loc string) store.OffsiteTarget {
	t.Helper()
	if !restic.IsRemoteRepo(loc) {
		markRepo(t, r.root+"/"+loc)
	}
	row, err := r.st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: name, Repo: loc, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func (r firstBackupRig) setDefault(t *testing.T, domain, home string) {
	t.Helper()
	if _, err := r.st.PutPlacementDefault(domain, home, nil); err != nil {
		t.Fatal(err)
	}
}

func (r firstBackupRig) home(t *testing.T, domain, key string) store.HomeState {
	t.Helper()
	h, err := r.st.ItemHome(store.ItemRef{Domain: domain, Key: key})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestAnOpenContainerTakesTheDefaultAtItsFirstBackup(t *testing.T) {
	rig := newFirstBackupRig(t)
	nas := rig.named(t, "NAS", "nas")
	rig.setDefault(t, "containers", nas.ID)
	if _, err := rig.svc.Backup(context.Background(), "nginx"); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if !hasRepo(rig.eng.backedUp, rig.root+"/nas") {
		t.Fatalf("backed up to %v, want NAS", rig.eng.backedUp)
	}
	if got := rig.home(t, "containers", "nginx"); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", got)
	}
}

func TestHistoryOnTheDomainPathKeepsAnOpenContainerThere(t *testing.T) {
	rig := newFirstBackupRig(t)
	nas := rig.named(t, "NAS", "nas")
	rig.setDefault(t, "containers", nas.ID)
	rig.eng.snapsByRepo[rig.root+"/backups/containers"] = []restic.Snapshot{{ID: "1111aaaa", Tags: []string{"container:nginx"}}}
	if _, err := rig.svc.Backup(context.Background(), "nginx"); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if !hasRepo(rig.eng.backedUp, rig.root+"/backups/containers") {
		t.Fatalf("backed up to %v, want the domain path", rig.eng.backedUp)
	}
	if got := rig.home(t, "containers", "nginx"); got.Repo != "" || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want the domain path chosen", got)
	}
}

func TestAnUnreadableDomainPathStopsTheFirstBackup(t *testing.T) {
	rig := newFirstBackupRig(t)
	nas := rig.named(t, "NAS", "nas")
	rig.setDefault(t, "containers", nas.ID)
	rig.eng.snapsErrFor[rig.root+"/backups/containers"] = errWrongKey
	if _, err := rig.svc.Backup(context.Background(), "nginx"); err == nil {
		t.Fatal("the first backup ran although the domain path could not be read")
	}
	if len(rig.eng.backedUp) != 0 {
		t.Fatalf("backed up to %v", rig.eng.backedUp)
	}
	if got := rig.home(t, "containers", "nginx"); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it still open", got)
	}
}

func TestASkippedBackupLeavesTheContainerOpen(t *testing.T) {
	rig := newFirstBackupRig(t)
	nas := rig.named(t, "NAS", "nas")
	rig.setDefault(t, "containers", nas.ID)
	if _, err := rig.st.UpsertTarget(store.Target{ContainerName: "nginx"}); err != nil {
		t.Fatal(err)
	}
	rig.dock.inspectErr = errors.New("Error response from daemon: No such container: nginx")
	if _, err := rig.svc.Backup(context.Background(), "nginx"); !errors.Is(err, backup.ErrContainerNotInstalled) {
		t.Fatalf("Backup = %v, want ErrContainerNotInstalled", err)
	}
	if got := rig.home(t, "containers", "nginx"); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it still open", got)
	}
}

func TestAFailedEnsureRepoLeavesTheContainerOpen(t *testing.T) {
	rig := newFirstBackupRig(t)
	nas, err := rig.st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "nas", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rig.setDefault(t, "containers", nas.ID)
	rig.eng.initErr = errors.New("permission denied")
	if _, err := rig.svc.Backup(context.Background(), "nginx"); err == nil {
		t.Fatal("Backup went on without a repository at NAS")
	}
	if got := rig.home(t, "containers", "nginx"); got.Choice != store.RepoOpen {
		t.Fatalf("home = %+v, want it still open", got)
	}
}

func TestADefaultOnASwitchedOffRepositoryFailsWithoutFallback(t *testing.T) {
	rig := newFirstBackupRig(t)
	nas := rig.named(t, "NAS", "nas")
	nas.Enabled = false
	if _, err := rig.st.UpsertOffsiteTarget(nas); err != nil {
		t.Fatal(err)
	}
	rig.setDefault(t, "containers", nas.ID)
	_, err := rig.svc.Backup(context.Background(), "nginx")
	if err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("Backup = %v, want a refusal that names the default", err)
	}
	if len(rig.eng.backedUp) != 0 {
		t.Fatalf("fell back to %v", rig.eng.backedUp)
	}
}

func TestAFirstBackupNeverReadsUnrelatedRepositories(t *testing.T) {
	rig := newFirstBackupRig(t)
	archive := rig.root + "/archive"
	rig.named(t, "Archive", "archive")
	box := rig.named(t, "Storagebox", "sftp:u1@box.example:/bv")
	if _, err := rig.st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS unmounted", Repo: "nas2", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	rig.eng.snapsByRepo[archive] = []restic.Snapshot{{ID: "cccc0001", Tags: []string{"container:nginx"}}}
	rig.eng.snapsErrFor[box.Repo] = errors.New("connection refused")
	rig.eng.snapsErrFor[rig.root+"/nas2"] = errors.New("share not mounted")
	if _, err := rig.svc.Backup(context.Background(), "nginx"); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	for _, repo := range []string{archive, box.Repo, rig.root + "/nas2"} {
		if hasRepo(rig.eng.listedRepos, repo) {
			t.Errorf("the first backup listed %s, which nothing points at", repo)
		}
		if hasRepo(rig.eng.prunedRepos, repo) {
			t.Errorf("the first backup trimmed %s", repo)
		}
	}
}

func TestAnOpenFileSetTakesTheDefaultAtItsFirstBackup(t *testing.T) {
	rig := newFirstBackupRig(t)
	nas := rig.named(t, "NAS", "nas")
	rig.setDefault(t, "files", nas.ID)
	set, err := rig.st.CreateFileSet(store.FileSet{Name: "docs", Path: "docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rig.svc.BackupFileSet(context.Background(), set.ID); err != nil {
		t.Fatalf("BackupFileSet: %v", err)
	}
	if !hasRepo(rig.eng.backedUp, rig.root+"/nas") {
		t.Fatalf("backed up to %v, want NAS", rig.eng.backedUp)
	}
	if got := rig.home(t, "files", set.ID); got.Repo != nas.ID || got.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", got)
	}
}

func TestAnOpenVMTakesTheDefaultAtItsFirstBackup(t *testing.T) {
	svc, eng, st, root := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})
	nas, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "NAS", Repo: "nas", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutPlacementDefault("vms", nas.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	if !hasRepo(eng.backedUp, filepath.Join(root, "nas")) {
		t.Fatalf("backed up to %v, want NAS", eng.backedUp)
	}
	h, err := st.ItemHome(store.ItemRef{Domain: "vms", Key: "plainvm"})
	if err != nil {
		t.Fatal(err)
	}
	if h.Repo != nas.ID || h.Choice != store.RepoChosen {
		t.Fatalf("home = %+v, want NAS chosen", h)
	}
}
