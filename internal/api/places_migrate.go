package api

import (
	"cmp"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// directSuffix is the ending of a direct repository's address after its
// target's folder at the same place.
const directSuffix = "-direct"

// placeKindsByScheme are the remote schemes a place can hold. b2:, gs:,
// swift: and azure: addresses take their keys from the container's
// environment, which no place knows about, so they stay without a place.
var placeKindsByScheme = map[string]places.Kind{
	"s3": places.KindS3, "rest": places.KindREST, "sftp": places.KindSFTP, "rclone": places.KindRclone,
}

// placeKindOf is the kind of place that can hold loc. A word and a colon that
// restic does not know is a mistyped remote, not a local folder.
func placeKindOf(loc string) (places.Kind, bool) {
	if strings.TrimSpace(loc) == "" || restic.LooksLikeUnprefixedRemote(loc) {
		return "", false
	}
	if !restic.IsRemoteRepo(loc) {
		return places.KindLocal, true
	}
	scheme, _, _ := strings.Cut(loc, ":")
	kind, ok := placeKindsByScheme[scheme]
	return kind, ok
}

// addressRoot is the part of loc that names its store and holds no folder:
// scheme, host and bucket of an s3 address, the server of a rest: or sftp://
// address, the host or remote with its colon in the colon forms, and nothing
// for a local path.
func addressRoot(kind places.Kind, loc string) (string, bool) {
	scheme, rest, _ := strings.Cut(loc, ":")
	switch kind {
	case places.KindLocal:
		return "", true
	case places.KindS3, places.KindREST:
		after := strings.TrimPrefix(strings.TrimPrefix(rest, "https://"), "http://")
		host, path, _ := strings.Cut(after, "/")
		if host == "" {
			return "", false
		}
		end := len(loc) - len(after) + len(host)
		if kind == places.KindS3 {
			bucket, _, _ := strings.Cut(path, "/")
			if bucket == "" {
				return "", false
			}
			end += 1 + len(bucket)
		}
		return loc[:end], true
	case places.KindSFTP, places.KindRclone:
		if url, ok := strings.CutPrefix(rest, "//"); ok {
			host, _, _ := strings.Cut(url, "/")
			if host == "" {
				return "", false
			}
			return loc[:len(loc)-len(url)+len(host)], true
		}
		host, _, found := strings.Cut(rest, ":")
		if !found || host == "" {
			return "", false
		}
		return loc[:len(scheme)+1+len(host)+1], true
	}
	return "", false
}

// placeSplit is an address read as a place and one folder under it. An empty
// folder means the address is the root of its bucket or server and so a
// repository of its own.
type placeSplit struct {
	kind   places.Kind
	base   string
	folder string
}

// splitPlaceAddress reads loc as a base and its last path element, and only
// when places.Join spells loc back byte for byte: a trailing slash, a folder
// right after "sftp:host:/", a local path without a parent or a scheme no
// place has leaves loc unsplit.
func splitPlaceAddress(loc string) (placeSplit, bool) {
	kind, ok := placeKindOf(loc)
	if !ok {
		return placeSplit{}, false
	}
	root, ok := addressRoot(kind, loc)
	if !ok {
		return placeSplit{}, false
	}
	if len(pathElements(loc[len(root):])) == 0 {
		if kind == places.KindLocal {
			return placeSplit{}, false
		}
		return placeSplit{kind: kind, base: loc}, true
	}
	base, folder := root, loc[len(root):]
	if i := strings.LastIndexByte(loc, '/'); i >= len(root) {
		base, folder = loc[:i], loc[i+1:]
	}
	if base == "" || folder == "" || places.Join(base, folder) != loc {
		return placeSplit{}, false
	}
	return placeSplit{kind: kind, base: base, folder: folder}, true
}

// fallbackProviders name the place of an address whose detected provider
// speaks another kind.
var fallbackProviders = map[places.Kind]string{
	places.KindLocal: "unraid-folder", places.KindS3: "s3-other", places.KindREST: "rest-server",
	places.KindSFTP: "sftp", places.KindRclone: "rclone",
}

// placeTraits are the settings a place holds once for all of its rows. Rows
// that differ in any of them cannot share a place without one of them
// changing how it runs.
type placeTraits struct {
	kind                                         places.Kind
	credsRef, storageClass                       string
	immutable                                    bool
	keepLast, keepDaily, keepWeekly, keepMonthly int
	limitUpload, limitDownload, growthBudgetGB   int
}

// globalLocalTraits are what a domain path without a place runs with: no
// credential set, no caps, and the global local keep-policy.
func globalLocalTraits(kind places.Kind, s store.Settings) placeTraits {
	return placeTraits{
		kind:        kind,
		keepLast:    s.RetentionKeepLast,
		keepDaily:   s.RetentionKeepDaily,
		keepWeekly:  s.RetentionKeepWeekly,
		keepMonthly: s.RetentionKeepMonthly,
	}
}

// primaryTraits add what a remote domain path takes from its primary row: the
// credential set whether the row is on or off, and the caps, append-only and
// the budget only while it is on. The row's storage class stays out, because
// primaryModeFor does not apply it to backups of the domain path.
func primaryTraits(kind places.Kind, s store.Settings, primary store.OffsiteTarget) placeTraits {
	t := globalLocalTraits(kind, s)
	t.credsRef = primary.CredsRef
	if primary.Enabled {
		t.immutable = primary.Immutable
		t.limitUpload, t.limitDownload = primary.LimitUpload, primary.LimitDownload
		t.growthBudgetGB = primary.GrowthBudgetGB
	}
	return t
}

// rowTraits are a row's own settings, its keep-policy included, which is the
// one a target and a direct repository age by.
func rowTraits(kind places.Kind, row store.OffsiteTarget) placeTraits {
	return placeTraits{
		kind:           kind,
		credsRef:       row.CredsRef,
		storageClass:   row.StorageClass,
		immutable:      row.Immutable,
		keepLast:       row.RetentionKeepLast,
		keepDaily:      row.RetentionKeepDaily,
		keepWeekly:     row.RetentionKeepWeekly,
		keepMonthly:    row.RetentionKeepMonthly,
		limitUpload:    row.LimitUpload,
		limitDownload:  row.LimitDownload,
		growthBudgetGB: row.GrowthBudgetGB,
	}
}

// namedTraits are a named repository's own settings with the global local
// keep-policy, which a named repository without a place ages by.
func namedTraits(kind places.Kind, s store.Settings, row store.OffsiteTarget) placeTraits {
	t := rowTraits(kind, row)
	local := globalLocalTraits(kind, s)
	t.keepLast, t.keepDaily, t.keepWeekly, t.keepMonthly = local.keepLast, local.keepDaily, local.keepWeekly, local.keepMonthly
	return t
}

// placesMigrationInput is everything the plan reads, gathered first so that
// the plan itself touches neither the store nor the key.
type placesMigrationInput struct {
	settings  store.Settings
	targets   []store.OffsiteTarget          // role offsite
	named     []store.OffsiteTarget          // role repo, direct repositories included
	primaries map[string]store.OffsiteTarget // domain -> its remote-primary row
	credSets  map[string]CloudCredSet        // by id, secrets included
	shared    CloudCreds                     // what a row without a set logs in with
	existing  []store.Place                  // names and addresses the plan must not take
	homes     map[string]string              // domains that have a home place already
	idle      map[string]bool                // domains whose path nothing uses, left without a place
}

// plannedPlace is one place as the plan builds it up.
type plannedPlace struct {
	store.MigratedPlace
	traits       placeTraits
	serverNamed  bool // named after its host, until a target joins it
	repository   bool // one repository by itself, so nothing goes under it
	singleDomain bool // its credentials belong to one domain
}

// targetPlace reports whether targets went to the place, whether or not a
// domain path made it first. Named repositories join later, so until then
// every row is a target.
func (pl *plannedPlace) targetPlace() bool { return len(pl.Rows) > 0 }

// placesPlan is what the migration writes and what it leaves without a place.
type placesPlan struct {
	places   []store.MigratedPlace
	unplaced []string // row ids, and "<domain> path" for a domain path
}

type placesPlanner struct {
	in       placesMigrationInput
	planned  []*plannedPlace
	byTarget map[string]*plannedPlace // target id -> the place it went to
	names    map[string]bool          // names taken, lower case
	unplaced []string
}

// planPlaces works out the places a database from before storage places turns
// into. A row goes onto a place only in a form the place spells back byte for
// byte, and what already has a place stays as it is.
func planPlaces(in placesMigrationInput) placesPlan {
	p := &placesPlanner{in: in, byTarget: map[string]*plannedPlace{}, names: map[string]bool{}}
	for _, e := range in.existing {
		p.names[strings.ToLower(e.Name)] = true
	}
	p.planDomainPaths()
	p.planTargets()
	p.markSingleDomainPlaces()
	p.addDefaultFolders()
	p.planNamedRepos()
	p.checkInvariant()
	plan := placesPlan{unplaced: p.unplaced}
	for _, pl := range p.planned {
		plan.places = append(plan.places, pl.MigratedPlace)
	}
	return plan
}

// planDomainPaths puts each domain path on a home place. Local paths with one
// parent share a place; remote ones share it only with the same credentials
// and the same safety settings from their primary rows.
func (p *placesPlanner) planDomainPaths() {
	for _, d := range places.Domains {
		path := domainPathRaw(d, p.in.settings)
		if p.in.homes[d] != "" || p.in.idle[d] || path == "" {
			continue
		}
		sp, ok := splitPlaceAddress(path)
		if !ok {
			p.unplaced = append(p.unplaced, d+" path")
			continue
		}
		traits, name := globalLocalTraits(sp.kind, p.in.settings), "Unraid"
		if sp.kind != places.KindLocal {
			traits, name = primaryTraits(sp.kind, p.in.settings, p.in.primaries[d]), storeName(sp.base)
		}
		pl := p.joinable(sp, traits, d)
		if p.claimedElsewhere(path, pl) {
			p.unplaced = append(p.unplaced, d+" path")
			continue
		}
		if pl == nil {
			pl = p.add(name, sp, traits, true)
			pl.serverNamed = sp.kind != places.KindLocal
		}
		pl.Place.Folders[d] = sp.folder
		pl.HomeDomains = append(pl.HomeDomains, d)
	}
}

// planTargets puts the targets on places, oldest first, so the later of two
// targets of one domain at one base gets a place of its own. A place is on
// while any of its targets is; each row keeps its own switch.
func (p *placesPlanner) planTargets() {
	targets := slices.Clone(p.in.targets)
	slices.SortStableFunc(targets, func(a, b store.OffsiteTarget) int {
		return cmp.Or(cmp.Compare(a.CreatedAt, b.CreatedAt), strings.Compare(a.ID, b.ID))
	})
	for _, t := range targets {
		if t.PlaceID != "" {
			continue
		}
		sp, ok := splitPlaceAddress(t.Repo)
		if !ok {
			p.unplaced = append(p.unplaced, t.ID)
			continue
		}
		traits := rowTraits(sp.kind, t)
		pl := p.joinable(sp, traits, t.Domain)
		if p.claimedElsewhere(t.Repo, pl) {
			p.unplaced = append(p.unplaced, t.ID)
			continue
		}
		if pl == nil {
			pl = p.add(placementTargetName(t), sp, traits, false)
		}
		if pl.serverNamed {
			p.rename(pl, placementTargetName(t))
		}
		pl.Place.Folders[t.Domain] = sp.folder
		pl.Place.Enabled = pl.Place.Enabled || t.Enabled
		pl.Rows = append(pl.Rows, store.PlaceRowRef{RowID: t.ID, Domain: t.Domain, Repo: t.Repo})
		p.byTarget[t.ID] = pl
	}
}

// markSingleDomainPlaces finds the target places whose credentials belong to
// one domain: an accepted mesh offer, a rest-server user named after the
// domain the way deploy.go names it, and places split from another at the
// same base by nothing but their credentials. They offer their own domain
// only.
func (p *placesPlanner) markSingleDomainPlaces() {
	var targets []*plannedPlace
	for _, pl := range p.planned {
		if pl.targetPlace() && !pl.repository {
			targets = append(targets, pl)
		}
	}
	for i, a := range targets {
		if p.meshCreds(a.traits.credsRef) || p.domainUser(a) {
			a.singleDomain = true
		}
		for _, b := range targets[i+1:] {
			if a.Place.Base != b.Place.Base || a.traits.credsRef == b.traits.credsRef {
				continue
			}
			other := b.traits
			other.credsRef = a.traits.credsRef
			if other == a.traits {
				a.singleDomain, b.singleDomain = true, true
			}
		}
	}
}

// domainUser reports whether a place's rows are all of one domain and log in
// as that domain's own rest-server user.
func (p *placesPlanner) domainUser(pl *plannedPlace) bool {
	if pl.traits.kind != places.KindREST || len(pl.Rows) == 0 {
		return false
	}
	domain := pl.Rows[0].Domain
	for _, r := range pl.Rows {
		if r.Domain != domain {
			return false
		}
	}
	for _, d := range pl.HomeDomains {
		if d != domain {
			return false
		}
	}
	return p.credsFor(pl.traits.credsRef).RESTUser == "bombvault-"+domain
}

// credsFor resolves a credential selector the way decodeCloudFor does: an
// empty or unknown one means the shared credentials.
func (p *placesPlanner) credsFor(ref string) CloudCreds {
	if set, ok := p.in.credSets[ref]; ok {
		return set.CloudCreds
	}
	return p.in.shared
}

// addDefaultFolders lets a target place offer the domains it has no folder for
// yet, under the usual folder names, so flash can be copied into the same
// bucket with one click. A folder whose address would meet one in use is left
// out, and places without targets, repository places and single-domain places
// get none.
func (p *placesPlanner) addDefaultFolders() {
	inUse := p.knownAddresses()
	defaults := places.DefaultFolders()
	for _, pl := range p.planned {
		if !pl.targetPlace() || pl.repository || pl.singleDomain {
			continue
		}
		for _, d := range places.Domains {
			if _, ok := pl.Place.Folders[d]; ok {
				continue
			}
			addr := places.Join(pl.Place.Base, defaults[d])
			meets := func(a string) bool { return repoLocationsOverlap(a, addr) }
			if slices.ContainsFunc(inUse, meets) || p.claimedElsewhere(addr, nil) {
				continue
			}
			pl.Place.Folders[d] = defaults[d]
		}
	}
}

// knownAddresses are the locations the database names, placed or not: every
// row, every domain path and every off-site field.
func (p *placesPlanner) knownAddresses() []string {
	var out []string
	add := func(loc string) {
		if loc != "" {
			out = append(out, loc)
		}
	}
	for _, r := range slices.Concat(p.in.targets, p.in.named) {
		add(r.Repo)
	}
	for _, d := range places.Domains {
		add(domainPathRaw(d, p.in.settings))
		add(offsiteRepoFromSettings(d, p.in.settings))
	}
	return out
}

// planNamedRepos places the rows of role repo. A direct repository joins its
// target's place or stays without one; every other named repository is a
// place of its own, however close it sits to another.
func (p *placesPlanner) planNamedRepos() {
	targets := map[string]store.OffsiteTarget{}
	for _, t := range p.in.targets {
		targets[t.ID] = t
	}
	for _, r := range p.in.named {
		switch {
		case r.PlaceID != "":
		case r.CompanionOf != "":
			p.planDirect(r, targets[r.CompanionOf])
		default:
			p.planNamed(r)
		}
	}
}

// planDirect puts a direct repository on its target's place when it sits right
// beside the target's folder and matches the target in every mirrored field,
// credentials included: a save of the place would otherwise hand it the
// target's keys. One that is on keeps its place on, since items back up to it.
func (p *placesPlanner) planDirect(r, target store.OffsiteTarget) {
	pl, ok := p.byTarget[target.ID]
	if !ok || pl.repository || r.Repo != target.Repo+directSuffix || !target.MirroredEqual(r) {
		p.unplaced = append(p.unplaced, r.ID)
		return
	}
	pl.Place.Enabled = pl.Place.Enabled || r.Enabled
	pl.Rows = append(pl.Rows, store.PlaceRowRef{RowID: r.ID, Domain: target.Domain, Suffix: directSuffix, Repo: r.Repo})
}

// planNamed makes a named repository a place that is itself one repository
// for every domain, since items of any domain can pick it.
func (p *placesPlanner) planNamed(r store.OffsiteTarget) {
	kind, ok := placeKindOf(r.Repo)
	if !ok || p.claimedElsewhere(r.Repo, nil) {
		p.unplaced = append(p.unplaced, r.ID)
		return
	}
	pl := p.add(placementTargetName(r), placeSplit{kind: kind, base: r.Repo}, namedTraits(kind, p.in.settings, r), false)
	for _, d := range places.Domains {
		pl.Place.Folders[d] = ""
	}
	pl.Place.OffPremises = r.OffPremises
	pl.Place.Enabled = r.Enabled
	pl.Rows = append(pl.Rows, store.PlaceRowRef{RowID: r.ID, Repo: r.Repo})
}

// checkInvariant drops every row and domain path whose place does not spell
// its address back byte for byte, the rule every later write to a place keeps.
// What it drops stays without a place and runs as it did.
func (p *placesPlanner) checkInvariant() {
	for _, pl := range p.planned {
		pl.Rows = slices.DeleteFunc(pl.Rows, func(r store.PlaceRowRef) bool {
			if addr, ok := places.Address(pl.Place.Base, pl.Place.Folders, r.Domain, r.Suffix); ok && addr == r.Repo {
				return false
			}
			p.unplaced = append(p.unplaced, r.RowID)
			return true
		})
		pl.HomeDomains = slices.DeleteFunc(pl.HomeDomains, func(d string) bool {
			if addr, ok := places.Address(pl.Place.Base, pl.Place.Folders, d, ""); ok && addr == domainPathRaw(d, p.in.settings) {
				return false
			}
			p.unplaced = append(p.unplaced, d+" path")
			return true
		})
	}
}

// joinable finds a place a row at sp can join: the same base, the same
// settings, the same shape, and no folder yet for the row's domain.
func (p *placesPlanner) joinable(sp placeSplit, traits placeTraits, domain string) *plannedPlace {
	for _, pl := range p.planned {
		_, taken := pl.Place.Folders[domain]
		if pl.Place.Base == sp.base && pl.traits == traits && pl.repository == (sp.folder == "") && !taken {
			return pl
		}
	}
	return nil
}

// claimedElsewhere reports whether addr is, holds or lies inside an address a
// place other than self stands for. Two places may share a base, never an
// address.
func (p *placesPlanner) claimedElsewhere(addr string, self *plannedPlace) bool {
	meets := func(a string) bool { return repoLocationsOverlap(a, addr) }
	for _, pl := range p.planned {
		if pl != self && slices.ContainsFunc(pl.addresses(), meets) {
			return true
		}
	}
	for _, e := range p.in.existing {
		for _, folder := range e.Folders {
			if meets(places.Join(e.Base, folder)) {
				return true
			}
		}
	}
	return false
}

// addresses are the locations a place stands for: one per folder, plus each
// row's own, which covers a direct repository beside its target's folder.
func (pl *plannedPlace) addresses() []string {
	out := make([]string, 0, len(pl.Place.Folders)+len(pl.Rows))
	for _, folder := range pl.Place.Folders {
		out = append(out, places.Join(pl.Place.Base, folder))
	}
	for _, r := range pl.Rows {
		out = append(out, r.Repo)
	}
	return out
}

// add starts a place for a row at sp. A remote place stands off the premises,
// as every remote target and remote domain path counts before it has a place;
// a home place is on from the start, because a domain path cannot be
// switched off.
func (p *placesPlanner) add(name string, sp placeSplit, traits placeTraits, home bool) *plannedPlace {
	pl := &plannedPlace{traits: traits, repository: sp.folder == ""}
	pl.Place = store.Place{
		Name:                 p.uniqueName(name),
		Provider:             p.provider(sp.kind, sp.base, traits.credsRef),
		Kind:                 string(sp.kind),
		Base:                 sp.base,
		Folders:              map[string]string{},
		CredsRef:             traits.credsRef,
		OffPremises:          sp.kind != places.KindLocal,
		StorageClass:         traits.storageClass,
		Immutable:            traits.immutable,
		RetentionKeepLast:    traits.keepLast,
		RetentionKeepDaily:   traits.keepDaily,
		RetentionKeepWeekly:  traits.keepWeekly,
		RetentionKeepMonthly: traits.keepMonthly,
		LimitUpload:          traits.limitUpload,
		LimitDownload:        traits.limitDownload,
		GrowthBudgetGB:       traits.growthBudgetGB,
		Enabled:              home,
	}
	p.planned = append(p.planned, pl)
	return pl
}

// rename gives a place the name want, or want with a number, and frees the
// name it had.
func (p *placesPlanner) rename(pl *plannedPlace, want string) {
	delete(p.names, strings.ToLower(pl.Place.Name))
	pl.Place.Name = p.uniqueName(want)
	pl.serverNamed = false
}

// uniqueName hands out want, or want with the first free number after it,
// case-insensitively, so no two places differ only in case.
func (p *placesPlanner) uniqueName(want string) string {
	want = strings.TrimSpace(want)
	name := want
	for n := 2; p.names[strings.ToLower(name)]; n++ {
		name = fmt.Sprintf("%s %d", want, n)
	}
	p.names[strings.ToLower(name)] = true
	return name
}

// provider picks the catalog entry a migrated place shows: another BombVault
// for an accepted mesh offer, otherwise what its address looks like (a share
// under remotes/, a folder on this server, a known cloud), as long as that
// provider speaks the place's kind. A wrong guess changes the mark and the
// form, never the address.
func (p *placesPlanner) provider(kind places.Kind, base, credsRef string) string {
	if kind == places.KindREST && p.meshCreds(credsRef) {
		return "bombvault"
	}
	id := places.DetectProvider(base)
	if found, ok := places.ProviderByID(id); ok && found.Kind == kind {
		return id
	}
	return fallbackProviders[kind]
}

// meshCreds reports whether a credential set came with an accepted mesh
// offer, which names it after the peer (mesh.go).
func (p *placesPlanner) meshCreds(ref string) bool {
	set, ok := p.in.credSets[ref]
	return ok && strings.HasPrefix(set.Name, "mesh: ")
}

// storeName names a remote home place after the host or remote it lives on,
// the part of a domain path a person recognises. locationParts leaves out any
// credentials the address carries.
func storeName(base string) string {
	where, _ := locationParts(base)
	if _, name, _ := strings.Cut(where, ":"); name != "" {
		return name
	}
	return base
}

// MigrateToPlaces moves a database from before storage places onto places,
// once. It runs at start rather than as a SQL migration because telling a mesh
// target or a rest-server user apart needs the decrypted credential sets. A
// run that fails writes no place, and the next start tries again.
//
// Stray targets leave sort order 0 first, as at every start, so the places do
// not depend on which of the two runs first at start, and an import that calls
// this gets the same.
func (s *Service) MigrateToPlaces() error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if settings.PlacesMigrated != 0 {
		return nil
	}
	if _, err := s.MoveTargetsOffPrimarySlot(); err != nil {
		return fmt.Errorf("move targets off sort order 0: %w", err)
	}
	in, err := s.placesMigrationInput(settings)
	if err != nil {
		return err
	}
	plan := planPlaces(in)
	if err := s.store.ApplyPlacesMigration(plan.places); err != nil {
		return fmt.Errorf("write the places: %w", err)
	}
	log.Printf("api: %s", movedOntoPlacesLine(len(plan.places), len(plan.unplaced)))
	return nil
}

func movedOntoPlacesLine(places, unplaced int) string {
	onto := fmt.Sprintf("%d places", places)
	if places == 1 {
		onto = "1 place"
	}
	left := fmt.Sprintf("%d rows or domain paths stay", unplaced)
	switch unplaced {
	case 0:
		left = "no row or domain path stays"
	case 1:
		left = "1 row or domain path stays"
	}
	return "moved the storage settings onto " + onto + "; " + left + " without a place"
}

// placesMigrationInput reads what planPlaces needs, secrets included.
func (s *Service) placesMigrationInput(settings store.Settings) (placesMigrationInput, error) {
	in := placesMigrationInput{settings: settings, primaries: map[string]store.OffsiteTarget{}, credSets: map[string]CloudCredSet{}}
	var err error
	if in.targets, err = s.store.ListOffsiteTargets(); err != nil {
		return in, fmt.Errorf("read the off-site targets: %w", err)
	}
	if in.named, err = s.store.ListNamedRepos(); err != nil {
		return in, fmt.Errorf("read the repositories: %w", err)
	}
	for _, d := range places.Domains {
		row, ok, err := s.store.PrimaryRemoteTarget(d)
		if err != nil {
			return in, fmt.Errorf("read the primary row of %s: %w", d, err)
		}
		if ok {
			in.primaries[d] = row
		}
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return in, fmt.Errorf("read the credential sets: %w", err)
	}
	for _, set := range sets {
		in.credSets[set.ID] = set
	}
	if in.shared, err = s.decodeCloud(settings); err != nil {
		return in, fmt.Errorf("read the shared credentials: %w", err)
	}
	if in.existing, err = s.store.ListPlaces(); err != nil {
		return in, fmt.Errorf("read the places: %w", err)
	}
	if in.homes, err = s.store.DomainPlaces(); err != nil {
		return in, fmt.Errorf("read the home places: %w", err)
	}
	// A ZFS domain that is off, has no copy target and never backed up to its
	// path does not get a place of its own for that path. Switched on later,
	// it chooses one on its row.
	zfsUsed := settings.ZFSEnabled || slices.ContainsFunc(in.targets, func(t store.OffsiteTarget) bool { return t.Domain == zfsDomain })
	if !zfsUsed {
		if zfsUsed, err = s.store.DomainPathBackedUp(zfsDomain); err != nil {
			return in, fmt.Errorf("read the backups of the ZFS path: %w", err)
		}
	}
	in.idle = map[string]bool{zfsDomain: !zfsUsed}
	return in, nil
}

// PlaceSwitchedOnZFS gives a ZFS domain switched on after the move onto places
// the home the move skipped while it was off: the place whose base holds the
// path takes its last element as the zfs folder, or the path gets a place of
// its own.
func (s *Service) PlaceSwitchedOnZFS() error {
	s.placeEditMu.Lock()
	defer s.placeEditMu.Unlock()
	return s.placeSwitchedOnZFSLocked()
}

// placeSwitchedOnZFSLocked is PlaceSwitchedOnZFS for a caller that holds
// placeEditMu, as an import does.
func (s *Service) placeSwitchedOnZFSLocked() error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if settings.PlacesMigrated == 0 || !settings.ZFSEnabled {
		return nil
	}
	sp, ok := splitPlaceAddress(settings.ZFSPath)
	if !ok || sp.folder == "" {
		return nil
	}
	homes, err := s.store.DomainPlaces()
	if err != nil || homes[zfsDomain] != "" {
		return err
	}
	in, err := s.placesMigrationInput(settings)
	if err != nil {
		return err
	}
	primary := in.primaries[zfsDomain]
	p := &placesPlanner{in: in, names: map[string]bool{}}
	for _, e := range in.existing {
		if e.Kind != string(sp.kind) || e.Base != sp.base || (sp.kind != places.KindLocal && e.CredsRef != primary.CredsRef) {
			continue
		}
		folder, has := e.Folders[zfsDomain]
		if has && folder != sp.folder || !has && p.claimedElsewhere(settings.ZFSPath, nil) {
			continue
		}
		e.Folders[zfsDomain] = sp.folder
		_, err := s.writePlace(store.PlaceWrite{Place: e, HomeDomains: map[string]string{zfsDomain: e.ID}}, nil)
		return err
	}
	in.targets, in.named = nil, nil
	in.idle = map[string]bool{}
	for _, d := range places.Domains {
		in.idle[d] = d != zfsDomain
	}
	for _, m := range planPlaces(in).places {
		if slices.Contains(m.HomeDomains, zfsDomain) {
			_, err := s.writePlace(store.PlaceWrite{Place: m.Place, HomeDomains: map[string]string{zfsDomain: ""}}, nil)
			return err
		}
	}
	return nil
}
