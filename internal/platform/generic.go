package platform

import "context"

// Generic implements Platform for a plain Docker host with no assumed share
// layout: no appdata-fallback convention (Tasks 2/3's bind/volume/compose-
// label discovery is expected to find everything on its own), an identity
// restore-destination default (no assumed subpath under the host mount), and
// no host-side update-status step to reconcile.
type Generic struct{}

var _ Platform = Generic{}

func (Generic) Kind() Kind { return KindGeneric }

// AppdataFallback: no convention to fall back to. An empty selection
// (config-only backup) beats guessing a folder that doesn't exist.
func (Generic) AppdataFallback(_, _ string) string { return "" }

// ForeignContainerDestBase: identity default — the host mount root itself,
// no assumed subpath.
func (Generic) ForeignContainerDestBase(hostMountRoot string) string { return hostMountRoot }

// ForeignVMDestBase: identity default — the host mount root itself, no
// assumed subpath.
func (Generic) ForeignVMDestBase(hostMountRoot string) string { return hostMountRoot }

// ReconcileContainerUpdateStatus: no generic-Docker-host UI to reconcile.
func (Generic) ReconcileContainerUpdateStatus(context.Context, SSHRunner, string) error {
	return nil
}
