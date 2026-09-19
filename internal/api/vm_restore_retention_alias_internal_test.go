package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestPrepareRestoreVMFollowsAlias is the VM twin of
// TestPrepareRestoreFollowsAlias: the restore lookup has to accept the
// identity SnapshotsVM lists, or the picker offers a snapshot that restore
// then refuses. The alias-tagged snapshot predates the link, so the alias
// claims it.
func TestPrepareRestoreVMFollowsAlias(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)

	// The disk path has to lie under HostMountRoot.
	const diskPath = "/host/user/domains/win11/vdisk1.img"
	def, err := json.Marshal(vmDefinition{DomainXML: "<domain/>", DiskPaths: []string{diskPath}})
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Definition: string(def)})
	if err != nil {
		t.Fatalf("upsert vm target: %v", err)
	}
	if _, err := st.AddAlias("vm", "windows-11", tg.ID); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// onlyAlias carries only the retired name's tag, as a snapshot written
	// before the rename does. other belongs to a different VM and must not be
	// reachable through "win11".
	onlyAlias := restic.Snapshot{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}, Paths: []string{diskPath}}
	other := restic.Snapshot{ID: "cccc3333", Tags: []string{"vm:ubuntu", "p2"}, Paths: []string{diskPath}}

	s := &Service{
		store:  st,
		cfg:    config.Config{HostMountRoot: "/host/user"},
		engine: aliasRestoreEngine{snaps: []restic.Snapshot{onlyAlias, other}},
	}
	ref := repoRef{repo: "rest:http://fake/vms", mode: restic.Mode{}}
	ctx := context.Background()

	t.Run("explicit alias-tagged snapshot resolves for the renamed entry", func(t *testing.T) {
		plan, err := s.prepareRestoreVMIn(ctx, ref, "win11", onlyAlias.ID, true)
		if err != nil {
			t.Fatalf("prepareRestoreVMIn: %v", err)
		}
		if plan.snapshotID != onlyAlias.ID {
			t.Fatalf("expected the plan to target the alias-tagged snapshot, got %+v", plan)
		}
	})

	t.Run("restore latest resolves to the alias-tagged snapshot when it is the only history", func(t *testing.T) {
		// Right after a rename, before the first backup under the new name,
		// every snapshot of this VM carries the old tag, so "latest" has to
		// find one of those.
		plan, err := s.prepareRestoreVMIn(ctx, ref, "win11", "latest", true)
		if err != nil {
			t.Fatalf("prepareRestoreVMIn latest: %v", err)
		}
		if plan.snapshotID != onlyAlias.ID {
			t.Fatalf("expected latest to resolve to the alias-tagged snapshot, got %+v", plan)
		}
	})

	t.Run("a different vm's snapshot is still refused", func(t *testing.T) {
		_, err := s.prepareRestoreVMIn(ctx, ref, "win11", other.ID, true)
		if err == nil {
			t.Fatal("expected refusal for a snapshot belonging to a different vm")
		}
	})
}

// TestPrepareRestoreVMForTargetForeignRestoreIgnoresLocalAlias is the VM twin
// of TestPrepareRestoreForTargetForeignRestoreIgnoresLocalAlias: a foreign
// restore takes only the item's own tag from its caller, so a same-named
// local entry's alias history cannot widen the ownership check.
func TestPrepareRestoreVMForTargetForeignRestoreIgnoresLocalAlias(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)

	// A local VM is also named "win11", as during a migration where the same
	// item exists on both instances, and was itself renamed from
	// "windows-11", leaving that alias behind.
	localTg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatalf("upsert local vm target: %v", err)
	}
	if _, err := st.AddAlias("vm", "windows-11", localTg.ID); err != nil {
		t.Fatalf("add local alias: %v", err)
	}

	const diskPath = "/host/user/domains/win11/vdisk1.img"
	def, err := json.Marshal(vmDefinition{DomainXML: "<domain/>", DiskPaths: []string{diskPath}})
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	// Built in memory from the decrypted foreign definition and not persisted
	// before prepareRestoreVMForTarget validates it, as foreignVMTarget hands
	// it to prepareForeignRestore.
	foreignTg := store.VMTarget{Name: "win11", Definition: string(def)}

	// The foreign repository holds a snapshot whose tag only happens to match
	// the local alias's old name. It is older than the local link, so the
	// local identity would claim it, and it must not be mistaken for this
	// item's history.
	collides := restic.Snapshot{ID: "dddd4444", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}, Paths: []string{diskPath}}
	own := restic.Snapshot{ID: "eeee5555", Tags: []string{"vm:win11", "p2"}, Paths: []string{diskPath}}

	s := &Service{
		store:  st,
		cfg:    config.Config{HostMountRoot: "/host/user"},
		engine: aliasRestoreEngine{snaps: []restic.Snapshot{collides, own}},
	}
	ref := repoRef{repo: "rest:http://fake/foreign-session", mode: restic.Mode{}}
	ctx := context.Background()
	// The identity prepareForeignRestore builds: the item's own tag, no local
	// alias history.
	id := tagIdentity("vm:win11")

	t.Run("a snapshot tagged with a local alias is refused", func(t *testing.T) {
		_, err := s.prepareRestoreVMForTarget(ctx, ref, "win11", collides.ID, foreignTg, id, "", "")
		if err == nil {
			t.Fatal("expected refusal: a local alias must not widen a foreign restore's accepted tags")
		}
	})

	t.Run("the item's own tag still restores normally", func(t *testing.T) {
		plan, err := s.prepareRestoreVMForTarget(ctx, ref, "win11", own.ID, foreignTg, id, "", "")
		if err != nil {
			t.Fatalf("prepareRestoreVMForTarget: %v", err)
		}
		if plan.snapshotID != own.ID {
			t.Fatalf("expected the plan to target the item's own snapshot, got %+v", plan)
		}
	})
}
