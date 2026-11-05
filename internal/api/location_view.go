package api

import (
	"errors"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/redact"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// errHiddenPasswordMoved refuses an edit that keeps a hidden password while the
// location points at another server, which that password was not typed for.
var errHiddenPasswordMoved = errors.New("the password of this location is hidden and stays with its server. To use another server, type the whole location with its password")

// unhideLocation is what a save of incoming writes when stored is the location
// it replaces. Every answer shows a location with its password hidden, so a form
// sends the marker back, and an edit of the rest of the location keeps the
// stored password.
func unhideLocation(incoming, stored string) (string, error) {
	incoming = strings.TrimSpace(incoming)
	if !locationRedacted(incoming) {
		return incoming, nil
	}
	restored, ok := redact.Restore(incoming, strings.TrimSpace(stored))
	if !ok {
		return "", errHiddenPasswordMoved
	}
	return restored, nil
}

// shownTarget is a target as an answer shows it. offsiteTargetToView itself keeps
// the location whole for the settings file, which decides on its own what it
// carries.
func shownTarget(t store.OffsiteTarget) offsiteTargetView {
	v := offsiteTargetToView(t)
	v.Repo = scrubRepoLocation(v.Repo)
	return v
}

// settingsLocationField is one location of the settings row: what a save brings
// and what is stored.
type settingsLocationField struct {
	incoming *string
	stored   string
}

// settingsLocationFields lists every location a settings save writes.
func settingsLocationFields(v *settingsView, cur store.Settings) []settingsLocationField {
	return []settingsLocationField{
		{&v.ContainersPath, cur.ContainersPath},
		{&v.VMsPath, cur.VMsPath},
		{&v.FlashPath, cur.FlashPath},
		{&v.ConfigPath, cur.ConfigPath},
		{&v.FilesPath, cur.FilesPath},
		{&v.ZFSPath, cur.ZFSPath},
		{&v.ContainersOffsite, cur.ContainersOffsite},
		{&v.VMsOffsite, cur.VMsOffsite},
		{&v.FlashOffsite, cur.FlashOffsite},
		{&v.ConfigOffsite, cur.ConfigOffsite},
		{&v.FilesOffsite, cur.FilesOffsite},
		{&v.ZFSOffsite, cur.ZFSOffsite},
		{&v.FlashZipExportPath, cur.FlashZipExportPath},
	}
}

// unhideSettingsLocations gives every location of a save its hidden password
// back, before anything judges or compares them.
func unhideSettingsLocations(v *settingsView, cur store.Settings) error {
	for _, f := range settingsLocationFields(v, cur) {
		if !locationRedacted(*f.incoming) {
			continue
		}
		restored, err := unhideLocation(*f.incoming, f.stored)
		if err != nil {
			return err
		}
		*f.incoming = restored
	}
	return nil
}

// rejectOptionLikeSettingsLocations holds the locations a save changes to
// restic.CheckLocation. One already stored stays out of it: the page posts every
// field with every save, and the refusal would otherwise stop all of them until
// that one is fixed, while its runs already say why they fail.
func rejectOptionLikeSettingsLocations(v settingsView, cur store.Settings) error {
	for _, f := range settingsLocationFields(&v, cur) {
		if strings.TrimSpace(*f.incoming) == strings.TrimSpace(f.stored) {
			continue
		}
		if err := restic.CheckLocation(*f.incoming); err != nil {
			return err
		}
	}
	return nil
}

// askedLocation gives a location the question before a save asks about its
// hidden password back, from what that save would replace.
func (h *Handler) askedLocation(domain, targetID, loc, as string) (string, error) {
	if !locationRedacted(loc) {
		return loc, nil
	}
	var stored string
	switch {
	case as == "field":
		cur, err := h.store.GetSettings()
		if err != nil {
			return "", err
		}
		stored = offsiteRepoFromSettings(domain, cur)
	case targetID != "":
		t, _, err := h.store.GetOffsiteTarget(targetID)
		if err != nil {
			return "", err
		}
		stored = t.Repo
	}
	return unhideLocation(loc, stored)
}
