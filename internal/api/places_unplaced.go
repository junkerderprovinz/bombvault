package api

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var errUnplacedRowMissing = errors.New("no such row without a place")

// unplacedPath is a domain path without a home place.
func (s *Service) unplacedPath(d placeData, domain string) (UnplacedRow, error) {
	row := UnplacedRow{Domain: domain, Role: "path", Repo: domainPathRaw(domain, d.settings)}
	if restic.IsRemoteRepo(row.Repo) {
		primary, found, err := s.store.PrimaryRemoteTarget(domain)
		if err != nil {
			return row, err
		}
		row.Immutable = found && primary.Enabled && primary.Immutable
	}
	var err error
	row.Items, err = s.pathItems(d, domain)
	return row, err
}

// pathItems counts the items whose backups go to the domain's own path. Flash
// and config are one item each, and a ZFS item goes there unless it names a
// repository; an item of another domain goes there while neither it nor the
// default it waits on names a repository.
func (s *Service) pathItems(d placeData, domain string) (int, error) {
	switch domain {
	case "flash", "config":
		return 1, nil
	case zfsDomain:
		items, err := s.store.ListZFSDatasets()
		if err != nil {
			return 0, err
		}
		n := 0
		for _, it := range items {
			if it.Repo == "" {
				n++
			}
		}
		return n, nil
	}
	defaultHome := ""
	for _, def := range d.defaults {
		if def.Domain == domain {
			defaultHome = def.Home
		}
	}
	items, err := s.domainItems(domain)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, it := range items {
		repo := it.home.Repo
		if it.home.Choice == store.RepoOpen {
			repo = defaultHome
		}
		if repo == "" {
			n++
		}
	}
	return n, nil
}

// copiedItems counts the items the target held copies of when it was last
// listed.
func (s *Service) copiedItems(t store.OffsiteTarget) (int, error) {
	copies, err := s.store.ItemCopiesForDomain(t.Domain)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range copies {
		if c.TargetID == t.ID {
			n++
		}
	}
	return n, nil
}

// protectable reports whether a row without a place takes an append-only
// switch. A direct repository follows its target, and nothing keeps a local
// address from deletion, though a flag already on there can still go off.
func protectable(r UnplacedRow) bool {
	return r.Role != "direct" && (restic.IsRemoteRepo(r.Repo) || r.Immutable)
}

// unplacedSwitchBody names a row without a place as adoptBody does, and the
// append-only flag it gets.
type unplacedSwitchBody struct {
	RowID     string `json:"rowId"`
	Domain    string `json:"domain"`
	Immutable bool   `json:"immutable"`
}

// setUnplacedAppendOnly switches the append-only flag of a row without a
// place; a place writes the flag of its own rows. It holds placeEditMu, so no
// adoption puts the row at a place in between.
func (s *Service) setUnplacedAppendOnly(body unplacedSwitchBody) error {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	d, err := s.readPlaceData()
	if err != nil {
		return err
	}
	rows, err := s.unplacedRows(d)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(rows, func(r UnplacedRow) bool {
		return r.RowID == body.RowID && (r.RowID != "" || r.Domain == body.Domain)
	})
	if i < 0 {
		return errUnplacedRowMissing
	}
	row := rows[i]
	switch {
	case row.Role == "direct":
		return errMirroredField
	case !restic.IsRemoteRepo(row.Repo) && (body.Immutable || row.Role == "path"):
		return errLocalAppendOnly
	case row.Role == "path":
		cfg, _, err := s.store.PrimaryRemoteTarget(row.Domain)
		if err != nil {
			return err
		}
		cfg.Immutable = body.Immutable
		_, err = s.SetPrimaryRemoteConfig(row.Domain, cfg)
		return err
	}
	_, err = s.store.SetUnplacedImmutable(row.RowID, body.Immutable)
	return err
}

// handlePatchUnplaced serves PATCH /api/places/unplaced.
func (h *Handler) handlePatchUnplaced(w http.ResponseWriter, r *http.Request) {
	var body unplacedSwitchBody
	if !decodeBody(w, r, &body) {
		return
	}
	err := h.svc.setUnplacedAppendOnly(body)
	switch {
	case errors.Is(err, errUnplacedRowMissing), errors.Is(err, sql.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": errUnplacedRowMissing.Error()})
	case err != nil:
		placementFail(w, err, nil)
	default:
		writeJSON(w, http.StatusOK, okEnvelope(nil))
	}
}
