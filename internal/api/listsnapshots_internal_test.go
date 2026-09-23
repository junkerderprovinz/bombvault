package api

import (
	"context"
	"errors"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

func TestIsRepoUninitialized(t *testing.T) {
	uninit := []error{
		errors.New("Fatal: unable to open config file: <config/> does not exist\nIs there a repository at the following location?"),
		errors.New("Fatal: repository does not exist: unable to open config file"),
		errors.New("REPOSITORY DOES NOT EXIST"), // case-insensitive
	}
	for _, e := range uninit {
		if !isRepoUninitialized(e) {
			t.Fatalf("expected uninitialized: %v", e)
		}
	}
	genuine := []error{
		nil,
		errors.New("Fatal: unable to open repository at rest:http://host: server response unexpected: 401 Unauthorized"),
		errors.New("Fatal: unable to open repository: dial tcp: lookup host: no such host"),
		errors.New("Fatal: wrong password or no key found"),
	}
	for _, e := range genuine {
		if isRepoUninitialized(e) {
			t.Fatalf("must NOT classify as uninitialized: %v", e)
		}
	}
}

// snapshotsStubEngine implements only Snapshots, which returns err.
type snapshotsStubEngine struct {
	ResticEngine
	err error
}

func (e snapshotsStubEngine) Snapshots(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, e.err
}

// A remote repo that does not exist yet lists as empty. A real remote error
// and a missing local repo are still returned.
func TestListSnapshotsRemoteUninitialized(t *testing.T) {
	notInit := errors.New("Fatal: unable to open config file: <config/> does not exist\nIs there a repository at the following location?\nrest:http://box:8000/flash")
	authErr := errors.New("Fatal: unable to open repository at rest:http://box:8000/flash: server response unexpected: 401 Unauthorized")

	t.Run("remote not-initialized -> empty", func(t *testing.T) {
		s := &Service{engine: snapshotsStubEngine{err: notInit}}
		snaps, err := s.listSnapshots(context.Background(), "rest:http://box:8000/flash", restic.Mode{})
		if err != nil || snaps != nil {
			t.Fatalf("remote uninitialized must yield (nil,nil), got snaps=%v err=%v", snaps, err)
		}
	})

	t.Run("remote auth error -> propagates", func(t *testing.T) {
		s := &Service{engine: snapshotsStubEngine{err: authErr}}
		_, err := s.listSnapshots(context.Background(), "rest:http://box:8000/flash", restic.Mode{})
		if err == nil {
			t.Fatal("a genuine remote auth error must propagate, not be masked as empty")
		}
	})

	t.Run("remote host out of reach -> propagates", func(t *testing.T) {
		dead := errors.New(`restic snapshots failed: Fatal: unable to open config file: Head "http://box:8000[path]": dial tcp: lookup box on 127.0.0.11:53: no such host`)
		s := &Service{engine: snapshotsStubEngine{err: dead}}
		_, err := s.listSnapshots(context.Background(), "rest:http://box:8000/flash", restic.Mode{})
		if err == nil {
			t.Fatal("a host that does not resolve holds no answer about snapshots and must not read as empty")
		}
	})

	t.Run("local uninitialized -> propagates (guarded upstream, not here)", func(t *testing.T) {
		s := &Service{engine: snapshotsStubEngine{err: notInit}}
		_, err := s.listSnapshots(context.Background(), "/mnt/user/backups/flash", restic.Mode{})
		if err == nil {
			t.Fatal("listSnapshots must not swallow a LOCAL repo error (only remotes are short-circuited here)")
		}
	})
}
