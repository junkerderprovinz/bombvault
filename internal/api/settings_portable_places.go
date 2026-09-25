package api

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// placeExport is a storage place in the settings file: every column but the
// secrets, which travel in the credentials block under the set CredsRef names.
type placeExport struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Provider             string            `json:"provider"`
	Kind                 string            `json:"kind"`
	Base                 string            `json:"base"`
	Folders              map[string]string `json:"folders"`
	CredsRef             string            `json:"credsRef"`
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
	CreatedAt            int64             `json:"createdAt"`
	UpdatedAt            int64             `json:"updatedAt"`
}

func placesToExport(rows []store.Place) []placeExport {
	out := make([]placeExport, 0, len(rows))
	for _, p := range rows {
		out = append(out, placeExport{
			ID:                   p.ID,
			Name:                 p.Name,
			Provider:             p.Provider,
			Kind:                 p.Kind,
			Base:                 p.Base,
			Folders:              p.Folders,
			CredsRef:             p.CredsRef,
			OffPremises:          p.OffPremises,
			StorageClass:         p.StorageClass,
			Immutable:            p.Immutable,
			RetentionKeepLast:    p.RetentionKeepLast,
			RetentionKeepDaily:   p.RetentionKeepDaily,
			RetentionKeepWeekly:  p.RetentionKeepWeekly,
			RetentionKeepMonthly: p.RetentionKeepMonthly,
			LimitUpload:          p.LimitUpload,
			LimitDownload:        p.LimitDownload,
			GrowthBudgetGB:       p.GrowthBudgetGB,
			Enabled:              p.Enabled,
			SortOrder:            p.SortOrder,
			CreatedAt:            p.CreatedAt,
			UpdatedAt:            p.UpdatedAt,
		})
	}
	return out
}

// toStorePlace trims the text fields, floors the numbers at zero and
// upper-cases the storage class, the way toStoreTarget treats a row.
func (p placeExport) toStorePlace() store.Place {
	folders := make(map[string]string, len(p.Folders))
	for domain, folder := range p.Folders {
		folders[domain] = strings.TrimSpace(folder)
	}
	return store.Place{
		ID:                   strings.TrimSpace(p.ID),
		Name:                 strings.TrimSpace(p.Name),
		Provider:             strings.TrimSpace(p.Provider),
		Kind:                 strings.TrimSpace(p.Kind),
		Base:                 strings.TrimSpace(p.Base),
		Folders:              folders,
		CredsRef:             strings.TrimSpace(p.CredsRef),
		OffPremises:          p.OffPremises,
		StorageClass:         strings.ToUpper(strings.TrimSpace(p.StorageClass)),
		Immutable:            p.Immutable,
		RetentionKeepLast:    max(0, p.RetentionKeepLast),
		RetentionKeepDaily:   max(0, p.RetentionKeepDaily),
		RetentionKeepWeekly:  max(0, p.RetentionKeepWeekly),
		RetentionKeepMonthly: max(0, p.RetentionKeepMonthly),
		LimitUpload:          max(0, p.LimitUpload),
		LimitDownload:        max(0, p.LimitDownload),
		GrowthBudgetGB:       max(0, p.GrowthBudgetGB),
		Enabled:              p.Enabled,
		SortOrder:            p.SortOrder,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
	}
}

// rejectInvalidPlaces checks the file's storage places the way the place form
// checks one, and that every home domain and placed row names a place of the
// file with a folder for it. It returns a user-facing sentence, or "".
func rejectInvalidPlaces(exp settingsExport, mountRoot string) string {
	byID := make(map[string]store.Place, len(exp.Places))
	names := make(map[string]bool, len(exp.Places))
	for i, fp := range exp.Places {
		p := fp.toStorePlace()
		_, dupID := byID[p.ID]
		switch {
		case p.ID == "":
			return fmt.Sprintf("storage place #%d: needs an id", i+1)
		case p.Name == "":
			return fmt.Sprintf("storage place #%d: needs a name", i+1)
		case dupID:
			return fmt.Sprintf("storage place #%d (%s): its id is used twice", i+1, p.Name)
		case names[p.Name]:
			return fmt.Sprintf("storage place #%d: the name %q is used twice", i+1, p.Name)
		}
		if msg := rejectInvalidPlace(p, mountRoot); msg != "" {
			return fmt.Sprintf("storage place #%d (%s): %s", i+1, p.Name, msg)
		}
		byID[p.ID] = p
		names[p.Name] = true
	}
	for _, domain := range slices.Sorted(maps.Keys(exp.StorageDomainPlaces)) {
		if !slices.Contains(places.Domains, domain) {
			return fmt.Sprintf("storageDomainPlaces names an unknown domain %q", domain)
		}
		p, ok := byID[strings.TrimSpace(exp.StorageDomainPlaces[domain])]
		switch {
		case !ok:
			return fmt.Sprintf("the %s domain is stored at a place the file does not carry", domain)
		case !p.Enabled:
			return fmt.Sprintf("the %s domain is stored at %s, which is switched off", domain, p.Name)
		}
		if _, ok := places.Address(p.Base, p.Folders, domain, ""); !ok {
			return fmt.Sprintf("the %s domain is stored at %s, which has no folder for it", domain, p.Name)
		}
	}
	for i, tv := range exp.OffsiteTargets {
		if msg := rejectInvalidPlaceLink(byID, tv); msg != "" {
			return fmt.Sprintf("off-site target #%d: %s", i+1, msg)
		}
	}
	for i, tv := range exp.NamedRepos {
		if msg := rejectInvalidPlaceLink(byID, tv); msg != "" {
			return fmt.Sprintf("repository #%d: %s", i+1, msg)
		}
	}
	return ""
}

// rejectInvalidPlace applies the refusals that need nothing but the place.
func rejectInvalidPlace(p store.Place, mountRoot string) string {
	provider, ok := places.ProviderByID(p.Provider)
	if !ok {
		return fmt.Sprintf("unknown provider %q", p.Provider)
	}
	if string(provider.Kind) != p.Kind {
		return fmt.Sprintf("kind %q does not match provider %q", p.Kind, p.Provider)
	}
	if p.Base == "" {
		return "needs a base address"
	}
	for _, domain := range slices.Sorted(maps.Keys(p.Folders)) {
		if !slices.Contains(places.Domains, domain) {
			return fmt.Sprintf("has a folder for an unknown domain %q", domain)
		}
		if !validPlaceFolder(p.Folders[domain]) {
			return fmt.Sprintf("the folder for %s leaves the base", domain)
		}
	}
	if p.StorageClass != "" && !restic.StorageClassAllowed(p.StorageClass) {
		return "unsupported storage class " + p.StorageClass + " (allowed: " + strings.Join(restic.AllowedStorageClasses, ", ") + ")"
	}
	if places.Kind(p.Kind) != places.KindLocal {
		if !restic.IsRemoteRepo(p.Base) {
			return "its base is not a remote location"
		}
		return ""
	}
	// The same refusals a domain path meets on save: the import writes these
	// addresses into the settings row after validateExport has passed it.
	for _, loc := range placeLocations(p) {
		switch {
		case restic.IsRemoteRepo(loc):
			return "a local place needs a path under the mount root, not a remote location"
		case restic.LooksLikeUnprefixedRemote(loc):
			return "its base looks like a remote without a restic prefix; an rclone remote needs rclone: in front"
		}
		if _, err := paths.Resolve(mountRoot, loc); err != nil {
			return "a local place needs relative subpaths under the mount root"
		}
	}
	return ""
}

// rejectInvalidPlaceLink checks that a row on a place names a place of the
// file with a folder for the row's domain, and that a target sits in the
// folder of its own domain, as every writer of a placed row leaves it.
func rejectInvalidPlaceLink(byID map[string]store.Place, tv offsiteTargetView) string {
	placeID := strings.TrimSpace(tv.PlaceID)
	if placeID == "" {
		return ""
	}
	p, ok := byID[placeID]
	switch {
	case !ok:
		return "it sits on a storage place the file does not carry"
	case strings.TrimSpace(tv.ID) == "":
		return "a row on a storage place needs its id"
	case strings.ContainsAny(tv.PlaceSuffix, `/\`):
		return "its place suffix must not hold a path"
	}
	if _, ok := places.Address(p.Base, p.Folders, tv.PlaceDomain, tv.PlaceSuffix); !ok {
		return fmt.Sprintf("its storage place %s has no folder for %s", p.Name, tv.PlaceDomain)
	}
	if tv.Domain != "" && tv.PlaceDomain != tv.Domain {
		return fmt.Sprintf("its storage place %s holds it for %q, not for its own domain %s", p.Name, tv.PlaceDomain, tv.Domain)
	}
	return ""
}

// validPlaceFolder refuses a folder that starts at the root or climbs out of
// its base, which Join would otherwise put anywhere.
func validPlaceFolder(folder string) bool {
	return !strings.HasPrefix(folder, "/") && !slices.Contains(strings.Split(folder, "/"), "..")
}

// placeLocations lists the base of a place and the address of each folder.
func placeLocations(p store.Place) []string {
	out := []string{p.Base}
	for _, domain := range slices.Sorted(maps.Keys(p.Folders)) {
		out = append(out, places.Join(p.Base, p.Folders[domain]))
	}
	return out
}
