package restic

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// Processes keeps count of the restic processes a Restic runs, so the server
// can end them before it exits. restic runs in a process group of its own,
// which the container's stop signal does not reach, and when the server exits
// first the container's end kills restic outright, with its lock left behind.
// The zero value is ready to use.
type Processes struct {
	mu      sync.Mutex
	ending  bool
	next    int
	cancels map[int]context.CancelFunc
	wg      sync.WaitGroup
}

// start returns the context one restic run goes under and the func that ends
// its count. Once End has begun the context comes back cancelled, so nothing
// new starts. A nil Processes counts nothing.
func (p *Processes) start(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)
	if p == nil {
		return ctx, cancel
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ending {
		cancel()
		return ctx, cancel
	}
	if p.cancels == nil {
		p.cancels = map[int]context.CancelFunc{}
	}
	id := p.next
	p.next++
	p.cancels[id] = cancel
	p.wg.Add(1)
	return ctx, func() {
		p.mu.Lock()
		delete(p.cancels, id)
		p.mu.Unlock()
		cancel()
		p.wg.Done()
	}
}

// End stops every restic run still going and waits up to grace for them to
// return. It reports whether they all did.
func (p *Processes) End(grace time.Duration) bool {
	p.mu.Lock()
	p.ending = true
	cancels := make([]context.CancelFunc, 0, len(p.cancels))
	for _, cancel := range p.cancels {
		cancels = append(cancels, cancel)
	}
	p.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

// command prepares one restic run in its own process group. The run counts
// for End until the returned func is called, which the caller defers.
func (r Restic) command(ctx context.Context, args ...string) (context.Context, *exec.Cmd, func()) {
	ctx, done := r.Processes.start(ctx)
	cmd := exec.CommandContext(ctx, r.bin(), args...) //nolint:gosec // G204: argv is constructed by typed builders in this package; no user input reaches here
	configureProcGroup(cmd)
	return ctx, cmd, done
}
