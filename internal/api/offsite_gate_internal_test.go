package api

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestOffsiteReplicatesOnOwnSchedule pins the #95-review fix: the predicate that
// decides "does a SEPARATE off-site cron drive this domain, or does the backup path
// replicate it (coupled)?" must agree with the scheduler's cron registration, which
// keys off ParseCadence(...).Enabled. Both a blank schedule AND the literal "off"
// are disabled cadences → coupled. If this ever regressed to a bare `!= ""` check,
// setting the off-site schedule to "off" would leave the domain replicated by
// nobody (inline skips, batched skips, no cron registered) — a silently rotting DR
// copy. Only a real enabled cadence means "its own schedule".
func TestOffsiteReplicatesOnOwnSchedule(t *testing.T) {
	s := &Service{}
	cases := []struct {
		name     string
		schedule string
		wantOwn  bool // true = a separate cron drives it; false = coupled to the backup run
	}{
		{"blank couples", "", false},
		{"off couples (regression: no silent DR rot)", "off", false},
		{"whitespace couples", "   ", false},
		{"invalid cadence defaults to coupled (safe direction)", "not-a-cadence", false},
		{"daily is its own schedule", "daily 02:00", true},
		{"weekly is its own schedule", "weekly Sun 03:00", true},
		{"everyN is its own schedule", "everyN 3 04:00", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settings := store.Settings{ContainersOffsiteSchedule: c.schedule}
			if got := s.offsiteReplicatesOnOwnSchedule("containers", settings); got != c.wantOwn {
				t.Fatalf("offsiteReplicatesOnOwnSchedule(%q) = %v, want %v", c.schedule, got, c.wantOwn)
			}
		})
	}
}
