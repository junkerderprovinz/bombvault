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

// EffectiveContainerSchedule answers the same question for one container, and
// EffectiveVMSchedule for one VM. Both delegate to effectiveItemSchedule, so
// the three domains can never drift into answering it differently.
//
// They exist because the question has no other answer in the tree: file sets
// had this helper from #199, containers and VMs never did, and "is this item
// actually backed up by anything" is exactly what a coverage report has to get
// right. Getting it wrong in the safe-looking direction is the dangerous one,
// because it reports something as protected while nothing backs it up.
func EffectiveContainerSchedule(t store.Target, s store.Settings) EffectiveSchedule {
	return effectiveItemSchedule(s.ContainersEnabled, t.IncludeInSchedule, t.ScheduleCadence, s.ContainersSchedule, s)
}

// EffectiveVMSchedule computes the outcome for one VM.
func EffectiveVMSchedule(v store.VMTarget, s store.Settings) EffectiveSchedule {
	return effectiveItemSchedule(s.VMsEnabled, v.IncludeInSchedule, v.ScheduleCadence, s.VMsSchedule, s)
}

// effectiveItemSchedule is the shared body: the same branches, in the same
// order, reading the same inputs the scheduler reads. See
// EffectiveFileSetSchedule below for what each branch mirrors and why.
func effectiveItemSchedule(domainEnabled, included bool, override, domainSchedule string, s store.Settings) EffectiveSchedule {
	if !domainEnabled || !included {
		return EffectiveSchedule{Kind: EffectiveNone}
	}
	if s.PerItemSchedules {
		switch cls := classifyItemOverride(override); {
		case cls.ownEntry:
			return EffectiveSchedule{Kind: EffectiveOwn, Spec: override}
		case !cls.inDomainRun:
			return EffectiveSchedule{Kind: EffectiveNone}
		}
	}
	domain := cadenceRuns(domainSchedule)
	everything := cadenceRuns(s.EverythingSchedule)
	switch {
	case domain && everything:
		return EffectiveSchedule{Kind: EffectiveBoth, Spec: domainSchedule, AlsoSpec: s.EverythingSchedule}
	case domain:
		return EffectiveSchedule{Kind: EffectiveDomain, Spec: domainSchedule}
	case everything:
		return EffectiveSchedule{Kind: EffectiveEverything, Spec: s.EverythingSchedule}
	default:
		return EffectiveSchedule{Kind: EffectiveNone}
	}
}

// EffectiveFileSetSchedule computes the outcome for one set.
func EffectiveFileSetSchedule(fs store.FileSet, s store.Settings) EffectiveSchedule {
	// The Folders domain toggle gates the domain job (`off: !settings.FilesEnabled`
	// in registerJobs) and the files leg of Backup Everything (the {"files",
	// settings.FilesEnabled, ...} row in everythingRun). With it off, no path
	// reaches a file set.
	// Enabled is checked by all three: registerPerItemEntries skips !fs.Enabled,
	// RunFilesJob skips it, and everythingRunFiles skips it. This is the branch
	// whose label lies, so it is the one worth naming plainly on screen.
	//
	// The body moved to effectiveItemSchedule when containers and VMs gained the
	// same helper. Sharing it is the point: three domains answering "what
	// actually happens to this item" with three copies of the same branches is
	// three places for them to drift.
	return effectiveItemSchedule(s.FilesEnabled, fs.Enabled, fs.ScheduleCadence, s.FilesSchedule, s)
}

// PausedByOverride reports whether an item's own schedule override switches its
// backups off. EffectiveSchedule collapses that into EffectiveNone together
// with a disabled domain and an excluded item, so the two cases need a reading
// of their own where they have to be told apart.
func PausedByOverride(override string) bool {
	cls := classifyItemOverride(override)
	return !cls.ownEntry && !cls.inDomainRun
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
