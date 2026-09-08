package schedule

import "github.com/junkerderprovinz/bombvault/internal/store"

// EffectiveFileSetSchedule answers the one question the Folders card could not
// answer before: for THIS set, with THESE settings, what actually happens?
//
// The card carries three controls that all read like scheduling, and manilx
// (#199) put them together in the only way that looked sensible and got the
// opposite of what he wanted:
//
//   - "Include in schedule" (FileSet.Enabled) reads like "part of the Folders
//     schedule". It is not. It gates the per-item entry, the domain job AND the
//     Backup Everything pass, so switching it off means the set is never backed
//     up automatically at all.
//   - "Schedule override" (FileSet.ScheduleCadence) is the actual per-set
//     cadence, hidden behind a button labelled "Set override".
//   - "Backup Everything" lives on a different card entirely and still reaches
//     into the same sets.
//
// A hint text can describe those rules; it cannot say what the current
// combination does. This does, and it does it by reading the SAME inputs the
// scheduler reads, in the same order, so the sentence on screen cannot drift
// from the job that runs. Every branch below names the code it mirrors.
//
// Deliberately a pure function of (set, settings): no store, no clock, no cron,
// so each outcome is a table test.
type EffectiveSchedule struct {
	// Kind is one of the Effective* constants below.
	Kind string `json:"kind"`
	// Spec is the cadence string that drives the primary run ("" for none).
	// It is the raw user-facing cadence ("weekly mon 02:00"), not a cron spec,
	// so the interface can render it with the formatter it already has.
	Spec string `json:"spec"`
	// AlsoSpec is the SECOND cadence, set only for EffectiveBoth: the run that
	// duplicates the first one.
	AlsoSpec string `json:"alsoSpec"`
}

const (
	// EffectiveNone: nothing backs this set up automatically. Either the Folders
	// domain is off, or "Include in schedule" is off, or the set's own override
	// is the literal "off".
	EffectiveNone = "none"
	// EffectiveOwn: the set has its own per-item entry on its own cadence, and is
	// excluded from both the domain run and Backup Everything.
	EffectiveOwn = "own"
	// EffectiveDomain: the set follows the Folders domain schedule, and Backup
	// Everything is not also covering it.
	EffectiveDomain = "domain"
	// EffectiveEverything: the Folders domain schedule is off, so the set is
	// backed up only as part of the Backup Everything pass. This is the shape
	// #199 was built for and the one manilx wanted.
	EffectiveEverything = "everything"
	// EffectiveBoth: the Folders domain schedule AND Backup Everything both cover
	// this set, so it is backed up twice. Legal, occasionally intended, and the
	// state manilx was in without knowing it.
	EffectiveBoth = "both"
)

// EffectiveFileSetSchedule computes the outcome for one set.
func EffectiveFileSetSchedule(fs store.FileSet, s store.Settings) EffectiveSchedule {
	// The Folders domain toggle gates the domain job (`off: !settings.FilesEnabled`
	// in registerJobs) and the files leg of Backup Everything (the {"files",
	// settings.FilesEnabled, ...} row in everythingRun). With it off, no path
	// reaches a file set.
	if !s.FilesEnabled {
		return EffectiveSchedule{Kind: EffectiveNone}
	}
	// Enabled is checked by all three: registerPerItemEntries skips !fs.Enabled,
	// RunFilesJob skips it, and everythingRunFiles skips it. This is the branch
	// whose label lies, so it is the one worth naming plainly on screen.
	if !fs.Enabled {
		return EffectiveSchedule{Kind: EffectiveNone}
	}
	if s.PerItemSchedules {
		switch cls := classifyItemOverride(fs.ScheduleCadence); {
		case cls.ownEntry:
			// Its own entry, and DomainRunFileSets drops it from the domain run
			// and from Backup Everything. Report the override the user typed,
			// not cls.Spec: cls.Spec is the compiled cron expression.
			return EffectiveSchedule{Kind: EffectiveOwn, Spec: fs.ScheduleCadence}
		case !cls.inDomainRun:
			// A literal "off" override: no entry, and out of every run.
			return EffectiveSchedule{Kind: EffectiveNone}
		}
	}
	// From here the set follows the domain default, so what covers it is
	// whichever of the two schedules is switched on.
	domain := cadenceRuns(s.FilesSchedule)
	everything := cadenceRuns(s.EverythingSchedule)
	switch {
	case domain && everything:
		return EffectiveSchedule{Kind: EffectiveBoth, Spec: s.FilesSchedule, AlsoSpec: s.EverythingSchedule}
	case domain:
		return EffectiveSchedule{Kind: EffectiveDomain, Spec: s.FilesSchedule}
	case everything:
		return EffectiveSchedule{Kind: EffectiveEverything, Spec: s.EverythingSchedule}
	default:
		return EffectiveSchedule{Kind: EffectiveNone}
	}
}

// cadenceRuns reports whether a cadence string would ever fire. An unparseable
// cadence counts as not running, which matches registerJobs: it logs and skips.
func cadenceRuns(cadence string) bool {
	cad, err := ParseCadence(cadence)
	if err != nil {
		return false
	}
	return cad.Enabled
}
