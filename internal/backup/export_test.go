package backup

import "time"

// SetHealthTimingForTest shrinks the health-wait poll interval and the
// no-healthcheck settle grace so the health-gated restart tests can exercise the
// wait loop without incurring real-time delays. It returns a function that
// restores the previous values (call it via t.Cleanup). Test-only hook — the
// production values are never mutated outside tests.
func SetHealthTimingForTest(poll, grace time.Duration) func() {
	prevPoll, prevGrace := healthPollInterval, healthNoCheckGrace
	healthPollInterval, healthNoCheckGrace = poll, grace
	return func() { healthPollInterval, healthNoCheckGrace = prevPoll, prevGrace }
}
