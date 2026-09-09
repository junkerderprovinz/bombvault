# Module Map: internal/store, internal/releasenotes, and remaining internal/ packages

**Analysis Date:** 2026-09-09

Scope: `internal/store/` (SQLite settings/state), `internal/releasenotes/` (release notes embedded into the binary), plus one-paragraph summaries of every `internal/` package not covered by the other module maps (cmd/build, api, backup, restic, web are out of scope here).

## Technology

**SQLite driver:** `modernc.org/sqlite` — pure Go, no CGo, registered under the driver name `"sqlite"` (blank import at `internal/store/store.go:8`). This is what keeps the multi-arch Docker build (amd64 + arm64) free of a C toolchain.

**Connection setup** — `Open(path)` in `internal/store/store.go:12`:
- `db.SetMaxOpenConns(1)` — a single pooled connection, so writes can never race (no `SQLITE_BUSY`). This is load-bearing for more than perf: `MutateSettings` (`internal/store/settings.go:454`) relies on the one-connection pool so its transaction serializes against all other writers. A nested store call inside a `MutateSettings` mutation fn deadlocks by design and is documented as forbidden.
- `PRAGMA journal_mode=WAL` — concurrent reads during writes.
- `PRAGMA foreign_keys=ON` — enforced per-connection (SQLite default is off).

**Database file location:** `<DataDir>/bombvault.sqlite`, derived in `internal/config/config.go:104` (`c.DBPath = filepath.Join(c.DataDir, "bombvault.sqlite")`). Opened/migrated/wired once in `cmd/bombvault/main.go:192-200` (`store.Open` → `store.Migrate` → `store.New`).

**Migration approach:** hand-rolled, forward-only, code-defined. No external migration tool. A single ordered slice `migrations` in `internal/store/migrate.go:68` (currently v1..v99, 1379 lines) plus `Migrate(db)` at `internal/store/migrate.go:1317`, which:
1. Creates the tracking table `schema_migrations (version INTEGER PRIMARY KEY, name TEXT, applied_at INTEGER)`.
2. For each migration not yet recorded: begins a transaction, optionally probes `alreadySatisfied` (see Concerns), executes the SQL body, records the version row, commits. Each migration is its own transaction, so a crash between migrations leaves a consistent prefix.

**Embedding:** `internal/releasenotes` uses `//go:embed notes/*.md` (`internal/releasenotes/releasenotes.go:15`). Release notes are compiled into the binary, not read from disk at runtime.

## Integrations

**Who consumes the store** (non-test imports of `internal/store`):
- `cmd/bombvault/main.go:192-200` — the only place `store.Open`/`store.Migrate` run in production; produces the single `*store.Repo` handed to the API service and scheduler.
- `internal/api/` — the HTTP service holds the `*store.Repo` and is the dominant consumer (settings CRUD, targets, runs, off-site targets, drills, fleet/mesh peers, receiver state). Example imports: `internal/api/api.go:13`, `internal/api/digest.go:11`, `internal/api/displayprefs.go:9`.
- `internal/schedule/` — reads the settings row to build per-domain cron cadences (`internal/schedule/schedule.go:18`, `internal/schedule/effective.go:3`); also consults run/job history for due-gates and catch-up.

**Who consumes releasenotes:** `internal/api` serves the "What's new" dialog from the embedded notes. The dialog used to fetch api.github.com at runtime, but the app's own CSP (`connect-src 'self'`) blocked it (#54) — see the package doc at `internal/releasenotes/releasenotes.go:1-7`.

**File-system coupling:**
- `internal/store` owns nothing on disk except the SQLite file itself (and its WAL/SHM siblings) under the host-mounted `/config` data dir.
- `Repo.VacuumInto(dst)` (`internal/store/vacuum.go:10`) writes a consistent single-file snapshot via `VACUUM INTO`; the config-backup and self-restore flows stage into it.
- `internal/selfrestore` swaps the live `bombvault.sqlite` file at boot — the only safe moment, since the running process holds the DB open (see the package summary below).

**Release-notes sync coupling:** every note must exist in BOTH `.github/release-notes/vX.Y.Z.md` AND `internal/releasenotes/notes/vX.Y.Z.md`. An embed-sync test fails otherwise (see Testing). As of this analysis both directories hold the same 64 files (v1.0.0 through v8.6.4).

## Architecture

**Schema organization.** Eighteen tables, all created by migrations in `internal/store/migrate.go`:

| Table | Purpose | Accessor file |
|---|---|---|
| `settings` | ONE row (`id = 1`, enforced by CHECK), ~90 columns of global app config | `internal/store/settings.go` |
| `targets` | registered containers (backup items) + their definitions/options | `internal/store/targets.go` |
| `vms` | registered VMs | `internal/store/vms.go` |
| `runs` | every backup/restore/replicate execution, incl. `completed`, `group_id`, `acknowledged` | `internal/store/runs.go` |
| `repo_stats` | restic repo statistics cache | `internal/store/repostats.go` |
| `restore_drills` / `tamper_tests` / `offsite_runs` | DR-drill, tamper-test, off-site run history | `internal/store/drills.go`, `internal/store/offsite.go` |
| `established_repos` | repos already "established" (seeded/proven) | `internal/store/established.go` |
| `file_sets` | folder-set backup definitions (Files domain) | `internal/store/filesets.go` |
| `watchdog_state` | overdue-backup watchdog episode state | `internal/store/watchdog.go` |
| `offsite_targets` | off-site replication targets incl. `role` | `internal/store/offsite_targets.go` |
| `received_repos` / `received_alert_state` | receiver-dashboard repo inventory + alert latch | `internal/store/received_repos.go`, `internal/store/received_alert_state.go` |
| `fleet_peers` / `mesh_offers` | Fleet peer list / mesh offers | `internal/store/fleet_peers.go`, `internal/store/mesh_offers.go` |
| `schedule_job_runs` | last-fire record per scheduled job (everyN due-gate) | `internal/store/schedule_job_runs.go` |
| `schema_migrations` | applied-migration ledger | `internal/store/migrate.go` |

**Settings vs state separation.** `settings` is configuration (single row, one Go struct `Settings` in `internal/store/settings.go:10-286`); everything else is state/history with one accessor file per domain. There is deliberately no JSON-blob settings table: the row is flat and columnar so each setting is individually migratable and defaultable. The few opaque exceptions are explicit and documented — `Definition` on targets, `DisplayPrefs`, and the encrypted blobs `RcloneConf`/`NotifyConf`/`CloudConf`/`CloudCredSets`/`RegistryAuths`/`TOTPSecret` (AES-256-GCM at rest, see `internal/secret`).

**Accessor pattern.** One `*store.Repo` (`internal/store/repo.go:12`) wrapping `*sql.DB`; methods grouped per domain file (`GetSettings`, `UpsertTarget`, `StartRun`, ...). IDs are 32-hex-char crypto-random strings from `newID()` (`internal/store/repo.go:31`). Timestamps are Unix seconds. Booleans are SQLite INTEGERs converted at the scan boundary (`internal/store/settings.go:389-415`) — Go structs carry `bool`, SQL carries 0/1.

**The settings read/write contract (the most important pattern in this package):**
- `getSettings`/`updateSettings` are free functions over the `settingsQuerier`/`settingsExecer` interfaces (`internal/store/settings.go:294-300`), satisfied by both `*sql.DB` and `*sql.Tx`, so the ~90-column scan and the full-row UPDATE each exist exactly once.
- `UpdateSettings` (`internal/store/settings.go:430`) is a full-row REPLACE. A Get→edit→Update pairing is a lost-update bug, not a patch.
- `MutateSettings(fn)` (`internal/store/settings.go:454`) is the ONLY sanctioned partial write: read + mutate + write in one transaction under `settingsMu`, with no-op detection (byte-identical rows are never rewritten). Production code must call this; a source-scan test enforces it (see Testing).

**Run lifecycle** (`internal/store/runs.go`): `StartRun` inserts a `'running'` row → `FinishRun` closes it and is the ONLY writer of `runs.completed = 1` ("reached its own conclusion", structural — `internal/store/runs.go:49-79`) → `ReapInterruptedRuns` (startup, global) and `FailRunningRun` (panic path, per-target) stamp `finished_at` WITHOUT `completed`, so due-gates can distinguish "the pass ran" from "the row was closed for a dead process". `group_id` ties child runs of a "Backup Everything" pass to the parent run.

## Structure

```
internal/store/
├── store.go                 # Open(path): driver, MaxOpenConns(1), WAL, FKs
├── repo.go                  # Repo struct, New(), newID()
├── migrate.go               # migrations slice (v1..v99) + Migrate()
├── settings.go              # Settings struct, Get/Update/MutateSettings
├── targets.go               # Target CRUD + hooks/excludes/order/cadence setters
├── vms.go                   # VM CRUD + ordering
├── runs.go                  # StartRun/FinishRun/FailRunningRun/Reap/grouping
├── filesets.go              # Files-domain folder sets
├── offsite.go / offsite_targets.go   # off-site run history + targets
├── repostats.go             # repo_stats cache
├── drills.go                # restore_drills
├── established.go           # established_repos
├── watchdog.go              # watchdog_state
├── received_repos.go / received_alert_state.go   # receiver state
├── fleet_peers.go / mesh_offers.go               # fleet/mesh state
├── schedule_job_runs.go     # per-job last-fire records
├── vacuum.go                # VacuumInto snapshot
├── helpers_test.go          # OpenMem(t) test helper
└── *_test.go                # 25 test files (see Testing)

internal/releasenotes/
├── releasenotes.go          # embed FS, Tag(), Notes()
├── releasenotes_test.go     # incl. TestNotesInSyncWithReleaseNotes
├── catalog_version_test.go  # TestTrueNASCatalogTracksLatestRelease
└── notes/v*.md              # 64 embedded notes, v1.0.0..v8.6.4
```

**Key files for common changes:**
- **Add a new setting:** (1) add the column via a NEW migration at the end of `migrations` in `internal/store/migrate.go` (`ALTER TABLE settings ADD COLUMN ...` with a `NOT NULL DEFAULT` that preserves current behavior); (2) add the field to `Settings` (`internal/store/settings.go`); (3) add it to the SELECT list and `Scan` in `getSettings`; (4) add it to the UPDATE in `updateSettings`. All four touchpoints are positional — do them together.
- **Add a new table:** append a migration with `CREATE TABLE IF NOT EXISTS <name> (...)`, then a new accessor file `internal/store/<name>.go` following `fleet_peers.go`/`received_repos.go` as the template.
- **Add a release note:** write the SAME content to `.github/release-notes/vX.Y.Z.md` and `internal/releasenotes/notes/vX.Y.Z.md` (repo convention: `cp .github/release-notes/*.md internal/releasenotes/notes/`). The embed-sync test fails on a missing or diverging copy.

## Conventions

**Naming.** Go-idiomatic throughout: exported methods read as verbs over domain nouns (`UpsertTarget`, `SetBackupOrder`, `LastEverythingPass`, `RecordScheduleJobRun`). SQL columns are `snake_case`, Go fields `PascalCase`, conversion at the scan boundary only. Files map 1:1 to the domain/table they access.

**Error handling.** Wrap, never mask: every return is `fmt.Errorf("Context: %w", err)` with the method name as context (e.g. `internal/store/settings.go:387`, `internal/store/vacuum.go:12`). Domain-specific sentinels exist where callers branch: `paths.ErrTraversal` (`internal/paths`), `sql.ErrNoRows` remapped to `"settings row missing: run Migrate first"` (`internal/store/settings.go:383`). Best-effort rollbacks are `tx.Rollback() //nolint:errcheck,gosec` with the original error taking priority (`internal/store/migrate.go:1346`). Zero-rows on an update is an explicit error where it matters (`FinishRun` checks `RowsAffected`, `internal/store/runs.go:75`).

**Transactions.** Raw `db.Begin()` / deferred `tx.Rollback()` / `tx.Commit()`; no wrapper abstraction. Shared SQL over `*sql.DB` and `*sql.Tx` goes through tiny local interfaces (`settingsQuerier`/`settingsExecer`) rather than duplicating query text — the stated reason is that a second, transaction-only copy of the 90-column pair is exactly the duplicate that drifts (`internal/store/settings.go:288-300`).

**Serialization discipline.** All writes to the single settings row go through `MutateSettings` (mutex + transaction). `UpdateSettings` is test-only in practice. Mutation fns must be pure — no DB access inside them (deadlock on the one pooled connection).

**Migration etiquette** — enforced by review culture and comments at `internal/store/migrate.go:50-67`: take the NEXT unused number, append at the end, NEVER edit a body that has been on main (":latest is published on every push to main, so a migration is in users' databases the moment it lands on main — tag or no tag"). New migrations run unconditionally; `alreadySatisfied` is reserved for the historical 89/90/91 collision only.

**Comments.** Unusually rich "why" comments explaining the failure each decision prevents (see `runs.completed` rationale at `internal/store/runs.go:49-57`, the numbering hazard at `internal/store/migrate.go:50-67`). Match this register when touching this package.

## Testing

**Run:** `go test ./...` from the repo root (needs restic >= 0.17 on PATH; these packages themselves don't invoke restic).

**In-memory DB pattern.** `OpenMem(t)` in `internal/store/helpers_test.go:10` opens `Open(":memory:")`, runs via `t.Cleanup`, and is the standard entry point: `db := store.OpenMem(t)` → `store.Migrate(db)` → `store.New(db)`. Some tests use `t.TempDir()` file DBs instead when file semantics matter (e.g. `internal/api/config_snapshot_internal_test.go`). The same three-line pattern is used across `internal/api` tests, so store schema changes are exercised there too.

**Test package split.** Black-box tests live in `store_test` and use the public API; migration-upgrade and runreason tests are internal (`package store`) because they need the unexported `migrations` slice (`internal/store/migrate_upgrade_internal_test.go`, `internal/store/migrate_recovery_internal_test.go`, `internal/store/runreason_internal_test.go`).

**Migration upgrade tests (convergence bar).** `internal/store/migrate_upgrade_internal_test.go` reconstructs every reachable historical database state FOR REAL — running actual migration bodies under the numbers each historical build used (helpers `bodyOf`, `applyAs`, `applyThrough` at lines 42-100) — and drives `Migrate` over each, asserting every state converges to the fresh-install schema and survives a second `Migrate` unchanged. When touching migrations, extend this file with the new reachable state.

**Convention-guard tests (source scans):**
- `internal/store/settings_writers_test.go:50` (`TestNoProductionCallerUsesUpdateSettings`) walks `cmd/` + `internal/` non-test sources and fails on any `.UpdateSettings(` call — the pairing is a shape, not a type error, so the guard is deliberately a source scan.
- `internal/store/duegate_scope_test.go` and friends pin due-gate scoping semantics against seeded run rows.

**The embed-sync test.** `TestNotesInSyncWithReleaseNotes` (`internal/releasenotes/releasenotes_test.go:47`) reads every `.github/release-notes/*.md` and asserts an identical (trimmed) copy is present in the embedded FS; it fails with "release note X is not embedded — copy it into internal/releasenotes/notes/" and skips (rather than fails) only when the source dir is unavailable (e.g. a tarball without `.github`). Related: `TestTrueNASCatalogTracksLatestRelease` (`internal/releasenotes/catalog_version_test.go:32`) pins `truenas-apps/app.yaml` `app_version` and `truenas-apps/ix_values.yaml` `tag` to the numerically-newest release note, so a catalog entry cannot advertise a version older than the published image.

## Concerns

**Migration numbering hazard (documented, scar tissue, not fixed-by-tooling).**
- Files: `internal/store/migrate.go:10-67`, `internal/store/migrate_upgrade_internal_test.go`
- Two branches once claimed the same migration numbers (89/90/91) and BOTH numberings reached users because `:latest` publishes on every push to main. The defense is now structural: `alreadySatisfied` probes (`columnPresent`, `internal/store/migrate.go:37`) plus fresh-numbered idempotent recovery migrations v92/v94. The comments state the rule loudly; nothing but review + the convergence tests prevents a repeat. Any change here must re-read the NUMBERING HAZARD note before renumbering anything.

**Settings row width / positional scan fragility.**
- Files: `internal/store/settings.go` (struct ~90 fields, one SELECT, one UPDATE, one Scan)
- Every new setting touches four positional lists (struct, SELECT, Scan args, UPDATE) that must stay in lockstep; the compiler catches only the struct. A mismatch surfaces at runtime as a scan error. Mitigation that exists: the shared querier/execer interfaces mean there is exactly ONE scan and ONE update to get right; tests in `settings_test.go`, `settings_mutate_test.go`, `settings_writers_test.go` round-trip the row. When adding a setting, add it to the round-trip test too.

**String-matching backfills in migrations.**
- Files: `internal/store/migrate.go` v93/v95 (`UPDATE runs SET completed ... WHERE error LIKE 'internal error (recovered panic):%'`)
- The v93 backfill and its v95 correction infer history by matching error-message text — the only option for rows that predate the structural column, and explicitly acknowledged in comments as a last resort. If the marker strings in `internal/api` or `internal/store/runs.go` ever change wording, these migrations' semantics are frozen at what shipped. Do not reword those marker strings casually.

**Accepted race: FailRunningRun vs DownloadFlashZip.**
- Files: `internal/store/runs.go:95-104` (documented in the doc comment)
- `api.Service.DownloadFlashZip` holds a `'restore'` run open without the batch/domain guards, so a concurrent panic-driven `FailRunningRun` for the flash target could mark the download's run failed. Self-heals (the download's own `FinishRun` overwrites by id); worst case is a transiently wrong Activity Log entry. Known, accepted, documented — do not "fix" by broadening `FailRunningRun` to global scope.

**Single-connection pool deadlock surface.**
- Files: `internal/store/store.go:18`, `internal/store/settings.go:449-453`
- `MaxOpenConns(1)` means any store call made from inside a `MutateSettings` fn (or from inside any open transaction's callback) deadlocks until the HTTP request times out. The constraint is comment-enforced only. New code that fns into settings must stay pure.

**Embed/catalog tests are checkout-relative.**
- Files: `internal/releasenotes/releasenotes_test.go:48-52`, `internal/releasenotes/catalog_version_test.go:33-38`
- Both resolve `../../.github/release-notes` relatively and `t.Skip` when absent — correct for CI and dev checkouts, but a naive vendored/module-only test run silently loses both guards. Acceptable; just don't assume the sync test ran when it "passed" in an odd environment.

**Package summaries — remaining internal/ packages** (api, backup, restic, cmd are mapped by other mappers):

- **`internal/ageseal`** — wraps `filippo.io/age` to encrypt the deliberately-plaintext export artifacts (tar.gz/xml/zip) to age public-key recipients (`internal/ageseal/ageseal.go`). Asymmetric by design: the server holds only recipient public keys; decryption happens off-box with the user's private key. Backs the `ExportEncryptEnabled`/`ExportAgeRecipients` settings.
- **`internal/compose`** — pure, stdlib-only helpers for reading docker-compose identity labels and topologically ordering containers by `depends_on` (`internal/compose/compose.go`). Shared by stack restore and the backup restart phase so there is one topological sort, not two drifting copies; importable by anything without cycles.
- **`internal/config`** — loads and validates process configuration from environment variables (`internal/config/config.go`); also derives `DBPath` from `DataDir` (line 104). Owns `PLATFORM` override and `TRUSTED_PROXY` parsing.
- **`internal/dockercli`** — wraps the official Docker SDK behind an interface (`internal/dockercli/dockercli.go`) so the backup orchestrator's host control is mockable; the concrete client is wired only in `cmd/bombvault` (DI seam with `internal/model`).
- **`internal/model`** — behavior-free container types shared across the DI seam (`internal/model/container.go`); stdlib-only so orchestrator and adapter never import each other.
- **`internal/notify`** — best-effort, time-bounded notifications to webhook (generic JSON/Discord/Slack/Gotify/ntfy), Matrix, SMTP email, Apprise API, and Healthchecks.io (`internal/notify/notify.go`); a notify failure never affects a backup. Config arrives as the encrypted `NotifyConf` settings blob; `config_backfill_test.go` proves upgrades never switch a working channel off (absent enable-gates decode as enabled).
- **`internal/paths`** — in-app path containment under the host mount root (`internal/paths/paths.go`); returns `ErrTraversal` on escape attempts. All user-supplied relative paths should pass through here.
- **`internal/platform`** — the seam for host-specific behavior across Unraid / TrueNAS Scale / generic Docker (`internal/platform/platform.go`, `detect.go`, `unraid.go`, `truenas.go`, `generic.go`); detection respects the `PLATFORM` override.
- **`internal/progress`** — carries live backup/restore/replicate percentages from the restic layer to the SPA's SSE endpoint (`internal/progress/progress.go`): context-carried Sink/CopySink (no signature churn) plus an in-process pub/sub Store keyed per target.
- **`internal/restickey`** — derives the restic repository password from `APP_KEY` via domain-separated HMAC-SHA256 (`internal/restickey/restickey.go`); the domain separation from `internal/secret` is deliberate.
- **`internal/schedule`** — per-domain in-process scheduler on `robfig/cron/v3` (`internal/schedule/schedule.go`), reading cadences from the store settings row; also the catch-up (anacron-style), per-item schedule overrides, everyN due-gating, and overlap guards. Heavy consumer of `runs`/`schedule_job_runs` history.
- **`internal/secret`** — AES-256-GCM encryption keyed from `APP_KEY` (for container definitions and the encrypted settings blobs), Argon2id password hash/verify (upgrading legacy HMAC hashes on login), session-token mint/verify, and a hand-written RFC 6238 TOTP (`internal/secret/secret.go`, `password.go`, `totp.go` — TOTP written out deliberately, ~40 lines, to avoid a dependency).
- **`internal/selfrestore`** — applies a staged restore of BombVault's own `/config` at next boot BEFORE `store.Open` (`internal/selfrestore/selfrestore.go`), the only safe moment to swap the live WAL-mode SQLite file; deliberately dependency-light (stdlib + the sqlite driver for a validity probe) so both `cmd/bombvault` and `internal/api` can import it without cycles.
- **`internal/spike`** — dependency-injected host-integration probes returning human-readable detail strings (`internal/spike/spike.go`); unit-testable with no Docker socket or restic binary.
- **`internal/sshconn`** — manages SSH to the libvirt host for `qemu+ssh://` virsh control and NVRAM file transfer (`internal/sshconn/sshconn.go`); no libvirt socket is ever bind-mounted, virsh runs ON the host over SSH.
- **`internal/template`** — rewrites host paths inside Unraid container-template XML (`<Config HOST_PATH>` elements) during restore/recreate (`internal/template/rewrite.go`).
- **`internal/virshcli`** — virsh CLI wrapper for VM lifecycle, including credential scrubbing of `user:pass@` userinfo in error output (`internal/virshcli/credential_scrub_internal_test.go` pins the fix, mirrored from `internal/restic` and `internal/api`).

---

*Module analysis: 2026-09-09*
