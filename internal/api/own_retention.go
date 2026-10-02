package api

import (
	"fmt"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// rejectInvalidOwnRetention refuses a keep-policy for something that is not a
// domain, on both write paths.
func rejectInvalidOwnRetention(in map[string]store.RetentionKeep) string {
	for domain := range in {
		if !slices.Contains(offsiteConfigDomains, domain) {
			return fmt.Sprintf("ownRetention: %q is not a domain", domain)
		}
	}
	return ""
}

// applyOwnRetention stores the submitted per-domain keep-policies, replacing
// the stored set. A nil map comes from a client or an export older than the
// setting and keeps what is stored; an empty one puts every domain back on the
// shared policy. in must have passed rejectInvalidOwnRetention.
func applyOwnRetention(cur *store.Settings, in map[string]store.RetentionKeep) {
	if in == nil {
		return
	}
	next := make(map[string]store.RetentionKeep, len(in))
	for domain, k := range in {
		next[domain] = store.RetentionKeep{
			KeepLast:    max(0, k.KeepLast),
			KeepDaily:   max(0, k.KeepDaily),
			KeepWeekly:  max(0, k.KeepWeekly),
			KeepMonthly: max(0, k.KeepMonthly),
			KeepYearly:  max(0, k.KeepYearly),
		}
	}
	cur.SetOwnRetention(next)
}
