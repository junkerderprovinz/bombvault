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

// TestRestoreSelectionChange is the RESTORE-01 tracer: restoring an OLDER
// snapshot after the user reshaped their selection must COMPLETE, handing the
// engine the mapped SNAPSHOT-path form (longest-prefix mapping against the
// chosen snapshot's recorded Paths) — not the stored list replayed verbatim,
// which misses the snapshot and used to fail the restore mid-loop AFTER the
// container had been stopped and removed (the exact bug RESTORE-01 fixes).
func TestRestoreSelectionChange(t *testing.T) {
	dir := t.TempDir()
	// Container paths are Linux-absolute under the host mount root; the restore
	// uses fakes (no real FS access to these paths), so a fixed Linux root is fine
	// (same reasoning as TestRestoreUsesStoredDefinitionWhenContainerDeleted).
	cfg := config.Config{
		AppKey:            strings.Repeat("a", 64),
		DataDir:           dir,
		HostMountRoot:     "/host/user",
		FlashTemplatesDir: dir + "/flash",
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	// A remote-style repo: the snapshot listing reaches the fake engine directly
	// (a local repo would need an on-disk marker under the fixed Linux root).
	s.ContainersPath = "rest:http://127.0.0.1/containers"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}

	// The user narrowed the selection after this snapshot was taken: the stored
	// positional truth (AppdataPaths, recorded by the latest backup) is the NEW
	// narrow shape, while the seeded snapshot carries the OLD wide Paths.
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

	eng := &fakeResticEngine{snaps: []restic.Snapshot{{
		ID:    "aaaa1111",
		Tags:  []string{"container:plex", "p1"},
		Paths: []string{"/host/user/user/appdata/plex"},
	}}}
	d := &fakeServiceDocker{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	if err := svc.Restore(context.Background(), "plex", "aaaa1111", true, "", false); err != nil {
		t.Fatalf("restore of an older snapshot after the selection changed must complete: %v", err)
	}

	// The engine must be handed the MAPPED SNAPSHOT-PATH form: the longest
	// ancestor of the stored path that the snapshot actually recorded. The
	// stored path itself would miss — restic restore <id>:<path> selectors must
	// come from the snapshot's Paths (restic.go RestoreSubtreeToArgs doc).
	want := "aaaa1111:/host/user/user/appdata/plex"
	if len(eng.restored) != 1 || !strings.HasSuffix(eng.restored[0], want) {
		t.Fatalf("restored = %v, want exactly one call ending in %q", eng.restored, want)
	}
	// The run completed end-to-end: the container was recreated.
	if d.createdIn.Config.Image == "" {
		t.Fatal("CreateAndStart was not called")
	}
	if d.createdIn.Config.Image != "plex:latest" {
		t.Fatalf("recreated with wrong image %q, want plex:latest", d.createdIn.Config.Image)
	}
}
