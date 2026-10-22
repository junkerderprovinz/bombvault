package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/backup"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// targetBeside is the restore target that puts a copy next to the original
// and leaves the original alone.
const targetBeside = "beside"

// besideSuffix is what a copy adds to the name of the folder or container it
// was restored next to.
const besideSuffix = "-restored"

// The labels of a container that is such a copy: the container it stands next
// to and the snapshot it came from.
const (
	restoredFromLabel     = "bombvault.restored-from"
	restoredSnapshotLabel = "bombvault.restored-snapshot"
)

// RestoreOptions are the choices a restore request makes beyond its snapshot.
type RestoreOptions struct {
	LeaveStopped   bool
	WithoutRuntime bool
	// Beside restores a copy next to the original instead of over it.
	Beside bool
	// BackupFirst backs up the current state before a restore replaces it. A
	// failed backup keeps the restore from starting.
	BackupFirst bool
}

// restoreTarget reads the target of a restore request: in place when it names
// none, next to the original for "beside".
func restoreTarget(target string) (beside bool, err error) {
	switch target {
	case "":
		return false, nil
	case targetBeside:
		return true, nil
	}
	return false, fmt.Errorf("unknown restore target %q", target)
}

// besideRoot is a folder a restore creates next to the one it copies, both as
// container paths.
type besideRoot struct {
	original string
	copy     string
}

// BesidePlan says where a restore next to the original puts its copy.
type BesidePlan struct {
	// Name is the container the restore creates, or else its first new folder.
	Name    string         `json:"name"`
	Folders []BesideFolder `json:"folders"`
	// Container is set when the restore also creates a stopped container.
	Container *BesideContainer `json:"container,omitempty"`
}

// BesideFolder pairs a folder with its copy, as host paths.
type BesideFolder struct {
	Original string `json:"original"`
	Copy     string `json:"copy"`
}

// BesideContainer is how the created container differs from the original
// apart from its name and folders.
type BesideContainer struct {
	Ports []BesidePort `json:"ports"`
	// Shared are the writable volumes the copy mounts together with the
	// original, because the backup holds nothing of them.
	Shared []string `json:"shared"`
	// FixedIP is the address the original keeps. The copy gets none.
	FixedIP string `json:"fixedIp,omitempty"`
	// Restart is the restart policy the original has. The copy has none, so
	// Docker never starts it by itself.
	Restart     string `json:"restart,omitempty"`
	HostNetwork bool   `json:"hostNetwork,omitempty"`
}

// BesidePort is one published port: Copy is empty when Docker picks the host
// port once the copy starts.
type BesidePort struct {
	Container string `json:"container"`
	Original  string `json:"original"`
	Copy      string `json:"copy"`
}

// besideRootOf names the folder of an appdata path that gets a copy next to
// it: the container's own folder below a data root, or else the path itself.
func besideRootOf(segments []string) func(string) string {
	return func(src string) string {
		root, rel := appdataRelPath(src, segments)
		if rel == "" {
			return src
		}
		first, _, _ := strings.Cut(rel, "/")
		return root + "/" + first
	}
}

// besideLayout turns what a restore in place would write, each snapshot
// subtree into its own Target, into a restore next to those targets. rootOf
// names the folder of a target that is copied as a whole. A folder inside
// another one shares that one's copy, so no copy is ever created inside an
// original.
func besideLayout(inPlace []backup.RestoreDir, rootOf func(string) string) ([]besideRoot, []backup.RestoreDir) {
	tops := make([]string, len(inPlace))
	for i, d := range inPlace {
		tops[i] = rootOf(path.Clean(d.Target))
	}
	tops = PruneMaximal(sortedUnique(tops))
	roots := make([]besideRoot, len(tops))
	for i, t := range tops {
		roots[i] = besideRoot{original: t, copy: t + besideSuffix}
	}
	dirs := make([]backup.RestoreDir, len(inPlace))
	for i, d := range inPlace {
		target, _ := besidePath(roots, path.Clean(d.Target))
		dirs[i] = backup.RestoreDir{Subtree: d.Subtree, Target: target}
	}
	return roots, dirs
}

// besidePath is where p lies in the copy of the root that holds it. It
// reports false for a path no root holds, which has no place in a copy.
func besidePath(roots []besideRoot, p string) (string, bool) {
	for _, r := range roots {
		if p == r.original || isStrictDescendant(p, r.original) {
			return r.copy + strings.TrimPrefix(p, r.original), true
		}
	}
	return "", false
}

// guardBesideRoots refuses a copy that would land on something that is there
// already, outside the Host Data mount, or on other storage than its original.
func (s *Service) guardBesideRoots(roots []besideRoot) error {
	for _, r := range roots {
		shown := s.toHostPath(r.copy)
		if !paths.Within(s.cfg.HostMountRoot, r.copy) {
			return destinationRefusal("a copy next to %s would lie outside the Host Data mount. Restore into a folder instead", s.toHostPath(r.original))
		}
		switch _, err := os.Lstat(r.copy); {
		case err == nil:
			return destinationRefusal("%s already exists. Move or remove it, then restore again", shown)
		case !errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("look at %s: %w", shown, err)
		}
		parent := path.Dir(r.copy)
		if fi, err := os.Stat(parent); err != nil || !fi.IsDir() {
			return destinationRefusal("%s is missing, so there is nothing to put a copy next to. Restore into a folder instead", s.toHostPath(parent))
		}
		if !s.restoreTargetMounted(r.copy) {
			return destinationRefusal("%s is not on a mounted pool or share, so a copy there would be written into the host's memory. Restore into a folder instead", shown)
		}
		// A folder that is a mount of its own has its neighbours on other storage.
		if own, err := s.diskStatFn()(r.original); err == nil {
			if around, err := s.diskStatFn()(parent); err == nil && own.Volume != around.Volume {
				return destinationRefusal("%s is a mount of its own, so a folder next to it lies on other storage. Restore into a folder instead", s.toHostPath(r.original))
			}
		}
	}
	return nil
}

// guardBesideRoom refuses a copy whose snapshot is larger than what is free
// where it goes. Copies on several volumes are left to the check's dry run,
// which measures each one.
func (s *Service) guardBesideRoom(ctx context.Context, ref repoRef, snapshotID string, roots []besideRoot) error {
	if isPlanOnly(ctx) || len(roots) == 0 {
		return nil
	}
	_, want, err := s.engine.StatsRestoreSize(ctx, ref.repo, snapshotID, ref.mode)
	if err != nil || want <= 0 {
		return nil
	}
	var at diskStatResult
	for i, r := range roots {
		st, err := s.diskStatFn()(nearestExistingDir(r.copy))
		if err != nil || (i > 0 && st.Volume != at.Volume) {
			return nil
		}
		at = st
	}
	if at.Free < uint64(want) {
		return destinationRefusal("not enough free space for a copy next to the original: it needs %d bytes and %s has %d free", want, s.toHostPath(path.Dir(roots[0].copy)), at.Free)
	}
	return nil
}

// makeBesideFolders creates the folders a copy is restored into. Every root
// has to be new, which is what keeps a restore next to the original from
// writing into anything that was there before it. The returned undo removes
// the roots again.
func (s *Service) makeBesideFolders(roots []besideRoot, dirs []backup.RestoreDir) (undo func(), err error) {
	var made []string
	undo = func() {
		for _, p := range made {
			if err := os.RemoveAll(p); err != nil {
				log.Printf("api: restore: removing the unfinished copy %q: %v", s.toHostPath(p), err)
			}
		}
	}
	for _, r := range roots {
		if err := os.Mkdir(r.copy, 0o755); err != nil { //nolint:gosec // G301: a restored folder on a share must stay readable for its user
			undo()
			if errors.Is(err, fs.ErrExist) {
				return nil, destinationRefusal("%s already exists. Move or remove it, then restore again", s.toHostPath(r.copy))
			}
			return nil, fmt.Errorf("create %s: %w", s.toHostPath(r.copy), err)
		}
		made = append(made, r.copy)
	}
	for _, d := range dirs {
		if err := paths.EnsureDirReadable(d.Target); err != nil {
			undo()
			return nil, fmt.Errorf("create %s: %w", s.toHostPath(d.Target), err)
		}
	}
	for _, r := range roots {
		likeOriginal(r)
	}
	return undo, nil
}

// likeOriginal gives a copy's root the owner and mode of the folder it
// copies, when that folder is still there.
func likeOriginal(r besideRoot) {
	info, err := os.Stat(r.original)
	if err != nil || !info.IsDir() {
		return
	}
	if uid, gid, ok := dirOwner(info); ok {
		if err := os.Lchown(r.copy, uid, gid); err != nil {
			log.Printf("api: restore: owner of the copy's folder: %v", err)
		}
	}
	if err := os.Chmod(r.copy, info.Mode().Perm()); err != nil {
		log.Printf("api: restore: mode of the copy's folder: %v", err)
	}
}

func (s *Service) besideFolders(roots []besideRoot) []BesideFolder {
	out := make([]BesideFolder, len(roots))
	for i, r := range roots {
		out[i] = BesideFolder{Original: s.toHostPath(r.original), Copy: s.toHostPath(r.copy)}
	}
	return out
}

// containerBeside is what makes a container restore plan one next to the
// original: the copy's folders and what the check reports about it.
type containerBeside struct {
	roots []besideRoot
	view  BesidePlan
}

// prepareRestoreBeside prepares the restore of a container next to the
// original. It builds on the plan of the restore in place, which has resolved
// the snapshot, the paths it holds and the definition.
func (s *Service) prepareRestoreBeside(ctx context.Context, name, snapshotID, source string) (containerRestorePlan, error) {
	plan, err := s.prepareRestore(ctx, name, snapshotID, true, source)
	if err != nil {
		return containerRestorePlan{}, err
	}
	copyName := name + besideSuffix
	if !validResourceName(copyName) {
		return containerRestorePlan{}, destinationRefusal("%s is too long a name for a container. Restore into a folder instead", copyName)
	}
	live, err := s.docker.InspectName(ctx, copyName)
	if err != nil {
		return containerRestorePlan{}, fmt.Errorf("look for a container named %s: %w", copyName, err)
	}
	if live != "" {
		return containerRestorePlan{}, destinationRefusal("a container named %s already exists. Remove or rename it, then restore again", copyName)
	}

	inPlace := make([]backup.RestoreDir, len(plan.appdataPaths))
	for i, p := range plan.appdataPaths {
		inPlace[i] = backup.RestoreDir{Subtree: p, Target: p}
	}
	roots, dirs := besideLayout(inPlace, besideRootOf(s.cfg.DataRootSegments))
	if err := s.guardBesideRoots(roots); err != nil {
		return containerRestorePlan{}, err
	}
	if !plan.recreateOnly {
		if err := s.guardBesideRoom(ctx, repoRef{plan.repo, plan.mode}, plan.snapshotID, roots); err != nil {
			return containerRestorePlan{}, err
		}
	}

	in := plan.inspect
	hostRoots := make([]besideRoot, len(roots))
	for i, r := range roots {
		hostRoots[i] = besideRoot{original: s.toHostPath(r.original), copy: s.toHostPath(r.copy)}
	}
	binds, shared, err := besideBinds(in.HostConfig.VolumeBinds(), hostRoots)
	if err != nil {
		return containerRestorePlan{}, err
	}
	allocs, err := s.docker.Allocations(ctx)
	if err != nil {
		return containerRestorePlan{}, fmt.Errorf("read the ports in use: %w", err)
	}
	view := BesideContainer{Shared: shared, FixedIP: in.Network.IPv4Address, HostNetwork: in.HostConfig.NetworkMode == "host"}
	if policy := in.HostConfig.RestartPolicy.Name; policy != "no" {
		view.Restart = policy
	}

	in.Name = copyName
	in.HostConfig = in.HostConfig.WithVolumeBinds(binds)
	in.Mounts = slices.Clone(in.Mounts)
	for i, m := range in.Mounts {
		if moved, ok := besidePath(hostRoots, path.Clean(m.Source)); ok {
			in.Mounts[i].Source = moved
		}
	}
	in.HostConfig.PortBindings, view.Ports = besidePorts(in.HostConfig.PortBindings, allocs)
	in.HostConfig.RestartPolicy = model.RestartPolicy{Name: "no"}
	in.Network.IPv4Address, in.Network.MACAddress = "", ""
	in.Networks = slices.Clone(in.Networks)
	for i := range in.Networks {
		in.Networks[i].IPv4Address, in.Networks[i].MACAddress = "", ""
	}
	in.Config.Labels = besideLabels(in.Config.Labels, name, plan.snapshotID)

	plan.inspect = in
	plan.restoreDirs = dirs
	plan.beside = &containerBeside{
		roots: roots,
		view:  BesidePlan{Name: copyName, Folders: s.besideFolders(roots), Container: &view},
	}
	return plan, nil
}

// besideBinds points the copy's volumes at its own folders. A volume above a
// copied folder, and one the backup holds nothing of, stays where it is; the
// writable ones of those are returned as shared. It refuses a container that
// would still reach the original's files: through another path to a copied
// folder, or because no volume leads into it at all.
func besideBinds(binds []string, roots []besideRoot) (out, shared []string, err error) {
	out = make([]string, len(binds))
	shared = []string{}
	reached := make([]bool, len(roots))
	for i, b := range binds {
		out[i] = b
		host, rest, ok := strings.Cut(b, ":")
		if !ok {
			continue
		}
		_, opts, _ := strings.Cut(rest, ":")
		writable := !slices.Contains(strings.Split(opts, ","), "ro")
		if !strings.HasPrefix(host, "/") {
			if writable {
				shared = append(shared, host)
			}
			continue
		}
		h := path.Clean(host)
		moved := false
		for j, r := range roots {
			if h == r.original || isStrictDescendant(h, r.original) {
				out[i] = r.copy + strings.TrimPrefix(h, r.original) + ":" + rest
				reached[j], moved = true, true
				break
			}
			if aliasInto(h, r.original) {
				return nil, nil, destinationRefusal("the volume %s reaches %s under another path, so a copy would write into the original's files. Restore into a folder instead", h, r.original)
			}
		}
		if !moved && writable {
			shared = append(shared, h)
		}
	}
	for j, r := range roots {
		if !reached[j] {
			return nil, nil, destinationRefusal("no volume of this container leads into %s, so a copy would run on the original's files. Restore into a folder instead", r.original)
		}
	}
	return out, shared, nil
}

// aliasInto reports whether bind lies in folder through Unraid's other name
// for it: /mnt/user/<share> and /mnt/<pool>/<share> are the same folder.
func aliasInto(bind, folder string) bool {
	rf, userF, okF := shareRel(folder)
	rb, userB, okB := shareRel(bind)
	return okF && okB && (userF || userB) && (rb == rf || isStrictDescendant(rb, rf))
}

// besidePorts gives every published port of the copy the next host port
// above the original's that no container holds. A port that is no single
// number is left for Docker to pick.
func besidePorts(bindings map[string][]model.PortBinding, allocs []model.Allocation) (map[string][]model.PortBinding, []BesidePort) {
	view := []BesidePort{}
	if len(bindings) == 0 {
		return bindings, view
	}
	taken := map[string]bool{}
	for _, a := range allocs {
		for _, hp := range a.HostPorts {
			taken[hp] = true
		}
	}
	proto := func(port string) string {
		if _, p, ok := strings.Cut(port, "/"); ok && p != "" {
			return p
		}
		return "tcp"
	}
	for port, binds := range bindings {
		for _, b := range binds {
			taken[b.HostPort+"/"+proto(port)] = true
		}
	}
	out := make(map[string][]model.PortBinding, len(bindings))
	for _, port := range slices.Sorted(maps.Keys(bindings)) {
		out[port] = make([]model.PortBinding, 0, len(bindings[port]))
		for _, b := range bindings[port] {
			original := b.HostPort
			if original != "" {
				b.HostPort = ""
				if n, err := strconv.Atoi(original); err == nil {
					for p := n + 1; p <= 65535; p++ {
						key := strconv.Itoa(p) + "/" + proto(port)
						if !taken[key] {
							taken[key] = true
							b.HostPort = strconv.Itoa(p)
							break
						}
					}
				}
				view = append(view, BesidePort{Container: port, Original: original, Copy: b.HostPort})
			}
			out[port] = append(out[port], b)
		}
	}
	return out, view
}

// besideLabels marks the copy as one and takes it out of its compose
// project, where a second container of the same service would count as a
// replica the next "up" removes.
func besideLabels(labels map[string]string, original, snapshotID string) map[string]string {
	out := make(map[string]string, len(labels)+2)
	for k, v := range labels {
		if !strings.HasPrefix(k, "com.docker.compose.") {
			out[k] = v
		}
	}
	out[restoredFromLabel] = original
	out[restoredSnapshotLabel] = snapshotID
	return out
}

// executeRestoreBeside runs a prepared container restore next to the
// original, publishing "container:<name>" progress like the restore in place.
func (s *Service) executeRestoreBeside(ctx context.Context, name string, plan containerRestorePlan) error {
	unlock := s.lockDomainFor("containers", "restore")
	defer unlock()
	rkey := "container:" + name
	rctx, startedAt := s.progBegin(ctx, rkey, "restore")
	undo := func() {}
	rerr := backup.RestoreContainerBeside(rctx, backup.BesideContainerDeps{
		Original:     name,
		Copy:         plan.inspect,
		RecreateOnly: plan.recreateOnly,
		RepoPath:     plan.repo,
		SnapshotID:   plan.snapshotID,
		Dirs:         plan.restoreDirs,
		MakeDirs: func() (err error) {
			undo, err = s.makeBesideFolders(plan.beside.roots, plan.restoreDirs)
			return err
		},
		WithoutRuntime: plan.withoutRuntime,
		TargetID:       plan.targetID,
		Docker:         s.docker,
		ImagePresent: func(ctx context.Context, ref string) bool {
			id, err := s.docker.ImageID(ctx, ref)
			return err == nil && id != ""
		},
		Restic: &resticAdapter{engine: s.engine, mode: plan.mode},
		Runs:   runsAdapter{st: s.store, ctx: ctx},
	})
	if rerr == nil {
		s.healRestoreDirOwnership(rctx, plan.repo, plan.snapshotID, plan.mode, plan.restoreDirs)
	} else {
		undo()
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	return rerr
}

// vmBeside is what makes a VM restore plan one next to the original.
type vmBeside struct {
	roots  []besideRoot
	dirs   []backup.VMRestoreDir
	images []backup.VMRestoreImage
	files  []backup.BesideFile
}

// prepareRestoreVMBeside prepares the restore of a VM's disks into new
// folders next to the ones they live in. It builds on the plan of the restore
// in place, taken as a plan only, since nothing here talks to libvirt.
func (s *Service) prepareRestoreVMBeside(ctx context.Context, name, snapshotID, source string) (vmRestorePlan, error) {
	// No backup of such a VM can go next to it, so this comes before the
	// backup is looked at and has nothing to say about which one was picked.
	if tg, err := s.store.GetVMTargetByName(name); err == nil {
		if dataset, ok := firstZvolOf(tg.Definition); ok {
			return vmRestorePlan{}, destinationRefusal("%s has a disk on the ZFS volume %s, and a copy of that cannot be put into a folder next to the original. Replace the original instead.", name, dataset)
		}
	}
	plan, err := s.prepareRestoreVM(planOnly(ctx), name, snapshotID, true, source)
	if err != nil {
		return vmRestorePlan{}, err
	}
	inPlace := plan.restoreDirs
	if len(inPlace) == 0 {
		for _, p := range plan.diskPaths {
			dir := path.Dir(p)
			if !slices.ContainsFunc(inPlace, func(d backup.VMRestoreDir) bool { return d.Target == dir }) {
				inPlace = append(inPlace, backup.VMRestoreDir{Subtree: dir, Target: dir})
			}
		}
	}
	roots, dirs := besideLayout(inPlace, func(dir string) string { return dir })
	if err := s.guardBesideRoots(roots); err != nil {
		return vmRestorePlan{}, err
	}
	if err := s.guardBesideRoom(ctx, repoRef{plan.repo, plan.mode}, plan.snapshotID, roots); err != nil {
		return vmRestorePlan{}, err
	}
	b := &vmBeside{roots: roots, dirs: dirs}
	if len(plan.images) > 0 {
		b.dirs = nil
		for _, img := range plan.images {
			img.Target, _ = besidePath(roots, img.Target)
			b.images = append(b.images, img)
		}
	}
	tg, err := s.store.GetVMTargetByName(name)
	if err != nil {
		return vmRestorePlan{}, errors.New("vm has not been backed up yet")
	}
	var def vmDefinition
	if err := json.Unmarshal([]byte(tg.Definition), &def); err != nil {
		return vmRestorePlan{}, fmt.Errorf("restore vm: unmarshal definition: %w", err)
	}
	if len(def.NVRAMBytes) > 0 && def.NVRAMHostPath != "" {
		b.files = append(b.files, backup.BesideFile{Path: roots[0].copy + "/" + path.Base(def.NVRAMHostPath), Data: def.NVRAMBytes})
	}
	plan.beside = b
	return plan, nil
}

// executeRestoreVMBeside runs a prepared VM restore next to the original,
// publishing "vm:<name>" progress like the restore in place.
func (s *Service) executeRestoreVMBeside(ctx context.Context, name string, plan vmRestorePlan) error {
	unlock := s.lockDomainFor("vms", "restore")
	defer unlock()
	rkey := "vm:" + name
	rctx, startedAt := s.progBegin(ctx, rkey, "restore")
	b := plan.beside
	targets := slices.Clone(b.dirs)
	for _, img := range b.images {
		targets = append(targets, backup.VMRestoreDir{Target: path.Dir(img.Target)})
	}
	undo := func() {}
	rerr := backup.RestoreVMBeside(rctx, backup.BesideVMDeps{
		RepoPath:   plan.repo,
		SnapshotID: plan.snapshotID,
		Dirs:       b.dirs,
		MakeDirs: func() (err error) {
			undo, err = s.makeBesideFolders(b.roots, targets)
			return err
		},
		Images:      b.images,
		ImageDumper: &vmBlockRestic{engine: s.engine, mode: plan.mode},
		Convert:     qemuImgConvert,
		Files:       b.files,
		TargetID:    plan.targetID,
		Restic:      &resticAdapter{engine: s.engine, mode: plan.mode},
		Runs:        runsAdapter{st: s.store, ctx: ctx},
	})
	if rerr == nil {
		s.healRestoreDirOwnership(rctx, plan.repo, plan.snapshotID, plan.mode, b.dirs)
	} else {
		undo()
	}
	s.progEnd(rkey, "restore", rerr == nil, startedAt)
	return rerr
}

// prepareRestoreFileSetBeside prepares the restore of a file set into a new
// folder next to its source folder. It is the restore into a folder with the
// folder chosen for the person, and with the snapshot's own layout below the
// set's folder kept, so the copy looks like the original.
func (s *Service) prepareRestoreFileSetBeside(ctx context.Context, id, snapshotID, source string) (fileSetRestorePlan, error) {
	set, err := s.store.GetFileSet(id)
	if err != nil {
		return fileSetRestorePlan{}, errFileSetNotFound
	}
	if strings.TrimSpace(set.Path) == "" {
		return fileSetRestorePlan{}, fmt.Errorf("file set %q has no source path configured: restore to a folder instead, or set a path first", set.Name)
	}
	src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
	if err != nil {
		return fileSetRestorePlan{}, errors.New("invalid file set path: must be a relative subpath under the host mount")
	}
	plan, err := s.prepareRestoreFileSet(planOnly(ctx), id, snapshotID, source, path.Clean(set.Path)+besideSuffix, false)
	if err != nil {
		return fileSetRestorePlan{}, err
	}
	roots := []besideRoot{{original: src, copy: src + besideSuffix}}
	if plan.target != roots[0].copy {
		return fileSetRestorePlan{}, destinationRefusal("a copy next to %s would lie outside the Host Data mount. Restore into a folder instead", s.toHostPath(src))
	}
	if err := s.guardBesideRoots(roots); err != nil {
		return fileSetRestorePlan{}, err
	}
	if err := s.guardBesideRoom(ctx, repoRef{plan.repo, plan.mode}, plan.snapshotID, roots); err != nil {
		return fileSetRestorePlan{}, err
	}
	// A snapshot of folders inside the set keeps them at their place below the
	// copy. One taken from elsewhere has no such place and fills the copy itself.
	// A single file keeps the place of the folder it lies in.
	at := path.Clean(plan.subtree)
	if plan.file {
		at = path.Dir(at)
	}
	if inside, ok := besidePath(roots, at); ok {
		plan.target = inside
	}
	plan.beside = &roots[0]
	return plan, nil
}

// runRestoreFileSetBeside creates the copy's folder and restores into it.
// The returned undo removes the folder again.
func (s *Service) runRestoreFileSetBeside(ctx context.Context, plan fileSetRestorePlan) (undo func(), err error) {
	undo, err = s.makeBesideFolders([]besideRoot{*plan.beside}, []backup.RestoreDir{{Target: plan.target}})
	if err != nil {
		return func() {}, err
	}
	return undo, s.runRestoreFileSet(ctx, plan)
}

// besideContainerScope mirrors backup.RestoreContainerBeside.
func besideContainerScope(plan containerRestorePlan, name string) restoreScope {
	in := plan.inspect
	view := plan.beside.view
	sc := restoreScope{ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, owner: name, container: &in, beside: &view}
	for _, d := range plan.restoreDirs {
		sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: d.Subtree, Target: d.Target})
	}
	return sc
}

// besideVMScope mirrors backup.RestoreVMBeside.
func (s *Service) besideVMScope(plan vmRestorePlan) restoreScope {
	b := plan.beside
	folders := s.besideFolders(b.roots)
	sc := restoreScope{
		ref: repoRef{plan.repo, plan.mode}, snapshotID: plan.snapshotID, images: b.images,
		beside: &BesidePlan{Name: path.Base(folders[0].Copy), Folders: folders},
	}
	for _, d := range b.dirs {
		sc.steps = append(sc.steps, restic.PreviewStep{SnapshotID: plan.snapshotID, Subtree: d.Subtree, Target: d.Target})
	}
	for _, f := range b.files {
		sc.extraFiles = append(sc.extraFiles, f.Path)
	}
	return sc
}

// besideFileSetScope is fileSetScope with where the copy goes.
func (s *Service) besideFileSetScope(plan fileSetRestorePlan) restoreScope {
	sc := fileSetScope(plan)
	folders := s.besideFolders([]besideRoot{*plan.beside})
	sc.beside = &BesidePlan{Name: path.Base(folders[0].Copy), Folders: folders}
	return sc
}
