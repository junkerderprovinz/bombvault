package platform

import (
	"context"
	"log"
	"os"
	"path/filepath"
)

// Detect resolves the platform. A non-empty override (the PLATFORM env var)
// wins; an unknown value is logged and falls back to KindGeneric, so a typo
// never selects another platform's conventions. Otherwise the Unraid
// dockerMan directory under flashDir, the mounted flash drive, means
// KindUnraid, and its absence KindGeneric.
//
// TrueNAS is never detected: a container that only has the Docker socket sees
// no reliable marker for it (the ix-apps dataset and the libvirt socket need
// not be mounted), so it takes PLATFORM=truenas.
func Detect(_ context.Context, override, flashDir string) Kind {
	if override != "" {
		switch Kind(override) {
		case KindUnraid, KindTrueNAS, KindGeneric:
			return Kind(override)
		default:
			log.Printf("platform: unrecognized PLATFORM=%q, falling back to %q", override, KindGeneric) //nolint:gosec // G706: override is %q-quoted
			return KindGeneric
		}
	}
	marker := filepath.Join(flashDir, "config/plugins/dockerMan")
	if _, err := os.Stat(marker); err == nil {
		return KindUnraid
	}
	return KindGeneric
}
