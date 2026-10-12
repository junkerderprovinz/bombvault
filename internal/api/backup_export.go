package api

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"filippo.io/age"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/sizetree"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// The forms a stored backup can be exported in.
const (
	exportTar  = "tar"
	exportZip  = "zip"
	exportRepo = "repository"
)

// ExportRequest says which backup of an entry to export, as what and where to.
type ExportRequest struct {
	// Form is "tar", "zip" or "repository".
	Form string `json:"form"`
	// SnapshotID names the backup. The repository form takes none to copy
	// every backup of the entry.
	SnapshotID string `json:"snapshotId"`
	// Folder lies below the host mount. Empty is the export folder beside the
	// repositories.
	Folder string `json:"folder"`
}

// ExportPreview names what an export writes before it writes anything.
type ExportPreview struct {
	// Folder is the path of the export folder on the host.
	Folder string   `json:"folder"`
	Files  []string `json:"files"`
	// Replaces are the files of the same name the export overwrites.
	Replaces []string `json:"replaces"`
	// Backups is how many backups the repository form copies.
	Backups int `json:"backups,omitempty"`
	// Bytes is what the archives hold before compression, 0 when unknown.
	Bytes int64 `json:"bytes,omitempty"`
	// Encryption is "age" for archives sealed for the export recipients,
	// "password" for a repository under this instance's restic password, and
	// "none" for plain archives or a repository without a password.
	Encryption string `json:"encryption"`
	// Segments marks the archive of a changed-block VM backup: it holds each
	// disk as numbered segment files, which give the disk image when joined in
	// order, next to their manifest.
	Segments bool `json:"segments,omitempty"`
	// Key is the progress key the running export reports under and is
	// cancelled by.
	Key string `json:"key"`
}

// exportPart is one file of an archive export: a snapshot written as a tar or a
// zip, or the stream of a zvol disk written as it is.
type exportPart struct {
	name   string
	snap   restic.Snapshot
	stream string
}

// exportPlan is a checked export, so the restic work runs detached from the
// request that asked for it.
type exportPlan struct {
	ExportPreview
	domain, form string
	// dir is the export folder as BombVault reaches it.
	dir        string
	set        backupSet
	parts      []exportPart
	ids        []string
	recipients []age.Recipient
	targetID   string
}

var unsafeInFileName = regexp.MustCompile(`[^\p{L}\p{N}._-]+`)

// fileStem turns the name of an entry into the start of a file name.
func fileStem(name string) string {
	return strings.Trim(unsafeInFileName.ReplaceAllString(name, "-"), "-.")
}

// planExport checks a request and works out the folder and the files it would
// write. Nothing is created.
func (s *Service) planExport(ctx context.Context, domain, key, source string, req ExportRequest) (exportPlan, error) {
	if !slices.Contains([]string{exportTar, exportZip, exportRepo}, req.Form) {
		return exportPlan{}, errors.New("unknown export form (tar, zip or repository)")
	}
	if req.SnapshotID == "" && req.Form != exportRepo {
		return exportPlan{}, errors.New("choose the backup to export")
	}
	if req.SnapshotID != "" && !backup.ValidSnapshotID(req.SnapshotID) {
		return exportPlan{}, backup.ErrInvalidSnapshotID
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return exportPlan{}, fmt.Errorf("read settings: %w", err)
	}
	plan := exportPlan{domain: domain, form: req.Form}
	if req.Folder == "" {
		plan.dir, err = s.exportDir(settings)
	} else if plan.dir, err = paths.ResolveForWrite(s.cfg.HostMountRoot, req.Folder); err != nil {
		err = errors.New("invalid export folder: must be a relative subpath under the host mount")
	}
	if err != nil {
		return exportPlan{}, err
	}
	plan.ExportPreview = ExportPreview{Folder: s.toHostPath(plan.dir), Replaces: []string{}, Encryption: "none", Key: "export:" + domain}
	if plan.targetID, err = s.runTargetID(domain, key); err != nil {
		return exportPlan{}, err
	}
	if plan.set, err = s.backupsOf(ctx, domain, key, source); err != nil {
		return exportPlan{}, err
	}
	var chosen *restic.Snapshot
	if req.SnapshotID != "" {
		snap, err := plan.set.find(req.SnapshotID)
		if err != nil {
			return exportPlan{}, err
		}
		chosen = &snap
	}
	if req.Form == exportRepo {
		return s.planRepositoryExport(ctx, plan, chosen)
	}
	return s.planArchiveExport(ctx, settings, plan, *chosen)
}

// planArchiveExport names one archive per snapshot of the backup. A ZFS run
// gives an archive per dataset, and a zvol disk of a VM its stream.
func (s *Service) planArchiveExport(ctx context.Context, settings store.Settings, plan exportPlan, chosen restic.Snapshot) (exportPlan, error) {
	recipients, sealed, err := s.exportRecipients(settings)
	if err != nil {
		return exportPlan{}, err
	}
	plan.recipients = recipients
	suffix := ""
	if sealed {
		plan.Encryption, suffix = "age", ".age"
	}
	parts, err := s.partsOf(ctx, plan.set, chosen)
	if err != nil {
		return exportPlan{}, err
	}
	ext := ".tar.gz"
	if plan.form == exportZip {
		ext = ".zip"
	}
	when := parseSnapshotTime(chosen.Time).UTC().Format("20060102-150405")
	var snaps []restic.Snapshot
	for _, label := range slices.Sorted(maps.Keys(parts)) {
		part := exportPart{snap: parts[label], name: fileStem(plan.set.name) + "-" + when + ext}
		switch {
		case plan.set.zfsRoot != "":
			part.name = fileStem(label) + "-" + when + ext
			// tank/a-b and tank/a/b come out as the same file name.
			for n := 2; slices.Contains(plan.Files, part.name+suffix); n++ {
				part.name = fmt.Sprintf("%s-%s-%d%s", fileStem(label), when, n, ext)
			}
		case label != "":
			part.name = fileStem(plan.set.name) + "-" + when + "-" + fileStem(label) + ".zfs.gz"
			part.stream = part.snap.Paths[0]
		}
		plan.parts = append(plan.parts, part)
		plan.Files = append(plan.Files, part.name+suffix)
		if _, err := os.Stat(filepath.Join(plan.dir, part.name+suffix)); err == nil {
			plan.Replaces = append(plan.Replaces, part.name+suffix)
		}
		plan.Segments = plan.Segments || slices.Contains(part.snap.Tags, backup.BlocksTag)
		snaps = append(snaps, part.snap)
	}
	if size := recordedSize(snaps); size != nil {
		plan.Bytes = *size
	}
	return plan, nil
}

// planRepositoryExport names the folder of a repository of its own and the
// snapshots that go into it: one backup, or all the entry has at the source.
func (s *Service) planRepositoryExport(ctx context.Context, plan exportPlan, chosen *restic.Snapshot) (exportPlan, error) {
	name := fileStem(plan.set.name) + "-restic"
	if err := freeForRepository(filepath.Join(plan.dir, name)); err != nil {
		return exportPlan{}, err
	}
	var snaps []restic.Snapshot
	if chosen != nil {
		parts, err := s.partsOf(ctx, plan.set, *chosen)
		if err != nil {
			return exportPlan{}, err
		}
		snaps = slices.Collect(maps.Values(parts))
		plan.Backups = 1
	} else {
		var err error
		if snaps, err = s.withZvolDisks(ctx, plan.set); err != nil {
			return exportPlan{}, err
		}
		plan.Backups = plan.set.backups()
	}
	if len(snaps) == 0 {
		return exportPlan{}, errors.New("this entry has no backups here")
	}
	for _, snap := range snaps {
		plan.ids = append(plan.ids, snap.ID)
	}
	plan.Files = []string{name + "/"}
	if plan.set.mode.Encrypted {
		plan.Encryption = "password"
	}
	return plan, nil
}

// freeForRepository accepts a folder that is not there yet or is empty, so a
// copy never lands in a repository that is in use, and a failed one never
// removes what somebody else put there.
func freeForRepository(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s already exists in the export folder; a copy needs a folder of its own, so remove it or choose another export folder", filepath.Base(dir))
	}
	held, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(held) > 0 {
		return fmt.Errorf("the folder %s is not empty; a copy needs a folder of its own, so remove it or choose another export folder", filepath.Base(dir))
	}
	return nil
}

// withZvolDisks returns every snapshot of the entry, for a VM with the zvol
// disk snapshots its runs wrote beside their own.
func (s *Service) withZvolDisks(ctx context.Context, set backupSet) ([]restic.Snapshot, error) {
	if set.domain != "vms" {
		return set.snaps, nil
	}
	all, err := s.listRepo(ctx, set.repo, set.mode)
	if err != nil {
		return nil, err
	}
	runs := map[string]bool{}
	for _, snap := range set.snaps {
		if tag := vmRunTag(set.snaps, snap.ID); tag != "" {
			runs[tag] = true
		}
	}
	out := slices.Clone(set.snaps)
	for _, snap := range all {
		isDisk := slices.ContainsFunc(snap.Tags, func(tag string) bool { return strings.Contains(tag, ":zvol:") })
		if isDisk && slices.ContainsFunc(snap.Tags, func(tag string) bool { return runs[tag] }) {
			out = append(out, snap)
		}
	}
	return out, nil
}

// archiveRoot is the folder of a snapshot its archive starts at. A backup of
// folders below the host mount starts at the mount, so unpacking it at the
// host's mount source gives every folder its place, as the export of a running
// container does. Any other backup starts at the folder that holds all of it.
func (s *Service) archiveRoot(set backupSet, snap restic.Snapshot) string {
	// These were read from inside a folder, so its entries are the root.
	if set.zfsRoot != "" || slices.Contains(snap.Tags, backup.BlocksTag) || slices.Contains(snap.Tags, restic.ImportedTag) {
		return "/"
	}
	below := func(p string) bool { return paths.Within(s.cfg.HostMountRoot, p) }
	if len(snap.Paths) > 0 && !slices.ContainsFunc(snap.Paths, func(p string) bool { return !below(p) }) {
		return path.Clean(s.cfg.HostMountRoot)
	}
	return sizetree.CommonRoot(snap.Paths)
}

// exportMeter counts what restic hands over and reports it at most once a
// second. It gives a percentage for a tar, whose length is about what the
// backup holds; a zip arrives compressed, so only its bytes are known.
type exportMeter struct {
	w       io.Writer
	publish func(bytes int64)
	read    int64
	last    time.Time
}

func (m *exportMeter) Write(p []byte) (int, error) {
	n, err := m.w.Write(p)
	m.read += int64(n)
	if now := time.Now(); now.Sub(m.last) >= time.Second {
		m.last = now
		m.publish(m.read)
	}
	return n, err
}

// runExport does the restic work of a planned export under the domain lock and
// returns how many bytes it wrote.
func (s *Service) runExport(ctx context.Context, plan exportPlan, startedAt int64) (int64, error) {
	unlock := s.lockDomainFor(plan.domain, "export")
	defer unlock()
	if err := paths.EnsureDir(plan.dir); err != nil {
		return 0, fmt.Errorf("create the export folder: %w", err)
	}
	if plan.form == exportRepo {
		return 0, s.copyExport(ctx, plan, startedAt)
	}
	var written, done int64
	for _, part := range plan.parts {
		meter := &exportMeter{publish: func(read int64) {
			ev := progress.Event{Key: plan.Key, Phase: "maintenance", Active: true, StartedAt: startedAt, Bytes: done + read}
			if plan.form == exportTar && plan.Bytes > 0 {
				ev.Percent = min(99, 100*float64(done+read)/float64(plan.Bytes))
			}
			s.publishProgress(ev)
		}}
		final, err := publishExport(filepath.Join(plan.dir, part.name), plan.recipients, func(w io.Writer) error {
			return s.dumpPart(ctx, plan, part, w, meter)
		})
		if err != nil {
			return written, fmt.Errorf("write %s: %w", part.name, err)
		}
		done += meter.read
		if fi, err := os.Stat(final); err == nil {
			written += fi.Size()
		}
	}
	return written, nil
}

// dumpPart streams one snapshot out of the repository into w, past the meter.
// restic writes the archive while it reads, so nothing is unpacked to disk in
// between.
func (s *Service) dumpPart(ctx context.Context, plan exportPlan, part exportPart, w io.Writer, meter *exportMeter) error {
	repo, mode := plan.set.repo, plan.set.mode
	if plan.form == exportZip && part.stream == "" {
		meter.w = w
		return s.engine.DumpZip(ctx, repo, part.snap.ID, s.archiveRoot(plan.set, part.snap), meter, mode)
	}
	// The meter sits before gzip, where the bytes are the backup's own.
	gz := gzip.NewWriter(w)
	meter.w = gz
	var err error
	if part.stream != "" {
		err = s.engine.DumpRaw(ctx, repo, part.snap.ID, part.stream, meter, mode)
	} else {
		err = s.engine.DumpTar(ctx, repo, part.snap.ID, s.archiveRoot(plan.set, part.snap), meter, mode)
	}
	if err != nil {
		return err
	}
	return gz.Close()
}

// copyExport creates the repository and copies the planned snapshots into it.
// It opens with the password of the repository it is copied from, which is
// this instance's own, or with none where encryption is off. A copy that fails
// removes the folder when it created it.
func (s *Service) copyExport(ctx context.Context, plan exportPlan, startedAt int64) (err error) {
	dest := filepath.Join(plan.dir, strings.TrimSuffix(plan.Files[0], "/"))
	// Asked again here, because what follows removes the folder on a failure.
	if err := freeForRepository(dest); err != nil {
		return err
	}
	_, statErr := os.Lstat(dest)
	created := errors.Is(statErr, fs.ErrNotExist)
	defer func() {
		if err != nil && created {
			_ = os.RemoveAll(dest)
		}
	}()
	source := plan.set.mode
	if err = s.engine.Init(ctx, dest, restic.Mode{Encrypted: source.Encrypted, Password: source.Password}); err != nil {
		return fmt.Errorf("create the repository: %w", err)
	}
	last := -1.0
	ctx = progress.WithCopySink(ctx, func(cp progress.CopyProgress) {
		if cp.Percent < 100 && cp.Percent >= last && cp.Percent-last < 1 {
			return
		}
		last = cp.Percent
		s.publishProgress(progress.Event{
			Key: plan.Key, Phase: "maintenance", Active: true, StartedAt: startedAt,
			Percent: cp.Percent, SnapshotIndex: cp.SnapshotIndex, SnapshotTotal: len(plan.ids),
		})
	})
	// The mode carries the credentials that reach the source; the new
	// repository is a local folder and takes none of its limits.
	source.Limits = restic.Limits{}
	return s.engine.Copy(ctx, dest, plan.set.repo, plan.ids, restic.Limits{}, source)
}

func (s *Service) publishProgress(ev progress.Event) {
	if s.progress != nil {
		s.progress.Publish(ev)
	}
}

// PreviewExport names the folder and the files an export would write.
func (s *Service) PreviewExport(ctx context.Context, domain, key, source string, req ExportRequest) (ExportPreview, error) {
	plan, err := s.planExport(ctx, domain, key, source, req)
	return plan.ExportPreview, err
}

// StartExport checks an export and runs it in the background as a run of kind
// "export" on the entry. started is false while a backup, a restore or another
// export is running.
func (s *Service) StartExport(ctx context.Context, domain, key, source string, req ExportRequest) (preview ExportPreview, started bool, err error) {
	if !s.batchActive.CompareAndSwap(false, true) {
		return ExportPreview{}, false, nil
	}
	plan, err := s.planExport(ctx, domain, key, source, req)
	if err != nil {
		s.batchActive.Store(false)
		return ExportPreview{}, false, err
	}
	bctx := context.WithoutCancel(ctx)
	go func() {
		var runID string
		defer s.recoverOperation("export: "+domain, nil, func(msg string) { s.finishExportRun(runID, 0, errors.New(msg)) })
		defer s.batchActive.Store(false)
		tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)
		defer tcancel()
		rctx, cancel := context.WithCancel(tctx)
		defer cancel()
		s.registerCancel(plan.Key, cancel)
		defer s.unregisterCancel(plan.Key)
		if plan.targetID != "" {
			var rErr error
			if runID, rErr = s.startRun(bctx, plan.targetID, "export"); rErr != nil {
				log.Printf("api: export: the run is not recorded: %v", rErr)
			}
		}
		_, startedAt := s.progBegin(rctx, plan.Key, "maintenance")
		written, xErr := s.runExport(rctx, plan, startedAt)
		s.progEnd(plan.Key, "maintenance", xErr == nil, startedAt)
		s.finishExportRun(runID, written, xErr)
		if xErr != nil {
			log.Printf("api: export of a %s backup failed: %v", domain, xErr) //nolint:gosec // G706: domain is one of the fixed domain names
		}
	}()
	return plan.ExportPreview, true, nil
}

// finishExportRun closes the run of an export with what it wrote, or with why
// it ended early.
func (s *Service) finishExportRun(runID string, written int64, xErr error) {
	if runID == "" {
		return
	}
	status := "success"
	switch {
	case errors.Is(xErr, context.Canceled):
		status = "cancelled"
	case xErr != nil:
		status = "failed"
	}
	if err := s.store.FinishRun(runID, status, "", written, truncateRunErr(xErr)); err != nil {
		log.Printf("api: export: the run is not closed: %v", err)
	}
}

// handleExportPreview serves POST /api/items/{domain}/{name}/export/preview?source=
// with the body {form, snapshotId, folder} and names the folder and the files
// the export would write.
func (h *Handler) handleExportPreview(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, itemDomains...)
	if !ok {
		return
	}
	var req ExportRequest
	if !decodeBody(w, r, &req) {
		return
	}
	preview, err := h.svc.PreviewExport(r.Context(), domain, key, sourceParam(r), req)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"export": preview}))
}

// handleExportBackup serves POST /api/items/{domain}/{name}/export?source= with
// the body of the preview and starts the export.
func (h *Handler) handleExportBackup(w http.ResponseWriter, r *http.Request) {
	domain, key, ok := h.itemParam(w, r, itemDomains...)
	if !ok {
		return
	}
	var req ExportRequest
	if !decodeBody(w, r, &req) {
		return
	}
	preview, started, err := h.svc.StartExport(r.Context(), domain, key, sourceParam(r), req)
	if err != nil {
		placementFail(w, err, nil)
		return
	}
	if !started {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "a backup or restore is already running"})
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"started": true, "export": preview}))
}
