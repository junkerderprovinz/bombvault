package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/fortress"
	"github.com/junkerderprovinz/parleyport/kit/group"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// The reasons a backup does not go back to its app, as ids the page turns
// into sentences.
const (
	appImportOff            = "off"
	appImportNoFortress     = "noFortress"
	appImportNotAnAppBackup = "notAnAppBackup"
	appImportNotRunning     = "notRunning"
	appImportNoApp          = "noApp"
	appImportOtherApp       = "otherApp"
	appImportRelay          = "relay"
	appImportPermissionOff  = "permissionOff"
	appImportBusy           = "busy"
	appImportVersionTooNew  = "versionTooNew"
	appImportWrongApp       = "wrongApp"
	appImportKeyMissing     = "keyMissing"
	appImportBroken         = "broken"
	appImportByApp          = "refused"
)

// appImportWait bounds handing a backup back, which the kit sends without a
// deadline of its own.
const appImportWait = 30 * time.Minute

// appImportRefusal is a backup that does not go back to its app, with the id
// the page words it by.
type appImportRefusal struct {
	code, msg string
}

func (e *appImportRefusal) Error() string { return e.msg }

func refuseAppImport(code, format string, a ...any) error {
	return &appImportRefusal{code: code, msg: fmt.Sprintf(format, a...)}
}

// appImporter is the part of a fortress that hands an app its backup back.
type appImporter interface {
	memberLister
	Import(ctx context.Context, id string, backup io.Reader, n int64) error
}

// ImportAppExport gives the own backup in snapshotID back to the app that runs
// in container name, which replaces what the app holds.
func (s *Service) ImportAppExport(ctx context.Context, name, source, snapshotID string) error {
	f := s.fortressNow()
	if f == nil {
		return refuseAppImport(appImportNoFortress, "BombVault is in no fortress, so no app can take a backup back")
	}
	return s.importAppExportTo(ctx, f, name, source, snapshotID)
}

func (s *Service) importAppExportTo(ctx context.Context, im appImporter, name, source, snapshotID string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if !settings.AppImportEnabled {
		return refuseAppImport(appImportOff, "giving backups back to apps is switched off in the settings")
	}
	ae, err := s.appExportFor(ctx, settings, name, source, snapshotID)
	if errors.Is(err, errNotAnAppExport) || errors.Is(err, backup.ErrInvalidSnapshotID) {
		return &appImportRefusal{code: appImportNotAnAppBackup, msg: err.Error()}
	}
	if err != nil {
		return err
	}
	in, err := s.docker.Inspect(ctx, name)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}
	if !in.Running {
		return refuseAppImport(appImportNotRunning, "%s is not running, so its app cannot take the backup", name)
	}
	m, ok := memberOffering(ctx, im, in.ID, fortress.ImportAction)
	switch {
	case !ok:
		return refuseAppImport(appImportNoApp, "no app in %s takes a backup back", name)
	case m.App.ID != ae.app.App:
		return refuseAppImport(appImportOtherApp, "the app in %s is %s, and the backup is of %s", name, m.App.ID, ae.app.App)
	case !m.Direct:
		return refuseAppImport(appImportRelay, "%s is reached through the relay alone, and a backup goes back only on this network", ae.app.App)
	}
	n, err := s.storedSize(ctx, ae)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, appImportWait)
	defer cancel()
	pr, pw := io.Pipe()
	read := make(chan error, 1)
	go func() {
		err := s.engine.DumpRaw(ctx, ae.repo, ae.snap.ID, ae.path, pw, ae.mode)
		_ = pw.CloseWithError(err)
		read <- err
	}()
	err = im.Import(ctx, m.ID, pr, n)
	// Closing the read side ends the dump when the app answers before the last
	// byte, as a refusal does.
	_ = pr.Close()
	readErr := <-read
	if err != nil {
		return appImportFailure(ae.app.App, err)
	}
	if readErr != nil {
		return fmt.Errorf("read the backup: %w", readErr)
	}
	log.Printf("fortress: gave %s in %s its own backup %s back", ae.app.App, name, shortID(ae.snap.ID)) //nolint:gosec // G706: the app id from a tag and a container name from Docker, %s on a log line
	return nil
}

// storedSize is how long the backup is, which the app is told before it reads
// it. restic records it with the snapshot; one written without a summary is
// asked for its file.
func (s *Service) storedSize(ctx context.Context, ae storedAppExport) (int64, error) {
	if ae.snap.Summary != nil && ae.snap.Summary.TotalBytesProcessed > 0 {
		return clampToInt64(ae.snap.Summary.TotalBytesProcessed), nil
	}
	size := int64(-1)
	err := s.engine.LsStreamNoLock(ctx, ae.repo, ae.snap.ID, ae.mode, func(e restic.FileEntry) {
		if e.Path == ae.path && e.Type == "file" {
			size = e.Size
		}
	})
	if err != nil {
		return 0, fmt.Errorf("measure the backup: %w", err)
	}
	if size < 0 {
		return 0, fmt.Errorf("snapshot %s is %w", shortID(ae.snap.ID), errNotAnAppExport)
	}
	return size, nil
}

// appImportFailure words what the app or the way to it answered.
func appImportFailure(app string, err error) error {
	var no *fortress.Refusal
	switch {
	case errors.Is(err, fortress.ErrPermissionOff):
		return refuseAppImport(appImportPermissionOff, "%s does not let BombVault give it a backup back", app)
	case errors.Is(err, fortress.ErrNoSuchAction):
		return refuseAppImport(appImportNoApp, "%s takes no backup back", app)
	case errors.Is(err, group.ErrNotDirect):
		return refuseAppImport(appImportRelay, "%s is reached through the relay alone, and a backup goes back only on this network", app)
	case errors.Is(err, group.ErrStreamBroken):
		return refuseAppImport(appImportBroken, "the backup broke off on its way to %s, which kept what it had", app)
	case errors.As(err, &no):
		code, ok := map[string]string{
			"busy":            appImportBusy,
			"version_too_new": appImportVersionTooNew,
			"wrong_app":       appImportWrongApp,
			"key_missing":     appImportKeyMissing,
			"stream_broken":   appImportBroken,
		}[no.Code]
		if !ok {
			code = appImportByApp
		}
		return refuseAppImport(code, "%s refused the backup: %s", app, no.Code)
	}
	return fmt.Errorf("give %s its backup back: %w", app, err)
}

// handleImportAppExport gives an app its own backup back.
// POST /api/containers/{name}/appexports/{id}/import?source=
//
// It replaces what the app holds, so only a person signed in with the password
// may. A refusal comes back with the id the page words it by.
func (h *Handler) handleImportAppExport(w http.ResponseWriter, r *http.Request) {
	name, ok := h.nameParam(w, r)
	if !ok || !h.requireAuthForSecrets(w, r, "giving an app its backup back") {
		return
	}
	err := h.svc.ImportAppExport(r.Context(), name, sourceParam(r), r.PathValue("id"))
	var refused *appImportRefusal
	switch {
	case errors.As(err, &refused):
		writeJSON(w, http.StatusOK, codedFailEnvelope(err, refused.code))
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"imported": true}))
	}
}
