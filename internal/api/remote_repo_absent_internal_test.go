package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const bucketRepo = "s3:http://192.168.2.53:9000/bombvault/dxp6800/vms"

// bucketEngine answers the config probe with openErr and records prunes.
type bucketEngine struct {
	ResticEngine
	openErr error
	pruned  []string
}

func (e *bucketEngine) RepoOpensErr(context.Context, string, restic.Mode) error { return e.openErr }

func (e *bucketEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (e *bucketEngine) Prune(_ context.Context, repo string, _ restic.Mode) error {
	e.pruned = append(e.pruned, repo)
	return errors.New("restic prune failed")
}

func bucketSvc(t *testing.T, eng *bucketEngine) (*Service, *store.Repo) {
	t.Helper()
	st := newSourceSeamStore(t)
	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.VMsPath = bucketRepo
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	svc := &Service{store: st, engine: eng, progress: progress.NewStore()}
	svc.cfg.HostMountRoot = t.TempDir()
	svc.cfg.AppKey = strings.Repeat("a", 64)
	return svc, st
}

func TestPruneLeavesAnEmptyBucketAlone(t *testing.T) {
	eng := &bucketEngine{openErr: errors.New("Fatal: repository does not exist: unable to open config file: Stat: The specified key does not exist.")}
	svc, _ := bucketSvc(t, eng)

	_, err := svc.PruneDomain(context.Background(), "vms", "local")
	if len(eng.pruned) != 0 {
		t.Fatalf("pruned %v, a bucket that never held a repository has nothing to prune", eng.pruned)
	}
	if err == nil || err.Error() != "no backups to prune yet" {
		t.Fatalf("err = %v, want the not-yet sentence", err)
	}
}

func TestPruneReportsABucketThatLostItsRepository(t *testing.T) {
	eng := &bucketEngine{openErr: errors.New("Fatal: repository does not exist: unable to open config file: Stat: The specified key does not exist.")}
	svc, st := bucketSvc(t, eng)
	if err := st.MarkRepoEstablished(bucketRepo); err != nil {
		t.Fatal(err)
	}

	_, err := svc.PruneDomain(context.Background(), "vms", "local")
	if err == nil || !strings.Contains(err.Error(), "was there before") {
		t.Fatalf("err = %v, want the repository named as gone", err)
	}
}

func TestPruneStillTriesABucketItCannotReach(t *testing.T) {
	eng := &bucketEngine{openErr: errors.New("Fatal: unable to open config file: dial tcp 192.168.2.53:9000: connect: no route to host")}
	svc, _ := bucketSvc(t, eng)

	if _, err := svc.PruneDomain(context.Background(), "vms", "local"); err == nil {
		t.Fatal("prune succeeded against a bucket it could not reach")
	}
	if len(eng.pruned) != 1 {
		t.Fatalf("pruned %v, want the one attempt that reports the real failure", eng.pruned)
	}
}
