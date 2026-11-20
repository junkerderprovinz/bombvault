package api

import (
	"context"
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

	if err := home.svc.pruneNodeRepos(context.Background(), n); err != nil {
		t.Fatal(err)
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
