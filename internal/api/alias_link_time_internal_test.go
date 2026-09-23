package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// linkedAt2024 is the linked_at every alias in this file carries:
// 2024-06-01T00:00:00Z.
var linkedAt2024 = time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC).Unix()

func newAliasLinkStore(t *testing.T) *store.Repo {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db)
}

// "latest" is the newest snapshot the entry owns. A snapshot under the old
// name taken after the link belongs to whichever machine took the name up, so
// it must neither win "latest" nor pass the ownership check as an explicit
// id.
func TestPrepareRestoreLatestSkipsPostLinkAliasSnapshot(t *testing.T) {
	st := newAliasLinkStore(t)
	const appdata = "/host/user/appdata/radarr"
	def, err := json.Marshal(containerDefinition{Inspect: model.Inspect{Name: "radarr"}})
	if err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr", Definition: string(def), AppdataPaths: []string{appdata}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("container", "radarr-movies", tg.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	// restic lists oldest first; the post-link snapshot is last.
	pre := restic.Snapshot{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}, Paths: []string{appdata}}
	own := restic.Snapshot{ID: "bbbb2222", Time: "2024-07-01T00:00:00Z", Tags: []string{"container:radarr", "p1"}, Paths: []string{appdata}}
	post := restic.Snapshot{ID: "cccc3333", Time: "2024-09-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}, Paths: []string{appdata}}
	s := &Service{
		store:  st,
		cfg:    config.Config{HostMountRoot: "/host/user"},
		engine: aliasRestoreEngine{snaps: []restic.Snapshot{pre, own, post}},
	}
	ref := repoRef{repo: "rest:http://fake/containers", mode: restic.Mode{}}
	ctx := context.Background()

	plan, err := s.prepareRestoreIn(ctx, ref, "radarr", "latest", true)
	if err != nil {
		t.Fatalf("prepareRestoreIn latest: %v", err)
	}
	if plan.snapshotID != own.ID {
		t.Fatalf("latest = %s, want the newest owned snapshot %s", plan.snapshotID, own.ID)
	}
	if _, err := s.prepareRestoreIn(ctx, ref, "radarr", post.ID, true); err == nil {
		t.Fatal("a post-link snapshot under the old name must not pass the ownership check")
	}
	if plan, err := s.prepareRestoreIn(ctx, ref, "radarr", pre.ID, true); err != nil || plan.snapshotID != pre.ID {
		t.Fatalf("the pre-link snapshot is the entry's own: plan %+v, err %v", plan, err)
	}
}

// TestPrepareRestoreVMLatestSkipsPostLinkAliasSnapshot is the VM twin.
func TestPrepareRestoreVMLatestSkipsPostLinkAliasSnapshot(t *testing.T) {
	st := newAliasLinkStore(t)
	const diskPath = "/host/user/domains/win11/vdisk1.img"
	def, err := json.Marshal(vmDefinition{DomainXML: "<domain/>", DiskPaths: []string{diskPath}})
	if err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Definition: string(def)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-11", tg.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	pre := restic.Snapshot{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}, Paths: []string{diskPath}}
	own := restic.Snapshot{ID: "bbbb2222", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11", "p2"}, Paths: []string{diskPath}}
	post := restic.Snapshot{ID: "cccc3333", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}, Paths: []string{diskPath}}
	s := &Service{
		store:  st,
		cfg:    config.Config{HostMountRoot: "/host/user"},
		engine: aliasRestoreEngine{snaps: []restic.Snapshot{pre, own, post}},
	}
	ref := repoRef{repo: "rest:http://fake/vms", mode: restic.Mode{}}
	ctx := context.Background()

	plan, err := s.prepareRestoreVMIn(ctx, ref, "win11", "latest", true)
	if err != nil {
		t.Fatalf("prepareRestoreVMIn latest: %v", err)
	}
	if plan.snapshotID != own.ID {
		t.Fatalf("latest = %s, want the newest owned snapshot %s", plan.snapshotID, own.ID)
	}
	if _, err := s.prepareRestoreVMIn(ctx, ref, "win11", post.ID, true); err == nil {
		t.Fatal("a post-link snapshot under the old name must not pass the ownership check")
	}
}

// retentionListEngine counts the listings retention makes and records the
// tag set of every forget.
type retentionListEngine struct {
	ResticEngine // nil: any other call panics
	snaps        []restic.Snapshot
	listCalls    int
	forgetTags   [][]string
}

func (e *retentionListEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	e.listCalls++
	return e.snaps, nil
}

func (e *retentionListEngine) ForgetPolicy(_ context.Context, _ string, _ restic.RetentionPolicy, _ restic.Mode, tags []string, _ bool) error {
	e.forgetTags = append(e.forgetTags, tags)
	return nil
}

func (e *retentionListEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

// Deciding which alias tags may join a forget costs one listing, and an entry
// that was never renamed must not pay it, in either domain.
func TestApplyRetentionListsOnlyForAnEntryWithAliases(t *testing.T) {
	st := newAliasLinkStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.RetentionKeepLast = 3
	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"}); err != nil {
		t.Fatal(err)
	}
	renamed, err := st.UpsertVMTarget(store.VMTarget{Name: "win10"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "windows-10", renamed.ID, linkedAt2024); err != nil {
		t.Fatal(err)
	}
	eng := &retentionListEngine{}
	s := &Service{store: st, engine: eng}
	ctx := context.Background()

	s.applyRetention(ctx, "rest:http://fake/containers", settings, restic.Mode{}, s.containerIdentity("plex"), "containers")
	s.applyRetention(ctx, "rest:http://fake/vms", settings, restic.Mode{}, s.vmIdentity("win11"), "vms")
	if eng.listCalls != 0 {
		t.Fatalf("an entry without aliases must not list snapshots for retention, got %d listings", eng.listCalls)
	}
	s.applyRetention(ctx, "rest:http://fake/vms", settings, restic.Mode{}, s.vmIdentity("win10"), "vms")
	if eng.listCalls != 1 {
		t.Fatalf("an entry with aliases lists exactly once, got %d listings", eng.listCalls)
	}
	if len(eng.forgetTags) != 3 || len(eng.forgetTags[0]) != 1 || eng.forgetTags[0][0] != "container:plex" ||
		len(eng.forgetTags[1]) != 1 || eng.forgetTags[1][0] != "vm:win11" {
		t.Fatalf("forget tag sets = %v", eng.forgetTags)
	}
}
