package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// rigDocker is a Docker host that keeps its containers by name and records
// everything asked of it, so a test can tell what a restore did to which one.
type rigDocker struct {
	dockercli.Docker

	mu         sync.Mutex
	containers map[string]model.Inspect
	images     map[string]bool
	calls      []string
}

func (d *rigDocker) note(call string) {
	d.calls = append(d.calls, call)
}

// did reports the recorded calls that begin with one of the verbs.
func (d *rigDocker) did(verbs ...string) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []string
	for _, c := range d.calls {
		if verb, _, _ := strings.Cut(c, ":"); slices.Contains(verbs, verb) {
			out = append(out, c)
		}
	}
	return out
}

func (d *rigDocker) forget() {
	d.mu.Lock()
	d.calls = nil
	d.mu.Unlock()
}

func (d *rigDocker) container(name string) (model.Inspect, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	in, ok := d.containers[name]
	return in, ok
}

func (d *rigDocker) List(context.Context) ([]dockercli.ContainerInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []dockercli.ContainerInfo
	for name, in := range d.containers {
		info := dockercli.ContainerInfo{ID: in.ID, Name: name, Labels: in.Config.Labels}
		for _, b := range in.HostConfig.Binds {
			if host, rest, ok := strings.Cut(b, ":"); ok {
				dest, _, _ := strings.Cut(rest, ":")
				info.Mounts = append(info.Mounts, dockercli.MountPoint{Source: host, Destination: dest})
			}
		}
		out = append(out, info)
	}
	slices.SortFunc(out, func(a, b dockercli.ContainerInfo) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (d *rigDocker) Inspect(_ context.Context, name string) (model.Inspect, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	in, ok := d.containers[name]
	if !ok {
		return model.Inspect{}, fmt.Errorf("Error response from daemon: No such container: %s", name)
	}
	return in, nil
}

func (d *rigDocker) InspectName(_ context.Context, name string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.containers[name]; ok {
		return name, nil
	}
	return "", nil
}

func (d *rigDocker) Self(context.Context) (string, error) { return "", nil }

// Networks lists br0, the one network the rig's containers use, so a restore
// check finds the network of a container that is gone.
func (d *rigDocker) Networks(context.Context) ([]string, error) { return []string{"br0"}, nil }

func (d *rigDocker) Stop(_ context.Context, name string, _ time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.note("stop:" + name)
	if in, ok := d.containers[name]; ok {
		in.Running = false
		d.containers[name] = in
	}
	return nil
}

func (d *rigDocker) Start(_ context.Context, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.note("start:" + name)
	if in, ok := d.containers[name]; ok {
		in.Running = true
		d.containers[name] = in
	}
	return nil
}

func (d *rigDocker) WaitRunning(context.Context, string, time.Duration) error { return nil }

func (d *rigDocker) Health(_ context.Context, name string) (model.Health, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return model.Health{Running: d.containers[name].Running}, nil
}

func (d *rigDocker) Remove(_ context.Context, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.note("remove:" + name)
	delete(d.containers, name)
	return nil
}

func (d *rigDocker) Pull(_ context.Context, image string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.note("pull:" + image)
	d.images[image] = true
	return nil
}

func (d *rigDocker) ImageID(_ context.Context, ref string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.images[ref] {
		return "sha256:" + ref, nil
	}
	return "", nil
}

func (d *rigDocker) CreateAndStart(_ context.Context, in model.Inspect, start bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	name := strings.TrimPrefix(in.Name, "/")
	d.note(fmt.Sprintf("create:%s:start=%v", name, start))
	if _, taken := d.containers[name]; taken {
		return fmt.Errorf("Conflict. The container name %q is already in use", "/"+name)
	}
	in.Running = start
	d.containers[name] = in
	return nil
}

func (d *rigDocker) Allocations(context.Context) ([]model.Allocation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []model.Allocation
	for name, in := range d.containers {
		a := model.Allocation{Name: name, IPv4: in.Network.IPv4Address}
		for port, binds := range in.HostConfig.PortBindings {
			_, proto, _ := strings.Cut(port, "/")
			for _, b := range binds {
				if b.HostPort != "" {
					a.HostPorts = append(a.HostPorts, b.HostPort+"/"+proto)
				}
			}
		}
		out = append(out, a)
	}
	return out, nil
}

func (d *rigDocker) Exec(_ context.Context, name string, _ []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.note("exec:" + name)
	return nil
}

// rigVirsh is a libvirt host with one domain. It records every call, so a
// test can tell that a restore asked it nothing.
type rigVirsh struct {
	mu    sync.Mutex
	name  string
	state string
	xml   string
	calls []string
}

var _ virshcli.Virsh = (*rigVirsh)(nil)

func (v *rigVirsh) note(call string) {
	v.mu.Lock()
	v.calls = append(v.calls, call)
	v.mu.Unlock()
}

func (v *rigVirsh) recorded() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.calls)
}

func (v *rigVirsh) forget() {
	v.mu.Lock()
	v.calls = nil
	v.mu.Unlock()
}

func (v *rigVirsh) setState(state string) {
	v.mu.Lock()
	v.state = state
	v.mu.Unlock()
}

func (v *rigVirsh) List(context.Context) ([]virshcli.VMInfo, error) {
	v.note("list")
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state == "" {
		return nil, nil
	}
	return []virshcli.VMInfo{{Name: v.name, State: v.state}}, nil
}

func (v *rigVirsh) State(context.Context, string) (string, error) {
	v.note("state")
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state, nil
}

func (v *rigVirsh) DumpXML(context.Context, string) (string, error) {
	v.note("dumpxml")
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.state == "" {
		return "", errors.New("error: failed to get domain")
	}
	return v.xml, nil
}

func (v *rigVirsh) DumpXMLInactive(ctx context.Context, name string) (string, error) {
	return v.DumpXML(ctx, name)
}

func (v *rigVirsh) Shutdown(context.Context, string) error {
	v.note("shutdown")
	v.setState("shut off")
	return nil
}

func (v *rigVirsh) Destroy(context.Context, string) error {
	v.note("destroy")
	v.setState("shut off")
	return nil
}

func (v *rigVirsh) Start(context.Context, string) error {
	v.note("start")
	v.setState("running")
	return nil
}

func (v *rigVirsh) Define(_ context.Context, xmlPath string) error {
	v.note("define")
	xml, err := os.ReadFile(xmlPath) //nolint:gosec // G304: the path the restore under test wrote
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.xml, v.state = string(xml), "shut off"
	v.mu.Unlock()
	return nil
}

func (v *rigVirsh) Undefine(context.Context, string) error {
	v.note("undefine")
	v.setState("")
	return nil
}

func (v *rigVirsh) Autostart(context.Context, string, bool) error {
	v.note("autostart")
	return nil
}

func (v *rigVirsh) IsActive(context.Context, string) (bool, error) {
	v.note("isactive")
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state == "running", nil
}

func (v *rigVirsh) SnapshotCreateDiskOnly(context.Context, string, string, bool, []string) error {
	v.note("snapshot")
	return nil
}

func (v *rigVirsh) BlockCommitActivePivot(context.Context, string, string) error {
	v.note("blockcommit")
	return nil
}

func (v *rigVirsh) GuestAgentPing(context.Context, string) bool { return false }

// restoreRig is a Service on the real restic binary with a temporary folder
// as its Host Data mount, which the host knows as /mnt/user. Docker and
// libvirt are the recording fakes above.
type restoreRig struct {
	svc      *Service
	st       *store.Repo
	docker   *rigDocker
	virsh    *rigVirsh
	progress *progress.Store
	root     string
}

func newRestoreRig(t *testing.T) *restoreRig {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("restic's <id>:<path> selector cannot carry a Windows drive letter")
	}
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "bombvault")
	if err := os.Mkdir(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	st := newTestStore(t)
	settings, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.ContainersPath = "backups/containers"
	settings.VMsPath = "backups/vms"
	settings.FilesPath = "backups/files"
	// Without a password restic skips its key derivation, which is most of what
	// a run on a repository this small costs.
	settings.EncryptionEnabled = false
	if err := st.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	r := &restoreRig{
		st:       st,
		docker:   &rigDocker{containers: map[string]model.Inspect{}, images: map[string]bool{}},
		virsh:    &rigVirsh{},
		progress: progress.NewStore(),
		root:     root,
	}
	cfg := config.Config{
		AppKey: strings.Repeat("a", 64), DataDir: dataDir,
		HostMountRoot: root, HostSourceRoot: "/mnt/user", DataRootSegments: []string{"appdata"},
	}
	r.svc = NewService(cfg, st, r.docker, r.virsh, restic.Restic{Bin: "restic"})
	r.svc.SetPlatform(platform.Generic{})
	r.svc.SetProgress(r.progress)
	writeMountFixture(t, "/", root)
	return r
}

// write puts files below the Host Data mount, creating their folders.
func (r *restoreRig) write(t *testing.T, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(r.root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { //nolint:gosec // G301: test folder
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil { //nolint:gosec // G306: test file
			t.Fatal(err)
		}
	}
}

// wait blocks until the detached restore has finished.
func (r *restoreRig) wait(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for r.svc.BackupInProgress() {
		if time.Now().After(deadline) {
			t.Fatal("the restore never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// await blocks until a stalled restore has reported in.
func await(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(60 * time.Second):
		t.Fatal("the restore never reached the engine")
	}
}

// lastRun is the newest run of a kind for a target.
func (r *restoreRig) lastRun(t *testing.T, targetID, kind string) store.Run {
	t.Helper()
	run, err := r.st.LastRunOfKind(targetID, kind)
	if err != nil || run == nil {
		t.Fatalf("no %s run for %s: %v", kind, targetID, err)
	}
	return *run
}

// treeSum fingerprints everything below dir: each entry's path, type, mode,
// link target and content. With times it covers the modification times too,
// which change on any write into a folder, so two equal sums mean nothing
// below dir was touched.
func treeSum(t *testing.T, dir string, times bool) string {
	t.Helper()
	h := sha256.New()
	add := func(format string, a ...any) { h.Write(fmt.Appendf(nil, format, a...)) }
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		add("%s\x00%v\x00", rel, info.Mode())
		if times {
			add("%d\x00", info.ModTime().UnixNano())
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			add("%s\x00", target)
		case info.Mode().IsRegular():
			content, err := os.ReadFile(p) //nolint:gosec // G304: a file below the test's own folder
			if err != nil {
				return err
			}
			sum := sha256.Sum256(content)
			h.Write(sum[:])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("fingerprint %s: %v", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// entries lists the names in a folder, sorted.
func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(list))
	for i, e := range list {
		names[i] = e.Name()
	}
	return names
}
