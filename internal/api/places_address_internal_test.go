package api

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestATargetHoldsWhatItWasLastSeenWith(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("containers", "B2", "s3:https://s3.example.com/bucket/container")
	if facts, err := f.svc.rowFacts(target); err != nil || facts.Established {
		t.Fatalf("a target never copied to = %+v, %v", facts, err)
	}
	f.listing("containers", target.ID, 100, copiesRow("container:nginx", 3, 90), copiesRow("stack:immich", 2, 80))
	if facts, err := f.svc.rowFacts(target); err != nil || !facts.Established || facts.Snapshots != 5 {
		t.Fatalf("a listed target = %+v, %v, want 5 snapshots", facts, err)
	}
}

func TestATargetMeasuredWithSnapshotsIsEstablished(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("vms", "B2", "s3:https://s3.example.com/bucket/vms")
	later := time.Now().Add(time.Hour).Unix()
	if err := f.st.AddRepoStat(store.RepoStat{Domain: "vms", Source: offsiteStatSource(target.ID), At: later, Snapshots: 7}); err != nil {
		t.Fatal(err)
	}
	if facts, err := f.svc.rowFacts(target); err != nil || !facts.Established || facts.Snapshots != 7 {
		t.Fatalf("a measured target = %+v, %v, want 7 snapshots", facts, err)
	}
}

func TestATargetOnceCopiedToIsEstablished(t *testing.T) {
	f := newPlacementFixture(t)
	target := f.target("vms", "B2", "s3:https://s3.example.com/bucket/vms")
	id, err := f.st.RecordOffsiteRunForTarget("vms", target.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.FinishOffsiteRun(id, true, ""); err != nil {
		t.Fatal(err)
	}
	if facts, err := f.svc.rowFacts(target); err != nil || !facts.Established {
		t.Fatalf("a target copied to = %+v, %v", facts, err)
	}
}

func TestANamedRepositoryIsEstablishedByItsItems(t *testing.T) {
	f := newPlacementFixture(t)
	repo := f.namedRepo("Storage Box", "sftp:u1@box.example:/bv")
	if facts, err := f.svc.rowFacts(repo); err != nil || facts.Established {
		t.Fatalf("an unused repository = %+v, %v", facts, err)
	}
	f.container("nginx", repo.ID)
	if facts, err := f.svc.rowFacts(repo); err != nil || !facts.Established || facts.Items != 1 {
		t.Fatalf("a repository in use = %+v, %v", facts, err)
	}
}

func TestALocalRepositoryOnceCreatedIsEstablished(t *testing.T) {
	f := newPlacementFixture(t)
	repo := f.namedRepo("NAS", "nas")
	if facts, err := f.svc.rowFacts(repo); err != nil || facts.Established {
		t.Fatalf("a repository never created = %+v, %v", facts, err)
	}
	loc, err := f.svc.resolveRepo("nas")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.MarkRepoEstablished(loc); err != nil {
		t.Fatal(err)
	}
	if facts, err := f.svc.rowFacts(repo); err != nil || !facts.Established {
		t.Fatalf("a created repository = %+v, %v", facts, err)
	}
}

func TestADomainPathIsEstablishedByABackupOrACountedSnapshot(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if facts, err := f.svc.domainPathFacts(settings, "containers"); err != nil || facts.Established {
		t.Fatalf("a domain path never backed up = %+v, %v", facts, err)
	}
	nginx := f.container("nginx", "")
	f.backupRun(nginx.ID, 100)
	if facts, err := f.svc.domainPathFacts(settings, "containers"); err != nil || !facts.Established {
		t.Fatalf("a domain path backed up = %+v, %v", facts, err)
	}
	if err := f.st.AddRepoStat(store.RepoStat{Domain: "vms", Source: "local", At: 100, Snapshots: 4}); err != nil {
		t.Fatal(err)
	}
	if facts, err := f.svc.domainPathFacts(settings, "vms"); err != nil || !facts.Established || facts.Snapshots != 4 {
		t.Fatalf("a measured domain path = %+v, %v, want 4 snapshots", facts, err)
	}
}

func TestALocalDomainPathOnceCreatedIsEstablished(t *testing.T) {
	f := newPlacementFixture(t)
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	loc, err := f.svc.resolveRepo(settings.FilesPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.MarkRepoEstablished(loc); err != nil {
		t.Fatal(err)
	}
	if facts, err := f.svc.domainPathFacts(settings, "files"); err != nil || !facts.Established {
		t.Fatalf("a created domain path = %+v, %v", facts, err)
	}
}

func TestAMoveToAnEmptyAddressIsAllowedWhenNothingLiesAtTheOldOne(t *testing.T) {
	f := newPlacementFixture(t)
	move := addressMove{Domain: "vms", Old: "old/vms", New: "new/vms"}
	if err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
}

func TestAMoveAwayFromBackupsIsRefusedWithTheirCount(t *testing.T) {
	f := newPlacementFixture(t)
	move := addressMove{Domain: "containers", Old: "old/containers", New: "new/containers",
		Facts: addressFacts{Established: true, Snapshots: 12}}
	err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{})
	var refused *placeEstablishedErr
	if !errors.As(err, &refused) || refused.snapshots != 12 || !slices.Equal(refused.domains, []string{"containers"}) ||
		placementCode(err) != "place-location-established" {
		t.Fatalf("checkMoves = %v, want place-location-established with 12 snapshots of containers", err)
	}
}

func TestAMoveOntoTheSameRepositoryIsAHandMadeMove(t *testing.T) {
	f := newPlacementFixture(t)
	f.eng.ids[f.localRepo("old/containers")] = "r1"
	f.eng.ids[f.localRepo("new/containers")] = "r1"
	move := addressMove{Domain: "containers", Old: "old/containers", New: "new/containers",
		Facts: addressFacts{Established: true, Snapshots: 12}}
	if err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
}

func TestAMoveOntoAnotherRepositoryIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	f.eng.ids[f.localRepo("old/containers")] = "r1"
	f.eng.ids[f.localRepo("new/containers")] = "r2"
	for _, facts := range []addressFacts{{Established: true}, {}} {
		move := addressMove{Domain: "containers", Old: "old/containers", New: "new/containers", Facts: facts}
		if err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{}); placementCode(err) != "place-location-established" {
			t.Errorf("with %+v at the old address: checkMoves = %v, want place-location-established", facts, err)
		}
	}
}

func TestEachMoveIsJudgedOnItsOwn(t *testing.T) {
	f := newPlacementFixture(t)
	f.eng.ids[f.localRepo("old/containers")] = "r1"
	f.eng.ids[f.localRepo("new/containers")] = "r1"
	moves := []addressMove{
		{Domain: "containers", Old: "old/containers", New: "new/containers", Facts: addressFacts{Established: true}},
		{Domain: "vms", Old: "old/vms", New: "new/vms"},
	}
	if err := f.svc.checkMoves(context.Background(), moves, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
}

func TestAnUnreachableNewAddressIsAProbeFailure(t *testing.T) {
	f := newPlacementFixture(t)
	addr := "s3:https://s3.example.com/bucket/container"
	f.eng.opens[addr] = false
	f.eng.openErr[addr] = errors.New("dial tcp 203.0.113.9:443: connect: connection refused")
	move := addressMove{Domain: "containers", Old: "s3:https://s3.example.com/old/container", New: addr}
	if err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{}); placementCode(err) != "place-probe-failed" {
		t.Fatalf("checkMoves = %v, want place-probe-failed", err)
	}
}

func TestANewFolderHoldingOtherFilesIsAProbeFailure(t *testing.T) {
	f := newPlacementFixture(t)
	dir := filepath.Join(filepath.FromSlash(f.root), "new", "vms")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	move := addressMove{Domain: "vms", Old: "old/vms", New: "new/vms"}
	if err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{}); placementCode(err) != "place-probe-failed" {
		t.Fatalf("checkMoves = %v, want place-probe-failed", err)
	}
}

func TestAProbeFailureSaysOnceThatTheTestFailed(t *testing.T) {
	f := newPlacementFixture(t)
	dir := filepath.Join(filepath.FromSlash(f.root), "new", "vms")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	move := addressMove{Domain: "vms", Old: "old/vms", New: "new/vms"}

	err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{})

	var failed *probeFailedErr
	if !errors.As(err, &failed) || failed.result.Error != "this folder holds other files and no restic repository" ||
		strings.Count(err.Error(), errPlaceProbeFailed.Error()) != 1 {
		t.Fatalf("checkMoves = %v, want the reason once behind the failed test", err)
	}
}

func TestARefusedMoveNamesOnlyTheDomainsItConcerns(t *testing.T) {
	f := newPlacementFixture(t)
	// The repository a place is itself belongs to no domain.
	move := addressMove{Old: "old", New: "new", Facts: addressFacts{Established: true, Items: 1}}

	err := f.svc.checkMoves(context.Background(), []addressMove{move}, restic.Mode{})

	var refused *placeEstablishedErr
	if !errors.As(err, &refused) || len(refused.domains) != 0 {
		t.Fatalf("checkMoves = %#v, want place-location-established naming no domain", err)
	}
}

func (f *placementFixture) movesOf(before, after store.Place) ([]addressMove, error) {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	rows, err := f.st.PlaceRows(before.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	homes, err := f.st.DomainPlaces()
	if err != nil {
		f.t.Fatal(err)
	}
	locks := &domainLocks{s: f.svc}
	defer locks.release()
	return f.svc.placeMoves(settings, before, after, rows, homes, locks)
}

func TestAnEditListsTheAddressesItMoves(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers")
	f.placeTarget(unraid, "flash", "")
	after := unraid
	after.Folders = maps.Clone(unraid.Folders)
	after.Folders["containers"] = "ct"

	moves, err := f.movesOf(unraid, after)

	if err != nil || len(moves) != 1 || moves[0].Domain != "containers" || moves[0].Old != "backups/containers" || moves[0].New != "backups/ct" {
		t.Fatalf("moves = %+v, %v, want only the containers path", moves, err)
	}
}

func TestAnEditThatDropsTheFolderOfAHomeDomainIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	unraid := f.storePlace(localPlace("Unraid", "backups"), "containers")
	after := unraid
	after.Folders = maps.Clone(unraid.Folders)
	delete(after.Folders, "containers")
	if _, err := f.movesOf(unraid, after); placementCode(err) != "place-home-domain" {
		t.Fatalf("placeMoves = %v, want place-home-domain", err)
	}
}

func TestAnEditThatDropsTheFolderOfARowIsRefused(t *testing.T) {
	f := newPlacementFixture(t)
	b2 := f.storePlace(s3Place("B2", "s3:https://s3.example.com/bucket"))
	f.placeTarget(b2, "vms", "")
	after := b2
	after.Folders = maps.Clone(b2.Folders)
	delete(after.Folders, "vms")
	if _, err := f.movesOf(b2, after); placementCode(err) != "place-in-use" {
		t.Fatalf("placeMoves = %v, want place-in-use", err)
	}
}
