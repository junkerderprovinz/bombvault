# Off-site DR drill opt-out (#37) — plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

#37 (BaukeZwart, confirmed): let the SCHEDULED off-site disaster-recovery (DR) drill be turned off — keeping the
free scheduled LOCAL integrity check — because the DR drill re-downloads the whole off-site snapshot each run
(egress cost on Backblaze). He'll run the remote DR check MANUALLY via the existing v4.7.0 button.

**Design (global, default ON):** one new `Settings.OffsiteDrillsEnabled` bool (default true = current behavior).
It gates ONLY the scheduled `{*,offsite,dr}` drill tasks; the scheduled `{*,local,subset}` checks and the manual
"Run off-site DR check" button are unaffected. The scorecard + posture chip go NEUTRAL by reusing the existing
`DrillsEnabled`-off path (change one guard); the dashboard pill needs a new `OffsiteDrillScheduled` flag to show a
muted "manual only" state instead of red. Scope is global (mirrors DrillsEnabled; per-domain is a non-breaking
future add). NOT changing the underlying DR verification.

## Global Constraints
- Branch `feat/offsite-dr-optout` (off main == v4.7.1). Go gates: build/vet, `gofmt -l internal/ cmd/` empty,
  `go test ./internal/... -count=1`, `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- **Default MUST be true end-to-end** (migration DEFAULT 1 + full settings.go read/write wiring), or upgrades
  silently disable the DR drill. Do NOT touch protectionChecks/protectionLevel branch logic beyond the single guard.

---

### Task 1 — Store: `OffsiteDrillsEnabled` field + migration v54
**Files:** `internal/store/settings.go`, `internal/store/migrate.go`, `internal/store/settings_test.go`.

- [ ] Add `OffsiteDrillsEnabled bool` to `store.Settings` next to `DrillsEnabled` (settings.go ~:92) with a doc
  comment ("scheduled off-site DR drill; default on"). SQLite stores bools as INTEGER, so wire ALL FOUR spots
  (mirror `DrillsEnabled` exactly — a miss silently drops the value):
  - SELECT column list: add `offsite_drills_enabled` (settings.go ~:136).
  - Scan: add `var offsiteDrillsEnabled int` (~:144) and `s.OffsiteDrillsEnabled = offsiteDrillsEnabled != 0` (~:177).
  - `UpdateSettings` UPDATE clause: add `offsite_drills_enabled = ?` (~:230) and bind `boolInt(s.OffsiteDrillsEnabled)` (~:260).
- [ ] Migration (migrate.go, highest is v53): append
  `{version: 54, name: "settings_offsite_drills_enabled", sql: "ALTER TABLE settings ADD COLUMN offsite_drills_enabled INTEGER NOT NULL DEFAULT 1;"}`.
  DEFAULT 1 (ON) preserves current behavior for upgraders + fresh installs.
- [ ] Test: after migrate, `GetSettings().OffsiteDrillsEnabled == true` (default); set false via `UpdateSettings`,
  read back false. Add to the existing settings round-trip test.
- [ ] Gates + commit `feat(store): OffsiteDrillsEnabled setting + migration (#37)`.

### Task 2 — Service/schedule/API: gate the scheduled DR + neutral scorecard + status flag
**Files:** `internal/schedule/schedule.go`, `internal/api/service.go`, `internal/api/handlers.go`,
`internal/schedule/schedule_test.go`, `internal/api/service_test.go`.

- [ ] `schedule.go` `drillTasks` (~:559-571): wrap ONLY the two `{containers,offsite,dr}` and `{flash,offsite,dr}`
  appends (~:564-568) in `if settings.OffsiteDrillsEnabled { ... }`. Leave the `enabledDrillDomains` `{local,subset}`
  loop untouched.
- [ ] `service.go` protection: change the `drillPeriod` guard (~:1171-1174) from `if settings.DrillsEnabled {` to
  `if settings.DrillsEnabled && settings.OffsiteDrillsEnabled {`. This is the ONLY protection-logic change — it
  makes `DrillState=""` (muted) and `protectionLevel` ignore DR (both branches are `drillPeriod>0`-guarded),
  reusing the proven global-drills-off neutral path. Do NOT edit protectionChecks/protectionLevel branch logic.
- [ ] `service.go` add `OffsiteDrillScheduled bool json:"offsiteDrillScheduled"` to `DomainStatusEntry` (~:826, near
  LastDRDrillOK), populated in the entry literal (~:1200-1223) as
  `settings.DrillsEnabled && settings.OffsiteDrillsEnabled && offsiteConfigured` (offsiteConfigured is already
  computed at ~:1133). The frontend pill uses this to show "manual only" vs red.
- [ ] `handlers.go`: add `OffsiteDrillsEnabled bool json:"offsiteDrillsEnabled"` to the `settingsView` DTO (~:870),
  copy it in `toView()` (~:929: `OffsiteDrillsEnabled: s.OffsiteDrillsEnabled`) and in the `handlePutSettings`
  `store.Settings{...}` literal (~:1109: `OffsiteDrillsEnabled: v.OffsiteDrillsEnabled`). (Bool needs no clamping.)
- [ ] Tests: (a) `schedule_test.go` — `drillTasks` with `OffsiteDrillsEnabled:false` omits the offsite/dr tasks but
  KEEPS the `{local,subset}` tasks; with true, includes them. (b) `service_test.go` — mirror
  `TestDomainStatusDrillCurrencyIgnoresDisabledDrills` with `OffsiteDrillsEnabled:false` (+ DrillsEnabled:true):
  a stale/failed DR drill yields `DrillState==""` and `Protection` not amber/red for DR; and assert
  `OffsiteDrillScheduled==false`. Confirm the existing v4.5.1 protection tests still pass.
- [ ] Gates + commit `feat(api): gate scheduled off-site DR drill + neutral scorecard + status flag (#37)`.

### Task 3 — i18n (before Task 4)
**Files:** `web/src/lib/i18n.ts` (en+de) + all 24 `web/src/lib/locales/*.ts`.

- [ ] Add these 4 keys to inline en+de AND all 24 locales (proper translation each, count-neutral):
  - `settings.offsiteDrills` = "Scheduled off-site DR drill"
  - `settings.offsiteDrillsHelp` = "Restores the full off-site snapshot on the drill schedule to prove remote
    recovery. This re-downloads the whole backup each run, which costs egress on metered clouds (e.g. Backblaze
    B2). Turn off to keep only the free local integrity check and run the off-site DR check manually."
  - `drill.manualOnly` = "Off-site DR: manual only"
  - `drill.manualOnlyTitle` = "Scheduled off-site DR drill is off. Run the off-site check manually with the button."
- [ ] Verify 26 hits/key + `tsc --noEmit` 0. Commit `i18n: off-site DR opt-out keys across all locales (#37)`.

### Task 4 — Frontend: Settings toggle + dashboard neutral pill + always-available Run-DR
**Files:** `web/src/lib/api.ts`, `web/src/pages/Settings.tsx`, `web/src/pages/Dashboard.tsx`.

- [ ] `api.ts`: add `offsiteDrillsEnabled: boolean;` to the `Settings` interface (~:125) and
  `offsiteDrillScheduled: boolean;` to the `DomainStatus` type (near lastDrDrillOK).
- [ ] `Settings.tsx`: in the "Automatic restore checks" card (advanced-gated, ~:1909-1964), add a `ToggleRow`
  bound to `settings.offsiteDrillsEnabled` (copy the DrillsEnabled ToggleRow ~:1915-1921), label
  `t("settings.offsiteDrills")` + help `t("settings.offsiteDrillsHelp")`, and add `offsiteDrillsEnabled` to that
  card's `save()` patch (~:1953). Disable/grey the sub-toggle when `!settings.drillsEnabled` (it only applies then).
- [ ] `Dashboard.tsx`: (1) where `drFailed` is computed (~:382) and the DR pill/detail render (~:429-473): when
  `d.offsiteConfigured && !d.offsiteDrillScheduled`, do NOT render the red drFailed pill/detail — render a MUTED
  "manual only" pill (`t("drill.manualOnly")`, title `t("drill.manualOnlyTitle")`, muted styling like the
  "no drill schedule" render, never red). (2) Still render the GREEN "proven restorable" badge when
  `d.lastDrDrillOK` (a real passed run, even manual, is honest proof). (3) Lift the "Run off-site DR check" button
  out of the drFailed-only block so it shows for ANY `d.offsiteConfigured` domain (manual runs always reachable);
  keep the inline `drRunError` red text for a live manual-run failure.
- [ ] Gates (`tsc --noEmit` + `npm run build`; restore `web/dist/index.html` if dirty). Commit
  `feat(web): off-site DR opt-out toggle + neutral "manual only" dashboard pill (#37)`.

## Self-review
Default true end-to-end (T1 migration + wiring). Neutral state reuses the proven DrillsEnabled-off path (single
guard change, T2) + the pill flag. Manual DR + local subset untouched. i18n before frontend. Covered.

## Handoff
Subagent-driven: T1 → T2 → T3 → T4. Then adversarial review (focus: default-true round-trip, protection neutral
not-red/not-false-green, manual path intact). Release **v4.8.0** (Minor). Reply #37 + close on confirm.
