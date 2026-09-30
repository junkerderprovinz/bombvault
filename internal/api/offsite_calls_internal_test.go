package api

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// countingEngine counts the restic calls each repository gets, since every
// call to a paid backend such as B2 is billed.
type countingEngine struct {
	ResticEngine
	mu    sync.Mutex
	calls map[string]map[string]int // repository, then command
	// lockedOnce makes the first call of that command at a repository fail
	// with a lock conflict.
	lockedOnce map[string]bool
}

func countCalls(f *placementFixture) *countingEngine {
	c := &countingEngine{ResticEngine: f.eng, calls: map[string]map[string]int{}, lockedOnce: map[string]bool{}}
	f.svc.engine = c
	return c
}

func (c *countingEngine) count(repo, cmd string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := filepath.ToSlash(repo)
	if c.calls[key] == nil {
		c.calls[key] = map[string]int{}
	}
	c.calls[key][cmd]++
	if c.lockedOnce[cmd] && c.calls[key][cmd] == 1 {
		return errors.New("unable to create lock in backend: repository is already locked by PID 11 on bombvault")
	}
	return nil
}

func (c *countingEngine) at(repo string) map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[repo]
}

func (c *countingEngine) RepoOpens(ctx context.Context, repo string, m restic.Mode) bool {
	_ = c.count(repo, "cat config")
	return c.ResticEngine.RepoOpens(ctx, repo, m)
}

func (c *countingEngine) RepoOpensErr(ctx context.Context, repo string, m restic.Mode) error {
	_ = c.count(repo, "cat config")
	return c.ResticEngine.RepoOpensErr(ctx, repo, m)
}

func (c *countingEngine) Init(ctx context.Context, repo string, m restic.Mode) error {
	_ = c.count(repo, "init")
	return c.ResticEngine.Init(ctx, repo, m)
}

func (c *countingEngine) Unlock(ctx context.Context, repo string, all bool, m restic.Mode) error {
	_ = c.count(repo, "unlock")
	return c.ResticEngine.Unlock(ctx, repo, all, m)
}

func (c *countingEngine) Snapshots(ctx context.Context, repo string, m restic.Mode) ([]restic.Snapshot, error) {
	if err := c.count(repo, "snapshots"); err != nil {
		return nil, err
	}
	return c.ResticEngine.Snapshots(ctx, repo, m)
}

func (c *countingEngine) Copy(ctx context.Context, dest, src string, ids []string, lim restic.Limits, m restic.Mode) error {
	if err := c.count(dest, "copy"); err != nil {
		return err
	}
	return c.ResticEngine.Copy(ctx, dest, src, ids, lim, m)
}

func (c *countingEngine) ForgetPolicy(ctx context.Context, repo string, p restic.RetentionPolicy, m restic.Mode, tags []string, prune bool) error {
	if err := c.count(repo, "forget"); err != nil {
		return err
	}
	return c.ResticEngine.ForgetPolicy(ctx, repo, p, m, tags, prune)
}

func (c *countingEngine) Prune(ctx context.Context, repo string, m restic.Mode) error {
	if err := c.count(repo, "prune"); err != nil {
		return err
	}
	return c.ResticEngine.Prune(ctx, repo, m)
}

const b2Containers = "b2:bucket:containers"

// b2Scene is plex on the domain path with both of its snapshots already at B2,
// so a run finds nothing to send.
func b2Scene(t *testing.T) (*placementFixture, store.OffsiteTarget) {
	t.Helper()
	f := newPlacementFixture(t)
	b2 := f.target("containers", "B2", b2Containers)
	f.container("plex", "")
	f.replicated("containers")
	f.hold(f.domainPath("containers"), snap("p1", 100, "container:plex"), snap("p2", 200, "container:plex"))
	f.hold(b2Containers, copied("b1", "p1", 100, "container:plex"), copied("b2", "p2", 200, "container:plex"))
	return f, b2
}

func TestARunToAnExistingTargetNeitherProbesNorUnlocksIt(t *testing.T) {
	f, _ := b2Scene(t)
	c := countCalls(f)
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	got := c.at(b2Containers)
	for _, cmd := range []string{"cat config", "init", "unlock"} {
		if got[cmd] != 0 {
			t.Errorf("%s ran %d times at B2, want none", cmd, got[cmd])
		}
	}
	if got["snapshots"] != 1 {
		t.Errorf("B2 was listed %d times, want once", got["snapshots"])
	}
}

func TestAFirstRunCreatesTheTargetRepository(t *testing.T) {
	f, _ := b2Scene(t)
	f.hold(b2Containers)
	f.eng.opens[b2Containers] = false
	f.eng.listErr[b2Containers] = errors.New("Fatal: repository does not exist: unable to open config file")
	c := countCalls(f)
	if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
		t.Fatal(err)
	}
	if got := c.at(b2Containers); got["init"] != 1 || got["copy"] != 1 {
		t.Fatalf("calls at B2 = %v, want one init and then the copy", got)
	}
}

func TestALockInTheWayIsClearedOnceAndTheCallRetried(t *testing.T) {
	for _, cmd := range []string{"snapshots", "copy", "forget", "prune"} {
		t.Run(cmd, func(t *testing.T) {
			f, b2 := b2Scene(t)
			keepLast(t, f, b2, 1)
			f.hold(f.domainPath("containers"), snap("p1", 100, "container:plex"), snap("p2", 200, "container:plex"), snap("p3", 300, "container:plex"))
			c := countCalls(f)
			c.lockedOnce[cmd] = true
			if err := f.svc.ReplicateOffsite(context.Background(), "containers"); err != nil {
				t.Fatal(err)
			}
			got := c.at(b2Containers)
			if got["unlock"] != 1 || got[cmd] < 2 {
				t.Fatalf("calls at B2 = %v, want one unlock and %s run again", got, cmd)
			}
		})
	}
}

func (c *countingEngine) Backup(_ context.Context, repo string, _, _ []string, _ restic.Mode, _ ...string) (restic.Summary, error) {
	if err := c.count(repo, "backup"); err != nil {
		return restic.Summary{}, err
	}
	return restic.Summary{SnapshotID: "n1"}, nil
}

func (c *countingEngine) BackupStdin(_ context.Context, repo string, _ io.Reader, _ string, _ []string, _ restic.Mode) (restic.Summary, error) {
	if err := c.count(repo, "backup"); err != nil {
		return restic.Summary{}, err
	}
	return restic.Summary{SnapshotID: "n1"}, nil
}

func (c *countingEngine) Forget(ctx context.Context, repo string, ids []string, prune bool, m restic.Mode) error {
	if err := c.count(repo, "forget"); err != nil {
		return err
	}
	return c.ResticEngine.Forget(ctx, repo, ids, prune, m)
}

// directFiles is the file set docs, with one file, on the direct repository of
// a B2 target.
func directFiles(t *testing.T) (*placementFixture, store.FileSet, string) {
	t.Helper()
	f := newPlacementFixture(t)
	d := f.direct(f.target("files", "B2", "b2:bucket:files"))
	set := f.fileSet("docs", d.ID)
	if err := os.WriteFile(filepath.Join(filepath.FromSlash(f.root), "files", "docs", "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	return f, set, d.Repo
}

func TestABackupIntoADirectRepositoryUnlocksNothingWithoutALock(t *testing.T) {
	f, set, loc := directFiles(t)
	c := countCalls(f)
	if _, err := f.svc.BackupFileSet(context.Background(), set.ID); err != nil {
		t.Fatal(err)
	}
	if got := c.at(loc); got["backup"] != 1 || got["unlock"] != 0 {
		t.Fatalf("calls at the direct repository = %v, want one backup and no unlock", got)
	}
}

func TestALockInTheWayOfADirectBackupIsClearedOnceAndTheBackupRetried(t *testing.T) {
	f, set, loc := directFiles(t)
	c := countCalls(f)
	c.lockedOnce["backup"] = true
	if _, err := f.svc.BackupFileSet(context.Background(), set.ID); err != nil {
		t.Fatal(err)
	}
	if got := c.at(loc); got["backup"] != 2 || got["unlock"] != 1 {
		t.Fatalf("calls at the direct repository = %v, want one unlock and the backup run again", got)
	}
}

func TestADeleteAtATargetUnlocksNothingWithoutALock(t *testing.T) {
	f, b2 := b2Scene(t)
	c := countCalls(f)
	if err := f.svc.DeleteBackups(context.Background(), "plex", offsiteSourcePrefix+b2.ID); err != nil {
		t.Fatal(err)
	}
	if got := c.at(b2Containers); got["forget"] != 1 || got["unlock"] != 0 {
		t.Fatalf("calls at B2 = %v, want one forget and no unlock", got)
	}
}

func TestALockInTheWayOfADeleteAtATargetIsClearedOnceAndTheDeleteRetried(t *testing.T) {
	f, b2 := b2Scene(t)
	c := countCalls(f)
	c.lockedOnce["forget"] = true
	if err := f.svc.DeleteBackups(context.Background(), "plex", offsiteSourcePrefix+b2.ID); err != nil {
		t.Fatal(err)
	}
	if got := c.at(b2Containers); got["forget"] != 2 || got["unlock"] != 1 {
		t.Fatalf("calls at B2 = %v, want one unlock and the forget run again", got)
	}
}

func TestAStreamIntoARemoteRepositoryClearsStaleLocksFirst(t *testing.T) {
	f, _, loc := directFiles(t)
	c := countCalls(f)
	e := unlockOnLockErr{ResticEngine: c, s: f.svc}
	if _, err := e.BackupStdin(context.Background(), loc, strings.NewReader("disk"), "/sda", nil, restic.Mode{}); err != nil {
		t.Fatal(err)
	}
	if got := c.at(loc); got["unlock"] != 1 || got["backup"] != 1 {
		t.Fatalf("calls at the direct repository = %v, want the unlock before the one backup", got)
	}
}

func TestDeletingARowAtATargetUnlocksOnlyWhenALockIsInTheWay(t *testing.T) {
	for _, want := range []struct {
		locked           bool
		forgets, unlocks int
	}{{false, 1, 0}, {true, 2, 1}} {
		f, b2 := nginxAtHomeAndB2(t)
		c := countCalls(f)
		c.lockedOnce["forget"] = want.locked
		place := []placeDelete{{Place: offsiteSourcePrefix + b2.ID, SnapshotIDs: []string{"b1b1b1b1"}}}
		if _, _, err := f.svc.timelineDelete(context.Background(), "containers", "nginx", "a1a1a1a1", place); err != nil {
			t.Fatal(err)
		}
		if got := c.at(b2.Repo); got["forget"] != want.forgets || got["unlock"] != want.unlocks {
			t.Fatalf("locked=%v: calls at B2 = %v, want %d forget and %d unlock", want.locked, got, want.forgets, want.unlocks)
		}
	}
}

func TestDeletingASnapshotAtATargetUnlocksOnlyWhenALockIsInTheWay(t *testing.T) {
	for _, want := range []struct {
		locked           bool
		forgets, unlocks int
	}{{false, 1, 0}, {true, 2, 1}} {
		f, b2 := nginxAtHomeAndB2(t)
		c := countCalls(f)
		c.lockedOnce["forget"] = want.locked
		if err := f.svc.DeleteSnapshot(context.Background(), "containers", "b1b1b1b1", offsiteSourcePrefix+b2.ID); err != nil {
			t.Fatal(err)
		}
		if got := c.at(b2.Repo); got["forget"] != want.forgets || got["unlock"] != want.unlocks {
			t.Fatalf("locked=%v: calls at B2 = %v, want %d forget and %d unlock", want.locked, got, want.forgets, want.unlocks)
		}
	}
}
