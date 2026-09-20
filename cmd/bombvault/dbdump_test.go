package main

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dbdump"
	"github.com/junkerderprovinz/bombvault/internal/model"
)

// fakeExecer plays the container side of a dump: run writes what the dump tool
// would write and returns its exit code.
type fakeExecer struct {
	run func(ctx context.Context, stdout, stderr io.Writer) (int, error)

	mu    sync.Mutex
	quick [][]string
}

func (f *fakeExecer) ExecStream(ctx context.Context, _ string, _ []string, stdout, stderr io.Writer) (int, error) {
	return f.run(ctx, stdout, stderr)
}

func (f *fakeExecer) ExecQuick(_ context.Context, _ string, cmd []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quick = append(f.quick, cmd)
	return nil
}

func (f *fakeExecer) quickCalls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]string(nil), f.quick...)
}

// eventLog records the order of the writes the helper makes, so a test can say
// what happened before what.
type eventLog struct {
	mu   sync.Mutex
	seen []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, event)
}

func (l *eventLog) events() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.seen...)
}

type recordedStdout struct {
	log *eventLog

	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *recordedStdout) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *recordedStdout) Close() error {
	if w.log != nil {
		w.log.add("stdout closed")
	}
	return nil
}

func (w *recordedStdout) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

type recordedStderr struct {
	log *eventLog

	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *recordedStderr) Write(p []byte) (int, error) {
	if w.log != nil {
		w.log.add("stderr " + strings.TrimRight(string(p), "\r\n"))
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *recordedStderr) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// blockedWriter takes accept bytes and then blocks until it is released, the
// shape restic 0.17.3 leaves behind when it stops reading the helper's stdout.
type blockedWriter struct {
	accept  int
	release chan struct{}

	mu   sync.Mutex
	took int
}

func (w *blockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	room := w.accept - w.took
	if room >= len(p) {
		w.took += len(p)
		w.mu.Unlock()
		return len(p), nil
	}
	w.mu.Unlock()
	<-w.release
	return len(p), nil
}

func postgresDump(size int) string {
	marker := dbdump.EnginePostgres.CompletionMarker() + "\n"
	return strings.Repeat("x", size-len(marker)) + marker
}

// helperArgs is a valid flag set with the shortest limit the dump script takes.
func helperArgs() []string {
	return []string{"--container", "pg", "--engine", "postgres", "--max-seconds", "60"}
}

func testDeps(ex dbdump.Execer) dbdumpDeps {
	return dbdumpDeps{
		newExecer: func() (dbdump.Execer, func(), error) { return ex, func() {}, nil },
		exit:      func(int) {},
		signals:   func(context.CancelFunc) func() { return func() {} },
	}
}

// shrinkDeadline and shrinkWriteStall bring the helper's two bounds within a
// test's patience. The dump script's own floor keeps --max-seconds at a minute,
// so the deadline cannot be reached by shortening the limit alone.
func shrinkDeadline(t *testing.T, after time.Duration) {
	t.Helper()
	prev := dbdumpHelperDeadline
	dbdumpHelperDeadline = func(time.Duration) time.Duration { return after }
	t.Cleanup(func() { dbdumpHelperDeadline = prev })
}

func shrinkWriteStall(t *testing.T, stall time.Duration) {
	t.Helper()
	prev := dbdumpWriteStall
	dbdumpWriteStall = stall
	t.Cleanup(func() { dbdumpWriteStall = prev })
}

func lastResult(t *testing.T, stderr string) dbdump.Result {
	t.Helper()
	res, ok := dbdump.ParseResult(strings.Split(stderr, "\n"))
	if !ok {
		t.Fatalf("no result line in stderr %q", stderr)
	}
	return res
}

func TestDBDumpStreamUsage(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"a path as the container", []string{"--container", "../x", "--engine", "postgres", "--max-seconds", "60"}},
		{"an option as the container", []string{"--container", "-rf", "--engine", "postgres", "--max-seconds", "60"}},
		{"an engine nobody dumps", []string{"--container", "pg", "--engine", "oracle", "--max-seconds", "60"}},
		{"a limit below the script's own floor", []string{"--container", "pg", "--engine", "postgres", "--max-seconds", "5"}},
		{"no flags at all", nil},
		{"no engine", []string{"--container", "pg", "--max-seconds", "60"}},
		{"an unknown flag", []string{"--container", "pg", "--engine", "postgres", "--max-seconds", "60", "--verbose"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex := &fakeExecer{run: func(context.Context, io.Writer, io.Writer) (int, error) {
				t.Error("a usage error must not reach the container")
				return 0, nil
			}}
			var stdout recordedStdout
			stderr := &recordedStderr{}
			code := runDBDumpStream(context.Background(), c.args, &stdout, stderr, testDeps(ex))

			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout.String() != "" {
				t.Errorf("stdout = %q, want nothing", stdout.String())
			}
			if res := lastResult(t, stderr.String()); res.Reason != dbdump.ReasonUsage {
				t.Errorf("reason = %q, want %q", res.Reason, dbdump.ReasonUsage)
			}
		})
	}
}

func TestDBDumpStreamStdoutIsOnlyTheDump(t *testing.T) {
	dump := postgresDump(64 << 10)
	ex := &fakeExecer{run: func(_ context.Context, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, dbdump.PIDLinePrefix+"4711\n")
		_, _ = io.WriteString(stderr, dbdump.ScopeLinePrefix+"all\n")
		_, err := io.WriteString(stdout, dump)
		return 0, err
	}}

	log := &eventLog{}
	stdout := &recordedStdout{log: log}
	stderr := &recordedStderr{log: log}
	code := runDBDumpStream(context.Background(), helperArgs(), stdout, stderr, testDeps(ex))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stdout.String() != dump {
		t.Errorf("stdout has %d bytes, want exactly the %d bytes of the dump", len(stdout.String()), len(dump))
	}
	res := lastResult(t, stderr.String())
	if !res.OK || res.Bytes != int64(len(dump)) || res.Scope != "all" {
		t.Errorf("result = %+v, want ok, %d bytes and scope all", res, len(dump))
	}
	if n := strings.Count(stderr.String(), dbdump.ResultLinePrefix); n != 1 {
		t.Errorf("%d result lines on stderr, want exactly 1", n)
	}

	events := log.events()
	if len(events) < 3 {
		t.Fatalf("events = %v", events)
	}
	if events[0] != "stderr "+dbdump.PIDLinePrefix+"4711" {
		t.Errorf("first event = %q, want the pid line", events[0])
	}
	if last := events[len(events)-1]; last != "stdout closed" {
		t.Errorf("last event = %q, want the stdout close", last)
	}
	if before := events[len(events)-2]; !strings.Contains(before, dbdump.ResultLinePrefix) {
		t.Errorf("event before the close = %q, want the result line", before)
	}
}

// TestDBDumpStreamHardDeadlineDoesNotWaitForWriter covers restic 0.17.3's
// defect #5683: when restic stops reading, the helper blocks in a write and
// restic waits for it. The helper's own deadline has to end the process without
// touching the blocked writer.
func TestDBDumpStreamHardDeadlineDoesNotWaitForWriter(t *testing.T) {
	shrinkDeadline(t, 30*time.Millisecond)

	ex := &fakeExecer{run: func(_ context.Context, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, dbdump.PIDLinePrefix+"4711\n")
		_, err := io.WriteString(stdout, postgresDump(8<<10))
		return 0, err
	}}
	writer := &blockedWriter{release: make(chan struct{})}
	stderr := &recordedStderr{}

	exited := make(chan int, 1)
	var atExit string
	deps := testDeps(ex)
	deps.exit = func(code int) {
		atExit = stderr.String()
		exited <- code
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		runDBDumpStream(context.Background(), helperArgs(), writer, stderr, deps)
	}()

	select {
	case code := <-exited:
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the helper waited for the blocked writer instead of giving up")
	}

	if res := lastResult(t, atExit); res.Reason != dbdump.ReasonTimeout {
		t.Errorf("reason = %q, want %q written before the exit", res.Reason, dbdump.ReasonTimeout)
	}
	stops := ex.quickCalls()
	if len(stops) != 1 {
		t.Fatalf("%d orphan stops, want exactly 1", len(stops))
	}
	want, err := dbdump.OrphanStopArgv(4711)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(stops[0], "\x00") != strings.Join(want, "\x00") {
		t.Errorf("orphan stop = %q, want %q", stops[0], want)
	}

	close(writer.release)
	<-done
}

// TestDBDumpStreamWriteWatchdog covers the same blocked reader on a dump that
// is still producing data: the writes stop completing, and the helper gives up
// long before its hard deadline.
func TestDBDumpStreamWriteWatchdog(t *testing.T) {
	shrinkDeadline(t, time.Hour)
	shrinkWriteStall(t, 40*time.Millisecond)

	ex := &fakeExecer{run: func(_ context.Context, stdout, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, dbdump.PIDLinePrefix+"4711\n")
		dump := postgresDump(4 << 20)
		for at := 0; at < len(dump); at += 64 << 10 {
			if _, err := io.WriteString(stdout, dump[at:min(at+64<<10, len(dump))]); err != nil {
				return 1, err
			}
		}
		return 0, nil
	}}
	writer := &blockedWriter{accept: 1 << 20, release: make(chan struct{})}
	stderr := &recordedStderr{}

	exited := make(chan int, 1)
	var atExit string
	deps := testDeps(ex)
	deps.exit = func(code int) {
		atExit = stderr.String()
		exited <- code
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		runDBDumpStream(context.Background(), helperArgs(), writer, stderr, deps)
	}()

	select {
	case code := <-exited:
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the write watchdog never fired")
	}
	if res := lastResult(t, atExit); res.Reason != dbdump.ReasonWrite {
		t.Errorf("reason = %q, want %q", res.Reason, dbdump.ReasonWrite)
	}

	close(writer.release)
	<-done
}

// TestDBDumpStreamSignalCancels checks the one thing the helper still does on a
// stop signal. It does not stop the dump inside the container: restic kills the
// helper milliseconds later, and an exec round trip would lose that race.
func TestDBDumpStreamSignalCancels(t *testing.T) {
	ex := &fakeExecer{run: func(ctx context.Context, _, stderr io.Writer) (int, error) {
		_, _ = io.WriteString(stderr, dbdump.PIDLinePrefix+"4711\n")
		<-ctx.Done()
		return 0, ctx.Err()
	}}

	var stdout recordedStdout
	stderr := &recordedStderr{}
	deps := testDeps(ex)
	deps.signals = func(cancel context.CancelFunc) func() {
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		return func() {}
	}

	code := runDBDumpStream(context.Background(), helperArgs(), &stdout, stderr, deps)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if res := lastResult(t, stderr.String()); res.Reason != dbdump.ReasonCancelled {
		t.Errorf("reason = %q, want %q", res.Reason, dbdump.ReasonCancelled)
	}
	if stops := ex.quickCalls(); len(stops) != 0 {
		t.Errorf("%d orphan stops, want none on a signal", len(stops))
	}
}

func TestDBDumpStreamNeverPrintsEnvironment(t *testing.T) {
	const secret = "topsecret"
	t.Setenv("RESTIC_PASSWORD", secret)

	dump := postgresDump(4 << 10)
	cases := []struct {
		name string
		args []string
		run  func(ctx context.Context, stdout, stderr io.Writer) (int, error)
	}{
		{
			name: "success",
			args: helperArgs(),
			run: func(_ context.Context, stdout, _ io.Writer) (int, error) {
				_, err := io.WriteString(stdout, dump)
				return 0, err
			},
		},
		{
			name: "usage error",
			args: []string{"--container", "pg", "--engine", "oracle", "--max-seconds", "60"},
			run: func(context.Context, io.Writer, io.Writer) (int, error) {
				return 0, nil
			},
		},
		{
			name: "the dump tool fails",
			args: helperArgs(),
			run: func(_ context.Context, _, stderr io.Writer) (int, error) {
				_, _ = io.WriteString(stderr, "pg_dumpall: error: connection to server failed\n")
				return 1, nil
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout recordedStdout
			stderr := &recordedStderr{}
			runDBDumpStream(context.Background(), c.args, &stdout, stderr, testDeps(&fakeExecer{run: c.run}))

			if strings.Contains(stdout.String(), secret) {
				t.Error("stdout carried the password")
			}
			if strings.Contains(stderr.String(), secret) {
				t.Errorf("stderr carried the password: %q", stderr.String())
			}
		})
	}
}

// TestContainerFlagUsesTheSharedNamePattern keeps the helper's flag and the
// api's route parameter on one rule: a name one of them takes is a name the
// other takes.
func TestContainerFlagUsesTheSharedNamePattern(t *testing.T) {
	shared := regexp.MustCompile(model.ResourceNamePattern)
	cases := []struct {
		name string
		want bool
	}{
		{"plex", true},
		{"my-db_1.0", true},
		{"a", true},
		{"", false},
		{"-rf", false},
		{"../x", false},
		{"a/b", false},
		{"a b", false},
		{".hidden", false},
		{strings.Repeat("a", 128), true},
		{strings.Repeat("a", 129), false},
	}
	for _, c := range cases {
		if got := shared.MatchString(c.name); got != c.want {
			t.Errorf("model.ResourceNamePattern matches %q = %v, want %v", c.name, got, c.want)
		}
		if got := validContainerName(c.name); got != c.want {
			t.Errorf("validContainerName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
	if validContainerName("a..b") {
		t.Error(`validContainerName("a..b") took a name the api rejects for traversal`)
	}
	if id := strings.Repeat("0a", 32); !validContainerName(id) {
		t.Errorf("validContainerName(%q) rejected a container id", id)
	}
}
