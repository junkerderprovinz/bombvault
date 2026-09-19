package api_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/api"
	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A backup from before the rename carries only the old name's tag, since
// nothing in the repository is rewritten, and one taken before the link
// belongs to the renamed entry. TestSnapshotsVMAliasClaimsOnlyPreLinkHistory
// pins the other side of that bound.
func TestSnapshotsVMFollowsAlias(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	s.VMsEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(dir, "backups", "vms")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	eng := &fakeResticEngine{snaps: []restic.Snapshot{
		{ID: "aaaa1111", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}, // pre-rename snapshot: only the old name
	}}
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "win11"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "windows-11", tg.ID); err != nil {
		t.Fatal(err)
	}
	svc := api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng)

	snaps, err := svc.SnapshotsVM(context.Background(), "win11", "")
	if err != nil {
		t.Fatalf("SnapshotsVM: %v", err)
	}
	if len(snaps) != 1 || snaps[0].ID != "aaaa1111" {
		t.Fatalf("expected the old-name-only snapshot to belong to the renamed entry, got %+v", snaps)
	}
}

// BackupVM's retention forgets the current name's tag and the alias's tag in
// one call. vmZvolTestService sets RetentionKeepLast, so the policy has
// something to do, and a file-only domain keeps the per-zvol retention calls
// out of the count.
func TestBackupVMRetentionFollowsAlias(t *testing.T) {
	svc, eng, st, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})

	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "plainvm"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAlias("vm", "plainvm-old", tg.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}

	if len(eng.forgetTags) != 1 {
		t.Fatalf("want exactly 1 retention call, got %d: %v", len(eng.forgetTags), eng.forgetTags)
	}
	got := strings.Split(eng.forgetTags[0], ",")
	want := map[string]bool{"vm:plainvm": true, "vm:plainvm-old": true}
	if len(got) != 2 || !want[got[0]] || !want[got[1]] {
		t.Fatalf("retention tags = %q, want both %v", eng.forgetTags[0], want)
	}
}
