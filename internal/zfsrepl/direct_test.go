package zfsrepl

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// localHost runs the host pipeline on this machine the way sshd runs it on
// the host: the three streams wired through, and no signal when the caller
// goes away.
type localHost struct{ stubStreamer }

func (localHost) RunStreams(_ context.Context, stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec // G204: the argv is the pipeline under test
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	return cmd.Run()
}

// fakeSSH stands in for the host's ssh: it notes its arguments and runs the
// remote command on this machine, or fails the way ssh does when the server
// does not answer.
const fakeSSH = `#!/bin/sh
printf '%s\n' "$@" >>"$FAKE_DIR/ssh-args"
while [ "$1" != -- ]; do shift; done
shift
if { [ "$1" = true ] && [ -n "$CHECK_DOWN" ]; } || { [ "$1" != true ] && [ -n "$STREAM_DOWN" ]; }; then
	echo "ssh: connect to host backup.lan port 22: Connection refused" >&2
	exit 255
fi
exec sh -c "$1"
`

// fakeZFS sends "somebytes" with a progress line before and after the
// second half, and receives into a file.
const fakeZFS = `#!/bin/sh
case $1 in
send)
	echo $$ >"$FAKE_DIR/send.pid"
	printf '%s\n' "$@" >"$FAKE_DIR/send-args"
	printf 'full\tcache/appdata@s\t9\nsize\t9\n' >&2
	printf '12:00:00\t4\tcache/appdata@s\n' >&2
	printf some
	if [ -n "$SEND_HANGS" ]; then while :; do sleep 1; done; fi
	printf bytes
	printf '12:00:01\t9\tcache/appdata@s\n' >&2
	if [ -n "$SEND_FAILS" ]; then echo "$SEND_FAILS" >&2; exit 1; fi
	;;
receive)
	echo $$ >"$FAKE_DIR/receive.pid"
	cat >"$FAKE_DIR/received"
	if [ -n "$RECEIVE_FAILS" ]; then printf '%s\n' "$RECEIVE_FAILS" >&2; exit 1; fi
	;;
esac
`

const testPub = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBombVault bombvault"

type hostRig struct {
	t      *testing.T
	dir    string
	end    End
	target End
}

func newHostRig(t *testing.T) *hostRig {
	t.Helper()
	for _, tool := range []string{"sh", "mkfifo", "mktemp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("needs a POSIX shell with %s", tool)
		}
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"ssh": fakeSSH, "zfs": fakeZFS} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // G306: an executable stand-in
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "id_ed25519.pub"), []byte(testPub+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_DIR", shPath(dir))
	for _, v := range []string{"CHECK_DOWN", "STREAM_DOWN", "SEND_HANGS", "SEND_FAILS", "RECEIVE_FAILS"} {
		t.Setenv(v, "")
	}

	hop := Hop{Host: "backup.lan", User: "root", Port: "22", Key: shPath(filepath.Join(dir, "id_ed25519")),
		KnownHosts: "/mnt/user/appdata/bombvault/ssh-replica/hosts/a b/known_hosts", PublicKey: testPub}
	target := NewSSHEnd(zfs.NewSSHHost(&exitRunner{}), &stubStreamer{})
	return &hostRig{t: t, dir: dir, target: target, end: Direct(NewSSHEnd(zfs.NewSSHHost(&exitRunner{}), &localHost{}), hop, target)}
}

// shPath is p as the shell names it, which on Windows is the MSYS form.
func shPath(p string) string {
	p = filepath.ToSlash(p)
	if runtime.GOOS == "windows" && len(p) > 1 && p[1] == ':' {
		p = "/" + strings.ToLower(p[:1]) + p[2:]
	}
	return p
}

func (r *hostRig) pipe(ctx context.Context, report func(int64)) (int64, bool, error) {
	r.t.Helper()
	p, ok := r.end.(Piper)
	if !ok {
		r.t.Fatal("Direct did not give the source a pipe")
	}
	return p.Pipe(ctx, []string{"zfs", "send", "-c", "-L", "-e", "-p", "cache/appdata@s"}, r.target,
		[]string{"zfs", "receive", "-s", "-u", "tank/bombvault-replica/bottich/cache/appdata"}, report)
}

func (r *hostRig) file(name string) string {
	b, _ := os.ReadFile(filepath.Join(r.dir, name)) //nolint:gosec // G304: a file the fakes wrote into the test's own folder
	return string(b)
}

func TestTheHostPipesTheSendIntoTheReceiveWithThePinnedKey(t *testing.T) {
	r := newHostRig(t)
	var seen []int64
	n, piped, err := r.pipe(context.Background(), func(done int64) { seen = append(seen, done) })
	if err != nil || !piped {
		t.Fatalf("pipe = %d %v %v", n, piped, err)
	}
	if got := r.file("received"); got != "somebytes" {
		t.Errorf("the receive took %q", got)
	}
	if n != 9 || !slices.Equal(seen, []int64{4, 9}) {
		t.Errorf("bytes %d, progress %v, want 9 after 4 and 9", n, seen)
	}
	if send := strings.Fields(r.file("send-args")); !slices.Equal(send[:3], []string{"send", "-v", "-P"}) || send[len(send)-1] != "cache/appdata@s" {
		t.Errorf("the send ran with %q", send)
	}

	args := r.file("ssh-args")
	for _, want := range []string{
		"\n-i\n" + shPath(filepath.Join(r.dir, "id_ed25519")) + "\n",
		"\nStrictHostKeyChecking=yes\n",
		"\nUserKnownHostsFile=\"/mnt/user/appdata/bombvault/ssh-replica/hosts/a b/known_hosts\"\n",
		"\nGlobalKnownHostsFile=/dev/null\n",
		"\nIdentityAgent=none\n",
		"\nroot@backup.lan\n--\n'zfs' 'receive' '-s' '-u' 'tank/bombvault-replica/bottich/cache/appdata'\n",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("the host's ssh ran without %q:\n%s", want, args)
		}
	}
	if strings.Contains(args, "accept-new") || strings.Contains(args, "StrictHostKeyChecking=no") {
		t.Errorf("the host's ssh would accept a host key it has not pinned:\n%s", args)
	}
}

func TestAPipeFailsWithTheCodeARelayedStreamGets(t *testing.T) {
	for _, c := range []struct {
		name string
		env  map[string]string
		code string
	}{
		{"a refused receive", map[string]string{"RECEIVE_FAILS": "cannot receive incremental stream: destination tank/bombvault-replica/bottich/cache/appdata has been modified\nsince most recent snapshot"}, "target-changed"},
		{"a server that stopped answering", map[string]string{"STREAM_DOWN": "1"}, "ssh-unreachable"},
		{"a send that failed under a receive that failed too", map[string]string{
			"SEND_FAILS":    "cannot open 'cache/appdata@s': dataset does not exist",
			"RECEIVE_FAILS": "cannot receive: failed to read from stream",
		}, "not-found"},
		{"a send that failed under a receive that ended", map[string]string{"SEND_FAILS": "cannot open 'cache/appdata@s': dataset does not exist"}, "not-found"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newHostRig(t)
			for k, v := range c.env {
				t.Setenv(k, v)
			}
			_, piped, err := r.pipe(context.Background(), nil)
			if !piped || Code(err) != c.code {
				t.Fatalf("piped %v, err %v (code %q), want %s", piped, err, Code(err), c.code)
			}
		})
	}

	// The relay reads the same words off the same receive.
	relayed := NewSSHEnd(zfs.NewSSHHost(&exitRunner{}), &stubStreamer{}).Receive(context.Background(), []string{"zfs", "receive", "t"}, strings.NewReader(""))
	if Code(relayed) != "target-changed" {
		t.Fatalf("the relayed receive = %v", relayed)
	}
}

func TestCancellingAPipedStreamStopsTheSendAndTheReceiveOnTheHost(t *testing.T) {
	r := newHostRig(t)
	t.Setenv("SEND_HANGS", "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var started atomic.Bool
	done := make(chan error, 1)
	go func() {
		_, _, err := r.pipe(ctx, func(int64) { started.Store(true) })
		done <- err
	}()
	for deadline := time.Now().Add(20 * time.Second); !started.Load() || r.file("receive.pid") == ""; {
		if time.Now().After(deadline) {
			t.Fatal("the pipeline never started")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a cancelled pipe reported success")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the pipe kept running after its run was cancelled")
	}
	for _, side := range []string{"send.pid", "receive.pid"} {
		pid := strings.TrimSpace(r.file(side))
		if pid == "" {
			t.Fatalf("%s is missing, the pipeline never started", side)
		}
		deadline := time.Now().Add(10 * time.Second)
		for exec.Command("sh", "-c", "kill -0 "+pid+" 2>/dev/null").Run() == nil { //nolint:gosec // G204: a pid the fake wrote
			if time.Now().After(deadline) {
				t.Fatalf("the %s side still runs", strings.TrimSuffix(side, ".pid"))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestAHostThatCannotReachTheServerLeavesTheStreamsToTheRelay(t *testing.T) {
	r := newHostRig(t)
	t.Setenv("CHECK_DOWN", "1")
	for range 2 {
		if _, piped, err := r.pipe(context.Background(), nil); piped || err != nil {
			t.Fatalf("piped %v, err %v, want the relay asked to stream", piped, err)
		}
	}
	if r.file("send.pid") != "" {
		t.Error("the host started a send it could not deliver")
	}
	if checks := strings.Count(r.file("ssh-args"), "\nroot@backup.lan\n"); checks != 1 {
		t.Errorf("the host checked %d times, want once per run", checks)
	}
}

func TestAKeyFileThatIsNotTheReplicaKeyIsNeverOffered(t *testing.T) {
	r := newHostRig(t)
	if err := os.WriteFile(filepath.Join(r.dir, "id_ed25519.pub"), []byte("ssh-ed25519 AAAAsomeoneelse root@tower\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, piped, _ := r.pipe(context.Background(), nil); piped {
		t.Fatal("the host piped with a key that is not the replica key")
	}
	if r.file("ssh-args") != "" {
		t.Error("the host's ssh ran with a key that is not the replica key")
	}
}

func TestOnlyTheServerTheSourceWasPairedWithIsPipedTo(t *testing.T) {
	r := newHostRig(t)
	other := NewSSHEnd(zfs.NewSSHHost(&exitRunner{}), &stubStreamer{})
	if _, piped, _ := r.end.(Piper).Pipe(context.Background(), []string{"zfs", "send", "a@b"}, other, []string{"zfs", "receive", "t"}, nil); piped {
		t.Fatal("the source piped to a server it was not paired with")
	}
	hop := Hop{Host: "backup.lan", User: "root", Port: "22", Key: "/mnt/user/app%data/id_ed25519", KnownHosts: "/k", PublicKey: testPub}
	src := NewSSHEnd(zfs.NewSSHHost(&exitRunner{}), &localHost{})
	if _, ok := Direct(src, hop, other).(Piper); ok {
		t.Error("a key path ssh would expand was handed to the host")
	}
	if _, ok := Direct(NewSSHEnd(zfs.NewSSHHost(&exitRunner{}), &stubStreamer{}), Hop{Key: "/k", KnownHosts: "/k"}, other).(Piper); ok {
		t.Error("a source that cannot run a pipeline was given one")
	}
}
