package api_test

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Backup Everything backs up a compose member's project folder once, with the
// folder's retention leaving the prune to the end of the round.
func TestBackupEverythingBacksUpComposeProjectFolders(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		AppKey:         strings.Repeat("a", 64),
		DataDir:        dir,
		HostMountRoot:  filepath.ToSlash(dir),
		HostSourceRoot: "/mnt",
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.ContainersEnabled = true
	s.RetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "immich-server", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}

	d := &fakeServiceDocker{inspect: model.Inspect{
		Name:    "/immich-server",
		Image:   "immich:latest",
		Running: true,
		Config: model.Config{Labels: map[string]string{
			"com.docker.compose.project":             "immich",
			"com.docker.compose.project.working_dir": "/mnt/appdata/immich",
		}},
	}}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, d, fakeVirsh{}, eng)

	if _, err := svc.BackupEverything(context.Background()); err != nil {
		t.Fatalf("BackupEverything: %v", err)
	}

	i := slices.Index(eng.forgetTags, "stack:immich")
	if i < 0 {
		t.Fatalf("no stack:immich retention ran; forget tags = %v", eng.forgetTags)
	}
	if eng.forgetPolicyPruned[i] {
		t.Fatal("the project folder's retention pruned inside the round")
	}
}
