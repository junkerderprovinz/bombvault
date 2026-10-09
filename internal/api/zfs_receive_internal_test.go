package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

const receiveRoot = "tank/bombvault-replica"

// receiveRig is a source and a receiving instance in one group, each with a
// host of its own, and the receiving instance's router served where its
// answers say it is.
type receiveRig struct {
	src, dst         *instance
	srcPool, dstPool *fakePool
	srv              *httptest.Server
	itemID           string
	now              time.Time
}

func newReceiveRig(t *testing.T) *receiveRig {
	t.Helper()
	guids := &atomic.Uint64{}
	r := &receiveRig{
		src:     newInstance(t, "tower", strings.Repeat("a1", 32)),
		dst:     newInstance(t, "attic", strings.Repeat("b2", 32)),
		srcPool: newFakePool(guids, "cache", "cache/appdata", "cache/appdata/db"),
		dstPool: newFakePool(guids, "tank"),
		now:     time.Date(2026, 10, 9, 3, 0, 0, 0, time.UTC),
	}
	for _, side := range []struct {
		in   *instance
		pool *fakePool
	}{{r.src, r.srcPool}, {r.dst, r.dstPool}} {
		side.in.svc.SetZFSHost(zfs.NewSSHHost(side.pool))
		side.in.svc.SetHostSSH(side.pool)
	}
	switchReceiver(t, r.dst, true)
	r.srv = httptest.NewUnstartedServer(r.dst.router)
	r.srv.StartTLS()
	t.Cleanup(r.srv.Close)
	// The receiving instance hands out the pin of the key it serves.
	served := servedCertificate.Load()
	servedCertificate.Store(&r.srv.TLS.Certificates[0])
	t.Cleanup(func() { servedCertificate.Store(served) })
	r.dst.svc.cfg.HTTPOnly = false
	if err := r.dst.st.SetGroupDirectURL(r.srv.URL, true); err != nil {
		t.Fatal(err)
	}
	pairThroughRelay(t, r.src, r.dst)

	item, err := r.src.st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.src.st.SetZFSReplicaTarget(item.ID, store.ZFSReplicaTargetPeer, r.dst.id(t)); err != nil {
		t.Fatal(err)
	}
	r.itemID = item.ID
	return r
}

// switchReceiver turns the Receiver module of in on or off.
func switchReceiver(t *testing.T, in *instance, on bool) {
	t.Helper()
	if _, err := in.st.MutateSettings(func(s *store.Settings) error {
		s.ReceiverEnabled = on
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func (r *receiveRig) item(t *testing.T) store.ZFSDataset {
	t.Helper()
	d, err := r.src.st.GetZFSDataset(r.itemID)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// request is the receiving instance's only request, as its page lists it.
func (r *receiveRig) request(t *testing.T) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/zfs/receive/requests", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: r.dst.session}) //nolint:gosec // G124: a cookie on a test request, never set by a server
	r.dst.router.ServeHTTP(w, req)
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("requests = %s, want one", w.Body.String())
	}
	return list[0]
}

func (r *receiveRig) decide(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	id, _ := r.request(t)["id"].(string)
	_, out := r.dst.do(t, http.MethodPost, "/api/zfs/receive/requests/"+id, body)
	return out
}

// allowed asks once, lets a person allow the request with keep and returns
// the source's End for the slot.
func (r *receiveRig) allowed(t *testing.T, keep [5]int) zfsrepl.End {
	t.Helper()
	ctx := context.Background()
	if _, err := r.src.svc.zfsReplicaPeerEnd(ctx, r.item(t)); err == nil {
		t.Fatal("the source got an End before anybody allowed it")
	}
	out := r.decide(t, map[string]any{"decision": "allow", "pool": "tank", "root": receiveRoot,
		"keep": map[string]any{"preset": "own", "own": keep}})
	if out["ok"] != true || out["state"] != store.ZFSReceiveAllowed {
		t.Fatalf("allow = %v", out)
	}
	end, err := r.src.svc.zfsReplicaPeerEnd(ctx, r.item(t))
	if err != nil {
		t.Fatalf("zfsReplicaPeerEnd after the allow: %v", err)
	}
	return end
}

func (r *receiveRig) srcEnd() zfsrepl.End {
	return zfsrepl.NewSSHEnd(zfs.NewSSHHost(r.srcPool), r.srcPool)
}

// run replicates the item once through end and moves the clock on a day.
func (r *receiveRig) run(t *testing.T, end zfsrepl.End) zfsrepl.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := zfsrepl.Run(ctx, r.srcEnd(), end, zfsrepl.Entry{
		Root: "cache/appdata", TargetBase: "tower", Keep: store.RetentionKeep{KeepLast: 1},
		Now: func() time.Time { return r.now },
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, m := range res.Members {
		if m.Code != "" {
			t.Fatalf("%s: %s: %v", m.Dataset, m.Code, m.Err)
		}
	}
	r.now = r.now.Add(24 * time.Hour)
	return res
}

func TestASourceWaitsUntilTheReceivingInstanceAllows(t *testing.T) {
	r := newReceiveRig(t)
	_, err := r.src.svc.zfsReplicaPeerEnd(context.Background(), r.item(t))
	if zfsrepl.Code(err) != "peer-waiting" {
		t.Fatalf("an End before any answer = %v, want peer-waiting", err)
	}
	if got := r.src.svc.zfsReplicaPeerState(r.item(t)); got != store.ZFSReceiveAsked {
		t.Fatalf("peer state = %q, want asked", got)
	}

	req := r.request(t)
	if req["peer"] != r.src.id(t) || req["peerName"] != "tower" || req["sourceServer"] != "tower" ||
		req["item"] != "cache/appdata" || req["state"] != store.ZFSReceiveAsked {
		t.Fatalf("request = %v", req)
	}
	if members, _ := req["members"].([]any); len(members) != 2 || members[0] != "cache/appdata" || members[1] != "cache/appdata/db" {
		t.Fatalf("members = %v", req["members"])
	}

	for name, body := range map[string]map[string]any{
		"a pool the host lacks":  {"decision": "allow", "pool": "nope", "root": "nope/r"},
		"a root on another pool": {"decision": "allow", "pool": "tank", "root": "other/r"},
		"an unknown decision":    {"decision": "maybe"},
	} {
		if out := r.decide(t, body); out["ok"] != false {
			t.Errorf("%s was accepted: %v", name, out)
		}
	}
	r.allowed(t, [5]int{0, 7, 3, 0, 0})
	peer := r.item(t).Replica.Peer
	if peer.State != store.ZFSReceiveAllowed || peer.Base != receiveRoot+"/tower" || peer.URL != r.srv.URL || len(peer.TokenEnc) == 0 {
		t.Fatalf("the source stored %+v", peer)
	}
}

func TestAFullRunLandsUnderTheReceiversRootAndItPrunesItself(t *testing.T) {
	r := newReceiveRig(t)
	end := r.allowed(t, [5]int{1, 0, 0, 0, 0})
	first := r.run(t, end)
	second := r.run(t, end)

	for _, m := range []string{"cache/appdata", "cache/appdata/db"} {
		target := receiveRoot + "/tower/" + m
		if got := r.dstPool.snapNames(target); !slices.Equal(got, []string{second.Snapshot}) {
			t.Errorf("%s holds %q, want only %s after its own keep rule", target, got, second.Snapshot)
		}
		if d := r.dstPool.dataset(target); d.props["readonly"] != "on" {
			t.Errorf("%s landed with %v, want it read-only", target, d.props)
		}
	}
	for _, res := range []zfsrepl.Result{first, second} {
		for _, m := range res.Members {
			if len(m.Pruned) != 0 {
				t.Fatalf("the source pruned %q on the receiving side", m.Pruned)
			}
		}
	}
	for _, level := range []string{receiveRoot + "/tower", receiveRoot + "/tower/cache"} {
		if got := r.dstPool.dataset(level).props[zfs.SourceProperty]; got != r.src.id(t) {
			t.Errorf("%s carries source %q, want the sending instance", level, got)
		}
	}
	if _, owned := r.dstPool.dataset(receiveRoot).props[zfs.SourceProperty]; owned {
		t.Error("the root shared by every source was marked as one source's")
	}
	receives := r.dstPool.callsOf("receive")
	if last := receives[len(receives)-1]; !slices.Contains(last, "-F") {
		t.Errorf("the increment onto the newest snapshot was received with %q, want -F", last)
	}
	for _, argv := range receives {
		if !slices.Contains(argv, "reservation") || !slices.Contains(argv, "refreservation") {
			t.Errorf("%q takes the source's reservations into this pool", argv)
		}
	}
	var sent int64
	for _, m := range second.Members {
		sent += m.Bytes
	}
	slot := r.request(t)
	if sent == 0 || slot["lastReceived"] == "" || int64(slot["bytes"].(float64)) != sent {
		t.Errorf("the slot noted %v, want the %d bytes of every member of the last run", slot, sent)
	}
}

func TestBringBackStreamsAReplicaSnapshotHome(t *testing.T) {
	r := newReceiveRig(t)
	end := r.allowed(t, [5]int{0, 7, 3, 0, 0})
	res := r.run(t, end)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	got, err := zfsrepl.BringBack(ctx, end, r.srcEnd(), zfsrepl.Restore{
		Replica: "tower/cache/appdata", Snapshot: res.Snapshot, Dataset: "cache/appdata",
		Now: func() time.Time { return r.now },
	})
	if err != nil {
		t.Fatalf("BringBack: %v", err)
	}
	if !strings.HasPrefix(got.Root, "cache/appdata-bombvault-restore-") || len(got.Members) != 2 {
		t.Fatalf("brought back %+v, want the root and its child next to the item", got)
	}
	for _, dest := range []string{got.Root, got.Root + "/db"} {
		if !slices.Equal(r.srcPool.snapNames(dest), []string{res.Snapshot}) {
			t.Fatalf("%s holds %q, want %s", dest, r.srcPool.snapNames(dest), res.Snapshot)
		}
	}
	if _, err := zfsrepl.BringBack(ctx, end, r.srcEnd(), zfsrepl.Restore{
		Replica: "tower/cache/appdata", Snapshot: "bombvault-replica-20200101000000", Dataset: "cache/appdata",
		Now: func() time.Time { return r.now },
	}); zfsrepl.Code(err) != "not-found" {
		t.Fatalf("bringing back a snapshot the replica lacks = %v, want not-found", err)
	}
}

func TestRevokingASlotStopsTheSourceUntilItAsksAgain(t *testing.T) {
	r := newReceiveRig(t)
	end := r.allowed(t, [5]int{0, 7, 3, 0, 0})
	res := r.run(t, end)

	if out := r.decide(t, map[string]any{"decision": "revoke"}); out["ok"] != true || out["state"] != store.ZFSReceiveRevoked {
		t.Fatalf("revoke = %v", out)
	}
	cut, err := zfsrepl.Run(context.Background(), r.srcEnd(), end, zfsrepl.Entry{
		Root: "cache/appdata", TargetBase: "tower", Now: func() time.Time { return r.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range cut.Members {
		if m.Code != "peer-revoked" {
			t.Fatalf("%s after the revoke failed with %q, want peer-revoked", m.Dataset, m.Code)
		}
	}
	if got := r.item(t).Replica.Peer; got.State != store.ZFSReceiveRevoked || got.TokenEnc != nil {
		t.Fatalf("the source after a refused stream = %+v, want it to know the slot is revoked", got)
	}
	if got := r.dstPool.snapNames(receiveRoot + "/tower/cache/appdata"); !slices.Equal(got, []string{res.Snapshot}) {
		t.Fatalf("the revoke took the received data: %q", got)
	}

	if _, err := r.src.svc.zfsReplicaPeerEnd(context.Background(), r.item(t)); zfsrepl.Code(err) != "peer-revoked" {
		t.Fatalf("a run after the revoke = %v, want peer-revoked", err)
	}
	if r.request(t)["state"] != store.ZFSReceiveRevoked {
		t.Fatal("a run asking after the revoke reopened the request")
	}
	if err := r.src.svc.zfsReplicaPeerRequest(context.Background(), r.item(t)); err != nil {
		t.Fatal(err)
	}
	if got := r.request(t)["state"]; got != store.ZFSReceiveAsked {
		t.Fatalf("asking anew from the source left the request %v", got)
	}
}

func TestASourceNamesARefusalAndASilentReceiver(t *testing.T) {
	r := newReceiveRig(t)
	if _, err := r.src.svc.zfsReplicaPeerEnd(context.Background(), r.item(t)); zfsrepl.Code(err) != "peer-waiting" {
		t.Fatalf("an End before any answer = %v, want peer-waiting", err)
	}
	if out := r.decide(t, map[string]any{"decision": "refuse"}); out["ok"] != true {
		t.Fatalf("refuse = %v", out)
	}
	if _, err := r.src.svc.zfsReplicaPeerEnd(context.Background(), r.item(t)); zfsrepl.Code(err) != "peer-refused" {
		t.Fatalf("an End after the refusal = %v, want peer-refused", err)
	}

	r = newReceiveRig(t)
	end := r.allowed(t, [5]int{0, 7, 3, 0, 0})
	r.srv.Close()
	res, err := zfsrepl.Run(context.Background(), r.srcEnd(), end, zfsrepl.Entry{
		Root: "cache/appdata", TargetBase: "tower", Now: func() time.Time { return r.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res.Members {
		if m.Code != "peer-unreachable" {
			t.Fatalf("%s with the receiver gone failed with %q, want peer-unreachable", m.Dataset, m.Code)
		}
	}
}

func TestASourceSendsNothingToAReceiverOnPlainHTTP(t *testing.T) {
	r := newReceiveRig(t)
	r.allowed(t, [5]int{0, 7, 3, 0, 0})
	plain := "http://" + strings.TrimPrefix(r.srv.URL, "https://")
	if err := r.dst.st.SetGroupDirectURL(plain, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.src.svc.zfsReplicaPeerEnd(context.Background(), r.item(t)); zfsrepl.Code(err) != "peer-insecure" {
		t.Fatalf("an End for a receiver at %s = %v, want peer-insecure", plain, err)
	}
}

func TestAReceiverSwitchedOffTurnsRequestsAwayUntilItIsOn(t *testing.T) {
	r := newReceiveRig(t)
	switchReceiver(t, r.dst, false)
	if _, err := r.src.svc.zfsReplicaPeerEnd(context.Background(), r.item(t)); zfsrepl.Code(err) != "receive-off" {
		t.Fatalf("an End while receiving is off = %v, want receive-off", err)
	}
	if got := r.src.svc.zfsReplicaPeerState(r.item(t)); got != store.ZFSReceiveOff {
		t.Fatalf("peer state = %q, want off", got)
	}
	if slots, _ := r.dst.st.ListZFSReceiveSlots(); len(slots) != 0 {
		t.Fatalf("the switched off receiver kept %d requests", len(slots))
	}

	switchReceiver(t, r.dst, true)
	end := r.allowed(t, [5]int{0, 7, 3, 0, 0})
	r.run(t, end)

	switchReceiver(t, r.dst, false)
	cut, err := zfsrepl.Run(context.Background(), r.srcEnd(), end, zfsrepl.Entry{
		Root: "cache/appdata", TargetBase: "tower", Now: func() time.Time { return r.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range cut.Members {
		if m.Code != "receive-off" {
			t.Fatalf("%s into an allowed slot with receiving off failed with %q, want receive-off", m.Dataset, m.Code)
		}
	}
	switchReceiver(t, r.dst, true)
	end, err = r.src.svc.zfsReplicaPeerEnd(context.Background(), r.item(t))
	if err != nil {
		t.Fatalf("an End once receiving is on again: %v", err)
	}
	r.run(t, end)
}

func TestAFolderOfAnotherInstanceTakesNothing(t *testing.T) {
	r := newReceiveRig(t)
	end := r.allowed(t, [5]int{0, 7, 3, 0, 0})
	r.dstPool.ds[receiveRoot] = newFakeDataset("filesystem")
	stranger := newFakeDataset("filesystem")
	stranger.props[zfs.SourceProperty] = "0123456789abcdef"
	r.dstPool.ds[receiveRoot+"/tower"] = stranger

	res, err := zfsrepl.Run(context.Background(), r.srcEnd(), end, zfsrepl.Entry{
		Root: "cache/appdata", TargetBase: "tower", Now: func() time.Time { return r.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res.Members {
		if m.Code != "target-owned" {
			t.Errorf("%s: code %q, want target-owned", m.Dataset, m.Code)
		}
	}
	if r.dstPool.dataset(receiveRoot+"/tower/cache") != nil {
		t.Fatal("something landed in the other instance's folder")
	}
}

// askSlot records a request of instance peer, named tower, for dataset
// straight through the peer route and returns the slot's id.
func askSlot(t *testing.T, in *instance, peer, dataset string) string {
	t.Helper()
	item := strings.Repeat(peer[:1], 32)
	body, _ := json.Marshal(peerZFSReceiveRequest{
		InstanceID: peer, Name: "tower", Item: item, Dataset: dataset,
		Members: []string{dataset}, Keep: store.DefaultZFSReplicaKeep,
	})
	status, raw := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodPost, Path: "/api/group/peer/zfs-receive", Body: body})
	if status != http.StatusOK {
		t.Fatalf("ask = %d %s", status, raw)
	}
	slots, err := in.st.ListZFSReceiveSlots()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range slots {
		if s.PeerID == peer && s.ItemID == item {
			return s.ID
		}
	}
	t.Fatalf("no slot for %s: %s", peer, raw)
	return ""
}

func allowSlot(t *testing.T, in *instance, id string) map[string]any {
	t.Helper()
	_, out := in.do(t, http.MethodPost, "/api/zfs/receive/requests/"+id, map[string]any{
		"decision": "allow", "pool": "tank", "root": receiveRoot,
	})
	return out
}

func TestTwoSourcesOfOneNameCannotBothBeAllowedIntoOneFolder(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	pool := newFakePool(&atomic.Uint64{}, "tank")
	in.svc.SetZFSHost(zfs.NewSSHHost(pool))
	in.svc.SetHostSSH(pool)
	switchReceiver(t, in, true)

	first, second := askSlot(t, in, "0a0a0a0a", "cache/appdata"), askSlot(t, in, "0b0b0b0b", "cache/appdata")
	if out := allowSlot(t, in, first); out["ok"] != true {
		t.Fatalf("allowing the first = %v", out)
	}
	if out := allowSlot(t, in, second); out["ok"] != false || out["code"] != "target-owned" {
		t.Fatalf("allowing a second source into %s/tower = %v, want target-owned", receiveRoot, out)
	}

	third := askSlot(t, in, "0c0c0c0c", "cache/system")
	pool.ds[receiveRoot] = newFakeDataset("filesystem")
	pool.ds[receiveRoot+"/tower"] = newFakeDataset("filesystem")
	pool.ds[receiveRoot+"/tower"].props[zfs.SourceProperty] = "0d0d0d0d"
	if out := allowSlot(t, in, third); out["ok"] != false || out["code"] != "target-owned" {
		t.Fatalf("allowing a source into a folder another instance marked = %v, want target-owned", out)
	}
}

func TestASlotReachesNothingInAFolderOfAnotherInstance(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	pool := newFakePool(&atomic.Uint64{}, "tank")
	in.svc.SetZFSHost(zfs.NewSSHHost(pool))
	in.svc.SetHostSSH(pool)
	switchReceiver(t, in, true)
	id := askSlot(t, in, "0b0b0b0b", "cache/appdata")
	if out := allowSlot(t, in, id); out["ok"] != true {
		t.Fatalf("allow = %v", out)
	}
	slot, _, _ := in.st.GetZFSReceiveSlot(id)
	token, err := secret.Decrypt(in.appKey, slot.TokenEnc)
	if err != nil {
		t.Fatal(err)
	}

	// Another instance called tower streamed into the folder after the allow.
	for _, ds := range []string{receiveRoot, receiveRoot + "/tower", receiveRoot + "/tower/cache", receiveRoot + "/tower/cache/appdata"} {
		pool.ds[ds] = newFakeDataset("filesystem")
		if ds != receiveRoot {
			pool.ds[ds].props[zfs.SourceProperty] = "0a0a0a0a"
		}
	}
	target := receiveRoot + "/tower/cache/appdata"
	const snap = "bombvault-replica-20261009030000"
	pool.ds[target].snaps = []fakeSnap{{snap, 11, 1}}

	member := "/api/zfs/receive/" + id + "/members/cache%2Fappdata"
	increment := append(streamRecord("cache/appdata@bombvault-replica-20261010030000", 12, 11, false), fakeTrailer...)
	for _, call := range []struct {
		method, path string
		body         []byte
	}{
		{http.MethodGet, "/api/zfs/receive/" + id + "/points", nil},
		{http.MethodGet, member + "/send?snapshot=" + snap, nil},
		{http.MethodPost, member + "/abort", nil},
		{http.MethodPut, member, increment},
	} {
		req := httptest.NewRequest(call.method, call.path, bytes.NewReader(call.body))
		req.Header.Set("Authorization", "Bearer "+string(token))
		w := httptest.NewRecorder()
		in.router.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), `"code":"target-owned"`) {
			t.Errorf("%s %s = %d %.200s, want target-owned", call.method, call.path, w.Code, w.Body)
		}
	}
	if got := pool.snapNames(target); !slices.Equal(got, []string{snap}) {
		t.Fatalf("the other instance's replica holds %q, want only %s", got, snap)
	}
}

// receiveSlots asks for two items of a source straight through the peer
// route and allows both, returning their ids and tokens.
func receiveSlots(t *testing.T, in *instance, pool *fakePool) (ids, tokens []string) {
	t.Helper()
	in.svc.SetZFSHost(zfs.NewSSHHost(pool))
	in.svc.SetHostSSH(pool)
	switchReceiver(t, in, true)
	for i, ds := range []string{"cache/appdata", "cache/system"} {
		body, _ := json.Marshal(peerZFSReceiveRequest{
			InstanceID: "0a0a0a0a", Name: "tower", Item: strings.Repeat(string(rune('a'+i)), 32), Dataset: ds,
			Members: []string{ds}, Keep: store.DefaultZFSReplicaKeep,
		})
		status, raw := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodPost, Path: "/api/group/peer/zfs-receive", Body: body})
		var ans peerZFSReceiveAnswer
		if status != http.StatusOK || json.Unmarshal(raw, &ans) != nil || ans.State != store.ZFSReceiveAsked {
			t.Fatalf("ask %s = %d %s", ds, status, raw)
		}
	}
	slots, err := in.st.ListZFSReceiveSlots()
	if err != nil || len(slots) != 2 {
		t.Fatalf("slots = %+v, %v", slots, err)
	}
	slices.SortFunc(slots, func(a, b store.ZFSReceiveSlot) int { return strings.Compare(a.Dataset, b.Dataset) })
	for _, s := range slots {
		if code, out := in.do(t, http.MethodPost, "/api/zfs/receive/requests/"+s.ID, map[string]any{
			"decision": "allow", "pool": "tank", "root": receiveRoot,
		}); code != http.StatusOK || out["ok"] != true {
			t.Fatalf("allow: %d %v", code, out)
		}
		got, _, _ := in.st.GetZFSReceiveSlot(s.ID)
		token, err := secret.Decrypt(in.appKey, got.TokenEnc)
		if err != nil {
			t.Fatal(err)
		}
		ids, tokens = append(ids, s.ID), append(tokens, string(token))
	}
	return ids, tokens
}

func slotCall(in *instance, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader("not a stream"))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	in.router.ServeHTTP(w, req)
	return w
}

func TestASlotAnswersOnlyItsOwnTokenAndMembers(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	ids, tokens := receiveSlots(t, in, newFakePool(&atomic.Uint64{}, "tank"))
	points := "/api/zfs/receive/" + ids[0] + "/points"

	if w := slotCall(in, http.MethodGet, points, tokens[0]); w.Code != http.StatusOK {
		t.Fatalf("the slot's own token = %d %s", w.Code, w.Body)
	}
	for name, token := range map[string]string{"no token": "", "a wrong token": "x" + tokens[0], "the other slot's token": tokens[1]} {
		if w := slotCall(in, http.MethodGet, points, token); w.Code != http.StatusUnauthorized {
			t.Errorf("%s = %d %s, want 401", name, w.Code, w.Body)
		}
	}
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, points + "?member=cache/system"},
		{http.MethodPut, "/api/zfs/receive/" + ids[0] + "/members/cache%2Fsystem"},
		{http.MethodPost, "/api/zfs/receive/" + ids[0] + "/members/cache%2Fappdata%2F..%2Fsystem/abort"},
		{http.MethodGet, "/api/zfs/receive/" + ids[0] + "/members/cache%2Fsystem/send?snapshot=bombvault-replica-20261009030000"},
	} {
		if w := slotCall(in, call.method, call.path, tokens[0]); w.Code != http.StatusForbidden {
			t.Errorf("%s %s outside the slot = %d %s, want 403", call.method, call.path, w.Code, w.Body)
		}
	}
	if w := slotCall(in, http.MethodPut, "/api/zfs/receive/"+ids[0]+"/members/cache%2Fappdata", tokens[0]); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), `"ok":false`) {
		t.Errorf("a body that is no send stream = %d %s, want a refusal", w.Code, w.Body)
	}
	if w := slotCall(in, http.MethodGet, "/api/zfs/receive/requests", tokens[0]); w.Code != http.StatusUnauthorized {
		t.Errorf("the request list with a slot token = %d, want the session gate", w.Code)
	}

	if code, out := in.do(t, http.MethodPost, "/api/zfs/receive/requests/"+ids[0], map[string]any{"decision": "revoke"}); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("revoke: %d %v", code, out)
	}
	w := slotCall(in, http.MethodGet, points, tokens[0])
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"state":"revoked"`) {
		t.Fatalf("a revoked slot = %d %s, want 403 saying revoked", w.Code, w.Body)
	}
}

func TestTheReceiveRouteRefusesARequestItCannotHonour(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	switchReceiver(t, in, true)
	good := peerZFSReceiveRequest{InstanceID: "0a0a0a0a", Name: "tower", Item: strings.Repeat("a", 32),
		Dataset: "cache/appdata", Members: []string{"cache/appdata"}, Keep: store.DefaultZFSReplicaKeep}
	for name, spoil := range map[string]func(*peerZFSReceiveRequest){
		"no instance id":          func(r *peerZFSReceiveRequest) { r.InstanceID = "" },
		"an id that is an option": func(r *peerZFSReceiveRequest) { r.InstanceID = "-oops" },
		"an item that is no id":   func(r *peerZFSReceiveRequest) { r.Item = "../x" },
		"a member outside":        func(r *peerZFSReceiveRequest) { r.Members = append(r.Members, "cache/system") },
		"no root among members":   func(r *peerZFSReceiveRequest) { r.Members = []string{"cache/appdata/db"} },
		"an unknown keep":         func(r *peerZFSReceiveRequest) { r.Keep.Preset = "forever" },
	} {
		req := good
		req.Members = slices.Clone(good.Members)
		spoil(&req)
		body, _ := json.Marshal(req)
		_, raw := in.svc.servePeer(context.Background(), relay.ProxyCall{Method: http.MethodPost, Path: "/api/group/peer/zfs-receive", Body: body})
		if strings.Contains(string(raw), `"ok":true`) {
			t.Errorf("%s was recorded: %s", name, raw)
		}
	}
	if slots, _ := in.st.ListZFSReceiveSlots(); len(slots) != 0 {
		t.Fatalf("refused requests left %d slots", len(slots))
	}
}

func TestARefusedRequestLeavesTheList(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	ids, _ := receiveSlots(t, in, newFakePool(&atomic.Uint64{}, "tank"))
	slot, err := in.st.AskZFSReceive(store.ZFSReceiveSlot{PeerID: "0b0b0b0b", ItemID: strings.Repeat("c", 32),
		Dataset: "tank/x", SourceServer: "barn", Members: []string{"tank/x"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, out := in.do(t, http.MethodPost, "/api/zfs/receive/requests/"+slot.ID, map[string]any{"decision": "refuse"}); out["ok"] != true {
		t.Fatalf("refuse = %v", out)
	}
	if _, out := in.do(t, http.MethodPatch, "/api/zfs/receive/requests/"+ids[0], map[string]any{
		"keep": map[string]any{"preset": "long", "own": []int{0, 0, 0, 0, 0}},
	}); out["ok"] != true || out["keep"].(map[string]any)["preset"] != "long" {
		t.Fatalf("patch keep = %v", out)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/zfs/receive/requests", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: in.session}) //nolint:gosec // G124: a cookie on a test request, never set by a server
	in.router.ServeHTTP(w, req)
	var list []zfsReceiveView
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatalf("requests = %s, want the two allowed ones", w.Body)
	}
	for _, v := range list {
		if v.State != store.ZFSReceiveAllowed {
			t.Fatalf("listed %+v", v)
		}
	}
}

func TestPinnedTLSTakesTheReceiversKeyAndNothingElse(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	t.Cleanup(srv.Close)
	pin := spkiPin(srv.Certificate())
	resp, err := peerEndClient("127.0.0.1", pin).Get(srv.URL)
	if err != nil {
		t.Fatalf("the pinned key was refused: %v", err)
	}
	_ = resp.Body.Close()
	for name, pin := range map[string]string{"another pin": strings.Repeat("0", 64), "no pin": ""} {
		if resp, err := peerEndClient("127.0.0.1", pin).Get(srv.URL); err == nil {
			_ = resp.Body.Close()
			t.Errorf("%s: a self-signed certificate was taken", name)
		}
	}
}

// chainCert issues a certificate for names under a fresh CA and returns it
// with a pool that trusts the CA.
func chainCert(t *testing.T, names ...string) (*x509.Certificate, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test root"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), NotBefore: caTmpl.NotBefore, NotAfter: caTmpl.NotAfter,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return leaf, roots
}

func TestPinnedTLSChecksAChainAgainstTheHostItDialled(t *testing.T) {
	leaf, roots := chainCert(t, "proxy.example", "192.168.1.10")
	other := strings.Repeat("0", 64)
	for _, c := range []struct {
		host, pin string
		ok        bool
	}{
		{"proxy.example", "", true},
		{"proxy.example", other, true},
		{"elsewhere.example", "", false},
		{"elsewhere.example", other, false},
		{"192.168.1.10", "", true},
		{"192.168.1.11", "", false},
		{"192.168.1.10", other, false},
	} {
		// crypto/tls leaves ServerName empty for an IP address, so the check
		// must not lean on it.
		err := pinnedTLS(c.host, c.pin, roots).VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}})
		if (err == nil) != c.ok {
			t.Errorf("a chain for proxy.example and 192.168.1.10 dialled as %s with pin %q: %v, want accepted %v", c.host, c.pin, err, c.ok)
		}
	}
}

func TestAnIncrementFromAnOlderBaseIsNeverRolledBackOnto(t *testing.T) {
	pool := newFakePool(&atomic.Uint64{}, "tank", receiveRoot, receiveRoot+"/tower", receiveRoot+"/tower/cache")
	target := receiveRoot + "/tower/cache/appdata"
	pool.ds[target] = newFakeDataset("filesystem")
	pool.ds[target].snaps = []fakeSnap{{"bombvault-replica-20261008030000", 11, 1}, {"bombvault-replica-20261009030000", 12, 2}}
	host := zfsrepl.NewSSHEnd(zfs.NewSSHHost(pool), pool)
	slot := store.ZFSReceiveSlot{PeerID: "0a0a0a0a", Root: receiveRoot, SourceServer: "tower"}

	for _, c := range []struct {
		from  uint64
		force bool
	}{{12, true}, {11, false}} {
		args, err := zfsSlotReceiveArgs(context.Background(), host, slot, target, zfs.StreamBegin{FromGUID: c.from, ToGUID: 13})
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(args, "-F") != c.force {
			t.Errorf("an increment from guid %d got %q, want -F %v", c.from, args, c.force)
		}
	}
	if _, err := zfsSlotReceiveArgs(context.Background(), host, slot, target, zfs.StreamBegin{ToGUID: 13}); zfsrepl.Code(err) != "dataset-exists" {
		t.Fatalf("a full stream onto a replica with snapshots = %v, want dataset-exists", err)
	}
}
