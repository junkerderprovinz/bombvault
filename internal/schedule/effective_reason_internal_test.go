package schedule

import (
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

func TestEffectiveNoneSaysWhyNothingRuns(t *testing.T) {
	on := func() (s store.Settings) {
		s = baseSettings()
		s.FilesSchedule = "weekly mon 02:00"
		return s
	}
	for _, c := range []struct {
		name     string
		enabled  bool
		override string
		edit     func(*store.Settings)
		want     string
	}{
		{"domain off", true, "", func(s *store.Settings) { s.FilesEnabled = false }, NoneDomainOff},
		{"left out of the schedule", false, "", nil, NoneExcluded},
		{"own schedule off", true, "off", nil, NoneOverrideOff},
		{"no schedule runs", true, "", func(s *store.Settings) { s.FilesSchedule = "off" }, NoneScheduleOff},
		{"a schedule that does not parse", true, "", func(s *store.Settings) { s.FilesSchedule = "weekly someday" }, NoneScheduleInvalid},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := on()
			if c.edit != nil {
				c.edit(&s)
			}
			got := EffectiveFileSetSchedule(set(c.enabled, c.override), s)
			if got.Kind != EffectiveNone || got.Reason != c.want {
				t.Fatalf("got %q/%q, want none/%q", got.Kind, got.Reason, c.want)
			}
		})
	}
}

func TestARunningScheduleCarriesNoReason(t *testing.T) {
	s := baseSettings()
	s.FilesSchedule = "weekly mon 02:00"
	if got := EffectiveFileSetSchedule(set(true, ""), s); got.Reason != "" {
		t.Fatalf("reason = %q for a set the domain schedule backs up", got.Reason)
	}
}
