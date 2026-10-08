package zfs

import (
	"fmt"
	"regexp"
)

// replicaListProps are what base resolution compares across the two hosts.
const replicaListProps = "name,guid,createtxg"

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
	// FromBookmark starts from Member#Base. zfs refuses -I from a bookmark,
	// so such a stream carries only the change up to Snap.
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
}

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

// SendArgs streams one member. Members go one by one rather than with -R,
// which would also send the children the item excludes.
func SendArgs(s SendSpec) ([]string, error) {
	if err := replicaMember(s.Member, s.Snap); err != nil {
		return nil, err
	}
	args := []string{zfsBinary, "send"}
	if s.Raw {
		args = append(args, "-w")
	} else {
		args = append(args, "-c", "-L", "-e")
	}
	if s.Base != "" {
		if !IsReplicaSnapshot(s.Base) {
			return nil, &NameError{Code: "invalid-name", Reason: "not a replica snapshot name: " + s.Base}
		}
		if s.FromBookmark {
			args = append(args, "-i", "#"+s.Base)
		} else {
			args = append(args, "-I", "@"+s.Base)
		}
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

// ReceiveArgs takes a stream into the target, resumably and unmounted. A
// received mountpoint is never applied, so a replica cannot mount over one of
// the target's own shares. Volumes have neither property and zfs refuses
// canmount on them.
func ReceiveArgs(r ReceiveSpec) ([]string, error) {
	if err := validateNameChars(r.Target); err != nil {
		return nil, err
	}
	if !ReplicaNameFits(r.Target) {
		return nil, &NameError{Code: "name-too-long", Reason: "dataset name is too long for a replica snapshot of it: " + r.Target}
	}
	if r.Full && r.Rollback {
		return nil, fmt.Errorf("zfs: a full receive into %q cannot roll anything back", r.Target)
	}
	args := []string{zfsBinary, "receive", "-s", "-u"}
	if r.Rollback {
		args = append(args, "-F")
	}
	if r.Full {
		args = append(args, "-o", "readonly=on")
		if !r.Volume {
			args = append(args, "-o", "canmount=noauto")
		}
	}
	if !r.Volume {
		args = append(args, "-x", "mountpoint")
	}
	return append(args, r.Target), nil
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

// ResumeTokenArgs reads the token an interrupted receive -s left on target.
func ResumeTokenArgs(target string) ([]string, error) {
	if err := validateNameChars(target); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "get", "-H", "-p", "-o", "value", "receive_resume_token", target}, nil
}
