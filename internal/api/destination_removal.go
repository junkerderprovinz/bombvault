package api

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// destinationRemoval is what removing a destination means for every section
// that copies to it. Nothing at the destination is touched: the backups there
// stay as they are and are no longer copied to, listed or aged.
type destinationRemoval struct {
	Sections []removalSection `json:"sections"`
	// Erasure is set on the removal an erasure preview carries: a digest of
	// what deleting the backups there takes, sections left behind included.
	Erasure string `json:"erasure,omitempty"`
}

// removalSection is one section's part of a destination. Entries and Backups
// are what the last listing of its repository there found, ListedAt when that
// was, 0 for never.
type removalSection struct {
	Domain   string `json:"domain"`
	TargetID string `json:"targetId"`
	Repo     string `json:"repo"`
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

// same reports whether two previews name the same sections and keep the same
// direct repositories for the same number of entries.
func (r destinationRemoval) same(o destinationRemoval) bool {
	return slices.EqualFunc(r.Sections, o.Sections, func(a, b removalSection) bool {
		if a.TargetID != b.TargetID || (a.Direct == nil) != (b.Direct == nil) {
			return false
		}
		return a.Direct == nil || (a.Direct.RepoID == b.Direct.RepoID && a.Direct.Items == b.Direct.Items && a.Direct.Kept == b.Direct.Kept)
	})
}

func (s *Service) destinationRemovalPreview(id string) (store.OffsiteTarget, destinationRemoval, error) {
	d, ok, err := s.store.GetDestination(id)
	if err != nil {
		return store.OffsiteTarget{}, destinationRemoval{}, err
	}
	if !ok {
		return store.OffsiteTarget{}, destinationRemoval{}, store.ErrNotDestination
	}
	held, err := s.store.DestinationTargets(id)
	if err != nil {
		return store.OffsiteTarget{}, destinationRemoval{}, err
	}
	out := destinationRemoval{Sections: make([]removalSection, 0, len(held))}
	for _, t := range held {
		section, err := s.removalSection(t)
		if err != nil {
			return store.OffsiteTarget{}, destinationRemoval{}, err
		}
		out.Sections = append(out.Sections, section)
	}
	return d, out, nil
}

func (s *Service) removalSection(t store.OffsiteTarget) (removalSection, error) {
	section := removalSection{
		Domain: t.Domain, TargetID: t.ID, Repo: scrubRepoLocation(t.Repo),
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
type removalChangedErr struct{ now destinationRemoval }

func (e *removalChangedErr) Error() string { return errPlacementStale.Error() }

func (e *removalChangedErr) Is(target error) bool { return target == errPlacementStale }

// RemoveDestination takes a destination away together with the targets of the
// sections that copy to it, after checking that they are the ones expect
// lists. The data there stays. The remote or credential set the destination
// brought along goes too, unless a repository that is kept still needs it.
func (s *Service) RemoveDestination(id string, expect destinationRemoval) (store.Settings, error) {
	d, now, err := s.destinationRemovalPreview(id)
	if err != nil {
		return store.Settings{}, err
	}
	if !now.same(expect) {
		return store.Settings{}, &removalChangedErr{now: now}
	}
	if err := s.store.RemoveDestination(id, func(st *store.Settings, primaries []string) {
		for _, domain := range primaries {
			setOffsiteRepoInSettings(st, domain, "")
			setOffsiteImmutableInSettings(st, domain, false)
		}
	}); err != nil {
		return store.Settings{}, err
	}
	s.dropDestinationSignIn(d)
	return s.store.GetSettings()
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
		Expect destinationRemoval `json:"expect"`
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
