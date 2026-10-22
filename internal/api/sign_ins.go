package api

import (
	"context"
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
	errUnknownSignIn      = errors.New("that saved sign-in does not exist any more")
	errSignInNotHere      = errors.New("a saved sign-in serves S3 services and restic REST servers only")
	errDefaultSignInStays = errors.New("this is the default sign-in: make another one the default first")
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
	// The region belongs to the sign-in, and restic signs every request of
	// the destination with it, whatever region the form had when it was
	// tested.
	typed := strings.TrimSpace(req.Settings["region"])
	if typed != "" && c.S3Region != "" && typed != c.S3Region {
		return fmt.Errorf("the saved sign-in %s signs in for the region %s, not %s: use new keys here, or a saved sign-in for %s", sets[i].Name, c.S3Region, typed, typed)
	}
	if typed == "" {
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

// MakeDefaultSignIn moves the default mark to the saved sign-in id in one
// write, so no reader ever sees two defaults or none.
func (s *Service) MakeDefaultSignIn(id string) error {
	missing := false
	err := s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
		if !slices.ContainsFunc(sets, func(c CloudCredSet) bool { return c.ID == id }) {
			missing = true
			return sets
		}
		for i := range sets {
			sets[i].Default = sets[i].ID == id
		}
		return sets
	})
	if err == nil && missing {
		return errUnknownSignIn
	}
	return err
}

// RemoveSignIn removes a saved sign-in nothing uses. The default goes only as
// the last one, since every place that names no sign-in falls back to it.
func (s *Service) RemoveSignIn(id string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == id })
	if i < 0 {
		return errUnknownSignIn
	}
	if sets[i].Default && len(sets) > 1 {
		return errDefaultSignInStays
	}
	defaultID := ""
	if d := defaultSetIndex(sets); d >= 0 {
		defaultID = sets[d].ID
	}
	users, err := s.signInUsers(defaultID)
	if err != nil {
		return err
	}
	if len(users[id]) > 0 {
		names := make([]string, len(users[id]))
		for j, u := range users[id] {
			names[j] = u.Name
		}
		return fmt.Errorf("%s still signs in with it", strings.Join(names, ", "))
	}
	// A row the list leaves out, such as a hidden target, would sign in with
	// the default without a word.
	inUse, err := s.store.CredsRefsInUse()
	if err != nil {
		return err
	}
	if inUse[id] {
		return fmt.Errorf("%s is still in use, so it stays", sets[i].Name)
	}
	return s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
		return slices.DeleteFunc(sets, func(c CloudCredSet) bool { return c.ID == id })
	})
}

// handleRemoveSignIn removes a saved sign-in nothing uses.
// DELETE /api/cloud/creds-sets/{id}
func (h *Handler) handleRemoveSignIn(w http.ResponseWriter, r *http.Request) {
	before, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.RemoveSignIn(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.signInsSaved(w, r, before)
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

// signInUse is a storage location that signs in with a saved sign-in, and
// for a domain path location the section whose folder does.
type signInUse struct {
	ref      string
	location storageLocation
	domain   string
}

// signInUses lists who signs in with which saved sign-in. A place that names
// none signs in with the default.
func (s *Service) signInUses(defaultID string) ([]signInUse, error) {
	locations, err := s.StorageLocations()
	if err != nil {
		return nil, err
	}
	var out []signInUse
	add := func(ref string, l storageLocation, domain string) {
		if ref == "" {
			ref = defaultID
		}
		if ref != "" {
			out = append(out, signInUse{ref: ref, location: l, domain: domain})
		}
	}
	for _, l := range locations {
		if l.Backend != "s3" && l.Backend != "rest" {
			continue
		}
		if l.Object != locationPath {
			add(l.CredsRef, l, "")
			continue
		}
		for _, sec := range l.Sections {
			row, _ := s.primaryRemoteTarget(sec.Domain)
			add(row.CredsRef, l, sec.Domain)
		}
	}
	return out, nil
}

// signInUsers lists, per saved sign-in, the storage locations and the pull
// sources that sign in with it.
func (s *Service) signInUsers(defaultID string) (map[string][]signInUser, error) {
	uses, err := s.signInUses(defaultID)
	if err != nil {
		return nil, err
	}
	out := map[string][]signInUser{}
	for _, u := range uses {
		if !slices.ContainsFunc(out[u.ref], func(x signInUser) bool { return x.ID == u.location.ID }) {
			out[u.ref] = append(out[u.ref], signInUser{ID: u.location.ID, Name: u.location.Name})
		}
	}
	sources, err := s.store.ListPullSources()
	if err != nil {
		return nil, err
	}
	for _, ps := range sources {
		if ref := strings.TrimSpace(ps.CredsRef); ref != "" {
			out[ref] = append(out[ref], signInUser{ID: "pull:" + ps.ID, Name: ps.Name})
		}
	}
	return out, nil
}

// signInCheck is what a test of an edited sign-in found at one storage
// location.
type signInCheck struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Reachable   bool   `json:"reachable"`
	Initialized bool   `json:"initialized"`
	Error       string `json:"error,omitempty"`
}

// TestSignIn signs in with an edited saved sign-in, before it is saved, to
// every storage location that uses it. A secret left blank is the stored one.
func (s *Service) TestSignIn(ctx context.Context, edited CloudCredSet) ([]signInCheck, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(sets, func(c CloudCredSet) bool { return c.ID == edited.ID })
	if i < 0 {
		return nil, errUnknownSignIn
	}
	creds := edited.CloudCreds
	if creds.S3Secret == "" {
		creds.S3Secret = sets[i].S3Secret
	}
	if creds.RESTPassword == "" {
		creds.RESTPassword = sets[i].RESTPassword
	}
	if err := validCloudCreds(&creds); err != nil {
		return nil, err
	}
	defaultID := ""
	if d := defaultSetIndex(sets); d >= 0 {
		defaultID = sets[d].ID
	}
	uses, err := s.signInUses(defaultID)
	if err != nil {
		return nil, err
	}
	checks := []signInCheck{}
	for _, u := range uses {
		if u.ref != edited.ID || slices.ContainsFunc(checks, func(c signInCheck) bool { return c.ID == u.location.ID }) {
			continue
		}
		check := signInCheck{ID: u.location.ID, Name: u.location.Name}
		var probeErr error
		check.Reachable, check.Initialized, probeErr = s.probeSignInUse(ctx, settings, u, creds)
		check.Error = scrubError(probeErr)
		checks = append(checks, check)
	}
	return checks, nil
}

// probeSignInUse opens the repository one use of a sign-in reaches, signed
// in with creds instead of the sign-in's own.
func (s *Service) probeSignInUse(ctx context.Context, settings store.Settings, u signInUse, creds CloudCreds) (reachable, initialized bool, err error) {
	loc, row, err := s.signInRepo(u, settings)
	if err != nil {
		return false, false, err
	}
	repo, err := s.resolveRepo(loc)
	if err != nil {
		return false, false, err
	}
	mode := s.offsiteModeForTarget(settings, row)
	mode.Env = cloudEnv(creds)
	reachable, initialized, err = s.probeOffsiteRepo(ctx, repo, mode)
	// The sign-in opened the repository, which is all this asks.
	if _, other := errors.AsType[*otherEncryptionErr](err); other {
		err = nil
	}
	return reachable, initialized, err
}

// signInRepo is the repository a use of a sign-in reaches, and the row whose
// settings it is opened with. A destination is reached through the folder of
// one of its sections, since its own folder holds no repository.
func (s *Service) signInRepo(u signInUse, settings store.Settings) (string, store.OffsiteTarget, error) {
	_, id, _ := strings.Cut(u.location.ID, ":")
	switch u.location.Object {
	case locationPath:
		row, _ := s.primaryRemoteTarget(u.domain)
		return domainPathRaw(u.domain, settings), row, nil
	case locationRepo:
		r, err := s.store.GetNamedRepo(id)
		return r.Repo, r, err
	case locationDestination:
		i := slices.IndexFunc(u.location.Sections, func(sec locationSection) bool { return sec.Use == useCopy })
		if i < 0 {
			d, ok, err := s.store.GetDestination(id)
			if err == nil && !ok {
				err = store.ErrNotDestination
			}
			return d.Repo, d, err
		}
		id = u.location.Sections[i].TargetID
	}
	t, ok, err := s.store.GetOffsiteTarget(id)
	if err == nil && !ok {
		err = store.ErrNotOffsiteTarget
	}
	return t.Repo, t, err
}

// handleTestSignIn tests an edited saved sign-in at every storage location
// that uses it, without saving it.
// POST /api/cloud/creds-sets/{id}/test
func (h *Handler) handleTestSignIn(w http.ResponseWriter, r *http.Request) {
	var edited CloudCredSet
	if !decodeBody(w, r, &edited) {
		return
	}
	edited.ID = r.PathValue("id")
	checks, err := h.svc.TestSignIn(r.Context(), edited)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"locations": checks}))
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
