package platform

import (
	"context"
	"log"
	"os"
	"path/filepath"
)

// Detect resolves which Platform Kind BombVault is running on.
//
// override, when non-empty (config.Config.PlatformOverride, the PLATFORM env
// var), wins outright: a recognized value (KindUnraid/KindTrueNAS/
// KindGeneric) is returned as-is; an unrecognized one is logged and treated
// as KindGeneric — a typo in PLATFORM must never make the detection silently
// pick the wrong platform's conventions.
//
// Otherwise, Detect probes for the Unraid dockerMan marker directory under
// flashDir (the container-visible mount of the Unraid USB flash's config
// folder — the same marker config.Config.FlashTemplatesDir's default path
// lives under). Found → KindUnraid. Not found → KindGeneric.
//
// Deliberately NOT attempted: automatic TrueNAS detection. There is no
// reliable filesystem marker visible from inside a Docker-socket-only
// container — TrueNAS's ix-apps dataset and libvirt socket are not
// guaranteed mounted, especially with VMs disabled. TrueNAS selection stays
// explicit-only (PLATFORM=truenas) even now that platform.TrueNAS{} is a
// real implementation (see main.go's Kind→Platform mapping) — only the
// selection mechanism was ever the open question, not whether TrueNAS has an
// implementation to select.
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
