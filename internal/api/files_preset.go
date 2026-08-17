package api

import (
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/platform"
)

// ---------------------------------------------------------------------------
// "Host system config" preset — the files domain's practical analogue of
// Unraid's flash-USB backup, for generic Docker hosts and TrueNAS Scale,
// which have no boot-USB device to capture. Unraid keeps using its dedicated
// flash domain for this purpose, so the preset is never offered there. See
// docs/superpowers/plans/2026-08-16-bombvault-platform-expansion.md Task 7
// and the design spec's flash-domain analogue section for the rationale.
//
// This is a suggested STARTING POINT, not a discovery result and not a claim
// of completeness — the operator reviews and edits it in the same Add-set
// dialog used for every other file set before saving (mirrors the read-only,
// nothing-persisted shape of the exclusion assistant in excludes_suggest.go).
// ---------------------------------------------------------------------------

// defaultHostConfigFileSet returns the suggested name/path/excludes for the
// "Host system config" preset for the given platform. ok is false for
// platform.KindUnraid — the flash domain already covers this purpose there,
// so the caller must not offer the preset in that case.
//
// name is "host-system-config", not the human-readable "Host system config"
// the UI's button is labeled with: a file set's Name feeds validResourceName
// (letters/digits/./_/- only, see handlers.go) AND becomes the restic tag
// (fileset:<Name>), so it cannot contain a space. It is still just a
// suggested starting value the operator can rename before saving, same as
// every other field here.
//
// path is relative to cfg.HostMountRoot, exactly like every other file set's
// Path field (see store.FileSet.Path, resolved via paths.Resolve) — it is
// NOT an absolute host path. The guess assumes the identity-bind convention
// documented for generic/TrueNAS deployments (HostSourceRoot == HostMountRoot,
// i.e. the host's root filesystem is reachable under the configured mount
// root, per internal/paths/paths.go's doc comment): a deployment that only
// binds a narrower data directory will not find "etc" on disk and must edit
// the path before saving — exactly the "starting point, not a guarantee"
// contract validateFileSet already enforces (the source path must exist).
func defaultHostConfigFileSet(kind platform.Kind) (name, path string, excludes []string, ok bool) {
	switch kind {
	case platform.KindGeneric, platform.KindTrueNAS:
		return "host-system-config", "etc", nil, true
	default:
		return "", "", nil, false
	}
}

// handleFileSetPreset returns the suggested "Host system config" file set
// for the CURRENT platform (see defaultHostConfigFileSet's doc comment for
// why the values are what they are). Read-only — nothing is persisted here;
// the frontend pre-fills the existing Add-set dialog with the result and the
// operator saves (or edits first) through the normal create-file-set path.
// GET /api/files/sets/preset
func (h *Handler) handleFileSetPreset(w http.ResponseWriter, _ *http.Request) {
	name, path, excludes, ok := defaultHostConfigFileSet(h.svc.platformFn().Kind())
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
