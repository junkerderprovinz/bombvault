package schedule

import "time"

// SetNodeJob wires the schedules home keeps for its nodes (SweepCadence). fn
// starts the runs that are due and learns how the earlier ones went, and next
// says when the soonest run comes. Call before Reload.
func (s *Scheduler) SetNodeJob(fn func() error, next func(time.Time) (time.Time, bool)) {
	s.nodeFn, s.nodeNext = fn, next
}
