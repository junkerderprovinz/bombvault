package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

func TestZFSBackupStoresEachDatasetsProperties(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	host.props = map[string]zfs.Properties{
		zfsRoot:  {"compression": "zstd", "casesensitivity": "insensitive"},
		zfsChild: {"recordsize": "1048576"},
	}
	d := zfsSeedItem(t, st, zfsRoot)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("backup: %v", err)
	}
	got, err := st.ZFSPropertiesOfItem(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	byDataset := map[string]map[string]string{}
	runs, _ := st.ListZFSRuns(d.ID, 1)
	members, _ := st.ListZFSRunMembers(runs[0].RunID)
	for _, m := range members {
		byDataset[m.Dataset] = got[m.ResticSnapshot]
	}
	if !reflect.DeepEqual(byDataset[zfsRoot], map[string]string(host.props[zfsRoot])) ||
		!reflect.DeepEqual(byDataset[zfsChild], map[string]string(host.props[zfsChild])) {
		t.Fatalf("stored %v, want the properties the host reported per dataset", byDataset)
	}
}

func TestZFSBackupRunsWhenThePropertiesCannotBeRead(t *testing.T) {
	s, st, host, _ := zfsRunFixture(t, zfsTwoDatasetTree())
	host.propsErr = errors.New("zfs get: permission denied")
	d := zfsSeedItem(t, st, zfsRoot)

	if _, err := s.BackupZFSDataset(context.Background(), d.ID); err != nil {
		t.Fatalf("a failed property read must not fail the backup: %v", err)
	}
	if got, _ := st.ZFSPropertiesOfItem(d.ID); len(got) != 0 {
		t.Fatalf("stored %v without having read anything", got)
	}
}

// zfsSeedProperties records a run instant whose members carry properties, the
// way a backup would have left it.
func zfsSeedProperties(t *testing.T, st *store.Repo, itemID string, props map[string]string) {
	t.Helper()
	runID, err := st.StartRun(itemID, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordZFSRun(runID, itemID, zfsStamp, -1, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.AddZFSRunMember(store.ZFSRunMember{RunID: runID, Dataset: zfsRoot, Outcome: "backed-up", ResticSnapshot: zfsRootSnapID}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetZFSRunMemberProperties(runID, zfsRoot, props); err != nil {
		t.Fatal(err)
	}
}

func TestZFSRestorePointCarriesTheStoredProperties(t *testing.T) {
	s, st, _, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	zfsSeedProperties(t, st, d.ID, map[string]string{"compression": "zstd"})

	points, err := s.ListZFSRestorePoints(context.Background(), d.ID, "local")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := zfsPointMember(points[0], zfsRoot)
	if !ok || m.Properties["compression"] != "zstd" {
		t.Fatalf("member = %+v, want the stored compression", m)
	}
}

func TestAnOffsiteCopyCarriesThePropertiesOfItsOriginal(t *testing.T) {
	s, st, _, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	zfsSeedProperties(t, st, d.ID, map[string]string{"compression": "zstd"})
	offsite := filepath.Join(s.cfg.HostMountRoot, "offsite")
	if err := os.MkdirAll(offsite, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(offsite, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, _ := st.GetSettings()
	settings.ZFSOffsite = filepath.ToSlash(offsite)
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	// restic copy gives each snapshot a new id and names the one it came from.
	copies := zfsRestorePointSnapshots(s.cfg.HostMountRoot)
	for i := range copies {
		copies[i].Original = copies[i].ID
		copies[i].ID = strings.Repeat("c", 63) + string(rune('0'+i))
	}
	eng.snaps = copies

	points, err := s.ListZFSRestorePoints(context.Background(), d.ID, "offsite")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := zfsPointMember(points[0], zfsRoot)
	if !ok || m.Properties["compression"] != "zstd" {
		t.Fatalf("off-site member = %+v, want the compression stored for its original", m)
	}
}

func TestZFSRestoreIntoANewDatasetCreatesItWithTheStoredProperties(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	root := s.cfg.HostMountRoot
	host.strictTree = true
	d := zfsSeedItem(t, st, zfsRoot)
	zfsSeedProperties(t, st, d.ID, map[string]string{
		"compression": "zstd", "casesensitivity": "insensitive", "mountpoint": "/mnt/cache/appdata",
	})
	const fresh = "cache/copy"
	records := zfsMountRecords
	host.onCreate = func(name string) {
		host.mu.Lock()
		host.tree = append(host.tree, zfsEntry(name, "/mnt/"+name))
		host.mu.Unlock()
		zfsMountRecords = func() []zfs.MountRecord {
			return append(records(), zfs.MountRecord{
				MountPoint: zfsMemberPath(root, name), Root: "/", FSType: "zfs", Source: name,
				Options: []string{"rw"}, Optional: []string{"master:9"},
			})
		}
	}
	t.Cleanup(func() { zfsMountRecords = records })

	req := zfsRestoreRequest(zfsRoot)
	req.NewDataset = fresh
	req.SafetySnapshot = false
	ack, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req)
	if err != nil || !started {
		t.Fatalf("start: %v %v", started, err)
	}
	if ack.Created != fresh || ack.Target != zfsMemberPath(root, fresh) {
		t.Fatalf("ack = %+v", ack)
	}
	if run := zfsAwaitRestore(t, st, d.ID); run.Status != "success" {
		t.Fatalf("restore run = %+v", run)
	}
	if !hostDid(host, "create -o casesensitivity=insensitive -o compression=zstd "+fresh) {
		t.Fatalf("host calls = %v, want a create with the stored properties and no mountpoint", host.recorded())
	}
	if hostDid(host, "snapshot "+fresh) {
		t.Fatal("a new dataset needs no safety snapshot")
	}
	want := "RestoreAll|" + zfsRootSnapID + "->" + zfsMemberPath(root, fresh)
	if len(eng.restores) != 1 || eng.restores[0] != want {
		t.Fatalf("restores = %v, want %q", eng.restores, want)
	}
}

// zfsNewDatasetHost makes a dataset the restore creates appear on the host
// and in the container's mount table, the way propagation brings it in.
func zfsNewDatasetHost(t *testing.T, s *Service, host *fakeZFSHost) {
	t.Helper()
	root := s.cfg.HostMountRoot
	host.strictTree = true
	records := zfsMountRecords
	host.onCreate = func(name string) {
		host.mu.Lock()
		host.tree = append(host.tree, zfsEntry(name, "/mnt/"+name))
		host.mu.Unlock()
		zfsMountRecords = func() []zfs.MountRecord {
			return append(records(), zfs.MountRecord{
				MountPoint: zfsMemberPath(root, name), Root: "/", FSType: "zfs", Source: name,
				Options: []string{"rw"}, Optional: []string{"master:9"},
			})
		}
	}
	t.Cleanup(func() { zfsMountRecords = records })
}

func TestZFSRestoreIntoANewDatasetSetsQuotasAfterTheFiles(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	zfsNewDatasetHost(t, s, host)
	d := zfsSeedItem(t, st, zfsRoot)
	zfsSeedProperties(t, st, d.ID, map[string]string{"compression": "zstd", "quota": "1024"})
	restoredAt := map[string]int{}
	host.onSet = func() {
		calls := host.calls
		restoredAt[calls[len(calls)-1]] = len(eng.restores)
	}
	const fresh = "cache/copy"
	req := zfsRestoreRequest(zfsRoot)
	req.NewDataset = fresh
	req.SafetySnapshot = false
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); err != nil || !started {
		t.Fatalf("start: %v %v", started, err)
	}
	if run := zfsAwaitRestore(t, st, d.ID); run.Status != "success" {
		t.Fatalf("restore run = %+v", run)
	}
	if !hostDid(host, "create -o compression=zstd "+fresh) {
		t.Fatalf("host calls = %v, want a create without the quota", host.recorded())
	}
	if at, ok := restoredAt["set quota=1024 "+fresh]; !ok || at != 1 {
		t.Fatalf("restores done when each set ran = %v, want the quota on %s after the files", restoredAt, fresh)
	}
	if hostDid(host, "set quota=1024 "+zfsRoot) {
		t.Fatal("the quota went onto the dataset the backup came from")
	}
}

func TestAQuotaThatCannotBeSetAfterTheFilesSaysTheFilesAreBack(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	zfsSeedProperties(t, st, d.ID, map[string]string{"compression": "zstd", "quota": "1024"})
	host.setErr = func(p zfs.Properties) error {
		if _, ok := p["quota"]; ok {
			return errors.New("cannot set property for 'cache/appdata': size is less than current used or reserved space")
		}
		return nil
	}
	req := zfsRestoreRequest(zfsRoot)
	req.ApplyProperties = true
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); err != nil || !started {
		t.Fatalf("start: %v %v", started, err)
	}
	run := zfsAwaitRestore(t, st, d.ID)
	if len(eng.restores) != 1 {
		t.Fatalf("restores = %v, want the files written", eng.restores)
	}
	if run.Status != "failed" || !strings.HasPrefix(run.Error, "set-limits-failed: ") {
		t.Fatalf("restore run = %+v, want the limits failure after the files", run)
	}
}

func TestZFSRestoreRefusesANewDatasetThatExists(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	host.strictTree = true
	d := zfsSeedItem(t, st, zfsRoot)
	req := zfsRestoreRequest(zfsRoot)
	req.NewDataset = zfsChild

	_, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req)
	if started || err == nil || !strings.Contains(err.Error(), "dataset-exists") {
		t.Fatalf("started=%v err=%v, want dataset-exists", started, err)
	}
	for _, c := range host.recorded() {
		if strings.HasPrefix(c, "create") {
			t.Fatalf("created %q although the name was taken", c)
		}
	}
}

func TestZFSRestoreSetsPropertiesOnlyWhenAsked(t *testing.T) {
	for _, apply := range []bool{false, true} {
		s, st, host, _ := zfsRestoreFixture(t)
		d := zfsSeedItem(t, st, zfsRoot)
		zfsSeedProperties(t, st, d.ID, map[string]string{"compression": "zstd", "utf8only": "on"})
		req := zfsRestoreRequest(zfsRoot)
		req.ApplyProperties = apply

		if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); err != nil || !started {
			t.Fatalf("start: %v %v", started, err)
		}
		zfsAwaitRestore(t, st, d.ID)
		set := hostDid(host, "set compression=zstd "+zfsRoot)
		if set != apply {
			t.Fatalf("apply=%v: host calls %v", apply, host.recorded())
		}
		if hostDid(host, "set utf8only") {
			t.Fatal("a creation-time property was set on an existing dataset")
		}
	}
}

func TestZFSRestoreRefusesToApplyPropertiesNobodyStored(t *testing.T) {
	s, st, _, _ := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	req := zfsRestoreRequest(zfsRoot)
	req.ApplyProperties = true
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); started || err == nil {
		t.Fatalf("started=%v err=%v, want a refusal", started, err)
	}
}

func TestZFSRestoreCheckIntoANewDatasetCreatesNothing(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	host.strictTree = true
	d := zfsSeedItem(t, st, zfsRoot)
	req := zfsRestoreRequest(zfsRoot)
	req.NewDataset = "cache/copy"
	req.SafetySnapshot = false

	plan, _, err := s.prepareRestoreZFS(planOnly(context.Background()), d.ID, "local", req)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range host.recorded() {
		if strings.HasPrefix(c, "create") {
			t.Fatalf("the check created the dataset: %q", c)
		}
	}
	if len(plan.steps) != 1 || plan.steps[0].target != zfsMemberPath(s.cfg.HostMountRoot, "cache/copy") {
		t.Fatalf("steps = %+v, want one restore into where the new dataset will be", plan.steps)
	}
}

func TestZFSRestoreCheckStillRefusesANewDatasetThatExists(t *testing.T) {
	s, st, host, _ := zfsRestoreFixture(t)
	host.strictTree = true
	d := zfsSeedItem(t, st, zfsRoot)
	req := zfsRestoreRequest(zfsRoot)
	req.NewDataset = zfsChild
	if _, _, err := s.prepareRestoreZFS(planOnly(context.Background()), d.ID, "local", req); err == nil || !strings.Contains(err.Error(), "dataset-exists") {
		t.Fatalf("err = %v, want dataset-exists", err)
	}
}

func TestZFSRestoreSetsQuotasAfterTheFilesAndTheRestBefore(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	d := zfsSeedItem(t, st, zfsRoot)
	zfsSeedProperties(t, st, d.ID, map[string]string{"compression": "zstd", "quota": "1024"})
	restoredAt := map[string]int{}
	host.onSet = func() {
		calls := host.calls
		restoredAt[calls[len(calls)-1]] = len(eng.restores)
	}
	req := zfsRestoreRequest(zfsRoot)
	req.ApplyProperties = true
	if _, started, err := s.StartRestoreZFS(context.Background(), d.ID, "local", req); err != nil || !started {
		t.Fatalf("start: %v %v", started, err)
	}
	if run := zfsAwaitRestore(t, st, d.ID); run.Status != "success" {
		t.Fatalf("restore run = %+v", run)
	}
	before, okB := restoredAt["set compression=zstd "+zfsRoot]
	after, okA := restoredAt["set quota=1024 "+zfsRoot]
	if !okB || !okA || before != 0 || after != 1 {
		t.Fatalf("restores done when each set ran = %v, want compression before the files and the quota after", restoredAt)
	}
}

// emptyTargetEngine answers a dry run for a target that must be empty, and
// records what that target held when restic would have compared against it.
type emptyTargetEngine struct {
	*zfsFakeEngine
	targets []string
	held    []int
}

func (e *emptyTargetEngine) RepoOpensErr(context.Context, string, restic.Mode) error { return nil }

func (e *emptyTargetEngine) RestorePreview(_ context.Context, _ string, st restic.PreviewStep, _ restic.Mode, onItem func(restic.PreviewItem)) error {
	e.targets = append(e.targets, st.Target)
	entries, _ := os.ReadDir(st.Target)
	e.held = append(e.held, len(entries))
	onItem(restic.PreviewItem{Action: "restored", Item: "/photos/a.jpg", Size: 5})
	return nil
}

// A dataset that does not exist yet is empty, so its plan lists every file as
// new, even when BombVault cannot see where its parent is mounted.
func TestZFSRestoreCheckIntoANewDatasetComparesAgainstNothing(t *testing.T) {
	for _, source := range []string{"/mnt", "/srv/data"} {
		s, st, host, eng := zfsRestoreFixture(t)
		host.strictTree = true
		s.cfg.HostSourceRoot = source
		check := &emptyTargetEngine{zfsFakeEngine: eng}
		s.engine = check
		s.diskStat = func(string) (diskStatResult, error) { return diskStatResult{Free: 1 << 30, Volume: "pool:cache"}, nil }
		if err := os.MkdirAll(filepath.Join(s.cfg.HostMountRoot, "unrelated"), 0o750); err != nil {
			t.Fatal(err)
		}
		d := zfsSeedItem(t, st, zfsRoot)
		req := zfsRestoreRequest(zfsRoot)
		req.NewDataset = "cache/copy"
		req.SafetySnapshot = false

		res, err := s.CheckRestore(context.Background(), RestoreCheckRequest{Kind: checkZFS, Name: d.ID, Source: "local", ZFS: req})
		if err != nil {
			t.Fatal(err)
		}
		if res.Plan == nil || res.Plan.Added != 1 || res.Plan.Extra != 0 {
			t.Fatalf("%s: plan = %+v", source, res.Plan)
		}
		for i, target := range check.targets {
			if target == s.cfg.HostMountRoot || check.held[i] != 0 {
				t.Fatalf("%s: the dry run compared against %s, which held %d entries", source, target, check.held[i])
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("%s: the empty folder %s outlived the check", source, target)
			}
		}
		if f := res.Plan.Files; len(f) != 1 || !strings.HasSuffix(f[0].Path, "cache/copy/photos/a.jpg") {
			t.Fatalf("%s: files = %+v", source, f)
		}
		// Where the dataset will be mounted decides the room; unknown, it stays grey.
		if want := map[bool]string{true: lineOK, false: lineSkip}[source == "/mnt"]; lineOf(t, res, lineSpace).Status != want {
			t.Fatalf("%s: space = %+v, want %s", source, lineOf(t, res, lineSpace), want)
		}
	}
}

func TestZFSRestoreCheckBlocksANewDatasetWhoseParentIsMissing(t *testing.T) {
	s, st, host, eng := zfsRestoreFixture(t)
	s.engine = &emptyTargetEngine{zfsFakeEngine: eng}
	host.strictTree = true
	d := zfsSeedItem(t, st, zfsRoot)
	req := zfsRestoreRequest(zfsRoot)
	req.NewDataset = "cache/nosuch/copy"
	req.SafetySnapshot = false

	res, err := s.CheckRestore(context.Background(), RestoreCheckRequest{Kind: checkZFS, Name: d.ID, Source: "local", ZFS: req})
	if err != nil {
		t.Fatal(err)
	}
	if res.Ready {
		t.Fatalf("a restore under a missing dataset was ready: %+v", res.Checks)
	}
	var space CheckLine
	for _, c := range res.Checks {
		if c.ID == lineSpace {
			space = c
		}
	}
	if space.Status != lineFail || space.Reason != reasonParentMissing || space.Detail != "cache/nosuch" {
		t.Fatalf("space line = %+v, want a failure naming cache/nosuch", space)
	}
	for _, c := range host.recorded() {
		if strings.HasPrefix(c, "create") {
			t.Fatalf("the check created something: %q", c)
		}
	}
}
