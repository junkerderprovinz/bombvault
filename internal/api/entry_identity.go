package api

import (
	"slices"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// entryIdentity is what one container or VM entry may claim in its repository:
// every snapshot carrying its current identity tag and, for each alias, the old
// name's snapshots taken before that alias was linked.
//
// A machine that reuses the old name later can only write its snapshots after
// the link, so the bound is a property of the snapshots themselves. It needs no
// Docker or virsh probe and holds when the reused name was backed up and then
// removed, when a probe would fail, and when the VMs domain is switched off.
//
// The bound cuts both ways: when the current name is another entry's alias,
// the snapshots under it from before that link are the other entry's, so this
// one owns only those from the link on (ceded).
//
// Readers go through owned (via snapshotsOwnedBy), retention through
// retentionTags, and the alias folds through aliasClaim.claims and
// aliasClaim.mixed.
type entryIdentity struct {
	tag     string // container:<name> or vm:<name>
	aliases []aliasClaim
	ceded   *cededClaim
	// readErr is set when a store read failed and the identity is partial, so a
	// gate refuses where a reader shows what could be read.
	readErr error
}

// aliasClaim is one old name's share of an entry: its identity tag and the
// moment it was linked.
type aliasClaim struct {
	tag      string
	linkedAt time.Time
}

// cededClaim is another entry's alias on this entry's current name. owner is
// that entry's current tag, for the log; "" when it could not be read.
// unknown marks an alias table that could not be read at all: nothing then
// shows the name's snapshots to be this entry's.
type cededClaim struct {
	aliasClaim
	owner   string
	unknown bool
}

// tagIdentity is an identity with no aliases: exactly the snapshots that carry
// tag. The foreign restore uses it, because this box's rename history says
// nothing about another box's repository.
func tagIdentity(tag string) entryIdentity { return entryIdentity{tag: tag} }

func newAliasClaim(prefix string, a store.Alias) aliasClaim {
	return aliasClaim{tag: prefix + a.OldName, linkedAt: time.Unix(a.LinkedAt, 0)}
}

// listTags is every tag the entry might own snapshots under: what restic is
// asked to list before owned narrows the result. It is not a forget set,
// because forget cannot apply the time bound; see retentionTags.
func (id entryIdentity) listTags() []string {
	if id.tag == "" {
		return nil
	}
	tags := make([]string, 0, 1+len(id.aliases))
	tags = append(tags, id.tag)
	for _, a := range id.aliases {
		tags = append(tags, a.tag)
	}
	return tags
}

// owns reports whether snap belongs to the entry: it carries the current tag
// and no other entry's alias claims it, or one of the entry's aliases does.
func (id entryIdentity) owns(snap restic.Snapshot) bool {
	if slices.Contains(snap.Tags, id.tag) && (id.ceded == nil || id.ceded.cedes(snap)) {
		return true
	}
	for _, a := range id.aliases {
		if a.claims(snap) {
			return true
		}
	}
	return false
}

// owned keeps the snapshots of listed the entry owns, in listed's order. A nil
// listing stays nil ("no repository yet").
func (id entryIdentity) owned(listed []restic.Snapshot) []restic.Snapshot {
	if listed == nil {
		return nil
	}
	out := make([]restic.Snapshot, 0, len(listed))
	for _, snap := range listed {
		if id.owns(snap) {
			out = append(out, snap)
		}
	}
	return out
}

// retentionTags splits the entry's tags for restic forget, which selects by
// tag and knows no time bound. An alias tag may join only while every
// snapshot under it is one the alias claims; the rest are withheld. The whole
// pass pauses while the current tag holds a snapshot that is not provably
// this entry's, because forgetting by that tag would age another entry's
// pre-link history under this entry's policy. Either way the snapshots left
// out are kept, and the per-identity pass leaves them alone too (see
// foldAliasedIdentityTags). listed must be the unfiltered listing of
// listTags, or a snapshot that is not the entry's would be invisible here.
func (id entryIdentity) retentionTags(listed []restic.Snapshot) (tags []string, withheld []aliasClaim, paused bool) {
	if id.ceded != nil && !id.ceded.cedesEvery(listed) {
		return nil, nil, true
	}
	tags = []string{id.tag}
	for _, a := range id.aliases {
		if a.claimsEvery(listed) {
			tags = append(tags, a.tag)
		} else {
			withheld = append(withheld, a)
		}
	}
	return tags, withheld, false
}

// takenAt is snap's Time when snap carries a's tag. A Time that does not
// parse cannot be placed on either side of the link, so it is not ok.
func (a aliasClaim) takenAt(snap restic.Snapshot) (time.Time, bool) {
	if !slices.Contains(snap.Tags, a.tag) {
		return time.Time{}, false
	}
	return snapshotTime(snap)
}

// snapshotTime is when snap was taken. A Time that does not parse places the
// snapshot on neither side of a link.
func snapshotTime(snap restic.Snapshot) (time.Time, bool) {
	ts, err := time.Parse(time.RFC3339Nano, snap.Time)
	return ts, err == nil
}

// claims reports whether snap is the old name's history from before the link.
func (a aliasClaim) claims(snap restic.Snapshot) bool {
	ts, ok := a.takenAt(snap)
	return ok && ts.Before(a.linkedAt)
}

// claimsEvery reports whether a claims every snapshot in listed that carries
// its tag.
func (a aliasClaim) claimsEvery(listed []restic.Snapshot) bool {
	for _, snap := range listed {
		if slices.Contains(snap.Tags, a.tag) && !a.claims(snap) {
			return false
		}
	}
	return true
}

// mixed reports whether a's tag holds snapshots a claims next to ones it does
// not: two machines' history under one tag, which no forget by that tag can
// keep apart.
func (a aliasClaim) mixed(listed []restic.Snapshot) bool {
	return !a.claimsEvery(listed) && slices.ContainsFunc(listed, a.claims)
}

// cedes reports whether snap, under the ceded name, is provably from the link
// on and so belongs to this entry rather than the other.
func (c cededClaim) cedes(snap restic.Snapshot) bool {
	ts, ok := c.takenAt(snap)
	return ok && !c.unknown && !ts.Before(c.linkedAt)
}

// cedesEvery reports whether every snapshot in listed under the ceded name
// is provably from the link on.
func (c cededClaim) cedesEvery(listed []restic.Snapshot) bool {
	for _, snap := range listed {
		if slices.Contains(snap.Tags, c.tag) && !c.cedes(snap) {
			return false
		}
	}
	return true
}
