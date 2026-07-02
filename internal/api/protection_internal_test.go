package api

import "testing"

// TestProtectionLevel exhaustively covers the red/amber/green aggregation,
// including the staleness path (a tamper test older than 2× its schedule period),
// which can't be reached through RecordTamperTest (it always stamps "now").
func TestProtectionLevel(t *testing.T) {
	const day = int64(86400)
	now := int64(1_700_000_000)
	week := 7 * day

	cases := []struct {
		name string
		in   protInputs
		want string
	}{
		{"disabled → empty", protInputs{enabled: false, offsiteConfigured: true}, ""},
		{"enabled, no off-site → red", protInputs{enabled: true, offsiteConfigured: false}, "red"},
		{"immutable, no tamper yet → red", protInputs{
			enabled: true, offsiteConfigured: true, offsiteImmutable: true, tamperPeriod: week,
		}, "red"},
		{"immutable, tamper failed → red", protInputs{
			enabled: true, offsiteConfigured: true, offsiteImmutable: true,
			hadTamper: true, lastTamperOK: false, lastTamperAt: now, tamperPeriod: week,
		}, "red"},
		{"immutable, tamper stale → red", protInputs{
			enabled: true, offsiteConfigured: true, offsiteImmutable: true,
			hadTamper: true, lastTamperOK: true, lastTamperAt: now - 15*day, tamperPeriod: week,
		}, "red"},
		{"immutable, tamper fresh → green", protInputs{
			enabled: true, offsiteConfigured: true, offsiteImmutable: true,
			hadTamper: true, lastTamperOK: true, lastTamperAt: now - day, tamperPeriod: week,
		}, "green"},
		{"configured, non-immutable, no tamper → green", protInputs{
			enabled: true, offsiteConfigured: true,
		}, "green"},
		{"replication overdue → amber", protInputs{
			enabled: true, offsiteConfigured: true,
			lastReplicationAt: now - 3*day, offsitePeriod: day,
		}, "amber"},
		{"replication only just late (< 2x) → green", protInputs{
			enabled: true, offsiteConfigured: true,
			lastReplicationAt: now - day - 1, offsitePeriod: day, // age < 2x period
		}, "green"},
		{"dr drill overdue → amber", protInputs{
			enabled: true, offsiteConfigured: true,
			lastDRDrillAt: now - 30*day, drillPeriod: week,
		}, "amber"},
		{"never replicated but scheduled → green (not overdue)", protInputs{
			enabled: true, offsiteConfigured: true, offsitePeriod: day, // lastReplicationAt 0
		}, "green"},
		{"red beats amber: stale tamper AND overdue replication → red", protInputs{
			enabled: true, offsiteConfigured: true, offsiteImmutable: true,
			hadTamper: true, lastTamperOK: true, lastTamperAt: now - 30*day, tamperPeriod: week,
			lastReplicationAt: now - 3*day, offsitePeriod: day,
		}, "red"},
		{"immutable+fresh tamper but replication overdue → amber", protInputs{
			enabled: true, offsiteConfigured: true, offsiteImmutable: true,
			hadTamper: true, lastTamperOK: true, lastTamperAt: now - day, tamperPeriod: week,
			lastReplicationAt: now - 3*day, offsitePeriod: day,
		}, "amber"},
	}
	for _, c := range cases {
		if got := protectionLevel(now, c.in); got != c.want {
			t.Errorf("%s: protectionLevel = %q, want %q", c.name, got, c.want)
		}
	}
}
