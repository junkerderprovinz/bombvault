package main

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

func TestPlatformForTrueNAS(t *testing.T) {
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

func TestPlatformForUnraid(t *testing.T) {
	if _, ok := platformFor(platform.KindUnraid).(platform.Unraid); !ok {
		t.Fatalf("platformFor(KindUnraid) = %T, want platform.Unraid", platformFor(platform.KindUnraid))
	}
}

func TestPlatformForGeneric(t *testing.T) {
	if _, ok := platformFor(platform.KindGeneric).(platform.Generic); !ok {
		t.Fatalf("platformFor(KindGeneric) = %T, want platform.Generic", platformFor(platform.KindGeneric))
	}
}

func TestPlatformForUnknownKindFallsBackToGeneric(t *testing.T) {
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

// TestLogSchedulerTimezone sets time.Local directly because Go caches it at
// startup, so changing TZ alone would not move the clock. A fixed zone keeps
// the test independent of tzdata.
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
