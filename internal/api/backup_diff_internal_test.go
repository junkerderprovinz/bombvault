package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestACompareRunsOnTheTwoBackupsOfEveryKind(t *testing.T) {
	f := newPlacementFixture(t)
	for item, repo := range backupOfEveryKind(f) {
		held, _ := f.eng.Snapshots(context.Background(), repo, restic.Mode{})
		later := held[0]
		later.ID = "b1b1b1b1"
		if strings.HasPrefix(item, "zfs/") {
			later = zfsSnap("b1b1b1b1", "tank/apps", secondRun)
		}
		f.hold(repo, held[0], later)
		f.eng.diffs = nil
		f.eng.diffChanges = map[string][]restic.DiffChange{"a1a1a1a1->b1b1b1b1": {{Path: "/etc/app.conf", Modifier: "M"}}}

		res := f.do(http.MethodGet, "/api/items/"+item+"/diff?from=a1a1a1a1&to=b1b1b1b1", nil)
		diff, _ := res["diff"].(map[string]any)
		if res["ok"] != true || diff["changedFiles"] != float64(1) || diff["addedBytes"] != float64(10) {
			t.Fatalf("%s: diff = %v", item, res)
		}
		change := diff["changes"].([]any)[0].(map[string]any)
		if change["path"] != "/etc/app.conf" || change["modifier"] != "M" {
			t.Errorf("%s: change = %v", item, change)
		}
		if !slices.Equal(f.eng.diffs, []string{"a1a1a1a1->b1b1b1b1"}) {
			t.Errorf("%s: restic compared %v", item, f.eng.diffs)
		}
		if got := diff["disks"] == true; got != strings.HasPrefix(item, "vms/") {
			t.Errorf("%s: disks = %v", item, diff["disks"])
		}
	}
}

func TestACompareOfZFSRunsGoesDatasetByDataset(t *testing.T) {
	f := newPlacementFixture(t)
	tank, repo := f.zfsItem("tank/apps")
	f.hold(repo,
		zfsSnap("a1a1a1a1", "tank/apps", firstRun),
		zfsSnap("a2a2a2a2", "tank/apps/db", firstRun),
		zfsSnap("a3a3a3a3", "tank/apps/cache", firstRun),
		zfsSnap("b1b1b1b1", "tank/apps", secondRun),
		zfsSnap("b2b2b2b2", "tank/apps/db", secondRun),
		zfsSnap("b4b4b4b4", "tank/apps/logs", secondRun),
	)
	f.eng.diffChanges = map[string][]restic.DiffChange{
		"a1a1a1a1->b1b1b1b1": {{Path: "/compose.yml", Modifier: "M"}},
		"a2a2a2a2->b2b2b2b2": {{Path: "/base/1", Modifier: "+"}, {Path: "/base/2", Modifier: "-"}},
	}
	// Any dataset of a run names the run.
	diff, err := f.svc.DiffBackups(context.Background(), zfsDomain, tank.ID, "local", "a2a2a2a2", "b4b4b4b4")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.eng.diffs, []string{"a1a1a1a1->b1b1b1b1", "a2a2a2a2->b2b2b2b2"}) {
		t.Fatalf("restic compared %v, want each dataset with itself", f.eng.diffs)
	}
	if diff.ChangedFiles != 3 || diff.AddedBytes != 30 {
		t.Errorf("totals = %+v, want the sum over both datasets", diff.DiffResult)
	}
	if len(diff.Changes) != 3 || diff.Changes[0].Part != "tank/apps" || diff.Changes[1].Part != "tank/apps/db" {
		t.Errorf("changes = %+v, want each under its dataset", diff.Changes)
	}
	if !slices.Equal(diff.OnlyBefore, []string{"tank/apps/cache"}) || !slices.Equal(diff.OnlyAfter, []string{"tank/apps/logs"}) {
		t.Errorf("only before %v, only after %v", diff.OnlyBefore, diff.OnlyAfter)
	}
}

func TestACompareOfAVMShowsDisksNotSegments(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("vms"),
		snap("a1a1a1a1", 100, "vm:win11", "p2", "blocks", "vmrun:r1"),
		snap("a2a2a2a2", 100, "vm:win11:zvol:sda", "p2", "vmrun:r1"),
		snap("b1b1b1b1", 200, "vm:win11", "p2", "blocks", "vmrun:r2"),
		snap("b2b2b2b2", 200, "vm:win11:zvol:sda", "p2", "vmrun:r2"),
	)
	f.eng.diffChanges = map[string][]restic.DiffChange{
		"a1a1a1a1->b1b1b1b1": {
			{Path: "/vda/00000001", Modifier: "M"}, {Path: "/vda/00000002", Modifier: "M"},
			{Path: "/vdb/00000007", Modifier: "M"}, {Path: "/bombvault-vm.json", Modifier: "M"},
		},
		"a2a2a2a2->b2b2b2b2": {{Path: "/vm/win11/sda", Modifier: "M"}},
	}
	diff, err := f.svc.DiffBackups(context.Background(), "vms", "win11", "local", "a1a1a1a1", "b1b1b1b1")
	if err != nil {
		t.Fatal(err)
	}
	want := []BackupChange{
		{Path: "/vda/", Modifier: "M"}, {Path: "/vdb/", Modifier: "M"}, {Path: "/bombvault-vm.json", Modifier: "M"},
		{Part: "sda", Path: "/vm/win11/sda", Modifier: "M"},
	}
	if !diff.Disks || !slices.Equal(diff.Changes, want) {
		t.Fatalf("diff = %+v, want one line per disk and the zvol disk of the same run", diff)
	}
	if diff.AddedBytes != 50 {
		t.Errorf("added bytes = %d, want the data of every segment and of the zvol disk", diff.AddedBytes)
	}
}

func TestACompareListsWhatWasAddedChangedAndRemoved(t *testing.T) {
	f := newResticFixture(t)
	set, err := f.st.CreateFileSet(store.FileSet{Name: "Docs", Path: "files/Docs", Enabled: true, RepoChosen: store.RepoChosen})
	if err != nil {
		t.Fatal(err)
	}
	f.write("docs", map[string]string{"keep.txt": "same", "edit.txt": "one", "gone.txt": "bye"})
	first := f.backup("files", "docs", "fileset:Docs")
	f.write("docs", map[string]string{"edit.txt": "two, and longer", "new.txt": "hello"})
	f.remove("docs", "gone.txt")
	second := f.backup("files", "docs", "fileset:Docs")

	res := f.do(http.MethodGet, "/api/items/files/"+set.ID+"/diff?from="+first.ID+"&to="+second.ID, nil)
	diff, _ := res["diff"].(map[string]any)
	if res["ok"] != true || diff["addedFiles"] != float64(1) || diff["removedFiles"] != float64(1) || diff["changedFiles"] != float64(1) {
		t.Fatalf("diff = %v", res)
	}
	if diff["addedBytes"] == float64(0) || diff["removedBytes"] == float64(0) {
		t.Errorf("bytes = %v added, %v removed, want both above zero", diff["addedBytes"], diff["removedBytes"])
	}
	got := map[string]string{}
	for _, c := range diff["changes"].([]any) {
		change := c.(map[string]any)
		got[change["path"].(string)] = change["modifier"].(string)
	}
	if got["/new.txt"] != "+" || got["/gone.txt"] != "-" || !strings.Contains(got["/edit.txt"], "M") || got["/keep.txt"] != "" {
		t.Fatalf("changes = %v", got)
	}
}
