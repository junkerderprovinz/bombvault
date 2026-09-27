package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// FlashPlugin is one Unraid plugin found in a flash backup: its .plg, its
// folder under config/plugins and the package files the .plg downloads to the
// flash, as far as the backup holds them.
type FlashPlugin struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Size     int64    `json:"size"`
	Packages []string `json:"packages"`
}

const flashPluginsDir = "/config/plugins"

var errFlashPluginName = errors.New("invalid plugin name")

// flashSnapshotRoot is the path a flash snapshot recorded for /boot. It is the
// configured flash mount at the time of the backup.
func flashSnapshotRoot(snaps []restic.Snapshot, id, fallback string) string {
	for _, sn := range snaps {
		if sn.ID == id && len(sn.Paths) == 1 {
			return path.Clean(sn.Paths[0])
		}
	}
	return path.Clean(fallback)
}

type flashPluginScan struct {
	repo    string
	mode    restic.Mode
	id      string
	root    string
	plugins []FlashPlugin
}

// FlashPlugins lists the plugins of one flash backup. The .plg files are small,
// so they are restored into a scratch folder once and read from there.
func (s *Service) FlashPlugins(ctx context.Context, snapshotID, source string) ([]FlashPlugin, error) {
	scan, err := s.scanFlashPlugins(ctx, snapshotID, source)
	if err != nil {
		return nil, err
	}
	return scan.plugins, nil
}

func (s *Service) scanFlashPlugins(ctx context.Context, snapshotID, source string) (flashPluginScan, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return flashPluginScan{}, fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "flash", source)
	if err != nil {
		return flashPluginScan{}, err
	}
	mode := s.repoModeFor(settings, "flash", source, repo)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return flashPluginScan{}, err
	}
	id, err := resolveFlashSnapshot(snaps, snapshotID)
	if err != nil {
		return flashPluginScan{}, err
	}
	scan := flashPluginScan{repo: repo, mode: mode, id: id, root: flashSnapshotRoot(snaps, id, s.cfg.FlashDir)}
	entries, err := s.engine.Ls(ctx, repo, id, mode)
	if err != nil {
		return flashPluginScan{}, err
	}
	pluginsDir := scan.root + flashPluginsDir
	sizes := make(map[string]int64, len(entries))
	var names []string
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		sizes[e.Path] = e.Size
		if rest, ok := strings.CutPrefix(e.Path, pluginsDir+"/"); ok && !strings.Contains(rest, "/") && strings.HasSuffix(rest, ".plg") {
			names = append(names, strings.TrimSuffix(rest, ".plg"))
		}
	}
	if len(names) == 0 {
		scan.plugins = []FlashPlugin{}
		return scan, nil
	}
	sort.Strings(names)

	tmp, err := os.MkdirTemp(s.cfg.DataDir, "flash-plugins-")
	if err != nil {
		return flashPluginScan{}, fmt.Errorf("scratch folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := s.engine.RestoreInclude(ctx, repo, id, escapeGlobLiteral(pluginsDir+"/")+"*.plg", tmp, mode); err != nil {
		return flashPluginScan{}, fmt.Errorf("read the plugin files: %w", err)
	}

	for _, name := range names {
		p := FlashPlugin{Name: name, Packages: []string{}}
		plgPath := pluginsDir + "/" + name + ".plg"
		p.Size = sizes[plgPath]
		folder := pluginsDir + "/" + name + "/"
		for f, n := range sizes {
			if strings.HasPrefix(f, folder) {
				p.Size += n
			}
		}
		raw, rErr := os.ReadFile(filepath.Join(tmp, filepath.FromSlash(plgPath))) //nolint:gosec // G304: a file restic just restored into our own scratch folder
		if rErr != nil {
			log.Printf("api: flash plugins: reading %s.plg failed: %v", name, rErr) //nolint:gosec // G706: the name is a file name from the snapshot, logged as is
		}
		var files []string
		p.Version, files = parsePlg(string(raw))
		for _, f := range files {
			rel, ok := strings.CutPrefix(f, "/boot/")
			if !ok {
				continue
			}
			inSnap := scan.root + "/" + path.Clean(rel)
			n, held := sizes[inSnap]
			if !held || strings.HasPrefix(inSnap, folder) || inSnap == plgPath {
				continue
			}
			p.Packages = append(p.Packages, "/boot/"+path.Clean(rel))
			p.Size += n
		}
		scan.plugins = append(scan.plugins, p)
	}
	return scan, nil
}

var (
	plgEntityRe  = regexp.MustCompile(`<!ENTITY\s+([A-Za-z0-9_.-]+)\s+(?:"([^"]*)"|'([^']*)')\s*>`)
	plgVersionRe = regexp.MustCompile(`(?is)<PLUGIN\b[^>]*?\bversion\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	plgFileRe    = regexp.MustCompile(`(?is)<FILE\b[^>]*?\bName\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// parsePlg reads the version and the FILE targets of a plugin file, with the
// DOCTYPE entities it defines put in. A .plg carries shell scripts in plain
// text, so it is read with patterns rather than an XML parser that would stop
// at the first bare ampersand.
func parsePlg(text string) (version string, files []string) {
	entities := map[string]string{}
	for _, m := range plgEntityRe.FindAllStringSubmatch(text, -1) {
		entities[m[1]] = m[2] + m[3]
	}
	expand := func(v string) string {
		for range 5 {
			if !strings.Contains(v, "&") {
				break
			}
			for k, val := range entities {
				v = strings.ReplaceAll(v, "&"+k+";", val)
			}
		}
		return v
	}
	if m := plgVersionRe.FindStringSubmatch(text); m != nil {
		version = expand(m[1] + m[2])
	}
	for _, m := range plgFileRe.FindAllStringSubmatch(text, -1) {
		files = append(files, expand(m[1]+m[2]))
	}
	return version, files
}

// validFlashPluginName keeps a plugin name to one path segment, so it can only
// ever name something inside config/plugins.
func validFlashPluginName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`) && !strings.ContainsRune(name, 0)
}

// StartRestoreFlashPlugin puts one plugin from a flash backup back into the
// live flash: its .plg, its folder and its package files. Other files on the
// flash are left alone. It runs detached and records a restore run.
func (s *Service) StartRestoreFlashPlugin(ctx context.Context, snapshotID, source, name string, confirm bool) (bool, error) {
	if !confirm {
		return false, errors.New("restoring a plugin writes into the flash and needs a confirmation")
	}
	if !validFlashPluginName(name) {
		return false, errFlashPluginName
	}
	if _, err := os.Stat(s.cfg.FlashDir); err != nil {
		return false, fmt.Errorf("the Unraid flash is not mounted at %s", s.cfg.FlashDir)
	}
	scan, err := s.scanFlashPlugins(ctx, snapshotID, source)
	if err != nil {
		return false, err
	}
	var plugin *FlashPlugin
	for i := range scan.plugins {
		if scan.plugins[i].Name == name {
			plugin = &scan.plugins[i]
		}
	}
	if plugin == nil {
		return false, fmt.Errorf("the plugin %q is not in this flash backup", name)
	}
	if !s.batchActive.CompareAndSwap(false, true) {
		return false, nil
	}
	unlock, ok := s.tryLockDomainFor("flash", "restore-plugin")
	if !ok {
		s.batchActive.Store(false)
		return false, errDomainBusy
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		var runID string
		defer s.recoverOperation("restore flash plugin: "+name, nil, func(msg string) {
			s.finishRestoreRun(runID, "", errors.New(msg))
		})
		defer s.batchActive.Store(false)
		defer unlock()
		runID, _ = s.startRun(bctx, store.FlashTargetID, "restore")
		pctx, startedAt := s.progBegin(bctx, "flash", "restore")
		rerr := s.restoreFlashPlugin(pctx, scan, *plugin)
		s.progEnd("flash", "restore", rerr == nil, startedAt)
		s.finishRestoreRun(runID, scan.id, rerr)
	}()
	return true, nil
}

func (s *Service) restoreFlashPlugin(ctx context.Context, scan flashPluginScan, p FlashPlugin) error {
	tmp, err := os.MkdirTemp(s.cfg.DataDir, "flash-plugin-")
	if err != nil {
		return fmt.Errorf("scratch folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	pluginsDir := scan.root + flashPluginsDir
	includes := []string{pluginsDir + "/" + p.Name + ".plg", pluginsDir + "/" + p.Name}
	for _, pkg := range p.Packages {
		includes = append(includes, scan.root+"/"+strings.TrimPrefix(pkg, "/boot/"))
	}
	for _, inc := range includes {
		if err := s.engine.RestoreInclude(ctx, scan.repo, scan.id, escapeGlobLiteral(inc), tmp, scan.mode); err != nil {
			return fmt.Errorf("restore %s: %w", strings.TrimPrefix(inc, scan.root), err)
		}
	}
	staged := filepath.Join(tmp, filepath.FromSlash(scan.root))
	return copyIntoFlash(staged, s.cfg.FlashDir)
}

// copyIntoFlash copies the restored files onto the flash. FAT32 has no owners,
// modes or links, so only contents and times are written and links are
// skipped.
func copyIntoFlash(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755) //nolint:gosec // G301: the flash is FAT32 and ignores modes
		case !d.Type().IsRegular():
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(p) //nolint:gosec // G304: a file restic just restored into our own scratch folder
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(target) //nolint:gosec // G304: target lies below the flash mount, joined from a path restic restored
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chtimes(target, info.ModTime(), info.ModTime())
	})
}
