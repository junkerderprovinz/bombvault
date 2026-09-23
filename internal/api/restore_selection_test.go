package api_test

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Restoring an older snapshot after the selection changed completes, with each
// stored path mapped against the paths that snapshot recorded.
func TestRestoreSelectionChange(t *testing.T) {
	dir := t.TempDir()
	// The fakes never touch these paths, so a fixed Linux root works everywhere.
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     "/host/user",
		FlashTemplatesDir: dir + "/flash",
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	// A local repo would need an on-disk marker under the fixed Linux root.
	s.ContainersPath = "rest:http://127.0.0.1/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// The selection was narrowed after the snapshot: AppdataPaths holds the new
	// narrow path, the snapshot the old wide one.
	defBytes, err := marshalDefinition(model.Inspect{
		Name:   "/plex",
		Config: model.Config{Image: "plex:latest"},
	}, "<xml/>")
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	if _, err := st.UpsertTarget(store.Target{
		ContainerName: "plex",
		AppdataPaths:  []string{"/host/user/user/appdata/plex/config"},
		Definition:    string(defBytes),
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	eng := &fakeResticEngine{
		snaps: []restic.Snapshot{{
			ID:    "aaaa1111",
			Tags:  []string{"container:plex", "p1"},
			Paths: []string{"/host/user/user/appdata/plex"},
		}},
		// A recorded root only says the backup covered /plex. A narrowed
		// selector is checked against the listing before the container is
		// torn down.
		lsEntries: []restic.FileEntry{
			{Path: "/host/user/user/appdata/plex", Type: "dir"},
			{Path: "/host/user/user/appdata/plex/config", Type: "dir"},
		},
	}
	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	if err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false); err != nil {
		t.Fatalf("restore of an older snapshot after the selection changed must complete: %v", err)
	}

	// The engine gets the stored path, not the recorded parent. A selector can
	// name any directory inside the snapshot (TestRestoreSubtreeBelowRecordedPath
	// in internal/restic), and restoring the parent would overwrite the live
	// folders the user left out of the backup.
	want := "aaaa1111:/host/user/user/appdata/plex/config"
	if len(eng.restored) != 1 || !strings.HasSuffix(eng.restored[0], want) {
		t.Fatalf("restored = %v, want exactly one call ending in %q", eng.restored, want)
	}
	if d.createdIn.Config.Image == "" {
		t.Fatal("CreateAndStart was not called")
	}
	if d.createdIn.Config.Image != "plex:latest" {
		t.Fatalf("recreated with wrong image %q, want plex:latest", d.createdIn.Config.Image)
	}
}

// restoreScaffold seeds a plex target with the given stored paths and a remote
// repo whose snapshots and listing come from the fake engine.
func restoreScaffold(t *testing.T, storedPaths []string, snaps []restic.Snapshot, lsEntries ...restic.FileEntry) (*store.Repo, *fakeServiceDocker, *fakeResticEngine, *api.Service) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     "/host/user",
		FlashTemplatesDir: dir + "/flash",
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "rest:http://127.0.0.1/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	defBytes, err := marshalDefinition(model.Inspect{
		Name:   "/plex",
		Config: model.Config{Image: "plex:latest"},
	}, "<xml/>")
	if err != nil {
		t.Fatalf("marshal definition: %v", err)
	}
	if _, err := st.UpsertTarget(store.Target{
		ContainerName: "plex",
		AppdataPaths:  storedPaths,
		Definition:    string(defBytes),
	}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	eng := &fakeResticEngine{snaps: snaps, lsEntries: lsEntries}
	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)
	return st, d, eng, svc
}

// restoreRunRow returns the one restore run in the store.
func restoreRunRow(t *testing.T, st *store.Repo) store.Run {
	t.Helper()
	runs, err := st.ListRuns(25)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	var row *store.Run
	for i := range runs {
		if runs[i].Kind == "restore" {
			if row != nil {
				t.Fatalf("expected one restore run row, got at least two: %v", runs)
			}
			row = &runs[i]
		}
	}
	if row == nil {
		t.Fatalf("no restore run row recorded; runs = %v", runs)
	}
	return *row
}

// A stored path the snapshot has no data for is skipped without aborting the
// restore, and the run row notes the skip with the path scrubbed.
func TestRestoreSelectionChangePerPathSkip(t *testing.T) {
	st, d, eng, svc := restoreScaffold(t,
		[]string{
			"/host/user/user/appdata/plex/config",
			"/host/user/mnt/disk2/appdata/jellyfin/config", // added after the snapshot
		},
		[]restic.Snapshot{{
			ID:    "aaaa1111",
			Tags:  []string{"container:plex", "p1"},
			Paths: []string{"/host/user/user/appdata/plex"},
		}},
		restic.FileEntry{Path: "/host/user/user/appdata/plex", Type: "dir"},
		restic.FileEntry{Path: "/host/user/user/appdata/plex/config", Type: "dir"})

	if err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false); err != nil {
		t.Fatalf("restore with one unmapped stored path must complete (per-path skip, never a global abort): %v", err)
	}

	want := "aaaa1111:/host/user/user/appdata/plex/config"
	if len(eng.restored) != 1 || !strings.HasSuffix(eng.restored[0], want) {
		t.Fatalf("restored = %v, want exactly one call ending in %q", eng.restored, want)
	}
	if d.createdIn.Config.Image == "" {
		t.Fatal("CreateAndStart was not called")
	}

	row := restoreRunRow(t, st)
	if row.Status != "success" {
		t.Fatalf("run status = %q, want success", row.Status)
	}
	if !strings.Contains(row.Error, "1 stored path") {
		t.Fatalf("run error note = %q, want it to report the skipped-path count", row.Error)
	}
	if n := strings.Count(row.Error, "[path]"); n != 1 {
		t.Fatalf("run error note = %q, want exactly 1 scrubbed [path] token, got %d", row.Error, n)
	}
	if strings.Contains(row.Error, "/host/") || strings.Contains(row.Error, "jellyfin") {
		t.Fatalf("run error note = %q leaks a raw path; it must be scrubbed to [path] first", row.Error)
	}
}

// Several skipped paths are all noted and none aborts the run. Every path
// scrubs to the same [path] token, so the order of the skips is checked before
// scrubbing, in TestMapRestorePaths.
func TestRestoreSelectionChangeSkipOrder(t *testing.T) {
	st, d, eng, svc := restoreScaffold(t,
		[]string{
			"/host/user/user/appdata/plex/config",
			"/host/user/mnt/disk2/appdata/extra",
			"/host/user/mnt/disk3/appdata/more",
		},
		[]restic.Snapshot{{
			ID:    "aaaa1111",
			Tags:  []string{"container:plex", "p1"},
			Paths: []string{"/host/user/user/appdata/plex"},
		}},
		restic.FileEntry{Path: "/host/user/user/appdata/plex", Type: "dir"},
		restic.FileEntry{Path: "/host/user/user/appdata/plex/config", Type: "dir"})

	if err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false); err != nil {
		t.Fatalf("restore with multiple unmapped stored paths must complete: %v", err)
	}

	want := "aaaa1111:/host/user/user/appdata/plex/config"
	if len(eng.restored) != 1 || !strings.HasSuffix(eng.restored[0], want) {
		t.Fatalf("restored = %v, want exactly one call ending in %q", eng.restored, want)
	}
	if d.createdIn.Config.Image == "" {
		t.Fatal("CreateAndStart was not called")
	}

	row := restoreRunRow(t, st)
	if row.Status != "success" {
		t.Fatalf("run status = %q, want success", row.Status)
	}
	if !strings.Contains(row.Error, "2 stored path") {
		t.Fatalf("run error note = %q, want it to report both skips", row.Error)
	}
	if n := strings.Count(row.Error, "[path]"); n != 2 {
		t.Fatalf("run error note = %q, want exactly 2 scrubbed [path] tokens, got %d", row.Error, n)
	}
}

// When no stored path maps onto the snapshot, the restore aborts in the
// synchronous prepare phase, before the container is stopped or removed.
func TestRestoreEmptyIntersection(t *testing.T) {
	_, d, eng, svc := restoreScaffold(t,
		[]string{"/host/user/user/appdata/plex/config"},
		[]restic.Snapshot{{
			ID:    "aaaa1111",
			Tags:  []string{"container:plex", "p1"},
			Paths: []string{"/mnt/other/appdata/entirely-different"},
		}})

	err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false)
	if err == nil {
		t.Fatal("restore with an empty intersection must abort, not run")
	}
	if !strings.Contains(err.Error(), "nothing to restore") {
		t.Fatalf("error = %v, want the explicit nothing-to-restore shape", err)
	}
	for _, c := range d.calls {
		if strings.HasPrefix(c, "stop:") || strings.HasPrefix(c, "remove:") {
			t.Fatalf("empty intersection must abort BEFORE teardown, but docker calls include %q (all: %v)", c, d.calls)
		}
	}
	if len(eng.restored) != 0 {
		t.Fatalf("no restic restore may run for an empty intersection, got %v", eng.restored)
	}
	if d.createdIn.Config.Image != "" {
		t.Fatal("container must not be recreated for an aborted restore")
	}
}

// A restore by explicit snapshot id maps against that snapshot's paths, not the
// newest one's. The older snapshot recorded the parent and reaches both stored
// folders, the newer one only config, so two restores prove the older was used.
func TestRestoreSelectionChangeExplicitSnapshotID(t *testing.T) {
	_, _, eng, svc := restoreScaffold(t,
		[]string{
			"/host/user/user/appdata/plex/config",
			"/host/user/user/appdata/plex/transcode",
		},
		[]restic.Snapshot{
			{
				ID:    "bbbb2222",
				Tags:  []string{"container:plex", "p1"},
				Paths: []string{"/host/user/user/appdata/plex/config"},
			},
			{
				ID:    "aaaa1111",
				Tags:  []string{"container:plex", "p1"},
				Paths: []string{"/host/user/user/appdata/plex"},
			},
		},
		restic.FileEntry{Path: "/host/user/user/appdata/plex", Type: "dir"},
		restic.FileEntry{Path: "/host/user/user/appdata/plex/config", Type: "dir"},
		restic.FileEntry{Path: "/host/user/user/appdata/plex/transcode", Type: "dir"})

	if err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false); err != nil {
		t.Fatalf("restore of the explicit older snapshot: %v", err)
	}

	want := []string{
		"aaaa1111:/host/user/user/appdata/plex/config",
		"aaaa1111:/host/user/user/appdata/plex/transcode",
	}
	if len(eng.restored) != len(want) {
		t.Fatalf("restored = %v, want %d calls (one per stored path, mapped against the explicitly chosen snapshot)", eng.restored, len(want))
	}
	for i, w := range want {
		if !strings.HasSuffix(eng.restored[i], w) {
			t.Fatalf("restored[%d] = %q, want a call ending in %q", i, eng.restored[i], w)
		}
	}
}

// A recorded ancestor does not prove a narrowed path is in the snapshot: an
// --exclude may have carved it out, or it did not exist yet. The path is checked
// against the snapshot's tree in the prepare phase and skipped if missing, never
// widened back to the ancestor, which would overwrite live data that was never
// backed up.
func TestRestoreNarrowedPathMissingFromSnapshot(t *testing.T) {
	t.Run("absent narrowed path is skipped and the run still completes", func(t *testing.T) {
		st, d, eng, svc := restoreScaffold(t,
			[]string{
				"/host/user/user/appdata/plex/config",
				"/host/user/user/appdata/plex/transcode", // excluded when this snapshot ran
			},
			[]restic.Snapshot{{
				ID:    "aaaa1111",
				Tags:  []string{"container:plex", "p1"},
				Paths: []string{"/host/user/user/appdata/plex"},
			}},
			restic.FileEntry{Path: "/host/user/user/appdata/plex", Type: "dir"},
			restic.FileEntry{Path: "/host/user/user/appdata/plex/config", Type: "dir"})

		if err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false); err != nil {
			t.Fatalf("a narrowed path missing from the snapshot must be skipped, not fatal: %v", err)
		}
		waitForBackupDone(t, svc)

		want := "aaaa1111:/host/user/user/appdata/plex/config"
		if len(eng.restored) != 1 || !strings.HasSuffix(eng.restored[0], want) {
			t.Fatalf("restored = %v, want exactly one call ending in %q (transcode is not in this snapshot)", eng.restored, want)
		}
		if d.createdIn.Config.Image == "" {
			t.Fatal("the container must still be recreated")
		}
		row := restoreRunRow(t, st)
		if row.Status != "success" {
			t.Fatalf("run status = %q, want success", row.Status)
		}
		if !strings.Contains(row.Error, "1 stored path") {
			t.Fatalf("run error note = %q, want it to report the one skip", row.Error)
		}
	})

	t.Run("every narrowed path absent aborts before the container is torn down", func(t *testing.T) {
		_, d, eng, svc := restoreScaffold(t,
			[]string{"/host/user/user/appdata/plex/transcode"},
			[]restic.Snapshot{{
				ID:    "aaaa1111",
				Tags:  []string{"container:plex", "p1"},
				Paths: []string{"/host/user/user/appdata/plex"},
			}},
			restic.FileEntry{Path: "/host/user/user/appdata/plex", Type: "dir"},
			restic.FileEntry{Path: "/host/user/user/appdata/plex/config", Type: "dir"})

		err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false)
		if err == nil {
			t.Fatal("nothing left to restore must abort, not run")
		}
		if !strings.Contains(err.Error(), "nothing to restore") {
			t.Fatalf("error = %v, want the explicit nothing-to-restore shape", err)
		}
		// The refusal is synchronous, so the container is still there.
		for _, c := range d.calls {
			if strings.HasPrefix(c, "stop:") || strings.HasPrefix(c, "remove:") {
				t.Fatalf("must abort before teardown, but docker calls include %q (all: %v)", c, d.calls)
			}
		}
		if len(eng.restored) != 0 {
			t.Fatalf("no restic restore may run, got %v", eng.restored)
		}
	})
}
