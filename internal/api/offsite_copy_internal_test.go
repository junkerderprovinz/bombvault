package api

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// forgetLines lists the keep-policy calls at one location as "<tags> keep-last <n>", sorted.
func forgetLines(f *placementFixture, loc string) []string {
	var out []string
	for _, c := range f.eng.forgets {
		if c.Repo == loc {
			out = append(out, fmt.Sprintf("%s keep-last %d prune=%v", strings.Join(c.Tags, ","), c.Policy.KeepLast, c.Prune))
		}
	}
	slices.Sort(out)
	return out
}

// namedScene is a containers domain with nginx on a local named repository and
// plex on the domain path, both copied to B2 with keep-last 3.
func namedScene(t *testing.T) (*placementFixture, string) {
	t.Helper()
	f := newPlacementFixture(t)
	keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 3)
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.container("plex", "")
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("p1", 100, "container:plex"), snap("p2", 200, "container:plex"))
	f.hold(f.root+"/nas", snap("n1", 100, "container:nginx"), snap("n2", 200, "container:nginx"), snap("w1", 150, "vm:win11"))
	f.hold("b2:bucket:containers", copied("b1", "n1", 100, "container:nginx"))
	return f, f.root + "/nas"
}

func TestWithoutRulesTheCopyCallsStayAsTheyWere(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		f, nas := namedScene(t)
		if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
			t.Fatal(err)
		}
		want := []copyCall{
			{Dest: "b2:bucket:containers", Src: f.domainPath("containers")},
			{Dest: "b2:bucket:containers", Src: nas, IDs: []string{"n2"}},
		}
		if !slices.EqualFunc(f.eng.copies, want, func(a, b copyCall) bool {
			return a.Dest == b.Dest && a.Src == b.Src && slices.Equal(a.IDs, b.IDs) && (a.IDs == nil) == (b.IDs == nil)
		}) {
			t.Fatalf("copies = %+v, want %+v", f.eng.copies, want)
		}
		if got, want := forgetLines(f, "b2:bucket:containers"), []string{"container:nginx keep-last 3 prune=false", "container:plex keep-last 3 prune=false"}; !slices.Equal(got, want) {
			t.Fatalf("forgets = %v, want %v", got, want)
		}
		if !slices.Equal(f.eng.prunes, []string{"b2:bucket:containers"}) {
			t.Fatalf("prunes = %v, want one at B2", f.eng.prunes)
		}
	})
	t.Run("hook", func(t *testing.T) {
		f, nas := namedScene(t)
		f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), nas, "container:nginx")
		if len(f.eng.copies) != 1 || f.eng.copies[0].Src != nas || !slices.Equal(f.eng.copies[0].IDs, []string{"n2"}) {
			t.Fatalf("copies = %+v, want n2 from the NAS", f.eng.copies)
		}
		if got, want := forgetLines(f, "b2:bucket:containers"), []string{"container:nginx keep-last 3 prune=false"}; !slices.Equal(got, want) {
			t.Fatalf("forgets = %v, want %v", got, want)
		}
	})
}

func TestAFailedTargetListingWithoutRulesHandsOverEveryID(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("containers", "B2", "b2:bucket:containers")
	nas := f.namedRepo("NAS", "nas")
	f.container("nginx", nas.ID)
	f.replicated("containers")
	f.hold(f.root+"/nas", snap("n1", 100, "container:nginx"), snap("n2", 200, "container:nginx"))
	f.eng.listErr["b2:bucket:containers"] = errors.New("503 service unavailable")

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(f.eng.copies, func(c copyCall) bool { return c.Src == f.root+"/nas" })
	if i < 0 || !slices.Equal(f.eng.copies[i].IDs, []string{"n1", "n2"}) {
		t.Fatalf("copies = %+v, want every nginx id from the NAS", f.eng.copies)
	}
}

func TestARemoteDomainPathIsCopiedWhole(t *testing.T) {
	f := newPlacementFixture(t)
	settings := settingsOf(t, f.svc)
	settings.ContainersPath = "s3:https://s3.example/primary"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.hold("s3:https://s3.example/primary", snap("a1", 100, "container:nginx"))

	f.svc.replicateOffsite(context.Background(), "containers", settingsOf(t, f.svc), "s3:https://s3.example/primary", "container:nginx")
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.eng.copies {
		if c.Src != "s3:https://s3.example/primary" || c.IDs != nil {
			t.Fatalf("copies = %+v, want the remote domain path whole each time", f.eng.copies)
		}
	}
	if len(f.eng.copies) != 2 {
		t.Fatalf("copies = %+v, want one from the hook and one from the run", f.eng.copies)
	}
}

func TestAnItemOnLocalIsLeftOutOfTheCopy(t *testing.T) {
	for name, path := range map[string]string{"local domain path": "", "remote domain path": "s3:https://s3.example/primary"} {
		t.Run(name, func(t *testing.T) {
			f := newPlacementFixture(t)
			src := f.domainPath("containers")
			if path != "" {
				settings := settingsOf(t, f.svc)
				settings.ContainersPath = path
				if err := f.st.UpdateSettings(settings); err != nil {
					t.Fatal(err)
				}
				src = path
			}
			f.target("containers", "B2", "b2:bucket:containers")
			f.replicated("containers")
			f.rule("containers", "container:plex", store.SkipAll)
			f.hold(src, snap("a1", 100, "container:nginx"), snap("p1", 100, "container:plex"), snap("s1", 100, "stack:immich"))

			if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
				t.Fatal(err)
			}
			if len(f.eng.copies) != 1 || !slices.Equal(f.eng.copies[0].IDs, []string{"a1", "s1"}) {
				t.Fatalf("copies = %+v, want a1 and s1 by id", f.eng.copies)
			}
		})
	}
}

func TestRulesAndAFailedTargetListingCopyNothing(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := keepLast(t, f, f.target("containers", "B2", "b2:bucket:containers"), 3)
	f.replicated("containers")
	f.rule("containers", "container:plex", store.SkipAll)
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"), snap("p1", 100, "container:plex"))
	f.eng.listErr["b2:bucket:containers"] = errors.New("503 service unavailable")

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err == nil {
		t.Fatal("a pass that could not tell what B2 holds reported success")
	}
	if len(f.eng.copies)+len(f.eng.forgets)+len(f.eng.prunes) != 0 {
		t.Fatalf("copied %+v, forgot %+v, pruned %v without knowing what B2 holds", f.eng.copies, f.eng.forgets, f.eng.prunes)
	}
	if runs := offsiteRuns(t, f, "containers"); len(runs) != 1 || !strings.HasPrefix(runs[0], b2.ID+" ok=0 ") {
		t.Fatalf("runs = %v, want one failed run at B2", runs)
	}
}

func TestTwelveHundredIDsGoInThreeBlocks(t *testing.T) {
	many := func(prefix, tag string) []restic.Snapshot {
		out := make([]restic.Snapshot, 0, 1200)
		for i := range 1200 {
			out = append(out, snap(fmt.Sprintf("%s%04d", prefix, i), int64(1000+i), tag))
		}
		return out
	}
	t.Run("filtered domain path", func(t *testing.T) {
		f := newPlacementFixture(t)
		f.target("containers", "B2", "b2:bucket:containers")
		f.replicated("containers")
		f.rule("containers", "container:plex", store.SkipAll)
		f.hold(f.domainPath("containers"), many("n", "container:nginx")...)
		if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
			t.Fatal(err)
		}
		assertBlocks(t, f.eng.copies, []int{500, 500, 200})
	})
	t.Run("named source without rules", func(t *testing.T) {
		f := newPlacementFixture(t)
		f.target("containers", "B2", "b2:bucket:containers")
		nas := f.namedRepo("NAS", "nas")
		f.container("nginx", nas.ID)
		f.replicated("containers")
		f.hold(f.root+"/nas", many("n", "container:nginx")...)
		if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
			t.Fatal(err)
		}
		assertBlocks(t, f.eng.copies, []int{500, 500, 200})
	})
}

// assertBlocks checks the sizes of the copy calls that named ids.
func assertBlocks(t *testing.T, calls []copyCall, want []int) {
	t.Helper()
	var got []int
	for _, c := range calls {
		if c.IDs != nil {
			got = append(got, len(c.IDs))
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("blocks = %v, want %v", got, want)
	}
}

func TestAnOwnerTakesItsRuleByTheExactName(t *testing.T) {
	f := newPlacementFixture(t)
	f.target("vms", "B2", "b2:bucket:vms")
	f.vm("a", "")
	f.vm("a:zvol:b", "")
	f.vm("web", "")
	f.vm("web2", "")
	f.replicated("vms")
	f.rule("vms", "vm:a", store.SkipAll)
	f.rule("vms", "vm:web", store.SkipAll)
	f.hold(f.domainPath("vms"),
		snap("r1", 100, "vm:a"),
		snap("d1", 100, "vm:a:zvol:b"),
		snap("w1", 100, "vm:web"),
		snap("w2", 100, "vm:web2"),
		snap("z1", 100, "vm:web:zvol:sda"),
	)
	if err := f.svc.ReplicateOffsite(context.Background(), "vms"); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.copies) != 1 || !slices.Equal(f.eng.copies[0].IDs, []string{"d1", "w2"}) {
		t.Fatalf("copies = %+v, want d1 (vm:a:zvol:b copies) and w2", f.eng.copies)
	}

	f.rule("vms", "vm:a:zvol:b", store.SkipAll)
	f.eng.copies = nil
	if err := f.svc.ReplicateOffsite(context.Background(), "vms"); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.eng.copies {
		if slices.Contains(c.IDs, "d1") {
			t.Fatalf("d1 was copied although both of its possible owners leave B2 out: %+v", f.eng.copies)
		}
	}
}

func TestAListingRecordsWhatTheTargetHolds(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket:containers")
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"), snap("a2", 200, "container:nginx"))
	f.hold("b2:bucket:containers", copied("b1", "a1", 100, "container:nginx"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	rows, err := f.st.ItemCopiesFor("containers", "container:nginx")
	if err != nil || len(rows) != 1 || rows[0].TargetID != b2.ID || rows[0].SnapshotCount != 2 || rows[0].LatestSnapshotAt != 200 {
		t.Fatalf("ItemCopiesFor = %+v, %v, want 2 at B2 up to 200", rows, err)
	}
	if o, listed, err := f.st.TargetObservationFor("containers", b2.ID); err != nil || !listed || o.ListedAt == 0 {
		t.Fatalf("TargetObservationFor = %+v listed=%v err=%v", o, listed, err)
	}
}

func TestTheSettingsFieldsTargetRecordsNothing(t *testing.T) {
	f := newPlacementFixture(t)
	settings := settingsOf(t, f.svc)
	settings.ContainersOffsite = "b2:bucket:containers"
	if err := f.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("a1", 100, "container:nginx"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	rows, err := f.st.ItemCopiesForDomain("containers")
	if err != nil || len(rows) != 0 {
		t.Fatalf("ItemCopiesForDomain = %+v, %v, want nothing for a target without an id", rows, err)
	}
}

func TestTheSnapshotIndexCountsOnAcrossCopyCalls(t *testing.T) {
	var got []int
	ctx := progress.WithCopySink(context.Background(), func(cp progress.CopyProgress) { got = append(got, cp.SnapshotIndex) })
	progress.CopySinkFrom(withIndexOffset(ctx, 500))(progress.CopyProgress{SnapshotIndex: 3})
	progress.CopySinkFrom(withIndexOffset(ctx, 0))(progress.CopyProgress{SnapshotIndex: 4})
	if !slices.Equal(got, []int{503, 4}) {
		t.Fatalf("indices = %v, want [503 4]", got)
	}
}
