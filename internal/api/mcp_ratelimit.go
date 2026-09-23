package api

import (
	"sync"
	"time"
)

// slidingWindow counts hits per key over a rolling window. The keys are MCP key
// ids, so the map is bounded by the number of keys used in this process, and a
// key whose window has run empty drops out of it.
type slidingWindow struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	hits   map[string][]time.Time
}

func newSlidingWindow(window time.Duration, max int) *slidingWindow {
	return &slidingWindow{window: window, max: max, hits: map[string][]time.Time{}}
}

// allow records a hit and reports whether it fit in the window. When it did
// not, the second value is how long until the oldest hit falls out.
func (s *slidingWindow) allow(key string, now time.Time) (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.pruneLocked(key, now)
	if len(kept) >= s.max {
		return false, s.window - now.Sub(kept[0])
	}
	s.hits[key] = append(kept, now)
	return true, 0
}

// reserve takes a slot for key and hands back the func that gives it up again.
// A caller that keeps the slot never calls it. Taking the slot and deciding
// whether it was free happen under one lock, so two starts arriving together
// cannot both spend the last one.
func (s *slidingWindow) reserve(key string, now time.Time) (release func(), ok bool, retry time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.pruneLocked(key, now)
	if len(kept) >= s.max {
		return nil, false, s.window - now.Sub(kept[0])
	}
	s.hits[key] = append(kept, now)
	return func() { s.give(key, now) }, true, 0
}

// give returns the slot reserve took at now.
func (s *slidingWindow) give(key string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hits := s.hits[key]
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Equal(now) {
			s.hits[key] = append(hits[:i], hits[i+1:]...)
			break
		}
	}
	if len(s.hits[key]) == 0 {
		delete(s.hits, key)
	}
}

// remaining is how many hits key still has in its window.
func (s *slidingWindow) remaining(key string, now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.max - len(s.pruneLocked(key, now))
}

// pruneLocked drops the hits that have left key's window and returns the rest.
func (s *slidingWindow) pruneLocked(key string, now time.Time) []time.Time {
	cutoff := now.Add(-s.window)
	hits := s.hits[key]
	kept := hits[:0]
	for _, at := range hits {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(s.hits, key)
		return nil
	}
	s.hits[key] = kept
	return kept
}
