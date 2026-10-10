package api

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	// Retention, Compression, the limits and Enabled are what the domain
	// targets take that keep none of their own.
	Retention     store.RetentionKeep `json:"retention"`
	Compression   string              `json:"compression"`
	LimitUpload   int                 `json:"limitUpload"`
	LimitDownload int                 `json:"limitDownload"`
	Enabled       bool                `json:"enabled"`
	// OffPremises counts the destination as a site of its own.
	OffPremises bool `json:"offPremises"`
	// Adoptable are the targets typed in by hand whose repositories lie
	// under the destination.
	Adoptable []adoptableTarget `json:"adoptable"`
}

// adoptableTarget is a target typed in by hand that a destination can take over.
type adoptableTarget struct {
	ID      string `json:"id"`
	Domain  string `json:"domain"`
	Name    string `json:"name"`
	Repo    string `json:"repo"`
	Primary bool   `json:"primary"`
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
	all, err := s.store.ListOffsiteTargets()
	if err != nil {
		return destinationView{}, err
	}
	adoptable := []adoptableTarget{}
	for _, t := range all {
		if t.Enabled && t.DestinationID == "" && !slices.Contains(domains, t.Domain) && repoUnder(d.Repo, t.Repo) {
			adoptable = append(adoptable, adoptableTarget{ID: t.ID, Domain: t.Domain, Name: t.Name,
				Repo: scrubRepoLocation(t.Repo), Primary: t.SortOrder == 0})
		}
	}
	return destinationView{
		ID: d.ID, Name: d.Name, Provider: d.Provider, Mark: providerMark(d.Provider), Repo: scrubRepoLocation(d.Repo),
		CredsRef: d.CredsRef, StorageClass: d.StorageClass, Immutable: d.Immutable,
		CreatedAt: d.CreatedAt, Domains: domains, Adoptable: adoptable,
		Retention: targetKeep(d), Compression: normalizedCompression(d.Compression),
		LimitUpload: d.LimitUpload, LimitDownload: d.LimitDownload,
		Enabled: d.Enabled, OffPremises: d.OffPremises,
	}, nil
}

// repoUnder reports whether repo is a folder below the destination at base.
func repoUnder(base, repo string) bool {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	repo = strings.TrimRight(strings.TrimSpace(repo), "/")
	if strings.HasSuffix(base, ":") {
		return len(repo) > len(base) && strings.HasPrefix(repo, base)
	}
	return strings.HasPrefix(repo, base+"/")
}

// AdoptIntoDestination hangs a target typed in by hand on the destination its
// repository lies under, after checking that the destination's sign-in opens
// that repository. The target keeps its repository and snapshots.
func (s *Service) AdoptIntoDestination(ctx context.Context, destID, targetID string) (store.OffsiteTarget, error) {
	d, ok, err := s.store.GetDestination(destID)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if !ok {
		return store.OffsiteTarget{}, store.ErrNotDestination
	}
	t, ok, err := s.store.GetOffsiteTarget(targetID)
	if err != nil {
		return store.OffsiteTarget{}, err
	}
	if !ok {
		return store.OffsiteTarget{}, store.ErrNotOffsiteTarget
	}
	if !repoUnder(d.Repo, t.Repo) {
		return store.OffsiteTarget{}, fmt.Errorf("%s does not lie under %s", scrubRepoLocation(t.Repo), d.Name)
	}
	// Taking the destination's flag would let retention prune a repository
	// the user protected.
	if t.Immutable && !d.Immutable {
		return store.OffsiteTarget{}, fmt.Errorf("this target is append-only and %s is not: switch on append-only for %s first", d.Name, d.Name)
	}
	if err := s.destinationOpens(ctx, d, t); err != nil {
		return store.OffsiteTarget{}, err
	}
	domain := t.Domain
	return s.store.AdoptIntoDestination(t.ID, d.ID, func(st *store.Settings) { setOffsiteImmutableInSettings(st, domain, d.Immutable) })
}

// destinationOpens checks that the sign-in of d opens the repository t
// already holds.
func (s *Service) destinationOpens(ctx context.Context, d, t store.OffsiteTarget) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	probe := t
	probe.CredsRef, probe.StorageClass = d.CredsRef, d.StorageClass
	reachable, initialized, err := s.probeOffsiteRepo(ctx, t.Repo, s.offsiteModeForTarget(settings, probe))
	if err != nil || !reachable || !initialized {
		why := fmt.Errorf("the sign-in of %s does not open %s", d.Name, scrubRepoLocation(t.Repo))
		if err != nil {
			why = fmt.Errorf("%w: %s", why, scrubError(err))
		}
		return why
	}
	return nil
}

// PrimaryFromDestination makes the folder a destination keeps for the domain
// the domain's primary off-site target. When the primary is already there, the
// destination's sign-in has to open the repository it holds, as for a
// takeover.
func (s *Service) PrimaryFromDestination(ctx context.Context, d store.OffsiteTarget, domain string) (store.OffsiteTarget, store.Settings, error) {
	loc := destinationLocation(d.Repo, domain)
	primary, ok, err := s.store.FieldOffsiteTarget(domain)
	if err != nil {
		return store.OffsiteTarget{}, store.Settings{}, err
	}
	if ok && primary.Enabled && primary.Repo == loc && primary.DestinationID != d.ID {
		if primary.Immutable && !d.Immutable {
			return store.OffsiteTarget{}, store.Settings{}, fmt.Errorf("this off-site copy is append-only and %s is not: switch on append-only for %s first", d.Name, d.Name)
		}
		if err := s.destinationOpens(ctx, d, primary); err != nil {
			return store.OffsiteTarget{}, store.Settings{}, err
		}
	}
	if _, err := s.store.PrimaryFromDestination(d.ID, domain, loc, func(st *store.Settings) {
		setOffsiteRepoInSettings(st, domain, loc)
		setOffsiteImmutableInSettings(st, domain, d.Immutable)
	}); err != nil {
		return store.OffsiteTarget{}, store.Settings{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.OffsiteTarget{}, store.Settings{}, err
	}
	// The sync fills in the retention and limits the settings give the primary.
	if err := s.syncPrimaryOffsiteTarget(domain, settings); err != nil {
		return store.OffsiteTarget{}, store.Settings{}, err
	}
	t, _, err := s.store.FieldOffsiteTarget(domain)
	return t, settings, err
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
	if p.Route == remotes.RouteS3 && merged["endpoint"] != "" {
		merged["endpoint"] = s3Host(merged["endpoint"])
	}
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

// s3Host turns a restic repository address pasted into the endpoint field,
// such as s3:http://nas:9000/bucket/folder, into the server alone. The AWS
// SDK reads s3: as a URL scheme and then fails on the region instead.
func s3Host(ep string) string {
	ep = strings.TrimSpace(ep)
	rest, ok := strings.CutPrefix(ep, "s3:")
	if !ok {
		return ep
	}
	scheme := ""
	if before, after, found := strings.Cut(rest, "://"); found {
		scheme, rest = before+"://", after
	}
	host, _, _ := strings.Cut(rest, "/")
	return scheme + host
}

// s3Endpoint is the host restic's S3 backend talks to, with its scheme.
func s3Endpoint(p remotes.Provider, settings map[string]string) (string, error) {
	ep := strings.TrimRight(s3Host(settings["endpoint"]), "/")
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
// destination. The self-backup's folder is not called config: at the root of
// a rest-server that path is the root repository's own config file, so a
// repository there could never be opened.
func destinationLocation(base, domain string) string {
	folder := domain
	if domain == "config" {
		folder = "selfbackup"
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, ":") {
		return base + folder
	}
	return base + "/" + folder
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

// destinationEdit is what can change on a destination after it is saved. A
// setting behind a pointer stays as it is when the body leaves it out.
type destinationEdit struct {
	Name          string               `json:"name"`
	StorageClass  string               `json:"storageClass"`
	Immutable     bool                 `json:"immutable"`
	Retention     *store.RetentionKeep `json:"retention"`
	Compression   *string              `json:"compression"`
	LimitUpload   *int                 `json:"limitUpload"`
	LimitDownload *int                 `json:"limitDownload"`
	Enabled       *bool                `json:"enabled"`
	OffPremises   *bool                `json:"offPremises"`
}

// apply writes the edit's optional settings into d.
func (e destinationEdit) apply(d *store.OffsiteTarget) error {
	if e.Retention != nil {
		k := clampKeep(*e.Retention)
		d.RetentionKeepLast, d.RetentionKeepDaily, d.RetentionKeepWeekly = k.KeepLast, k.KeepDaily, k.KeepWeekly
		d.RetentionKeepMonthly, d.RetentionKeepYearly = k.KeepMonthly, k.KeepYearly
	}
	if e.Compression != nil {
		mode := strings.ToLower(strings.TrimSpace(*e.Compression))
		if _, err := restic.ParseCompression(mode); err != nil {
			return err
		}
		d.Compression = mode
	}
	if e.LimitUpload != nil {
		d.LimitUpload = max(0, *e.LimitUpload)
	}
	if e.LimitDownload != nil {
		d.LimitDownload = max(0, *e.LimitDownload)
	}
	if e.Enabled != nil {
		d.Enabled = *e.Enabled
	}
	if e.OffPremises != nil {
		d.OffPremises = *e.OffPremises
	}
	return nil
}

// handleUpdateDestination renames a destination or changes its storage class
// or append-only flag, for every domain that copies to it. Its keep-policy,
// compression, limits and switch reach the domain targets that keep none of
// their own. The answer lists what the change means for direct repositories.
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
	if err := e.apply(&d); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	before, err := h.store.DestinationTargets(d.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	saved, err := h.store.SaveDestination(d)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	warnings := h.svc.destinationSaveWarnings(before)
	// A primary that follows the destination carries its append-only flag in
	// the settings as well, and the scheduler reads it there.
	s, err := h.svc.settleLinkedPrimaries()
	if err == nil {
		err = h.scheduler.ReloadWithGates(s, h.dueGates())
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	v, err := h.svc.destinationView(saved)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"destination": v, "warnings": warnings}))
}

// destinationSaveWarnings is what a saved destination means for the direct
// repositories of its domain targets. before holds those targets as they were.
// The save stands either way, so a failed read is logged, not returned.
func (s *Service) destinationSaveWarnings(before []store.OffsiteTarget) []saveWarning {
	out := []saveWarning{}
	for _, was := range before {
		is, ok, err := s.store.GetOffsiteTarget(was.ID)
		if err == nil && ok {
			var ws []saveWarning
			ws, err = s.directSaveWarnings(was, is)
			out = append(out, ws...)
		}
		if err != nil {
			log.Printf("api: target %s: could not check its direct repository after its destination was saved: %v", was.ID, err) //nolint:gosec // G706: the id is store-generated
		}
	}
	return out
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

// handleAdoptIntoDestination hangs a target typed in by hand on the
// destination its repository lies under.
// POST /api/offsite/destinations/{id}/adopt/{target}
func (h *Handler) handleAdoptIntoDestination(w http.ResponseWriter, r *http.Request) {
	if _, err := h.svc.AdoptIntoDestination(r.Context(), r.PathValue("id"), r.PathValue("target")); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// Taking over a primary sets the domain's append-only flag, which the
	// scheduler reads.
	if s, err := h.store.GetSettings(); err == nil {
		if err := h.scheduler.ReloadWithGates(s, h.dueGates()); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
			return
		}
	}
	d, ok, err := h.store.GetDestination(r.PathValue("id"))
	if err != nil || !ok {
		writeJSON(w, http.StatusOK, failEnvelope(errors.Join(err, store.ErrNotDestination)))
		return
	}
	h.answerDestination(w, d)
}

// handlePrimaryFromDestination puts a domain's primary off-site copy into the
// folder a destination keeps for the domain. The answer carries the off-site
// field and append-only flag as the settings form shows them.
// POST /api/offsite/destinations/{id}/primary/{domain}
func (h *Handler) handlePrimaryFromDestination(w http.ResponseWriter, r *http.Request) {
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
	if msg := h.rejectOffsiteTargetOnNamedRepo(t); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	primary, _, err := h.store.FieldOffsiteTarget(domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.nestedTargetLocation(primary.ID, t); err != nil {
		placementFail(w, err, nil)
		return
	}
	stored, s, err := h.svc.PrimaryFromDestination(r.Context(), d, domain)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.scheduler.ReloadWithGates(s, h.dueGates()); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"target":    offsiteTargetToView(stored),
		"location":  scrubRepoLocation(offsiteRepoFromSettings(domain, s)),
		"immutable": offsiteImmutableFor(domain, s),
	}))
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
