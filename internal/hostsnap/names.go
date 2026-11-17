package hostsnap

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	// PlanPrefix marks the snapshots a schedule takes. Retention only ever
	// matches this prefix followed by the plan's own id, so a snapshot someone
	// renamed or took by hand is never removed by it.
	PlanPrefix = "bombvault-auto-"
	// HoldTag is the hold BombVault places and the only one it releases.
	HoldTag = "bombvault-keep"
	// BtrfsFolder holds the snapshots BombVault takes of a btrfs subvolume,
	// below the top level of the filesystem.
	BtrfsFolder = ".bombvault-snapshots"

	maxNameLen = 200
	stampLen   = 14
	// stampLayout is UTC, so the hour a DST change repeats cannot give two
	// snapshots the same name.
	stampLayout = "20060102150405"
)

// ErrInvalidName refuses a snapshot name the tools would read as something
// else, or that does not fit.
var ErrInvalidName = errors.New("invalid-name")

var (
	planIDRe   = regexp.MustCompile(`^[0-9a-f]{8}$`)
	planNameRe = regexp.MustCompile(`^bombvault-auto-([0-9a-f]{8})-([0-9]{14})$`)
	backupRe   = regexp.MustCompile(`^bombvault-([0-9]{14})$`)
	replicaRe  = regexp.MustCompile(`^bombvault-replica-[0-9]{14}$`)
	safetyRe   = regexp.MustCompile(`^bombvault-prerestore-[0-9]{14}$`)
)

// ValidateName checks a snapshot name someone typed. The characters are the
// ones ZFS, btrfs and libvirt all accept, so a name means the same everywhere,
// and a leading dash could never reach an argv as an option.
func ValidateName(name string) error {
	if name == "" || len(name) > maxNameLen || name == "." || name == ".." || name[0] == '-' {
		return ErrInvalidName
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_' || c == '.' || c == ':' || c == '-':
		default:
			return ErrInvalidName
		}
	}
	return nil
}

// PlanSnapshotName is the name a plan gives the snapshot it takes at now.
func PlanSnapshotName(planID string, now time.Time) string {
	return PlanPrefix + shortPlanID(planID) + "-" + now.UTC().Format(stampLayout)
}

// shortPlanID is the part of a plan id its snapshot names carry. Store ids are
// 32 hex characters; eight of them keep the names short and still tell the
// plans of one source apart.
func shortPlanID(id string) string {
	id = strings.ToLower(id)
	if len(id) > 8 {
		id = id[:8]
	}
	return id
}

// PlanOf returns the short plan id a snapshot name carries, and false for a
// name no plan took.
func PlanOf(name string) (string, bool) {
	m := planNameRe.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// OwnedByPlan reports whether a plan with the given id took the snapshot.
func OwnedByPlan(name, planID string) bool {
	short, ok := PlanOf(name)
	return ok && planIDRe.MatchString(short) && short == shortPlanID(planID)
}

// Owner names who made a snapshot, from its name alone: "plan" for a
// schedule, "backup" and "replica" for the snapshots a backup or a replica
// run keeps for itself, "safety" for the one an in-place restore left, and
// "" for everything else.
func Owner(name string) string {
	switch {
	case planNameRe.MatchString(name):
		return "plan"
	case backupRe.MatchString(name):
		return "backup"
	case replicaRe.MatchString(name):
		return "replica"
	case safetyRe.MatchString(name):
		return "safety"
	}
	return ""
}

// managedByRuns reports whether a backup or replica run needs the snapshot,
// which is why this page never renames, holds or deletes it.
func managedByRuns(name string) bool {
	o := Owner(name)
	return o == "backup" || o == "replica"
}

// DefaultName is the name the create dialog offers.
func DefaultName(now time.Time) string {
	return "manual-" + now.UTC().Format(stampLayout)
}
