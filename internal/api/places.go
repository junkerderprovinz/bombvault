package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errPlaceLocationEstablished = errors.New("backups lie at the old address, and the new one does not hold the same repository")
	errPlaceHomeDomain          = errors.New("a domain keeps its backups at this place")
	errPlaceIsRepository        = errors.New("this place is itself a repository, so it takes no second role and no folder below it")
	errPlaceAddressTaken        = errors.New("a repository of this domain already lies at that address")
	errPlaceOff                 = errors.New("this place is switched off")
	errPlaceNameMissing         = errors.New("a place needs a name")
	errPlaceUnasked             = errors.New("say where the device stands: here or at another site")
	errUnknownProvider          = errors.New("that is not a provider this server can connect")
)

// writePlace writes a place and, when edit is set and changes them, the
// credential sets edit returns, in one WritePlace. The sets are read under
// credSetsMu, which every other writer of them takes too.
func (s *Service) writePlace(w store.PlaceWrite, edit func([]CloudCredSet) []CloudCredSet) (store.Place, error) {
	s.credSetsMu.Lock()
	defer s.credSetsMu.Unlock()
	if edit != nil {
		settings, err := s.store.GetSettings()
		if err != nil {
			return store.Place{}, err
		}
		sets, err := s.decodeCloudCredSets(settings)
		if err != nil {
			return store.Place{}, fmt.Errorf("read the credential sets: %w", err)
		}
		if next := edit(slices.Clone(sets)); !slices.Equal(next, sets) {
			enc, err := s.encodeCloudCredSets(next)
			if err != nil {
				return store.Place{}, err
			}
			w.CredSetsBlob = []byte(enc)
		}
	}
	return s.store.WritePlace(w)
}

// withPlaceCreds returns set with the credentials c written onto it, the
// inverse of placeCredsOf.
func withPlaceCreds(set CloudCredSet, c places.Creds) CloudCredSet {
	set.S3KeyID, set.S3Secret, set.S3Region, set.S3StorageClass = c.S3KeyID, c.S3Secret, c.S3Region, c.S3StorageClass
	set.RESTUser, set.RESTPassword = c.RESTUser, c.RESTPassword
	set.WebDAVURL, set.WebDAVVendor, set.WebDAVUser, set.WebDAVPass = c.WebDAVURL, c.WebDAVVendor, c.WebDAVUser, c.WebDAVPass
	set.AzureAccount, set.AzureKey = c.AzureAccount, c.AzureKey
	return set
}

// PlaceUsage is what a place is used for, as the database knows it.
type PlaceUsage struct {
	HomeDomains []string `json:"homeDomains"`
	Defaults    []string `json:"defaults"`
	CopyDomains []string `json:"copyDomains"`
	Items       int      `json:"items"`
	// Copies are the snapshots its targets held at their last listing, which
	// stay there when the place is removed.
	Copies int `json:"copies"`
}

// PlaceCredsView is a place's credentials without their secrets.
type PlaceCredsView struct {
	Shared bool              `json:"shared"` // the place runs on the shared credentials, not a set of its own
	Fields map[string]string `json:"fields"` // non-secret values by catalog field key
	Set    []string          `json:"set"`    // secret field keys that hold a value
}

// PlaceTestStatus is the last thing known about reaching a place.
type PlaceTestStatus struct {
	At     int64  `json:"at"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Source string `json:"source"` // "test" or "run"
}

// PlaceView is a place as the Storage tab shows it. It is built from the
// database alone: listing places never opens a repository.
type PlaceView struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Provider             string            `json:"provider"`
	Kind                 string            `json:"kind"`
	Base                 string            `json:"base"`
	Folders              map[string]string `json:"folders"`
	OffPremises          bool              `json:"offPremises"`
	StorageClass         string            `json:"storageClass"`
	Immutable            bool              `json:"immutable"`
	RetentionKeepLast    int               `json:"retentionKeepLast"`
	RetentionKeepDaily   int               `json:"retentionKeepDaily"`
	RetentionKeepWeekly  int               `json:"retentionKeepWeekly"`
	RetentionKeepMonthly int               `json:"retentionKeepMonthly"`
	LimitUpload          int               `json:"limitUpload"`
	LimitDownload        int               `json:"limitDownload"`
	GrowthBudgetGB       int               `json:"growthBudgetGb"`
	Enabled              bool              `json:"enabled"`
	SortOrder            int               `json:"sortOrder"`
	Usage                PlaceUsage        `json:"usage"`
	Locked               map[string]bool   `json:"locked"`
	Repository           bool              `json:"repository"`
	LastTest             *PlaceTestStatus  `json:"lastTest,omitempty"`
	Creds                PlaceCredsView    `json:"creds"`
}

// UnplacedRow is a repository or domain path whose address fits no place. It
// works as before and waits in the group without a place.
type UnplacedRow struct {
	RowID  string `json:"rowId"`  // "" for a domain's own path
	Domain string `json:"domain"` // "" for a named repository, which no domain owns
	Role   string `json:"role"`   // "path", "target", "repository" or "direct"
	Name   string `json:"name"`
	Repo   string `json:"repo"` // as stored
}

// credField ties a catalog field key to its value in a credential set.
type credField struct {
	key    string
	secret bool
	value  func(c CloudCredSet) string
}

// credFields are the credential fields of each kind. A local folder needs
// none, sftp goes by the host key and rclone by its own config.
var credFields = map[places.Kind][]credField{
	places.KindS3: {
		{"keyId", false, func(c CloudCredSet) string { return c.S3KeyID }},
		{"secret", true, func(c CloudCredSet) string { return c.S3Secret }},
		{"region", false, func(c CloudCredSet) string { return c.S3Region }},
	},
	places.KindREST: {
		{"user", false, func(c CloudCredSet) string { return c.RESTUser }},
		{"password", true, func(c CloudCredSet) string { return c.RESTPassword }},
	},
	places.KindWebDAV: {
		{"url", false, func(c CloudCredSet) string { return c.WebDAVURL }},
		{"user", false, func(c CloudCredSet) string { return c.WebDAVUser }},
		{"password", true, func(c CloudCredSet) string { return c.WebDAVPass }},
	},
	places.KindAzure: {
		{"account", false, func(c CloudCredSet) string { return c.AzureAccount }},
		{"secret", true, func(c CloudCredSet) string { return c.AzureKey }},
	},
}

// placeData is what one answer about places reads, once.
type placeData struct {
	settings store.Settings
	places   []store.Place
	homes    map[string]string                // domain -> home place id
	rows     map[string][]store.OffsiteTarget // place id -> the rows it holds
	defaults []store.PlacementDefault
	repos    []store.OffsiteTarget // named repositories in picker order
}

func (s *Service) readPlaceData() (placeData, error) {
	d := placeData{rows: map[string][]store.OffsiteTarget{}}
	var err error
	if d.settings, err = s.store.GetSettings(); err != nil {
		return d, err
	}
	if d.places, err = s.store.ListPlaces(); err != nil {
		return d, err
	}
	if d.homes, err = s.store.DomainPlaces(); err != nil {
		return d, err
	}
	for _, p := range d.places {
		if d.rows[p.ID], err = s.store.PlaceRows(p.ID); err != nil {
			return d, err
		}
	}
	if d.defaults, err = s.store.ListPlacementDefaults(); err != nil {
		return d, err
	}
	d.repos, err = s.store.ListNamedRepos()
	return d, err
}

// placeViews is every place and every row without one.
func (s *Service) placeViews() ([]PlaceView, []UnplacedRow, error) {
	d, err := s.readPlaceData()
	if err != nil {
		return nil, nil, err
	}
	views := make([]PlaceView, 0, len(d.places))
	for _, p := range d.places {
		v, err := s.placeView(d, p)
		if err != nil {
			return nil, nil, err
		}
		views = append(views, v)
	}
	unplaced, err := s.unplacedRows(d)
	return views, unplaced, err
}

func (s *Service) placeView(d placeData, p store.Place) (PlaceView, error) {
	rows := d.rows[p.ID]
	v := PlaceView{
		ID: p.ID, Name: p.Name, Provider: p.Provider, Kind: p.Kind, Base: p.Base, Folders: map[string]string{},
		OffPremises: p.OffPremises, StorageClass: p.StorageClass, Immutable: p.Immutable,
		RetentionKeepLast: p.RetentionKeepLast, RetentionKeepDaily: p.RetentionKeepDaily,
		RetentionKeepWeekly: p.RetentionKeepWeekly, RetentionKeepMonthly: p.RetentionKeepMonthly,
		LimitUpload: p.LimitUpload, LimitDownload: p.LimitDownload, GrowthBudgetGB: p.GrowthBudgetGB,
		Enabled: p.Enabled, SortOrder: p.SortOrder,
		Usage:      PlaceUsage{HomeDomains: []string{}, Defaults: []string{}, CopyDomains: []string{}},
		Locked:     map[string]bool{},
		Repository: placeIsRepository(p, rows),
		Creds:      s.placeCredsView(d.settings, p),
	}
	maps.Copy(v.Folders, p.Folders)
	for dom := range p.Folders {
		v.Locked[dom] = false
	}
	// A row without a domain is the place's one repository, so it locks
	// every domain there.
	lock := func(domain string) {
		if domain != "" {
			v.Locked[domain] = true
			return
		}
		for dom := range p.Folders {
			v.Locked[dom] = true
		}
	}
	for _, dom := range places.Domains {
		if d.homes[dom] != p.ID {
			continue
		}
		v.Usage.HomeDomains = append(v.Usage.HomeDomains, dom)
		f, err := s.domainPathFacts(d.settings, dom)
		if err != nil {
			return v, err
		}
		if f.Established {
			lock(dom)
		}
	}
	byID := namedReposByID(d.repos)
	for _, def := range d.defaults {
		if repo, ok := byID[def.Home]; ok && repo.PlaceID == p.ID {
			v.Usage.Defaults = append(v.Usage.Defaults, def.Domain)
		}
	}
	copying := map[string]bool{}
	for _, r := range rows {
		if r.Role == store.RolePrimary {
			continue
		}
		f, err := s.rowFacts(r)
		if err != nil {
			return v, err
		}
		if f.Established {
			lock(r.PlaceDomain)
		}
		v.Usage.Items += f.Items
		if r.Role == store.RoleOffsite {
			copying[r.Domain] = copying[r.Domain] || r.Enabled
			v.Usage.Copies += f.Snapshots
		}
	}
	for _, dom := range places.Domains {
		if copying[dom] {
			v.Usage.CopyDomains = append(v.Usage.CopyDomains, dom)
		}
	}
	last, err := s.placeLastRun(rows)
	v.LastTest = last
	return v, err
}

// placeIsRepository reports whether the place is itself a repository: a domain
// there uses the base address, so nothing may sit below it.
func placeIsRepository(p store.Place, rows []store.OffsiteTarget) bool {
	for _, f := range p.Folders {
		if f == "" {
			return true
		}
	}
	return slices.ContainsFunc(rows, func(r store.OffsiteTarget) bool {
		return r.PlaceDomain == "" && r.Role != store.RolePrimary
	})
}

// placeLastRun is what the copy runs to the place's targets last said: a
// failure after a target's last success wins over every success.
func (s *Service) placeLastRun(rows []store.OffsiteTarget) (*PlaceTestStatus, error) {
	var last *PlaceTestStatus
	for _, r := range rows {
		if r.Role != store.RoleOffsite {
			continue
		}
		run, found, err := s.store.LatestSuccessfulOffsiteRunForTarget(r.Domain, r.ID)
		if err != nil {
			return nil, err
		}
		failedAt, err := s.store.FirstOffsiteFailureAfter(r.ID, run.StartedAt)
		if err != nil {
			return nil, err
		}
		switch {
		case failedAt > 0:
			return &PlaceTestStatus{At: failedAt, Source: "run"}, nil
		case found && (last == nil || run.StartedAt > last.At):
			last = &PlaceTestStatus{At: run.StartedAt, OK: true, Source: "run"}
		}
	}
	return last, nil
}

// placeCredsView shows the credentials the place's rows run with: its own
// set, or the shared credentials while it names none.
func (s *Service) placeCredsView(settings store.Settings, p store.Place) PlaceCredsView {
	v := PlaceCredsView{Fields: map[string]string{}, Set: []string{}}
	fields := credFields[places.Kind(p.Kind)]
	if len(fields) == 0 {
		return v
	}
	v.Shared = p.CredsRef == ""
	set, err := s.credSetFor(settings, p.CredsRef)
	if err != nil {
		log.Printf("api: place %s: could not read its credentials: %v", p.ID, err) //nolint:gosec // G706: the id is store-generated
		return v
	}
	for _, f := range fields {
		switch value := f.value(set); {
		case value == "":
		case f.secret:
			v.Set = append(v.Set, f.key)
		default:
			v.Fields[f.key] = value
		}
	}
	return v
}

// unplacedRows are the domain paths without a home place and the targets and
// named repositories at no place.
func (s *Service) unplacedRows(d placeData) ([]UnplacedRow, error) {
	out := []UnplacedRow{}
	for _, dom := range places.Domains {
		if d.homes[dom] == "" {
			out = append(out, UnplacedRow{Domain: dom, Role: "path", Repo: domainPathRaw(dom, d.settings)})
		}
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		if t.PlaceID == "" {
			out = append(out, UnplacedRow{RowID: t.ID, Domain: t.Domain, Role: "target", Name: t.Name, Repo: t.Repo})
		}
	}
	for _, r := range d.repos {
		if r.PlaceID != "" {
			continue
		}
		role := "repository"
		if r.CompanionOf != "" {
			role = "direct"
		}
		out = append(out, UnplacedRow{RowID: r.ID, Role: role, Name: r.Name, Repo: r.Repo})
	}
	return out, nil
}

// handlePlacesCatalog serves GET /api/places/catalog, the catalog in tile
// order as the add window reads it.
func (h *Handler) handlePlacesCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"providers": places.Catalog}))
}

// handleListPlaces serves GET /api/places.
func (h *Handler) handleListPlaces(w http.ResponseWriter, _ *http.Request) {
	views, unplaced, err := h.svc.placeViews()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"places": views, "unplaced": unplaced}))
}

// placeProbeFn is the probe behind the add window and a credentials edit.
func (s *Service) placeProbeFn() func(context.Context, ProbeRequest) (places.ProbeResult, error) {
	if s.placeProber != nil {
		return s.placeProber
	}
	return s.ProbePlace
}

// probeFailedErr is a place-probe-failed refusal carrying what the probe said.
type probeFailedErr struct{ result places.ProbeResult }

func (e *probeFailedErr) Error() string {
	if e.result.Error == "" {
		return errPlaceProbeFailed.Error()
	}
	return errPlaceProbeFailed.Error() + ": " + e.result.Error
}

func (e *probeFailedErr) Is(target error) bool { return target == errPlaceProbeFailed }

// placeFail answers a refused place write with what its error carries.
func placeFail(w http.ResponseWriter, err error) {
	extra := map[string]any{}
	var probe *probeFailedErr
	if errors.As(err, &probe) {
		extra["probe"] = probe.result
	}
	placementFail(w, err, extra)
}

// createPlaceBody is the add window's submit: what it tested, and the answers
// it asked for after the test.
type createPlaceBody struct {
	ProbeRequest
	Name        string            `json:"name"`
	OffPremises *bool             `json:"offPremises"`
	Folders     map[string]string `json:"folders"`
}

// createPlace probes the form once more, since no secret is kept between the
// test and the submit, and writes the place with a credential set of its own
// when its kind has one.
func (s *Service) createPlace(ctx context.Context, body createPlaceBody) (store.Place, error) {
	provider, ok := places.ProviderByID(body.Provider)
	if !ok || body.PlaceID != "" {
		return store.Place{}, errUnknownProvider
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return store.Place{}, errPlaceNameMissing
	}
	if body.Folders != nil {
		if err := checkFolders(body.Folders); err != nil {
			return store.Place{}, err
		}
	}
	off, err := offPremisesFor(provider, body.OffPremises)
	if err != nil {
		return store.Place{}, err
	}
	probe, err := s.placeProbeFn()(ctx, body.ProbeRequest)
	if err != nil {
		return store.Place{}, err
	}
	if !probe.OK {
		return store.Place{}, &probeFailedErr{result: probe}
	}
	folders, err := newPlaceFolders(body.Folders, probe)
	if err != nil {
		return store.Place{}, err
	}
	// The probe hands the form back completed and without its secrets, which
	// come from what was typed.
	fields := map[string]string{}
	maps.Copy(fields, body.Fields)
	maps.Copy(fields, probe.Fields)
	// The probe's base names a WebDAV remote after a throwaway id, so the base
	// is built again for the id the place gets.
	id := newPlaceID()
	base, err := places.Base(provider, fields, id)
	if err != nil {
		return store.Place{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Place{}, err
	}
	p := newPlace(settings, provider, name, base, folders, off)
	p.ID = id
	if len(credFields[provider.Kind]) == 0 {
		return s.writePlace(store.PlaceWrite{Place: p}, nil)
	}
	set := withPlaceCreds(CloudCredSet{ID: newCredSetID(), Name: name, Kind: string(provider.Kind)}, places.CredsFromFields(provider, fields))
	p.CredsRef = set.ID
	return s.writePlace(store.PlaceWrite{Place: p}, func(sets []CloudCredSet) []CloudCredSet {
		return append(sets, set)
	})
}

// newPlaceID mints a place's id before its first write, in the form the store
// gives every row, since a WebDAV place's base names its rclone remote after it.
func newPlaceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("newPlaceID: %v", err))
	}
	return hex.EncodeToString(b)
}

// newPlaceFolders are a new place's folders: the ones the window sent, or the
// usual ones. An address that already holds a repository becomes a place that
// is itself that repository, every domain at its base, since a folder below it
// would be a repository inside one.
func newPlaceFolders(sent map[string]string, probe places.ProbeResult) (map[string]string, error) {
	_, isRepository := probe.RepoIDs[""]
	switch {
	case !isRepository && sent == nil:
		return places.DefaultFolders(), nil
	case !isRepository:
		return sent, nil
	case sent == nil:
		all := map[string]string{}
		for _, d := range places.Domains {
			all[d] = ""
		}
		return all, nil
	}
	for _, f := range sent {
		if f != "" {
			return nil, errPlaceIsRepository
		}
	}
	return sent, nil
}

// checkFolders refuses a folder map the address rules cannot hold: a domain
// BombVault does not know, a folder that is not one plain name, or a folder
// beside a domain kept at the base itself, which would put one repository
// inside another.
func checkFolders(folders map[string]string) error {
	for d, f := range folders {
		if !slices.Contains(places.Domains, d) {
			return fmt.Errorf("%q is not a domain", d)
		}
		if f != strings.TrimSpace(f) || f == "." || f == ".." || strings.ContainsAny(f, `/\:`) || len(f) > 255 {
			return fmt.Errorf("the folder %q is not one plain name", f)
		}
	}
	values := slices.Collect(maps.Values(folders))
	if slices.Contains(values, "") && slices.ContainsFunc(values, func(f string) bool { return f != "" }) {
		return errPlaceIsRepository
	}
	return nil
}

// offPremisesFor is where a new place stands: fixed by the provider for a
// cloud or this server, the answer to the window's question for a device.
func offPremisesFor(provider places.Provider, answer *bool) (bool, error) {
	switch {
	case provider.OffPremises != nil:
		return *provider.OffPremises, nil
	case answer == nil:
		return false, errPlaceUnasked
	}
	return *answer, nil
}

// newPlace is a place before its first write, with the global rules as its
// template: the local retention for a folder place, the off-site retention,
// limits and budget for a remote one.
func newPlace(settings store.Settings, provider places.Provider, name, base string, folders map[string]string, off bool) store.Place {
	p := store.Place{Name: name, Provider: provider.ID, Kind: string(provider.Kind), Base: base, Folders: folders, OffPremises: off, Enabled: true}
	if provider.Kind == places.KindLocal {
		p.RetentionKeepLast, p.RetentionKeepDaily = settings.RetentionKeepLast, settings.RetentionKeepDaily
		p.RetentionKeepWeekly, p.RetentionKeepMonthly = settings.RetentionKeepWeekly, settings.RetentionKeepMonthly
		return p
	}
	p.RetentionKeepLast, p.RetentionKeepDaily = settings.OffsiteRetentionKeepLast, settings.OffsiteRetentionKeepDaily
	p.RetentionKeepWeekly, p.RetentionKeepMonthly = settings.OffsiteRetentionKeepWeekly, settings.OffsiteRetentionKeepMonthly
	p.LimitUpload, p.LimitDownload, p.GrowthBudgetGB = settings.OffsiteLimitUpload, settings.OffsiteLimitDownload, settings.OffsiteGrowthBudgetGB
	return p
}

// placeViewByID is one place as the list shows it.
func (s *Service) placeViewByID(id string) (PlaceView, error) {
	d, err := s.readPlaceData()
	if err != nil {
		return PlaceView{}, err
	}
	i := slices.IndexFunc(d.places, func(p store.Place) bool { return p.ID == id })
	if i < 0 {
		return PlaceView{}, store.ErrPlaceNotFound
	}
	return s.placeView(d, d.places[i])
}

// writePlaceAnswer answers a place write with the place as the list shows it.
func (h *Handler) writePlaceAnswer(w http.ResponseWriter, id string, extra map[string]any) {
	view, err := h.svc.placeViewByID(id)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := map[string]any{"place": view}
	maps.Copy(out, extra)
	writeJSON(w, http.StatusOK, okEnvelope(out))
}

// handleProbePlace serves POST /api/places/probe. The probe's result is the
// answer, ok false when it reached no verdict; a request the probe cannot run
// is refused.
func (h *Handler) handleProbePlace(w http.ResponseWriter, r *http.Request) {
	var req ProbeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	res, err := h.svc.placeProbeFn()(r.Context(), req)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleCreatePlace serves POST /api/places.
func (h *Handler) handleCreatePlace(w http.ResponseWriter, r *http.Request) {
	var body createPlaceBody
	if !decodeBody(w, r, &body) {
		return
	}
	p, err := h.svc.createPlace(r.Context(), body)
	if err != nil {
		placeFail(w, err)
		return
	}
	h.writePlaceAnswer(w, p.ID, nil)
}
