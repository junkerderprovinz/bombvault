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
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var (
	errPlaceLocationEstablished = errors.New("backups lie at the old address, and the new one does not hold the same repository")
	errPlaceHomeDomain          = errors.New("a domain keeps its backups at this place")
	errPlaceIsRepository        = errors.New("this place is itself a repository, so it takes no second role and no folder below it")
	errPlaceFolderBlank         = errors.New("a folder needs a name, since only a place that is itself a repository keeps a domain at its base")
	errPlaceAddressTaken        = errors.New("a repository of this domain already lies at that address")
	errPlaceOff                 = errors.New("this place is switched off")
	errPlaceRepoShared          = errors.New("another domain backs up to this repository, so it cannot become the repository of one domain here")
	errPlaceKeepsLess           = errors.New("the place keeps fewer snapshots than this repository does now, and its next prune would forget the rest; raise the place's retention first")
	errPlaceAppendOnlyOff       = errors.New("this repository is append-only and the place is not, so joining it would let a prune delete its snapshots")
	errPlaceNameMissing         = errors.New("a place needs a name")
	errPlaceUnasked             = errors.New("say where the device stands: here or at another site")
	errUnknownProvider          = errors.New("that is not a provider this server can connect")
	errLocalAppendOnly          = errors.New("a folder on this server cannot be kept from deletion, so it takes no append-only switch")
	errPlaceNothingToTest       = errors.New("no domain backs up or copies to this place, so there is nothing to test")
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
	// Repositories are the domain paths and rows the place's switches reach,
	// which the question before append-only goes off counts.
	Repositories int `json:"repositories"`
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
	// CredsRef names the credential set the place keeps its credentials in,
	// "" for the shared ones. The values themselves stay in Creds, masked.
	CredsRef   string           `json:"credsRef"`
	Usage      PlaceUsage       `json:"usage"`
	Locked     map[string]bool  `json:"locked"`
	Repository bool             `json:"repository"`
	LastTest   *PlaceTestStatus `json:"lastTest,omitempty"`
	Creds      PlaceCredsView   `json:"creds"`
}

// UnplacedRow is a repository or domain path whose address fits no place. It
// keeps working at its stored address and waits in the group without a place.
type UnplacedRow struct {
	RowID  string `json:"rowId"`  // "" for a domain's own path
	Domain string `json:"domain"` // "" for a named repository, which no domain owns
	Role   string `json:"role"`   // "path", "target", "repository" or "direct"
	Name   string `json:"name"`
	Repo   string `json:"repo"` // as stored, without a password it carries
	// Immutable is the row's append-only flag. A domain path keeps it on its
	// domain's primary row, which guards the path only while it is remote.
	Immutable   bool `json:"immutable"`
	Protectable bool `json:"protectable"` // the row takes an append-only switch of its own
	Items       int  `json:"items"`       // items whose backups go there, for the question before append-only goes off
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
		ID: p.ID, Name: p.Name, Provider: p.Provider, Kind: p.Kind, Base: scrubRepoLocation(p.Base), Folders: map[string]string{},
		OffPremises: p.OffPremises, StorageClass: p.StorageClass, Immutable: p.Immutable,
		RetentionKeepLast: p.RetentionKeepLast, RetentionKeepDaily: p.RetentionKeepDaily,
		RetentionKeepWeekly: p.RetentionKeepWeekly, RetentionKeepMonthly: p.RetentionKeepMonthly,
		LimitUpload: p.LimitUpload, LimitDownload: p.LimitDownload, GrowthBudgetGB: p.GrowthBudgetGB,
		Enabled: p.Enabled, SortOrder: p.SortOrder, CredsRef: p.CredsRef,
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
		v.Usage.Repositories++
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
		v.Usage.Repositories++
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
	last, err := s.placeLastTest(p.ID, rows)
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

// placeLastRun is what the copy runs to the place's targets last said: the
// newest failure after a target's last success, with its reason, wins over
// every success.
func (s *Service) placeLastRun(rows []store.OffsiteTarget) (*PlaceTestStatus, error) {
	var ok, failed *PlaceTestStatus
	for _, r := range rows {
		if r.Role != store.RoleOffsite {
			continue
		}
		run, found, err := s.store.LatestSuccessfulOffsiteRunForTarget(r.Domain, r.ID)
		if err != nil {
			return nil, err
		}
		failure, hasFailure, err := s.store.LatestOffsiteFailureAfter(r.ID, run.StartedAt)
		if err != nil {
			return nil, err
		}
		switch {
		case hasFailure && (failed == nil || failure.StartedAt > failed.At):
			failed = &PlaceTestStatus{At: failure.StartedAt, Error: failure.Error, Source: "run"}
		case found && (ok == nil || run.StartedAt > ok.At):
			ok = &PlaceTestStatus{At: run.StartedAt, OK: true, Source: "run"}
		}
	}
	if failed != nil {
		return failed, nil
	}
	return ok, nil
}

// placeLastTest is the newer of the place's last test in this process and
// what the copy runs to its targets last said.
func (s *Service) placeLastTest(id string, rows []store.OffsiteTarget) (*PlaceTestStatus, error) {
	run, err := s.placeLastRun(rows)
	if err != nil {
		return nil, err
	}
	s.placeTestMu.Lock()
	test, tested := s.placeTests[id]
	s.placeTestMu.Unlock()
	if tested && (run == nil || test.At >= run.At) {
		return &test, nil
	}
	return run, nil
}

// placeCredsView shows the credentials the place's rows run with: its own
// set, or the shared credentials while it names none.
func (s *Service) placeCredsView(settings store.Settings, p store.Place) PlaceCredsView {
	v := PlaceCredsView{Fields: map[string]string{}, Set: []string{}}
	fields := credFields[places.Kind(p.Kind)]
	if len(fields) == 0 {
		return v
	}
	set, err := s.credSetFor(settings, p.CredsRef)
	if err != nil {
		log.Printf("api: place %s: could not read its credentials: %v", p.ID, err) //nolint:gosec // G706: the id is store-generated
		return v
	}
	// A set the place names but that is gone falls back to the shared one.
	v.Shared = set.ID == ""
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
		if d.homes[dom] != "" {
			continue
		}
		row, err := s.unplacedPath(d, dom)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		if t.PlaceID != "" {
			continue
		}
		items, err := s.copiedItems(t)
		if err != nil {
			return nil, err
		}
		out = append(out, UnplacedRow{RowID: t.ID, Domain: t.Domain, Role: "target", Name: t.Name, Repo: t.Repo, Immutable: t.Immutable, Items: items})
	}
	for _, r := range d.repos {
		if r.PlaceID != "" {
			continue
		}
		role := "repository"
		if r.CompanionOf != "" {
			role = "direct"
		}
		items, err := s.store.ItemsUsingNamedRepo(r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, UnplacedRow{RowID: r.ID, Role: role, Name: r.Name, Repo: r.Repo, Immutable: r.Immutable, Items: items})
	}
	for i := range out {
		out[i].Protectable = protectable(out[i])
		out[i].Repo = scrubRepoLocation(out[i].Repo)
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
	var moved *placeEstablishedErr
	if errors.As(err, &moved) {
		extra["snapshots"], extra["domains"] = moved.snapshots, moved.domains
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
	// An import drops and rebuilds every place under this lock, and would drop
	// a place written in between.
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
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

// patchPlaceBody is an edit from a place's details. Pointers, so a field the
// form did not send stays as stored.
type patchPlaceBody struct {
	Name                 *string `json:"name"`
	OffPremises          *bool   `json:"offPremises"`
	StorageClass         *string `json:"storageClass"`
	Immutable            *bool   `json:"immutable"`
	RetentionKeepLast    *int    `json:"retentionKeepLast"`
	RetentionKeepDaily   *int    `json:"retentionKeepDaily"`
	RetentionKeepWeekly  *int    `json:"retentionKeepWeekly"`
	RetentionKeepMonthly *int    `json:"retentionKeepMonthly"`
	LimitUpload          *int    `json:"limitUpload"`
	LimitDownload        *int    `json:"limitDownload"`
	GrowthBudgetGB       *int    `json:"growthBudgetGb"`
	Enabled              *bool   `json:"enabled"`
	// Address is the provider's form the base is built from, secrets left out.
	Address map[string]string `json:"address"`
	// Fields is the provider's credential form; a secret left blank keeps
	// the stored one.
	Fields  map[string]string `json:"fields"`
	Folders map[string]string `json:"folders"`
}

// credsChange is the credential set a details edit writes.
type credsChange struct {
	set  CloudCredSet                        // the place's set once written
	fork bool                                // set is new, and the place moves onto it
	edit func([]CloudCredSet) []CloudCredSet // writes the change into the stored sets
}

// placeCredsChange works out what an edit's credential fields change, nil for
// nothing. A place on the shared credentials, on a set that is missing, or on
// a set anything else names gets a set of its own, so the edit reaches this
// place alone.
func (s *Service) placeCredsChange(settings store.Settings, p store.Place, fields map[string]string) (*credsChange, error) {
	provider, _ := places.ProviderByID(p.Provider)
	if len(credFields[provider.Kind]) == 0 {
		return nil, nil
	}
	cur, err := s.credSetFor(settings, p.CredsRef)
	if err != nil {
		return nil, err
	}
	typed := places.CredsFromFields(provider, fields)
	next := withPlaceCreds(cur, overlayCreds(placeCredsOf(cur), typed))
	if next == cur {
		return nil, nil
	}
	fork := p.CredsRef == "" || cur.ID != p.CredsRef
	if !fork {
		if fork, err = s.credSetNamedElsewhere(p.CredsRef, p.ID); err != nil {
			return nil, err
		}
	}
	if !fork {
		// Applied again to the stored set inside the write, so a change another
		// writer made to it in between is kept.
		return &credsChange{set: next, edit: func(sets []CloudCredSet) []CloudCredSet {
			for i := range sets {
				if sets[i].ID == p.CredsRef {
					sets[i] = withPlaceCreds(sets[i], overlayCreds(placeCredsOf(sets[i]), typed))
				}
			}
			return sets
		}}, nil
	}
	next.ID, next.Name, next.KeptFor, next.Kind = newCredSetID(), p.Name, "", string(provider.Kind)
	return &credsChange{set: next, fork: true, edit: func(sets []CloudCredSet) []CloudCredSet {
		return append(sets, next)
	}}, nil
}

// credSetNamedElsewhere reports whether anything but the rows of the place
// except names the credential set: another place, a pull source, or a row
// the place does not hold, a domain's primary row among them, since that one
// carries the credentials of a remote domain path. With except empty every
// row counts.
func (s *Service) credSetNamedElsewhere(id, except string) (bool, error) {
	all, err := s.store.ListPlaces()
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(all, func(p store.Place) bool { return p.ID != except && p.CredsRef == id }) {
		return true, nil
	}
	sources, err := s.store.ListPullSources()
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(sources, func(src store.PullSource) bool { return src.CredsRef == id }) {
		return true, nil
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return false, err
	}
	repos, err := s.store.ListNamedRepos()
	if err != nil {
		return false, err
	}
	rows := slices.Concat(targets, repos)
	for _, d := range places.Domains {
		primary, found, err := s.store.PrimaryRemoteTarget(d)
		if err != nil {
			return false, err
		}
		if found {
			rows = append(rows, primary)
		}
	}
	return slices.ContainsFunc(rows, func(r store.OffsiteTarget) bool {
		return r.CredsRef == id && (except == "" || r.PlaceID != except)
	}), nil
}

// applyPlacePatch merges the sent settings onto a place. A home place stays
// on, since the domains it holds would have nowhere to write.
func applyPlacePatch(p store.Place, b patchPlaceBody, homes map[string]string) (store.Place, error) {
	if b.Name != nil {
		name := strings.TrimSpace(*b.Name)
		if name == "" {
			return p, errPlaceNameMissing
		}
		p.Name = name
	}
	if b.OffPremises != nil {
		// A cloud or this server keeps the answer its provider fixes, as when
		// the place was added.
		provider, _ := places.ProviderByID(p.Provider)
		p.OffPremises = *b.OffPremises
		if provider.OffPremises != nil {
			p.OffPremises = *provider.OffPremises
		}
	}
	if b.StorageClass != nil {
		class := strings.ToUpper(strings.TrimSpace(*b.StorageClass))
		if class != "" && !restic.StorageClassAllowed(class) {
			return p, fmt.Errorf("unsupported storage class %s (allowed: %s)", class, strings.Join(restic.AllowedStorageClasses, ", "))
		}
		p.StorageClass = class
	}
	if b.Immutable != nil {
		if *b.Immutable && p.Kind == string(places.KindLocal) {
			return p, errLocalAppendOnly
		}
		p.Immutable = *b.Immutable
	}
	for _, f := range []struct{ dst, sent *int }{
		{&p.RetentionKeepLast, b.RetentionKeepLast}, {&p.RetentionKeepDaily, b.RetentionKeepDaily},
		{&p.RetentionKeepWeekly, b.RetentionKeepWeekly}, {&p.RetentionKeepMonthly, b.RetentionKeepMonthly},
		{&p.LimitUpload, b.LimitUpload}, {&p.LimitDownload, b.LimitDownload}, {&p.GrowthBudgetGB, b.GrowthBudgetGB},
	} {
		if f.sent != nil {
			*f.dst = max(0, *f.sent)
		}
	}
	if b.Enabled != nil {
		if !*b.Enabled && slices.Contains(slices.Collect(maps.Values(homes)), p.ID) {
			return p, errPlaceHomeDomain
		}
		p.Enabled = *b.Enabled
	}
	return p, nil
}

// patchPlace applies an edit from the details and returns the warnings the
// direct repositories of the place's targets get from it. New credentials are
// tried before anything is written, and a new base or new folders are held to
// the address rules, opened with the credentials the edit brings.
func (s *Service) patchPlace(ctx context.Context, id string, body patchPlaceBody) (store.Place, []saveWarning, error) {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	p, err := s.store.GetPlace(id)
	if err != nil {
		return store.Place{}, nil, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.Place{}, nil, err
	}
	homes, err := s.store.DomainPlaces()
	if err != nil {
		return store.Place{}, nil, err
	}
	rows, err := s.store.PlaceRows(id)
	if err != nil {
		return store.Place{}, nil, err
	}
	next, err := applyPlacePatch(p, body, homes)
	if err != nil {
		return store.Place{}, nil, err
	}
	var change *credsChange
	if body.Fields != nil {
		if change, err = s.placeCredsChange(settings, next, body.Fields); err != nil {
			return store.Place{}, nil, err
		}
		if change != nil && change.fork {
			next.CredsRef = change.set.ID
		}
	}
	if body.Address != nil {
		provider, _ := places.ProviderByID(p.Provider)
		if next.Base, err = places.Base(provider, body.Address, p.ID); err != nil {
			return store.Place{}, nil, err
		}
		if next.Base != p.Base && provider.Kind == places.KindREST && restPathDeep(body.Address) && !placeIsRepository(p, rows) {
			return store.Place{}, nil, errRestPathTooDeep
		}
		if next.Base != p.Base && provider.Kind == places.KindLocal {
			if err := s.localPlaceReady(placeProbe{provider: provider, base: next.Base}); err != nil {
				return store.Place{}, nil, probeRefusal(err)
			}
		}
	}
	if body.Folders != nil {
		if err := checkFolderChange(p, rows, body.Folders); err != nil {
			return store.Place{}, nil, err
		}
		next.Folders = maps.Clone(body.Folders)
	}
	// On a new base checkMoves opens every moved address with the new
	// credentials; otherwise the addresses the place has are tried here.
	if change != nil && next.Base == p.Base {
		probe, err := s.placeProbeFn()(ctx, ProbeRequest{PlaceID: p.ID, Fields: body.Fields})
		if err != nil {
			return store.Place{}, nil, err
		}
		if !probe.OK {
			return store.Place{}, nil, &probeFailedErr{result: probe}
		}
	}
	locks := &domainLocks{s: s}
	defer locks.release()
	if next.Base != p.Base || !maps.Equal(next.Folders, p.Folders) {
		moves, err := s.placeMoves(settings, p, next, rows, homes, locks)
		if err != nil {
			return store.Place{}, nil, err
		}
		if err := s.checkNesting(settings, moves); err != nil {
			return store.Place{}, nil, err
		}
		mode, err := s.pendingMode(settings, next, change)
		if err != nil {
			return store.Place{}, nil, err
		}
		if err := s.checkMoves(ctx, moves, mode); err != nil {
			return store.Place{}, nil, err
		}
		// With nothing moved, checkMoves opened nothing, so the new credentials
		// are tried at the folders of the new base.
		if change != nil && len(moves) == 0 {
			if probe := s.probeFolders(ctx, next.Base, next.Folders, mode); !probe.OK {
				return store.Place{}, nil, &probeFailedErr{result: probe}
			}
		}
	}
	var edit func([]CloudCredSet) []CloudCredSet
	if change != nil {
		edit = change.edit
	}
	saved, err := s.writePlace(store.PlaceWrite{Place: next}, edit)
	if err != nil {
		return store.Place{}, nil, err
	}
	after, err := s.store.PlaceRows(id)
	if err != nil {
		return store.Place{}, nil, err
	}
	warnings := s.placeSaveWarnings(rows, after)
	if change != nil {
		// settings is the row from before the write, so a direct repository the
		// new values do not open can go back to the old ones.
		warnings = append(warnings, s.directCredsWarnings(ctx, settings, func(_, target store.OffsiteTarget) bool {
			return target.PlaceID == p.ID
		})...)
	}
	return saved, warnings, nil
}

// checkFolderChange refuses folders a place cannot take: any folder below a
// place that is itself a repository, and a blank one anywhere else, which
// would make the place one and move its rows onto the base.
func checkFolderChange(p store.Place, rows []store.OffsiteTarget, folders map[string]string) error {
	if err := checkFolders(folders); err != nil {
		return err
	}
	repository := placeIsRepository(p, rows)
	for _, f := range folders {
		switch {
		case repository && f != "":
			return errPlaceIsRepository
		case !repository && f == "":
			return errPlaceFolderBlank
		}
	}
	return nil
}

// placeSaveWarnings compares each target of the place before and after a
// save, as a target save does. The save stands either way, so a failed read
// is logged, not returned.
func (s *Service) placeSaveWarnings(before, after []store.OffsiteTarget) []saveWarning {
	out := []saveWarning{}
	for _, a := range after {
		i := slices.IndexFunc(before, func(b store.OffsiteTarget) bool { return b.ID == a.ID })
		if a.Role != store.RoleOffsite || i < 0 {
			continue
		}
		w, err := s.directSaveWarnings(before[i], a)
		if err != nil {
			log.Printf("api: target %s: could not check its direct repository after a place save: %v", a.ID, err) //nolint:gosec // G706: the id is store-generated
			continue
		}
		out = append(out, w...)
	}
	return out
}

// handlePatchPlace serves PATCH /api/places/{id}.
func (h *Handler) handlePatchPlace(w http.ResponseWriter, r *http.Request) {
	var body patchPlaceBody
	if !decodeBody(w, r, &body) {
		return
	}
	p, warnings, err := h.svc.patchPlace(r.Context(), r.PathValue("id"), body)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case err != nil:
		placeFail(w, err)
	default:
		h.writePlaceAnswer(w, p.ID, map[string]any{"warnings": warnings})
	}
}

// deletePlace removes an unused place with its targets, and its credential
// set once nothing else names it.
func (s *Service) deletePlace(id string) (int, error) {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	p, err := s.store.GetPlace(id)
	if err != nil {
		return 0, err
	}
	removed, err := s.store.DeletePlaceIfUnused(id)
	if err != nil {
		return 0, err
	}
	if p.CredsRef != "" {
		s.dropUnusedCredSet(p.CredsRef)
	}
	return removed, nil
}

// dropUnusedCredSet removes a removed place's credential set when nothing
// else names it. The place is gone either way, so a failure only leaves the
// set behind and is logged.
func (s *Service) dropUnusedCredSet(id string) {
	named, err := s.credSetNamedElsewhere(id, "")
	if err == nil && !named {
		err = s.editCloudCredSets(func(sets []CloudCredSet) []CloudCredSet {
			return slices.DeleteFunc(sets, func(c CloudCredSet) bool { return c.ID == id })
		})
	}
	if err != nil {
		log.Printf("api: credential set %q: could not remove it with its place: %v", id, err)
	}
}

// placeHoldersView is what keeps a place from being removed, as the refusal
// names it.
func placeHoldersView(h store.PlaceHolders) map[string]any {
	items := make([]map[string]string, 0, len(h.Items))
	for _, it := range h.Items {
		items = append(items, map[string]string{"domain": it.Domain, "key": it.Key})
	}
	return map[string]any{"homeDomains": h.HomeDomains, "defaults": h.Defaults, "items": items, "directInUse": h.DirectInUse}
}

// handleDeletePlace serves DELETE /api/places/{id}.
func (h *Handler) handleDeletePlace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	removed, err := h.svc.deletePlace(id)
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case errors.Is(err, store.ErrPlaceInUse):
		holders, hErr := h.store.PlaceHolders(id)
		if hErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(hErr))
			return
		}
		placementFail(w, err, map[string]any{"holders": placeHoldersView(holders)})
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"removedTargets": removed}))
	}
}

// testPlace probes every address the place stands for: each domain's folder,
// and every row it holds under another ending. A local place is also held to
// what the add window checks, since the folders of a share with nothing
// mounted only look absent. The outcome is kept for the places list.
func (s *Service) testPlace(ctx context.Context, id string) (places.ProbeResult, error) {
	p, err := s.store.GetPlace(id)
	if err != nil {
		return places.ProbeResult{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return places.ProbeResult{}, err
	}
	rows, err := s.store.PlaceRows(id)
	if err != nil {
		return places.ProbeResult{}, err
	}
	mode, err := s.placeMode(settings, p)
	if err != nil {
		return places.ProbeResult{}, err
	}
	res := s.probeFolders(ctx, p.Base, p.Folders, mode)
	reason := ""
	fail := func(msg string) {
		res.OK = false
		if reason == "" {
			reason = msg
		}
	}
	if p.Kind == string(places.KindLocal) {
		provider, _ := places.ProviderByID(p.Provider)
		if err := s.localPlaceReady(placeProbe{provider: provider, base: p.Base}); err != nil {
			fail(probeReason(err))
		}
	}
	probed := map[string]bool{}
	for _, d := range places.Domains {
		if problem, failed := res.Errors[d]; failed {
			fail(problem.Message)
		}
		if addr, ok := store.PlaceAddress(p, d, ""); ok {
			probed[addr] = true
		}
	}
	for _, r := range rows {
		// A domain's primary row lies at the domain's folder, probed above.
		if r.Role == store.RolePrimary || probed[r.Repo] {
			continue
		}
		probed[r.Repo] = true
		// A direct repository may still run on credentials the place has left.
		if at := s.probeFolder(ctx, r.Repo, s.rowMode(settings, r)); at.problem != nil {
			fail(at.problem.Message)
		}
	}
	if !res.OK {
		res.Code, res.Error = placementCode(errPlaceProbeFailed), reason
	}
	s.recordPlaceTest(id, res)
	return res, nil
}

func (s *Service) recordPlaceTest(id string, res places.ProbeResult) {
	s.placeTestMu.Lock()
	defer s.placeTestMu.Unlock()
	if s.placeTests == nil {
		s.placeTests = map[string]PlaceTestStatus{}
	}
	s.placeTests[id] = PlaceTestStatus{At: time.Now().Unix(), OK: res.OK, Error: res.Error, Source: "test"}
}

// handleTestPlace serves POST /api/places/{id}/test. The probe's result is the
// answer.
func (h *Handler) handleTestPlace(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.testPlace(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrPlaceNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "no such place"})
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, res)
	}
}
