package api

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// agingRepos is storage that holds the snapshots of a node and of another
// server, and records how home ages and prunes it.
type agingRepos struct {
	ResticEngine
	mu      sync.Mutex
	forgets []string
	prunes  []string
}

func (a *agingRepos) RepoOpensErr(context.Context, string, restic.Mode) error { return nil }

func (a *agingRepos) Snapshots(_ context.Context, _ string, m restic.Mode) ([]restic.Snapshot, error) {
	all := []restic.Snapshot{
		{ID: "1", Hostname: m.Host, Tags: []string{"container:web"}},
		{ID: "2", Hostname: m.Host, Tags: []string{"container:db"}},
		{ID: "3", Hostname: "bv-another", Tags: []string{"container:mail"}},
	}
	if m.Sharing == restic.SharingNone {
		return all, nil
	}
	return slices.DeleteFunc(all, func(sn restic.Snapshot) bool { return sn.Hostname != m.Host }), nil
}

func (a *agingRepos) ForgetPolicy(_ context.Context, repo string, p restic.RetentionPolicy, m restic.Mode, tags []string, prune bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if m.NoLock || prune || !p.Direct {
		a.forgets = append(a.forgets, "a forget without its lock, with a prune or sparing direct snapshots")
	}
	a.forgets = append(a.forgets, fmt.Sprintf("%s %s %s %d", repo, m.Host, strings.Join(tags, ","), p.KeepLast))
	return nil
}

func (a *agingRepos) Prune(_ context.Context, repo string, m restic.Mode) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prunes = append(a.prunes, repo+" "+m.Host)
	return nil
}

func TestHomeAgesANodesRepositoriesByItsRulesUnderTheNodesHostAlone(t *testing.T) {
	home := newInstance(t, "home", strings.Repeat("a1", 32))
	repos := &agingRepos{}
	home.svc.engine = repos
	if _, err := home.st.MutateSettings(func(s *store.Settings) error { s.RetentionKeepLast = 3; return nil }); err != nil {
		t.Fatal(err)
	}
	nodeKey := strings.Repeat("b2", 32)
	keyEnc, err := secret.Encrypt(home.svc.cfg.AppKey, []byte(nodeKey))
	if err != nil {
		t.Fatal(err)
	}
	passwordEnc, err := secret.Encrypt(home.svc.cfg.AppKey, []byte(restickey.Derive(nodeKey)))
	if err != nil {
		t.Fatal(err)
	}
	n := store.Node{ID: "aabbccddeeff00", Name: "vps", AppKeyEnc: keyEnc, RelayMode: "project", CreatedAt: 1, AdoptedAt: 1}
	if err := home.st.AddNode(n); err != nil {
		t.Fatal(err)
	}
	for _, d := range nodeDomains {
		rr := store.ReceivedRepo{ID: nodeRepoID(n.ID, d), NodeID: n.ID, Name: "vps " + d, Repo: "rest:https://backup.example.org/vps/" + d,
			ResticPasswordEnc: passwordEnc, Enabled: d == "containers"}
		if _, err := home.st.CreateReceivedRepo(rr); err != nil {
			t.Fatal(err)
		}
	}

	for _, d := range nodeDomains {
		if err := home.svc.pruneNodeSection(context.Background(), n, d); err != nil {
			t.Fatal(err)
		}
	}
	host := restickey.Host(nodeKey)
	repo := "rest:https://backup.example.org/vps/containers"
	want := []string{repo + " " + host + " container:db 3", repo + " " + host + " container:web 3"}
	slices.Sort(repos.forgets)
	if !slices.Equal(repos.forgets, want) {
		t.Errorf("forgets = %q, want %q", repos.forgets, want)
	}
	if !slices.Equal(repos.prunes, []string{repo + " " + host}) {
		t.Errorf("prunes = %q, want the repository a watch is on, once", repos.prunes)
	}
}

func TestANodeAtAStorageLocationAgesByThatLocationsKeepRule(t *testing.T) {
	home := newInstance(t, "home", strings.Repeat("a1", 32))
	repos := &agingRepos{}
	home.svc.engine = repos
	if _, err := home.st.MutateSettings(func(s *store.Settings) error { s.RetentionKeepLast = 3; return nil }); err != nil {
		t.Fatal(err)
	}
	loc, err := home.st.SaveDestination(store.OffsiteTarget{Name: "Wasabi", Repo: "s3:https://s3.example.org/bucket", RetentionKeepLast: 7, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	nodeKey := strings.Repeat("b2", 32)
	keyEnc, err := secret.Encrypt(home.svc.cfg.AppKey, []byte(nodeKey))
	if err != nil {
		t.Fatal(err)
	}
	passwordEnc, err := secret.Encrypt(home.svc.cfg.AppKey, []byte(restickey.Derive(nodeKey)))
	if err != nil {
		t.Fatal(err)
	}
	n := store.Node{ID: "aabbccddeeff00", Name: "vps", AppKeyEnc: keyEnc, RelayMode: "project", CreatedAt: 1, AdoptedAt: 1, Placement: nodePlaceLocation, PlacementTarget: loc.ID}
	if err := home.st.AddNode(n); err != nil {
		t.Fatal(err)
	}
	repo := "s3:https://s3.example.org/bucket/bombvault-node-aabbccddeeff-files"
	if _, err := home.st.CreateReceivedRepo(store.ReceivedRepo{ID: nodeRepoID(n.ID, "files"), NodeID: n.ID, Repo: repo, ResticPasswordEnc: passwordEnc, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := home.svc.pruneNodeSection(context.Background(), n, "files"); err != nil {
		t.Fatal(err)
	}
	if len(repos.forgets) == 0 || !strings.HasSuffix(repos.forgets[0], " 7") {
		t.Errorf("forgets = %q, want the location's seven", repos.forgets)
	}
}

// lockedRepos is a repository whose first prune meets a lock, and which
// records how it was unlocked.
type lockedRepos struct {
	agingRepos
	locked  bool
	unlocks []bool
}

func (l *lockedRepos) Prune(ctx context.Context, repo string, m restic.Mode) error {
	l.mu.Lock()
	first := !l.locked
	l.locked = true
	l.mu.Unlock()
	if first {
		return errors.New("Fatal: unable to create lock in backend: repository is already locked")
	}
	return l.agingRepos.Prune(ctx, repo, m)
}

func (l *lockedRepos) Unlock(_ context.Context, _ string, removeAll bool, _ restic.Mode) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.unlocks = append(l.unlocks, removeAll)
	return nil
}

func TestAPruneThatMeetsALockClearsOnlyStaleLocks(t *testing.T) {
	node := newInstance(t, "vps", strings.Repeat("b2", 32))
	repos := &lockedRepos{}
	node.svc.engine = repos
	if err := node.svc.ageAndPrune(context.Background(), "/srv/repo", restic.Mode{Encrypted: true, Password: "pw"}, "bv-node", restic.RetentionPolicy{KeepLast: 3, Direct: true}); err != nil {
		t.Fatal(err)
	}
	if len(repos.unlocks) != 1 || repos.unlocks[0] {
		t.Errorf("unlocks = %v, want one that leaves the locks of live runs alone", repos.unlocks)
	}
	if len(repos.prunes) != 1 {
		t.Errorf("prunes = %v, want the prune made once the lock was cleared", repos.prunes)
	}
}
