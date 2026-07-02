# Restore UX: live progress + cancel + busy-feedback — design

**Source:** GitHub issue #24 (manilx). A 700 GB restore-to-folder ran ~3.5 h behind a bare spinner
with no progress and no way to stop it; a "Back up selected" started during the restore appeared to
do nothing with no explanation.

**Branch:** `feat/v4`. **Stack:** Go backend + React/Vite SPA, restic 0.17.3 engine, in-process
progress pub/sub over SSE.

**Goal:** while a restore runs, show its live percentage in the restore UI, let the user cancel it
safely, and never let a blocked backup look like it silently did nothing.

---

## Current state (reconnaissance anchors)

- **Restore is already async + progress-wired.** Every `Start*Restore` wrapper takes the
  `batchActive` single-flight (`service.go` ~2323/2571/2767/3773, `stacks.go` 249), detaches with
  `context.WithoutCancel` + `context.WithTimeout(bctx, restoreTimeout=48h)`, and the sync core
  (`executeRestore` `service.go:2270`, `executeRestoreVM` `:3738`, flash, stack loop) takes
  `lockDomain(domain)` and installs a restic progress sink via `progBegin(ctx, key, "restore")`.
- **Restore progress already flows end-to-end.** `RestorePathArgs`/`RestoreIncludeArgs`
  (`restic.go:246`/`287`) pass `--json`; `run` sets `RESTIC_PROGRESS_FPS=3` when a sink is present
  (`restic.go:519`) and `statusPercent` (`:606`) parses `percent_done`. Events flow as
  `progress.Event{Key, Phase:"restore", Percent, Active}` (`internal/progress/progress.go:42`) over
  `GET /api/progress` (`sse.go:17`) to the frontend `useProgress()` (`web/src/lib/progress.ts:136`).
  The container/VM **card** `ProgressBar` (`Containers.tsx:707`, `VMs.tsx:675`) already fills during a
  restore. Only per-file/byte counts are discarded (only percent is parsed).
- **Cancel does not exist.** All detached runs use `context.WithoutCancel`; the only cancel funcs are
  local `defer cancel()` from the `WithTimeout`. No registry, no endpoint. `UnlockDomain`
  (`service.go:4688`) clears restic *repo* locks, not the running process.
- **Busy responses are asymmetric.** Bulk `handleBackupAll` returns **HTTP 409** `{ok:false,error}`
  (`handlers.go:421`); single backup / all restore handlers return **HTTP 200** `{ok:false,error}`
  (`:388`,`:497`). `StartRestore` *does* take `batchActive`, so a UI backup during a restore gets the
  409 today. Two residual gaps: (1) the frontend `batchActive` flag is derived only from the
  `"batch:containers"` progress key (`Containers.tsx:936`), so a single restore doesn't disable the
  backup buttons — it relies on the 409 round-trip. (2) A scheduler/maintenance op holds **only**
  `lockDomain` (not `batchActive`); a bulk backup's `batchActive` CAS then succeeds, the goroutine
  launches, and the first `Backup()` blocks on `lockDomain` silently while the UI says "started".

---

## A. Restore progress in the panel (%-bar, reuse)

No backend change for percent. Frontend only:

- In `RestorePanel.tsx` (its `isPending` blocks: file-level ~195, `RecreateButton` ~373,
  `RestoreToFolder` ~436, whole-snapshot ~725) and the VM restore section (`VMs.tsx` ~356), read
  `useProgress()[key]` (key = `"container:"+name` / `"vm:"+name`) and render a labeled `ProgressBar`
  with the restore percentage and a "Restoring… NN%" caption. Indeterminate (`percent<=0`) until the
  first status line arrives, matching the backup bars.
- Give the card `ProgressBar` a phase-aware caption/label so a restore reads "Restoring" and a backup
  "Backing up" (the event already carries `phase`), removing today's ambiguity.
- New i18n keys (en+de inline in `i18n.ts`; 24 locale files deferred to a batch): `restore.progress`
  ("Restoring… {pct}%"), phase labels as needed.

Out of scope this round: per-file / bytes-done / ETA readout (would require parsing `total_files` /
`bytes_restored` from the status JSON — noted as a future enrichment).

## B. Cancel (full, graceful, type-differentiated)

**Principle: cancelled ≠ failed.** A user cancel is an intentional, recorded outcome that fires no
failure notification and cleanly releases the lock + clears the progress bar.

**Backend**

- **Run-cancel registry** on `Service`: `runCancels map[string]context.CancelFunc` + `sync.Mutex`.
  Each `Start*Restore` builds the detached context as
  `bctx := context.WithoutCancel(ctx)` → `tctx, tcancel := context.WithTimeout(bctx, restoreTimeout)`
  → `rctx, cancel := context.WithCancel(tctx)`; registers `cancel` under the progress key on launch;
  the goroutine `defer`s: delete the registry entry, `tcancel()`, `cancel()`. This covers
  `StartRestore`, `StartRestoreFiles`, `StartRestoreToPath`, `StartRestoreVM`, `StartRestoreStack`.
  (The DR-drill `runDRDrill` is a sandbox op — it may register too so a drill is cancellable, cleaning
  up via its existing marker-guarded path; optional, low priority.)
- **Endpoint** `POST /api/restore/cancel` body `{key}` (authGate): look up `runCancels[key]`, call it,
  return `{ok:true, cancelled:bool}`. Cancelling an unknown/finished key is a no-op success.
- **Graceful outcome.** `executeRestore*` distinguish `errors.Is(err, context.Canceled)` from a real
  error: record the restore run with a new terminal status **`"cancelled"`** (extend the runs store's
  status set + `finishRestoreRun`/`beginRestoreRun`), **do not** call the failure notifier, emit the
  terminal `progEnd(key, "restore", ok=false)` so the bar clears. Stack restore: a cancel aborts the
  loop and records the stack run cancelled. For in-place restores the target is already partially
  rebuilt — the recorded "cancelled" run + the UI warning (below) are the contract; the backend does
  not attempt a rollback.
- Distinguish "cancelled" from "error" wherever runs are surfaced (history list, the run row the
  frontend polls) so the UI can render it neutrally.

**Frontend**

- A **Cancel** button rendered next to the restore `ProgressBar` (in the panel `isPending` blocks and
  on the card during a restore). On click, a confirmation whose text depends on restore type:
  - **safe** (restore-to-folder, file-level-to-folder, drill/sandbox): light confirm — "Cancel the
    restore? The partial output folder is left as-is."
  - **in-place** (container/VM recreate, whole-snapshot in place, stack): hard confirm naming the
    consequence — "{name} is mid-restore. Cancelling leaves it partially restored — you'll need to
    restore again to get a working {name}. Cancel anyway?"
- `useBackupWatch` (`backupWatch.ts`) gains a **`cancelled`** terminal state distinct from
  `success`/`error`, driven by the run row's new status; renders a neutral "Restore cancelled" banner
  (not the red error banner).
- New i18n keys: `restore.cancel`, `restore.cancelConfirmSafe`, `restore.cancelConfirmInPlace`,
  `restore.cancelled`, `restore.cancelling`.

## C. Busy-feedback (both fixes)

**(a) Frontend awareness.** Replace the `"batch:containers"`-only `batchActive` derivation with a
broader "something is running" signal from `useProgress()`: if any key is `active` with phase
`backup`/`restore`/`replicate`, disable the "Back up selected" / restore-start buttons and show a
clear inline hint ("a restore is running" / "a backup is running") instead of relying on the 409
round-trip. Applies on Containers, VMs, Flash, and the restore panels.

**(b) Lock-only stall.** Introduce a lightweight **per-domain activity tracker** at the `lockDomain`
chokepoint: record which operation currently holds (or is running against) each domain
(`domainActivity map[string]string` under a mutex, set when the lock is acquired with a reason
label — backup / restore / prune / verify / replicate / scheduled — cleared on unlock). The backup
starters (`StartBackup`, `StartBackupAll`, VM/flash) check it up front and, if the target domain is
busy, return the same clear **409** `{ok:false,error:"<op> is running on <domain>"}` instead of
launching a goroutine that then blocks silently on the mutex. Expose the tracker (or reuse the
progress stream) so the frontend can name what's running. This closes the "appears to do nothing"
case for scheduler/maintenance contention, not just UI-initiated restores.

Because `lockDomain` returns an unlock closure, the reason can be threaded via a
`lockDomainFor(domain, reason)` variant (or a small wrapper) so callers label their hold; the plain
`lockDomain` stays for paths that don't need a label.

---

## Data model / API changes

- Runs store: add a terminal status value `"cancelled"` for restore runs (alongside
  `running`/`success`/`error`); `finishRestoreRun` accepts it; the run row JSON exposes it.
- New endpoint `POST /api/restore/cancel` (authGate).
- No DB migration for schema shape beyond the status string (it is a free-text/enum column already
  used for run status — verify the column accepts it; if it is a constrained enum, widen it).

## Error handling

- Cancel of an unknown/already-finished key → `{ok:true, cancelled:false}` (idempotent, no error).
- A restore that errors for a real reason still records `error` + fires the failure notifier
  (unchanged); only `context.Canceled` maps to `cancelled`.
- The activity tracker must always clear on unlock (defer), even on panic, so a crashed op can't wedge
  a domain as permanently busy.

## Testing

- Go: `TestRestoreCancelledRecordsCancelledNotError` (a fake engine restore that returns
  `context.Canceled` → run status `cancelled`, no failure notification, terminal progress emitted);
  `TestBackupStarterRefusesBusyDomain` (domain activity set → `StartBackupAll` returns the busy error
  without launching); cancel-registry set/delete lifecycle; idempotent cancel of an unknown key.
- Frontend gate: `tsc --noEmit` + `npm run build`, then revert `web/dist/index.html`.

## Out of scope (YAGNI / later)

- Per-file / bytes / ETA restore readout (percent only this round).
- Cancelling *backups* (the registry is built to allow it later; not wired now).
- Any change to the 48 h `restoreTimeout` (it stays a safety cap, not a UX bound).

## File map

- `internal/api/service.go` — cancel registry + `WithCancel` in each `Start*Restore`; activity
  tracker at `lockDomain`; `executeRestore*` cancelled-vs-error handling; busy-check in backup
  starters.
- `internal/api/handlers.go` — `POST /api/restore/cancel`; busy 409 responses.
- `internal/api/api.go` — route registration.
- `internal/store` (runs) — `cancelled` status plumbing.
- `web/src/lib/backupWatch.ts` — `cancelled` terminal state.
- `web/src/lib/progress.ts` / consumers — broaden the running-signal; phase-aware label.
- `web/src/components/RestorePanel.tsx`, `web/src/pages/{Containers,VMs,Flash}.tsx`,
  `web/src/components/ProgressBar.tsx` — in-panel restore bar + cancel button + busy hints.
- `web/src/lib/api.ts` — `cancelRestore(key)` call.
- `web/src/lib/i18n.ts` — new keys (en+de).
