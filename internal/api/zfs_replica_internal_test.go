package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

const replicaSnap = "bombvault-replica-20261009030000"

// replicaHost answers the short zfs commands of a replica from what a test
// put in it: datasets with the owner they were marked with, their snapshot
// and bookmark listings, and the pools.
type replicaHost struct {
	mu     sync.Mutex
	calls  [][]string
	owners map[string]string
	points map[string]string
	tree   []string
	pools  string
	fail   error
	// locked are the encrypted datasets.
	locked map[string]bool
}

func newReplicaHost() *replicaHost {
	return &replicaHost{owners: map[string]string{}, points: map[string]string{}}
}

func (h *replicaHost) Run(_ context.Context, args []string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, args)
	if h.fail != nil {
		return "", h.fail
	}
	last := args[len(args)-1]
	switch {
	case slices.Contains(args, zfs.SourceProperty):
		owner, ok := h.owners[last]
		if !ok {
			return "", &zfs.CmdError{Args: args, Code: "not-found"}
		}
		return owner + "\n", nil
	case args[1] == "get":
		enc := "off"
		if h.locked[last] {
			enc = "aes-256-gcm"
		}
		return "type\tfilesystem\nencryption\t" + enc + "\nreceive_resume_token\t-\n", nil
	case args[1] == "list" && slices.Contains(args, "0"):
		return h.pools, nil
	case args[1] == "list" && slices.Contains(args, "snapshot,bookmark"):
		return h.points[last], nil
	case args[1] == "list":
		var b strings.Builder
		for _, name := range h.tree {
			enc := "off"
			if h.locked[name] {
				enc = "aes-256-gcm"
			}
			fmt.Fprintf(&b, "%s\tfilesystem\t/mnt/%s\tyes\ton\t%s\t-\thidden\t1024\t1024\n", name, name, enc)
		}
		return b.String(), nil
	}
	return "", nil
}

func (h *replicaHost) Send(context.Context, []string) (io.ReadCloser, func() error, error) {
	return nil, nil, errors.New("replicaHost streams nothing")
}

func (h *replicaHost) Receive(context.Context, []string, io.Reader) error {
	return errors.New("replicaHost streams nothing")
}

// did returns the recorded argv whose subcommand is sub.
func (h *replicaHost) did(sub string) [][]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out [][]string
	for _, c := range h.calls {
		if len(c) > 1 && c[1] == sub {
			out = append(out, c)
		}
	}
	return out
}

type replicaRig struct {
	t      *testing.T
	s      *Service
	st     *store.Repo
	host   *replicaHost
	server *replicaHost
	srv    store.ZFSReplicaServer
	item   store.ZFSDataset
	h      http.Handler
	// entries are what the engine was asked to run.
	entries []zfsrepl.Entry
	result  func(e zfsrepl.Entry) (zfsrepl.Result, error)
}

// newReplicaRig is an instance named bottich with one ZFS item replicating to
// one ZFS server. The engine is faked: it reports every member current and
// hands each one to Finished the way the real one does.
func newReplicaRig(t *testing.T) *replicaRig {
	t.Helper()
	st := newTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ZFSEnabled, settings.ZFSSchedule, settings.InstanceName = true, "daily 03:00", "bottich"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DataDir: t.TempDir(), AppKey: strings.Repeat("a", 64)}
	r := &replicaRig{t: t, st: st, host: newReplicaHost(), server: newReplicaHost()}
	r.s = &Service{store: st, cfg: cfg, repoMu: map[string]*sync.Mutex{zfsDomain: {}}}
	r.s.replica.hostEnd = func() (zfsrepl.End, error) { return r.host, nil }
	r.s.replica.serverEnd = func(store.ZFSReplicaServer, string) (zfsrepl.End, error) { return r.server, nil }
	r.s.replica.run = func(_ context.Context, _, _ zfsrepl.End, e zfsrepl.Entry) (zfsrepl.Result, error) {
		r.entries = append(r.entries, e)
		if r.result != nil {
			return r.result(e)
		}
		res := zfsrepl.Result{Snapshot: replicaSnap}
		for _, ds := range []string{e.Root, e.Root + "/plex"} {
			m := zfsrepl.MemberResult{Dataset: ds, Target: e.TargetBase + "/" + ds, Snapshot: replicaSnap, GUID: 1 << 63, Bytes: 4096}
			e.Finished(m)
			res.Members = append(res.Members, m)
		}
		return res, nil
	}

	r.srv, err = st.CreateZFSReplicaServer(store.ZFSReplicaServer{
		Name: "backup", Host: "backup.lan", User: "root", Port: 22, Pool: "tank", Root: "tank/bombvault-replica", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	r.item = zfsSeedItem(t, st, "cache/appdata")
	if err := st.SetZFSReplicaTarget(r.item.ID, store.ZFSReplicaTargetServer, r.srv.ID); err != nil {
		t.Fatal(err)
	}
	r.item, _ = st.GetZFSDataset(r.item.ID)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	sched.SetZFSJob(func(string) error { return nil }, st.ListZFSDatasets)
	r.h = NewHandler(cfg, st, nil, r.s, sched, nil).Router()
	return r
}

func (r *replicaRig) call(method, path, body string) (int, map[string]any, []any) {
	r.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.h.ServeHTTP(w, req)
	var obj map[string]any
	var list []any
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			r.t.Fatalf("%s %s answered %q", method, path, w.Body.String())
		}
	}
	return w.Code, obj, list
}

func (r *replicaRig) lastRun() store.ZFSReplicaRun {
	r.t.Helper()
	run, ok, err := r.st.LatestZFSReplicaRun(r.item.ID)
	if err != nil || !ok {
		r.t.Fatalf("no replica run (err %v)", err)
	}
	return run
}

func TestServerRoutesAnswerBareReadsAndEnvelopedChanges(t *testing.T) {
	r := newReplicaRig(t)
	r.server.pools = "tank\t1000\t3000\n"

	code, _, list := r.call(http.MethodGet, "/api/zfs/replica/servers", "")
	if code != http.StatusOK || len(list) != 1 {
		t.Fatalf("list = %d %v, want the one server bare", code, list)
	}
	srv, _ := list[0].(map[string]any)
	if srv["freeBytes"] != 3000.0 || srv["sizeBytes"] != 4000.0 {
		t.Errorf("server sizes = %v, want what its pool reported", srv)
	}
	if used, _ := srv["usedBy"].([]any); len(used) != 1 || used[0] != r.item.ID {
		t.Errorf("usedBy = %v, want the item", srv["usedBy"])
	}

	_, m, _ := r.call(http.MethodPost, "/api/zfs/replica/servers",
		`{"name":"second","host":"two.lan","user":"root","port":22,"pool":"pool","root":"pool/bombvault-replica"}`)
	if m["ok"] != true || m["id"] == "" || m["name"] != "second" || m["enabled"] != true {
		t.Fatalf("create = %v, want ok with the server's fields", m)
	}
	_, m, _ = r.call(http.MethodPost, "/api/zfs/replica/servers",
		`{"name":"bad","host":"-oProxyCommand=x","user":"root","port":22,"pool":"pool","root":"pool/r"}`)
	if m["ok"] != false {
		t.Errorf("a host starting with a dash was stored: %v", m)
	}
	_, m, _ = r.call(http.MethodPatch, "/api/zfs/replica/servers/0123456789abcdef0123456789abcdef", `{"enabled":false}`)
	if m["ok"] != false {
		t.Errorf("patching a server that does not exist = %v", m)
	}
	id := r.srv.ID
	_, m, _ = r.call(http.MethodPatch, "/api/zfs/replica/servers/"+id, `{"enabled":false}`)
	if m["ok"] != true || m["enabled"] != false {
		t.Errorf("patch = %v", m)
	}

	_, m, _ = r.call(http.MethodPost, "/api/zfs/replica/servers/"+id+"/test", "")
	pools, _ := m["pools"].([]any)
	if m["ok"] != true || m["code"] != "ok" || len(pools) != 1 {
		t.Errorf("test = %v, want ok with the pools", m)
	}
	r.server.fail = &zfs.CmdError{Code: "ssh-auth", Stderr: "Permission denied (publickey)."}
	_, m, _ = r.call(http.MethodPost, "/api/zfs/replica/servers/test", `{"host":"three.lan","user":"root","port":22}`)
	if m["ok"] != false || m["code"] != "ssh-auth" {
		t.Errorf("a refused test = %v, want its code", m)
	}
}

func TestDeletingAServerInUseNeedsDetachAndCleansTheItems(t *testing.T) {
	r := newReplicaRig(t)
	r.host.tree = []string{"cache/appdata"}
	r.host.points["cache/appdata"] = "cache/appdata@" + replicaSnap + "\t7\t10\ncache/appdata#" + replicaSnap + "\t7\t10\n"

	_, m, _ := r.call(http.MethodDelete, "/api/zfs/replica/servers/"+r.srv.ID, "")
	if m["ok"] != false || m["code"] != "in-use" {
		t.Fatalf("delete of a used server = %v, want in-use", m)
	}
	_, m, _ = r.call(http.MethodDelete, "/api/zfs/replica/servers/"+r.srv.ID+"?detach=1", "")
	if m["ok"] != true {
		t.Fatalf("delete with detach = %v", m)
	}
	if d, _ := r.st.GetZFSDataset(r.item.ID); d.Replica.TargetKind != store.ZFSReplicaTargetNone {
		t.Errorf("the item still replicates to %+v", d.Replica)
	}
	if len(r.host.did("release")) != 1 || len(r.host.did("destroy")) != 1 {
		t.Errorf("source cleanup = release %q, destroy %q", r.host.did("release"), r.host.did("destroy"))
	}
}

func TestTheReplicaKeyIsMadeOnceAndServedBare(t *testing.T) {
	r := newReplicaRig(t)
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}
	_, first, _ := r.call(http.MethodGet, "/api/zfs/replica/key", "")
	_, second, _ := r.call(http.MethodGet, "/api/zfs/replica/key", "")
	key, _ := first["publicKey"].(string)
	if !strings.HasPrefix(key, "ssh-ed25519 ") || second["publicKey"] != key {
		t.Errorf("keys = %v and %v, want one ed25519 key", first, second)
	}
	if _, ok := first["ok"]; ok {
		t.Errorf("the key answered in an envelope: %v", first)
	}
}

func TestARunRecordsItsMembersAndWhereEachStands(t *testing.T) {
	r := newReplicaRig(t)
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/run", "")
	if m["ok"] != true || m["runId"] == "" {
		t.Fatalf("run = %v, want ok with a run id", m)
	}
	r.s.replica.work.Wait()

	run := r.lastRun()
	if run.ID != m["runId"] || run.Status != "success" || run.Kind != store.ZFSReplicaRunKind || run.Bytes != 8192 {
		t.Errorf("run = %+v", run.Run)
	}
	if len(run.Members) != 2 || run.Members[0].Snapshot != replicaSnap {
		t.Errorf("members = %+v", run.Members)
	}
	st, ok, _ := r.st.GetZFSReplicaState(r.item.ID, "cache/appdata/plex")
	want := "tank/bombvault-replica/bottich/cache/appdata/plex"
	if !ok || st.TargetPath != want || st.SourceGUID != "9223372036854775808" || st.TargetBase != replicaSnap {
		t.Errorf("state = %+v, want the member at %s", st, want)
	}
	if e := r.entries[0]; e.TargetBase != "tank/bombvault-replica/bottich" || e.Owner == "" || e.Owner != r.s.instanceID() {
		t.Errorf("entry = %+v, want the server folder and this instance as owner", e)
	}
}

func TestARenamedInstanceKeepsWritingIntoItsFirstFolder(t *testing.T) {
	r := newReplicaRig(t)
	ctx := context.Background()
	landsUnder := func() string {
		t.Helper()
		if err := r.s.ReplicateZFSDataset(ctx, r.item.ID); err != nil {
			t.Fatal(err)
		}
		return r.entries[len(r.entries)-1].TargetBase
	}
	landsUnder()
	settings, _ := r.st.GetSettings()
	settings.InstanceName = "tower"
	if err := r.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if got := landsUnder(); got != "tank/bombvault-replica/bottich" {
		t.Errorf("the run after the rename lands under %q", got)
	}

	for _, target := range []zfsReplicaTargetExport{{Kind: store.ZFSReplicaTargetNone}, {Kind: store.ZFSReplicaTargetServer, ID: r.srv.ID}} {
		if err := r.s.PatchZFSReplica(ctx, r.item.ID, ZFSReplicaPatch{Target: &target}); err != nil {
			t.Fatal(err)
		}
	}
	if states, _ := r.st.ListZFSReplicaStates(r.item.ID); len(states) != 0 {
		t.Fatalf("switching off kept the member state %+v", states)
	}
	if got := landsUnder(); got != "tank/bombvault-replica/bottich" {
		t.Errorf("the run after switching off and on again lands under %q", got)
	}
}

func TestAFolderAnotherInstanceCreatedIsRefused(t *testing.T) {
	r := newReplicaRig(t)
	r.server.owners["tank/bombvault-replica/bottich"] = "someoneelse"

	err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID)
	if code := zfsReplicaCode(err); code != "target-owned" {
		t.Fatalf("run = %v (%s), want target-owned", err, code)
	}
	if len(r.entries) != 0 {
		t.Error("the engine ran into a folder another instance owns")
	}
	run := r.lastRun()
	if run.Status != "failed" || zfsRunCode(run.Error) != "target-owned" {
		t.Errorf("run = %+v, want failed with the code at the end", run.Run)
	}
}

func TestAnEmptyParentOfOurOwnIsHandedOnAsPlaceholder(t *testing.T) {
	r := newReplicaRig(t)
	id := r.s.instanceID()
	r.server.owners["tank/bombvault-replica/bottich"] = id
	r.server.owners["tank/bombvault-replica/bottich/cache/appdata"] = id
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	if got := r.entries[0].Placeholders; !slices.Equal(got, []string{"tank/bombvault-replica/bottich/cache/appdata"}) {
		t.Errorf("placeholders = %q", got)
	}
	if st, _, _ := r.st.GetZFSReplicaState(r.item.ID, "cache/appdata"); !st.CreatedParent {
		t.Error("the root's state does not say it landed on our own parent")
	}

	r.server.owners["tank/bombvault-replica/bottich/cache/appdata"] = "someoneelse"
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	if got := r.entries[1].Placeholders; got != nil {
		t.Errorf("a run with state looked for placeholders again: %q", got)
	}
}

func TestFailedMembersFailTheRunAndKeepTheirState(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	r.result = func(e zfsrepl.Entry) (zfsrepl.Result, error) {
		m := zfsrepl.MemberResult{Dataset: e.Root, Target: e.TargetBase + "/" + e.Root, Code: "no-common-base"}
		e.Finished(m)
		return zfsrepl.Result{Snapshot: "bombvault-replica-20261010030000", Members: []zfsrepl.MemberResult{m}}, nil
	}
	err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID)
	if zfsReplicaCode(err) != "no-common-base" {
		t.Fatalf("run = %v, want no-common-base", err)
	}
	if st, ok, _ := r.st.GetZFSReplicaState(r.item.ID, "cache/appdata"); !ok || st.SourceBase != replicaSnap {
		t.Errorf("the failed member lost its state: %+v", st)
	}
	if _, ok, _ := r.st.GetZFSReplicaState(r.item.ID, "cache/appdata/plex"); ok {
		t.Error("a member the run did not find keeps its state")
	}

	_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
	if v["state"] != "failed" || v["code"] != "no-common-base" {
		t.Errorf("view = %v, want the failure and its code", v)
	}
}

func TestOnlyOneRunPerItemAndItDoesNotTakeTheDomainLock(t *testing.T) {
	r := newReplicaRig(t)
	release := make(chan struct{})
	started := make(chan struct{})
	r.result = func(e zfsrepl.Entry) (zfsrepl.Result, error) {
		close(started)
		<-release
		return zfsrepl.Result{}, nil
	}
	if _, err := r.s.StartZFSReplica(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	<-started
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/run", "")
	if m["ok"] != false {
		t.Errorf("a second run started: %v", m)
	}
	if _, busy := r.s.domainBusy(zfsDomain); busy {
		t.Error("a replica run holds the ZFS domain lock")
	}
	_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
	if v["state"] != "running" {
		t.Errorf("state while running = %v", v["state"])
	}
	close(release)
	r.s.replica.work.Wait()
	if r.s.zfsReplicaRunning(r.item.ID) {
		t.Error("the item lock outlived its run")
	}
}

func TestASuccessfulBackupStartsTheReplicaOnlyWhenItFollowsBackups(t *testing.T) {
	r := newReplicaRig(t)
	r.s.replicateAfterZFSBackup(context.Background(), r.item)
	r.s.replica.work.Wait()
	if len(r.entries) != 1 {
		t.Fatalf("after a backup the replica ran %d times, want once", len(r.entries))
	}
	if err := r.st.SetZFSReplicaAfterBackup(r.item.ID, false); err != nil {
		t.Fatal(err)
	}
	d, _ := r.st.GetZFSDataset(r.item.ID)
	r.s.replicateAfterZFSBackup(context.Background(), d)
	r.s.replica.work.Wait()
	if len(r.entries) != 1 {
		t.Error("a replica on its own cadence ran after a backup")
	}
}

func TestARunToADisabledServerIsRefusedWithoutARunRow(t *testing.T) {
	r := newReplicaRig(t)
	r.srv.Enabled = false
	if err := r.st.UpdateZFSReplicaServer(r.srv); err != nil {
		t.Fatal(err)
	}
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/run", "")
	if m["ok"] != false || m["code"] != "server-disabled" {
		t.Errorf("run = %v, want server-disabled", m)
	}
	if _, ok, _ := r.st.LatestZFSReplicaRun(r.item.ID); ok {
		t.Error("a refused run left a row")
	}
	_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
	if v["state"] != "waiting" {
		t.Errorf("state = %v, want waiting", v["state"])
	}
}

func TestSwitchingTheReplicaOffCleansTheSourceAndKeepsTheTarget(t *testing.T) {
	r := newReplicaRig(t)
	r.host.tree = []string{"cache/appdata", "cache/appdata/plex"}
	r.host.points["cache/appdata"] = "cache/appdata@manual\t5\t9\ncache/appdata@" + replicaSnap + "\t7\t10\ncache/appdata#" + replicaSnap + "\t7\t10\n"
	r.host.points["cache/appdata/plex"] = "cache/appdata/plex#bombvault-replica-20261008030000\t3\t8\n"

	_, m, _ := r.call(http.MethodPatch, "/api/zfs/datasets/"+r.item.ID+"/replica", `{"target":{"kind":"none","id":"x"}}`)
	if m["ok"] != true {
		t.Fatalf("patch = %v", m)
	}
	var destroyed []string
	for _, c := range r.host.did("destroy") {
		destroyed = append(destroyed, c[len(c)-1])
	}
	want := []string{"cache/appdata@" + replicaSnap}
	if !slices.Equal(destroyed, want) {
		t.Errorf("destroyed %q, want %q", destroyed, want)
	}
	if rel := r.host.did("release"); len(rel) != 1 {
		t.Errorf("released %q, want the held replica snapshot", rel)
	}
	if len(r.server.calls) != 0 {
		t.Errorf("the target was touched: %q", r.server.calls)
	}
	d, _ := r.st.GetZFSDataset(r.item.ID)
	if d.Replica.TargetKind != store.ZFSReplicaTargetNone || d.Replica.TargetID != "" {
		t.Errorf("replica = %+v, want switched off", d.Replica)
	}
}

func TestRemovingAnEntryCleansItsReplicaFromTheSource(t *testing.T) {
	r := newReplicaRig(t)
	r.host.tree = []string{"cache/appdata"}
	r.host.points["cache/appdata"] = "cache/appdata@" + replicaSnap + "\t7\t10\n"
	if _, err := r.s.deleteZFSDatasetLocked(context.Background(), r.item.ID, false); err != nil {
		t.Fatal(err)
	}
	if len(r.host.did("release")) != 1 || len(r.host.did("destroy")) != 1 || len(r.host.did("bookmark")) != 1 {
		t.Errorf("release %q, destroy %q, bookmark %q, want the snapshot kept as a bookmark",
			r.host.did("release"), r.host.did("destroy"), r.host.did("bookmark"))
	}
}

func TestPatchValidatesBeforeItWrites(t *testing.T) {
	r := newReplicaRig(t)
	for _, body := range []string{
		`{"target":{"kind":"server","id":"nope"}}`,
		`{"target":{"kind":"elsewhere","id":""}}`,
		`{"cadence":"every 3 days"}`,
		`{"cadence":"everyN 3 03:00"}`,
		`{"keep":{"preset":"forever","own":[0,0,0,0,0]}}`,
		`{"keep":{"preset":"own","own":[0,-1,0,0,0]}}`,
	} {
		_, m, _ := r.call(http.MethodPatch, "/api/zfs/datasets/"+r.item.ID+"/replica", body)
		if m["ok"] != false {
			t.Errorf("%s was accepted", body)
		}
	}
	d, _ := r.st.GetZFSDataset(r.item.ID)
	if d.Replica.TargetID != r.srv.ID || d.Replica.Keep != store.DefaultZFSReplicaKeep {
		t.Errorf("a refused patch changed the item: %+v", d.Replica)
	}

	_, m, _ := r.call(http.MethodPatch, "/api/zfs/datasets/"+r.item.ID+"/replica",
		`{"afterBackup":false,"cadence":"daily 05:00","keep":{"preset":"long","own":[0,0,0,0,0]}}`)
	if m["ok"] != true {
		t.Fatalf("patch = %v", m)
	}
	_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
	keep, _ := v["keep"].(map[string]any)
	if v["afterBackup"] != false || v["cadence"] != "daily 05:00" || keep["preset"] != "long" || v["peerState"] != "" {
		t.Errorf("view after the patch = %v", v)
	}
}

func TestTheViewListsTheSnapshotsOnTheTargetNewestFirst(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	replica := "tank/bombvault-replica/bottich/cache/appdata"
	r.server.points[replica] = replica + "@bombvault-replica-20261008030000\t3\t8\n" +
		replica + "@manual\t4\t9\n" + replica + "@" + replicaSnap + "\t7\t10\n"

	_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
	snaps, _ := v["snapshots"].([]any)
	if len(snaps) != 2 {
		t.Fatalf("snapshots = %v, want the two replica snapshots", v["snapshots"])
	}
	first, _ := snaps[0].(map[string]any)
	if first["name"] != replicaSnap || first["created"] != "2026-10-09T03:00:00Z" {
		t.Errorf("newest = %v", first)
	}
	members, _ := v["members"].([]any)
	if len(members) != 2 || v["state"] != "ok" || v["lastBytes"] != 8192.0 {
		t.Errorf("view = %v", v)
	}
}

func TestARestoreBringsTheTreeBackAndMountsWhatIsNotEncrypted(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	replica := "tank/bombvault-replica/bottich/cache/appdata"
	r.server.points[replica] = replica + "@" + replicaSnap + "\t7\t10\n"
	r.server.tree = []string{replica, replica + "/plex"}
	var asked zfsrepl.Restore
	r.s.replica.bringBack = func(_ context.Context, _, _ zfsrepl.End, rs zfsrepl.Restore) (zfsrepl.Restored, error) {
		asked = rs
		root := zfsrepl.RestoreName(rs.Dataset, rs.Now())
		return zfsrepl.Restored{
			Root: root,
			Members: []zfsrepl.RestoredMember{
				{Dataset: root},
				{Dataset: root + "/vault", Encrypted: true},
				{Dataset: root + "/vm", Volume: true},
				{Dataset: root + "/plex"},
			},
			Skipped: []string{replica + "/later"},
		}, nil
	}

	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/restore", `{"snapshot":"bombvault-replica-20261001030000"}`)
	if m["ok"] != false || m["code"] != "not-found" {
		t.Errorf("a snapshot the target lacks = %v", m)
	}
	_, m, _ = r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/restore", `{"snapshot":"`+replicaSnap+`"}`)
	root, _ := m["dataset"].(string)
	if m["ok"] != true || m["runId"] == "" || !strings.HasPrefix(root, "cache/appdata-bombvault-restore-") || m["keyNeeded"] != false {
		t.Fatalf("restore = %v", m)
	}
	r.s.replica.work.Wait()
	if asked.Replica != replica || asked.Snapshot != replicaSnap || zfsrepl.RestoreName("cache/appdata", asked.Now()) != root {
		t.Errorf("bring back = %+v", asked)
	}
	var mounted []string
	for _, c := range r.host.did("mount") {
		mounted = append(mounted, c[len(c)-1])
	}
	if want := []string{root, root + "/plex"}; !slices.Equal(mounted, want) {
		t.Errorf("mounted %q, want %q", mounted, want)
	}
	runs, _ := r.st.RecentRunsOfKind(r.item.ID, "restore", 1)
	if len(runs) != 1 || runs[0].Status != "success" ||
		!strings.Contains(runs[0].Error, root+"/vault") || !strings.Contains(runs[0].Error, replica+"/later") {
		t.Errorf("restore run = %+v, want a success naming the locked and the left out member", runs)
	}
}

func TestAnEncryptedRootIsAnnouncedAsNeedingItsKey(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	replica := "tank/bombvault-replica/bottich/cache/appdata"
	r.server.points[replica] = replica + "@" + replicaSnap + "\t7\t10\n"
	r.server.tree = []string{replica}
	r.server.locked = map[string]bool{replica: true}
	r.s.replica.bringBack = func(_ context.Context, _, _ zfsrepl.End, rs zfsrepl.Restore) (zfsrepl.Restored, error) {
		root := zfsrepl.RestoreName(rs.Dataset, rs.Now())
		return zfsrepl.Restored{Root: root, Members: []zfsrepl.RestoredMember{{Dataset: root, Encrypted: true}}}, nil
	}
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/restore", `{"snapshot":"`+replicaSnap+`"}`)
	if m["ok"] != true || m["keyNeeded"] != true {
		t.Fatalf("restore = %v, want keyNeeded", m)
	}
	r.s.replica.work.Wait()
	if mounts := r.host.did("mount"); mounts != nil {
		t.Errorf("an encrypted root was mounted: %q", mounts)
	}
}

func TestAnEncryptedChildUnderAPlainRootIsAnnouncedAsNeedingItsKey(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	replica := "tank/bombvault-replica/bottich/cache/appdata"
	r.server.points[replica] = replica + "@" + replicaSnap + "\t7\t10\n"
	r.server.tree = []string{replica, replica + "/plex", replica + "/vault"}
	r.server.locked = map[string]bool{replica + "/vault": true}
	r.s.replica.bringBack = func(_ context.Context, _, _ zfsrepl.End, rs zfsrepl.Restore) (zfsrepl.Restored, error) {
		root := zfsrepl.RestoreName(rs.Dataset, rs.Now())
		return zfsrepl.Restored{Root: root, Members: []zfsrepl.RestoredMember{{Dataset: root}, {Dataset: root + "/vault", Encrypted: true}}}, nil
	}
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/restore", `{"snapshot":"`+replicaSnap+`"}`)
	if m["ok"] != true || m["keyNeeded"] != true {
		t.Fatalf("restore = %v, want keyNeeded for the encrypted child", m)
	}
	r.s.replica.work.Wait()
}

func TestWithTheZFSDomainOffNoReplicaRuns(t *testing.T) {
	r := newReplicaRig(t)
	settings, _ := r.st.GetSettings()
	settings.ZFSEnabled = false
	if err := r.st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/run", "")
	if m["ok"] != false || m["code"] != "domain-off" {
		t.Errorf("run = %v, want domain-off", m)
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil || len(r.entries) != 0 {
		t.Errorf("a scheduled replica ran with the domain off: %v, %d runs", err, len(r.entries))
	}
}

func TestTheLocalPoolsAnswerBare(t *testing.T) {
	r := newReplicaRig(t)
	r.host.pools = "cache\t10\t20\n"
	_, _, list := r.call(http.MethodGet, "/api/zfs/replica/local-pools", "")
	if len(list) != 1 {
		t.Fatalf("local pools = %v", list)
	}
	if p, _ := list[0].(map[string]any); p["name"] != "cache" || p["sizeBytes"] != 30.0 || p["freeBytes"] != 20.0 {
		t.Errorf("pool = %v", p)
	}
}

func TestACurrentReplicaIsTheZFSCopyOffThePremisesButNoBackup(t *testing.T) {
	r := newReplicaRig(t)
	settings, _ := r.st.GetSettings()
	now := time.Now().Unix()

	if _, _, ok := r.s.zfsReplicaCoverage(now, settings); ok {
		t.Fatal("a replica that never ran covers the domain")
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	if at, _, ok := r.s.zfsReplicaCoverage(now, settings); !ok || at == 0 {
		t.Fatal("a current replica does not cover the domain")
	}
	statuses, err := r.s.domainStatusFrom(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range statuses {
		if e.Domain != zfsDomain {
			continue
		}
		if !e.OffsiteConfigured || !e.OffPremisesCovered || e.ReplicaState != "ok" || e.Protection == "red" {
			t.Errorf("zfs status = %+v, want the replica as the off-site copy", e)
		}
	}

	sites, rule := r.s.zfsItemSites(now, r.item, settings, 0)
	if rule != "one-copy" || sites != 2 {
		t.Errorf("replica without a backup: %d sites, %s; want one copy on two sites", sites, rule)
	}
	sites, rule = r.s.zfsItemSites(now, r.item, settings, now)
	if rule != "met" || sites != 2 {
		t.Errorf("replica and a local backup: %d sites, %s; want met", sites, rule)
	}
	if _, current := r.s.zfsReplicaCurrency(now+3*86400, r.item, settings); current {
		t.Error("a replica three days old still counts against a daily schedule")
	}

	lines := r.s.zfsReplicaDigestLines(now, settings)
	if len(lines) != 1 || !strings.Contains(lines[0], "cache/appdata: current") {
		t.Errorf("digest lines = %q", lines)
	}
}

// replicaMembers reads the members of the item's replica view by dataset.
func (r *replicaRig) replicaMembers() (map[string]any, map[string]map[string]any) {
	r.t.Helper()
	_, v, _ := r.call(http.MethodGet, "/api/zfs/datasets/"+r.item.ID+"/replica", "")
	members := map[string]map[string]any{}
	list, _ := v["members"].([]any)
	for _, m := range list {
		mm, _ := m.(map[string]any)
		members[mm["dataset"].(string)] = mm
	}
	return v, members
}

func TestARunCutShortByARestartLeavesItsUnfinishedMembersWaitingToResume(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	runID, err := r.st.StartRun(r.item.ID, store.ZFSReplicaRunKind)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.st.AddZFSReplicaRunMember(store.ZFSReplicaRunMember{
		RunID: runID, ItemID: r.item.ID, Dataset: "cache/appdata", Snapshot: replicaSnap, FinishedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.st.ReapInterruptedRuns(); err != nil {
		t.Fatal(err)
	}

	v, members := r.replicaMembers()
	if v["state"] != "failed" || v["code"] != "interrupted" {
		t.Errorf("replica = %v %v, want failed with interrupted", v["state"], v["code"])
	}
	if m := members["cache/appdata"]; m["state"] != "ok" {
		t.Errorf("the member that finished = %v", m)
	}
	if m := members["cache/appdata/plex"]; m["state"] != "failed" || m["code"] != "interrupted" {
		t.Errorf("the member the restart cut off = %v, want it waiting to resume", m)
	}
}

func TestARunStoppedByAShutdownEndsWithItsReasonCode(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	r.s.replica.run = func(ctx context.Context, _, _ zfsrepl.End, e zfsrepl.Entry) (zfsrepl.Result, error) {
		m := zfsrepl.MemberResult{Dataset: e.Root, Target: e.TargetBase + "/" + e.Root, Snapshot: replicaSnap, GUID: 1 << 63}
		e.Finished(m)
		r.s.EndDetachedWork()
		<-ctx.Done()
		return zfsrepl.Result{Snapshot: replicaSnap, Members: []zfsrepl.MemberResult{m}}, ctx.Err()
	}
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err == nil {
		t.Fatal("a run the shutdown stopped succeeded")
	}
	if run := r.lastRun(); run.Status != "cancelled" || zfsRunCode(run.Error) != "interrupted" {
		t.Errorf("run = %s %q, want cancelled with interrupted", run.Status, run.Error)
	}
	if _, members := r.replicaMembers(); members["cache/appdata/plex"]["code"] != "interrupted" {
		t.Errorf("the member the shutdown cut off = %v", members["cache/appdata/plex"])
	}
}

func TestARestoreThatFailsPartwayNamesWhatLanded(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	replica := "tank/bombvault-replica/bottich/cache/appdata"
	r.server.points[replica] = replica + "@" + replicaSnap + "\t7\t10\n"
	r.server.tree = []string{replica, replica + "/plex", replica + "/vault"}
	var root string
	r.s.replica.bringBack = func(_ context.Context, _, _ zfsrepl.End, rs zfsrepl.Restore) (zfsrepl.Restored, error) {
		root = zfsrepl.RestoreName(rs.Dataset, rs.Now())
		got := zfsrepl.Restored{Root: root, Members: []zfsrepl.RestoredMember{{Dataset: root}, {Dataset: root + "/plex"}}}
		return got, &zfs.CmdError{Code: "ssh-unreachable", Stderr: "client_loop: send disconnect: Broken pipe"}
	}
	_, m, _ := r.call(http.MethodPost, "/api/zfs/datasets/"+r.item.ID+"/replica/restore", `{"snapshot":"`+replicaSnap+`"}`)
	if m["ok"] != true {
		t.Fatalf("restore = %v", m)
	}
	r.s.replica.work.Wait()
	runs, _ := r.st.RecentRunsOfKind(r.item.ID, "restore", 1)
	if len(runs) != 1 || runs[0].Status != "failed" || zfsRunCode(runs[0].Error) != "ssh-unreachable" ||
		!strings.Contains(runs[0].Error, root+", "+root+"/plex") {
		t.Errorf("restore run = %+v, want a failure naming what landed", runs)
	}
}

func TestAPanickingReplicaFailsOnlyItsOwnRunAndEndsItsBar(t *testing.T) {
	r := newReplicaRig(t)
	r.s.progress = progress.NewStore()
	r.result = func(zfsrepl.Entry) (zfsrepl.Result, error) { panic("boom") }
	backupID, err := r.st.StartRun(r.item.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.s.StartZFSReplica(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	r.s.replica.work.Wait()
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err == nil {
		t.Error("a scheduled replica that panicked succeeded")
	}
	r.s.replicateAfterZFSBackup(context.Background(), r.item)
	r.s.replica.work.Wait()

	runs, _ := r.st.RecentRunsOfKind(r.item.ID, store.ZFSReplicaRunKind, 10)
	if len(runs) != 3 {
		t.Fatalf("%d replica runs, want 3", len(runs))
	}
	for _, run := range runs {
		if run.Status != "failed" {
			t.Errorf("replica run = %s %q, want failed", run.Status, run.Error)
		}
	}
	if backup, _ := r.st.GetRun(backupID); backup.Status != "running" {
		t.Errorf("the backup running beside it = %+v, want it untouched", backup)
	}
	for _, e := range r.s.progress.Snapshot() {
		if e.Key == zfsReplicaProgressKey(r.item.ID) && e.Active {
			t.Errorf("the replica bar is still active: %+v", e)
		}
	}
}

func TestAPanickingBringBackFailsOnlyItsOwnRun(t *testing.T) {
	r := newReplicaRig(t)
	if err := r.s.ReplicateZFSDataset(context.Background(), r.item.ID); err != nil {
		t.Fatal(err)
	}
	replica := "tank/bombvault-replica/bottich/cache/appdata"
	r.server.points[replica] = replica + "@" + replicaSnap + "\t7\t10\n"
	r.server.tree = []string{replica}
	r.s.replica.bringBack = func(context.Context, zfsrepl.End, zfsrepl.End, zfsrepl.Restore) (zfsrepl.Restored, error) {
		panic("boom")
	}
	backupID, err := r.st.StartRun(r.item.ID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	ack, err := r.s.StartZFSReplicaRestore(context.Background(), r.item.ID, replicaSnap)
	if err != nil {
		t.Fatal(err)
	}
	r.s.replica.work.Wait()
	if run, _ := r.st.GetRun(ack.RunID); run.Status != "failed" {
		t.Errorf("restore run = %s %q, want failed", run.Status, run.Error)
	}
	if backup, _ := r.st.GetRun(backupID); backup.Status != "running" {
		t.Errorf("the backup running beside it = %+v, want it untouched", backup)
	}
}

func TestAPeerThatDoesNotAnswerStillTakesTheReplicaEdit(t *testing.T) {
	r := newReplicaRig(t)
	r.host.tree = []string{"cache/appdata"}
	_, m, _ := r.call(http.MethodPatch, "/api/zfs/datasets/"+r.item.ID+"/replica",
		`{"target":{"kind":"peer","id":"peer-offline"},"afterBackup":false,"cadence":"daily 03:00"}`)
	if m["ok"] != true {
		t.Fatalf("patch = %v, want the stored edit answered as one", m)
	}
	d, _ := r.st.GetZFSDataset(r.item.ID)
	if d.Replica.TargetKind != store.ZFSReplicaTargetPeer || d.Replica.AfterBackup || d.Replica.Cadence != "daily 03:00" {
		t.Errorf("replica = %+v", d.Replica)
	}
}

func TestAServerForgetsItsHostKeyOnlyWhenItsAddressChanges(t *testing.T) {
	r := newReplicaRig(t)
	pin := r.s.zfsReplicaKnownHosts(r.srv.ID)
	if err := os.MkdirAll(filepath.Dir(pin), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pin, []byte("backup.lan ssh-ed25519 AAAA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, m, _ := r.call(http.MethodPatch, "/api/zfs/replica/servers/"+r.srv.ID, `{"user":"replica"}`); m["ok"] != true {
		t.Fatalf("patch = %v", m)
	}
	if _, err := os.Stat(pin); err != nil {
		t.Errorf("a new user on the same host dropped the pinned key: %v", err)
	}
	if _, m, _ := r.call(http.MethodPatch, "/api/zfs/replica/servers/"+r.srv.ID, `{"host":"other.lan"}`); m["ok"] != true {
		t.Fatalf("patch = %v", m)
	}
	if _, err := os.Stat(pin); !os.IsNotExist(err) {
		t.Errorf("another host kept the pinned key: %v", err)
	}
}

func TestAServerIDNeverReachesTheReplicaKey(t *testing.T) {
	r := newReplicaRig(t)
	srv, err := r.st.CreateZFSReplicaServer(store.ZFSReplicaServer{
		ID: "id_ed25519", Name: "odd", Host: "odd.lan", User: "root", Port: 22, Pool: "tank", Root: "tank/r", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(r.s.zfsReplicaKeyDir(), "id_ed25519")
	if err := os.MkdirAll(r.s.zfsReplicaKeyDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	host := "new.lan"
	if _, err := r.s.PatchZFSReplicaServer(srv.ID, ZFSReplicaServerPatch{Host: &host}); err != nil {
		t.Fatal(err)
	}
	if err := r.s.DeleteZFSReplicaServer(context.Background(), srv.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(key); err != nil {
		t.Errorf("forgetting the host key of server %s took the replica key: %v", srv.ID, err)
	}
}
