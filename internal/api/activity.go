package api

import (
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/progress"
)

// ActivityItem is one running operation as the progress stream reports it.
type ActivityItem struct {
	Domain    string
	Item      string
	Phase     string
	Percent   float64
	StartedAt int64
}

// ActivitySnapshot is everything BombVault has in flight at one moment.
type ActivitySnapshot struct {
	Items             []ActivityItem
	DomainsBusy       map[string]string
	BackupRunning     bool
	EverythingRunning bool
}

// ActivitySnapshot reads the in-memory progress store and the domain locks. It
// touches neither the database nor restic.
func (s *Service) ActivitySnapshot() ActivitySnapshot {
	out := ActivitySnapshot{
		Items:             []ActivityItem{},
		DomainsBusy:       map[string]string{},
		BackupRunning:     s.BackupInProgress(),
		EverythingRunning: s.EverythingInProgress(),
	}
	if s.progress != nil {
		for _, e := range s.progress.Snapshot() {
			out.Items = append(out.Items, activityItem(e))
		}
	}
	for _, domain := range mcpDomains {
		if reason, busy := s.domainBusy(domain); busy {
			out.DomainsBusy[domain] = reason
		}
	}
	return out
}

// activityItem splits a progress key into the domain and the thing it names. A
// prefix nobody here knows is passed through as its own domain rather than
// dropped: an operation missing from the list reads as an idle server.
func activityItem(e progress.Event) ActivityItem {
	item := ActivityItem{Phase: e.Phase, Percent: e.Percent, StartedAt: e.StartedAt}
	prefix, name, found := strings.Cut(e.Key, ":")
	if !found {
		item.Domain = e.Key
		return item
	}
	switch prefix {
	case "container":
		item.Domain, item.Item = "containers", name
	case "vm":
		item.Domain, item.Item = "vms", name
	case "files":
		item.Domain, item.Item = "files", name
	case "batch":
		item.Domain = name
	default:
		item.Domain, item.Item = prefix, name
	}
	return item
}
