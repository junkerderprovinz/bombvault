package schedule

import "time"

// SetSharedUpkeepJob wires the upkeep of the shared repositories this server
// holds (SweepCadence). fn runs whatever is due, and next says when the
// soonest pass comes. Call before Reload.
func (s *Scheduler) SetSharedUpkeepJob(fn func() error, next func(time.Time) (time.Time, bool)) {
	s.sharedUpkeepFn, s.sharedUpkeepNext = fn, next
}
