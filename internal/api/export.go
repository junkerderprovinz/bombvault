package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/ageseal"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// exportRecipients resolves the age recipients for the plain export paths from
// settings. It returns (recipients, enabled, error): when export encryption is
// OFF it returns (nil, false, nil) and the exports stay byte-identical plaintext.
// When ON it parses settings.ExportAgeRecipients; a parse failure OR an empty
// recipient set is a HARD error (enabled=true, err!=nil), so an export path can
// fail loudly BEFORE writing anything and NEVER falls back to plaintext.
func (s *Service) exportRecipients(settings store.Settings) ([]age.Recipient, bool, error) {
	if !settings.ExportEncryptEnabled {
		return nil, false, nil
	}
	recips, err := ageseal.ParseRecipients(settings.ExportAgeRecipients)
	if err != nil {
		return nil, true, fmt.Errorf("export encryption is on but the age recipients are invalid: %w", err)
	}
	if len(recips) == 0 {
		return nil, true, errors.New("export encryption is on but no age recipient is configured")
	}
	return recips, true, nil
}

// sealOrRename publishes the temp file at tmpPath as its final artifact: a plain
// os.Rename to plainFinal when recipients is empty, or an age-encrypted copy at
// plainFinal+".age" (removing the plaintext tmp) when recipients is set. It
// returns the final path. Keeps the three on-disk export paths (container/VM tar,
// flash zip) DRY. On an encryption failure tmpPath is left for the caller's
// cleanup and no plaintext artifact is published.
func sealOrRename(tmpPath, plainFinal string, recipients []age.Recipient) (string, error) {
	if len(recipients) == 0 {
		if err := os.Rename(tmpPath, plainFinal); err != nil { //nolint:gosec // G703: operator-configured export path
			return "", err
		}
		return plainFinal, nil
	}
	final := plainFinal + ".age"
	if err := ageseal.EncryptFile(tmpPath, final, recipients); err != nil {
		return "", err
	}
	_ = os.Remove(tmpPath) // drop the plaintext temp once the ciphertext is published
	return final, nil
}

// writeExportFile writes data to path, or (when recipients is non-empty) to
// path+".age" age-encrypted, returning the final path written. Used for the
// tool-free .xml sidecars so an encrypted export never leaves a plaintext .xml.
func writeExportFile(path string, data []byte, recipients []age.Recipient) (string, error) {
	if len(recipients) == 0 {
		if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // G306: 0600 export file
			return "", err
		}
		return path, nil
	}
	final := path + ".age"
	f, err := os.OpenFile(final, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: under the operator-configured export dir
	if err != nil {
		return "", err
	}
	w, err := ageseal.WrapWriter(f, recipients)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(final)
		return "", err
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		_ = f.Close()
		_ = os.Remove(final)
		return "", err
	}
	if err := w.Close(); err != nil { // flush the age stream
		_ = f.Close()
		_ = os.Remove(final)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(final)
		return "", err
	}
	return final, nil
}

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
	// Resolve the age recipients up front: with export encryption on but no valid
	// recipient this fails BEFORE any artifact is written, so a plaintext export is
	// never produced when the user asked for encryption.
	recipients, _, err := s.exportRecipients(settings)
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

	// Write the Unraid template (the recreate recipe) as <name>.xml (or .xml.age
	// when encryption is on) when present.
	if xml, ok, _ := template.Read(s.cfg.FlashTemplatesDir, name); ok && xml != "" {
		if _, err := writeExportFile(filepath.Join(dir, name+".xml"), []byte(xml), recipients); err != nil {
			return "", fmt.Errorf("write template xml: %w", err)
		}
	}

	// Write the appdata as <name>.tar.gz (or .tar.gz.age). A stateless container
	// (no existing paths) gets only the .xml above.
	if len(appdata) > 0 {
		if _, err := s.writeTarGz(filepath.Join(dir, name+".tar.gz"), appdata, recipients); err != nil {
			return "", fmt.Errorf("write tar: %w", err)
		}
	}
	return dir, nil
}

// writeTarGz writes a gzip-compressed tar of srcPaths to dest. Entry names are
// relative to the host mount root, so extracting the archive at the host's /mnt
// reconstructs the original layout. Non-regular files (symlinks, devices) are
// skipped for safety. When recipients is non-empty the whole gzip/tar stream is
// age-encrypted and the archive is published at dest+".age" instead of dest; it
// returns the final path actually written.
func (s *Service) writeTarGz(dest string, srcPaths []string, recipients []age.Recipient) (finalPath string, err error) {
	// Write to a temp file and atomically rename on success. On ANY failure the
	// temp file is removed, so a half-written ("valid-looking" but incomplete)
	// archive is never left behind and a previous good export at dest survives.
	// When encrypting, the temp file holds CIPHERTEXT (the age writer wraps the
	// file), so no plaintext archive ever touches disk.
	final := dest
	if len(recipients) > 0 {
		final = dest + ".age"
	}
	tmp := final + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // G304: tmp is built from a validated name under the operator-configured export dir
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = f.Close()      // idempotent: harmless if already closed below
			_ = os.Remove(tmp) //nolint:gosec // G703: tmp = final+".tmp"; final is built from a validResourceName-checked name under the operator-configured export dir
		}
	}()

	// Build the writer chain: file [-> age] -> gzip -> tar. When encrypting, the
	// age writer sits directly above the file so the gzip/tar bytes are sealed.
	var sink io.Writer = f
	var ageW io.WriteCloser
	if len(recipients) > 0 {
		ageW, err = ageseal.WrapWriter(f, recipients)
		if err != nil {
			return "", err
		}
		sink = ageW
	}
	gz := gzip.NewWriter(sink)
	tw := tar.NewWriter(gz)

	root := filepath.Clean(s.cfg.HostMountRoot)
	for _, p := range dedupPaths(srcPaths) {
		if err = addToTar(tw, root, p); err != nil {
			return "", err
		}
	}
	// Close in order (tar → gzip → [age] → file) so every buffer is flushed before
	// the atomic publish; any close error aborts the rename.
	if err = tw.Close(); err != nil {
		return "", err
	}
	if err = gz.Close(); err != nil {
		return "", err
	}
	if ageW != nil {
		if err = ageW.Close(); err != nil {
			return "", err
		}
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	err = os.Rename(tmp, final) //nolint:gosec // G703: tmp/final are built from a validResourceName-checked name under the operator-configured export dir
	if err != nil {
		return "", err
	}
	return final, nil
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

// ExportVM writes a TOOL-FREE plain export of a VM next to the restic repo:
// <name>.tar.gz of its disk image(s) plus <name>.xml (the persistent domain
// definition), so it can be restored by extracting the disks and `virsh define`
// without BombVault or restic. Not encrypted (that is the point). Returns the
// export directory. A running VM is exported crash-consistent (best-effort, "just
// in case"); for a clean image, export while the VM is shut off.
func (s *Service) ExportVM(ctx context.Context, name string) (string, error) {
	// VM names legitimately contain spaces ("Home Assistant", "Windows 11"), so
	// use the VM-aware validator (still blocks path separators, "..", leading "-"
	// and control chars — safe as a filename below), not the Docker-strict
	// validResourceName which rejects any space.
	if !validVMName(name) {
		return "", fmt.Errorf("unsafe vm name %q", name)
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}
	dir, err := s.exportDir(settings)
	if err != nil {
		return "", err
	}
	// Resolve recipients up front so an encryption-on / no-recipient export fails
	// before writing any plaintext artifact (same fail-loud rule as ExportContainer).
	recipients, _, err := s.exportRecipients(settings)
	if err != nil {
		return "", err
	}
	if err := paths.EnsureDir(dir); err != nil {
		return "", fmt.Errorf("create export dir: %w", err)
	}
	if s.ssh != nil {
		if err := s.ssh.EnsureKnownHost(ctx); err != nil {
			return "", fmt.Errorf("export vm: ssh: %w", err)
		}
	}
	// Disk layout from the LIVE XML; definition from the inactive (clean) XML.
	liveXML, err := s.virsh.DumpXML(ctx, name)
	if err != nil {
		return "", fmt.Errorf("export vm: dumpxml: %w", err)
	}
	domain, err := virshcli.ParseDomain(liveXML)
	if err != nil {
		return "", fmt.Errorf("export vm: parse domain: %w", err)
	}
	if len(domain.DiskPaths) == 0 {
		return "", fmt.Errorf("export vm: no disk paths found for %q", name)
	}
	var diskPaths []string
	for _, hp := range domain.DiskPaths {
		cp, ok := s.toContainerPath(hp)
		if !ok {
			return "", fmt.Errorf("export vm: disk %q is not under the host mount and can't be reached", hp)
		}
		diskPaths = append(diskPaths, cp)
	}
	defXML := liveXML
	if inactive, ierr := s.virsh.DumpXMLInactive(ctx, name); ierr == nil && strings.TrimSpace(inactive) != "" {
		defXML = inactive
	}
	if _, err := writeExportFile(filepath.Join(dir, name+".xml"), []byte(defXML), recipients); err != nil {
		return "", fmt.Errorf("export vm: write xml: %w", err)
	}
	if _, err := s.writeTarGz(filepath.Join(dir, name+".tar.gz"), diskPaths, recipients); err != nil {
		return "", fmt.Errorf("export vm: write tar: %w", err)
	}
	return dir, nil
}

// handleExportVM writes a plain tar+xml export of a VM and returns the export
// folder. POST /api/vms/{name}/export
func (h *Handler) handleExportVM(w http.ResponseWriter, r *http.Request) {
	name, ok := h.vmNameParam(w, r)
	if !ok {
		return
	}
	dir, err := h.svc.ExportVM(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"path": dir}))
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
