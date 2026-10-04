package api

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/remotes"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// destinationView is a destination as the off-site screens show it.
type destinationView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider"`
	Mark         string   `json:"mark,omitempty"`
	Repo         string   `json:"repo"`
	CredsRef     string   `json:"credsRef"`
	StorageClass string   `json:"storageClass"`
	Immutable    bool     `json:"immutable"`
	CreatedAt    int64    `json:"createdAt"`
	Domains      []string `json:"domains"`
}

func (s *Service) destinationView(d store.OffsiteTarget) (destinationView, error) {
	derived, err := s.store.DestinationTargets(d.ID)
	if err != nil {
		return destinationView{}, err
	}
	domains := make([]string, 0, len(derived))
	for _, t := range derived {
		domains = append(domains, t.Domain)
	}
	return destinationView{
		ID: d.ID, Name: d.Name, Provider: d.Provider, Mark: providerMark(d.Provider), Repo: scrubRepoLocation(d.Repo),
		CredsRef: d.CredsRef, StorageClass: d.StorageClass, Immutable: d.Immutable,
		CreatedAt: d.CreatedAt, Domains: domains,
	}, nil
}

// draftRequest is a destination as the wizard has it before saving.
type draftRequest struct {
	Provider string            `json:"provider"`
	Settings map[string]string `json:"settings"`
	// Dir is the folder a listing shows, a new folder goes into, or the
	// destination is saved at.
	Dir string `json:"dir"`
	// Folder is the name of a new folder.
	Folder       string `json:"folder"`
	Name         string `json:"name"`
	StorageClass string `json:"storageClass"`
	Immutable    bool   `json:"immutable"`
}

var errNotThroughRclone = errors.New("this kind of destination is not reached through rclone")

func findProvider(id string) (remotes.Provider, error) {
	p, ok := remotes.FindProvider(id)
	if !ok {
		return remotes.Provider{}, fmt.Errorf("unknown provider %q", id)
	}
	return p, nil
}

// sshKeyPath is the private key BombVault signs into hosts with.
func (s *Service) sshKeyPath() string { return filepath.Join(s.cfg.DataDir, "ssh", "id_ed25519") }

// draftFor turns the wizard's fields into an rclone remote. The provider's
// presets are applied last, so a form cannot talk a Nextcloud remote out of
// the dialect that makes it work.
func (s *Service) draftFor(p remotes.Provider, settings map[string]string) (remotes.Draft, error) {
	if p.Route != remotes.RouteRclone && p.Route != remotes.RouteS3 {
		return remotes.Draft{}, errNotThroughRclone
	}
	merged := make(map[string]string, len(settings)+len(p.Preset)+1)
	for k, v := range settings {
		merged[k] = strings.TrimSpace(v)
	}
	maps.Copy(merged, p.Preset)
	if p.Auth == remotes.AuthSSHKey {
		merged["key_file"] = s.sshKeyPath()
		delete(merged, "pass")
	}
	return remotes.Draft{Backend: p.Backend, Settings: merged}, nil
}

// DraftCheck is what a connection test of a draft found.
type DraftCheck struct {
	Reachable   bool `json:"reachable"`
	Initialized bool `json:"initialized"`
}

// CheckDraft reaches a destination that is not saved yet: through rclone for
// a remote, through restic for a rest-server, on the disk for a path.
func (s *Service) CheckDraft(ctx context.Context, req draftRequest) (DraftCheck, error) {
	p, err := findProvider(req.Provider)
	if err != nil {
		return DraftCheck{}, err
	}
	switch p.Route {
	case remotes.RouteRest:
		repo := "rest:" + strings.TrimSpace(req.Settings["url"])
		mode, err := s.draftRestMode(req.Settings)
		if err != nil {
			return DraftCheck{}, err
		}
		reachable, initialized, err := s.probeOffsiteRepo(ctx, repo, mode)
		return DraftCheck{Reachable: reachable, Initialized: initialized}, err
	case remotes.RoutePath:
		dir, err := s.resolveRepo(strings.TrimSpace(req.Settings["path"]))
		if err != nil {
			return DraftCheck{}, err
		}
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			return DraftCheck{}, errors.New("there is no folder at this path under the host data mount")
		}
		return DraftCheck{Reachable: true}, nil
	}
	d, err := s.draftFor(p, req.Settings)
	if err != nil {
		return DraftCheck{}, err
	}
	if _, err := s.remotes().Folders(ctx, "", &d, req.Dir); err != nil {
		return DraftCheck{}, err
	}
	return DraftCheck{Reachable: true}, nil
}

func (s *Service) draftRestMode(settings map[string]string) (restic.Mode, error) {
	stored, err := s.store.GetSettings()
	if err != nil {
		return restic.Mode{}, err
	}
	mode := s.ModeFor(stored)
	mode.Env = cloudEnv(CloudCreds{RESTUser: settings["user"], RESTPassword: settings["pass"]})
	return mode, nil
}

// DraftFolders lists the folders in req.Dir of a draft. At the top it also
// asks for the free space; free is nil when the backend does not report it.
func (s *Service) DraftFolders(ctx context.Context, req draftRequest) (folders []string, free *int64, err error) {
	p, err := findProvider(req.Provider)
	if err != nil {
		return nil, nil, err
	}
	d, err := s.draftFor(p, req.Settings)
	if err != nil {
		return nil, nil, err
	}
	folders, err = s.remotes().Folders(ctx, "", &d, req.Dir)
	if err != nil || strings.Trim(req.Dir, "/") != "" {
		return folders, nil, err
	}
	if n, ok := s.remotes().Free(ctx, "", &d); ok {
		free = &n
	}
	return folders, free, nil
}

// DraftMakeFolder creates req.Folder in req.Dir of a draft.
func (s *Service) DraftMakeFolder(ctx context.Context, req draftRequest) (string, error) {
	p, err := findProvider(req.Provider)
	if err != nil {
		return "", err
	}
	d, err := s.draftFor(p, req.Settings)
	if err != nil {
		return "", err
	}
	return s.remotes().MakeFolder(ctx, "", &d, req.Dir, req.Folder)
}

// editRcloneConf changes the stored rclone config under a lock, so two saves
// cannot each drop the other's remote.
func (s *Service) editRcloneConf(edit func(conf string) (string, error)) error {
	s.rcloneConfMu.Lock()
	defer s.rcloneConfMu.Unlock()
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	conf, err := s.decodeRcloneConf(settings)
	if err != nil {
		return err
	}
	next, err := edit(conf)
	if err != nil {
		return err
	}
	if next == conf {
		return nil
	}
	return s.SetRcloneConf(next)
}

// freeRemoteName picks a remote name from the provider's id that the config
// does not use yet.
func freeRemoteName(conf, base string) string {
	taken := map[string]bool{}
	for _, b := range splitRcloneSections(conf) {
		taken[strings.ToLower(b.name)] = true
	}
	name := base
	for i := 2; taken[strings.ToLower(name)]; i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

// s3Endpoint is the host restic's S3 backend talks to, with its scheme.
func s3Endpoint(p remotes.Provider, settings map[string]string) (string, error) {
	ep := strings.TrimRight(strings.TrimSpace(settings["endpoint"]), "/")
	if ep == "" && p.ID == "aws" {
		ep = "s3.amazonaws.com"
		if region := strings.TrimSpace(settings["region"]); region != "" {
			ep = "s3." + region + ".amazonaws.com"
		}
	}
	if ep == "" {
		return "", errors.New("the endpoint is missing")
	}
	if !strings.Contains(ep, "://") {
		ep = "https://" + ep
	}
	return ep, nil
}

// SaveNewDestination stores a destination the wizard checked. A remote's
// settings go into the rclone config and an S3 or rest-server login into a
// credential set of its own; either is taken back when the destination
// cannot be stored.
func (s *Service) SaveNewDestination(ctx context.Context, req draftRequest) (store.OffsiteTarget, error) {
	p, err := findProvider(req.Provider)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	dir := strings.Trim(strings.TrimSpace(req.Dir), "/")
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = p.Name
	}
	class := strings.ToUpper(strings.TrimSpace(req.StorageClass))
	if class != "" && !restic.StorageClassAllowed(class) {
		return store.OffsiteTarget{}, fmt.Errorf("unsupported storage class %s (allowed: %s)", class, strings.Join(restic.AllowedStorageClasses, ", "))
	}
	d := store.OffsiteTarget{Name: name, Provider: p.ID, Immutable: req.Immutable, StorageClass: class}
	var undo func()

	switch p.Route {
	case remotes.RouteRclone:
		draft, err := s.draftFor(p, req.Settings)
		if err != nil {
			return store.OffsiteTarget{}, err
		}
		settings, err := s.remotes().Settings(ctx, draft)
		if err != nil {
			return store.OffsiteTarget{}, err
		}
		var remote string
		err = s.editRcloneConf(func(conf string) (string, error) {
			remote = freeRemoteName(conf, p.ID)
			section, err := settingsSection(remote, draft.Backend, settings)
			if err != nil {
				return "", err
			}
			return replaceRcloneSection(conf, remote, section), nil
		})
		if err != nil {
			return store.OffsiteTarget{}, err
		}
		d.Repo = "rclone:" + remotes.Join(remote, dir)
		undo = func() { s.removeRcloneRemote(remote) }
	case remotes.RouteS3:
		if dir == "" {
			return store.OffsiteTarget{}, errors.New("choose a bucket first")
		}
		ep, err := s3Endpoint(p, req.Settings)
		if err != nil {
			return store.OffsiteTarget{}, err
		}
		d.Repo = "s3:" + ep + "/" + dir
		if d.CredsRef, err = s.addCredSet(name, CloudCreds{
			S3KeyID: strings.TrimSpace(req.Settings["access_key_id"]), S3Secret: req.Settings["secret_access_key"],
			S3Region: strings.TrimSpace(req.Settings["region"]), S3StorageClass: class,
		}); err != nil {
			return store.OffsiteTarget{}, err
		}
		ref := d.CredsRef
		undo = func() { s.removeCredSet(ref) }
	case remotes.RouteRest:
		url := strings.TrimRight(strings.TrimSpace(req.Settings["url"]), "/")
		if url == "" {
			return store.OffsiteTarget{}, errors.New("the address is missing")
		}
		d.Repo = "rest:" + url
		if d.CredsRef, err = s.addCredSet(name, CloudCreds{RESTUser: strings.TrimSpace(req.Settings["user"]), RESTPassword: req.Settings["pass"]}); err != nil {
			return store.OffsiteTarget{}, err
		}
		ref := d.CredsRef
		undo = func() { s.removeCredSet(ref) }
	case remotes.RoutePath:
		d.Repo = strings.TrimRight(strings.TrimSpace(req.Settings["path"]), "/")
		if _, err := s.resolveRepo(d.Repo); err != nil {
			return store.OffsiteTarget{}, err
		}
	}

	saved, err := s.store.SaveDestination(d)
	if err != nil {
		if undo != nil {
			undo()
		}
		return store.OffsiteTarget{}, err
	}
	return saved, nil
}

func (s *Service) addCredSet(name string, c CloudCreds) (string, error) {
	id, err := randomHex(16)
	if err != nil {
		return "", err
	}
	err = s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
		return append(sets, CloudCredSet{ID: id, Name: name, CloudCreds: c})
	})
	return id, err
}

// removeCredSet drops a credential set nothing refers to any more.
func (s *Service) removeCredSet(id string) {
	inUse, err := s.store.CredsRefsInUse()
	if err != nil || inUse[id] {
		return
	}
	_ = s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
		return slices.DeleteFunc(sets, func(c CloudCredSet) bool { return c.ID == id })
	})
}

// removeRcloneRemote drops a remote no stored location refers to any more.
func (s *Service) removeRcloneRemote(remote string) {
	prefix := "rclone:" + remote + ":"
	for _, list := range []func() ([]store.OffsiteTarget, error){s.store.ListOffsiteTargets, s.store.ListNamedRepos, s.store.ListDestinations} {
		rows, err := list()
		if err != nil {
			return
		}
		if slices.ContainsFunc(rows, func(t store.OffsiteTarget) bool { return strings.HasPrefix(t.Repo, prefix) }) {
			return
		}
	}
	_ = s.editRcloneConf(func(conf string) (string, error) {
		return replaceRcloneSection(conf, remote, ""), nil
	})
}

// DeleteDestination removes a destination no domain copies to, along with
// the remote or credential set it brought along.
func (s *Service) DeleteDestination(id string) error {
	d, ok, err := s.store.GetDestination(id)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := s.store.DeleteDestinationIfUnused(id); err != nil {
		return err
	}
	if remote, ok := strings.CutPrefix(d.Repo, "rclone:"); ok {
		if name, _, ok := strings.Cut(remote, ":"); ok {
			s.removeRcloneRemote(name)
		}
	}
	if d.CredsRef != "" {
		s.removeCredSet(d.CredsRef)
	}
	return nil
}

// destinationLocation is where a domain's repository sits under a
// destination.
func destinationLocation(base, domain string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, ":") {
		return base + domain
	}
	return base + "/" + domain
}

// handleOffsiteProviders lists the providers the wizard offers and how
// rclone describes the settings of each one's backend.
// GET /api/offsite/providers
func (h *Handler) handleOffsiteProviders(w http.ResponseWriter, r *http.Request) {
	backends, err := h.svc.remotes().Backends(r.Context())
	answer := map[string]any{"providers": remotes.Providers(), "backends": backends}
	if err != nil {
		answer["backends"] = map[string]remotes.Backend{}
		answer["backendsError"] = err.Error()
	}
	writeJSON(w, http.StatusOK, okEnvelope(answer))
}

// handleCheckDraft tests a destination before it is saved.
// POST /api/offsite/drafts/check
func (h *Handler) handleCheckDraft(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "testing a storage destination") {
		return
	}
	var req draftRequest
	if !decodeBody(w, r, &req) {
		return
	}
	got, err := h.svc.CheckDraft(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"reachable": got.Reachable, "initialized": got.Initialized}))
}

// handleDraftFolders lists a folder of a destination before it is saved.
// POST /api/offsite/drafts/folders
func (h *Handler) handleDraftFolders(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "browsing a storage destination") {
		return
	}
	var req draftRequest
	if !decodeBody(w, r, &req) {
		return
	}
	folders, free, err := h.svc.DraftFolders(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	body := map[string]any{"folders": folders}
	if free != nil {
		body["free"] = *free
	}
	writeJSON(w, http.StatusOK, okEnvelope(body))
}

// handleDraftMakeFolder creates a folder on a destination before it is saved.
// POST /api/offsite/drafts/mkdir
func (h *Handler) handleDraftMakeFolder(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "creating a folder on a storage destination") {
		return
	}
	var req draftRequest
	if !decodeBody(w, r, &req) {
		return
	}
	created, err := h.svc.DraftMakeFolder(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"dir": created}))
}

// handleListDestinations lists every destination.
// GET /api/offsite/destinations
func (h *Handler) handleListDestinations(w http.ResponseWriter, _ *http.Request) {
	list, err := h.store.ListDestinations()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	views := make([]destinationView, 0, len(list))
	for _, d := range list {
		v, err := h.svc.destinationView(d)
		if err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"destinations": views}))
}

// handleCreateDestination saves a destination the wizard set up.
// POST /api/offsite/destinations
func (h *Handler) handleCreateDestination(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "adding a storage destination") {
		return
	}
	var req draftRequest
	if !decodeBody(w, r, &req) {
		return
	}
	d, err := h.svc.SaveNewDestination(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.answerDestination(w, d)
}

// destinationEdit is what can change on a destination after it is saved.
type destinationEdit struct {
	Name         string `json:"name"`
	StorageClass string `json:"storageClass"`
	Immutable    bool   `json:"immutable"`
}

// handleUpdateDestination renames a destination or changes its storage class
// or append-only flag, for every domain that copies to it.
// PUT /api/offsite/destinations/{id}
func (h *Handler) handleUpdateDestination(w http.ResponseWriter, r *http.Request) {
	d, ok, err := h.store.GetDestination(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": store.ErrNotDestination.Error()})
		return
	}
	var e destinationEdit
	if !decodeBody(w, r, &e) {
		return
	}
	class := strings.ToUpper(strings.TrimSpace(e.StorageClass))
	if class != "" && !restic.StorageClassAllowed(class) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "unsupported storage class " + class})
		return
	}
	if name := strings.TrimSpace(e.Name); name != "" {
		d.Name = name
	}
	d.StorageClass, d.Immutable = class, e.Immutable
	saved, err := h.store.SaveDestination(d)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	h.answerDestination(w, saved)
}

// handleDeleteDestination removes a destination no domain copies to.
// DELETE /api/offsite/destinations/{id}
func (h *Handler) handleDeleteDestination(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteDestination(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleDestinationForDomain returns the domain's target under a destination,
// creating it the first time the destination is ticked in that domain. A new
// target starts unticked for every item, and the caller ticks it where it
// was asked for.
// POST /api/offsite/destinations/{id}/domains/{domain}
func (h *Handler) handleDestinationForDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if !validOffsiteDomain(domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": invalidOffsiteDomain})
		return
	}
	d, ok, err := h.store.GetDestination(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": store.ErrNotDestination.Error()})
		return
	}
	t := store.OffsiteTarget{Domain: domain, Repo: destinationLocation(d.Repo, domain)}
	if existing, err := h.store.DestinationTargets(d.ID); err == nil {
		if i := slices.IndexFunc(existing, func(x store.OffsiteTarget) bool { return x.Domain == domain }); i >= 0 {
			writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(existing[i]), "created": false}))
			return
		}
	}
	if msg := h.rejectOffsiteTargetOnNamedRepo(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.nestedTargetLocation("", t); err != nil {
		placementFail(w, err, nil)
		return
	}
	stored, created, err := h.store.EnsureDestinationTarget(d.ID, domain, t.Repo)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"target": offsiteTargetToView(stored), "created": created}))
}

func (h *Handler) answerDestination(w http.ResponseWriter, d store.OffsiteTarget) {
	v, err := h.svc.destinationView(d)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"destination": v}))
}

// providerMark is the mark of the provider a destination was set up with.
func providerMark(id string) string {
	p, _ := remotes.FindProvider(id)
	return p.Mark
}
