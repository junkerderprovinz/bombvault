# Self-config-backup (the `config` domain) — design

**Source:** GitHub issue #26 (manilx): *"Ideally, BombVault would also be able to back itself up, including
the entire folder, ready to be deployed on Unraid :)"*. This closes the last DR gap after the guided
Recovery tab (v4.1.0) and the recovery-kit credentials (v4.1.1): BombVault can back up **its own settings,
targets and credentials** so a fresh Unraid install restores all of it in one shot (the `APP_KEY` still comes
from the recovery kit).

**Branch:** `feat/config-backup` (off `main` == v4.1.1). **Stack:** Go single-binary backend (restic engine,
`modernc.org/sqlite`) + the embedded React/Vite/TS SPA.

**Goal:** a fourth backup domain, **`config`**, that restic-backs-up BombVault's own `/config` folder (the
SQLite settings DB + `rclone.conf` + the libvirt SSH keypair) with **no container stop** — scheduled and
on-demand, off-site-replicated like the other domains, and restorable through the Recovery tab so a rebuilt
box recovers BombVault itself before it discovers anything else.

**Why this is allowed even though `ErrSelfBackup` forbids self-backup:** `ErrSelfBackup`
(`service.go:1702`) forbids backing up BombVault's own **container**, because a container backup would
`docker stop` the very container mid-backup and crash it. The `config` domain sidesteps that entirely — it is
a **folder/DB backup** with no lifecycle (exactly like the Flash domain), so nothing stops mid-backup. The
container-stop that is forbidden during a *backup* is, by contrast, the correct and controlled mechanism for
*applying a restore* (see §4), because at that moment no backup is in flight.

---

## Template: the Flash domain

The three existing domains are **containers**, **vms**, **flash**. **Flash** is the correct analog: a
whole-folder restic backup with **no lifecycle** (no stop/start), a fixed singleton target id, its own repo
and off-site settings, and a scheduler job. `config` is almost a clone of Flash, with two genuinely new
problems Flash does not have:

1. **A live, actively-written SQLite DB** — the raw `.sqlite` cannot be safely snapshotted (WAL mode). §3.
2. **A self-referential restore** — the running process holds the DB open and caches boot-time state, so a
   restore cannot be applied live; it must be staged and applied on restart. §4.

The Flash backup itself (`internal/backup/flash_orchestrator.go`) is a thin record-around-restic with **no
`docker stop`/`virsh` anywhere** — `BackupFlash` just: resolve repo → `EnsureRepo` → `unlockStale` →
`restic backup <dir>` → retention/offsite/stats. `config` follows this exactly; the only change is that its
`SourceDir` is a **staged snapshot directory** (§3), not a live mount.

---

## 1. What the config backup captures

The `/config` folder (`DataDir`, default `/config`, `internal/config/config.go:43`) holds:

- `bombvault.sqlite` (+ `-wal` + `-shm` sidecars in WAL mode) — the settings DB: all domain settings, target
  definitions, run history, schedules, off-site config. `config.go:58`.
- `rclone.conf` — off-site rclone remotes, materialized from the DB at boot (`main.go:110`). `service.go:4944`.
- `ssh/id_ed25519` + `ssh/known_hosts` — the libvirt SSH keypair used for VM backups. `sshconn.go:31-35`.

All three go into the snapshot. Transient probe files (`.bombvault-write-test`, `main.go:42`) and any
existing staging/restore-staging dirs are excluded.

## 2. Backup path (Flash clone)

Mirror every Flash touch-point for `config`:

- **Settings** (`internal/store/settings.go`): add `ConfigEnabled`, `ConfigPath`, `ConfigSchedule`,
  `ConfigOffsite`, `ConfigOffsiteSchedule`, `ConfigOffsiteImmutable` to the struct, the `GetSettings`
  SELECT/Scan, and the `UpdateSettings` UPDATE/Exec — one line each, mirroring the six `Flash*` lines.
- **Migration** (`internal/store/migrate.go`): new forward-only migrations from **v44** (current max is 43),
  one single-column `ALTER TABLE settings ADD COLUMN` each: `config_enabled INTEGER NOT NULL DEFAULT 0`,
  `config_path TEXT NOT NULL DEFAULT 'user/bombvault/config'`, `config_schedule TEXT DEFAULT 'off'`, plus the
  three off-site columns. Idempotent.
- **Orchestrator** (`internal/backup/config_orchestrator.go`, new): a clone of `flash_orchestrator.go` —
  `ConfigRestic`, `ConfigBackupDeps`, `BackupConfig`; tag `"config"`; `SourceDir` = the staged snapshot dir.
- **Service** (`internal/api/service.go`): `BackupConfig` (clone of `BackupFlash` `:4028`) that first **stages
  the snapshot** (§3) then runs the orchestrator + retention/offsite/stats; `StartBackupConfig` (clone of
  `StartBackupFlash` `:1949`, the detached-goroutine single-flight launcher). `resticAdapter` already
  satisfies the generic `Backup`, so it satisfies `ConfigRestic` too.
- **Store helpers** (`internal/store/runs.go`): `const ConfigTargetID = "config"` (reserved singleton run id,
  like `FlashTargetID`), `LastSuccessfulConfigBackup`, and a `WHEN target_id = 'config' THEN 'config'` arm in
  `RunCounts`. Run-history name maps: `handlers.go:1539-1540`, `service.go:952`.
- **Repo resolution + mutex** (`service.go`): `configRepoPath`, and a `case "config"` arm in `repoFor`
  (`:4747`), `offsiteRepoFor`, `offsiteScheduleFor`, `offsiteImmutableFor`, the domain-validation switch
  (`:4306`), and the `repoMu` map init (`:217`).
- **API** (`internal/api/api.go`): `POST /api/config/backup`, `GET /api/config/snapshots`, and
  `POST /api/config/restore` (in-place staged restore, §4) — handlers in `handlers.go` cloned from
  `handleBackupFlash`/`handleSnapshotsFlash`.
- **Settings DTO** (`handlers.go`): add the `config*` JSON fields to the GET/PUT settings body and mapping,
  include `ConfigOffsite` in off-site path validation and `ConfigOffsiteSchedule` in the cadence-validation
  loop.
- **Scheduler** (`internal/schedule/schedule.go`): `SetConfigJob`, a `config` task in `ReloadWithDueChecks`
  with a `configLastRun` due-gate param (signature change → `main.go` + `handlers.go:1063` call sites), and
  the `config` arms in the off-site + enabled-domain lists. **Exclude `config` from `drillDomains`** (a
  sandbox DR-drill of the settings DB is meaningless — same exclusion VMs get).
- **DomainStatus + metrics** (`service.go` + `metrics.go`): a `{"config", settings.ConfigEnabled,
  settings.ConfigSchedule, s.store.LastSuccessfulConfigBackup}` row in the domain-status list (`:802-804`) and
  the protection-scorecard list (`:1157`). Metrics iterate `DomainStatus()` generically — `config` emits
  automatically.
- **Frontend** (`web/src/pages/Config.tsx`, new): a Flash-style page (backup-now, schedule, off-site,
  snapshots list) + `/config` route + a `configEnabled`-gated NavItem + `api.ts` client fns + the `config*`
  fields on the `Settings` DTO type.
- **i18n**: a `config.*` key block + `nav.config` in `i18n.ts` **and all 24 locale files** (release gate).

**Opt-in, but nudged.** Like the other domains, `config` is off until the user enables it and points it at a
repo (it needs a target). To make manilx's intent land, the Recovery tab and the Dashboard fresh-install /
scorecard surface a clear "protect BombVault's own settings" nudge when `config` is disabled.

## 3. SQLite-consistent snapshot (Problem 1)

The DB runs in **WAL mode** (`store.go:19`, `PRAGMA journal_mode=WAL`, `SetMaxOpenConns(1)`), so the newest
committed data lives in `bombvault.sqlite-wal`, not the main file. Restic-backing-up the raw `.sqlite` while
the process holds it open would capture a **torn/stale** DB. Solution:

- Right before the orchestrator runs, `BackupConfig` populates a **staging directory** (e.g.
  `/config/.snapshot/`): `db.Exec("VACUUM INTO ?", stagingDBPath)` produces a fully-consistent single-file
  snapshot with the WAL folded in (standard SQLite ≥3.27 SQL, supported by `modernc.org/sqlite`, **no new
  dependency, no C backup API**). Then copy `rclone.conf` and the `ssh/` dir alongside.
- restic backs up the staging dir; the staging dir is removed afterward (best-effort; a leftover is
  overwritten next run and is excluded from its own backup).
- The single pooled connection (`MaxOpenConns(1)`) means no concurrent writer can interleave with the VACUUM.

`SourceDir` for the `config` orchestrator is therefore the **staging dir**, never `/config` itself.

## 4. Self-referential restore (Problem 2) — staged, applied on restart

The running process holds the DB open (WAL, one connection) and caches boot-time state that a restored DB
would NOT retroactively change: env (`config.Load`, `main.go:56`), the on-disk `rclone.conf` (written from the
DB at boot, `main.go:110`), the SSH key (`main.go:97`), and the resolved `selfContainerName`
(`service.go:1709`). Overwriting the live `.sqlite` under the open handle is a classic corruption / stale-state
hazard. Therefore restore is **staged and applied on restart**:

1. **`RestoreConfig`** does a real `restic restore` of the chosen snapshot into a staging dir
   (`/config/.restore-staging/`) and writes a restore-pending marker. It **never** touches the live DB.
2. **Apply = restart.** BombVault triggers its own restart the **robust, sanctioned way**: over the mounted
   **Docker socket** (`docker restart <selfContainerName>`) — the same daemon path it already uses to
   stop/start other containers for backups. This is daemon-driven, so it does **not** depend on a
   `--restart` policy being set (the weakness of a plain `os.Exit`), and it does **not** rely on BombVault
   being the container's PID-1 main process (the fragility of an in-process `syscall.Exec`). `selfContainerName`
   is already resolved for the "hide my own container" feature.
   - **Fallback:** if the self-name can't be resolved or the socket call fails, the UI shows a clear
     "restart the BombVault container to apply" instruction. Recovery still completes on the next manual
     restart; nothing is lost.
3. **On boot, before `store.Open`:** `main.go` checks for the restore-pending marker; if present and the
   staging DB is valid, it atomically swaps `/config/.restore-staging/*` → live (`bombvault.sqlite`,
   `rclone.conf`, `ssh/`), removes stale `-wal`/`-shm`, and clears the marker. The swap runs **before** the DB
   is opened, so there is no open-handle hazard. Idempotent: a half-completed swap leaves the marker, so it is
   retried on the next boot.
4. After the swap, normal boot re-reads env, opens the restored DB, runs migrations (forward-only, idempotent,
   `migrate.go:405`), re-materializes `rclone.conf` from the restored DB, and re-registers the scheduler — the
   moment the restored settings actually take effect.

**UI copy** states plainly: "Restoring BombVault's own configuration requires a restart to apply. BombVault
will restart itself and come back with your settings restored." The Recovery step optimistically shows
"restarting…" and polls for the app to return.

## 5. Recovery-tab integration (Problem 3) — config restore comes first

The current Recovery flow (`web/src/pages/Recovery.tsx`) is: Step 1 connection check → Step 2 **attach
backups (user manually types repo paths)** → Step 3 discover → Step 4 restore-all → Step 5 kit. A config
restore is precisely the thing that would **auto-provide** those repo paths — so it must run **before**
discovery, but it needs a bootstrap repo location to restore *from*. This is chicken-and-egg, and the resolution
is the same one `APP_KEY` already uses: **one bootstrap value the user still supplies.**

- Add a **Step 2a "Restore BombVault's own settings"** (before Step 3 discovery). The user supplies only the
  **config-repo location** (local path or off-site URL) + the `APP_KEY` (already covered by Step 1).
  BombVault restores the config DB from it and restarts.
- Because §4 requires a restart, this step is **not** purely in-page like the others: it ends with
  "restarting to apply…", the UI polls, and when BombVault returns it comes back with `containersPath` /
  `vmsPath` / `flashPath` / off-site / rclone / cloud creds **all pre-filled** — collapsing Step 2's manual
  entry. Steps 3/4 then proceed unchanged against the restored settings.
- The config-repo location is the single bootstrap seed the user must still provide (same role `APP_KEY`
  plays). The **recovery kit** (`export.go` / `RecoveryKit` `service.go:5217`) also prints the config-repo
  path, so the user has it written down.
- Step 2a is **optional/skippable**: a user who prefers manual attach (Step 2) can skip it; a user who has a
  config backup uses it and skips most of Step 2.

## 6. Naming & scope

- **Internal domain key: `config`.** Safe against `containers`/`vms`/`flash` (no code path uses `"config"` as
  a domain today). `ConfigTargetID = "config"` won't collide (container/VM ids are hex/UUID; Flash uses the
  literal `"flash"`).
- **Display name: "App configuration" / "Settings backup."** The word "config" already names the `/config`
  DataDir mount; user-facing copy uses the clearer label to avoid ambiguity while the internal key stays
  `config`.
- Does **not** conflict with `ErrSelfBackup` (different mechanism: no container lifecycle during backup).

## Error handling

- **VACUUM fails / disk full** → the backup run records failure via the normal run-status path; no partial
  snapshot is shipped (restic only runs on a fully-written staging dir).
- **APP_KEY mismatch on config restore** → the existing mapped guidance ("APP_KEY differs…", `handlers.go:78`).
- **Self-restart fails** (name unresolved / socket error) → fall back to the manual-restart instruction; the
  staged restore + marker persist, so the next manual restart applies it.
- **Swap-on-boot fails / staging DB invalid** → leave the live DB untouched, clear/keep the marker safely, and
  log; BombVault boots on the pre-restore DB rather than a corrupt one (fail-safe, never fail-open).

## Testing

- **Go gates:** `go build ./... && go vet ./...`, `gofmt -l` empty, `go test ./... -count=1`,
  `golangci-lint run ./internal/...` clean.
- **Unit tests:**
  - the VACUUM-INTO staging producing a readable, consistent snapshot (open the staged file, sanity-query);
  - `ConfigBackupDeps`/`BackupConfig` records a run and tags `"config"` (orchestrator test, Flash test as
    template);
  - the boot-time staging→live **swap** function: marker present + valid staging → files swapped, marker
    cleared, `-wal`/`-shm` removed; invalid staging → live untouched; idempotent on re-run;
  - `LastSuccessfulConfigBackup` / `RunCounts` `config` arm;
  - settings round-trip (GET→PUT→GET) preserves the new `config*` fields.
- **Frontend gate:** `cd web && npx tsc --noEmit && npm run build`, then restore the built
  `web/dist/index.html`.

## Out of scope (YAGNI)

- Backing up BombVault's **container/image** itself (that stays forbidden by `ErrSelfBackup`; `config` restore
  onto a freshly-deployed container is the sanctioned path — the image comes from the registry).
- A config **DR-drill** (excluded from `drillDomains`; a sandbox restore of the settings DB is meaningless).
- Making `config` enabled-by-default (it needs a target repo the user must choose first; solved by the nudge).
- Encrypting the config snapshot differently from the other domains (it uses the same repo encryption / restic
  password derivation — the credentials it contains are already what the recovery kit prints).

## File map

- **Create:** `internal/backup/config_orchestrator.go`, `web/src/pages/Config.tsx`,
  `docs/superpowers/plans/2026-07-04-self-config-backup.md` (plan).
- **Modify (backend):** `internal/store/settings.go`, `internal/store/migrate.go`, `internal/store/runs.go`,
  `internal/api/service.go` (`BackupConfig`/`StartBackupConfig`/`RestoreConfig`/`SnapshotsConfig`,
  `configRepoPath`, all domain switches, DomainStatus/scorecard), `internal/api/handlers.go` (routes' handlers,
  settings DTO, run-name maps), `internal/api/api.go` (routes), `internal/schedule/schedule.go`
  (`SetConfigJob` + task + due-gate), `cmd/.../main.go` (boot-time staging→live swap before `store.Open`;
  `SetConfigJob` wiring; `ReloadWithDueChecks` signature), `internal/api/deploy.go:54` (add `"config"` if
  selectable), `internal/api/export.go` (config-repo path in the recovery kit).
- **Modify (frontend):** `web/src/app/router.tsx` (+`/config` route), `web/src/components/Sidebar.tsx`
  (+NavItem), `web/src/pages/Recovery.tsx` (Step 2a config restore + restart/poll), `web/src/lib/api.ts`
  (config client fns), `web/src/lib/i18n.ts` (+`config.*`, `nav.config`) + all 24
  `web/src/lib/locales/*.json`.
- **Docs:** this spec; the implementation plan.
