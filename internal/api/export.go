package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// exportDir returns the plain-export folder: a sibling of the containers repo
// for a local repo, or a fixed folder under the host mount for a remote repo.
func (s *Service) exportDir(settings store.Settings) (string, error) {
	repo, err := s.containersRepoPath(settings)
	if err != nil {
		return "", err
	}
	if restic.IsRemoteRepo(repo) {
		return paths.Resolve(s.cfg.HostMountRoot, "user/bombvault/export")
	}
	return filepath.Join(filepath.Dir(repo), "export"), nil
}

// ExportContainer writes a TOOL-FREE plain backup of a container next to the
// restic repo: <name>.tar.gz of its backup folders (the same paths restic uses)
// plus <name>.xml, the Unraid template, so it can be restored by simply
// extracting the tar and re-adding the template — no BombVault or restic needed.
// The export is NOT encrypted (that is the point); restic stays the encrypted,
// incremental engine. Returns the export directory.
func (s *Service) ExportContainer(ctx context.Context, name string) (string, error) {
	// Defense-in-depth: the handler already validates {name} via nameParam, but the
	// name becomes a filename here, so re-run the same strict validator (rejects
	// path separators, a leading "-", "..", control chars) — one source of truth.
	if !validResourceName(name) {
		return "", fmt.Errorf("unsafe container name %q", name)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}
	dir, err := s.exportDir(settings)
	if err != nil {
		return "", err
	}
	if err := paths.EnsureDir(dir); err != nil {
		return "", fmt.Errorf("create export dir: %w", err)
	}

	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return "", fmt.Errorf("inspect container: %w", err)
	}
	appdata := s.effectiveBackupPaths(name, in)

	// Write the Unraid template (the recreate recipe) as <name>.xml when present.
	if xml, ok, _ := template.Read(s.cfg.FlashTemplatesDir, name); ok && xml != "" {
		if err := os.WriteFile(filepath.Join(dir, name+".xml"), []byte(xml), 0o600); err != nil { //nolint:gosec // G306: 0600 export file
			return "", fmt.Errorf("write template xml: %w", err)
		}
	}

	// Write the appdata as <name>.tar.gz. A stateless container (no existing
	// paths) gets only the .xml above.
	if len(appdata) > 0 {
		if err := s.writeTarGz(filepath.Join(dir, name+".tar.gz"), appdata); err != nil {
			return "", fmt.Errorf("write tar: %w", err)
		}
	}
	return dir, nil
}

// writeTarGz writes a gzip-compressed tar of srcPaths to dest. Entry names are
// relative to the host mount root, so extracting the archive at the host's /mnt
// reconstructs the original layout. Non-regular files (symlinks, devices) are
// skipped for safety.
func (s *Service) writeTarGz(dest string, srcPaths []string) (err error) {
	// Write to a temp file and atomically rename on success. On ANY failure the
	// temp file is removed, so a half-written ("valid-looking" but incomplete)
	// archive is never left behind and a previous good export at dest survives.
	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: tmp is built from a validated name under the operator-configured export dir
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close() // idempotent: harmless if already closed below
			_ = os.Remove(tmp) //nolint:gosec // G703: tmp = dest+".tmp"; dest is built from a validResourceName-checked name under the operator-configured export dir
		}
	}()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	root := filepath.Clean(s.cfg.HostMountRoot)
	for _, p := range dedupPaths(srcPaths) {
		if err = addToTar(tw, root, p); err != nil {
			return err
		}
	}
	// Close in order (tar → gzip → file) so every buffer is flushed before the
	// atomic publish; any close error aborts the rename.
	if err = tw.Close(); err != nil {
		return err
	}
	if err = gz.Close(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	err = os.Rename(tmp, dest) //nolint:gosec // G703: tmp/dest are built from a validResourceName-checked name under the operator-configured export dir
	return err
}

// dedupPaths cleans the source paths and drops exact duplicates plus any path
// nested under another, so an operator who selects both a parent folder and a
// child of it does not archive the child's files twice (duplicate tar entries).
func dedupPaths(in []string) []string {
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(in))
	for _, p := range in {
		c := filepath.Clean(p)
		if !seen[c] {
			seen[c] = true
			cleaned = append(cleaned, c)
		}
	}
	out := make([]string, 0, len(cleaned))
	for _, p := range cleaned {
		nested := false
		for _, q := range cleaned {
			if p != q && strings.HasPrefix(p, q+string(filepath.Separator)) {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, p)
		}
	}
	return out
}

// addToTar walks p and writes each regular file/dir into tw with a name relative
// to root.
func addToTar(tw *tar.Writer, root, p string) error {
	// Pick a traversal-free top-level name for this source path. Normally p is
	// under root (the host mount), so entries are named relative to root and
	// extracting at the host's /mnt reconstructs the original layout. If p is NOT
	// under root (e.g. a selected path saved under a previous HostMountRoot),
	// filepath.Rel would yield a "../.."-prefixed name that escapes on extraction
	// (CWE-22 in the produced artifact) — root it at its own base instead.
	base, rerr := filepath.Rel(root, p)
	if rerr != nil || base == ".." || strings.HasPrefix(base, ".."+string(filepath.Separator)) {
		base = filepath.Base(p)
	}
	base = filepath.ToSlash(base)

	//nolint:gosec // G703: p is a backup source path (container-translated, existence-filtered, under the host mount), not raw user input
	return filepath.Walk(p, func(file string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if !fi.IsDir() && !fi.Mode().IsRegular() {
			return nil // skip symlinks / devices / sockets
		}
		// file is always under p, so this Rel is clean and never escapes.
		sub, serr := filepath.Rel(p, file)
		if serr != nil {
			return serr
		}
		name := base
		if sub != "." {
			name += "/" + filepath.ToSlash(sub)
		}
		hdr, herr := tar.FileInfoHeader(fi, "")
		if herr != nil {
			return herr
		}
		hdr.Name = name
		if fi.IsDir() {
			hdr.Name += "/"
			return tw.WriteHeader(hdr)
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		src, oerr := os.Open(file) //nolint:gosec // G304: file comes from filepath.Walk over an operator backup path
		if oerr != nil {
			return oerr
		}
		defer src.Close() //nolint:errcheck // read-only file
		_, cerr := io.Copy(tw, src)
		return cerr
	})
}

// handleExportContainer writes a plain tar+xml export of a container and returns
// the export folder. POST /api/containers/{name}/export
func (h *Handler) handleExportContainer(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok {
		return
	}
	dir, err := h.svc.ExportContainer(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"path": dir}))
}
