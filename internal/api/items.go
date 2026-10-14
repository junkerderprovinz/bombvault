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
// container or VM the host knows and no row exists for yet. Section is the
// domain it belongs to, and TakesSectionPlace says that its next backup goes
// where the section sends new backups rather than to a place of its own.
type itemFacts struct {
	ID                string
	Name              string
	Included          bool
	Paused            bool
	Schedule          schedule.EffectiveSchedule
	Section           string
	TakesSectionPlace bool
}

func containerFacts(t store.Target, s store.Settings) itemFacts {
	return itemFacts{
		ID:                t.ID,
		Name:              t.ContainerName,
		Included:          t.IncludeInSchedule,
		Paused:            schedule.PausedByOverride(t.ScheduleCadence, s.PerItemSchedules),
		Schedule:          schedule.EffectiveContainerSchedule(t, s),
		Section:           "containers",
		TakesSectionPlace: t.RepoChosen == store.RepoOpen,
	}
}

func vmFacts(vm store.VMTarget, s store.Settings) itemFacts {
	return itemFacts{
		ID:                vm.ID,
		Name:              vm.Name,
		Included:          vm.IncludeInSchedule,
		Paused:            schedule.PausedByOverride(vm.ScheduleCadence, s.PerItemSchedules),
		Schedule:          schedule.EffectiveVMSchedule(vm, s),
		Section:           "vms",
		TakesSectionPlace: vm.RepoChosen == store.RepoOpen,
	}
}

func fileSetFacts(set store.FileSet, s store.Settings) itemFacts {
	return itemFacts{
		ID:                set.ID,
		Name:              set.Name,
		Included:          set.Enabled,
		Paused:            schedule.PausedByOverride(set.ScheduleCadence, s.PerItemSchedules),
		Schedule:          schedule.EffectiveFileSetSchedule(set, s),
		Section:           "files",
		TakesSectionPlace: set.RepoChosen == store.RepoOpen,
	}
}

func zfsDatasetFacts(d store.ZFSDataset, s store.Settings) itemFacts {
	return itemFacts{
		ID:                d.ID,
		Name:              d.Dataset,
		Included:          d.Enabled,
		Paused:            schedule.PausedByOverride(d.ScheduleCadence, s.PerItemSchedules),
		Schedule:          schedule.EffectiveZFSDatasetSchedule(d, s),
		Section:           zfsDomain,
		TakesSectionPlace: zfsTakesSectionPlace(d),
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
	RenameFrom   string `json:"renameFrom,omitempty"`
	RenameReason string `json:"renameReason,omitempty"`
	// BackupDeclined is set while the person does not want the item backed up
	// and no schedule reaches it.
	BackupDeclined bool `json:"backupDeclined,omitempty"`
	// NoPlace is set while the item's next backup would go where its section
	// sends new backups and the section has every storage location switched
	// off for them.
	NoPlace bool `json:"noPlace,omitempty"`
	// Replica is how a ZFS item's replica stands, absent for an item that
	// replicates nowhere.
	Replica *itemReplicaView `json:"replica,omitempty"`
	Runs    []store.RunBrief `json:"runs"`
}

// itemReplicaView is the newest word on an item's replica: when it last
// succeeded and what its latest run did, "success", "failed", "running" or
// "cancelled", empty before the first.
type itemReplicaView struct {
	LastAt     int64  `json:"lastAt"`
	LastStatus string `json:"lastStatus"`
}

// itemHistory is what the runs table says about the items of one answer, with
// the items the person declined and the sections that have no place for them.
type itemHistory struct {
	stamps    map[string]store.BackupStamp
	sizes     map[string]int64
	runs      map[string][]store.RunBrief
	declined  declinedBackups
	placeless placeless
	replicas  map[string]itemReplicaView
}

func (h *Handler) readItemHistory() (itemHistory, error) {
	declined, err := h.svc.declinedBackups()
	if err != nil {
		return itemHistory{}, err
	}
	placeless, err := h.svc.placelessSections()
	if err != nil {
		return itemHistory{}, err
	}
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
	replicas, err := h.readReplicaViews("")
	if err != nil {
		return itemHistory{}, err
	}
	return itemHistory{stamps: stamps, sizes: sizes, runs: runs, declined: declined, placeless: placeless, replicas: replicas}, nil
}

// readReplicaViews reads how the replica of each ZFS item that has one
// stands, of the one item id names or of all of them for an empty id.
func (h *Handler) readReplicaViews(id string) (map[string]itemReplicaView, error) {
	items, err := h.store.ListZFSDatasets()
	if err != nil {
		return nil, err
	}
	out := map[string]itemReplicaView{}
	for _, d := range items {
		if (id != "" && d.ID != id) || d.Replica.TargetKind == store.ZFSReplicaTargetNone {
			continue
		}
		var v itemReplicaView
		if v.LastAt, err = h.store.LastZFSReplicaSuccess(d.ID); err != nil {
			return nil, err
		}
		run, ok, err := h.store.LatestZFSReplicaRun(d.ID)
		if err != nil {
			return nil, err
		}
		if ok {
			v.LastStatus = run.Status
		}
		out[d.ID] = v
	}
	return out, nil
}

// readHistoryOf is readItemHistory for one target. An item without a stored
// row has no id and no runs.
func (h *Handler) readHistoryOf(targetID string) (itemHistory, error) {
	declined, err := h.svc.declinedBackups()
	if err != nil {
		return itemHistory{}, err
	}
	placeless, err := h.svc.placelessSections()
	if err != nil || targetID == "" {
		return itemHistory{declined: declined, placeless: placeless}, err
	}
	stamps, err := h.store.LatestBackupsOf(targetID)
	if err != nil {
		return itemHistory{}, err
	}
	sizes, err := h.store.LatestSourceBytesOf(targetID)
	if err != nil {
		return itemHistory{}, err
	}
	runs, err := h.store.RecentRunsOf(targetID, itemRunStrip)
	if err != nil {
		return itemHistory{}, err
	}
	replicas, err := h.readReplicaViews(targetID)
	if err != nil {
		return itemHistory{}, err
	}
	return itemHistory{stamps: stamps, sizes: sizes, runs: runs, declined: declined, placeless: placeless, replicas: replicas}, nil
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
		BackupDeclined:    hist.declined.holds(kind, key, f.Schedule),
		NoPlace:           f.TakesSectionPlace && hist.placeless[f.Section],
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
		items = append(items, fileSetRow(hist, set, settings))
	}
	datasets, err := h.store.ListZFSDatasets()
	if err != nil {
		return nil, nil, err
	}
	for _, d := range datasets {
		items = append(items, zfsDatasetRow(hist, d, settings))
	}
	if settings.FlashEnabled {
		items = append(items, flashRow(hist, settings))
	}
	if settings.ConfigEnabled {
		items = append(items, configRow(hist, settings))
	}
	return items, unlisted, nil
}

func fileSetRow(hist itemHistory, set store.FileSet, settings store.Settings) backupItemView {
	v := hist.row("files", set.ID, fileSetFacts(set, settings))
	v.KindDisabled = !settings.FilesEnabled
	return v
}

func zfsDatasetRow(hist itemHistory, d store.ZFSDataset, settings store.Settings) backupItemView {
	v := hist.row(zfsDomain, d.ID, zfsDatasetFacts(d, settings))
	v.KindDisabled = !settings.ZFSEnabled
	if r, ok := hist.replicas[d.ID]; ok {
		v.Replica = &r
	}
	return v
}

func flashRow(hist itemHistory, settings store.Settings) backupItemView {
	return hist.row("flash", "flash", itemFacts{
		ID: store.FlashTargetID, Name: "flash", Included: true, Section: "flash", TakesSectionPlace: true,
		Schedule: schedule.EffectiveFlashSchedule(settings),
	})
}

func configRow(hist itemHistory, settings store.Settings) backupItemView {
	return hist.row("config", "config", itemFacts{
		ID: store.ConfigTargetID, Name: "config", Included: true, Section: "config", TakesSectionPlace: true,
		Schedule: schedule.EffectiveConfigSchedule(settings),
	})
}

var (
	errNoSuchItem = errors.New("no such item")
	// errHostSilent is an item no stored row names while its host did not say
	// whether it exists.
	errHostSilent = errors.New("the host did not answer, so the item cannot be looked up")
)

// handleGetItem serves GET /api/items/{kind}/{key}: the row GET /api/items has
// for one item.
func (h *Handler) handleGetItem(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	item, err := h.backupItem(r.Context(), settings, r.PathValue("kind"), r.PathValue("key"))
	switch {
	case errors.Is(err, errNoSuchItem), errors.Is(err, errUnknownItemKind):
		writeJSON(w, http.StatusNotFound, codedFailEnvelope(err, "not-found"))
	case err != nil:
		writeJSON(w, http.StatusOK, failEnvelope(err))
	default:
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"item": item}))
	}
}

// backupItem builds one row of backupItems and reads no more than that row
// needs: a folder set asks neither Docker nor libvirt, and the runs table is
// read for one target.
func (h *Handler) backupItem(ctx context.Context, settings store.Settings, kind, key string) (backupItemView, error) {
	withHistory := func(targetID string, row func(itemHistory) backupItemView) (backupItemView, error) {
		hist, err := h.readHistoryOf(targetID)
		if err != nil {
			return backupItemView{}, err
		}
		return row(hist), nil
	}
	notStored := func(err error) (backupItemView, error) {
		if errors.Is(err, sql.ErrNoRows) {
			return backupItemView{}, errNoSuchItem
		}
		return backupItemView{}, err
	}
	switch kind {
	case "container":
		return h.containerItem(ctx, settings, key)
	case "vm":
		return h.vmItem(ctx, settings, key)
	case "files":
		set, err := h.store.GetFileSet(key)
		if err != nil {
			return notStored(err)
		}
		return withHistory(set.ID, func(hist itemHistory) backupItemView { return fileSetRow(hist, set, settings) })
	case zfsDomain:
		d, err := h.store.GetZFSDataset(key)
		if err != nil {
			return notStored(err)
		}
		return withHistory(d.ID, func(hist itemHistory) backupItemView { return zfsDatasetRow(hist, d, settings) })
	case "flash":
		if key != kind || !settings.FlashEnabled {
			return backupItemView{}, errNoSuchItem
		}
		return withHistory(store.FlashTargetID, func(hist itemHistory) backupItemView { return flashRow(hist, settings) })
	case "config":
		if key != kind || !settings.ConfigEnabled {
			return backupItemView{}, errNoSuchItem
		}
		return withHistory(store.ConfigTargetID, func(hist itemHistory) backupItemView { return configRow(hist, settings) })
	}
	return backupItemView{}, errUnknownItemKind
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

// containerSources is what the container rows of one answer are built from: the
// stored entries, what Docker lists, and the facts read once for all of them.
type containerSources struct {
	settings store.Settings
	targets  []store.Target
	byName   map[string]store.Target
	docker   dockerState
	self     string
	former   aliasIndex
	shapes   map[string]string
}

func (h *Handler) readContainerSources(ctx context.Context, settings store.Settings) (containerSources, error) {
	targets, err := h.store.ListTargets()
	if err != nil {
		return containerSources{}, err
	}
	byName := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byName[t.ContainerName] = t
	}
	shapes, err := h.store.BackedUpShapes()
	if err != nil {
		log.Printf("api: items: the change notice is unavailable: %v", err)
	}
	return containerSources{
		settings: settings,
		targets:  targets,
		byName:   byName,
		docker:   h.listDocker(ctx),
		self:     h.svc.SelfContainerName(ctx),
		former:   h.formerNames("container"),
		shapes:   shapes,
	}, nil
}

// databases recognises the database servers among infos. That inspects each
// container, which is not worth it for rows the page leaves out while
// containers are not backed up.
func (src containerSources) databases(ctx context.Context, svc *Service, infos []dockercli.ContainerInfo) map[string]dbDumpRow {
	if !src.settings.ContainersEnabled {
		return nil
	}
	return svc.dbDumpRows(ctx, infos, src.byName)
}

func (src containerSources) row(hist itemHistory, t store.Target) backupItemView {
	v := hist.row("container", t.ContainerName, containerFacts(t, src.settings))
	v.Self = src.self != "" && t.ContainerName == src.self
	v.KindDisabled = !src.settings.ContainersEnabled
	v.FormerNames = src.former[t.ID]
	return v
}

// liveRow is the row of a container Docker lists, with its stored entry when
// it has one.
func (src containerSources) liveRow(ctx context.Context, svc *Service, hist itemHistory, c dockercli.ContainerInfo, databases map[string]dbDumpRow) backupItemView {
	t, stored := src.byName[c.Name]
	if !stored {
		t = store.Target{ContainerName: c.Name}
	}
	installed := true
	v := src.row(hist, t)
	v.State, v.Installed = c.State, &installed
	v.RestoredFrom = c.Labels[restoredFromLabel]
	v.IP, v.Image, v.Stack = c.IP, c.Image, c.Stack
	if db, ok := databases[c.Name]; ok {
		v.DBTier, v.DBDumpLabelOff, v.DBDataCoverage = db.Tier, db.LabelOff, db.Coverage
		v.DBDumpOff = t.DBDumpOff
		v.DBDumpsGlobalOff = !src.settings.DBDumpsEnabled
	}
	if stored && !v.Self && (v.LastBackup > 0 || src.shapes[t.ID] != "") {
		v.ChangedSinceBackup = svc.changesSinceBackup(ctx, c, t, src.shapes[t.ID])
	}
	return v
}

// goneRow is the row of a stored entry Docker does not list. Whether the
// container is gone is known only when Docker answered.
func (src containerSources) goneRow(hist itemHistory, t store.Target) backupItemView {
	v := src.row(hist, t)
	v.Image, v.Stack = storedImageAndStack(t)
	if src.docker.answered {
		gone := false
		v.Installed = &gone
	}
	return v
}

// notInstalled returns the stored entries whose container is gone, none while
// Docker did not answer.
func (src containerSources) notInstalled() []store.Target {
	if !src.docker.answered {
		return nil
	}
	var out []store.Target
	for _, t := range src.targets {
		if _, live := src.docker.live[t.ContainerName]; !live {
			out = append(out, t)
		}
	}
	return out
}

// containerItems lists the containers Docker knows, each with its stored entry
// when it has one, followed by the entries whose container is gone. listed is
// false when Docker did not answer.
func (h *Handler) containerItems(ctx context.Context, settings store.Settings, hist itemHistory) (rows []backupItemView, listed bool, err error) {
	src, err := h.readContainerSources(ctx, settings)
	if err != nil {
		return nil, false, err
	}
	infos := src.docker.infos
	databases := src.databases(ctx, h.svc, infos)

	rows = make([]backupItemView, 0, len(infos)+len(src.targets))
	hasBackup := make(map[string]bool, len(infos))
	for _, c := range infos {
		v := src.liveRow(ctx, h.svc, hist, c, databases)
		hasBackup[c.Name] = v.LastBackup > 0
		rows = append(rows, v)
	}
	for _, t := range src.targets {
		if _, live := src.docker.live[t.ContainerName]; !live {
			rows = append(rows, src.goneRow(hist, t))
		}
	}
	// The live rows stand first and in Docker's order, so a hint finds its row
	// by the container's place in that list.
	hints := h.svc.renameHints(infos, src.notInstalled(), hasBackup)
	for i, c := range infos {
		if hint, ok := hints[c.Name]; ok {
			rows[i].RenameFrom, rows[i].RenameReason = hint.OldName, hint.Reason
		}
	}
	return rows, src.docker.answered, nil
}

// containerItem is the row containerItems has for one container. Docker is
// asked for its list, which is one call and what a rename hint is matched
// against, but only this container is inspected.
func (h *Handler) containerItem(ctx context.Context, settings store.Settings, name string) (backupItemView, error) {
	src, err := h.readContainerSources(ctx, settings)
	if err != nil {
		return backupItemView{}, err
	}
	t, stored := src.byName[name]
	c, live := src.docker.live[name]
	if !stored && !live {
		if !src.docker.answered {
			return backupItemView{}, errHostSilent
		}
		return backupItemView{}, errNoSuchItem
	}
	hist, err := h.readHistoryOf(t.ID)
	if err != nil {
		return backupItemView{}, err
	}
	if !live {
		return src.goneRow(hist, t), nil
	}
	v := src.liveRow(ctx, h.svc, hist, c, src.databases(ctx, h.svc, []dockercli.ContainerInfo{c}))
	if v.LastBackup == 0 {
		if hint, ok := h.svc.suggestRenames(src.docker.infos, src.notInstalled())[name]; ok {
			v.RenameFrom, v.RenameReason = hint.OldName, hint.Reason
		}
	}
	return v, nil
}

// vmSources is containerSources for VMs. libvirt is asked only while the VMs
// kind is on, so a host without VMs is not dialled on every load.
type vmSources struct {
	settings store.Settings
	targets  []store.VMTarget
	byName   map[string]store.VMTarget
	infos    []virshcli.VMInfo
	live     map[string]virshcli.VMInfo
	answered bool
	former   aliasIndex
	trueNAS  bool
}

func (h *Handler) readVMSources(ctx context.Context, settings store.Settings) (vmSources, error) {
	targets, err := h.store.ListVMTargets()
	if err != nil {
		return vmSources{}, err
	}
	src := vmSources{
		settings: settings,
		targets:  targets,
		byName:   make(map[string]store.VMTarget, len(targets)),
		former:   h.formerNames("vm"),
		trueNAS:  h.svc.platformFn().Kind() == platform.KindTrueNAS,
	}
	for _, t := range targets {
		src.byName[t.Name] = t
	}
	if settings.VMsEnabled {
		var vErr error
		if src.infos, vErr = h.svc.virsh.List(ctx); vErr != nil {
			log.Printf("api: items: the VM list is unavailable: %v", vErr)
		}
		src.answered = vErr == nil
	}
	src.live = make(map[string]virshcli.VMInfo, len(src.infos))
	for _, vm := range src.infos {
		src.live[vm.Name] = vm
	}
	return src, nil
}

func (src vmSources) row(hist itemHistory, t store.VMTarget) backupItemView {
	v := hist.row("vm", t.Name, vmFacts(t, src.settings))
	v.KindDisabled = !src.settings.VMsEnabled
	v.FormerNames = src.former[t.ID]
	return v
}

// liveRow is the row of a VM libvirt lists. Key is the libvirt name and Name
// what the platform shows for it.
func (src vmSources) liveRow(hist itemHistory, vm virshcli.VMInfo) backupItemView {
	t, stored := src.byName[vm.Name]
	if !stored {
		t = store.VMTarget{Name: vm.Name}
	}
	installed := true
	v := src.row(hist, t)
	v.Name = vmDisplayName(vm, src.trueNAS)
	v.State, v.Installed = vm.State, &installed
	return v
}

func (src vmSources) goneRow(hist itemHistory, t store.VMTarget) backupItemView {
	v := src.row(hist, t)
	if src.answered {
		gone := false
		v.Installed = &gone
	}
	return v
}

func (src vmSources) notInstalled() []store.VMTarget {
	if !src.answered {
		return nil
	}
	var out []store.VMTarget
	for _, t := range src.targets {
		if _, live := src.live[t.Name]; !live {
			out = append(out, t)
		}
	}
	return out
}

// vmItems is containerItems for VMs. listed is false only when libvirt was
// asked and did not answer.
func (h *Handler) vmItems(ctx context.Context, settings store.Settings, hist itemHistory) (rows []backupItemView, listed bool, err error) {
	src, err := h.readVMSources(ctx, settings)
	if err != nil {
		return nil, false, err
	}
	rows = make([]backupItemView, 0, len(src.infos)+len(src.targets))
	hasBackup := make(map[string]bool, len(src.infos))
	for _, vm := range src.infos {
		v := src.liveRow(hist, vm)
		hasBackup[vm.Name] = v.LastBackup > 0
		rows = append(rows, v)
	}
	for _, t := range src.targets {
		if _, live := src.live[t.Name]; !live {
			rows = append(rows, src.goneRow(hist, t))
		}
	}
	hints := h.svc.vmRenameHints(ctx, src.infos, src.notInstalled(), hasBackup)
	for i, vm := range src.infos {
		if hint, ok := hints[vm.Name]; ok {
			rows[i].RenameFrom, rows[i].RenameReason = hint.OldName, hint.Reason
		}
	}
	return rows, src.answered || !settings.VMsEnabled, nil
}

// vmItem is the row vmItems has for one VM.
func (h *Handler) vmItem(ctx context.Context, settings store.Settings, name string) (backupItemView, error) {
	src, err := h.readVMSources(ctx, settings)
	if err != nil {
		return backupItemView{}, err
	}
	t, stored := src.byName[name]
	vm, live := src.live[name]
	if !stored && !live {
		if settings.VMsEnabled && !src.answered {
			return backupItemView{}, errHostSilent
		}
		return backupItemView{}, errNoSuchItem
	}
	hist, err := h.readHistoryOf(t.ID)
	if err != nil {
		return backupItemView{}, err
	}
	if !live {
		return src.goneRow(hist, t), nil
	}
	v := src.liveRow(hist, vm)
	if v.LastBackup == 0 {
		if hint, ok := h.svc.suggestVMRenames(ctx, src.infos, src.notInstalled(), true)[name]; ok {
			v.RenameFrom, v.RenameReason = hint.OldName, hint.Reason
		}
	}
	return v, nil
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
