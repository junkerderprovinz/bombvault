package api_test

// GET /api/diagnostics: the support bundle.
//
// A file made to be attached to a bug report is the worst possible place for a
// credential, and it is also the file most likely to be posted in public. So it
// is gated like the recovery kit, and every member inside it is checked, not
// just the response as a whole: a ZIP is a container, and "the secret is not in
// the response body" is trivially true of any compressed archive.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/logring"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zipMembers unpacks the response and returns each member's name and contents,
// so assertions run against what a recipient actually opens.
func zipMembers(t *testing.T, raw []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("the bundle is not a readable zip: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		out[f.Name] = string(b)
	}
	return out
}

// TestDiagnosticsRefusedWhenAuthDisabled: the bundle carries the whole
// configuration and the recent log, so it fails closed exactly as the recovery
// kit does rather than being fetchable by anyone on the LAN.
func TestDiagnosticsRefusedWhenAuthDisabled(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	seedExportSecrets(t, svc)

	w := getRaw(t, h, "/api/diagnostics", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 with auth off", w.Code)
	}
	if !strings.Contains(w.Body.String(), "login password") {
		t.Fatalf("a refusal must say what to do, got %s", w.Body.String())
	}
	if strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("a refusal must not stream a file")
	}
}

// TestDiagnosticsCarriesNoSecretInAnyMember is the test the whole feature hangs
// on. Every seeded credential is checked against every unpacked member, so a
// new member added later cannot quietly become a leak.
func TestDiagnosticsCarriesNoSecretInAnyMember(t *testing.T) {
	h, _, svc := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	seedExportSecrets(t, svc)
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("content type = %q, want application/zip", ct)
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("the bundle must download as a file, got %s", w.Header().Get("Content-Disposition"))
	}

	members := zipMembers(t, w.Body.Bytes())
	for name, body := range members {
		for _, secret := range allSeededSecrets() {
			if strings.Contains(body, secret) {
				t.Fatalf("%s carries a stored secret. A diagnostics bundle is made to be "+
					"attached to a bug report and posted in public.", name)
			}
		}
	}
}

// TestDiagnosticsCarriesTheMembersSupportNeeds: the gate and the redaction are
// worth nothing if the file is empty. These are the four questions a support
// thread opens with: what does the host look like, how is it configured, what
// ran recently, and what is scheduled next.
func TestDiagnosticsCarriesTheMembersSupportNeeds(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	members := zipMembers(t, w.Body.Bytes())
	for _, want := range []string{"manifest.json", "spike.json", "settings.json", "runs.json", "scheduler.json", "log.txt"} {
		if _, ok := members[want]; !ok {
			t.Fatalf("the bundle is missing %s. Members present: %v", want, memberNames(members))
		}
	}

	// The manifest has to say the file is redacted, or a recipient cannot tell
	// it apart from a configuration backup and may treat it as one.
	if !strings.Contains(members["manifest.json"], "redacted") {
		t.Fatalf("the manifest must state that the bundle is redacted: %s", members["manifest.json"])
	}
}

func memberNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestDiagnosticsScrubsTheLog closes the loop the log member opens. Run errors
// are scrubbed because restic and rclone print their repository URL, credential
// and all, on failure, and the same output goes to the standard logger, which
// is what the ring records. A bundle that redacted the runs table and then
// shipped the identical string inside log.txt would be redacted in name only.
func TestDiagnosticsScrubsTheLog(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	// main() is what tees the standard logger into the ring, and main does not
	// run here. Without this the ring stays empty, log.txt comes out blank, and
	// the assertion below would pass while proving nothing at all.
	prev := log.Writer()
	log.SetOutput(logring.Default.Tee(io.Discard))
	t.Cleanup(func() { log.SetOutput(prev) })

	// Exactly the shape restic fails with, written through the standard logger
	// so it travels the real path into the ring.
	const logged = "restic: Fatal: unable to open repository at rest:https://admin:LOGGED-PASSWORD@backup.example:8000/repo"
	log.Print(logged)

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	members := zipMembers(t, w.Body.Bytes())
	// The line has to be IN there, or this test is green because the bundle
	// carries no log rather than because it carries a redacted one.
	if !strings.Contains(members["log.txt"], "unable to open repository") {
		t.Fatalf("log.txt did not capture the logged line at all, so this test proves nothing: %q", members["log.txt"])
	}
	if strings.Contains(members["log.txt"], "LOGGED-PASSWORD") {
		t.Fatalf("log.txt carries a password that was logged during a failure.\n" +
			"The runs table is scrubbed for exactly this reason; the log has to be too.")
	}
}

// Every line the standard logger writes starts with a date whose slashes look
// like a path. The scrubber must leave it alone, or lines of different days can
// no longer be told apart, while a path later in the line is still hidden.
func TestDiagnosticsLogKeepsTheDateOfEachLine(t *testing.T) {
	h, _, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	prev, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(logring.Default.Tee(io.Discard))
	log.SetFlags(log.LstdFlags)
	t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(prevFlags) })

	log.Print("api: dated line reading /mnt/user/secret/share")
	today := time.Now().Format("2006/01/02")

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var line string
	for _, l := range strings.Split(zipMembers(t, w.Body.Bytes())["log.txt"], "\n") {
		if strings.Contains(l, "dated line reading") {
			line = l
		}
	}
	if !strings.HasPrefix(line, today+" ") {
		t.Fatalf("the line lost its date: %q", line)
	}
	if strings.Contains(line, "/mnt/user/secret") {
		t.Fatalf("the path after the date was not scrubbed: %q", line)
	}
}

// TestDiagnosticsCarriesDBDumpState: a dump that keeps failing is what a
// support thread opens with, so the bundle names every recognised database and
// how its last dump went. The reason detail stays out: it is the dump tool's
// own message and can quote a row.
func TestDiagnosticsCarriesDBDumpState(t *testing.T) {
	d := &fakeServiceDocker{
		listOut: []dockercli.ContainerInfo{
			{Name: "immich_postgres", Image: "postgres:16"},
			{Name: "plex", Image: "plexinc/pms-docker:latest"},
		},
		inspects: map[string]model.Inspect{
			"immich_postgres": {Running: true, Config: model.Config{
				Image: "postgres:16", Env: []string{"POSTGRES_PASSWORD=x"},
			}, Mounts: []model.Mount{
				{Type: "bind", Source: "/mnt/user/appdata/immich_postgres", Destination: "/var/lib/postgresql/data"},
			}},
		},
	}
	h, st := dbFieldsRouterHarness(t, d)

	if _, err := st.UpsertTarget(store.Target{ContainerName: "immich_postgres"}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetTargetByContainer("immich_postgres")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRun(tg.ID, "dbdump")
	if err != nil {
		t.Fatal(err)
	}
	const detail = "role \"postgres\" does not exist"
	if err := st.FinishRun(runID, "failed", "", 0, "database dump failed: the database refused the login: "+detail); err != nil {
		t.Fatal(err)
	}
	importID, err := st.StartRun(tg.ID, "dbimport")
	if err != nil {
		t.Fatal(err)
	}
	const row = "Duplicate entry 'alice@example.com' for key 'email'"
	if err := st.FinishRun(importID, "failed", "", 0, store.ReasonDBImportFailed+
		": the previous data folder is kept at /host/mnt/user/appdata/immich_postgres.bombvault-before-import-20260917-021403; exit 1: ERROR 1062 (23000) at line 812: "+row); err != nil {
		t.Fatal(err)
	}

	cookie := loginCookie(t, h, "correct horse battery staple")
	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	members := zipMembers(t, w.Body.Bytes())
	dump, ok := members["dbdump.json"]
	if !ok {
		t.Fatalf("the bundle is missing dbdump.json. Members present: %v", memberNames(members))
	}
	for _, want := range []string{"immich_postgres", "curated", "postgres", "stopped", "failed", "the database refused the login"} {
		if !strings.Contains(dump, want) {
			t.Errorf("dbdump.json does not name %q: %s", want, dump)
		}
	}
	if strings.Contains(dump, detail) {
		t.Errorf("dbdump.json carries the dump tool's own message, which can quote a row: %s", dump)
	}
	if strings.Contains(dump, "plex") {
		t.Errorf("dbdump.json lists a container that is not a database: %s", dump)
	}

	runs := members["runs.json"]
	for _, want := range []string{"the database refused the login", store.ReasonDBImportFailed} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs.json does not name %q: %s", want, runs)
		}
	}
	for _, quoted := range []string{detail, "alice@example.com"} {
		if strings.Contains(runs, quoted) {
			t.Errorf("runs.json carries %q, which a database tool said and which can quote a row: %s", quoted, runs)
		}
	}
}

func TestDiagnosticsHasZFSFile(t *testing.T) {
	d := &fakeServiceDocker{}
	h, st := dbFieldsRouterHarness(t, d)

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ZFSEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	item, err := st.CreateZFSDataset(store.ZFSDataset{
		Dataset: "cache/appdata", Enabled: true, ExcludedChildren: []string{"cache/appdata/cachey"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetZFSCheck(item.ID, "ok", "", "/mnt/cache/appdata", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceZFSMembers(item.ID, []store.ZFSMember{
		{ItemID: item.ID, Dataset: "cache/appdata/secret", Outcome: "key-not-loaded"},
	}); err != nil {
		t.Fatal(err)
	}

	cookie := loginCookie(t, h, "correct horse battery staple")
	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	members := zipMembers(t, w.Body.Bytes())
	zfsJSON, ok := members["zfs.json"]
	if !ok {
		t.Fatalf("the bundle is missing zfs.json. Members present: %v", memberNames(members))
	}
	for _, want := range []string{"cache/appdata", "cache/appdata/cachey", "key-not-loaded", "propagation", "mounts"} {
		if !strings.Contains(zfsJSON, want) {
			t.Errorf("zfs.json does not carry %q: %s", want, zfsJSON)
		}
	}
}

// The bundle has to say what detection currently holds against the history, so
// a report about a backup that looks wrong carries the finding that says so.
func TestDiagnosticsBundleHasAnomalies(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})
	cookie := loginCookie(t, h, "correct horse battery staple")

	tg, err := st.UpsertTarget(store.Target{ContainerName: "plex"})
	if err != nil {
		t.Fatal(err)
	}
	seedAnomaly(t, st, store.Anomaly{
		ID: "shrink", Detector: "source", Metric: "source_bytes_shrink", Severity: "critical",
		ScopeKind: "item", ScopeID: tg.ID, TargetID: tg.ID, Domain: "container",
		LastSeenAt: time.Now().Unix(),
	})

	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	members := zipMembers(t, w.Body.Bytes())
	file, ok := members["anomalies.json"]
	if !ok {
		t.Fatalf("the bundle is missing anomalies.json. Members present: %v", memberNames(members))
	}
	for _, want := range []string{`"summary"`, `"open"`, `"source_bytes_shrink"`} {
		if !strings.Contains(file, want) {
			t.Fatalf("anomalies.json must carry %s: %s", want, file)
		}
	}
}

// TestDiagnosticsCarriesOnlyMCPCounts: a key's name is the operator's own word
// for one of their machines, so the bundle reports how many keys there are and
// nothing else about them. The four-character hint is allowed and is what the
// MCP log lines carry instead of the name, so the assertion is on names and
// fingerprints.
func TestDiagnosticsCarriesOnlyMCPCounts(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	// Without the tee the ring stays empty and log.txt proves nothing.
	prev := log.Writer()
	log.SetOutput(logring.Default.Tee(io.Discard))
	t.Cleanup(func() { log.SetOutput(prev) })

	const label = "Laptop"
	key, id := createMCPKey(t, h, label, true)
	if res := mcpCallTool(t, h, key, "get_health", ""); res.IsError {
		t.Fatalf("get_health: %v", res.Structured)
	}
	row, err := st.GetMCPKey(id)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := st.UpsertTarget(store.Target{ContainerName: "plex"}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.GetTargetByContainer("plex")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := st.StartRunWith(tg.ID, "backup", store.RunMeta{StartedVia: "mcp", StartedViaKey: id})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.FinishRun(runID, "success", "abc123", 1, ""); err != nil {
		t.Fatal(err)
	}

	cookie := loginCookie(t, h, "correct horse battery staple")
	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	members := zipMembers(t, w.Body.Bytes())

	var manifest struct {
		RemovedOn []string `json:"removedOnPurpose"`
		MCP       struct {
			ActiveKeys         int   `json:"activeKeys"`
			KeysAllowedToStart int   `json:"keysAllowedToStart"`
			UnusableKeys       int   `json:"unusableKeys"`
			LastUsedAt         int64 `json:"lastUsedAt"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(members["manifest.json"]), &manifest); err != nil {
		t.Fatalf("decode manifest.json: %v", err)
	}
	if manifest.MCP.ActiveKeys != 1 || manifest.MCP.KeysAllowedToStart != 1 || manifest.MCP.UnusableKeys != 0 {
		t.Errorf("mcp counts = %+v, want one active key that may start backups", manifest.MCP)
	}
	if manifest.MCP.LastUsedAt == 0 {
		t.Errorf("mcp.lastUsedAt is zero although a tool was called: %+v", manifest.MCP)
	}
	if !slices.ContainsFunc(manifest.RemovedOn, func(s string) bool { return strings.Contains(s, "MCP keys") }) {
		t.Errorf("the manifest does not say that MCP keys were left out: %v", manifest.RemovedOn)
	}

	runs := members["runs.json"]
	for _, want := range []string{`"startedVia": "mcp"`, `"startedViaKey": "` + id + `"`, `"startedViaLabel": ""`} {
		if !strings.Contains(runs, want) {
			t.Errorf("runs.json does not carry %s: %s", want, runs)
		}
	}

	// The MCP lines have to be in the log, or the name assertion below passes
	// because the bundle carries no MCP output rather than no names.
	if !strings.Contains(members["log.txt"], "tool get_health") {
		t.Fatalf("log.txt did not capture the tool call: %q", members["log.txt"])
	}
	for name, body := range members {
		for _, secret := range []string{label, row.Digest, key} {
			if strings.Contains(body, secret) {
				t.Errorf("%s carries the key's name or fingerprint. The bundle is made to be attached to a bug report.", name)
			}
		}
	}
}

// One bundle answers for all four of the database dumps, the ZFS domain,
// anomaly detection and the MCP keys, so a report about any of them arrives
// with its state and settings attached.
func TestDiagnosticsNamesDumpsZFSAnomaliesAndMCPTogether(t *testing.T) {
	h, st, _ := newTestRouterSvc(t, &fakeServiceDocker{}, &fakeResticEngine{})

	s, err := st.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.ZFSEnabled = true
	if err := st.UpdateSettings(s); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateZFSDataset(store.ZFSDataset{Dataset: "cache/appdata", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	tg, err := st.UpsertTarget(store.Target{ContainerName: "postgres"})
	if err != nil {
		t.Fatal(err)
	}
	seedAnomaly(t, st, store.Anomaly{
		ID: "shrink", Detector: "source", Metric: "dump_bytes_shrink", Severity: "critical",
		ScopeKind: "dump", ScopeID: tg.ID, TargetID: tg.ID, Domain: "container",
		LastSeenAt: time.Now().Unix(),
	})
	createMCPKey(t, h, "Laptop", false)

	cookie := loginCookie(t, h, "correct horse battery staple")
	w := getRaw(t, h, "/api/diagnostics", cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	members := zipMembers(t, w.Body.Bytes())

	for member, wants := range map[string][]string{
		"dbdump.json":    {"["},
		"zfs.json":       {`"cache/appdata"`},
		"anomalies.json": {`"dump_bytes_shrink"`},
		"manifest.json":  {`"activeKeys": 1`},
		"settings.json":  {`"dbDumpsEnabled"`, `"zfsEnabled": true`, `"anomalyEnabled"`},
	} {
		body, ok := members[member]
		if !ok {
			t.Errorf("the bundle is missing %s. Members present: %v", member, memberNames(members))
			continue
		}
		if strings.Contains(body, `"error"`) {
			t.Errorf("%s holds an error instead of the state: %s", member, body)
		}
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not carry %s: %s", member, want, body)
			}
		}
	}
}
