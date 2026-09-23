// Package platform holds the few behaviors that differ between the hosts
// BombVault runs on: Unraid with its share conventions, a plain Docker host,
// and TrueNAS Scale. The Docker API, restic and the compose discovery are the
// same everywhere.
package platform

import "context"

// Kind is the detected platform, or the one set with the PLATFORM override.
type Kind string

const (
	KindUnraid  Kind = "unraid"
	KindTrueNAS Kind = "truenas"
	KindGeneric Kind = "generic"
)

// SSHRunner runs one command on the host and returns its trimmed stdout. It is
// declared here because internal/api imports this package; api.HostSSH
// satisfies it as is.
type SSHRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// Platform supplies the behaviors that depend on the host: the appdata
// fallback, the default destinations for a restore from another instance,
// and the step that refreshes the host's UI after a container was recreated
// with a newer image.
type Platform interface {
	Kind() Kind

	// AppdataFallback returns the host path to try for a container's data
	// when discovery found nothing, or "" to back up the configuration only.
	// The caller translates the result through the host mount root like any
	// bind mount and keeps it only if it exists. An implementation that uses
	// hostMountRoot must therefore still return a host path: a
	// container-visible one would not translate and be dropped.
	AppdataFallback(hostMountRoot, containerName string) string

	// ForeignContainerDestBase returns the container-visible destination for
	// containers restored from another instance when no target is configured.
	ForeignContainerDestBase(hostMountRoot string) string

	// ForeignVMDestBase is ForeignContainerDestBase for VMs.
	ForeignVMDestBase(hostMountRoot string) string

	// ReconcileContainerUpdateStatus makes the host's UI show a post-backup
	// image update. Platforms without such a step do nothing. ssh may be nil,
	// which an implementation must treat as a skip, not an error.
	ReconcileContainerUpdateStatus(ctx context.Context, ssh SSHRunner, imageRef string) error
}
