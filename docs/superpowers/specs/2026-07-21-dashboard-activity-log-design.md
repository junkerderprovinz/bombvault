# Dashboard Activity Log — Design

**Goal:** One scrollable **log window** on the BombVault dashboard — a flat, timestamped list of lines, exactly like a Docker container log (`docker logs -f`). It streams what is happening live (with an in-place percentage on the running line where the engine gives a real one), you scroll up to see what happened when, and when nothing is running the tail shows what is scheduled next.

**Approved by the user (2026-07-21):** a single flat scrollable log list, **no zones/regions** — "like the log of a docker container". Progress shown inline on the running line where possible; honest "…working" where the engine gives no percentage.

## What exists today (audit, file:line)

- **SSE progress** `GET /api/progress` (`internal/api/sse.go`) streams `progress.Event{Key,Phase,Percent,Active}` (`internal/progress/progress.go:42`). Frontend shared store `web/src/lib/progress.ts` (`useProgress()` → map keyed by event key). Phases: `backup` | `restore` | `replicate`. Backup/restore carry a real `Percent`; off-site `replicate` is indeterminate (`restic copy` has no progress; `service.go:1691`).
- **Run history** `runs` table (`internal/store/runs.go`): `Run{ID,TargetID,Kind,Status,StartedAt,FinishedAt,SnapshotID,Bytes,Error}`, `StartRun(targetID,kind)`, `ListRuns(limit)`. Today only `kind` `backup` | `restore` | `update` are recorded; **prune and verify write NO run row and emit NO progress event.** Domain attributed by `target_id` membership (`targets`/`vms`/`file_sets` + reserved `flash`/`config`).
- **Scheduler** (`internal/schedule/schedule.go`): holds `*cron.Cron` + `entryIDs`, exposes **no next-fire accessor**. `/api/status` `DomainStatusEntry` (`service.go:932`) has no next-run time.
- **Dashboard** (`web/src/pages/Dashboard.tsx`): posture + history page; only live element is the off-site spinner; "Next backup" is cadence text only.

## Design

A single scrollable **log window** — a flat, timestamped list of lines, like `docker logs -f`. **No separate zones.** New lines appear at the **bottom**; the view **auto-follows the tail** while scrolled to the bottom, and stops following once the user scrolls up to read back (a "jump to latest" affordance returns to the tail). Monospace-ish, quiet, dense — a real log.

### Line format
One line per event: `HH:MM:SS  <glyph> <message>`, coloured by status. Examples:
- `03:00:01  ▶ Scheduled backup started — containers`
- `03:00:02  ⋯ Backing up plex … 42%`   ← updates IN PLACE while running (real % for backup/restore)
- `03:01:15  ✓ plex backed up — 1.2 GB in 74s`
- `03:04:10  ✓ Retention prune done — containers`
- `03:04:11  ↗ Off-site upload — containers …`   ← indeterminate (no %)
- `03:20:00  ✗ Verify failed — vms: repository is already locked`
- `— idle — next: backup (containers) at 03:00 (in 6h 40m) —`   ← trailing line when nothing runs

### One stream: live + history merged
- **Completed** operations are lines from Run History (`ListRuns`), authoritative, each with its result + duration, ordered by time.
- **Currently running** operations are live lines at the tail, from the SSE store: backup/restore show an **in-place updating percentage** (real, from restic); off-site/prune/verify show an indeterminate `…`. When an op finishes (SSE `active:false`), its live line is superseded by the Run-History line on the next `/api/runs` poll — deduped by target + start time, so nothing appears twice.
- **Idle:** a trailing line shows the soonest scheduled op with a live countdown, from the new scheduler next-fire accessor. (Not a zone — just the last line of the log.)

### Filter / search
A lightweight filter/search field above the log narrows the visible lines — like `docker logs | grep`: by domain (containers/vms/flash/config/files), by type (backup/restore/prune/verify/off-site), or free text. It does not create zones; it only filters the one list.

## Changes

### Backend (Go) — small, additive
1. **Prune/Verify emit SSE progress.** Around the domain-lock section of prune (`PruneDomain`) and verify (`CheckDomain`) in `service.go`, publish `progress.Event{Key:"prune:<domain>"/"verify:<domain>", Phase:"maintenance", Percent:0, Active:true}` on start and `Active:false` on finish, via the existing `progress.Store`. Indeterminate. Reuses the SSE pipe.
2. **Prune/Verify write run records.** `StartRun(<reservedDomainID>, "prune"/"verify")` + `FinishRun(...)` around prune/verify, with a reserved domain target id ("containers"/"vms"/"flash"/"config"/"files"). Existing last-successful-backup / everyN-gate / RunCounts queries all filter `kind='backup'`, so `prune`/`verify` rows never pollute them; `ListRuns` returns all kinds → they appear in the log.
3. **Scheduler next-fire accessor.** `Scheduler.NextRuns() []NextRun` over each registered entry's `cron.Entry.Next()`, mapping entry → job label/domain → `{Job, Domain, NextRun time.Time}` for every enabled job (backup per domain, off-site per domain, drills, tamper). Expose via a new `GET /api/schedule/next` (soonest first).

### Frontend (React/TS)
4. **Extend `progress.ts`**: add `maintenance` to `ProgressPhase`, kept distinct in `handleMessage` (today unknown phases collapse to `backup`). No change for existing consumers.
5. **New `ActivityLog` component** (`web/src/components/ActivityLog.tsx`): a flat, scrollable, auto-following log list. It merges (a) Run-History rows (`listRuns`) into completed log lines and (b) live SSE events into tail lines with in-place % (real for backup/restore, `…` otherwise), deduped by target+start; a trailing idle "next up" line from `/api/schedule/next`. Auto-follow-tail with a "jump to latest" button; a filter/search field. Reuse existing status colours.
6. **Wire into `Dashboard.tsx`**: render `ActivityLog` as a prominent full-width card near the top (below `SummaryTier`). Keep the existing cards.
7. **API client** (`web/src/lib/api.ts`): add `getScheduleNext()`. `Run.kind` already a string (new kinds fit).
8. **i18n**: new strings (line templates, glyph labels, filter, kinds, idle/next) in `web/src/lib/i18n.ts` (en+de) + all locale files, one writer.

## Honest limitations
- **Real percentage only for backup + restore.** Off-site upload, prune and verify give no machine-readable progress from restic → the running line shows `…`, not a percent. A fake percent would be dishonest.
- History depth is bounded by `ListRuns(limit)`; scrolling further back can page more if needed (later — start with a generous limit).

## Testing
- Go: unit-test `Scheduler.NextRuns()` (enabled jobs → non-zero next; disabled → absent); prune/verify run-record + SSE-event emission (fake progress store + store); existing gate/RunCounts tests still pass (prune/verify excluded by `kind='backup'`).
- Frontend: `tsc --noEmit`; a light render test of `ActivityLog` (a fake progress map + history rows produce merged, ordered, deduped lines; filter narrows) if the harness supports it.

## Out of scope
- No parallelizing backups (shared restic lock — v6.5.0); scattered start times remain inherent, the log only makes them visible.
- No per-file live progress for off-site/prune/verify (engine gives none).
