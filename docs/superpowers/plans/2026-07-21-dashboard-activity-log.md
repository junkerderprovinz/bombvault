# Dashboard Activity Log — Implementation Plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development.

**Goal:** A flat, scrollable, `docker logs`-style activity log on the dashboard — live lines (with in-place % for backup/restore, `…` for off-site/prune/verify), a merged reverse history, an idle "next up" line, and a filter. Spec: `docs/superpowers/specs/2026-07-21-dashboard-activity-log-design.md`.

**Tech Stack:** Go (SSE progress store, runs SQLite, robfig/cron scheduler), React/Vite/TS SPA.

## Global Constraints

- Before push: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `golangci-lint run ./...`, `go test ./...` (restic ≥0.17 on PATH). Frontend when `web/` changes: `cd web && npx tsc --noEmit` (Docker stage builds the SPA; do NOT hand-build/commit `web/dist` — it is a placeholder rebuilt by CI).
- 3-digit SemVer; NEVER tag without approval. Public repo, no real data.
- i18n: every new UI string goes into `web/src/lib/i18n.ts` (en+de) AND all `web/src/lib/locales/*.ts` in the SAME change (one writer).
- Reuse existing seams: progress `Store.Publish` + service `progBegin`/`progEnd` (`service.go:383,406`), `runs` `StartRun`/`FinishRun`/`ListRuns` (`store/runs.go`), scheduler `entryIDs` (`schedule.go:276`).

---

### Task 1: Backend — Scheduler.NextRuns() + GET /api/schedule/next

**Files:** `internal/schedule/schedule.go`, `internal/schedule/schedule_test.go`, `internal/api/` (handler + route registration), `cmd/bombvault/main.go` (if a wiring seam is needed).

**Interfaces (produces):**
- `type NextRun struct { Job string; Domain string; Next time.Time }` (Job e.g. "backup"/"offsite"/"drill"/"tamper"; Domain e.g. "containers"/"vms"/... or "" where N/A).
- `func (s *Scheduler) NextRuns() []NextRun` — soonest first, only enabled entries with a non-zero `Next`.

**Approach:** the registration loop (`schedule.go:573-613`) currently appends only `cron.EntryID` to `entryIDs`. Change tracking to also keep each entry's job+domain label (e.g. replace `entryIDs []cron.EntryID` with `entries []scheduledEntry{ id cron.EntryID; job, domain string }`, updating `Reload*` remove logic at `schedule.go:411-414` accordingly). Each `domainSpec` already has `.name` (`schedule.go:384-389`) — derive job+domain from it. `NextRuns()` reads `s.c.Entry(e.id).Next` per entry, drops zero times, sorts ascending.

- [ ] Step 1: test `TestNextRunsReturnsEnabledEntries` — build a Scheduler, Reload with a daily container schedule enabled, assert `NextRuns()` contains a containers/backup entry with `Next.After(now)`; a disabled domain is absent. Run → FAIL.
- [ ] Step 2: implement the entry-label tracking + `NextRuns()` + the `NextRun` type. Keep `Start`/`Stop`/`Reload` behavior identical.
- [ ] Step 3: run the scheduler tests → PASS. `go build/vet/gofmt`.
- [ ] Step 4: add `GET /api/schedule/next` — a handler that calls `scheduler.NextRuns()` and returns JSON `[{"job","domain","next":<RFC3339>}]`. Register the route next to the other GET routes (find the router in `internal/api/api.go`); the handler needs the `*schedule.Scheduler` (wire it into the API service/handlers if not already reachable — mirror how other scheduler-touching handlers get it).
- [ ] Step 5: `go build/vet/gofmt/golangci-lint`; `go test ./internal/schedule/ ./internal/api/`. Commit `feat(schedule): NextRuns() + GET /api/schedule/next for the activity log`.

### Task 2: Backend — Prune/Verify emit SSE progress + write run records

**Files:** `internal/api/service.go` (`PruneDomain` ~7296, `CheckDomain` ~6569), a small helper for the reserved per-domain run id, `internal/api/*_test.go`.

**Interfaces (consumes):** `s.progBegin(ctx,key,phase)`/`s.progEnd(key,phase,ok)` (`service.go:383,406`), `s.store.StartRun(targetID,kind)`/`FinishRun` (`store/runs.go:24,38`), `store.FlashTargetID`/`ConfigTargetID`.

**Approach:**
- Add `func domainRunTargetID(domain string) string` returning the reserved run target id per domain: `flash`→`store.FlashTargetID`, `config`→`store.ConfigTargetID`, else the domain literal ("containers"/"vms"/"files"). These literals are never real target ids (those are hex/UUID) and the backup gates/RunCounts filter `kind='backup'`, so `prune`/`verify` rows never pollute them.
- In `PruneDomain`: after acquiring the domain lock and before the restic prune, `s.progBegin(ctx, "prune:"+domain, "maintenance")` and `defer s.progEnd("prune:"+domain, "maintenance", <ok>)`; record `runID,_ := s.store.StartRun(domainRunTargetID(domain), "prune")` and `defer s.store.FinishRun(runID, statusFrom(err), "", 0, errMsgFrom(err))`. Guard `s.progress != nil` (it is optional — progBegin/progEnd already no-op on nil, confirm).
- In `CheckDomain`: the same with `"verify:"+domain` / kind `"verify"`. (Note CheckDomain already holds the domain lock and has the 15-min ctx — insert the begin/run there, end on return.)
- Do NOT change what prune/verify actually do; only add the progress event + run record around them.

- [ ] Step 1: test (in `internal/api`, using the existing fake progress store / store harness) that `PruneDomain` and `CheckDomain` (a) publish a `maintenance`-phase begin+terminal event under key `prune:<domain>`/`verify:<domain>` and (b) record a run with the right kind + reserved target id + terminal status. Run → FAIL.
- [ ] Step 2: implement `domainRunTargetID` + wrap `PruneDomain` and `CheckDomain`.
- [ ] Step 3: confirm existing gate/RunCounts/last-successful tests still pass (prune/verify excluded by `kind='backup'`). `go build/vet/gofmt/golangci-lint`; `go test ./internal/api/ ./internal/store/`.
- [ ] Step 4: Commit `feat(api): prune/verify emit maintenance progress + write run records`.

### Task 3: Frontend — progress.ts phase + api client

**Files:** `web/src/lib/progress.ts`, `web/src/lib/api.ts`.

- [ ] Step 1: `progress.ts` — add `"maintenance"` to `ProgressPhase` (`progress.ts:17`) and preserve it in `handleMessage` (`progress.ts:110`, which today maps unknown phases to `backup`) so `prune:`/`verify:` events keep phase `maintenance`. `anyActive`/`busyPhraseKey` unchanged (maintenance is not a start-blocking phase — leave it out of `anyActive`'s backup/restore/replicate check so it doesn't disable bulk buttons).
- [ ] Step 2: `api.ts` — add `getScheduleNext(): Promise<ScheduleNext[]>` for `GET /api/schedule/next` and a `ScheduleNext { job: string; domain: string; next: string }` type (mirror the existing fetch helpers).
- [ ] Step 3: `cd web && npx tsc --noEmit` → passes. Commit `feat(web): maintenance progress phase + getScheduleNext client`.

### Task 4: Frontend — ActivityLog component (flat log)

**Files:** `web/src/components/ActivityLog.tsx` (+ a small `logLines` helper module if it keeps the component clean), a light test if the harness supports it.

**Behavior:**
- A single scrollable `<div>` of monospace-ish log lines. Data merged from: (a) `listRuns(limit)` polled (each finished run → one completed line: `HH:MM:SS <glyph> <kind> <target/domain> — <result>[, duration]`); (b) `useProgress()` live (each active key → a tail line; backup/restore show `… NN%` in place from `percent`; `replicate`/`maintenance` show `…`); (c) `getScheduleNext()` polled → a trailing idle line when nothing is active.
- **Dedupe:** a live key that also has a just-finished run (same target + overlapping time) shows once — prefer the history line once the run is recorded; drop the live line when its SSE `active` clears (progress.ts already drops it after linger).
- **Ordering:** ascending by time, newest at the bottom.
- **Auto-follow tail:** stick to the bottom while the user is at the bottom; when scrolled up, stop auto-scrolling and show a "jump to latest" button.
- **Filter/search:** a small input + domain/type chips above the log that narrow the visible lines (client-side).
- Map run `kind` + `status` + `Phase` to glyph + colour (reuse the existing status colours; ▶ start, ⋯ running, ✓ success, ✗ failed, ↗ off-site).
- All visible strings via i18n keys (Task 5 adds them; reference the keys here).

- [ ] Step 1: build `ActivityLog` consuming `listRuns`, `useProgress`, `getScheduleNext`; implement merge+dedupe+order into a `LogLine[]`, render, auto-follow, filter.
- [ ] Step 2: `cd web && npx tsc --noEmit` → passes. If a component-test harness exists, add a test that a fake progress map + runs produce ordered, deduped lines and the filter narrows them; else note skipped.
- [ ] Step 3: Commit `feat(web): ActivityLog flat docker-logs-style activity panel`.

### Task 5: Frontend — wire into Dashboard + i18n

**Files:** `web/src/pages/Dashboard.tsx`, `web/src/lib/i18n.ts` + all `web/src/lib/locales/*.ts`.

- [ ] Step 1: render `<ActivityLog/>` as a prominent full-width card near the top of `Dashboard.tsx` (below `SummaryTier`, above the existing cards). Keep everything else.
- [ ] Step 2: add all new i18n keys (log line templates with placeholders, kind labels backup/restore/prune/verify/off-site, glyph alt text, filter label, idle/next line, "jump to latest") to the en+de blocks in `i18n.ts` AND every locale in `web/src/lib/locales/` — one writer, accurate translations, key parity across all files.
- [ ] Step 3: `cd web && npx tsc --noEmit` → passes; verify key parity (each new key present in every locale). Commit `feat(web): mount ActivityLog on the dashboard + i18n`.

## Self-Review
- Backend additions are additive (new event phase, new run kinds with reserved ids, new read-only endpoint) — no change to backup/restore/gate behavior; RunCounts/last-successful stay `kind='backup'`.
- Honest: real % only for backup/restore; off-site/prune/verify indeterminate.
- After the build: run the whole suite + tsc, then (with approval) release as a minor.
