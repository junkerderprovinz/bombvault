package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/notify"
	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/schedule"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// settingsExportSchema is the envelope version emitted by the export and the only
// version the import accepts. Bump it (and widen the import's accepted set) when a
// breaking change lands in the exported shape.
const settingsExportSchema = 1

// exportCredentials carries the decrypted off-site backend secrets, so the file
// works on an instance with another APP_KEY. It is present only on an export
// with ?includeCredentials=true, and the import encrypts each value again with
// its own key. It never carries the APP_KEY, the derived restic repository
// passwords, the login password hash or the session epoch: those belong to the
// instance, and an import keeps them.
type exportCredentials struct {
	// Cloud is the S3 / restic-REST backend credentials in cleartext.
	Cloud CloudCreds `json:"cloud"`
	// Rclone is the full decrypted rclone config (INI text).
	Rclone string `json:"rclone"`
	// Notify is the notification config in cleartext (SMTP password / Matrix token
	// included).
	Notify notify.Config `json:"notify"`
	// CredSets is every named credential set with its secrets. Places and rows
	// name them by id, so a file without them would leave a restored instance
	// pointing at sets it does not have.
	CredSets []CloudCredSet `json:"credSets,omitempty"`
}

// settingsExport is the portable configuration envelope written by the export
// and read by the import. Settings is the view the SPA edits, with secrets
// blanked and auth, session and managed fields left out; Credentials is present
// only when secrets were requested. It carries nothing about snapshots or run
// history, which the import never touches. NamedRepos travel with the file
// because an item's repo column holds the id of one of them: without them the
// import would rebuild items that point at repositories that do not exist.
type settingsExport struct {
	SchemaVersion  int                 `json:"schemaVersion"`
	ExportedAt     string              `json:"exportedAt"`
	AppVersion     string              `json:"appVersion"`
	Settings       settingsView        `json:"settings"`
	OffsiteTargets []offsiteTargetView `json:"offsiteTargets"`
	NamedRepos     []offsiteTargetView `json:"namedRepos,omitempty"`
	// PlacementDefaults and CopyRules carry each domain's placement default and
	// its copy rules. Always present, even empty, so the import can tell a
	// missing block (an older file, leave the table alone) from an empty one
	// (replace it with nothing); see importedPlacement.
	PlacementDefaults []placementDefaultExport `json:"placementDefaults"`
	CopyRules         []copyRuleExport         `json:"copyRules"`
	// Places and StorageDomainPlaces are always present too, so the import can
	// tell a file from before storage places (no block) from one whose
	// instance had none (an empty one).
	Places              []placeExport      `json:"places"`
	StorageDomainPlaces map[string]string  `json:"storageDomainPlaces"`
	Credentials         *exportCredentials `json:"credentials,omitempty"`
	// predatesZFS is set when the file carries no zfsEnabled key: it comes from
	// a build without the ZFS domain, so its empty ZFS fields say nothing about
	// the ZFS setup of the instance it is applied to.
	predatesZFS bool
}

func (e *settingsExport) UnmarshalJSON(b []byte) error {
	type plain settingsExport
	if err := json.Unmarshal(b, (*plain)(e)); err != nil {
		return err
	}
	var probe struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return err
	}
	_, hasZFS := probe.Settings["zfsEnabled"]
	e.predatesZFS = !hasZFS
	return nil
}

// buildSettingsView is the export's settings block: the view the settings page
// edits, without what belongs to this instance alone, the recovery-kit
// acknowledgement and the registry logins with their tokens. toView already
// blanks the metrics and widget tokens, and the auth, session and encrypted
// fields are not part of the view.
func buildSettingsView(s store.Settings) settingsView {
	v := toView(s)
	v.RecoveryKitAck = false
	v.RegistryAuths = nil
	// An import never installs the Backup Everything hooks, and a hook is often
	// a dead-man's-switch ping whose URL is itself the secret
	// (https://hc-ping.com/<uuid>), so a file that gets mailed around leaves
	// them out.
	v.EverythingPreHook = ""
	v.EverythingPostHook = ""
	return v
}

// redactedValue replaces a secret in a repo location on the plain export path
// (scrubRepoLocation): the "user:pass" of its userinfo, which leaves
// redactedLocationMarker, or the value of an rclone password parameter. The
// import recognises it and keeps a working location here in its place where
// importedLocation or restoredLocation says so.
const (
	redactedValue          = "[redacted]"
	redactedLocationMarker = redactedValue + "@"
)

// scrubRepoLocation takes the password out of a restic repo location and keeps
// the rest, scheme included, so the location still names its destination. A
// location is no secret, but rest:https://user:pass@host:8000/repo carries one,
// and the plain export can be fetched by any host on the LAN in trusted-LAN
// mode.
//
// It matches with repoUserinfoRe, the pattern the error scrubber uses on repo
// locations, and leaves a userinfo without a password alone: the sftp short
// form sftp:user@host:/repo holds a login name, and redacting that would turn
// a working location into one the operator has to retype. urlPasswordRe then
// reaches the URLs repoUserinfoRe stops short of, and rclonePassRe the
// password parameters of an rclone connection string, which rclone only
// obscures.
func scrubRepoLocation(loc string) string {
	loc = repoUserinfoRe.ReplaceAllStringFunc(loc, func(match string) string {
		m := repoUserinfoRe.FindStringSubmatch(match)
		scheme := m[1] + ":" + m[2]
		userinfo := match[len(scheme) : len(match)-1]
		if !strings.Contains(userinfo, ":") || hostPortPathRe.MatchString(userinfo) {
			return match
		}
		return scheme + redactedLocationMarker
	})
	loc = urlPasswordRe.ReplaceAllStringFunc(loc, func(match string) string {
		if hostPortPathRe.MatchString(match[2 : len(match)-1]) {
			return match
		}
		return "//" + redactedLocationMarker
	})
	if strings.HasPrefix(loc, "rclone:") {
		loc = rclonePassRe.ReplaceAllString(loc, "${1}"+redactedValue)
	}
	return loc
}

// urlPasswordRe matches the "user:password@" of a URL anywhere in a location,
// such as the quoted url of an rclone connection string, where repoUserinfoRe
// stops at the quote. Like repoUserinfoRe, it lets the password hold "/".
var urlPasswordRe = regexp.MustCompile(`//[^@\s"'/]*:[^@\s"']*@`)

// hostPortPathRe matches what those two patterns take for "user:password"
// when it is a host and port followed by a path that holds the "@", as in
// rest:https://host:8000/repo@2.
var hostPortPathRe = regexp.MustCompile(`^[^:/]*:\d*/`)

// rclonePassRe matches a password parameter of an rclone connection string
// with its value, quoted or not.
var rclonePassRe = regexp.MustCompile(`([,:]\w*pass(?:word)?=)('(?:[^']|'')*'|"(?:[^"]|"")*"|[^,:]*)`)

// locationRedacted reports whether a repo location reached the import with its
// embedded credential already stripped by a plain export.
func locationRedacted(loc string) bool {
	return strings.Contains(loc, redactedValue)
}

// redactExportLocations strips URL-embedded credentials out of every repo
// location the envelope carries: the domain paths and off-site locations in
// the settings block, each place's base and each row's repo. A remote domain
// path is its home place's base plus a folder, so redacting the base alone
// would still hand the credential out.
//
// Applied to the plain export only. The credentialed variant already hands out
// every stored secret in the clear behind requireAuthForSecrets, so scrubbing
// there would leave the one export that is meant to be a complete, portable copy
// as the only one that is not.
func redactExportLocations(exp *settingsExport) {
	for _, loc := range []*string{
		&exp.Settings.ContainersPath, &exp.Settings.VMsPath, &exp.Settings.FlashPath,
		&exp.Settings.ConfigPath, &exp.Settings.FilesPath, &exp.Settings.ZFSPath,
		&exp.Settings.ContainersOffsite, &exp.Settings.VMsOffsite, &exp.Settings.FlashOffsite,
		&exp.Settings.ConfigOffsite, &exp.Settings.FilesOffsite, &exp.Settings.ZFSOffsite,
	} {
		*loc = scrubRepoLocation(*loc)
	}
	for i := range exp.Places {
		exp.Places[i].Base = scrubRepoLocation(exp.Places[i].Base)
	}
	for i := range exp.OffsiteTargets {
		exp.OffsiteTargets[i].Repo = scrubRepoLocation(exp.OffsiteTargets[i].Repo)
	}
	for i := range exp.NamedRepos {
		exp.NamedRepos[i].Repo = scrubRepoLocation(exp.NamedRepos[i].Repo)
	}
}

// redactedLocations names the repo-location slots in a file whose credential the
// exporting instance stripped, for the log line an apply writes.
func redactedLocations(exp settingsExport) []string {
	var out []string
	for _, f := range []struct{ name, loc string }{
		{"containersPath", exp.Settings.ContainersPath},
		{"vmsPath", exp.Settings.VMsPath},
		{"flashPath", exp.Settings.FlashPath},
		{"configPath", exp.Settings.ConfigPath},
		{"filesPath", exp.Settings.FilesPath},
		{"containersOffsite", exp.Settings.ContainersOffsite},
		{"vmsOffsite", exp.Settings.VMsOffsite},
		{"flashOffsite", exp.Settings.FlashOffsite},
		{"configOffsite", exp.Settings.ConfigOffsite},
		{"filesOffsite", exp.Settings.FilesOffsite},
		{"zfsOffsite", exp.Settings.ZFSOffsite},
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
	for _, tv := range exp.NamedRepos {
		if !locationRedacted(tv.Repo) {
			continue
		}
		name := strings.TrimSpace(tv.Name)
		if name == "" {
			name = strings.TrimSpace(tv.ID)
		}
		out = append(out, "repository "+name)
	}
	for _, p := range exp.Places {
		if locationRedacted(p.Base) {
			out = append(out, "storage place "+strings.TrimSpace(p.Name))
		}
	}
	return out
}

// handleExportSettings streams the portable settings file as a JSON download.
// GET /api/settings/export?includeCredentials=true|false (default false). The
// body is never logged.
//
// The plain export carries no secret: toView blanks the metrics and widget
// tokens, buildSettingsView drops the registry-auth list and the hook commands,
// and redactExportLocations strips the password a repo location can carry in
// its own URL. It is served behind the session authGate like every other /api
// route.
//
// The credentialed export hands out the decrypted S3 keys, the restic-REST
// password, the whole rclone config, the SMTP password and the Matrix access
// token, the recovery kit's class of payload. So it takes the recovery kit's
// second gate as well: auth has to be enabled. In trusted-LAN mode authGate
// lets every request through, and any host on the LAN could fetch every
// backend credential this instance holds with one GET.
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
	namedRepos, err := h.store.ListNamedRepos()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	defaults, err := h.store.ListPlacementDefaults()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	rules, err := h.store.ListCopyRules()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	placeRows, err := h.store.ListPlaces()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	homes, err := h.store.DomainPlaces()
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	exp := settingsExport{
		SchemaVersion:       settingsExportSchema,
		ExportedAt:          time.Now().UTC().Format(time.RFC3339),
		AppVersion:          Version,
		Settings:            buildSettingsView(s),
		OffsiteTargets:      offsiteTargetsToViews(targets),
		NamedRepos:          offsiteTargetsToViews(namedRepos),
		PlacementDefaults:   placementDefaultsToExport(defaults),
		CopyRules:           copyRulesToExport(rules),
		Places:              placesToExport(placeRows),
		StorageDomainPlaces: homes,
	}

	if withCredentials {
		creds, cErr := h.collectCredentials(s)
		if cErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(cErr))
			return
		}
		exp.Credentials = creds
	} else {
		// Without a credentials block the file must not carry one out inside a
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
		// Log only the failure, never the body: with credentials it holds the
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
	sets, err := h.svc.decodeCloudCredSets(s)
	if err != nil {
		return nil, fmt.Errorf("read credential sets: %w", err)
	}
	return &exportCredentials{Cloud: cloud, Rclone: rclone, Notify: notifyConf, CredSets: sets}, nil
}

// importSummary is the preview payload: what an apply would change.
type importSummary struct {
	SchemaVersion     int                 `json:"schemaVersion"`
	ExportedAt        string              `json:"exportedAt"`
	AppVersion        string              `json:"appVersion"`
	OffsiteTargets    int                 `json:"offsiteTargets"`
	NamedRepos        int                 `json:"namedRepos"`
	PlacementDefaults *int                `json:"placementDefaults"`
	CopyRules         *int                `json:"copyRules"`
	Places            *int                `json:"places"`
	NewTargets        []newTargetRow      `json:"newTargets"`
	Credentials       importCredsPresence `json:"credentials"`
	SettingsGroups    []string            `json:"settingsGroups"`
}

// importCredsPresence reports which credential kinds the file carries (never the
// values) and how many credential sets. All zero when the file has no
// credentials block.
type importCredsPresence struct {
	Present  bool `json:"present"`
	Cloud    bool `json:"cloud"`
	Rclone   bool `json:"rclone"`
	Notify   bool `json:"notify"`
	CredSets int  `json:"credSets"`
}

// handleImportSettings validates a settings-export file and, with ?apply=true,
// writes it. POST /api/settings/import (body = the export JSON). Without apply
// it returns a summary of what an apply would change and writes nothing; with
// it, credentials are re-encrypted with this instance's APP_KEY and the
// settings, rows, credentials and storage places are written.
//
// It never touches backup repositories, snapshots or run history. A missing
// credentials block leaves the stored secrets alone. An unsupported
// schemaVersion or a malformed file is refused with a clear error.
func (h *Handler) handleImportSettings(w http.ResponseWriter, r *http.Request) {
	exp, ok := decodeExport(w, r)
	if !ok {
		return
	}
	if msg := validateExport(exp, h.cfg.HostMountRoot); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	exp = placeFileLocations(exp)
	if msg := h.rejectImportCollisions(exp); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := h.checkImportedPlacement(exp); err != nil {
		placementFail(w, err, nil)
		return
	}

	apply := truthy(r.URL.Query().Get("apply"))
	if !apply {
		summary := summarizeExport(exp)
		summary.NewTargets = h.svc.importNewTargets(r.Context(), exp)
		writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
			"preview": true,
			"summary": summary,
		}))
		return
	}

	if err := h.applyImport(r.Context(), exp); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"applied": true,
		"summary": summarizeExport(exp),
	}))
}

// decodeExport reads the request body as a settingsExport. Unlike decodeBody it
// accepts fields it does not know, so a newer file of the same schema version
// still reads, but it refuses a malformed body.
func decodeExport(w http.ResponseWriter, r *http.Request) (settingsExport, bool) {
	var exp settingsExport
	// The import replaces the whole configuration, so it needs the guard
	// decodeBody gives every other write.
	if !crossOriginGuard(w, r) {
		return exp, false
	}
	if r.Body == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "missing request body"})
		return exp, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20) // a settings file, not a data blob
	if err := json.NewDecoder(r.Body).Decode(&exp); err != nil {
		if errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "empty import file"})
			return exp, false
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "malformed import file: expected a BombVault settings-export JSON"})
		return exp, false
	}
	return exp, true
}

// rejectImportCollisions refuses a file that would leave a named repository on
// a domain path, an off-site destination, an off-site target or another named
// repository, as the named-repository forms do. It returns a user-facing
// sentence, or "". It sits apart from validateExport, a pure function over the
// file, because it resolves locations through the running service.
//
// The check runs on the state the apply leaves behind, which differs from the
// file in four ways:
//
//   - a file with no namedRepos block keeps every stored row, since applyImport
//     calls replaceNamedRepos only for a non-empty block;
//   - a file row that is in use here keeps its stored location, since the move
//     goes through SetNamedRepoLocationIfUnused;
//   - a stored row in use here that the file lacks survives, since the delete
//     goes through DeleteNamedRepoIfUnused;
//   - a redacted location in the file can leave this instance's own in place
//     (importedLocation, restoredLocation).
//
// A collision this misses shows at that repository's first backup, or not at
// all: a repository whose append-only flag and credentials answer for a domain
// that never set them.
func (h *Handler) rejectImportCollisions(exp settingsExport) string {
	const unchecked = "could not check this file against the repositories already set up; try again"
	// One repository that exists after the apply. A file row is named by its
	// number in the file, a stored row the file lacks by its name.
	type repoRow struct{ label, loc string }
	// The locations the apply keeps go into rows, never into exp:
	// handleImportSettings passes exp on to the apply, where a pinned location
	// would hide its notice that a move was refused.
	rows := make([]repoRow, 0, len(exp.NamedRepos))
	stored, sErr := h.store.ListNamedRepos()
	if sErr != nil {
		return unchecked
	}
	storedByID := make(map[string]store.OffsiteTarget, len(stored))
	for _, r := range stored {
		storedByID[r.ID] = r
	}
	switch {
	case len(exp.NamedRepos) == 0:
		for _, r := range stored {
			rows = append(rows, repoRow{fmt.Sprintf("the repository %q already set up here", scrubSafeName(r.Name)), r.Repo})
		}
	default:
		inFile := make(map[string]bool, len(exp.NamedRepos))
		for i, tv := range exp.NamedRepos {
			id := strings.TrimSpace(tv.ID)
			inFile[id] = true
			loc := tv.Repo
			if cur, ok := storedByID[id]; ok && id != "" {
				// A row the apply cannot move keeps the location it has. Validating the
				// one the file asks for would pass a check the stored state then fails.
				n, cErr := h.store.ItemsUsingNamedRepo(id)
				if cErr != nil {
					// Refused rather than guessed: pinning the location on a read error
					// would validate one the apply may not write, and the response
					// would still say applied.
					return unchecked
				}
				if n != 0 {
					loc = cur.Repo
				} else {
					// An unused row moves, but a redacted location in the file leaves
					// the stored one, and that is the one that can collide.
					loc = importedLocation(cur.Repo, tv.Repo)
				}
			}
			rows = append(rows, repoRow{fmt.Sprintf("repository #%d", i+1), loc})
		}
		// The stored rows the apply keeps because they are in use and the file
		// lacks them.
		for _, r := range stored {
			if inFile[r.ID] {
				continue
			}
			n, cErr := h.store.ItemsUsingNamedRepo(r.ID)
			if cErr != nil {
				return unchecked
			}
			if n != 0 {
				rows = append(rows, repoRow{fmt.Sprintf("the repository %q already set up here", scrubSafeName(r.Name)), r.Repo})
			}
		}
	}
	resolve := func(loc string) (string, bool) {
		loc = strings.TrimSpace(loc)
		if loc == "" {
			return "", false
		}
		out, err := h.svc.resolveRepo(loc)
		if err != nil {
			return "", false // validateExport already refused what cannot resolve
		}
		return out, true
	}
	type place struct{ label, loc string }
	var occupied []place
	here, gErr := h.store.GetSettings()
	if gErr != nil {
		return unchecked
	}
	s := mergeImportedSettings(here, exp.Settings)
	offsite := exp.OffsiteTargets
	if exp.predatesZFS {
		// The apply keeps this instance's ZFS locations, so those are the ones
		// a repository in the file must not land on.
		keepZFSSettings(&s, here)
		targets, err := h.store.ListOffsiteTargets()
		if err != nil {
			return unchecked
		}
		for _, t := range targets {
			if t.Domain == zfsDomain {
				offsite = append(slices.Clip(offsite), offsiteTargetView{Repo: t.Repo})
			}
		}
	}
	for _, p := range []place{
		{"the Containers path", s.ContainersPath}, {"the VMs path", s.VMsPath},
		{"the Flash path", s.FlashPath}, {"the Config path", s.ConfigPath},
		{"the Folders path", s.FilesPath},
		{"the ZFS datasets path", s.ZFSPath},
		{"the Containers off-site destination", s.ContainersOffsite},
		{"the VMs off-site destination", s.VMsOffsite},
		{"the Flash off-site destination", s.FlashOffsite},
		{"the Config off-site destination", s.ConfigOffsite},
		{"the Folders off-site destination", s.FilesOffsite},
		{"the ZFS datasets off-site destination", s.ZFSOffsite},
	} {
		if loc, ok := resolve(p.loc); ok {
			occupied = append(occupied, place{p.label, loc})
		}
	}
	for _, tv := range offsite {
		loc := tv.Repo
		if locationRedacted(loc) {
			target, _, err := h.store.GetOffsiteTarget(strings.TrimSpace(tv.ID))
			if err != nil {
				return unchecked
			}
			loc = importedLocation(target.Repo, loc)
		}
		if loc, ok := resolve(loc); ok {
			occupied = append(occupied, place{"an off-site destination", loc})
		}
	}
	for i, a := range occupied {
		for _, b := range occupied[i+1:] {
			if repoLocationsOverlap(a.loc, b.loc) && !sameRepoLocation(a.loc, b.loc) {
				log.Printf("api: settings import: %s lies inside or around %s; imported anyway so older files still load", a.label, b.label) //nolint:gosec // G706: the labels are fixed text
			}
		}
	}
	if len(rows) == 0 {
		return ""
	}
	seen := make([]string, 0, len(rows))
	for _, row := range rows {
		loc, ok := resolve(row.loc)
		if !ok {
			continue
		}
		for _, p := range occupied {
			if sameRepoLocation(p.loc, loc) {
				return fmt.Sprintf("%s is at %s; a repository has to be a different place", row.label, p.label)
			}
			if repoLocationsOverlap(p.loc, loc) {
				log.Printf("api: settings import: %s lies inside or around %s; imported anyway so older files still load", row.label, p.label) //nolint:gosec // G706: the labels are fixed text around %q-quoted names
			}
		}
		for _, other := range seen {
			if sameRepoLocation(other, loc) {
				return fmt.Sprintf("%s names the same place as an earlier one; two rows over one repository would give every question about it two answers", row.label)
			}
			if repoLocationsOverlap(other, loc) {
				log.Printf("api: settings import: %s lies inside or around an earlier repository; imported anyway so older files still load", row.label) //nolint:gosec // G706: the label is fixed text around a %q-quoted name
			}
		}
		seen = append(seen, loc)
	}
	return ""
}

// validateExport checks that the file is a supported, well-formed export. It
// returns a user-facing error, or "" when the file is valid.
//
// Every check here is one the settings save enforces as well: an import must
// not store what the save then refuses. The SPA always PUTs the whole settings
// object, so one field this lets through and handlePutSettings rejects blocks
// every later save from every card, the card that would fix it included. The
// everyN, path and DR-target checks are the functions handlePutSettings calls,
// so extending one extends both.
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
	// Named repositories carry no domain, since an item picks one whatever its
	// domain, so they are checked against their own rules rather than the
	// off-site contract: a name, and every refusal in staticNamedRepoRefusals (a
	// location, no unprefixed rclone remote, a local path inside the mount
	// root). Those need nothing but the string and are the create form's own.
	//
	// The collision refusals need the running service to resolve every
	// location, so they live in rejectImportCollisions and run on the same
	// request. They sit apart because this function is pure over the file, not
	// because the import may skip them.
	for i, tv := range exp.NamedRepos {
		if strings.TrimSpace(tv.Name) == "" {
			return fmt.Sprintf("repository #%d: needs a name", i+1)
		}
		loc := strings.TrimSpace(tv.Repo)
		if loc == "" {
			return fmt.Sprintf("repository #%d: needs a location", i+1)
		}
		if msg := staticNamedRepoRefusals(loc, mountRoot); msg != "" {
			return fmt.Sprintf("repository #%d (%s): %s", i+1, tv.Name, msg)
		}
	}
	if exp.Credentials != nil {
		if msg := rejectInvalidCredSets(exp.Credentials.CredSets); msg != "" {
			return msg
		}
	}
	if msg := rejectInvalidPlaces(exp, mountRoot); msg != "" {
		return msg
	}
	// Every schedule cadence in the imported settings must parse, with the
	// grammar the settings save uses, so an apply cannot install a schedule
	// that never runs.
	for _, cad := range exportCadences(exp.Settings) {
		if _, err := schedule.ParseCadence(cad); err != nil {
			return "invalid schedule in settings: " + scrubError(err)
		}
	}
	// The everyN restriction of the settings save. An imported
	// "containersOffsiteSchedule": "everyN 3 04:00" would otherwise be stored
	// and then fail every later save of the whole Schedules tab. Sharing the
	// check also lets the drill, tamper-test and digest cadences that take
	// everyN import exactly where a UI-set one is accepted.
	if msg := rejectEveryNSchedules(exp.Settings); msg != "" {
		return "invalid schedule in settings: " + msg
	}
	// Repo locations. A file from a box with another mount root can carry an
	// absolute containersPath such as "/mnt/user/backups". Stored, it would fail
	// every later settings save with "must be a relative subpath under the mount
	// root", and no setting, the path included, could be changed without
	// editing the database.
	if msg := rejectInvalidSettingsPaths(exp.Settings, mountRoot); msg != "" {
		return "invalid path in settings: " + msg
	}
	// The DR-drill targets, checked as the save checks them.
	if msg := rejectInvalidSettingsNames(exp.Settings); msg != "" {
		return "invalid settings: " + msg
	}
	// The two anomaly presets, through the guard the save uses, so an import
	// cannot persist a value the Settings page then refuses to save.
	if msg := rejectInvalidAnomalySettings(exp.Settings); msg != "" {
		return "invalid settings: " + msg
	}
	return ""
}

// exportCadences lists every schedule a settings view carries. It follows the
// parse check in handlePutSettings: a cadence left out here imports unchecked.
func exportCadences(v settingsView) []string {
	return []string{
		v.ContainersSchedule, v.VMsSchedule, v.FlashSchedule, v.ConfigSchedule, v.FilesSchedule, v.ZFSSchedule,
		v.ContainersOffsiteSchedule, v.VMsOffsiteSchedule, v.FlashOffsiteSchedule, v.ConfigOffsiteSchedule, v.FilesOffsiteSchedule,
		v.ZFSOffsiteSchedule,
		v.DrillsSchedule, v.TamperTestSchedule, v.DigestSchedule, v.EverythingSchedule,
	}
}

// summarizeExport builds the preview/summary payload for a validated export.
func summarizeExport(exp settingsExport) importSummary {
	return importSummary{
		SchemaVersion:     exp.SchemaVersion,
		ExportedAt:        exp.ExportedAt,
		AppVersion:        exp.AppVersion,
		OffsiteTargets:    len(exp.OffsiteTargets),
		NamedRepos:        len(exp.NamedRepos),
		PlacementDefaults: countIfPresent(exp.PlacementDefaults),
		CopyRules:         countIfPresent(exp.CopyRules),
		Places:            countIfPresent(exp.Places),
		NewTargets:        []newTargetRow{},
		Credentials:       credsPresence(exp.Credentials),
		SettingsGroups:    settingsGroups(exp.Settings),
	}
}

// credsPresence reports which credential kinds the file carries.
func credsPresence(c *exportCredentials) importCredsPresence {
	if c == nil {
		return importCredsPresence{}
	}
	return importCredsPresence{
		Present:  true,
		Cloud:    cloudCredsMeaningful(c.Cloud),
		Rclone:   strings.TrimSpace(c.Rclone) != "",
		Notify:   notifyMeaningful(c.Notify),
		CredSets: len(c.CredSets),
	}
}

// settingsGroups names the groups of settings the imported view carries a value
// for, so the preview can say which areas an apply fills. It only describes: an
// apply writes the whole settings block either way.
func settingsGroups(v settingsView) []string {
	var groups []string
	add := func(name string, on bool) {
		if on {
			groups = append(groups, name)
		}
	}
	// The dump switch counts when it is off, the mirror image of the flags
	// above: it is on by default, so switching it off is what an apply imposes.
	add("domains", v.ContainersEnabled || v.VMsEnabled || v.FlashEnabled || v.ConfigEnabled || v.FilesEnabled || v.ZFSEnabled ||
		v.ContainersPath != "" || v.VMsPath != "" || v.FlashPath != "" || v.ConfigPath != "" || v.FilesPath != "" || v.ZFSPath != "" ||
		(v.DBDumpsEnabled != nil && !*v.DBDumpsEnabled))
	add("schedules", v.ContainersSchedule != "" || v.VMsSchedule != "" || v.FlashSchedule != "" ||
		v.ConfigSchedule != "" || v.FilesSchedule != "" || v.ZFSSchedule != "")
	// The whole-server pass is an area of its own: it is the one setting an
	// apply can switch on for a server that never ran it, so the preview names it.
	add("everything", v.EverythingSchedule != "")
	add("retention", v.RetentionKeepLast > 0 || v.RetentionKeepDaily > 0 || v.RetentionKeepWeekly > 0 || v.RetentionKeepMonthly > 0 ||
		v.OffsiteRetentionKeepLast > 0 || v.OffsiteRetentionKeepDaily > 0 || v.OffsiteRetentionKeepWeekly > 0 || v.OffsiteRetentionKeepMonthly > 0)
	add("offsite", v.ContainersOffsite != "" || v.VMsOffsite != "" || v.FlashOffsite != "" || v.ConfigOffsite != "" ||
		v.FilesOffsite != "" || v.ZFSOffsite != "")
	add("drills", v.DrillsEnabled || v.DrillsSchedule != "" || v.OffsiteDrillsEnabled)
	add("digest", v.DigestEnabled || v.DigestSchedule != "")
	add("monitoring", v.MetricsEnabled || v.WidgetTokenSet)
	add("language", v.DefaultLanguage != "")
	// Named only when the file departs from the defaults, so importing a file
	// that leaves detection as it ships says nothing about it.
	add("anomalies", (v.AnomalyEnabled != nil && !*v.AnomalyEnabled) ||
		(v.AnomalyRetentionHold != nil && !*v.AnomalyRetentionHold) ||
		(v.AnomalySensitivity != "" && v.AnomalySensitivity != string(sensBalanced)) ||
		(v.AnomalyNotifyMin != "" && v.AnomalyNotifyMin != "critical"))
	add("exportEncryption", v.ExportEncryptEnabled || v.ExportAgeRecipients != "")
	return groups
}

// applyImport writes a validated export: the settings row, a full replace of the
// off-site targets, any credentials (re-encrypted with the local APP_KEY) and the
// storage places. It never touches repos, snapshots or run history.
func (h *Handler) applyImport(ctx context.Context, exp settingsExport) error {
	installed, err := h.installedForImport(ctx, exp)
	if err != nil {
		return fmt.Errorf("the settings were not imported: %w", err)
	}
	// From the read of the places to their rebuild, so no place edit writes a
	// place back over the import and a second import waits for this rebuild.
	h.svc.placeEditMu.Lock()
	defer h.svc.placeEditMu.Unlock()
	// While the places are gone a domain path or a placed repository would age
	// by the retention in the settings row instead of its place's, so no
	// backup, copy or prune runs until they are back.
	locks := domainLocks{s: h.svc}
	release := sync.OnceFunc(locks.release)
	defer release()
	for _, d := range places.Domains {
		if err := locks.take(d); err != nil {
			return fmt.Errorf("the settings were not imported, %s is busy: %w", d, err)
		}
	}
	// Read before the drop: a place whose base arrives redacted keeps the base
	// it has here under the same id. A file without places rebuilds them only
	// on an instance that was on places, since moving an instance onto places
	// is the start's job.
	prior, err := h.store.ListPlaces()
	if err != nil {
		return fmt.Errorf("the settings were not imported: %w", err)
	}
	here, err := h.store.GetSettings()
	if err != nil {
		return fmt.Errorf("the settings were not imported: %w", err)
	}
	onPlaces := len(prior) > 0 || here.PlacesMigrated != 0
	rebuild := exp.Places == nil && onPlaces
	before, err := h.placesBefore(prior)
	if err != nil {
		return fmt.Errorf("the settings were not imported: %w", err)
	}
	// The steps below then write rows on no place, so none of the rules that
	// guard a placed row gets in their way. The places come back at the end,
	// from the file or from the migration over what it imported, and a write
	// that fails before then puts back the ones this instance had.
	if err := h.store.DropPlaces(); err != nil {
		return fmt.Errorf("the settings were not imported: %w", err)
	}
	if err := h.writeImport(exp, installed, prior); err != nil {
		if !onPlaces {
			return err
		}
		if pErr := h.putPlacesBack(before); pErr != nil {
			return errors.Join(err, fmt.Errorf("the storage places could not be put back, so this instance runs without places: %w", pErr))
		}
		return err
	}

	// Mirror the imported off-site config into the primary off-site target rows and
	// re-arm the scheduler, exactly like a settings save, so the imported schedules
	// take effect. A test wiring may have no scheduler.
	s, err := h.store.GetSettings()
	if err != nil {
		return err
	}
	if _, err := h.svc.moveTargetsOffPrimarySlot(s); err != nil {
		return err
	}
	h.svc.syncAllPrimaryOffsiteTargets(s)
	// The places are rebuilt before the reload, so a reload that fails leaves
	// no instance that had places on none.
	var placesErr error
	if rebuild {
		if err := h.svc.MigrateToPlaces(); err != nil {
			placesErr = fmt.Errorf("the settings were imported, but the storage places could not be built from them, so this instance runs without places: %w", err)
		}
	}
	if err := h.svc.placeSwitchedOnZFSLocked(); err != nil {
		log.Printf("places: could not give the ZFS path a place, it stays under Without a place: %v", err)
	}
	// A backup the reload finds due waits for its domain.
	release()
	if h.scheduler != nil {
		if err := h.scheduler.ReloadWithGates(s, h.dueGates()); err != nil {
			return errors.Join(placesErr, err)
		}
	}
	return placesErr
}

// writeImport writes what the file carries, the places last.
func (h *Handler) writeImport(exp settingsExport, installed store.Installed, prior []store.Place) error {
	// A hook command in the file is not installed (see mergeImportedSettings).
	// Saying so tells an operator moving to a new box that the hook is the one
	// part of Backup Everything that did not travel, before the night the
	// dead-man's-switch stays silent.
	if strings.TrimSpace(exp.Settings.EverythingPreHook) != "" || strings.TrimSpace(exp.Settings.EverythingPostHook) != "" {
		log.Print("api: settings import: the file carries Backup Everything pre/post-hook commands, which are not installed. " +
			"A hook is a shell command this host runs, so it is set on the instance, never by an imported file. " +
			"Enter it under Settings > Schedules > Backup Everything if you want it here.")
	}

	// Likewise for a location whose credential the export stripped: only the
	// operator can put the password back.
	if slots := redactedLocations(exp); len(slots) > 0 {
		log.Printf("api: settings import: these repo locations arrived with their embedded credential removed (%s); "+
			"a plain export never writes a password into a URL. An off-site location or repository this instance already has is kept, "+
			"and so is a domain path where the file names the same location; anywhere else the location lands with the marker in it. "+
			"Re-enter the credential in the repo URL, or export again with credentials included.", strings.Join(slots, ", "))
	}

	// The merge keeps the per-instance fields the file omits (login password,
	// session epoch, recovery-kit ack, registry auths, credential blobs). It runs
	// inside the transaction so they are read at write time: a password change
	// landing during a slow import must not be reverted by this write.
	var anomalyChanged bool
	if _, err := h.store.MutateSettings(func(cur *store.Settings) error {
		merged := mergeImportedSettings(*cur, exp.Settings)
		if exp.predatesZFS {
			keepZFSSettings(&merged, *cur)
		}
		anomalyChanged = anomalySettingsMoved(*cur, merged)
		*cur = merged
		return nil
	}); err != nil {
		return err
	}
	if anomalyChanged {
		h.svc.anomalies.settingsChanged()
	}

	// Replace the off-site targets with the imported set (a clean, deterministic
	// round-trip): drop the current rows, then upsert each imported target
	// preserving its id + created_at so the far instance reproduces the source.
	if err := h.replaceOffsiteTargets(exp.OffsiteTargets, exp.Settings, exp.predatesZFS); err != nil {
		return err
	}

	// The placement defaults and copy rules, written here so an imported default
	// already protects the named repository it points at before the delete loop
	// below runs.
	h.svc.placementMu.Lock()
	err := h.store.ImportPlacement(importedPlacement(exp), installed)
	h.svc.placementMu.Unlock()
	if err != nil {
		return err
	}

	// The named repositories the same way, but only when the file carries them.
	// An older file has no namedRepos block, and reading that as "the source had
	// none" would delete the repositories this instance uses and leave every
	// item pointing at an id that no longer exists.
	if len(exp.NamedRepos) > 0 {
		if err := h.replaceNamedRepos(exp.NamedRepos); err != nil {
			return err
		}
	}

	// Each credential kind the file fills is re-encrypted with the local key. An
	// empty kind or a missing block leaves the stored secret alone: the import
	// adds secrets and never wipes one.
	if exp.Credentials != nil {
		if err := h.applyImportedCredentials(*exp.Credentials); err != nil {
			return err
		}
	}

	// Last among the writes, so a place claims only what the steps above
	// stored; see placedImport.
	if exp.Places != nil {
		return h.replacePlaces(exp, prior)
	}
	return nil
}

// installedForImport reads the containers and VMs installed on the host when
// the file carries copy rules: a rule on an entry's former name moves to the
// entry only while nothing installed answers to that name.
func (h *Handler) installedForImport(ctx context.Context, exp settingsExport) (store.Installed, error) {
	if exp.CopyRules == nil {
		return store.Installed{}, nil
	}
	containers, err := h.svc.heldContainerNames(ctx)
	if err != nil {
		return store.Installed{}, err
	}
	vms, err := h.svc.heldVMNames(ctx)
	if err != nil {
		return store.Installed{}, err
	}
	return store.Installed{Containers: containers, VMs: vms}, nil
}

// offsiteRepoFromView reads a domain's off-site location straight off the
// export's own settings block, the view-typed twin of offsiteRepoFromSettings.
func offsiteRepoFromView(domain string, v settingsView) string {
	switch domain {
	case "containers":
		return v.ContainersOffsite
	case "vms":
		return v.VMsOffsite
	case "flash":
		return v.FlashOffsite
	case "config":
		return v.ConfigOffsite
	case "files":
		return v.FilesOffsite
	case "zfs":
		return v.ZFSOffsite
	}
	return ""
}

// replaceOffsiteTargets makes the off-site targets the file's set: a target
// the file carries is updated in place and keeps its id, created_at,
// observations and direct repository, one it lacks is deleted. The file's
// sort orders are then settled the way the offsite_targets_primary_slot
// migration settles them, against fileSettings, the file's own off-site fields,
// rather than the merged settings just written: a redacted field can be kept at
// this instance's working location by importedLocation while the row for it
// lands under a fresh id with nothing to keep, so the row's redacted repo would
// never match the merged field and the domain would end up with no row on
// sort_order 0. The file's field and its row were redacted the same way on
// export, so comparing the file against itself always matches. With keepZFS
// the ZFS destinations stay as they are, because the file cannot describe them.
func (h *Handler) replaceOffsiteTargets(views []offsiteTargetView, fileSettings settingsView, keepZFS bool) error {
	current, err := h.store.ListOffsiteTargets()
	if err != nil {
		return err
	}
	inFile := make(map[string]bool, len(views))
	for _, tv := range views {
		inFile[strings.TrimSpace(tv.ID)] = true
	}
	// Remember each row's location before any row changes: a location the file
	// carries redacted must not overwrite the working one this instance already
	// has for that id (see importedLocation).
	currentRepo := make(map[string]string, len(current))
	for _, t := range current {
		currentRepo[t.ID] = t.Repo
		if inFile[t.ID] || (keepZFS && t.Domain == zfsDomain) {
			continue
		}
		if err := h.store.DeleteOffsiteTarget(t.ID); err != nil {
			return err
		}
	}
	for _, tv := range views {
		t := tv.toStoreTarget()
		t.ID = strings.TrimSpace(tv.ID) // preserve the exported id (empty → store mints one)
		t.CreatedAt = tv.CreatedAt      // preserve the exported timestamp (0 → store stamps now)
		t.Repo = importedLocation(currentRepo[t.ID], t.Repo)
		saved, err := h.store.UpsertOffsiteTarget(t)
		if err != nil {
			return err
		}
		if saved.Role != store.RoleOffsite {
			log.Printf("api: settings import: off-site target %q has the id of a named repository here, so the file's copy was left unapplied", t.Name) //nolint:gosec // G706: the name is %q-quoted
		}
	}
	for _, d := range offsiteConfigDomains {
		if keepZFS && d == zfsDomain {
			continue
		}
		if err := h.store.NormalizeOffsiteSortOrder(d, offsiteRepoFromView(d, fileSettings)); err != nil {
			return err
		}
	}
	return nil
}

// replaceNamedRepos does for the named repositories (#204) what
// replaceOffsiteTargets does for the targets: it drops the current rows and
// inserts the file's, each under its own id, so the items that name one still
// find it.
//
// A repository in use stays when the file leaves it out. Deleting it would put
// its items back on their domain path and send their next backup elsewhere,
// which the delete route refuses, and an import must not get around that.
//
// A row the file adds may become a target's direct repository again through
// importedLink. A row that is already here keeps its link whatever the file
// says, since the upsert never rewrites companion_of of a stored id.
func (h *Handler) replaceNamedRepos(views []offsiteTargetView) error {
	current, err := h.store.ListNamedRepos()
	if err != nil {
		return err
	}
	stored := make(map[string]store.OffsiteTarget, len(current))
	imported := make(map[string]bool, len(views))
	for _, tv := range views {
		imported[strings.TrimSpace(tv.ID)] = true
	}
	for _, t := range current {
		stored[t.ID] = t
		if imported[t.ID] {
			continue // replaced below, id and all
		}
		// The delete route's own count-and-delete transaction.
		use, dErr := h.store.DeleteNamedRepoIfUnused(t.ID)
		if errors.Is(dErr, store.ErrDirectRepo) {
			log.Printf("api: settings import: repository %q is the direct repository of a target here, so it stays", t.Name) //nolint:gosec // G706: the name is %q-quoted
			continue
		}
		if dErr != nil {
			return dErr
		}
		if use.InUse() {
			log.Printf("api: settings import: repository %q is not in the imported file but still in use here (items: %d, defaults: %s), so it stays", t.Name, use.Items, strings.Join(use.DefaultDomains, ", ")) //nolint:gosec // G706: the name is %q-quoted
		}
	}
	for _, tv := range views {
		t := tv.toStoreTarget()
		t.Role = store.RoleRepo // toStoreTarget defaults to the off-site role
		t.Domain = ""           // a named repository belongs to no single domain
		t.ID = strings.TrimSpace(tv.ID)
		t.CreatedAt = tv.CreatedAt
		wanted := importedLocation(stored[t.ID].Repo, t.Repo)
		if tv.OffPremises == nil {
			// The file carries no opinion: an id this instance already has keeps
			// its stored mark, a brand new row takes the same default a plain
			// POST would.
			if _, known := stored[t.ID]; known {
				t.OffPremises = stored[t.ID].OffPremises
			} else {
				t.OffPremises = restic.IsRemoteRepo(wanted)
			}
		}
		// The location goes through the guarded transaction, like the delete
		// above, as the PATCH route requires: backups already written stay where
		// they are, so a moved location would have the next backup succeed into
		// an empty repository. Name, limits and flags move no data and take the
		// upsert.
		t.Repo = stored[t.ID].Repo
		if t.Repo == "" {
			t.Repo = wanted // a row this instance does not have yet: nothing to move
		}
		if stored[t.ID].Repo == "" {
			// Only a row this instance does not have yet may claim the target
			// named in its file entry.
			var err error
			if t.CompanionOf, t.CompanionLost, err = h.importedLink(tv.CompanionOf); err != nil {
				return err
			}
			if t.CompanionLost {
				log.Printf("api: settings import: repository %q belonged to a target that is not here; imported as a plain repository", t.Name) //nolint:gosec // G706: the name is %q-quoted
			}
		}
		saved, err := h.store.UpsertOffsiteTarget(t)
		if err != nil {
			return err
		}
		if saved.Role != store.RoleRepo {
			log.Printf("api: settings import: repository %q has the id of an off-site target here, so the file's copy was left unapplied", t.Name) //nolint:gosec // G706: the name is %q-quoted
		}
		if t.Repo == wanted {
			continue
		}
		// A direct repository keeps its location the way it keeps its other
		// mirrored fields: its target writes there, and the PATCH refuses the
		// same move.
		if stored[t.ID].CompanionOf != "" {
			log.Printf("api: settings import: repository %q takes its location from its target, so the file's %s was not applied", t.Name, shortRepoName(wanted)) //nolint:gosec // G706: the name is %q-quoted and the location is shortened
			continue
		}
		// The mark describes the location and moves with it; a file that carries
		// one is taken at its word.
		mark := restic.IsRemoteRepo(wanted)
		if tv.OffPremises != nil {
			mark = *tv.OffPremises
		}
		use, mErr := h.store.SetNamedRepoLocationIfUnused(t.ID, wanted, mark)
		if mErr != nil {
			return mErr
		}
		if use.InUse() {
			log.Printf("api: settings import: repository %q is in use here (items: %d, defaults: %s), so its location was not moved to the one in the file; the backups already written stay where they are", t.Name, use.Items, strings.Join(use.DefaultDomains, ", ")) //nolint:gosec // G706: the name is %q-quoted
		}
	}
	return nil
}

// importedLocation picks the repo location an apply writes into one slot: the
// file's, unless that one arrived redacted by a plain export and this instance
// already has a location there. Writing "rest:https://[redacted]@host:8000/repo"
// over a working location would quietly break the replication this instance
// already runs, and nothing else in the file says a credential was removed;
// applyImport logs which slots arrived redacted.
//
// An empty slot takes the redacted value rather than staying empty: the marker
// shows in Settings and the next run fails against a location the operator can
// repair by typing the password back in, where a blank off-site location just
// stops replicating without a word.
func importedLocation(existing, imported string) string {
	if locationRedacted(imported) && strings.TrimSpace(existing) != "" {
		return existing
	}
	return imported
}

// restoredLocation is importedLocation for a slot that is never empty on this
// instance, such as a domain path at its local default. It keeps existing
// only when the redacted file names that same location, so a redacted remote
// location lands visibly instead of losing to a default.
func restoredLocation(existing, imported string) string {
	if locationRedacted(imported) && scrubRepoLocation(existing) == imported {
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
	if len(c.CredSets) > 0 {
		if err := h.svc.importCredSets(c.CredSets); err != nil {
			return fmt.Errorf("store credential sets: %w", err)
		}
	}
	return nil
}

// importCredSets adds the file's credential sets to the stored ones, a set
// with a stored id taking its place the way a save of that set would. Sets the
// file does not carry stay: pull sources name sets a settings file knows
// nothing about.
func (s *Service) importCredSets(sets []CloudCredSet) error {
	return s.editCloudCredSets(func(stored []CloudCredSet) []CloudCredSet {
		for _, in := range sets {
			in.Name = strings.TrimSpace(in.Name)
			in.S3StorageClass = strings.ToUpper(strings.TrimSpace(in.S3StorageClass))
			i := slices.IndexFunc(stored, func(c CloudCredSet) bool { return c.ID == in.ID })
			if i < 0 {
				stored = append(stored, in)
				continue
			}
			stored[i] = keepStoredValues(stored[i], in)
		}
		return stored
	})
}

// rejectInvalidCredSets applies the refusals SetCloudCredSets applies to a
// save. It returns a user-facing sentence, or "".
func rejectInvalidCredSets(sets []CloudCredSet) string {
	seen := make(map[string]bool, len(sets))
	for i, set := range sets {
		name := strings.TrimSpace(set.Name)
		switch {
		case name == "":
			return fmt.Sprintf("credential set #%d: needs a name", i+1)
		case set.ID == "":
			return fmt.Sprintf("credential set #%d (%s): needs an id", i+1, name)
		case seen[set.ID]:
			return fmt.Sprintf("credential set #%d (%s): its id is used twice", i+1, name)
		}
		seen[set.ID] = true
		if class := strings.ToUpper(strings.TrimSpace(set.S3StorageClass)); class != "" && !restic.StorageClassAllowed(class) {
			return fmt.Sprintf("credential set #%d (%s): unsupported S3 storage class %s (allowed: %s)", i+1, name, class, strings.Join(restic.AllowedStorageClasses, ", "))
		}
	}
	return ""
}

// keepZFSSettings puts the instance's own ZFS setup back over a merge from a
// file that predates the domain, which would otherwise switch it off.
func keepZFSSettings(out *store.Settings, existing store.Settings) {
	out.ZFSEnabled = existing.ZFSEnabled
	out.ZFSPath = existing.ZFSPath
	out.ZFSSchedule = existing.ZFSSchedule
	out.ZFSOffsite = existing.ZFSOffsite
	out.ZFSOffsiteSchedule = existing.ZFSOffsiteSchedule
	out.ZFSOffsiteImmutable = existing.ZFSOffsiteImmutable
}

// mergeImportedSettings maps the imported view onto a Settings row, keeping the
// per-instance fields the export leaves out and clamping numbers the way the
// settings save does. The metrics and widget tokens arrive blank in the view
// and keep their stored value: an import never wipes them.
//
// It starts from the existing row and overwrites the portable fields, so a
// column nobody lists here keeps its value. A fresh store.Settings would write
// a zero into every unlisted column instead, switching off Backup Everything or
// clearing the instance name without an error or a hint in the preview.
//
// EverythingPreHook and EverythingPostHook are never taken from the file. They
// are shell commands this host runs (HostShell, via `sh -c`), and a settings
// file is something people mail each other and download from forum threads,
// so importing them would let it install any command on the box. They are kept
// from this instance like the credential fields; applyImport logs when a file
// carried them.
func mergeImportedSettings(existing store.Settings, v settingsView) store.Settings {
	out := existing

	// Kept from this instance because out starts as its row: the login password
	// hash, the session epoch, the recovery-kit acknowledgement, the
	// registry-auth blob, the metrics and widget tokens, the fleet token, fleet
	// switch and instance name, the encrypted credential blobs
	// (applyImportedCredentials writes those when the file carries them) and
	// the two Backup Everything hook commands. Every assignment below is a
	// field the file may set.

	out.EncryptionEnabled = v.EncryptionEnabled
	out.ContainersEnabled = v.ContainersEnabled
	out.VMsEnabled = v.VMsEnabled
	out.FlashEnabled = v.FlashEnabled
	out.ConfigEnabled = v.ConfigEnabled
	out.FilesEnabled = v.FilesEnabled
	out.ZFSEnabled = v.ZFSEnabled
	// Domain paths through restoredLocation: a plain export redacts a remote
	// one, and the local default a fresh instance holds is no location to keep
	// over it.
	out.ContainersPath = restoredLocation(existing.ContainersPath, v.ContainersPath)
	out.VMsPath = restoredLocation(existing.VMsPath, v.VMsPath)
	out.FlashPath = restoredLocation(existing.FlashPath, v.FlashPath)
	out.ConfigPath = restoredLocation(existing.ConfigPath, v.ConfigPath)
	out.FilesPath = restoredLocation(existing.FilesPath, v.FilesPath)
	out.ZFSPath = restoredLocation(existing.ZFSPath, v.ZFSPath)
	out.RestoreFolder = v.RestoreFolder
	// Off-site locations, via importedLocation: a location the plain export
	// stripped a credential out of never overwrites a working one here.
	out.ContainersOffsite = importedLocation(existing.ContainersOffsite, v.ContainersOffsite)
	out.VMsOffsite = importedLocation(existing.VMsOffsite, v.VMsOffsite)
	out.FlashOffsite = importedLocation(existing.FlashOffsite, v.FlashOffsite)
	out.ConfigOffsite = importedLocation(existing.ConfigOffsite, v.ConfigOffsite)
	out.FilesOffsite = importedLocation(existing.FilesOffsite, v.FilesOffsite)
	out.ZFSOffsite = importedLocation(existing.ZFSOffsite, v.ZFSOffsite)
	out.ContainersOffsiteSchedule = v.ContainersOffsiteSchedule
	out.VMsOffsiteSchedule = v.VMsOffsiteSchedule
	out.FlashOffsiteSchedule = v.FlashOffsiteSchedule
	out.ConfigOffsiteSchedule = v.ConfigOffsiteSchedule
	out.FilesOffsiteSchedule = v.FilesOffsiteSchedule
	out.ZFSOffsiteSchedule = v.ZFSOffsiteSchedule
	out.ContainersSchedule = v.ContainersSchedule
	out.VMsSchedule = v.VMsSchedule
	out.FlashSchedule = v.FlashSchedule
	out.ConfigSchedule = v.ConfigSchedule
	out.FilesSchedule = v.FilesSchedule
	out.ZFSSchedule = v.ZFSSchedule
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
	out.ZFSOffsiteImmutable = v.ZFSOffsiteImmutable
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
	// An export written before the switch existed carries no value for it, and
	// taking that as "off" would stop dumping databases on the instance the
	// file is applied to.
	if v.DBDumpsEnabled != nil {
		out.DBDumpsEnabled = *v.DBDumpsEnabled
	}
	// Same contract as the dump switch, and the same reason: a file written
	// before this version carries none of the four, and reading that as "off"
	// would take detection and the data-loss pause with it.
	applyAnomalySettings(&out, v)

	return out
}

// cloudCredsMeaningful reports whether any cloud credential field is set.
func cloudCredsMeaningful(c CloudCreds) bool {
	return c != CloudCreds{}
}

// notifyMeaningful reports whether a notify config carries a channel or a
// policy, which is whether storing it would do anything: SetNotifyConfig clears
// an empty one.
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
