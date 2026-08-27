package main

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// TestPlatformForTrueNASReturnsRealImplementation confirms PLATFORM=truenas
// (platform.KindTrueNAS) now resolves to a real platform.TrueNAS{} instance —
// NOT the Phase A fallback-to-generic-with-warning path, which today only
// applies to a genuinely unrecognized Kind.
func TestPlatformForTrueNASReturnsRealImplementation(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	p := platformFor(platform.KindTrueNAS)
	if _, ok := p.(platform.TrueNAS); !ok {
		t.Fatalf("platformFor(KindTrueNAS) = %T, want platform.TrueNAS", p)
	}
	if got := p.Kind(); got != platform.KindTrueNAS {
		t.Fatalf("platformFor(KindTrueNAS).Kind() = %q, want %q", got, platform.KindTrueNAS)
	}
	if strings.Contains(buf.String(), "no implementation") {
		t.Fatalf("platformFor(KindTrueNAS) must not log the Phase A not-implemented-yet warning anymore, log=%q", buf.String())
	}
}

// TestPlatformForUnraidUnchanged / TestPlatformForGenericUnchanged pin that
// this task did not disturb the two already-implemented mappings.
func TestPlatformForUnraidUnchanged(t *testing.T) {
	if _, ok := platformFor(platform.KindUnraid).(platform.Unraid); !ok {
		t.Fatalf("platformFor(KindUnraid) = %T, want platform.Unraid", platformFor(platform.KindUnraid))
	}
}

func TestPlatformForGenericUnchanged(t *testing.T) {
	if _, ok := platformFor(platform.KindGeneric).(platform.Generic); !ok {
		t.Fatalf("platformFor(KindGeneric) = %T, want platform.Generic", platformFor(platform.KindGeneric))
	}
}

// TestPlatformForUnknownKindStillFallsBackToGenericWithWarning: the
// fallback-with-warning path must still exist for a genuinely unrecognized
// Kind — Task 9 only removes it for the now-implemented KindTrueNAS case.
func TestPlatformForUnknownKindStillFallsBackToGenericWithWarning(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	p := platformFor(platform.Kind("amiga-os"))
	if _, ok := p.(platform.Generic); !ok {
		t.Fatalf("platformFor(unknown) = %T, want platform.Generic", p)
	}
	if !strings.Contains(buf.String(), "amiga-os") {
		t.Fatalf("an unrecognized Kind must still be logged, log=%q", buf.String())
	}
}

// TestLogSchedulerTimezone pins the three outcomes the boot log has to
// distinguish, because the difference between them is exactly the difference
// between a backup running when the operator thinks it does and one running
// hours later. time.Local is set directly rather than via TZ alone: Go resolves
// time.Local once and caches it, so setting the variable mid-process would not
// move the clock and the test would pass for the wrong reason. A fixed zone is
// used instead of LoadLocation so the test needs no tzdata on the test runner.
func TestLogSchedulerTimezone(t *testing.T) {
	prevLocal := time.Local
	defer func() { time.Local = prevLocal }()

	cases := []struct {
		name     string
		tz       string
		local    *time.Location
		want     []string
		unwanted []string
	}{
		{
			name:     "unset falls back to UTC and says so",
			tz:       "",
			local:    time.UTC,
			want:     []string{"planning in UTC", "TZ is NOT set", "02:30 UTC"},
			unwanted: []string{"did NOT resolve"},
		},
		{
			name:     "unresolvable zone is called out, not silently UTC",
			tz:       "Europe/Berln",
			local:    time.UTC,
			want:     []string{"did NOT resolve", "Europe/Berln", "fell back to UTC"},
			unwanted: []string{"TZ is NOT set"},
		},
		{
			name:     "resolved zone is named",
			tz:       "Europe/Berlin",
			local:    time.FixedZone("CEST", 2*60*60),
			want:     []string{"planning in CEST", "Europe/Berlin"},
			unwanted: []string{"TZ is NOT set", "did NOT resolve"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TZ", tc.tz)
			time.Local = tc.local

			var buf strings.Builder
			prev := log.Writer()
			log.SetOutput(&buf)
			defer log.SetOutput(prev)

			logSchedulerTimezone()

			got := buf.String()
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("log missing %q\ngot: %s", want, got)
				}
			}
			for _, bad := range tc.unwanted {
				if strings.Contains(got, bad) {
					t.Errorf("log must not contain %q\ngot: %s", bad, got)
				}
			}
		})
	}
}
