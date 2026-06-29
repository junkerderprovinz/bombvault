package api

import "testing"

// TestRPOStatus exhaustively covers the pure RPO status decision: the boundaries
// between ok / warn / overdue, plus the never / off short-circuits.
func TestRPOStatus(t *testing.T) {
	const day = int64(86400)
	const now = int64(1_000_000_000)

	cases := []struct {
		name      string
		last      int64
		period    int64
		scheduled bool
		want      string
	}{
		{"not scheduled", now - day, day, false, "off"},
		{"zero period", now - day, 0, true, "off"},
		{"scheduled but never run", 0, day, true, "never"},
		{"fresh backup is ok", now, day, true, "ok"},
		{"exactly one period is ok", now - day, day, true, "ok"},
		{"just past one period is warn", now - day - 1, day, true, "warn"},
		{"between one and two periods is warn", now - day*2, day, true, "warn"},
		{"exactly two periods is warn", now - day*2, day, true, "warn"},
		{"just past two periods is overdue", now - day*2 - 1, day, true, "overdue"},
		{"far overdue", now - day*30, day, true, "overdue"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rpoStatus(now, c.last, c.period, c.scheduled); got != c.want {
				t.Fatalf("rpoStatus(now, last=%d, period=%d, scheduled=%v) = %q, want %q",
					c.last, c.period, c.scheduled, got, c.want)
			}
		})
	}
}
