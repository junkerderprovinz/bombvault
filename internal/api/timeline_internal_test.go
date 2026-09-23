package api

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func rowKeys(rows []timelineRow) []string {
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, r.Key)
	}
	return keys
}

func markOf(t *testing.T, rows []timelineRow, key, place string) timelineMark {
	t.Helper()
	for _, r := range rows {
		if r.Key != key {
			continue
		}
		for _, m := range r.Places {
			if m.Place == place {
				return m
			}
		}
	}
	t.Fatalf("no mark of %s at %s in %+v", key, place, rows)
	return timelineMark{}
}

func TestTimelineShowsACopyInTheRowOfItsOriginal(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.hold("b2:bucket:containers", copied("b9b9b9b9", "a1a1a1a1", 1_758_000_000, "container:nginx"))
	ctx := context.Background()

	tl, err := f.svc.Timeline(ctx, "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Places) != 2 {
		t.Fatalf("places = %+v, want the home and B2", tl.Places)
	}
	if p := tl.Places[0]; p.Place != "local" || p.Kind != "home" || p.State != "read" || p.Label != "" {
		t.Fatalf("home = %+v", p)
	}
	if p := tl.Places[1]; p.Place != "offsite:"+b2.ID || p.Label != "B2" || !p.Remote || p.State != "unchecked" {
		t.Fatalf("B2 = %+v", p)
	}
	if n := f.eng.lists["b2:bucket:containers"]; n != 0 {
		t.Fatalf("opening the timeline listed B2 %d times", n)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("rows = %v", got)
	}

	place, rows, err := f.svc.TimelinePlace(ctx, "containers", "nginx", "offsite:"+b2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if place.State != "read" || f.eng.lists["b2:bucket:containers"] != 1 {
		t.Fatalf("B2 = %+v after %d listings, want read with one", place, f.eng.lists["b2:bucket:containers"])
	}
	if m := markOf(t, rows, "a1a1a1a1", "offsite:"+b2.ID); !slices.Equal(m.SnapshotIDs, []string{"b9b9b9b9"}) {
		t.Fatalf("B2 mark = %+v", m)
	}
}

func TestTimelineReadsALocalTargetWhenItOpens(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	nasLoc := f.localRepo("copies/containers")
	nas := f.target("containers", "NAS copy", "copies/containers")
	f.hold(f.domainPath("containers"), snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.hold(nasLoc, copied("c1c1c1c1", "a1a1a1a1", 1_758_000_000, "container:nginx"))

	tl, err := f.svc.Timeline(context.Background(), "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if p := tl.Places[1]; p.Remote || p.State != "read" {
		t.Fatalf("NAS copy = %+v, want read at once", p)
	}
	if len(tl.Rows) != 1 {
		t.Fatalf("rows = %+v, want one row with two marks", tl.Rows)
	}
	markOf(t, tl.Rows, "a1a1a1a1", "local")
	markOf(t, tl.Rows, "a1a1a1a1", "offsite:"+nas.ID)
}

func TestTimelineGroupsByOriginNotByTags(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	hz := f.target("containers", "Hetzner", "sftp:u@hetzner:/containers")
	f.hold("b2:bucket:containers",
		copied("b1b1b1b1", "a1a1a1a1", 1_758_000_000, "container:nginx"),
		copied("b2b2b2b2", "a1a1a1a1", 1_758_000_000, "container:nginx", "keep"),
		copied("b7b7b7b7", "a7a7a7a7", 1_757_000_000, "container:nginx"),
	)
	// restic keeps the first original on a copy of a copy, so Hetzner's copy of
	// B2's b1 still names a1.
	f.hold("sftp:u@hetzner:/containers", copied("c1c1c1c1", "a1a1a1a1", 1_758_000_000, "container:nginx"))
	ctx := context.Background()

	_, atB2, err := f.svc.TimelinePlace(ctx, "containers", "nginx", "offsite:"+b2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowKeys(atB2); !slices.Equal(got, []string{"a1a1a1a1", "a7a7a7a7"}) {
		t.Fatalf("B2 rows = %v, want a1 and the backup only B2 holds", got)
	}
	m := markOf(t, atB2, "a1a1a1a1", "offsite:"+b2.ID)
	if !slices.Equal(m.SnapshotIDs, []string{"b2b2b2b2", "b1b1b1b1"}) || !slices.Contains(m.Tags, "keep") {
		t.Fatalf("B2 mark = %+v, want both copies newest first with the newest one's tags", m)
	}

	_, atHz, err := f.svc.TimelinePlace(ctx, "containers", "nginx", "offsite:"+hz.ID)
	if err != nil {
		t.Fatal(err)
	}
	markOf(t, atHz, "a1a1a1a1", "offsite:"+hz.ID)
}

func TestTimelineShowsSwitchedOffTargetsAndRefusesForeignPlaces(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	hz := f.target("containers", "Hetzner", "sftp:u@hetzner:/containers")
	hz.Enabled = false
	if _, err := f.st.UpsertOffsiteTarget(hz); err != nil {
		t.Fatal(err)
	}
	vmsB2 := f.target("vms", "B2", "b2:bucket:vms")
	f.hold("sftp:u@hetzner:/containers", copied("h1h1h1h1", "a1a1a1a1", 1_758_000_000, "container:nginx"))
	ctx := context.Background()

	tl, err := f.svc.Timeline(ctx, "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Places) != 2 || tl.Places[1].Place != "offsite:"+hz.ID || tl.Places[1].Enabled {
		t.Fatalf("places = %+v, want Hetzner listed switched off", tl.Places)
	}
	_, rows, err := f.svc.TimelinePlace(ctx, "containers", "nginx", "offsite:"+hz.ID)
	if err != nil {
		t.Fatal(err)
	}
	markOf(t, rows, "a1a1a1a1", "offsite:"+hz.ID)

	for _, place := range []string{"offsite:ffffffffffffffffffffffffffffffff", "offsite:" + vmsB2.ID} {
		if _, _, err := f.svc.TimelinePlace(ctx, "containers", "nginx", place); !errors.Is(err, errUnknownOffsiteTarget) {
			t.Errorf("%s: err = %v, want errUnknownOffsiteTarget", place, err)
		}
	}
}

func TestTimelineReadsTheItemsOwnRepository(t *testing.T) {
	f := newPlacementFixture(t)
	nasLoc := f.localRepo("nas/bv")
	nas := f.namedRepo("NAS Keller", "nas/bv")
	f.container("nginx", nas.ID)
	f.hold(nasLoc, snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	f.hold(f.domainPath("containers"), snap("d1d1d1d1", 1_757_000_000, "container:nginx"))

	tl, err := f.svc.Timeline(context.Background(), "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if p := tl.Places[0]; p.Label != "NAS Keller" || p.State != "read" {
		t.Fatalf("home = %+v, want NAS Keller read", p)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("rows = %v, want only what the item's repository holds", got)
	}
}

func TestTimelineWaitsForARemoteHomeLikeATarget(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.container("nginx", box.ID)
	f.hold("sftp:u@box:/bv", snap("a1a1a1a1", 1_758_000_000, "container:nginx"))
	ctx := context.Background()

	tl, err := f.svc.Timeline(ctx, "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if p := tl.Places[0]; !p.Remote || p.State != "unchecked" || len(tl.Rows) != 0 || f.eng.lists["sftp:u@box:/bv"] != 0 {
		t.Fatalf("home = %+v, rows %+v, want it unchecked and unlisted", p, tl.Rows)
	}
	place, rows, err := f.svc.TimelinePlace(ctx, "containers", "nginx", "local")
	if err != nil {
		t.Fatal(err)
	}
	if place.State != "read" {
		t.Fatalf("home = %+v", place)
	}
	markOf(t, rows, "a1a1a1a1", "local")
}

func TestTimelineNamesAPlaceItCannotReadAndLeavesOtherItemsOut(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.container("nginx2", "")
	nasLoc := f.localRepo("copies/containers")
	f.target("containers", "NAS copy", "copies/containers")
	f.eng.listErr[nasLoc] = errors.New("permission denied")
	f.hold(f.domainPath("containers"),
		snap("a1a1a1a1", 1_758_000_000, "container:nginx"),
		snap("e1e1e1e1", 1_758_000_000, "container:nginx2"),
	)

	tl, err := f.svc.Timeline(context.Background(), "containers", "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if p := tl.Places[1]; p.State != "unreadable" || !strings.Contains(p.Error, "permission denied") {
		t.Fatalf("NAS copy = %+v, want unreadable with the reason", p)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("rows = %v, want nginx2's backup left out", got)
	}
}

func TestFileSetTimelineFollowsTheSetsName(t *testing.T) {
	f := newPlacementFixture(t)
	set := f.fileSet("Photos 2024", "")
	f.hold(f.domainPath("files"),
		snap("a1a1a1a1", 1_758_000_000, "fileset:Photos 2024"),
		snap("a2a2a2a2", 1_758_000_000, "fileset:Photos"),
	)
	tl, err := f.svc.Timeline(context.Background(), "files", set.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("rows = %v", got)
	}
}

func TestVMTimelineShowsRunsAndMarksAPlaceWithoutTheirDisksIncomplete(t *testing.T) {
	f := newPlacementFixture(t)
	f.zvolVM("web")
	b2 := f.target("vms", "B2", "b2:bucket:vms")
	f.hold(f.domainPath("vms"),
		snap("a1a1a1a1", 1_758_000_000, "vm:web", "vmrun:r1"),
		snap("a2a2a2a2", 1_758_000_000, "vm:web:zvol:sda", "vmrun:r1"),
	)
	f.hold("b2:bucket:vms", copied("b1b1b1b1", "a1a1a1a1", 1_758_000_000, "vm:web", "vmrun:r1"))
	ctx := context.Background()

	tl, err := f.svc.Timeline(ctx, "vms", "web")
	if err != nil {
		t.Fatal(err)
	}
	if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("rows = %v, want the run only", got)
	}
	if m := markOf(t, tl.Rows, "a1a1a1a1", "local"); m.Incomplete {
		t.Fatalf("home mark = %+v, the disk is there", m)
	}
	_, rows, err := f.svc.TimelinePlace(ctx, "vms", "web", "offsite:"+b2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m := markOf(t, rows, "a1a1a1a1", "offsite:"+b2.ID); !m.Incomplete {
		t.Fatalf("B2 mark = %+v, want incomplete: B2 lacks the disk", m)
	}
}

func TestVMTimelineOfAVMWithoutZvolDisksIsNeverIncomplete(t *testing.T) {
	f := newPlacementFixture(t)
	f.vm("files-only", "")
	f.hold(f.domainPath("vms"), snap("a1a1a1a1", 1_758_000_000, "vm:files-only"))

	tl, err := f.svc.Timeline(context.Background(), "vms", "files-only")
	if err != nil {
		t.Fatal(err)
	}
	if m := markOf(t, tl.Rows, "a1a1a1a1", "local"); m.Incomplete {
		t.Fatalf("mark = %+v", m)
	}
}

func TestVMTimelineNeverShowsADiskSnapshotAsARow(t *testing.T) {
	f := newPlacementFixture(t)
	f.vm("a", "")
	f.vm("a:zvol:b", "")
	f.hold(f.domainPath("vms"),
		snap("a1a1a1a1", 1_758_000_000, "vm:a", "vmrun:r1"),
		snap("a2a2a2a2", 1_758_000_000, "vm:a:zvol:b", "vmrun:r1"),
		snap("a3a3a3a3", 1_759_000_000, "vm:a:zvol:b", "vmrun:r2"),
	)
	ctx := context.Background()

	a, err := f.svc.Timeline(ctx, "vms", "a")
	if err != nil {
		t.Fatal(err)
	}
	if got := rowKeys(a.Rows); !slices.Equal(got, []string{"a1a1a1a1"}) {
		t.Fatalf("a rows = %v, want its run without the disk of the same run", got)
	}
	of, err := f.svc.Timeline(ctx, "vms", "a:zvol:b")
	if err != nil {
		t.Fatal(err)
	}
	if len(of.Rows) != 0 {
		t.Fatalf("a:zvol:b rows = %+v, want none: a3 names both VMs and no run settles which", of.Rows)
	}
}

func TestFlashAndConfigTimelinesShowTheDomainPathAndItsTargets(t *testing.T) {
	for _, domain := range []string{"flash", "config"} {
		t.Run(domain, func(t *testing.T) {
			f := newPlacementFixture(t)
			home := f.singletonRepo(domain)
			b2 := f.target(domain, "B2", "b2:bucket:"+domain)
			f.hold(home,
				snap("a0a0a0a0", 1_757_000_000),
				snap("a1a1a1a1", 1_758_000_000, domain),
			)
			f.hold("b2:bucket:"+domain, copied("b1b1b1b1", "a1a1a1a1", 1_758_000_000, domain))
			ctx := context.Background()

			tl, err := f.svc.Timeline(ctx, domain, domain)
			if err != nil {
				t.Fatal(err)
			}
			if len(tl.Places) != 2 || tl.Places[0].State != "read" || tl.Places[1].Place != "offsite:"+b2.ID {
				t.Fatalf("places = %+v", tl.Places)
			}
			if got := rowKeys(tl.Rows); !slices.Equal(got, []string{"a1a1a1a1", "a0a0a0a0"}) {
				t.Fatalf("rows = %v, want every snapshot of the repository", got)
			}
			_, rows, err := f.svc.TimelinePlace(ctx, domain, domain, "offsite:"+b2.ID)
			if err != nil {
				t.Fatal(err)
			}
			markOf(t, rows, "a1a1a1a1", "offsite:"+b2.ID)
		})
	}
}
