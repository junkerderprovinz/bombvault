package api

import (
	"testing"
	"time"

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
