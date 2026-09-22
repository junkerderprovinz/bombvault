package api

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
)

// forgottenAt collects the snapshot ids a run forgot at one location.
func forgottenAt(f *placementFixture, loc string) []string {
	var ids []string
	for _, d := range f.eng.deletes {
		if d.Repo == loc {
			ids = append(ids, d.IDs...)
		}
	}
	slices.Sort(ids)
	return ids
}

func TestDeletingAContainerAtATargetKeepsItsEntry(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.container("nginx2", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold("b2:bucket:containers",
		copied("c1", "a1", 100, "container:nginx"),
		copied("c2", "a2", 200, "container:nginx"),
		copied("c3", "a3", 300, "container:nginx2"),
	)
	f.listing("containers", b2.ID, 400, copiesRow("container:nginx", 2, 200), copiesRow("container:nginx2", 1, 300))

	if err := f.svc.DeleteBackups(context.Background(), "nginx", "offsite:"+b2.ID); err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); !slices.Equal(got, []string{"c1", "c2"}) {
		t.Fatalf("forgotten at B2 = %v, want c1 and c2", got)
	}
	if _, err := f.st.GetTargetByContainer("nginx"); err != nil {
		t.Fatalf("the entry must stay: %v", err)
	}
	copies, err := f.st.ItemCopiesForDomain("containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 1 || copies[0].Identity != "container:nginx2" {
		t.Fatalf("observed copies after the delete = %+v, want only nginx2", copies)
	}
}

func TestDeletingAFileSetAtATargetKeepsTheSet(t *testing.T) {
	f := newPlacementFixture(t)
	set := f.fileSet("Photos 2024", "")
	b2 := f.target("files", "B2", "b2:bucket:files")
	f.hold("b2:bucket:files",
		copied("f1", "a1", 100, "fileset:Photos 2024"),
		copied("f2", "a2", 100, "fileset:Photos"),
	)
	if err := f.svc.DeleteBackupsFileSet(context.Background(), set.ID, "offsite:"+b2.ID); err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, "b2:bucket:files"); !slices.Equal(got, []string{"f1"}) {
		t.Fatalf("forgotten at B2 = %v, want f1", got)
	}
	if _, err := f.st.GetFileSet(set.ID); err != nil {
		t.Fatalf("the set must stay: %v", err)
	}
}

func TestDeletingAVMAtATargetTakesItsDisksAndLeavesOtherVMs(t *testing.T) {
	f := newPlacementFixture(t)
	f.vm("web", "")
	f.vm("web2", "")
	b2 := f.target("vms", "B2", "b2:bucket:vms")
	f.hold("b2:bucket:vms",
		copied("v1", "a1", 100, "vm:web", "vmrun:r1"),
		copied("v2", "a2", 100, "vm:web:zvol:sda", "vmrun:r1"),
		copied("v3", "a3", 100, "vm:web2", "vmrun:r2"),
		copied("v4", "a4", 100, "vm:web2:zvol:sda", "vmrun:r2"),
	)
	if err := f.svc.DeleteBackupsVM(context.Background(), "web", "offsite:"+b2.ID); err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, "b2:bucket:vms"); !slices.Equal(got, []string{"v1", "v2"}) {
		t.Fatalf("forgotten at B2 = %v, want v1 and v2", got)
	}
	if _, err := f.st.GetVMTargetByName("web"); err != nil {
		t.Fatalf("the entry must stay: %v", err)
	}
}

// A disk snapshot alone in its run has no sibling naming which of its two
// candidates owns it, so it belongs to neither and no delete at the target
// reaches it. Its twin in a run that does hold the VM goes with the VM.
func TestDeletingAVMAtATargetLeavesADiskNoRunSiblingClaims(t *testing.T) {
	f := newPlacementFixture(t)
	f.vm("a", "")
	f.vm("a:zvol:b", "")
	b2 := f.target("vms", "B2", "b2:bucket:vms")
	f.hold("b2:bucket:vms",
		copied("v1", "a1", 100, "vm:a", "vmrun:r1"),
		copied("v2", "a2", 100, "vm:a:zvol:b", "vmrun:r1"),
		copied("v3", "a3", 200, "vm:a:zvol:b", "vmrun:r2"),
	)
	if err := f.svc.DeleteBackupsVM(context.Background(), "a:zvol:b", "offsite:"+b2.ID); err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, "b2:bucket:vms"); len(got) != 0 {
		t.Fatalf("forgotten at B2 = %v, want nothing: v2 is a disk of VM a and nothing claims v3", got)
	}
	if err := f.svc.DeleteBackupsVM(context.Background(), "a", "offsite:"+b2.ID); err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, "b2:bucket:vms"); !slices.Equal(got, []string{"v1", "v2"}) {
		t.Fatalf("forgotten at B2 = %v, want v1 and v2; v3 is claimed by nobody", got)
	}
}

func TestDeletingAtASwitchedOffTargetReachesThatTarget(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@hetzner:/containers")
	hz.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(hz); err != nil {
		t.Fatal(err)
	}
	f.hold("b2:bucket:containers", copied("b1", "a1", 100, "container:nginx"))
	f.hold("sftp:u@hetzner:/containers", copied("h1", "a1", 100, "container:nginx"))

	if err := f.svc.DeleteBackups(context.Background(), "nginx", "offsite:"+hz.ID); err != nil {
		t.Fatal(err)
	}
	if got := forgottenAt(f, "sftp:u@hetzner:/containers"); !slices.Equal(got, []string{"h1"}) {
		t.Fatalf("forgotten at Hetzner = %v, want h1", got)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); len(got) != 0 {
		t.Fatalf("a delete at Hetzner reached B2: %v", got)
	}
}

func TestAppendOnlyRefusesDeletingAtASwitchedOffTarget(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	hz := f.target("containers", "Hetzner", "sftp:u@hetzner:/containers")
	hz.Enabled = false
	hz.Immutable = true
	if _, err := f.st.UpsertOffsiteTarget(hz); err != nil {
		t.Fatal(err)
	}
	f.hold("sftp:u@hetzner:/containers", copied("h1", "a1", 100, "container:nginx"))

	err := f.svc.DeleteBackups(context.Background(), "nginx", "offsite:"+hz.ID)
	if !errors.Is(err, errAppendOnlyOffsiteTarget) {
		t.Fatalf("err = %v, want the append-only refusal", err)
	}
	if len(f.eng.deletes) != 0 {
		t.Fatalf("a refused delete reached the engine: %+v", f.eng.deletes)
	}
}

func TestAppendOnlySwitchedOnWhileTheDeleteRunsRefuses(t *testing.T) {
	for _, c := range []struct {
		domain, identity, loc string
		run                   func(f *placementFixture, source string) error
	}{
		{"containers", "container:nginx", "b2:bucket:containers", func(f *placementFixture, source string) error {
			f.container("nginx", "")
			return f.svc.DeleteBackups(context.Background(), "nginx", source)
		}},
		{"vms", "vm:web", "b2:bucket:vms", func(f *placementFixture, source string) error {
			f.vm("web", "")
			return f.svc.DeleteBackupsVM(context.Background(), "web", source)
		}},
	} {
		t.Run(c.domain, func(t *testing.T) {
			f := newPlacementFixture(t)
			b2 := f.target(c.domain, "B2", c.loc)
			f.hold(c.loc, copied("b1", "a1", 100, c.identity))
			// The flag is switched on once the listing is already in hand, which
			// is the moment the second question exists for.
			f.eng.onSnapshots = sync.OnceFunc(func() { f.appendOnly(b2.ID) })

			if err := c.run(f, "offsite:"+b2.ID); !errors.Is(err, errAppendOnlyOffsiteTarget) {
				t.Fatalf("err = %v, want the append-only refusal from inside the lock", err)
			}
			if len(f.eng.deletes) != 0 {
				t.Fatalf("the delete went through: %+v", f.eng.deletes)
			}
		})
	}
}

func TestDeletingAtAnUnknownTargetIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.target("containers", "B2", "b2:bucket:containers")
	err := f.svc.DeleteBackups(context.Background(), "nginx", "offsite:0123456789abcdef0123456789abcdef")
	if !errors.Is(err, errUnknownOffsiteTarget) {
		t.Fatalf("err = %v, want errUnknownOffsiteTarget", err)
	}
}

func TestBulkDeleteRoutesTakeASource(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	set := f.fileSet("Photos", "")
	b2c := f.target("containers", "B2", "b2:bucket:containers")
	b2f := f.target("files", "B2", "b2:bucket:files")
	f.hold("b2:bucket:containers", copied("c1", "a1", 100, "container:nginx"))
	f.hold("b2:bucket:files", copied("f1", "a2", 100, "fileset:Photos"))

	if m := f.do("DELETE", "/api/containers/nginx/backups?source=offsite:"+b2c.ID, nil); m["ok"] != true {
		t.Fatalf("container route: %v", m)
	}
	if m := f.do("DELETE", "/api/files/sets/"+set.ID+"/backups?source=offsite:"+b2f.ID, nil); m["ok"] != true {
		t.Fatalf("file set route: %v", m)
	}
	if got := forgottenAt(f, "b2:bucket:containers"); !slices.Equal(got, []string{"c1"}) {
		t.Fatalf("container route forgot %v", got)
	}
	if got := forgottenAt(f, "b2:bucket:files"); !slices.Equal(got, []string{"f1"}) {
		t.Fatalf("file set route forgot %v", got)
	}
	if _, err := f.st.GetTargetByContainer("nginx"); err != nil {
		t.Fatalf("the container entry must stay: %v", err)
	}
	if _, err := f.st.GetFileSet(set.ID); err != nil {
		t.Fatalf("the set must stay: %v", err)
	}
}
