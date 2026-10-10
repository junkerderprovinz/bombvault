package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// tarFiles reads a gzip-compressed tar into its files by name.
func tarFiles(t *testing.T, gz []byte) map[string]string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	files := map[string]string{}
	for tr := tar.NewReader(zr); ; {
		hdr, err := tr.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		files[strings.TrimPrefix(hdr.Name, "/")] = string(body)
	}
}

// exported reads a file of the fixture's export folder.
func (f *resticFixture) exported(folder, name string) []byte {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.FromSlash(f.root), filepath.FromSlash(folder), name)) //nolint:gosec // G304: a file below the test's own temp folder
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}

// wait blocks until the export, backup or restore a request started is over.
func (f *resticFixture) wait() {
	f.t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for f.svc.batchActive.Load() {
		if time.Now().After(deadline) {
			f.t.Fatal("the run never finished")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAStoredBackupIsExportedAsAnArchive(t *testing.T) {
	f := newResticFixture(t)
	plex, err := f.st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	f.write("plex", map[string]string{"config.yml": "port: 32400", "db/library.db": "movies"})
	snap := f.backup("containers", "plex", "container:plex", "imported")
	body := map[string]any{"form": "tar", "snapshotId": snap.ID, "folder": "exports"}
	name := "plex-" + parseSnapshotTime(snap.Time).UTC().Format("20060102-150405") + ".tar.gz"

	res := f.do(http.MethodPost, "/api/items/containers/plex/export/preview", body)
	preview, _ := res["export"].(map[string]any)
	if res["ok"] != true || preview["folder"] != "/mnt/exports" || preview["encryption"] != "none" || preview["key"] != "export:containers" {
		t.Fatalf("preview = %v", res)
	}
	if files := preview["files"].([]any); len(files) != 1 || files[0] != name || len(preview["replaces"].([]any)) != 0 {
		t.Fatalf("preview names %v and replaces %v, want %s and nothing", preview["files"], preview["replaces"], name)
	}
	if preview["bytes"] != float64(len("port: 32400")+len("movies")) {
		t.Errorf("bytes = %v, want the size of the two files", preview["bytes"])
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(f.root), "exports")); err == nil {
		t.Fatal("the preview created the export folder")
	}

	if res := f.do(http.MethodPost, "/api/items/containers/plex/export", body); res["started"] != true {
		t.Fatalf("export = %v", res)
	}
	f.wait()
	got := tarFiles(t, f.exported("exports", name))
	if got["config.yml"] != "port: 32400" || got["db/library.db"] != "movies" {
		t.Fatalf("the archive holds %v", got)
	}
	runs, err := f.st.ListRunsFiltered(store.RunFilter{TargetIDs: []string{plex.ID}, Kinds: []string{"export"}, Limit: 5})
	if err != nil || len(runs) != 1 || runs[0].Status != "success" || runs[0].Bytes == 0 {
		t.Fatalf("runs = %+v, %v, want one finished export with its size", runs, err)
	}
	res = f.do(http.MethodPost, "/api/items/containers/plex/export/preview", body)
	if replaces := res["export"].(map[string]any)["replaces"].([]any); len(replaces) != 1 || replaces[0] != name {
		t.Fatalf("a second preview replaces %v, want the file of the first export", replaces)
	}

	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := f.st.GetSettings()
	settings.ExportEncryptEnabled, settings.ExportAgeRecipients = true, id.Recipient().String()
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	plan, err := f.svc.planExport(ctx, "containers", "plex", "local", ExportRequest{Form: "zip", SnapshotID: snap.ID, Folder: "exports"})
	if err != nil {
		t.Fatal(err)
	}
	sealed := strings.TrimSuffix(name, ".tar.gz") + ".zip.age"
	if plan.Encryption != "age" || !slices.Equal(plan.Files, []string{sealed}) {
		t.Fatalf("plan = %+v, want one sealed zip", plan.ExportPreview)
	}
	if _, err := f.svc.runExport(ctx, plan, 0); err != nil {
		t.Fatal(err)
	}
	plain := ageRoundTripDecrypt(t, f.exported("exports", sealed), id)
	zr, err := zip.NewReader(bytes.NewReader(plain), int64(len(plain)))
	if err != nil {
		t.Fatalf("the sealed export is no zip: %v", err)
	}
	var names []string
	for _, entry := range zr.File {
		names = append(names, strings.TrimPrefix(entry.Name, "/"))
	}
	if !slices.Contains(names, "config.yml") || !slices.Contains(names, "db/library.db") {
		t.Fatalf("the zip holds %v", names)
	}
}

func TestARepositoryExportCopiesOnlyTheEntrysBackups(t *testing.T) {
	f := newResticFixture(t)
	f.write("plex", map[string]string{"config.yml": "port: 32400"})
	first := f.backup("containers", "plex", "container:plex")
	f.write("plex", map[string]string{"config.yml": "port: 32401"})
	second := f.backup("containers", "plex", "container:plex")
	f.backup("containers", "plex", "container:sonarr")
	ctx := context.Background()

	plan, err := f.svc.planExport(ctx, "containers", "plex", "local", ExportRequest{Form: "repository", Folder: "exports"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Backups != 2 || plan.Encryption != "password" || !slices.Equal(plan.Files, []string{"plex-restic/"}) {
		t.Fatalf("plan = %+v", plan.ExportPreview)
	}
	if _, err := f.svc.runExport(ctx, plan, 0); err != nil {
		t.Fatal(err)
	}
	// The copy opens with the password of the repository it came from.
	_, mode := f.repoOf("containers")
	dest := filepath.Join(filepath.FromSlash(f.root), "exports", "plex-restic")
	copies, err := f.svc.engine.Snapshots(ctx, dest, restic.Mode{Encrypted: true, Password: mode.Password})
	if err != nil {
		t.Fatalf("the copy does not open with this instance's password: %v", err)
	}
	var origins []string
	for _, c := range copies {
		origins = append(origins, c.Original)
	}
	slices.Sort(origins)
	want := []string{first.ID, second.ID}
	slices.Sort(want)
	if !slices.Equal(origins, want) {
		t.Fatalf("the copy holds %v, want the two backups of plex", origins)
	}
	if _, err := f.svc.planExport(ctx, "containers", "plex", "local", ExportRequest{Form: "repository", Folder: "exports"}); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("a second copy into the same folder = %v, want a refusal", err)
	}
}

func TestAnArchiveOfFoldersBelowTheMountStartsAtTheMount(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("restic spells a Windows path differently inside a snapshot")
	}
	f := newResticFixture(t)
	set, err := f.st.CreateFileSet(store.FileSet{Name: "My Docs", Path: "files/docs", Enabled: true, RepoChosen: store.RepoChosen})
	if err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(f.root, "files", "docs")
	if err := os.MkdirAll(docs, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "a.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo, mode := f.repoOf("files")
	if err := f.svc.engine.Init(ctx, repo, mode); err != nil {
		t.Fatal(err)
	}
	sum, err := f.svc.engine.Backup(ctx, repo, []string{docs}, []string{"fileset:My Docs"}, mode)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := f.svc.planExport(ctx, "files", set.ID, "local", ExportRequest{Form: "tar", SnapshotID: sum.SnapshotID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plan.Files[0], "My-Docs-") || plan.Folder != "/mnt/backups/export" {
		t.Fatalf("plan = %+v, want a file named after the set in the export folder beside the repositories", plan.ExportPreview)
	}
	if _, err := f.svc.runExport(ctx, plan, 0); err != nil {
		t.Fatal(err)
	}
	if got := tarFiles(t, f.exported("backups/export", plan.Files[0])); got["files/docs/a.txt"] != "one" {
		t.Fatalf("the archive holds %v, want the file under its path below the mount", got)
	}
}

func TestAnExportNamesOneFilePerDatasetOrDisk(t *testing.T) {
	f := newPlacementFixture(t)
	tank, zfsRepo := f.zfsItem("tank/apps")
	f.hold(zfsRepo,
		zfsSnap("a1a1a1a1", "tank/apps", firstRun),
		zfsSnap("a2a2a2a2", "tank/apps/db", firstRun),
		zfsSnap("b1b1b1b1", "tank/apps", secondRun),
	)
	disk := snap("d2d2d2d2", 1_767_225_600, "vm:win11:zvol:sda", "p2", "vmrun:r1")
	disk.Paths = []string{"/vm-disks/tank/vms/win11-sda@bombvault-1"}
	f.hold(f.domainPath("vms"),
		snap("d1d1d1d1", 1_767_225_600, "vm:win11", "p2", "blocks", "vmrun:r1"), disk,
		snap("e1e1e1e1", 1_767_312_000, "vm:win11", "p2"),
	)
	ctx := context.Background()

	plan, err := f.svc.planExport(ctx, zfsDomain, tank.ID, "local", ExportRequest{Form: "tar", SnapshotID: "a2a2a2a2"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"tank-apps-20260101-000000.tar.gz", "tank-apps-db-20260101-000000.tar.gz"}; !slices.Equal(plan.Files, want) {
		t.Errorf("a ZFS run names %v, want %v", plan.Files, want)
	}
	plan, err = f.svc.planExport(ctx, "vms", "win11", "local", ExportRequest{Form: "zip", SnapshotID: "d1d1d1d1"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"win11-20260101-000000.zip", "win11-20260101-000000-sda.zfs.gz"}; !slices.Equal(plan.Files, want) || !plan.Segments {
		t.Errorf("a VM run names %v with segments %v, want %v and the segment mark", plan.Files, plan.Segments, want)
	}
	if plan.parts[1].stream != disk.Paths[0] {
		t.Errorf("the disk is read from %q, want its stream", plan.parts[1].stream)
	}

	plan, err = f.svc.planExport(ctx, "vms", "win11", "local", ExportRequest{Form: "repository", SnapshotID: "d1d1d1d1"})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(plan.ids)
	if !slices.Equal(plan.ids, []string{"d1d1d1d1", "d2d2d2d2"}) || plan.Backups != 1 {
		t.Errorf("one VM run copies %v as %d backups, want the run with its disk as one", plan.ids, plan.Backups)
	}
	plan, err = f.svc.planExport(ctx, "vms", "win11", "local", ExportRequest{Form: "repository"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ids) != 3 || plan.Backups != 2 || plan.Encryption != "password" {
		t.Errorf("every backup of the VM = %v as %d backups, encryption %q", plan.ids, plan.Backups, plan.Encryption)
	}
	if plan, err = f.svc.planExport(ctx, zfsDomain, tank.ID, "local", ExportRequest{Form: "repository"}); err != nil || plan.Backups != 2 || len(plan.ids) != 3 {
		t.Errorf("every backup of the ZFS entry = %+v, %v, want three snapshots as two runs", plan.ExportPreview, err)
	}
}

func TestAnExportRefusesWhatItCannotWrite(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 100, "container:nginx"))
	ctx := context.Background()
	for name, req := range map[string]ExportRequest{
		"an unknown form":            {Form: "iso", SnapshotID: "a1a1a1a1"},
		"an archive of no backup":    {Form: "tar"},
		"a folder outside the mount": {Form: "tar", SnapshotID: "a1a1a1a1", Folder: "../etc"},
		"an absolute folder":         {Form: "zip", SnapshotID: "a1a1a1a1", Folder: "/etc"},
		"another entry's backup":     {Form: "tar", SnapshotID: "ffff0000"},
	} {
		if _, err := f.svc.PreviewExport(ctx, "containers", "nginx", "local", req); err == nil {
			t.Errorf("%s was planned", name)
		}
	}
	if _, err := f.svc.PreviewExport(ctx, "containers", "ghost", "local", ExportRequest{Form: "repository"}); err == nil {
		t.Error("a copy of an entry without backups was planned")
	}
}

func TestArchiveRootIsTheMountForFoldersBelowIt(t *testing.T) {
	s := &Service{cfg: config.Config{HostMountRoot: "/host/user"}}
	for _, c := range []struct {
		name string
		set  backupSet
		snap restic.Snapshot
		want string
	}{
		{"appdata below the mount", backupSet{}, restic.Snapshot{Paths: []string{"/host/user/appdata/plex", "/host/user/media/plex"}}, "/host/user"},
		{"the flash drive", backupSet{}, restic.Snapshot{Paths: []string{"/host/boot"}}, "/host/boot"},
		{"folders on both sides", backupSet{}, restic.Snapshot{Paths: []string{"/host/user/a", "/host/boot/b"}}, "/host"},
		{"a ZFS dataset", backupSet{zfsRoot: "tank"}, restic.Snapshot{Paths: []string{"/host/user/tank/.zfs/snapshot/" + firstRun}}, "/"},
		{"a changed-block VM backup", backupSet{}, restic.Snapshot{Tags: []string{"blocks"}, Paths: []string{"/config/vm-blocks/x"}}, "/"},
		{"an imported archive", backupSet{}, restic.Snapshot{Tags: []string{"imported"}, Paths: []string{"/host/user/staging"}}, "/"},
	} {
		if got := s.archiveRoot(c.set, c.snap); got != c.want {
			t.Errorf("%s starts at %q, want %q", c.name, got, c.want)
		}
	}
}
