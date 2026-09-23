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
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// A VM backup names each former name of its entry, oldest link first, and an
// entry without one keeps its two tags
// (TestBackupVMFileOnlyIsByteIdenticalToBeforeZvolTPMWiring).
func TestBackupVMTagsEveryFormerNameOfTheEntry(t *testing.T) {
	svc, eng, st, _ := vmZvolTestService(t, fileOnlyVMDomainXML, &zvolTPMSSH{})
	tg, err := st.UpsertVMTarget(store.VMTarget{Name: "plainvm"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "plain-vm", tg.ID, 200); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddAliasAt("vm", "Plain VM", tg.ID, 100); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.BackupVM(context.Background(), "plainvm"); err != nil {
		t.Fatalf("BackupVM: %v", err)
	}
	want := "vm:plainvm,p2,formerly:Plain VM,formerly:plain-vm"
	if got := strings.Join(eng.lastTags, ","); got != want {
		t.Fatalf("tags = %q, want %q", got, want)
	}
}

const (
	windows11Def = `{"domain_xml":"<domain><name>windows-11</name></domain>","method":"graceful"}`
	win11Def     = `{"domain_xml":"<domain><name>win11</name></domain>","method":"graceful"}`
)

// vmsAfterConfigLoss is an install with no rows whose VMs repository holds
// snaps and, for each name in defs, the encrypted definition file a backup
// mirrors beside them.
func vmsAfterConfigLoss(t *testing.T, snaps []restic.Snapshot, defs map[string]string) (*api.Service, *store.Repo, *fakeResticEngine) {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	writeVMDefs(t, filepath.Join(establishLocalRepo(t, dir, s.VMsPath), "vm-def"), defs)
	eng := &fakeResticEngine{snaps: snaps}
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), st, eng
}

func writeVMDefs(t *testing.T, dir string, defs map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, def := range defs {
		enc, err := secret.Encrypt(strings.Repeat("a", 64), []byte(def))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".def"), enc, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// renamedVMSnaps is windows-11's backup from before its rename to win11 and
// two backups of win11 that name windows-11 as a former name.
var renamedVMSnaps = []restic.Snapshot{
	{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}},
	{ID: "post1", Time: "2024-06-01T12:30:00.75Z", Tags: []string{"vm:win11", "p2", "formerly:windows-11"}},
	{ID: "post2", Time: "2024-08-01T00:00:00Z", Tags: []string{"vm:win11", "p2", "formerly:windows-11"}},
}

func vmRowNames(t *testing.T, st *store.Repo) string {
	t.Helper()
	rows, err := st.ListVMTargets()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return joined(sortedCopy(names))
}

// win11Linked is win11's definition recording windows-11 as its former name,
// with the definition the entry had there.
func win11Linked(t *testing.T) string {
	t.Helper()
	return withLinkRecords(t, win11Def, storedLink{Name: "windows-11", LinkedAt: unixOf(t, linkTime), PrevDefinition: windows11Def})
}

// After a /config loss the repository alone rebuilds one row for the renamed
// VM, linked to its former name at the time its definition records and
// keeping the old definition the record carries.
func TestDiscoverVMsFoldsAFormerNameIntoItsSuccessor(t *testing.T) {
	svc, st, _ := vmsAfterConfigLoss(t, renamedVMSnaps, map[string]string{"windows-11": windows11Def, "win11": win11Linked(t)})
	ctx := context.Background()

	if _, err := svc.DiscoverVMs(ctx, true); err != nil {
		t.Fatalf("DiscoverVMs (dry run): %v", err)
	}
	if rows := vmRowNames(t, st); rows != "" {
		t.Fatalf("a dry run wrote rows %q", rows)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); err == nil {
		t.Fatal("a dry run wrote an alias")
	}

	if _, err := svc.DiscoverVMs(ctx, false); err != nil {
		t.Fatalf("DiscoverVMs: %v", err)
	}
	if rows := vmRowNames(t, st); rows != "win11" {
		t.Fatalf("rows = %q, want win11 only", rows)
	}
	tg, err := st.GetVMTargetByName("win11")
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.AliasByOldName("vm", "windows-11")
	if err != nil || a.TargetID != tg.ID {
		t.Fatalf("alias = %+v, %v; want windows-11 linked to win11", a, err)
	}
	if want := unixOf(t, linkTime); a.LinkedAt != want {
		t.Fatalf("linked_at = %d, want the recorded %d", a.LinkedAt, want)
	}
	if a.PrevDefinition != windows11Def {
		t.Fatalf("prev_definition = %q, want the one win11's record carries", a.PrevDefinition)
	}
	snaps, err := svc.SnapshotsVM(ctx, "win11", "")
	if err != nil || joined(snapshotIDs(snaps)) != "post1,post2,pre1" {
		t.Fatalf("SnapshotsVM(win11) = %v, %v; want both names' backups", snapshotIDs(snaps), err)
	}
}

// The readability probe writes nothing, and a row left by an earlier Discover
// must not change that.
func TestDiscoverVMsDryRunLinksNothingEvenWhenTheSuccessorHasARow(t *testing.T) {
	svc, st, _ := vmsAfterConfigLoss(t, renamedVMSnaps, map[string]string{"windows-11": windows11Def, "win11": withLinks(t, win11Def, "windows-11")})
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "win11", Definition: win11Def}); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.DiscoverVMs(context.Background(), true); err != nil {
		t.Fatalf("DiscoverVMs (dry run): %v", err)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); err == nil {
		t.Fatal("a dry run wrote an alias")
	}
	if rows := vmRowNames(t, st); rows != "win11" {
		t.Fatalf("rows = %q, want the existing win11 only", rows)
	}
}

// When two rebuilt VMs both record windows-11 as a former name, the link goes
// to the first by name, so the same repository always rebuilds the same way.
func TestDiscoverVMsLinksAFormerNameToTheFirstClaimantByName(t *testing.T) {
	second := restic.Snapshot{ID: "b1", Time: "2024-07-01T00:00:00Z", Tags: []string{"vm:win11-b", "p2", "formerly:windows-11"}}
	snaps := append(append([]restic.Snapshot(nil), renamedVMSnaps...), second)
	linked := withLinks(t, win11Def, "windows-11")
	svc, st, _ := vmsAfterConfigLoss(t, snaps, map[string]string{"windows-11": windows11Def, "win11": linked, "win11-b": linked})

	if _, err := svc.DiscoverVMs(context.Background(), false); err != nil {
		t.Fatalf("DiscoverVMs: %v", err)
	}
	if rows := vmRowNames(t, st); rows != "win11,win11-b" {
		t.Fatalf("rows = %q, want win11 and win11-b", rows)
	}
	tg, err := st.GetVMTargetByName("win11")
	if err != nil {
		t.Fatal(err)
	}
	if a, err := st.AliasByOldName("vm", "windows-11"); err != nil || a.TargetID != tg.ID {
		t.Fatalf("alias = %+v, %v; want windows-11 linked to win11", a, err)
	}
}

// A later Discover neither moves nor second-guesses a link. When windows-11
// has since been backed up by another VM, which the link's time bound keeps
// out of the entry, that VM comes back as an entry of its own beside it.
func TestDiscoverVMsLeavesAnExistingLinkAsItIs(t *testing.T) {
	svc, st, eng := vmsAfterConfigLoss(t, renamedVMSnaps, map[string]string{"windows-11": windows11Def, "win11": withLinks(t, win11Def, "windows-11")})
	ctx := context.Background()
	if _, err := svc.DiscoverVMs(ctx, false); err != nil {
		t.Fatalf("first DiscoverVMs: %v", err)
	}
	first, err := st.AliasByOldName("vm", "windows-11")
	if err != nil {
		t.Fatalf("windows-11 must be linked: %v", err)
	}
	eng.snaps = append(eng.snaps, restic.Snapshot{ID: "later1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}})

	if _, err := svc.DiscoverVMs(ctx, false); err != nil {
		t.Fatalf("second DiscoverVMs: %v", err)
	}
	if rows := vmRowNames(t, st); rows != "win11,windows-11" {
		t.Fatalf("rows = %q, want win11 and the later VM's windows-11", rows)
	}
	if a, err := st.AliasByOldName("vm", "windows-11"); err != nil || a != first {
		t.Fatalf("alias = %+v, %v; want it unchanged from %+v", a, err, first)
	}
}

// vmsOnTwoRepositories is vmsAfterConfigLoss with a named repository, Cold,
// beside the domain's own. Each holds its snapshots and definition files.
func vmsOnTwoRepositories(t *testing.T, own, cold []restic.Snapshot, ownDefs, coldDefs map[string]string) (*api.Service, *store.Repo) {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}
	st := newMemStore(t)
	s := mustSettings(t, st)
	s.VMsPath = "backups/vms"
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	ownRepo := establishLocalRepo(t, dir, s.VMsPath)
	coldRepo := establishLocalRepo(t, dir, "backups/cold")
	if _, err := st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: "Cold", Repo: "backups/cold", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	writeVMDefs(t, filepath.Join(ownRepo, "vm-def"), ownDefs)
	writeVMDefs(t, filepath.Join(coldRepo, "vm-def"), coldDefs)
	eng := &fakeResticEngine{snapsByRepo: map[string][]restic.Snapshot{ownRepo: own, coldRepo: cold}}
	return api.NewService(cfg, st, &fakeServiceDocker{}, fakeVirsh{}, eng), st
}

// A later VM's backups under windows-11 are looked for in the repository its
// definition file sits beside, a named one here, and bring it back on it.
func TestDiscoverVMsLooksForALaterVMInTheOldNamesOwnRepository(t *testing.T) {
	later := restic.Snapshot{ID: "later1", Time: "2024-09-01T00:00:00Z", Tags: []string{"vm:windows-11", "p2"}}
	cold := append(append([]restic.Snapshot(nil), renamedVMSnaps...), later)
	svc, st := vmsOnTwoRepositories(t, nil, cold, nil, map[string]string{"windows-11": windows11Def, "win11": win11Linked(t)})

	if _, err := svc.DiscoverVMs(context.Background(), false); err != nil {
		t.Fatalf("DiscoverVMs: %v", err)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); err != nil {
		t.Fatalf("windows-11 must be linked: %v", err)
	}
	named, err := st.ListNamedRepos()
	if err != nil || len(named) != 1 {
		t.Fatalf("named repositories = %+v, %v", named, err)
	}
	if tg, err := st.GetVMTargetByName("windows-11"); err != nil || tg.Repo != named[0].ID {
		t.Fatalf("windows-11 = %+v, %v; want the later VM's entry on Cold", tg, err)
	}
}

// A record written without the definition the entry had under windows-11
// links it without one, whatever windows-11's own definition file holds, and
// an unlink, which would need that definition, refuses.
func TestDiscoverVMsLinksWithoutAnOldDefinitionAndUnlinkRefuses(t *testing.T) {
	svc, st, _ := vmsAfterConfigLoss(t, renamedVMSnaps, map[string]string{"windows-11": windows11Def, "win11": withLinks(t, win11Def, "windows-11")})
	ctx := context.Background()

	if _, err := svc.DiscoverVMs(ctx, false); err != nil {
		t.Fatalf("DiscoverVMs: %v", err)
	}
	a, err := st.AliasByOldName("vm", "windows-11")
	if err != nil || a.PrevDefinition != "" {
		t.Fatalf("alias = %+v, %v; want windows-11 linked with no definition", a, err)
	}
	if err := svc.UnlinkVMAlias(ctx, "windows-11"); err == nil || !strings.Contains(err.Error(), "not kept") {
		t.Fatalf("UnlinkVMAlias = %v, want a refusal because no definition was kept", err)
	}
	if _, err := st.AliasByOldName("vm", "windows-11"); err != nil {
		t.Fatalf("the refused unlink must keep the alias: %v", err)
	}
}

// A recorded former name goes into the formerly: tag of the next backup, so
// one that is no VM name or holds a comma is not linked.
func TestDiscoverVMsDoesNotLinkAnUnsafeFormerName(t *testing.T) {
	for _, old := range []string{"windows,11", "-windows-11"} {
		t.Run(old, func(t *testing.T) {
			snaps := []restic.Snapshot{
				{ID: "pre1", Time: "2024-01-01T00:00:00Z", Tags: []string{"vm:" + old, "p2"}},
				{ID: "post1", Time: "2024-06-01T00:00:00Z", Tags: []string{"vm:win11", "p2", "formerly:" + old}},
			}
			svc, st, _ := vmsAfterConfigLoss(t, snaps, map[string]string{old: windows11Def, "win11": withLinks(t, win11Def, old)})

			if _, err := svc.DiscoverVMs(context.Background(), false); err != nil {
				t.Fatalf("DiscoverVMs: %v", err)
			}
			if _, err := st.AliasByOldName("vm", old); err == nil {
				t.Fatalf("%q must not be linked", old)
			}
			if _, err := st.GetVMTargetByName("win11"); err != nil {
				t.Fatalf("win11 must still be rebuilt: %v", err)
			}
		})
	}
}
