package restic

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// resticHelperEnv set to "1" turns a child copy of the test binary into a
// restic stand-in that blocks until it is killed.
const resticHelperEnv = "BOMBVAULT_RESTIC_SLEEPER"

// TestResticSleeper is a helper, not a test. In the child process run() spawns
// it blocks like a long restore until exec.CommandContext kills it; in a normal
// test run it returns at once.
func TestResticSleeper(t *testing.T) {
	if os.Getenv(resticHelperEnv) != "1" {
		return
	}
	time.Sleep(30 * time.Second) // bounded so a stray child can't wedge CI
}

// TestRunWrapsCancelAsContextCanceled cancels a real child mid-run. cmd.Wait
// reports the kill as an *ExitError, so run() has to check ctx.Err() and wrap
// context.Canceled itself, or every caller would record a user cancel as a
// failure.
func TestRunWrapsCancelAsContextCanceled(t *testing.T) {
	// authEnv passes the helper env on to the child. t.Setenv also rules out
	// t.Parallel, so TestResticSleeper never sees the env during another test.
	r := Restic{Bin: os.Args[0]}
	t.Setenv(resticHelperEnv, "1")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond) // let the child start and block
		cancel()
	}()

	// -test.run limits the child to the sleeper and "--" ends the go test flags.
	// "restore" only feeds subcommand() for the error text; with no sink in ctx,
	// run() takes the buffered path.
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

// TestRunKeepsDeadlineExceededDistinct checks that a deadline such as the 48h
// restore cap stays context.DeadlineExceeded. A restore that ran past its cap
// has failed; it was not cancelled.
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
