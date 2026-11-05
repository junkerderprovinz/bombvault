package store_test

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func finishedBackup(t *testing.T, st *store.Repo, target, snapshot, note string) string {
	t.Helper()
	id, err := st.StartRun(target, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(id, "success", snapshot, 1, note); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTheLastCompleteBackupIsNotOneThatSawLess(t *testing.T) {
	st := newRepo(t)
	finishedBackup(t, st, "t1", "aa11", "")
	finishedBackup(t, st, "t1", "bb22", store.NoteBackupEmpty+"; "+store.NoteOlderBackupsKept)

	run, err := st.LastCompleteBackup("t1")
	if err != nil {
		t.Fatal(err)
	}
	if run == nil || run.SnapshotID != "aa11" {
		t.Fatalf("last complete backup = %+v, want aa11", run)
	}
	if none, err := st.LastCompleteBackup("t2"); err != nil || none != nil {
		t.Fatalf("a target without runs answers %+v, %v", none, err)
	}
}

func TestOnlyTheNewestSuccessDecidesWhetherOldBackupsAreHeld(t *testing.T) {
	st := newRepo(t)
	finishedBackup(t, st, "t1", "aa11", store.NoteBackupEmpty+"; "+store.NoteOlderBackupsKept)
	held, err := st.TargetsWithOlderBackupsKept()
	if err != nil || !held["t1"] {
		t.Fatalf("held = %v, %v, want t1", held, err)
	}
	finishedBackup(t, st, "t1", "bb22", "")
	if held, err = st.TargetsWithOlderBackupsKept(); err != nil || held["t1"] {
		t.Fatalf("held = %v, %v, after a complete backup", held, err)
	}
}

func TestARunNoteGoesInFrontOfWhatTheRunSays(t *testing.T) {
	st := newRepo(t)
	plain := finishedBackup(t, st, "t1", "aa11", "")
	noted := finishedBackup(t, st, "t2", "bb22", "said before")
	for id, want := range map[string]string{plain: "new", noted: "new; said before"} {
		if err := st.AddRunNote(id, "new"); err != nil {
			t.Fatal(err)
		}
		run, err := st.GetRun(id)
		if err != nil {
			t.Fatal(err)
		}
		if run.Error != want {
			t.Fatalf("note = %q, want %q", run.Error, want)
		}
	}
}
