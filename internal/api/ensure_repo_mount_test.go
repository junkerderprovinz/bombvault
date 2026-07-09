package api_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// #55: once a repo has been established at a local destination, a later failure
// to find its `config` (the backing share unmounted after boot) must NOT trigger
// a re-init — that would write an empty repo shadowing the real backups. It must
// return ErrBackupPathNotMounted instead. A genuinely new location still inits.
func TestEnsureRepoRefusesReInitWhenEstablishedRepoVanishes(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)
	mode := restic.Mode{Encrypted: true, Password: "pw"}

	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	// Establish it: a `config` marker makes RepoOpens true → EnsureRepo records it.
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureRepo(context.Background(), repo, mode); err != nil {
		t.Fatalf("establishing an existing repo should succeed: %v", err)
	}
	if len(eng.inited) != 0 {
		t.Fatalf("opening an existing repo must not init, got %v", eng.inited)
	}

	// The backing share vanishes (late mount at boot): the config is gone.
	if err := os.Remove(filepath.Join(repo, "config")); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureRepo(context.Background(), repo, mode); !errors.Is(err, api.ErrBackupPathNotMounted) {
		t.Fatalf("a vanished established repo must return ErrBackupPathNotMounted, got %v", err)
	}
	if len(eng.inited) != 0 {
		t.Fatalf("must NOT re-init an established-but-vanished repo, got inits %v", eng.inited)
	}

	// A genuinely new location (never established) still initialises normally.
	fresh := filepath.Join(dir, "fresh")
	if err := svc.EnsureRepo(context.Background(), fresh, mode); err != nil {
		t.Fatalf("a fresh location should init, got %v", err)
	}
	if len(eng.inited) != 1 || eng.inited[0] != fresh {
		t.Fatalf("the fresh location should have been inited once, got %v", eng.inited)
	}
}
