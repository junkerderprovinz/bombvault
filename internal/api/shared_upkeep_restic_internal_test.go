package api

import (
	"context"
	"errors"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const strangerHost = "bv-ffffffffffff"

// sharedRestic is a shared repository with real restic that Tower, Cube, a
// host no member writes under and the legacy host each backed up the same
// container and folder set into, three times over.
func sharedRestic(t *testing.T) (*resticFixture, store.SharedRepo, string, restic.Mode) {
	t.Helper()
	f := newResticFixture(t)
	g, err := f.st.GetGroupState()
	if err != nil {
		t.Fatal(err)
	}
	sr, err := f.st.PutSharedRepo(store.SharedRepo{Repo: "backups/shared", Owner: g.InstanceID, Members: []store.SharedMember{tower, cube}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.PutSharedUpkeepRules(store.SharedUpkeep{RepoID: sr.ID, KeepSet: true, Keep: store.RetentionKeep{KeepLast: 1},
		PruneCadence: "weekly sun 03:00", PruneHours: 6, MaxUnused: 0, CheckSubset: 100}); err != nil {
		t.Fatal(err)
	}
	dest, mode, _, err := f.svc.sharedAccess(sr)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := f.svc.engine.Init(ctx, dest, mode); err != nil {
		t.Fatal(err)
	}
	// The library is the same on every server and large enough next to the
	// rest that what dedup saves shows in the numbers.
	library := make([]byte, 256<<10)
	_, _ = rand.NewChaCha8([32]byte{1}).Read(library)
	for round := range 3 {
		f.write("plex", map[string]string{"config.xml": "plex " + strconv.Itoa(round), "library.db": string(library)})
		f.write("docs", map[string]string{"letter.txt": "docs " + strconv.Itoa(round)})
		for _, host := range []string{towerHost, cubeHost, strangerHost, restic.LegacyHost} {
			m := mode
			m.Host = host
			for dir, tag := range map[string]string{"plex": "container:plex", "docs": "fileset:docs"} {
				src := filepath.Join(filepath.FromSlash(f.root), "src", dir)
				if _, err := f.svc.engine.BackupDir(ctx, dest, src, []string{tag}, m); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return f, sr, dest, mode
}

// countByHost counts the snapshots of each host and tag in the repository.
func countByHost(t *testing.T, f *resticFixture, dest string, mode restic.Mode) map[string]int {
	t.Helper()
	snaps, err := f.svc.engine.SnapshotsOfEveryHost(context.Background(), dest, mode)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, sn := range snaps {
		for _, tag := range identityTags([]restic.Snapshot{sn}) {
			out[sn.Hostname+" "+tag]++
		}
	}
	return out
}

func TestTheOwnerKeepsASharedRepositoryWithRestic(t *testing.T) {
	f, sr, dest, mode := sharedRestic(t)
	ctx := context.Background()
	members := &fakeMembers{held: map[string][]string{cubeHost: {"fileset:docs"}}}

	if err := f.svc.keepSharedRepo(ctx, sr, members, time.Now(), true); err != nil {
		t.Fatal(err)
	}

	got := countByHost(t, f, dest, mode)
	want := map[string]int{
		towerHost + " container:plex": 1, towerHost + " fileset:docs": 1,
		cubeHost + " container:plex": 1, cubeHost + " fileset:docs": 3,
		strangerHost + " container:plex": 3, strangerHost + " fileset:docs": 3,
		restic.LegacyHost + " container:plex": 3, restic.LegacyHost + " fileset:docs": 3,
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%s: %d snapshots, want %d", k, got[k], n)
		}
	}
	u, err := f.st.SharedUpkeepOf(sr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Forget == nil || u.Forget.Forgotten != 6 || !slices.Equal(u.Forget.Unknown, []string{restic.LegacyHost, strangerHost}) {
		t.Fatalf("forget report %+v", u.Forget)
	}
	if u.Prune == nil || u.Prune.Error != "" || !u.Prune.Checked || u.Prune.CheckError != "" {
		t.Fatalf("prune report %+v", u.Prune)
	}
	if err := f.svc.engine.CheckData(ctx, dest, 100, mode); err != nil {
		t.Fatalf("check after the upkeep: %v", err)
	}

	if u.Space == nil || u.Space.Total <= 0 || len(u.Space.Hosts) != 2 {
		t.Fatalf("space %+v", u.Space)
	}
	var apart int64
	for _, h := range u.Space.Hosts {
		if h.Bytes <= 0 || h.Bytes > u.Space.Total {
			t.Fatalf("%s uses %d bytes of %d", h.Name, h.Bytes, u.Space.Total)
		}
		apart += h.Bytes
	}
	if apart <= u.Space.Total {
		t.Fatalf("the members apart use %d bytes, the repository holds %d; want more apart", apart, u.Space.Total)
	}
	if len(members.told) != 2 {
		t.Fatalf("told %v, want both members", members.told)
	}
}

func TestAMemberNeverForgetsOrPrunesASharedRepositoryWithRestic(t *testing.T) {
	f, sr, dest, _ := sharedRestic(t)
	ctx := context.Background()
	sr.Owner = "nas-id"
	if _, err := f.st.PutSharedRepo(sr); err != nil {
		t.Fatal(err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	member := f.svc.offsiteModeForTarget(settings, store.OffsiteTarget{Domain: "containers", Repo: sr.Repo, Enabled: true})
	every := member
	every.Sharing = restic.SharingOwner
	before := countByHost(t, f, dest, every)

	if err := f.svc.keepSharedRepo(ctx, sr, &fakeMembers{}, time.Now(), true); !errors.Is(err, errNotSharedOwner) {
		t.Fatalf("upkeep on a member: %v, want errNotSharedOwner", err)
	}
	if err := f.svc.startSharedUpkeep(sr.ID, &fakeMembers{}); !errors.Is(err, errNotSharedOwner) {
		t.Fatalf("upkeep started on a member: %v, want errNotSharedOwner", err)
	}
	snaps, err := f.svc.engine.SnapshotsOfEveryHost(ctx, dest, every)
	if err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"forget": f.svc.engine.Forget(ctx, dest, []string{snaps[0].ID}, true, member),
		"policy": f.svc.engine.ForgetPolicy(ctx, dest, restic.RetentionPolicy{KeepLast: 1}, member, []string{"container:plex"}, true),
		"prune":  f.svc.engine.Prune(ctx, dest, member),
	} {
		if !errors.Is(err, restic.ErrSharedMemberPrunes) {
			t.Errorf("%s on a member: %v, want ErrSharedMemberPrunes", name, err)
		}
	}
	if after := countByHost(t, f, dest, every); !mapsEqual(before, after) {
		t.Fatalf("the repository changed on a member: %v, was %v", after, before)
	}
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
