package zfsrepl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/sshconn"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// Hop is how the source host reaches a ZFS server with an ssh of its own: the
// server's address, and the replica key and the server's pinned host key as
// paths on that host.
type Hop struct {
	Host, User, Port string
	Key              string
	KnownHosts       string
	// PublicKey is the line Key's .pub file has to hold, so a path that leads
	// to some other key is never used.
	PublicKey string
}

// PipeRunner is the part of an sshconn.Conn a host pipeline runs over.
type PipeRunner interface {
	RunStreams(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args ...string) error
}

var _ PipeRunner = (*sshconn.Conn)(nil)

// directEnd is the source host as an End that pipes its sends into the
// receives on one ZFS server by itself.
type directEnd struct {
	*SSHEnd
	run    PipeRunner
	hop    Hop
	target *SSHEnd
	once   sync.Once
	usable bool
}

var _ Piper = (*directEnd)(nil)

// Direct returns source as an End that pipes its sends into the receives on
// target by itself, through hop, once a check from the host found that this
// works. A source or target it cannot pair that way comes back as it is.
func Direct(source End, hop Hop, target End) End {
	src, ok := source.(*SSHEnd)
	if !ok {
		return source
	}
	run, ok := src.conn.(PipeRunner)
	dst, isSSH := target.(*SSHEnd)
	if !ok || !isSSH || !hostPathFits(hop.Key) || !hostPathFits(hop.KnownHosts) {
		return source
	}
	return &directEnd{SSHEnd: src, run: run, hop: hop, target: dst}
}

// hostPathFits rules out a path ssh would read as something else: it expands
// % and ~ in both paths, and splits the known_hosts option at blanks unless
// they sit inside double quotes.
func hostPathFits(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.ContainsAny(p, "%\"\\\n\r\t")
}

func (e *directEnd) Pipe(ctx context.Context, sendArgs []string, to End, recvArgs []string, report func(int64)) (int64, bool, error) {
	if to != End(e.target) || !e.check(ctx) {
		return 0, false, nil
	}
	// The pipeline on the host stops once its stdin ends, which closing hold
	// does here and a dropped connection does there.
	stdin, hold, err := os.Pipe()
	if err != nil {
		return 0, false, nil
	}
	stop := context.AfterFunc(ctx, func() { _ = hold.Close() })
	defer func() {
		stop()
		_ = hold.Close()
		_ = stdin.Close()
	}()

	send := withProgress(e.host.Argv(sendArgs))
	recv := e.target.host.Argv(recvArgs)
	stderr := &pipeLog{report: report}
	var recvSaid limitedBuffer
	err = e.run.RunStreams(ctx, stdin, &recvSaid, stderr, e.argv("pipe", sshconn.RemoteCommand(recv...), send...)...)
	n, err := stderr.result(send, recv, recvSaid.String(), err)
	return n, true, err
}

// checkTimeout bounds the check, which waits on two ssh handshakes.
const checkTimeout = 30 * time.Second

// check asks the host once whether it reaches the server with the replica key
// and pin, and logs the answer, which tells a slow run from a fast one.
func (e *directEnd) check(ctx context.Context) bool {
	e.once.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, checkTimeout)
		defer cancel()
		var out limitedBuffer
		if err := e.run.RunStreams(ctx, nil, &out, &out, e.argv("check", "true", e.hop.PublicKey)...); err != nil {
			log.Printf("zfs replica: the host cannot stream to %s itself, so the streams go through BombVault: %v: %s", e.hop.Host, err, strings.TrimSpace(out.String()))
			return
		}
		e.usable = true
		log.Printf("zfs replica: the host streams to %s itself", e.hop.Host)
	})
	return e.usable
}

func (e *directEnd) argv(mode, remote string, rest ...string) []string {
	h := e.hop
	return append([]string{"sh", "-c", hostPipeline, "bombvault-replica",
		mode, h.Key, h.KnownHosts, h.Port, h.User + "@" + h.Host, sshconn.StreamCiphers, remote}, rest...)
}

// withProgress has zfs send print, once a second, how far it got.
func withProgress(argv []string) []string {
	return slices.Concat(argv[:2], []string{"-v", "-P"}, argv[2:])
}

// hostPipeline runs on the source host. Its arguments are the mode, the
// replica key, the pinned known_hosts, the server's port and user@host, the
// ciphers and the command for the server; then the zfs send argv to pipe, or
// the public key line to check.
//
// The pipe goes through a fifo so that both sides have a pid and a status of
// their own without pipefail, which dash lacks. The server's stderr comes back on
// stdout, the send's on stderr, which ends with a line naming both statuses.
// sshd sends a command without a terminal no signal when its connection goes,
// it only ends its stdin, so a reader of stdin stops both sides then. A
// background job starts on /dev/null, which is why that reader gets fd 3.
const hostPipeline = `mode=$1 key=$2 known=$3 port=$4 dest=$5 ciphers=$6 remote=$7
shift 7
hop() {
	exec ssh -F none -i "$key" -p "$port" -o IdentitiesOnly=yes -o IdentityAgent=none \
		-o BatchMode=yes -o StrictHostKeyChecking=yes -o UpdateHostKeys=no \
		-o "UserKnownHostsFile=\"$known\"" -o GlobalKnownHostsFile=/dev/null \
		-o ConnectTimeout=10 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 \
		-o "Ciphers=$ciphers" "$dest" -- "$remote"
}
if [ "$mode" = check ]; then
	[ "$(cat -- "$key.pub")" = "$1" ] || { echo "$key.pub holds another key" >&2; exit 1; }
	command -v mkfifo >/dev/null && command -v mktemp >/dev/null || { echo "mkfifo or mktemp is missing" >&2; exit 1; }
	hop </dev/null
fi
dir=$(mktemp -d) || exit 1
trap 'rm -rf "$dir"' EXIT
mkfifo "$dir/stream" || exit 1
exec 3<&0
"$@" >"$dir/stream" 3<&- &
send=$!
hop <"$dir/stream" 2>&1 >/dev/null 3<&- &
recv=$!
{ while read -r line; do :; done; kill "$send" "$recv"; } <&3 >/dev/null 2>&1 &
watch=$!
wait "$recv"
r=$?
wait "$send"
s=$?
kill "$watch" 2>/dev/null
printf '\n` + pipeEnd + `%s %s\n' "$s" "$r" >&2
[ "$s" = 0 ] && [ "$r" = 0 ]
`

// pipeEnd starts the last line of the pipeline's stderr.
const pipeEnd = "bombvault-pipe "

// pipeLog reads what the host pipeline writes to stderr: the progress lines of
// zfs send -v -P, which go to report, the statuses it ends with, and whatever
// else the send said.
type pipeLog struct {
	report func(int64)

	mu       sync.Mutex
	line     []byte
	said     limitedBuffer
	done     int64
	ended    bool
	sendExit int
	recvExit int
}

func (l *pipeLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.line = append(l.line, p...)
	for {
		i := bytes.IndexByte(l.line, '\n')
		if i < 0 {
			return len(p), nil
		}
		l.take(string(l.line[:i]))
		l.line = l.line[i+1:]
	}
}

func (l *pipeLog) take(s string) {
	if st, ok := strings.CutPrefix(s, pipeEnd); ok {
		_, err := fmt.Sscanf(st, "%d %d", &l.sendExit, &l.recvExit)
		l.ended = err == nil
		return
	}
	if n, ok := progressBytes(s); ok {
		l.done = n
		if l.report != nil {
			l.report(n)
		}
		return
	}
	// -v also prints what the stream holds and what a resume token says,
	// which the stderr of a relayed send does not carry.
	if s == "" || strings.Contains(s, "\t") || s == "resume token contents:" || strings.HasPrefix(s, "nvlist version") {
		return
	}
	_, _ = l.said.Write([]byte(s + "\n"))
}

// result is how the pipeline fared, with the reason codes a relayed stream
// would carry.
func (l *pipeLog) result(send, recv []string, recvSaid string, runErr error) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	said := strings.TrimSpace(l.said.String())
	if !l.ended {
		// The pipeline, or the connection to the host, died before it could
		// say how the two sides fared.
		return l.done, &zfs.CmdError{Args: send, Stderr: said, Code: zfs.Classify(said, runErr), Err: runErr}
	}
	return l.done, settle(exited(send, said, l.sendExit), exited(recv, strings.TrimSpace(recvSaid), l.recvExit))
}

// progressBytes reads a line zfs send -v -P prints each second: the time, the
// bytes sent so far and the snapshot, with a block count before the snapshot
// at a higher verbosity.
func progressBytes(s string) (int64, bool) {
	f := strings.Split(s, "\t")
	if len(f) < 3 || len(f[0]) != 8 || f[0][2] != ':' || f[0][5] != ':' {
		return 0, false
	}
	n, err := strconv.ParseInt(f[1], 10, 64)
	return n, err == nil
}

// exited is the error of a command that ended with status, nil for 0.
func exited(argv []string, stderr string, status int) error {
	if status == 0 {
		return nil
	}
	err := exitStatus(status)
	return &zfs.CmdError{Args: argv, Stderr: stderr, Code: zfs.Classify(stderr, err), Err: err}
}

type exitStatus int

func (e exitStatus) Error() string { return "exit status " + strconv.Itoa(int(e)) }
func (e exitStatus) ExitCode() int { return int(e) }

// stderrLimit is as much of a stream's stderr as an error keeps.
const stderrLimit = 64 << 10

// limitedBuffer keeps the first stderrLimit bytes written to it, so a host
// that floods a stream does not fill this process's memory.
type limitedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room := stderrLimit - l.b.Len(); room > 0 {
		l.b.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
