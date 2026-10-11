package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

var errOwnData = errors.New("this would write into BombVault's own data folder or a folder that holds it; choose another folder")

// ownDataGuard is the engine every restore of the service runs through. It
// refuses a restore before restic writes anything when the folder it writes
// into is BombVault's own data folder, lies inside it or holds it, however
// Host Data or a link reaches that folder.
type ownDataGuard struct {
	ResticEngine
	s *Service
}

func (g ownDataGuard) RestorePath(ctx context.Context, repo, snapshotID, p string, mode restic.Mode) error {
	if err := g.s.refuseOwnData(p); err != nil {
		return err
	}
	return g.ResticEngine.RestorePath(ctx, repo, snapshotID, p, mode)
}

func (g ownDataGuard) RestoreAll(ctx context.Context, repo, snapshotID, target string, mode restic.Mode, excludes ...string) error {
	if err := g.s.refuseOwnData(target); err != nil {
		return err
	}
	return g.ResticEngine.RestoreAll(ctx, repo, snapshotID, target, mode, excludes...)
}

func (g ownDataGuard) RestoreVerify(ctx context.Context, repo, snapshotID string, files []string, target string, mode restic.Mode) error {
	if err := g.s.refuseOwnData(target); err != nil {
		return err
	}
	return g.ResticEngine.RestoreVerify(ctx, repo, snapshotID, files, target, mode)
}

// RestoreInclude checks where the included path lands below target, which
// for the target "/" is the path itself.
func (g ownDataGuard) RestoreInclude(ctx context.Context, repo, snapshotID, includePath, target string, mode restic.Mode) error {
	lands := includePath
	if target != "/" {
		lands = path.Join(slashClean(target), includePath)
	}
	if err := g.s.refuseOwnData(lands); err != nil {
		return err
	}
	return g.ResticEngine.RestoreInclude(ctx, repo, snapshotID, includePath, target, mode)
}

func (g ownDataGuard) RestoreSubtreeTo(ctx context.Context, repo, snapshotID, subtreePath, target string, mode restic.Mode) error {
	if err := g.s.refuseOwnData(target); err != nil {
		return err
	}
	return g.ResticEngine.RestoreSubtreeTo(ctx, repo, snapshotID, subtreePath, target, mode)
}

func (g ownDataGuard) RestoreSubtreeInclude(ctx context.Context, repo, snapshotID, subtreePath, includePath, target string, mode restic.Mode) error {
	if err := g.s.refuseOwnData(path.Join(slashClean(target), includePath)); err != nil {
		return err
	}
	return g.ResticEngine.RestoreSubtreeInclude(ctx, repo, snapshotID, subtreePath, includePath, target, mode)
}

// refuseOwnData returns errOwnData for a folder that is the data folder, lies
// inside it or holds it, compared after links are followed. A folder inside
// the data folder that does not come through Host Data is one of the service's
// own scratch folders and passes. With Host Data inside the data folder every
// folder would lie inside it, so there only the folders that hold it are
// refused.
func (s *Service) refuseOwnData(dest string) error {
	if s.cfg.DataDir == "" || dest == "" {
		return nil
	}
	data := slashClean(s.cfg.DataDir)
	host := slashClean(s.cfg.HostMountRoot)
	named := slashClean(dest)
	if isStrictSubpath(data, named) && (s.cfg.HostMountRoot == "" || !isStrictSubpath(host, named)) {
		return nil
	}
	inside := s.cfg.HostMountRoot == "" || !(host == data || isStrictSubpath(data, host))
	held := []string{data, resolvedPath(data)}
	if f, err := os.Open(mountinfoPath); err == nil {
		held = append(held, dataAliases(f, resolvedPath(data), host, listDirs)...)
		_ = f.Close()
	}
	for _, d := range []string{named, resolvedPath(named)} {
		for _, h := range held {
			if d == h || isStrictSubpath(d, h) || (inside && isStrictSubpath(h, d)) {
				return errOwnData
			}
		}
	}
	return nil
}

func slashClean(p string) string { return path.Clean(filepath.ToSlash(p)) }

// resolvedPath follows the links in p as far as p exists and keeps the rest
// as named, since a restore creates what is missing below the deepest folder
// that is there.
func resolvedPath(p string) string {
	rest := ""
	for cur := p; ; {
		if real, err := filepath.EvalSymlinks(filepath.FromSlash(cur)); err == nil {
			return slashClean(path.Join(filepath.ToSlash(real), rest))
		}
		parent := path.Dir(cur)
		if parent == cur {
			return p
		}
		rest = path.Join(path.Base(cur), rest)
		cur = parent
	}
}

func listDirs(dir string) []string {
	entries, err := os.ReadDir(filepath.FromSlash(dir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// dataAliases lists the folders below the Host Data root that show the data
// folder: the mounts of the same filesystem that reach it, read from the
// mountinfo records in r. On Unraid, which serves its user shares through
// shfs, a folder of a share is also the same folder on every pool and disk,
// so each one directly below the root counts with the same path below it.
func dataAliases(r io.Reader, data, host string, dirs func(string) []string) []string {
	type mount struct{ dev, root, point string }
	var mounts []mount
	var own mount
	shares := false
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		left, right, _ := strings.Cut(sc.Text(), " - ")
		fields := strings.Fields(left)
		if len(fields) < 5 {
			continue
		}
		shares = shares || strings.HasPrefix(right, "fuse.shfs ")
		m := mount{dev: fields[2], root: path.Clean(unescapeOctal(fields[3])), point: path.Clean(unescapeOctal(fields[4]))}
		mounts = append(mounts, m)
		if (m.point == data || isStrictSubpath(m.point, data)) && len(m.point) >= len(own.point) {
			own = m
		}
	}
	if own.dev == "" {
		return nil
	}
	inFS := path.Join(own.root, strings.TrimPrefix(data, own.point))
	var out []string
	for _, m := range mounts {
		if m.dev != own.dev || !(m.root == inFS || isStrictSubpath(m.root, inFS)) || !isStrictSubpath(host, m.point) {
			continue
		}
		alias := path.Join(m.point, strings.TrimPrefix(inFS, m.root))
		out = append(out, alias)
		_, below, _ := strings.Cut(strings.TrimPrefix(alias, host+"/"), "/")
		if !shares || below == "" {
			continue
		}
		for _, d := range dirs(host) {
			out = append(out, path.Join(host, d, below))
		}
	}
	return out
}
