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
	"github.com/junkerderprovinz/bombvault/internal/spike"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// placementFixture is a Service and a Handler over an in-memory store, with a
// restic fake that answers per location and domain paths that exist.
type placementFixture struct {
	t    *testing.T
	svc  *Service
	h    *Handler
	st   *store.Repo
	db   *sql.DB // for facts no store writer can set, such as when a run happened
	eng  *placementEngine
	dock *placementDocker
	root string // host mount root, slash-spelled
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
		snaps:   map[string][]restic.Snapshot{},
		listErr: map[string]error{},
		opens:   map[string]bool{},
		lists:   map[string]int{},
	}
	dock := &placementDocker{installed: map[string]bool{}}
	svc := NewService(cfg, st, dock, nil, eng)
	sched := schedule.New(func(string) error { return nil }, st.ListTargets)
	f := &placementFixture{
		t: t, svc: svc, h: NewHandler(cfg, st, dock, svc, sched, spike.DefaultProbes()),
		st: st, db: db, eng: eng, dock: dock, root: root,
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
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	rec := httptest.NewRecorder()
	f.h.Router().ServeHTTP(rec, jsonReq(method, path, rd))
	if rec.Code != http.StatusOK {
		f.t.Fatalf("%s %s = %d: %s", method, path, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		f.t.Fatalf("%s %s: %v", method, path, err)
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
	opens   map[string]bool // RepoOpens; a missing entry is true
	lists   map[string]int  // Snapshots calls per location
	copies  []copyCall
	forgets []forgetCall
	deletes []forgetIDsCall
	prunes  []string
	ensured []string // Init calls: the repositories EnsureRepo had to create
	// onSnapshots, when set, runs synchronously after every Snapshots call, so a
	// test can rewrite state a caller already captured before it listed.
	onSnapshots func()
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

func (e *placementEngine) Init(_ context.Context, repo string, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ensured = append(e.ensured, filepath.ToSlash(repo))
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
	return errors.New("repository does not exist: unable to open config file")
}

func (e *placementEngine) Snapshots(_ context.Context, repo string, _ restic.Mode) ([]restic.Snapshot, error) {
	e.mu.Lock()
	key := filepath.ToSlash(repo)
	e.lists[key]++
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
func (e *placementEngine) Copy(_ context.Context, dest, src string, ids []string, _ restic.Limits, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	d, s := filepath.ToSlash(dest), filepath.ToSlash(src)
	e.copies = append(e.copies, copyCall{Dest: d, Src: s, IDs: slices.Clone(ids)})
	held := map[string]bool{}
	for _, sn := range e.snaps[d] {
		held[sn.ID], held[sn.Original] = true, true
	}
	for _, sn := range e.snaps[s] {
		key := restic.Identity(sn)
		if (ids != nil && !slices.Contains(ids, sn.ID)) || held[key] {
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
func (e *placementEngine) ForgetPolicy(_ context.Context, repo string, p restic.RetentionPolicy, _ restic.Mode, tag string, prune bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := filepath.ToSlash(repo)
	var tags []string
	if tag != "" {
		tags = []string{tag}
	}
	e.forgets = append(e.forgets, forgetCall{Repo: key, Tags: tags, Policy: p, Prune: prune})
	if tags == nil || p.KeepLast == 0 {
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
	e.mu.Lock()
	defer e.mu.Unlock()
	key := filepath.ToSlash(repo)
	e.deletes = append(e.deletes, forgetIDsCall{Repo: key, IDs: slices.Clone(ids)})
	e.snaps[key] = slices.DeleteFunc(e.snaps[key], func(sn restic.Snapshot) bool {
		return slices.ContainsFunc(ids, func(id string) bool { return strings.HasPrefix(sn.ID, id) })
	})
	return nil
}

func (e *placementEngine) Prune(_ context.Context, repo string, _ restic.Mode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prunes = append(e.prunes, filepath.ToSlash(repo))
	return nil
}

func (e *placementEngine) Unlock(context.Context, string, bool, restic.Mode) error { return nil }

func (e *placementEngine) Stats(context.Context, string, string, restic.Mode) (restic.StatsResult, error) {
	return restic.StatsResult{}, nil
}

// placementDocker reports the named containers as installed with nothing mounted.
type placementDocker struct {
	dockercli.Docker
	installed map[string]bool
}

func (d *placementDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	out := []dockercli.ContainerInfo{}
	for _, name := range slices.Sorted(maps.Keys(d.installed)) {
		if d.installed[name] {
			out = append(out, dockercli.ContainerInfo{ID: name, Name: name, State: "running"})
		}
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
