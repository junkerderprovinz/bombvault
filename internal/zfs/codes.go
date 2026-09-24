package zfs

// AllCodes is every reason code the ZFS domain stores or sends. The frontend
// has a sentence for each one, and web/src/lib/zfsCodes.test.ts reads these
// literals, so each entry stays one quoted string on its own.
var AllCodes = []string{
	// Connection
	"ok",
	"ssh-missing",
	"host-placeholder",
	"host-fallback",
	"ssh-unreachable",
	"ssh-auth",
	"zfs-not-found",
	"zfs-permission",
	"uri-mismatch",
	"zfs-error",
	"propagation-missing",

	// Item
	"invalid-name",
	"name-too-long",
	"invalid-exclude",
	"not-found",
	"not-filesystem",
	"overlaps-item",
	"docker-storage",
	"nothing-readable",
	"snapshot-failed",
	"containers-busy",
	"consistency-stop-failed",
	"pre-snapshot-failed",
	"container-unknown",
	"container-is-self",
	"hook-container-missing",
	"leftover-snapshots",

	// Member
	"zvol",
	"canmount-off",
	"legacy-mount",
	"no-mountpoint",
	"not-mounted",
	"key-not-loaded",
	"snapdir-disabled",
	"not-visible",
	"shfs-only",
	"snapshot-not-visible",
	"snapshot-loop",
	"backup-failed",
	"not-reached",
	"gone",

	// Restore
	"read-only-mount",
	"destination-not-mounted",
	"not-enough-space",
	"safety-snapshot-failed",
	"safety-name-too-long",
}

// MemberOutcomes are the member states that are not problems. They carry their
// own labels, not the amber sentences of AllCodes.
var MemberOutcomes = []string{
	"backed-up",
	"empty",
	"excluded",
}
