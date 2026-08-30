// Package platform is the seam between BombVault's core (the Docker Engine
// API, restic, and the compose/data-root discovery in internal/api — all of
// which are already platform-neutral) and the small set of behaviors that
// differ by the host BombVault runs on: Unraid's array/share conventions,
// a plain generic Docker host, and TrueNAS Scale.
//
// See the design notes, §4 for the design rationale and Task 5 for the
// exact seam this package fills.
package platform

import "context"

// Kind identifies which platform BombVault detected (or was explicitly told,
// via the PLATFORM override) it is running on.
type Kind string

const (
	KindUnraid  Kind = "unraid"
	KindTrueNAS Kind = "truenas"
	KindGeneric Kind = "generic"
)

// SSHRunner is the minimal host-SSH capability ReconcileContainerUpdateStatus
// needs: run one command on the host and get its trimmed stdout back. It is
// declared here, consumer-side, rather than importing a concrete SSH type,
// for the same reason internal/api declares its own HostSSH subset instead of
// depending on *sshconn.Conn directly: internal/platform sits BELOW
// internal/api in the dependency graph (api will construct and hold a
// Platform, so platform cannot import api without a cycle), and api.HostSSH
// already satisfies this interface structurally — no adapter needed at the
// call site.
type SSHRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// Platform supplies the handful of behaviors BombVault's core does not know
// how to derive on its own: the appdata-fallback convention, the
// cross-instance restore-destination defaults, and the host-side step (if
// any) that reconciles the host's own UI after BombVault recreates a
// container with a newer image.
type Platform interface {
	// Kind reports which concrete platform this is.
	Kind() Kind

	// AppdataFallback returns the last-resort absolute HOST path to try for a
	// container's persistent data when no bind/volume/compose-label/label-
	// override candidate matched anything (internal/api's resolveAppdataPaths
	// translates the result through the configured host-mount root exactly as
	// it does for every discovered bind mount, then only keeps it if it
	// actually exists). Empty string means "no convention — give up",
	// producing an empty (config-only) backup selection rather than a guessed
	// folder that doesn't exist. hostMountRoot is the container-visible mount
	// root, provided for platforms whose convention needs it; Unraid's
	// convention is a fixed HOST-side path and does not. An implementation
	// that DOES use hostMountRoot must still return a value in HOST terms
	// (translatable back through the caller's configured host-source root) —
	// e.g. resolving it to the real host path that mount corresponds to.
	// Simply joining a subpath onto hostMountRoot and returning that verbatim
	// yields a container-visible path mislabeled as a host one: the caller's
	// translation will not recognize it, silently discarding this candidate
	// and falling back to its own hardcoded guess instead.
	AppdataFallback(hostMountRoot, containerName string) string

	// ForeignContainerDestBase returns the default cross-instance restore
	// destination (a container-visible path already rooted at hostMountRoot)
	// for the containers domain when no explicit target/RestoreFolder is
	// configured.
	ForeignContainerDestBase(hostMountRoot string) string

	// ForeignVMDestBase returns the default cross-instance restore
	// destination (a container-visible path already rooted at hostMountRoot)
	// for the vms domain when no explicit target/RestoreFolder is configured.
	ForeignVMDestBase(hostMountRoot string) string

	// ReconcileContainerUpdateStatus runs whatever host-side step (if any)
	// makes the host's own UI reflect a post-backup image update. A no-op on
	// any platform without one — this is how the existing Unraid-only #116
	// step already behaves off Unraid today (nil SSH → skipped), preserved
	// exactly rather than special-cased. ssh may be nil (no SSH configured);
	// an implementation that needs it must treat that as "skip, no error".
	ReconcileContainerUpdateStatus(ctx context.Context, ssh SSHRunner, imageRef string) error
}
