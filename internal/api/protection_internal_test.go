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

// TestProtectionChecksConsistentWithLevel pins that the per-check states
// protectionChecks emits never contradict protectionLevel: a {never,failed,stale}
// tamper state coincides with a red chip (via the immutable branch), an "overdue"
// replication/drill state coincides with amber, and an all-good posture is green
// with empty check states. This is the invariant that makes the dashboard card a
// pure renderer that cannot diverge from the chip.
func TestProtectionChecksConsistentWithLevel(t *testing.T) {
	const day = int64(86400)
	now := int64(1_700_000_000)
	week := 7 * day

	cases := []struct {
		name       string
		in         protInputs
		wantChecks protChecks
		wantLevel  string
	}{
		{
			name:       "all-good non-immutable → empty checks + green",
			in:         protInputs{enabled: true, offsiteConfigured: true},
			wantChecks: protChecks{Tamper: "", Replication: "", Drill: ""},
			wantLevel:  "green",
		},
		{
			name: "immutable + fresh tamper → ok + green",
			in: protInputs{
				enabled: true, offsiteConfigured: true, offsiteImmutable: true,
				hadTamper: true, lastTamperOK: true, lastTamperAt: now - day, tamperPeriod: week,
			},
			wantChecks: protChecks{Tamper: "ok"},
			wantLevel:  "green",
		},
		{
			name: "immutable + stale tamper → stale + red",
			in: protInputs{
				enabled: true, offsiteConfigured: true, offsiteImmutable: true,
				hadTamper: true, lastTamperOK: true, lastTamperAt: now - 15*day, tamperPeriod: week,
			},
			wantChecks: protChecks{Tamper: "stale"},
			wantLevel:  "red",
		},
		{
			name: "immutable + never tamper → never + red",
			in: protInputs{
				enabled: true, offsiteConfigured: true, offsiteImmutable: true, tamperPeriod: week,
			},
			wantChecks: protChecks{Tamper: "never"},
			wantLevel:  "red",
		},
		{
			name: "immutable + failed tamper → failed + red",
			in: protInputs{
				enabled: true, offsiteConfigured: true, offsiteImmutable: true,
				hadTamper: true, lastTamperOK: false, lastTamperAt: now, tamperPeriod: week,
			},
			wantChecks: protChecks{Tamper: "failed"},
			wantLevel:  "red",
		},
		{
			name: "overdue replication → overdue + amber",
			in: protInputs{
				enabled: true, offsiteConfigured: true,
				lastReplicationAt: now - 3*day, offsitePeriod: day,
			},
			wantChecks: protChecks{Replication: "overdue"},
			wantLevel:  "amber",
		},
		{
			// A PASSED but stale drill: currency drives the row (a failed drill would
			// instead read "failed", covered by TestProtectionChecksDrillHonorsOutcome).
			name: "overdue drill → overdue + amber",
			in: protInputs{
				enabled: true, offsiteConfigured: true,
				lastDRDrillAt: now - 30*day, lastDRDrillOK: true, drillPeriod: week,
			},
			wantChecks: protChecks{Drill: "overdue"},
			wantLevel:  "amber",
		},
		{
			// A recent but FAILED DR drill: the row reads a red "failed", so the chip
			// must NOT read green over it — protectionLevel downgrades it to amber (a
			// failed restorability proof needs attention; other protections are fine).
			name: "failed recent drill → failed row + amber chip (chip can't be green over a red row)",
			in: protInputs{
				enabled: true, offsiteConfigured: true,
				lastDRDrillAt: now - day, lastDRDrillOK: false, drillPeriod: week,
			},
			wantChecks: protChecks{Drill: "failed"},
			wantLevel:  "amber",
		},
	}
	for _, c := range cases {
		gotChecks := protectionChecks(now, c.in)
		if gotChecks != c.wantChecks {
			t.Errorf("%s: protectionChecks = %+v, want %+v", c.name, gotChecks, c.wantChecks)
		}
		gotLevel := protectionLevel(now, c.in)
		if gotLevel != c.wantLevel {
			t.Errorf("%s: protectionLevel = %q, want %q", c.name, gotLevel, c.wantLevel)
		}
		// Invariant: a red-inducing tamper state must coincide with a red chip.
		redTamper := gotChecks.Tamper == "never" || gotChecks.Tamper == "failed" || gotChecks.Tamper == "stale"
		if redTamper && gotLevel != "red" {
			t.Errorf("%s: tamper %q must coincide with a red chip, got %q", c.name, gotChecks.Tamper, gotLevel)
		}
		// Invariant: an "overdue" replication/drill must coincide with (at least) amber.
		if (gotChecks.Replication == "overdue" || gotChecks.Drill == "overdue") && gotLevel == "green" {
			t.Errorf("%s: an overdue check must not coincide with a green chip", c.name)
		}
		// Invariant: a red "failed" drill row must not coincide with a green chip.
		if gotChecks.Drill == "failed" && gotLevel == "green" {
			t.Errorf("%s: a failed drill row must not coincide with a green chip, got %q", c.name, gotLevel)
		}
	}
}

// TestProtectionChecksDrillHonorsOutcome pins that the DR-drill scorecard row
// reflects the latest drill's OUTCOME, not just its recency: a recorded DR drill
// that FAILED reads "failed" (a red row) even when it is recent, so the row can't
// go green-by-currency while the off-site "proven restorable" pill (lastDRDrillOK)
// reads red. A passed recent drill stays "ok"; no drill yet stays "never"; and a
// failed drill beats currency (still "failed" even when also overdue).
func TestProtectionChecksDrillHonorsOutcome(t *testing.T) {
	const day = int64(86400)
	now := int64(1_700_000_000)
	week := 7 * day

	// A domain with a drill schedule set and an off-site configured, so the Drill
	// row makes a claim; each case then varies only the latest drill's at/ok.
	base := func() protInputs {
		return protInputs{enabled: true, offsiteConfigured: true, drillPeriod: week}
	}
	with := func(at int64, ok bool) protInputs {
		in := base()
		in.lastDRDrillAt = at
		in.lastDRDrillOK = ok
		return in
	}

	cases := []struct {
		name string
		in   protInputs
		want string
	}{
		{"recent failed drill → failed (not green-by-recency)", with(now-day, false), "failed"},
		{"recent passed drill → ok", with(now-day, true), "ok"},
		{"no drill yet → never", base(), "never"},
		{"overdue passed drill → overdue", with(now-30*day, true), "overdue"},
		{"overdue failed drill → failed (outcome beats currency)", with(now-30*day, false), "failed"},
	}
	for _, c := range cases {
		if got := protectionChecks(now, c.in).Drill; got != c.want {
			t.Errorf("%s: Drill = %q, want %q", c.name, got, c.want)
		}
	}
}
