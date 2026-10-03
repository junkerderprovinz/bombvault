package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

type tagRecordingBlockEngine struct {
	ResticEngine
	tags    [][]string
	locked  int
	unlocks int
}

func (e *tagRecordingBlockEngine) BackupDirFrom(_ context.Context, _, _, _ string, tags []string, _ restic.Mode) (restic.Summary, error) {
	e.tags = append(e.tags, slices.Clone(tags))
	if e.locked > 0 {
		e.locked--
		return restic.Summary{}, errors.New("unable to create lock in backend: repository is already locked by PID 11 on bombvault")
	}
	return restic.Summary{}, nil
}

func (e *tagRecordingBlockEngine) Unlock(context.Context, string, bool, restic.Mode) error {
	e.unlocks++
	return nil
}

func (e *tagRecordingBlockEngine) LsTimes(context.Context, string, string, string, restic.Mode) ([]restic.NodeTime, error) {
	return nil, nil
}

func TestAChangedBlockBackupIntoADirectRepositoryCarriesTheDirectTag(t *testing.T) {
	eng := &tagRecordingBlockEngine{}
	r := &vmBlockRestic{engine: eng, extraTags: []string{restic.DirectTag}}
	if _, err := r.BackupDir(context.Background(), "repo", "dir", "", []string{"vm:win", "p2"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"vm:win", "p2", restic.DirectTag}
	if len(eng.tags) != 1 || !slices.Equal(eng.tags[0], want) {
		t.Fatalf("tags = %v, want %v", eng.tags, want)
	}
}

func TestAChangedBlockBackupClearsALockInItsWayAndTriesOnceMore(t *testing.T) {
	eng := &tagRecordingBlockEngine{locked: 1}
	r := &vmBlockRestic{engine: eng, svc: &Service{engine: eng}}
	if _, err := r.BackupDir(context.Background(), "s3:https://s3.example/bucket/vms", "dir", "", []string{"vm:win"}); err != nil {
		t.Fatal(err)
	}
	if len(eng.tags) != 2 || eng.unlocks != 1 {
		t.Fatalf("%d backups and %d unlocks, want 2 and 1", len(eng.tags), eng.unlocks)
	}
}

func TestAChangedBlockBackupUnlocksNothingWithoutALock(t *testing.T) {
	eng := &tagRecordingBlockEngine{}
	r := &vmBlockRestic{engine: eng, svc: &Service{engine: eng}}
	if _, err := r.BackupDir(context.Background(), "s3:https://s3.example/bucket/vms", "dir", "", []string{"vm:win"}); err != nil {
		t.Fatal(err)
	}
	if eng.unlocks != 0 {
		t.Fatalf("%d unlocks without a lock, want none", eng.unlocks)
	}
}
