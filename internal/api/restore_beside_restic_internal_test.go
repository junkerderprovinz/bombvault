package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const plexImage = "plexinc/pms:1.40"

// seedPlex puts a running container on the rig with an appdata folder and a
// media share, and has the service back it up. It returns the container's
// target row and the snapshot.
func (r *restoreRig) seedPlex(t *testing.T) (store.Target, string) {
	t.Helper()
	r.write(t, map[string]string{
		"appdata/plex/config/settings.json": `{"name":"plex"}`,
		"appdata/plex/library.db":           "library at backup time",
		"appdata/plex/logs/plex.log":        "started",
		"media/film.mkv":                    "a film",
	})
	if err := os.Symlink("library.db", filepath.Join(r.root, "appdata/plex/current")); err != nil {
		t.Fatal(err)
	}
	in := model.Inspect{ID: "id-plex", Name: "/plex", Running: true, Config: model.Config{Image: plexImage}}
	in.HostConfig.Binds = []string{"/mnt/user/appdata/plex:/config", "/mnt/user/media:/media"}
	in.Mounts = []model.Mount{
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config"},
		{Type: "bind", Source: "/mnt/user/media", Destination: "/media"},
	}
	in.HostConfig.PortBindings = map[string][]model.PortBinding{"32400/tcp": {{HostPort: "32400"}}}
	in.HostConfig.RestartPolicy = model.RestartPolicy{Name: "unless-stopped"}
	in.HostConfig.NetworkMode = "br0"
	in.Network = model.NetworkEndpoint{Name: "br0", IPv4Address: "192.168.20.50", MACAddress: "02:42:c0:a8:14:32"}
	r.docker.containers["plex"] = in
	r.docker.images[plexImage] = true

	if _, err := r.svc.Backup(context.Background(), "plex"); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	tg, err := r.st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	return tg, r.lastRun(t, tg.ID, "backup").SnapshotID
}

func (r *restoreRig) check(t *testing.T, req RestoreCheckRequest) RestoreCheck {
	t.Helper()
	req.Source = "local"
	res, err := r.svc.CheckRestore(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckRestore: %v", err)
	}
	return res
}

// watch collects the progress events of one key until the test ends.
func (r *restoreRig) watch(t *testing.T, key string) func() []progress.Event {
	t.Helper()
	ch, stop := r.progress.Subscribe()
	t.Cleanup(stop)
	return func() []progress.Event {
		var out []progress.Event
		for {
			select {
			case e := <-ch:
				if e.Key == key {
					out = append(out, e)
				}
			default:
				return out
			}
		}
	}
}

func TestRestoreBesideContainerLeavesTheOriginalUntouched(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedPlex(t)
	appdata := filepath.Join(r.root, "appdata", "plex")
	media := filepath.Join(r.root, "media")
	atBackup := treeSum(t, appdata, false)
	// The original moves on after the backup, so a copy that equals the
	// backup cannot have been read from it.
	r.write(t, map[string]string{"appdata/plex/library.db": "a newer library", "appdata/plex/added-later.txt": "x"})
	before, mediaBefore := treeSum(t, appdata, true), treeSum(t, media, true)
	r.docker.forget()

	res := r.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: snap, Target: targetBeside})
	if !res.Ready {
		t.Fatalf("checks = %+v", res.Checks)
	}
	wantBytes := int64(len(`{"name":"plex"}`) + len("library at backup time") + len("started"))
	if c := lineOf(t, res, lineSpace); c.Status != lineOK || c.Need != wantBytes {
		t.Fatalf("space = %+v, want %d bytes needed", c, wantBytes)
	}
	if p := res.Plan; p.InPlace || p.Added != 4 || p.Changed != 0 || p.Extra != 0 || p.Beside == nil || p.Beside.Name != "plex-restored" {
		t.Fatalf("plan = %+v", p)
	}
	if got := entries(t, filepath.Dir(appdata)); !slices.Equal(got, []string{"plex"}) {
		t.Fatalf("a check created something next to the original: %v", got)
	}

	events := r.watch(t, "container:plex")
	started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{Beside: true})
	if err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	r.wait(t)

	if got := treeSum(t, appdata, true); got != before {
		t.Fatal("the original's appdata changed")
	}
	if got := treeSum(t, media, true); got != mediaBefore {
		t.Fatal("a folder the backup holds nothing of changed")
	}
	if got := treeSum(t, appdata+besideSuffix, false); got != atBackup {
		t.Fatal("the copy is not what the backup held")
	}
	if got := entries(t, filepath.Dir(appdata)); !slices.Equal(got, []string{"plex", "plex-restored"}) {
		t.Fatalf("next to the original lies %v", got)
	}

	if calls := r.docker.did("stop", "remove", "start", "exec", "pull"); len(calls) != 0 {
		t.Fatalf("the restore asked Docker to %v", calls)
	}
	if original, _ := r.docker.container("plex"); !original.Running || original.HostConfig.Binds[0] != "/mnt/user/appdata/plex:/config" {
		t.Fatalf("the original is no longer what it was: %+v", original)
	}
	made, ok := r.docker.container("plex-restored")
	if !ok || made.Running {
		t.Fatalf("want a stopped container plex-restored, got %+v (found=%v)", made, ok)
	}
	if want := []string{"/mnt/user/appdata/plex-restored:/config", "/mnt/user/media:/media"}; !slices.Equal(made.HostConfig.Binds, want) {
		t.Fatalf("binds = %q, want %q", made.HostConfig.Binds, want)
	}
	if got := made.HostConfig.PortBindings["32400/tcp"]; len(got) != 1 || got[0].HostPort != "32401" {
		t.Fatalf("ports = %+v, want 32401, since the original holds 32400", got)
	}
	if made.HostConfig.RestartPolicy.Name != "no" || made.Network.IPv4Address != "" || made.Network.MACAddress != "" {
		t.Fatalf("the copy could start by itself or claims the original's address: %+v %+v", made.HostConfig.RestartPolicy, made.Network)
	}
	if made.Config.Labels[restoredFromLabel] != "plex" || made.Config.Labels[restoredSnapshotLabel] != snap {
		t.Fatalf("labels = %v", made.Config.Labels)
	}

	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" || run.SnapshotID != snap {
		t.Fatalf("run = %+v", run)
	}
	seen := events()
	if len(seen) < 2 || seen[0].Phase != "restore" || !seen[0].Active || seen[len(seen)-1].Active {
		t.Fatalf("progress = %+v, want a restore that began and ended", seen)
	}

	// The first copy stays what it is: a second restore next to the original
	// finds its name taken and writes nothing.
	withCopy := treeSum(t, filepath.Dir(appdata), true)
	if started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{Beside: true}); started || !isDestinationRefusal(err) {
		t.Fatalf("second restore: started=%v err=%v, want a refusal", started, err)
	}
	if got := treeSum(t, filepath.Dir(appdata), true); got != withCopy {
		t.Fatal("the refused second restore wrote into the original or the first copy")
	}
}

// noRoom makes every volume of the rig report ten free bytes, and returns the
// way back.
func (r *restoreRig) noRoom() func() {
	r.svc.diskStat = func(string) (diskStatResult, error) {
		return diskStatResult{Free: 10, Volume: "dev:1"}, nil
	}
	return func() { r.svc.diskStat = nil }
}

// inTheWay puts a file below the Host Data mount, and returns the removal of
// its folder.
func (r *restoreRig) inTheWay(t *testing.T, rel string) func() {
	t.Helper()
	r.write(t, map[string]string{rel: "mine"})
	return func() {
		if err := os.RemoveAll(filepath.Dir(filepath.Join(r.root, rel))); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRestoreBesideContainerRefusals(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedPlex(t)
	for name, tc := range map[string]struct {
		arrange func(t *testing.T) (undo func())
		want    string
	}{
		"a folder of that name is there": {
			arrange: func(t *testing.T) func() { return r.inTheWay(t, "appdata/plex-restored/keep.txt") },
			want:    "/mnt/user/appdata/plex-restored already exists",
		},
		"a container of that name is there": {
			arrange: func(*testing.T) func() {
				r.docker.containers["plex-restored"] = model.Inspect{Name: "/plex-restored"}
				return func() { delete(r.docker.containers, "plex-restored") }
			},
			want: "a container named plex-restored already exists",
		},
		"the volume has no room for a copy": {
			arrange: func(*testing.T) func() { return r.noRoom() },
			want:    "not enough free space",
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer tc.arrange(t)()
			before := treeSum(t, filepath.Join(r.root, "appdata"), true)
			containers := len(r.docker.containers)
			r.docker.forget()

			started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{Beside: true})

			if started || !isDestinationRefusal(err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("started=%v err=%v, want a refusal saying %q", started, err, tc.want)
			}
			if r.svc.BackupInProgress() {
				t.Fatal("a refused restore must not hold the single-flight guard")
			}
			if got := treeSum(t, filepath.Join(r.root, "appdata"), true); got != before {
				t.Fatal("a refused restore changed the appdata share")
			}
			if len(r.docker.did("create", "stop", "remove", "start")) != 0 || len(r.docker.containers) != containers {
				t.Fatalf("a refused restore asked Docker to %v", r.docker.calls)
			}
			if run, _ := r.st.LastRunOfKind(tg.ID, "restore"); run != nil {
				t.Fatalf("a refused restore recorded a run: %+v", run)
			}
			if res := r.check(t, RestoreCheckRequest{Kind: checkContainer, Name: "plex", SnapshotID: snap, Target: targetBeside}); res.Ready {
				t.Fatalf("the check lets a restore start that is refused: %+v", res.Checks)
			}
		})
	}
}

// stallingEngine is the real engine until a restore into a folder begins. It
// then writes one file there, reports in and waits to be cancelled.
type stallingEngine struct {
	ResticEngine
	entered chan struct{}
}

func (e *stallingEngine) RestoreSubtreeTo(ctx context.Context, _, _, _, target string, _ restic.Mode) error {
	if err := os.WriteFile(filepath.Join(target, "half-restored"), []byte("x"), 0o600); err != nil {
		return err
	}
	close(e.entered)
	<-ctx.Done()
	return fmt.Errorf("restic restore: %w", ctx.Err())
}

func (r *restoreRig) stall() chan struct{} {
	entered := make(chan struct{})
	r.svc.engine = &stallingEngine{ResticEngine: r.svc.engine, entered: entered}
	return entered
}

func TestRestoreBesideContainerCancelledLeavesNoHalfCopy(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedPlex(t)
	appdata := filepath.Join(r.root, "appdata", "plex")
	before := treeSum(t, appdata, true)
	entered := r.stall()

	if started, err := r.svc.StartRestore(context.Background(), "plex", snap, "local", RestoreOptions{Beside: true}); err != nil || !started {
		t.Fatalf("StartRestore: started=%v err=%v", started, err)
	}
	await(t, entered)
	if _, err := os.Stat(filepath.Join(appdata+besideSuffix, "half-restored")); err != nil {
		t.Fatalf("the restore is not writing into the copy's folder: %v", err)
	}
	running := slices.ContainsFunc(r.progress.Snapshot(), func(e progress.Event) bool {
		return e.Key == "container:plex" && e.Phase == "restore" && e.Active
	})
	if !running {
		t.Fatalf("no running restore under container:plex: %+v", r.progress.Snapshot())
	}
	if !r.svc.CancelRun("container:plex") {
		t.Fatal("the restore cannot be cancelled under its progress key")
	}
	r.wait(t)

	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "cancelled" {
		t.Fatalf("run = %+v, want cancelled", run)
	}
	if got := entries(t, filepath.Dir(appdata)); !slices.Equal(got, []string{"plex"}) {
		t.Fatalf("a cancelled restore left %v next to the original", got)
	}
	if got := treeSum(t, appdata, true); got != before {
		t.Fatal("the original's appdata changed")
	}
	if _, made := r.docker.container("plex-restored"); made {
		t.Fatal("a cancelled restore created the container")
	}
}

const winXML = `<domain type='kvm'>
  <name>win</name>
  <uuid>6f1c2f0e-3c57-4d3b-9a43-0c2d3b1f7a10</uuid>
  <memory unit='KiB'>1048576</memory>
  <vcpu>2</vcpu>
  <os><type arch='x86_64' machine='q35'>hvm</type></os>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='/mnt/user/domains/win/vdisk1.img'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <disk type='file' device='disk'>
      <driver name='qemu' type='raw'/>
      <source file='/mnt/user/domains/win/vdisk2.img'/>
      <target dev='vdb' bus='virtio'/>
    </disk>
  </devices>
</domain>`

// seedWin puts a shut-off VM with two disks on the rig and has the service
// back it up.
func (r *restoreRig) seedWin(t *testing.T) (store.VMTarget, string) {
	t.Helper()
	r.write(t, map[string]string{
		"domains/win/vdisk1.img": strings.Repeat("system disk ", 512),
		"domains/win/vdisk2.img": strings.Repeat("data disk ", 256),
	})
	r.virsh.name, r.virsh.state, r.virsh.xml = "win", "shut off", winXML
	if _, err := r.svc.BackupVM(context.Background(), "win"); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	tg, err := r.st.GetVMTargetByName("win")
	if err != nil {
		t.Fatal(err)
	}
	return tg, r.lastRun(t, tg.ID, "backup").SnapshotID
}

// redefine rewrites the stored definition of the VM, as a later backup would
// have recorded it.
func (r *restoreRig) redefine(t *testing.T, tg store.VMTarget, change func(*vmDefinition)) {
	t.Helper()
	var def vmDefinition
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		t.Fatal(err)
	}
	change(&def)
	raw, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	tg.Definition = string(raw)
	if _, err := r.st.UpsertVMTarget(tg); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreBesideVMLeavesTheDisksAndTheDomainUntouched(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedWin(t)
	r.redefine(t, tg, func(def *vmDefinition) {
		def.NVRAMHostPath = "/etc/libvirt/qemu/nvram/6f1c2f0e_VARS-pure-efi.fd"
		def.NVRAMBytes = []byte("uefi variables")
	})
	disks := filepath.Join(r.root, "domains", "win")
	atBackup := treeSum(t, disks, false)
	r.write(t, map[string]string{"domains/win/vdisk2.img": "the data disk moved on"})
	before := treeSum(t, disks, true)
	r.virsh.setState("running")
	r.virsh.forget()

	res := r.check(t, RestoreCheckRequest{Kind: checkVM, Name: "win", SnapshotID: snap, Target: targetBeside})
	wantBytes := int64(len("system disk ")*512 + len("data disk ")*256)
	if c := lineOf(t, res, lineSpace); !res.Ready || c.Need != wantBytes {
		t.Fatalf("ready=%v space=%+v, want %d bytes needed", res.Ready, c, wantBytes)
	}
	wantFolders := []BesideFolder{{Original: "/mnt/user/domains/win", Copy: "/mnt/user/domains/win-restored"}}
	if p := res.Plan; p.InPlace || p.Added != 3 || p.Beside == nil || p.Beside.Name != "win-restored" || !slices.Equal(p.Beside.Folders, wantFolders) || p.Beside.Container != nil {
		t.Fatalf("plan = %+v, beside %+v", p, p.Beside)
	}

	started, err := r.svc.StartRestoreVM(context.Background(), "win", snap, "local", RestoreOptions{Beside: true})
	if err != nil || !started {
		t.Fatalf("StartRestoreVM: started=%v err=%v", started, err)
	}
	r.wait(t)

	if got := treeSum(t, disks, true); got != before {
		t.Fatal("the VM's disks changed")
	}
	nvram := filepath.Join(disks+besideSuffix, "6f1c2f0e_VARS-pure-efi.fd")
	if got, err := os.ReadFile(nvram); err != nil || string(got) != "uefi variables" { //nolint:gosec // G304: a file below the test's own folder
		t.Fatalf("nvram = %q, %v", got, err)
	}
	if err := os.Remove(nvram); err != nil {
		t.Fatal(err)
	}
	if got := treeSum(t, disks+besideSuffix, false); got != atBackup {
		t.Fatal("the copied disks are not what the backup held")
	}
	if calls := r.virsh.recorded(); len(calls) != 0 {
		t.Fatalf("the restore asked libvirt to %v", calls)
	}
	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "success" || run.SnapshotID != snap {
		t.Fatalf("run = %+v", run)
	}
}

func TestRestoreBesideVMRefusals(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedWin(t)
	for name, tc := range map[string]struct {
		arrange func(t *testing.T) (undo func())
		want    string
	}{
		"a folder of that name is there": {
			arrange: func(t *testing.T) func() { return r.inTheWay(t, "domains/win-restored/vdisk1.img") },
			want:    "/mnt/user/domains/win-restored already exists",
		},
		"the volume has no room for a copy": {
			arrange: func(*testing.T) func() { return r.noRoom() },
			want:    "not enough free space",
		},
		"a disk is a ZFS volume": {
			arrange: func(t *testing.T) func() {
				r.svc.ssh = &fakeHostSSH{}
				r.redefine(t, tg, func(def *vmDefinition) {
					def.DomainXML = strings.Replace(def.DomainXML, "</devices>",
						"<disk type='block' device='disk'><source dev='/dev/zvol/tank/vms/win-data'/><target dev='vdc' bus='virtio'/></disk></devices>", 1)
				})
				return func() {
					r.svc.ssh = nil
					r.redefine(t, tg, func(*vmDefinition) {})
				}
			},
			want: "the ZFS volume tank/vms/win-data",
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer tc.arrange(t)()
			before := treeSum(t, filepath.Join(r.root, "domains"), true)
			r.virsh.forget()

			started, err := r.svc.StartRestoreVM(context.Background(), "win", snap, "local", RestoreOptions{Beside: true})

			if started || !isDestinationRefusal(err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("started=%v err=%v, want a refusal saying %q", started, err, tc.want)
			}
			if got := treeSum(t, filepath.Join(r.root, "domains"), true); got != before {
				t.Fatal("a refused restore changed the domains share")
			}
			if calls := r.virsh.recorded(); len(calls) != 0 {
				t.Fatalf("a refused restore asked libvirt to %v", calls)
			}
			if res := r.check(t, RestoreCheckRequest{Kind: checkVM, Name: "win", SnapshotID: snap, Target: targetBeside}); res.Ready {
				t.Fatalf("the check lets a restore start that is refused: %+v", res.Checks)
			}
		})
	}
}

func TestRestoreBesideVMCancelledLeavesNoHalfCopy(t *testing.T) {
	r := newRestoreRig(t)
	tg, snap := r.seedWin(t)
	disks := filepath.Join(r.root, "domains", "win")
	before := treeSum(t, disks, true)
	entered := r.stall()

	if started, err := r.svc.StartRestoreVM(context.Background(), "win", snap, "local", RestoreOptions{Beside: true}); err != nil || !started {
		t.Fatalf("StartRestoreVM: started=%v err=%v", started, err)
	}
	await(t, entered)
	if !r.svc.CancelRun("vm:win") {
		t.Fatal("the restore cannot be cancelled under its progress key")
	}
	r.wait(t)

	if run := r.lastRun(t, tg.ID, "restore"); run.Status != "cancelled" {
		t.Fatalf("run = %+v, want cancelled", run)
	}
	if got := entries(t, filepath.Dir(disks)); !slices.Equal(got, []string{"win"}) {
		t.Fatalf("a cancelled restore left %v next to the original", got)
	}
	if got := treeSum(t, disks, true); got != before {
		t.Fatal("the VM's disks changed")
	}
}

// seedFotos creates a file set on the fotos share and has the service back it
// up. selected narrows the set to those folders inside it.
func (r *restoreRig) seedFotos(t *testing.T, selected ...string) (store.FileSet, string) {
	t.Helper()
	r.write(t, map[string]string{
		"fotos/2024/urlaub/strand.jpg": "strand",
		"fotos/2025/garten.jpg":        "garten",
		"fotos/import/roh.dng":         "roh",
		"fotos/liesmich.txt":           "fotos",
	})
	set, err := r.st.CreateFileSet(store.FileSet{Name: "Fotos", Path: "fotos", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) > 0 {
		sel := make([]string, len(selected))
		for i, s := range selected {
			sel[i] = filepath.Join(r.root, "fotos", s)
		}
		if err := r.st.SetFileSetSelectedPaths(set.ID, sel); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.svc.BackupFileSet(context.Background(), set.ID); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	return set, r.lastRun(t, set.ID, "backup").SnapshotID
}

func TestRestoreBesideFileSetLeavesTheOriginalUntouched(t *testing.T) {
	r := newRestoreRig(t)
	set, snap := r.seedFotos(t)
	fotos := filepath.Join(r.root, "fotos")
	atBackup := treeSum(t, fotos, false)
	r.write(t, map[string]string{"fotos/2025/garten.jpg": "bearbeitet", "fotos/2026/neu.jpg": "neu"})
	before := treeSum(t, fotos, true)

	res := r.check(t, RestoreCheckRequest{Kind: checkFileSet, Name: set.ID, SnapshotID: snap, Target: targetBeside})
	wantBytes := int64(len("strand") + len("garten") + len("roh") + len("fotos"))
	if c := lineOf(t, res, lineSpace); !res.Ready || c.Need != wantBytes {
		t.Fatalf("ready=%v space=%+v, want %d bytes needed", res.Ready, c, wantBytes)
	}
	wantFolders := []BesideFolder{{Original: "/mnt/user/fotos", Copy: "/mnt/user/fotos-restored"}}
	if p := res.Plan; p.InPlace || p.Added != 4 || p.Beside == nil || p.Beside.Name != "fotos-restored" || !slices.Equal(p.Beside.Folders, wantFolders) {
		t.Fatalf("plan = %+v, beside %+v", p, p.Beside)
	}
	if _, err := os.Stat(fotos + besideSuffix); !os.IsNotExist(err) {
		t.Fatalf("a check created the copy's folder: %v", err)
	}

	events := r.watch(t, "files:Fotos")
	target, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", "", false, RestoreOptions{Beside: true})
	if err != nil || !started || target != fotos+besideSuffix {
		t.Fatalf("StartRestoreFileSet: target=%q started=%v err=%v", target, started, err)
	}
	r.wait(t)

	if got := treeSum(t, fotos, true); got != before {
		t.Fatal("the set's folder changed")
	}
	if got := treeSum(t, fotos+besideSuffix, false); got != atBackup {
		t.Fatal("the copy is not what the backup held")
	}
	if run := r.lastRun(t, set.ID, "restore"); run.Status != "success" || run.SnapshotID != snap {
		t.Fatalf("run = %+v", run)
	}
	seen := events()
	if len(seen) < 2 || seen[0].Phase != "restore" || !seen[0].Active || seen[len(seen)-1].Active {
		t.Fatalf("progress = %+v, want a restore that began and ended", seen)
	}
}

func TestRestoreBesideFileSetKeepsTheLayoutOfASelection(t *testing.T) {
	for name, tc := range map[string]struct {
		selected []string
		want     []string
	}{
		"one folder of the set":  {selected: []string{"2024"}, want: []string{"2024/urlaub/strand.jpg"}},
		"two folders of the set": {selected: []string{"2024", "2025"}, want: []string{"2024/urlaub/strand.jpg", "2025/garten.jpg"}},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRestoreRig(t)
			set, snap := r.seedFotos(t, tc.selected...)
			fotos := filepath.Join(r.root, "fotos")
			before := treeSum(t, fotos, true)

			if _, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", "", false, RestoreOptions{Beside: true}); err != nil || !started {
				t.Fatalf("StartRestoreFileSet: started=%v err=%v", started, err)
			}
			r.wait(t)

			var got []string
			err := filepath.WalkDir(fotos+besideSuffix, func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					rel, _ := filepath.Rel(fotos+besideSuffix, p)
					got = append(got, rel)
				}
				return err
			})
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("the copy holds %v (%v), want %v at their places inside the set", got, err, tc.want)
			}
			if treeSum(t, fotos, true) != before {
				t.Fatal("the set's folder changed")
			}
		})
	}
}

func TestRestoreBesideFileSetRefusals(t *testing.T) {
	r := newRestoreRig(t)
	set, snap := r.seedFotos(t)
	repath := func(t *testing.T, p string) {
		moved := set
		moved.Path = p
		if err := r.st.UpdateFileSet(moved); err != nil {
			t.Fatal(err)
		}
	}
	for name, tc := range map[string]struct {
		arrange func(t *testing.T) (undo func())
		target  string
		want    string
	}{
		"a folder of that name is there": {
			arrange: func(t *testing.T) func() { return r.inTheWay(t, "fotos-restored/keep.txt") },
			want:    "/mnt/user/fotos-restored already exists",
		},
		"the volume has no room for a copy": {
			arrange: func(*testing.T) func() { return r.noRoom() },
			want:    "not enough free space",
		},
		"the set has no folder": {
			arrange: func(t *testing.T) func() {
				repath(t, "")
				return func() { repath(t, set.Path) }
			},
			want: "no source path configured",
		},
		"a target folder is named as well": {
			arrange: func(*testing.T) func() { return func() {} },
			target:  "restore/fotos",
			want:    "takes no target folder",
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer tc.arrange(t)()
			before := treeSum(t, filepath.Join(r.root, "fotos"), true)
			around := entries(t, r.root)

			_, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", tc.target, false, RestoreOptions{Beside: true})

			if started || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("started=%v err=%v, want a refusal saying %q", started, err, tc.want)
			}
			if r.svc.BackupInProgress() {
				t.Fatal("a refused restore must not hold the single-flight guard")
			}
			if treeSum(t, filepath.Join(r.root, "fotos"), true) != before || !slices.Equal(entries(t, r.root), around) {
				t.Fatal("a refused restore changed the set's folder or what lies next to it")
			}
		})
	}
}

func TestRestoreBesideFileSetCancelledLeavesNoHalfCopy(t *testing.T) {
	r := newRestoreRig(t)
	set, snap := r.seedFotos(t)
	fotos := filepath.Join(r.root, "fotos")
	before := treeSum(t, fotos, true)
	entered := r.stall()

	if _, started, err := r.svc.StartRestoreFileSet(context.Background(), set.ID, snap, "local", "", false, RestoreOptions{Beside: true}); err != nil || !started {
		t.Fatalf("StartRestoreFileSet: started=%v err=%v", started, err)
	}
	await(t, entered)
	if !r.svc.CancelRun("files:Fotos") {
		t.Fatal("the restore cannot be cancelled under its progress key")
	}
	r.wait(t)

	if run := r.lastRun(t, set.ID, "restore"); run.Status != "cancelled" {
		t.Fatalf("run = %+v, want cancelled", run)
	}
	if _, err := os.Stat(fotos + besideSuffix); !os.IsNotExist(err) {
		t.Fatalf("a cancelled restore left its folder behind: %v", err)
	}
	if treeSum(t, fotos, true) != before {
		t.Fatal("the set's folder changed")
	}
}
