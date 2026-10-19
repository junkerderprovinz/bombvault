package store

import (
	"slices"
	"testing"
)

func migrated(t *testing.T) *Repo {
	t.Helper()
	db := OpenMem(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func TestTheUpgradeLeavesTheSharedSignInDueForConversion(t *testing.T) {
	db := OpenMem(t)
	bootAs(t, db, signInsMigration-1)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	pending, err := r.SignInsPending()
	if err != nil || !pending {
		t.Fatalf("pending=%v (%v), want the conversion due", pending, err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := r.FinishSignIns(SignInsNote{}); err != nil {
		t.Fatal(err)
	}
	if pending, _ := r.SignInsPending(); pending {
		t.Fatal("the conversion stays due after it ran")
	}
}

func TestTheNoteShowsWhatTheConversionDidUntilItIsDismissed(t *testing.T) {
	r := migrated(t)
	if _, show, err := r.SignInsNoteToShow(); err != nil || show {
		t.Fatalf("shown=%v (%v) before the conversion ran", show, err)
	}
	want := SignInsNote{SetID: "set", Merged: true, Duplicates: 2, Linked: []string{"containers", "vms"}}
	if err := r.FinishSignIns(want); err != nil {
		t.Fatal(err)
	}
	got, show, err := r.SignInsNoteToShow()
	if err != nil || !show || got.SetID != want.SetID || !got.Merged || got.Duplicates != 2 || !slices.Equal(got.Linked, want.Linked) {
		t.Fatalf("note = %+v shown=%v (%v)", got, show, err)
	}
	if err := r.DismissSignInsNote(); err != nil {
		t.Fatal(err)
	}
	if _, show, _ := r.SignInsNoteToShow(); show {
		t.Fatal("a dismissed note still shows")
	}
}

func TestAConversionThatChangedNothingShowsNoNote(t *testing.T) {
	r := migrated(t)
	if err := r.FinishSignIns(SignInsNote{}); err != nil {
		t.Fatal(err)
	}
	if _, show, _ := r.SignInsNoteToShow(); show {
		t.Fatal("an empty note shows")
	}
}

func TestReplaceCredsRefMovesEveryRowToTheSignInThatStays(t *testing.T) {
	r := migrated(t)
	tg, err := r.UpsertOffsiteTarget(OffsiteTarget{Domain: "vms", Name: "A", Repo: "s3:h/b/vms", CredsRef: "old", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceCredsRef("old", "new"); err != nil {
		t.Fatal(err)
	}
	got, _, err := r.GetOffsiteTarget(tg.ID)
	if err != nil || got.CredsRef != "new" {
		t.Fatalf("creds_ref = %q (%v)", got.CredsRef, err)
	}
}

func TestMoveOwnFolderWritesThePathTheRowAndEndsTheCopyThere(t *testing.T) {
	r := migrated(t)
	if _, err := r.UpsertPrimaryRemoteTarget("containers", OffsiteTarget{Repo: "s3:old/containers", Immutable: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	copyThere, err := r.UpsertOffsiteTarget(OffsiteTarget{Domain: "containers", Name: "Garage", Repo: "s3:garage/bv/containers", Enabled: true, SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}

	err = r.MoveOwnFolder(OwnFolder{
		Domain:  "containers",
		Row:     &OffsiteTarget{Repo: "s3:garage/bv/containers", DestinationID: "dest", CredsRef: "set"},
		EndCopy: copyThere.ID,
	}, func(s *Settings) { s.ContainersPath = "s3:garage/bv/containers" })
	if err != nil {
		t.Fatal(err)
	}

	settings, _ := r.GetSettings()
	if settings.ContainersPath != "s3:garage/bv/containers" {
		t.Fatalf("path = %q", settings.ContainersPath)
	}
	row, ok, err := r.PrimaryRemoteTarget("containers")
	if err != nil || !ok || row.DestinationID != "dest" || row.CredsRef != "set" || row.Immutable || !row.Enabled {
		t.Fatalf("row = %+v found=%v (%v), want the new one alone", row, ok, err)
	}
	if _, ok, _ := r.GetOffsiteTarget(copyThere.ID); ok {
		t.Fatal("the copy that became the own folder is still copied to")
	}

	if err := r.MoveOwnFolder(OwnFolder{Domain: "containers"}, func(s *Settings) { s.ContainersPath = "user/bv/containers" }); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := r.PrimaryRemoteTarget("containers"); ok {
		t.Fatal("a folder on this host kept a remote-primary row")
	}
}
