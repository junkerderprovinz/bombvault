package api

import (
	"fmt"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// rejectInvalidOwnRetention refuses a keep-policy for something that is not a
// domain, on both write paths. field names the setting in the message.
func rejectInvalidOwnRetention(field string, in map[string]store.RetentionKeep) string {
	for domain := range in {
		if !slices.Contains(offsiteConfigDomains, domain) {
			return fmt.Sprintf("%s: %q is not a domain", field, domain)
		}
	}
	return ""
}

// applyOwnRetention stores the submitted per-domain local keep-policies,
// replacing the stored set. A nil map comes from a client or an export older
// than the setting and keeps what is stored; an empty one puts every domain
// back on the shared policy. in must have passed rejectInvalidOwnRetention.
func applyOwnRetention(cur *store.Settings, in map[string]store.RetentionKeep) {
	if in != nil {
		cur.SetOwnRetention(clampKeeps(in))
	}
}

// applyOwnOffsiteRetention is applyOwnRetention for the off-site policies.
func applyOwnOffsiteRetention(cur *store.Settings, in map[string]store.RetentionKeep) {
	if in != nil {
		cur.SetOwnOffsiteRetention(clampKeeps(in))
	}
}

func clampKeeps(in map[string]store.RetentionKeep) map[string]store.RetentionKeep {
	next := make(map[string]store.RetentionKeep, len(in))
	for domain, k := range in {
		next[domain] = clampKeep(k)
	}
	return next
}

func clampKeep(k store.RetentionKeep) store.RetentionKeep {
	return store.RetentionKeep{
		KeepLast:    max(0, k.KeepLast),
		KeepDaily:   max(0, k.KeepDaily),
		KeepWeekly:  max(0, k.KeepWeekly),
		KeepMonthly: max(0, k.KeepMonthly),
		KeepYearly:  max(0, k.KeepYearly),
	}
}
