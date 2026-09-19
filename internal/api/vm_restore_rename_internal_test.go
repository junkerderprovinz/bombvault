package api

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// Unraid moves a VM's disk folder along with its name, so a snapshot taken
// before the rename holds the disks under the old folder.
const (
	win11Dir     = "/host/user/domains/win11"
	windows11Dir = "/host/user/domains/windows-11"
)

const win11DomainXML = `<domain type='kvm'><name>win11</name><devices>` +
	`<disk type='file' device='disk'><source file='/mnt/domains/win11/vdisk1.img'/><target dev='hdc'/></disk>` +
	`</devices></domain>`

// The renamed entry keeps its name, disk paths and domain XML, and the old
// folder of the snapshot picked from its history is restored into the one the
// VM uses. Its newest snapshot, taken after the rename, restores in place.
func TestPrepareRestoreVMRestoresAPreRenameSnapshotIntoTheCurrentFolder(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}, Paths: []string{windows11Dir + "/vdisk1.img"}},
		{ID: "bbbb2222bbbb2222", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}, Paths: []string{win11Dir + "/vdisk1.img"}},
	}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	disks := []string{win11Dir + "/vdisk1.img"}
	tg, err := s.store.UpsertVMTarget(vmTargetJSON(t, "win11", win11DomainXML, disks, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.AddAliasAt("vm", "windows-11", tg.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}

	ref := repoRef{repo: repoDir}
	plan, err := s.prepareRestoreVMIn(context.Background(), ref, "win11", "aaaa1111aaaa1111", true)
	if err != nil {
		t.Fatalf("prepareRestoreVMIn: %v", err)
	}
	want := []backup.VMRestoreDir{{Subtree: windows11Dir, Target: win11Dir}}
	if !reflect.DeepEqual(plan.restoreDirs, want) {
		t.Fatalf("restoreDirs = %v, want %v", plan.restoreDirs, want)
	}
	if !reflect.DeepEqual(plan.diskPaths, disks) {
		t.Fatalf("diskPaths = %v, want the current ones %v", plan.diskPaths, disks)
	}
	if plan.domainXML != virshcli.EnsureNVRAMTemplate(win11DomainXML) {
		t.Fatalf("domain XML changed: %q", plan.domainXML)
	}

	latest, err := s.prepareRestoreVMIn(context.Background(), ref, "win11", "latest", true)
	if err != nil {
		t.Fatalf("prepareRestoreVMIn latest: %v", err)
	}
	if latest.snapshotID != "bbbb2222bbbb2222" || latest.restoreDirs != nil {
		t.Fatalf("latest = %s with restoreDirs %v, want bbbb2222bbbb2222 in place", latest.snapshotID, latest.restoreDirs)
	}
}

// A snapshot that predates a disk in the same folder restores in place: the
// older disk is rolled back and the one added since is left alone.
func TestPrepareRestoreVMRestoresInPlaceWhenTheSnapshotPredatesADiskInTheSameFolder(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:win11"}, Paths: []string{win11Dir + "/vdisk1.img"}}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	disks := []string{win11Dir + "/vdisk1.img", win11Dir + "/vdisk2.img"}
	tg := vmTargetJSON(t, "win11", win11DomainXML, disks, "")

	plan, err := s.prepareRestoreVMForTarget(context.Background(), repoRef{repo: repoDir}, "win11", "latest", tg, tagIdentity("vm:win11"), "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if plan.restoreDirs != nil || !reflect.DeepEqual(plan.diskPaths, disks) {
		t.Fatalf("plan = %+v, want the in-place restore of %v", plan, disks)
	}
}

// A disk outside the renamed folder stays where it is, and still gets
// restored, because a folder list replaces the in-place restore for every
// disk.
func TestPrepareRestoreVMRemapsOnlyTheFolderThatMoved(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{
		ID: "deadbeef12345678", Tags: []string{"vm:win11"},
		Paths: []string{"/host/user/vmdata/games.img", windows11Dir + "/vdisk1.img", windows11Dir + "/vdisk2.img"},
	}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	disks := []string{"/host/user/vmdata/games.img", win11Dir + "/vdisk1.img", win11Dir + "/vdisk2.img"}
	tg := vmTargetJSON(t, "win11", win11DomainXML, disks, "")

	plan, err := s.prepareRestoreVMForTarget(context.Background(), repoRef{repo: repoDir}, "win11", "latest", tg, tagIdentity("vm:win11"), "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	want := []backup.VMRestoreDir{
		{Subtree: "/host/user/vmdata", Target: "/host/user/vmdata"},
		{Subtree: windows11Dir, Target: win11Dir},
	}
	if !reflect.DeepEqual(plan.restoreDirs, want) {
		t.Fatalf("restoreDirs = %v, want %v", plan.restoreDirs, want)
	}
}

// Two disks may share a file name in different folders, and a snapshot that
// holds them where they are restores in place.
func TestPrepareRestoreVMKeepsSameNamedDisksInPlaceWhenNothingMoved(t *testing.T) {
	disks := []string{win11Dir + "/vdisk1.img", "/host/user/fast/win11/vdisk1.img"}
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:win11"}, Paths: disks}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	tg := vmTargetJSON(t, "win11", win11DomainXML, disks, "")

	plan, err := s.prepareRestoreVMForTarget(context.Background(), repoRef{repo: repoDir}, "win11", "latest", tg, tagIdentity("vm:win11"), "", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	if plan.restoreDirs != nil || !reflect.DeepEqual(plan.diskPaths, disks) {
		t.Fatalf("plan = %+v, want the in-place restore of %v", plan, disks)
	}
}

// The snapshot decides where the disks are read from, the destination folder
// where they go.
func TestPrepareRestoreVMCrossInstanceReadsAPreRenameSnapshotFromItsOwnFolder(t *testing.T) {
	eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{
		ID: "deadbeef12345678", Tags: []string{"vm:win11"}, Paths: []string{windows11Dir + "/vdisk1.img"},
	}}}
	s := vmRestoreSvc(t, eng)
	repoDir := filepath.Join(t.TempDir(), "repo")
	seedResticRepoDir(t, repoDir)
	writeMountFixture(t, "/", "/host/user", "/host/user/vmrestore")
	tg := vmTargetJSON(t, "win11", win11DomainXML, []string{win11Dir + "/vdisk1.img"}, "")

	plan, err := s.prepareRestoreVMForTarget(context.Background(), repoRef{repo: repoDir}, "win11", "latest", tg, tagIdentity("vm:win11"), "/host/user/vmrestore", "")
	if err != nil {
		t.Fatalf("prepareRestoreVMForTarget: %v", err)
	}
	want := []backup.VMRestoreDir{{Subtree: windows11Dir, Target: "/host/user/vmrestore/win11"}}
	if !reflect.DeepEqual(plan.restoreDirs, want) {
		t.Fatalf("restoreDirs = %v, want %v", plan.restoreDirs, want)
	}
	if !reflect.DeepEqual(plan.diskPaths, []string{"/host/user/vmrestore/win11/vdisk1.img"}) {
		t.Fatalf("diskPaths = %v, want the destination", plan.diskPaths)
	}
	if !strings.Contains(plan.domainXML, "file='/mnt/vmrestore/win11/vdisk1.img'") {
		t.Fatalf("XML disk source must point at the destination, got %q", plan.domainXML)
	}
}

// A disk the snapshot does not hold, or cannot be told apart in it, and a
// snapshot folder whose disks are split across two folders stop the restore
// while the running VM is still defined and untouched.
func TestRestoreVMRefusesAnUnpairedDiskBeforeTouchingTheVM(t *testing.T) {
	cases := []struct {
		name     string
		disks    []string
		snapshot []string
		wantErr  []string
	}{
		{
			name:     "the snapshot lacks a disk of a moved folder",
			disks:    []string{win11Dir + "/vdisk1.img", win11Dir + "/vdisk2.img"},
			snapshot: []string{windows11Dir + "/vdisk1.img"},
			wantErr:  []string{"no disk named vdisk2.img"},
		},
		{
			name:     "the snapshot lacks a disk of a new folder",
			disks:    []string{win11Dir + "/vdisk1.img", "/host/user/fast/win11/vdisk2.img"},
			snapshot: []string{win11Dir + "/vdisk1.img"},
			wantErr:  []string{"no disk named vdisk2.img"},
		},
		{
			name:     "the snapshot holds the name twice",
			disks:    []string{win11Dir + "/vdisk1.img"},
			snapshot: []string{windows11Dir + "/vdisk1.img", "/host/user/fast/windows-11/vdisk1.img"},
			wantErr:  []string{"vdisk1.img is not unique"},
		},
		{
			name:     "the definition holds the name twice",
			disks:    []string{win11Dir + "/vdisk1.img", "/host/user/fast/win11/vdisk1.img"},
			snapshot: []string{windows11Dir + "/vdisk1.img"},
			wantErr:  []string{"vdisk1.img is not unique"},
		},
		{
			name:     "a moved folder's disks now sit in two folders",
			disks:    []string{win11Dir + "/vdisk1.img", "/host/user/fast/win11/vdisk2.img"},
			snapshot: []string{windows11Dir + "/vdisk1.img", windows11Dir + "/vdisk2.img"},
			wantErr:  []string{"/mnt/domains/windows-11", "/mnt/domains/win11", "/mnt/fast/win11"},
		},
		{
			name:     "a disk moved out of a folder that stayed",
			disks:    []string{win11Dir + "/vdisk1.img", "/host/user/fast/win11/vdisk2.img"},
			snapshot: []string{win11Dir + "/vdisk1.img", win11Dir + "/vdisk2.img"},
			wantErr:  []string{"/mnt/domains/win11", "/mnt/fast/win11"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eng := &foreignRecordingEngine{snaps: []restic.Snapshot{{ID: "deadbeef12345678", Tags: []string{"vm:win11"}, Paths: tc.snapshot}}}
			s := vmRestoreSvc(t, eng)
			v := &recordingVirsh{state: "running"}
			s.virsh = v
			settings, err := s.store.GetSettings()
			if err != nil {
				t.Fatal(err)
			}
			settings.VMsPath = "rest:http://fake/vms"
			if err := s.store.UpdateSettings(settings); err != nil {
				t.Fatal(err)
			}
			if _, err := s.store.UpsertVMTarget(vmTargetJSON(t, "win11", "<domain/>", tc.disks, "")); err != nil {
				t.Fatal(err)
			}

			err = s.RestoreVM(context.Background(), "win11", "latest", true, "", false)
			if err == nil {
				t.Fatalf("RestoreVM = nil, want a refusal containing %q", tc.wantErr)
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("RestoreVM = %v, want a refusal containing %q", err, want)
				}
			}
			if len(v.teardown) != 0 || len(eng.restores) != 0 {
				t.Fatalf("virsh saw %v and restic restored %v; want nothing touched", v.teardown, eng.restores)
			}
		})
	}
}
