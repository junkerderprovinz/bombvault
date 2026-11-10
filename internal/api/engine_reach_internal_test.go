package api

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// keyedEngine is an engine with the keys and the id of a repository, the way
// restic.Restic has them beyond ResticEngine.
type keyedEngine struct {
	ResticEngine
}

func (keyedEngine) AddKey(context.Context, string, restic.Mode, string, string, string) error {
	return nil
}
func (keyedEngine) Keys(context.Context, string, restic.Mode) ([]restic.Key, error) { return nil, nil }
func (keyedEngine) RemoveKey(context.Context, string, restic.Mode, string) error    { return nil }
func (keyedEngine) RepoID(context.Context, string, restic.Mode) (string, error) {
	return "5f3c0a1e", nil
}

func TestTheKeysAndTheIDOfARepositoryReachThroughTheOwnDataGuard(t *testing.T) {
	s := NewService(config.Config{AppKey: oldKey}, nil, nil, nil, keyedEngine{})
	t.Cleanup(s.stopForeignJanitor)
	if _, err := s.sharedEngine(); err != nil {
		t.Errorf("the keys of a shared repository: %v", err)
	}
	if _, ok := engineAs[repoIdentifier](s.engine); !ok {
		t.Error("the id of a repository cannot be read")
	}
}

// blockEngine is an engine with the changed-block methods of restic.Restic.
type blockEngine struct {
	ResticEngine
}

func (blockEngine) BackupDirFrom(context.Context, string, string, string, []string, restic.Mode) (restic.Summary, error) {
	return restic.Summary{}, nil
}

func (blockEngine) LsTimes(context.Context, string, string, string, restic.Mode) ([]restic.NodeTime, error) {
	return nil, nil
}

func TestTheChangedBlockMethodsReachThroughTheOwnDataGuard(t *testing.T) {
	s := NewService(config.Config{AppKey: oldKey}, nil, nil, nil, blockEngine{})
	t.Cleanup(s.stopForeignJanitor)
	if _, ok := engineAs[vmBlockEngine](s.engine); !ok {
		t.Error("a VM cannot be read block by block behind the guard")
	}
	if _, ok := engineAs[sharedEngine](s.engine); ok {
		t.Error("an engine without keys hands out keys behind the guard")
	}
}
