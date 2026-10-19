package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errUnknownSignIn = errors.New("that saved sign-in does not exist any more")
	errSignInNotHere = errors.New("a saved sign-in serves S3 services and restic REST servers only")
)

// foldShared makes the shared sign-in the default: the saved sign-in with the
// same keys takes the mark, or a new one called Default sign-in holds them.
// It returns the default's id and whether that sign-in existed already. sets
// is changed in place.
func foldShared(sets []CloudCredSet, shared CloudCreds) ([]CloudCredSet, string, bool) {
	if shared == (CloudCreds{}) {
		return sets, "", false
	}
	for i := range sets {
		sets[i].Default = false
	}
	if i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.KeptFor == "" && c.CloudCreds == shared }); i >= 0 {
		sets[i].Default = true
		return sets, sets[i].ID, true
	}
	id := newCredSetID()
	return append(sets, CloudCredSet{ID: id, Name: freeSetName(sets, defaultSignInName), Default: true, CloudCreds: shared}), id, false
}

// mergeIdentical keeps one saved sign-in of each set of keys: the first keeps
// its name, and the ones after it go. gone maps each id that went to the one
// that stays. Kept sets belong to their direct repository and stay apart.
func mergeIdentical(sets []CloudCredSet) (out []CloudCredSet, gone map[string]string) {
	gone = map[string]string{}
	for _, c := range sets {
		if c.KeptFor == "" && c.CloudCreds != (CloudCreds{}) {
			i := slices.IndexFunc(out, func(o CloudCredSet) bool { return o.KeptFor == "" && o.CloudCreds == c.CloudCreds })
			if i >= 0 {
				gone[c.ID] = out[i].ID
				out[i].Default = out[i].Default || c.Default
				continue
			}
		}
		out = append(out, c)
	}
	return out, gone
}

// ConvertSharedSignIns runs once, at the first start after the shared cloud
// sign-in became a saved one. The shared sign-in becomes the default: a saved
// sign-in with the same keys takes the mark, or a new one called Default
// sign-in holds them. Saved sign-ins with the same keys as another are merged
// into it, and a remote own folder typed in by hand that lies in a storage
// location's folder for its section follows that location from then on. What
// it did stays for the note on the Sign-ins card.
func (s *Service) ConvertSharedSignIns() error {
	pending, err := s.store.SignInsPending()
	if err != nil || !pending {
		return err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return fmt.Errorf("read the saved sign-ins: %w", err)
	}
	shared, err := s.decodeSharedBlob(settings.CloudConf)
	if err != nil {
		return fmt.Errorf("read the shared sign-in: %w", err)
	}
	merged, gone := mergeIdentical(sets)
	// The rows move first: a start that stops in between finds the sets
	// still there and merges them again.
	for from, to := range gone {
		if err := s.store.ReplaceCredsRef(from, to); err != nil {
			return err
		}
	}
	folded, id, existed := foldShared(merged, shared)
	note := store.SignInsNote{SetID: id, Merged: existed, Duplicates: len(gone)}
	if len(gone) > 0 || settings.CloudConf != "" {
		enc, err := s.encodeCloudCredSets(folded)
		if err != nil {
			return err
		}
		if _, err := s.store.MutateSettings(func(st *store.Settings) error {
			st.CloudCredSets, st.CloudConf = enc, ""
			return nil
		}); err != nil {
			return err
		}
	}
	note.Linked = s.linkTypedOwnFolders()
	return s.store.FinishSignIns(note)
}

// linkTypedOwnFolders hangs each remote own folder typed in by hand on the
// storage location whose folder for the section it is. A folder that signs in
// differently, or that is append-only where the location is not, stays as
// typed, since following the location would change how it is reached or let
// retention prune it.
func (s *Service) linkTypedOwnFolders() []string {
	settings, err := s.store.GetSettings()
	if err != nil {
		log.Printf("api: sign-ins: could not read the settings to link own folders: %v", err)
		return nil
	}
	destinations, err := s.store.ListDestinations()
	if err != nil {
		log.Printf("api: sign-ins: could not read the storage locations to link own folders: %v", err)
		return nil
	}
	var linked []string
	for _, domain := range offsiteConfigDomains {
		path := strings.TrimSpace(domainPathRaw(domain, settings))
		if !restic.IsRemoteRepo(path) {
			continue
		}
		row, ok, err := s.store.PrimaryRemoteTarget(domain)
		if err != nil || ok && row.DestinationID != "" {
			continue
		}
		i := slices.IndexFunc(destinations, func(d store.OffsiteTarget) bool {
			return sameRepoLocation(destinationLocation(d.Repo, domain), path)
		})
		if i < 0 {
			continue
		}
		d := destinations[i]
		own, oErr := s.decodeCloudFor(settings, row.CredsRef)
		theirs, tErr := s.decodeCloudFor(settings, d.CredsRef)
		if oErr != nil || tErr != nil || own != theirs || row.Immutable && !d.Immutable {
			continue
		}
		row.Repo, row.DestinationID, row.Enabled = path, d.ID, true
		if _, err := s.store.UpsertPrimaryRemoteTarget(domain, row); err != nil {
			log.Printf("api: sign-ins: could not link the own folder of %s to %s: %v", domain, d.Name, err) //nolint:gosec // G706: domain is a fixed literal, the name the user's own
			continue
		}
		linked = append(linked, domain)
	}
	return linked
}

// signInFromSaved fills a draft's login fields from the saved sign-in it
// names, so the wizard tests, browses and saves with keys the page never
// holds.
func (s *Service) signInFromSaved(req *draftRequest) error {
	if req.CredsRef == "" {
		return nil
	}
	p, err := findProvider(req.Provider)
	if err != nil {
		return err
	}
	if p.Route != remotes.RouteS3 && p.Route != remotes.RouteRest {
		return errSignInNotHere
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == req.CredsRef })
	if i < 0 {
		return errUnknownSignIn
	}
	c := sets[i].CloudCreds
	if req.Settings == nil {
		req.Settings = map[string]string{}
	}
	if p.Route == remotes.RouteRest {
		req.Settings["user"], req.Settings["pass"] = c.RESTUser, c.RESTPassword
		return nil
	}
	req.Settings["access_key_id"], req.Settings["secret_access_key"] = c.S3KeyID, c.S3Secret
	if strings.TrimSpace(req.Settings["region"]) == "" {
		req.Settings["region"] = c.S3Region
	}
	return nil
}

// importSignIns stores the saved sign-ins of a settings file over the ones
// with the same id and adds the rest. A file that marks a default moves the
// mark there.
func (s *Service) importSignIns(sets []CloudCredSet) error {
	for i := range sets {
		sets[i].Name = strings.TrimSpace(sets[i].Name)
		if sets[i].ID == "" || sets[i].Name == "" {
			return fmt.Errorf("a saved sign-in in the file has no id or no name")
		}
		if err := validCloudCreds(&sets[i].CloudCreds); err != nil {
			return fmt.Errorf("saved sign-in %q: %w", sets[i].Name, err)
		}
	}
	fileDefault := defaultSetIndex(sets) >= 0
	return s.editCloudCredSets(func(stored []CloudCredSet) []CloudCredSet {
		if fileDefault {
			for i := range stored {
				stored[i].Default = false
			}
		}
		for _, c := range sets {
			i := slices.IndexFunc(stored, func(o CloudCredSet) bool { return o.ID == c.ID })
			if i < 0 {
				stored = append(stored, c)
				continue
			}
			if !fileDefault {
				c.Default = stored[i].Default
			}
			stored[i] = c
		}
		return stored
	})
}

// adoptSharedSignIn takes the shared sign-in of a settings file from before
// sign-ins were one list through the same steps as the conversion at start.
func (s *Service) adoptSharedSignIn(c CloudCreds) error {
	if err := validCloudCreds(&c); err != nil {
		return err
	}
	return s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
		out, _, _ := foldShared(sets, c)
		return out
	})
}

// signInUser is a storage location that signs in with a saved sign-in.
type signInUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// signInUsers lists, per saved sign-in, the storage locations that sign in
// with it. A location that names none signs in with the default.
func (s *Service) signInUsers(defaultID string) (map[string][]signInUser, error) {
	locations, err := s.StorageLocations()
	if err != nil {
		return nil, err
	}
	out := map[string][]signInUser{}
	add := func(ref string, l storageLocation) {
		if ref == "" {
			ref = defaultID
		}
		if ref == "" || slices.ContainsFunc(out[ref], func(u signInUser) bool { return u.ID == l.ID }) {
			return
		}
		out[ref] = append(out[ref], signInUser{ID: l.ID, Name: l.Name})
	}
	for _, l := range locations {
		if l.Backend != "s3" && l.Backend != "rest" {
			continue
		}
		if l.Object != locationPath {
			add(l.CredsRef, l)
			continue
		}
		for _, sec := range l.Sections {
			row, _ := s.primaryRemoteTarget(sec.Domain)
			add(row.CredsRef, l)
		}
	}
	return out, nil
}

// signInsNoteView is what the note on the Sign-ins card says.
type signInsNoteView struct {
	// SetName is the default sign-in the shared one became, empty when the
	// install had none or the sign-in is gone since.
	SetName    string   `json:"setName"`
	Merged     bool     `json:"merged"`
	Duplicates int      `json:"duplicates"`
	Linked     []string `json:"linked"`
}

func (s *Service) signInsNote(sets []CloudCredSet) (*signInsNoteView, error) {
	note, show, err := s.store.SignInsNoteToShow()
	if err != nil || !show {
		return nil, err
	}
	view := &signInsNoteView{Merged: note.Merged, Duplicates: note.Duplicates, Linked: note.Linked}
	if view.Linked == nil {
		view.Linked = []string{}
	}
	if i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == note.SetID }); i >= 0 {
		view.SetName = sets[i].Name
	}
	return view, nil
}

// handleDismissSignInsNote stops showing the note on the Sign-ins card.
// DELETE /api/cloud/creds-sets/note
func (h *Handler) handleDismissSignInsNote(w http.ResponseWriter, _ *http.Request) {
	if err := h.store.DismissSignInsNote(); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
