package api

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

var errNestedLocation = errors.New("this location lies inside another repository or target, or contains one")

// repoLocationsOverlap reports whether two locations are the same place or one
// lies inside the other, path element by path element.
func repoLocationsOverlap(a, b string) bool {
	storeA, pathA := locationParts(a)
	storeB, pathB := locationParts(b)
	if storeA != storeB {
		return false
	}
	if len(pathA) > len(pathB) {
		pathA, pathB = pathB, pathA
	}
	return slices.Equal(pathA, pathB[:len(pathA)])
}

// locationParts splits a location into the store it lives in (scheme with host,
// bucket or remote, empty for a local path) and the path elements below it.
// Credentials in a URL do not name a different place.
func locationParts(loc string) (string, []string) {
	loc = strings.TrimSpace(loc)
	if !restic.IsRemoteRepo(loc) {
		return "", pathElements(filepath.ToSlash(filepath.Clean(loc)))
	}
	scheme, rest, _ := strings.Cut(loc, ":")
	switch scheme {
	case "rest", "s3":
		host, path, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(rest, "https://"), "http://"), "/")
		return scheme + ":" + hostOnly(host), pathElements(path)
	case "sftp":
		if url, ok := strings.CutPrefix(rest, "//"); ok {
			host, path, _ := strings.Cut(url, "/")
			return scheme + ":" + hostOnly(host), pathElements(path)
		}
		host, path, _ := strings.Cut(rest, ":")
		return scheme + ":" + hostOnly(host), pathElements(path)
	}
	name, path, _ := strings.Cut(rest, ":")
	return scheme + ":" + name, pathElements(path)
}

func hostOnly(host string) string {
	if i := strings.LastIndexByte(host, '@'); i >= 0 {
		host = host[i+1:]
	}
	return strings.ToLower(host)
}

func pathElements(p string) []string {
	var out []string
	for _, e := range strings.Split(p, "/") {
		if e != "" && e != "." {
			out = append(out, e)
		}
	}
	return out
}

// locationSelf is what the location being checked already stands for, so
// locationClash does not hold a place against itself: rows by id, and the
// domain whose own repository or off-site field it is.
type locationSelf struct {
	ids   []string
	own   string
	field string
}

// locationClash refuses a location that is, holds or lies inside a domain
// repository, an off-site field, an off-site target or a named repository.
// Two rows over one place, or one inside the other, would give every question
// about that place two answers.
func (s *Service) locationClash(settings store.Settings, loc string, self locationSelf) error {
	for _, d := range offsiteConfigDomains {
		// Two domains may share one repository; one inside the other may not.
		if own, err := s.repoFor(settings, d, "local"); err == nil && d != self.own && repoLocationsOverlap(own, loc) &&
			(self.own == "" || !sameRepoLocation(own, loc)) {
			return fmt.Errorf("%w: the %s domain's own repository", errNestedLocation, d)
		}
		if off := offsiteRepoFromSettings(d, settings); off != "" && d != self.field {
			if offLoc, err := s.resolveRepo(off); err == nil && repoLocationsOverlap(offLoc, loc) {
				return fmt.Errorf("%w: the %s domain's off-site destination", errNestedLocation, d)
			}
		}
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		return fmt.Errorf("read the off-site targets to check this location: %w", err)
	}
	for _, t := range targets {
		if slices.Contains(self.ids, t.ID) {
			continue
		}
		if tLoc, rErr := s.resolveRepo(t.Repo); rErr == nil && repoLocationsOverlap(tLoc, loc) {
			return fmt.Errorf("%w: the off-site destination %q", errNestedLocation, t.Name)
		}
	}
	named, err := s.store.ListNamedRepos()
	if err != nil {
		return fmt.Errorf("read the repositories to check this location: %w", err)
	}
	for _, r := range named {
		if slices.Contains(self.ids, r.ID) {
			continue
		}
		if other, rErr := s.resolveRepo(r.Repo); rErr == nil && repoLocationsOverlap(other, loc) {
			return fmt.Errorf("%w: another repository", errNestedLocation)
		}
	}
	return nil
}
