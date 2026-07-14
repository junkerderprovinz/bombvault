# Implementation Plan — Files Domain (#62) + Foreign-Repo Restore (#61)

> **For agentic workers:** executed task-by-task by fresh subagents; each task is self-contained (files, interfaces, gates). Tasks run SEQUENTIALLY (each builds on the previous).

**Repo:** `d:/nextcloud/it/github/bombvault` (POSIX `/d/nextcloud/it/github/bombvault`)
**Specs:** `docs/superpowers/specs/2026-07-14-files-domain-design.md`, `docs/superpowers/specs/2026-07-14-foreign-repo-restore-design.md`
**Order:** Tasks 1–7 = files domain (backend → frontend). Tasks 8–11 = foreign-repo restore (backend → frontend; Task 10 reuses the file-set restore from Task 5). Task 12 = README + docs.

## Shared conventions (repeated inside each task so tasks stay self-contained)

- **Go gate:** `cd /d/nextcloud/it/github/bombvault && go build ./... && go vet ./... && go test ./...` (tests need `restic` >= 0.17 on PATH; restic-less environments skip the roundtrip tests but everything else must pass).
- **Frontend gate:** `npm --prefix /d/nextcloud/it/github/bombvault/web run build` (runs `tsc --noEmit` then vite build) **plus the locale key-parity check**:
  ```sh
  cd /d/nextcloud/it/github/bombvault/web
  for f in src/lib/locales/*.ts; do printf '%s %s\n' "$(grep -cE '^\s+"[a-zA-Z0-9.]+":' "$f")" "$f"; done | sort | uniq -c -w4
  grep -cE '^\s+"[a-zA-Z0-9.]+":' src/lib/i18n.ts
  ```
  All 24 locale files must print the SAME count, and `i18n.ts` must print exactly 2x that count (`en` + `de` live inline in `i18n.ts`). Baseline before this plan: 680 per locale, 1360 in i18n.ts. Every UI task adds its keys to `en` AND `de` in `web/src/lib/i18n.ts` and adds REAL translations (not English copies) to all 24 files in `web/src/lib/locales/*.ts` — i18n is part of each UI task, never a separate task.
- **Domain id:** the new domain is `files` (plural, matching `containers|vms|flash|config`). Snapshot tag: `fileset:<Name>` (mirrors `container:<name>` in `internal/backup/orchestrator.go:304` and `vm:<name>` in `internal/backup/vm_orchestrator.go:231`). Run attribution: `runs.target_id` = the file set's stable `file_sets.id`. Progress keys: `files:<name>` per set, `batch:files` for the whole-domain run.
- **New API endpoints** are registered in `Handler.Router()` in `internal/api/api.go` — that mux registration is the parity list of every frontend-called endpoint. They are automatically protected by `authGate` (`internal/api/handlers.go:1971`); do **NOT** add anything to the authGate public allowlist (`/api/auth`, `/api/login`, `/api/health`, `/metrics` — handlers.go:1981 and :2001).
- **File-set `Path` convention:** stored as a RELATIVE subpath under the host mount root (`cfg.HostMountRoot`, default `/host/user` = host `/mnt`), exactly like `settings.ContainersPath`; resolved with `paths.Resolve` (`internal/paths/paths.go:24`) and displayed as `/mnt/...` by prefixing `hostMountRoot` — the same convention `FolderBrowser` (`web/src/components/FolderBrowser.tsx`) already implements.

---

## Task 1 — Store layer: `files` settings columns + `file_sets` table + run helpers

**Goal:** Persist the files domain: six new settings columns, the `file_sets` item table with CRUD, and the last-successful-backup query the scheduler's everyN due-gate needs.

**Files:**
- `internal/store/migrate.go` — append migrations v58–v64 (current max is v57 `settings_prune_image_after_update`), one `ALTER` per column per house pattern (see v44–v49, the config domain's identical block):
  - v58 `settings_files_enabled` → `ALTER TABLE settings ADD COLUMN files_enabled INTEGER NOT NULL DEFAULT 0;`
  - v59 `settings_files_path` → `TEXT NOT NULL DEFAULT 'user/bombvault/files'`
  - v60 `settings_files_schedule` → `TEXT NOT NULL DEFAULT 'off'`
  - v61 `settings_files_offsite` → `TEXT NOT NULL DEFAULT ''`
  - v62 `settings_files_offsite_schedule` → `TEXT NOT NULL DEFAULT ''`
  - v63 `settings_files_offsite_immutable` → `INTEGER NOT NULL DEFAULT 0`
  - v64 `file_sets` → `CREATE TABLE file_sets (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, path TEXT NOT NULL, excludes TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 1, created_at INTEGER NOT NULL);`
- `internal/store/settings.go` — add `FilesEnabled bool`, `FilesPath string`, `FilesSchedule string`, `FilesOffsite string`, `FilesOffsiteSchedule string`, `FilesOffsiteImmutable bool` to `Settings`; extend the SELECT in `GetSettings` and the UPDATE in `UpdateSettings` (mirror how `Config*` fields are threaded — note the `boolInt` int-scan pattern for bools).
- `internal/store/filesets.go` (new) — mirror `internal/store/vms.go` (JSON-slice handling like `targets.go` `scanTarget`):
  ```go
  type FileSet struct { ID, Name, Path string; Excludes []string; Enabled bool; CreatedAt int64 }
  func (r *Repo) CreateFileSet(fs FileSet) (FileSet, error)      // newID() when ID==""
  func (r *Repo) UpdateFileSet(fs FileSet) error                  // by ID; name/path/excludes/enabled
  func (r *Repo) ListFileSets() ([]FileSet, error)                // ORDER BY name
  func (r *Repo) GetFileSet(id string) (FileSet, error)
  func (r *Repo) GetFileSetByName(name string) (FileSet, error)
  func (r *Repo) SetFileSetEnabled(id string, enabled bool) error
  func (r *Repo) DeleteFileSet(id string) error                   // tx: DELETE runs WHERE target_id=id, then the row (mirror DeleteVMTarget)
  ```
- `internal/store/runs.go` — add `LastSuccessfulFilesBackup() (time.Time, error)` scoped `target_id IN (SELECT id FROM file_sets)` (mirror `LastSuccessfulVMBackup` at line 137). Extend `RunCounts()` (line 256) CASE with `WHEN target_id IN (SELECT id FROM file_sets) THEN 'files'`.
- `internal/store/filesets_test.go` (new) — mirror `internal/store/vms_test.go`.

**Interfaces produced:** `store.FileSet`, the seven CRUD funcs, `Repo.LastSuccessfulFilesBackup`, `Settings.Files*` fields. Consumed by Tasks 2–5.

**Tests:** `filesets_test.go` pins CRUD round-trip, unique-name conflict, excludes JSON round-trip, `DeleteFileSet` removing runs; extend `internal/store/settings_test.go` round-trip with the six new fields; extend `internal/store/runs_test.go` for `LastSuccessfulFilesBackup` and the `files` bucket in `RunCounts`.

**Gate:** `go build ./... && go vet ./... && go test ./internal/store/...` then `go test ./...`.

---

## Task 2 — Backup path: files orchestrator + Service backup methods

**Goal:** One restic snapshot per enabled file set, tagged `fileset:<Name>`, recorded as a run, with retention/off-site/stats identical to flash.

**Files:**
- `internal/backup/files_orchestrator.go` (new) — mirror `internal/backup/flash_orchestrator.go` exactly (no stop/start lifecycle, no defs):
  ```go
  type FilesRestic interface { Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error) }
  type FileSetBackupDeps struct { SourceDir, Repo, TargetID, SetName string; Excludes []string; Restic FilesRestic; Runs Runs }
  func BackupFileSetDir(ctx context.Context, d FileSetBackupDeps) (Summary, error)
  ```
  Tags: `[]string{"fileset:" + d.SetName}`. `Runs.Start(d.TargetID, kindBackup)` / `Finish` bracketing as in `BackupFlash`.
- `internal/backup/files_orchestrator_test.go` (new) — mirror `flash_orchestrator_test.go` (fake restic + fake runs; pins tag, excludes pass-through, run failure recording).
- `internal/api/service.go`:
  - `NewService` (line 214): add `"files": {}` to the `repoMu` map.
  - `filesRepoPath(settings store.Settings) (string, error)` next to `flashRepoPath` (line 432) → `s.resolveRepo(settings.FilesPath)`.
  - `BackupFileSet(ctx context.Context, id string) (backup.Summary, error)` — mirror `BackupFlash` (line 4861): detach + 12h cap, `defer s.lockDomain("files")()`, resolve set via `s.store.GetFileSet(id)`, resolve source dir via `paths.Resolve(s.cfg.HostMountRoot, set.Path)`, **fail with a clear "source path not found" error when `os.Stat` misses**, `EnsureRepo`, `unlockStale`, `notifyBackupStart(ctx,"files")`, `progBegin(ctx, "files:"+set.Name, "backup")`, call `backup.BackupFileSetDir` via the existing `resticAdapter`+`runsAdapter`, then `notifyBackup(ctx, "files", set.Name, ...)`, `applyRetention`, `makeRepoReadable`, `replicateOffsite(ctx,"files",...)`, `maybeCollectStats(ctx,"files")`.
  - `StartBackupFileSet(ctx, id) (bool, error)` — mirror `StartBackupVM` (line 2492): `batchActive` CAS, `domainBusy("files")`, detached goroutine.
  - `StartBackupFilesAll(ctx, ids []string) (bool, error)` — mirror `StartBackupAll` (line 2403) with progress key `batch:files` and no self-container skip.
- `internal/api/service_test.go` — extend the existing `fakeResticEngine` (line 2864) usage with file-set backup tests.

**Interfaces consumed:** Task 1's `store.FileSet`, `GetFileSet`, `ListFileSets`, `Settings.FilesPath`.
**Interfaces produced:** `Service.BackupFileSet`, `Service.StartBackupFileSet`, `Service.StartBackupFilesAll`, `Service.filesRepoPath`, progress keys `files:<name>` / `batch:files`. Consumed by Tasks 3–5.

**Tests:** orchestrator tests as above; service test pinning: backup of an enabled set produces `engine.Backup` call with tag `fileset:<Name>` and the set's excludes; a missing source dir records a failed run and returns an error; `internal/api/restore_ux_internal_test.go:19` (`repoMu` literal) updated to include `"files"`.

**Gate:** `go build ./... && go vet ./... && go test ./...`.

---

## Task 3 — Scheduler + process wiring for the files domain

**Goal:** `files` is schedulable exactly like the other domains: cadence, everyN due-gate, off-site schedule, Healthchecks aggregation, drills enumeration.

**Files:**
- `internal/schedule/schedule.go`:
  - `type ListFileSetsFunc func() ([]store.FileSet, error)` next to `ListVMTargetsFunc` (line 26).
  - `func (s *Scheduler) SetFilesJob(backupFn BackupFunc, listFn ListFileSetsFunc)` — mirror `SetVMJob` (line 280); `BackupFunc` receives the file set **ID**.
  - `func RunFilesJob(sets []store.FileSet, backupFn BackupFunc) (attempted, failed int)` — mirror `RunVMsJob` (line 690), gate on `set.Enabled`, call `backupFn(set.ID)`.
  - `ReloadWithDueChecks` (line 379): **signature grows a fifth `filesLastRun LastRunFunc`**; add a `files` `domainSpec` (cadence `settings.FilesSchedule`, fn wraps `runAggregatedHC("files", ...)` like containers/vms since it is a multi-item domain), and `offsite("files", settings.FilesOffsiteSchedule)` in the offsite block (line 473).
  - `Reload` (line 372) passes the extra nil.
  - `enabledDrillDomains` (line 610): add `FilesEnabled → "files"`. `immutableOffsiteDomains` (line 631): add `FilesOffsiteImmutable`. `drillTasks` (line 587): files gets the local subset check via `enabledDrillDomains`; add files to the off-site DR block beside flash (`settings.FilesEnabled && settings.FilesOffsite != ""` → `{files, offsite, dr}`) — files are DR-capable like flash.
- **All `ReloadWithDueChecks` callers** (compile errors will find them): `internal/api/handlers.go:1176`, `cmd/bombvault/main.go:256`, plus `internal/schedule/schedule_test.go` / `schedule_internal_test.go`.
- `internal/api/api.go` — `Handler` struct gains `filesLastRun schedule.LastRunFunc` (next to line 30) seeded in `NewHandler` from `st.LastSuccessfulFilesBackup`.
- `cmd/bombvault/main.go` (~line 227) — wire:
  ```go
  scheduler.SetFilesJob(func(id string) error {
      ctx := notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(context.Background()))
      _, bErr := svc.BackupFileSet(ctx, id)
      return bErr
  }, st.ListFileSets)
  filesLastRun := schedule.LastRunFunc(st.LastSuccessfulFilesBackup)
  ```
- `internal/notify/notify.go` — extend the `HealthchecksByDomain` doc comment (line 93) and `normalizeHCDomain` (line 163): `"files"` passes through as canonical key `"files"`.

**Interfaces consumed:** Task 1's `ListFileSets`/`LastSuccessfulFilesBackup`, Task 2's `BackupFileSet`.
**Interfaces produced:** `Scheduler.SetFilesJob`, `RunFilesJob`, 5-arg `ReloadWithDueChecks`.

**Tests:** `internal/schedule/schedule_test.go` — `RunFilesJob` skips disabled sets and counts failures; `ReloadWithDueChecks` registers a `files` entry for `FilesSchedule: "daily 03:00"` and a `files-offsite` entry; drill task list includes `{files, local, subset}` and (with `FilesOffsite` set) `{files, offsite, dr}` — mirror `schedule_internal_test.go:157`'s config-domain test.

**Gate:** `go build ./... && go vet ./... && go test ./...`.

---

## Task 4 — Domain-enumeration parity across handlers, service, status, runs, deploy, tamper

**Goal:** Every place the fixed domain set is enumerated accepts/reports `files`, so maintenance ops, dashboard status, history, metrics, notifications and the off-site wizard all work for the new domain with zero special-casing.

**Files (each is a small mechanical edit — the compile/tests keep it honest):**
- `internal/api/deploy.go:54` — add `"files"` to the switch (rest-server deploy snippet).
- `internal/api/tamper.go:54` — add `"files"`.
- `internal/api/handlers.go` domain switches: `handleCheck` (line 1247), `handleRunDrill` (1267), `handleDrills` (1288), `handleUnlock` (1329), `handlePrune` (1346), `handleDeleteSnapshot` (1363), `handleReplicateOffsite` (1380), `handleTestOffsite` (1398), `handleDeploySnippet` (1421), and the tamper-test handler — add `"files"` to each `case "containers", "vms", "flash"...` list.
- `internal/api/service.go`:
  - `offsiteRepoFor` (line 739), `offsiteScheduleFor` (line 756), `offsiteImmutableFor` (line 776): add `case "files"` → the `Files*` settings fields.
  - `repoFor` (line 5987): route `"files"` to `filesRepoPath`/`FilesOffsite`.
  - `runSubsetDrill` domain switch (line 5469): add `"files"`.
  - `runDRDrill` (line ~5600): allow `"files"` beside `"containers", "flash"` (the sandbox restore of a file-set snapshot is cheap, per spec); check `sandboxRestoreVerify` (line 5807) picks a snapshot generically — follow the flash path.
  - `DomainStatus` (line 1098): append `{"files", settings.FilesEnabled, settings.FilesSchedule, s.store.LastSuccessfulFilesBackup}` to the `domains` literal (line 1105–1115).
  - `runDomains` (line 1290): map every `file_sets.id` → `"files"` (needs `ListFileSets`).
  - `HistoryDay` (line 1277): add `Files DayStat` json:"files" and extend `bucketRunsByDay` (line 1312).
- `internal/api/handlers.go` `handleRuns` (line 1669): extend the `name`/`domain` maps with file sets (`name[fs.ID] = fs.Name; domain[fs.ID] = "files"`).
- `internal/api/handlers.go` `settingsView` (line 861), `toView` (line 932), `handlePutSettings` mapping (line 1115): add `filesEnabled`, `filesPath`, `filesSchedule`, `filesOffsite`, `filesOffsiteSchedule`, `filesOffsiteImmutable` (mirror the `config*` fields precisely, including the immutable-warning block at line 1186 gaining `s.FilesOffsiteImmutable`).

**Interfaces consumed:** Tasks 1–3. **Interfaces produced:** `/api/status` returns a 5th `DomainStatusEntry{Domain:"files"}`; `/api/runs` attributes file-set runs as `domain:"files"`; `/api/settings` carries the six new fields; all `/api/{check,verify,unlock,prune,offsite,snapshots}/{domain}` routes accept `files`.

**Tests:** extend `internal/api/status_internal_test.go` (DomainStatus includes files, disabled ⇒ `status:"off"`), `internal/api/history_internal_test.go` (files bucket), `internal/api/handlers_test.go` (PUT/GET settings round-trips the new fields; `POST /api/check/files` no longer 400s), `internal/api/notify_internal_test.go` (files domain healthchecks key), `internal/api/metrics_test.go` if it pins the domain list.

**Gate:** `go build ./... && go vet ./... && go test ./...`.

---

## Task 5 — Files REST API: CRUD, backup, snapshots, restore, discover

**Goal:** The complete `/api/files/*` surface: manage sets, fire async backups, list snapshots per set, restore (original path or chosen folder, never silent), and rebuild the set list from a bare repo.

**Files:**
- `internal/api/service.go`:
  - `FileSetView` struct (mirror `VMView`): `{ID, Name, Path string; Excludes []string; Enabled bool; LastBackup int64; PathExists bool}` — `LastBackup` from `store.LastSuccessfulBackup(fs.ID)` (runs-based, cheap; do NOT spawn restic per row), `PathExists` from `os.Stat` of the resolved path.
  - `ListFileSetViews(ctx) ([]FileSetView, error)`.
  - Validation helper `validateFileSet(fs store.FileSet) error`: name passes `validResourceName` (handlers.go:289 — it feeds restic tags + progress keys), path passes `paths.Resolve(s.cfg.HostMountRoot, fs.Path)` AND exists on disk (validate existence at save).
  - `SnapshotsFileSet(ctx, id, source string) ([]restic.Snapshot, error)` — mirror `Snapshots` (line 3285): resolve repo via `repoFor(settings, "files", source)`, `localRepoMissing` short-circuit, filter tag `"fileset:" + set.Name`.
  - `StartRestoreFileSet(ctx, id, snapshotID, source, targetSubPath string, confirm bool) (target string, started bool, err error)` — mirror `StartRestoreToPath` (line 3715): validate snapshot ownership via `SnapshotsFileSet` + `snapshotBelongs` (line 3845); `targetSubPath == ""` ⇒ restore in place to the set's original resolved path via `engine.RestorePath` (requires `confirm` — return `backup.ErrNotConfirmed` otherwise, like `prepareRestore` line 3015); non-empty ⇒ `paths.Resolve` + `paths.EnsureDir` + `engine.RestoreInclude(..., "/", target, ...)`. Detached goroutine records a run (generalize `beginRestoreRun` line 3545 to `beginRestoreRunForTarget(targetID string)`), progress key `files:<name>`, cancel registration via `registerCancel`.
  - `DeleteBackupsFileSet(ctx, id string) error` — mirror `DeleteBackupsVM` (line 3928): forget all `fileset:<Name>`-tagged snapshots + `DeleteFileSet` history.
  - `DiscoverFileSets(ctx, dryRun bool) (int, error)` — mirror `Discover` (line 2769) but **do NOT follow the defs-dir pattern** (no defs for files): collect names from `fileset:` tags, and for each unknown name upsert a **disabled** `store.FileSet{Name: name, Path: "", Enabled: false}` (path unknown from tags alone; the UI flags "set path before backup"; restore-to-folder works without a path). `dryRun` returns the count only (probe).
- `internal/api/handlers.go` — new handlers (mirror the VM handlers block at line 2024):
  `handleListFileSets`, `handleCreateFileSet`, `handlePatchFileSet`, `handleDeleteFileSet`, `handleBackupFileSet`, `handleBackupFilesAll`, `handleSnapshotsFileSet`, `handleRestoreFileSet`, `handleDeleteBackupsFileSet`, `handleDiscoverFiles` (probe param like handleDiscover line 385–396).
- `internal/api/api.go` `Router()` — register:
  ```
  GET    /api/files                      → handleListFileSets
  POST   /api/files/sets                 → handleCreateFileSet
  PATCH  /api/files/sets/{id}            → handlePatchFileSet
  DELETE /api/files/sets/{id}            → handleDeleteFileSet
  DELETE /api/files/sets/{id}/backups    → handleDeleteBackupsFileSet
  POST   /api/files/sets/{id}/backup     → handleBackupFileSet
  POST   /api/files/backup-all           → handleBackupFilesAll
  GET    /api/files/sets/{id}/snapshots  → handleSnapshotsFileSet
  POST   /api/files/sets/{id}/restore    → handleRestoreFileSet
  POST   /api/files/discover             → handleDiscoverFiles
  ```

**Interfaces consumed:** Tasks 1–4. **Interfaces produced (for Tasks 6, 7, 10):** the ten routes above; `Service.SnapshotsFileSet`, `Service.StartRestoreFileSet(ctx, id, snapshotID, source, targetSubPath, confirm)`, `Service.DiscoverFileSets`, `FileSetView` JSON (`{id,name,path,excludes,enabled,lastBackup,pathExists}`).

**Tests:** new `internal/api/files_internal_test.go` + additions to `handlers_test.go`: create/patch/delete round-trip; create rejects a traversal path and a non-existent path; restore without `confirm` and without target returns the not-confirmed error; restore-to-folder resolves + creates the target under the mount root and refuses `../`; discover from a fake engine returning `fileset:docs` tags creates one disabled set (and probe creates none); snapshots filtered by tag.

**Gate:** `go build ./... && go vet ./... && go test ./...`.

---

## Task 6 — Frontend: Files tab (page, nav, API client) + i18n

**Goal:** A first-class Files tab: list of file sets (name, path, excludes count, enabled, last backup, per-set Backup now, restore panel), add/edit/remove dialogs, empty state ("Backrest replacement" one-liner).

**Files:**
- `web/src/lib/api.ts` — add `FileSetView` interface (`{id,name,path,excludes,enabled,lastBackup,pathExists}`) and functions `listFileSets()`, `createFileSet()`, `patchFileSet()`, `deleteFileSet()`, `deleteFileSetBackups()`, `backupFileSet(id)`, `backupFilesAll(ids)`, `fileSetSnapshots(id, source?)`, `restoreFileSet(id, snapshot, confirm, target?, source?)`, `discoverFiles(probe=false)` — exact paths from Task 5. Extend the `Settings` interface (line 86) with `filesEnabled, filesPath, filesSchedule, filesOffsite, filesOffsiteSchedule, filesOffsiteImmutable`; extend `HistoryDay` (line 253) with `files: DayStat`; note `Run.domain` (line 177) now also carries `"files"`.
- `web/src/pages/Files.tsx` (new) — model on `web/src/pages/VMs.tsx` (the closest per-item domain page): a row per set with enabled switch (PATCH `enabled`), a backup button via `useBackupWatch` (`web/src/lib/backupWatch.ts`) with progressKey `files:<name>`, `matchRun: (r) => r.domain === "files" && r.target === name`, `start: () => backupFileSet(id)`; an expandable snapshot panel modeled on `VMRestorePanel` (VMs.tsx line 485) whose restore control offers "original location" (confirm dialog) vs "to folder" (`FolderBrowser` for the target subpath); add/edit dialog with name, `FolderBrowser` path picker, excludes textarea (one pattern per line), enabled toggle; `pathExists === false` renders a warning chip; empty state paragraph.
- `web/src/app/router.tsx` — add the `/files` route.
- `web/src/components/Sidebar.tsx` — `filesEnabled` gate + NavItem `nav.files` (mirror lines 317–319 and 487–489).
- `web/src/lib/i18n.ts` + ALL 24 `web/src/lib/locales/*.ts` — new keys: `nav.files`, `files.title`, `files.empty`, `files.addSet`, `files.editSet`, `files.name`, `files.path`, `files.excludes`, `files.excludesHint`, `files.enabled`, `files.pathMissing`, `files.deleteSet`, `files.deleteSetConfirm`, `files.deleteBackupsConfirm`, `files.restoreOriginal`, `files.restoreOriginalConfirm`, `files.restoreToFolder`, `files.backupAll`, plus whatever the dialogs need. Real translations in every locale.

**Interfaces consumed:** Task 5 routes + `FileSetView`. **Produces:** the `/files` page and `nav.files` key used by Task 7.

**Gate:** `npm --prefix /d/nextcloud/it/github/bombvault/web run build` + key-parity loop (all 24 locales equal, i18n.ts = 2x).

---

## Task 7 — Frontend: Settings, Dashboard, Recovery integration for files + i18n

**Goal:** Files appears everywhere the other domains do: Settings (enable toggle, storage path, schedule, off-site + immutable), Dashboard (protection card row + heatmap), Recovery (discover + restore), Notify per-domain Healthchecks.

**Files:**
- `web/src/pages/Settings.tsx`:
  - General/Storage tab: `filesEnabled` toggle + `FolderBrowser` for `filesPath` (mirror `configPath`/`flashPath`).
  - Schedules tab: new `FilesSection` mirroring `VMsSection` (line 1325) with the shared `CadenceBuilder` bound to `filesSchedule`, plus the per-set include list (enabled toggles reusing `patchFileSet`) mirroring `ContainersSection`'s included-list (line 1250); include `filesSchedule` + `filesOffsiteSchedule` in `buildSchedulePatch` (line 1684).
  - Off-site tab: `filesOffsite` URL + immutable toggle + off-site schedule (mirror the config domain rows).
  - NotifyCard per-domain Healthchecks list (line 680–686): add `["files", t("nav.files")]`.
  - IntegrityCard's stable literal domain list (near line 955): add `files`.
- `web/src/pages/Dashboard.tsx` — add the `files` label mapping (`t("nav.files")`) wherever `containers|vms|flash|config` labels are mapped, and extend the heatmap to read `HistoryDay.files`.
- `web/src/pages/Recovery.tsx` — extend `checkReadable` (line 145) and `runDiscover` (line 300) to include `discoverFiles(probe)` / re-fetch `listFileSets()`; render discovered file sets in Step 5 with a restore row (files restore uses `fireAndWaitRun` with `matchRun: r.domain === "files"`). Extend `web/src/lib/api.ts` `discoverAll()` (line 793) to a third discover.
- `web/src/lib/i18n.ts` + ALL 24 locales — keys for the new Settings section labels (`settings.filesPath`, `schedule.files.*` as needed), `recovery.filesFound`-style strings. Real translations everywhere.

**Interfaces consumed:** Tasks 4–6.

**Gate:** `npm --prefix web run build` + key-parity loop; `go build ./...` still green (no backend edits expected).

---

## Task 8 — Backend refactor: repo/mode seam for restore preparation (no behavior change)

**Goal:** Extract the repo+mode resolution out of the container/VM restore preparation so a caller can supply a NON-settings repo (the foreign session). Pure refactor — every existing test stays green unchanged.

**Files:**
- `internal/api/service.go`:
  - New internal type `repoRef struct { repo string; mode restic.Mode }`.
  - `prepareRestore` (line 3012) currently resolves `repo, err := s.repoFor(settings, "containers", source)` + `mode := s.ModeFor(settings)` (lines 3032–3040): split into `prepareRestoreIn(ctx, ref repoRef, name, snapshotID string, confirm bool) (containerRestorePlan, error)` containing everything from the `GetTargetByContainer` lookup down, with `prepareRestore` becoming resolve-then-delegate. The internal snapshot-ownership check must list snapshots against `ref` — extract `snapshotsForTag(ctx context.Context, repo string, mode restic.Mode, tag string) ([]restic.Snapshot, error)` from the tag-filter loop in `Snapshots` (line 3305–3319) and use it in both places.
  - Same split for `prepareRestoreVM` (line 4594) → `prepareRestoreVMIn(ctx, ref, name, snapshotID, confirm)`.
  - `executeRestore` (line 3130) and `executeRestoreVM` (line 4703) need no change.

**Interfaces produced (consumed by Tasks 9–10):** `repoRef`, `prepareRestoreIn`, `prepareRestoreVMIn`, `snapshotsForTag`.

**Tests:** no new behavior — the pin is the existing suite passing unmodified. Add one small internal test asserting `prepareRestore` and `prepareRestoreIn` produce identical plans for the same inputs.

**Gate:** `go build ./... && go vet ./... && go test ./...` — with `git diff --stat` showing changes ONLY in `internal/api/service.go` (+ the one new test).

---

## Task 9 — Foreign-repo session backend: open / inventory / close, with hard read-only + no-persistence guards

**Goal:** `POST /api/foreign/open` opens another BombVault instance's repo read-only with the OTHER instance's APP_KEY, returns a TTL'd in-memory session id + a domain→items→snapshots inventory, and provably never touches `Settings` or writes to the repo.

**Files:**
- `internal/api/foreign.go` (new):
  ```go
  type ForeignItem struct { Name string `json:"name"`; Snapshots []restic.Snapshot `json:"snapshots"` }
  type ForeignInventory struct { Containers, VMs, FileSets []ForeignItem } // json: containers/vms/fileSets
  type foreignSession struct { id, repo, key string; mode restic.Mode; expires time.Time }
  func (s *Service) OpenForeign(ctx context.Context, location, foreignKey string) (string, ForeignInventory, error)
  func (s *Service) CloseForeign(id string)
  func (s *Service) foreignSession(id string) (foreignSession, error) // TTL-checked, sweeps expired
  ```
  - Session store: `foreignMu sync.Mutex` + `map[string]foreignSession` fields on `Service` (struct at service.go line 135 area), TTL 30 minutes, id from `crypto/rand` (mirror `randomDeployPassword` in `deploy.go:39`).
  - `foreignKey` validated as 64 lowercase hex (same shape as `config.appKeyRe` — define a local regexp; the config one is unexported).
  - Location resolution: `s.resolveRepo(location)` (service.go:410) — a relative subpath under the host mount (a mounted share) or a restic remote URL verbatim. Remote backends use the LOCAL instance's already-stored cloud env (`s.cloudEnvFor`) — no new credential persistence.
  - Mode detection **read-only**: build `encMode := restic.Mode{Encrypted: true, Password: restickey.Derive(foreignKey), Env: cloudEnv}` and `plainMode := restic.Mode{Env: cloudEnv}`; probe with `s.engine.RepoOpens(ctx, repo, ...)` (the `restic cat config` probe, ResticEngine line 65). **Do NOT follow the `EnsureRepo` pattern (service.go:1771) — it INITIALIZES a missing repo, which would write into the foreign location.** A repo that opens with neither mode returns a clear "wrong key or not a BombVault/restic repo" error.
  - Inventory: `s.listSnapshots(ctx, repo, mode)` once, then group by tag prefixes `container:`, `vm:`, `fileset:` (the same prefixes Discover cuts — see service.go:2794).
- `internal/api/handlers.go` — `handleForeignOpen` (`POST /api/foreign/open` body `{location, key}` → `{ok, session, inventory}`), `handleForeignClose` (`POST /api/foreign/close` body `{session}`). Key is never logged; errors scrubbed via `scrubError`.
- `internal/api/api.go` `Router()` — register `POST /api/foreign/open` and `POST /api/foreign/close` (authGate-protected automatically; do NOT touch the public allowlist).
- `internal/api/foreign_internal_test.go` (new, `package api`):
  - **Settings-untouched guard:** read `store.GetSettings()` before, run `OpenForeign`, read again, `reflect.DeepEqual` must hold. The Recovery attach flow persists via `putSettings`/`UpdateSettings` — foreign mode must never call `UpdateSettings`.
  - **Read-only guard:** a call-recording fake engine asserts `OpenForeign` performs no `Init`, `Forget`, `ForgetPolicy`, `Prune`, `TagAdd`, `Backup`, or `Copy` calls.
  - Session TTL expiry and unknown-session errors.

**Interfaces consumed:** `restickey.Derive`, `resolveRepo`, `listSnapshots`.
**Interfaces produced (for Tasks 10–11):** `Service.OpenForeign`, `Service.CloseForeign`, `Service.foreignSession`, `ForeignInventory` JSON, routes `/api/foreign/open|close`.

**Gate:** `go build ./... && go vet ./... && go test ./...`.

---

## Task 10 — Foreign restore backend: containers, VMs, file sets from the session repo

**Goal:** `POST /api/foreign/restore` restores one item snapshot from the foreign repo through the EXISTING restore paths (async, progress, recorded runs); the restored object becomes a normal local container/VM/file-set.

**Files:**
- `internal/api/foreign.go`:
  ```go
  func (s *Service) StartForeignRestore(ctx context.Context, sessionID, domain, item, snapshotID string, confirm bool, targetSubPath string) (bool, error)
  ```
  - Resolve `foreignSession(sessionID)` → `repoRef{sess.repo, sess.mode}` (Task 8 type).
  - `confirm == false` → return `backup.ErrNotConfirmed` (same conflict discipline as `prepareRestore`; the UI confirms when a same-named local container/VM exists — no silent overwrite).
  - **containers:** read the encrypted def from the foreign repo's defs dir — `readStoredDef(filepath.Join(sess.repo, "def"), filepath.Join(filepath.Dir(sess.repo), "bombvault-defs"), fn)` (helpers at service.go:2674/2584, filename via `defFileName` line 2750) — decrypt with `secret.Decrypt(sess.key, enc)` (**the foreign APP_KEY, not `s.cfg.AppKey`**), `store.UpsertTarget` locally with the plain definition + appdata paths (exactly what `Discover` does at line 2831), then `prepareRestoreIn(ctx, ref, name, snapshotID, true)` + detached `executeRestore` under `batchActive` (mirror `StartRestore`, line 3215: progress key `container:<name>`, cancel registration, run recorded against the upserted target).
  - **vms:** mirror with `vm-def` (`vmDefsDir` layout, service.go:2851), `UpsertVMTarget`, `prepareRestoreVMIn` + `executeRestoreVM` (progress `vm:<name>`).
  - **files:** no defs — restore the `fileset:<item>`-tagged snapshot into `targetSubPath` (required for files; `paths.Resolve` + `EnsureDir` + `engine.RestoreInclude(ctx, sess.repo, snap, "/", target, sess.mode)`), reusing Task 5's detached-run helpers with progress `files:<item>`; upsert a local disabled `store.FileSet` when the name is unknown so the run is attributable.
  - Snapshot ownership: `snapshotsForTag(ctx, sess.repo, sess.mode, "<prefix>:"+item)` + `snapshotBelongs` (line 3845); `"latest"` resolves to the newest matching snapshot.
  - **Never** call `EnsureRepo`, `applyRetention`, `Forget*`, `Prune`, `TagAdd`, or `makeRepoReadable` against `sess.repo`.
- `internal/api/handlers.go` — `handleForeignRestore` (`POST /api/foreign/restore` body `{session, domain, item, snapshot, confirm, target}` → `{ok, started}` / 409 busy like `handleBackup` line 421).
- `internal/api/api.go` `Router()` — register `POST /api/foreign/restore`.
- `internal/api/foreign_internal_test.go` — extend:
  - restore round-trip with fake engine + fake docker: def decrypts with the SESSION key, a local target row appears, the restore plan's repo == the session repo (not the settings repo).
  - settings byte-identical after open+restore.
  - engine-call assertion: restore performs only reads + `RestorePath`/`RestoreInclude` — no writes to the foreign repo.
  - unconfirmed restore returns the not-confirmed sentinel; unknown session 4xx.

**Interfaces consumed:** Task 8 (`repoRef`, `prepareRestoreIn`, `prepareRestoreVMIn`, `snapshotsForTag`), Task 9 (session), Task 5 (file-set restore helpers).
**Interfaces produced (for Task 11):** route `POST /api/foreign/restore`; runs appear in `/api/runs` with `kind:"restore"` so `fireAndWaitRun` works unchanged.

**Gate:** `go build ./... && go vet ./... && go test ./...`.

---

## Task 11 — Frontend: "Restore from another BombVault repo" card on Recovery + i18n

**Goal:** A clearly separated card on the Recovery page with the 3-step flow (connect → browse → restore) that leaves local settings untouched.

**Files:**
- `web/src/lib/api.ts` — `foreignOpen(location, key)`, `foreignClose(session)`, `foreignRestore({session, domain, item, snapshot, confirm, target?})` + `ForeignInventory`/`ForeignItem` types matching Task 9/10 JSON.
- `web/src/pages/Recovery.tsx` — append a new `StepCard`-based section **visually separate from the attach steps**:
  1. **Connect:** location input (a `FolderBrowser` for a mounted-share subpath under `hostMountRoot`, plus a free-text field accepting `rest:`/`s3:`/… URLs — mirror the `offsiteInput` styling at Recovery.tsx line 401) + a password-type APP_KEY field; "Connect" calls `foreignOpen` and stores `{session, inventory}` in component state only.
  2. **Browse:** inventory grouped Containers / VMs / File sets, each item with a snapshot select (default latest) — same list styling as `RestoreRow` (line 50).
  3. **Restore:** per-item Restore button via `fireAndWaitRun` (`web/src/lib/backupWatch.ts` line 315) with `matchRun: (r) => r.domain === "<container|vm|files>" && r.target === item` and `start: () => foreignRestore({...confirm: true})`; when the item name already exists locally, a confirm with the explicit overwrite warning fires first; file-set items additionally require a target folder (`FolderBrowser`).
  - Copy makes the semantics explicit: "Reads the other repo, changes nothing over there, and leaves your own backup settings untouched."
  - **Anti-pattern note:** the neighbouring `connectPreview` (line 178) persists via `putSettings` — the foreign card must never call `putSettings`; on unmount/leave call `foreignClose`.
- `web/src/lib/i18n.ts` + ALL 24 locales — keys: `recovery.foreignTitle`, `recovery.foreignIntro`, `recovery.foreignLocation`, `recovery.foreignLocationHint`, `recovery.foreignKey`, `recovery.foreignConnect`, `recovery.foreignConnected`, `recovery.foreignEmpty`, `recovery.foreignRestore`, `recovery.foreignExistsConfirm`, `recovery.foreignTargetFolder`, `recovery.foreignClose`, error strings. Real translations in every locale.

**Interfaces consumed:** Tasks 9–10 routes; existing `fireAndWaitRun`, `StepCard`, `FolderBrowser`.

**Gate:** `npm --prefix web run build` + key-parity loop (all 24 equal, i18n.ts = 2x).

---

## Task 12 — README, docs, final full gate

**Goal:** Document both features and run every gate across the finished tree.

**Files:**
- `README.md` — feature bullets + sections: "Files backup" (arbitrary host folders under `/mnt`, per-set excludes, schedules/off-site/drills parity — note that sources must be visible under the container's `/host/user` mount, which the Unraid template's `/mnt` mapping covers, see `templates/my-BombVault.xml` line 78–85) and "Restore from another BombVault repo" (Recovery page, one-time read-only session, settings untouched; server-to-server federation explicitly out of scope per #61).
- Verify the two spec files' promises against the implementation (checklist pass): fixed-domain enumeration sites all extended (grep `"containers", "vms", "flash", "config"` — every hit must now include `files` or have a documented reason), foreign mode has the settings-untouched test, all new endpoints registered in `Router()`.
- CA template `<Overview>` and unraid-apps entry are explicitly post-release follow-ups — do not edit the template XML here.

**Gate (full):**
```sh
cd /d/nextcloud/it/github/bombvault
go build ./... && go vet ./... && go test ./...
npm --prefix web run build
cd web && for f in src/lib/locales/*.ts; do grep -cE '^\s+"[a-zA-Z0-9.]+":' "$f"; done | sort -u   # exactly one number
grep -rn '"containers", "vms", "flash", "config"' internal/ | grep -v files                          # audit leftovers
```

---

## Cross-cutting "do NOT follow the existing pattern" list

1. **Recovery attach persists settings** (`Recovery.tsx` `connectPreview` → `putSettings`; backend `handlePutSettings` → `store.UpdateSettings`): the foreign mode (Tasks 9–11) must never touch this path — sessions are in-memory with TTL; guarded by the settings-byte-identical test.
2. **`EnsureRepo` initializes missing repos** (`service.go:1771`): foreign open/restore must use `RepoOpens` probes only — `EnsureRepo` against a foreign location would create an empty repo on someone else's storage.
3. **Defs-dir mirroring** (`writeDefToStorage`/`defsDir`): the files domain has no recreate-definition concept — `DiscoverFileSets` works from `fileset:` tags alone; do not create a `<repo>/def` for files.
4. **Def decryption key**: foreign defs decrypt with the SESSION's foreign APP_KEY, never `s.cfg.AppKey`.
5. **authGate public allowlist** stays exactly 4 entries — new endpoints are protected by default via registration in `Router()`.
