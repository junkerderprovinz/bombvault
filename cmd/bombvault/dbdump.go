package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// dbdumpDeps is what the helper reaches outside itself for.
type dbdumpDeps struct {
	newExecer func() (dbdump.Execer, func(), error)
	exit      func(int)
	signals   func(cancel context.CancelFunc) (stop func())
}

func defaultDBDumpDeps() dbdumpDeps {
	return dbdumpDeps{
		newExecer: func() (dbdump.Execer, func(), error) {
			dc, err := dockercli.New()
			if err != nil {
				return nil, nil, err
			}
			return dc, func() { _ = dc.Close() }, nil
		},
		exit:    os.Exit,
		signals: notifyHelperSignals,
	}
}

var (
	// dbdumpHelperDeadline is when the helper gives up on its own, counted from
	// its start. restic starts it only once the repository is open, so the
	// dump's own limit plus a minute covers the whole run.
	dbdumpHelperDeadline = func(max time.Duration) time.Duration { return max + time.Minute }
	// dbdumpWriteStall is how long one write to stdout may stay blocked before
	// the helper stops waiting for a reader that has gone away.
	dbdumpWriteStall = 10 * time.Minute
)

const (
	dbdumpStdoutBuffer = 1 << 20
	// dbdumpOrphanStopBound caps what a deadline spends on the dump it leaves
	// behind, since it is on its way out of the process.
	dbdumpOrphanStopBound = 5 * time.Second
)

var (
	containerNameRe = regexp.MustCompile(model.ResourceNamePattern)
	containerIDRe   = regexp.MustCompile(`^[0-9a-f]{12,64}$`)
)

type dbdumpStreamOptions struct {
	container string
	engine    dbdump.Engine
	max       time.Duration
}

// runDBDumpStream writes one database dump to stdout and one result line to
// stderr. restic runs it as the command of a `backup --stdin-from-command`, so
// stdout carries the dump and nothing else, while everything the helper has to
// say goes to stderr, which restic forwards line by line.
//
// It gives up on its own on the hard deadline and on a stdout write that has
// been blocked for dbdumpWriteStall, both from a timer goroutine that never
// touches the writer: restic 0.17.3 waits forever for a child blocked in a
// write it has stopped reading (restic #5683), and the containers domain would
// stay locked behind it until the whole backup's cap runs out.
func runDBDumpStream(ctx context.Context, args []string, stdout, stderr io.Writer, deps dbdumpDeps) int {
	opts, err := parseDBDumpStreamFlags(args)
	if err != nil {
		writeResult(stderr, dbdump.Result{V: 1, Reason: dbdump.ReasonUsage, Detail: dbdump.ScrubDetail(err.Error())})
		return 2
	}
	ex, closeExecer, err := deps.newExecer()
	if err != nil {
		writeResult(stderr, dbdump.Result{V: 1, Reason: dbdump.ReasonDocker, Detail: dbdump.ScrubDetail(err.Error())})
		return 1
	}
	defer closeExecer()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := deps.signals(cancel)
	defer stop()

	// Both the dump's own lines and the result line go through the tap, which
	// serialises them and picks up the pid a deadline would need.
	errs := &pidTap{out: stderr}
	var verdict sync.Once
	giveUp := func(reason string) {
		verdict.Do(func() {
			writeResult(errs, dbdump.Result{V: 1, Reason: reason})
			stopOrphan(ex, opts.container, errs.pid())
			deps.exit(1)
		})
	}

	deadline := time.AfterFunc(dbdumpHelperDeadline(opts.max), func() { giveUp(dbdump.ReasonTimeout) })
	defer deadline.Stop()

	watched := &stallWatch{w: stdout}
	stalled := make(chan struct{})
	defer close(stalled)
	go watchWrites(stalled, watched, dbdumpWriteStall, func() { giveUp(dbdump.ReasonWrite) })

	out := bufio.NewWriterSize(watched, dbdumpStdoutBuffer)
	res := dbdump.Stream(ctx, ex, dbdump.StreamOptions{
		Container: opts.container,
		Engine:    opts.engine,
		Max:       opts.max,
	}, out, errs)
	if err := out.Flush(); err != nil && res.OK {
		res = dbdump.Result{V: 1, Reason: dbdump.ReasonWrite, Bytes: res.Bytes, Scope: res.Scope, Detail: dbdump.ScrubDetail(err.Error())}
	}

	verdict.Do(func() { writeResult(errs, res) })
	// restic reaps the command as soon as its stdout ends and can drop whatever
	// the child wrote to stderr in between, so the result line goes out first
	// and stdout is closed here rather than at process exit.
	if c, ok := stdout.(io.Closer); ok {
		_ = c.Close()
	}
	if !res.OK {
		return 1
	}
	return 0
}

func parseDBDumpStreamFlags(args []string) (dbdumpStreamOptions, error) {
	fs := flag.NewFlagSet("dbdump-stream", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	container := fs.String("container", "", "name or id of the database container")
	engine := fs.String("engine", "", "postgres, mysql or mariadb")
	maxSeconds := fs.Int("max-seconds", 0, "time limit for the dump inside the container")
	if err := fs.Parse(args); err != nil {
		return dbdumpStreamOptions{}, err
	}
	if fs.NArg() > 0 {
		return dbdumpStreamOptions{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if !validContainerName(*container) {
		return dbdumpStreamOptions{}, fmt.Errorf("container %q is neither a container name nor an id", *container)
	}
	e, ok := dbdump.ParseEngine(*engine)
	if !ok {
		return dbdumpStreamOptions{}, fmt.Errorf("engine %q is not postgres, mysql or mariadb", *engine)
	}
	// The dump script owns the bounds of its time limit, so let it judge them.
	if _, err := dbdump.ExecArgv(e, *maxSeconds); err != nil {
		return dbdumpStreamOptions{}, err
	}
	return dbdumpStreamOptions{container: *container, engine: e, max: time.Duration(*maxSeconds) * time.Second}, nil
}

// validContainerName takes what the api takes as a resource name, plus a bare
// container id.
func validContainerName(name string) bool {
	if containerIDRe.MatchString(name) {
		return true
	}
	return containerNameRe.MatchString(name) && !strings.Contains(name, "..")
}

// watchWrites fires once a single write to stdout has been blocked for stall.
// A dump that merely produces nothing, such as one waiting on a lock inside
// the database, has no write in flight and is left to its own time limit.
func watchWrites(done <-chan struct{}, w *stallWatch, stall time.Duration, fire func()) {
	tick := time.NewTicker(stall / 4)
	defer tick.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-tick.C:
			if since := w.blockedSince(); !since.IsZero() && now.Sub(since) >= stall {
				fire()
				return
			}
		}
	}
}

// stallWatch remembers when the write to stdout that has not yet returned
// began.
type stallWatch struct {
	w io.Writer

	mu    sync.Mutex
	since time.Time
}

func (s *stallWatch) Write(p []byte) (int, error) {
	s.mark(time.Now())
	defer s.mark(time.Time{})
	return s.w.Write(p)
}

func (s *stallWatch) mark(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.since = t
}

func (s *stallWatch) blockedSince() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.since
}

// stopOrphan signals the dump the helper is about to leave running inside the
// container. Only the helper's own deadlines come here: on a stop signal restic
// kills the helper milliseconds later, and an exec round trip loses that race.
func stopOrphan(ex dbdump.Execer, container string, pid int) {
	argv, err := dbdump.OrphanStopArgv(pid)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dbdumpOrphanStopBound)
	defer cancel()
	_ = ex.ExecQuick(ctx, container, argv)
}

func writeResult(w io.Writer, res dbdump.Result) {
	_, _ = io.WriteString(w, res.Line()+"\n")
}

// pidTap forwards the container's stderr unchanged and remembers the pid the
// dump reported, which is what a deadline needs to stop it.
type pidTap struct {
	out io.Writer

	mu sync.Mutex
	id int
}

func (t *pidTap) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if pid, ok := dbdump.ParsePID(strings.Split(string(p), "\n")); ok {
		t.id = pid
	}
	return t.out.Write(p)
}

func (t *pidTap) pid() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.id
}
