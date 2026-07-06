# Dashboard times + domain filters + drill-schedule-in-Plans (#39/#40/#41/#42/#43) — plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

Batch of UI feature requests, one v4.9.0 (Minor), separate commits:
- **#39** (ptmorris1): show start + end + duration in the Dashboard "Last Backups" card.
- **#40** (ptmorris1): search filter on the Containers domain view (+ VMs).
- **#41** (ptmorris1): filter by scheduled/not-scheduled and backed-up/never-backed-up.
- **#42** (manilx): move the "Automatic restore checks" (drill) schedule from Settings into the Plans (Jobs) page.
- **#43** (manilx): i18n hint clarification only (the zip-export "Keep history" is separate from restic retention).

All frontend-only except #39's small DTO addition. Data for #40/#41 is already client-side.

## Global Constraints
- Branch `feat/views-and-plans` (off main == v4.8.0). Go gates: build/vet, `gofmt -l internal/ cmd/` empty,
  `go test ./internal/... -count=1`, `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- i18n keys land in Task 2 (before the frontend tasks) so tsc passes. New keys across en+de inline + all 24
  `web/src/lib/locales/*.ts`. Count-neutral phrasing (no plural forms).

## New i18n keys (Task 2 adds these; later tasks use exactly these)
- `dashboard.duration` = "Duration"
- `containers.searchPlaceholder` = "Search containers…"
- `vms.searchPlaceholder` = "Search VMs…"
- `filter.all` = "All"
- `filter.scheduled` = "Scheduled"
- `filter.notScheduled` = "Not scheduled"
- `filter.backedUp` = "Backed up"
- `filter.neverBackedUp` = "Never backed up"
- `filter.schedule` = "Schedule"   (group label)
- `filter.backup` = "Backup"       (group label)
- `filter.noMatch` = "No items match the current filters."
- MODIFY `flash.zipExport.keepHistoryHint`: append " This is separate from the restic retention: off keeps a single, always-overwritten file (never fills the destination); on keeps the newest N and deletes older ones."

---

### Task 1 — #39 backend: expose the last backup's START time
**Files:** `internal/api/handlers.go`, `web/src/lib/api.ts`, `internal/api/handlers_test.go` (if present).

- [ ] `containerView` (handlers.go:162-181): add `LastBackupStarted *int64 \`json:"lastBackupStarted"\``. It is
  populated where `LastBackup` is set from the run (handlers.go ~:219-221 and ~:246-248 — the full `run` is in
  hand; `LastBackup` uses `run.FinishedAt`). Add `v.LastBackupStarted = &run.StartedAt` (or the pointer form the
  neighbours use) at both sites.
- [ ] `VMView` + `listVMs` (mirror the same: find where VMView.LastBackup is set from a run, add LastBackupStarted).
- [ ] `web/src/lib/api.ts`: add `lastBackupStarted: number | null;` to the `Container` type (~:22, near
  `lastBackup`) and the `VM` type (~:1069-1076).
- [ ] Gates (Go build/vet/gofmt/test/golangci + `cd web && npx tsc --noEmit`). Commit
  `feat(api): expose last backup start time on container/VM views (#39)`.

### Task 2 — i18n (before the frontend tasks)
**Files:** `web/src/lib/i18n.ts` (en+de) + all 24 `web/src/lib/locales/*.ts`.

- [ ] Add all the "New i18n keys" above (proper translation per locale) AND append the clarification sentence to
  the existing `flash.zipExport.keepHistoryHint` value in every one of the 26 maps.
- [ ] Verify each key has 26 hits + `tsc --noEmit` 0. Commit `i18n: keys for dashboard times, domain filters, zip-keep hint (#39/#40/#41/#43)`.

### Task 3 — #39 frontend: start/end/duration in Last Backups
**Files:** `web/src/pages/Dashboard.tsx`.

- [ ] `LastBackupsCard` (Dashboard.tsx:808-868) sources from `listContainers()` and renders `formatTs(c.lastBackup)`
  (finish) per backed-up row (~:841-849). Change the backed-up row to show start → end + duration:
  `formatTs(c.lastBackupStarted)` … `formatTs(c.lastBackup)` and, when both present, a duration
  `formatDuration(c.lastBackup - c.lastBackupStarted)`. Keep the never-backed-up rows unchanged.
- [ ] Add a small `formatDuration(seconds: number): string` helper near `formatTs` (Dashboard.tsx:16-19):
  compact, plural-free, e.g. `< 60 → "12s"`, `< 3600 → "3m 5s"`, else `"1h 2m"`. Use `t("dashboard.duration")`
  as the element `title`. Guard null `lastBackupStarted` (older data): fall back to showing just the finish time.
- [ ] Gates (`tsc` + `npm run build`; restore `web/dist/index.html`). Commit
  `feat(web): show start, end and duration in the Last Backups card (#39)`.

### Task 4 — #40 + #41 Containers: search + schedule/backup filters
**Files:** `web/src/pages/Containers.tsx`.

- [ ] Read the existing installed/not-installed `FilterControl` + localStorage helpers (Containers.tsx:145-187),
  its state (~:1125), and where it renders in the controls row (~:1324); the derived arrays are at
  `sorted/live/orphans` (~:1163-1165) and the maps at ~:1397/1421.
- [ ] Add: (a) a controlled search `<input>` (state, placeholder `t("containers.searchPlaceholder")`) filtering
  by case-insensitive substring of `c.name` (and `c.image`); (b) two chip `FilterControl`s cloning the existing
  one — SCHEDULE (`filter.all`/`filter.scheduled`=`includeInSchedule`/`filter.notScheduled`) and BACKUP
  (`filter.all`/`filter.backedUp`=`lastBackup != null`/`filter.neverBackedUp`). Persist chip state in localStorage
  like the existing `bv-containers-filter`.
- [ ] Compose all predicates (search + 2 new chips + the existing installed toggle) into ONE filter applied to
  `containers` BEFORE the `sorted/live/orphans` derivation (~:1163-1165), so they combine. Show `t("filter.noMatch")`
  when the live+orphans result is empty but containers exist.
- [ ] Gates + commit `feat(web): search + schedule/backup filters on the Containers view (#40, #41)`.

### Task 5 — #40 + #41 VMs: search + filters (net-new FilterControl)
**Files:** `web/src/pages/VMs.tsx`.

- [ ] VMs.tsx currently has only a SortControl (controls row ~:974-995); derived arrays at ~:868-870. Add the SAME
  search box (`vms.searchPlaceholder`, filter by `v.name` only — VMs have no image) + SCHEDULE + BACKUP chip
  filters (VM has `includeInSchedule` + `lastBackup`, no `installed`). Clone the FilterControl pattern from
  Containers.tsx (a VM-scoped copy; VMs' own localStorage key e.g. `bv-vms-filter`). Compose before ~:868-870,
  empty-state `t("filter.noMatch")`.
- [ ] Gates + commit `feat(web): search + schedule/backup filters on the VMs view (#40, #41)`.

### Task 6 — #42: move the drill schedule into Plans (Jobs)
**Files:** `web/src/pages/Jobs.tsx`, `web/src/pages/Settings.tsx`.

- [ ] Read the Jobs page (the "Plans" view): domain sections ContainersSection/VMsSection/FlashSection
  (Jobs.tsx:131/209/255), rendered stacked (~:432-469), ONE shared SaveBar (~:472) whose `handleSave`
  (~:391) + `buildSchedulePatch` (~:376) PUT the whole settings object merged on `savedSettings`. And the drill
  card in Settings.tsx:1912-1977 (`drillsEnabled` toggle, `offsiteDrillsEnabled` sub-toggle, `drillsSchedule`
  CadenceBuilder, `drillsSubsetPct` input, its own SaveBar + state at :1212-1213).
- [ ] Add a "Restore checks" section to Jobs.tsx (a 4th section beside Containers/VMs/Flash) with the drillsEnabled
  toggle + offsiteDrillsEnabled sub-toggle (`t("settings.offsiteDrills")`, disabled unless drillsEnabled) + the
  `drillsSchedule` CadenceBuilder + the `drillsSubsetPct` input (`t("verify.subsetPct")`). Title `t("verify.auto")`.
  Make it VISIBLE in Plans (NOT advanced-gated). Reuse the `ToggleRow` component (lift/import from Settings.tsx or
  a shared spot — if it's local to Settings.tsx, export it or copy a minimal toggle).
- [ ] Wire the 4 drill fields into Jobs' save: extend `buildSchedulePatch`/`handleSave` (Jobs.tsx:376-413) to
  include `drillsEnabled, offsiteDrillsEnabled, drillsSchedule, drillsSubsetPct`, so the shared SaveBar persists
  them. CRITICAL: Jobs' `handleSave` must `window.dispatchEvent(new Event("bv:settings-changed"))` after the PUT
  (Settings' save did this to refresh the Dashboard drill pills) — add it if not already there.
- [ ] Remove the drill card from Settings.tsx (:1912-1977) AND its now-unused state (`drillsSaveState`/`drillsSaveError`
  :1212-1213) and any now-unused imports; verify no dangling references (tsc will catch).
- [ ] Gates + commit `feat(web): move the restore-check schedule into Plans (#42)`.

## Self-review
#39 (DTO start + card render), #40 (search), #41 (chip filters), #42 (move to Plans), #43 (hint) all covered.
Filter data is client-side; #39's only backend touch is the start-time DTO field. i18n before frontend.

## Handoff
Subagent-driven: T1 → T2 → then T3/T4/T5/T6 (disjoint files: Dashboard / Containers / VMs / Jobs+Settings — safe
in parallel). Then adversarial review (filter composition, #42 save+event+Settings cleanup, #39 null-start guard).
Release **v4.9.0** (Minor). Reply/close #39/#40/#41/#42.
