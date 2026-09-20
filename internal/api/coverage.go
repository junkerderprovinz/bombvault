package api

import (
	"context"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Coverage answers a question the Dashboard could not: what on this server is
// NOT backed up by anything?
//
// The protection card next to it is per DOMAIN. It reports whether the
// containers that ARE scheduled ran on time, which is worth knowing and says
// nothing at all about the container nobody ever added. That is the gap
// operators actually fall into, because an item that was never set up looks
// exactly like one that is fine: absent from every list, absent from every
// error, and absent from the traffic light.
//
// "Protected" here means precisely what the scheduler means: some automatic run
// would back this item up. It is computed with schedule.Effective*Schedule, the
// same functions the scheduler's own reasoning uses, rather than by re-deriving
// the rules — three toggles and an override reach every item, and three of the
// four can silently mean "never".

// CoverageReason says WHY an item is unprotected, so the interface can offer
// the fix rather than only the fact.
const (
	// CoverageNotSetUp: the item exists on the host but BombVault has no row for
	// it. Nothing is switched off; it was simply never added.
	CoverageNotSetUp = "not-set-up"
	// CoverageNotIncluded: the item has a row with "include in schedule" off.
	// The label reads like "skipped by the domain schedule" and actually means
	// no automatic run touches it at all.
	CoverageNotIncluded = "not-included"
	// CoverageOverrideOff: the item carries a per-item override of literally
	// "off", which removes it from its own entry, the domain run and Backup
	// Everything alike.
	CoverageOverrideOff = "override-off"
	// CoverageNoSchedule: the item is included and not overridden, but neither
	// its domain schedule nor Backup Everything is switched on, so nothing ever
	// fires. This one is invisible on the item's own page.
	CoverageNoSchedule = "no-schedule"
	// CoverageDBNotScheduled: the container runs a database server and no
	// automatic run backs it up, so neither its files nor its dumps are kept.
	CoverageDBNotScheduled = "db-not-scheduled"
	// CoverageDBDumpFailing: the container is backed up and its latest database
	// dump did not succeed. Acknowledging the failure clears the dashboard
	// badge and does nothing about the missing dump.
	CoverageDBDumpFailing = "db-dump-failing"
	// CoverageDBDumpOnlyCopyOff: the dump is switched off although the files
	// backup copies the data while the server runs, so nothing holds a
	// consistent copy of this database.
	CoverageDBDumpOnlyCopyOff = "db-dump-only-copy-off"
)

// CoverageItem is one unprotected item.
type CoverageItem struct {
	Name string `json:"name"`
	// Reason is one of the Coverage* constants.
	Reason string `json:"reason"`
	// NeverBackedUp distinguishes "unprotected and there is not even an old
	// copy" from "unprotected, but a manual backup exists". The second is a gap;
	// the first is a hole.
	NeverBackedUp bool `json:"neverBackedUp"`
}

// CoverageDomain is one domain's part of the answer.
type CoverageDomain struct {
	Domain string `json:"domain"`
	// Enabled is the domain's own switch. A switched-off domain is a DECISION,
	// not a failure: its items are reported as neither protected nor
	// unprotected, because counting them would make the card cry wolf on a
	// correctly configured server, and a card that cries wolf gets hidden.
	Enabled     bool           `json:"enabled"`
	Total       int            `json:"total"`
	Protected   int            `json:"protected"`
	Unprotected []CoverageItem `json:"unprotected"`
}

// CoverageReport is the whole answer.
type CoverageReport struct {
	Domains []CoverageDomain `json:"domains"`
	// Total and Protected count only the domains that are switched on, so the
	// ratio on screen answers "of what I asked BombVault to protect, how much
	// is actually protected".
	Total     int `json:"total"`
	Protected int `json:"protected"`
}

// Coverage builds the report.
func (s *Service) Coverage(ctx context.Context) (CoverageReport, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return CoverageReport{}, err
	}

	report := CoverageReport{}
	for _, d := range []CoverageDomain{
		s.coverContainers(ctx, settings),
		s.coverVMs(settings),
		s.coverFileSets(settings),
	} {
		if d.Unprotected == nil {
			d.Unprotected = []CoverageItem{}
		}
		if d.Enabled {
			report.Total += d.Total
			report.Protected += d.Protected
		}
		report.Domains = append(report.Domains, d)
	}
	return report, nil
}

// coverContainers walks the LIVE container list, not the stored rows.
//
// That is the whole reason this is not a database query: a container running on
// the host with no row has no include flag, no override and no backup, because
// there is no row to hold any of them. Reading only stored rows would miss
// exactly the item that most needs naming.
func (s *Service) coverContainers(ctx context.Context, settings store.Settings) CoverageDomain {
	out := CoverageDomain{Domain: "containers", Enabled: settings.ContainersEnabled}
	// A switched-off domain produces no unprotected items, only the fact that it
	// is off. Listing its items would put a permanent red list in front of an
	// operator who deliberately does not use that domain, and the only way to
	// clear it would be to switch on a domain they do not want.
	if !out.Enabled {
		return out
	}

	live, err := s.docker.List(ctx)
	if err != nil {
		// Docker unreachable is not a coverage answer. Reporting zero items
		// would read as "everything is fine" on the one host where nothing can
		// be checked at all.
		log.Printf("api: coverage: listing containers failed, the container domain is reported as unknown: %v", err)
		return out
	}

	// BombVault never backs itself up through the container path, so listing it
	// would be a permanent false alarm nobody can clear.
	self := s.SelfContainerName(ctx)

	rows := map[string]store.Target{}
	if stored, sErr := s.store.ListTargets(); sErr == nil {
		for _, t := range stored {
			rows[t.ContainerName] = t
		}
	}

	dbRows := s.dbDumpRows(ctx, live, rows)

	for _, c := range live {
		if c.Name == "" || (self != "" && c.Name == self) {
			continue
		}
		out.Total++
		row, known := rows[c.Name]
		if !known {
			out.Unprotected = append(out.Unprotected, CoverageItem{
				Name: c.Name, Reason: CoverageNotSetUp, NeverBackedUp: true,
			})
			continue
		}
		db, isDB := dbRows[c.Name]
		eff := schedule.EffectiveContainerSchedule(row, settings)
		if eff.Kind == schedule.EffectiveNone {
			reason := reasonFor(row.IncludeInSchedule, row.ScheduleCadence, settings.PerItemSchedules)
			if isDB {
				reason = CoverageDBNotScheduled
			}
			out.Unprotected = append(out.Unprotected, CoverageItem{
				Name: c.Name, Reason: reason, NeverBackedUp: s.neverBackedUp(row.ID),
			})
			continue
		}
		// A database whose dumps are missing counts as unprotected although the
		// schedule reaches it: for these containers the files alone are not an
		// answer to what is backed up.
		if gap := s.dbDumpGap(settings, row, db); gap != "" {
			out.Unprotected = append(out.Unprotected, CoverageItem{
				Name: c.Name, Reason: gap, NeverBackedUp: s.neverBackedUp(row.ID),
			})
			continue
		}
		out.Protected++
	}
	return out
}

// dbDumpGap names what is missing from a scheduled database container's dumps,
// or "" when nothing is. A container that is no database, or one nobody has
// named an engine for yet, has no gap: there is nothing to dump it with.
func (s *Service) dbDumpGap(settings store.Settings, tg store.Target, db dbDumpRow) string {
	if db.Engine == "" {
		return ""
	}
	if !settings.DBDumpsEnabled || tg.DBDumpOff || db.LabelOff {
		if db.Coverage != dbCoverageStopped {
			return CoverageDBDumpOnlyCopyOff
		}
		return ""
	}
	run, err := s.store.LastRunOfKind(tg.ID, "dbdump")
	if err != nil {
		log.Printf("api: coverage: reading the last database dump of %q failed: %v", tg.ContainerName, err) //nolint:gosec // G706: name is %q-quoted
		return ""
	}
	if run != nil && run.Status != "success" {
		return CoverageDBDumpFailing
	}
	return ""
}

// coverVMs reads the STORED VM rows rather than asking libvirt.
//
// Deliberately different from containers, and the asymmetry is not an
// oversight: ListVMs only connects to libvirt when the VM domain is on, because
// an unconditional virsh-over-SSH connect spams the log of every user who does
// not back up VMs. A coverage report must not be the thing that starts doing
// that. The cost is that a VM defined on the host but never added to BombVault
// is not named here the way an unknown container is; the VM tab is where that
// one shows up.
func (s *Service) coverVMs(settings store.Settings) CoverageDomain {
	out := CoverageDomain{Domain: "vms", Enabled: settings.VMsEnabled}
	// A switched-off domain produces no unprotected items, only the fact that it
	// is off. Listing its items would put a permanent red list in front of an
	// operator who deliberately does not use that domain, and the only way to
	// clear it would be to switch on a domain they do not want.
	if !out.Enabled {
		return out
	}

	rows, err := s.store.ListVMTargets()
	if err != nil {
		log.Printf("api: coverage: listing VM targets failed: %v", err)
		return out
	}
	for _, v := range rows {
		out.Total++
		if schedule.EffectiveVMSchedule(v, settings).Kind != schedule.EffectiveNone {
			out.Protected++
			continue
		}
		out.Unprotected = append(out.Unprotected, CoverageItem{
			Name:          v.Name,
			Reason:        reasonFor(v.IncludeInSchedule, v.ScheduleCadence, settings.PerItemSchedules),
			NeverBackedUp: s.neverBackedUp(v.ID),
		})
	}
	return out
}

// coverFileSets walks the stored sets; a file set has no existence outside
// BombVault, so there is no "never added" case to find.
func (s *Service) coverFileSets(settings store.Settings) CoverageDomain {
	out := CoverageDomain{Domain: "files", Enabled: settings.FilesEnabled}
	// A switched-off domain produces no unprotected items, only the fact that it
	// is off. Listing its items would put a permanent red list in front of an
	// operator who deliberately does not use that domain, and the only way to
	// clear it would be to switch on a domain they do not want.
	if !out.Enabled {
		return out
	}

	sets, err := s.store.ListFileSets()
	if err != nil {
		log.Printf("api: coverage: listing file sets failed: %v", err)
		return out
	}
	for _, fs := range sets {
		out.Total++
		if schedule.EffectiveFileSetSchedule(fs, settings).Kind != schedule.EffectiveNone {
			out.Protected++
			continue
		}
		out.Unprotected = append(out.Unprotected, CoverageItem{
			Name:          fs.Name,
			Reason:        reasonFor(fs.Enabled, fs.ScheduleCadence, settings.PerItemSchedules),
			NeverBackedUp: s.neverBackedUp(fs.ID),
		})
	}
	return out
}

// reasonFor names which of the three switches left this item uncovered, in the
// order the scheduler checks them.
func reasonFor(included bool, override string, perItem bool) string {
	if !included {
		return CoverageNotIncluded
	}
	if perItem && override == "off" {
		return CoverageOverrideOff
	}
	return CoverageNoSchedule
}

// neverBackedUp reports whether the item has no successful backup at all. A
// read error answers false: claiming "never backed up" because a query failed
// would be the more alarming of the two wrong answers.
func (s *Service) neverBackedUp(targetID string) bool {
	if targetID == "" {
		return true
	}
	run, err := s.store.LastSuccessfulBackup(targetID)
	if err != nil {
		return false
	}
	return run == nil
}

// handleCoverage serves the report.
// GET /api/coverage
//
// A GET behind the session gate like the rest of the read API: it names items
// and their schedule state, never a path, a location or a credential.
func (h *Handler) handleCoverage(w http.ResponseWriter, r *http.Request) {
	report, err := h.svc.Coverage(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"coverage": report}))
}
