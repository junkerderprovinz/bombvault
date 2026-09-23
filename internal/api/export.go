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

// exportRecipients returns the age recipients for the plain exports and whether
// export encryption is enabled. With encryption on, an invalid or empty recipient
// list is an error, so the caller fails before writing anything instead of
// falling back to plaintext.
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

// sealOrRename publishes tmpPath as plainFinal, or as an age-encrypted
// plainFinal+".age" when recipients is set, and returns the final path. If
// encryption fails, tmpPath is left for the caller to clean up.
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
	_ = os.Remove(tmpPath)
	return final, nil
}

// writeExportFile writes data to path, or encrypted to path+".age" when
// recipients is set, and returns the path written.
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
	if err := w.Close(); err != nil {
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

// ExportContainer writes a plain backup of a container to the export directory
// and returns that directory: <name>.tar.gz of its backup folders plus <name>.xml,
// its Unraid template. Restoring needs neither BombVault nor restic, only tar and
// the template. Both files are age-encrypted when export encryption is on.
func (s *Service) ExportContainer(ctx context.Context, name string) (string, error) {
	// The handler validated name already; it is checked again because it becomes
	// a filename here.
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

	if xml, ok, _ := template.Read(s.cfg.FlashTemplatesDir, name); ok && xml != "" {
		if _, err := writeExportFile(filepath.Join(dir, name+".xml"), []byte(xml), recipients); err != nil {
			return "", fmt.Errorf("write template xml: %w", err)
		}
	}

	// A stateless container gets only the template.
	if len(appdata) > 0 {
		if _, err := s.writeTarGz(filepath.Join(dir, name+".tar.gz"), appdata, recipients); err != nil {
			return "", fmt.Errorf("write tar: %w", err)
		}
	}
	return dir, nil
}

// writeTarGz writes a gzip-compressed tar of srcPaths to dest and returns the
// path written. Entry names are relative to the host mount root, so extracting
// the archive at the host's /mnt restores the original layout. Symlinks, devices
// and other non-regular files are skipped. With recipients set, the stream is
// age-encrypted and written to dest+".age".
func (s *Service) writeTarGz(dest string, srcPaths []string, recipients []age.Recipient) (finalPath string, err error) {
	// The archive goes to a temp file that is renamed only on success, so a
	// failed export never replaces a previous good one. When encrypting, the temp
	// file already holds ciphertext.
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
			_ = f.Close()
			_ = os.Remove(tmp) //nolint:gosec // G703: tmp = final+".tmp"; final is built from a validResourceName-checked name under the operator-configured export dir
		}
	}()

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
	// Close tar, gzip, age and file in that order so every buffer is flushed
	// before the rename.
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

// dedupPaths cleans the source paths and drops duplicates and any path nested
// under another, so selecting both a folder and its child does not archive the
// child twice.
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

// addToTar walks p and writes each regular file and directory into tw, named
// relative to root.
func addToTar(tw *tar.Writer, root, p string) error {
	// A path outside root (for example one saved under an earlier HostMountRoot)
	// would get a "../"-prefixed name that escapes the target on extraction, so
	// it is archived under its own base name instead.
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
			return nil
		}
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

// ExportVM writes a plain export of a VM to the export directory and returns
// that directory: <name>.tar.gz of its disk images plus <name>.xml, the
// persistent domain definition. Restoring needs only tar and `virsh define`.
// Both files are age-encrypted when export encryption is on. A running VM is
// exported crash-consistent; shut it off first for a clean image.
func (s *Service) ExportVM(ctx context.Context, name string) (string, error) {
	// VM names may contain spaces, which validResourceName rejects.
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
	// Disks come from the live XML, the exported definition from the inactive one.
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
