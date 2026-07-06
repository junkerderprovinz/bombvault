# Surface WHY/WHICH drill failed + let a manual run clear it (#30 reopened) — plan

> REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use `- [ ]`.

#30 (manilx) reopened: after v4.5.1 the scheduled OFF-SITE DR drill still fails while a manual LOCAL subset
check passes, and the dashboard red never explains WHY or WHICH check. Root cause (verified): the scheduled job
runs `{offsite,dr}` (fails) AND the default manual button runs `{local,subset}` (passes) — two different drills
wired to different store rows; the red dashboard row reads ONLY the `offsite/dr` row, so a manual local pass can
never clear it. The failure reason IS stored in `restore_drills.detail` but is never carried to the dashboard.
Also the scheduled 12h-lock-timeout skip records NO row, freezing the red with no reason.

This change makes the bug **self-diagnosing** (show the reason + which check on the dashboard/settings) and lets
a manual run of the failing check clear the red. It does NOT speculatively fix the underlying off-site DR failure
— that is DEFERRED pending manilx's recorded `detail` (do not guess; this was the second miss).

## Global Constraints
- Branch `fix/drill-surface-and-manual` (off main == v4.6.0). Go gates: build/vet, `gofmt -l` empty,
  `go test ./internal/... -count=1`, `golangci-lint run ./internal/...` 0. Frontend: `tsc --noEmit` + `npm run build`.
- No AI attribution. i18n before frontend (en is a full typed map → missing key fails tsc).

---

### Task 1 — Backend: carry the drill reason into /api/status + record the busy-skip
**Files:** `internal/api/service.go`, `web/src/lib/api.ts`, `internal/api/service_test.go` (or an internal test).

- [ ] Add two fields to `DomainStatusEntry` (service.go:798-839, after `LastVerifiedOK` / near `LastDRDrillOK`):
  ```go
  VerifiedDetail string `json:"verifiedDetail"` // scrubbed reason of the last LOCAL subset drill (empty on success)
  DrillDetail    string `json:"drillDetail"`    // scrubbed reason of the last OFF-SITE DR drill (empty on success)
  ```
- [ ] In `buildStatus` (service.go ~:1118-1157): hoist the detail out of the two `if ... found` blocks. In the
  local block (:1125) add `var verifiedDetail string` and set `verifiedDetail = drill.Detail`. In the DR block
  (:1154) add `var drDetail string` and set `drDetail = dr.Detail`. Populate the entry (service.go:1207-1216):
  `VerifiedDetail: verifiedDetail,` and `DrillDetail: drDetail,`. (Detail is "" on success, so carrying it
  unconditionally is safe.)
- [ ] Record the scheduled busy/lock-timeout SKIP instead of silently returning. In `RunRestoreDrill` at the
  `wait` branch skip (service.go ~:5042-5043, where `waitLockDomainFor` fails and it `return
  store.RestoreDrill{}, errDomainBusy`): first read the real `AddRestoreDrill` signature + `store.RestoreDrill`
  fields (internal/store/drills.go), then write a dated failed row and notify, e.g.:
  ```go
  skip := store.RestoreDrill{Domain: domain, Source: source, Kind: kind, At: time.Now().Unix(), OK: false,
      Detail: "skipped: repository busy longer than " + drillLockWait.String() + " (a backup or off-site copy held it)"}
  if _, aErr := s.store.AddRestoreDrill(skip); aErr != nil { log.Printf("api: drill: record busy-skip for %q: %v", domain, aErr) } //nolint:gosec // G706
  s.notifyDrillFailure(ctx, domain, source, skip.Detail)
  return skip, errDomainBusy
  ```
  Match the actual `AddRestoreDrill` arg shape (it may take fields, not a struct — adapt). Do NOT touch the
  manual `tryLockDomainFor` busy path (that is a user-initiated immediate retry, correctly silent).
- [ ] Mirror the two fields into the TS `DomainStatus` interface (web/src/lib/api.ts ~:181-200):
  `verifiedDetail: string;` and `drillDetail: string;`.
- [ ] Test: build a status with a failed off-site DR drill row carrying a `Detail` and assert the entry's
  `DrillDetail` equals it; assert the busy-skip path writes a `RestoreDrill` row (or, if hard to unit-test the
  lock path, at least a store round-trip that `AddRestoreDrill` + `LatestRestoreDrillKind` carry `Detail`).
- [ ] Gates + commit `feat(api): surface the drill failure reason in /api/status + record scheduled busy-skips (#30)`.

### Task 2 — i18n: labels for the two checks + the reason (do BEFORE Task 3)
**Files:** `web/src/lib/i18n.ts` (inline en+de) + all 24 `web/src/lib/locales/*.ts`.

- [ ] Add keys (proper translation per locale; one writer, all files this pass):
  - `drill.checkOffsiteDr` = "off-site DR restore"
  - `drill.checkLocal` = "local integrity check"
  - `drill.failReasonPrefix` = "reason:"  (shown before the scrubbed detail)
  - `drill.runOffsiteDr` = "Run off-site DR check"
  - `drill.runningOffsiteDr` = "Running off-site DR check…"
- [ ] Verify each of the 24 locales + inline en/de has all 5 keys; `tsc --noEmit` 0. Commit
  `i18n: drill reason + check labels across all locales (#30)`.

### Task 3 — Frontend: show the reason + which check, and let a manual DR run clear the red
**Files:** `web/src/pages/Dashboard.tsx`, `web/src/pages/Settings.tsx`.

- [ ] Dashboard drill row (Dashboard.tsx ~:493-507, the `bad`/failed case ~:500) and the red "proven restorable
  off-site" pill (~:373-382): label it `t("drill.checkOffsiteDr")` and, when `d.drillDetail` is non-empty,
  render the reason (`t("drill.failReasonPrefix") + " " + d.drillDetail`) inline (small, muted-red) and/or as the
  element `title` tooltip. Read how the row/pill are built and follow the file's styling.
- [ ] Add a **Run off-site DR check** affordance where the red DR state shows (dashboard failing row/pill or the
  domain card): a button `t("drill.runOffsiteDr")` that calls the existing `runDrill(domain, "offsite", "dr")`
  (web/src/lib/api.ts) and, on completion, refetches status (dispatch `bv:settings-changed` or the page's status
  refetch) so a pass clears the red. Show `t("drill.runningOffsiteDr")` + disabled while in flight, and on
  failure surface the returned `drill.detail`. (This makes a MANUAL run able to clear the dashboard, manilx ask #1.)
- [ ] Settings drill panel: on the IDLE "last recorded drill" line (Settings.tsx ~:1119-1120) render the stored
  `drill.detail` on failure (not only right after a manual run), and label WHICH check via source+kind
  (`t("drill.checkOffsiteDr")` when source==="offsite"&&kind==="dr", else `t("drill.checkLocal")`).
- [ ] Gates (`tsc --noEmit` + `npm run build`; restore `web/dist/index.html` if dirty). Commit
  `feat(web): show drill failure reason + which check, add manual off-site DR run from the dashboard (#30)`.

## Deferred (NOT in this change)
- The actual OFF-SITE DR failure fix — needs manilx's recorded `detail` (remote lock vs timeout vs strict
  byte/file-count mismatch). Once his reason is known, branch the fix. Do NOT guess.

## Handoff
Subagent-driven: T1 backend → T2 i18n → T3 frontend. Then adversarial review of the diff, release **v4.7.0**
(Minor — new status fields + UI), update vault/daily. Keep #30 OPEN (real DR fix still pending manilx's error).
