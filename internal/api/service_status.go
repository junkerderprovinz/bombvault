package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// aggregateTamper folds a domain's off-site tamper verdicts worst-of across
// its destinations for the ransomware scorecard: had is true only when every
// destination has a recorded verdict, protected only when every one refused
// the delete, and at is the oldest verdict timestamp, so the least recently
// checked destination drives the overdue judgement. A single-destination
// domain reads exactly LatestTamperTest(domain).
func (s *Service) aggregateTamper(domain string) (had, protected bool, at int64) {
	targets := s.offsiteTargetsFor(domain)
	if len(targets) <= 1 {
		tt, found, err := s.store.LatestTamperTest(domain)
		if err != nil || !found {
			return false, false, 0
		}
		return true, tt.Protected, tt.At
	}
	protected = true
	for _, t := range targets {
		tt, found, err := s.store.LatestTamperTestForTarget(domain, t.ID)
		if err != nil || !found {
			return false, false, 0 // an untested destination → no protected claim
		}
		had = true
		if !tt.Protected {
			protected = false
		}
		if at == 0 || tt.At < at {
			at = tt.At
		}
	}
	return had, protected, at
}

// aggregateReplicationCurrency folds a domain's last-successful-replication
// currency worst-of across its off-site destinations: ok only when every
// destination has landed a successful copy, and at is the oldest of those,
// since the least recently replicated destination sets the domain's
// freshness. A single-destination domain reads exactly
// LatestSuccessfulOffsiteRun(domain).
func (s *Service) aggregateReplicationCurrency(domain string) (at int64, ok bool) {
	targets := s.offsiteTargetsFor(domain)
	if len(targets) <= 1 {
		run, found, err := s.store.LatestSuccessfulOffsiteRun(domain)
		if err != nil || !found {
			return 0, false
		}
		return run.StartedAt, true
	}
	for _, t := range targets {
		run, found, err := s.store.LatestSuccessfulOffsiteRunForTarget(domain, t.ID)
		if err != nil || !found {
			return 0, false // a never-replicated destination → domain is not current
		}
		if at == 0 || run.StartedAt < at {
			at = run.StartedAt
		}
	}
	return at, true
}

// DomainStatusEntry is the per-domain RPO (protection) status: whether a
// domain's backups are current relative to its schedule. It drives the
// dashboard's green/amber/red "are my backups current?" indicator.
type DomainStatusEntry struct {
	Domain   string `json:"domain"`   // "containers" | "vms" | "flash" | "config" | "files"
	Enabled  bool   `json:"enabled"`  // domain switched on in Settings
	Schedule string `json:"schedule"` // the domain's own cadence string (e.g. "daily 02:30")
	// CoveredBy carries the "Backup Everything" cadence when that pass is the
	// only thing backing this domain up: its own schedule is off, but the pass
	// includes it. It is empty whenever the domain has a schedule of its own, so
	// a client can tell "the domain's cadence" from "covered by the whole-server
	// pass". Without it a domain the pass backs up nightly would read "Not
	// scheduled" (#177).
	CoveredBy     string `json:"coveredBy"`
	LastSuccess   int64  `json:"lastSuccess"`   // unix time of the last successful backup, 0 = none
	PeriodSeconds int64  `json:"periodSeconds"` // expected RPO window in seconds, 0 = no expectation
	Status        string `json:"status"`        // "off" | "never" | "overdue" | "warn" | "ok"
	// LastVerified is the unix time of the last local restore-verification drill
	// (`restic check --read-data-subset`), 0 = never verified. LastVerifiedOK is
	// its outcome. These drive the dashboard's "last verified restorable" badge
	// without an extra round-trip.
	LastVerified   int64 `json:"lastVerified"`
	LastVerifiedOK bool  `json:"lastVerifiedOK"`
	// VerifiedDetail and DrillDetail carry the scrubbed failure reason of the
	// last local subset drill and the last off-site DR drill, so the dashboard
	// can show why and which check failed (#30). Both are "" on success.
	VerifiedDetail string `json:"verifiedDetail"`
	DrillDetail    string `json:"drillDetail"`

	// Ransomware-protection scorecard facts: whether the domain has an off-site
	// copy, whether it is flagged append-only (immutable), and the age-stamped
	// outcomes of the three protection checks (the active tamper test, the
	// off-site replication and the off-site DR drill). Protection is the
	// red/amber/green aggregate (see protectionLevel); it is "" for a disabled
	// domain, which the dashboard then leaves blank. These extend /api/status so
	// the dashboard card needs no second round-trip.
	OffsiteConfigured bool `json:"offsiteConfigured"`
	// OffPremisesCovered: every item of the domain lives somewhere that is a site
	// of its own, so the dashboard does not claim there is no copy off the premises.
	OffPremisesCovered bool  `json:"offPremisesCovered"`
	OffsiteImmutable   bool  `json:"offsiteImmutable"`
	LastTamperAt       int64 `json:"lastTamperAt"`
	LastTamperOK       bool  `json:"lastTamperOK"`
	LastReplicationAt  int64 `json:"lastReplicationAt"`
	LastReplicationOK  bool  `json:"lastReplicationOK"`
	LastDRDrillAt      int64 `json:"lastDrDrillAt"`
	LastDRDrillOK      bool  `json:"lastDrDrillOK"`
	// LastOffsiteSubsetAt and LastOffsiteSubsetOK stamp the latest off-site
	// subset drill (`restic check --read-data-subset` against the off-site repo),
	// the cheaper integrity check every domain, VMs included, can run alongside
	// the DR sandbox-restore drill. They drive the dashboard's "off-site
	// verified" badge (#63), independent of the DR fields above.
	LastOffsiteSubsetAt int64 `json:"lastOffsiteSubsetAt"`
	LastOffsiteSubsetOK bool  `json:"lastOffsiteSubsetOK"`
	// OffsiteDrillScheduled is true only when the scheduler runs an off-site DR
	// drill for this domain (DrillsEnabled and OffsiteDrillsEnabled set, and an
	// off-site repo configured). When it is false but the domain has an off-site
	// repo, the dashboard shows a muted "manual only" pill instead of a red
	// drFailed (#37).
	OffsiteDrillScheduled bool   `json:"offsiteDrillScheduled"`
	Protection            string `json:"protection"` // "" (disabled) | "red" | "amber" | "green"

	// Per-check states derived from the same inputs Protection aggregates (see
	// protectionChecks), so the dashboard card renders each checklist row as a
	// pure function of the backend and never contradicts the chip. EncryptionOn
	// and PruneStrategySet are the two config-level facts the card also renders,
	// served here so it needs no separate /api/settings round-trip.
	TamperState      string `json:"tamperState"`      // "" | "never" | "failed" | "stale" | "ok"
	ReplicationState string `json:"replicationState"` // "" | "never" | "overdue" | "ok" | "paused"
	DrillState       string `json:"drillState"`       // "" | "never" | "failed" | "overdue" | "ok"
	EncryptionOn     bool   `json:"encryptionOn"`     // repo encryption is enabled
	PruneStrategySet bool   `json:"pruneStrategySet"` // an off-site retention strategy is configured
}

// rpoStatus is the pure status decision from the inputs, so it can be unit-tested
// exhaustively without a store. scheduled is true when the domain is enabled and
// has an RPO expectation (periodSeconds > 0):
//
//   - "off"     scheduled is false (disabled / no schedule / unparseable period)
//   - "never"   scheduled but no successful backup yet (lastSuccess == 0)
//   - "overdue" age > period*2
//   - "warn"    age > period   (and <= period*2)
//   - "ok"      otherwise
func rpoStatus(nowUnix, lastSuccess, periodSeconds int64, scheduled bool) string {
	if !scheduled || periodSeconds <= 0 {
		return "off"
	}
	if lastSuccess <= 0 {
		return "never"
	}
	age := nowUnix - lastSuccess
	switch {
	case age > periodSeconds*2:
		return "overdue"
	case age > periodSeconds:
		return "warn"
	default:
		return "ok"
	}
}

// cadencePeriodSeconds parses a cadence string to its expected period in seconds,
// returning 0 for an empty or unparseable cadence (i.e. "no expectation").
func cadencePeriodSeconds(cadence string) int64 {
	if strings.TrimSpace(cadence) == "" {
		return 0
	}
	cad, err := schedule.ParseCadence(cadence)
	if err != nil {
		return 0
	}
	return cad.PeriodSeconds()
}

// domainCoverage answers how often one domain is backed up, and by what,
// given its own cadence and the "Backup Everything" cadence. It returns the
// RPO window in seconds and, when the pass is the only thing covering the
// domain, that pass's cadence string.
//
// Backup Everything runs the domains as a sixth, independent pseudo-domain,
// so a user can leave every per-domain schedule off and let the pass do the
// work (#177). The per-domain cadence alone then answers zero: the status
// shows "Not scheduled" for a domain backed up nightly, and the overdue
// watchdog, whose job is to notice when backups stop, falls silent for it.
//
// When both are scheduled the window is the shorter of the two, since
// whichever fires more often bounds how stale a backup can get. The pass
// only touches enabled domains, so the caller's own enabled check still
// governs.
func domainCoverage(ownCadence, everythingCadence string) (period int64, coveredBy string) {
	own := cadencePeriodSeconds(ownCadence)
	every := cadencePeriodSeconds(everythingCadence)
	switch {
	case own == 0 && every == 0:
		return 0, ""
	case own == 0:
		return every, strings.TrimSpace(everythingCadence)
	case every == 0 || own <= every:
		return own, ""
	default:
		return every, ""
	}
}

// protInputs carries the facts protectionLevel aggregates, so the decision is a
// pure function of its inputs (unit-testable without a store) and mirrors
// rpoStatus's shape.
type protInputs struct {
	enabled           bool
	offsiteConfigured bool
	offsiteImmutable  bool
	hadTamper         bool
	lastTamperOK      bool
	lastTamperAt      int64
	tamperPeriod      int64 // seconds; 0 = no/invalid tamper schedule
	lastReplicationAt int64 // last successful replication (currency source)
	offsitePeriod     int64 // seconds; 0 = replication coupled to each backup (no own schedule)
	lastBackupAt      int64 // last successful backup (coupled-replication currency basis)
	backupPeriod      int64 // seconds; the domain's backup RPO period (coupled-grace basis)
	lastDRDrillAt     int64
	lastDRDrillOK     bool             // outcome of the latest DR drill (only meaningful when lastDRDrillAt != 0)
	drillPeriod       int64            // seconds; 0 = no drill schedule
	paused            bool             // replication waits for the placement default to be confirmed
	byTarget          bool             // copy rules decide per target; targets replaces the domain-wide pair above
	targets           []targetCurrency // the enabled targets items are copied to
}

// replicationState decides the off-site replication currency (""/never/overdue/ok)
// from the same inputs protectionLevel and protectionChecks share, so the chip and
// the checklist row can never disagree.
//
//   - No off-site configured → "" (there is no replication to be current; the
//     missing-off-site case is handled as red by protectionLevel).
//   - Decoupled (offsitePeriod>0): the standard rpoStatus against the off-site's
//     own schedule, using the last successful replication.
//   - Coupled (offsitePeriod==0, the default): replication rides each backup, so
//     the claim is "the last successful backup has a corresponding successful
//     off-site copy". It goes overdue only once the gap between the last backup and
//     the last successful replication exceeds a grace of 2× the backup period
//     (conservative: a backup replicating shortly after is fine; a never-replicated
//     backup is flagged only once it has sat unreplicated beyond the grace). Amber,
//     never red.
//
// A paused domain reports "paused". With copy rules each enabled target is judged
// by the items copied there, and the worst of them counts; a target no item is
// copied to has no claim to make.
func replicationState(now int64, in protInputs) string {
	if !in.offsiteConfigured {
		return ""
	}
	if in.paused {
		return "paused"
	}
	if !in.byTarget {
		return targetReplicationState(now, in, in.lastBackupAt, in.lastReplicationAt)
	}
	worst := ""
	for _, t := range in.targets {
		if st := targetReplicationState(now, in, t.lastBackupAt, t.lastReplicationAt); replicationRank(st) > replicationRank(worst) {
			worst = st
		}
	}
	return worst
}

// targetReplicationState is the currency of one target, or of the domain as a
// whole, from the last successful backup and the last successful copy.
func targetReplicationState(now int64, in protInputs, lastBackupAt, lastReplicationAt int64) string {
	if in.offsitePeriod > 0 {
		switch rpoStatus(now, lastReplicationAt, in.offsitePeriod, true) {
		case "overdue":
			return "overdue"
		case "never":
			return "never"
		default:
			return "ok"
		}
	}
	// Coupled path: only meaningful once a backup exists and there is an RPO basis.
	if lastBackupAt == 0 || in.backupPeriod <= 0 {
		return ""
	}
	grace := in.backupPeriod * 2
	if lastReplicationAt == 0 {
		// Never replicated: overdue only once the backup has sat unreplicated > grace
		// (a just-made first backup replicating shortly after must not instantly flag).
		if now-lastBackupAt > grace {
			return "overdue"
		}
		return "ok"
	}
	if lastReplicationAt < lastBackupAt && lastBackupAt-lastReplicationAt > grace {
		return "overdue"
	}
	return "ok"
}

// replicationRank orders the states from nothing to claim up to overdue.
func replicationRank(state string) int {
	switch state {
	case "overdue":
		return 3
	case "never":
		return 2
	case "ok":
		return 1
	}
	return 0
}

// protectionLevel aggregates a domain's ransomware-protection posture into a
// red/amber/green chip. The far side enforces immutability, so this never
// goes green on configuration claims alone:
//
//   - ""    the domain is disabled; the dashboard shows nothing for it.
//   - red   the domain is enabled but has no off-site copy at all, or the
//     off-site is flagged immutable yet the append-only guarantee is
//     unproven: the tamper test is missing, last failed, or is stale (older
//     than 2× its schedule period). A non-immutable off-site makes no
//     append-only claim, so a missing tamper test does not make it red.
//   - amber protection exists but a scheduled time-check is overdue by the
//     same period-doubling rule backups use (rpoStatus "overdue"): the
//     off-site replication (only with a decoupled off-site schedule) or the
//     off-site DR drill (only with a drill schedule). Also amber when the
//     latest scheduled DR drill failed: the chip can't read green over the
//     red "failed" drill row, but other protections may still be fine.
//   - green otherwise.
//
// An off-site copy that is simply not flagged immutable stays green here: it
// is a real copy, and whether the far side can enforce append-only is a
// property of the destination the user picked, not a failed check. The
// dashboard's scorecard still marks that row amber rather than grey, so the
// gap is visible without the chip nagging about a chosen setup; it is the
// one place a row's colour and this chip differ (see appendOnlyRow in
// Dashboard.tsx).
func protectionLevel(now int64, in protInputs) string {
	if !in.enabled {
		return "" // disabled domains carry no protection posture
	}
	if !in.offsiteConfigured {
		return "red" // enabled but no off-site copy, so unprotected
	}
	if in.offsiteImmutable {
		tamperStale := in.tamperPeriod > 0 && now-in.lastTamperAt > in.tamperPeriod*2
		if !in.hadTamper || !in.lastTamperOK || tamperStale {
			return "red" // an append-only claim that cannot currently be proven
		}
	}
	// Replication currency: overdue is amber. Decoupled off-sites use their own
	// schedule; coupled (default) off-sites are checked against the last backup
	// with a conservative grace (see replicationState), so off-site health shows
	// in the configuration most users run. A paused replication copies nothing
	// at all, which is amber as well.
	if st := replicationState(now, in); st == "overdue" || st == "paused" {
		return "amber"
	}
	// A recorded DR drill that failed downgrades the chip to amber, never green
	// over a red row. The guard matches protectionChecks' "failed" branch (a
	// drill schedule is set and the latest recorded drill failed), so the chip
	// and the scorecard row never disagree on a failed drill.
	if in.drillPeriod > 0 && in.lastDRDrillAt != 0 && !in.lastDRDrillOK {
		return "amber"
	}
	if rpoStatus(now, in.lastDRDrillAt, in.drillPeriod, in.drillPeriod > 0) == "overdue" {
		return "amber"
	}
	return "green"
}

// protChecks is the per-check state the ransomware scorecard renders. Tamper
// and Replication derive from the same protInputs protectionLevel
// aggregates, so those rows never contradict the chip. Drill also honors the
// latest DR drill's outcome (a failed drill reads "failed", red) to agree
// with the off-site "proven restorable" pill; protectionLevel downgrades
// that case to amber, so a red Drill row never sits next to a green chip. An
// empty state means the check makes no claim and is rendered muted, not as
// a failure.
type protChecks struct {
	Tamper      string // "" | "never" | "failed" | "stale" | "ok"
	Replication string // "" | "never" | "overdue" | "ok" | "paused"
	Drill       string // "" | "never" | "failed" | "overdue" | "ok"
}

// protectionChecks mirrors protectionLevel for Tamper and Replication and
// layers the DR-drill outcome on top of currency for Drill:
//
//   - Tamper ∈ {never,failed,stale} is precisely the immutable branch that
//     turns the chip red; a non-immutable off-site makes no append-only
//     claim, so "".
//   - Replication "overdue" is precisely the amber branch (rpoStatus
//     "overdue", the same period-doubling rule). "never" and "ok" stay
//     non-amber so they match a green chip.
//   - Drill mirrors that currency (never/overdue/ok) for a passed drill, but
//     a recorded DR drill that failed reads "failed" (red) regardless of
//     recency, so the row agrees with the off-site "proven restorable" pill
//     (lastDRDrillOK). protectionLevel downgrades this case to amber, so the
//     red row never sits next to a green chip.
//
// Replication does not surface a red "replication failed": nothing else
// consumes lastReplicationOK, so only its currency is mirrored.
func protectionChecks(now int64, in protInputs) protChecks {
	var c protChecks

	// Tamper (append-only): only an immutable off-site makes an append-only claim.
	switch {
	case !in.offsiteImmutable:
		c.Tamper = ""
	case !in.hadTamper:
		c.Tamper = "never"
	case !in.lastTamperOK:
		c.Tamper = "failed"
	case in.tamperPeriod > 0 && now-in.lastTamperAt > in.tamperPeriod*2:
		c.Tamper = "stale"
	default:
		c.Tamper = "ok"
	}

	// Replication currency: decoupled off-sites use their own schedule; coupled
	// (default) off-sites are checked against the last backup with a grace (see
	// replicationState). "" when there is nothing to claim yet.
	c.Replication = replicationState(now, in)

	// DR drill outcome and currency, only when a drill schedule is set. A
	// recorded DR drill that failed reads "failed" (a red row) regardless of
	// recency, so the row can't go green by currency while the off-site "proven
	// restorable" pill (lastDRDrillOK) reads red. "never" stays for no drill
	// yet; a passed drill keeps the overdue/ok currency logic.
	if in.drillPeriod > 0 {
		switch {
		case in.lastDRDrillAt != 0 && !in.lastDRDrillOK:
			c.Drill = "failed"
		default:
			switch rpoStatus(now, in.lastDRDrillAt, in.drillPeriod, true) {
			case "overdue":
				c.Drill = "overdue"
			case "never":
				c.Drill = "never"
			default:
				c.Drill = "ok"
			}
		}
	}

	return c
}

// DomainStatus returns the RPO (protection) status of each domain (containers,
// vms, flash, config, files): whether its backups are current relative to its
// schedule. The enabled flag + cadence come from Settings; the last successful
// backup time comes from the store's per-domain helpers.
func (s *Service) DomainStatus() ([]DomainStatusEntry, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	return s.domainStatusFrom(settings)
}

// domainStatusFrom is DomainStatus for a caller that already holds the settings
// row. Both status endpoints do: /api/status also serves the "Backup Everything"
// cadence from it, and /api/fleet/status has it from the token gate. Settings is
// an ~80-column row and nothing caches it, so handing it in saves each of them a
// second full read of the same row.
func (s *Service) domainStatusFrom(settings store.Settings) ([]DomainStatusEntry, error) {
	now := time.Now().Unix()

	domains := []struct {
		name     string
		enabled  bool
		schedule string
		lastFn   func() (time.Time, error)
	}{
		{"containers", settings.ContainersEnabled, settings.ContainersSchedule, s.store.LastSuccessfulContainerBackup},
		{"vms", settings.VMsEnabled, settings.VMsSchedule, s.store.LastSuccessfulVMBackup},
		{"flash", settings.FlashEnabled, settings.FlashSchedule, s.store.LastSuccessfulFlashBackup},
		{"config", settings.ConfigEnabled, settings.ConfigSchedule, s.store.LastSuccessfulConfigBackup},
		{"files", settings.FilesEnabled, settings.FilesSchedule, s.store.LastSuccessfulFilesBackup},
	}

	out := make([]DomainStatusEntry, 0, len(domains))
	for _, d := range domains {
		last, lErr := d.lastFn()
		if lErr != nil {
			return nil, fmt.Errorf("domain %s last-success: %w", d.name, lErr)
		}
		var lastUnix int64
		if !last.IsZero() {
			lastUnix = last.Unix()
		}

		// A period is only meaningful for an enabled domain that something backs up
		// on a cadence, its own or the "Backup Everything" pass (see
		// domainCoverage). An unparseable cadence (the settings PUT validates)
		// collapses to period 0, "off".
		period, coveredBy := domainCoverage(d.schedule, settings.EverythingSchedule)
		scheduled := d.enabled && period > 0

		// The latest local restore-verification drill drives the "last verified
		// restorable" badge. Best-effort: a read error leaves the badge at "never"
		// (0 / false) rather than failing the whole status query.
		var lastVerified int64
		var lastVerifiedOK bool
		var verifiedDetail string
		if drill, found, dErr := s.store.LatestRestoreDrill(d.name, "local"); dErr == nil && found {
			lastVerified = drill.At
			lastVerifiedOK = drill.OK
			verifiedDetail = drill.Detail
		}

		// Ransomware-protection scorecard facts. All reads are best-effort: a store
		// error leaves the relevant fact at its zero value (a missing check), which
		// the aggregate then treats conservatively rather than failing the query.
		offsiteConfigured := s.offsiteRepoFor(d.name, settings) != ""
		var offPremisesCovered bool
		if validPlacementDomain(d.name) {
			copied, covered, cErr := s.placementCoverage(settings, d.name)
			if cErr != nil {
				log.Printf("api: status %s: placement could not be read, off-site stays as configured: %v", d.name, cErr) //nolint:gosec // G706: domain is a fixed literal
			} else {
				offsiteConfigured = offsiteConfigured && copied
				offPremisesCovered = covered
			}
		}
		offsiteImmutable := offsiteImmutableFor(d.name, settings)

		// Tamper facts are aggregated worst-of across the domain's off-site
		// destinations (protected only when every destination is, currency from the
		// oldest). A single-destination domain reads exactly LatestTamperTest(domain).
		hadTamper, lastTamperOK, lastTamperAt := s.aggregateTamper(d.name)
		// Currency uses the last successful replication, as backups use their last
		// success: a replication that keeps failing then reads stale, overdue and
		// amber instead of staying fresh off a failed attempt's timestamp.
		// Aggregated worst-of (the oldest successful copy across destinations);
		// LatestSuccessfulOffsiteRun(domain) for a single destination.
		lastReplicationAt, lastReplicationOK := s.aggregateReplicationCurrency(d.name)
		var lastDRDrillAt int64
		var lastDRDrillOK bool
		var drDetail string
		if dr, found, drErr := s.store.LatestRestoreDrillKind(d.name, "offsite", "dr"); drErr == nil && found {
			lastDRDrillAt = dr.At
			lastDRDrillOK = dr.OK
			drDetail = dr.Detail
		}
		// The latest off-site subset drill (an integrity check against the off-site
		// repo) drives the dashboard's "off-site verified" badge (#63). It is the
		// only off-site drill available for VMs (DR restores are refused for them),
		// so it is read for every domain. Best-effort like the reads above.
		var lastOffsiteSubsetAt int64
		var lastOffsiteSubsetOK bool
		if sub, found, subErr := s.store.LatestRestoreDrillKind(d.name, "offsite", "subset"); subErr == nil && found {
			lastOffsiteSubsetAt = sub.At
			lastOffsiteSubsetOK = sub.OK
		}

		// The DR-drill currency only makes a claim when the scheduler runs off-site
		// DR drills (DrillsEnabled and OffsiteDrillsEnabled); otherwise a stale
		// lastDRDrillAt must not read overdue. Opting out of the scheduled off-site
		// DR drill (#37) leaves drillPeriod at 0, so DrillState is "" (muted) and
		// protectionLevel ignores DR, whose branches both require drillPeriod>0.
		var drillPeriod int64
		if settings.DrillsEnabled && settings.OffsiteDrillsEnabled {
			drillPeriod = cadencePeriodSeconds(settings.DrillsSchedule)
		}

		in := protInputs{
			enabled:           d.enabled,
			offsiteConfigured: offsiteConfigured,
			offsiteImmutable:  offsiteImmutable,
			hadTamper:         hadTamper,
			lastTamperOK:      lastTamperOK,
			lastTamperAt:      lastTamperAt,
			tamperPeriod:      cadencePeriodSeconds(settings.TamperTestSchedule),
			lastReplicationAt: lastReplicationAt,
			offsitePeriod:     cadencePeriodSeconds(s.offsiteScheduleFor(d.name, settings)),
			lastBackupAt:      lastUnix,
			backupPeriod:      period,
			lastDRDrillAt:     lastDRDrillAt,
			lastDRDrillOK:     lastDRDrillOK,
			drillPeriod:       drillPeriod,
		}
		in.paused, in.byTarget, in.targets = s.placementCurrency(settings, d.name)
		// The chip (protection) and each row (checks) derive from the same
		// protInputs. The Tamper and Replication rows mirror the chip's red and
		// amber branches exactly. The Drill row also honors the latest drill's
		// outcome (a failed drill reads a red "failed"), and protectionLevel
		// downgrades a failed drill to amber under the same guard, so no row can
		// contradict the chip.
		protection := protectionLevel(now, in)
		checks := protectionChecks(now, in)

		// An off-site retention strategy is "configured" when the far side prunes
		// (immutable), a growth budget is set, or an off-site keep policy is set.
		pruneStrategySet := offsiteImmutable ||
			settings.OffsiteGrowthBudgetGB > 0 ||
			settings.OffsiteRetentionKeepLast > 0 ||
			settings.OffsiteRetentionKeepDaily > 0 ||
			settings.OffsiteRetentionKeepWeekly > 0 ||
			settings.OffsiteRetentionKeepMonthly > 0

		out = append(out, DomainStatusEntry{
			Domain:                d.name,
			Enabled:               d.enabled,
			Schedule:              d.schedule,
			CoveredBy:             coveredBy,
			LastSuccess:           lastUnix,
			PeriodSeconds:         period,
			Status:                rpoStatus(now, lastUnix, period, scheduled),
			LastVerified:          lastVerified,
			LastVerifiedOK:        lastVerifiedOK,
			VerifiedDetail:        verifiedDetail,
			OffsiteConfigured:     offsiteConfigured,
			OffPremisesCovered:    offPremisesCovered,
			OffsiteImmutable:      offsiteImmutable,
			LastTamperAt:          lastTamperAt,
			LastTamperOK:          lastTamperOK,
			LastReplicationAt:     lastReplicationAt,
			LastReplicationOK:     lastReplicationOK,
			LastDRDrillAt:         lastDRDrillAt,
			LastDRDrillOK:         lastDRDrillOK,
			LastOffsiteSubsetAt:   lastOffsiteSubsetAt,
			LastOffsiteSubsetOK:   lastOffsiteSubsetOK,
			OffsiteDrillScheduled: settings.DrillsEnabled && settings.OffsiteDrillsEnabled && offsiteConfigured,
			DrillDetail:           drDetail,
			Protection:            protection,
			TamperState:           checks.Tamper,
			ReplicationState:      checks.Replication,
			DrillState:            checks.Drill,
			EncryptionOn:          settings.EncryptionEnabled,
			PruneStrategySet:      pruneStrategySet,
		})
	}
	return out, nil
}

// DayStat is the per-domain backup outcome count for a single calendar day.
type DayStat struct {
	OK     int `json:"ok"`
	Failed int `json:"failed"`
}

// HistoryDay is one calendar day's backup outcomes split by domain, for the
// dashboard's GitHub-contributions-style backup-health heatmap.
type HistoryDay struct {
	Date       string  `json:"date"` // local YYYY-MM-DD
	Containers DayStat `json:"containers"`
	VMs        DayStat `json:"vms"`
	Flash      DayStat `json:"flash"`
	Config     DayStat `json:"config"`
	Files      DayStat `json:"files"`
}

// runDomains is the target_id → domain map ("container" | "vm" | "flash" |
// "config" | "files") used to attribute each run to its domain, the same
// mapping handleRuns uses: container targets, VM targets, file sets, and the
// singleton flash/config ids. Best-effort: an unknown id (e.g. a deleted
// target) maps to "" and is ignored by the bucketer.
func (s *Service) runDomains() map[string]string {
	domain := map[string]string{store.FlashTargetID: "flash", store.ConfigTargetID: "config"}
	if cts, err := s.store.ListTargets(); err == nil {
		for _, t := range cts {
			domain[t.ID] = "container"
		}
	}
	if vts, err := s.store.ListVMTargets(); err == nil {
		for _, t := range vts {
			domain[t.ID] = "vm"
		}
	}
	if fss, err := s.store.ListFileSets(); err == nil {
		for _, fs := range fss {
			domain[fs.ID] = "files"
		}
	}
	return domain
}

// bucketRunsByDay is the pure heatmap-bucketing core: it produces one HistoryDay
// for every local calendar day in [startUnix, endUnix] (ascending), tallying
// each backup run's success/failed outcome into its domain via the target_id →
// domain map. Days with no runs come back with zeros so the frontend gets a
// contiguous grid. Non-backup kinds and "running" runs are ignored, as are runs
// whose target maps to no known domain. Kept free of the store/clock so it can
// be unit-tested directly.
func bucketRunsByDay(runs []store.Run, domain map[string]string, startUnix, endUnix int64) []HistoryDay {
	// Map each local day to its index in the output grid. Indices stay valid even
	// as the slice grows (unlike pointers into a slice that append may reallocate).
	idx := map[string]int{}
	start := time.Unix(startUnix, 0).Local()
	end := time.Unix(endUnix, 0).Local()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	out := make([]HistoryDay, 0)
	for d := startDay; !d.After(endDay); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		idx[date] = len(out)
		out = append(out, HistoryDay{Date: date})
	}

	for _, run := range runs {
		if run.Kind != "backup" {
			continue
		}
		dom := domain[run.TargetID]
		if dom == "" {
			continue // unknown / deleted target
		}
		date := time.Unix(run.StartedAt, 0).Local().Format("2006-01-02")
		i, ok := idx[date]
		if !ok {
			continue // outside the window; the query already bounds it
		}
		var stat *DayStat
		switch dom {
		case "container":
			stat = &out[i].Containers
		case "vm":
			stat = &out[i].VMs
		case "flash":
			stat = &out[i].Flash
		case "config":
			stat = &out[i].Config
		case "files":
			stat = &out[i].Files
		default:
			continue
		}
		switch run.Status {
		case "success":
			stat.OK++
		case "failed":
			stat.Failed++
		}
	}
	return out
}

// BackupHistory returns one HistoryDay per calendar day in the last `days` days
// (ascending, including empty days with zeros) for the dashboard heatmap. days
// is capped at 366. Runs are bucketed by local calendar day and by domain.
func (s *Service) BackupHistory(days int) ([]HistoryDay, error) {
	if days < 1 {
		days = 1
	}
	if days > 366 {
		days = 366
	}
	now := time.Now()
	since := now.AddDate(0, 0, -(days - 1))
	// Widen the store query to the start of the earliest day so a run early on the
	// first day isn't missed by an intra-day cutoff; the bucketer bounds the grid.
	startUnix := time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location()).Unix()
	runs, err := s.store.RunsSince(startUnix)
	if err != nil {
		return nil, fmt.Errorf("read runs: %w", err)
	}
	return bucketRunsByDay(runs, s.runDomains(), startUnix, now.Unix()), nil
}

// repoStatsMinInterval is the minimum age of the latest sample before a
// backup re-collects repo stats. Stats (two restic stats passes over the
// whole repo) are expensive, so once a day is plenty for a size/dedup trend,
// and a domain backed up many times an hour samples only once.
const repoStatsMinInterval = 20 * time.Hour

// CollectStats samples a domain's repository size for source ("local"/"offsite")
// and records it for the size/dedup trend. It is best-effort and idempotent: a
// missing or empty (zero-snapshot) repo records nothing and returns nil, so it
// never turns an otherwise-good backup into a failure. Any restic error is
// returned so the (throttled) caller can log it.
func (s *Service) CollectStats(ctx context.Context, domain, source string) error {
	settings, repo, err := s.domainRepoSource(domain, source)
	if err != nil {
		return err
	}
	// No repo yet (local not initialised) → nothing to measure, not an error.
	if localRepoMissing(repo) {
		return nil
	}
	mode := s.repoModeFor(settings, domain, source, repo)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	if len(snaps) == 0 {
		return nil // empty repo, nothing to measure
	}
	raw, err := s.engine.Stats(ctx, repo, "raw-data", mode)
	if err != nil {
		return err
	}
	restoreSize, err := s.engine.Stats(ctx, repo, "restore-size", mode)
	if err != nil {
		return err
	}
	return s.store.AddRepoStat(store.RepoStat{
		Domain:      domain,
		Source:      source,
		At:          time.Now().Unix(),
		RawSize:     raw.TotalSize,
		RestoreSize: restoreSize.TotalSize,
		Snapshots:   int64(len(snaps)),
	})
}

// claimStatsRun takes the in-flight slot for domain+source, reporting false when
// someone else already holds it. See statsMu's own comment for why this is a set
// rather than a singleflight.
func (s *Service) claimStatsRun(key string) bool {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	if s.statsRunning[key] {
		return false
	}
	if s.statsRunning == nil {
		s.statsRunning = map[string]bool{}
	}
	s.statsRunning[key] = true
	return true
}

// releaseStatsRun frees the slot. Deferred by every claimant, so a panic in a
// sampling goroutine cannot wedge sampling for the life of the process.
func (s *Service) releaseStatsRun(key string) {
	s.statsMu.Lock()
	delete(s.statsRunning, key)
	s.statsMu.Unlock()
}

// collectStatsGuarded runs one sample unless one is already running for this
// domain+source. The throttle is re-read after the slot is taken: a caller
// that missed the throttle and then waited while the holder finished and
// wrote its row would otherwise re-measure a repo that was just measured.
func (s *Service) collectStatsGuarded(ctx context.Context, domain, source string) error {
	key := domain + "/" + source
	if !s.claimStatsRun(key) {
		return nil
	}
	defer s.releaseStatsRun(key)
	if s.statsSampledRecently(domain, source) {
		return nil
	}
	return s.CollectStats(ctx, domain, source)
}

// statsSampledRecently reports whether domain+source already has a sample
// younger than repoStatsMinInterval. A read error counts as not recently
// sampled: sampling is best-effort and a broken read should not silently
// stop it forever.
func (s *Service) statsSampledRecently(domain, source string) bool {
	latest, found, err := s.store.LatestRepoStat(domain, source)
	if err != nil || !found {
		return false
	}
	return time.Since(time.Unix(latest.At, 0)) < repoStatsMinInterval
}

// RepoStats returns the recorded repo-size samples for a domain + source
// (ascending by time), a thin passthrough to the store.
func (s *Service) RepoStats(domain, source string, limit int) ([]store.RepoStat, error) {
	return s.store.ListRepoStats(domain, source, limit)
}

// maybeCollectStats samples a domain's local repo size after a successful
// backup, throttled to repoStatsMinInterval so frequent backups don't re-scan
// the repo each time. It never blocks or fails the backup: the work runs in a
// detached goroutine (request values kept, cancellation dropped, with its own
// timeout) and any error is only logged. Call this on each domain's success
// path.
func (s *Service) maybeCollectStats(ctx context.Context, domain string) {
	if s.statsSampledRecently(domain, "local") {
		return // sampled recently enough
	}
	// Detach from the request (keep its values) so the sampling survives the
	// handler returning, with a hard cap so a wedged restic can't leak a goroutine.
	bg := context.WithoutCancel(ctx)
	go func() {
		cctx, cancel := context.WithTimeout(bg, 5*time.Minute)
		defer cancel()
		// Guarded, not bare: the check above reads a row this work only writes at
		// the end, so it is the throttle that cannot see a sample in progress.
		if err := s.collectStatsGuarded(cctx, domain, "local"); err != nil {
			log.Printf("api: stats: %s: collect failed (backup is safe): %v", domain, err) //nolint:gosec // G706: domain is a fixed literal
		}
	}()
}

// collectStatsAfterItem is the per-item success hook: it samples after a single
// backup, and does nothing when that backup is one item of a round.
//
// A round measures once, at the end, next to the batched prune and the
// batched off-site copy, keyed on the same "part of a round" flag those two
// use. Measuring per item piles up concurrent restic processes (see statsMu)
// and measures the wrong thing: a repo halfway through being written to,
// recorded N times a night into a series the Storage card plots as one point
// per day.
func (s *Service) collectStatsAfterItem(ctx context.Context, domain string) {
	if bulkReplicateSuppressed(ctx) {
		return
	}
	s.maybeCollectStats(ctx, domain)
}

// MaybeCollectStatsAfterBulk is the round's own sampling point, for the
// scheduler to call once a domain's loop is done. It is exported, like
// PruneAfterBulk and ReplicateOffsiteAfterBulk, because the after-bulk hooks
// are wired from main. It stays throttled and guarded, so a domain whose
// items run on their own per-item cadences cannot turn it into per-item
// sampling.
func (s *Service) MaybeCollectStatsAfterBulk(ctx context.Context, domain string) {
	s.maybeCollectStats(ctx, domain)
}

// CollectStatsAsync samples a domain+source repo size in the background (detached,
// throttled to repoStatsMinInterval). Used to populate the Storage card for repos
// that already have backups but no sample yet (e.g. on upgrade, or before the next
// scheduled backup). Best-effort; errors are only logged. domain/source are always
// from a fixed whitelist (handler-validated or literal).
func (s *Service) CollectStatsAsync(domain, source string) {
	source = collectStatsSource(source)
	if latest, found, err := s.store.LatestRepoStat(domain, source); err == nil && found &&
		time.Since(time.Unix(latest.At, 0)) < repoStatsMinInterval {
		return // sampled recently enough
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		// Guarded: this is the one sampling path a browser can drive without bound.
		// The Storage card asks for four domains on every mount, and the throttle
		// above cannot fire while a repo has no sample yet (found=false), so every
		// tab and remount would start another fan-out.
		if err := s.collectStatsGuarded(ctx, domain, source); err != nil {
			log.Printf("api: stats: %s/%s: async collect failed: %v", domain, source, err) //nolint:gosec // G706: domain/source are fixed-whitelist values
		}
	}()
}

// collectStatsSource normalises a stats source: any off-site source, bare
// "offsite" (primary target) or the per-target "offsite:<id>" form, samples
// the off-site repo and passes through unchanged; everything else collapses
// to "local". isOffsiteSource rather than a literal "offsite" compare lets a
// per-target source reach repoFor's off-site resolution instead of being
// clobbered to "local".
func collectStatsSource(source string) string {
	if isOffsiteSource(source) {
		return source
	}
	return "local"
}

// CollectStatsOnStartup samples each enabled domain's local repo shortly
// after boot so the Storage card shows data for repos that already have
// backups, instead of "no data" until the next backup runs. Best-effort and
// throttled.
func (s *Service) CollectStatsOnStartup() {
	settings, err := s.store.GetSettings()
	if err != nil {
		return
	}
	for _, d := range []struct {
		name    string
		enabled bool
	}{
		{"containers", settings.ContainersEnabled},
		{"vms", settings.VMsEnabled},
		{"flash", settings.FlashEnabled},
		{"config", settings.ConfigEnabled},
		{"files", settings.FilesEnabled},
	} {
		if d.enabled {
			s.CollectStatsAsync(d.name, "local")
		}
	}
}
