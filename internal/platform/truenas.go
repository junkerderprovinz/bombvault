package platform

// TrueNAS implements Platform for TrueNAS Scale. It embeds Generic{}: since
// 24.10 TrueNAS Apps is plain Docker + Compose (no Kubernetes/k3s layer), so
// containers behave identically to a generic Docker host — no TrueNAS-
// specific appdata-fallback or restore-destination convention is needed, and
// there is no TrueNAS UI update-status step to reconcile the way Unraid's
// dynamix.docker.manager has one (#116). See
// the design notes §5
// for the full TrueNAS-fit rationale, including what this type deliberately
// does NOT attempt (zvol-aware VM disk backup, NVRAM/TPM, domain-name
// versioning — separate, VM-domain-specific work, not part of the Platform
// interface).
//
// Kind is the one method TrueNAS overrides rather than inheriting: reporting
// the embedded Generic{}'s KindGeneric would make PLATFORM=truenas
// indistinguishable from PLATFORM=generic everywhere Kind() is consulted
// (settings/status payloads, future TrueNAS-specific gating), defeating the
// purpose of KindTrueNAS existing as its own value. Every other Platform
// method is intentionally NOT overridden — do not add new interface methods
// here speculatively; only add what a concrete need (e.g. the VM-domain work
// referenced above) actually requires once it exists.
type TrueNAS struct{ Generic }

var _ Platform = TrueNAS{}

func (TrueNAS) Kind() Kind { return KindTrueNAS }
