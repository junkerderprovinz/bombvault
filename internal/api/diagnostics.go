package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/logring"
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
	Notes     []string `json:"notes"`
}

// diagLogCap bounds the log member. The ring itself is larger; this is what a
// bundle carries, chosen so the file stays mailable.
const diagLogCap = 128 << 10

// handleDiagnostics streams a redacted support bundle as a ZIP.
// GET /api/diagnostics
//
// Gated by requireAuthForSecrets, the same gate the recovery kit and the
// credentialed settings export use. authGate alone is a deliberate pass-through
// in trusted-LAN mode, and this file carries the whole configuration plus the
// recent log: not credentials, but far more than the read API's usual fare, and
// the one file most likely to be posted in public.
//
// Deliberately NOT age-sealed, even when export encryption is on. The flash
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

	files, err := h.buildDiagnostics()
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
		// The body is never logged, only the fact that writing it failed —
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
func (h *Handler) buildDiagnostics() ([]diagFile, error) {
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

	// manifest.json — what this file is.
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
		},
		Notes: []string{
			"This is a support bundle, not a configuration backup. Do not restore from it.",
			"log.txt holds this process's recent output. After a container restart it starts empty; use `docker logs` for the run that failed.",
		},
	}, nil)

	// spike.json — the host-integration check. Read from the cache the way
	// handleSpikeCached does; the probes shell out, so a bundle must not be a
	// way to re-run them on every request.
	h.spikeMu.RLock()
	ran, checks, allOK := h.spikeRan, h.spikeChecks, h.spikeAllOK
	h.spikeMu.RUnlock()
	if !ran {
		checks, allOK = h.runSpikeAndCache()
	}
	add("spike.json", map[string]any{"checks": checks, "allOK": allOK}, nil)

	// settings.json — through the export's own redaction, so this file and the
	// portable export can never disagree about what counts as a secret.
	s, sErr := h.store.GetSettings()
	if sErr != nil {
		add("settings.json", nil, sErr)
	} else {
		exp := settingsExport{Settings: buildSettingsView(s)}
		redactExportLocations(&exp)
		add("settings.json", exp.Settings, nil)
	}

	// runs.json — recent history. Run.Error holds raw restic/rclone/Docker
	// output, which handleRuns can serve as-is because it sits behind the
	// session gate. A file meant to be attached to a public bug report cannot,
	// so every error goes through scrubSecrets here.
	runs, rErr := h.store.ListRuns(200)
	if rErr != nil {
		add("runs.json", nil, rErr)
	} else {
		names, domains := h.runTargetMaps()
		views := make([]runView, 0, len(runs))
		for _, run := range runs {
			run.Error = scrubSecrets(run.Error)
			views = append(views, runView{Run: run, Target: names[run.TargetID], Domain: domains[run.TargetID]})
		}
		add("runs.json", views, nil)
	}

	// scheduler.json — what is planned next, nil-guarded the way
	// handleScheduleNext is.
	if h.scheduler == nil {
		add("scheduler.json", map[string]any{"next": []any{}}, nil)
	} else {
		add("scheduler.json", map[string]any{"next": h.scheduler.NextRuns()}, nil)
	}

	// log.txt — the recent output, scrubbed like the run errors and capped so
	// the bundle stays mailable.
	logText := scrubSecrets(logring.Default.String())
	if len(logText) > diagLogCap {
		logText = "[…older output dropped…]\n" + logText[len(logText)-diagLogCap:]
	}
	files = append(files, diagFile{Name: "log.txt", Data: []byte(logText)})

	return files, nil
}
