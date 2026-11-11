package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

var errNotAnAppExport = errors.New("not the own backup of an app in this container")

// appExportFile is an app's own backup on its way out of BombVault.
type appExportFile struct {
	app, id, time string
	// ext is what the first bytes say the file is, "" where they do not tell.
	ext    string
	sealed bool
}

// name follows the way the apps name their own backups, with the snapshot
// added so two of one app never collide in a download folder.
func (f appExportFile) name() string {
	out := fileSafe(f.app) + "-backup"
	if at, err := time.Parse(time.RFC3339, f.time); err == nil {
		out += "-" + at.Format("20060102-150405")
	}
	out += "-" + shortID(f.id) + f.ext
	if f.sealed {
		out += ".age"
	}
	return out
}

func (f appExportFile) contentType() string {
	switch {
	case f.sealed:
		return "application/octet-stream"
	case f.ext == ".zip":
		return "application/zip"
	case f.ext == ".gz":
		return "application/gzip"
	}
	return "application/octet-stream"
}

// fileSafe keeps what an app id may put into an attachment's file name.
func fileSafe(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, text)
}

// backupExt is the extension the first bytes of a backup call for. The apps
// hand over a backup without saying what kind it is.
func backupExt(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte("PK\x03\x04")):
		return ".zip"
	case bytes.HasPrefix(head, []byte{0x1f, 0x8b}):
		return ".gz"
	}
	return ""
}

// firstBytes shows the first bytes restic delivers to look before they go on.
type firstBytes struct {
	dst  io.Writer
	look func([]byte)
	seen bool
}

func (f *firstBytes) Write(p []byte) (int, error) {
	if !f.seen {
		f.seen = true
		f.look(p)
	}
	return f.dst.Write(p)
}

// DownloadAppExport streams the own backup an app in container name handed
// over to w, age-sealed when export encryption is on. onStart gets the file
// with the first byte, so the handler can name the attachment. check runs the
// refusals and streams nothing.
func (s *Service) DownloadAppExport(ctx context.Context, name, source, snapshotID string, check bool, onStart func(appExportFile), w io.Writer) error {
	if !backup.ValidSnapshotID(snapshotID) {
		return backup.ErrInvalidSnapshotID
	}
	if !validItemSource(source) {
		return errInvalidItemSource
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	// With sealing on but broken, a plaintext stream must never start.
	recipients, sealed, err := s.exportRecipients(settings)
	if err != nil {
		return err
	}
	repo, err := s.containerRepoForName(settings, name, source)
	if err != nil {
		return err
	}
	mode := s.repoModeFor(settings, "containers", source, repo)
	// Owned by the entry's rule, so a backup under a former name comes too and
	// another machine's under that name does not.
	snaps, err := s.snapshotsOwnedBy(ctx, repo, mode, s.containerAppExportIdentity(name))
	if err != nil {
		return err
	}
	i := slices.IndexFunc(snaps, func(sn restic.Snapshot) bool { return sn.ID == snapshotID })
	if i < 0 {
		return fmt.Errorf("snapshot %s is %w", shortID(snapshotID), errNotAnAppExport)
	}
	app, takenAs, ok := appExportOf(snaps[i])
	path := appExportPath(takenAs, app.App)
	if !ok || !slices.Contains(snaps[i].Paths, path) {
		return fmt.Errorf("snapshot %s is %w", shortID(snapshotID), errNotAnAppExport)
	}
	if check {
		return nil
	}

	f := appExportFile{app: app.App, id: snapshotID, time: snaps[i].Time, sealed: sealed}
	dst := w
	var ageW *sealOnFirstWrite
	if sealed {
		ageW = &sealOnFirstWrite{dst: dst, recipients: recipients}
		dst = ageW
	}
	look := &firstBytes{dst: dst, look: func(head []byte) {
		f.ext = backupExt(head)
		onStart(f)
	}}
	log.Printf("api: the own backup of %s in %q: streaming snapshot %s to a download", app.App, name, shortID(snapshotID)) //nolint:gosec // G706: the app id from a tag and name %q-quoted
	if err := s.engine.DumpRaw(ctx, repo, snapshotID, path, look, mode); err != nil {
		return err
	}
	if ageW != nil {
		return ageW.Close()
	}
	return nil
}

// handleDownloadAppExport streams the own backup of an app in a container.
// GET /api/containers/{name}/appexports/{id}/download?source=&check=1
//
// The backup can hold the app's secrets in the clear, so only a person signed
// in with the password gets it. check=1 is the preflight the page runs before
// the browser takes over the download.
func (h *Handler) handleDownloadAppExport(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok || !h.requireAuthForSecrets(w, r, "downloading the own backup of an app") {
		return
	}
	id := r.PathValue("id")
	if r.URL.Query().Get("check") == "1" {
		if err := h.svc.DownloadAppExport(r.Context(), name, sourceParam(r), id, true, nil, nil); err != nil {
			writeJSON(w, http.StatusOK, failEnvelope(err))
			return
		}
		writeJSON(w, http.StatusOK, okEnvelope(nil))
		return
	}

	var file appExportFile
	lw := &headerOnFirstWrite{w: w, header: func() {
		w.Header().Set("Content-Type", file.contentType())
		w.Header().Set("Content-Disposition", `attachment; filename="`+file.name()+`"`)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
	}}
	err := h.svc.DownloadAppExport(r.Context(), name, sourceParam(r), id, false, func(f appExportFile) { file = f }, lw)
	switch {
	case err == nil:
	case !lw.wrote:
		writeJSON(w, http.StatusConflict, failEnvelope(err))
	default:
		// A response that ends cleanly would be saved as a whole backup.
		log.Printf("api: download of an app's own backup in %q broke off: %v", name, err) //nolint:gosec // G706: name is %q-quoted
		panic(http.ErrAbortHandler)
	}
}
