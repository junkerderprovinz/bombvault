package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/paths"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// strOr returns *p or "" when p is nil.
func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// settingsView is the JSON shape for GET/PUT /api/settings.
type settingsView struct {
	EncryptionEnabled         bool   `json:"encryptionEnabled"`
	ContainersEnabled         bool   `json:"containersEnabled"`
	VMsEnabled                bool   `json:"vmsEnabled"`
	FlashEnabled              bool   `json:"flashEnabled"`
	ConfigEnabled             bool   `json:"configEnabled"`
	FilesEnabled              bool   `json:"filesEnabled"`
	ContainersPath            string `json:"containersPath"`
	VMsPath                   string `json:"vmsPath"`
	FlashPath                 string `json:"flashPath"`
	ConfigPath                string `json:"configPath"`
	FilesPath                 string `json:"filesPath"`
	RestoreFolder             string `json:"restoreFolder"`
	ContainersOffsite         string `json:"containersOffsite"`
	VMsOffsite                string `json:"vmsOffsite"`
	FlashOffsite              string `json:"flashOffsite"`
	ConfigOffsite             string `json:"configOffsite"`
	FilesOffsite              string `json:"filesOffsite"`
	ContainersOffsiteSchedule string `json:"containersOffsiteSchedule"`
	VMsOffsiteSchedule        string `json:"vmsOffsiteSchedule"`
	FlashOffsiteSchedule      string `json:"flashOffsiteSchedule"`
	ConfigOffsiteSchedule     string `json:"configOffsiteSchedule"`
	FilesOffsiteSchedule      string `json:"filesOffsiteSchedule"`
	ContainersSchedule        string `json:"containersSchedule"`
	VMsSchedule               string `json:"vmsSchedule"`
	FlashSchedule             string `json:"flashSchedule"`
	ConfigSchedule            string `json:"configSchedule"`
	FilesSchedule             string `json:"filesSchedule"`
	// Scheduled flash ZIP export: enable, destination folder (relative subpath
	// under the mount root), and how many timestamped zips to keep (0 = a single
	// overwriting flash-latest.zip).
	FlashZipExportEnabled bool   `json:"flashZipExportEnabled"`
	FlashZipExportPath    string `json:"flashZipExportPath"`
	FlashZipExportKeep    int    `json:"flashZipExportKeep"`
	DefaultLanguage       string `json:"defaultLanguage"`
	// Retention keep-policy (0 = that dimension off; all 0 = retention off).
	RetentionKeepLast    int `json:"retentionKeepLast"`
	RetentionKeepDaily   int `json:"retentionKeepDaily"`
	RetentionKeepWeekly  int `json:"retentionKeepWeekly"`
	RetentionKeepMonthly int `json:"retentionKeepMonthly"`
	// Separate off-site retention keep-policy (all 0 = off-site keeps everything).
	OffsiteRetentionKeepLast    int `json:"offsiteRetentionKeepLast"`
	OffsiteRetentionKeepDaily   int `json:"offsiteRetentionKeepDaily"`
	OffsiteRetentionKeepWeekly  int `json:"offsiteRetentionKeepWeekly"`
	OffsiteRetentionKeepMonthly int `json:"offsiteRetentionKeepMonthly"`
	// Off-site transfer bandwidth caps (KiB/s; 0 = unlimited).
	OffsiteLimitUpload   int `json:"offsiteLimitUpload"`
	OffsiteLimitDownload int `json:"offsiteLimitDownload"`
	// BackupCores caps the CPU threads each restic child uses (GOMAXPROCS);
	// 0 means every core, restic's own default.
	BackupCores int `json:"backupCores"`
	// The opt-in Prometheus /metrics endpoint and its optional bearer token.
	// Like every secret here, GET returns the token blank with MetricsTokenSet
	// reporting whether one is stored, and a blank token on PUT keeps the
	// stored one. MetricsTokenSet sits on the struct because the strict PUT
	// decoder has to accept a round-tripped GET body.
	MetricsEnabled  bool   `json:"metricsEnabled"`
	MetricsToken    string `json:"metricsToken"`
	MetricsTokenSet bool   `json:"metricsTokenSet"`
	// WidgetToken authorizes the embeddable dashboard widget, with the same
	// secret contract as MetricsToken. POST and DELETE /api/widget/token set
	// and clear it; it is part of the PUT round-trip so a full settings save
	// cannot wipe it.
	WidgetToken    string `json:"widgetToken"`
	WidgetTokenSet bool   `json:"widgetTokenSet"`
	// Scheduled restore-verification drills (restic check --read-data-subset).
	DrillsEnabled   bool   `json:"drillsEnabled"`
	DrillsSchedule  string `json:"drillsSchedule"`
	DrillsSubsetPct int    `json:"drillsSubsetPct"`
	// OffsiteDrillsEnabled gates the scheduled off-site DR drill alone; the local
	// subset check and the manual DR button are unaffected. Default on.
	OffsiteDrillsEnabled bool `json:"offsiteDrillsEnabled"`
	// RecoveryKitAck dismisses the dashboard nag once the user has downloaded +
	// safely stored the encryption-key recovery kit.
	RecoveryKitAck bool `json:"recoveryKitAck"`
	// Per-domain "off-site repo is append-only (immutable)" flags: BombVault then
	// skips its own off-site prune and refuses off-site deletes.
	ContainersOffsiteImmutable bool `json:"containersOffsiteImmutable"`
	VMsOffsiteImmutable        bool `json:"vmsOffsiteImmutable"`
	FlashOffsiteImmutable      bool `json:"flashOffsiteImmutable"`
	ConfigOffsiteImmutable     bool `json:"configOffsiteImmutable"`
	FilesOffsiteImmutable      bool `json:"filesOffsiteImmutable"`
	// Off-site growth budget in GB (0 = alarm off) + tamper-test cadence +
	// DR-drill target container/VM ('' = auto).
	OffsiteGrowthBudgetGB int    `json:"offsiteGrowthBudgetGB"`
	TamperTestSchedule    string `json:"tamperTestSchedule"`
	DRDrillTarget         string `json:"drDrillTarget"`
	DRDrillTargetVM       string `json:"drDrillTargetVm"`
	PruneImageAfterUpdate bool   `json:"pruneImageAfterUpdate"`
	// Size cap (MB) for restic's persistent cache under /config; LRU per-repo
	// eviction after scheduled runs. 0 = no limit (default 4096).
	ResticCacheMaxMB int `json:"resticCacheMaxMB"`
	// Weekly digest notification: one summary message per cadence fire through
	// the existing notify fan-out. Off by default.
	DigestEnabled  bool   `json:"digestEnabled"`
	DigestSchedule string `json:"digestSchedule"`
	// CatchUpMissed runs a scheduled backup the server slept through (it was off
	// across the scheduled fire) shortly after the next app start, anacron-style.
	// Default on.
	CatchUpMissed bool `json:"catchUpMissed"`
	// WatchdogEnabled turns on the daily overdue-backup watchdog: one push
	// notification per overdue episode through the notify channels. Default on.
	WatchdogEnabled bool `json:"watchdogEnabled"`
	// Optional age public-key encryption for the plain export paths (tool-free
	// tar.gz, xml and zip exports). Recipients are public keys (age1... or SSH),
	// so they round-trip in the clear. With encryption on and no valid recipient
	// every export fails rather than writing plaintext.
	ExportEncryptEnabled bool   `json:"exportEncryptEnabled"`
	ExportAgeRecipients  string `json:"exportAgeRecipients"`
	// ReceiverEnabled gates the read-only receiver dashboard, for a box that
	// receives immutable off-site copies and monitors them. Off by default.
	ReceiverEnabled bool `json:"receiverEnabled"`
	// RestartHealthWait makes the restart of the "stop other containers during
	// backup" set wait for each stopped dependency to become healthy (or running
	// plus a short grace without a healthcheck) before starting what depends on
	// it. The depends_on order always applies. Default on.
	// RestartHealthTimeoutSec caps that wait per container (default 120).
	RestartHealthWait       bool `json:"restartHealthWait"`
	RestartHealthTimeoutSec int  `json:"restartHealthTimeoutSec"`
	// ReconcileUnraidUpdateStatus asks Unraid to refresh its own cached "update
	// available" status after the post-backup update step recreates a container,
	// so the Docker tab's stale banner clears. It runs over the host SSH link and
	// a failure is not fatal. Default on.
	ReconcileUnraidUpdateStatus bool `json:"reconcileUnraidUpdateStatus"`
	// PerItemSchedules lets an included container or VM with a non-empty
	// scheduleCadence run on its own cadence. Off by default, which keeps the
	// domain schedule in charge of every item.
	PerItemSchedules bool `json:"perItemSchedules"`
	// RegistryAuths are the private registry credentials for the post-backup
	// update pull. Each token follows the MetricsToken contract; a host missing
	// from the list is deleted, and nil (an old client) keeps the stored list.
	RegistryAuths []registryAuthView `json:"registryAuths"`
	// FleetEnabled gates the read-only Fleet view of peer instances this box
	// polls for their protection status. Off by default.
	FleetEnabled bool `json:"fleetEnabled"`
	// PullEnabled gates fetching another instance's backups into this box's own
	// repository. Off by default, and unlike the two flags above it writes data.
	PullEnabled bool `json:"pullEnabled"`
	// InstanceName is this instance's display name, reported to polling fleet
	// peers so their Fleet page can label this box. Not a secret.
	InstanceName string `json:"instanceName"`
	// FleetToken lets other instances' Fleet views poll GET /api/fleet/status,
	// with the same secret contract as WidgetToken. POST and DELETE
	// /api/fleet/token set and clear it.
	FleetToken    string `json:"fleetToken"`
	FleetTokenSet bool   `json:"fleetTokenSet"`
	// EverythingSchedule is the cadence of the "Backup Everything" pass over all
	// domains; 'off', the default, leaves it inert.
	EverythingSchedule string `json:"everythingSchedule"`
	// The hooks run in BombVault's own container before and after the whole
	// pass. They are never echoed, because a useful hook often carries a secret
	// in its URL (a healthchecks.io ping is a UUID) and without a login password
	// anyone on the LAN can read this. The ...Set flags report presence, a blank
	// field on PUT keeps the stored command, and the ...Clear flags remove one,
	// since blank alone could never get rid of it.
	EverythingPreHook       string `json:"everythingPreHook"`
	EverythingPostHook      string `json:"everythingPostHook"`
	EverythingPreHookSet    bool   `json:"everythingPreHookSet"`
	EverythingPostHookSet   bool   `json:"everythingPostHookSet"`
	EverythingPreHookClear  bool   `json:"everythingPreHookClear"`
	EverythingPostHookClear bool   `json:"everythingPostHookClear"`
}

// registryAuthView is one container registry credential in the settings view.
// Token is write-only; TokenSet sits on the struct because the strict PUT
// decoder has to accept a round-tripped GET body.
type registryAuthView struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Token    string `json:"token"`
	TokenSet bool   `json:"tokenSet"`
}

func toView(s store.Settings) settingsView {
	return settingsView{
		EncryptionEnabled: s.EncryptionEnabled,
		ContainersEnabled: s.ContainersEnabled,
		VMsEnabled:        s.VMsEnabled,
		FlashEnabled:      s.FlashEnabled,
		ConfigEnabled:     s.ConfigEnabled,
		FilesEnabled:      s.FilesEnabled,
		ContainersPath:    s.ContainersPath,
		VMsPath:           s.VMsPath,
		FlashPath:         s.FlashPath,
		ConfigPath:        s.ConfigPath,
		FilesPath:         s.FilesPath,
		RestoreFolder:     s.RestoreFolder,
		// Verbatim: the credentialed export, gated on a login password, needs a
		// complete copy. The other exits scrub on their own, through
		// redactExportLocations and scrubGetSettingsSecrets.
		ContainersOffsite:           s.ContainersOffsite,
		VMsOffsite:                  s.VMsOffsite,
		FlashOffsite:                s.FlashOffsite,
		ConfigOffsite:               s.ConfigOffsite,
		FilesOffsite:                s.FilesOffsite,
		ContainersOffsiteSchedule:   s.ContainersOffsiteSchedule,
		VMsOffsiteSchedule:          s.VMsOffsiteSchedule,
		FlashOffsiteSchedule:        s.FlashOffsiteSchedule,
		ConfigOffsiteSchedule:       s.ConfigOffsiteSchedule,
		FilesOffsiteSchedule:        s.FilesOffsiteSchedule,
		ContainersSchedule:          s.ContainersSchedule,
		VMsSchedule:                 s.VMsSchedule,
		FlashSchedule:               s.FlashSchedule,
		ConfigSchedule:              s.ConfigSchedule,
		FilesSchedule:               s.FilesSchedule,
		FlashZipExportEnabled:       s.FlashZipExportEnabled,
		FlashZipExportPath:          s.FlashZipExportPath,
		FlashZipExportKeep:          s.FlashZipExportKeep,
		DefaultLanguage:             s.DefaultLanguage,
		RetentionKeepLast:           s.RetentionKeepLast,
		RetentionKeepDaily:          s.RetentionKeepDaily,
		RetentionKeepWeekly:         s.RetentionKeepWeekly,
		RetentionKeepMonthly:        s.RetentionKeepMonthly,
		OffsiteRetentionKeepLast:    s.OffsiteRetentionKeepLast,
		OffsiteRetentionKeepDaily:   s.OffsiteRetentionKeepDaily,
		OffsiteRetentionKeepWeekly:  s.OffsiteRetentionKeepWeekly,
		OffsiteRetentionKeepMonthly: s.OffsiteRetentionKeepMonthly,
		OffsiteLimitUpload:          s.OffsiteLimitUpload,
		OffsiteLimitDownload:        s.OffsiteLimitDownload,
		BackupCores:                 s.BackupCores,
		MetricsEnabled:              s.MetricsEnabled,
		MetricsToken:                "", // secret, never echoed; MetricsTokenSet reports presence
		MetricsTokenSet:             s.MetricsToken != "",
		WidgetToken:                 "", // secret, never echoed; WidgetTokenSet reports presence
		WidgetTokenSet:              s.WidgetToken != "",
		DrillsEnabled:               s.DrillsEnabled,
		DrillsSchedule:              s.DrillsSchedule,
		DrillsSubsetPct:             s.DrillsSubsetPct,
		OffsiteDrillsEnabled:        s.OffsiteDrillsEnabled,
		RecoveryKitAck:              s.RecoveryKitAck,
		ContainersOffsiteImmutable:  s.ContainersOffsiteImmutable,
		VMsOffsiteImmutable:         s.VMsOffsiteImmutable,
		FlashOffsiteImmutable:       s.FlashOffsiteImmutable,
		ConfigOffsiteImmutable:      s.ConfigOffsiteImmutable,
		FilesOffsiteImmutable:       s.FilesOffsiteImmutable,
		OffsiteGrowthBudgetGB:       s.OffsiteGrowthBudgetGB,
		TamperTestSchedule:          s.TamperTestSchedule,
		DRDrillTarget:               s.DRDrillTarget,
		DRDrillTargetVM:             s.DRDrillTargetVM,
		PruneImageAfterUpdate:       s.PruneImageAfterUpdate,
		ResticCacheMaxMB:            s.ResticCacheMaxMB,
		DigestEnabled:               s.DigestEnabled,
		DigestSchedule:              s.DigestSchedule,
		CatchUpMissed:               s.CatchUpMissed,
		WatchdogEnabled:             s.WatchdogEnabled,
		ReconcileUnraidUpdateStatus: s.ReconcileUnraidUpdateStatus,
		ExportEncryptEnabled:        s.ExportEncryptEnabled,
		ExportAgeRecipients:         s.ExportAgeRecipients,
		ReceiverEnabled:             s.ReceiverEnabled,
		RestartHealthWait:           s.RestartHealthWait,
		RestartHealthTimeoutSec:     s.RestartHealthTimeoutSec,
		PerItemSchedules:            s.PerItemSchedules,
		FleetEnabled:                s.FleetEnabled,
		PullEnabled:                 s.PullEnabled,
		InstanceName:                s.InstanceName,
		FleetToken:                  "", // secret, never echoed; FleetTokenSet reports presence
		FleetTokenSet:               s.FleetToken != "",
		EverythingSchedule:          s.EverythingSchedule,
		EverythingPreHook:           s.EverythingPreHook,
		EverythingPostHook:          s.EverythingPostHook,
		EverythingPreHookSet:        s.EverythingPreHook != "",
		EverythingPostHookSet:       s.EverythingPostHook != "",
	}
}

// clampHealthTimeoutSec keeps the restart health-wait timeout between 5 seconds
// and an hour, so a typo cannot let one stuck dependency block a restart for
// days. A non-positive value falls back to the 120s default.
func clampHealthTimeoutSec(sec int) int {
	if sec <= 0 {
		return 120
	}
	return min(3600, max(5, sec))
}

// scrubGetSettingsSecrets applies GET /api/settings' own policy to the shared
// view, which without a login password any host on the LAN can read.
//
// Off-site locations are scrubbed rather than blanked: a location is not a
// secret, and the wizard's backend inference and cron snippet need it, but it
// can carry one (rest:https://user:pass@host/repo). A location that comes back
// on PUT with the marker keeps the stored one. Hooks are blanked, because a
// hook's whole value is often the secret; the ...Set flags report presence.
func scrubGetSettingsSecrets(v settingsView) settingsView {
	v.ContainersOffsite = scrubRepoLocation(v.ContainersOffsite)
	v.VMsOffsite = scrubRepoLocation(v.VMsOffsite)
	v.FlashOffsite = scrubRepoLocation(v.FlashOffsite)
	v.ConfigOffsite = scrubRepoLocation(v.ConfigOffsite)
	v.FilesOffsite = scrubRepoLocation(v.FilesOffsite)
	v.EverythingPreHook = ""
	v.EverythingPostHook = ""
	return v
}

func (h *Handler) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	view := scrubGetSettingsSecrets(toView(s))
	// Registry credentials are stored encrypted, which toView cannot decode.
	// Tokens are never echoed; TokenSet reports presence.
	regs, err := h.svc.decodeRegistryAuths(s)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	view.RegistryAuths = make([]registryAuthView, 0, len(regs))
	for _, a := range regs {
		view.RegistryAuths = append(view.RegistryAuths, registryAuthView{
			Host: a.Host, Username: a.Username, TokenSet: a.Token != "",
		})
	}
	// Nested under "settings" so a client can GET, edit and PUT back the same
	// object; hostMountRoot and platform sit beside it so the strict PUT
	// decoder never sees them. platform is the detected or overridden
	// platform.Kind ("unraid", "generic", "truenas") and cannot be changed here.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"settings":      view,
		"hostMountRoot": h.cfg.HostMountRoot,
		"platform":      string(h.svc.platformFn().Kind()),
	})
}

// rejectEveryNSchedules returns the user-facing error for a view whose off-site
// replication cadence uses "everyN", or "" when it is acceptable. everyN is a
// daily trigger plus a "has the interval elapsed?" gate, and a replication job
// has no last run to answer that with, so it would fire daily.
//
// Every settings write path calls it, handlePutSettings and validateExport
// alike, because the UI always PUTs the full object and one bad imported field
// would break every later save. The domain, drill, tamper-test, digest and
// Everything schedules record their last run, so everyN works for them. The
// scheduler also refuses to register an everyN it cannot enforce; this is the
// friendlier check at save time.
func rejectEveryNSchedules(v settingsView) string {
	for _, cad := range []string{
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
	} {
		if c, _ := schedule.ParseCadence(cad); c.IntervalDays > 0 {
			return "this schedule does not support 'everyN': use 'daily HH:MM', 'weekly DOW HH:MM', or a cron expression"
		}
	}
	return ""
}

// rejectSettingsPathOnNamedRepo refuses a settings save that moves a domain's
// own repository or an off-site destination onto a location a named repository
// already occupies. It returns a user-facing sentence, or "".
//
// Only the fields this save changes, compared with cur, are checked. The SPA
// sends the whole settings object from every card, so checking everything would
// refuse every save over an existing collision the operator is not touching. A
// failed store read refuses the save, since it can simply be repeated.
func (h *Handler) rejectSettingsPathOnNamedRepo(v settingsView, cur store.Settings) string {
	rows, err := h.store.ListNamedRepos()
	if err != nil {
		return "could not check this path against the repositories you set up; try again"
	}
	if len(rows) == 0 {
		return ""
	}
	for _, f := range []struct{ label, loc, was string }{
		{"Containers", v.ContainersPath, cur.ContainersPath},
		{"VMs", v.VMsPath, cur.VMsPath},
		{"Flash", v.FlashPath, cur.FlashPath},
		{"Config", v.ConfigPath, cur.ConfigPath},
		{"Folders", v.FilesPath, cur.FilesPath},
		{"Containers off-site", v.ContainersOffsite, cur.ContainersOffsite},
		{"VMs off-site", v.VMsOffsite, cur.VMsOffsite},
		{"Flash off-site", v.FlashOffsite, cur.FlashOffsite},
		{"Config off-site", v.ConfigOffsite, cur.ConfigOffsite},
		{"Folders off-site", v.FilesOffsite, cur.FilesOffsite},
	} {
		if strings.TrimSpace(f.loc) == "" {
			continue
		}
		if sameRepoLocation(strings.TrimSpace(f.loc), strings.TrimSpace(f.was)) {
			continue // unchanged by this save
		}
		loc, rErr := h.svc.resolveRepo(f.loc)
		if rErr != nil {
			continue // rejectInvalidSettingsPaths already refused what cannot resolve
		}
		for _, r := range rows {
			other, oErr := h.svc.resolveRepo(r.Repo)
			if oErr != nil || !repoLocationsOverlap(other, loc) {
				continue
			}
			return fmt.Sprintf("the %s path is, holds or lies inside the repository %q you set up under Repositories; pick a different folder, or remove that repository first", f.label, r.Name)
		}
	}
	return ""
}

// rejectNestedSettingsPath refuses a save that moves a domain path or an
// off-site field into or around another repository or target. Like
// rejectSettingsPathOnNamedRepo it checks only the fields this save changes.
func (h *Handler) rejectNestedSettingsPath(v settingsView, cur store.Settings) string {
	next := cur
	next.ContainersPath, next.VMsPath, next.FlashPath, next.ConfigPath, next.FilesPath =
		v.ContainersPath, v.VMsPath, v.FlashPath, v.ConfigPath, v.FilesPath
	next.ContainersOffsite, next.VMsOffsite, next.FlashOffsite, next.ConfigOffsite, next.FilesOffsite =
		v.ContainersOffsite, v.VMsOffsite, v.FlashOffsite, v.ConfigOffsite, v.FilesOffsite
	for _, f := range []struct {
		label, domain, loc, was string
		field                   bool
	}{
		{"Containers", "containers", v.ContainersPath, cur.ContainersPath, false},
		{"VMs", "vms", v.VMsPath, cur.VMsPath, false},
		{"Flash", "flash", v.FlashPath, cur.FlashPath, false},
		{"Config", "config", v.ConfigPath, cur.ConfigPath, false},
		{"Folders", "files", v.FilesPath, cur.FilesPath, false},
		{"Containers off-site", "containers", v.ContainersOffsite, cur.ContainersOffsite, true},
		{"VMs off-site", "vms", v.VMsOffsite, cur.VMsOffsite, true},
		{"Flash off-site", "flash", v.FlashOffsite, cur.FlashOffsite, true},
		{"Config off-site", "config", v.ConfigOffsite, cur.ConfigOffsite, true},
		{"Folders off-site", "files", v.FilesOffsite, cur.FilesOffsite, true},
	} {
		if strings.TrimSpace(f.loc) == "" || sameRepoLocation(strings.TrimSpace(f.loc), strings.TrimSpace(f.was)) {
			continue
		}
		loc, err := h.svc.resolveRepo(f.loc)
		if err != nil {
			continue // rejectInvalidSettingsPaths already refused what cannot resolve
		}
		self := locationSelf{own: f.domain}
		if f.field {
			self = locationSelf{field: f.domain}
			row, ok, err := h.store.FieldOffsiteTarget(f.domain)
			if err != nil {
				return "could not check this path against the off-site targets; try again"
			}
			if ok {
				self.ids = []string{row.ID}
			}
		}
		if err := h.svc.locationClash(next, loc, self); err != nil {
			return fmt.Sprintf("the %s path: %s", f.label, scrubError(err))
		}
	}
	return ""
}

// rejectInvalidSettingsPaths validates every repo location a settings row
// carries: the restore folder is always local, a remote backend (rclone:/s3:/
// rest:/sftp:/b2:) is accepted verbatim, an unprefixed remote-looking value is
// refused with guidance, and a local path must resolve under the mount root.
// Returns a user-facing message, or "" when the whole set is acceptable. Both
// settings write paths share it, for the reason rejectEveryNSchedules gives.
func rejectInvalidSettingsPaths(v settingsView, mountRoot string) string {
	// Restores land on the local mount root. A remote-looking value such as
	// "s3:foo" would slip past the containment check below, which skips remotes.
	if v.RestoreFolder != "" && restic.IsRemoteRepo(v.RestoreFolder) {
		return "restore folder must be a local path under the mount root"
	}

	// A blank off-site field means none.
	for _, sub := range []string{
		v.ContainersPath, v.VMsPath, v.FlashPath, v.ConfigPath, v.FilesPath, v.RestoreFolder,
		v.ContainersOffsite, v.VMsOffsite, v.FlashOffsite, v.ConfigOffsite, v.FilesOffsite,
	} {
		if sub == "" || restic.IsRemoteRepo(sub) {
			continue
		}
		// A "word:" prefix that is not a known remote is almost always a
		// mistyped off-site path ("BackBlaze:bucket" for
		// "rclone:BackBlaze:bucket"), not a local folder of that name.
		if restic.LooksLikeUnprefixedRemote(sub) {
			return fmt.Sprintf("%q looks like a remote backend but is missing its prefix; off-site backends need one of rclone:/s3:/rest:/sftp:/b2:, for example rclone:%s", sub, sub)
		}
		if _, err := paths.Resolve(mountRoot, sub); err != nil {
			log.Printf("api: settings: rejected path %q: %v", sub, err)
			return "invalid backup path: must be a relative subpath under the mount root, or an rclone:/s3: remote"
		}
	}
	return ""
}

// rejectInvalidSettingsNames validates the DR-drill targets, which are
// container and VM names from the UI dropdown, with the same rules as the
// name-keyed routes. Both settings write paths share it.
func rejectInvalidSettingsNames(v settingsView) string {
	if dt := strings.TrimSpace(v.DRDrillTarget); dt != "" && !validResourceName(dt) {
		return "invalid DR-drill target"
	}
	// VM names may contain spaces ("Windows 11").
	if dt := strings.TrimSpace(v.DRDrillTargetVM); dt != "" && !validVMName(dt) {
		return "invalid DR-drill target"
	}
	return ""
}

func (h *Handler) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var v settingsView
	if !decodeBody(w, r, &v) {
		return
	}

	// The same guard the import path applies, so a value one path refuses
	// cannot arrive through the other.
	if msg := rejectInvalidSettingsPaths(v, h.cfg.HostMountRoot); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	// validateNamedRepo keeps a named repository off a domain's own path; this
	// is the other direction, a domain path moved onto a named repository,
	// which would leave a row answering for the domain with its own empty
	// credentials and an append-only flag the domain never set.
	cur, curErr := h.store.GetSettings()
	if curErr != nil {
		writeJSON(w, http.StatusOK, failEnvelope(curErr))
		return
	}
	if msg := h.rejectSettingsPathOnNamedRepo(v, cur); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if msg := h.rejectNestedSettingsPath(v, cur); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}

	for _, cad := range []string{
		v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule, v.ConfigSchedule, v.FilesSchedule,
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
		v.DrillsSchedule, v.TamperTestSchedule, v.DigestSchedule, v.EverythingSchedule,
	} {
		if _, err := schedule.ParseCadence(cad); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": false, "error": scrubError(err),
			})
			return
		}
	}
	if msg := rejectEveryNSchedules(v); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": msg,
		})
		return
	}

	if msg := rejectInvalidSettingsNames(v); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}

	// This snapshot only spots the VMs domain being switched on. Nothing written
	// back may come from it: the SSH test below can burn its whole timeout, so
	// by the time of the write it may be minutes old and would revert another
	// save. What is kept is read inside the transaction.
	existing, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	// Enabling the VMs domain needs a working SSH connection to the host, or the
	// tab would appear with nothing able to back up. It is checked only when the
	// domain is switched on, so a brief host outage does not block other saves.
	if v.VMsEnabled && !existing.VMsEnabled {
		if tErr := h.svc.VMSSHTest(r.Context()); tErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": "Can't enable VM backup yet: " + scrubError(tErr) + ". Set up the SSH key under “VM Backup over SSH” and click Test connection first.",
			})
			return
		}
	}

	// mergeRegistryAuths refuses malformed input with a message the client has
	// to get verbatim, since a registry host can contain "/" and failEnvelope
	// would turn it into "[path]". It is carried out of the callback for that.
	var registryInputErr error

	// The form's own fields are assigned one by one onto the current row inside
	// the transaction, never as a whole struct literal, so a column this form
	// does not own keeps its stored value, and the auth hash and session epoch
	// come from the row as it is now rather than from the stale snapshot.
	before := h.svc.fieldTargets()
	s, err := h.store.MutateSettings(func(cur *store.Settings) error {
		cur.EncryptionEnabled = v.EncryptionEnabled
		cur.ContainersEnabled = v.ContainersEnabled
		cur.VMsEnabled = v.VMsEnabled
		cur.FlashEnabled = v.FlashEnabled
		cur.ConfigEnabled = v.ConfigEnabled
		cur.FilesEnabled = v.FilesEnabled
		cur.ContainersPath = v.ContainersPath
		cur.VMsPath = v.VMsPath
		cur.FlashPath = v.FlashPath
		cur.ConfigPath = v.ConfigPath
		cur.FilesPath = v.FilesPath
		cur.RestoreFolder = v.RestoreFolder
		// GET hands these locations out with any credential replaced by the
		// redaction marker, so a location that still carries it keeps the stored
		// one, as the settings import does. Otherwise the next unrelated save
		// would destroy the stored password.
		keepLocation := func(incoming, stored string) string {
			if locationRedacted(incoming) {
				return stored
			}
			return incoming
		}
		cur.ContainersOffsite = keepLocation(v.ContainersOffsite, cur.ContainersOffsite)
		cur.VMsOffsite = keepLocation(v.VMsOffsite, cur.VMsOffsite)
		cur.FlashOffsite = keepLocation(v.FlashOffsite, cur.FlashOffsite)
		cur.ConfigOffsite = keepLocation(v.ConfigOffsite, cur.ConfigOffsite)
		cur.FilesOffsite = keepLocation(v.FilesOffsite, cur.FilesOffsite)
		cur.ContainersOffsiteSchedule = v.ContainersOffsiteSchedule
		cur.VMsOffsiteSchedule = v.VMsOffsiteSchedule
		cur.FlashOffsiteSchedule = v.FlashOffsiteSchedule
		cur.ConfigOffsiteSchedule = v.ConfigOffsiteSchedule
		cur.FilesOffsiteSchedule = v.FilesOffsiteSchedule
		cur.ContainersSchedule = v.ContainersSchedule
		cur.VMsSchedule = v.VMsSchedule
		cur.FlashSchedule = v.FlashSchedule
		cur.ConfigSchedule = v.ConfigSchedule
		cur.FilesSchedule = v.FilesSchedule
		cur.FlashZipExportEnabled = v.FlashZipExportEnabled
		cur.FlashZipExportPath = v.FlashZipExportPath
		cur.FlashZipExportKeep = max(0, v.FlashZipExportKeep)
		cur.DefaultLanguage = v.DefaultLanguage
		cur.RetentionKeepLast = max(0, v.RetentionKeepLast)
		cur.RetentionKeepDaily = max(0, v.RetentionKeepDaily)
		cur.RetentionKeepWeekly = max(0, v.RetentionKeepWeekly)
		cur.RetentionKeepMonthly = max(0, v.RetentionKeepMonthly)
		cur.OffsiteRetentionKeepLast = max(0, v.OffsiteRetentionKeepLast)
		cur.OffsiteRetentionKeepDaily = max(0, v.OffsiteRetentionKeepDaily)
		cur.OffsiteRetentionKeepWeekly = max(0, v.OffsiteRetentionKeepWeekly)
		cur.OffsiteRetentionKeepMonthly = max(0, v.OffsiteRetentionKeepMonthly)
		cur.OffsiteLimitUpload = max(0, v.OffsiteLimitUpload)
		cur.OffsiteLimitDownload = max(0, v.OffsiteLimitDownload)
		// A number above the machine's thread count caps nothing. 0 means every
		// core.
		cur.BackupCores = min(max(0, v.BackupCores), runtime.NumCPU())
		cur.MetricsEnabled = v.MetricsEnabled
		cur.DrillsEnabled = v.DrillsEnabled
		cur.DrillsSchedule = v.DrillsSchedule
		cur.DrillsSubsetPct = max(1, min(100, v.DrillsSubsetPct))
		cur.OffsiteDrillsEnabled = v.OffsiteDrillsEnabled
		cur.RecoveryKitAck = v.RecoveryKitAck
		cur.ContainersOffsiteImmutable = v.ContainersOffsiteImmutable
		cur.VMsOffsiteImmutable = v.VMsOffsiteImmutable
		cur.FlashOffsiteImmutable = v.FlashOffsiteImmutable
		cur.ConfigOffsiteImmutable = v.ConfigOffsiteImmutable
		cur.FilesOffsiteImmutable = v.FilesOffsiteImmutable
		cur.OffsiteGrowthBudgetGB = max(0, v.OffsiteGrowthBudgetGB)
		cur.TamperTestSchedule = v.TamperTestSchedule
		cur.DRDrillTarget = strings.TrimSpace(v.DRDrillTarget)
		cur.DRDrillTargetVM = strings.TrimSpace(v.DRDrillTargetVM)
		cur.PruneImageAfterUpdate = v.PruneImageAfterUpdate
		cur.ResticCacheMaxMB = max(0, v.ResticCacheMaxMB)
		cur.DigestEnabled = v.DigestEnabled
		cur.DigestSchedule = v.DigestSchedule
		cur.CatchUpMissed = v.CatchUpMissed
		cur.WatchdogEnabled = v.WatchdogEnabled
		cur.ReconcileUnraidUpdateStatus = v.ReconcileUnraidUpdateStatus
		cur.ExportEncryptEnabled = v.ExportEncryptEnabled
		cur.ExportAgeRecipients = strings.TrimSpace(v.ExportAgeRecipients)
		cur.ReceiverEnabled = v.ReceiverEnabled
		cur.RestartHealthWait = v.RestartHealthWait
		cur.RestartHealthTimeoutSec = clampHealthTimeoutSec(v.RestartHealthTimeoutSec)
		cur.PerItemSchedules = v.PerItemSchedules
		cur.FleetEnabled = v.FleetEnabled
		cur.PullEnabled = v.PullEnabled
		cur.InstanceName = strings.TrimSpace(v.InstanceName)
		cur.EverythingSchedule = v.EverythingSchedule
		// Blank keeps the stored command, like the tokens below: GET never
		// echoes a hook, so every card submits blanks, and removing one needs
		// the ...Clear flag.
		switch {
		case v.EverythingPreHookClear:
			cur.EverythingPreHook = ""
		case strings.TrimSpace(v.EverythingPreHook) != "":
			cur.EverythingPreHook = strings.TrimSpace(v.EverythingPreHook)
		}
		switch {
		case v.EverythingPostHookClear:
			cur.EverythingPostHook = ""
		case strings.TrimSpace(v.EverythingPostHook) != "":
			cur.EverythingPostHook = strings.TrimSpace(v.EverythingPostHook)
		}

		// Blank keeps the stored token. It is read inside the transaction, so a
		// token minted by POST /api/{widget,fleet}/token while the form was open
		// is kept, not reverted.
		if t := strings.TrimSpace(v.MetricsToken); t != "" {
			cur.MetricsToken = t
		}
		if t := strings.TrimSpace(v.WidgetToken); t != "" {
			cur.WidgetToken = t
		}
		if t := strings.TrimSpace(v.FleetToken); t != "" {
			cur.FleetToken = t
		}
		// nil (an old client) keeps the stored registry list. A present list
		// replaces it, a blank token taking the stored one for its host, read
		// here for the same reason as the tokens above. Decoding and encoding
		// touch no store, so both are safe inside the transaction.
		if v.RegistryAuths != nil {
			stored, dErr := h.svc.decodeRegistryAuths(*cur)
			if dErr != nil {
				return dErr
			}
			merged, mErr := mergeRegistryAuths(v.RegistryAuths, stored)
			if mErr != nil {
				registryInputErr = mErr
				return mErr
			}
			blob, eErr := h.svc.EncodeRegistryAuths(merged)
			if eErr != nil {
				return eErr
			}
			cur.RegistryAuths = blob
		}
		return nil
	})
	if registryInputErr != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": registryInputErr.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// The replication path reads each domain's primary offsite_targets row, so
	// the saved off-site config is mirrored there; settings stay the source for
	// the fallback path.
	h.svc.syncAllPrimaryOffsiteTargets(s)
	// The CPU cap reaches restic through the environment of the next child it
	// starts, so it applies without a restart; a running backup keeps its value.
	restic.SetMaxProcs(s.BackupCores)
	if err := h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun, h.configLastRun, h.filesLastRun, h.everythingLastRun); err != nil {
		// The settings are saved, but the scheduler could not re-register.
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": scrubError(err)})
		return
	}
	// An immutable off-site repo with an off-site retention policy gets a note,
	// not a failure: BombVault never prunes an append-only repo, so the policy
	// does nothing until the far side enforces it.
	notes := []string{}
	if (s.ContainersOffsiteImmutable || s.VMsOffsiteImmutable || s.FlashOffsiteImmutable || s.ConfigOffsiteImmutable || s.FilesOffsiteImmutable) &&
		(s.OffsiteRetentionKeepLast > 0 || s.OffsiteRetentionKeepDaily > 0 ||
			s.OffsiteRetentionKeepWeekly > 0 || s.OffsiteRetentionKeepMonthly > 0) {
		notes = append(notes, "The off-site repo is append-only (immutable), so BombVault will not apply the off-site retention policy; enforce retention far-side (e.g. a rest-server prune cron) or use a maintenance window.")
	}
	warnings := []saveWarning{}
	after := h.svc.fieldTargets()
	for _, d := range offsiteConfigDomains {
		b, hadRow := before[d]
		a, hasRow := after[d]
		if hadRow && hasRow {
			warnings = append(warnings, h.svc.targetSaveWarnings(r.Context(), b, a)...)
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"warnings": warnings, "notes": notes}))
}

// handleRecoveryKit streams the encryption-key recovery kit as a download.
// GET /api/recovery-kit
//
// Besides authGate it requires a login password to be set: the kit holds the
// APP_KEY, the derived restic password and the off-site credentials, and
// decrypts every repo for good, including the append-only off-site archives
// meant to survive a host compromise. The body carries the real repo
// locations, unscrubbed, and is never logged.
func (h *Handler) handleRecoveryKit(w http.ResponseWriter, _ *http.Request) {
	if !h.requireAuthForSecrets(w, "downloading the recovery kit") {
		return
	}
	kit, err := h.svc.RecoveryKit()
	if err != nil {
		// A build failure (settings read) is reported as JSON before any body is
		// streamed; the secret body is never logged.
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="bombvault-recovery-kit.md"`)
	w.WriteHeader(http.StatusOK)
	if _, wErr := w.Write([]byte(kit)); wErr != nil {
		// Log only the failure, never the body (it contains the master key).
		log.Printf("api: recovery-kit: write failed: %v", wErr)
	}
}

// handleRecoveryKitAck records that the user has stored the recovery kit, which
// dismisses the dashboard nag. It changes that one flag through
// MutateSettings, so a settings change made elsewhere meanwhile survives.
// POST /api/recovery-kit/ack
func (h *Handler) handleRecoveryKitAck(w http.ResponseWriter, _ *http.Request) {
	if _, err := h.store.MutateSettings(func(s *store.Settings) error {
		s.RecoveryKitAck = true
		return nil
	}); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleRcloneInfo returns the configured rclone remote names (never secrets).
// GET /api/rclone
func (h *Handler) handleRcloneInfo(w http.ResponseWriter, _ *http.Request) {
	remotes, err := h.svc.RcloneRemotes()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if remotes == nil {
		remotes = []string{}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"remotes": remotes}))
}

// handleSetRclone stores the rclone config (encrypted) and writes the on-disk
// file. An empty conf clears it. POST /api/rclone  body {conf}
func (h *Handler) handleSetRclone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Conf string `json:"conf"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.svc.SetRcloneConf(body.Conf); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGetNotify returns the notification config without the stored
// credentials: the SMTP password and Matrix access token are blanked and
// reported through "is-set" flags, like the cloud credentials. GET /api/notify
func (h *Handler) handleGetNotify(w http.ResponseWriter, _ *http.Request) {
	c, err := h.svc.NotifyConfig()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	smtpPasswordSet := c.SMTPPassword != ""
	matrixTokenSet := c.MatrixToken != ""
	c.SMTPPassword = ""
	c.MatrixToken = ""
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"notify":          c,
		"smtpPasswordSet": smtpPasswordSet,
		"matrixTokenSet":  matrixTokenSet,
	}))
}

// decodeNotifyBody decodes a POSTed notify.Config, refusing unknown fields like
// every other body. notify.Config has its own UnmarshalJSON, which
// DisallowUnknownFields cannot see into, so the raw body goes to
// notify.Config.DecodeStrict instead.
func decodeNotifyBody(w http.ResponseWriter, r *http.Request, c *notify.Config) bool {
	var raw json.RawMessage
	if !decodeBody(w, r, &raw) {
		return false
	}
	if err := c.DecodeStrict(raw); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "invalid request body"})
		return false
	}
	return true
}

// fillNotifySecrets fills blank credential fields from the stored config. An
// unchanged form submits them blank, since handleGetNotify never sends them.
func (h *Handler) fillNotifySecrets(c notify.Config) (notify.Config, error) {
	if c.SMTPPassword != "" && c.MatrixToken != "" {
		return c, nil
	}
	cur, err := h.svc.NotifyConfig()
	if err != nil {
		return c, nil
	}
	// A stored secret is refilled only for the destination it was stored for.
	// Otherwise POST /api/notify/test with a homeserver of the caller's choice
	// and a blank token would send the stored Matrix token there. A changed
	// destination is refused rather than blanked, which would silently stop
	// notifications while the setup reported success.
	if c.MatrixToken == "" && cur.MatrixToken != "" {
		if !sameMatrixTarget(c, cur) {
			return c, errors.New("enter the Matrix access token again: the stored one belongs to the previous homeserver")
		}
		c.MatrixToken = cur.MatrixToken
	}
	if c.SMTPPassword == "" && cur.SMTPPassword != "" {
		if !sameSMTPTarget(c, cur) {
			return c, errors.New("enter the SMTP password again: the stored one belongs to the previous server")
		}
		c.SMTPPassword = cur.SMTPPassword
	}
	return c, nil
}

// sameMatrixTarget reports whether a request names the same Matrix destination
// the stored token was saved for. The token is sent to the homeserver and
// grants access to the room, so a changed room is a changed destination too.
func sameMatrixTarget(req, cur notify.Config) bool {
	return req.MatrixHomeserver == cur.MatrixHomeserver && req.MatrixRoom == cur.MatrixRoom
}

// sameSMTPTarget is the SMTP counterpart: host, port and username together
// identify the account the stored password belongs to.
func sameSMTPTarget(req, cur notify.Config) bool {
	return req.SMTPHost == cur.SMTPHost && req.SMTPPort == cur.SMTPPort && req.SMTPUsername == cur.SMTPUsername
}

// handleSetNotify stores the notification config (encrypted). A blank SMTP password
// or Matrix token keeps the stored one. POST /api/notify
func (h *Handler) handleSetNotify(w http.ResponseWriter, r *http.Request) {
	var c notify.Config
	if !decodeNotifyBody(w, r, &c) {
		return
	}
	filled, err := h.fillNotifySecrets(c)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.SetNotifyConfig(filled); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// handleGetCloud returns the cloud backend credentials without the secrets:
// the other fields plus "is-set" flags. GET /api/cloud
func (h *Handler) handleGetCloud(w http.ResponseWriter, _ *http.Request) {
	c, err := h.svc.CloudConfig()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"s3KeyId":         c.S3KeyID,
		"s3Region":        c.S3Region,
		"restUser":        c.RESTUser,
		"s3StorageClass":  c.S3StorageClass,
		"s3SecretSet":     c.S3Secret != "",
		"restPasswordSet": c.RESTPassword != "",
	}))
}

// handleSetCloud stores the cloud-backend credentials (encrypted). A blank secret
// field keeps the stored one. POST /api/cloud
func (h *Handler) handleSetCloud(w http.ResponseWriter, r *http.Request) {
	var c CloudCreds
	if !decodeBody(w, r, &c) {
		return
	}
	before, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.SetCloudCreds(c); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	// The S3 storage class is part of the cloud creds, and replication reads it
	// from each domain's primary off-site target. A failed read skips the sync.
	if settings, sErr := h.store.GetSettings(); sErr == nil {
		h.svc.syncAllPrimaryOffsiteTargets(settings)
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"warnings": h.svc.directCredsWarnings(r.Context(), before, func(direct, target store.OffsiteTarget) bool {
			return direct.CredsRef == "" || target.CredsRef == ""
		}),
	}))
}

// handleGetCloudCredSets returns the additional named credential sets without
// secrets, with the same is-set flags as handleGetCloud.
// GET /api/cloud/creds-sets
func (h *Handler) handleGetCloudCredSets(w http.ResponseWriter, _ *http.Request) {
	sets, err := h.svc.CloudCredSets()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	out := make([]map[string]any, len(sets))
	for i, c := range sets {
		out[i] = map[string]any{
			"id":              c.ID,
			"name":            c.Name,
			"s3KeyId":         c.S3KeyID,
			"s3Region":        c.S3Region,
			"restUser":        c.RESTUser,
			"s3StorageClass":  c.S3StorageClass,
			"s3SecretSet":     c.S3Secret != "",
			"restPasswordSet": c.RESTPassword != "",
		}
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{"sets": out}))
}

// handleSetCloudCredSets replaces the whole list of additional named
// credential sets. A blank secret field on a set whose id matches a stored set
// keeps the stored secret, as in handleSetCloud. POST /api/cloud/creds-sets
func (h *Handler) handleSetCloudCredSets(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Sets []CloudCredSet `json:"sets"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	before, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.SetCloudCredSets(body.Sets); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"warnings": h.svc.directCredsWarnings(r.Context(), before, func(direct, target store.OffsiteTarget) bool {
			return direct.CredsRef != "" || target.CredsRef != ""
		}),
	}))
}

// handleTestNotify sends a test notification using the POSTed config (so the
// user can test the form before saving). POST /api/notify/test
func (h *Handler) handleTestNotify(w http.ResponseWriter, r *http.Request) {
	var c notify.Config
	if !decodeNotifyBody(w, r, &c) {
		return
	}
	filled, err := h.fillNotifySecrets(c)
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	if err := h.svc.TestNotify(r.Context(), filled); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}
