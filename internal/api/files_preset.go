package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// defaultHostConfigFileSet returns the "Host system config" file set suggested
// on hosts without Unraid's boot USB. ok is false on Unraid, where the flash
// domain already covers this.
//
// The name has no spaces because it becomes the restic tag fileset:<name>. The
// path is relative to HostMountRoot and assumes the host's root filesystem is
// mounted there. With a narrower mount "etc" does not exist and the preset is
// not offered, see handleFileSetPreset.
func defaultHostConfigFileSet(kind platform.Kind) (name, path string, excludes []string, ok bool) {
	switch kind {
	case platform.KindGeneric, platform.KindTrueNAS:
		return "host-system-config", "etc", nil, true
	default:
		return "", "", nil, false
	}
}

// handleFileSetPreset returns the suggested file set for the current platform.
// The UI pre-fills its add-set dialog with it; nothing is saved here.
// GET /api/files/sets/preset
func (h *Handler) handleFileSetPreset(w http.ResponseWriter, _ *http.Request) {
	name, path, excludes, ok := defaultHostConfigFileSet(h.svc.platformFn().Kind())
	if ok {
		if _, err := os.Stat(filepath.Join(h.svc.cfg.HostMountRoot, path)); err != nil {
			ok = false
		}
	}
	if excludes == nil {
		excludes = []string{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"offered":  ok,
		"name":     name,
		"path":     path,
		"excludes": excludes,
	}))
}
