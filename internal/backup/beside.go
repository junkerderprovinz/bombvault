package backup

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// ErrBesideTaken is returned when the name a copy would get already belongs
// to a container.
var ErrBesideTaken = errors.New("restore: the copy's name is taken")

// BesideContainerDeps bundles everything RestoreContainerBeside needs.
type BesideContainerDeps struct {
	// Original is the container the snapshot belongs to. Nothing is asked of
	// it and nothing is done to it.
	Original string
	// Copy is the definition the new container is created from. Its name, its
	// appdata binds and its published ports are its own.
	Copy model.Inspect
	// RecreateOnly creates the copy of a container that has no snapshot.
	RecreateOnly bool
	RepoPath     string
	SnapshotID   string
	// Dirs restores each snapshot subtree into a folder MakeDirs creates.
	Dirs []RestoreDir
	// MakeDirs creates the copy's folders, which must be new. It runs after
	// every check that can still refuse the restore.
	MakeDirs       func() error
	WithoutRuntime bool
	// TargetID is the original's run-recording target id.
	TargetID string

	Docker Docker
	// ImageID is the id of the image ref names on the host, empty when the
	// host does not have it. An image on the host is not pulled, since a pull
	// would move the tag the original runs on.
	ImageID func(ctx context.Context, ref string) string
	Restic  Restic
	Runs    Runs
}

// RestoreContainerBeside restores a snapshot into new folders and creates a
// second, stopped container on them. It never stops, removes or starts
// anything, so a failure leaves the original as it was.
func RestoreContainerBeside(ctx context.Context, d BesideContainerDeps) error {
	if !d.RecreateOnly && !snapshotIDRe.MatchString(d.SnapshotID) {
		return ErrInvalidSnapshotID
	}
	runID, err := d.Runs.Start(d.TargetID, kindRestore)
	if err != nil {
		return fmt.Errorf("restore: record run start: %w", err)
	}
	if err := runContainerBeside(ctx, d); err != nil {
		_ = d.Runs.Finish(runID, restoreOutcome(err), Summary{}, truncateErr(err))
		return err
	}
	snap := d.SnapshotID
	if d.RecreateOnly {
		snap = ""
	}
	note := ""
	if d.WithoutRuntime && d.Copy.HostConfig.UsesExtraRuntime() {
		note = store.NoteRestoredWithoutRuntime
	}
	if err := d.Runs.Finish(runID, statusSuccess, Summary{SnapshotID: snap}, note); err != nil {
		return fmt.Errorf("restore: record run finish: %w", err)
	}
	return nil
}

func runContainerBeside(ctx context.Context, d BesideContainerDeps) error {
	name := normalizeName(d.Copy.Name)
	if name == "" || name == normalizeName(d.Original) {
		return fmt.Errorf("restore: the copy needs a name of its own, not %q", name)
	}
	live, err := d.Docker.InspectName(ctx, name)
	if err != nil {
		return fmt.Errorf("restore: look for a container named %q: %w", name, err)
	}
	if live != "" {
		return fmt.Errorf("%w: a container named %q already exists", ErrBesideTaken, name)
	}
	if err := CheckRestoreNetwork(ctx, d.Docker, d.Copy); err != nil {
		return err
	}
	if len(d.Dirs) > 0 {
		if err := d.Restic.VerifySnapshot(ctx, d.RepoPath, d.SnapshotID); err != nil {
			return fmt.Errorf("restore: snapshot preflight: %w", err)
		}
	}
	in := d.Copy
	ref := pullRef(d.Copy)
	now := d.ImageID(ctx, ref)
	switch backedUp := d.Copy.Image; {
	case backedUp != "" && backedUp != ref && backedUp != now && d.ImageID(ctx, backedUp) != "":
		// The tag moved on since the backup. The copy starts on the image the
		// backup ran, and the tag the original follows stays where it is.
		in.Config.Image = backedUp
	case now == "":
		if err := d.Docker.Pull(ctx, ref); err != nil {
			return fmt.Errorf("restore: pull image: %w", err)
		}
	}
	if err := d.MakeDirs(); err != nil {
		return err
	}
	for _, rd := range d.Dirs {
		if err := d.Restic.RestoreSubtreeTo(ctx, d.RepoPath, d.SnapshotID, rd.Subtree, rd.Target); err != nil {
			return fmt.Errorf("restore: restic restore: %w", err)
		}
	}
	in.HostConfig.Links, _ = usableLinks(ctx, d.Docker, in.HostConfig.Links, false)
	if d.WithoutRuntime {
		in.HostConfig.Runtime = ""
		in.HostConfig.DeviceRequests = nil
	}
	if err := d.Docker.CreateAndStart(ctx, in, false); err != nil {
		var missing *model.MissingRuntimeError
		if errors.As(err, &missing) {
			return &runtimeRefusal{missing}
		}
		return fmt.Errorf("restore: create the copy: %w", err)
	}
	return nil
}

// BesideFile is a small file a restore writes next to the disks it restored.
type BesideFile struct {
	Path string
	Data []byte
}

// BesideVMDeps bundles everything RestoreVMBeside needs.
type BesideVMDeps struct {
	RepoPath   string
	SnapshotID string
	// Dirs restores each disk folder of the snapshot into a folder MakeDirs
	// creates.
	Dirs []VMRestoreDir
	// MakeDirs creates the copy's folders, which must be new.
	MakeDirs func() error
	// Images are the disks of a changed-block snapshot, rebuilt at targets
	// inside those folders.
	Images      []VMRestoreImage
	ImageDumper BlockDumper
	Convert     ImageConverter
	// Files are written once the disks are there.
	Files []BesideFile
	// TargetID is the VM's run-recording target id.
	TargetID string

	Restic Restic
	Runs   Runs
}

// RestoreVMBeside restores a VM's disks into new folders. It defines no
// domain and asks libvirt nothing, so the VM runs on undisturbed.
func RestoreVMBeside(ctx context.Context, d BesideVMDeps) error {
	if !snapshotIDRe.MatchString(d.SnapshotID) {
		return ErrInvalidSnapshotID
	}
	runID, err := d.Runs.Start(d.TargetID, kindRestore)
	if err != nil {
		return fmt.Errorf("vm restore: record run start: %w", err)
	}
	if err := runVMBeside(ctx, d); err != nil {
		_ = d.Runs.Finish(runID, restoreOutcome(err), Summary{}, truncateErr(err))
		return err
	}
	if err := d.Runs.Finish(runID, statusSuccess, Summary{SnapshotID: d.SnapshotID}, ""); err != nil {
		return fmt.Errorf("vm restore: record run finish: %w", err)
	}
	return nil
}

func runVMBeside(ctx context.Context, d BesideVMDeps) error {
	if len(d.Dirs) == 0 && len(d.Images) == 0 {
		return errors.New("vm restore: no disks to restore")
	}
	if err := d.Restic.VerifySnapshot(ctx, d.RepoPath, d.SnapshotID); err != nil {
		return fmt.Errorf("vm restore: snapshot preflight: %w", err)
	}
	if err := d.MakeDirs(); err != nil {
		return err
	}
	for _, img := range d.Images {
		if err := RestoreBlockImage(ctx, d.ImageDumper, d.Convert, d.RepoPath, d.SnapshotID, img); err != nil {
			return fmt.Errorf("vm restore: %w", err)
		}
	}
	for _, rd := range d.Dirs {
		if err := d.Restic.RestoreSubtreeTo(ctx, d.RepoPath, d.SnapshotID, rd.Subtree, rd.Target); err != nil {
			return fmt.Errorf("vm restore: restic restore: %w", err)
		}
	}
	for _, f := range d.Files {
		if err := os.WriteFile(f.Path, f.Data, 0o600); err != nil {
			return fmt.Errorf("vm restore: write %s: %w", f.Path, err)
		}
	}
	return nil
}
