package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// keyring is one repository's keys, shared by the engines of every instance
// in a test, however each of them names the repository.
type keyring struct {
	mu   sync.Mutex
	next int
	keys []keyringKey
}

type keyringKey struct {
	restic.Key
	password string
}

func (k *keyring) opens(password string) bool {
	return slices.ContainsFunc(k.keys, func(e keyringKey) bool { return e.password == password })
}

func (k *keyring) add(password, host, user string) {
	k.next++
	k.keys = append(k.keys, keyringKey{Key: restic.Key{ID: fmt.Sprintf("%08x", k.next), HostName: host, UserName: user}, password: password})
}

func (k *keyring) hosts() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for _, e := range k.keys {
		out = append(out, e.HostName+"/"+e.UserName)
	}
	return out
}

// keyringEngine is an instance's engine with the keys of the one shared
// repository in a test.
type keyringEngine struct {
	ResticEngine
	ring  *keyring
	inits []string
}

func (e *keyringEngine) Init(_ context.Context, repo string, m restic.Mode) error {
	e.ring.mu.Lock()
	defer e.ring.mu.Unlock()
	e.inits = append(e.inits, repo)
	e.ring.add(m.Password, "bombvault", "root")
	return nil
}

func (e *keyringEngine) RepoOpensErr(_ context.Context, _ string, m restic.Mode) error {
	e.ring.mu.Lock()
	defer e.ring.mu.Unlock()
	if m.Encrypted && e.ring.opens(m.Password) {
		return nil
	}
	return errors.New("Fatal: wrong password or no key found")
}

func (e *keyringEngine) AddKey(_ context.Context, _ string, m restic.Mode, password, host, user string) error {
	e.ring.mu.Lock()
	defer e.ring.mu.Unlock()
	if !e.ring.opens(m.Password) {
		return errors.New("Fatal: wrong password or no key found")
	}
	e.ring.add(password, host, user)
	return nil
}

func (e *keyringEngine) Keys(_ context.Context, _ string, m restic.Mode) ([]restic.Key, error) {
	e.ring.mu.Lock()
	defer e.ring.mu.Unlock()
	if !e.ring.opens(m.Password) {
		return nil, errors.New("Fatal: wrong password or no key found")
	}
	var out []restic.Key
	for _, k := range e.ring.keys {
		key := k.Key
		key.Current = k.password == m.Password
		out = append(out, key)
	}
	return out, nil
}

func (e *keyringEngine) RemoveKey(_ context.Context, _ string, m restic.Mode, id string) error {
	e.ring.mu.Lock()
	defer e.ring.mu.Unlock()
	if !e.ring.opens(m.Password) {
		return errors.New("Fatal: wrong password or no key found")
	}
	i := slices.IndexFunc(e.ring.keys, func(k keyringKey) bool { return k.ID == id })
	if i < 0 {
		return errors.New("key not found")
	}
	if e.ring.keys[i].password == m.Password {
		return errors.New("refusing to remove key currently used to access repository")
	}
	e.ring.keys = slices.Delete(e.ring.keys, i, i+1)
	return nil
}

func (e *keyringEngine) SnapshotsOfEveryHost(context.Context, string, restic.Mode) ([]restic.Snapshot, error) {
	return nil, nil
}

// sharedPair is a group of two instances: nas, which runs a receiver, and
// tower, which wants to copy into nas's shared repository. Both encrypt.
func sharedPair(t *testing.T) (nas, tower *instance, ring *keyring) {
	t.Helper()
	nas = newInstance(t, "nas", strings.Repeat("a1", 32))
	tower = newInstance(t, "tower", strings.Repeat("b2", 32))
	pairThroughRelay(t, nas, tower)
	runReceiver(t, nas)
	// Nothing answers there, so the append-only check gives up at once.
	if err := nas.st.SetReceiverServerHost("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	restore := receiverStartWait
	receiverStartWait = 0
	t.Cleanup(func() { receiverStartWait = restore })
	ring = &keyring{}
	for _, in := range []*instance{nas, tower} {
		if _, err := in.st.MutateSettings(func(s *store.Settings) error {
			s.EncryptionEnabled = true
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		in.svc.engine = &keyringEngine{ResticEngine: in.engine, ring: ring}
	}
	return nas, tower, ring
}

func setUpShared(t *testing.T, nas *instance) sharedServerView {
	t.Helper()
	v, err := nas.svc.SetUpSharedRepo(context.Background(), receiverServerInput{Folder: "shared"})
	if err != nil {
		t.Fatalf("set up the shared repository: %v", err)
	}
	return v
}

func joinOf(t *testing.T, in *instance, direction string, other *instance) store.SharedJoin {
	t.Helper()
	j, ok, err := in.st.GetSharedJoin(direction, other.id(t))
	if err != nil || !ok {
		t.Fatalf("no %s join with %s: %v", direction, other.id(t), err)
	}
	return j
}

// waitForJoin waits until in holds its join with other in state.
func waitForJoin(t *testing.T, in *instance, direction string, other *instance, state string) store.SharedJoin {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, ok, err := in.st.GetSharedJoin(direction, other.id(t))
		if err == nil && ok && j.State == state {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("the %s join is %+v (%v), want %s", direction, j, ok, state)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func joined(t *testing.T, nas, tower *instance) {
	t.Helper()
	if _, err := tower.svc.JoinSharedRepo(context.Background(), nas.id(t)); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := nas.svc.AllowSharedJoin(context.Background(), tower.id(t)); err != nil {
		t.Fatalf("allow: %v", err)
	}
	waitForJoin(t, tower, store.SharedJoinOut, nas, store.SharedJoinJoined)
	waitForJoin(t, nas, store.SharedJoinIn, tower, store.SharedJoinJoined)
}

func htpasswdUsers(t *testing.T, nas *instance) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(nas.svc.cfg.HostMountRoot, "shared", htpasswdName)) //nolint:gosec // a test's own folder
	if err != nil {
		t.Fatal(err)
	}
	var users []string
	for l := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		if user, _, ok := strings.Cut(l, ":"); ok {
			users = append(users, user)
		}
	}
	return users
}

func TestASharedRepositoryRunsOnASecondAppendOnlyServerEveryLoginReaches(t *testing.T) {
	nas, _, ring := sharedPair(t)
	v := setUpShared(t, nas)

	dock := nas.svc.docker.(*receiverDocker)
	if len(dock.created) != 1 {
		t.Fatalf("created %d containers, want 1", len(dock.created))
	}
	c := dock.created[0]
	opts := strings.Join(c.Config.Env, " ")
	if c.Name != sharedServerName || !strings.Contains(opts, "--append-only") || strings.Contains(opts, "--private-repos") {
		t.Fatalf("container %s with %s, want %s append-only without private repositories", c.Name, opts, sharedServerName)
	}
	if _, ok := c.HostConfig.PortBindings["8000/tcp"]; !ok || c.HostConfig.PortBindings["8000/tcp"][0].HostPort != "8001" {
		t.Fatalf("ports %v, want 8001 published", c.HostConfig.PortBindings)
	}
	if v.Port != 8001 || v.Repo != "shared/shared" {
		t.Fatalf("view %+v", v)
	}
	sr, ok, err := nas.st.SharedRepoAt("shared/shared")
	if err != nil || !ok || sr.Owner != nas.id(t) || len(sr.Members) != 0 {
		t.Fatalf("shared repository %+v, %v, %v; want it owned by nas with nobody in it yet", sr, ok, err)
	}
	if got := ring.hosts(); len(got) != 1 {
		t.Fatalf("keys %v, want the owner's alone", got)
	}
	if got := htpasswdUsers(t, nas); len(got) != 1 || got[0] != v.User {
		t.Fatalf("logins %v, want only the owner's %s", got, v.User)
	}
}

func TestASharedRepositoryNeedsTheReceiverAndEncryption(t *testing.T) {
	t.Run("receiver", func(t *testing.T) {
		nas, _, _ := sharedPair(t)
		if err := nas.st.DeleteReceiverServer(); err != nil {
			t.Fatal(err)
		}
		if _, err := nas.svc.SetUpSharedRepo(context.Background(), receiverServerInput{Folder: "shared"}); !errors.Is(err, errSharedNeedsReceiver) {
			t.Fatalf("err = %v, want %v", err, errSharedNeedsReceiver)
		}
	})
	t.Run("encryption", func(t *testing.T) {
		nas, _, _ := sharedPair(t)
		if _, err := nas.st.MutateSettings(func(s *store.Settings) error {
			s.EncryptionEnabled = false
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := nas.svc.SetUpSharedRepo(context.Background(), receiverServerInput{Folder: "shared"}); !errors.Is(err, errSharedNeedsEncryption) {
			t.Fatalf("err = %v, want %v", err, errSharedNeedsEncryption)
		}
	})
}

func TestAJoinWaitsForAPersonOnTheOwner(t *testing.T) {
	nas, tower, ring := sharedPair(t)
	setUpShared(t, nas)

	state, err := tower.svc.JoinSharedRepo(context.Background(), nas.id(t))
	if err != nil || state != store.SharedJoinAsked {
		t.Fatalf("join: %q, %v", state, err)
	}
	in := joinOf(t, nas, store.SharedJoinIn, tower)
	if in.State != store.SharedJoinAsked || in.MemberName != "tower" || in.Host != tower.svc.resticHost() {
		t.Fatalf("the owner holds %+v", in)
	}
	if joinOf(t, tower, store.SharedJoinOut, nas).State != store.SharedJoinAsked {
		t.Fatal("the member does not hold its request")
	}
	if got := ring.hosts(); len(got) != 1 {
		t.Fatalf("keys %v before anyone allowed the join", got)
	}
}

func TestAnAllowedMemberWritesWithItsOwnKeyAndTheOneTimeKeyGoes(t *testing.T) {
	nas, tower, ring := sharedPair(t)
	setUpShared(t, nas)
	joined(t, nas, tower)

	host := tower.svc.resticHost()
	keys := ring.hosts()
	if len(keys) != 2 || keys[1] != host+"/tower" {
		t.Fatalf("keys %v, want the owner's and %s/tower, without the one-time key", keys, host)
	}
	if !ring.opens(tower.svc.backupPassword()) {
		t.Fatal("the member's own password does not open the repository")
	}
	in := joinOf(t, nas, store.SharedJoinIn, tower)
	if len(in.OnceEnc) != 0 || in.OnceKey != "" {
		t.Fatalf("the owner still keeps the one-time key: %+v", in)
	}
	sr, _, err := nas.st.SharedRepoAt("shared/shared")
	if err != nil || len(sr.Members) != 1 || sr.Members[0].Host != host || sr.Members[0].Instance != tower.id(t) {
		t.Fatalf("the owner's members %+v, %v", sr.Members, err)
	}
	if got := htpasswdUsers(t, nas); !slices.Contains(got, in.User) {
		t.Fatalf("logins %v, want the member's %s", got, in.User)
	}

	out := joinOf(t, tower, store.SharedJoinOut, nas)
	if len(out.OnceEnc) != 0 || !strings.HasPrefix(out.Repo, "rest:http://127.0.0.1:8001/") {
		t.Fatalf("the member holds %+v", out)
	}
	mine, ok, err := tower.st.SharedRepoAt(out.Repo)
	if err != nil || !ok || mine.Owner != nas.id(t) {
		t.Fatalf("the member's shared repository %+v, %v, %v", mine, ok, err)
	}
	targets, err := tower.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	var at []store.OffsiteTarget
	for _, tg := range targets {
		if tg.Repo == out.Repo {
			at = append(at, tg)
		}
	}
	if len(at) != len(offsiteConfigDomains) {
		t.Fatalf("%d targets at the shared repository, want one per section", len(at))
	}
	for _, tg := range at {
		if tg.Enabled || !tg.Immutable || targetAgingPolicy(tg).Any() || tg.CredsRef == "" {
			t.Fatalf("target %+v, want it off, append-only, without a keep policy and with its own login", tg)
		}
	}
}

func TestAHostAnotherMemberWritesUnderIsRefused(t *testing.T) {
	nas, tower, _ := sharedPair(t)
	setUpShared(t, nas)
	sr, _, err := nas.st.SharedRepoAt("shared/shared")
	if err != nil {
		t.Fatal(err)
	}
	sr.Members = []store.SharedMember{{Host: tower.svc.resticHost(), Instance: "cube-id", Name: "cube"}}
	if _, err := nas.st.PutSharedRepo(sr); err != nil {
		t.Fatal(err)
	}
	if _, err := tower.svc.JoinSharedRepo(context.Background(), nas.id(t)); err == nil {
		t.Fatal("a join under a host another member writes under went through")
	}
	if _, ok, _ := nas.st.GetSharedJoin(store.SharedJoinIn, tower.id(t)); ok {
		t.Fatal("the owner keeps the refused join")
	}
}

func TestARefusedJoinGetsNoKey(t *testing.T) {
	nas, tower, ring := sharedPair(t)
	setUpShared(t, nas)
	if _, err := tower.svc.JoinSharedRepo(context.Background(), nas.id(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.svc.RefuseSharedJoin(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	waitForJoin(t, tower, store.SharedJoinOut, nas, store.SharedJoinRefused)
	if got := ring.hosts(); len(got) != 1 {
		t.Fatalf("keys %v after a refusal", got)
	}
}

func TestAMemberTakenOutLosesItsKeyAndLoginButKeepsItsPart(t *testing.T) {
	nas, tower, ring := sharedPair(t)
	v := setUpShared(t, nas)
	joined(t, nas, tower)
	out := joinOf(t, tower, store.SharedJoinOut, nas)

	if _, err := nas.svc.RemoveSharedMember(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	if got := ring.hosts(); len(got) != 1 {
		t.Fatalf("keys %v, want the owner's alone", got)
	}
	if got := htpasswdUsers(t, nas); !slices.Equal(got, []string{v.User}) {
		t.Fatalf("logins %v, want only the owner's", got)
	}
	sr, _, err := nas.st.SharedRepoAt("shared/shared")
	if err != nil || len(sr.Members) != 0 {
		t.Fatalf("members %+v, %v", sr.Members, err)
	}

	waitForJoin(t, tower, store.SharedJoinOut, nas, store.SharedJoinRemoved)
	if got := tower.svc.sharingAt(out.Repo); got != restic.SharingMember {
		t.Fatalf("the former member's part is %v, want it to stay a member's", got)
	}
	targets, err := tower.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range targets {
		if tg.Repo == out.Repo && tg.Enabled {
			t.Fatalf("target %s still copies to the repository it was taken out of", tg.Domain)
		}
	}
}

func TestAMemberThatLeavesTakesItsKeyAlong(t *testing.T) {
	nas, tower, ring := sharedPair(t)
	setUpShared(t, nas)
	joined(t, nas, tower)

	if err := tower.svc.LeaveSharedRepo(context.Background(), nas.id(t)); err != nil {
		t.Fatal(err)
	}
	if got := ring.hosts(); len(got) != 1 {
		t.Fatalf("keys %v, want the owner's alone", got)
	}
	if _, ok, _ := nas.st.GetSharedJoin(store.SharedJoinIn, tower.id(t)); ok {
		t.Fatal("the owner keeps the member that left")
	}
	if joinOf(t, tower, store.SharedJoinOut, nas).State != store.SharedJoinRemoved {
		t.Fatal("the member does not know it left")
	}
}

func TestOnlyTheServerAJoinIsForCollectsItsKey(t *testing.T) {
	nas, tower, _ := sharedPair(t)
	setUpShared(t, nas)
	if _, err := tower.svc.JoinSharedRepo(context.Background(), nas.id(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.svc.AllowSharedJoin(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	ans := peerCallAs(t, nas, tower.id(t), "/api/group/peer/shared-repo/join", map[string]any{"name": "tower", "host": tower.svc.resticHost()})
	if ans["ok"] == true || ans["once"] != nil {
		t.Fatalf("an unsigned call in the member's name got %v", ans)
	}
}

func TestAMemberTakesOnlyAPlainFolderAsTheSharedRepository(t *testing.T) {
	at := groupReceiverView{URL: "http://192.168.1.20:8001"}
	if got, err := sharedRepoLocation(at, "shared"); err != nil || got != "rest:http://192.168.1.20:8001/shared" {
		t.Fatalf("shared: %q, %v", got, err)
	}
	for _, path := range []string{"", "../repo", "a/b", "shared?x=1", "user@evil", "config"} {
		if got, err := sharedRepoLocation(at, path); err == nil {
			t.Errorf("%q gave %q, want a refusal", path, got)
		}
	}
	if got, err := sharedRepoLocation(groupReceiverView{URL: "http://evil.example/x:8001"}, "shared"); err == nil {
		t.Errorf("an address with a path gave %q, want a refusal", got)
	}
}

func TestAMemberTakenOutCanJoinAgainWithANewKey(t *testing.T) {
	nas, tower, ring := sharedPair(t)
	setUpShared(t, nas)
	joined(t, nas, tower)
	if _, err := nas.svc.RemoveSharedMember(context.Background(), tower.id(t)); err != nil {
		t.Fatal(err)
	}
	waitForJoin(t, tower, store.SharedJoinOut, nas, store.SharedJoinRemoved)

	joined(t, nas, tower)
	if !ring.opens(tower.svc.backupPassword()) {
		t.Fatal("the member's own password does not open the repository again")
	}
}
