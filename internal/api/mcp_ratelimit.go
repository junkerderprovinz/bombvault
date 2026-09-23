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
