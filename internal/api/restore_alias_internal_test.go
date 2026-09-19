package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// aliasRestoreEngine returns a fixed snapshot list for any repository and
// mode. These tests stop at the prepare phase, so any other call hits the nil
// embed and panics.
type aliasRestoreEngine struct {
	ResticEngine
	snaps []restic.Snapshot
}

func (e aliasRestoreEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return e.snaps, nil
}

// Snapshots lists a renamed container's alias-tagged history next to its
// current-name one, so the restore ownership check has to accept the same
// identity, or the picker offers a snapshot that restore then refuses.
// AddAlias links at the current time, so the alias claims the older snapshot.
// It is an internal test because the plan's snapshotID is unexported.
//
// The "rest:" location keeps localRepoMissing away from the disk, and
// HostMountRoot and AppdataPaths are POSIX strings because paths.Within needs
// a leading "/", which a Windows temp path lacks.
func TestPrepareRestoreFollowsAlias(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)

	const appdata = "/host/user/appdata/radarr"
	def, err := json.Marshal(containerDefinition{Inspect: model.Inspect{Name: "radarr"}})
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "radarr", Definition: string(def), AppdataPaths: []string{appdata}})
	if err != nil {
		t.Fatalf("upsert target: %v", err)
	}
	if _, err := st.AddAlias("container", "radarr-movies", tg.ID); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// onlyAlias carries only the retired name's tag, as a snapshot written
	// before the rename does, since nothing in the repository is rewritten.
	// other belongs to a different container and must not be reachable
	// through "radarr".
	onlyAlias := restic.Snapshot{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:radarr-movies", "p1"}, Paths: []string{appdata}}
	other := restic.Snapshot{ID: "cccc3333", Tags: []string{"container:sonarr", "p1"}, Paths: []string{appdata}}

	s := &Service{
		store:  st,
		cfg:    config.Config{HostMountRoot: "/host/user"},
		engine: aliasRestoreEngine{snaps: []restic.Snapshot{onlyAlias, other}},
	}
	ref := repoRef{repo: "rest:http://fake/containers", mode: restic.Mode{}}
	ctx := context.Background()

	t.Run("explicit alias-tagged snapshot resolves for the renamed entry", func(t *testing.T) {
		plan, err := s.prepareRestoreIn(ctx, ref, "radarr", onlyAlias.ID, true)
		if err != nil {
			t.Fatalf("prepareRestoreIn: %v", err)
		}
		if plan.snapshotID != onlyAlias.ID {
			t.Fatalf("expected the plan to target the alias-tagged snapshot, got %+v", plan)
		}
	})

	t.Run("restore latest resolves to the alias-tagged snapshot when it is the only history", func(t *testing.T) {
		// Right after a rename, before the first backup under the new name,
		// every snapshot of this container carries the old tag, so "latest"
		// has to find one of those.
		plan, err := s.prepareRestoreIn(ctx, ref, "radarr", "latest", true)
		if err != nil {
			t.Fatalf("prepareRestoreIn latest: %v", err)
		}
		if plan.snapshotID != onlyAlias.ID {
			t.Fatalf("expected latest to resolve to the alias-tagged snapshot, got %+v", plan)
		}
	})

	t.Run("a different container's snapshot is still refused", func(t *testing.T) {
		_, err := s.prepareRestoreIn(ctx, ref, "radarr", other.ID, true)
		if err == nil {
			t.Fatal("expected refusal for a snapshot belonging to a different container")
		}
	})
}

// A foreign restore takes its identity from the caller, which passes only the
// item's own tag. A same-named local entry's alias history says nothing about
// someone else's repository and must not widen the ownership check.
// TestPrepareRestoreFollowsAlias is the same-instance counterpart, where the
// identity does include the aliases.
func TestPrepareRestoreForTargetForeignRestoreIgnoresLocalAlias(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)

	// A local container is also named "radarr", as during a migration where
	// the same item exists on both instances, and was itself renamed from
	// "old-tag", leaving that alias behind.
	localTg, err := st.UpsertTarget(store.Target{ContainerName: "radarr"})
	if err != nil {
		t.Fatalf("upsert local target: %v", err)
	}
	if _, err := st.AddAlias("container", "old-tag", localTg.ID); err != nil {
		t.Fatalf("add local alias: %v", err)
	}

	const appdata = "/host/user/appdata/radarr"
	def, err := json.Marshal(containerDefinition{Inspect: model.Inspect{Name: "radarr"}})
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	// Built in memory from the decrypted foreign definition and not persisted
	// before prepareRestoreForTarget validates it, as foreignContainerTarget
	// hands it to prepareForeignRestore.
	foreignTg := store.Target{ContainerName: "radarr", Definition: string(def), AppdataPaths: []string{appdata}}

	// The foreign repository holds a snapshot whose tag only happens to match
	// the local alias's old name. It is older than the local link, so the
	// local identity would claim it, and it must not be mistaken for this
	// item's history.
	collides := restic.Snapshot{ID: "dddd4444", Time: "2024-01-01T00:00:00Z", Tags: []string{"container:old-tag", "p1"}, Paths: []string{appdata}}
	own := restic.Snapshot{ID: "eeee5555", Tags: []string{"container:radarr", "p1"}, Paths: []string{appdata}}

	s := &Service{
		store:  st,
		cfg:    config.Config{HostMountRoot: "/host/user"},
		engine: aliasRestoreEngine{snaps: []restic.Snapshot{collides, own}},
	}
	ref := repoRef{repo: "rest:http://fake/foreign-session", mode: restic.Mode{}}
	ctx := context.Background()
	// The identity prepareForeignRestore builds: the item's own tag, no local
	// alias history.
	id := tagIdentity("container:radarr")

	t.Run("a snapshot tagged with a local alias is refused", func(t *testing.T) {
		_, err := s.prepareRestoreForTarget(ctx, ref, "radarr", collides.ID, foreignTg, id, "", false)
		if err == nil {
			t.Fatal("expected refusal: a local alias must not widen a foreign restore's accepted tags")
		}
	})

	t.Run("the item's own tag still restores normally", func(t *testing.T) {
		plan, err := s.prepareRestoreForTarget(ctx, ref, "radarr", own.ID, foreignTg, id, "", false)
		if err != nil {
			t.Fatalf("prepareRestoreForTarget: %v", err)
		}
		if plan.snapshotID != own.ID {
			t.Fatalf("expected the plan to target the item's own snapshot, got %+v", plan)
		}
	})
}
