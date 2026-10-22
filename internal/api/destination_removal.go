package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// locationRemoval is what removing a storage location means for every section
// that uses it. Nothing at the location is touched: the backups there stay as
// they are and are no longer written to, listed or aged.
type locationRemoval struct {
	Sections []removalSection `json:"sections"`
	// Erasure is set on the removal an erasure preview carries: a digest of
	// the snapshots deleting the backups there takes, sections left behind
	// included.
	Erasure string `json:"erasure,omitempty"`
	// Suggest is the folder a section folder's removal offers its sections
	// first.
	Suggest string `json:"suggest,omitempty"`
}

// Uses of a location by a section in a removal.
const (
	removalCopy = "copy"
	removalHome = "home"
	removalRepo = "repo"
)

// removalSection is one section's part of a location. Entries and Backups
// are what the last listing of its repository there found, ListedAt when that
// was, 0 for never.
type removalSection struct {
	Domain   string `json:"domain"`
	Use      string `json:"use"`
	TargetID string `json:"targetId"`
	Repo     string `json:"repo"`
	// Items and Default say what of a section backs up to a named repository:
	// that many entries, and its default.
	Items   int  `json:"items,omitempty"`
	Default bool `json:"default,omitempty"`
	// Folder is the last segment of a section's own folder, which it keeps
	// under the folder it moves to.
	Folder string `json:"folder,omitempty"`
	// Primary marks the section's built-in off-site copy, whose field in the
	// settings is cleared with it.
	Primary    bool  `json:"primary"`
	AppendOnly bool  `json:"appendOnly"`
	Entries    int   `json:"entries"`
	Backups    int   `json:"backups"`
	ListedAt   int64 `json:"listedAt"`
	LastCopyAt int64 `json:"lastCopyAt"`
	// Direct is the repository at the destination that entries back up to
	// without a copy at home, when the section has one.
	Direct *removalDirect `json:"direct,omitempty"`
}

// removalDirect is a target's direct repository in a removal. Kept means an
// entry or a default still backs up to it, so it stays as a repository of its
// own and those backups go on.
type removalDirect struct {
	RepoID         string   `json:"repoId"`
	Name           string   `json:"name"`
	Items          int      `json:"items"`
	DefaultDomains []string `json:"defaultDomains"`
	Kept           bool     `json:"kept"`
}

// same reports whether two previews name the same sections, send the same
// entries elsewhere and keep the same direct repositories for the same number
// of entries.
func (r locationRemoval) same(o locationRemoval) bool {
	return slices.EqualFunc(r.Sections, o.Sections, func(a, b removalSection) bool {
		if a.Domain != b.Domain || a.Use != b.Use || a.TargetID != b.TargetID || a.Repo != b.Repo ||
			a.Items != b.Items || a.Default != b.Default || (a.Direct == nil) != (b.Direct == nil) {
			return false
		}
		return a.Direct == nil || (a.Direct.RepoID == b.Direct.RepoID && a.Direct.Items == b.Direct.Items && a.Direct.Kept == b.Direct.Kept)
	})
}

func (s *Service) destinationRemovalPreview(id string) (store.OffsiteTarget, locationRemoval, error) {
	d, ok, err := s.store.GetDestination(id)
	if err != nil {
		return store.OffsiteTarget{}, locationRemoval{}, err
	}
	if !ok {
		return store.OffsiteTarget{}, locationRemoval{}, store.ErrNotDestination
	}
	if err := s.refuseOwnFoldersIn(id); err != nil {
		return store.OffsiteTarget{}, locationRemoval{}, err
	}
	held, err := s.store.DestinationTargets(id)
	if err != nil {
		return store.OffsiteTarget{}, locationRemoval{}, err
	}
	out := locationRemoval{Sections: make([]removalSection, 0, len(held))}
	for _, t := range held {
		section, err := s.removalSection(t)
		if err != nil {
			return store.OffsiteTarget{}, locationRemoval{}, err
		}
		out.Sections = append(out.Sections, section)
	}
	return d, out, nil
}

// errOwnFolderThere refuses to remove a storage location that a section's own
// folder lies in. Its backups would go on there with nothing listing the place.
var errOwnFolderThere = errors.New("a section's own folder lies in this storage location; move it elsewhere first")

// refuseOwnFoldersIn answers errOwnFolderThere while a section's own folder
// was put into the destination with the id and is still there.
func (s *Service) refuseOwnFoldersIn(id string) error {
	for _, domain := range offsiteConfigDomains {
		row, ok, err := s.store.PrimaryRemoteTarget(domain)
		if err != nil {
			return err
		}
		if d, linked := s.ownFolderLocation(row); ok && linked && d.ID == id {
			return errOwnFolderThere
		}
	}
	return nil
}

func (s *Service) removalSection(t store.OffsiteTarget) (removalSection, error) {
	section := removalSection{
		Domain: t.Domain, Use: removalCopy, TargetID: t.ID, Repo: scrubRepoLocation(t.Repo),
		Primary: t.SortOrder == 0, AppendOnly: t.Immutable,
	}
	copies, err := s.store.ItemCopiesForDomain(t.Domain)
	if err != nil {
		return removalSection{}, err
	}
	for _, c := range copies {
		if c.TargetID == t.ID {
			section.Entries++
			section.Backups += c.SnapshotCount
		}
	}
	listing, _, err := s.store.TargetObservationFor(t.Domain, t.ID)
	if err != nil {
		return removalSection{}, err
	}
	section.ListedAt = listing.ListedAt
	last, _, err := s.store.LatestSuccessfulOffsiteRunForTarget(t.Domain, t.ID)
	if err != nil {
		return removalSection{}, err
	}
	section.LastCopyAt = last.FinishedAt

	direct, found, err := s.store.CompanionFor(t.ID)
	if err != nil || !found {
		return section, err
	}
	use, err := s.namedRepoUse(direct.ID)
	if err != nil {
		return removalSection{}, err
	}
	section.Direct = &removalDirect{
		RepoID: direct.ID, Name: s.repoName(direct), Items: use.Items,
		DefaultDomains: slices.Clone(use.DefaultDomains), Kept: use.InUse(),
	}
	if section.Direct.DefaultDomains == nil {
		section.Direct.DefaultDomains = []string{}
	}
	return section, nil
}

// removalChangedErr is the refusal of a removal whose sections are no longer
// the ones that were shown. It carries them as they are.
type removalChangedErr struct{ now locationRemoval }

func (e *removalChangedErr) Error() string { return errPlacementStale.Error() }

func (e *removalChangedErr) Is(target error) bool { return target == errPlacementStale }

// RemoveDestination takes a destination away together with the targets of the
// sections that copy to it, after checking that they are the ones expect
// lists, and the switched-off rows that cleared off-site fields left on its
// places, which no page lists. The data there stays. The remote or credential set the destination
// brought along goes too, unless a repository that is kept still needs it.
func (s *Service) RemoveDestination(id string, expect locationRemoval) (store.Settings, error) {
	d, now, err := s.destinationRemovalPreview(id)
	if err != nil {
		return store.Settings{}, err
	}
	if !now.same(expect) {
		return store.Settings{}, &removalChangedErr{now: now}
	}
	behind := func(t store.OffsiteTarget, st store.Settings) bool {
		return t.DestinationID == "" && hiddenTarget(t, st, nil) && (repoUnder(d.Repo, t.Repo) || sameRepoLocation(d.Repo, t.Repo))
	}
	if err := s.store.RemoveDestination(id, behind, clearOffsiteFields); err != nil {
		return store.Settings{}, err
	}
	s.dropDestinationSignIn(d)
	return s.store.GetSettings()
}

// clearOffsiteFields empties the off-site field of each domain whose primary
// target went, so a settings save cannot bring the copy back.
func clearOffsiteFields(st *store.Settings, primaries []string) {
	for _, domain := range primaries {
		setOffsiteRepoInSettings(st, domain, "")
		setOffsiteImmutableInSettings(st, domain, false)
	}
}

// dropDestinationSignIn removes the rclone remote or the credential set a
// deleted destination brought along, once nothing refers to it any more.
func (s *Service) dropDestinationSignIn(d store.OffsiteTarget) {
	if remote, ok := strings.CutPrefix(d.Repo, "rclone:"); ok {
		if name, _, ok := strings.Cut(remote, ":"); ok {
			s.removeRcloneRemote(name)
		}
	}
	if d.CredsRef != "" {
		s.removeCredSet(d.CredsRef)
	}
}

// handleDestinationRemovalPreview says what removing a destination would
// detach.
// GET /api/offsite/destinations/{id}/removal
func (h *Handler) handleDestinationRemovalPreview(w http.ResponseWriter, r *http.Request) {
	_, removal, err := h.svc.destinationRemovalPreview(r.PathValue("id"))
	if errors.Is(err, store.ErrNotDestination) {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"removal": removal}))
}

// handleRemoveDestination removes a destination that sections still copy to.
// The body carries the preview that was shown; when the sections have changed
// since, nothing is removed and the answer carries them as they are.
// POST /api/offsite/destinations/{id}/removal
func (h *Handler) handleRemoveDestination(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Expect locationRemoval `json:"expect"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	settings, err := h.svc.RemoveDestination(r.PathValue("id"), body.Expect)
	var changed *removalChangedErr
	switch {
	case errors.Is(err, store.ErrNotDestination):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
		return
	case errors.As(err, &changed):
		placementFail(w, err, map[string]any{"removal": changed.now})
		return
	case err != nil:
		placementFail(w, err, nil)
		return
	}
	// A removed primary target took the section's off-site settings with it,
	// and the scheduler reads those.
	if err := h.scheduler.ReloadWithGates(settings, h.dueGates()); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
