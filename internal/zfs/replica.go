package zfs

import (
	"fmt"
	"regexp"
	"strings"
)

// replicaListProps are what base resolution compares across the two hosts.
const replicaListProps = "name,guid,createtxg"

const stateProps = "type,encryption,receive_resume_token"

// resumeTokenRe is the shape zfs prints for receive_resume_token: a version,
// a checksum, the payload length and the compressed payload, all in hex.
var resumeTokenRe = regexp.MustCompile(`^[0-9]+-[0-9a-f]+-[0-9a-f]+-[0-9a-f]+$`)

// SendSpec is one member's stream of a replica run.
type SendSpec struct {
	Member string
	Snap   string
	// Base is the replica snapshot or bookmark the stream starts from, or ""
	// for a full stream.
	Base string
	// FromBookmark starts from Member#Base instead of Member@Base.
	FromBookmark bool
	// Raw sends an encrypted member as it lies on disk, so the target never
	// holds its key and later incrementals stay raw.
	Raw bool
}

// ReceiveSpec is how a stream lands on the target.
type ReceiveSpec struct {
	Target string
	// Full marks the first stream into a new dataset. It lands read-only and,
	// for a filesystem, never mounts by itself.
	Full   bool
	Volume bool
	// Rollback throws away whatever changed on the replica since its newest
	// snapshot. It only applies to an incremental.
	Rollback bool
	// Replace lets a full stream land on an empty parent BombVault created
	// earlier for another entry, which is what a pool root finds at its path.
	Replace bool
}

// receivedLocalProps are the properties a filesystem stream never sets on the
// receiving host, so a replica neither mounts over the target's own data nor
// shares itself there.
var receivedLocalProps = []string{"mountpoint", "sharenfs", "sharesmb"}

// receivedSpaceProps are the space the source sets aside for a dataset or
// volume, which its replica must not claim from the receiving host's pool. zfs
// drops them from a stream when they are excluded, since they do not inherit.
var receivedSpaceProps = []string{"reservation", "refreservation"}

// replicaMember checks a dataset and replica snapshot name pair for any
// builder below. The dataset may be a descendant read off the host or a path
// on the target, so the replica name limit applies instead of the root's.
func replicaMember(dataset, snap string) error {
	if err := validateNameChars(dataset); err != nil {
		return err
	}
	if !IsReplicaSnapshot(snap) {
		return &NameError{Code: "invalid-name", Reason: "not a replica snapshot name: " + snap}
	}
	if !ReplicaNameFits(dataset) {
		return &NameError{Code: "name-too-long", Reason: "dataset name is too long for a replica snapshot of it: " + dataset}
	}
	return nil
}

// ReplicaSnapshotArgs takes the one atomic snapshot of a whole item tree that
// a replica run sends its members from.
func ReplicaSnapshotArgs(root, snap string) ([]string, error) {
	if err := ValidateDatasetName(root); err != nil {
		return nil, err
	}
	if err := replicaMember(root, snap); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "snapshot", "-r", root + "@" + snap}, nil
}

// SendArgs streams one member with its properties. Members go one by one
// rather than with -R, which would also send the children the item excludes.
// An increment is always -i: -I would carry every foreign snapshot in between,
// which the target's retention never prunes, and a bookmark only takes -i.
func SendArgs(s SendSpec) ([]string, error) {
	if err := replicaMember(s.Member, s.Snap); err != nil {
		return nil, err
	}
	args := []string{zfsBinary, "send"}
	if s.Raw {
		args = append(args, "-w", "-p")
	} else {
		args = append(args, "-c", "-L", "-e", "-p")
	}
	if s.Base != "" {
		if !IsReplicaSnapshot(s.Base) {
			return nil, &NameError{Code: "invalid-name", Reason: "not a replica snapshot name: " + s.Base}
		}
		from := "@" + s.Base
		if s.FromBookmark {
			from = "#" + s.Base
		}
		args = append(args, "-i", from)
	}
	return append(args, s.Member+"@"+s.Snap), nil
}

// EstimateArgs asks how many bytes SendArgs would stream, without sending.
func EstimateArgs(s SendSpec) ([]string, error) {
	args, err := SendArgs(s)
	if err != nil {
		return nil, err
	}
	return dryRun(args), nil
}

// ResumeSendArgs continues a stream the target kept a resume token for. The
// token carries the original flags, so none are added.
func ResumeSendArgs(token string) ([]string, error) {
	if !resumeTokenRe.MatchString(token) {
		return nil, fmt.Errorf("zfs: not a receive resume token: %.40q", token)
	}
	return []string{zfsBinary, "send", "-t", token}, nil
}

// EstimateResumeArgs asks how many bytes a resumed stream still has to send.
func EstimateResumeArgs(token string) ([]string, error) {
	args, err := ResumeSendArgs(token)
	if err != nil {
		return nil, err
	}
	return dryRun(args), nil
}

func dryRun(send []string) []string {
	return append([]string{send[0], send[1], "-n", "-v", "-P"}, send[2:]...)
}

// ReceiveArgs takes a stream into the target, resumably and unmounted. zfs
// refuses canmount on a volume and only warns about the local properties, so a
// volume gets neither; the reservations stay behind for both.
func ReceiveArgs(r ReceiveSpec) ([]string, error) {
	if err := receiveTarget(r.Target); err != nil {
		return nil, err
	}
	if r.Full && r.Rollback {
		return nil, fmt.Errorf("zfs: a full receive into %q cannot roll anything back", r.Target)
	}
	if r.Replace && !r.Full {
		return nil, fmt.Errorf("zfs: only a full receive into %q can replace it", r.Target)
	}
	args := []string{zfsBinary, "receive", "-s", "-u"}
	if r.Rollback || r.Replace {
		args = append(args, "-F")
	}
	if r.Full {
		args = append(args, "-o", "readonly=on")
		if !r.Volume {
			args = append(args, "-o", "canmount=noauto")
		}
	}
	if !r.Volume {
		args = appendExcluded(args, receivedLocalProps)
	}
	args = appendExcluded(args, receivedSpaceProps)
	return append(args, r.Target), nil
}

// RestoreReceiveArgs lands a replica snapshot as a new dataset that behaves
// like any other: writable, mounting where its parent says. It is not
// resumable, so a cut stream leaves nothing behind.
func RestoreReceiveArgs(target string, volume bool) ([]string, error) {
	if err := receiveTarget(target); err != nil {
		return nil, err
	}
	args := appendExcluded([]string{zfsBinary, "receive", "-u"}, []string{"readonly"})
	if !volume {
		args = appendExcluded(args, append([]string{"canmount"}, receivedLocalProps...))
	}
	return append(args, target), nil
}

func receiveTarget(target string) error {
	if err := validateNameChars(target); err != nil {
		return err
	}
	if !ReplicaNameFits(target) {
		return &NameError{Code: "name-too-long", Reason: "dataset name is too long for a replica snapshot of it: " + target}
	}
	return nil
}

func appendExcluded(args, props []string) []string {
	for _, p := range props {
		args = append(args, "-x", p)
	}
	return args
}

// CreateParentArgs makes one level of the path a replica lands under. It never
// mounts and holds no data of its own; -p makes a level that appeared in the
// meantime a success. A non-empty owner is set as SourceProperty in the same
// step, so a parent of that owner never exists unmarked.
func CreateParentArgs(dataset, owner string) ([]string, error) {
	if err := validateNameChars(dataset); err != nil {
		return nil, err
	}
	args := []string{zfsBinary, "create", "-p", "-u", "-o", "canmount=off"}
	if owner != "" {
		if !ValidSourceID(owner) {
			return nil, fmt.Errorf("zfs: not an instance id: %.40q", owner)
		}
		args = append(args, "-o", SourceProperty+"="+owner)
	}
	return append(args, dataset), nil
}

// AbortReceiveArgs drops the partial state of an interrupted receive whose
// token can no longer be resumed.
func AbortReceiveArgs(target string) ([]string, error) {
	if err := validateNameChars(target); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "receive", "-A", target}, nil
}

// HoldArgs keeps the member's replica snapshot until a newer one is on both
// sides.
func HoldArgs(member, snap string) ([]string, error) {
	if err := replicaMember(member, snap); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "hold", ReplicaHoldTag, member + "@" + snap}, nil
}

// ReleaseArgs lifts the hold HoldArgs placed, so the snapshot can go.
func ReleaseArgs(member, snap string) ([]string, error) {
	if err := replicaMember(member, snap); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "release", ReplicaHoldTag, member + "@" + snap}, nil
}

// BookmarkArgs keeps a member's replicated state as a bookmark, which costs no
// space and still serves as the base of an incremental once the snapshot is
// gone.
func BookmarkArgs(member, snap string) ([]string, error) {
	if err := replicaMember(member, snap); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "bookmark", member + "@" + snap, member + "#" + snap}, nil
}

// DestroyReplicaArgs removes one replica snapshot of one dataset, for the
// target's retention.
func DestroyReplicaArgs(dataset, snap string) ([]string, error) {
	if err := replicaMember(dataset, snap); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "destroy", dataset + "@" + snap}, nil
}

// DestroyReplicaRecursiveArgs removes a replica snapshot from the whole source
// tree, the excluded children included, since ReplicaSnapshotArgs took it on
// all of them.
func DestroyReplicaRecursiveArgs(root, snap string) ([]string, error) {
	if err := ValidateDatasetName(root); err != nil {
		return nil, err
	}
	if err := replicaMember(root, snap); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "destroy", "-r", root + "@" + snap}, nil
}

// ReplicaPointsArgs lists the snapshots and bookmarks of one dataset, oldest
// first, without its children.
func ReplicaPointsArgs(dataset string) ([]string, error) {
	if err := validateNameChars(dataset); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "list", "-H", "-p", "-t", "snapshot,bookmark", "-o", replicaListProps, "-s", "createtxg", "-d", "1", dataset}, nil
}

// DatasetStateArgs reads what a replica run needs to know about one dataset
// before it streams: its type, its encryption and the token an interrupted
// receive -s left on it. A dataset that does not exist fails as not-found.
func DatasetStateArgs(dataset string) ([]string, error) {
	if err := validateNameChars(dataset); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "get", "-H", "-p", "-o", "property,value", stateProps, dataset}, nil
}

// DestroyReplicaBookmarkArgs removes one replica bookmark of a source member,
// once no snapshot on the target shares its guid, so that it cannot be a
// base.
func DestroyReplicaBookmarkArgs(dataset, name string) ([]string, error) {
	if err := replicaMember(dataset, name); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "destroy", dataset + "#" + name}, nil
}

// SourceProperty is the user property set on every parent BombVault creates
// for a replica, holding the id of the instance whose members land below it.
// A folder that carries another id belongs to someone else, even when that
// instance goes by the same name.
const SourceProperty = "bombvault:source"

var sourceIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,63}$`)

// ValidSourceID reports whether id can stand in SourceProperty: an instance id
// of the group, which never starts with a dash.
func ValidSourceID(id string) bool { return sourceIDRe.MatchString(id) }

// SourcePropertyArgs reads SourceProperty as set on the dataset itself, so a
// value inherited from a parent or received with a stream does not count.
func SourcePropertyArgs(dataset string) ([]string, error) {
	if err := validateNameChars(dataset); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "get", "-H", "-p", "-s", "local", "-o", "value", SourceProperty, dataset}, nil
}

// ParseSourceProperty reads the output of SourcePropertyArgs, "" where the
// property is not set on the dataset itself.
func ParseSourceProperty(out string) string {
	v := strings.TrimSpace(out)
	if v == "-" {
		return ""
	}
	return v
}

// MountArgs mounts a filesystem where its mountpoint property says.
func MountArgs(dataset string) ([]string, error) {
	if err := validateNameChars(dataset); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "mount", dataset}, nil
}

// Pool is one pool as PoolsArgs reports it. Size is what its top dataset holds
// plus what is still free there, which is what a replica can fill.
type Pool struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	FreeBytes int64  `json:"freeBytes"`
}

// PoolsArgs lists the top dataset of every pool on a host.
func PoolsArgs() []string {
	return []string{zfsBinary, "list", "-H", "-p", "-d", "0", "-o", "name,used,available"}
}

// ParsePools reads the listing of PoolsArgs.
func ParsePools(out string) ([]Pool, error) {
	lines := splitLines(out)
	pools := make([]Pool, 0, len(lines))
	for _, line := range lines {
		f := strings.Split(line, "\t")
		if len(f) != 3 || strings.Contains(f[0], "/") {
			return nil, fmt.Errorf("zfs list of the pools: %q is not a pool line", line)
		}
		used, err := parseNum(f[1])
		if err != nil {
			return nil, fmt.Errorf("zfs list of the pools: used of %q: %w", f[0], err)
		}
		free, err := parseNum(f[2])
		if err != nil {
			return nil, fmt.Errorf("zfs list of the pools: available of %q: %w", f[0], err)
		}
		pools = append(pools, Pool{Name: f[0], SizeBytes: used + free, FreeBytes: free})
	}
	return pools, nil
}
