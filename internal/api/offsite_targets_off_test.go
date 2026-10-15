package api_test

import (
	"context"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Creating a target also fills the domain's off-site settings column, which
// stays set when the target is switched off.
func TestAnOffsiteCopyWithEveryTargetSwitchedOffIsRefusedUpFront(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	const repo = "rest:http://192.168.1.2:8000/containers"
	if _, err := st.MutateSettings(func(s *store.Settings) error { s.ContainersOffsite = repo; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Domain: "containers", Name: "Primary", Repo: repo, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	eng := &fakeResticEngine{}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	if err := svc.StartReplicateOffsite("containers"); err == nil || !strings.Contains(err.Error(), "switched off") {
		t.Fatalf("StartReplicateOffsite = %v, want it refused because the target is switched off", err)
	}
	if err := svc.ReplicateOffsite(context.Background(), "containers"); err == nil || !strings.Contains(err.Error(), "switched off") {
		t.Fatalf("ReplicateOffsite = %v, want it refused because the target is switched off", err)
	}
	if len(eng.copied) != 0 {
		t.Fatalf("copied %v to a switched-off target", eng.copied)
	}
}
