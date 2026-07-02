package restic

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// resticHelperEnv, when "1" in a spawned copy of the test binary, makes
// TestResticSleeper act as a long-running stand-in for restic: it blocks until
// its process is killed. That lets the parent point run()'s restic binary at
// this test binary, cancel run()'s context mid-execution, and observe the error
// run() surfaces for a killed child.
const resticHelperEnv = "BOMBVAULT_RESTIC_SLEEPER"

// TestResticSleeper is not a real test. When the helper env is set (only in the
// child process run() spawns) it blocks, simulating a long restic restore, so
// exec.CommandContext can kill it on cancel. In a normal test run the env is
// unset and it returns immediately.
func TestResticSleeper(t *testing.T) {
	if os.Getenv(resticHelperEnv) != "1" {
		return
	}
	time.Sleep(30 * time.Second) // bounded so a stray child can't wedge CI
}

// TestRunWrapsCancelAsContextCanceled is the REAL-path cancel test the fake
// engine cannot provide. run() spawns a killable child; the test cancels the
// context mid-run. exec.CommandContext then kills the child, which cmd.Wait
// reports as an *ExitError ("signal: killed" / "exit status ...") — NOT
// context.Canceled. run() must detect ctx.Err() and re-wrap so
// errors.Is(err, context.Canceled) holds; otherwise every finish site records a
// user cancel as "failed" (the whole cancel feature is inert in production).
func TestRunWrapsCancelAsContextCanceled(t *testing.T) {
	// Point restic at THIS test binary and mark it (via the env authEnv passes on
	// to the child) as the blocking sleeper. t.Setenv also forbids t.Parallel, so
	// TestResticSleeper never sees the env set during an unrelated parallel test.
	r := Restic{Bin: os.Args[0]}
	t.Setenv(resticHelperEnv, "1")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond) // let the child start and block
		cancel()
	}()

	// run() execs os.Args[0] with these args: -test.run filters the child to the
	// sleeper, "--" ends the go-test flags. subcommand() reads "restore" only for
	// the error string; no sink is in ctx, so run() takes the buffered path.
	_, err := r.run(ctx, []string{"-test.run=^TestResticSleeper$", "--", "restore"}, Mode{})
	if err == nil {
		t.Fatal("run() must return an error when its context is cancelled mid-run")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled run must unwrap to context.Canceled, got %v", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a plain cancel must not also look like a deadline, got %v", err)
	}
}

// TestRunKeepsDeadlineExceededDistinct pins that a ctx DEADLINE (the 48h restore
// cap) stays context.DeadlineExceeded and does NOT collapse into
// context.Canceled — a wedged restore that blew its cap must still record
// "failed", never "cancelled".
func TestRunKeepsDeadlineExceededDistinct(t *testing.T) {
	r := Restic{Bin: os.Args[0]}
	t.Setenv(resticHelperEnv, "1")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := r.run(ctx, []string{"-test.run=^TestResticSleeper$", "--", "restore"}, Mode{})
	if err == nil {
		t.Fatal("run() must return an error when its context deadline is exceeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a deadline-exceeded run must unwrap to context.DeadlineExceeded, got %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("a deadline must NOT be recorded as a user cancel, got %v", err)
	}
}
