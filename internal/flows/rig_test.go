//go:build flows

// Package flows drives the real server through its HTTP API, with real restic
// and real folders, and checks what happened on disk against what the API told
// the user. Containers, VMs and ZFS need a host, so folder sets, the flash and
// the self-backup carry the flows.
//
// Run it with restic on PATH:
//
//	go test -tags flows -count=1 -timeout 30m -v ./internal/flows/
//
// The update flows build the release before this tree, so the clone needs its
// tags. FLOWS_PREVIOUS_TAG picks another release to update from. Without -v go
// test keeps a passing package's output to itself, and with it the list of known
// failures this suite ends on.
package flows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restickey"
)

// appKey is fixed so a data folder written by one binary opens under the next.
var appKey = strings.Repeat("7c", 32)

var (
	currentBinary string
	moduleRoot    string
	buildDir      string
)

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	if _, err := exec.LookPath("restic"); err != nil {
		fmt.Fprintln(os.Stderr, "flows: restic is not on PATH")
		return 1
	}
	var err error
	if buildDir, err = os.MkdirTemp("", "bombvault-flows"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(buildDir) }()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "flows: find the module root:", err)
		return 1
	}
	moduleRoot = strings.TrimSpace(string(out))
	if currentBinary, err = buildServer(moduleRoot, "current"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	code := m.Run()
	reportKnownFailures()
	return code
}

// buildServer compiles cmd/bombvault below dir. -trimpath lets the build of an
// older tree reuse the compile cache.
func buildServer(dir, name string) (string, error) {
	bin := filepath.Join(buildDir, name)
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-trimpath", "-o", bin, "./cmd/bombvault")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build %s: %w\n%s", name, err, out)
	}
	return bin, nil
}

// bombvault is one server process with its folders. The folders outlive the
// process, so a test can stop one binary and start another on the same data.
type bombvault struct {
	t     *testing.T
	root  string
	mount string
	data  string
	flash string
	base  string
	proc  *exec.Cmd
	// exited closes when proc has ended, with exitErr set.
	exited  chan struct{}
	exitErr error
	out     *syncBuffer
	// http keeps the session cookie once a password is set.
	http     *http.Client
	password string
	// platform overrides what the server detects, "" leaves it to the flash.
	platform string
}

// newBombvault lays out the folders of an install with nothing in them.
func newBombvault(t *testing.T) *bombvault {
	t.Helper()
	// restic records a path with its links followed, so the mount root has to be
	// spelled that way too. It stays slash-spelled on every system, as
	// paths.Resolve reads it.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := &bombvault{
		t: t, root: dir, data: filepath.Join(dir, "config"), flash: filepath.Join(dir, "boot"),
		mount: filepath.ToSlash(filepath.Join(dir, "mnt")), out: &syncBuffer{},
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	b.http = &http.Client{Jar: jar}
	for _, sub := range []string{b.data, filepath.Join(dir, "home"), b.hostPath("user"), filepath.Join(b.flash, "config", "plugins", "dockerMan")} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(b.flash, "config", "ident.cfg"), []byte("NAME=\"flows\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		b.stop()
		if t.Failed() {
			t.Logf("server log:\n%s", b.out.tail(4000))
		}
	})
	return b
}

// mountOnStorage puts the host mount on a real mount point. A restore into a
// folder or next to the original refuses a folder on no mount of its own, since
// on Unraid that lies in RAM, and a temp folder is on none. On a plain host the
// mount itself counts, and every Linux has /dev/shm. Only one install at a time
// can use it.
func (b *bombvault) mountOnStorage() {
	b.t.Helper()
	const root = "/dev/shm"
	if _, err := os.Stat(root); runtime.GOOS != "linux" || err != nil {
		b.t.Skip("this restore needs a folder on a mount of its own, and this system has no /dev/shm")
	}
	b.mount, b.platform = root, "generic"
	clear := func() {
		if err := os.RemoveAll(b.hostPath("user")); err != nil {
			b.t.Error(err)
		}
	}
	clear()
	b.t.Cleanup(clear)
	if err := os.MkdirAll(b.hostPath("user"), 0o755); err != nil {
		b.t.Fatal(err)
	}
}

// start runs bin on this install's folders and waits until it answers. Docker
// and the libvirt host point nowhere, so the server finds an empty host, and
// HOME is its own because the server writes an ssh config there at every start.
func (b *bombvault) start(bin string) {
	b.t.Helper()
	port := freePort(b.t)
	home := filepath.Join(b.root, "home")
	cmd := exec.Command(bin)
	cmd.Dir = b.root
	cmd.Stdout, cmd.Stderr = b.out, b.out
	cmd.Env = append(os.Environ(),
		"APP_KEY="+appKey,
		"DATA_DIR="+b.data,
		"HOST_MOUNT_ROOT="+b.mount,
		"HOST_SOURCE_ROOT=/mnt",
		"HTTP_ONLY=true",
		"BIND_HOST=127.0.0.1",
		"PORT="+port,
		"DOCKER_HOST=tcp://127.0.0.1:1",
		"LIBVIRT_HOST=127.0.0.1",
		"LIBVIRT_SSH_PORT=1",
		"FLASH_DIR="+b.flash,
		"FLASH_TEMPLATES_DIR="+filepath.Join(b.flash, "config", "plugins", "dockerMan", "templates-user"),
		"HOME="+home,
		"USERPROFILE="+home,
	)
	if b.platform != "" {
		cmd.Env = append(cmd.Env, "PLATFORM="+b.platform)
	}
	if err := cmd.Start(); err != nil {
		b.t.Fatalf("start %s: %v", bin, err)
	}
	b.proc, b.exited = cmd, make(chan struct{})
	go func() {
		b.exitErr = cmd.Wait()
		close(b.exited)
	}()
	b.base = "http://127.0.0.1:" + port
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-b.exited:
			b.proc = nil
			b.t.Fatalf("the server exited during its start: %v\n%s", b.exitErr, b.out.tail(4000))
		default:
		}
		if resp, err := http.Get(b.base + "/api/health"); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if b.password != "" {
					b.call("POST", "/api/login", map[string]any{"password": b.password}, nil)
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	b.t.Fatalf("the server did not answer within a minute\n%s", b.out.tail(4000))
}

// stop ends the process the way a container stop does where the system has
// signals. Windows has none to send, so there it is killed.
func (b *bombvault) stop() {
	if b.proc == nil {
		return
	}
	if err := b.proc.Process.Signal(syscall.SIGTERM); err != nil {
		_ = b.proc.Process.Kill()
	}
	select {
	case <-b.exited:
	case <-time.After(30 * time.Second):
		_ = b.proc.Process.Kill()
		<-b.exited
		b.t.Errorf("the server did not stop within 30 seconds of being asked")
	}
	b.proc = nil
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
}

// syncBuffer collects the output of a process that writes while a test reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) tail(n int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.buf.String()
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

// signIn sets a login password and signs in. A fresh install has none, and
// pruning or deleting backups by hand asks for one.
func (b *bombvault) signIn() {
	b.t.Helper()
	b.password = "a long enough password for the flows"
	b.call("POST", "/api/auth/password", map[string]any{"password": b.password}, nil)
	b.call("POST", "/api/login", map[string]any{"password": b.password}, nil)
}

// hostPath is where a path typed relative to the host mount lies on disk.
func (b *bombvault) hostPath(sub string) string {
	return filepath.Join(filepath.FromSlash(b.mount), filepath.FromSlash(sub))
}

// send makes one request and returns its status and body.
func (b *bombvault) send(method, path string, in any, headers ...string) (int, []byte) {
	b.t.Helper()
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			b.t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, b.base+path, body)
	if err != nil {
		b.t.Fatal(err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := b.http.Do(req)
	if err != nil {
		b.t.Fatalf("%s %s: %v\n%s", method, path, err, b.out.tail(2000))
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		b.t.Fatalf("%s %s: read the answer: %v", method, path, err)
	}
	return resp.StatusCode, raw
}

// call fails the test on any answer but 200 with ok true, and decodes into out
// when it is given. A busy domain is waited out: the restore probe after an
// item's first backup holds it for a moment, as it does for a person.
func (b *bombvault) call(method, path string, in, out any, headers ...string) {
	b.t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for {
		status, raw := b.send(method, path, in, headers...)
		if status == http.StatusConflict && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		var envelope struct {
			OK *bool `json:"ok"`
		}
		_ = json.Unmarshal(raw, &envelope)
		if status != http.StatusOK || (envelope.OK != nil && !*envelope.OK) {
			b.t.Fatalf("%s %s = %d: %s", method, path, status, raw)
		}
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				b.t.Fatalf("%s %s: decode %s: %v", method, path, raw, err)
			}
		}
		return
	}
}

// settings reads the settings as the page holds them, with their revision.
func (b *bombvault) settings() (map[string]any, string) {
	b.t.Helper()
	var got struct {
		Settings map[string]any `json:"settings"`
		Revision string         `json:"revision"`
	}
	b.call("GET", "/api/settings", nil, &got)
	return got.Settings, got.Revision
}

// changeSettings saves the whole settings object as the page does, with one
// edit made to it.
func (b *bombvault) changeSettings(edit func(map[string]any)) {
	b.t.Helper()
	settings, revision := b.settings()
	edit(settings)
	b.call("PUT", "/api/settings", settings, nil, "X-Settings-Revision", revision)
}

// write puts a file below a path relative to the host mount.
func (b *bombvault) write(sub, name, body string, mode os.FileMode) {
	b.t.Helper()
	p := filepath.Join(b.hostPath(sub), filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		b.t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		b.t.Fatal(err)
	}
}

// resticSnapshot is a snapshot as restic itself lists it.
type resticSnapshot struct {
	ID    string    `json:"id"`
	Time  time.Time `json:"time"`
	Tags  []string  `json:"tags"`
	Paths []string  `json:"paths"`
}

// restic runs restic on a repository with the password the server derives, so
// a check reads the repository without going through the server.
func (b *bombvault) restic(repo string, args ...string) []byte {
	b.t.Helper()
	cmd := exec.Command("restic", append([]string{"-r", repo, "--no-lock", "--no-cache"}, args...)...)
	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+restickey.Derive(appKey))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		b.t.Fatalf("restic %s on %s: %v\n%s", strings.Join(args, " "), repo, err, stderr.String())
	}
	return out
}

// snapshotsIn lists a repository's snapshots, those of tag when it is given.
func (b *bombvault) snapshotsIn(sub, tag string) []resticSnapshot {
	b.t.Helper()
	args := []string{"snapshots", "--json"}
	if tag != "" {
		args = append(args, "--tag", tag)
	}
	var out []resticSnapshot
	if err := json.Unmarshal(b.restic(b.hostPath(sub), args...), &out); err != nil {
		b.t.Fatalf("read the snapshots of %s: %v", sub, err)
	}
	return out
}

// repoBytes is what a repository holds on disk after dedup, the number a prune
// has to bring down.
func (b *bombvault) repoBytes(sub string) int64 {
	b.t.Helper()
	var stats struct {
		TotalSize int64 `json:"total_size"`
	}
	if err := json.Unmarshal(b.restic(b.hostPath(sub), "stats", "--mode", "raw-data", "--json"), &stats); err != nil {
		b.t.Fatalf("read the size of %s: %v", sub, err)
	}
	return stats.TotalSize
}

// run is a recorded run as the run history lists it.
type run struct {
	TargetID   string `json:"targetId"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	SnapshotID string `json:"snapshotId"`
	Error      string `json:"error"`
}

func (b *bombvault) runs() []run {
	b.t.Helper()
	var got struct {
		Runs []run `json:"runs"`
	}
	b.call("GET", "/api/runs", nil, &got)
	return got.Runs
}

// runsOf lists the finished runs of one kind, the newest first.
func (b *bombvault) runsOf(kind string) []run {
	b.t.Helper()
	return slices.DeleteFunc(b.runs(), func(r run) bool { return r.Kind != kind || r.Status == "running" })
}

// settle waits until nothing runs any more: no run open, no progress bar up
// and no batch holding the server. Everything the flows start runs in the
// background, and retention and the batched prune come after the run they
// belong to is closed.
func (b *bombvault) settle() {
	b.t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	for quiet := 0; quiet < 3; {
		if time.Now().After(deadline) {
			b.t.Fatalf("still busy after five minutes\n%s", b.out.tail(4000))
		}
		if b.busy() {
			quiet = 0
		} else {
			quiet++
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (b *bombvault) busy() bool {
	b.t.Helper()
	if slices.ContainsFunc(b.runs(), func(r run) bool { return r.Status == "running" }) {
		return true
	}
	_, raw := b.send("GET", "/api/progress?snapshot", nil)
	if bytes.Contains(raw, []byte(`"active":true`)) {
		return true
	}
	// The page has no question for whether a batch holds the server. A restore
	// of a set that does not exist answers that first and starts nothing.
	_, raw = b.send("POST", "/api/files/sets/no-such-set/restore-files", map[string]any{"snapshotId": "0", "paths": []string{"/x"}})
	return bytes.Contains(raw, []byte("already running"))
}

// awaitRun waits for a finished run of kind beyond the known ones, for work the
// server starts on its own after answering.
func (b *bombvault) awaitRun(kind string, known int) run {
	b.t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if got := b.runsOf(kind); len(got) > known {
			return got[0]
		}
		time.Sleep(50 * time.Millisecond)
	}
	b.t.Fatalf("no %s run was recorded\n%s", kind, b.out.tail(4000))
	return run{}
}
