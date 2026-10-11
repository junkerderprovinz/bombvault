package api

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// place is where a repository keeps its data, named so that the spellings of
// one place BombVault can tell apart compare equal: a symlink and its target,
// an Unraid share under user, user0, a disk or the cache, and two rclone
// remotes over the same backend.
type place struct {
	// store is the backend and account, empty for a local directory.
	store string
	elems []string
	// dir is a local place's directory with its symlinks resolved.
	dir string
}

func (p place) same(o place) bool {
	if p.store == o.store && slices.Equal(p.elems, o.elems) {
		return true
	}
	return p.store == "" && o.store == "" && sameDir(p.dir, o.dir)
}

// overlaps reports whether the two places are one or one lies inside the other.
func (p place) overlaps(o place) bool {
	if p.same(o) {
		return true
	}
	if p.store != o.store {
		return false
	}
	a, b := p.elems, o.elems
	if len(a) > len(b) {
		a, b = b, a
	}
	return slices.Equal(a, b[:len(a)])
}

func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	return err == nil && os.SameFile(ai, bi)
}

// places names the place of resolved repository locations.
type places struct {
	root []string
	// remotes holds the options of each rclone remote by its lowercased name.
	remotes map[string]map[string]string
}

// placesNow reads what naming a place takes from the settings as they stand. A
// config that cannot be read leaves every rclone place unnamed.
func (s *Service) placesNow() places {
	p := places{root: pathElements(filepath.ToSlash(realPath(filepath.Clean(s.cfg.HostMountRoot))))}
	settings, err := s.store.GetSettings()
	if err != nil {
		return p
	}
	if conf, err := s.decodeRcloneConf(settings); err == nil {
		p.remotes = rcloneOptions(conf)
	}
	return p
}

func rcloneOptions(conf string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, block := range splitRcloneSections(conf) {
		if block.name == "" {
			continue
		}
		opts := map[string]string{}
		for _, line := range strings.Split(block.text, "\n")[1:] {
			line = strings.TrimSpace(line)
			if line == "" || line[0] == '#' || line[0] == ';' {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				opts[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
			}
		}
		out[strings.ToLower(block.name)] = opts
	}
	return out
}

// of names the place of a resolved location. ok is false when it cannot be
// named, and such a place may be any other.
func (p places) of(loc string) (place, bool) {
	return p.named(strings.TrimSpace(loc), 0)
}

func (p places) named(loc string, depth int) (place, bool) {
	if !restic.IsRemoteRepo(loc) {
		return p.local(loc), true
	}
	rest, ok := strings.CutPrefix(loc, "rclone:")
	if !ok {
		store, elems := locationParts(loc)
		return place{store: store, elems: elems}, true
	}
	name, path, _ := strings.Cut(rest, ":")
	opts, known := p.remotes[strings.ToLower(name)]
	if !known || depth > 8 {
		return place{}, false
	}
	typ := strings.ToLower(opts["type"])
	switch {
	case typ == "alias":
		return p.named("rclone:"+joinRemote(opts["remote"], path), depth+1)
	case typ == "local":
		return p.local(path), true
	case opts["upstreams"] != "":
		return place{}, false
	case opts["remote"] != "":
		inner, ok := p.named("rclone:"+opts["remote"], depth+1)
		if !ok {
			return place{}, false
		}
		return place{store: typ + "(" + inner.store + "/" + strings.Join(inner.elems, "/") + ")", elems: pathElements(path)}, true
	}
	return place{store: "rclone:" + typ + "{" + accountOf(opts) + "}", elems: pathElements(path)}, true
}

func joinRemote(remote, path string) string {
	if path == "" {
		return remote
	}
	if strings.HasSuffix(remote, ":") || strings.HasSuffix(remote, "/") {
		return remote + path
	}
	return remote + "/" + path
}

// accountKeys are the rclone options that say which server and account a
// remote reaches. Logins and tokens are left out, so two remotes over one
// account under different keys name one place.
var accountKeys = []string{"host", "port", "user", "url", "endpoint", "region", "account", "root_folder_id", "team_drive", "drive_id"}

func accountOf(opts map[string]string) string {
	var b strings.Builder
	for _, k := range accountKeys {
		if v := strings.ToLower(strings.TrimRight(opts[k], "/")); v != "" {
			b.WriteString(k + "=" + v + ";")
		}
	}
	return b.String()
}

// unraidShare matches the folders under /mnt that hold one Unraid share each:
// the user share with and without the cache, the array disks and the cache.
var unraidShare = regexp.MustCompile(`^(user|user0|disk[0-9]+|cache)$`)

func (p places) local(loc string) place {
	dir := realPath(filepath.Clean(loc))
	elems := pathElements(filepath.ToSlash(dir))
	at := -1
	if len(p.root) > 0 && len(elems) > len(p.root) && slices.Equal(elems[:len(p.root)], p.root) && !unraidShare.MatchString(p.root[len(p.root)-1]) {
		at = len(p.root)
	} else if i := slices.Index(elems, "mnt"); i >= 0 && i+1 < len(elems) {
		at = i + 1
	}
	if at >= 0 && unraidShare.MatchString(elems[at]) {
		elems = slices.Clone(elems)
		elems[at] = "user"
	}
	return place{elems: elems, dir: dir}
}

// realPath resolves the symlinks of the longest part of path that exists and
// keeps the rest as written.
func realPath(path string) string {
	var tail []string
	for dir := path; ; dir = filepath.Dir(dir) {
		real, err := filepath.EvalSymlinks(dir)
		if err == nil {
			return filepath.Join(append([]string{real}, tail...)...)
		}
		if !errors.Is(err, fs.ErrNotExist) || filepath.Dir(dir) == dir {
			return path
		}
		tail = append([]string{filepath.Base(dir)}, tail...)
	}
}
