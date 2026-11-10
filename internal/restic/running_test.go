package restic

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

var sleeperArgs = []string{"-test.run=^TestResticSleeper$", "--", "stats"}

func TestEndStopsARunningResticAndWaitsForIt(t *testing.T) {
	r := Restic{Bin: os.Args[0], Processes: &Processes{}}
	t.Setenv(resticHelperEnv, "1")

	ran := make(chan error, 1)
	go func() {
		_, err := r.run(context.Background(), sleeperArgs, Mode{})
		ran <- err
	}()
	time.Sleep(200 * time.Millisecond) // let the child start and block

	if !r.Processes.End(10 * time.Second) {
		t.Fatal("End gave up on a restic that stops when asked")
	}
	select {
	case err := <-ran:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the stopped run returned %v, want it cancelled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("End returned while the run it stopped was still going")
	}
}

func TestNoResticStartsAfterEnd(t *testing.T) {
	r := Restic{Bin: os.Args[0], Processes: &Processes{}}
	t.Setenv(resticHelperEnv, "1")
	r.Processes.End(time.Second)

	start := time.Now()
	_, err := r.run(context.Background(), sleeperArgs, Mode{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a run after End returned %v, want it cancelled", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("a run after End took %s, it should not start at all", took)
	}
}

func TestEndGivesUpAfterItsGrace(t *testing.T) {
	p := &Processes{}
	_, done := p.start(context.Background())
	defer done()

	start := time.Now()
	if p.End(100 * time.Millisecond) {
		t.Fatal("End reported every run finished while one was still open")
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("End waited %s past a 100ms grace", took)
	}
}
