package api

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The kinds of object a storage location is read from. The kind leads the
// location's id.
const (
	locationPath        = "path"
	locationRepo        = "repo"
	locationDestination = "destination"
	locationTarget      = "target"
)

const (
	useHome = "home"
	useCopy = "copy"
)

// storageLocation is one place backups are kept, as a person would name it:
// the folder the domain repositories sit in, a named repository, a
// destination with the targets derived from it, or an off-site target set up
// for one domain. No field carries a secret: an address has its credentials
// cut out, and CredsRef only names a credential set.
type storageLocation struct {
	// ID is the kind of object and its id, such as destination:4f2a.
	ID     string `json:"id"`
	Object string `json:"object"`
	// Kind is offsite for a remote backend and for every place copies go to,
	// local for a folder on this host that backups are written into.
	Kind string `json:"kind"`
	// Provider is the provider a destination was set up with, "" when the
	// location was typed in. Backend is what restic talks to: local, or the
	// scheme of the address.
	Provider string `json:"provider"`
	Mark     string `json:"mark,omitempty"`
	Backend  string `json:"backend"`
	Name     string `json:"name"`
	Where    string `json:"where"`
	CredsRef string `json:"credsRef,omitempty"`
	Enabled  bool   `json:"enabled"`
	// OffPremises says the location counts as a site of its own.
	OffPremises bool              `json:"offPremises"`
	Sections    []locationSection `json:"sections"`
	// Retention, Compression and the limits are what the location carries
	// itself. One it has no value for is left out: the domain folder has no
	// compression or limits apart from its sections'.
	Retention     *store.RetentionKeep `json:"retention,omitempty"`
	Compression   string               `json:"compression,omitempty"`
	LimitUpload   *int                 `json:"limitUpload,omitempty"`
	LimitDownload *int                 `json:"limitDownload,omitempty"`
	Protection    locationProtection   `json:"protection"`
	Capacity      locationCapacity     `json:"capacity"`

	// volumes are the repositories whose volume stands for the location's
	// room, the first one that can be measured counting.
	volumes []string
}

// locationSection is one domain's use of a location: the home its backups are
// written to, or a copy of them.
type locationSection struct {
	Domain string `json:"domain"`
	Use    string `json:"use"`
	// TargetID is the off-site target behind a copy, and behind a home that is
	// a target's direct repository.
	TargetID string `json:"targetId,omitempty"`
	// RepoID is the named repository behind a home, "" for the domain path.
	RepoID string `json:"repoId,omitempty"`
	Where  string `json:"where"`
	// Enabled is the target's switch for a copy, the repository's for a home
	// in one, and the domain's own for its path.
	Enabled bool `json:"enabled"`
	// Primary marks the copy the domain's off-site settings describe.
	Primary       bool                `json:"primary,omitempty"`
	Immutable     bool                `json:"immutable"`
	Retention     store.RetentionKeep `json:"retention"`
	Compression   string              `json:"compression"`
	LimitUpload   int                 `json:"limitUpload"`
	LimitDownload int                 `json:"limitDownload"`
	LastCopy      *copyState          `json:"lastCopy,omitempty"`
	LastTamper    *store.TamperTest   `json:"lastTamper,omitempty"`
}

// copyState is when a target last took a copy and whether one failed since.
type copyState struct {
	// At is when the last copy that succeeded began, 0 when none has.
	At int64 `json:"at"`
	OK bool  `json:"ok"`
	// FailingSince is when the first failed copy after At began.
	FailingSince int64 `json:"failingSince,omitempty"`
}

// locationProtection is what keeps a location's backups from being deleted.
type locationProtection struct {
	Immutable bool `json:"immutable"`
	// Testable says the tamper test can probe the location, which it can for
	// a rest-server only.
	Testable bool `json:"testable"`
	// LastTamper folds the sections' last tests: the oldest of them, and
	// protected only when every one was.
	LastTamper *tamperState `json:"lastTamper,omitempty"`
}

type tamperState struct {
	At        int64 `json:"at"`
	Protected bool  `json:"protected"`
}

// locationCapacity is the room on the volume a location sits on and how fast
// the repositories there fill it. A figure nobody measured is left out.
type locationCapacity struct {
	At    *int64 `json:"at,omitempty"`
	Free  *int64 `json:"freeBytes,omitempty"`
	Used  *int64 `json:"usedBytes,omitempty"`
	Total *int64 `json:"totalBytes,omitempty"`
	// Source names what measured the room, as in StorageForecast.
	Source string `json:"source,omitempty"`
	// Unsupported marks a backend that reports no room at all, such as S3, B2
	// or a REST server.
	Unsupported bool `json:"unsupported,omitempty"`
	// StoredBytes is what the repositories here held at their last size sample.
	StoredBytes        *int64   `json:"storedBytes,omitempty"`
	GrowthBytesPerWeek *int64   `json:"growthBytesPerWeek,omitempty"`
	WeeksToFull        *float64 `json:"weeksToFull,omitempty"`
}

// probedVolume is the answer of a capacity probe asked for by hand.
type probedVolume struct {
	sample      store.VolumeSample
	unsupported bool
}

func (s *Service) probedVolume(key string) (probedVolume, bool) {
	s.probedMu.Lock()
	defer s.probedMu.Unlock()
	p, ok := s.probedVolumes[key]
	return p, ok
}

func (s *Service) keepProbedVolume(key string, p probedVolume) {
	s.probedMu.Lock()
	defer s.probedMu.Unlock()
	if s.probedVolumes == nil {
		s.probedVolumes = map[string]probedVolume{}
	}
	s.probedVolumes[key] = p
}

// errNoSuchLocation is returned for an id that names no storage location.
var errNoSuchLocation = errors.New("no such storage location")

// capacityRefreshEvery is how soon a remote may be asked for its room again
// by hand.
const capacityRefreshEvery = 60

// sizeSeries names one repository's size samples.
type sizeSeries struct{ domain, source string }

// locationBuilder reads the four kinds of object once and turns them into
// locations.
type locationBuilder struct {
	s        *Service
	settings store.Settings
	now      time.Time
	targets  []store.OffsiteTarget
	// directs is each target's direct repository, by target id.
	directs map[string]store.OffsiteTarget
	// copying counts the enabled targets of each domain.
	copying map[string]int
	reader  capacityReader
}

// StorageLocations lists every place backups are kept: the domain folders,
// the named repositories, the destinations and the targets that follow none.
// Remote volumes are not asked for their room here; RefreshLocationCapacity
// does that for one location.
func (s *Service) StorageLocations() ([]storageLocation, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	b := locationBuilder{s: s, settings: settings, now: time.Now(), reader: capacityReader{s: s},
		directs: map[string]store.OffsiteTarget{}, copying: map[string]int{}}
	if b.targets, err = s.store.ListOffsiteTargets(); err != nil {
		return nil, err
	}
	for _, t := range b.targets {
		if t.Enabled {
			b.copying[t.Domain]++
		}
	}
	named, err := s.store.ListNamedRepos()
	if err != nil {
		return nil, err
	}
	for _, r := range named {
		if r.CompanionOf != "" {
			b.directs[r.CompanionOf] = r
		}
	}
	users, err := s.store.NamedRepoDomains()
	if err != nil {
		return nil, err
	}
	destinations, err := s.store.ListDestinations()
	if err != nil {
		return nil, err
	}

	out := b.pathLocations()
	for _, r := range named {
		// A direct repository is the same place as its target and is listed there.
		if r.CompanionOf == "" {
			out = append(out, b.repoLocation(r, users[r.ID]))
		}
	}
	for _, d := range destinations {
		out = append(out, b.destinationLocation(d))
	}
	for _, t := range b.targets {
		if t.DestinationID == "" && !b.leftover(t) {
			out = append(out, b.targetLocation(t))
		}
	}
	for i := range out {
		if out[i].Sections == nil {
			out[i].Sections = []locationSection{}
		}
	}
	return out, nil
}

// StorageLocation returns the location with the given id.
func (s *Service) StorageLocation(id string) (storageLocation, error) {
	all, err := s.StorageLocations()
	if err != nil {
		return storageLocation{}, err
	}
	i := slices.IndexFunc(all, func(l storageLocation) bool { return l.ID == id })
	if i < 0 {
		return storageLocation{}, errNoSuchLocation
	}
	return all[i], nil
}

// RefreshLocationCapacity asks the remote a location sits on how much room it
// has and keeps the answer for the reads that follow. A local disk is
// measured on every read, and a backend that reports no room is not asked.
func (s *Service) RefreshLocationCapacity(ctx context.Context, id string) error {
	loc, err := s.StorageLocation(id)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(loc.volumes, func(repo string) bool { return remoteVolumeSource(repo) != "" })
	if i < 0 {
		return nil
	}
	repo, key, now := loc.volumes[i], remoteVolumeKey(loc.volumes[i]), s.anomalies.nowUnix()
	if now-s.anomalies.lastVolumeProbes()[key] < capacityRefreshEvery {
		return nil
	}
	sample, err := s.probeVolume(ctx, domainRepoRef{Loc: repo}, map[string]int64{}, now)
	if errors.Is(err, errAboutUnsupported) {
		s.keepProbedVolume(key, probedVolume{sample: store.VolumeSample{At: now}, unsupported: true})
		return nil
	}
	if err != nil {
		return err
	}
	// A volume the capacity rule already follows gets the reading as one more
	// sample. Any other stays out of its table, or the rule would start
	// judging a volume no backup measures.
	if last, followed := s.newestVolumeReadings()[key]; followed {
		sample.Domains = last.Domains
		if err := s.store.AddVolumeSample(*sample); err != nil {
			return err
		}
		s.anomalies.MarkVolumeDirty()
		return nil
	}
	s.keepProbedVolume(key, probedVolume{sample: *sample})
	return nil
}

// pathRoot is the folder a domain path sits in: the location without its last
// segment. A remote whose address has no folder at all is its own root.
func pathRoot(loc string) string {
	loc = strings.TrimRight(strings.TrimSpace(loc), "/")
	host := 0
	if i := strings.Index(loc, "://"); i >= 0 {
		host = i + len("://")
	}
	if i := strings.LastIndexByte(loc[host:], '/'); i >= 0 {
		return loc[:host+i]
	}
	if !restic.IsRemoteRepo(loc) {
		return ""
	}
	if host > 0 {
		return loc
	}
	return loc[:strings.LastIndexByte(loc, ':')+1]
}

// backendOf names what restic talks to at loc.
func backendOf(loc string) string {
	if !restic.IsRemoteRepo(loc) {
		return "local"
	}
	scheme, _, _ := strings.Cut(strings.TrimSpace(loc), ":")
	return scheme
}

func locationKind(loc string) string {
	if restic.IsRemoteRepo(loc) {
		return "offsite"
	}
	return "local"
}

// pathLocations groups the domain paths by the folder they sit in: domains
// whose paths share that folder are one location, since that folder is what
// was picked as the place for the backups. Two logins on one remote folder
// count as the same place.
func (b *locationBuilder) pathLocations() []storageLocation {
	var out []storageLocation
	at := map[string]int{}
	series := map[string][]sizeSeries{}
	for _, domain := range offsiteConfigDomains {
		raw := strings.TrimSpace(domainPathRaw(domain, b.settings))
		if raw == "" {
			continue
		}
		root := scrubRepoLocation(pathRoot(raw))
		i, known := at[root]
		if !known {
			i = len(out)
			at[root] = i
			name := root
			if cut := strings.LastIndexAny(root, "/:"); cut >= 0 {
				name = root[cut+1:]
			}
			keep := sharedLocalKeep(b.settings)
			out = append(out, storageLocation{
				ID: locationPath + ":" + repoLocationKey(root), Object: locationPath,
				Kind: locationKind(raw), Backend: backendOf(raw), Name: name, Where: root,
				Enabled: true, OffPremises: restic.IsRemoteRepo(raw), Retention: &keep,
				Protection: locationProtection{Immutable: true, Testable: backendOf(raw) == "rest"},
			})
		}
		sec := b.pathSection(domain, raw)
		loc := &out[i]
		loc.Sections = append(loc.Sections, sec)
		loc.Protection.Immutable = loc.Protection.Immutable && sec.Immutable
		if repo, err := b.s.resolveRepo(raw); err == nil {
			loc.volumes = append(loc.volumes, repo)
		}
		series[root] = append(series[root], sizeSeries{domain, "local"})
	}
	for i := range out {
		out[i].Protection.LastTamper = foldTamper(out[i].Sections)
		out[i].Capacity = b.capacity(out[i].volumes, series[out[i].Where])
	}
	return out
}

// pathSection is a domain's own repository. A remote one takes its limits and
// its append-only flag from the domain's safety row, as its backups do.
func (b *locationBuilder) pathSection(domain, raw string) locationSection {
	sec := locationSection{
		Domain: domain, Use: useHome, Where: scrubRepoLocation(raw), Enabled: domainEnabled(b.settings, domain),
		Retention:   localKeep(b.settings, domain),
		Compression: normalizedCompression(b.settings.CompressionFor(domain)),
	}
	if !restic.IsRemoteRepo(raw) {
		return sec
	}
	if row, ok := b.s.primaryRemoteTarget(domain); ok && row.Enabled {
		sec.Immutable, sec.LimitUpload, sec.LimitDownload = row.Immutable, row.LimitUpload, row.LimitDownload
		sec.LastTamper = b.lastTamper(domain, row.ID)
	}
	return sec
}

// repoLocation is a named repository with the domains that keep backups
// there. What is written to it ages by the local policy of its domain.
func (b *locationBuilder) repoLocation(r store.OffsiteTarget, domains []string) storageLocation {
	keep := sharedLocalKeep(b.settings)
	loc := storageLocation{
		ID: locationRepo + ":" + r.ID, Object: locationRepo, Kind: locationKind(r.Repo),
		Provider: r.Provider, Mark: providerMark(r.Provider), Backend: backendOf(r.Repo),
		Name: r.Name, Where: scrubRepoLocation(r.Repo), CredsRef: r.CredsRef,
		Enabled: r.Enabled, OffPremises: r.OffPremises,
		Retention: &keep, Compression: normalizedCompression(r.Compression),
		LimitUpload: &r.LimitUpload, LimitDownload: &r.LimitDownload,
		Protection: locationProtection{Immutable: r.Immutable, Testable: backendOf(r.Repo) == "rest"},
	}
	for _, domain := range domains {
		loc.Sections = append(loc.Sections, locationSection{
			Domain: domain, Use: useHome, RepoID: r.ID, Where: loc.Where, Enabled: r.Enabled, Immutable: r.Immutable,
			Retention: localKeep(b.settings, domain), Compression: loc.Compression,
			LimitUpload: r.LimitUpload, LimitDownload: r.LimitDownload,
		})
	}
	if repo, err := b.s.resolveRepo(r.Repo); err == nil {
		loc.volumes = []string{repo}
	}
	loc.Capacity = b.capacity(loc.volumes, nil)
	return loc
}

// destinationLocation is a destination with the targets derived from it, one
// section per domain.
func (b *locationBuilder) destinationLocation(d store.OffsiteTarget) storageLocation {
	loc := storageLocation{
		ID: locationDestination + ":" + d.ID, Object: locationDestination, Kind: "offsite",
		Provider: d.Provider, Mark: providerMark(d.Provider), Backend: backendOf(d.Repo),
		Name: d.Name, Where: scrubRepoLocation(d.Repo), CredsRef: d.CredsRef,
		Enabled: true, OffPremises: true,
		Protection: locationProtection{Immutable: d.Immutable, Testable: backendOf(d.Repo) == "rest"},
	}
	if repo, err := b.s.resolveRepo(d.Repo); err == nil {
		loc.volumes = []string{repo}
	}
	var derived []store.OffsiteTarget
	for _, t := range b.targets {
		if t.DestinationID == d.ID {
			derived = append(derived, t)
		}
	}
	slices.SortStableFunc(derived, func(x, y store.OffsiteTarget) int {
		return cmp.Compare(slices.Index(offsiteConfigDomains, x.Domain), slices.Index(offsiteConfigDomains, y.Domain))
	})
	var series []sizeSeries
	for _, t := range derived {
		b.addTarget(&loc, t)
		series = append(series, b.seriesOf(t))
	}
	loc.Protection.LastTamper = foldTamper(loc.Sections)
	loc.Capacity = b.capacity(loc.volumes, series)
	return loc
}

// targetLocation is an off-site target that follows no destination.
func (b *locationBuilder) targetLocation(t store.OffsiteTarget) storageLocation {
	keep := targetKeep(t)
	loc := storageLocation{
		ID: locationTarget + ":" + t.ID, Object: locationTarget, Kind: "offsite",
		Provider: t.Provider, Mark: providerMark(t.Provider), Backend: backendOf(t.Repo),
		Name: t.Name, Where: scrubRepoLocation(t.Repo), CredsRef: t.CredsRef,
		Enabled: t.Enabled, OffPremises: true,
		Retention: &keep, Compression: normalizedCompression(t.Compression),
		LimitUpload: &t.LimitUpload, LimitDownload: &t.LimitDownload,
		Protection: locationProtection{Immutable: t.Immutable, Testable: backendOf(t.Repo) == "rest"},
	}
	b.addTarget(&loc, t)
	loc.Protection.LastTamper = foldTamper(loc.Sections)
	loc.Capacity = b.capacity(loc.volumes, []sizeSeries{b.seriesOf(t)})
	return loc
}

// leftover reports whether t is the row a cleared off-site field leaves
// behind: switched off in the primary slot, kept only so the next fill finds
// its id again.
func (b *locationBuilder) leftover(t store.OffsiteTarget) bool {
	return t.SortOrder == 0 && !t.Enabled && offsiteRepoFromSettings(t.Domain, b.settings) == ""
}

// addTarget adds a target's copy to loc and, when the target has a direct
// repository, the home that repository is.
func (b *locationBuilder) addTarget(loc *storageLocation, t store.OffsiteTarget) {
	loc.Sections = append(loc.Sections, locationSection{
		Domain: t.Domain, Use: useCopy, TargetID: t.ID, Where: scrubRepoLocation(t.Repo),
		Enabled: t.Enabled, Primary: t.SortOrder == 0, Immutable: t.Immutable,
		Retention: targetKeep(t), Compression: normalizedCompression(t.Compression),
		LimitUpload: t.LimitUpload, LimitDownload: t.LimitDownload,
		LastCopy: b.lastCopy(t), LastTamper: b.lastTamper(t.Domain, t.ID),
	})
	if repo, err := b.s.resolveRepo(t.Repo); err == nil {
		loc.volumes = append(loc.volumes, repo)
	}
	direct, ok := b.directs[t.ID]
	if !ok {
		return
	}
	loc.Sections = append(loc.Sections, locationSection{
		Domain: t.Domain, Use: useHome, TargetID: t.ID, RepoID: direct.ID, Where: scrubRepoLocation(direct.Repo),
		Enabled: direct.Enabled, Immutable: direct.Immutable,
		Retention: targetKeep(direct), Compression: normalizedCompression(direct.Compression),
		LimitUpload: direct.LimitUpload, LimitDownload: direct.LimitDownload,
	})
	if repo, err := b.s.resolveRepo(direct.Repo); err == nil {
		loc.volumes = append(loc.volumes, repo)
	}
}

func targetKeep(t store.OffsiteTarget) store.RetentionKeep {
	return store.RetentionKeep{
		KeepLast: t.RetentionKeepLast, KeepDaily: t.RetentionKeepDaily, KeepWeekly: t.RetentionKeepWeekly,
		KeepMonthly: t.RetentionKeepMonthly, KeepYearly: t.RetentionKeepYearly,
	}
}

// seriesOf names the size samples of a target's repository. A domain's only
// enabled target is sampled under the bare off-site source.
func (b *locationBuilder) seriesOf(t store.OffsiteTarget) sizeSeries {
	if t.Enabled && b.copying[t.Domain] == 1 {
		return sizeSeries{t.Domain, "offsite"}
	}
	return sizeSeries{t.Domain, offsiteStatSource(t.ID)}
}

// lastCopy reads a target's last successful copy and the first failure since.
// Runs from before targets had ids carry none, and count for a domain's only
// enabled target.
func (b *locationBuilder) lastCopy(t store.OffsiteTarget) *copyState {
	run, found, err := b.s.store.LatestSuccessfulOffsiteRunForTarget(t.Domain, t.ID)
	if err == nil && !found && t.Enabled && b.copying[t.Domain] == 1 {
		run, found, err = b.s.store.LatestSuccessfulOffsiteRun(t.Domain)
	}
	if err != nil {
		return nil
	}
	failing, err := b.s.store.FirstOffsiteFailureAfter(t.ID, run.StartedAt)
	if err != nil || (!found && failing == 0) {
		return nil
	}
	return &copyState{At: run.StartedAt, OK: found && failing == 0, FailingSince: failing}
}

func (b *locationBuilder) lastTamper(domain, targetID string) *store.TamperTest {
	test, found, err := b.s.store.LatestTamperTestForTarget(domain, targetID)
	if err != nil || !found {
		return nil
	}
	return &test
}

func foldTamper(sections []locationSection) *tamperState {
	var out *tamperState
	for _, sec := range sections {
		switch {
		case sec.LastTamper == nil:
		case out == nil:
			out = &tamperState{At: sec.LastTamper.At, Protected: sec.LastTamper.Protected}
		default:
			out.At = min(out.At, sec.LastTamper.At)
			out.Protected = out.Protected && sec.LastTamper.Protected
		}
	}
	return out
}

// capacity measures the first of repos that can be measured and adds up the
// growth of the repositories behind series.
func (b *locationBuilder) capacity(repos []string, series []sizeSeries) locationCapacity {
	var out locationCapacity
	for _, repo := range repos {
		c := b.reader.measure(repoCapacity{}, repo)
		if c.Free != nil {
			out.At, out.Free, out.Used, out.Total, out.Source = c.At, c.Free, c.Used, c.Total, c.Source
			out.Unsupported = false
			break
		}
		out.Unsupported = out.Unsupported || c.Unsupported
	}
	var stored, growth int64
	var sized, grows bool
	for _, sr := range series {
		stats, err := b.s.store.ListRepoStats(sr.domain, sr.source, 0)
		if err != nil || len(stats) == 0 {
			continue
		}
		stored, sized = stored+stats[len(stats)-1].RawSize, true
		if week, ok := growthBytesPerWeek(stats, b.now); ok {
			growth, grows = growth+week, true
		}
	}
	if sized {
		out.StoredBytes = &stored
	}
	if grows {
		out.GrowthBytesPerWeek = &growth
		if out.Free != nil {
			if weeks, ok := weeksToFull(*out.Free, growth); ok {
				out.WeeksToFull = &weeks
			}
		}
	}
	return out
}

// handleListStorageLocations lists every storage location with the room last
// measured for it.
// GET /api/storage/locations
func (h *Handler) handleListStorageLocations(w http.ResponseWriter, _ *http.Request) {
	list, err := h.svc.StorageLocations()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if list == nil {
		list = []storageLocation{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"locations": list}))
}

// handleGetStorageLocation returns one storage location. With
// ?refresh=capacity its remote is asked for its room first; a probe that
// fails leaves the last reading in place and is named in capacityError.
// GET /api/storage/locations/{id}
func (h *Handler) handleGetStorageLocation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body := map[string]any{}
	if r.URL.Query().Get("refresh") == "capacity" {
		err := h.svc.RefreshLocationCapacity(r.Context(), id)
		if err != nil && !errors.Is(err, errNoSuchLocation) {
			body["capacityError"] = scrubError(err)
		}
	}
	loc, err := h.svc.StorageLocation(id)
	if errors.Is(err, errNoSuchLocation) {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	body["location"] = loc
	writeJSON(w, http.StatusOK, okEnvelope(body))
}
