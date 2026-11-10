package api

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	towerHost = "bv-0123456789ab"
	cubeHost  = "bv-ba9876543210"
)

// sharedEngine is one shared repository as restic would hold it: it selects
// by host and tag as restic does and ages a selection as one group.
type sharedEngine struct {
	ResticEngine
	mu        sync.Mutex
	snaps     []restic.Snapshot
	forgets   [][]string
	previews  []string
	prunes    []restic.Mode
	checks    []int
	whole     int64
	byHost    map[string]int64
	onPrune   func()
	pruneErr  error
	measured  int
	statsErr  error
	callOrder []string
}

func (e *sharedEngine) SnapshotsOfEveryHost(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.snaps), nil
}

func (e *sharedEngine) ForgetPreview(_ context.Context, _ string, p restic.RetentionPolicy, m restic.Mode, tags ...string) ([]restic.ForgetGroup, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.previews = append(e.previews, m.Host+" "+strings.Join(tags, ","))
	var group []restic.Snapshot
	for _, sn := range e.snaps {
		if (m.Sharing == restic.SharingNone || sn.Hostname == m.Host) && slices.ContainsFunc(tags, func(t string) bool { return slices.Contains(sn.Tags, t) }) {
			group = append(group, sn)
		}
	}
	removed, ok := p.Forgets(group)
	if !ok {
		return nil, errors.New("cannot order the group")
	}
	g := restic.ForgetGroup{Tags: tags, Host: m.Host}
	for _, sn := range group {
		if removed[sn.ID] {
			g.Remove = append(g.Remove, sn)
		} else {
			g.Keep = append(g.Keep, sn)
		}
	}
	return []restic.ForgetGroup{g}, nil
}

func (e *sharedEngine) Forget(_ context.Context, _ string, ids []string, _ bool, m restic.Mode) error {
	if m.Sharing == restic.SharingMember {
		return restic.ErrSharedMemberPrunes
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.forgets = append(e.forgets, slices.Clone(ids))
	e.callOrder = append(e.callOrder, "forget")
	e.snaps = slices.DeleteFunc(e.snaps, func(sn restic.Snapshot) bool { return slices.Contains(ids, sn.ID) })
	return nil
}

func (e *sharedEngine) Prune(_ context.Context, _ string, m restic.Mode) error {
	if m.Sharing == restic.SharingMember {
		return restic.ErrSharedMemberPrunes
	}
	if e.onPrune != nil {
		e.onPrune()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prunes = append(e.prunes, m)
	e.callOrder = append(e.callOrder, "prune")
	return e.pruneErr
}

func (e *sharedEngine) CheckData(_ context.Context, _ string, subset int, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checks = append(e.checks, subset)
	e.callOrder = append(e.callOrder, "check")
	return nil
}

func (e *sharedEngine) Stats(_ context.Context, _, _ string, m restic.Mode) (restic.StatsResult, error) {
	return restic.StatsResult{TotalSize: e.byHost[m.Host]}, nil
}

func (e *sharedEngine) StatsOfEveryHost(context.Context, string, string, restic.Mode) (restic.StatsResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.measured++
	return restic.StatsResult{TotalSize: e.whole}, e.statsErr
}

func (e *sharedEngine) left() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	ids := make([]string, 0, len(e.snaps))
	for _, sn := range e.snaps {
		ids = append(ids, sn.ID)
	}
	slices.Sort(ids)
	return ids
}

// snapAt is a snapshot of tag under host, taken on day of January.
func snapAt(id, host, tag string, day int) restic.Snapshot {
	return restic.Snapshot{ID: id, Hostname: host, Tags: []string{tag}, Time: time.Date(2026, time.January, day, 3, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)}
}

// fakeMembers answers for the members of a shared repository the way they
// would through the group.
type fakeMembers struct {
	mu          sync.Mutex
	held        map[string][]string
	unreachable map[string]bool
	// busy is how many more times a member answers that it is still copying,
	// -1 for always.
	busy     map[string]int
	quieted  []string
	released []string
	told     map[string]store.SharedSpace
}

func (f *fakeMembers) heldBy(_ context.Context, m store.SharedMember) ([]string, error) {
	if f.unreachable[m.Host] {
		return nil, errMemberSilent
	}
	return f.held[m.Host], nil
}

func (f *fakeMembers) quiet(_ context.Context, m store.SharedMember, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.unreachable[m.Host] {
		return false, errMemberSilent
	}
	if n := f.busy[m.Host]; n != 0 {
		if n > 0 {
			f.busy[m.Host] = n - 1
		}
		return true, nil
	}
	f.quieted = append(f.quieted, m.Host)
	return false, nil
}

func (f *fakeMembers) release(_ context.Context, m store.SharedMember) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, m.Host)
}

func (f *fakeMembers) tellSpace(_ context.Context, m store.SharedMember, space store.SharedSpace) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.told == nil {
		f.told = map[string]store.SharedSpace{}
	}
	f.told[m.Host] = space
}

func upkeepService(t *testing.T, eng *sharedEngine) *Service {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	s := NewService(config.Config{AppKey: strings.Repeat("a", 64), DataDir: t.TempDir(), HostMountRoot: t.TempDir()}, st, nil, nil, eng)
	t.Cleanup(s.StopGroup)
	return s
}

// holdSharedRepo makes s the owner of a shared repository the given members
// back up into, keeping every member's last snapshot.
func holdSharedRepo(t *testing.T, s *Service, members ...store.SharedMember) store.SharedRepo {
	t.Helper()
	g, err := s.store.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	sr, err := s.store.PutSharedRepo(store.SharedRepo{Repo: "backups/shared", Owner: g.InstanceID, Members: members})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, KeepSet: true, Keep: store.RetentionKeep{KeepLast: 1},
		PruneCadence: "weekly sun 03:00", PruneHours: 6, MaxUnused: 5, CheckSubset: 5}); err != nil {
		t.Fatal(err)
	}
	return sr
}

var (
	tower = store.SharedMember{Host: towerHost, Instance: "tower-id", Name: "Tower"}
	cube  = store.SharedMember{Host: cubeHost, Instance: "cube-id", Name: "Cube"}
)

func forgetPass(t *testing.T, s *Service, sr store.SharedRepo, members sharedMembers) store.SharedForgetReport {
	t.Helper()
	u, err := s.store.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s.forgetShared(context.Background(), sr, u, members, time.Now())
}

func TestTheOwnerAgesEachMembersHistoryOfAnEntryApart(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{
		snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2),
		snapAt("c1", cubeHost, "container:plex", 3), snapAt("c2", cubeHost, "container:plex", 4),
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)

	r := forgetPass(t, s, sr, &fakeMembers{})
	if got := eng.left(); !slices.Equal(got, []string{"c2", "t2"}) {
		t.Fatalf("left %v, want the newest of each server", got)
	}
	if len(eng.forgets) != 1 || r.Forgotten != 2 || r.Error != "" {
		t.Fatalf("forget calls %v, report %+v; want both old snapshots in one call", eng.forgets, r)
	}
}

func TestAnEntryAMemberHoldsKeepsItsWholeHistory(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{
		snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2),
		snapAt("c1", cubeHost, "container:plex", 3), snapAt("c2", cubeHost, "container:plex", 4),
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)

	forgetPass(t, s, sr, &fakeMembers{held: map[string][]string{cubeHost: {"container:plex"}}})
	if got := eng.left(); !slices.Equal(got, []string{"c1", "c2", "t2"}) {
		t.Fatalf("left %v, want all of Cube's and Tower's newest", got)
	}
}

func TestAMemberThatCannotSayWhatItHoldsKeepsItsWholeHistory(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{
		snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2),
		snapAt("c1", cubeHost, "fileset:docs", 3), snapAt("c2", cubeHost, "fileset:docs", 4),
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)

	r := forgetPass(t, s, sr, &fakeMembers{unreachable: map[string]bool{cubeHost: true}})
	if got := eng.left(); !slices.Equal(got, []string{"c1", "c2", "t2"}) {
		t.Fatalf("left %v, want all of Cube's", got)
	}
	if !slices.Equal(r.Skipped, []string{"Cube"}) {
		t.Fatalf("skipped %v, want Cube", r.Skipped)
	}
}

func TestSnapshotsUnderAHostNoMemberWritesAreNeverForgotten(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{
		snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2),
		snapAt("l1", restic.LegacyHost, "container:plex", 3), snapAt("l2", restic.LegacyHost, "container:plex", 4),
		snapAt("x1", "bv-ffffffffffff", "container:plex", 5), snapAt("x2", "bv-ffffffffffff", "container:plex", 6),
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)

	r := forgetPass(t, s, sr, &fakeMembers{})
	if got := eng.left(); !slices.Equal(got, []string{"l1", "l2", "t2", "x1", "x2"}) {
		t.Fatalf("left %v, want everything of the unknown hosts", got)
	}
	if !slices.Equal(r.Unknown, []string{"bombvault", "bv-ffffffffffff"}) {
		t.Fatalf("unknown %v", r.Unknown)
	}
	for _, p := range eng.previews {
		if !strings.HasPrefix(p, towerHost+" ") {
			t.Fatalf("asked restic about %q, want only about Tower", p)
		}
	}
}

// Until the owner saves the card of the repository, each section ages by the
// owner's own off-site rules for it.
func TestEachSectionAgesByTheOwnersOffSiteRulesUntilTheCardIsSaved(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{
		snapAt("p1", towerHost, "container:plex", 1), snapAt("p2", towerHost, "container:plex", 2), snapAt("p3", towerHost, "container:plex", 3),
		snapAt("d1", towerHost, "fileset:docs", 1), snapAt("d2", towerHost, "fileset:docs", 2), snapAt("d3", towerHost, "fileset:docs", 3),
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)
	if err := s.store.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, PruneCadence: "weekly sun 03:00", PruneHours: 6}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.MutateSettings(func(st *store.Settings) error {
		st.OffsiteRetentionKeepLast = 2
		st.SetOwnOffsiteRetention(map[string]store.RetentionKeep{"files": {KeepLast: 1}})
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	forgetPass(t, s, sr, &fakeMembers{})
	if got := eng.left(); !slices.Equal(got, []string{"d3", "p2", "p3"}) {
		t.Fatalf("left %v, want two of the container and one of the folder set", got)
	}
}

func TestOnlyTheOwnerKeepsASharedRepository(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{
		snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2),
	}}
	s := upkeepService(t, eng)
	sr, err := s.store.PutSharedRepo(store.SharedRepo{Repo: "rest:http://nas:8001/shared", Owner: "nas-id", Members: []store.SharedMember{tower}})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.keepSharedRepo(context.Background(), sr, &fakeMembers{}, time.Now(), true); !errors.Is(err, errNotSharedOwner) {
		t.Fatalf("err = %v, want errNotSharedOwner", err)
	}
	if len(eng.forgets) != 0 || len(eng.prunes) != 0 || len(eng.previews) != 0 {
		t.Fatalf("a member forgot %v, pruned %d times and asked %v", eng.forgets, len(eng.prunes), eng.previews)
	}
}

// quickBusyWait lets the owner ask a busy member again at once and give up
// soon after.
func quickBusyWait(t *testing.T) {
	t.Helper()
	poll, wait := sharedBusyPoll, sharedBusyWait
	sharedBusyPoll, sharedBusyWait = time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { sharedBusyPoll, sharedBusyWait = poll, wait })
}

func prunePass(t *testing.T, s *Service, sr store.SharedRepo, members sharedMembers) store.SharedPruneReport {
	t.Helper()
	u, err := s.store.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s.pruneShared(context.Background(), sr, u, members, time.Now().Add(time.Hour))
}

func TestThePruneWaitsForAMemberThatIsStillCopying(t *testing.T) {
	quickBusyWait(t)
	eng := &sharedEngine{}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)
	members := &fakeMembers{busy: map[string]int{cubeHost: 2}}

	r := prunePass(t, s, sr, members)
	if len(eng.prunes) != 1 || len(r.Postponed) != 0 || r.Error != "" {
		t.Fatalf("pruned %d times, report %+v; want one prune once Cube was done", len(eng.prunes), r)
	}
	slices.Sort(members.quieted)
	slices.Sort(members.released)
	if want := []string{towerHost, cubeHost}; !slices.Equal(members.quieted, want) || !slices.Equal(members.released, want) {
		t.Fatalf("quieted %v, released %v; want both %v", members.quieted, members.released, want)
	}
}

func TestAMemberStillCopyingWhenTheWaitIsOverMovesThePruneToTheNextWindow(t *testing.T) {
	quickBusyWait(t)
	eng := &sharedEngine{}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)
	members := &fakeMembers{busy: map[string]int{cubeHost: -1}}

	r := prunePass(t, s, sr, members)
	if len(eng.prunes) != 0 || !slices.Equal(r.Postponed, []string{"Cube"}) {
		t.Fatalf("pruned %d times, report %+v; want no prune and Cube named", len(eng.prunes), r)
	}
	if !slices.Equal(members.released, []string{towerHost}) {
		t.Fatalf("released %v, want Tower, which had agreed", members.released)
	}
}

func TestAMemberThatDoesNotAnswerHoldsUpNoPrune(t *testing.T) {
	quickBusyWait(t)
	eng := &sharedEngine{}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)

	if r := prunePass(t, s, sr, &fakeMembers{unreachable: map[string]bool{cubeHost: true}}); len(eng.prunes) != 1 || r.Error != "" {
		t.Fatalf("pruned %d times, report %+v; want one prune", len(eng.prunes), r)
	}
}

func TestTheOwnerChecksWhatItPrunedAsTheRulesSay(t *testing.T) {
	eng := &sharedEngine{}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)
	if err := s.store.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, PruneCadence: "weekly sun 03:00", PruneHours: 6, MaxUnused: 10, CheckSubset: 7}); err != nil {
		t.Fatal(err)
	}

	r := prunePass(t, s, sr, &fakeMembers{})
	if !slices.Equal(eng.callOrder, []string{"prune", "check"}) || eng.prunes[0].MaxUnused != 10 || !slices.Equal(eng.checks, []int{7}) {
		t.Fatalf("calls %v, prune %+v, checks %v; want a prune leaving 10%% and a check of 7%%", eng.callOrder, eng.prunes, eng.checks)
	}
	if !r.Checked || r.CheckError != "" || eng.prunes[0].Sharing != restic.SharingOwner {
		t.Fatalf("report %+v, mode %+v", r, eng.prunes[0])
	}
}

func TestAFailedPruneIsNotCheckedAndSaysWhy(t *testing.T) {
	eng := &sharedEngine{pruneErr: errors.New("disk full")}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)

	r := prunePass(t, s, sr, &fakeMembers{})
	if len(eng.checks) != 0 || r.Checked || !strings.Contains(r.Error, "disk full") {
		t.Fatalf("checks %v, report %+v", eng.checks, r)
	}
}

func TestTheSpaceOfASharedRepositoryCountsEachMemberApart(t *testing.T) {
	eng := &sharedEngine{whole: 1200, byHost: map[string]int64{towerHost: 1000, cubeHost: 900}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower, cube)
	members := &fakeMembers{}

	space, err := s.measureShared(context.Background(), sr, members, time.Unix(500, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := store.SharedSpace{At: 500, Total: 1200, Hosts: []store.SharedHostSpace{{Host: towerHost, Name: "Tower", Bytes: 1000}, {Host: cubeHost, Name: "Cube", Bytes: 900}}}
	u, err := s.store.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc([]store.SharedSpace{space, *u.Space}, []store.SharedSpace{want, want}, sameSpace) {
		t.Fatalf("measured %+v, kept %+v; want %+v", space, u.Space, want)
	}
	if len(members.told) != 2 || !sameSpace(members.told[cubeHost], want) {
		t.Fatalf("told %+v, want both members", members.told)
	}
}

func sameSpace(a, b store.SharedSpace) bool {
	return a.At == b.At && a.Total == b.Total && slices.Equal(a.Hosts, b.Hosts)
}

func TestThePruneRunsOnceInEachWindowAndTheForgetOnceADay(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2)}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)
	if err := s.store.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, KeepSet: true, Keep: store.RetentionKeep{KeepLast: 5},
		PruneCadence: "weekly tue 03:00", PruneHours: 6, MaxUnused: 5, CheckSubset: 5}); err != nil {
		t.Fatal(err)
	}
	// The 10th and the 17th of March 2026 are Tuesdays.
	day := func(d, h int) time.Time { return time.Date(2026, time.March, d, h, 0, 0, 0, time.Local) }
	for _, step := range []struct {
		at               time.Time
		prunes, previews int
	}{
		{day(10, 4), 1, 1},
		{day(10, 5), 1, 1},
		{day(10, 11), 1, 1},
		{day(11, 5), 1, 2},
		{day(11, 12), 1, 2},
		{day(17, 4), 2, 3},
	} {
		if err := s.keepSharedRepo(context.Background(), sr, &fakeMembers{}, step.at, false); err != nil {
			t.Fatal(err)
		}
		if len(eng.prunes) != step.prunes || len(eng.previews) != step.previews {
			t.Fatalf("at %v: %d prunes and %d forget passes, want %d and %d", step.at, len(eng.prunes), len(eng.previews), step.prunes, step.previews)
		}
	}
}

func TestPruningNowOpensAWindowAtOnce(t *testing.T) {
	eng := &sharedEngine{}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)

	if err := s.keepSharedRepo(context.Background(), sr, &fakeMembers{}, time.Date(2026, time.March, 10, 12, 0, 0, 0, time.Local), true); err != nil {
		t.Fatal(err)
	}
	if len(eng.prunes) != 1 {
		t.Fatalf("%d prunes, want one", len(eng.prunes))
	}
}

func TestTheScheduledUpkeepKeepsOnlyTheRepositoriesThisServerHolds(t *testing.T) {
	eng := &sharedEngine{snaps: []restic.Snapshot{snapAt("t1", towerHost, "container:plex", 1), snapAt("t2", towerHost, "container:plex", 2)}}
	s := upkeepService(t, eng)
	holdSharedRepo(t, s, tower)
	if _, err := s.store.PutSharedRepo(store.SharedRepo{Repo: "rest:http://nas:8001/shared", Owner: "nas-id", Members: []store.SharedMember{tower}}); err != nil {
		t.Fatal(err)
	}

	if err := s.runSharedUpkeep(context.Background(), &fakeMembers{}, time.Date(2026, time.March, 11, 12, 0, 0, 0, time.Local)); err != nil {
		t.Fatal(err)
	}
	if got := eng.left(); !slices.Equal(got, []string{"t2"}) || len(eng.previews) != 1 {
		t.Fatalf("left %v after %d forget passes, want t2 after one", got, len(eng.previews))
	}
}

func TestOneUpkeepOfTheSharedRepositoriesRunsAtATime(t *testing.T) {
	pruning, done := make(chan struct{}), make(chan struct{})
	eng := &sharedEngine{onPrune: func() {
		close(pruning)
		<-done
	}}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)
	ctx := context.Background()
	first := make(chan error, 1)
	go func() { first <- s.runSharedUpkeepNow(ctx, sr.ID, &fakeMembers{}) }()
	<-pruning

	if err := s.runSharedUpkeep(ctx, &fakeMembers{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.runSharedUpkeepNow(ctx, sr.ID, &fakeMembers{}); !errors.Is(err, errSharedUpkeepBusy) {
		t.Fatalf("err = %v, want errSharedUpkeepBusy", err)
	}
	close(done)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if len(eng.prunes) != 1 {
		t.Fatalf("%d prunes, want one", len(eng.prunes))
	}
}

func TestTheNextUpkeepIsTheSoonerOfTheDailyForgetAndTheNextWindow(t *testing.T) {
	eng := &sharedEngine{}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)
	if _, ok := upkeepService(t, &sharedEngine{}).NextSharedUpkeep(time.Now()); ok {
		t.Fatal("a server that holds no shared repository has an upkeep to come")
	}
	// The 10th of March 2026 is a Tuesday, the window opens on Sunday the 15th.
	now := time.Date(2026, time.March, 10, 12, 0, 0, 0, time.Local)
	if err := s.store.NoteSharedForget(sr.ID, store.SharedForgetReport{At: now.Add(-2 * time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	if next, ok := s.NextSharedUpkeep(now); !ok || !next.Equal(now.Add(22*time.Hour)) {
		t.Fatalf("next %v, %v; want the forget a day after the last", next, ok)
	}
	if err := s.store.NoteSharedForget(sr.ID, store.SharedForgetReport{At: now.Add(5 * 24 * time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	if next, ok := s.NextSharedUpkeep(now); !ok || !next.Equal(time.Date(2026, time.March, 15, 3, 0, 0, 0, time.Local)) {
		t.Fatalf("next %v, %v; want Sunday's window", next, ok)
	}
}

// Measuring reads every tree of the repository, so a tick that has nothing
// else to do leaves it to the daily forget, even while there are no numbers.
func TestANewSharedRepositoryIsMeasuredWithItsForgetAndNotOnEveryTick(t *testing.T) {
	eng := &sharedEngine{statsErr: errors.New("the repository is busy")}
	s := upkeepService(t, eng)
	sr := holdSharedRepo(t, s, tower)
	ctx := context.Background()
	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.Local)

	for _, later := range []time.Duration{0, time.Minute, 2 * time.Minute} {
		_ = s.keepSharedRepo(ctx, sr, &fakeMembers{}, now.Add(later), false)
	}
	if eng.measured != 1 {
		t.Fatalf("measured %d times, want once with the first forget", eng.measured)
	}
}
