# BombVault 2.0 (Go) — Phase 1: Containers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a single static Go binary that backs up and restores Docker containers via restic, with an embedded React SPA, in-app settings (encryption toggle, paths, per-domain schedule), and a host-integration spike.

**Architecture:** Go HTTP server (`cmd/bombvault`) serves a JSON API and an embedded React/Vite/Tailwind SPA (`embed.FS`). State in SQLite (`modernc.org/sqlite`, pure-Go). Docker via the official SDK behind an interface; restic via CLI. Dependency-injected orchestrator for testable backup/restore. Per-domain in-process scheduler.

**Tech Stack:** Go 1.23, `modernc.org/sqlite`, `github.com/docker/docker/client`, `github.com/robfig/cron/v3`, restic ≥0.17 (CLI), React 18 + Vite + TypeScript + Tailwind, golangci-lint, GitHub Actions (multi-arch buildx).

**Branch:** `feat/go-rewrite`. The TS tree is still present; this plan ADDS the Go tree alongside it and removes the TS tree in the final packaging task (Task 21), so `main` history stays intact and `ts-final` tags the old code.

**Spec:** `docs/superpowers/specs/2026-06-08-bombvault-go-rewrite-design.md`

---

## File Structure

```
go.mod / go.sum
cmd/bombvault/main.go            # wiring: config, store, scheduler, server, banners
internal/config/config.go        # env config + validation (APP_KEY, dirs, mount root, ports)
internal/store/store.go          # sqlite open + connection
internal/store/migrate.go        # forward-only migrations
internal/store/settings.go       # settings repo (typed)
internal/store/targets.go        # targets repo
internal/store/runs.go           # runs repo
internal/restickey/restickey.go  # derive restic password from APP_KEY (HMAC)
internal/restic/restic.go        # argv builders + run; init/backup/restore/snapshots
internal/dockercli/dockercli.go  # Docker interface + real impl (official SDK)
internal/paths/paths.go          # in-app path containment under the host mount root
internal/template/template.go    # read/write Unraid container template XML
internal/backup/orchestrator.go  # BackupContainer + RestoreContainer (DI)
internal/schedule/schedule.go    # per-domain scheduler
internal/spike/spike.go          # host-integration probes
internal/api/api.go              # router + middleware
internal/api/handlers.go         # JSON handlers
internal/api/spa.go              # embed + serve the built SPA
internal/api/server.go           # HTTPS (self-signed) / HTTP_ONLY + banners
web/                             # React app (Vite); build output embedded by internal/api/spa.go
Dockerfile
.golangci.yml
.github/workflows/build.yml
.github/workflows/lint.yml
```

**Module path:** `github.com/junkerderprovinz/bombvault`.

**DI seam:** `internal/backup` and `internal/schedule` depend only on interfaces (`Docker`, `Restic`, `Templates`, `Runs`) — never import `dockercli`/`restic` directly — so they are unit-testable with fakes. Real adapters are wired only in `cmd/bombvault`.

---

## Wave A — Foundation

### Task 1: Go module + skeleton + CI

**Files:**
- Create: `go.mod`, `cmd/bombvault/main.go`, `.golangci.yml`, `.github/workflows/lint.yml`

- [ ] **Step 1: Init module**

Run: `go mod init github.com/junkerderprovinz/bombvault && go mod tidy`

- [ ] **Step 2: Minimal main**

```go
// cmd/bombvault/main.go
package main

import "fmt"

func main() { fmt.Println("bombvault") }
```

- [ ] **Step 3: golangci config**

```yaml
# .golangci.yml
run:
  timeout: 5m
linters:
  enable: [errcheck, govet, staticcheck, ineffassign, unused, gosec]
```

- [ ] **Step 4: lint.yml (go vet + build + test stub + golangci)**

```yaml
# .github/workflows/lint.yml
name: Lint
on:
  push: { branches: [main, feat/go-rewrite] }
  pull_request: { branches: [main] }
jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - run: go build ./...
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v6
        with: { version: latest }
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }
      - name: install restic
        run: sudo apt-get update && sudo apt-get install -y restic && restic version
      - run: go test ./...
```

- [ ] **Step 5: Verify** — `go build ./... && go vet ./...` succeed.
- [ ] **Step 6: Commit** — `git add go.mod cmd .golangci.yml .github/workflows/lint.yml && git commit -m "chore(go): module skeleton + CI"`

### Task 2: config

**Files:** Create `internal/config/config.go`, `internal/config/config_test.go`

`Config` fields: `AppKey string`, `DataDir string` (default `/config`), `HostMountRoot string` (default `/host/user`), `Port int` (3000), `HTTPSPort int` (3443), `HTTPOnly bool`, `FlashTemplatesDir string` (default `/host/boot/config/plugins/dockerMan/templates-user` — note: read from the broad mount), `DBPath string` (derived `DataDir/bombvault.sqlite`).

- [ ] **Step 1: Failing test** — `Load` rejects a bad APP_KEY and accepts a valid one.

```go
func TestLoadValidatesAppKey(t *testing.T) {
	_, err := config.Load(map[string]string{"APP_KEY": "short"})
	if err == nil { t.Fatal("expected error for short APP_KEY") }
	c, err := config.Load(map[string]string{"APP_KEY": strings.Repeat("a", 64)})
	if err != nil { t.Fatalf("unexpected: %v", err) }
	if c.HTTPSPort != 3443 { t.Fatalf("default HTTPSPort wrong: %d", c.HTTPSPort) }
}
```

- [ ] **Step 2: Run, see it fail** — `go test ./internal/config/` → FAIL (undefined).
- [ ] **Step 3: Implement** — `Load(env map[string]string) (Config, error)`: validate `APP_KEY` matches `^[0-9a-f]{64}$`; apply defaults; parse ints; `HTTPOnly` from `"true"`. `DBPath = filepath.Join(DataDir, "bombvault.sqlite")`. Provide `LoadFromEnv()` reading `os.Environ` for `main`.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(config): env config + APP_KEY validation"`

### Task 3: store open + migrations

**Files:** Create `internal/store/store.go`, `internal/store/migrate.go`, `internal/store/migrate_test.go`

- [ ] **Step 1: Failing test** — running migrations on a fresh `:memory:` DB twice is idempotent and creates `settings`, `targets`, `runs`, `schema_migrations`.

```go
func TestMigrateIdempotent(t *testing.T) {
	db := store.OpenMem(t)
	if err := store.Migrate(db); err != nil { t.Fatal(err) }
	if err := store.Migrate(db); err != nil { t.Fatalf("second migrate: %v", err) }
	for _, tbl := range []string{"settings", "targets", "runs", "schema_migrations"} {
		var n int
		row := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tbl)
		if err := row.Scan(&n); err != nil || n != 1 { t.Fatalf("table %s missing", tbl) }
	}
}
```

- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — `Open(path string) (*sql.DB, error)` using driver `"sqlite"` (`modernc.org/sqlite`), set `PRAGMA journal_mode=WAL`, `foreign_keys=ON`. `OpenMem(t)` test helper. `Migrate(db)`: a `[]migration{version, name, sql}` slice applied inside a tx, tracked in `schema_migrations(version PK, name, applied_at)`. Migration v1 SQL:

```sql
CREATE TABLE settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  encryption_enabled INTEGER NOT NULL DEFAULT 1,
  containers_enabled INTEGER NOT NULL DEFAULT 1,
  vms_enabled        INTEGER NOT NULL DEFAULT 0,
  flash_enabled      INTEGER NOT NULL DEFAULT 0,
  containers_path TEXT NOT NULL DEFAULT 'backups/bombvault/containers',
  vms_path        TEXT NOT NULL DEFAULT 'backups/bombvault/vms',
  flash_path      TEXT NOT NULL DEFAULT 'backups/bombvault/flash',
  containers_schedule TEXT NOT NULL DEFAULT 'off',
  vms_schedule        TEXT NOT NULL DEFAULT 'off',
  flash_schedule      TEXT NOT NULL DEFAULT 'off',
  default_language TEXT NOT NULL DEFAULT ''
);
INSERT INTO settings (id) VALUES (1);
CREATE TABLE targets (
  id TEXT PRIMARY KEY,
  container_name TEXT NOT NULL UNIQUE,
  appdata_paths TEXT NOT NULL,
  include_in_schedule INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE runs (
  id TEXT PRIMARY KEY,
  target_id TEXT NOT NULL REFERENCES targets(id),
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  finished_at INTEGER,
  snapshot_id TEXT,
  bytes INTEGER,
  error TEXT
);
CREATE INDEX idx_runs_target ON runs(target_id);
```

- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** — `git commit -am "feat(store): sqlite open + forward-only migrations"`

### Task 4: store repositories (settings, targets, runs)

**Files:** Create `internal/store/settings.go`, `targets.go`, `runs.go`, and `*_test.go`.

Types: `Settings struct{...}` mirroring the columns; `Target struct{ID, ContainerName string; AppdataPaths []string; IncludeInSchedule bool; CreatedAt int64}`; `Run struct{...}`.

- [ ] **Step 1: Failing tests** — `GetSettings` returns defaults; `UpdateSettings` round-trips; `UpsertTarget`/`GetTargetByContainer` round-trip with JSON `appdata_paths`; `StartRun`/`FinishRun`/`LastSuccessfulBackup` work; `FinishRun(failed)` records error.

```go
func TestTargetRoundtrip(t *testing.T) {
	db := store.OpenMem(t); store.Migrate(db)
	r := store.New(db)
	tg, _ := r.UpsertTarget(store.Target{ContainerName: "plex", AppdataPaths: []string{"/host/user/appdata/plex"}})
	got, _ := r.GetTargetByContainer("plex")
	if got.ID != tg.ID || got.AppdataPaths[0] != "/host/user/appdata/plex" { t.Fatal("roundtrip") }
}
```

- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — `New(db) *Repo` exposing `GetSettings()`, `UpdateSettings(Settings)`, `UpsertTarget(Target)`, `GetTargetByContainer(name)`, `ListTargets()`, `SetInclude(name, bool)`, `StartRun(targetID, kind)`, `FinishRun(id, status, snapshotID, bytes, errMsg)`, `LastSuccessfulBackup(targetID)`, `ListRuns(limit)`. IDs via `crypto/rand` hex (helper `newID()`); `appdata_paths` via `encoding/json`; `UpsertTarget` uses `INSERT ... ON CONFLICT(container_name) DO UPDATE`.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** — `git commit -am "feat(store): settings/targets/runs repositories"`

---

## Wave B — restic engine

### Task 5: restickey

**Files:** Create `internal/restickey/restickey.go`, `restickey_test.go`

- [ ] **Step 1: Failing test** — deterministic, 64-hex, differs by APP_KEY, ≠ APP_KEY.

```go
func TestDerive(t *testing.T) {
	a := restickey.Derive(strings.Repeat("a",64))
	if a != restickey.Derive(strings.Repeat("a",64)) { t.Fatal("not deterministic") }
	if a == restickey.Derive(strings.Repeat("b",64)) { t.Fatal("collision") }
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a) { t.Fatal("format") }
}
```

- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — `Derive(appKey string) string`: `hmac.New(sha256.New, hexDecode(appKey))`, write `"bombvault:restic-repo"`, return hex digest.
- [ ] **Step 4: Run → PASS.** **Step 5: Commit** — `git commit -am "feat(restickey): derive restic password from APP_KEY"`

### Task 6: restic adapter

**Files:** Create `internal/restic/restic.go`, `restic_args_test.go`, `restic_roundtrip_test.go`

Design: `type Restic struct{ Bin string }`; `Mode` = `{Encrypted bool; Password string}`. All commands set env: encrypted → `RESTIC_PASSWORD=<pw>`; unencrypted → `RESTIC_INSECURE_NO_PASSWORD=true` **and** pass `--insecure-no-password`.

- [ ] **Step 1: Failing arg tests** (pure builders, no I/O):

```go
func TestBackupArgs(t *testing.T) {
	got := restic.BackupArgs("/repo", []string{"-weird","/p"}, []string{"container:plex"}, restic.Mode{Encrypted:true})
	want := []string{"-r","/repo","backup","--json","--tag","container:plex","--","-weird","/p"}
	if !reflect.DeepEqual(got, want) { t.Fatalf("%v", got) }
}
func TestRestoreArgsNoPassword(t *testing.T) {
	got := restic.RestoreArgs("/repo","abc123","/", restic.Mode{Encrypted:false})
	want := []string{"-r","/repo","restore","--insecure-no-password","--target","/","--","abc123"}
	if !reflect.DeepEqual(got, want) { t.Fatalf("%v", got) }
}
```

- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement builders** — `InitArgs`, `BackupArgs`, `RestoreArgs`, `SnapshotsArgs`. Encrypted=false adds `--insecure-no-password` after the subcommand. Always `--` before positional paths/snapshot-id (arg-injection guard). `run(ctx, args, mode)` via `exec.CommandContext`, sets env (password NEVER in argv), `maxBuffer` not needed; on error log full stderr server-side but return `fmt.Errorf("restic %s failed", subcommand(args))` (scrub). `ParseBackupSummary([]byte) (Summary, error)` scans `--json` lines for `message_type=="summary"`. `Snapshots` parses `--json`.
- [ ] **Step 4: Run arg tests → PASS.**
- [ ] **Step 5: Roundtrip test (build-tagged or skipped if no restic):**

```go
func TestRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil { t.Skip("no restic") }
	dir := t.TempDir(); repo := filepath.Join(dir,"repo"); src := filepath.Join(dir,"src")
	os.MkdirAll(src,0o755); os.WriteFile(filepath.Join(src,"f.txt"), []byte("hi"), 0o644)
	r := restic.Restic{Bin:"restic"}; m := restic.Mode{Encrypted:false}
	if err := r.Init(ctx, repo, m); err != nil { t.Fatal(err) }
	sum, err := r.Backup(ctx, repo, []string{src}, []string{"t"}, m); if err != nil { t.Fatal(err) }
	out := filepath.Join(dir,"out")
	if err := r.Restore(ctx, repo, sum.SnapshotID, out, m); err != nil { t.Fatal(err) }
	// assert restored file exists under out
}
```

- [ ] **Step 6: Run** (locally skips, CI runs) → PASS. **Commit** — `git commit -am "feat(restic): argv builders + run + roundtrip (encrypted + passwordless)"`

---

## Wave C — docker, template, orchestrator

### Task 7: dockercli (interface + real impl)

**Files:** Create `internal/dockercli/dockercli.go`, `types.go`

- [ ] **Step 1:** Define the `Docker` interface (consumed by the orchestrator) + `ContainerInfo`/`ContainerInspect` structs (Id, Name, Image, Config{Image,Env,Cmd,User}, HostConfig{Binds,PortBindings,RestartPolicy,CapAdd,CapDrop,Privileged,SecurityOpt,ReadonlyRootfs,NetworkMode,Devices}, Mounts[]).

```go
type Docker interface {
	List(ctx context.Context) ([]ContainerInfo, error)
	Inspect(ctx context.Context, name string) (ContainerInspect, error)
	Stop(ctx context.Context, name string, timeout time.Duration) error
	Start(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	Pull(ctx context.Context, image string) error
	CreateAndStart(ctx context.Context, in ContainerInspect) error
	InspectName(ctx context.Context, name string) (string, error) // "" if absent
}
```

- [ ] **Step 2:** Implement `Client` over `github.com/docker/docker/client` (`client.NewClientWithOpts(client.FromEnv, client.WithHost("unix:///var/run/docker.sock"), client.WithAPIVersionNegotiation())`). Map dockerode-equivalent fields. `CreateAndStart` builds `container.Config`/`container.HostConfig` preserving the security fields (SEC parity). `InspectName` returns "" on "No such container".
- [ ] **Step 3:** No unit test for the real impl (integration-only; covered by the box-gate). `go build` must pass. **Commit** — `git commit -am "feat(dockercli): Docker interface + official-SDK impl"`

### Task 8: template read/write

**Files:** Create `internal/template/template.go`, `template_test.go`

- [ ] **Step 1: Failing test** — `Write` then `Read` round-trips to `<dir>/my-<Name>.xml`; `Read` returns `("", false)` when absent; `Name` casing preserved.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — `FileName(name) = "my-"+name+".xml"`; `Read(dir, name) (string, bool)`; `Write(dir, name, xml) error` (MkdirAll). 
- [ ] **Step 4: PASS. Commit** — `git commit -am "feat(template): read/write Unraid container template"`

### Task 9: backup orchestrator

**Files:** Create `internal/backup/orchestrator.go`, `orchestrator_test.go`

Interfaces (DI): `Docker` (subset of Task 7), `Restic{ Backup(...); Restore(...) }`, `Templates{ Read; Write }`, `Runs{ Start(targetID,kind) (runID); Finish(runID, status, snap, bytes, err) }`.

- [ ] **Step 1: Failing tests with fakes:**
  - `BackupContainer` calls Stop→Backup→Start and records success; **Start is called even when Backup returns an error**, and the error is re-thrown + recorded failed.
  - `RestoreContainer` aborts (no stop/remove) when `confirmed=false`.
  - `RestoreContainer` aborts when the live name mismatches the target (wrong-target guard).
  - `RestoreContainer` happy path: pull→stop→remove→Restore(target "/")→Write template→CreateAndStart→record success.

```go
func TestBackupAlwaysStarts(t *testing.T) {
	d := &fakeDocker{}; r := &fakeRestic{backupErr: errors.New("boom")}
	err := backup.BackupContainer(ctx, backup.BackupDeps{ContainerRef:"plex", Docker:d, Restic:r, /*...*/})
	if err == nil { t.Fatal("expected error") }
	if !d.started { t.Fatal("container must be restarted even on backup failure") }
}
```

- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — port the TS orchestrator: `BackupContainer` (defer/finally Start), `RestoreContainer` with the SEC fixes: validate `snapshotID` against `^[0-9a-f]{8,64}$`, restore target `"/"`, recreate preserves security fields (delegated to `Docker.CreateAndStart` which already maps them), `InspectName` re-check before stop/remove.
- [ ] **Step 4: PASS. Commit** — `git commit -am "feat(backup): container backup/restore orchestrator (ported SEC fixes)"`

---

## Wave D — paths, scheduler, spike

### Task 10: path containment

**Files:** Create `internal/paths/paths.go`, `paths_test.go`

- [ ] **Step 1: Failing test** — `Resolve(root="/host/user", sub="backups/x")` → `/host/user/backups/x`; rejects `..` traversal and absolute `sub`.

```go
func TestResolveRejectsTraversal(t *testing.T) {
	if _, err := paths.Resolve("/host/user", "../etc"); err == nil { t.Fatal("must reject ..") }
	got, err := paths.Resolve("/host/user", "backups/x"); if err != nil || got != "/host/user/backups/x" { t.Fatal(got) }
}
```

- [ ] **Step 2: FAIL. Step 3: Implement** — clean-join + verify `strings.HasPrefix(cleaned, root+sep)`; reject otherwise. `EnsureDir(path)` = `MkdirAll(path, 0o700)`.
- [ ] **Step 4: PASS. Commit** — `git commit -am "feat(paths): in-app path containment under host mount"`

### Task 11: per-domain scheduler

**Files:** Create `internal/schedule/schedule.go`, `schedule_test.go`

- [ ] **Step 1: Failing test** — `ParseCadence("daily 02:30")` → a cron spec `"30 2 * * *"`; `"off"` → disabled; `"weekly Mon 03:00"` → `"0 3 * * 1"`; raw cron passes through. A `Scheduler` with an injected clock + a fake "run job" fires the containers job at the due minute.
- [ ] **Step 2: FAIL. Step 3: Implement** — `ParseCadence(string) (spec string, enabled bool, err error)`; `Scheduler` wraps `robfig/cron/v3`, registers one entry per enabled domain reading `Settings.*_schedule`; `Reload(settings)` re-registers (called after settings update). The containers job: for each `include_in_schedule` target, call an injected `BackupFunc(containerName)`. Sequential.
- [ ] **Step 4: PASS. Commit** — `git commit -am "feat(schedule): per-domain in-process scheduler"`

### Task 12: spike

**Files:** Create `internal/spike/spike.go`, `spike_test.go`

- [ ] **Step 1: Failing test** — `Run(deps)` with injected probe funcs returns a `[]Check{Name,OK,Detail}` and an overall `AllOK`; a failing probe yields `OK=false` and never panics.
- [ ] **Step 2: FAIL. Step 3: Implement** — probes: docker (`Docker.List` ok), restic (`exec restic version` ≥0.17), qemu-img, rclone, path writable (write+remove a temp file under the chosen container path), libvirt (best-effort). Each wrapped in recover-free error handling. `Run` returns the slice + AllOK.
- [ ] **Step 4: PASS. Commit** — `git commit -am "feat(spike): host-integration probes"`

---

## Wave E — API + server

### Task 13: API handlers

**Files:** Create `internal/api/api.go`, `handlers.go`, `handlers_test.go`

Handlers depend on: `*store.Repo`, `dockercli.Docker`, a `backupService` (wires orchestrator + restic + paths + settings → resolves repo path, mode from `encryption_enabled`, password from restickey), `spike.Runner`, `schedule.Scheduler`.

- [ ] **Step 1: Failing handler tests** (httptest, fake Docker + in-mem store):
  - `GET /api/containers` → JSON list with `lastBackup`/`include`.
  - `POST /api/containers/plex/backup` (fake restic ok) → `{"ok":true}`; on failure → `{"ok":false,"error":...}` with HTTP 200 (graceful) and the error scrubbed.
  - `GET/PUT /api/settings` round-trips; PUT validates the path stays under the mount root and rejects bad cadence.
  - `POST /api/spike` returns checks.
- [ ] **Step 2: FAIL. Step 3: Implement** — `net/http` + `http.ServeMux` (Go 1.22 method+path patterns, e.g. `mux.HandleFunc("POST /api/containers/{name}/backup", ...)`). JSON encode/decode helpers; consistent `{ok,error}` envelope for mutations. The backup/restore handlers call the orchestrator via the `backupService`. Settings PUT calls `scheduler.Reload`. Implement **all** spec §5 endpoints: `GET /api/containers`, `POST /api/containers/{name}/backup`, `GET /api/containers/{name}/snapshots`, `POST /api/containers/{name}/restore` (body `{snapshotId, confirm}`), `PATCH /api/containers/{name}` (body `{includeInSchedule}`), `GET /api/settings`, `PUT /api/settings`, `POST /api/spike`, `GET /api/runs`, `GET /api/health`.
- [ ] **Step 4: PASS. Commit** — `git commit -am "feat(api): JSON handlers (containers, settings, spike, runs)"`

### Task 14: embed SPA + HTTPS server + banners

**Files:** Create `internal/api/spa.go`, `internal/api/server.go`, `web/dist/.gitkeep` (placeholder so embed compiles before the React build exists), update `cmd/bombvault/main.go`

- [ ] **Step 1:** `spa.go`: `//go:embed all:../../web/dist` (adjust path) → `embed.FS`; serve static files, fall back to `index.html` for client routes; serve API under `/api/`. NOTE: embed needs the dir to exist with at least one file — commit `web/dist/.gitkeep` until Task 19 produces a real build (CI builds React before `go build`).
- [ ] **Step 2:** `server.go`: `ListenAndServe` choosing HTTPS (self-signed cert generated at first boot via `openssl` into `DataDir/certs`, like the TS `ensureSelfSigned`) unless `HTTPOnly`. Bind `0.0.0.0` (NOT `$HOSTNAME` — the TS boot bug). Print the ASCII init banner + the `BOMBVAULT IS READY -> (HTTPS 3443)` box on listen.
- [ ] **Step 3:** `main.go`: `config.LoadFromEnv` → `store.Open`+`Migrate` → build adapters → `scheduler.Start` → `server.Run`. On fatal, log and exit 1.
- [ ] **Step 4:** `go build ./...` passes; manual `go run ./cmd/bombvault` with a test APP_KEY + `HTTP_ONLY=true` prints the READY banner and serves `/api/health`.
- [ ] **Step 5: Commit** — `git commit -am "feat(server): embed SPA + HTTPS/HTTP server + banners + main wiring"`

---

## Wave F — React UI (subagent can build in `web/` in parallel once the API shape from Task 13 is fixed)

### Task 15: React scaffold

**Files:** Create `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/tailwind.config.js`, `web/index.html`, `web/src/main.tsx`, `web/src/lib/api.ts`, `web/src/lib/i18n.ts`, `web/src/lib/theme.ts`

- [ ] **Step 1:** Scaffold Vite React-TS; add Tailwind; `vite.config.ts` sets `build.outDir = "dist"` and `server.proxy["/api"] = "https://localhost:3443"` (dev). `base: "/"`.
- [ ] **Step 2:** `api.ts`: typed `fetchJSON` wrapper returning `{ok,error}`-aware results; functions `listContainers`, `backupNow`, `listSnapshots`, `restore`, `getSettings`, `putSettings`, `runSpike`, `listRuns`.
- [ ] **Step 3:** `theme.ts` (dark/light via `data-theme` + localStorage, IBM-Carbon tokens in Tailwind config), `i18n.ts` (port the locale JSONs from the TS `lib/i18n/locales`; start with `en`+`de`, others copied).
- [ ] **Step 4:** `npm --prefix web install && npm --prefix web run build` produces `web/dist`. **Commit** — `git commit -am "feat(web): React+Vite+Tailwind scaffold + API client"`

### Task 16: Sidebar layout + Dashboard

**Files:** Create `web/src/app/Layout.tsx`, `web/src/app/router.tsx`, `web/src/pages/Dashboard.tsx`, `web/src/components/Sidebar.tsx`, `TopBar.tsx`

- [ ] **Step 1:** `Sidebar` (Dashboard, Containers, VMs*, Flash*, Settings — VMs/Flash shown only when their setting is enabled; `*` items disabled with a "later" tooltip). `TopBar` (theme toggle + language switcher). IBM-Carbon dark default. **Visual style: match the clean, card-based feel of VolumeVault's dashboard — the implementer should open the reference image** https://github.com/Darkdragon14/VolumeVault/blob/main/public/previews/dashboard.png (rounded cards, generous spacing, muted surfaces, a left sidebar, status chips).
- [ ] **Step 2:** `Dashboard` shows last backups + run history (`listRuns`) + spike status chip.
- [ ] **Step 3:** `npm run build` ok; eyeball via dev server. **Commit** — `git commit -am "feat(web): sidebar layout + dashboard"`

### Task 17: Containers page

**Files:** Create `web/src/pages/Containers.tsx`, `web/src/components/BackupButton.tsx`, `RestorePanel.tsx`, `IncludeToggle.tsx`

- [ ] **Step 1:** Table: name, image, status, last backup, **include-in-schedule** toggle, **Back up now** (inline pending + inline error/success — NOT a page crash), **Snapshots/Restore** disclosure.
- [ ] **Step 2:** `RestorePanel`: list snapshots, confirm-checkbox-gated Restore, inline result (restore errors render inline from the start — fixes the TS boundary issue).
- [ ] **Step 3:** A **Schedule** card at the top of the Containers tab editing `containers_schedule` (off / daily HH:MM / weekly DOW HH:MM / cron) via `putSettings`.
- [ ] **Step 4:** `npm run build` ok. **Commit** — `git commit -am "feat(web): containers page (backup/restore inline + per-domain schedule)"`

### Task 18: Settings page

**Files:** Create `web/src/pages/Settings.tsx`, `web/src/components/SpikePanel.tsx`

- [ ] **Step 1:** Sections: **Encryption** on/off (with the "fixed per repo at init" warning); **Backup paths** (subpath per domain under the mount root, with the resolved absolute preview); **Domains** on/off toggles (Container/VM/Flash; enabling reveals the tab); **Spike** panel with a **"Check now"** button + short explanation + results table.
- [ ] **Step 2:** All edits persist via `putSettings`; spike via `runSpike`.
- [ ] **Step 3:** `npm run build` ok. **Commit** — `git commit -am "feat(web): settings page (encryption, paths, domains, spike)"`

---

## Wave G — packaging, CI, docs, cleanup

### Task 19: Dockerfile (multi-stage, single binary)

**Files:** Create `Dockerfile` (replace the TS one)

- [ ] **Step 1:** Stage 1 `node:22-slim` → `npm --prefix web ci && npm --prefix web run build`. Stage 2 `golang:1.23` → copy go sources + the built `web/dist`, `CGO_ENABLED=0 go build -o /bombvault ./cmd/bombvault`. Stage 3 runtime `debian:stable-slim` (need restic/docker-cli/qemu-utils/rclone/openssl/libvirt-clients/ca-certificates) → copy `/bombvault`, set `ENV DATA_DIR=/config HOST_MOUNT_ROOT=/host/user PORT=3000 HTTPS_PORT=3443`, `EXPOSE 3000 3443`, `ENTRYPOINT ["/bombvault"]`. Print banner via the binary (no shell script needed). Pin restic ≥0.17 (use debian-backports or download the release binary if the distro version is older).
- [ ] **Step 2:** `docker build .` succeeds locally (or rely on CI). **Commit** — `git commit -am "build: multi-stage Dockerfile (React build + static Go binary)"`

### Task 20: build.yml (multi-arch + GHCR)

**Files:** Create `.github/workflows/build.yml`

- [ ] **Step 1:** buildx amd64+arm64; login GHCR; push `:latest` + `:sha-...` on `main` (build-only on PRs). `paths-ignore: ['**.md','LICENSE','docs/**']`.
- [ ] **Step 2:** Verify YAML. **Commit** — `git commit -am "ci: multi-arch image build + GHCR push"`

### Task 21: README + template + remove TS tree

**Files:** Modify `README.md`, `my-BombVault.xml` (at repo root in the parent dir is the delivery copy; also keep `templates/` if used); Delete the TS sources (`app/`, `components/`, `lib/`, `server/`, `test/`, `next.config.mjs`, `package.json` TS one, etc.).

- [ ] **Step 1:** Update README (Go stack, one-click backup, in-app settings, trust model). 
- [ ] **Step 2:** Simplify the Unraid template: `APP_KEY`, one broad mount `/mnt/user`→`/host/user` (rw), `docker.sock` (rw), libvirt-sock (optional), `/config` volume, ports; drop the per-domain dest mounts + path/encryption/schedule fields (now in-app). Deliver `my-BombVault.xml` to `d:\nextcloud\it\github\` per convention.
- [ ] **Step 3:** Remove the TS tree in the same commit (Go is now the app). `go build ./...` + `go test ./...` + `npm --prefix web run build` all green.
- [ ] **Step 4: Commit** — `git commit -am "chore: Go is the app — update README + template, remove TS tree"`

---

## Verification Gates (per the spec §13)

- **Every task:** `go test ./...` green, `golangci-lint run` clean, `go build ./...` ok; React tasks: `npm --prefix web run build` ok.
- **Wave B/C end:** `/code-review` on the diff; restic roundtrip green in CI.
- **Phase end:** `/security-review` (port the TS SEC checklist: arg-injection, path traversal, recreate privilege parity, error scrubbing, the broad-mount trust note), full `/code-review`, multi-arch image builds + pushes `:latest`.
- **Box-gate (user, real Unraid):** Force-Update `:latest`; run the spike; one-click backup of a small container; confirm-restore; verify it reappears in the Docker tab. CI cannot test Docker/KVM.

## Riskiest-first

Validate **Docker SDK over docker.sock + a restic backup→restore roundtrip from the Go binary inside the container on the real box** as early as possible (after Wave E gives a runnable server with the backup endpoint) — before investing in the full UI polish.
