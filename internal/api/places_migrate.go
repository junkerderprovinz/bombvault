package api

import (
	"fmt"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

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

// placesMigrationInput is everything the plan reads, gathered first so that
// the plan itself touches neither the store nor the key.
type placesMigrationInput struct {
	settings  store.Settings
	primaries map[string]store.OffsiteTarget // domain -> its remote-primary row
	credSets  map[string]CloudCredSet        // by id, secrets included
	existing  []store.Place                  // names and addresses the plan must not take
	homes     map[string]string              // domains that have a home place already
}

// plannedPlace is one place as the plan builds it up.
type plannedPlace struct {
	store.MigratedPlace
	traits     placeTraits
	home       bool // made for domain paths, which take no default folders
	repository bool // one repository by itself, so nothing goes under it
}

// placesPlan is what the migration writes and what it leaves without a place.
type placesPlan struct {
	places   []store.MigratedPlace
	unplaced []string // row ids, and "<domain> path" for a domain path
}

type placesPlanner struct {
	in       placesMigrationInput
	planned  []*plannedPlace
	names    map[string]bool // names taken, lower case
	unplaced []string
}

// planPlaces works out the places a database from before storage places turns
// into. A row goes onto a place only in a form the place spells back byte for
// byte, and what already has a place stays as it is.
func planPlaces(in placesMigrationInput) placesPlan {
	p := &placesPlanner{in: in, names: map[string]bool{}}
	for _, e := range in.existing {
		p.names[strings.ToLower(e.Name)] = true
	}
	p.planDomainPaths()
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
		if p.in.homes[d] != "" {
			continue
		}
		path := domainPathRaw(d, p.in.settings)
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
		}
		pl.Place.Folders[d] = sp.folder
		pl.HomeDomains = append(pl.HomeDomains, d)
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
	pl := &plannedPlace{traits: traits, home: home, repository: sp.folder == ""}
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
