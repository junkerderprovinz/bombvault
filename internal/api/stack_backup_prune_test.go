package api_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// TestManualBackupOfAComposeMemberPrunesOnce: a manual "Back up now" of one
// compose member also backs up its project folder, and the two retention
// passes that follow (the folder's, then the member's own) share a single
// prune instead of each running one.
func TestManualBackupOfAComposeMemberPrunesOnce(t *testing.T) {
	dir := t.TempDir()
	root := filepath.ToSlash(dir)
	cfg := config.Config{
		AppKey:         strings.Repeat("a", 64),
		DataDir:        dir,
		HostMountRoot:  root,
		HostSourceRoot: "/mnt",
	}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.EncryptionEnabled = false
	s.ContainersPath = "backups/containers"
	s.RetentionKeepLast = 3
	if err := st.UpdateSettings(s); err != nil {
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

	if _, err := svc.Backup(context.Background(), "immich-server"); err != nil {
		t.Fatalf("backup: %v", err)
	}

	wantTags := []string{"stack:immich", "container:immich-server"}
	if len(eng.forgetTags) != len(wantTags) {
		t.Fatalf("forget tags = %v, want %v", eng.forgetTags, wantTags)
	}
	for i, tag := range wantTags {
		if eng.forgetTags[i] != tag {
			t.Fatalf("forget tags = %v, want %v", eng.forgetTags, wantTags)
		}
	}

	pruned := 0
	for _, p := range eng.forgetPolicyPrunes {
		if p {
			pruned++
		}
	}
	if pruned != 1 {
		t.Fatalf("pruned %d times across %v, want exactly 1", pruned, eng.forgetTags)
	}
}
