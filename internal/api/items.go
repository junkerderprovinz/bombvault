package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

// itemFacts is what a stored row and the settings say about one backed-up
// item, whatever its kind. ID is the target id its runs carry, empty for a
// container or VM the host knows and no row exists for yet.
type itemFacts struct {
	ID       string
	Name     string
	Included bool
	Paused   bool
	Schedule schedule.EffectiveSchedule
}

func containerFacts(t store.Target, s store.Settings) itemFacts {
	return itemFacts{
		ID:       t.ID,
		Name:     t.ContainerName,
		Included: t.IncludeInSchedule,
		Paused:   schedule.PausedByOverride(t.ScheduleCadence, s.PerItemSchedules),
		Schedule: schedule.EffectiveContainerSchedule(t, s),
	}
}

func vmFacts(vm store.VMTarget, s store.Settings) itemFacts {
	return itemFacts{
		ID:       vm.ID,
		Name:     vm.Name,
		Included: vm.IncludeInSchedule,
		Paused:   schedule.PausedByOverride(vm.ScheduleCadence, s.PerItemSchedules),
		Schedule: schedule.EffectiveVMSchedule(vm, s),
	}
}

func fileSetFacts(set store.FileSet, s store.Settings) itemFacts {
	return itemFacts{
		ID:       set.ID,
		Name:     set.Name,
		Included: set.Enabled,
		Paused:   schedule.PausedByOverride(set.ScheduleCadence, s.PerItemSchedules),
		Schedule: schedule.EffectiveFileSetSchedule(set, s),
	}
}

func zfsDatasetFacts(d store.ZFSDataset, s store.Settings) itemFacts {
	return itemFacts{
		ID:       d.ID,
		Name:     d.Dataset,
		Included: d.Enabled,
		Paused:   schedule.PausedByOverride(d.ScheduleCadence, s.PerItemSchedules),
		Schedule: schedule.EffectiveZFSDatasetSchedule(d, s),
	}
}

// dockerState is the container list one call works from, and whether Docker
// gave it.
type dockerState struct {
	infos    []dockercli.ContainerInfo
	live     map[string]dockercli.ContainerInfo
	answered bool
}

func (h *Handler) listDocker(ctx context.Context) dockerState {
	infos, err := h.docker.List(ctx)
	if err != nil {
		log.Printf("api: items: the container list is unavailable: %v", err)
	}
	live := make(map[string]dockercli.ContainerInfo, len(infos))
	for _, c := range infos {
		live[c.Name] = c
	}
	return dockerState{infos: infos, live: live, answered: err == nil}
}

// itemRunStrip is how many of its newest runs an item of GET /api/items carries.
const itemRunStrip = 14

// backupItemView is one row of GET /api/items: a container, a VM, a folder set,
// a ZFS item, or the single flash or configuration backup.
type backupItemView struct {
	Kind string `json:"kind"`
	// Key is what the routes of the kind take: the container name, the libvirt
	// name, the id of a folder set or ZFS item, "flash" or "config".
	Key               string                     `json:"key"`
	Name              string                     `json:"name"`
	Included          bool                       `json:"included"`
	Paused            bool                       `json:"paused"`
	EffectiveSchedule schedule.EffectiveSchedule `json:"effectiveSchedule"`
	LastBackup        int64                      `json:"lastBackup"`
	LastRunStatus     string                     `json:"lastRunStatus"`
	// LastRunError is why the newest backup failed, empty unless it did.
	LastRunError string `json:"lastRunError,omitempty"`
	// SourceBytes is what the newest measured backup read, nil before the first.
	SourceBytes *int64 `json:"sourceBytes"`
	// State and Installed are what Docker or libvirt say about a container or
	// VM. Both are left out when the host was not asked or did not answer.
	State        string `json:"state,omitempty"`
	Installed    *bool  `json:"installed,omitempty"`
	Self         bool   `json:"self,omitempty"`
	KindDisabled bool   `json:"kindDisabled,omitempty"`
	// RestoredFrom names the container this one is a copy of, when a restore
	// created it next to that one.
	RestoredFrom string `json:"restoredFrom,omitempty"`
	// IP, Image and Stack are a container's address, image and compose project.
	// A container that is gone has no address, and its image and project are
	// the ones its stored definition names.
	IP    string `json:"ip,omitempty"`
	Image string `json:"image,omitempty"`
	Stack string `json:"stack,omitempty"`
	// The database fields are set for a live container recognised as a database
	// server, named and valued as on a row of GET /api/containers.
	DBTier           string `json:"dbTier,omitempty"`
	DBDumpOff        bool   `json:"dbDumpOff,omitempty"`
	DBDumpLabelOff   bool   `json:"dbDumpLabelOff,omitempty"`
	DBDumpsGlobalOff bool   `json:"dbDumpsGlobalOff,omitempty"`
	DBDataCoverage   string `json:"dbDataCoverage,omitempty"`
	// FormerNames are the names a container or VM had before, oldest link first.
	FormerNames []string `json:"formerNames,omitempty"`
	// ChangedSinceBackup is how a live container differs from its last good backup.
	ChangedSinceBackup []DefinitionChange `json:"changedSinceBackup,omitempty"`
	// RenameFrom and RenameReason name the not-installed entry a live container
	// or VM without backups of its own looks renamed from.
	RenameFrom   string           `json:"renameFrom,omitempty"`
	RenameReason string           `json:"renameReason,omitempty"`
	Runs         []store.RunBrief `json:"runs"`
}

// itemHistory is what the runs table says about every item, read once per
// listing.
type itemHistory struct {
	stamps map[string]store.BackupStamp
	sizes  map[string]int64
	runs   map[string][]store.RunBrief
}

func (h *Handler) readItemHistory() (itemHistory, error) {
	stamps, err := h.store.LatestBackupsByTarget()
	if err != nil {
		return itemHistory{}, err
	}
	sizes, err := h.store.LatestSourceBytesByTarget()
	if err != nil {
		return itemHistory{}, err
	}
	runs, err := h.store.RecentRunsByTarget(itemRunStrip)
	if err != nil {
		return itemHistory{}, err
	}
	return itemHistory{stamps: stamps, sizes: sizes, runs: runs}, nil
}

func (hist itemHistory) row(kind, key string, f itemFacts) backupItemView {
	stamp := hist.stamps[f.ID]
	v := backupItemView{
		Kind:              kind,
		Key:               key,
		Name:              f.Name,
		Included:          f.Included,
		Paused:            f.Paused,
		EffectiveSchedule: f.Schedule,
		LastBackup:        stamp.LastSuccessAt,
		LastRunStatus:     stamp.LastRunStatus,
		LastRunError:      stamp.LastRunError,
		Runs:              []store.RunBrief{},
	}
	if size, ok := hist.sizes[f.ID]; ok {
		v.SourceBytes = &size
	}
	if runs, ok := hist.runs[f.ID]; ok {
		v.Runs = runs
	}
	return v
}

// handleListItems serves GET /api/items: every item of every kind in one list.
func (h *Handler) handleListItems(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	items, unlisted, err := h.backupItems(r.Context(), settings)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"items": items, "unlisted": unlisted}))
}

// backupItems collects the rows of GET /api/items. Containers, VMs, folder sets
// and ZFS items stay listed while their kind is switched off and say so. Flash
// and the configuration backup are one item each and exist only while their
// kind is on.
//
// unlisted names the kinds whose host did not answer. Their rows are the stored
// ones alone, so a container or VM without a row is missing from them.
func (h *Handler) backupItems(ctx context.Context, settings store.Settings) (items []backupItemView, unlisted []string, err error) {
	hist, err := h.readItemHistory()
	if err != nil {
		return nil, nil, err
	}
	unlisted = []string{}
	containers, listed, err := h.containerItems(ctx, settings, hist)
	if err != nil {
		return nil, nil, err
	}
	if !listed {
		unlisted = append(unlisted, "container")
	}
	vms, listed, err := h.vmItems(ctx, settings, hist)
	if err != nil {
		return nil, nil, err
	}
	if !listed {
		unlisted = append(unlisted, "vm")
	}
	items = slices.Concat(containers, vms)

	sets, err := h.store.ListFileSets()
	if err != nil {
		return nil, nil, err
	}
	for _, set := range sets {
		v := hist.row("files", set.ID, fileSetFacts(set, settings))
		v.KindDisabled = !settings.FilesEnabled
		items = append(items, v)
	}
	datasets, err := h.store.ListZFSDatasets()
	if err != nil {
		return nil, nil, err
	}
	for _, d := range datasets {
		v := hist.row(zfsDomain, d.ID, zfsDatasetFacts(d, settings))
		v.KindDisabled = !settings.ZFSEnabled
		items = append(items, v)
	}
	if settings.FlashEnabled {
		items = append(items, hist.row("flash", "flash", itemFacts{
			ID: store.FlashTargetID, Name: "flash", Included: true,
			Schedule: schedule.EffectiveFlashSchedule(settings),
		}))
	}
	if settings.ConfigEnabled {
		items = append(items, hist.row("config", "config", itemFacts{
			ID: store.ConfigTargetID, Name: "config", Included: true,
			Schedule: schedule.EffectiveConfigSchedule(settings),
		}))
	}
	return items, unlisted, nil
}

// formerNames reads the names the entries of a kind had before. A failed read
// costs the list those names and nothing else.
func (h *Handler) formerNames(kind string) aliasIndex {
	aliases, err := h.store.ListAliases(kind)
	if err != nil {
		log.Printf("api: items: the former %s names are unavailable: %v", kind, err)
		return nil
	}
	return newAliasIndex(aliases)
}

// storedImageAndStack reads the image and the compose project out of a
// container's saved definition.
func storedImageAndStack(t store.Target) (image, stack string) {
	var def containerDefinition
	if t.Definition == "" || json.Unmarshal([]byte(t.Definition), &def) != nil {
		return "", ""
	}
	return def.Inspect.Config.Image, def.Inspect.Config.Labels["com.docker.compose.project"]
}

// containerItems lists the containers Docker knows, each with its stored entry
// when it has one, followed by the entries whose container is gone. listed is
// false when Docker did not answer.
func (h *Handler) containerItems(ctx context.Context, settings store.Settings, hist itemHistory) (rows []backupItemView, listed bool, err error) {
	targets, err := h.store.ListTargets()
	if err != nil {
		return nil, false, err
	}
	byName := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byName[t.ContainerName] = t
	}
	docker := h.listDocker(ctx)
	self := h.svc.SelfContainerName(ctx)
	former := h.formerNames("container")
	shapes, err := h.store.BackedUpShapes()
	if err != nil {
		log.Printf("api: items: the change notice is unavailable: %v", err)
	}
	// Recognising a database inspects the container, which is not worth it for
	// rows the page leaves out while containers are not backed up.
	var databases map[string]dbDumpRow
	if settings.ContainersEnabled {
		databases = h.svc.dbDumpRows(ctx, docker.infos, byName)
	}
	row := func(t store.Target) backupItemView {
		v := hist.row("container", t.ContainerName, containerFacts(t, settings))
		v.Self = self != "" && t.ContainerName == self
		v.KindDisabled = !settings.ContainersEnabled
		v.FormerNames = former[t.ID]
		return v
	}

	installed, gone := true, false
	rows = make([]backupItemView, 0, len(docker.infos)+len(targets))
	hasBackup := make(map[string]bool, len(docker.infos))
	for _, c := range docker.infos {
		t, stored := byName[c.Name]
		if !stored {
			t = store.Target{ContainerName: c.Name}
		}
		v := row(t)
		v.State, v.Installed = c.State, &installed
		v.RestoredFrom = c.Labels[restoredFromLabel]
		v.IP, v.Image, v.Stack = c.IP, c.Image, c.Stack
		if db, ok := databases[c.Name]; ok {
			v.DBTier, v.DBDumpLabelOff, v.DBDataCoverage = db.Tier, db.LabelOff, db.Coverage
			v.DBDumpOff = t.DBDumpOff
			v.DBDumpsGlobalOff = !settings.DBDumpsEnabled
		}
		hasBackup[c.Name] = v.LastBackup > 0
		if stored && !v.Self && (hasBackup[c.Name] || shapes[t.ID] != "") {
			v.ChangedSinceBackup = h.svc.changesSinceBackup(ctx, c, t, shapes[t.ID])
		}
		rows = append(rows, v)
	}
	var notInstalled []store.Target
	for _, t := range targets {
		if _, live := docker.live[t.ContainerName]; live {
			continue
		}
		v := row(t)
		v.Image, v.Stack = storedImageAndStack(t)
		if docker.answered {
			v.Installed = &gone
			notInstalled = append(notInstalled, t)
		}
		rows = append(rows, v)
	}
	// The live rows stand first and in Docker's order, so a hint finds its row
	// by the container's place in that list.
	hints := h.svc.renameHints(docker.infos, notInstalled, hasBackup)
	for i, c := range docker.infos {
		if hint, ok := hints[c.Name]; ok {
			rows[i].RenameFrom, rows[i].RenameReason = hint.OldName, hint.Reason
		}
	}
	return rows, docker.answered, nil
}

// vmItems is containerItems for VMs. libvirt is asked only while the VMs kind
// is on, so a host without VMs is not dialled on every load, and listed is
// false only when it was asked and did not answer. Key is the libvirt name and
// Name what the platform shows for it.
func (h *Handler) vmItems(ctx context.Context, settings store.Settings, hist itemHistory) (rows []backupItemView, listed bool, err error) {
	targets, err := h.store.ListVMTargets()
	if err != nil {
		return nil, false, err
	}
	byName := make(map[string]store.VMTarget, len(targets))
	for _, t := range targets {
		byName[t.Name] = t
	}
	var infos []virshcli.VMInfo
	answered := false
	if settings.VMsEnabled {
		var vErr error
		if infos, vErr = h.svc.virsh.List(ctx); vErr != nil {
			log.Printf("api: items: the VM list is unavailable: %v", vErr)
		}
		answered = vErr == nil
	}
	former := h.formerNames("vm")
	row := func(t store.VMTarget) backupItemView {
		v := hist.row("vm", t.Name, vmFacts(t, settings))
		v.KindDisabled = !settings.VMsEnabled
		v.FormerNames = former[t.ID]
		return v
	}

	trueNAS := h.svc.platformFn().Kind() == platform.KindTrueNAS
	live := make(map[string]bool, len(infos))
	hasBackup := make(map[string]bool, len(infos))
	installed, gone := true, false
	rows = make([]backupItemView, 0, len(infos)+len(targets))
	for _, vm := range infos {
		live[vm.Name] = true
		t, ok := byName[vm.Name]
		if !ok {
			t = store.VMTarget{Name: vm.Name}
		}
		v := row(t)
		v.Name = vmDisplayName(vm, trueNAS)
		v.State, v.Installed = vm.State, &installed
		hasBackup[vm.Name] = v.LastBackup > 0
		rows = append(rows, v)
	}
	var notInstalled []store.VMTarget
	for _, t := range targets {
		if live[t.Name] {
			continue
		}
		v := row(t)
		if answered {
			v.Installed = &gone
			notInstalled = append(notInstalled, t)
		}
		rows = append(rows, v)
	}
	hints := h.svc.vmRenameHints(ctx, infos, notInstalled, hasBackup)
	for i, vm := range infos {
		if hint, ok := hints[vm.Name]; ok {
			rows[i].RenameFrom, rows[i].RenameReason = hint.OldName, hint.Reason
		}
	}
	return rows, answered || !settings.VMsEnabled, nil
}

var errUnknownItemKind = errors.New("unknown item kind")

// itemTargetID resolves an item of GET /api/items to the id its runs carry.
// found is false for an item without a stored row, which has no runs either.
func (h *Handler) itemTargetID(kind, key string) (id string, found bool, err error) {
	stored := func(id string, err error) (string, bool, error) {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return id, err == nil, err
	}
	switch kind {
	case "container":
		t, err := h.store.GetTargetByContainer(key)
		return stored(t.ID, err)
	case "vm":
		vm, err := h.store.GetVMTargetByName(key)
		return stored(vm.ID, err)
	case "files":
		set, err := h.store.GetFileSet(key)
		return stored(set.ID, err)
	case zfsDomain:
		d, err := h.store.GetZFSDataset(key)
		return stored(d.ID, err)
	case "flash", "config":
		return domainRunTargetID(kind), key == kind, nil
	}
	return "", false, errUnknownItemKind
}
