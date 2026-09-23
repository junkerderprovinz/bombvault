package platform

// TrueNAS implements Platform for TrueNAS Scale. Since 24.10 its apps are
// plain Docker Compose, so everything except Kind comes from Generic. Kind is
// its own so that PLATFORM=truenas stays distinguishable in the settings and
// status payloads.
type TrueNAS struct{ Generic }

var _ Platform = TrueNAS{}

func (TrueNAS) Kind() Kind { return KindTrueNAS }
