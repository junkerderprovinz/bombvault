---
phase: 03-selection-trust-controls
verified_at: 2026-09-10T21:02:53Z
status: passed
score: 14/14 must-haves verified
behavior_unverified: 0 # every behavior-dependent truth (reset body/gate, queue serialization, union, argv emission, narrowing gate) is exercised by a passing behavioral test re-run first-hand at HEAD — no presence-only truths remain
overrides_applied: 0
unverified_prohibitions: 2 # judgment-tier reads (map keys never reach argv; no fanout/no snapshot comparison) — non-authoritative LLM-judge verdicts per ADR-550 D4, human review recommended (table below)
re_verification:
  previous_status: none
  previous_score: none
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification_needed:

  - test: "Visual UAT in a real browser: open the container panel FoldersEditor with a multi-mount container, verify the muted '{n} paths' line inside every root label (including '0 paths' on a deselected mount), expand an '{n} exclusions' disclosure (chevron rotation, mono ltr break-all relative rows with hover titles), the 'Skip cache folders (CACHEDIR.TAG)' switch row with its InfoBubble under every root, the Reset selection button with the fail-tone confirm dialog, and (after unchecking a mount on a backed-up container) the role=status text-statusWarn narrowing note under the tree"
    expected: "Preview/exclusions/toggle sub-rows sit on the indent-16 py-1 rhythm consistent with the Phase 2 rows; the switch label reads at the 12px register; the confirm dialog carries the destructive (bg-statusFailSolid) treatment; the note renders between the tree and the Add row"
    why_human: "jsdom asserts classes, roles, wire bodies and text only; rendered register, tone legibility and dialog feel need human eyes (the plans' own D5 human-judgment coverage item in 03-02-SUMMARY)"

  - test: "Real-instance smoke (optional, same class as Phase 1's item): on the Unraid instance, flip one mount's CACHEDIR switch, reload the panel (switch state persists from the mounts response), run a backup of that container, then confirm the run's restic argv carries --exclude-caches; flip it off and confirm the next backup's argv does not"
    expected: "PATCH persists the full map; mounts serves it back verbatim; the argv flag appears exactly when the stored union is true (scrubbed server log shows the flag right of the verb, left of --)"
    why_human: "Real host paths (FUSE), real SQLite persistence across restarts, real restic child — the mocked-api harness and Go unit fakes prove the contracts, not the deployment"

  - test: "Review the 2 judgment-tier prohibitions below (LLM-judge verdicts are non-authoritative)"
    expected: "A human confirms the holds verdicts or deposits corrections"
    why_human: "ADR-550 D4: judgment-tier prohibitions cannot be silently absorbed into a pass"

  - test: "Confirm the MVP-mode format decision: the ROADMAP marks Phase 3 'Mode: mvp' but the goal is not in User Story format (gsd user-story.validate -> false, re-run this session), so this verification is goal-backward against the 4 numbered Success Criteria (Phase 1 precedent)"
    expected: "Either accept that basis or run /gsd mvp-phase 3 to set a User Story goal and re-verify in MVP form"
    why_human: "Format decision belongs to the user"

  - test: "Decide the REQUIREMENTS.md INTEG-04 bookkeeping: the row still reads '[ ] ... UI semantics pending' and the traceability table 'Backend landed (Phase 1); UI semantics pending', although the documented UI semantics now exist and are verified (truths 7-9)"
    expected: "Flip the row to Complete at ship time (/gsd-ship or /gsd-docs-update) or leave it to the milestone close — pure metadata, no code impact"
    why_human: "Requirement-status bookkeeping is a developer decision; SELECT-03/INTEG-03/RESTIC-01 were already flipped during planning, INTEG-04 was not after execution"
---

# Phase 3: Selection Trust & Controls — Verification Report

**Phase Goal:** Users can see and control exactly what a selection will back up — per-mount effective-selection preview (including a narrowing note when a selection shrinks an item with existing snapshots), reviewable exclusions, defined semantics for deselecting everything, and a per-root CACHEDIR.TAG toggle.
**Verified:** 2026-09-10T21:02:53Z
**Status:** human_needed
**Re-verification:** No — initial verification
**Verified HEAD:** 90e4d5bd (includes review-fix commits 6aa9f24f, a5be53f2, daee9390, 4976fa07 and the post-fix dist rebuild f13963af — all part of the delivered phase)

## Verification Method

All claims re-derived from the current codebase (key files read in full or in the relevant ranges: `selectionTree.ts`, `SelectionTree.tsx`, `FoldersEditor` in `Containers.tsx`, `api.ts`, `targets.go`, `migrate.go`, `restic.go` BackupArgs, `service.go` SetExcludeCaches/union/Backup, handlers) and re-executed first-hand on this host (direct `node node_modules/...` invocations per the Windows caveat; `go` exported to PATH):

- Go, targeted first-hand at HEAD (`-count=1` unless cached-pass shown): `TestBackupArgs` family incl. **TestBackupArgsExcludeCaches (3 sub-tests) PASS**; `TestSetExcludeCachesRoundTripAndUpsertPreserves PASS`; `TestPatchContainerExcludeCaches (4 sub-tests) + TestContainerMountsExcludeCaches + TestBackupExcludeCachesUnion PASS`; `TestEmptySelectionGuard (5 sub-tests, the Phase 1 backstop) PASS`; `TestNoProductionCallerUsesUpdateSettings PASS`; `gofmt -l .` empty; `go build ./...` OK.
- Web, targeted first-hand at HEAD: phase suites + i18n (`Containers.tree.dom.test.tsx`, `selectionTree.test.ts`, `SelectionTree.dom.test.tsx`, `SelectionTree.keyboard.dom.test.tsx`, `i18n.parity/orphans/quality`) — **7 files / 235 tests, all pass** (6.5s). `tsc --noEmit` clean (re-run).
- Session evidence at the same HEAD (per verification context, re-confirmed where load-bearing): full `go test ./...` EXIT=0 (Windows-reduced suite; POSIX-only tests proven on CI Linux per repo convention), full web suite 91 files / **2217 tests** all passed (2214 after 03-03 + 3 review-fix tests — arithmetic consistent), `go vet` clean.
- `git log` confirms all 15 phase commits (3e33a956..27a29a55) + 4 review-fix commits + dist rebuild f13963af on docker-folders; working tree clean of tracked changes.

## Goal Achievement

### Observable Truths

Merged set: 4 ROADMAP success criteria (the contract) + plan must-have truths consolidated under them. Every behavior-dependent truth has a passing behavioral test re-run first-hand, so none are left PRESENT_BEHAVIOR_UNVERIFIED.

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | **SC1a** — every root row (each mount incl. unreachable, each standalone custom) renders a muted "{n} paths" preview line inside the root treeitem label, always rendered including "0 paths" | ✓ VERIFIED | `SelectionTree.tsx:277-283` (mounts) and `:303-307` (customs): `text-xs text-carbon-textMuted`, `t("folders.previewPaths").replace("{n}", rootIncludeCount(...))`, no condition on count; dom test "announces '{n} paths' inside every root row" pins included/deselected(0)/unreachable(0)/custom(1) |
| 2 | **SC1b** — the count derives only from the stored (includes) mirror via `rootIncludeCount` over `isAtOrUnder`, never from checked nodes or loaded children, never existence-filtered | ✓ VERIFIED | `selectionTree.ts:184-190` (helper walks the includes set only); unit tables at `selectionTree.test.ts:597-684` incl. "include ABOVE the root counts 0", segment-alignment sibling case, "existence-unfiltered BY DESIGN" pin; dom carve-out test asserts the count unchanged while a child exclusion is created |
| 3 | **SC1c** — the visible count equals the positional truth: per-root counts agree with `toFlatList` bare entries, which is what the next backup PATCH carries and restic receives | ✓ VERIFIED | Agreement test `selectionTree.test.ts:641-660` (sum over roots == bare entries == 4); the carve-out/narrowing dom test asserts the exact PATCH bodies (`[MOUNT, media, "!MOUNT/transcoding"]`) alongside the announced counts in the same timeline; Phase 1 engine contract (positionals = maximal-root includes) unchanged — `TestBackupArgs` family green at HEAD |
| 4 | **SC1d / SELECT-03 second half** — after a successful narrowing save on an item with existing snapshots, a role=status text-statusWarn note renders under the tree; never on widening, never when lastBackup is null, never for the reset save; queue-collapsed bursts net correctly; transient per editor session | ✓ VERIFIED | `Containers.tsx:1060-1069` (`attempted < lastSavedCountRef.current && lastBackup !== null` on the acknowledged attempt), `:1384-1388` (role="status" text-statusWarn placement under tree, above Add row); `lastSavedCountRef` seeded at load (`:917`) and threaded `lastBackup={container.lastBackup}` at `:2440`; 6 dom tests incl. the 5→3→4 collapsed-burst net and the close/reopen transient case — all in the 235-test first-hand run |
| 5 | **SC2a / INTEG-03** — under every root with at least one exclusion strictly below it a collapsible "{n} exclusions" section renders (button with aria-expanded/aria-controls + chevron); expanded body lists relative paths (root prefix stripped), mono ltr break-all with title, muted, lexically sorted, fully non-interactive | ✓ VERIFIED | `selectionTree.ts:202-212` (`rootExclusions`); `SelectionTree.tsx:554-606` (disclosure button + plain `ul`/`li`, no controls, WR-03 collision-free ids via `useId` + `encodeURIComponent` at `:422-430`); unit tables + 5 dom tests (disclosure/expand, zero-hidden, dormant, non-interactive, never-expanded completeness) |
| 6 | **SC2b / D-04** — a fully-deselected root with remembered exclusions renders all three signals together (unchecked checkbox, "0 paths", "{n} exclusions") and is never hidden; zero-exclusion roots render no section at all | ✓ VERIFIED | Same rendering path renders active and dormant roots identically (`:554-561` comment + code); dom tests "D-04 dormant case" and "renders no section at all for a root with zero exclusions" |
| 7 | **SC3a / INTEG-04 client half** — unchecking the last checked folder is blocked client-side before any request, and the zero-include block message references Reset selection while the hint no longer claims unticking everything reverts to the automatic default | ✓ VERIFIED | Phase 2 D-04 floor carried in `onToggle` (blockedPath + shake, no PATCH — pinned by the Phase 2 dom test still green in the suite); copy: `i18n.ts:741-742` emptySelectionBlocked ends "...use Reset selection.", `:720` hint ends "Unticking everything is blocked; use Reset selection to return to the automatic appdata default."; dom tests pin both texts and the absence of the old claim |
| 8 | **SC3b / D-05** — Reset selection is the explicit, documented exit: fail-tone confirm naming both consequences, one serialized PATCH whose body is `{backupPaths: [], excludeCaches: {}}` with NO selectionSource key, refetch-on-ok re-rendering the auto-detected selection (exclusions/custom/caches gone, lastSavedCount re-baselined), failure = verbatim toast + button shake with zero local mutation; WR-01 sticky-pending-reset and WR-02 serialized reload included | ✓ VERIFIED | `Containers.tsx:1281-1296` (confirm -> queue, never a direct fetch), `:1003-1023` (reset body composition, no source), `:1044-1058` + `:1117-1127` (reload flag on the queue tail), `:1429-1443` (button); dom tests: full reset flow (asserts `patches[0].opts` undefined and body equality), failed reset, toggle-stacked-behind-pending-reset (WR-01), refetch-waits-for-stacked-mutation (WR-02) |
| 9 | **SC3c backend guard** — the item never silently flips: the Phase 1 PATCH guard refuses tree-sourced empty saves over a non-empty stored selection; a no-source empty save (the reset shape) clears to auto-detection exactly as documented | ✓ VERIFIED | `service.go:3858-3874` (strictly `selectionSource == "tree"` gated, refusal before any store write); `TestEmptySelectionGuard` re-run PASS at HEAD with all 5 sub-cases incl. "legacy empty list still clears to auto-detection" and "tree-source empty list over a non-empty selection is refused" |
| 10 | **SC4a / RESTIC-01 UI** — every root carries a "Skip cache folders (CACHEDIR.TAG)" switch with caller-drawn label and InfoBubble scope tooltip, disabled on unreachable mounts and while in flight; optimistic flip, quiet success, full-map PATCH, failure reverts + verbatim toast + shake; switch state persists from the mounts response | ✓ VERIFIED | `SelectionTree.tsx:513-543` (Toggle hideLabel + label + InfoBubble, `disabled={spec.unreachable \|\| busy}`); `Containers.tsx:1305-1312` (onToggleCaches optimistic + queue), `:856-895` (caches mirror from `r.excludeCaches`); dom tests: rendering/disabled, caches-only body `{excludeCaches: {[MOUNT]: true}}`, stacked no-overlap timeline (`maxConcurrentPatches === 1`, asserted at 3 sites), failure revert/shake |
| 11 | **SC4b persistence** — migration v100 `target_exclude_caches` appends after v99 (no renumbering); `Target.ExcludeCaches` + five positional SQL sites + owned `SetExcludeCaches` (Upsert never resets the column); PATCH field (nil=untouched, {}=clear) with atomic whole-save containment validation + 64-entry cap; mounts always serves an object | ✓ VERIFIED | `migrate.go:1304-1315` (single `+` line in the phase diff — no other migration entry touched); `targets.go:36-41,125-207,525-546,601`; `handlers.go:1126-1133,1180-1184,1349-1360`; `service.go:10129-10156`; first-hand PASS: `TestSetExcludeCachesRoundTripAndUpsertPreserves`, `TestPatchContainerExcludeCaches` (4 sub-cases incl. atomic reject and decode reject), `TestContainerMountsExcludeCaches` |
| 12 | **SC4c argv mapping** — a backup of an item whose stored map holds >=1 true emits the constant `--exclude-caches` right of the backup verb, between the tag loop and the exclude loop, left of `--`; zero value byte-identical argv; union recomputed from the fresh UpsertTarget re-read, independent of selection inclusion (A1) | ✓ VERIFIED | `restic.go:391-399` (emission site + argv-discipline comment); `service.go:4187` (`mode.ExcludeCaches = anyRootExcludeCaches(tg.ExcludeCaches)` with the fresh-read comment); first-hand PASS: `TestBackupArgsExcludeCaches` (byte-exact presence + two absence pins) and `TestBackupExcludeCachesUnion` (A1 edge: toggled root deselected, flag still fires; all-false and cleared maps leave it off) |
| 13 | **T-03-07 serialization** — reset, toggles and caches flips all ride the one-deep composed queue: one fetchJSON per drain, never two concurrent container PATCHes from the editor, owed classes only | ✓ VERIFIED | `Containers.tsx:979-1037` (owedRef/pendingDescs composition into one `ContainerTargetsBody` via `setContainerTargets`); dom harness counts `maxConcurrentPatches` and pins `=== 1` across the mixed timelines (`:741, :806, :1097`) |
| 14 | **i18n + phase gate** — 7 new keys + 2 text changes present with placeholders intact and no em dashes across all 42 locale tables; web/dist rebuilt and committed after the review fixes so the embedded SPA carries the phase surfaces | ✓ VERIFIED | First-hand grep: each of previewPaths/exclusions/narrowedNote/resetSelection/resetConfirm/cachedirToggle/cachedirScope/emptySelectionBlocked in 41 files (i18n.ts en+de + 40 locales); the only em-dash hit near the new keys is a code comment (`i18n.ts:754`); parity/orphans/quality green in the 235-test run; `web/dist/index.html` committed in f13963af references `assets/index-Cs2G_CZh.js` which exists on disk and contains "Skip cache folders (CACHEDIR.TAG)" |

**Score:** 14/14 truths verified (0 present-behavior-unverified)

### Required Artifacts

All present, substantive (no stubs), wired, data flowing.

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/store/migrate.go` | migration v100 `target_exclude_caches` | ✓ VERIFIED | Appended after v99, numbering-hazard comments preserved, why-comment modeled on the v53 entry |
| `internal/store/targets.go` | `Target.ExcludeCaches` + 5 SQL sites + `SetExcludeCaches` | ✓ VERIFIED | INSERT-only upsert (ON CONFLICT omission), 3 SELECTs, scanTarget unmarshal; owned setter with Upsert fallback |
| `internal/restic/restic.go` | `Mode.ExcludeCaches` + BackupArgs emission | ✓ VERIFIED | Limits-precedent why-comment; flag pinned between tags and excludes |
| `internal/api/handlers.go` | PATCH `excludeCaches` field + mounts response key | ✓ VERIFIED | Non-pointer map (nil=untouched), nil-safe `{}` serving |
| `internal/api/service.go` | `SetExcludeCaches`, mounts 5th return, union into Mode | ✓ VERIFIED | Containment validation + 64 cap; union at the single Backup mode site from the fresh re-read |
| `web/src/lib/selectionTree.ts` | `rootIncludeCount` + `rootExclusions` pure helpers | ✓ VERIFIED | D-01/D-03/D-04 why-comments; table-tested incl. toFlatList agreement |
| `web/src/components/SelectionTree.tsx` | preview sub-line, exclusions disclosure/list, CACHEDIR toggle sub-row | ✓ VERIFIED | All three surfaces in the root-row walk; WR-03 ids; presentation wrappers keep treeitem/group shape |
| `web/src/pages/Containers.tsx` | SaveDesc classes, composed drain, reset, narrowing note, caches state | ✓ VERIFIED | Read in full at the relevant ranges (725-1150, 1270-1447); lastBackup threaded at 2440 |
| `web/src/lib/api.ts` | `ContainerMountsResponse.excludeCaches` + `setContainerTargets` | ✓ VERIFIED | `:882,905-919`; `setBackupPaths` deleted (grep confirms no callers), single composed-body PATCH helper |
| `web/src/lib/i18n.ts` + 40 locales | 7 keys + 2 text changes x 42 tables | ✓ VERIFIED | 41-file grep per key; parity/orphans/quality green |
| `web/dist` | rebuilt and committed (phase gate) | ✓ VERIFIED | f13963af updates the tracked index.html to the fresh asset hash; bundle grep carries the phase copy (resolves the Phase 2 deviation — the refresh-tracked-index practice was adopted here) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| PATCH body `excludeCaches` | `targets.exclude_caches` column | `Service.SetExcludeCaches` containment -> `store.SetExcludeCaches` UPDATE | ✓ WIRED | handlers.go:1180 -> service.go:10143 -> targets.go:529; atomic reject before any write (test-pinned) |
| `service.Backup` target re-read | argv flag | `anyRootExcludeCaches(tg.ExcludeCaches)` -> `mode.ExcludeCaches` -> `BackupArgs` | ✓ WIRED | service.go:4187 -> restic.go:397; proven end-to-end by `TestBackupExcludeCachesUnion` through the Mode-recording fake |
| `targets.exclude_caches` | SPA toggle state | scanTarget -> `ContainerMounts` 5th return -> mounts `excludeCaches` key -> `r.excludeCaches` apply | ✓ WIRED | targets.go:601 -> service.go:3793 -> handlers.go:1360 -> Containers.tsx:920; round-trip test-pinned |
| CACHEDIR switch | serialized PATCH | `onToggleCaches` -> `scheduleSave` (cls "caches") -> composed drain -> `setContainerTargets` | ✓ WIRED | One fetch per drain; `maxConcurrentPatches === 1` pinned |
| Reset descriptor | auto-detection state | queue drain `{backupPaths: [], excludeCaches: {}}` (no source) -> Phase 1 guard fall-through -> `setLoaded(false)` refetch | ✓ WIRED | Guard pass-by-omission pinned by `TestEmptySelectionGuard` legacy sub-case; refetch serialized on the queue tail |
| `container.lastBackup` | narrowing gate | ContainerRow prop -> FoldersEditor `lastBackup` -> attempted<lastSaved && !== null | ✓ WIRED | Containers.tsx:2440 -> 765/778 -> 1068 |
| Preview count | argv truth | `rootIncludeCount(root, includes)` <- mounts-served mirror; `toFlatList` -> PATCH -> `NormalizeSelection` -> maximal-root positionals | ✓ WIRED | Agreement test + body-pinned dom test + Phase 1 contract green at HEAD |

### Data-Flow Trace (Level 4)

Every rendered value chains to a real data source: preview counts and exclusion lists <- (includes, exclusions) state <- `getContainerMounts` response <- store `backupPaths`; switch state <- `excludeCaches` state <- mounts response <- `targets.exclude_caches`; narrowing note <- `lastSavedCountRef` (load-time mirror + acknowledged attempts) and `container.lastBackup` (existing wire field); argv flag <- stored map union. No static returns, hardcoded lists, or mocks in production paths. ✓ FLOWING everywhere.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| argv flag presence/absence/position | `go test ./internal/restic/ -run TestBackupArgs -v` | all PASS incl. TestBackupArgsExcludeCaches (3 sub-tests, byte-exact) | ✓ PASS |
| store round-trip / Upsert preserves | `go test ./internal/store/ -run TestSetExcludeCaches -v -count=1` | PASS | ✓ PASS |
| PATCH/mounts boundary + backup union | `go test ./internal/api/ -run 'ExcludeCaches' -v -count=1` | 6 sub-tests PASS (accept/atomic-reject/decode/absent/serve/union A1) | ✓ PASS |
| Empty-selection guard (reset backstop) | `go test ./internal/api/ -run TestEmptySelectionGuard -v -count=1` | 5 sub-tests PASS | ✓ PASS |
| MutateSettings prohibition | `go test ./internal/store/ -run TestNoProductionCallerUsesUpdateSettings -v -count=1` | PASS | ✓ PASS |
| Phase web suites + i18n | `node node_modules/vitest/vitest.mjs run <7 phase files>` | 7 files / 235 tests passed | ✓ PASS |
| Strict typecheck | `node node_modules/typescript/bin/tsc --noEmit` | clean | ✓ PASS |
| Go chain at HEAD | `gofmt -l .` (empty), `go build ./...` | clean / OK (full `go test ./...` EXIT=0 this session; vet clean) | ✓ PASS |

### Probe Execution

Step 7c: SKIPPED — no probes declared in PLAN/SUMMARY and no `scripts/*/tests/probe-*.sh` exist in the repo.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| SELECT-03 | 03-02, 03-03 | Effective-selection preview "N paths" matching the restic positionals + narrowing communication | ✓ SATISFIED | Truths 1-4 |
| INTEG-03 | 03-02 | Exclusions reviewable after the fact, visible list near the mount, consistent styling | ✓ SATISFIED | Truths 5-6 |
| INTEG-04 | 01-04 (backend), 03-02/03-03 (UI) | Defined, UI-documented empty-deselect semantics; never a silent flip to auto-detection | ✓ SATISFIED (code) | Truths 7-9; NOTE: REQUIREMENTS.md row still unchecked/"UI semantics pending" — bookkeeping only, human item 5 |
| RESTIC-01 | 03-01, 03-03 | Per-mount/root CACHEDIR.TAG toggle maps to restic `--exclude-caches` in BackupArgs, covered by restic_args_test.go | ✓ SATISFIED | Truths 10-12 (TestBackupArgsExcludeCaches is exactly the required cover) |

Orphaned requirements: none — REQUIREMENTS.md maps exactly these 4 IDs to Phase 3 and all are claimed by plans.

### Review Fix Verification (03-REVIEW.md -> 03-REVIEW-FIX.md)

All 4 warning-tier findings verified fixed in the current code, not just claimed:

| Finding | Fix in code | Test pin | Status |
|---------|-------------|----------|--------|
| WR-01 confirmed reset silently dropped when stacked | `scheduleSave` sticky pending-reset (`Containers.tsx:962`) + split-window contract comments | dom test "a toggle stacked behind a PENDING reset never displaces it" | ✓ VERIFIED |
| WR-02 post-reset refetch races stacked mutations | `queueRef.reload` flag consumed in the finally chain only when no drain owed (`:1117-1127`), `chained` captured before the recursive call | dom test "the post-reset refetch waits for a mutation stacked during the reset PATCH's flight" | ✓ VERIFIED |
| WR-03 exclusions-disclosure DOM id collisions | `useId()` prefix + `encodeURIComponent(spec.path)` (`SelectionTree.tsx:422-430`) | dom test rendering two colliding roots, distinct aria-controls | ✓ VERIFIED |
| WR-04 reset leaves orphaned excludeCaches keys | reset drain composes `excludeCaches: {}` (`Containers.tsx:1019`); confirm copy names the caches consequence (42 locales) | dom test models the orphan scenario, pins the composed body and post-reset no-switch-ON | ✓ VERIFIED |

IN-01..IN-04 (info-tier) remain documented and deliberately skipped (fix_scope=critical_warning) — acceptable per the review itself; see Anti-Patterns.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/api/service.go | 7540 | `TODO: parse virsh dominfo output` | ℹ️ Info | Pre-existing (blamed 154cf374, 2026-06-10 — long predates the phase) |
| web/src/pages/Containers.tsx | 331 | `TODO(#follow-up)` stake-detail copy | ℹ️ Info | Pre-existing (b98c992f, 2026-08-19; flagged in the Phase 2 verification already) |
| web/src/pages/Containers.tsx | queue | stacked-descriptor failure-revert window | ℹ️ Info | Pre-existing plan-02 behavior; logged in deferred-items.md, needs a GSD decision |
| .planning/REQUIREMENTS.md | 38/106 | INTEG-04 row stale ("UI semantics pending") | ℹ️ Info | Docs bookkeeping; human item 5 |
| SelectionTree.tsx | 282 | per-row counts use at-or-under, so an unreachable mount with a standalone sub-include custom row both announce that include | ℹ️ Info | By-design derivation (pinned unit semantics); each row's number stays individually true, only cross-row sums can double-announce in this rare edge |

No TBD/FIXME/XXX introduced by this phase (grep over all phase-modified files: zero hits). No disabled tests, no stub implementations, no placeholder copy in the new surfaces.

### Prohibitions

Test-backed verdicts are authoritative (named tests re-run PASS); the last two are judgment-tier LLM reads — non-authoritative, human review recommended.

| # | Prohibition | Verdict | Evidence |
|---|-------------|---------|----------|
| 1 | Per-container state never in the settings row / never via MutateSettings | holds (test) | `SetExcludeCaches` is a direct targets UPDATE; `TestNoProductionCallerUsesUpdateSettings` PASS |
| 2 | No shipped migration edited or renumbered; v100 appends after v99 | holds (test) | Phase diff shows exactly one `+version: 100` line; v99 intact |
| 3 | Flag never left of the backup verb or after `--` | holds (test) | `TestBackupArgsExcludeCaches` byte-exact argv pin |
| 4 | No user-controlled string reaches argv through this feature | holds (code-read; argv test pins the bare constant) | Only the boolean union reaches Mode; keys validated and dropped (`service.go:10143-10156`, `restic.go:397`) |
| 5 | Preview never computed from checked nodes or loaded children | holds (test) | Helper signature + collapsed/never-loaded dom tests |
| 6 | Exclusions list never interactive | holds (test) | Non-interactive dom test (no controls, rows not focusable) |
| 7 | Fully-deselected root / dormant exclusions never hidden | holds (test) | D-04 dormant dom test |
| 8 | No singular/plural copy fork, no em dashes | holds (test) | Single `{n}` keys; i18n quality/parity green; grep clean |
| 9 | Reset never sent outside the serialized queue | holds (test) | Reset rides scheduleSave only; single-serialized-fetch dom pin |
| 10 | Toggle never ships without the scope tooltip | holds (test) | InfoBubble content asserted beside every enabled switch |
| 11 | Two container PATCHes from one editor never overlap | holds (test) | `maxConcurrentPatches === 1` at 3 assertion sites |
| 12 | No fanout into the ExcludesEditor, no snapshot-content comparison | holds (judgment) | Exclusions list carries no controls; narrowing trigger reads only include counts + lastBackup (code read) |

### Human Verification Required

See frontmatter `human_verification_needed` — 5 items: (1) real-browser visual UAT of the five new surfaces, (2) optional real-instance CACHEDIR smoke through a real backup argv, (3) judgment-tier prohibition review (items 4 and 12), (4) the MVP-mode format confirmation (goal is not User Story format; verified against the 4 numbered SCs per Phase 1 precedent — `gsd user-story.validate` returns false), (5) the REQUIREMENTS.md INTEG-04 bookkeeping decision.

### Gaps Summary

No structural gaps. All 4 ROADMAP success criteria are verified against the current codebase with first-hand test executions (Go: argv/store/boundary/union/guard/prohibition tests all PASS; web: 235 phase-targeted tests, tsc, plus the session's full-suite 2217/2217), the review's four warnings are confirmed fixed in code with pinning tests, the reset's composed body deviation from the plan wording (`excludeCaches: {}` added by WR-04) is a documented review-driven improvement that preserves the load-bearing no-selectionSource shape, and web/dist was rebuilt and committed after the fixes (resolving the Phase 2 dist deviation by adopting the refresh-tracked-index practice). The status is human_needed solely for the human surface above — chiefly real-browser visual UAT of surfaces jsdom cannot observe, plus two small bookkeeping decisions.

---

_Verified: 2026-09-10T21:02:53Z_
_Verifier: Claude (gsd-verifier)_
