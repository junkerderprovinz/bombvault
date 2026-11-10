package api

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	sharedOldAt = "rest:http://127.0.0.1:8001/shared"
	sharedNewAt = "rest:http://192.168.1.20:8001/shared"
)

// movedToCube is heldByCube with cube set up on another machine: its
// rest-server for the repository answers at an address of its own, and tower
// reads towerReads at each location it reaches.
func movedToCube(t *testing.T, towerReads map[string]string) (nas, tower, cube *instance) {
	t.Helper()
	nas, tower, cube, sr, _ := heldByCube(t, "5f3c0a1e", true)
	runReceiver(t, cube)
	if err := cube.st.SaveSharedRepoServer(store.SharedRepoServer{ContainerName: sharedServerName, Folder: "shared", Port: sharedServerPort, User: "cube", Repo: sr.Repo}); err != nil {
		t.Fatal(err)
	}
	tower.svc.engine.(*identifiedEngine).at = towerReads
	return nas, tower, cube
}

func targetsAt(t *testing.T, in *instance, repo string) []store.OffsiteTarget {
	t.Helper()
	all, err := in.st.ListOffsiteTargets()
	if err != nil {
		t.Fatal(err)
	}
	return slices.DeleteFunc(all, func(tg store.OffsiteTarget) bool { return tg.Repo != repo })
}

func membershipWith(t *testing.T, in, owner *instance) sharedMembershipView {
	t.Helper()
	all, err := in.svc.sharedMemberships()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(all, func(m sharedMembershipView) bool { return m.OwnerID == owner.id(t) })
	if i < 0 {
		t.Fatalf("no membership with %s in %+v", owner.id(t), all)
	}
	return all[i]
}

// stayedWithNas fails unless tower still copies to nas's old address and
// names nas as the holder there.
func stayedWithNas(t *testing.T, nas, tower *instance) {
	t.Helper()
	if got := ownerAt(t, tower, sharedOldAt).Owner; got != nas.id(t) {
		t.Errorf("tower names %s as the holder, want nas", got)
	}
	if moved := targetsAt(t, tower, sharedNewAt); len(moved) != 0 {
		t.Errorf("%d of tower's targets point at the new address", len(moved))
	}
	if kept := targetsAt(t, tower, sharedOldAt); len(kept) != len(offsiteConfigDomains) {
		t.Errorf("%d of tower's targets point at the old address, want %d", len(kept), len(offsiteConfigDomains))
	}
}

func TestAMemberFollowsTheNewHolderToItsNewAddress(t *testing.T) {
	_, tower, cube := movedToCube(t, map[string]string{sharedNewAt: "5f3c0a1e"})
	before := targetsAt(t, tower, sharedOldAt)
	if len(before) == 0 {
		t.Fatal("tower has no targets at the shared repository")
	}
	switched := before[0]
	switched.Enabled, switched.Compression = true, "max"
	if _, err := tower.st.UpsertOffsiteTarget(switched); err != nil {
		t.Fatal(err)
	}

	cube.svc.tellSharedHolders(context.Background())

	sr := ownerAt(t, tower, sharedNewAt)
	if sr.Owner != cube.id(t) {
		t.Errorf("tower names %s as the holder, want cube", sr.Owner)
	}
	if _, ok, _ := tower.st.SharedRepoAt(sharedOldAt); ok {
		t.Error("tower still knows the repository at the old address")
	}
	after := targetsAt(t, tower, sharedNewAt)
	if len(after) != len(before) {
		t.Fatalf("%d targets at the new address, want %d", len(after), len(before))
	}
	for _, b := range before {
		i := slices.IndexFunc(after, func(a store.OffsiteTarget) bool { return a.ID == b.ID })
		if i < 0 {
			t.Fatalf("target %s did not move", b.ID)
		}
		a := after[i]
		want := b
		if b.ID == switched.ID {
			want = switched
		}
		if a.CredsRef != want.CredsRef || a.Immutable != want.Immutable || a.Enabled != want.Enabled || a.Compression != want.Compression || a.Name != want.Name {
			t.Errorf("target %s is %+v after the move, want the settings of %+v", a.ID, a, want)
		}
	}
	if got := joinOf(t, tower, store.SharedJoinOut, cube).Repo; got != sharedNewAt {
		t.Errorf("tower's join is at %s, want the new address", got)
	}
	// The next copy goes where the target points, and so does every read.
	if repo, _, err := tower.svc.sharedReach(sr); err != nil || repo != sharedNewAt {
		t.Errorf("tower reaches the repository at %s (%v), want the new address", repo, err)
	}
	if m := membershipWith(t, tower, cube); m.Repo != sharedNewAt || m.Move != nil {
		t.Errorf("tower's card shows %s with %+v, want the new address and nothing held back", m.Repo, m.Move)
	}
	if left, err := cube.st.SharedHolderNotices(); err != nil || len(left) != 0 {
		t.Errorf("still to tell: %+v, %v", left, err)
	}
}

func TestAMemberStaysWhenTheNewAddressHoldsAnotherRepository(t *testing.T) {
	nas, tower, cube := movedToCube(t, map[string]string{sharedOldAt: "5f3c0a1e", sharedNewAt: "9d8c7b6a"})

	cube.svc.tellSharedHolders(context.Background())

	stayedWithNas(t, nas, tower)
	if got := joinOf(t, tower, store.SharedJoinOut, nas).Repo; got != sharedOldAt {
		t.Errorf("tower's join is at %s, want the old address", got)
	}
	m := membershipWith(t, tower, nas)
	if m.Move == nil || m.Move.Repo != sharedNewAt || m.Move.Reason == "" {
		t.Errorf("tower's card says %+v, want the new address and why it was not followed", m.Move)
	}
}

func TestAnUnreachableNewAddressIsTriedAgainLater(t *testing.T) {
	nas, tower, cube := movedToCube(t, map[string]string{})
	ctx := context.Background()

	cube.svc.tellSharedHolders(ctx)

	stayedWithNas(t, nas, tower)
	left, err := cube.st.SharedHolderNotices()
	if err != nil || !slices.ContainsFunc(left, func(n store.SharedHolderNotice) bool { return n.Instance == tower.id(t) }) {
		t.Fatalf("cube forgot that tower has still to hear of it: %+v, %v", left, err)
	}
	if m := membershipWith(t, tower, nas); m.Move == nil || m.Move.Reason == "" {
		t.Errorf("tower's card says %+v, want why it still copies to the old address", m.Move)
	}

	tower.svc.engine.(*identifiedEngine).at[sharedNewAt] = "5f3c0a1e"
	cube.svc.tellSharedHolders(ctx)

	if got := ownerAt(t, tower, sharedNewAt).Owner; got != cube.id(t) {
		t.Errorf("tower names %s as the holder once the address answers, want cube", got)
	}
	if m := membershipWith(t, tower, cube); m.Move != nil {
		t.Errorf("tower's card still says %+v after it followed", m.Move)
	}
}

func TestAMemberAtTheAddressTheHolderNamesChangesOnlyTheHolder(t *testing.T) {
	_, tower, cube := movedToCube(t, map[string]string{sharedOldAt: "5f3c0a1e"})
	if err := cube.st.SetReceiverServerHost("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	before := targetsAt(t, tower, sharedOldAt)

	cube.svc.tellSharedHolders(context.Background())

	if got := ownerAt(t, tower, sharedOldAt).Owner; got != cube.id(t) {
		t.Errorf("tower names %s as the holder, want cube", got)
	}
	if after := targetsAt(t, tower, sharedOldAt); len(after) != len(before) {
		t.Errorf("%d targets at the address, want %d", len(after), len(before))
	}
	if got := joinOf(t, tower, store.SharedJoinOut, cube).Repo; got != sharedOldAt {
		t.Errorf("tower's join is at %s, want the same address", got)
	}
}

func TestAMemberChecksTheNewAddressByTheRulesOfAJoin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		server peerReceiver
		path   string
	}{
		{"a host with a path", peerReceiver{Host: "192.168.1.20/other", Port: 8001}, sharedRepoFolder},
		{"a host with a login", peerReceiver{Host: "tower:pw@192.168.1.20", Port: 8001}, sharedRepoFolder},
		{"no port", peerReceiver{Host: "192.168.1.20"}, sharedRepoFolder},
		{"a port beyond the range", peerReceiver{Host: "192.168.1.20", Port: 70000}, sharedRepoFolder},
		{"no host it can hand out", peerReceiver{Port: 8001, NeedsHost: true}, sharedRepoFolder},
		{"a host only its own network reaches", peerReceiver{Host: "192.168.1.20", HostFromGroup: true, Port: 8001}, sharedRepoFolder},
		{"a folder of rest-server's own", peerReceiver{Host: "192.168.1.20", Port: 8001}, "keys"},
		{"a folder above", peerReceiver{Host: "192.168.1.20", Port: 8001}, "../shared"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nas, tower, cube := movedToCube(t, map[string]string{sharedOldAt: "5f3c0a1e", sharedNewAt: "5f3c0a1e"})
			in := peerSharedHolder{RepoID: "5f3c0a1e", Server: &tc.server, Path: tc.path}

			ans, err := tower.svc.takeSharedHolder(context.Background(), cube.id(t), in)

			if err != nil || ans.Taken || ans.Challenge != "" || !ans.Later {
				t.Errorf("answer %+v, %v; want to be told again later, without a check", ans, err)
			}
			stayedWithNas(t, nas, tower)
			if m := membershipWith(t, tower, nas); m.Move == nil || m.Move.Reason == "" {
				t.Errorf("tower's card says %+v, want why the address was not followed", m.Move)
			}
		})
	}
}

func TestAnAddressTheHolderDidNotSignIsRefused(t *testing.T) {
	_, tower, cube := movedToCube(t, map[string]string{sharedOldAt: "5f3c0a1e", sharedNewAt: "5f3c0a1e"})
	in := peerSharedHolder{RepoID: "5f3c0a1e"}
	var err error
	if in.peerProof, err = cube.svc.signCall(keydeposit.ProofSharedHolder, tower.id(t), in.signed()); err != nil {
		t.Fatal(err)
	}
	in.Server, in.Path = &peerReceiver{Host: "192.168.1.20", Port: 8001}, sharedRepoFolder

	err = cube.svc.callMember(context.Background(), tower.id(t), "POST", "/api/group/peer/shared-holder", in, &peerSharedHolderAnswer{})

	if !errors.Is(err, errMemberRefused) {
		t.Errorf("err = %v, want the call refused", err)
	}
}
