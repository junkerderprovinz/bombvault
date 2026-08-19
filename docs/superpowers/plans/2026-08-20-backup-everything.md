# "Backup Everything" sequential job — implementation plan

> **For agentic workers:** fresh subagent per task, then a spec-compliance
> review, then a code-quality review; fix and re-review until both pass; commit
> after each task. A final whole-branch review runs after Task 7. Design spec:
> `docs/superpowers/specs/2026-08-20-backup-everything-design.md` — read it
> first, it has the full rationale for every decision below; this plan is the
> task breakdown only.

Branch: `feat/backup-everything-job`.

## Task 1: Store & schema layer

**Files:**
- Modify: `internal/store/migrate.go` — two new migrations (next available
  version numbers after 88): (a) `settings` gains `everything_schedule TEXT
  NOT NULL DEFAULT 'off'`, `everything_pre_hook TEXT NOT NULL DEFAULT ''`,
  `everything_post_hook TEXT NOT NULL DEFAULT ''`; (b) `runs` gains `group_id
  TEXT NOT NULL DEFAULT ''` + `CREATE INDEX IF NOT EXISTS idx_runs_group ON
  runs(group_id)`.
- Modify: `internal/store/settings.go` — `Settings` struct gains
  `EverythingSchedule`, `EverythingPreHook`, `EverythingPostHook string`;
  wire into `GetSettings`' SELECT/Scan and `UpdateSettings`' UPDATE, following
  the exact pattern `ConfigSchedule`/`ConfigPath` already use (plain string
  columns, no bool flag needed).
- Modify: `internal/store/runs.go` — `Run` struct gains `GroupID string
  json:"groupId"`; `EverythingTargetID = "everything"` constant (alongside
  `FlashTargetID`/`ConfigTargetID`, same doc-comment style); `scanRun` +
  every `SELECT ... FROM runs` (StartRun's own row is fine as-is; `ListRuns`,
  `RunsSince`) gain `group_id`; new `SetRunGroup(runID, groupID string)
  error` (a plain `UPDATE runs SET group_id = ? WHERE id = ?`, following
  `AcknowledgeRuns`' error-handling shape — RowsAffected==0 is NOT an error
  here, just log-worthy at most, since the caller treats it best-effort); new
  `LastSuccessfulEverythingBackup() (time.Time, error)`, copy-pasted from
  `LastSuccessfulFlashBackup`/`LastSuccessfulConfigBackup` verbatim except the
  target id.
- Test: `internal/store/settings_test.go` — round-trip test for the 3 new
  fields (save + reload). `internal/store/runs_test.go` — `SetRunGroup` sets
  the column and a plain `ListRuns`/scan round-trips `GroupID` correctly
  (including the zero-value `""` case, i.e. an ungrouped run is unaffected);
  `LastSuccessfulEverythingBackup` mirrors the existing flash/config tests'
  shape (no rows → zero time; a success sets it; a failure doesn't).
  `internal/store/migrate_test.go` likely already has a
  generic "migrations apply cleanly / are idempotent" test — confirm the new
  migrations pass it; add nothing new there unless a targeted schema-shape
  assertion is the established pattern (check the file first).

Do NOT touch `runsAdapter`/context plumbing here — that's Task 3. This task is
pure schema + store CRUD, independently testable and low-risk.

## Task 2: Host-shell-exec seam + Dockerfile curl

**Files:**
- New: `internal/api/hostshell.go` (or add to an existing small adapter file
  if one obviously fits better — check `internal/api/service.go`'s existing
  adapter-struct neighborhood, e.g. near `resticAdapter`/`sshZFSHost`, before
  deciding) — a `HostShell` interface (`Run(ctx context.Context, cmd string)
  error`), a real `execHostShell` implementation using
  `exec.CommandContext(ctx, "sh", "-c", cmd)` under a fixed 5-minute
  `context.WithTimeout`, combined stdout+stderr captured and logged on
  failure (never returned to the caller as backup-blocking — hooks are
  best-effort by design per the spec), with the same `//nolint:gosec // G204`
  justification style already used at `internal/restic/restic.go:823` (the
  command string is OPERATOR-CONFIGURED via Settings, not user/request input
  — same trust model as the existing per-container hook's `Docker.Exec(ctx,
  ref, []string{"sh","-c",d.PreHook})`).
- Modify: `internal/api/service.go` (or wherever `Service`'s field list and
  `NewService`/setter methods live) — add a `hostShell HostShell` field,
  defaulted to the real adapter inside `NewService`, plus a `SetHostShell`
  setter mirroring `SetHostSSH`/`SetProgress` for test injection.
- Modify: `Dockerfile` — add `curl` to the runtime stage's `apt-get install
  -y --no-install-recommends` line (do NOT add it to the `apt-get purge`
  line — `wget`/`bzip2`/`unzip` stay purged as build-only tools; `curl`
  is a permanent runtime dependency). Update the `HEALTHCHECK` comment block
  ONLY if it becomes misleading (it currently says "the image needs no shell
  or curl" in the context of the Go healthcheck subcommand specifically —
  reword narrowly if needed so it doesn't read as "this image has no curl at
  all", without rewriting the whole comment).
- Test: `internal/api` — a small test using a FAKE `HostShell` (records the
  command it was asked to run, can be told to fail) proving `SetHostShell`
  overrides the default. Do not write a test that actually shells out to
  `/bin/sh` (the CI runner may not be POSIX in every job, and it is
  unnecessary — the seam's own unit test is about the interface/wiring, not
  about `os/exec` itself).

This task has NO dependency on Tasks 1/3 and could be done in parallel with
either, but keep it sequential (same branch, one writer at a time) — do it
right after Task 1.

## Task 3: Run-group context plumbing

**Files:**
- Modify: `internal/api/service.go` — add `runGroupKey{}` / `WithRunGroup(ctx
  context.Context, groupID string) context.Context` / `runGroupFromContext(ctx
  context.Context) string`, placed right next to (and doc-comment-styled
  like) `bulkReplicateSuppressKey`/`WithBulkReplicateSuppressed`/
  `bulkReplicateSuppressed` (~line 2374-2398) — same idiom, same file
  neighborhood.
- Modify: `runsAdapter` (currently `type runsAdapter struct{ st *store.Repo
  }`, positional-constructed as `runsAdapter{s.store}` at its ~6 call sites)
  — add a `ctx context.Context` field; update `Start` to call
  `r.st.SetRunGroup(id, gid)` best-effort when `runGroupFromContext(r.ctx)` is
  non-empty; update EVERY existing construction site (`runsAdapter{s.store}`
  → `runsAdapter{st: s.store, ctx: ctx}`, using named fields once there are
  two) to pass the ALREADY-IN-SCOPE `ctx` — read each call site fresh (line
  numbers will have shifted from investigation) to confirm `ctx` really is
  the right variable in scope at each (it should be — every one of these
  functions takes `ctx context.Context`).
- Modify: `startedRunsAdapter` (VM-specific — pre-obtains its run id via a
  raw `s.store.StartRun(tg.ID, "backup")` call inside `BackupVM`, BEFORE
  constructing the adapter) — right after that raw `StartRun` call, add the
  same best-effort `SetRunGroup` stamp using `runGroupFromContext(ctx)` (no
  struct field needed here since the stamp happens inline, not inside a
  later `Start()` call).
- **Critical regression guard:** when `ctx` carries no group value (every
  single existing caller today, and every restore/other-kind `runsAdapter`
  construction site that isn't part of a Backup Everything pass),
  `runGroupFromContext` returns `""` and NOTHING about run creation changes —
  no extra DB write even attempted. Prove this explicitly in the test below,
  not just by inspection.
- Test: `internal/api` — a table/unit test that constructs a `runsAdapter`
  (or drives it through the real `Start` call) with (a) a plain
  `context.Background()` → confirms the created run's `GroupID` is `""`
  (byte-identical to pre-change behavior), and (b) a `WithRunGroup(ctx,
  "abc")`-wrapped context → confirms the created run's `GroupID` is `"abc"`.
  Cover both `runsAdapter` and the VM inline-stamp path (may need a fake/test
  double around `BackupVM`'s dependencies — check how existing `BackupVM`
  tests are already set up and reuse that harness rather than building a new
  one).

Depends on Task 1 (`SetRunGroup` must exist).

## Task 4: Core orchestration — `internal/api/everything.go`

**Files:**
- New: `internal/api/everything.go` — `EverythingSummary` (or reuse an
  existing shape if one fits — check first) representing the pass's
  outcome for the caller; `func (s *Service) BackupEverything(ctx
  context.Context) (EverythingSummary, error)` implementing the full
  sequence from the design spec's Decision 5 (containers → vms → flash →
  files → config, group-stamped + suppressed context for the three
  multi-item domains, direct calls for the two singletons, global pre-hook
  best-effort before the sequence, global post-hook unconditional exactly
  once after, structured per-domain breakdown into `FinishRun`'s `errMsg`
  only on failure); `func (s *Service) StartBackupEverything(ctx
  context.Context) (bool, error)` per Decision 7 (the `s.everythingActive
  atomic.Bool` guard + background goroutine + `context.WithoutCancel`,
  mirroring `StartBackupAll`'s exact shape — read `StartBackupAll`
  (`service.go:3556`) fresh and copy its structure, not just its vibe).
- Modify: `internal/api/handlers.go` — `runTargetMaps()` gains
  `name[store.EverythingTargetID] = "Backup Everything"`,
  `domain[store.EverythingTargetID] = "everything"`.
- Test: `internal/api` — using fakes for `s.Backup`/`s.BackupVM`/etc. is not
  straightforward (they're concrete methods, not interface-injected) — this
  is genuinely the hardest task to unit test. Investigate how EXISTING tests
  drive a full `Backup()`/`BackupVM()` call today (check `service_test.go`,
  `startbackupall_offsite_test.go`, `restore_ux_internal_test.go` for the
  fake Docker/virsh/restic harness they build) and reuse that SAME harness
  to build a minimal multi-domain fixture. Required coverage, in priority
  order:
  1. **Order**: containers run before vms before flash before files before
     config (assert via a shared ordered log the fakes append to, or via
     timestamps/call-order recording on the fakes).
  2. **Hook fires exactly once**: a fake `HostShell` (Task 2) records call
     count; assert it is invoked exactly once for the post-hook regardless of
     whether every domain succeeded, and that the pre-hook (if configured) is
     invoked at most once before any domain step starts.
  3. **Survives one domain failing**: make one domain's fake dependency error
     (e.g. a fake Docker client that fails one container) and assert the
     REMAINING domains still ran (their own fakes recorded a call) and the
     post-hook still fired.
  4. **Parent run status**: all-domains-clean → parent run `status="success"`,
     `error=""`; one domain partially failing → parent run `status="failed"`
     with a non-empty `error` naming the failing domain/item.
  5. **Group stamping integration**: at least one test confirms a child run
     produced during the pass has `GroupID == ` the parent run's id (ties
     Task 3's mechanism to the real call path, not just its own isolated
     unit test).
  6. **`StartBackupEverything` re-entrancy**: a second call while one is
     already in flight returns `(false, nil)` (or however `StartBackupAll`'s
     equivalent guard signals busy — match its exact contract).

  If the existing fake-adapter harness makes points 1-4 impractical to wire
  for ALL FIVE domains in one test file (e.g. VM/libvirt fakes are heavy),
  it is acceptable to test the SEQUENCING/HOOK/FAILURE-SURVIVAL logic (1-3)
  against a smaller subset (e.g. just containers + flash + config, which are
  the cheapest to fake) as long as the real production code path is
  unconditional over all five — do not special-case the implementation
  itself to make testing easier. Document in the test file's comment exactly
  which domains are exercised and why the others are not, so this is a
  visible, deliberate scope note rather than a silent gap.

Depends on Tasks 1, 2, 3.

## Task 5: Scheduler + HTTP wiring

**Files:**
- Modify: `internal/schedule/schedule.go` — `ReloadWithDueChecks`'s
  signature gains a 6th `everythingLastRun LastRunFunc` parameter (update
  `Reload`'s pass-through call too); a 6th `domainSpec` appended to the
  `domains` slice (`cadence: settings.EverythingSchedule, name:
  "everything"`) — its `fn` calls a new `s.everythingFn func() error` field
  (nil-guarded exactly like `s.configJob`/`s.backupFlash`, logged when unset)
  via a new `SetEverythingJob(fn func() error)` setter, mirroring
  `SetConfigJob` exactly (singleton-shaped: no bulk/list function needed,
  `BackupEverything` already loops internally). Confirm
  `jobDomainFromName`/`NextRuns` need NO special-casing (the default `return
  "backup", name` branch already produces `job="backup", domain="everything"`
  correctly — verify, don't assume).
- Modify: `cmd/bombvault/main.go` — `scheduler.SetEverythingJob(func() error
  { return svc.BackupEverything(context.Background()) })` (return only the
  error, discard the summary — mirrors every other `SetXJob` closure's
  shape); add `everythingLastRun :=
  schedule.LastRunFunc(st.LastSuccessfulEverythingBackup)`; thread it as the
  6th arg into the existing `scheduler.ReloadWithDueChecks(...)` call.
- Modify: `internal/api/api.go` (`Handler` struct + `NewHandler`) — add
  `everythingLastRun schedule.LastRunFunc` field, initialized from
  `schedule.LastRunFunc(st.LastSuccessfulEverythingBackup)` inside
  `NewHandler` exactly like the other 5 `...LastRun` fields (no constructor
  signature change needed — `NewHandler` already receives `st`).
- Modify: `internal/api/handlers.go` — `handlePutSettings`'s
  `h.scheduler.ReloadWithDueChecks(...)` call gains the 6th
  `h.everythingLastRun` arg; `settingsView` struct gains `EverythingSchedule
  string \`json:"everythingSchedule"\``, `EverythingPreHook string
  \`json:"everythingPreHook"\``, `EverythingPostHook string
  \`json:"everythingPostHook"\``; `toView`/the `store.Settings{...}`
  construction in `handlePutSettings` both gain the 3 fields (plain
  pass-through, no secret-blanking needed — these are not credentials); the
  cadence-validation loop gains `v.EverythingSchedule` — but in the loop that
  allows `everyN` (the domain-schedule loop, NOT the offsite/drills/tamper/
  digest one that rejects it) per the design spec's explicit call-out. New
  `handleBackupEverything` handler (`POST /api/backup-everything`, no path
  params) mirroring `handleBackupAll`'s response shape (`{ok, started}` on
  success, a clear busy message when `StartBackupEverything` returns
  `(false, nil)`).
- Modify: `internal/api/api.go` — register `mux.HandleFunc("POST
  /api/backup-everything", h.handleBackupEverything)` near the other
  domain-level manual-trigger routes (e.g. next to `/api/flash/backup`/
  `/api/config/backup`, or near `/api/containers/backup-all` — pick whichever
  neighborhood reads more naturally once the surrounding routes are in front
  of you).
- Test: `internal/schedule` — a `ReloadWithDueChecks`-level test proving: an
  empty/"off" `EverythingSchedule` registers NO cron entry (feature
  genuinely inert by default — the core "opt-in" requirement); a valid
  cadence registers exactly one, and it survives a `Reload` cycle
  (add/remove) like every other domain. `internal/api` — a handler test for
  `POST /api/backup-everything` (started=true on first call, refused on a
  concurrent second call) and a `handlePutSettings` round-trip test proving
  the 3 new fields save and reload correctly and an invalid
  `EverythingSchedule` is rejected with a clear error (same shape as the
  existing invalid-cadence test for another domain — copy its pattern).

Depends on Task 4 (needs `BackupEverything`/`StartBackupEverything` to exist).

## Task 6: Frontend — Settings card, API client, Activity Log domain

**Files:**
- Modify: `web/src/lib/api.ts` — `Settings` interface gains
  `everythingSchedule: string`, `everythingPreHook: string`,
  `everythingPostHook: string`; new `backupEverythingNow(): Promise<OkEnvelope
  & { started?: boolean }>` calling `POST /api/backup-everything`, placed near
  `backupFlashNow`/`backupConfigNow`.
- Modify: `web/src/pages/Settings.tsx` — a new section component (e.g.
  `EverythingSection`, follow whatever naming the other `*Section`
  components use once you read the file) rendered in the **schedules** tab
  (verify current tab id — confirmed "schedules" as of this investigation,
  but re-check, the file may have shifted), using `Card` +
  `CadenceBuilder` (bound to `settings.everythingSchedule`, wired through
  `buildSchedulePatch()` exactly like `filesSchedule`) + two `<input>` fields
  for the pre/post hook commands styled like `Containers.tsx`'s
  `HooksEditor` (monospace font class, a placeholder example — the post-hook
  placeholder should read like a healthcheck ping, e.g. `curl -fsS
  https://hc-ping.com/your-uuid`) + a persistent `bg-statusWarnBg`/
  `text-statusWarn` warning box describing the overlap-with-per-domain-
  schedules risk (mirror the tamper-schedule-inactive warning's exact
  markup/classes) + a manual "Run Backup Everything now" button calling
  `backupEverythingNow()` with the same busy/toast/disabled handling pattern
  `Containers.tsx`'s bulk-backup button uses (check its exact 409/busy
  message handling and mirror it, not the SSE progress plumbing — a
  cross-domain pass doesn't have one existing progress key to hook into, so a
  simple started/error message is enough for this feature; do not invent a
  new SSE progress key unless it is genuinely trivial given the existing
  `useProgress()`/`publishBatch` machinery — check before deciding either
  way and note the choice).
- Modify: `web/src/lib/activityLog.ts` — add `"everything"` to `LogDomain`,
  `DOMAIN_KEYS` (→ `"activityLog.domainEverything"`), `normalizeDomain`
  (pass-through case), `LogFilterDomain`.
- Test: `web/src/lib/activityLog.test.ts` (extend the existing file) — a case
  proving a run with `domain: "everything"` resolves to the new label and is
  matched by the new filter value. If a genuinely new PURE helper function
  gets extracted while building the Settings card (unlikely — this task
  mostly reuses `CadenceBuilder`/`Card` as-is), give it a
  `CadenceBuilder.test.ts`-style pure test; do not force a jsdom component
  test if nothing new and stateful enough to warrant one was written (match
  the existing test-coverage density in this codebase, don't invent
  coverage for its own sake).

Do NOT add any new i18n key STRINGS as final English copy without also
touching `i18n.ts`'s `en`/`de` blocks in the SAME commit — see Task 7 for the
other 24 locale files, but `en`/`de` are source-of-truth and belong with the
component that introduces the key, not deferred to Task 7.

Depends on Task 5 (needs the settings field names/route to be final).

## Task 7: i18n — all 24 locale files

**Files:** every `web/src/lib/locales/*.ts` (ar, cs, da, el, es, fi, fr, he,
hu, it, ja, ko, nl, no, pl, pt, ro, ru, sv, th, tr, uk, vi, zh — 24 files).

Add the SAME new keys Task 6 introduced into `en`/`de` (in `i18n.ts`) to all
24 locale files, with REAL, natural translations matching each locale's
existing terminology for "schedule"/"backup"/"hook" (cross-reference how
`hooks.pre`/`hooks.post`/`settings.filesSchedule`-style existing keys are
already phrased in that SAME file, for consistency) — never machine-literal
or copied-English placeholder text.

**Operational requirements for whoever executes this task (do not delegate
further):**
- Do this work YOURSELF. Do NOT spawn or delegate to sub-agents for any
  subset of the 24 files — edit every file directly yourself and commit at
  the end. (This repo has been bitten twice by a translator agent spawning
  parallel children that never got committed, leaving the tree looking
  clean with the keys silently missing.)
- Read `web/src/lib/i18n.ts`'s `en` block for the exact new key names/
  `{placeholder}` tokens Task 6 added, and one existing locale file in full
  (to match its phrasing register) before starting.
- After editing all 24 files, verify your own work before calling it done:
  every new key must appear in EVERY file exactly once (0 = missed a file,
  2+ = duplicate-inserted).
- Commit only after that verification passes.

Depends on Task 6 (the keys must exist in `en`/`de` first).

## Final whole-branch review

After Task 7, a fresh review pass across the WHOLE branch (not just the last
task's diff) — cross-task consistency (does the parent run's Activity Log
line actually read sensibly end-to-end; does a genuinely empty/"off"
`EverythingSchedule` leave every existing schedule byte-for-byte unaffected;
did any task leave a TODO/simplification it should have flagged) — same
pattern as prior features' whole-branch review step, documented in the PR
description alongside per-task disclosed limitations.

## Test/CI gates before opening the PR

`go build/vet/gofmt/golangci-lint`, `go test ./...` (restic on PATH);
`npx tsc --noEmit`, `npm run lint`, `npx vitest run`, `npm run build` (commit
`web/dist`); full CI (Lint + Test + Build Docker Image + the frontend job)
green before reporting done.
