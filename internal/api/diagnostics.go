package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/logring"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// diagFile is one member of the bundle: a name and the bytes behind it.
type diagFile struct {
	Name string
	Data []byte
}

// diagManifest is the bundle's own cover sheet. It exists so a recipient can
// tell at a glance what the file is, which version produced it, and above all
// that it is REDACTED: without that line a support bundle looks exactly like a
// configuration backup, and someone will eventually try to restore from one.
type diagManifest struct {
	Product   string   `json:"product"`
	Version   string   `json:"version"`
	CreatedAt string   `json:"createdAt"`
	Redacted  bool     `json:"redacted"`
	RemovedOn []string `json:"removedOnPurpose"`
	MCP       diagMCP  `json:"mcp"`
	Notes     []string `json:"notes"`
}

// diagMCP is everything the bundle says about the MCP keys: how many there are,
// how many may start backups, how many an APP_KEY change broke, when a client
// last used one, and how many calls and refusals their logs hold. A key's name
// is the operator's own word for one of their machines and the hint identifies
// the key itself, so neither belongs in a file written to be attached to a bug
// report, and neither does the log itself.
type diagMCP struct {
	ActiveKeys         int   `json:"activeKeys"`
	KeysAllowedToStart int   `json:"keysAllowedToStart"`
	UnusableKeys       int   `json:"unusableKeys"`
	LastUsedAt         int64 `json:"lastUsedAt"`
	ActivityEvents     int   `json:"activityEvents"`
	ActivityRefusals   int   `json:"activityRefusals"`
}

// mcpDiagnostics counts the keys for the manifest.
func (h *Handler) mcpDiagnostics() (diagMCP, error) {
	rows, err := h.store.ListMCPKeys()
	if err != nil {
		return diagMCP{}, err
	}
	var d diagMCP
	d.ActivityEvents, d.ActivityRefusals, err = h.store.MCPKeyEventTotals()
	if err != nil {
		return diagMCP{}, err
	}
	for _, k := range rows {
		if k.LastUsedAt > d.LastUsedAt {
			d.LastUsedAt = k.LastUsedAt
		}
		if k.RevokedAt != 0 {
			continue
		}
		d.ActiveKeys++
		if k.CanStartBackups {
			d.KeysAllowedToStart++
		}
		if h.mcpKeyUnusable(k) != "" {
			d.UnusableKeys++
		}
	}
	return d, nil
}

// diagLogCap bounds the log member. The ring itself is larger; this is what a
// bundle carries, chosen so the file stays mailable.
const diagLogCap = 128 << 10

// diagDBDump is one recognised database container: what would be dumped, what
// stands in the way, and how the last attempt ended. LastReason is the stored
// reason constant without the detail behind it, which is the dump tool's own
// message and can quote a row of the database.
type diagDBDump struct {
	Container       string `json:"container"`
	Tier            string `json:"tier"`
	Engine          string `json:"engine"`
	SuggestedEngine string `json:"suggestedEngine,omitempty"`
	ChosenEngine    string `json:"chosenEngine,omitempty"`
	DumpOff         bool   `json:"dumpOff"`
	LabelOff        bool   `json:"labelOff"`
	GlobalOff       bool   `json:"globalOff"`
	DataCoverage    string `json:"dataCoverage"`
	PreHookDumps    bool   `json:"preHookDumps"`
	LastStatus      string `json:"lastStatus,omitempty"`
	LastAt          int64  `json:"lastAt,omitempty"`
	LastReason      string `json:"lastReason,omitempty"`
}

// diagZFS is what the ZFS domain looks like without asking the host: the items
// and how their last look at the tree ended, the propagation check the
// container can make on its own, and the zfs lines of its mount table.
type diagZFS struct {
	Enabled      bool          `json:"enabled"`
	Items        []diagZFSItem `json:"items"`
	Propagation  string        `json:"propagation"`
	Unpropagated []string      `json:"unpropagated"`
	Mounts       []string      `json:"mounts"`
}

// diagZFSItem is one item with the state its last preflight or run left.
type diagZFSItem struct {
	Dataset          string          `json:"dataset"`
	Enabled          bool            `json:"enabled"`
	CheckCode        string          `json:"checkCode,omitempty"`
	CheckDetail      string          `json:"checkDetail,omitempty"`
	CheckAt          int64           `json:"checkAt,omitempty"`
	HostMountpoint   string          `json:"hostMountpoint,omitempty"`
	ExcludedChildren []string        `json:"excludedChildren,omitempty"`
	LeftoverCount    int             `json:"leftoverCount"`
	SafetyCount      int             `json:"safetyCount"`
	Members          []diagZFSMember `json:"members"`
}

// diagZFSMember is one dataset of an item's tree as the last run left it.
type diagZFSMember struct {
	Dataset string `json:"dataset"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

// zfsDiagnostics describes the ZFS domain from the database and the container's
// own mount table. It reaches no host: a support bundle must not hang on an SSH
// session, and the machine being unreachable is often why it is being written.
func (h *Handler) zfsDiagnostics() (diagZFS, error) {
	settings, err := h.store.GetSettings()
	if err != nil {
		return diagZFS{}, err
	}
	out := diagZFS{Enabled: settings.ZFSEnabled, Items: []diagZFSItem{}, Propagation: "ok", Unpropagated: []string{}}
	recs := zfsMountRecords()
	if top, nested := zfs.Propagation(recs, h.cfg.HostMountRoot); !top || len(nested) > 0 {
		out.Propagation = "propagation-missing"
		out.Unpropagated = nested
	}
	out.Mounts = zfsMountLines(recs)

	items, err := h.store.ListZFSDatasets()
	if err != nil {
		return out, err
	}
	for _, d := range items {
		row := diagZFSItem{
			Dataset: d.Dataset, Enabled: d.Enabled,
			CheckCode: d.LastCheckCode, CheckDetail: scrubSecrets(d.LastCheckDetail), CheckAt: d.LastCheckAt,
			HostMountpoint: d.LastHostMountpoint, ExcludedChildren: d.ExcludedChildren,
			LeftoverCount: d.LeftoverCount, Members: []diagZFSMember{},
		}
		if safety, sErr := h.store.ListZFSSafetySnapshots(d.ID); sErr == nil {
			row.SafetyCount = len(safety)
		}
		if members, mErr := h.store.ListZFSMembers(d.ID); mErr == nil {
			for _, m := range members {
				row.Members = append(row.Members, diagZFSMember{
					Dataset: m.Dataset, Outcome: m.Outcome, Detail: scrubSecrets(m.Detail),
				})
			}
		}
		out.Items = append(out.Items, row)
	}
	slices.SortFunc(out.Items, func(a, b diagZFSItem) int { return strings.Compare(a.Dataset, b.Dataset) })
	return out, nil
}

// zfsMountLines renders the zfs entries of the container's mount table, so a
// missing dataset mount can be read off the bundle.
func zfsMountLines(recs []zfs.MountRecord) []string {
	out := []string{}
	for _, r := range recs {
		if r.FSType != "zfs" {
			continue
		}
		out = append(out, r.Source+" -> "+r.MountPoint+" ("+strings.Join(r.Options, ",")+")")
	}
	return out
}

// dbDumpDiagnostics describes every container this host runs a database in,
// sorted by name so two bundles of the same box can be diffed.
func (h *Handler) dbDumpDiagnostics(ctx context.Context) ([]diagDBDump, error) {
	settings, err := h.store.GetSettings()
	if err != nil {
		return nil, err
	}
	infos, err := h.docker.List(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := h.store.ListTargets()
	if err != nil {
		return nil, err
	}
	byName := make(map[string]store.Target, len(targets))
	for _, t := range targets {
		byName[t.ContainerName] = t
	}

	out := []diagDBDump{}
	for name, db := range h.svc.dbDumpRows(ctx, infos, byName) {
		t := byName[name]
		row := diagDBDump{
			Container: name, Tier: db.Tier, Engine: db.Engine, SuggestedEngine: db.Suggested,
			ChosenEngine: t.DBDumpEngine, DumpOff: t.DBDumpOff, LabelOff: db.LabelOff,
			GlobalOff: !settings.DBDumpsEnabled, DataCoverage: db.Coverage, PreHookDumps: db.HookOverlap,
		}
		if last := h.lastDBDump(t.ID); last != nil {
			row.LastStatus, row.LastAt, row.LastReason = last.Status, last.At, dbDumpReasonHead(last.Error)
		}
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b diagDBDump) int { return strings.Compare(a.Container, b.Container) })
	return out, nil
}

// handleDiagnostics streams a redacted support bundle as a ZIP.
// GET /api/diagnostics
//
// Gated by requireAuthForSecrets, the same gate the recovery kit and the
// credentialed settings export use. authGate alone is a deliberate pass-through
// in trusted-LAN mode, and this file carries the whole configuration plus the
// recent log: not credentials, but far more than the read API's usual fare, and
// the one file most likely to be posted in public.
//
// Never age-sealed, even when export encryption is on. The flash
// download and the plain exports seal because they are the user's own data
// going somewhere the user controls. This file's entire purpose is to be opened
// by somebody else, and a sealed bundle the recipient cannot read would defeat
// it. The redaction, not encryption, is what makes it safe to send.
//
// Buffered rather than streamed: a bundle is kilobytes, and building it in
// memory means a failure halfway through still answers a clean JSON envelope
// instead of a truncated file with a 200 on it.
func (h *Handler) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if !h.requireAuthForSecrets(w, "downloading the diagnostics bundle") {
		return
	}

	files, err := h.buildDiagnostics(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		fw, cErr := zw.Create(f.Name)
		if cErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("building the bundle: %w", cErr)))
			return
		}
		if _, wErr := fw.Write(f.Data); wErr != nil {
			writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("building the bundle: %w", wErr)))
			return
		}
	}
	if cErr := zw.Close(); cErr != nil {
		writeJSON(w, http.StatusOK, failEnvelope(fmt.Errorf("building the bundle: %w", cErr)))
		return
	}

	name := "bombvault-diagnostics-" + time.Now().Format("2006-01-02") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if _, err := w.Write(buf.Bytes()); err != nil {
		// The body is never logged, only the fact that writing it failed:
		// the same rule the settings export follows.
		log.Printf("api: diagnostics: writing the bundle failed: %v", err)
	}
}

// buildDiagnostics assembles the members. Pure apart from its reads: it takes no
// ResponseWriter, so the redaction can be tested without a router.
//
// No single failing read fails the bundle. A support file that refuses to be
// produced because one query returned an error is worthless precisely when it
// is needed, so a failure is recorded as the member's content and the rest is
// still collected.
func (h *Handler) buildDiagnostics(ctx context.Context) ([]diagFile, error) {
	var files []diagFile

	add := func(name string, v any, err error) {
		if err != nil {
			files = append(files, diagFile{Name: name, Data: []byte(`{"error":` + strconv.Quote(scrubSecrets(err.Error())) + `}`)})
			return
		}
		b, mErr := json.MarshalIndent(v, "", "  ")
		if mErr != nil {
			files = append(files, diagFile{Name: name, Data: []byte(`{"error":` + strconv.Quote(mErr.Error()) + `}`)})
			return
		}
		files = append(files, diagFile{Name: name, Data: b})
	}

	// manifest.json: what this file is.
	mcpCounts, mErr := h.mcpDiagnostics()
	if mErr != nil {
		log.Printf("api: diagnostics: reading the MCP key counts failed: %v", mErr)
	}
	add("manifest.json", diagManifest{
		Product:   "BombVault",
		Version:   Version,
		CreatedAt: time.Now().Format(time.RFC3339),
		Redacted:  true,
		RemovedOn: []string{
			"login password hash, TOTP secret and recovery codes",
			"metrics, widget and fleet tokens",
			"the rclone config, S3/REST credentials and named credential sets",
			"notification credentials (SMTP password, Matrix token)",
			"registry authentications",
			"the Backup Everything pre/post hook commands",
			"passwords embedded in repository locations",
			"MCP keys, their names and what each key did (only counts are included)",
		},
		MCP: mcpCounts,
		Notes: []string{
			"This is a support bundle, not a configuration backup. Do not restore from it.",
			"log.txt holds this process's recent output. After a container restart it starts empty; use `docker logs` for the run that failed.",
		},
	}, nil)

	// spike.json: the host-integration check. Read from the cache the way
	// handleSpikeCached does; the probes shell out, so a bundle must not be a
	// way to re-run them on every request.
	h.spikeMu.RLock()
	ran, checks, allOK := h.spikeRan, h.spikeChecks, h.spikeAllOK
	h.spikeMu.RUnlock()
	if !ran {
		checks, allOK = h.runSpikeAndCache()
	}
	add("spike.json", map[string]any{"checks": checks, "allOK": allOK}, nil)

	// settings.json, through the export's own redaction, so this file and the
	// portable export can never disagree about what counts as a secret.
	s, sErr := h.store.GetSettings()
	if sErr != nil {
		add("settings.json", nil, sErr)
	} else {
		exp := settingsExport{Settings: buildSettingsView(s)}
		redactExportLocations(&exp)
		add("settings.json", exp.Settings, nil)
	}

	// dbdump.json holds which databases are dumped and how the last dump went.
	dumps, dErr := h.dbDumpDiagnostics(ctx)
	add("dbdump.json", dumps, dErr)

	// zfs.json holds the ZFS items, their trees and what the container can see
	// of the host's mounts.
	zfsDiag, zErr := h.zfsDiagnostics()
	add("zfs.json", zfsDiag, zErr)

	// runs.json: recent history. Run.Error holds raw restic/rclone/Docker
	// output, which handleRuns can serve as-is because it sits behind the
	// session gate. A file meant to be attached to a public bug report cannot,
	// so every error goes through scrubSecrets here, and a dump or an import
	// loses the database tool's own message. The name of the MCP key behind a
	// run goes the same way; the id stays, and the card maps it back.
	runs, rErr := h.store.ListRuns(200)
	if rErr != nil {
		add("runs.json", nil, rErr)
	} else {
		for i := range runs {
			runs[i].Error = scrubSecrets(shareableRunError(runs[i].Kind, runs[i].Error))
		}
		views := h.runViews(runs)
		for i := range views {
			views[i].StartedViaLabel = ""
		}
		add("runs.json", views, nil)
	}

	// anomalies.json holds what detection currently has against the history:
	// the figures the page polls and the open findings behind them. The rows
	// carry ids, metrics and numbers, and no name the runs member does not
	// already give away.
	page, aErr := h.svc.ListAnomalies(ctx, store.AnomalyFilter{Limit: store.MaxAnomalyLimit})
	if aErr != nil {
		add("anomalies.json", nil, aErr)
	} else {
		add("anomalies.json", map[string]any{
			"summary": h.svc.AnomalySummary(ctx), "open": page.Anomalies,
		}, nil)
	}

	// scheduler.json: what is planned next, nil-guarded the way
	// handleScheduleNext is.
	if h.scheduler == nil {
		add("scheduler.json", map[string]any{"next": []any{}}, nil)
	} else {
		add("scheduler.json", map[string]any{"next": h.scheduler.NextRuns()}, nil)
	}

	// log.txt: the recent output, scrubbed like the run errors and capped so
	// the bundle stays mailable.
	logText := scrubLogLines(logring.Default.String())
	if len(logText) > diagLogCap {
		logText = "[…older output dropped…]\n" + logText[len(logText)-diagLogCap:]
	}
	files = append(files, diagFile{Name: "log.txt", Data: []byte(logText)})

	return files, nil
}

// logDatePrefix is the date the standard logger starts every line with. Its
// slashes read as a path to the scrubber.
var logDatePrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} `)

// scrubLogLines scrubs the log line by line and leaves each line's date as it
// is, so lines of different days can still be told apart.
func scrubLogLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		date := logDatePrefix.FindString(line)
		lines[i] = date + scrubSecrets(line[len(date):])
	}
	return strings.Join(lines, "\n")
}
