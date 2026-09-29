package api

import (
	"fmt"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// storedCompression reads a mode from the store for a restic.Mode, where the
// zero value is restic's default, so a mode built without a repository row
// equals one built with a row that never chose. Everything written to the store
// went through ParseCompression, so a value it refuses can only be a hand edit,
// and restic's default is the safe reading of that.
func storedCompression(stored string) restic.Compression {
	c, err := restic.ParseCompression(stored)
	if err != nil || c == restic.CompressionAuto {
		return ""
	}
	return c
}

// offsiteCompressionKey is the settings key of a domain's primary off-site
// destination, next to the domain name that keys its own repository.
func offsiteCompressionKey(domain string) string { return "offsite:" + domain }

// compressionView is what the settings page reads: every key with its mode,
// restic's default included, so the page never has to guess a missing one.
func compressionView(s store.Settings) map[string]string {
	out := make(map[string]string, 2*len(offsiteConfigDomains))
	for _, d := range offsiteConfigDomains {
		for _, key := range []string{d, offsiteCompressionKey(d)} {
			out[key] = normalizedCompression(s.CompressionFor(key))
		}
	}
	return out
}

// rejectInvalidCompression refuses an unknown repository key or mode on both
// write paths.
func rejectInvalidCompression(in map[string]string) string {
	for key, mode := range in {
		domain := strings.TrimPrefix(key, "offsite:")
		if !slices.Contains(offsiteConfigDomains, domain) {
			return fmt.Sprintf("compression: %q is not a repository", key)
		}
		if _, err := restic.ParseCompression(mode); err != nil {
			return err.Error()
		}
	}
	return ""
}

// applyCompression stores the submitted modes. restic's default is stored as
// no entry at all, so the row of an install that never touched the setting
// stays as it was. A nil map comes from a client or an export older than the
// setting and keeps what is stored. in must have passed
// rejectInvalidCompression.
func applyCompression(cur *store.Settings, in map[string]string) {
	if in == nil {
		return
	}
	var next store.Settings
	for key, mode := range in {
		if c := storedCompression(mode); c != "" {
			next.SetCompression(key, string(c))
		}
	}
	cur.Compression = next.Compression
}

// normalizedCompression is a row's mode as the API reports and stores it.
func normalizedCompression(stored string) string {
	if c := storedCompression(stored); c != "" {
		return string(c)
	}
	return string(restic.CompressionAuto)
}
