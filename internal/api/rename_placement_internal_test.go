package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// copiedTo lists the snapshot ids the runs so far copied to dest, sorted.
func copiedTo(f *placementFixture, dest string) []string {
	f.t.Helper()
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	var ids []string
	for _, c := range f.eng.copies {
		if c.Dest != dest {
			continue
		}
		if c.IDs == nil {
			f.t.Fatalf("a whole-repository copy from %s to %s", c.Src, dest)
		}
		ids = append(ids, c.IDs...)
	}
	slices.Sort(ids)
	return ids
}

func TestTakeoverAndUnlinkCarryTheCopyRule(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.rule("containers", "container:nginx", store.SkipAll)
	f.dock.installed = map[string]bool{"web": true}
	ctx := context.Background()

	if err := f.svc.TakeOverContainer(ctx, "nginx", "web"); err != nil {
		t.Fatalf("TakeOverContainer: %v", err)
	}
	if skip, ok := ruleOf(t, f, "containers", "container:web"); !ok || !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("rule of web = %v (found %v), want [*]", skip, ok)
	}
	if _, ok := ruleOf(t, f, "containers", "container:nginx"); ok {
		t.Fatal("the former name kept a rule of its own")
	}

	if err := f.svc.UnlinkContainerAlias(ctx, "nginx"); err != nil {
		t.Fatalf("UnlinkContainerAlias: %v", err)
	}
	if skip, ok := ruleOf(t, f, "containers", "container:nginx"); !ok || !slices.Equal(skip, []string{store.SkipAll}) {
		t.Fatalf("rule of nginx after the unlink = %v (found %v), want [*]", skip, ok)
	}
}

func TestATakeoverOntoANameWithOnlyACopyRuleIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.rule("containers", "container:web", store.SkipAll)
	f.dock.installed = map[string]bool{"web": true}

	err := f.svc.TakeOverContainer(context.Background(), "nginx", "web")
	if !errors.Is(err, store.ErrCopyRuleTaken) {
		t.Fatalf("err = %v, want ErrCopyRuleTaken", err)
	}
	if _, err := f.st.GetTargetByContainer("nginx"); err != nil {
		t.Fatalf("the entry left its name after a refused takeover: %v", err)
	}
}

func TestTheContainerTakeoverRoutesRefuseACopyRuleWithItsCode(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.container("db", "")
	f.rule("containers", "container:web", store.SkipAll)
	f.dock.installed = map[string]bool{"web": true, "postgres": true}

	res := f.do(http.MethodPost, "/api/containers/web/takeover", map[string]any{"from": "nginx"})
	if res["ok"] != false || res["code"] != "copy-rule-taken" {
		t.Fatalf("takeover = %v, want the copy-rule-taken refusal", res)
	}

	if err := f.svc.TakeOverContainer(context.Background(), "db", "postgres"); err != nil {
		t.Fatalf("TakeOverContainer: %v", err)
	}
	f.rule("containers", "container:db", store.SkipAll)
	res = f.do(http.MethodDelete, "/api/containers/postgres/alias/db", nil)
	if res["ok"] != false || res["code"] != "copy-rule-taken" {
		t.Fatalf("unlink = %v, want the copy-rule-taken refusal", res)
	}
}

func TestATakeoverKeepsTheRowOfANameWithACopyRule(t *testing.T) {
	f := newPlacementFixture(t)
	f.container("nginx", "")
	f.container("web", "")
	f.rule("containers", "container:web", store.SkipAll)
	f.dock.installed = map[string]bool{"web": true}

	err := f.svc.TakeOverContainer(context.Background(), "nginx", "web")
	if err == nil || !strings.Contains(err.Error(), "copy rule") {
		t.Fatalf("err = %v, want a refusal that names the copy rule", err)
	}
	if _, err := f.st.GetTargetByContainer("web"); err != nil {
		t.Fatalf("the row of web was removed although it has a copy rule: %v", err)
	}
}

func TestAVMTakeoverKeepsTheRowOfANameWithACopyRule(t *testing.T) {
	f := newPlacementFixture(t)
	f.vm("win11", "")
	f.rule("vms", "vm:win11", store.SkipAll)

	err := f.svc.removeEmptyVMRow(context.Background(), "win11")
	if err == nil || !strings.Contains(err.Error(), "copy rule") {
		t.Fatalf("err = %v, want a refusal that names the copy rule", err)
	}
	if _, err := f.st.GetVMTargetByName("win11"); err != nil {
		t.Fatalf("the row of win11 was removed although it has a copy rule: %v", err)
	}
}

func TestADeletedEntryKeepsTheHistoryItTookOverOutOfTheTarget(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", "b2:bucket/containers")
	f.listing("containers", b2.ID, 50)
	f.container("nginx", "")
	f.container("db", "")
	f.rule("containers", "container:nginx", store.SkipAll)
	if err := f.st.RenameTargetWithAlias("nginx", "web", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.st.DeleteTarget("web"); err != nil {
		t.Fatal(err)
	}
	f.hold(f.domainPath("containers"),
		snap("aaaa0001", 100, "container:nginx"),
		snap("aaaa0002", 300, "container:web"),
		snap("aaaa0003", 300, "container:db"))

	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatalf("ReplicateOffsite: %v", err)
	}
	if got, want := copiedTo(f, b2.Repo), []string{"aaaa0003"}; !slices.Equal(got, want) {
		t.Fatalf("copied %v to B2, want %v", got, want)
	}
}

// takenOver is newName, formerly oldName and linked at 200, beside a new entry
// that carries oldName today, with B2 as the domain's only target.
func takenOver(t *testing.T, domain, oldName, newName string) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	b2 := f.target(domain, "B2", "b2:bucket/"+domain)
	f.listing(domain, b2.ID, 50)
	aliasDomain, id := "container", ""
	if domain == "vms" {
		aliasDomain = "vm"
		id = f.vm(newName, "").ID
		f.vm(oldName, "")
	} else {
		id = f.container(newName, "").ID
		f.container(oldName, "")
	}
	if _, err := f.st.AddAliasAt(aliasDomain, oldName, id, 200); err != nil {
		t.Fatal(err)
	}
	return f, b2
}

func TestAFormerNameBelongsToTheRenamedEntryUntilTheLink(t *testing.T) {
	f, _ := takenOver(t, "containers", "nginx", "web")
	c, err := f.svc.ownerContextFor("containers")
	if err != nil {
		t.Fatal(err)
	}
	got := c.owners([]restic.Snapshot{
		snap("aaaa0001", 100, "container:nginx"),
		snap("aaaa0002", 300, "container:nginx"),
		{ID: "aaaa0003", Time: "not a time", Tags: []string{"container:nginx"}},
	})
	for id, want := range map[string]snapshotOwner{
		"aaaa0001": {Possible: []string{"container:web"}, Owner: "container:web"},
		"aaaa0002": {Possible: []string{"container:nginx"}, Owner: "container:nginx"},
		"aaaa0003": {Possible: []string{"container:nginx", "container:web"}},
	} {
		if o := got[id]; o.Owner != want.Owner || !slices.Equal(o.Possible, want.Possible) {
			t.Errorf("%s: owner %+v, want %+v", id, o, want)
		}
	}
}

func TestAFormerNamesDiskSnapshotFollowsItsVM(t *testing.T) {
	f, _ := takenOver(t, "vms", "windows-11", "win11")
	c, err := f.svc.ownerContextFor("vms")
	if err != nil {
		t.Fatal(err)
	}
	got := c.owners([]restic.Snapshot{
		snap("bbbb0001", 100, "vm:windows-11", "vmrun:r1"),
		snap("bbbb0002", 100, "vm:windows-11:zvol:sda", "vmrun:r1"),
		snap("bbbb0003", 300, "vm:windows-11", "vmrun:r2"),
		snap("bbbb0004", 300, "vm:windows-11:zvol:sda", "vmrun:r2"),
	})
	for id, want := range map[string]string{
		"bbbb0001": "vm:win11",
		"bbbb0002": "vm:win11",
		"bbbb0003": "vm:windows-11",
		"bbbb0004": "vm:windows-11",
	} {
		if o := got[id]; o.Owner != want {
			t.Errorf("%s: owner %+v, want %s", id, o, want)
		}
	}
}

func TestUnreadableAliasesLeaveEveryEntryAPossibleOwner(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	for _, name := range []string{"db", "nginx", "web"} {
		if _, err := st.UpsertTarget(store.Target{ContainerName: name}); err != nil {
			t.Fatal(err)
		}
	}
	b2, err := st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "B2", Repo: "b2:bucket/containers", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"container:nginx", "container:web"} {
		if err := st.SetCopyRule("containers", id, []string{store.SkipAll}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("DROP TABLE target_aliases"); err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st}

	c, err := s.ownerContextFor("containers")
	if err != nil {
		t.Fatalf("an unreadable alias table stopped the run: %v", err)
	}
	o := c.owners([]restic.Snapshot{snap("aaaa0001", 100, "container:nginx")})["aaaa0001"]
	if want := []string{"container:db", "container:nginx", "container:web"}; o.Owner != "" || !slices.Equal(o.Possible, want) {
		t.Fatalf("owner = %+v, want every entry possible and none settled", o)
	}

	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.readPlacement(settings, "containers")
	if err != nil {
		t.Fatal(err)
	}
	if !p.copiesTo(b2.ID, o.Possible) {
		t.Fatal("db follows the default and may own the snapshot, so it must be copied")
	}
	if err := st.SetCopyRule("containers", "container:db", []string{store.SkipAll}); err != nil {
		t.Fatal(err)
	}
	if p, err = s.readPlacement(settings, "containers"); err != nil {
		t.Fatal(err)
	}
	if p.copiesTo(b2.ID, o.Possible) {
		t.Fatal("every possible owner leaves B2 out, so the snapshot stays out")
	}
}

func TestTakenOverContainerHistoryFollowsTheRuleOfItsOwner(t *testing.T) {
	for _, c := range []struct {
		name  string
		local string
		want  []string
	}{
		{"new nginx on local beside web", "container:nginx", []string{"aaaa0001", "aaaa0003"}},
		{"web on local beside a live nginx", "container:web", []string{"aaaa0002"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, b2 := takenOver(t, "containers", "nginx", "web")
			f.rule("containers", c.local, store.SkipAll)
			f.hold(f.domainPath("containers"),
				snap("aaaa0001", 100, "container:nginx"),
				snap("aaaa0002", 300, "container:nginx"),
				snap("aaaa0003", 400, "container:web"))

			if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
				t.Fatalf("ReplicateOffsite: %v", err)
			}
			if got := copiedTo(f, b2.Repo); !slices.Equal(got, c.want) {
				t.Fatalf("copied %v to B2, want %v", got, c.want)
			}
		})
	}
}

func TestTakenOverVMHistoryFollowsTheRuleOfItsOwner(t *testing.T) {
	for _, c := range []struct {
		name  string
		local string
		want  []string
	}{
		{"new windows-11 on local beside win11", "vm:windows-11", []string{"bbbb0001", "bbbb0002", "bbbb0005"}},
		{"win11 on local beside a live windows-11", "vm:win11", []string{"bbbb0003", "bbbb0004"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, b2 := takenOver(t, "vms", "windows-11", "win11")
			f.rule("vms", c.local, store.SkipAll)
			f.hold(f.domainPath("vms"),
				snap("bbbb0001", 100, "vm:windows-11", "vmrun:r1"),
				snap("bbbb0002", 100, "vm:windows-11:zvol:sda", "vmrun:r1"),
				snap("bbbb0003", 300, "vm:windows-11", "vmrun:r2"),
				snap("bbbb0004", 300, "vm:windows-11:zvol:sda", "vmrun:r2"),
				snap("bbbb0005", 400, "vm:win11", "vmrun:r3"))

			if err := f.svc.ReplicateOffsite(context.Background(), "vms"); err != nil {
				t.Fatalf("ReplicateOffsite: %v", err)
			}
			if got := copiedTo(f, b2.Repo); !slices.Equal(got, c.want) {
				t.Fatalf("copied %v to B2, want %v", got, c.want)
			}
		})
	}
}

func TestASnapshotWithoutAReadableTimeStaysOutOnlyWhenBothPossibleOwnersExclude(t *testing.T) {
	undated := restic.Snapshot{ID: "aaaa0009", Time: "not a time", Tags: []string{"container:nginx"}}
	for _, c := range []struct {
		name  string
		local []string
		want  []string
	}{
		{"the renamed entry still copies", []string{"container:nginx"}, []string{"aaaa0009"}},
		{"both possible owners on local", []string{"container:nginx", "container:web"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, b2 := takenOver(t, "containers", "nginx", "web")
			for _, id := range c.local {
				f.rule("containers", id, store.SkipAll)
			}
			f.hold(f.domainPath("containers"), undated)

			if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
				t.Fatalf("ReplicateOffsite: %v", err)
			}
			if got := copiedTo(f, b2.Repo); !slices.Equal(got, c.want) {
				t.Fatalf("copied %v to B2, want %v", got, c.want)
			}
		})
	}
}
