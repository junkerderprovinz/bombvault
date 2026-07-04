# Self-config-backup (the `config` domain) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for
> tracking.

**Goal:** Add a fourth backup domain, `config`, so BombVault restic-backs-up its own `/config` folder (SQLite
settings DB + `rclone.conf` + `ssh/` keypair) with no container stop — scheduled + on-demand, off-site-
replicated, and restorable through the Recovery tab so a rebuilt Unraid box recovers BombVault itself.

**Architecture:** The `config` domain is a clone of the **Flash** domain (whole-folder restic backup, no
lifecycle) with three genuinely new pieces: (1) the backup source is a **VACUUM-INTO staging snapshot** of the
live WAL-mode DB, not the live folder; (2) restore is **staged + applied on restart** via a boot-time
staging→live swap that runs before the DB is opened; (3) the restart is triggered by BombVault restarting
**itself over the mounted Docker socket** (new `dockercli.Restart`), with a manual-restart fallback.

**Tech Stack:** Go (`modernc.org/sqlite`, restic CLI adapter, Docker SDK over the mounted socket), embedded
React/Vite/TS SPA. Spec: `docs/superpowers/specs/2026-07-04-self-config-backup-design.md`.

## Global Constraints

- **Branch:** `feat/config-backup` (already created off `main` == v4.1.1). Sequential implementers (single
  working tree; no parallel implementers).
- **Internal domain key:** `config` (lowercase). **Reserved run id:** `store.ConfigTargetID = "config"`.
  **User-facing label:** "App configuration" / "Settings backup" (never re-label the `/config` mount).
- **Migrations start at v44** (current max is 43). One `ALTER TABLE settings ADD COLUMN` per column (SQLite
  ADD COLUMN is single-column), forward-only, idempotent.
- **Go gates per task:** `go build ./... && go vet ./...`, `gofmt -l internal/ cmd/` empty,
  `go test ./... -count=1` green, `golangci-lint run ./internal/...` 0 issues.
- **Frontend gate per FE task:** `cd web && npx tsc --noEmit && npm run build`, then
  `git checkout -- web/dist/index.html` (do not commit the rebuilt bundle hash churn unless the task is the
  final bundle build).
- **Commits:** no AI attribution / no Claude trailer. Commit via PowerShell (agent git hooks block commits in
  don't-ask). Frequent, one per task.
- **Recovery-kit / creds:** any credential the kit prints stays ONLY in the downloaded kit, never in logs.
- **i18n:** new keys land in `i18n.ts` (en+de) AND all 24 `web/src/lib/locales/*.json` (release gate) — the
  final task, done in-session by one writer (no delegation).

## File map

- **Create:** `internal/backup/config_orchestrator.go`, `internal/selfrestore/selfrestore.go` (+ test),
  `web/src/pages/Config.tsx`.
- **Modify (backend):** `internal/store/settings.go`, `internal/store/migrate.go`, `internal/store/runs.go`,
  `internal/api/service.go`, `internal/api/handlers.go`, `internal/api/api.go`,
  `internal/dockercli/dockercli.go`, `internal/schedule/schedule.go`, `cmd/bombvault/main.go`,
  `internal/api/deploy.go`, `internal/api/export.go` (recovery kit).
- **Modify (frontend):** `web/src/app/router.tsx`, `web/src/components/Sidebar.tsx`,
  `web/src/pages/Recovery.tsx`, `web/src/lib/api.ts`, `web/src/lib/i18n.ts`, `web/src/lib/locales/*.json`.

---

### Task 1: Settings columns + migrations

**Files:**
- Modify: `internal/store/settings.go` (struct + `GetSettings` + `UpdateSettings`)
- Modify: `internal/store/migrate.go:400` (append migrations v44–v49)
- Test: `internal/store/settings_test.go` (or the existing store test file)

**Interfaces:**
- Produces: six new `store.Settings` fields — `ConfigEnabled bool`, `ConfigPath string`,
  `ConfigSchedule string`, `ConfigOffsite string`, `ConfigOffsiteSchedule string`,
  `ConfigOffsiteImmutable bool` — persisted in columns `config_enabled`, `config_path`, `config_schedule`,
  `config_offsite`, `config_offsite_schedule`, `config_offsite_immutable`.

- [ ] **Step 1: Write the failing test** — round-trip the new fields.

```go
func TestSettingsConfigFieldsRoundTrip(t *testing.T) {
	db := openTestDB(t) // existing helper that opens a temp DB + Migrate
	r := New(db)
	s, err := r.GetSettings()
	if err != nil { t.Fatal(err) }
	s.ConfigEnabled = true
	s.ConfigPath = "user/bombvault/config"
	s.ConfigSchedule = "daily 03:30"
	s.ConfigOffsite = "rclone:remote:bombvault-config"
	s.ConfigOffsiteSchedule = "weekly Sun 04:00"
	s.ConfigOffsiteImmutable = true
	if err := r.UpdateSettings(s); err != nil { t.Fatal(err) }
	got, err := r.GetSettings()
	if err != nil { t.Fatal(err) }
	if !got.ConfigEnabled || got.ConfigPath != "user/bombvault/config" ||
		got.ConfigSchedule != "daily 03:30" || got.ConfigOffsite != "rclone:remote:bombvault-config" ||
		got.ConfigOffsiteSchedule != "weekly Sun 04:00" || !got.ConfigOffsiteImmutable {
		t.Fatalf("config fields not round-tripped: %+v", got)
	}
}
```

If no `openTestDB` helper exists, mirror the setup used by the nearest existing settings/store test (open
`store.Open(filepath.Join(t.TempDir(), "t.sqlite"))`, `store.Migrate(db)`, `New(db)`).

- [ ] **Step 2: Run it — expect FAIL** (`undefined: got.ConfigEnabled`).
  `go test ./internal/store/ -run TestSettingsConfigFieldsRoundTrip -v`

- [ ] **Step 3: Add the six migrations** — append to the `migrations` slice in `migrate.go` after v43 (`:400`):

```go
{
	// The `config` self-backup domain: BombVault backs up its own /config folder
	// (settings DB + rclone.conf + ssh keypair). Mirrors the flash domain's
	// settings columns. One ALTER per column (SQLite ADD COLUMN is single-column).
	version: 44, name: "settings_config_enabled",
	sql:     "ALTER TABLE settings ADD COLUMN config_enabled INTEGER NOT NULL DEFAULT 0;",
},
{
	version: 45, name: "settings_config_path",
	sql:     "ALTER TABLE settings ADD COLUMN config_path TEXT NOT NULL DEFAULT 'user/bombvault/config';",
},
{
	version: 46, name: "settings_config_schedule",
	sql:     "ALTER TABLE settings ADD COLUMN config_schedule TEXT NOT NULL DEFAULT 'off';",
},
{
	version: 47, name: "settings_config_offsite",
	sql:     "ALTER TABLE settings ADD COLUMN config_offsite TEXT NOT NULL DEFAULT '';",
},
{
	version: 48, name: "settings_config_offsite_schedule",
	sql:     "ALTER TABLE settings ADD COLUMN config_offsite_schedule TEXT NOT NULL DEFAULT '';",
},
{
	version: 49, name: "settings_config_offsite_immutable",
	sql:     "ALTER TABLE settings ADD COLUMN config_offsite_immutable INTEGER NOT NULL DEFAULT 0;",
},
```

- [ ] **Step 4: Add the struct fields** — in `settings.go`, after `FlashOffsiteImmutable` (`:96`) add
  `ConfigOffsiteImmutable bool`; group the rest with their flash siblings for readability:
  after `FlashEnabled` add `ConfigEnabled bool`; after `FlashPath` add `ConfigPath string`;
  after `FlashOffsite` add `ConfigOffsite string`; after `FlashOffsiteSchedule` add
  `ConfigOffsiteSchedule string`; after `FlashSchedule` add `ConfigSchedule string`.

- [ ] **Step 5: Wire `GetSettings`** — add the six columns to the SELECT list, add int scan vars
  `configEnabled`, `configImmutable` (bools are stored as INTEGER), scan `config_path`, `config_schedule`,
  `config_offsite`, `config_offsite_schedule` into the string fields, and set
  `s.ConfigEnabled = configEnabled != 0` / `s.ConfigOffsiteImmutable = configImmutable != 0`. Keep column
  order identical between SELECT and Scan.

- [ ] **Step 6: Wire `UpdateSettings`** — add the six columns to the UPDATE SET list and the matching Exec
  args (`boolInt(s.ConfigEnabled)`, `s.ConfigPath`, `s.ConfigSchedule`, `s.ConfigOffsite`,
  `s.ConfigOffsiteSchedule`, `boolInt(s.ConfigOffsiteImmutable)`), in the same order.

- [ ] **Step 7: Run tests — expect PASS.** `go test ./internal/store/ -run TestSettingsConfig -v`

- [ ] **Step 8: Commit.**
```bash
git add internal/store/settings.go internal/store/migrate.go internal/store/settings_test.go
git commit -m "feat(config): settings columns + migrations for the config backup domain"
```

---

### Task 2: Store run helpers (`ConfigTargetID`, last-success, RunCounts)

**Files:**
- Modify: `internal/store/runs.go:131` (const), `:135` (last-success clone), `:217` (RunCounts)
- Test: `internal/store/runs_test.go`

**Interfaces:**
- Produces: `const store.ConfigTargetID = "config"`; `func (r *Repo) LastSuccessfulConfigBackup() (time.Time, error)`;
  `RunCounts` attributes `target_id = ConfigTargetID` runs to the `"config"` domain.

- [ ] **Step 1: Write the failing test.**
```go
func TestLastSuccessfulConfigBackupAndCounts(t *testing.T) {
	db := openTestDB(t)
	r := New(db)
	id, _ := r.StartRun(ConfigTargetID, "backup")
	if err := r.FinishRun(id, "success", "snap1", 100, ""); err != nil { t.Fatal(err) }
	ts, err := r.LastSuccessfulConfigBackup()
	if err != nil { t.Fatal(err) }
	if ts.IsZero() { t.Fatal("expected a last-success time for config") }
	counts, err := r.RunCounts()
	if err != nil { t.Fatal(err) }
	if counts["config"]["success"] != 1 {
		t.Fatalf("expected 1 config success, got %v", counts["config"])
	}
}
```
(Use the same StartRun/FinishRun API the flash/vm run tests use; match their exact method names.)

- [ ] **Step 2: Run it — expect FAIL** (`undefined: ConfigTargetID`).

- [ ] **Step 3: Add the const + last-success helper** after the flash ones (`:131`/`:135`):
```go
// ConfigTargetID is the reserved runs.target_id for the singleton config self-
// backup domain (BombVault's own /config). Like FlashTargetID it is a fixed
// literal, distinct from the hex/UUID ids of container and VM targets.
const ConfigTargetID = "config"

// LastSuccessfulConfigBackup drives the config domain everyN due-gate, scoped to
// the reserved config target id.
func (r *Repo) LastSuccessfulConfigBackup() (time.Time, error) {
	row := r.db.QueryRow(`
		SELECT finished_at
		FROM runs
		WHERE kind = 'backup' AND status = 'success' AND finished_at IS NOT NULL AND target_id = ?
		ORDER BY finished_at DESC
		LIMIT 1`, ConfigTargetID)
	return scanLastBackupTime(row, "LastSuccessfulConfigBackup")
}
```

- [ ] **Step 4: Extend `RunCounts`** — add a `config` arm to the CASE (before the flash arm) and pass a
  second bind param. In `runs.go:220-230`, change the CASE to:
```go
		SELECT
		  CASE
		    WHEN target_id = ?                              THEN 'config'
		    WHEN target_id = ?                              THEN 'flash'
		    WHEN target_id IN (SELECT id FROM vms)          THEN 'vms'
		    WHEN target_id IN (SELECT id FROM targets)      THEN 'containers'
		    ELSE ''
		  END AS domain,
```
  and change the query args from `FlashTargetID` to `ConfigTargetID, FlashTargetID` (config bind first, to
  match the new first `?`). Update the doc comment's domain list to include `"config"`.

- [ ] **Step 5: Run tests — expect PASS.**

- [ ] **Step 6: Commit.** `feat(config): reserved run id, last-success gate, RunCounts arm`

---

### Task 3: VACUUM-INTO staging snapshot

**Files:**
- Modify: `internal/store/runs.go` or a new `internal/store/vacuum.go` (add `VacuumInto`)
- Modify: `internal/api/service.go` (add `stageConfigSnapshot` + `configSnapshotDir`)
- Test: `internal/store/vacuum_test.go`, `internal/api/service_test.go`

**Interfaces:**
- Produces: `func (r *Repo) VacuumInto(dst string) error` — writes a fully-consistent single-file snapshot of
  the live DB to `dst` (which must NOT already exist). `func (s *Service) configSnapshotDir() string` (=
  `filepath.Join(s.cfg.DataDir, ".snapshot")`) and `func (s *Service) stageConfigSnapshot() (string, error)`
  returning the staging dir populated with `bombvault.sqlite` (+ `rclone.conf`, `ssh/` if present).

- [ ] **Step 1: Write the failing test for `VacuumInto`.**
```go
func TestVacuumIntoProducesConsistentSnapshot(t *testing.T) {
	db := openTestDB(t)
	r := New(db)
	s, _ := r.GetSettings(); s.ContainersPath = "marker/path"
	if err := r.UpdateSettings(s); err != nil { t.Fatal(err) }
	dst := filepath.Join(t.TempDir(), "snap.sqlite")
	if err := r.VacuumInto(dst); err != nil { t.Fatal(err) }
	// Open the snapshot as an independent DB and read the marker back.
	snapDB, err := store.Open(dst)   // if store.Open is in-package, call Open(dst)
	if err != nil { t.Fatal(err) }
	defer snapDB.Close()
	var path string
	if err := snapDB.QueryRow("SELECT containers_path FROM settings WHERE id = 1").Scan(&path); err != nil {
		t.Fatal(err)
	}
	if path != "marker/path" { t.Fatalf("snapshot inconsistent: got %q", path) }
}
```

- [ ] **Step 2: Run it — expect FAIL** (`undefined: r.VacuumInto`).

- [ ] **Step 3: Implement `VacuumInto`.**
```go
// VacuumInto writes a fully-consistent single-file snapshot of the live database
// to dst using SQLite's `VACUUM INTO`. Because the DB runs in WAL mode with a
// single pooled connection, this is the safe way to snapshot it: it folds the WAL
// in and cannot capture a torn/partial write. dst must not already exist (SQLite
// refuses to overwrite), so callers stage into a freshly-created directory.
func (r *Repo) VacuumInto(dst string) error {
	if _, err := r.db.Exec("VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("VacuumInto %q: %w", dst, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the VacuumInto test — expect PASS.**

- [ ] **Step 5: Write the failing test for `stageConfigSnapshot`** (service test). Build a Service with a
  temp `DataDir` containing a live DB + a `rclone.conf` + an `ssh/id_ed25519`, call `stageConfigSnapshot`,
  and assert the returned dir contains all three and a readable `bombvault.sqlite`. Reuse the existing
  service-test constructor (`newTestService`/`testService` — match the helper used by `TestBackupRefusesSelf`
  in `service_test.go:1128`); if the helper doesn't expose `DataDir`, set `cfg.DataDir = t.TempDir()` before
  constructing.

```go
func TestStageConfigSnapshot(t *testing.T) {
	svc, dataDir := newServiceWithTempConfig(t) // helper: cfg.DataDir=dataDir, DB migrated, store wired
	os.WriteFile(filepath.Join(dataDir, "rclone.conf"), []byte("[r]\ntype = local\n"), 0o600)
	os.MkdirAll(filepath.Join(dataDir, "ssh"), 0o700)
	os.WriteFile(filepath.Join(dataDir, "ssh", "id_ed25519"), []byte("key"), 0o600)
	dir, err := svc.stageConfigSnapshot()
	if err != nil { t.Fatal(err) }
	for _, p := range []string{"bombvault.sqlite", "rclone.conf", filepath.Join("ssh", "id_ed25519")} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil { t.Fatalf("missing %s: %v", p, err) }
	}
}
```

- [ ] **Step 6: Run it — expect FAIL.**

- [ ] **Step 7: Implement `configSnapshotDir` + `stageConfigSnapshot`** in `service.go` (near the flash
  helpers). Add a small `copyFile`/`copyTree` helper if none exists in the package (grep first; there may
  already be one used by restore-to-folder).
```go
func (s *Service) configSnapshotDir() string { return filepath.Join(s.cfg.DataDir, ".snapshot") }

// stageConfigSnapshot builds a consistent, restic-ready copy of BombVault's own
// /config state in a staging dir: a VACUUM-INTO snapshot of the live DB plus the
// rclone.conf and ssh/ keypair (copied as-is; they are static files). The live
// DB is never handed to restic directly (WAL). Returns the staging dir; the
// caller removes it after the backup.
func (s *Service) stageConfigSnapshot() (string, error) {
	dir := s.configSnapshotDir()
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("config snapshot: clear staging: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("config snapshot: mkdir staging: %w", err)
	}
	if err := s.store.VacuumInto(filepath.Join(dir, "bombvault.sqlite")); err != nil {
		return "", err
	}
	// rclone.conf + ssh/ are static on disk; copy if present (best-effort presence).
	if src := filepath.Join(s.cfg.DataDir, "rclone.conf"); fileExists(src) {
		if err := copyFile(src, filepath.Join(dir, "rclone.conf"), 0o600); err != nil {
			return "", fmt.Errorf("config snapshot: copy rclone.conf: %w", err)
		}
	}
	if src := filepath.Join(s.cfg.DataDir, "ssh"); dirExists(src) {
		if err := copyTree(src, filepath.Join(dir, "ssh")); err != nil {
			return "", fmt.Errorf("config snapshot: copy ssh: %w", err)
		}
	}
	return dir, nil
}
```
(If `fileExists`/`dirExists`/`copyFile`/`copyTree` don't exist, add minimal versions in the same file. Skip
the DB's `-wal`/`-shm` sidecars — VACUUM INTO already produced a consolidated file.)

- [ ] **Step 8: Run tests — expect PASS.** `go test ./internal/store/ ./internal/api/ -run 'Vacuum|StageConfig' -v`

- [ ] **Step 9: Commit.** `feat(config): VACUUM-INTO staging snapshot of /config`

---

### Task 4: Config backup orchestrator

**Files:**
- Create: `internal/backup/config_orchestrator.go`
- Test: `internal/backup/config_orchestrator_test.go`

**Interfaces:**
- Produces: `backup.ConfigRestic` (same `Backup` surface as `FlashRestic`), `backup.ConfigBackupDeps`
  (`SourceDir, Repo, TargetID string; Restic ConfigRestic; Runs Runs`), and
  `func BackupConfig(ctx, ConfigBackupDeps) (Summary, error)` — records a run, runs one `restic backup` of
  `SourceDir` tagged `"config"`.

- [ ] **Step 1: Write the failing test** — mirror the flash orchestrator behaviour with fakes.
```go
func TestBackupConfigRecordsRunAndTags(t *testing.T) {
	fr := &fakeRestic{summary: Summary{SnapshotID: "s1", Bytes: 42}}
	runs := &fakeRuns{}
	sum, err := BackupConfig(context.Background(), ConfigBackupDeps{
		SourceDir: "/config/.snapshot", Repo: "/repo", TargetID: "config", Restic: fr, Runs: runs,
	})
	if err != nil { t.Fatal(err) }
	if sum.SnapshotID != "s1" { t.Fatalf("got %+v", sum) }
	if len(fr.tags) != 1 || fr.tags[0] != "config" { t.Fatalf("tags=%v", fr.tags) }
	if runs.finishedStatus != "success" { t.Fatalf("status=%s", runs.finishedStatus) }
}
```
(Reuse the existing fake restic/runs from `internal/backup`'s tests — grep `orchestrator_test.go` /
`vm_orchestrator_test.go` for `fakeRestic`/`fakeRuns` and match their fields. If the flash path has no fake,
add a minimal `fakeRestic` capturing `paths`/`tags` and a `fakeRuns` capturing start/finish.)

- [ ] **Step 2: Run it — expect FAIL.**

- [ ] **Step 3: Implement `config_orchestrator.go`** (clone of `flash_orchestrator.go`, `config` tag):
```go
package backup

import (
	"context"
	"fmt"
)

// ConfigRestic is the restic surface the config self-backup domain needs. Like
// flash it is a plain directory backup (of a staged snapshot of /config), so
// there is no lifecycle to manage.
type ConfigRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string) (Summary, error)
}

// ConfigBackupDeps bundles everything BackupConfig needs. SourceDir is the staged
// snapshot directory (VACUUM-INTO DB + rclone.conf + ssh/), NOT the live /config.
type ConfigBackupDeps struct {
	SourceDir string
	Repo      string
	TargetID  string // store.ConfigTargetID
	Restic    ConfigRestic
	Runs      Runs
}

// BackupConfig backs up BombVault's own staged /config snapshot via restic. A
// thin record-around-restic (no stop/start): the DB was made consistent upstream
// by VACUUM INTO, so this just snapshots the staging directory.
func BackupConfig(ctx context.Context, d ConfigBackupDeps) (Summary, error) {
	runID, err := d.Runs.Start(d.TargetID, kindBackup)
	if err != nil {
		return Summary{}, fmt.Errorf("config backup: start run: %w", err)
	}
	summary, err := d.Restic.Backup(ctx, d.Repo, []string{d.SourceDir}, []string{"config"})
	if err != nil {
		_ = d.Runs.Finish(runID, statusFailed, "", 0, truncateErr(err))
		return Summary{}, err
	}
	if err := d.Runs.Finish(runID, statusSuccess, summary.SnapshotID, summary.Bytes, ""); err != nil {
		return summary, fmt.Errorf("config backup: record run: %w", err)
	}
	return summary, nil
}
```

- [ ] **Step 4: Run tests — expect PASS.** `go test ./internal/backup/ -run BackupConfig -v`

- [ ] **Step 5: Commit.** `feat(config): backup orchestrator (record-around-restic, config tag)`

---

### Task 5: Service backup — `BackupConfig`, `StartBackupConfig`, repo resolution, domain switches

**Files:**
- Modify: `internal/api/service.go` (`configRepoPath`; `case "config"` in `repoFor` `:4747`,
  `offsiteRepoFor` `:489`, `offsiteScheduleFor` `:504`, `offsiteImmutableFor` `:522`, domain-validation
  switch `:4306`; `repoMu` map init `:217`; `BackupConfig`; `StartBackupConfig`)
- Modify: `internal/api/deploy.go:54` (add `"config"` to the domain guard)
- Test: `internal/api/service_test.go`

**Interfaces:**
- Consumes: `backup.ConfigBackupDeps`/`BackupConfig` (Task 4); `s.stageConfigSnapshot`/`configSnapshotDir`
  (Task 3); `store.ConfigTargetID` (Task 2); `store.Settings.Config*` (Task 1).
- Produces: `func (s *Service) BackupConfig(ctx) (backup.Summary, error)`;
  `func (s *Service) StartBackupConfig(ctx) (bool, error)`; `func (s *Service) configRepoPath(store.Settings) (string, error)`.

- [ ] **Step 1: Add `configRepoPath` + all `case "config"` switch arms.** Read `flashRepoPath` (`service.go`
  ~`:403`) and each switch listed above; add the exact `config` analog:
  - `configRepoPath` = `flashRepoPath` with `settings.ConfigPath`.
  - `repoFor`: `case "config": return s.configRepoPath(settings)` (plus its source/offsite handling mirroring
    the flash arm — copy the flash arm's structure exactly).
  - `offsiteRepoFor`/`offsiteScheduleFor`/`offsiteImmutableFor`: `case "config"` returning
    `settings.ConfigOffsite` / `settings.ConfigOffsiteSchedule` / `settings.ConfigOffsiteImmutable`.
  - domain-validation switch (`:4306`): add `"config"` to the accepted set (`case "containers","vms","flash","config"`).
  - `repoMu` map init (`:217`): add `"config": {}`. Also update the test map init at `service_test.go:19`.
  - `deploy.go:54`: add `"config"` to `case "containers","vms","flash"`.

- [ ] **Step 2: Write the failing test — end-to-end config backup to a temp local repo.**
```go
func TestBackupConfigEndToEnd(t *testing.T) {
	svc, dataDir := newServiceWithTempConfig(t) // real restic engine OR the repo's existing test engine
	s, _ := svc.store.GetSettings()
	s.ConfigEnabled = true
	s.ConfigPath = filepath.Join(t.TempDir(), "configrepo") // resolved local repo
	svc.store.UpdateSettings(s)
	sum, err := svc.BackupConfig(context.Background())
	if err != nil { t.Fatalf("BackupConfig: %v", err) }
	if sum.SnapshotID == "" { t.Fatal("no snapshot id") }
	// staging dir cleaned up
	if _, err := os.Stat(svc.configSnapshotDir()); !os.IsNotExist(err) {
		t.Fatalf("staging dir not cleaned: %v", err)
	}
	// a successful config run recorded
	if ts, _ := svc.store.LastSuccessfulConfigBackup(); ts.IsZero() {
		t.Fatal("no successful config run recorded")
	}
}
```
(Match how `TestSnapshotsFlashRemoteOffsiteLists`/flash backup tests construct the service + restic engine.
If those tests use a fake engine, use the same fake and assert the run + staging cleanup rather than a real
snapshot.)

- [ ] **Step 3: Run it — expect FAIL** (`svc.BackupConfig` undefined).

- [ ] **Step 4: Implement `BackupConfig` (clone of `BackupFlash` `:4028`, with staging + cleanup):**
```go
func (s *Service) BackupConfig(ctx context.Context) (backup.Summary, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 12*time.Hour)
	defer cancel()
	defer s.lockDomain("config")()
	settings, err := s.store.GetSettings()
	if err != nil {
		return backup.Summary{}, fmt.Errorf("read settings: %w", err)
	}
	stagingDir, err := s.stageConfigSnapshot()
	if err != nil {
		return backup.Summary{}, err
	}
	defer func() { _ = os.RemoveAll(stagingDir) }() // never leave the snapshot on disk
	repo, err := s.configRepoPath(settings)
	if err != nil {
		return backup.Summary{}, err
	}
	mode := s.ModeFor(settings)
	if err := s.EnsureRepo(ctx, repo, mode); err != nil {
		return backup.Summary{}, err
	}
	s.unlockStale(ctx, repo, mode)
	fctx := s.progBegin(ctx, "config", "backup")
	sum, err := backup.BackupConfig(fctx, backup.ConfigBackupDeps{
		SourceDir: stagingDir,
		Repo:      repo,
		TargetID:  store.ConfigTargetID,
		Restic:    &resticAdapter{engine: s.engine, mode: mode},
		Runs:      runsAdapter{s.store},
	})
	s.progEnd("config", "backup", err == nil)
	s.notifyBackup(ctx, "config", "", err == nil, sum, err)
	if err != nil {
		return backup.Summary{}, err
	}
	s.applyRetention(ctx, repo, settings, mode)
	s.replicateOffsite(ctx, "config", settings, mode, repo)
	s.maybeCollectStats(ctx, "config")
	return sum, nil
}
```

- [ ] **Step 5: Implement `StartBackupConfig` (clone of `StartBackupFlash` `:1949`):** identical body,
  `"config"` in `domainBusy`, log line, and calling `s.BackupConfig(bctx)`.

- [ ] **Step 6: Run tests — expect PASS.** `go test ./internal/api/ -run 'BackupConfig' -v`

- [ ] **Step 7: Full gates + commit.** `go build ./... && go vet ./... && go test ./... -count=1`
  `feat(config): service BackupConfig/StartBackupConfig + repo resolution`

---

### Task 6: DomainStatus + protection scorecard (metrics follow automatically)

**Files:**
- Modify: `internal/api/service.go:802-804` (domain-status list), `:1157` (scorecard enabled list)
- Test: `internal/api/service_test.go`

**Interfaces:**
- Consumes: `settings.ConfigEnabled/ConfigSchedule`, `s.store.LastSuccessfulConfigBackup`.
- Produces: a `DomainStatus()` entry with `Domain == "config"`. NOTE: the `DomainStatusEntry.Domain` field
  doc/type union (`service.go:536`) says `"containers"|"vms"|"flash"` — widen the comment to include
  `"config"` (it's a string field, so no type change, just keep the doc honest).

- [ ] **Step 1: Write the failing test.**
```go
func TestDomainStatusIncludesConfig(t *testing.T) {
	svc, _ := newServiceWithTempConfig(t)
	s, _ := svc.store.GetSettings(); s.ConfigEnabled = true; s.ConfigSchedule = "daily 03:30"
	svc.store.UpdateSettings(s)
	found := false
	for _, d := range svc.DomainStatus() {
		if d.Domain == "config" { found = true }
	}
	if !found { t.Fatal("DomainStatus missing config entry") }
}
```
(If `DomainStatus` needs a ctx, pass `context.Background()`; match the real signature.)

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Add the `config` row** to the domain list at `:802-804`, mirroring the flash row:
  `{Domain: "config", Enabled: settings.ConfigEnabled, Schedule: settings.ConfigSchedule, LastSuccess: s.store.LastSuccessfulConfigBackup}`
  (match the exact field names/shape of the existing flash entry). Add the enabled entry to the scorecard
  list at `:1157` the same way.

- [ ] **Step 4: Run — expect PASS.** Confirm metrics emit `config` by eyeballing `metrics.go` iterates
  `DomainStatus()` generically (no code change needed).

- [ ] **Step 5: Commit.** `feat(config): domain status + protection scorecard entry`

---

### Task 7: RestoreConfig + boot-time staging→live swap

**Files:**
- Create: `internal/selfrestore/selfrestore.go` (+ `selfrestore_test.go`)
- Modify: `internal/api/service.go` (`RestoreConfig`, `SnapshotsConfig`, `resolveConfigSnapshot`)
- Modify: `cmd/bombvault/main.go` (call the swap before `store.Open`)
- Test: `internal/selfrestore/selfrestore_test.go`

**Interfaces:**
- Produces: package `selfrestore` with
  `StagingRoot(dataDir) string`, `MarkerPath(dataDir) string`, `WriteMarker(dataDir) error`,
  `RestoredSnapshotDir(dataDir) string`, and `ApplyPending(dataDir string) (applied bool, err error)`.
  Service: `func (s *Service) RestoreConfig(ctx, snapshotID, source string) error` (stages + writes marker),
  `func (s *Service) SnapshotsConfig(ctx, source string) ([]restic.Snapshot, error)`.

- [ ] **Step 1: Write the failing swap test.** Cover: no marker → no-op; valid staging → files swapped +
  wal/shm removed + marker cleared; invalid staged DB → live untouched + staging moved aside + marker cleared.
```go
func TestApplyPendingSwapsValidStaging(t *testing.T) {
	dataDir := t.TempDir()
	// live DB with a marker value
	live := filepath.Join(dataDir, "bombvault.sqlite")
	writeSQLiteWithMarker(t, live, "OLD")
	os.WriteFile(live+"-wal", []byte("stale"), 0o600)
	// staged restored DB at the deterministic restic path
	staged := selfrestore.RestoredSnapshotDir(dataDir)
	os.MkdirAll(staged, 0o700)
	writeSQLiteWithMarker(t, filepath.Join(staged, "bombvault.sqlite"), "NEW")
	if err := selfrestore.WriteMarker(dataDir); err != nil { t.Fatal(err) }

	applied, err := selfrestore.ApplyPending(dataDir)
	if err != nil || !applied { t.Fatalf("applied=%v err=%v", applied, err) }
	if readSQLiteMarker(t, live) != "NEW" { t.Fatal("live DB not replaced") }
	if _, err := os.Stat(live + "-wal"); !os.IsNotExist(err) { t.Fatal("stale -wal not removed") }
	if _, err := os.Stat(selfrestore.MarkerPath(dataDir)); !os.IsNotExist(err) { t.Fatal("marker not cleared") }
	if _, err := os.Stat(selfrestore.StagingRoot(dataDir)); !os.IsNotExist(err) { t.Fatal("staging not removed") }
}

func TestApplyPendingNoMarkerIsNoop(t *testing.T) {
	applied, err := selfrestore.ApplyPending(t.TempDir())
	if err != nil || applied { t.Fatalf("expected no-op, applied=%v err=%v", applied, err) }
}
```
(`writeSQLiteWithMarker` opens `store.Open`, migrates, sets `containers_path`; `readSQLiteMarker` reads it
back. Put these helpers in the selfrestore test using the `store` package, or assert on a plain file if you
prefer to keep selfrestore dependency-free — a plain-file variant is fine since the swap is byte-level.)

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Implement `selfrestore.go`.** Deterministic restic path: the backup source is
  `<dataDir>/.snapshot`, so restic (restore `--target <StagingRoot>`) recreates it at
  `filepath.Join(StagingRoot, dataDir, ".snapshot")`.
```go
// Package selfrestore applies a staged restore of BombVault's own /config on the
// next boot, BEFORE the DB is opened — the only safe moment to swap the settings
// database, since the running process otherwise holds it open (WAL). The API
// layer stages a restore + writes a marker; main.go calls ApplyPending at boot.
package selfrestore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const stagingDirName = ".restore-staging"
const markerName = ".restore-pending"

func StagingRoot(dataDir string) string { return filepath.Join(dataDir, stagingDirName) }
func MarkerPath(dataDir string) string  { return filepath.Join(dataDir, markerName) }

// RestoredSnapshotDir is where restic recreates the staged snapshot subtree under
// the staging root (restic restores absolute paths beneath --target).
func RestoredSnapshotDir(dataDir string) string {
	return filepath.Join(StagingRoot(dataDir), dataDir, ".snapshot")
}

func WriteMarker(dataDir string) error {
	return os.WriteFile(MarkerPath(dataDir), []byte("pending"), 0o600)
}

// ApplyPending swaps a staged config restore into place if the marker is present.
// It NEVER runs while the DB is open (call it from main before store.Open). It is
// fail-safe: an invalid/absent staged DB leaves the live DB untouched and clears
// the pending state (moving bad staging aside) so boot never loops.
func ApplyPending(dataDir string) (bool, error) {
	if _, err := os.Stat(MarkerPath(dataDir)); os.IsNotExist(err) {
		return false, nil
	}
	staged := RestoredSnapshotDir(dataDir)
	stagedDB := filepath.Join(staged, "bombvault.sqlite")
	if !validSQLite(stagedDB) {
		// Bad or missing staged DB: don't touch live; clear the pending state.
		_ = os.Rename(StagingRoot(dataDir), StagingRoot(dataDir)+".bad")
		_ = os.Remove(MarkerPath(dataDir))
		return false, fmt.Errorf("selfrestore: staged config DB missing/invalid at %q; kept live DB", stagedDB)
	}
	// Swap DB last; rclone.conf + ssh/ first (their staleness is harmless if we crash).
	if src := filepath.Join(staged, "rclone.conf"); fileExists(src) {
		if err := replace(src, filepath.Join(dataDir, "rclone.conf")); err != nil {
			return false, err
		}
	}
	if src := filepath.Join(staged, "ssh"); dirExists(src) {
		_ = os.RemoveAll(filepath.Join(dataDir, "ssh"))
		if err := os.Rename(src, filepath.Join(dataDir, "ssh")); err != nil {
			return false, err
		}
	}
	live := filepath.Join(dataDir, "bombvault.sqlite")
	_ = os.Remove(live + "-wal")
	_ = os.Remove(live + "-shm")
	if err := replace(stagedDB, live); err != nil {
		return false, err
	}
	_ = os.RemoveAll(StagingRoot(dataDir))
	_ = os.Remove(MarkerPath(dataDir))
	return true, nil
}

func validSQLite(path string) bool {
	if !fileExists(path) {
		return false
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false
	}
	defer db.Close() //nolint:errcheck
	var n int
	return db.QueryRow("PRAGMA schema_version").Scan(&n) == nil
}

func replace(src, dst string) error {
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("selfrestore: remove %q: %w", dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("selfrestore: move %q -> %q: %w", src, dst, err)
	}
	return nil
}

func fileExists(p string) bool { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
func dirExists(p string) bool  { fi, err := os.Stat(p); return err == nil && fi.IsDir() }
```

- [ ] **Step 4: Run swap tests — expect PASS.** `go test ./internal/selfrestore/ -v`

- [ ] **Step 5: Wire the swap into `main.go`** — between `ensureDataDirWritable` (`:66`) and `store.Open`
  (`:68`):
```go
	if applied, err := selfrestore.ApplyPending(cfg.DataDir); err != nil {
		log.Printf("selfrestore: %v", err) // fail-safe: boot continues on the live DB
	} else if applied {
		log.Printf("selfrestore: applied a staged config restore; booting on the restored settings")
	}
```
(add the import `"github.com/junkerderprovinz/bombvault/internal/selfrestore"`).

- [ ] **Step 6: Implement `SnapshotsConfig` + `resolveConfigSnapshot` + `RestoreConfig`** in `service.go`
  (clone `SnapshotsFlash` `:4155`; clone `resolveFlashSnapshot` `:4079` renamed with a config-worded empty
  message; `RestoreConfig` restores the whole snapshot into the staging root and writes the marker):
```go
func (s *Service) RestoreConfig(ctx context.Context, snapshotID, source string) error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	repo, err := s.repoFor(settings, "config", source)
	if err != nil {
		return err
	}
	mode := s.ModeFor(settings)
	snaps, err := s.engine.Snapshots(ctx, repo, mode)
	if err != nil {
		return err
	}
	id, err := resolveConfigSnapshot(snaps, snapshotID)
	if err != nil {
		return err
	}
	root := selfrestore.StagingRoot(s.cfg.DataDir)
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("config restore: clear staging: %w", err)
	}
	runID, err := s.store.StartRun(store.ConfigTargetID, "restore")
	if err != nil {
		return fmt.Errorf("config restore: start run: %w", err)
	}
	// Restore the whole config snapshot (its only subtree is <DataDir>/.snapshot)
	// into the staging root; the boot swap applies it on the next restart.
	if rerr := s.engine.RestoreInclude(ctx, repo, id, s.configSnapshotDir(), root, mode); rerr != nil {
		_ = s.store.FinishRun(runID, "failed", "", 0, rerr.Error())
		return rerr
	}
	if merr := selfrestore.WriteMarker(s.cfg.DataDir); merr != nil {
		_ = s.store.FinishRun(runID, "failed", "", 0, merr.Error())
		return merr
	}
	_ = s.store.FinishRun(runID, "success", id, 0, "")
	return nil
}
```
(Confirm `s.engine` exposes `RestoreInclude` — the `Engine` interface the service holds must include it; if
it's only on the concrete `restic.Restic`, add `RestoreInclude` to the service's engine interface, matching
how `DumpZip`/`Snapshots` are declared.)

- [ ] **Step 7: Run gates — expect PASS.** `go build ./... && go test ./internal/... -count=1`

- [ ] **Step 8: Commit.** `feat(config): staged restore + boot-time staging→live swap`

---

### Task 8: `dockercli.Restart` + service self-restart

**Files:**
- Modify: `internal/dockercli/dockercli.go:155` (add `Restart` after `Start`)
- Modify: `internal/api/service.go` (add `ScheduleSelfRestart`; the service's docker interface gains `Restart`)
- Test: `internal/dockercli` (if it has a fake API) + `internal/api/service_test.go`

**Interfaces:**
- Produces: `func (c *dockercli.Client) Restart(ctx, name string, timeout time.Duration) error` (wraps the
  SDK `ContainerRestart`); `func (s *Service) ScheduleSelfRestart() bool` — resolves the self container name,
  and if found spawns a short-delayed goroutine that restarts it over the socket; returns whether an auto-
  restart was scheduled (false ⇒ the caller shows the manual-restart instruction).

- [ ] **Step 1: Add `dockercli.Restart`.** Match the SDK's `ContainerRestart(ctx, id, container.StopOptions)`
  signature used elsewhere in the file (grep the existing `Stop` for the exact `container.StopOptions`/
  timeout-seconds pattern and mirror it):
```go
// Restart asks the daemon to restart the named container (stop then start). Used
// for BombVault's own restart-to-apply after a config restore: the daemon does
// both halves even though THIS process is killed during the stop, so it is the
// robust way to relaunch ourselves (no dependency on a --restart policy).
func (c *Client) Restart(ctx context.Context, name string, timeout time.Duration) error {
	secs := int(timeout.Seconds())
	if err := c.api.ContainerRestart(ctx, name, container.StopOptions{Timeout: &secs}); err != nil {
		return fmt.Errorf("dockercli: restart %q: %w", name, err)
	}
	return nil
}
```
(Add `Restart` to the `dockerAPI`/client interface that the service depends on — grep how `Stop`/`Start` are
declared on the service's docker interface (`ServiceDocker` or similar) and add `Restart` there + to any fake,
e.g. `fakeServiceDocker` in `testutil_test.go:120` which already fakes `Self`.)

- [ ] **Step 2: Write the failing test for `ScheduleSelfRestart`.**
```go
func TestScheduleSelfRestartReturnsFalseWithoutSelfName(t *testing.T) {
	svc := newServiceWithFakeDocker(t, "" /* Self returns "" */)
	if svc.ScheduleSelfRestart() { t.Fatal("expected false when self-name is unknown") }
}
func TestScheduleSelfRestartInvokesRestart(t *testing.T) {
	fake := &fakeServiceDocker{selfName: "BombVault", restarted: make(chan string, 1)}
	svc := newServiceWithDocker(t, fake)
	if !svc.ScheduleSelfRestart() { t.Fatal("expected true when self-name is known") }
	select {
	case name := <-fake.restarted:
		if name != "BombVault" { t.Fatalf("restarted %q", name) }
	case <-time.After(3 * time.Second):
		t.Fatal("Restart was not called")
	}
}
```
(Give `fakeServiceDocker` a `Restart` that pushes the name onto a channel. Keep the scheduled delay small and
overridable so the test doesn't wait long — e.g. a package var `selfRestartDelay = 1500 * time.Millisecond`
the test sets to `10*time.Millisecond`.)

- [ ] **Step 3: Run — expect FAIL.**

- [ ] **Step 4: Implement `ScheduleSelfRestart`.**
```go
var selfRestartDelay = 1500 * time.Millisecond // brief pause so the HTTP response flushes first

// ScheduleSelfRestart restarts BombVault's own container over the Docker socket
// shortly after returning, so a staged config restore is applied on the reboot.
// Returns false (no restart scheduled) when the self container can't be resolved
// — the caller then instructs the user to restart manually.
func (s *Service) ScheduleSelfRestart() bool {
	name := s.SelfContainerName(context.Background())
	if name == "" {
		return false
	}
	go func() {
		time.Sleep(selfRestartDelay)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.dc.Restart(ctx, name, 10*time.Second); err != nil {
			log.Printf("api: self-restart of %q failed: %v (restart the container manually to apply)", name, err)
		}
	}()
	return true
}
```
(`s.dc` = the service's docker field; match its real name.)

- [ ] **Step 5: Run — expect PASS.** `go test ./internal/api/ -run SelfRestart -v`

- [ ] **Step 6: Commit.** `feat(config): docker self-restart to apply a config restore`

---

### Task 9: API routes, handlers, settings DTO

**Files:**
- Modify: `internal/api/api.go:158` (three routes)
- Modify: `internal/api/handlers.go` (`handleBackupConfig`, `handleSnapshotsConfig`, `handleRestoreConfig`;
  settings DTO GET `:846-879` + PUT `:1017-1050`; off-site validation `:928`; cadence validation `:970`;
  run-name maps `:1539-1540`)
- Test: `internal/api/handlers_test.go`

**Interfaces:**
- Consumes: `s.StartBackupConfig`, `s.SnapshotsConfig`, `s.RestoreConfig`, `s.ScheduleSelfRestart`.
- Produces: `POST /api/config/backup`, `GET /api/config/snapshots`, `POST /api/config/restore`; the settings
  JSON now carries `configEnabled/configPath/configSchedule/configOffsite/configOffsiteSchedule/configOffsiteImmutable`.

- [ ] **Step 1: Add routes** at `api.go:158` (inside the authGate group, next to the flash routes):
```go
	mux.HandleFunc("POST /api/config/backup", h.handleBackupConfig)
	mux.HandleFunc("GET /api/config/snapshots", h.handleSnapshotsConfig)
	mux.HandleFunc("POST /api/config/restore", h.handleRestoreConfig)
```

- [ ] **Step 2: Add `handleBackupConfig` + `handleSnapshotsConfig`** — clones of `handleBackupFlash`
  (`:1968`) and `handleSnapshotsFlash` (`:1982`), calling `StartBackupConfig` / `SnapshotsConfig`.

- [ ] **Step 3: Add `handleRestoreConfig`** — decode `{source, snapshot}`, call `RestoreConfig`, then
  schedule the self-restart, and report whether it was auto-scheduled:
```go
func (h *Handler) handleRestoreConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source   string `json:"source"`
		Snapshot string `json:"snapshot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err) // match the package's error-writer
		return
	}
	if err := h.svc.RestoreConfig(r.Context(), body.Snapshot, body.Source); err != nil {
		writeMappedErr(w, err) // reuse the APP_KEY-mismatch mapping used by other restore handlers
		return
	}
	auto := h.svc.ScheduleSelfRestart()
	writeJSON(w, http.StatusOK, map[string]bool{"staged": true, "autoRestart": auto})
}
```
(Use the exact request-decode / error-writer / JSON-writer helpers the neighbouring handlers use.)

- [ ] **Step 4: Extend the settings DTO** — add the six `config*` JSON fields to the settings response struct
  and request struct, map them GET (`:846-879`) and PUT (`:1017-1050`) both directions, add `ConfigOffsite`
  to the off-site path-validation loop (`:928`) and `ConfigOffsiteSchedule` to the cadence-validation loop
  (`:970`). Add `store.ConfigTargetID: "config"` / display `"App configuration"` to the run-history name/
  domain maps (`handlers.go:1539-1540`, and `service.go:952`).

- [ ] **Step 5: Write/extend a handler test** — PUT settings with config fields then GET returns them; and
  `handleRestoreConfig` with a fake service returns `autoRestart` reflecting the fake's self-name. Mirror the
  existing `TestSnapshots`/settings handler tests.

- [ ] **Step 6: Run gates — expect PASS.** `go test ./internal/api/ -count=1`

- [ ] **Step 7: Commit.** `feat(config): API routes, handlers, settings DTO`

---

### Task 10: Scheduler — config job, due-gate, off-site, drill exclusion

**Files:**
- Modify: `internal/schedule/schedule.go` (`SetConfigJob`, config task in `ReloadWithDueChecks`,
  `configLastRun` param, off-site + enabled lists, exclude `config` from `drillDomains`)
- Modify: `cmd/bombvault/main.go` (`SetConfigJob` wiring, `configLastRun`, `ReloadWithDueChecks` call)
- Modify: `internal/api/handlers.go:1063` (the settings-save `ReloadWithDueChecks` call — new param)
- Test: `internal/schedule/schedule_test.go`

**Interfaces:**
- Produces: `func (s *Scheduler) SetConfigJob(fn func() error)`; `ReloadWithDueChecks` gains a trailing
  `configLastRun LastRun` parameter.

- [ ] **Step 1: Write the failing test** — a config job is registered when enabled + scheduled, and config
  is NOT in the drill domain list.
```go
func TestConfigJobScheduledAndExcludedFromDrills(t *testing.T) {
	sc := New(noopBackup, noTargets)
	sc.SetConfigJob(func() error { return nil })
	s := store.Settings{ConfigEnabled: true, ConfigSchedule: "daily 03:30"}
	if err := sc.ReloadWithDueChecks(s, zeroLR, zeroLR, zeroLR, zeroLR); err != nil { t.Fatal(err) }
	if !sc.hasJob("config") { t.Fatal("expected a config backup job") } // use the test hook the pkg exposes
	for _, d := range drillDomains { if d == "config" { t.Fatal("config must be excluded from drills") } }
}
```
(Match the existing scheduler test style — `schedule_test.go` already exercises flash/vm jobs; copy its job-
assertion helper and `LastRun` zero value.)

- [ ] **Step 2: Run — expect FAIL** (arity mismatch / no config job).

- [ ] **Step 3: Implement.** Add a `configJob func() error` field + `SetConfigJob`; add a `configLastRun`
  param to `ReloadWithDueChecks` (after `flashLastRun`); build the config task next to the flash task
  (`:389-400`) gated by `settings.ConfigSchedule` + `settings.ConfigEnabled`, using `configLastRun` in the
  everyN due-gate; add `config` to the off-site task list (`offsite("config", …)`) and the enabled-domain
  list; ensure `drillDomains` (`:412`-ish) does NOT include `config` (leave it out, like VMs).

- [ ] **Step 4: Update the two call sites.** `main.go:143-146` add
  `scheduler.SetConfigJob(func() error { _, e := svc.BackupConfig(context.Background()); return e })`;
  `main.go:162` add `configLastRun := schedule.LastRunFunc(st.LastSuccessfulConfigBackup)`; `main.go:165`
  pass it as the new trailing arg. `handlers.go:1063` pass the config last-run the same way (build it from
  `h.store.LastSuccessfulConfigBackup`).

- [ ] **Step 5: Run gates — expect PASS.** `go test ./... -count=1`

- [ ] **Step 6: Commit.** `feat(config): scheduler job, due-gate, off-site; excluded from drills`

---

### Task 11: Recovery kit — config repo location

**Files:**
- Modify: `internal/api/service.go:5217` (`RecoveryKit`) / `internal/api/export.go`
- Test: `internal/api/service_test.go` (extend the recovery-kit test)

**Interfaces:**
- Produces: the recovery-kit text includes the config repo location (local path + off-site URL when set), so
  a rebuilt box knows where to restore BombVault's own settings from — the one bootstrap seed the user needs.

- [ ] **Step 1: Write the failing test** — with `ConfigPath` set, the kit output contains that path (extend
  the existing recovery-kit test; grep for the current kit assertion and add a config check).

- [ ] **Step 2: Run — expect FAIL.**

- [ ] **Step 3: Add a "BombVault settings backup (config domain)" section** to the kit body listing the
  resolved `configRepoPath(settings)` and `settings.ConfigOffsite` (only when non-empty), alongside the
  existing repo-locations section. No credentials beyond what the kit already prints.

- [ ] **Step 4: Run — expect PASS. Commit.** `feat(config): recovery kit lists the config repo location`

---

### Task 12: Frontend — Config page, route, NavItem, API client

**Files:**
- Create: `web/src/pages/Config.tsx`
- Modify: `web/src/app/router.tsx` (+`/config`), `web/src/components/Sidebar.tsx` (+NavItem, `configEnabled`-
  gated), `web/src/lib/api.ts` (client fns + `Settings` DTO `config*` fields), `web/src/lib/i18n.ts`
  (minimal `config.*` + `nav.config` en/de so `tsc` passes — full ×24 in Task 14)

**Interfaces:**
- Produces: `api.ts` — `backupConfigNow()`, `listConfigSnapshots(source?)`, `restoreConfig(snapshot, source)`
  (POSTs `/api/config/restore`, returns `{staged, autoRestart}`); `Settings` type gains
  `configEnabled/configPath/configSchedule/configOffsite/configOffsiteSchedule/configOffsiteImmutable`.

- [ ] **Step 1: Add the API client fns + `Settings` fields** in `api.ts`, mirroring the flash trio
  (`backupFlashNow`/`listFlashSnapshots`/`flashDownloadURL` at `:1064-1083`). `restoreConfig` returns the
  `{staged, autoRestart}` shape.

- [ ] **Step 2: Build `Config.tsx`** — model on `web/src/pages/Flash.tsx`: an enable toggle + backup path +
  schedule + off-site + a "Back up now" button (`backupConfigNow`) + a snapshots list, with copy that frames
  it as "protect BombVault's own settings so a rebuilt server restores itself." (Restore lives in the
  Recovery tab — Task 13 — not here, to keep the self-referential restart flow in one place; this page is
  backup + status only.)

- [ ] **Step 3: Add the route + NavItem** — `/config` in `router.tsx`; a `configEnabled`-gated NavItem in
  `Sidebar.tsx` (mirror the flash NavItem gating at `:255`/`:300`-`:301`), with an inline SVG icon.

- [ ] **Step 4: Add minimal i18n keys** — `config.title/subtitle/backupNow/backingUp/schedule/…` and
  `nav.config` in `i18n.ts` (en+de only for now).

- [ ] **Step 5: Gate.** `cd web && npx tsc --noEmit && npm run build`, then `git checkout -- web/dist/index.html`.

- [ ] **Step 6: Commit.** `feat(config): frontend Config page + route + nav + API client`

---

### Task 13: Recovery tab — config-restore pre-discovery step

**Files:**
- Modify: `web/src/pages/Recovery.tsx` (new Step 2a before discovery), `web/src/lib/api.ts` (a
  `waitForAppBack()` health-poll helper if none exists)

**Interfaces:**
- Consumes: `api.restoreConfig` (Task 12), the existing settings/getStatus calls.
- Produces: a guided "Restore BombVault's own settings" step that stages the restore, triggers the restart,
  polls until the app returns, then lets the user continue to discovery with settings pre-filled.

- [ ] **Step 1: Add Step 2a** between the current connection check and discovery: a card asking only for the
  **config-repo location** (path or off-site URL) + a reminder that the `APP_KEY` must match (Step 1). A
  "Restore settings" button calls `restoreConfig`. On `{autoRestart:true}` show "BombVault is restarting to
  apply your settings…" and poll `/api/health` (or the lightest existing GET) until it responds, then reload
  the page/settings and advance. On `{autoRestart:false}` show the manual-restart instruction ("restart the
  BombVault container in Unraid, then continue"). The step is **skippable** (a user without a config backup
  proceeds to the manual Step 2 attach).

- [ ] **Step 2: Add `waitForAppBack()`** in `api.ts` if needed — poll a cheap endpoint with a timeout +
  backoff; resolve when it 200s. (Keep it a pure, testable function.)

- [ ] **Step 3: Gate.** `cd web && npx tsc --noEmit && npm run build`, then `git checkout -- web/dist/index.html`.

- [ ] **Step 4: Commit.** `feat(config): Recovery-tab step to restore BombVault's own settings first`

---

### Task 14: i18n ×24 + final bundle

**Files:**
- Modify: `web/src/lib/i18n.ts` (finalize en+de `config.*` + `nav.config`), all 24 `web/src/lib/locales/*.json`

**Constraint:** done in-session by ONE writer (no sub-agent delegation — per the i18n-translator lesson).
Every locale file gets the full `config.*` key block + `nav.config`, translated (not copied-through), each
key present exactly once.

- [ ] **Step 1: Finalize the en+de key set** in `i18n.ts` (source of `TranslationKey`).
- [ ] **Step 2: Add the same keys to all 24 locale files**, translated per locale. Match the existing key
  nesting/naming used for `flash.*` / `recovery.*`.
- [ ] **Step 3: Verify** — every locale has each new key exactly once; `tsc` reports no missing-key errors.
  `cd web && npx tsc --noEmit && npm run build` (commit the rebuilt `web/dist` this time — it's the release
  bundle).
- [ ] **Step 4: Commit.** `i18n(config): config domain keys in all 24 locale files + build`

---

## Self-review

- **Spec coverage:** §1 capture → T3; §2 backup path → T1/T2/T4/T5/T6/T9/T10; §3 VACUUM snapshot → T3; §4
  staged restore + boot swap + docker self-restart → T7/T8; §5 Recovery-tab step → T13; §6 naming/scope →
  keys throughout + T14; recovery kit → T11; frontend page → T12. All covered.
- **Type consistency:** `store.ConfigTargetID` (T2) used by T5/T7/T9/T10; `ConfigBackupDeps` (T4) consumed by
  T5; `selfrestore.{StagingRoot,MarkerPath,RestoredSnapshotDir,WriteMarker,ApplyPending}` (T7) used by T7
  service + main.go; `ScheduleSelfRestart` (T8) used by T9; `restoreConfig` `{staged,autoRestart}` (T12) used
  by T13. Names consistent across tasks.
- **Placeholder scan:** every novel function (VacuumInto, stageConfigSnapshot, BackupConfig, ApplyPending,
  RestoreConfig, dockercli.Restart, ScheduleSelfRestart, handleRestoreConfig) carries real code; clone tasks
  cite exact anchors + the substitution list. The one deliberate verify-at-implementation note is the exact
  SDK `ContainerRestart` signature and whether `RestoreInclude`/`Restart` sit on the service's engine/docker
  interfaces (grep-and-match instructions given, not left vague).

## Execution handoff

Subagent-driven, sequential implementers on `feat/config-backup`, two-stage review per task, tsc+build /
go-gates per task. Wave gate after T11 (backend complete): `/code-review` on the branch diff + a boot smoke
of the built image. After T14: `/security-review`, then release as the next Minor (v4.2.0) — the config
domain is additive.
