package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/hostload"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func bottleneckService(t *testing.T) (*Service, *store.Repo, store.Target) {
	t.Helper()
	dir := t.TempDir()
	old := procDir
	procDir = dir
	t.Cleanup(func() { procDir = old })
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: dir}, st, nil, nil, nil)
	return svc, st, tg
}

// seedTimedRuns records successful backups whose restic took ms each, oldest
// first, and returns the id of the last.
func seedTimedRuns(t *testing.T, st *store.Repo, targetID string, ms ...int64) string {
	t.Helper()
	var id string
	for _, m := range ms {
		var err error
		if id, err = st.StartRun(targetID, "backup"); err != nil {
			t.Fatal(err)
		}
		if err := st.FinishRunMeasured(id, "success", "abcdef12", 1, "", &store.RunMetrics{SourceBytes: 1, ResticMS: m}, ""); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func busyDisk() hostload.Summary {
	cpu := 0.2
	return hostload.Summary{Samples: 40, CPU: &cpu, Disks: []hostload.DiskLoad{{Name: "sdf", Busy: 0.97}, {Name: "sdb", Busy: 0.3}}}
}

func storedLoad(t *testing.T, st *store.Repo, id string) runLoad {
	t.Helper()
	run, err := st.GetRun(id)
	if err != nil {
		t.Fatal(err)
	}
	var l runLoad
	if err := json.Unmarshal([]byte(run.Load), &l); err != nil {
		t.Fatalf("load %q: %v", run.Load, err)
	}
	return l
}

func TestASlowRunNamesTheOneSaturatedDisk(t *testing.T) {
	svc, st, tg := bottleneckService(t)
	id := seedTimedRuns(t, st, tg.ID, 60_000, 62_000, 58_000, 200_000)
	svc.recordRunLoad(id, busyDisk())
	l := storedLoad(t, st, id)
	if !l.Slow || l.Cause == nil || l.Cause.Kind != hostload.CauseDisk || l.Cause.Name != "sdf" {
		t.Fatalf("load %+v cause %+v", l, l.Cause)
	}
	if got := bottleneckSentence(l.Cause); got != "The disk sdf was 97% busy." {
		t.Fatalf("sentence %q", got)
	}
}

func TestARunAtTheUsualSpeedGetsNoCause(t *testing.T) {
	svc, st, tg := bottleneckService(t)
	id := seedTimedRuns(t, st, tg.ID, 60_000, 62_000, 58_000, 61_000)
	svc.recordRunLoad(id, busyDisk())
	l := storedLoad(t, st, id)
	if l.Slow || l.Cause != nil || l.Samples != 40 {
		t.Fatalf("load %+v", l)
	}
	if runBottleneck(mustRun(t, st, id).Load) != nil {
		t.Fatal("a normal run shows a bottleneck")
	}
}

func TestASlowRunWithoutEnoughHistoryGetsNoCause(t *testing.T) {
	svc, st, tg := bottleneckService(t)
	id := seedTimedRuns(t, st, tg.ID, 60_000, 200_000)
	svc.recordRunLoad(id, busyDisk())
	if l := storedLoad(t, st, id); l.Slow {
		t.Fatalf("load %+v", l)
	}
}

func TestAFewSecondsSlowerIsNotSlow(t *testing.T) {
	svc, st, tg := bottleneckService(t)
	id := seedTimedRuns(t, st, tg.ID, 2_000, 2_000, 2_000, 10_000)
	svc.recordRunLoad(id, busyDisk())
	if l := storedLoad(t, st, id); l.Slow {
		t.Fatalf("load %+v", l)
	}
}

func TestTheHooksMeasureABackupRunFromStartToFinish(t *testing.T) {
	svc, st, tg := bottleneckService(t)
	svc.load = hostload.NewSampler(procDir, t.TempDir(), 10*time.Millisecond)
	id, err := st.StartRun(tg.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := st.FinishRun(id, "success", "abcdef12", 1, ""); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for mustRun(t, st, id).Load == "" {
		if time.Now().After(deadline) {
			t.Fatal("no load was stored")
		}
		time.Sleep(20 * time.Millisecond)
	}
	other, err := st.StartRun(tg.ID, "prune")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := st.FinishRun(other, "success", "", 0, ""); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := mustRun(t, st, other).Load; got != "" {
		t.Fatalf("a prune was measured: %s", got)
	}
}

func TestBottleneckSentenceNamesTheRole(t *testing.T) {
	for c, want := range map[hostload.Cause]string{
		{Kind: hostload.CauseDisk, Name: "disk1", Role: hostload.RoleTarget, Share: 0.98}: "The target disk disk1 was 98% busy.",
		{Kind: hostload.CauseDisk, Name: "cache", Role: hostload.RoleSource, Share: 0.91}: "The source disk cache was 91% busy.",
		{Kind: hostload.CauseCPU, Share: 0.95}:                                            "The CPU was 95% busy.",
		{Kind: hostload.CauseCPULimit, Share: 0.97}:                                       "BombVault used 97% of the CPU limit of its container.",
		{Kind: hostload.CauseUpload, Share: 1}:                                            "The upload ran at 100% of the repository's upload limit.",
	} {
		if got := bottleneckSentence(&c); got != want {
			t.Errorf("%+v: %q, want %q", c, got, want)
		}
	}
}

func mustRun(t *testing.T, st *store.Repo, id string) store.Run {
	t.Helper()
	run, err := st.GetRun(id)
	if err != nil {
		t.Fatal(err)
	}
	return run
}
