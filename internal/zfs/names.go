package zfs

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// SnapshotPrefix marks the snapshot a backup run takes, so a leftover is
	// recognisable and the sweeper never touches anything else.
	SnapshotPrefix = "bombvault-"
	// PreRestorePrefix marks the safety snapshot taken before an in-place
	// restore. It is a different prefix so the sweeper leaves it alone; the
	// user deletes it.
	PreRestorePrefix = "bombvault-prerestore-"

	// MaxSnapshotNameLen is ZFS's limit for "<dataset>@<snapshot>".
	MaxSnapshotNameLen = 255
	// MaxDatasetNameLen is the longest item root BombVault accepts, so that
	// the longest name it ever builds from it, a pre-restore snapshot with a
	// 14 digit stamp, still fits MaxSnapshotNameLen. Descendants come from the
	// host and may be longer; SnapshotNameFits is the limit that applies to
	// them.
	MaxDatasetNameLen = MaxSnapshotNameLen - len(PreRestorePrefix) - 1 - stampLen

	stampLen = 14
	// stampLayout renders the UTC instant of a snapshot name. A local time
	// would give two snapshots the same name in the hour a DST change repeats.
	stampLayout = "20060102150405"
)

var (
	snapshotRe   = regexp.MustCompile(`^bombvault-[0-9]{14}$`)
	preRestoreRe = regexp.MustCompile(`^bombvault-prerestore-[0-9]{14}$`)
)

// NameError says why a name was refused. Code is a reason code of the ZFS
// domain, "invalid-name" or "name-too-long", and Reason names the part at
// fault so the message can point at it.
type NameError struct {
	Code   string
	Reason string
}

func (e *NameError) Error() string { return e.Reason }

// ValidateDatasetName checks that name is one BombVault may put into an argv.
// It accepts the ZFS component characters plus inner spaces, which Unraid
// shares with spaces turn into.
func ValidateDatasetName(name string) error {
	if err := validateNameChars(name); err != nil {
		return err
	}
	if len(name) > MaxDatasetNameLen {
		return &NameError{Code: "name-too-long", Reason: "dataset name is longer than " + strconv.Itoa(MaxDatasetNameLen) + " bytes"}
	}
	return nil
}

// SnapshotNameFits reports whether a BombVault snapshot of name stays inside
// ZFS's 255 byte limit. A descendant that does not fit makes the whole
// recursive snapshot fail, so the preflight refuses the item and names it.
func SnapshotNameFits(name string) bool {
	return len(name)+1+len(SnapshotPrefix)+stampLen <= MaxSnapshotNameLen
}

// validateNameChars checks the shape of a dataset name without the length
// limit of an item root, which does not apply to descendants read off the host.
func validateNameChars(name string) error {
	if name == "" {
		return &NameError{Code: "invalid-name", Reason: "dataset name is empty"}
	}
	for _, comp := range strings.Split(name, "/") {
		if err := validateComponent(comp); err != nil {
			return err
		}
	}
	return nil
}

func validateComponent(comp string) error {
	switch {
	case comp == "":
		return &NameError{Code: "invalid-name", Reason: "dataset name has an empty component"}
	case comp == "." || comp == "..":
		return &NameError{Code: "invalid-name", Reason: "dataset name has a " + comp + " component"}
	case comp[0] == '-':
		return &NameError{Code: "invalid-name", Reason: "dataset name component starts with a dash: " + comp}
	case comp[0] == ' ' || comp[len(comp)-1] == ' ':
		return &NameError{Code: "invalid-name", Reason: "dataset name component starts or ends with a space: " + comp}
	}
	for i := 0; i < len(comp); i++ {
		c := comp[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_' || c == '.' || c == ':' || c == '-' || c == ' ':
		default:
			return &NameError{Code: "invalid-name", Reason: "dataset name holds a character ZFS does not allow: " + comp}
		}
	}
	return nil
}

// SnapshotName is the host snapshot name for a backup taken at now.
func SnapshotName(now time.Time) string {
	return SnapshotPrefix + now.UTC().Format(stampLayout)
}

// PreRestoreSnapshotName is the safety snapshot name for a restore started at now.
func PreRestoreSnapshotName(now time.Time) string {
	return PreRestorePrefix + now.UTC().Format(stampLayout)
}

// IsBombVaultSnapshot reports whether snap is a snapshot a backup run took.
// Everything the sweeper and the recursive destroy touch passes through here.
func IsBombVaultSnapshot(snap string) bool { return snapshotRe.MatchString(snap) }

// IsPreRestoreSnapshot reports whether snap is a safety snapshot.
func IsPreRestoreSnapshot(snap string) bool { return preRestoreRe.MatchString(snap) }

// StampFromPath returns the snapshot name at the end of a restic snapshot's
// first path, which is the directory a run read: .../.zfs/snapshot/bombvault-<ts>.
// It is what ties the member snapshots of one run together without the database.
func StampFromPath(p string) (string, bool) {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return "", false
	}
	snap := p[i+1:]
	if !IsBombVaultSnapshot(snap) {
		return "", false
	}
	if !strings.HasSuffix(p[:i], "/.zfs/snapshot") {
		return "", false
	}
	return snap, true
}
