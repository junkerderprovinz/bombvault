package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// settingsExportSchema is the envelope version emitted by the export and the only
// version the import accepts. Bump it (and widen the import's accepted set) when a
// breaking change lands in the exported shape.
const settingsExportSchema = 1

// exportCredentials carries the DECRYPTED off-site backend secrets so the export
// file is portable to an instance with a DIFFERENT APP_KEY. It is present only
// when the export was requested with ?includeCredentials=true, and the import
// RE-ENCRYPTS each value with the local key before storing. It never carries the
// APP_KEY itself, the derived restic repo passwords, the login password hash, or
// the session epoch — those are instance-local and are always preserved on import.
type exportCredentials struct {
	// Cloud is the S3 / restic-REST backend credentials in cleartext.
	Cloud CloudCreds `json:"cloud"`
	// Rclone is the full decrypted rclone config (INI text).
	Rclone string `json:"rclone"`
	// Notify is the notification config in cleartext (SMTP password / Matrix token
	// included).
	Notify notify.Config `json:"notify"`
}

// settingsExport is the portable configuration envelope written by the export and
// consumed by the import. Settings is the same user-facing view the SPA edits
// (secrets blanked, auth/session/managed fields excluded); OffsiteTargets is the
// non-secret off-site DESTINATION list; Credentials is present only when secrets
// were requested. It intentionally carries NOTHING about backup repositories,
// snapshots or run history — the import touches none of those.
type settingsExport struct {
	SchemaVersion  int                 `json:"schemaVersion"`
	ExportedAt     string              `json:"exportedAt"`
	AppVersion     string              `json:"appVersion"`
	Settings       settingsView        `json:"settings"`
	OffsiteTargets []offsiteTargetView `json:"offsiteTargets"`
	Credentials    *exportCredentials  `json:"credentials,omitempty"`
}

// buildSettingsView returns the export's settings block: the user-facing view with
// the per-instance state fields (recovery-kit ack, registry-auth list) cleared, so
// the file carries only the portable configuration. Secrets (metrics/widget tokens)
// are already blanked by toView, and auth/session/encrypted-blob fields are not part
// of settingsView at all.
func buildSettingsView(s store.Settings) settingsView {
	v := toView(s)
	v.RecoveryKitAck = false // per-instance dashboard-nag state, not portable config
	v.RegistryAuths = nil    // secret token blobs are out of scope for the portable file
	return v
}

// handleExportSettings streams the portable settings/off-site/credentials envelope
// as a downloadable JSON attachment. GET /api/settings/export?includeCredentials=
// true|false (default false). With credentials the file is as sensitive as the
// recovery kit — it holds the decrypted off-site backend secrets — so the body is
// never logged. It is served behind the same session authGate as every other
// /api route.
func (h *Handler) handleExportSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	targets, err := h.store.ListOffsiteTargets()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	exp := settingsExport{
		SchemaVersion:  settingsExportSchema,
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		AppVersion:     Version,
		Settings:       buildSettingsView(s),
		OffsiteTargets: offsiteTargetsToViews(targets),
	}

	if truthy(r.URL.Query().Get("includeCredentials")) {
		creds, cErr := h.collectCredentials(s)
		if cErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(cErr))
			return
		}
		exp.Credentials = creds
	}

	body, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	filename := "bombvault-settings-" + time.Now().Format("2006-01-02") + ".json"
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	if _, wErr := w.Write(body); wErr != nil {
		// Log only the failure, never the body — with credentials it holds the
		// decrypted off-site secrets.
		log.Printf("api: settings export: write failed: %v", wErr)
	}
}

// collectCredentials decrypts the stored off-site backend secrets into a portable
// (cleartext) credentials block. Called only on an includeCredentials export.
func (h *Handler) collectCredentials(s store.Settings) (*exportCredentials, error) {
	cloud, err := h.svc.decodeCloud(s)
	if err != nil {
		return nil, fmt.Errorf("read cloud credentials: %w", err)
	}
	rclone, err := h.svc.decodeRcloneConf(s)
	if err != nil {
		return nil, fmt.Errorf("read rclone config: %w", err)
	}
	notifyConf, err := h.svc.NotifyConfig()
	if err != nil {
		return nil, fmt.Errorf("read notification config: %w", err)
	}
	return &exportCredentials{Cloud: cloud, Rclone: rclone, Notify: notifyConf}, nil
}

// importSummary is the preview payload: what an apply WOULD change, without writing.
type importSummary struct {
	SchemaVersion  int                 `json:"schemaVersion"`
	ExportedAt     string              `json:"exportedAt"`
	AppVersion     string              `json:"appVersion"`
	OffsiteTargets int                 `json:"offsiteTargets"`
	Credentials    importCredsPresence `json:"credentials"`
	SettingsGroups []string            `json:"settingsGroups"`
}

// importCredsPresence reports which credential kinds the file carries (never the
// values). All false when the file has no credentials block.
type importCredsPresence struct {
	Present bool `json:"present"`
	Cloud   bool `json:"cloud"`
	Rclone  bool `json:"rclone"`
	Notify  bool `json:"notify"`
}

// handleImportSettings validates a settings-export file and, with ?apply=true,
// writes it. POST /api/settings/import (body = the export JSON).
//
//   - apply=false / ?preview (the default): VALIDATE the file and return a summary
//     of what WOULD change, writing nothing.
//   - apply=true: validate, RE-ENCRYPT any credentials with THIS instance's APP_KEY
//     and write the settings + off-site targets + credentials.
//
// It never touches backup repositories, snapshots or run history. A missing
// credentials block leaves the existing off-site secrets untouched (they are not
// wiped). An unsupported schemaVersion or a malformed file is rejected with a clear
// error.
func (h *Handler) handleImportSettings(w http.ResponseWriter, r *http.Request) {
	exp, ok := decodeExport(w, r)
	if !ok {
		return
	}
	if msg := validateExport(exp); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}

	apply := truthy(r.URL.Query().Get("apply"))
	if !apply {
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
			"preview": true,
			"summary": summarizeExport(exp),
		}))
		return
	}

	if err := h.applyImport(r, exp); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"applied": true,
		"summary": summarizeExport(exp),
	}))
}

// decodeExport reads the request body as a settingsExport. Unlike decodeBody it
// tolerates unknown (future) fields so a newer file with the same schemaVersion is
// not rejected outright, but it still rejects a syntactically malformed body.
func decodeExport(w http.ResponseWriter, r *http.Request) (settingsExport, bool) {
	var exp settingsExport
	if r.Body == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "missing request body"})
		return exp, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20) // 4 MiB — a config file, not a data blob
	if err := json.NewDecoder(r.Body).Decode(&exp); err != nil {
		if err == io.EOF {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "empty import file"})
			return exp, false
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "malformed import file: expected a BombVault settings-export JSON"})
		return exp, false
	}
	return exp, true
}

// validateExport checks the envelope is a supported, structurally-sane export.
// Returns a user-facing error string, or "" when valid.
func validateExport(exp settingsExport) string {
	if exp.SchemaVersion != settingsExportSchema {
		return fmt.Sprintf("unsupported schemaVersion %d (this build reads version %d)", exp.SchemaVersion, settingsExportSchema)
	}
	// Off-site targets must each map to a valid domain + non-empty repo. Validate
	// against the same contract the CRUD endpoints enforce, so a bad file is
	// rejected before any write.
	for i, tv := range exp.OffsiteTargets {
		if msg := validateOffsiteTargetInput(tv.toStoreTarget()); msg != "" {
			return fmt.Sprintf("off-site target #%d: %s", i+1, msg)
		}
	}
	// Every schedule cadence in the imported settings must parse (same grammar the
	// settings save enforces), so an apply cannot install an un-runnable schedule.
	for _, cad := range exportCadences(exp.Settings) {
		if _, err := schedule.ParseCadence(cad); err != nil {
			return "invalid schedule in settings: " + scrubError(err)
		}
	}
	return ""
}

// exportCadences lists every schedule string carried in a settings view.
func exportCadences(v settingsView) []string {
	return []string{
		v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule, v.ConfigSchedule, v.FilesSchedule,
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
		v.DrillsSchedule, v.TamperTestSchedule, v.DigestSchedule,
	}
}

// summarizeExport builds the preview/summary payload for a validated export.
func summarizeExport(exp settingsExport) importSummary {
	return importSummary{
		SchemaVersion:  exp.SchemaVersion,
		ExportedAt:     exp.ExportedAt,
		AppVersion:     exp.AppVersion,
		OffsiteTargets: len(exp.OffsiteTargets),
		Credentials:    credsPresence(exp.Credentials),
		SettingsGroups: settingsGroups(exp.Settings),
	}
}

// credsPresence reports which credential kinds the file carries.
func credsPresence(c *exportCredentials) importCredsPresence {
	if c == nil {
		return importCredsPresence{}
	}
	return importCredsPresence{
		Present: true,
		Cloud:   cloudCredsMeaningful(c.Cloud),
		Rclone:  strings.TrimSpace(c.Rclone) != "",
		Notify:  notifyMeaningful(c.Notify),
	}
}

// settingsGroups names the setting groups the imported view carries a value for,
// so the preview can tell the user which areas an apply would populate. It is
// descriptive only — an apply writes the whole settings block regardless.
func settingsGroups(v settingsView) []string {
	var groups []string
	add := func(name string, on bool) {
		if on {
			groups = append(groups, name)
		}
	}
	add("domains", v.ContainersEnabled || v.VMsEnabled || v.FlashEnabled || v.ConfigEnabled || v.FilesEnabled ||
		v.ContainersPath != "" || v.VMsPath != "" || v.FlashPath != "" || v.ConfigPath != "" || v.FilesPath != "")
	add("schedules", v.ContainersSchedule != "" || v.VMsSchedule != "" || v.FlashSchedule != "" ||
		v.ConfigSchedule != "" || v.FilesSchedule != "")
	add("retention", v.RetentionKeepLast > 0 || v.RetentionKeepDaily > 0 || v.RetentionKeepWeekly > 0 || v.RetentionKeepMonthly > 0 ||
		v.OffsiteRetentionKeepLast > 0 || v.OffsiteRetentionKeepDaily > 0 || v.OffsiteRetentionKeepWeekly > 0 || v.OffsiteRetentionKeepMonthly > 0)
	add("offsite", v.ContainersOffsite != "" || v.VMsOffsite != "" || v.FlashOffsite != "" || v.ConfigOffsite != "" || v.FilesOffsite != "")
	add("drills", v.DrillsEnabled || v.DrillsSchedule != "" || v.OffsiteDrillsEnabled)
	add("digest", v.DigestEnabled || v.DigestSchedule != "")
	add("monitoring", v.MetricsEnabled || v.WidgetTokenSet)
	add("language", v.DefaultLanguage != "")
	add("exportEncryption", v.ExportEncryptEnabled || v.ExportAgeRecipients != "")
	return groups
}

// applyImport writes a validated export: the settings row, a full replace of the
// off-site targets, and any credentials (re-encrypted with the local APP_KEY). It
// never touches repos, snapshots or run history.
func (h *Handler) applyImport(r *http.Request, exp settingsExport) error {
	existing, err := h.store.GetSettings()
	if err != nil {
		return err
	}

	// Map the imported view onto a fresh Settings, PRESERVING the per-instance
	// fields the file intentionally omits (auth password, session epoch,
	// recovery-kit ack, registry-auth blob) and the encrypted credential blobs
	// (those are updated separately below, only when the file carries them).
	s := mergeImportedSettings(existing, exp.Settings)
	if err := h.store.UpdateSettings(s); err != nil {
		return err
	}

	// Replace the off-site targets with the imported set (a clean, deterministic
	// round-trip): drop the current rows, then upsert each imported target
	// preserving its id + created_at so the far instance reproduces the source.
	if err := h.replaceOffsiteTargets(exp.OffsiteTargets); err != nil {
		return err
	}

	// Credentials: present block → re-encrypt each NON-EMPTY kind with the local
	// key. An empty kind (or a missing block) leaves the existing secret untouched
	// — import is additive, it never wipes secrets.
	if exp.Credentials != nil {
		if err := h.applyImportedCredentials(*exp.Credentials); err != nil {
			return err
		}
	}

	// Mirror the imported off-site config into the primary off-site target rows and
	// re-arm the scheduler, exactly like a settings save, so the imported schedules
	// take effect. The scheduler may be absent in a stripped test wiring — guard it.
	s, err = h.store.GetSettings()
	if err != nil {
		return err
	}
	h.svc.syncAllPrimaryOffsiteTargets(s)
	if h.scheduler != nil {
		if err := h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun, h.configLastRun, h.filesLastRun); err != nil {
			return err
		}
	}
	_ = r
	return nil
}

// replaceOffsiteTargets drops all current off-site targets and re-inserts the
// imported set, preserving each id + created_at for an exact round-trip.
func (h *Handler) replaceOffsiteTargets(views []offsiteTargetView) error {
	current, err := h.store.ListOffsiteTargets()
	if err != nil {
		return err
	}
	for _, t := range current {
		if err := h.store.DeleteOffsiteTarget(t.ID); err != nil {
			return err
		}
	}
	for _, tv := range views {
		t := tv.toStoreTarget()
		t.ID = strings.TrimSpace(tv.ID) // preserve the exported id (empty → store mints one)
		t.CreatedAt = tv.CreatedAt      // preserve the exported timestamp (0 → store stamps now)
		if _, err := h.store.UpsertOffsiteTarget(t); err != nil {
			return err
		}
	}
	return nil
}

// applyImportedCredentials re-encrypts and stores each non-empty credential kind
// with the local APP_KEY. An empty kind is left untouched (no wipe).
func (h *Handler) applyImportedCredentials(c exportCredentials) error {
	if cloudCredsMeaningful(c.Cloud) {
		if err := h.svc.SetCloudCreds(c.Cloud); err != nil {
			return fmt.Errorf("store cloud credentials: %w", err)
		}
	}
	if strings.TrimSpace(c.Rclone) != "" {
		if err := h.svc.SetRcloneConf(c.Rclone); err != nil {
			return fmt.Errorf("store rclone config: %w", err)
		}
	}
	if notifyMeaningful(c.Notify) {
		if err := h.svc.SetNotifyConfig(c.Notify); err != nil {
			return fmt.Errorf("store notification config: %w", err)
		}
	}
	return nil
}

// mergeImportedSettings maps the imported view onto a Settings row, keeping the
// per-instance fields the export omits and clamping numeric fields the same way
// the settings save does. Secret tokens (metrics/widget) blanked in the view are
// preserved from the existing row — import never wipes them.
func mergeImportedSettings(existing store.Settings, v settingsView) store.Settings {
	metricsToken := existing.MetricsToken // view blanks it; blank = keep stored
	widgetToken := existing.WidgetToken   // same contract

	return store.Settings{
		EncryptionEnabled:           v.EncryptionEnabled,
		ContainersEnabled:           v.ContainersEnabled,
		VMsEnabled:                  v.VMsEnabled,
		FlashEnabled:                v.FlashEnabled,
		ConfigEnabled:               v.ConfigEnabled,
		FilesEnabled:                v.FilesEnabled,
		ContainersPath:              v.ContainersPath,
		VMsPath:                     v.VMsPath,
		FlashPath:                   v.FlashPath,
		ConfigPath:                  v.ConfigPath,
		FilesPath:                   v.FilesPath,
		RestoreFolder:               v.RestoreFolder,
		ContainersOffsite:           v.ContainersOffsite,
		VMsOffsite:                  v.VMsOffsite,
		FlashOffsite:                v.FlashOffsite,
		ConfigOffsite:               v.ConfigOffsite,
		FilesOffsite:                v.FilesOffsite,
		ContainersOffsiteSchedule:   v.ContainersOffsiteSchedule,
		VMsOffsiteSchedule:          v.VMsOffsiteSchedule,
		FlashOffsiteSchedule:        v.FlashOffsiteSchedule,
		ConfigOffsiteSchedule:       v.ConfigOffsiteSchedule,
		FilesOffsiteSchedule:        v.FilesOffsiteSchedule,
		ContainersSchedule:          v.ContainersSchedule,
		VMsSchedule:                 v.VMsSchedule,
		FlashSchedule:               v.FlashSchedule,
		ConfigSchedule:              v.ConfigSchedule,
		FilesSchedule:               v.FilesSchedule,
		FlashZipExportEnabled:       v.FlashZipExportEnabled,
		FlashZipExportPath:          v.FlashZipExportPath,
		FlashZipExportKeep:          max(0, v.FlashZipExportKeep),
		DefaultLanguage:             v.DefaultLanguage,
		RetentionKeepLast:           max(0, v.RetentionKeepLast),
		RetentionKeepDaily:          max(0, v.RetentionKeepDaily),
		RetentionKeepWeekly:         max(0, v.RetentionKeepWeekly),
		RetentionKeepMonthly:        max(0, v.RetentionKeepMonthly),
		OffsiteRetentionKeepLast:    max(0, v.OffsiteRetentionKeepLast),
		OffsiteRetentionKeepDaily:   max(0, v.OffsiteRetentionKeepDaily),
		OffsiteRetentionKeepWeekly:  max(0, v.OffsiteRetentionKeepWeekly),
		OffsiteRetentionKeepMonthly: max(0, v.OffsiteRetentionKeepMonthly),
		OffsiteLimitUpload:          max(0, v.OffsiteLimitUpload),
		OffsiteLimitDownload:        max(0, v.OffsiteLimitDownload),
		MetricsEnabled:              v.MetricsEnabled,
		MetricsToken:                metricsToken,
		WidgetToken:                 widgetToken,
		DrillsEnabled:               v.DrillsEnabled,
		DrillsSchedule:              v.DrillsSchedule,
		DrillsSubsetPct:             max(1, min(100, v.DrillsSubsetPct)),
		OffsiteDrillsEnabled:        v.OffsiteDrillsEnabled,
		ContainersOffsiteImmutable:  v.ContainersOffsiteImmutable,
		VMsOffsiteImmutable:         v.VMsOffsiteImmutable,
		FlashOffsiteImmutable:       v.FlashOffsiteImmutable,
		ConfigOffsiteImmutable:      v.ConfigOffsiteImmutable,
		FilesOffsiteImmutable:       v.FilesOffsiteImmutable,
		OffsiteGrowthBudgetGB:       max(0, v.OffsiteGrowthBudgetGB),
		TamperTestSchedule:          v.TamperTestSchedule,
		DRDrillTarget:               strings.TrimSpace(v.DRDrillTarget),
		PruneImageAfterUpdate:       v.PruneImageAfterUpdate,
		ResticCacheMaxMB:            max(0, v.ResticCacheMaxMB),
		DigestEnabled:               v.DigestEnabled,
		DigestSchedule:              v.DigestSchedule,
		CatchUpMissed:               v.CatchUpMissed,
		WatchdogEnabled:             v.WatchdogEnabled,
		ExportEncryptEnabled:        v.ExportEncryptEnabled,
		ExportAgeRecipients:         strings.TrimSpace(v.ExportAgeRecipients),
		ReceiverEnabled:             v.ReceiverEnabled,
		// Per-instance / secret-managed fields preserved from the target instance:
		AuthPasswordHash: existing.AuthPasswordHash,
		SessionEpoch:     existing.SessionEpoch,
		RecoveryKitAck:   existing.RecoveryKitAck,
		RegistryAuths:    existing.RegistryAuths,
		// Encrypted credential blobs: kept as-is here; updated separately (and only
		// when the file carries them) by applyImportedCredentials.
		RcloneConf: existing.RcloneConf,
		NotifyConf: existing.NotifyConf,
		CloudConf:  existing.CloudConf,
	}
}

// cloudCredsMeaningful reports whether any cloud credential field is set.
func cloudCredsMeaningful(c CloudCreds) bool {
	return c != CloudCreds{}
}

// notifyMeaningful reports whether a notify config carries any channel or policy —
// i.e. whether storing it would do anything (SetNotifyConfig clears an empty one).
func notifyMeaningful(c notify.Config) bool {
	return c.Configured() || (c.On != "" && c.On != "never")
}

// truthy parses the export/import boolean query flags. Empty (absent) is false;
// "1"/"true"/"yes"/"on" (any case) is true.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
