package backup

import "time"

// SetHealthTimingForTest shortens the health poll interval and the grace for
// containers without a healthcheck, and returns a function that restores them.
func SetHealthTimingForTest(poll, grace time.Duration) func() {
	prevPoll, prevGrace := healthPollInterval, healthNoCheckGrace
	healthPollInterval, healthNoCheckGrace = poll, grace
	return func() { healthPollInterval, healthNoCheckGrace = prevPoll, prevGrace }
}
