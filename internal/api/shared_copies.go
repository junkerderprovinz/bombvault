package api

import (
	"log"
	"sync"
	"time"
)

// sharedCopies keeps this server's copies to shared repositories apart from
// the prunes of their owners. An owner's word that it prunes and a copy that
// starts meet under one lock, so either the owner hears of the copy or the
// copy waits for the next pass. The word lives in memory: after a restart a
// copy may meet the prune at restic's lock, where it waits and gives up
// without touching it.
type sharedCopies struct {
	mu sync.Mutex
	// running counts the copies under way to each repository.
	running map[string]int
	// quiet is until when each owner prunes.
	quiet map[string]time.Time
}

// begin starts a copy to repo, which the server with group instance id owner
// holds, unless that owner prunes there at now. release ends it.
func (c *sharedCopies) begin(repo, owner string, now time.Time) (release func(), ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Before(c.quiet[owner]) {
		return nil, false
	}
	if c.running == nil {
		c.running = map[string]int{}
	}
	c.running[repo]++
	return sync.OnceFunc(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.running[repo]--; c.running[repo] == 0 {
			delete(c.running, repo)
		}
	}), true
}

// hush is an owner's word that it prunes repos until until. While a copy to
// one of them runs, the answer is busy and nothing is promised.
func (c *sharedCopies) hush(owner string, repos []string, until time.Time) (busy bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, repo := range repos {
		if c.running[repo] > 0 {
			return true
		}
	}
	if c.quiet == nil {
		c.quiet = map[string]time.Time{}
	}
	c.quiet[owner] = until
	return false
}

// lift is the owner's word that its prune is over.
func (c *sharedCopies) lift(owner string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.quiet, owner)
}

// beginSharedCopy starts a copy to the off-site target at repo, or reports
// that the owner of the shared repository there prunes it. A repository no
// one shares needs no word from anybody.
func (s *Service) beginSharedCopy(repo string) (release func(), ok bool) {
	sr, shared, err := s.store.SharedRepoAt(repo)
	if err != nil {
		log.Printf("api: could not read whether %s is shared, so the copy goes ahead: %v", scrubRepoLocation(repo), err) //nolint:gosec // G706: credentials are scrubbed from the location
		return func() {}, true
	}
	if !shared {
		return func() {}, true
	}
	return s.sharedCopies.begin(repo, sr.Owner, time.Now())
}
