package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

const (
	firstRun  = "bombvault-20260101000000"
	secondRun = "bombvault-20260102000000"
)

// backupOfEveryKind holds one backup of a container, a VM, a folder set, the
// flash drive, the configuration and a ZFS entry, and returns the route of
// each entry with the location its backup lies at.
func backupOfEveryKind(f *placementFixture) map[string]string {
	f.t.Helper()
	docs := f.fileSet("Docs", "")
	tank, zfsRepo := f.zfsItem("tank/apps")
	items := map[string]string{
		"containers/nginx": f.domainPath("containers"),
		"vms/win11":        f.domainPath("vms"),
		"files/" + docs.ID: f.domainPath("files"),
		"flash/flash":      f.singletonRepo("flash"),
		"config/config":    f.singletonRepo("config"),
		"zfs/" + tank.ID:   zfsRepo,
	}
	f.hold(items["containers/nginx"], snap("a1a1a1a1", 100, "container:nginx", "p1"))
	f.hold(items["vms/win11"], snap("a1a1a1a1", 100, "vm:win11", "p2"))
	f.hold(items["files/"+docs.ID], snap("a1a1a1a1", 100, "fileset:Docs"))
	f.hold(items["flash/flash"], snap("a1a1a1a1", 100))
	f.hold(items["config/config"], snap("a1a1a1a1", 100))
	f.hold(zfsRepo, zfsSnap("a1a1a1a1", "tank/apps", firstRun))
	return items
}

func TestATagMarksABackupOfEveryKind(t *testing.T) {
	f := newPlacementFixture(t)
	for item, repo := range backupOfEveryKind(f) {
		res := f.do(http.MethodPost, "/api/items/"+item+"/tag", map[string]any{
			"snapshotId": "a1a1a1a1", "tags": []string{"before upgrade"},
		})
		if res["ok"] != true {
			t.Fatalf("%s: tag = %v", item, res)
		}
		held, _ := f.eng.Snapshots(context.Background(), repo, restic.Mode{})
		if !slices.Contains(held[0].Tags, "before upgrade") {
			t.Errorf("%s: the backup carries %v, want the new tag", item, held[0].Tags)
		}
	}
}

func TestATagRefusesWhatBombVaultReadsAMeaningFrom(t *testing.T) {
	f := newPlacementFixture(t)
	f.hold(f.domainPath("vms"), snap("a1a1a1a1", 100, "vm:win11", "p2", "blocks"))
	ctx := context.Background()

	for _, tag := range []string{"blocks", "imported", "p2", "vm:win11", "bv:direct", "import:plex"} {
		if err := f.svc.TagBackup(ctx, "vms", "win11", "local", "a1a1a1a1", nil, []string{tag}); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Errorf("removing %q = %v, want a refusal", tag, err)
		}
		if err := f.svc.TagBackup(ctx, "vms", "win11", "local", "a1a1a1a1", []string{tag}, nil); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Errorf("adding %q = %v, want a refusal", tag, err)
		}
	}
	if len(f.eng.tags) != 0 {
		t.Fatalf("a refused tag reached restic: %+v", f.eng.tags)
	}
}

func TestATagOnAZFSRunMarksEveryDatasetOfIt(t *testing.T) {
	f := newPlacementFixture(t)
	tank, repo := f.zfsItem("tank/apps")
	f.hold(repo,
		zfsSnap("a1a1a1a1", "tank/apps", firstRun),
		zfsSnap("a2a2a2a2", "tank/apps/db", firstRun),
		zfsSnap("b1b1b1b1", "tank/apps", secondRun),
		zfsSnap("c1c1c1c1", "tank/other", firstRun),
	)
	if err := f.svc.TagBackup(context.Background(), zfsDomain, tank.ID, "local", "a2a2a2a2", []string{"keep"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.eng.tags) != 1 || !slices.Equal(f.eng.tags[0].IDs, []string{"a1a1a1a1", "a2a2a2a2"}) {
		t.Fatalf("tagged %+v, want the two datasets of the first run in one call", f.eng.tags)
	}
}

func TestATagStaysWithinTheEntry(t *testing.T) {
	f := newResticFixture(t)
	f.write("data", map[string]string{"a.txt": "one"})
	f.backup("vms", "data", "vm:win11", "p2")
	other := f.backup("vms", "data", "vm:debian", "p2")

	res := f.do(http.MethodPost, "/api/items/vms/win11/tag", map[string]any{
		"snapshotId": other.ID, "tags": []string{"mine"},
	})
	if res["ok"] != false || !strings.Contains(res["error"].(string), "does not belong to this VM") {
		t.Fatalf("tagging another VM's backup = %v", res)
	}
	if got := f.snapshot("vms", other.ID); slices.Contains(got.Tags, "mine") {
		t.Fatalf("the other VM's backup was tagged: %v", got.Tags)
	}
}

func TestATagIsReplacedInOneRewrite(t *testing.T) {
	f := newResticFixture(t)
	f.write("data", map[string]string{"a.txt": "one"})
	snap := f.backup("flash", "data")
	ctx := context.Background()

	if err := f.svc.TagBackup(ctx, "flash", "flash", "local", snap.ID, []string{"draft"}, nil); err != nil {
		t.Fatal(err)
	}
	tagged := f.snapshot("flash", snap.ID)
	if tagged.Original != snap.ID || !slices.Equal(tagged.Tags, []string{"draft"}) {
		t.Fatalf("the tagged backup is %+v, want the tag and the first id as its original", tagged)
	}
	if err := f.svc.TagBackup(ctx, "flash", "flash", "local", tagged.ID, []string{"final"}, []string{"draft"}); err != nil {
		t.Fatal(err)
	}
	if got := f.snapshot("flash", snap.ID); !slices.Equal(got.Tags, []string{"final"}) {
		t.Fatalf("tags = %v, want only the new one", got.Tags)
	}
}

func TestATagOnARealZFSRunFindsItsDatasetsByTheirStamp(t *testing.T) {
	skipWithoutSnapshotStamps(t)
	f := newResticFixture(t)
	d := f.zfsEntry("tank/apps")
	root := f.zfsBackup("tank/apps", firstRun, map[string]string{"a.txt": "one"})
	child := f.zfsBackup("tank/apps/db", firstRun, map[string]string{"b.txt": "two"})
	later := f.zfsBackup("tank/apps", secondRun, map[string]string{"a.txt": "three"})

	res := f.do(http.MethodPost, "/api/items/zfs/"+d.ID+"/tag", map[string]any{
		"snapshotId": child.ID, "tags": []string{"keep"},
	})
	if res["ok"] != true {
		t.Fatalf("tag = %v", res)
	}
	for _, snap := range []restic.Snapshot{root, child} {
		if got := f.snapshot(zfsDomain, snap.ID); !slices.Contains(got.Tags, "keep") {
			t.Errorf("the snapshot of %v carries %v, want the tag of its run", snap.Tags, got.Tags)
		}
	}
	if got := f.snapshot(zfsDomain, later.ID); slices.Contains(got.Tags, "keep") {
		t.Errorf("the next run was tagged as well: %v", got.Tags)
	}
}
