package api

import (
	"fmt"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// restoreFolders is where a restore of each section into the restore folder
// lands, keyed by section.
func restoreFolders(s store.Settings) map[string]string {
	out := make(map[string]string, len(offsiteConfigDomains))
	for _, section := range offsiteConfigDomains {
		out[section] = s.RestoreFolderFor(section)
	}
	return out
}

// rejectInvalidRestoreFolders holds a section's own restore folder to what the
// shared one has to be: a relative path under the mount root.
func rejectInvalidRestoreFolders(in map[string]string, mountRoot, sourceRoot string) error {
	for section, folder := range in {
		if !slices.Contains(offsiteConfigDomains, section) {
			return fmt.Errorf("restoreFolderOverrides: %q is not a section", section)
		}
		folder = strings.TrimSpace(folder)
		if folder == "" {
			continue
		}
		if restic.IsRemoteRepo(folder) {
			return errRestoreFolderRemote
		}
		if err := localPathRefusal(mountRoot, sourceRoot, folder); err != nil {
			return err
		}
	}
	return nil
}

// applyRestoreFolders stores the submitted switch and the sections' own
// folders. An empty folder puts its section back on the shared one. v must
// have passed rejectInvalidRestoreFolders.
func applyRestoreFolders(cur *store.Settings, v settingsView) {
	if v.RestoreFolderPerSection != nil {
		cur.RestoreFolderPerSection = *v.RestoreFolderPerSection
	}
	if v.RestoreFolderOverrides == nil {
		return
	}
	own := make(map[string]string, len(v.RestoreFolderOverrides))
	for section, folder := range v.RestoreFolderOverrides {
		if folder = strings.TrimSpace(folder); folder != "" {
			own[section] = folder
		}
	}
	cur.SetOwnRestoreFolders(own)
}
