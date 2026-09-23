package platform

import "context"

// Generic implements Platform for a plain Docker host with no assumed share
// layout.
type Generic struct{}

var _ Platform = Generic{}

func (Generic) Kind() Kind { return KindGeneric }

// AppdataFallback returns "": a configuration-only backup beats guessing a
// folder that does not exist.
func (Generic) AppdataFallback(_, _ string) string { return "" }

// ForeignContainerDestBase returns the host mount root itself.
func (Generic) ForeignContainerDestBase(hostMountRoot string) string { return hostMountRoot }

// ForeignVMDestBase returns the host mount root itself.
func (Generic) ForeignVMDestBase(hostMountRoot string) string { return hostMountRoot }

// ReconcileContainerUpdateStatus does nothing; a plain Docker host has no UI
// to refresh.
func (Generic) ReconcileContainerUpdateStatus(context.Context, SSHRunner, string) error {
	return nil
}
