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
	// The Backup Everything hook commands are host-local and are never installed
	// by an import (mergeImportedSettings), so carrying them would only leak what
	// they contain — typically a dead-man's-switch ping whose URL IS its secret
	// (https://hc-ping.com/<uuid>) — into a file that gets mailed around.
	v.EverythingPreHook = ""
	v.EverythingPostHook = ""
	return v
}

// redactedLocationMarker replaces the "user:pass@" userinfo of a repo location on
// the plain export path (scrubRepoLocation). The import recognises it, so a
// location that arrives redacted can never overwrite a working one
// (importedLocation).
const redactedLocationMarker = "[redacted]@"

// scrubRepoLocation strips a URL-embedded credential out of a restic repo
// location, leaving the location itself — scheme, host, bucket, path — intact.
//
// It reuses credentialRe (handlers.go), the SAME userinfo pattern the error
// scrubber matches on, rather than growing a second pattern that would drift
// from it. What it deliberately does NOT reuse is scrubSecrets: that runs
// absPathRe first, and a repo location IS a path-shaped string, so the whole
// location would come out as "[path]" and the export would name no destination
// at all. Only the credential half applies here.
//
// A location is not itself a secret — which bucket a box replicates to is the
// portable part of a settings file — but it can CARRY one: restic accepts
// rest:https://user:pass@host:8000/repo, and s3:, sftp: and b2: locations take
// the same userinfo syntax (the generated recovery kit documents it in so many
// words). Emitting one verbatim would have put a live password in a file that
// any host on the LAN can fetch in trusted-LAN mode.
func scrubRepoLocation(loc string) string {
	return credentialRe.ReplaceAllString(loc, redactedLocationMarker)
}

// locationRedacted reports whether a repo location reached the import with its
// embedded credential already stripped by a plain export.
func locationRedacted(loc string) bool {
	return strings.Contains(loc, redactedLocationMarker)
}

// redactExportLocations strips URL-embedded credentials out of every repo
// location the envelope carries: the five per-domain off-site locations in the
// settings block and each off-site target's repo.
//
// Applied to the PLAIN export only. The credentialed variant already hands out
// every stored secret in the clear behind requireAuthForSecrets, so scrubbing
// there would leave the one export that is meant to be a complete, portable copy
// as the only one that is not.
func redactExportLocations(exp *settingsExport) {
	exp.Settings.ContainersOffsite = scrubRepoLocation(exp.Settings.ContainersOffsite)
	exp.Settings.VMsOffsite = scrubRepoLocation(exp.Settings.VMsOffsite)
	exp.Settings.FlashOffsite = scrubRepoLocation(exp.Settings.FlashOffsite)
	exp.Settings.ConfigOffsite = scrubRepoLocation(exp.Settings.ConfigOffsite)
	exp.Settings.FilesOffsite = scrubRepoLocation(exp.Settings.FilesOffsite)
	for i := range exp.OffsiteTargets {
		exp.OffsiteTargets[i].Repo = scrubRepoLocation(exp.OffsiteTargets[i].Repo)
	}
}

// redactedLocations names the repo-location slots in a file whose credential the
// exporting instance stripped, for the log line an apply writes.
func redactedLocations(exp settingsExport) []string {
	var out []string
	for _, f := range []struct{ name, loc string }{
		{"containersOffsite", exp.Settings.ContainersOffsite},
		{"vmsOffsite", exp.Settings.VMsOffsite},
		{"flashOffsite", exp.Settings.FlashOffsite},
		{"configOffsite", exp.Settings.ConfigOffsite},
		{"filesOffsite", exp.Settings.FilesOffsite},
	} {
		if locationRedacted(f.loc) {
			out = append(out, f.name)
		}
	}
	for _, tv := range exp.OffsiteTargets {
		if !locationRedacted(tv.Repo) {
			continue
		}
		name := strings.TrimSpace(tv.Name)
		if name == "" {
			name = strings.TrimSpace(tv.ID)
		}
		out = append(out, "off-site target "+name)
	}
	return out
}

// handleExportSettings streams the portable settings/off-site/credentials envelope
// as a downloadable JSON attachment. GET /api/settings/export?includeCredentials=
// true|false (default false). The body is never logged.
//
// The plain export carries no secret: toView blanks the metrics/widget tokens,
// buildSettingsView drops the registry-auth list and the hook commands, and
// redactExportLocations strips the "user:pass@" a repo location can carry inside
// its own URL. That last one is the reason this comment used to be wrong — a
// location was emitted verbatim, and a rest:/s3:/sftp:/b2: location is allowed to
// hold a live password. With all three closed the plain file is served behind the
// session authGate like every other /api route.
//
// The CREDENTIALED export is a different animal: it hands out the decrypted S3
// keys, the restic-REST password, the WHOLE rclone config, the SMTP password and
// the Matrix access token. That is the recovery kit's class of payload, so it
// takes the recovery kit's second gate — auth must actually be ENABLED. Without
// it, the trusted-LAN mode (no login password → authGate is a pass-through) let
// any host on the LAN fetch every backend credential this instance holds with a
// single unauthenticated GET.
func (h *Handler) handleExportSettings(w http.ResponseWriter, r *http.Request) {
	withCredentials := truthy(r.URL.Query().Get("includeCredentials"))
	if withCredentials && !h.requireAuthForSecrets(w, "exporting settings with credentials") {
		return
	}

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

	if withCredentials {
		creds, cErr := h.collectCredentials(s)
		if cErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(cErr))
			return
		}
		exp.Credentials = creds
	} else {
		// No credentials block — so the file must not smuggle one out inside a
		// repo URL either.
		redactExportLocations(&exp)
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
	if msg := validateExport(exp, h.cfg.HostMountRoot); msg != "" {
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
	// decodeBody's twin needs decodeBody's guard. This is the most powerful write
	// in the API — it replaces the entire configuration — and having its own copy
	// of the decode logic is exactly how it came to be missing the check the
	// ordinary settings PUT has.
	if !crossOriginGuard(w, r) {
		return exp, false
	}
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
//
// Every check here is one the SETTINGS SAVE also enforces, and that is the whole
// contract: an import must not be able to persist a row the UI's own save path
// then refuses. The SPA always PUTs the full settings object, so a single field
// this let through but handlePutSettings rejects blocks EVERY later save from
// EVERY card — including the card that would fix it. The everyN guard below was
// the first field to be shared for that reason; the path and DR-target guards
// are now shared the same way, through the same functions handlePutSettings
// calls, so extending one cannot leave the other behind.
func validateExport(exp settingsExport, mountRoot string) string {
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
	// …and must respect the SAME everyN restriction the settings save enforces
	// (#166). Without this, an imported "containersOffsiteSchedule": "everyN 3
	// 04:00" would persist happily and then make EVERY later settings save fail
	// from any card: the UI always PUTs the full settings object, so one poisoned
	// field blocks the whole Schedules tab. One guard, both write paths — which
	// also means the drills/tamper-test/digest cadences that everyN now genuinely
	// supports import exactly where a UI-set one is accepted, with no second list
	// left to drift out of step.
	if msg := rejectEveryNSchedules(exp.Settings); msg != "" {
		return "invalid schedule in settings: " + msg
	}
	// Repo locations. Without this an imported `containersPath` of
	// "/mnt/user/backups" — absolute rather than the required relative subpath,
	// which is exactly what a file produced on a box with a different mount root
	// carries — persisted happily and then made every later settings save fail
	// with "invalid backup path: must be a relative subpath under the mount root".
	// The user could not change ANY setting, including the path itself, without
	// editing the database.
	if msg := rejectInvalidSettingsPaths(exp.Settings, mountRoot); msg != "" {
		return "invalid path in settings: " + msg
	}
	// The DR-drill targets, which had the same asymmetry: validated on PUT,
	// unchecked on import.
	if msg := rejectInvalidSettingsNames(exp.Settings); msg != "" {
		return "invalid settings: " + msg
	}
	return ""
}

// exportCadences lists every schedule string carried in a settings view. It must
// stay in step with the parse-validation loop in handlePutSettings — a cadence
// missing here imports without ever being checked for grammar.
func exportCadences(v settingsView) []string {
	return []string{
		v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule, v.ConfigSchedule, v.FilesSchedule,
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
		v.DrillsSchedule, v.TamperTestSchedule, v.DigestSchedule, v.EverythingSchedule,
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
	// The whole-server pass is its own area, not part of "schedules": it is the
	// one setting an apply can switch ON for a box that never ran it, so the
	// preview has to name it.
	add("everything", v.EverythingSchedule != "")
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
	// Map the imported view onto the CURRENT row, PRESERVING the per-instance
	// fields the file intentionally omits (auth password, session epoch,
	// recovery-kit ack, registry-auth blob) and the encrypted credential blobs
	// (those are updated separately below, only when the file carries them).
	// The merge runs inside MutateSettings' transaction so those preserved
	// fields are read at write time: an import is a slow request (a 4 MiB body,
	// a full validation pass), and a password change or credential save landing
	// in that window must not be reverted by the row this writes back.
	// A hook command in the file is not installed (see mergeImportedSettings for
	// why a settings file may not hand this host a command to run). Say so, so an
	// operator moving to a new box learns that the one part of their Backup
	// Everything setup that did NOT travel is the hook, instead of discovering it
	// the night the dead-man's-switch does not ping.
	if strings.TrimSpace(exp.Settings.EverythingPreHook) != "" || strings.TrimSpace(exp.Settings.EverythingPostHook) != "" {
		log.Print("api: settings import: the file carries Backup Everything pre/post-hook commands — NOT installed. " +
			"A hook is a shell command this host runs, so it is set on the instance, never by an imported file. " +
			"Enter it under Settings > Schedules > Backup Everything if you want it here.")
	}

	// Same deal for a location whose credential the exporting instance stripped:
	// say so, because the operator is the only one who can put the password back.
	if slots := redactedLocations(exp); len(slots) > 0 {
		log.Printf("api: settings import: these repo locations arrived with their embedded credential removed (%s) — "+
			"a plain export never writes a password into a URL. Where this instance already has a location it is KEPT; "+
			"anywhere else the location lands with the marker still in it. Re-enter the credential in the repo URL, "+
			"or export again with credentials included.", strings.Join(slots, ", "))
	}

	if _, err := h.store.MutateSettings(func(cur *store.Settings) error {
		*cur = mergeImportedSettings(*cur, exp.Settings)
		return nil
	}); err != nil {
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
	s, err := h.store.GetSettings()
	if err != nil {
		return err
	}
	h.svc.syncAllPrimaryOffsiteTargets(s)
	if h.scheduler != nil {
		if err := h.scheduler.ReloadWithDueChecks(s, h.containersLastRun, h.vmsLastRun, h.flashLastRun, h.configLastRun, h.filesLastRun, h.everythingLastRun); err != nil {
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
	// Remember each row's location BEFORE the rows are dropped: a location the
	// file carries redacted must not overwrite the working one this instance
	// already has for that id (see importedLocation).
	currentRepo := make(map[string]string, len(current))
	for _, t := range current {
		currentRepo[t.ID] = t.Repo
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
		t.Repo = importedLocation(currentRepo[t.ID], t.Repo)
		if _, err := h.store.UpsertOffsiteTarget(t); err != nil {
			return err
		}
	}
	return nil
}

// importedLocation picks the repo location an apply writes into one slot: the
// file's, unless that one arrived redacted (a plain export from an instance whose
// location carried a credential) AND this instance already has a location there —
// then the working one stays.
//
// Writing "rest:https://[redacted]@host:8000/repo" over a location that works
// would turn importing a settings file into breaking the off-site replication the
// target instance already had running, and it would do it quietly: nothing else in
// the file says a credential was removed. Keeping what is there is the harmless
// direction, the same one mergeImportedSettings takes for the hook commands and
// the credential blobs; applyImport logs which slots it applied to.
//
// A slot this instance has NOTHING in keeps the redacted value instead of being
// left empty. That is deliberate: the marker is visible in Settings and the next
// run fails against a location an operator can repair by typing the password back
// in, whereas a silently blank off-site location is a box that just stops
// replicating and says nothing.
func importedLocation(existing, imported string) string {
	if locationRedacted(imported) && strings.TrimSpace(existing) != "" {
		return existing
	}
	return imported
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
//
// It starts from the EXISTING row and overwrites the portable fields, and that
// direction is the point. It used to build a fresh store.Settings composite
// literal, which silently wrote a zero value into every column nobody had
// remembered to list: applying an import cleared everythingSchedule (the
// whole-server pass, switched off), everythingPostHook (the dead-man's-switch
// ping that proves the pass completed, deleted), the fleet fields and the
// instance name — with no error, and nothing in the preview to hint at it. A
// literal makes forgetting a field a WIPE; starting from the row makes
// forgetting a field a no-op, which is the harmless direction and the only one
// that stays safe as columns are added.
//
// EverythingPreHook / EverythingPostHook are deliberately NOT taken from the
// file. They are shell commands this host executes (HostShell, via `sh -c`), so
// importing them would let a settings file — a thing users mail each other and
// download from forum threads — install an arbitrary command on the box. They
// get the treatment the credential fields get: kept from the instance being
// imported into, never installed by the file. applyImport logs when a file
// carried them so the operator is not left guessing why a hook did not travel.
func mergeImportedSettings(existing store.Settings, v settingsView) store.Settings {
	out := existing

	// Kept from the target instance, and kept by CONSTRUCTION rather than by a
	// line someone has to remember: the login password hash, the session epoch,
	// the recovery-kit acknowledgement, the registry-auth blob, the metrics and
	// widget tokens (the view blanks them, and blank means keep), the fleet
	// token / fleet switch / instance name, the encrypted credential blobs
	// (applyImportedCredentials updates those, and only when the file carries
	// them), and the two Backup Everything hook commands. Every assignment below
	// is a field the file is allowed to set.

	out.EncryptionEnabled = v.EncryptionEnabled
	out.ContainersEnabled = v.ContainersEnabled
	out.VMsEnabled = v.VMsEnabled
	out.FlashEnabled = v.FlashEnabled
	out.ConfigEnabled = v.ConfigEnabled
	out.FilesEnabled = v.FilesEnabled
	out.ContainersPath = v.ContainersPath
	out.VMsPath = v.VMsPath
	out.FlashPath = v.FlashPath
	out.ConfigPath = v.ConfigPath
	out.FilesPath = v.FilesPath
	out.RestoreFolder = v.RestoreFolder
	// Off-site locations, via importedLocation: a location the plain export
	// stripped a credential out of never overwrites a working one here.
	out.ContainersOffsite = importedLocation(existing.ContainersOffsite, v.ContainersOffsite)
	out.VMsOffsite = importedLocation(existing.VMsOffsite, v.VMsOffsite)
	out.FlashOffsite = importedLocation(existing.FlashOffsite, v.FlashOffsite)
	out.ConfigOffsite = importedLocation(existing.ConfigOffsite, v.ConfigOffsite)
	out.FilesOffsite = importedLocation(existing.FilesOffsite, v.FilesOffsite)
	out.ContainersOffsiteSchedule = v.ContainersOffsiteSchedule
	out.VMsOffsiteSchedule = v.VMsOffsiteSchedule
	out.FlashOffsiteSchedule = v.FlashOffsiteSchedule
	out.ConfigOffsiteSchedule = v.ConfigOffsiteSchedule
	out.FilesOffsiteSchedule = v.FilesOffsiteSchedule
	out.ContainersSchedule = v.ContainersSchedule
	out.VMsSchedule = v.VMsSchedule
	out.FlashSchedule = v.FlashSchedule
	out.ConfigSchedule = v.ConfigSchedule
	out.FilesSchedule = v.FilesSchedule
	// The whole-server pass's cadence. Missing here until now, so an import
	// switched Backup Everything off on the instance it was applied to.
	out.EverythingSchedule = v.EverythingSchedule
	out.FlashZipExportEnabled = v.FlashZipExportEnabled
	out.FlashZipExportPath = v.FlashZipExportPath
	out.FlashZipExportKeep = max(0, v.FlashZipExportKeep)
	out.DefaultLanguage = v.DefaultLanguage
	out.RetentionKeepLast = max(0, v.RetentionKeepLast)
	out.RetentionKeepDaily = max(0, v.RetentionKeepDaily)
	out.RetentionKeepWeekly = max(0, v.RetentionKeepWeekly)
	out.RetentionKeepMonthly = max(0, v.RetentionKeepMonthly)
	out.OffsiteRetentionKeepLast = max(0, v.OffsiteRetentionKeepLast)
	out.OffsiteRetentionKeepDaily = max(0, v.OffsiteRetentionKeepDaily)
	out.OffsiteRetentionKeepWeekly = max(0, v.OffsiteRetentionKeepWeekly)
	out.OffsiteRetentionKeepMonthly = max(0, v.OffsiteRetentionKeepMonthly)
	out.OffsiteLimitUpload = max(0, v.OffsiteLimitUpload)
	out.OffsiteLimitDownload = max(0, v.OffsiteLimitDownload)
	out.MetricsEnabled = v.MetricsEnabled
	out.DrillsEnabled = v.DrillsEnabled
	out.DrillsSchedule = v.DrillsSchedule
	out.DrillsSubsetPct = max(1, min(100, v.DrillsSubsetPct))
	out.OffsiteDrillsEnabled = v.OffsiteDrillsEnabled
	out.ContainersOffsiteImmutable = v.ContainersOffsiteImmutable
	out.VMsOffsiteImmutable = v.VMsOffsiteImmutable
	out.FlashOffsiteImmutable = v.FlashOffsiteImmutable
	out.ConfigOffsiteImmutable = v.ConfigOffsiteImmutable
	out.FilesOffsiteImmutable = v.FilesOffsiteImmutable
	out.OffsiteGrowthBudgetGB = max(0, v.OffsiteGrowthBudgetGB)
	out.TamperTestSchedule = v.TamperTestSchedule
	out.DRDrillTarget = strings.TrimSpace(v.DRDrillTarget)
	out.DRDrillTargetVM = strings.TrimSpace(v.DRDrillTargetVM)
	out.PruneImageAfterUpdate = v.PruneImageAfterUpdate
	out.ResticCacheMaxMB = max(0, v.ResticCacheMaxMB)
	out.DigestEnabled = v.DigestEnabled
	out.DigestSchedule = v.DigestSchedule
	out.CatchUpMissed = v.CatchUpMissed
	out.WatchdogEnabled = v.WatchdogEnabled
	out.ReconcileUnraidUpdateStatus = v.ReconcileUnraidUpdateStatus
	out.ExportEncryptEnabled = v.ExportEncryptEnabled
	out.ExportAgeRecipients = strings.TrimSpace(v.ExportAgeRecipients)
	out.ReceiverEnabled = v.ReceiverEnabled
	out.RestartHealthWait = v.RestartHealthWait
	out.RestartHealthTimeoutSec = clampHealthTimeoutSec(v.RestartHealthTimeoutSec)
	out.PerItemSchedules = v.PerItemSchedules

	return out
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
