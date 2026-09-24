package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
)

// itemSelection is what one item is configured to back up. Its fingerprint
// separates a selection the user narrowed from a source that lost its data:
// both show the same drop in size and mean opposite things. Only what the user
// or the host can change belongs in it, never what is on disk at backup time,
// so an unmounted share keeps the fingerprint it had.
type itemSelection struct {
	Kind     string   `json:"kind"`
	Root     string   `json:"root,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
	Caches   []string `json:"caches,omitempty"`
	Engine   string   `json:"engine,omitempty"`
	Scope    string   `json:"scope,omitempty"`
	// Children are the datasets a ZFS item leaves out with their subtrees.
	Children []string `json:"children,omitempty"`
}

// selectionFingerprint returns the first 16 hex characters of the SHA-256 over
// sel. The lists are sorted and deduplicated first, so the order a container's
// mounts or a user's exclude lines happen to arrive in never reads as a change.
func selectionFingerprint(sel itemSelection) string {
	sel.Paths = canonicalStrings(sel.Paths)
	sel.Excludes = canonicalStrings(sel.Excludes)
	sel.Caches = canonicalStrings(sel.Caches)
	sel.Children = canonicalStrings(sel.Children)
	raw, _ := json.Marshal(sel) //nolint:errcheck // a struct of strings always marshals
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

func canonicalStrings(xs []string) []string {
	if len(xs) == 0 {
		return nil
	}
	out := slices.Clone(xs)
	slices.Sort(out)
	return slices.Compact(out)
}

// enabledKeys returns the keys m maps to true.
func enabledKeys(m map[string]bool) []string {
	var out []string
	for k, on := range m {
		if on {
			out = append(out, k)
		}
	}
	return out
}
