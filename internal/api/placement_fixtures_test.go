package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// placementFixture is a Service and a Handler over an in-memory store, with a
// restic fake that answers per location and domain paths that exist.
type placementFixture struct {
	t     *testing.T
	svc   *Service
	h     *Handler
	st    *store.Repo
	db    *sql.DB // for facts no store writer can set, such as when a run happened
	eng   *placementEngine
	dock  *placementDocker
	virsh *placementVirsh
	root  string // host mount root, slash-spelled
	// session is the session cookie requests carry, empty for none.
	session string
}

func newPlacementFixture(t *testing.T) *placementFixture {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	settings.VMsPath = "backups/vms"
	settings.FilesPath = "backups/files"
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(dir)
	cfg := config.Config{AppKey: strings.Repeat("a", 64), DataDir: dir, HostMountRoot: root}
	eng := &placementEngine{
		snaps:         map[string][]restic.Snapshot{},
		listErr:       map[string]error{},
		copyErr:       map[string]error{},
		opens:         map[string]bool{},
		openErr:       map[string]error{},
		initErr:       map[string]error{},
		initStaysShut: map[string]bool{},
		lists:         map[string]int{},
	}
	dock := &placementDocker{installed: map[string]bool{}}
	virsh := &placementVirsh{}
	svc := NewService(cfg, st, dock, virsh, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	f := &placementFixture{
		t: t, svc: svc, h: NewHandler(cfg, st, dock, svc, sched, spike.DefaultProbes()),
		st: st, db: db, eng: eng, dock: dock, virsh: virsh, root: root,
	}
	for _, domain := range store.PlacementDomains {
		f.makeRepo(f.domainPath(domain))
	}
	return f
}

// domainPath is where a domain's own repository lies.
func (f *placementFixture) domainPath(domain string) string {
	return f.root + "/backups/" + domain
}

// makeRepo leaves a local repository the service treats as created.
func (f *placementFixture) makeRepo(loc string) {
	f.t.Helper()
	dir := filepath.FromSlash(loc)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("x"), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

// localRepo makes a local repository at rel under the mount root and returns its
// location slash-spelled, the key hold and listErr are set under.
func (f *placementFixture) localRepo(rel string) string {
	f.t.Helper()
	dir, err := f.svc.resolveRepo(rel)
	if err != nil {
		f.t.Fatalf("resolve %s: %v", rel, err)
	}
	loc := filepath.ToSlash(dir)
	f.makeRepo(loc)
	return loc
}

// singletonRepo gives flash or config its domain repository and returns its
// location slash-spelled.
func (f *placementFixture) singletonRepo(domain string) string {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	if domain == "flash" {
		return f.localRepo(settings.FlashPath)
	}
	return f.localRepo(settings.ConfigPath)
}

// target adds an enabled target behind the domain's last one.
func (f *placementFixture) target(domain, name, location string) store.OffsiteTarget {
	f.t.Helper()
	t, err := f.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: domain, Name: name, Repo: location, Enabled: true})
	if err != nil {
		f.t.Fatalf("target %s: %v", name, err)
	}
	f.sampled(domain, t.ID)
	return t
}

// fieldTarget fills the domain's off-site field and returns the row it edits.
func (f *placementFixture) fieldTarget(domain, location string) store.OffsiteTarget {
	f.t.Helper()
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	switch domain {
	case "containers":
		settings.ContainersOffsite = location
	case "vms":
		settings.VMsOffsite = location
	case "files":
		settings.FilesOffsite = location
	case "flash":
		settings.FlashOffsite = location
	case "config":
		settings.ConfigOffsite = location
	}
	if err := f.st.UpdateSettings(settings); err != nil {
		f.t.Fatal(err)
	}
	if err := f.svc.syncPrimaryOffsiteTarget(domain, settings); err != nil {
		f.t.Fatal(err)
	}
	t, ok, err := f.st.FieldOffsiteTarget(domain)
	if err != nil || !ok {
		f.t.Fatalf("field target of %s: ok=%v err=%v", domain, ok, err)
	}
	f.sampled(domain, t.ID)
	return t
}

// sampled gives a target a fresh size sample, so a replication does not list it
// again in the background to measure it.
func (f *placementFixture) sampled(domain, targetID string) {
	f.t.Helper()
	for _, source := range []string{"offsite", offsiteStatSource(targetID)} {
		if err := f.st.AddRepoStat(store.RepoStat{Domain: domain, Source: source, At: time.Now().Unix()}); err != nil {
			f.t.Fatal(err)
		}
	}
}

// namedRepo adds an enabled named repository; a local one gets its folder.
func (f *placementFixture) namedRepo(name, location string) store.OffsiteTarget {
	f.t.Helper()
	r, err := f.st.UpsertOffsiteTarget(store.OffsiteTarget{Role: store.RoleRepo, Name: name, Repo: location, Enabled: true})
	if err != nil {
		f.t.Fatalf("named repo %s: %v", name, err)
	}
	if !restic.IsRemoteRepo(location) {
		f.makeRepo(f.root + "/" + location)
	}
	return r
}

// direct creates the target's direct repository at the suggested location, with
// the config marker a local repository there would have.
func (f *placementFixture) direct(target store.OffsiteTarget) store.OffsiteTarget {
	f.t.Helper()
	loc := directLocationFor(target).Location
	row, err := f.st.CreateCompanionRepo(target.ID, placementTargetName(target)+" direct", loc)
	if err != nil {
		f.t.Fatalf("CreateCompanionRepo: %v", err)
	}
	if restic.IsRemoteRepo(loc) {
		return row
	}
	dir, err := f.svc.resolveRepo(loc)
	if err != nil {
		f.t.Fatalf("resolve %s: %v", loc, err)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), nil, 0o600); err != nil {
		f.t.Fatal(err)
	}
	return row
}

// offPremises marks an existing named repository as standing off the
// premises, for a test that needs one without going through the API.
func (f *placementFixture) offPremises(repoID string) {
	f.t.Helper()
	r, err := f.st.GetNamedRepo(repoID)
	if err != nil {
		f.t.Fatalf("offPremises %s: %v", repoID, err)
	}
	r.OffPremises = true
	if _, err := f.st.UpsertOffsiteTarget(r); err != nil {
		f.t.Fatalf("offPremises %s: %v", repoID, err)
	}
}

// appendOnly flags an existing off-site target append-only, for a test that
// needs one without going through the API.
func (f *placementFixture) appendOnly(targetID string) {
	f.t.Helper()
	t, ok, err := f.st.GetOffsiteTarget(targetID)
	if err != nil || !ok {
		f.t.Fatalf("appendOnly %s: found %v, %v", targetID, ok, err)
	}
	t.Immutable = true
	if _, err := f.st.UpsertOffsiteTarget(t); err != nil {
		f.t.Fatalf("appendOnly %s: %v", targetID, err)
	}
}

func (f *placementFixture) container(name, repoID string) store.Target {
	f.t.Helper()
	if _, err := f.st.WritePlacement(store.ItemRef{Domain: "containers", Key: name}, &store.HomeWrite{Repo: repoID, Choice: store.RepoChosen}, nil, nil); err != nil {
		f.t.Fatalf("container %s: %v", name, err)
	}
	tg, err := f.st.GetTargetByContainer(name)
	if err != nil {
		f.t.Fatalf("container %s: %v", name, err)
	}
	return tg
}

func (f *placementFixture) vm(name, repoID string) store.VMTarget {
	f.t.Helper()
	if _, err := f.st.WritePlacement(store.ItemRef{Domain: "vms", Key: name}, &store.HomeWrite{Repo: repoID, Choice: store.RepoChosen}, nil, nil); err != nil {
		f.t.Fatalf("vm %s: %v", name, err)
	}
	vm, err := f.st.GetVMTargetByName(name)
	if err != nil {
		f.t.Fatalf("vm %s: %v", name, err)
	}
	return vm
}

// zvolVM adds a VM whose definition holds one zvol disk, sda, the disk a restore
// of one of its runs looks up as a snapshot of its own.
func (f *placementFixture) zvolVM(name string) store.VMTarget {
	f.t.Helper()
	xml := `<domain type='kvm'><name>` + name + `</name><devices><disk type='block' device='disk'>` +
		`<source dev='/dev/zvol/tank/vms/` + name + `-sda'/><target dev='sda' bus='sata'/></disk></devices></domain>`
	def, err := json.Marshal(vmDefinition{DomainXML: xml})
	if err != nil {
		f.t.Fatal(err)
	}
	vm := f.vm(name, "")
	vm.Definition = string(def)
	vm, err = f.st.UpsertVMTarget(vm)
	if err != nil {
		f.t.Fatalf("zvol vm %s: %v", name, err)
	}
	return vm
}

// fileSet adds an enabled file set with a source folder under files/.
func (f *placementFixture) fileSet(name, repoID string) store.FileSet {
	f.t.Helper()
	path := "files/" + name
	if err := os.MkdirAll(filepath.FromSlash(f.root+"/"+path), 0o750); err != nil {
		f.t.Fatal(err)
	}
	fs, err := f.st.CreateFileSet(store.FileSet{Name: name, Path: path, Enabled: true, Repo: repoID, RepoChosen: store.RepoChosen})
	if err != nil {
		f.t.Fatalf("file set %s: %v", name, err)
	}
	return fs
}

// zfsItem adds a ZFS entry for the root dataset and returns it with the
// location of the domain repository its runs are written to.
func (f *placementFixture) zfsItem(dataset string) (store.ZFSDataset, string) {
	f.t.Helper()
	d, err := f.st.CreateZFSDataset(store.ZFSDataset{Dataset: dataset, Enabled: true})
	if err != nil {
		f.t.Fatalf("zfs entry %s: %v", dataset, err)
	}
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	return d, f.localRepo(settings.ZFSPath)
}

// zfsRun records a run of a ZFS entry that backed up the given datasets.
func (f *placementFixture) zfsRun(itemID, stamp string, datasets ...string) {
	f.t.Helper()
	id, err := f.st.StartRun(itemID, "backup")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.st.RecordZFSRun(id, itemID, stamp, -1, ""); err != nil {
		f.t.Fatal(err)
	}
	for _, dataset := range datasets {
		if err := f.st.AddZFSRunMember(store.ZFSRunMember{RunID: id, Dataset: dataset, Outcome: outcomeBackedUp}); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := f.st.FinishRun(id, "success", "", 0, ""); err != nil {
		f.t.Fatal(err)
	}
}

// zfsSnap is the snapshot one run wrote for one dataset, read from the host
// snapshot stamp names.
func zfsSnap(id, dataset, stamp string) restic.Snapshot {
	at, _ := zfs.StampTime(stamp)
	return restic.Snapshot{
		ID: id, Time: at.Format(time.RFC3339), Tags: []string{"zfs:" + dataset},
		Paths: []string{"/host/" + dataset + "/.zfs/snapshot/" + stamp},
	}
}

func (f *placementFixture) rule(domain, identity string, skip ...string) {
	f.t.Helper()
	if err := f.st.SetCopyRule(domain, identity, skip); err != nil {
		f.t.Fatalf("rule %s: %v", identity, err)
	}
}

func (f *placementFixture) setDefault(domain, home string, skip ...string) {
	f.t.Helper()
	if _, err := f.st.PutPlacementDefault(domain, home, skip); err != nil {
		f.t.Fatalf("default %s: %v", domain, err)
	}
}

func (f *placementFixture) listing(domain, targetID string, at int64, rows ...store.ItemCopies) {
	f.t.Helper()
	if err := f.st.RecordTargetListing(domain, targetID, at, rows); err != nil {
		f.t.Fatalf("listing %s: %v", targetID, err)
	}
}

// backupRun records a successful backup of an item that finished at at. No
// store writer takes a time, so the stamp is set after the writers ran.
func (f *placementFixture) backupRun(itemID string, at int64) {
	f.t.Helper()
	id, err := f.st.StartRun(itemID, "backup")
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.st.FinishRun(id, "success", "", 0, ""); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE runs SET started_at = ?, finished_at = ? WHERE id = ?`, at, at, id); err != nil {
		f.t.Fatal(err)
	}
}

// replicated gives the domain a successful replication long ago, the state of
// every database that has replicated before.
func (f *placementFixture) replicated(domain string) {
	f.t.Helper()
	id, err := f.st.RecordOffsiteRun(domain, 1)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.st.FinishOffsiteRun(id, true, ""); err != nil {
		f.t.Fatal(err)
	}
}

func (f *placementFixture) rcloneConf(conf string) {
	f.t.Helper()
	if err := f.svc.SetRcloneConf(conf); err != nil {
		f.t.Fatal(err)
	}
}

// hold sets what a listing of the location returns.
func (f *placementFixture) hold(location string, snaps ...restic.Snapshot) {
	f.eng.mu.Lock()
	defer f.eng.mu.Unlock()
	f.eng.snaps[filepath.ToSlash(location)] = snaps
}

// do sends one request through the router and returns its JSON body. Anything
// but 200 fails the test.
func (f *placementFixture) do(method, path string, body any) map[string]any {
	f.t.Helper()
	code, out := f.call(method, path, body)
	if code != http.StatusOK {
		f.t.Fatalf("%s %s = %d: %v", method, path, code, out)
	}
	return out
}

// call sends one request through the router, signed in once signIn ran, and
// returns its status and JSON body.
func (f *placementFixture) call(method, path string, body any) (int, map[string]any) {
	f.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	r := jsonReq(method, path, rd)
	if f.session != "" {
		r.AddCookie(&http.Cookie{Name: f.h.sessionCookieNameFor(), Value: f.session}) //nolint:gosec // G124: a cookie on a test request
	}
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
	}
	return rec.Code, out
}

// signIn gives the server a login password and the requests after it a
// session of a person signed in with it.
func (f *placementFixture) signIn() {
	f.t.Helper()
	f.withPassword("a password of the owner")
	settings, err := f.st.GetSettings()
	if err != nil {
		f.t.Fatal(err)
	}
	f.session = secret.NewSessionToken(f.svc.cfg.AppKey, settings.AuthPasswordHash, settings.SessionEpoch, time.Hour)
}

func doJSON(t *testing.T, h *Handler, method, path string, body any) map[string]any {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	rec := httptest.NewRecorder()
	h.Router().ServeHTTP(rec, jsonReq(method, path, rd))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s = %d: %s", method, path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return out
}

func snap(id string, at int64, tags ...string) restic.Snapshot {
	return restic.Snapshot{ID: id, Time: time.Unix(at, 0).UTC().Format(time.RFC3339), Tags: tags}
}

func copied(id, original string, at int64, tags ...string) restic.Snapshot {
	s := snap(id, at, tags...)
	s.Original = original
	return s
}

func copiesRow(identity string, count int, latest int64) store.ItemCopies {
	return store.ItemCopies{Identity: identity, SnapshotCount: count, LatestSnapshotAt: latest}
}

// copyID is the id a copy gets at its destination: new, and the same for the
// same snapshot and destination, so a test can name it.
func copyID(dest, id string) string {
	sum := sha256.Sum256([]byte(dest + "\x00" + id))
	return hex.EncodeToString(sum[:8])
}

// placementEngine answers restic per location and records what a run did.
type placementEngine struct {
	ResticEngine
	mu      sync.Mutex
	snaps   map[string][]restic.Snapshot // by slash-spelled location
	listErr map[string]error
	copyErr map[string]error // Copy fails for this source and copies nothing
	opens   map[string]bool  // RepoOpens; a missing entry is true
	openErr map[string]error // RepoOpensErr's message for a location in opens=false; default is a generic one
	lists   map[string]int   // Snapshots calls per location
	initErr map[string]error // Init fails here and leaves the location as it was
	// initStaysShut lists locations where Init succeeds and the repository
	// still does not open afterwards.
	initStaysShut map[string]bool
	// forgetErr fails a Forget by ids at this location and forgets nothing.
	forgetErr   map[string]error
	checks      []string         // the repositories Check ran on
	checkErr    map[string]error // Check fails here
	copies      []copyCall
	forgets     []forgetCall
	deletes     []forgetIDsCall
	prunes      []string
	tags        []tagCall
	diffs       []string                       // "from->to" of each DiffStream call
	diffChanges map[string][]restic.DiffChange // what a DiffStream call reports, by "from->to"
	files       map[string][]restic.FileEntry  // what a snapshot lists, by its id
	ensured     []string                       // Init calls: the repositories EnsureRepo had to create
	// onSnapshots, when set, runs synchronously after every Snapshots call, so a
	// test can rewrite state a caller already captured before it listed.
	onSnapshots func()
	// onCopy and onForget, when set, run before a Copy or a Forget by ids does
	// anything, so a test can hold a run in the middle.
	onCopy, onForget func()
	// onCheck, when set, runs as a Check starts.
	onCheck func()
	// onUnlock, when set, runs as a stale-lock clear starts.
	onUnlock func()
	// copyDrops are the source snapshot ids a Copy leaves out without a word.
	copyDrops map[string]bool
	// signIns lists the environment each Init, Snapshots, Check and Copy ran
	// with, as "op location".
	signIns map[string][][]string
}

func (e *placementEngine) signedIn(op, repo string, mode restic.Mode) {
	if e.signIns == nil {
		e.signIns = map[string][][]string{}
	}
	key := op + " " + filepath.ToSlash(repo)
	e.signIns[key] = append(e.signIns[key], slices.Clone(mode.Env))
}

type copyCall struct {
	Dest, Src string
	IDs       []string // nil for a whole-repository copy
}

type forgetCall struct {
	Repo   string
	Tags   []string // nil for the repository-wide pass
	Policy restic.RetentionPolicy
	Prune  bool
}

type forgetIDsCall struct {
	Repo string
	IDs  []string
}

type tagCall struct {
	Repo        string
	IDs         []string
	Add, Remove []string
}

func (e *placementEngine) Init(_ context.Context, repo string, mode restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := filepath.ToSlash(repo)
	e.ensured = append(e.ensured, key)
	e.signedIn("init", repo, mode)
	if err := e.initErr[key]; err != nil {
		return err
	}
	if _, known := e.opens[key]; known && !e.initStaysShut[key] {
		e.opens[key] = true
		delete(e.listErr, key)
	}
	return nil
}

func (e *placementEngine) RepoOpens(_ context.Context, repo string, _ restic.Mode) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	open, known := e.opens[filepath.ToSlash(repo)]
	return open || !known
}

func (e *placementEngine) RepoOpensErr(ctx context.Context, repo string, mode restic.Mode) error {
	if e.RepoOpens(ctx, repo, mode) {
		return nil
	}
	if err := e.openErr[filepath.ToSlash(repo)]; err != nil {
		return err
	}
	return errors.New("repository does not exist: unable to open config file")
}

func (e *placementEngine) Snapshots(_ context.Context, repo string, mode restic.Mode) ([]restic.Snapshot, error) {
	e.mu.Lock()
	key := filepath.ToSlash(repo)
	e.lists[key]++
	e.signedIn("list", repo, mode)
	err := e.listErr[key]
	snaps := slices.Clone(e.snaps[key])
	hook := e.onSnapshots
	e.mu.Unlock()
	if hook != nil {
		hook()
	}
	if err != nil {
		return nil, err
	}
	return snaps, nil
}

// Copy puts a copy with a new id and the source's identity at the destination
// for every snapshot asked for that the destination does not hold yet.
func (e *placementEngine) Copy(_ context.Context, dest, src string, ids []string, _ restic.Limits, mode restic.Mode) error {
	if e.onCopy != nil {
		e.onCopy()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	d, s := filepath.ToSlash(dest), filepath.ToSlash(src)
	e.copies = append(e.copies, copyCall{Dest: d, Src: s, IDs: slices.Clone(ids)})
	e.signedIn("copy", dest, mode)
	if err := e.copyErr[s]; err != nil {
		return err
	}
	held := map[string]bool{}
	for _, sn := range e.snaps[d] {
		held[sn.ID], held[sn.Original] = true, true
	}
	for _, sn := range e.snaps[s] {
		key := restic.Identity(sn)
		if (ids != nil && !slices.Contains(ids, sn.ID)) || held[key] || e.copyDrops[sn.ID] {
			continue
		}
		held[key] = true
		c := sn
		c.ID, c.Original = copyID(d, sn.ID), key
		e.snaps[d] = append(e.snaps[d], c)
	}
	return nil
}

// ForgetPolicy keeps the newest KeepLast snapshots carrying one of the tags, as
// one group, and records the rest of the policy; without a tag it only records.
func (e *placementEngine) ForgetPolicy(_ context.Context, repo string, p restic.RetentionPolicy, _ restic.Mode, tags []string, prune bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := filepath.ToSlash(repo)
	tags = slices.Clone(tags)
	e.forgets = append(e.forgets, forgetCall{Repo: key, Tags: tags, Policy: p, Prune: prune})
	if len(tags) == 0 || p.KeepLast == 0 {
		return nil
	}
	var mine []restic.Snapshot
	for _, sn := range e.snaps[key] {
		if slices.ContainsFunc(tags, func(t string) bool { return slices.Contains(sn.Tags, t) }) {
			mine = append(mine, sn)
		}
	}
	slices.SortStableFunc(mine, func(a, b restic.Snapshot) int { return strings.Compare(b.Time, a.Time) })
	drop := map[string]bool{}
	for _, sn := range mine[min(p.KeepLast, len(mine)):] {
		drop[sn.ID] = true
	}
	e.snaps[key] = slices.DeleteFunc(e.snaps[key], func(sn restic.Snapshot) bool { return drop[sn.ID] })
	return nil
}

func (e *placementEngine) Forget(_ context.Context, repo string, ids []string, _ bool, _ restic.Mode) error {
	if e.onForget != nil {
		e.onForget()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	key := filepath.ToSlash(repo)
	e.deletes = append(e.deletes, forgetIDsCall{Repo: key, IDs: slices.Clone(ids)})
	if err := e.forgetErr[key]; err != nil {
		return err
	}
	e.snaps[key] = slices.DeleteFunc(e.snaps[key], func(sn restic.Snapshot) bool {
		return slices.ContainsFunc(ids, func(id string) bool { return strings.HasPrefix(sn.ID, id) })
	})
	return nil
}

func (e *placementEngine) Check(_ context.Context, repo string, mode restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := filepath.ToSlash(repo)
	e.checks = append(e.checks, key)
	e.signedIn("check", repo, mode)
	if e.onCheck != nil {
		e.onCheck()
	}
	return e.checkErr[key]
}

func (e *placementEngine) Prune(_ context.Context, repo string, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prunes = append(e.prunes, filepath.ToSlash(repo))
	return nil
}

// ForgetPreview keeps every snapshot under the tag.
func (e *placementEngine) ForgetPreview(_ context.Context, repo string, _ restic.RetentionPolicy, _ restic.Mode, tag string) ([]restic.ForgetGroup, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return []restic.ForgetGroup{{Tags: []string{tag}, Keep: snapshotsTagged(e.snaps[filepath.ToSlash(repo)], tag)}}, nil
}

func (e *placementEngine) Unlock(context.Context, string, bool, restic.Mode) error {
	if e.onUnlock != nil {
		e.onUnlock()
	}
	return nil
}

// DiffStream reports the changes set for the pair and counts each as a changed
// file of ten new bytes.
func (e *placementEngine) DiffStream(_ context.Context, _, from, to string, _ restic.Mode, onChange func(restic.DiffChange)) (restic.DiffResult, error) {
	e.mu.Lock()
	pair := from + "->" + to
	e.diffs = append(e.diffs, pair)
	changes := e.diffChanges[pair]
	e.mu.Unlock()
	for _, c := range changes {
		onChange(c)
	}
	return restic.DiffResult{ChangedFiles: len(changes), AddedBytes: int64(10 * len(changes))}, nil
}

func (e *placementEngine) LsStreamNoLock(_ context.Context, _, id string, _ restic.Mode, onEntry func(restic.FileEntry)) error {
	e.mu.Lock()
	entries := e.files[id]
	e.mu.Unlock()
	for _, entry := range entries {
		onEntry(entry)
	}
	return nil
}

// Tag changes the tags of the named snapshots where they lie. restic would
// also give each a new id, which no caller of the fake reads.
func (e *placementEngine) Tag(_ context.Context, repo string, ids, add, remove []string, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := filepath.ToSlash(repo)
	e.tags = append(e.tags, tagCall{Repo: key, IDs: slices.Clone(ids), Add: add, Remove: remove})
	for i, sn := range e.snaps[key] {
		if !slices.Contains(ids, sn.ID) {
			continue
		}
		tags := slices.DeleteFunc(slices.Clone(sn.Tags), func(t string) bool { return slices.Contains(remove, t) })
		e.snaps[key][i].Tags = append(tags, add...)
	}
	return nil
}

func (e *placementEngine) Stats(context.Context, string, string, restic.Mode) (restic.StatsResult, error) {
	return restic.StatsResult{}, nil
}

// placementDocker reports the named containers as installed with nothing
// mounted, or listErr.
type placementDocker struct {
	dockercli.Docker
	installed map[string]bool
	listErr   error
}

func (d *placementDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	if d.listErr != nil {
		return nil, d.listErr
	}
	out := []dockercli.ContainerInfo{}
	for _, name := range slices.Sorted(maps.Keys(d.installed)) {
		if d.installed[name] {
			out = append(out, dockercli.ContainerInfo{ID: name, Name: name, State: "running"})
		}
	}
	return out, nil
}

// Self reports no own container: the placement fixture never runs as BombVault
// itself, and the list routes must not panic on the embedded nil Docker.
func (d *placementDocker) Self(context.Context) (string, error) { return "", nil }

// placementVirsh reports the named VMs as defined, or listErr.
type placementVirsh struct {
	virshcli.Virsh
	defined []string
	listErr error
}

func (v *placementVirsh) List(context.Context) ([]virshcli.VMInfo, error) {
	if v.listErr != nil {
		return nil, v.listErr
	}
	out := make([]virshcli.VMInfo, 0, len(v.defined))
	for _, name := range v.defined {
		out = append(out, virshcli.VMInfo{Name: name})
	}
	return out, nil
}

// openContainer adds a container row whose location is still open.
func (f *placementFixture) openContainer(name string) {
	f.t.Helper()
	if _, err := f.st.UpsertTarget(store.Target{ContainerName: name}); err != nil {
		f.t.Fatalf("open container %s: %v", name, err)
	}
}

func (f *placementFixture) openVM(name string) {
	f.t.Helper()
	if _, err := f.st.UpsertVMTarget(store.VMTarget{Name: name}); err != nil {
		f.t.Fatalf("open vm %s: %v", name, err)
	}
}

func (f *placementFixture) openFileSet(name string) store.FileSet {
	f.t.Helper()
	set, err := f.st.CreateFileSet(store.FileSet{Name: name, Path: "sets/" + name, Enabled: true})
	if err != nil {
		f.t.Fatalf("open file set %s: %v", name, err)
	}
	return set
}

// paused pauses the domain the way a rebuild-detection check would, without a
// listing having to find the history itself.
func (f *placementFixture) paused(domain string) {
	f.t.Helper()
	if _, err := f.st.PausePlacement(domain); err != nil {
		f.t.Fatalf("paused %s: %v", domain, err)
	}
}

func (f *placementFixture) home(item store.ItemRef) store.HomeState {
	f.t.Helper()
	h, err := f.st.ItemHome(item)
	if err != nil {
		f.t.Fatalf("home of %+v: %v", item, err)
	}
	return h
}
