# Dashboard Activity Log — Design

**Goal:** One unified "Activity Log" panel on the BombVault dashboard that shows, live, what is running right now (with progress bars where the engine gives a real percentage), a scrollable reverse-chronological history of finished operations (so the user can scroll back and see what happened when), a filter by domain/type, and — when nothing is running — the operations scheduled to run next, with real countdowns.

**Approved by the user (2026-07-21):** one panel with a filter, not multiple separate windows. Progress bars where possible; honest indeterminate "working" bars where the engine gives no percentage.

## What exists today (audit, file:line)

- **SSE progress** `GET /api/progress` (`internal/api/sse.go`) streams `progress.Event{Key,Phase,Percent,Active}` (`internal/progress/progress.go:42`). Frontend shared store `web/src/lib/progress.ts` (`useProgress()` → map keyed by event key). Phases today: `backup` | `restore` | `replicate`. Backup/restore carry a real `Percent` (restic `--json percent_done`); off-site `replicate` is indeterminate (`restic copy` has no machine-readable progress; sink discarded, `service.go:1691`).
- **Run history** `runs` table (`internal/store/runs.go`): `Run{ID,TargetID,Kind,Status,StartedAt,FinishedAt,SnapshotID,Bytes,Error}`, `StartRun(targetID,kind)`, `ListRuns(limit)`. Today only `kind` `backup` | `restore` | `update` are recorded; **prune and verify write NO run row and emit NO progress event.** Domain is attributed by `target_id` membership (`targets`/`vms`/`file_sets` tables + reserved `flash`/`config` literals).
- **Scheduler** (`internal/schedule/schedule.go`): holds `*cron.Cron` + `entryIDs` but exposes **no next-fire accessor**; the only `.Next()` use measures interval length from a fixed base, not next-fire-from-now. `/api/status` `DomainStatusEntry` (`service.go:932`) carries `Schedule` text, `PeriodSeconds`, `LastSuccess` — no next-run time.
- **Dashboard** (`web/src/pages/Dashboard.tsx`): posture + history page. The only live element is `OffsiteIndicator` (off-site spinner). "Next backup" is cadence text ("Daily 03:00"), explicitly not a real next-run time.

## Design

A single `ActivityLog` card, placed prominently near the top of the dashboard. Three regions:

### 1. Live (top) — what is running now
Subscribes to the existing shared SSE store (`useProgress()`). One row per active `Event`:
- **Backup / Restore** of container/vm/flash/files/config (`key` `container:<name>` / `vm:<name>` / `flash` / `files:<set>` / `config`, phase `backup`/`restore`): real percentage progress bar. Label e.g. "Backing up plex … 42%".
- **Batch** (`batch:containers` / `batch:files`): count-based bar (the backend already emits these).
- **Off-site upload** (`offsite:<domain>`, phase `replicate`): indeterminate "working" bar / spinner ("Uploading off-site (containers) …"). No percentage — honest.
- **Prune / Verify** (NEW `key` `prune:<domain>` / `verify:<domain>`, phase `maintenance`): indeterminate bar ("Pruning containers …" / "Verifying containers …"). No percentage.

### 2. History (below, scrollable) — what happened when
Reverse-chronological list (newest just under the live rows; a newly-started op pushes finished ones down), from `ListRuns`. Each row: op icon, kind, target/domain, start → end, duration, result (success/failed/cancelled), error on failure. Scrollable to page back through recent days.

### 3. Idle → Up Next
When nothing is live, the top region shows the next scheduled operations with real countdowns ("Next: backup (containers) in 2h 14m — 03:00"), from a new scheduler next-fire accessor. Covers per-domain backup, per-domain off-site, drills, tamper. The history log stays below.

### Filter
A single control filters BOTH the live rows and the history log by domain (containers/vms/flash/config/files) and/or type (backup/restore/prune/verify/off-site). Replaces the "separate window per task" idea with one filterable view. Frontend-only.

## Changes

### Backend (Go) — small, additive
1. **Prune/Verify emit SSE progress.** In `service.go`, around the domain-lock section of prune (`PruneDomain`) and verify (`CheckDomain`), publish `progress.Event{Key:"prune:<domain>"/"verify:<domain>", Phase:"maintenance", Percent:0, Active:true}` on start and `Active:false` on finish (via the existing `progress.Store`). Indeterminate — no percent. This reuses the SSE pipe (audit's recommended option a).
2. **Prune/Verify write run records.** Record `StartRun(<domainReservedID>, "prune"/"verify")` + `FinishRun(...)` around prune/verify, using a reserved domain target id per domain ("containers"/"vms"/"flash"/"config"/"files"). The existing last-successful-backup / everyN-gate / RunCounts queries all filter `kind='backup'`, so `prune`/`verify` rows never pollute them (verify with the reserved ids). `ListRuns` returns all kinds → they appear in the history log.
3. **Scheduler next-fire accessor.** Add `Scheduler.NextRuns() []NextRun` (over the tracked `cron.Entry.Next()` for each registered `entryID`, mapping entry → job label/domain), returning `{Job, Domain, NextRun time.Time}` for every enabled job (backup per domain, off-site per domain, drills, tamper). Expose via a new `GET /api/schedule/next` returning the soonest-first list.

### Frontend (React/TS)
4. **Extend `progress.ts`**: add `maintenance` to `ProgressPhase` and keep it distinct in `handleMessage` (today unknown phases collapse to `backup`). No behavior change for existing consumers (they check specific phases).
5. **New `ActivityLog` component** (`web/src/components/ActivityLog.tsx` + maybe `ActivityRow`): live rows (from `useProgress()`, real bar for backup/restore, indeterminate for replicate/maintenance) + scrollable history (from `listRuns`) + idle up-next (from `/api/schedule/next`) + the domain/type filter. Reuse the existing `ProgressBar` and status-chip styling.
6. **Wire into `Dashboard.tsx`**: render `ActivityLog` as a prominent card near the top (below `SummaryTier`). Keep the existing cards. The `SummaryTier` "Next backup" cadence text can stay (or link to the panel).
7. **API client** (`web/src/lib/api.ts`): add `getScheduleNext()` for the new endpoint; `Run.kind` already covers new kinds (string).
8. **i18n**: new strings (live labels, up-next, filter, kinds) in `web/src/lib/i18n.ts` (en+de) + all locale files, one writer.

## Honest limitations
- **Real percentage only for backup + restore.** Off-site upload, prune and verify give no machine-readable progress from restic, so they show an indeterminate "working" bar, not a percent. A fake percent would be dishonest.
- The up-next countdown is only as accurate as the cron next-fire; a run already overdue/queued shows accordingly.

## Testing
- Go: unit-test `Scheduler.NextRuns()` (registered jobs → non-zero next times; disabled → absent); the prune/verify run-record + SSE-event emission (fake progress store + store). Existing gate/RunCounts tests must still pass (prune/verify rows excluded).
- Frontend: `tsc --noEmit`; a light render test of `ActivityLog` (live row from a fake progress map, history rows, filter) if the test harness supports it.

## Out of scope
- No parallelizing backups (they share the restic lock — v6.5.0). Scattered start times remain inherent; this panel only makes them visible + predictable.
- No per-file live progress for off-site/prune/verify (engine gives none).
