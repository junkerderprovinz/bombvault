package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestAnExactTagNeverMatchesALongerName(t *testing.T) {
	c := ownerContext{domain: "vms", known: map[string]bool{"vm:web": true, "vm:web2": true}}
	if got := c.identityOf("vm:web2"); !slices.Equal(got, []string{"vm:web2"}) {
		t.Errorf("identityOf(vm:web2) = %v", got)
	}
	if got := c.identityOf("vm:web:zvol:sda"); !slices.Equal(got, []string{"vm:web"}) {
		t.Errorf("a disk of web = %v, want [vm:web]", got)
	}
}

func TestAFileSetNameWithSpacesIsMatchedExactly(t *testing.T) {
	c := ownerContext{domain: "files"}
	if got := c.identityOf("fileset:Photos 2024"); !slices.Equal(got, []string{"fileset:Photos 2024"}) {
		t.Errorf("identityOf = %v", got)
	}
	if got := c.identityOf("container:nginx"); len(got) != 0 {
		t.Errorf("another domain's tag = %v, want none", got)
	}
}

func TestProjectFoldersBelongToContainersAndMarkersToNobody(t *testing.T) {
	c := ownerContext{domain: "containers"}
	if got := c.identityOf("stack:immich"); !slices.Equal(got, []string{"stack:immich"}) {
		t.Errorf("identityOf(stack:immich) = %v", got)
	}
	for _, tag := range []string{"formerly:old", "bv:direct", "p1", "container:", "vmrun:1"} {
		if got := c.identityOf(tag); len(got) != 0 {
			t.Errorf("identityOf(%s) = %v, want none", tag, got)
		}
	}
	flash := ownerContext{domain: "flash"}
	if got := flash.identityOf("flash"); !slices.Equal(got, []string{"flash"}) {
		t.Errorf("flash owns %v", got)
	}
}

func TestADiskTagOfTwoKnownVMsBelongsToBoth(t *testing.T) {
	both := ownerContext{domain: "vms", known: map[string]bool{"vm:a": true, "vm:a:zvol:b": true}}
	if got := both.identityOf("vm:a:zvol:b"); !slices.Equal(got, []string{"vm:a", "vm:a:zvol:b"}) {
		t.Errorf("both known = %v", got)
	}
	exact := ownerContext{domain: "vms", known: map[string]bool{"vm:a:zvol:b": true}}
	if got := exact.identityOf("vm:a:zvol:b"); !slices.Equal(got, []string{"vm:a:zvol:b"}) {
		t.Errorf("only the exact name known = %v", got)
	}
	none := ownerContext{domain: "vms", known: map[string]bool{}}
	if got := none.identityOf("vm:a:zvol:b"); !slices.Equal(got, []string{"vm:a"}) {
		t.Errorf("neither known = %v, want the run's VM", got)
	}
	if got := none.identityOf("vm:a:zvol:b:c"); !slices.Equal(got, []string{"vm:a:zvol:b:c"}) {
		t.Errorf("a device with a colon is no disk tag, got %v", got)
	}
}

func TestTheRunSiblingSettlesAnAmbiguousSnapshot(t *testing.T) {
	c := ownerContext{domain: "vms", known: map[string]bool{"vm:a": true, "vm:a:zvol:b": true}}
	snaps := []restic.Snapshot{
		snap("r1", 100, "vm:a", "vmrun:1"),
		snap("d1", 100, "vm:a:zvol:b", "vmrun:1"),
		snap("r2", 200, "vm:a:zvol:b", "vmrun:2"),
		snap("d2", 200, "vm:a:zvol:b:zvol:sdc", "vmrun:2"),
		snap("f1", 300, "vm:a:zvol:b"),
	}
	owners := c.owners(snaps)
	for id, want := range map[string]string{"r1": "vm:a", "d1": "vm:a", "r2": "vm:a:zvol:b", "d2": "vm:a:zvol:b", "f1": ""} {
		if got := owners[id].Owner; got != want {
			t.Errorf("owner of %s = %q, want %q", id, got, want)
		}
	}
	if got := owners["f1"].Possible; !slices.Equal(got, []string{"vm:a", "vm:a:zvol:b"}) {
		t.Errorf("an undecided snapshot may belong to %v, want both", got)
	}
}

func TestTheOwnerContextKnowsItemsAndRuleNames(t *testing.T) {
	f := newPlacementFixture(t)
	f.vm("a", "")
	f.rule("vms", "vm:a:zvol:b", store.SkipAll)
	c, err := f.svc.ownerContextFor("vms")
	if err != nil || !c.known["vm:a"] || !c.known["vm:a:zvol:b"] || c.domain != "vms" {
		t.Fatalf("ownerContextFor = %+v, %v", c, err)
	}
}
