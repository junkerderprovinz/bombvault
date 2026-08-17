package main

import (
	"log"
	"strings"
	"testing"

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
