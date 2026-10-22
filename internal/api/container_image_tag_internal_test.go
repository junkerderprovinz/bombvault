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

// The night after an update the stored definition already names the new
// image, so rolling back to the snapshot from before the update has to take
// the image from that snapshot.
func TestRestoringAnOlderSnapshotBringsBackTheImageItRan(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	const appdata = "/host/user/appdata/plex"
	def, err := json.Marshal(containerDefinition{Inspect: model.Inspect{Name: "plex", Image: "sha256:new"}})
	if err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex", Definition: string(def), AppdataPaths: []string{appdata}})
	if err != nil {
		t.Fatal(err)
	}
	before := restic.Snapshot{ID: "aaaa1111", Time: "2026-10-20T03:00:00Z", Tags: []string{"container:plex", "p1", imageTagPrefix + "sha256:old"}, Paths: []string{appdata}}
	after := restic.Snapshot{ID: "bbbb2222", Time: "2026-10-21T03:00:00Z", Tags: []string{"container:plex", "p1", imageTagPrefix + "sha256:new"}, Paths: []string{appdata}}
	untagged := restic.Snapshot{ID: "cccc3333", Time: "2026-01-01T03:00:00Z", Tags: []string{"container:plex", "p1"}, Paths: []string{appdata}}
	s := &Service{
		store:  st,
		cfg:    config.Config{HostMountRoot: "/host/user"},
		engine: aliasRestoreEngine{snaps: []restic.Snapshot{untagged, before, after}},
	}
	ref := repoRef{repo: "rest:http://fake/plex", mode: restic.Mode{}}
	for _, c := range []struct {
		snap restic.Snapshot
		want string
	}{{before, "sha256:old"}, {after, "sha256:new"}, {untagged, "sha256:new"}} {
		plan, err := s.prepareRestoreForTarget(context.Background(), ref, "plex", c.snap.ID, tg, tagIdentity("container:plex"), "", false)
		if err != nil {
			t.Fatalf("prepare %s: %v", c.snap.ID, err)
		}
		if plan.inspect.Image != c.want {
			t.Errorf("restore of %s recreates on %q, want %q", c.snap.ID, plan.inspect.Image, c.want)
		}
	}
}

func TestAContainerBackupNamesTheImageItRan(t *testing.T) {
	r := newRestoreRig(t)
	r.seedPlex(t)
	in := r.docker.containers["plex"]
	in.Image = "sha256:old"
	r.docker.containers["plex"] = in
	if _, err := r.svc.Backup(context.Background(), "plex"); err != nil {
		t.Fatal(err)
	}
	snaps := r.snapshotsOf(t, "containers")
	if got := taggedImage(snaps[len(snaps)-1].Tags); got != "sha256:old" {
		t.Fatalf("the backup is tagged %v, want the image it ran", snaps[len(snaps)-1].Tags)
	}
}
