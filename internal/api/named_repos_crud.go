package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Named repositories (#204): the locations a container, a VM or a folder set can
// be pointed at instead of its domain's own repository.
//
// The issue asked to back "a VM or folder" up straight to a B2 bucket or a NAS
// share, past the domain repository. The first cut took a free-text location
// typed into each item, which works and is miserable: ten containers meant
// typing the same bucket path ten times, and correcting it later meant finding
// all ten. So the locations are written down ONCE, here, and picked per item.
//
// They are rows in offsite_targets with role = RoleRepo. That table already
// carries everything such a location needs - a repo string, a credential set, an
// S3 storage class, bandwidth limits, an enabled flag - and every query in
// internal/store/offsite_targets.go filters on an explicit role, so these rows
// are invisible to the replication loop and to the off-site CRUD by
// construction. See the RoleRepo doc comment for the full argument.

// namedRepoView is the wire shape. The location is sent back as stored, never
// resolved: the resolved form can carry a host path the browser has no business
// knowing, and the picker only needs to identify the place.
type namedRepoView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Repo          string `json:"repo"`
	CredsRef      string `json:"credsRef"`
	StorageClass  string `json:"storageClass"`
	LimitUpload   int    `json:"limitUpload"`
	LimitDownload int    `json:"limitDownload"`
	// Immutable marks the repository append-only: nothing on this box may delete
	// from it. Set here and on the Repositories card, and honoured by all six
	// gates that can destroy data: the retention after a backup, snapshot delete
	// and the three bulk deletes ask primaryIsImmutable, while prune asks
	// refAppendOnly, which reads the row the reference already carries. The field
	// was stored and read long before there was a way to set it, and then for a
	// while it could be set while the five primaryIsImmutable gates
	// short-circuited past it on a local path.
	Immutable bool `json:"immutable"`
	Enabled   bool `json:"enabled"`
	// OffPremises counts the repository as a site of its own for sites and 3-2-1
	// on the cards. It never changes what is copied.
	OffPremises bool `json:"offPremises"`
	// InUse is how many containers, VMs and folder sets currently point here.
	// The interface needs it to explain why a repository cannot be deleted
	// BEFORE the attempt, rather than only in the error afterwards.
	InUse int `json:"inUse"`
	// CompanionOf is the off-site target this repository is the direct
	// repository of, "" for a plain named repository.
	CompanionOf string `json:"companionOf"`
	// CompanionLost marks a direct repository whose target an import deleted:
	// it still holds backups, but nothing on this box links to it anymore.
	CompanionLost bool `json:"companionLost"`
}

func (h *Handler) namedRepoViews(rows []store.OffsiteTarget) []namedRepoView {
	out := make([]namedRepoView, 0, len(rows))
	for _, t := range rows {
		// A count that could not be read is reported as UNKNOWN (-1), never as
		// zero. Zero is the answer that unlocks the delete button, drops the
		// location lock and skips the switch-off confirmation - three protections
		// turned off by a failed database read, which is the one moment they are
		// least likely to be right.
		n, err := h.store.ItemsUsingNamedRepo(t.ID)
		if err != nil {
			log.Printf("api: repos: could not count the items using %q: %v", t.ID, err) //nolint:gosec // G706: the id is %q-quoted (a settings import can carry an id from the file, so it is not necessarily store-generated)
			n = -1
		}
		out = append(out, namedRepoView{
			ID: t.ID, Name: t.Name, Repo: t.Repo, CredsRef: t.CredsRef,
			StorageClass: t.StorageClass, LimitUpload: t.LimitUpload,
			LimitDownload: t.LimitDownload, Immutable: t.Immutable, Enabled: t.Enabled,
			OffPremises: t.OffPremises, InUse: n,
			CompanionOf: t.CompanionOf, CompanionLost: t.CompanionLost,
		})
	}
	return out
}

// handleListNamedRepos serves GET /api/repos.
func (h *Handler) handleListNamedRepos(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListNamedRepos()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "repos": h.namedRepoViews(rows)})
}

// namedRepoBody is the create/update payload. Pointers on update so a form that
// does not know about a field cannot clear it - the same discipline the per-item
// setters use, and for the same reason: a repository that moves by accident
// looks exactly like a working one.
type namedRepoBody struct {
	Name          *string `json:"name"`
	Repo          *string `json:"repo"`
	CredsRef      *string `json:"credsRef"`
	StorageClass  *string `json:"storageClass"`
	LimitUpload   *int    `json:"limitUpload"`
	LimitDownload *int    `json:"limitDownload"`
	Immutable     *bool   `json:"immutable"`
	Enabled       *bool   `json:"enabled"`
	// CompanionOf routes the create to handleCreateDirectRepo instead: a direct
	// repository takes everything but its name and location from the target.
	CompanionOf *string `json:"companionOf"`
	OffPremises *bool   `json:"offPremises"`
}

// applyTo merges the sent fields onto a row.
func (b namedRepoBody) applyTo(t *store.OffsiteTarget) {
	if b.Name != nil {
		t.Name = strings.TrimSpace(*b.Name)
	}
	if b.Repo != nil {
		t.Repo = strings.TrimSpace(*b.Repo)
	}
	if b.CredsRef != nil {
		t.CredsRef = strings.TrimSpace(*b.CredsRef)
	}
	if b.StorageClass != nil {
		// Upper-cased on the way in, like the off-site CRUD does it: the field is
		// now actually applied to restic's -o s3.storage-class, and "deep_archive"
		// is not a class any provider knows.
		t.StorageClass = strings.ToUpper(strings.TrimSpace(*b.StorageClass))
	} else {
		// A STORED value is upper-cased too, because the merged row is validated
		// against the allowlist. Rows created before the check existed accepted
		// "standard_ia" verbatim, and without this normalisation such a row could
		// never be edited again: a PATCH carrying only {immutable:true} would be
		// refused over a field the card does not even offer, so neither safety
		// toggle could be used and the only escape was deleting the repository.
		t.StorageClass = strings.ToUpper(strings.TrimSpace(t.StorageClass))
	}
	if b.LimitUpload != nil {
		t.LimitUpload = *b.LimitUpload
	}
	if b.LimitDownload != nil {
		t.LimitDownload = *b.LimitDownload
	}
	if b.Immutable != nil {
		t.Immutable = *b.Immutable
	}
	if b.Enabled != nil {
		t.Enabled = *b.Enabled
	}
	if b.OffPremises != nil {
		t.OffPremises = *b.OffPremises
	}
}

// staticNamedRepoRefusals holds the refusals that need nothing but the location
// string and the mount root, so the CREATE form and the settings IMPORT apply
// exactly the same ones. It returns a user-facing sentence, or "" when the
// location passes.
//
// It exists because the import used to mirror two of the four checks by hand.
// A shared function cannot drift; a copy always does, and here the drift let a
// file install a location the form refuses.
//
// The messages carry NO SLASH: every error leaving the API goes through
// scrubError, whose absPathRe redacts any slash-led token, so an example path
// in one of these arrives unusable. Measured in the browser, twice.
func staticNamedRepoRefusals(loc, mountRoot string) string {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return "a repository needs a location: a folder under the host mount, or a restic remote such as b2:bucket"
	}
	// "BackBlaze:bucket/cold" is an rclone REMOTE NAME, not a restic location:
	// restic does not know that scheme, so it would be taken for a relative path
	// and the repository would be created as a folder literally called
	// "BackBlaze:bucket" under the mount root. The backups would work, they would
	// simply be in the wrong place - which is only discovered by looking.
	if restic.LooksLikeUnprefixedRemote(loc) {
		return "this location looks like a remote but carries no restic prefix. An rclone remote needs one: write rclone: in front of the remote name, as in rclone:myremote:bucket"
	}
	// A local location must stay under the mount root, the same rule the settings
	// paths are held to on save.
	if !restic.IsRemoteRepo(loc) && mountRoot != "" {
		if _, err := paths.Resolve(mountRoot, loc); err != nil {
			return scrubError(err)
		}
	}
	return ""
}

// validateNamedRepo refuses a row the next backup could not use. The location is
// resolved here rather than at backup time for the same reason the per-item
// choice is: a location that only fails when a run happens fails inside a run
// record, hours later, with nobody watching.
// checkClass says whether this request actually CARRIED a storage class. A PATCH
// that does not mention the field must not be refused over a value stored before
// the allowlist existed: the Repositories card has no storage-class input, so
// such a row could otherwise never be edited again, and the two safety toggles
// that ARE on the card would be unreachable on exactly the repository somebody
// most wants to protect.
// checkLocation is true for a create and for an edit that moves the location,
// the same scoping handleUpdateOffsiteTarget uses. A row that already sits
// inside another place, which an import can install, would otherwise be refused
// every edit over a field the request does not touch, down to switching it off.
func (h *Handler) validateNamedRepo(t store.OffsiteTarget, checkClass, checkLocation bool) error {
	if t.Name == "" {
		return errors.New("a repository needs a name, so it can be told apart in the picker")
	}
	// The location refusals that need nothing but the string and the mount root,
	// shared with the settings import so the two cannot disagree.
	if msg := staticNamedRepoRefusals(t.Repo, h.cfg.HostMountRoot); msg != "" {
		return errors.New(msg)
	}
	// The same class whitelist the off-site destinations are held to. The field is
	// applied to restic's -o s3.storage-class now, so an unsupported value stops
	// being a harmless label and becomes a backup that fails at the provider.
	if checkClass && t.StorageClass != "" && !restic.StorageClassAllowed(t.StorageClass) {
		return fmt.Errorf("unsupported storage class %s (allowed: %s)", t.StorageClass, strings.Join(restic.AllowedStorageClasses, ", "))
	}
	loc, err := h.svc.resolveRepo(t.Repo)
	if err != nil {
		return err
	}
	if !checkLocation {
		return nil
	}
	// A named repository may not sit on, in or around a domain's own repository,
	// a domain's off-site destination, an off-site target or another named
	// repository. Two rows over one place, or one inside the other, would give
	// every question about that place two answers, and a destination that is
	// really a named repository would take an item's only copy while
	// replication still treats it as a second one.
	//
	// A settings read that fails takes the whole validation with it rather than
	// dropping the guard: a refusal that quietly does not apply when the database
	// hiccups is not a refusal.
	settings, sErr := h.store.GetSettings()
	if sErr != nil {
		return fmt.Errorf("read settings to check this location: %w", sErr)
	}
	return h.svc.locationClash(settings, loc, locationSelf{ids: []string{t.ID}})
}

// sameRepoLocation reports whether two RESOLVED locations name the same place.
// It is the ONE answer to that question in this package: the duplicate refusal,
// the domain-own refusal, the reciprocal refusal on an off-site destination, the
// identity of a domainRepoRef, namedRepoForLocation and the off-site self-copy
// guard all ask it here.
//
// That it is one function is the point. The same question used to be answered
// three ways - a raw ==, this normalisation, and a path.Clean somewhere else -
// so one physical repository could be "the same place" to one guard and a
// different place to the next, which is how a named repository got past a
// refusal that was written precisely to stop it.
//
// A REMOTE location is a URL-ish string: trailing slashes carry no meaning and
// the scheme is case-insensitive, so "B2:bucket/cold/" and "b2:bucket/cold" are
// one place. A LOCAL path gets neither treatment - it is cleaned and compared as
// it is. Case-folding anything in a local path was a bug: it made
// /mnt/user/Archiv and /mnt/user/archiv the same place on a case-sensitive
// filesystem, where they are two directories.
//
// KNOWN LIMIT, stated rather than papered over: on Unraid one directory is
// reachable under several spellings - user/..., user0/... and diskN/... are all
// legal paths to the same files. This compares strings, so it calls those three
// different places. Answering correctly needs identity the string does not
// carry (device plus inode), which is a bigger change than this guard; until
// then a second repository registered under an alias of an existing one gets
// past every refusal built on this function.
func sameRepoLocation(a, b string) bool {
	norm := func(loc string) string {
		if restic.IsRemoteRepo(loc) {
			loc = strings.TrimRight(loc, "/")
			if i := strings.IndexByte(loc, ':'); i > 0 {
				return strings.ToLower(loc[:i]) + loc[i:]
			}
			return loc
		}
		return filepath.Clean(loc)
	}
	return norm(a) == norm(b)
}

// handleCreateNamedRepo serves POST /api/repos.
func (h *Handler) handleCreateNamedRepo(w http.ResponseWriter, r *http.Request) {
	var body namedRepoBody
	if !decodeBody(w, r, &body) {
		return
	}
	if body.CompanionOf != nil && strings.TrimSpace(*body.CompanionOf) != "" {
		h.handleCreateDirectRepo(w, r, body)
		return
	}
	// A new repository is ON unless the caller says otherwise: somebody who just
	// created one means to use it.
	row := store.OffsiteTarget{Role: store.RoleRepo, Enabled: true}
	body.applyTo(&row)
	if body.OffPremises == nil {
		row.OffPremises = restic.IsRemoteRepo(row.Repo)
	}
	if err := h.validateNamedRepo(row, body.StorageClass != nil, true); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	saved, err := h.store.UpsertOffsiteTarget(row)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "repo": h.namedRepoViews([]store.OffsiteTarget{saved})[0]})
}

// handleUpdateNamedRepo serves PATCH /api/repos/{id}.
//
// The LOCATION of a repository that is already in use cannot be changed. Moving
// it would leave every item pointing at a different place while its snapshots
// stay where they were written, and the next backup would succeed into an empty
// repository - indistinguishable from a working backup until somebody goes
// looking for a snapshot. The name, the limits and the on/off switch stay
// editable, because none of those move any data.
func (h *Handler) handleUpdateNamedRepo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	row, err := h.store.GetNamedRepo(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no such repository"})
		return
	}
	var body namedRepoBody
	if !decodeBody(w, r, &body) {
		return
	}
	if body.CompanionOf != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false,
			"error": "a repository is tied to a target when it is created or connected, not by an edit"})
		return
	}
	if row.CompanionOf != "" {
		if fields := mirroredFieldsChanged(body, row); len(fields) > 0 {
			placementFail(w, errMirroredField, map[string]any{"fields": fields})
			return
		}
	}
	current := strings.TrimSpace(row.Repo)
	moving := body.Repo != nil && strings.TrimSpace(*body.Repo) != current
	body.applyTo(&row)
	if err := h.validateNamedRepo(row, body.StorageClass != nil, moving); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// The everything-but-the-location write goes FIRST, with the location left as
	// it is; the move follows under the store's count-and-write transaction.
	//
	// The order matters. Moving first and upserting second meant a failed upsert
	// left the repository already moved while the response said the edit had
	// failed - the one inconsistency this endpoint must not produce, since a
	// moved location is exactly what makes the next backup land in an empty
	// repository. This way every failure leaves a state somebody can read: the
	// location is only ever moved by the last statement, and a refusal of it
	// leaves the other fields saved and the location where it was.
	newLocation := row.Repo
	row.Repo = current
	saved, err := h.store.UpsertOffsiteTarget(row)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if moving {
		use, mErr := h.store.SetNamedRepoLocationIfUnused(id, newLocation)
		if mErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(mErr))
			return
		}
		if use.InUse() {
			// The rest of the edit IS saved - name, limits, flags - and the answer
			// says so. Reporting a bare failure over a request that did write
			// something leaves the operator with a screen that disagrees with the
			// database, which is how a "failed" save gets repeated until it does
			// something unintended.
			placementFail(w, errRepoInUse, map[string]any{
				"error":          "this repository is in use, so its location was NOT moved; the backups already written stay where they are. Everything else you changed was saved. Create a second repository and point the items at it instead",
				"items":          use.Items,
				"defaultDomains": append([]string{}, use.DefaultDomains...),
				"repo":           h.namedRepoViews([]store.OffsiteTarget{saved})[0],
			})
			return
		}
		saved.Repo = newLocation
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "repo": h.namedRepoViews([]store.OffsiteTarget{saved})[0]})
}

// handleDeleteNamedRepo serves DELETE /api/repos/{id}.
//
// Refused while anything still points here. Deleting would put those items back
// on their domain repository silently, and their next backup would land
// somewhere else and look exactly like a working backup. A direct repository is
// refused outright and the answer names its target, which is where it goes.
func (h *Handler) handleDeleteNamedRepo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	row, err := h.store.GetNamedRepo(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "no such repository"})
		return
	}
	// Counted and deleted in ONE transaction, so an item that starts pointing
	// here while this request is in flight blocks the delete instead of being
	// put back on its domain repository without a word.
	use, err := h.store.DeleteNamedRepoIfUnused(id)
	if errors.Is(err, store.ErrDirectRepo) {
		placementFail(w, err, map[string]any{"target": h.offsiteTargetRef(row.CompanionOf)})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if use.InUse() {
		placementFail(w, errRepoInUse, map[string]any{
			"items":          use.Items,
			"defaultDomains": append([]string{}, use.DefaultDomains...),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
