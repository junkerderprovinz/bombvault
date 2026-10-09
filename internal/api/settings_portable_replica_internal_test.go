package api

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// seedReplica gives an instance a ZFS server with its own key directory, an
// item replicating there, an allowed receive slot for another instance and a
// second item with the answer of its peer target.
func seedReplica(t *testing.T, st *store.Repo) (store.ZFSReplicaServer, store.ZFSDataset) {
	t.Helper()
	server, err := st.CreateZFSReplicaServer(store.ZFSReplicaServer{
		Name: "attic", Host: "10.0.0.5", User: "replica", Port: 2222, Pool: "tank", Root: "tank/bombvault-replica",
		Enabled: true, KeyDir: "/config/ssh-replica/hidden-key-dir",
	})
	if err != nil {
		t.Fatal(err)
	}
	item, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetZFSReplicaTarget(item.ID, store.ZFSReplicaTargetServer, server.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.SetZFSReplicaAfterBackup(item.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := st.SetZFSReplicaCadence(item.ID, "daily 05:00"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetZFSReplicaKeep(item.ID, store.ZFSReplicaKeep{Preset: "short"}); err != nil {
		t.Fatal(err)
	}
	slot, err := st.AskZFSReceive(store.ZFSReceiveSlot{
		PeerID: "peer-b", PeerName: "barn", ItemID: "their-item", Dataset: "tank/photos", SourceServer: "barn",
		Members: []string{"tank/photos"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DecideZFSReceive(slot.ID, store.ZFSReceiveDecision{
		State: store.ZFSReceiveAllowed, Pool: "tank", Root: "tank/slotroot", TokenEnc: []byte("slottoken"),
	}); err != nil {
		t.Fatal(err)
	}
	peered, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/system", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetZFSReplicaTarget(peered.ID, store.ZFSReplicaTargetPeer, "peer-b"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetZFSReplicaPeer(peered.ID, store.ZFSReplicaPeer{
		State: store.ZFSReceiveAllowed, Slot: "peerslot", TokenEnc: []byte("peertoken"), Base: "tank/peerbase", URL: "https://10.0.0.9:3443",
	}); err != nil {
		t.Fatal(err)
	}
	return server, item
}

func TestExportImportCarriesZFSServersAndReplicaSettingsButNoKeysOrSlots(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	server, _ := seedReplica(t, srcStore)

	body, exp := doExport(t, src, "?includeCredentials=true")
	for _, secret := range []string{"hidden-key-dir", "slotroot", "slottoken", "peerslot", "peertoken", "peerbase", "10.0.0.9"} {
		if bytes.Contains(body, []byte(secret)) {
			t.Fatalf("the export carries %q", secret)
		}
	}
	if exp.ZFSReplica == nil || len(exp.ZFSReplica.Servers) != 1 || len(exp.ZFSReplica.Items) != 2 {
		t.Fatalf("export replica block = %+v", exp.ZFSReplica)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	item, err := dstStore.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	preview := doImport(t, dst, body, "")
	groups, _ := preview["summary"].(map[string]any)["settingsGroups"].([]any)
	if !slices.Contains(groups, any("zfsReplica")) {
		t.Fatalf("the preview names %v, want zfsReplica among them", groups)
	}
	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}

	got, ok, err := dstStore.GetZFSReplicaServer(server.ID)
	if err != nil || !ok {
		t.Fatalf("the server did not arrive: %v", err)
	}
	if got.Name != "attic" || got.Host != "10.0.0.5" || got.User != "replica" || got.Port != 2222 ||
		got.Pool != "tank" || got.Root != "tank/bombvault-replica" || !got.Enabled || got.KeyDir != "" {
		t.Fatalf("imported server = %+v, want the source's fields and no key directory", got)
	}
	d, err := dstStore.GetZFSDataset(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := store.ZFSReplica{TargetKind: store.ZFSReplicaTargetServer, TargetID: server.ID, Cadence: "daily 05:00",
		Keep: store.ZFSReplicaKeep{Preset: "short"}}
	if !reflect.DeepEqual(d.Replica, want) {
		t.Fatalf("imported replica = %+v, want %+v", d.Replica, want)
	}
	if slots, err := dstStore.ListZFSReceiveSlots(); err != nil || len(slots) != 0 {
		t.Fatalf("receive slots after the import = %+v, %v; want none", slots, err)
	}
}

func TestAFileWithoutTheReplicaLeavesTheServersAlone(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "zfsReplica")
	older, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	dst, dstStore := newPortableHandler(t, appKeyB)
	server, item := seedReplica(t, dstStore)
	if env := doImport(t, dst, older, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	if _, ok, _ := dstStore.GetZFSReplicaServer(server.ID); !ok {
		t.Fatal("a file from before the replica deleted this instance's server")
	}
	if d, _ := dstStore.GetZFSDataset(item.ID); d.Replica.TargetID != server.ID {
		t.Fatalf("a file from before the replica detached the item: %+v", d.Replica)
	}
}

func TestImportRefusesAReplicaBlockAFormWouldRefuse(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	seedReplica(t, srcStore)
	body, base := doExport(t, src, "")
	if ctl, _ := newPortableHandler(t, appKeyB); doImport(t, ctl, body, "")["ok"] != true {
		t.Fatal("the unspoiled file is refused, so the cases below prove nothing")
	}

	for name, spoil := range map[string]func(*zfsReplicaExport){
		"host that is an ssh option": func(r *zfsReplicaExport) { r.Servers[0].Host = "-oProxyCommand=touch /tmp/x" },
		"user with a space":          func(r *zfsReplicaExport) { r.Servers[0].User = "root x" },
		"root on another pool":       func(r *zfsReplicaExport) { r.Servers[0].Root = "other/replica" },
		"port out of range":          func(r *zfsReplicaExport) { r.Servers[0].Port = 70000 },
		"two servers with one id":    func(r *zfsReplicaExport) { r.Servers = append(r.Servers, r.Servers[0]) },
		"target on a missing server": func(r *zfsReplicaExport) { r.Items[0].Target.ID = "missing" },
		"unknown target kind":        func(r *zfsReplicaExport) { r.Items[0].Target.Kind = "cloud" },
		"unknown keep preset":        func(r *zfsReplicaExport) { r.Items[0].Keep.Preset = "forever" },
		"negative keep count":        func(r *zfsReplicaExport) { r.Items[0].Keep = store.ZFSReplicaKeep{Preset: "own", Own: [5]int{0, -1}} },
		"cadence that does not parse": func(r *zfsReplicaExport) {
			r.Items[0].Cadence = "sometimes"
		},
		"cadence every few days": func(r *zfsReplicaExport) {
			r.Items[0].Cadence = "everyN 3 03:00"
		},
	} {
		t.Run(name, func(t *testing.T) {
			exp := base
			rep := *base.ZFSReplica
			rep.Servers = slices.Clone(rep.Servers)
			rep.Items = slices.Clone(rep.Items)
			spoil(&rep)
			exp.ZFSReplica = &rep
			body, err := json.Marshal(exp)
			if err != nil {
				t.Fatal(err)
			}

			dst, dstStore := newPortableHandler(t, appKeyB)
			if env := doImport(t, dst, body, "?apply=true"); env["ok"] != false {
				t.Fatalf("the import accepted it: %v", env)
			}
			if servers, _ := dstStore.ListZFSReplicaServers(); len(servers) != 0 {
				t.Fatalf("a refused import wrote %d servers", len(servers))
			}
		})
	}
}

func TestAnImportThatDropsAServerCleansItsItemsAndWaitsForTheirRuns(t *testing.T) {
	src, srcStore := newPortableHandler(t, appKeyA)
	seedSource(t, src, srcStore)
	body, _ := doExport(t, src, "")

	dst, dstStore := newPortableHandler(t, appKeyB)
	server, item := seedReplica(t, dstStore)
	host := newReplicaHost()
	host.tree = []string{"cache/appdata"}
	host.points["cache/appdata"] = "cache/appdata@" + replicaSnap + "\t7\t10\n"
	dst.svc.replica.hostEnd = func() (zfsrepl.End, error) { return host, nil }

	unlock, _ := dst.svc.lockZFSReplica(item.ID)
	env := doImport(t, dst, body, "?apply=true")
	unlock()
	if env["ok"] != false {
		t.Fatalf("an import while the item replicates = %v, want it refused", env)
	}
	if _, ok, _ := dstStore.GetZFSReplicaServer(server.ID); !ok {
		t.Fatal("a refused import deleted the server")
	}

	if env := doImport(t, dst, body, "?apply=true"); env["ok"] != true {
		t.Fatalf("apply envelope wrong: %v", env)
	}
	if d, _ := dstStore.GetZFSDataset(item.ID); d.Replica.TargetKind != store.ZFSReplicaTargetNone {
		t.Fatalf("the item still replicates to %+v", d.Replica)
	}
	if calls := host.did("destroy"); len(calls) != 1 {
		t.Errorf("source cleanup = %q, want the item's replica snapshot removed", calls)
	}
	if dst.svc.zfsReplicaRunning(item.ID) {
		t.Error("the import kept the item's lock")
	}
}
