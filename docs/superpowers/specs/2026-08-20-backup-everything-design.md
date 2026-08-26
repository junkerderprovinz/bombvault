# "Backup Everything" sequential job — design

> **For agentic workers:** implement via fresh-subagent-per-task + two-stage review
> (spec compliance, then code quality), matching this repo's established pattern
> (PRs #149/#151/#153/#155/#157). Plan: `docs/superpowers/plans/2026-08-20-backup-everything.md`.

## Origin

An Unraid forum user asked for a way to run a curl (dead-man's-switch ping) once
ALL of: container backups, folder backups, flash backup, VM backups, and the
self-backup are complete — today this is only possible after each individual
domain's own backup (per-container `PreHook`/`PostHook`,
`store.Target.PreHook`/`PostHook`, executed in `internal/backup/orchestrator.go`).

## What exists today (verified against the actual code)

- Five fully independent cron entries (`internal/schedule/schedule.go`'s
  `ReloadWithDueChecks`, the `domains` slice at line ~796): containers, vms,
  flash, config, files — each its own cadence from `store.Settings`, each its
  own off-site sub-schedule. Nothing joins them; there is no "all domains done"
  moment anywhere in the code.
- `StartBackupAll` (`internal/api/service.go:3556`, `POST /api/containers/backup-all`)
  only covers containers, despite the generic name.
- Settings' schedules tab already hosts one `Card` per domain
  (`ContainersSection`/`VMsSection`/`FlashSection`/`FilesSection` in
  `Settings.tsx`) using the shared `CadenceBuilder` component
  (`web/src/components/CadenceBuilder.tsx`) bound to one `store.Settings`
  cadence field each, saved via one page-level `SaveBar`.
- Every domain's backup orchestrator (`internal/backup/{orchestrator,
  vm_orchestrator, flash_orchestrator, files_orchestrator,
  config_orchestrator}.go`) starts its Run row through the SAME `backup.Runs`
  interface (`Start(targetID, kind string) (string, error)`), implemented in
  `internal/api/service.go` by `runsAdapter`/`startedRunsAdapter`. This is the
  ONE real choke point common to all five domains' run-recording — see the
  grouping decision below.
- `store.Run` (`internal/store/runs.go`) already has two reserved singleton
  target ids for domains with no per-item table: `FlashTargetID = "flash"`,
  `ConfigTargetID = "config"`, each with its own `LastSuccessfulXBackup()`
  query feeding the everyN due-gate.
- The Activity Log (`web/src/lib/activityLog.ts`) resolves a run's domain via
  a CLOSED map (`DOMAIN_KEYS`/`LogDomain`/`normalizeDomain`) fed by the
  backend's `runTargetMaps()` (`internal/api/handlers.go:2176`), which is a
  SEPARATE concern from the Dashboard's heatmap/storage domain arrays
  (`web/src/pages/Dashboard.tsx`'s `HeatDomain`/`StorageDomain`) — those stay
  a closed 5-domain set on purpose (a "Backup Everything" pass has no bytes/
  retention/offsite of its own to plot there).
- BombVault's own runtime image (`Dockerfile`, `debian:stable-slim`) has
  neither `curl` nor `wget` in the final runtime stage today (wget is a
  build-only tool, purged before the runtime layer). The container's own
  healthcheck deliberately avoids needing a shell/curl by using a Go
  subcommand instead. This matters because a GLOBAL hook — unlike the
  per-container hook, which execs INSIDE the target container via the Docker
  API — has no single container to exec into and must run as a local shell
  command in BombVault's OWN container.

## Decisions

### 1. A 6th pseudo-domain in the scheduler, not a heuristic

"Backup Everything" gets its OWN cadence field (`Settings.EverythingSchedule`,
`CadenceBuilder`-driven, empty/"off" by default) and its own cron entry in
`ReloadWithDueChecks`'s `domains` slice, mirroring exactly how
containers/vms/flash/config/files are each registered. It does not replace or
gate the five existing schedules — a user can run both; the Settings UI
carries an explicit warning about the overlap/double-backup risk next to the
new field (a static warning box, not a live conflict detector — over-engineering
a "smart" overlap check was rejected as unnecessary for a v1).

### 2. Grouping the five domain-runs under one parent: `runs.group_id`, stamped through the existing `backup.Runs` seam

Two options were on the table: (a) a new run "kind", or (b) a way to group the
five domain-runs under one parent. Chosen: **(b)**, via a new nullable
`runs.group_id TEXT NOT NULL DEFAULT ''` column (migration), populated through
the SAME `backup.Runs.Start` choke point every domain orchestrator already
calls (see above) — NOT by touching each of the ~5 raw `StartRun` call sites
in `service.go` individually, and NOT by inventing a parallel `RunSnapshots`-
style join table.

Mechanism (mirrors the EXISTING `WithBulkReplicateSuppressed`/
`bulkReplicateSuppressed` context-flag idiom in `service.go`, used for exactly
this kind of cross-cutting "this call is part of a bigger batch" signal):

- `runGroupKey{}` / `WithRunGroup(ctx, groupID) context.Context` /
  `runGroupFromContext(ctx) string` — new, unexported except the `With...`
  setter (only `internal/api` needs it; nothing outside the package sets or
  reads it).
- `runsAdapter` (and the VM-specific `startedRunsAdapter`, which pre-obtains
  its run id before the orchestrator's own `Runs.Start` call — see its doc
  comment) gain a `ctx context.Context` field, captured at construction. Their
  `Start`/pre-obtain call now additionally does: `if gid :=
  runGroupFromContext(ctx); gid != "" { _ = s.store.SetRunGroup(id, gid) }` —
  best-effort, never fails the backup over bookkeeping. When `ctx` carries no
  group (every existing caller today), this is a pure no-op — byte-identical
  behavior to before this change.
- The new `BackupEverything` orchestration (`internal/api/everything.go`)
  starts a PARENT run (`target_id = store.EverythingTargetID`, `kind =
  "backup"`, mirroring the flash/config singleton pattern exactly) and wraps
  the context it hands to each domain's own `s.Backup`/`s.BackupVM`/
  `s.BackupFlash`/`s.BackupFileSet`/`s.BackupConfig` call with
  `WithRunGroup(ctx, parentRunID)` — so every CHILD run this pass produces
  carries `group_id = parentRunID`, durably queryable, while remaining a
  fully independent, normal Run row in every other respect (its own kind,
  status, bytes, snapshot id — nothing about how an individual domain's run
  is recorded changes).

**Why not (a) a new "kind":** every existing kind ("backup"/"restore"/
"prune"/"verify"/"offsite"/"update"/"drill"/"drdrill"/"tamper"/"export") is a
VERB describing what the row itself IS; "Backup Everything" is not a new verb,
it's a `kind="backup"` row like any other, just against a different
(singleton, pseudo-domain) target. Reusing `kind="backup"` means it flows
through every existing kind-based code path (dashboard counts, Activity Log's
default backup-line formatter) with ZERO new formatting logic needed — the
existing `lineBackupSuccess`/`lineBackupFailed`/`lineBackupSkipped` keys
already read `{name}`/`{bytes}`/`{duration}`/`{error}`, so the parent row
renders correctly out of the box (`Target: "Backup Everything"`, from a
`runTargetMaps()` entry mirroring `FlashTargetID: "Unraid flash"`) — see
Decision 3 for why `Domain` also needs a matching addition.

**What `group_id` is (and isn't) used for in this PR:** it makes the pass
durably traceable/queryable (any child run can be traced back to the parent
pass that triggered it) and is the honest, schema-correct answer to "share
one run/session identifier". Visually NESTING the five children under the
parent row in the Activity Log UI is NOT implemented in this PR — the log
still shows the parent as one line and every child domain's own run as its
own line, exactly as today. That's flagged as a deliberate, scoped-out
follow-up in the PR description, not a silent gap.

### 3. The parent run's own record must show WHICH domains failed

Per-domain outcome is NOT re-derived by joining on `group_id` for the parent
row's OWN status text (that would require a UI change to the Activity Log
line-formatter, out of scope here). Instead, `BackupEverything` builds a
short structured breakdown string (e.g. `containers: 4/5 ok (radarr: exec:
container not running); vms: ok; flash: ok; files: 2/2 ok; config: ok`) and
passes it as `FinishRun`'s `errMsg` — but ONLY when at least one domain had a
failure (status `"failed"`); on a clean pass every domain's line reads "ok" and
the row's `Error` stays empty, matching every other backup run's convention
(no error text on success). The pre-existing per-domain child Run rows are
completely unaffected and still show full success/failure detail for anyone
who drills into that domain's own history — this satisfies "a failure isn't
silently swallowed" without inventing new failure-reporting plumbing.

Overall Status is `"success"` iff every domain step had zero item failures
(a domain with zero eligible items — e.g. Files disabled/empty — counts as
success, not a failure); otherwise `"failed"`. `SnapshotID` stays empty
(no single snapshot represents a 5-domain pass) and `Bytes` stays 0 (each
child run already carries its own bytes; summing across domains that don't
share a unit of comparison was judged not worth the loop restructuring it
would need — see the PR's "known limitations").

### 4. Activity Log: reuse `runTargetMaps`/`DOMAIN_KEYS`, do NOT touch the Dashboard's closed domain arrays

`internal/api/handlers.go`'s `runTargetMaps()` gets one more entry:
`name[store.EverythingTargetID] = "Backup Everything"`,
`domain[store.EverythingTargetID] = "everything"`. `web/src/lib/
activityLog.ts` gets a matching `"everything"` entry in `LogDomain`,
`DOMAIN_KEYS` (→ a new i18n key), `normalizeDomain` (pass-through) and
`LogFilterDomain` (so it's filterable). Dashboard.tsx's `HeatDomain`/
`StorageDomain` closed unions and the 5-domain heatmap/storage grid are
DELIBERATELY untouched — "Backup Everything" has no retention/off-site/bytes
identity of its own to plot there; it is a sequencing pass over domains that
already have their own cards.

### 5. The pass itself: reuse each domain's REAL backup entry point, not the scheduler's private closures

`internal/schedule/schedule.go`'s per-domain `fn func()` closures (registered
inside `ReloadWithDueChecks`) are private to that function and explicitly OFF
LIMITS to modify (they encode the five existing independent schedules' own
behavior). `BackupEverything` therefore does NOT reach into `schedule.go` —
it lives in a new `internal/api/everything.go` (the same shape as the other
cross-domain orchestration files already in that package: `digest.go`,
`watchdog.go`, `receiver_watch.go`, each wired via `scheduler.SetXJob` in
`cmd/bombvault/main.go`) and independently calls:

- containers: `schedule.DomainRunTargets(s.store.ListTargetsScheduleOrder(), settings.PerItemSchedules)` then loops calling `s.Backup(runCtx, name)` per included target — same shape as `schedule.RunContainersJob`, reimplemented inline here (not by importing the scheduler's private closure) so bytes/failures can be collected uniformly; the loop's SKIP/CONTINUE-ON-ERROR semantics are IDENTICAL to `RunContainersJob`.
- vms: `store.SortVMTargetsForRun` + `schedule.DomainRunVMTargets`, then `s.BackupVM(runCtx, name)` per included VM.
- files: `s.store.ListFileSets()`, then `s.BackupFileSet(runCtx, id)` per enabled set.
- flash: `s.BackupFlash(ctx)` (no bulk suppression — a singleton, exactly like `SetFlashJob`'s closure in `main.go`).
- config: `s.BackupConfig(ctx)` (same).

`runCtx` for the three multi-item domains is wrapped EXACTLY like `main.go`'s
existing scheduled closures: `WithRunGroup(WithBulkReplicateSuppressed(
notify.WithMessagesSuppressed(notify.WithHealthchecksSuppressed(ctx))),
parentRunID)` — so each item's inline off-site replication/Healthchecks ping/
message notification is suppressed exactly as a real scheduled domain run
suppresses them, and `s.PruneAfterBulk`/`s.ReplicateOffsiteAfterBulk` (the
SAME exported methods `main.go`'s `SetOffsiteAfterBulkJob`/
`SetPruneAfterBulkJob` closures call) run once per domain after that domain's
loop — byte-for-byte the same aggregation behavior a real scheduled run gets.
`s.ScheduledHealthchecksStart`/`ScheduledHealthchecksResult`/
`ScheduledNotifyResult` (same exported methods `SetHealthchecksAggregator`'s
closures call) bracket each multi-item domain's loop too. **Every one of
these is an already-exported, already-used-by-the-scheduler method — nothing
about how an individual domain aggregates/notifies/replicates changes.**

A domain step that fails entirely BEFORE producing any per-item outcome (e.g.
`ListTargetsScheduleOrder` errors) is caught and recorded as that domain's own
failure in the breakdown — it must never propagate up and abort the remaining
domains (explicit requirement: "survive one domain failing").

### 6. Global hooks: a new local-shell-exec seam, `curl` added to the runtime image

The global pre/post-hook fields are plain `sh -c <command>` strings — SAME
mechanism as the existing per-container `PreHook`/`PostHook`
(`store.Target.PreHook`/`PostHook`) — but they run in BombVault's OWN
container (there is no single target container for a whole-pass hook), via a
new small DI seam (mirrors `backup.Docker`/`backup.ZFSHost`'s existing
adapter-interface pattern):

```go
type HostShell interface {
    Run(ctx context.Context, cmd string) error
}
```

A real adapter shells `exec.CommandContext(ctx, "sh", "-c", cmd)` with a fixed
bounded timeout (5 minutes — generous for a healthcheck ping, short enough
that a hung command can't wedge the pass indefinitely); `Service` gets a
`hostShell HostShell` field defaulted to the real adapter in `NewService`
(no new wiring needed in `main.go`) plus a setter for test injection, mirroring
`SetHostSSH`/`SetProgress`. The pre-hook is BEST-EFFORT (logged on failure,
never aborts the pass — unlike the per-container pre-hook, a global hook has
no snapshot-consistency contract to protect). The post-hook fires EXACTLY
ONCE, unconditionally, after every domain step has been attempted (success or
failure) — the explicit dead-man's-switch requirement.

`curl` is added to the Dockerfile's runtime `apt-get install` line (small,
~2-3 MB) specifically so the stated use case (a curl healthcheck ping) works
out of the box; `wget` stays removed (build-stage-only tool, unrelated).

### 7. Manual trigger: `POST /api/backup-everything`, background-goroutine shape (mirrors `StartBackupAll`)

`StartBackupEverything(ctx) (bool, error)` guards re-entrancy with an
`atomic.Bool` (`s.everythingActive`, mirroring `s.batchActive` exactly),
detaches from the request via `context.WithoutCancel` (each domain step
already applies its own hold/hard-cap via `backupHoldCtx` inside `s.Backup`/
`s.BackupVM`/etc., so the top-level call needs no deadline of its own — same
reasoning `StartBackupAll`'s own doc comment gives), and runs
`BackupEverything` in a background goroutine, returning `{started: true}`
immediately. The SCHEDULED entry point (`scheduler.SetEverythingJob`) calls
`BackupEverything` directly (synchronous on the cron goroutine, exactly like
`SetFlashJob`/`SetConfigJob`'s closures) — `cron`'s own
`SkipIfStillRunning` chain already prevents a scheduled fire from overlapping
itself; `everythingActive` additionally prevents a manual click from
overlapping either the scheduled run or a second manual click. No new
per-domain busy pre-flight check is added — each domain step's own existing
lock (`s.lockDomain`) already governs contention with any OTHER concurrent
operation on that domain exactly as it does for every other caller today; a
domain that is busy simply surfaces as that domain's own failure in the
pass's breakdown, which is consistent with "survive one domain failing".

## Settings / API surface

- `store.Settings`: `EverythingSchedule`, `EverythingPreHook`,
  `EverythingPostHook string` (mirrors `ConfigSchedule`/`PreHook`/`PostHook`
  naming exactly).
- Migration (settings): `everything_schedule TEXT NOT NULL DEFAULT 'off'`,
  `everything_pre_hook TEXT NOT NULL DEFAULT ''`, `everything_post_hook TEXT
  NOT NULL DEFAULT ''`.
- Migration (runs): `group_id TEXT NOT NULL DEFAULT ''` + an index.
- `settingsView`/`toView`/`handlePutSettings`: three new fields, round-tripped
  like every other plain (non-secret) string field; `EverythingSchedule`
  joins the existing cadence-validation loop (`schedule.ParseCadence`) — it
  MAY use `everyN` (a due-gate is wired via `LastSuccessfulEverythingBackup`,
  matching the five existing domains, not the off-site/drills/tamper/digest
  schedules which reject `everyN`).
- New route: `POST /api/backup-everything` → `handleBackupEverything`,
  response shape mirrors `handleBackupAll` (`{ok, started}` / 409-style
  `{ok:false}` when already running).
- `runTargetMaps()`: `+2` entries (name + domain) for `EverythingTargetID`.

## Frontend surface

- `web/src/lib/api.ts`: `Settings` interface `+3` fields;
  `backupEverythingNow(): Promise<OkEnvelope & {started?: boolean}>` →
  `POST /api/backup-everything`.
- `web/src/pages/Settings.tsx`: new `EverythingSection` (or inline block) in
  the **schedules** tab (NOT the notifications tab — that hosts the unrelated
  `NotifyCard`), using `Card` + `CadenceBuilder` (bound to
  `settings.everythingSchedule`) + two hook `<input>` fields matching
  `Containers.tsx`'s `HooksEditor` field style (monospace, placeholder
  examples) + a `bg-statusWarnBg`/`text-statusWarn` warning box (mirrors the
  existing tamper-schedule-inactive warning) describing the double-backup
  overlap risk + a manual "Run Backup Everything now" button calling
  `backupEverythingNow()`. Wired into `buildSchedulePatch()` like every other
  schedule field.
- `web/src/lib/activityLog.ts`: `"everything"` added to `LogDomain`,
  `DOMAIN_KEYS`, `normalizeDomain`, `LogFilterDomain`.
- i18n: new keys for the card's title/hint/hook labels/warning/button/toast
  text, plus `activityLog.domainEverything` — added to `i18n.ts` (en+de
  inline) AND all 24 `web/src/lib/locales/*.ts` files with real translations
  (house rule, not machine output).

## Explicitly out of scope (touch nothing here)

- The five existing domains' own scheduled-job closures/logic in
  `schedule.go` — read-only reference, never modified.
- Any new retention/pruning model — every domain step's retention behaves
  exactly as it does when triggered individually (no new code path).
- Dashboard heatmap/storage 5-domain arrays.
- Auto-disabling either schedule when both are configured — the UI only
  warns; the user's choice/responsibility per the spec.
- Nesting child runs visually under the parent in the Activity Log UI (the
  data — `group_id` — exists; the UI still lists it as one line + five
  separately-rendered child lines, exactly as today).
