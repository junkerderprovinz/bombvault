---
phase: 02-container-panel-tree-selection
plan: "02"
subsystem: web/spa-container-panel
tags: [selection-tree, tree-outcome-rows, patch-queue, d-04-guard, dom-tests]
requires:
  - "02-01 tracer slice: selectionTree.ts (I,E) arithmetic, SelectionTree.tsx row rendering, FoldersEditor live-save wiring, the 4 folders.* i18n keys"
  - "Phase 1 browse contract: status trio + truncated flag on the wire; coded empty-selection envelope"
provides:
  - "Pinned TREE-06/D-06 per-node outcome rendering: couldNotRead fallback, retry re-entry through a real refetch, truncated notice row outside the treeitem structure and setsize arithmetic, loading/empty/no-access trio distinct, rejected promises settle with zero spinners, listing failures never block selection"
  - "FoldersEditor one-deep serialized PATCH queue: dirty-flag burst collapse, send-latest-once drain, failure revert re-derived from the live mirror by set-difference inverse (newer toggles always survive), busy/shake keyed per row"
  - "Full D-04 treatment: whole-item include counting against the ref mirror, glim-shake replay via nonce-in-key, text-xs text-statusWarn inline line; coded empty-selection envelope handled defensively (verbatim toast + revert)"
affects:
  - web/src/pages/Containers.tsx
  - web/src/components/SelectionTree.dom.test.tsx
  - Phase 4 File Sets (reuses SelectionTree)
tech-stack:
  added: []
  patterns:
    - "serialized one-deep save queue over a ref mirror (queueRef inFlight/dirty + pendingDesc/pendingRows) — batch-safe rapid toggles, latest-wins drain"
    - "failure revert by set-difference inverse onto the LIVE mirror instead of captured snapshots (Pitfall 5 pattern)"
key-files:
  created: []
  modified:
    - web/src/components/SelectionTree.dom.test.tsx
    - web/src/pages/Containers.tsx
key-decisions:
  - "The queue reads the mirror through a ref, never a state closure: two rapid toggles inside one React batch must each see their predecessor's effect"
  - "Failure revert un-applies the failed mutation's set-difference delta onto the live mirror (applyToggle's inverse), so a toggle stacked behind a failing save survives; a captured snapshot is the Pitfall 5 bug"
  - "The drain sends the LIVE mirror at attempt start, not the initiating desc's sets — the desc is only the failure-revert recipe"
  - "Custom add/remove ride the same queue as structural saves (toast-only failure preserved) so no two backupPaths PATCHes from this editor are ever concurrent"
  - "Task 1 rows landed with plan 01's documented deviations; this plan's tests are the pin the plan asked for, mutation-verified (fallback/truncated mutations fail 5 of the new cases)"
patterns-established:
  - "Serialized queue + ref-mirror for any future live-save editor that can burst (Phase 4 File Sets editor will face the same shape)"
requirements-completed: [TREE-06, TREE-02, INTEG-01]
coverage:
  - id: D1
    description: "Per-node outcome rows (TREE-06/D-06): couldNotRead fallback, retry re-entry + refetch, truncated notice outside treeitem/setsize, trio distinct, rejected promise settles with zero spinners, listing errors never block a toggle's PATCH"
    requirement: TREE-06
    verification:
      - kind: unit
        ref: "web/src/components/SelectionTree.dom.test.tsx (describe 'SelectionTree per-node outcome rows (TREE-06/D-06, plan 02)', 6 tests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "One-deep serialized PATCH queue: burst collapse to one drain, ordered bodies, failing first save never clobbers the second toggle (revert re-derived from the live mirror), no await-per-toggle path remains"
    requirement: INTEG-01
    verification:
      - kind: unit
        ref: "web/src/components/SelectionTree.dom.test.tsx (describe 'FoldersEditor serialized save queue and D-04 guard (plan 02)', serialize + burst cases)"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-04 pre-PATCH guard: zero-include toggle blocked before any request, row shake + text-xs text-statusWarn line, whole-item counting (custom include keeps a mount uncheck legal), coded empty-selection backstop toasts verbatim and reverts"
    requirement: TREE-02
    verification:
      - kind: unit
        ref: "web/src/components/SelectionTree.dom.test.tsx (blocked-shake, custom-include-present, backstop cases + plan-01 D-04 block case)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Visual adequacy of the new row states (muted no-access row + compact retry, truncated notice placement, warn line under the blocked row) in a real browser"
    verification: []
    human_judgment: true
    rationale: "jsdom asserts state, roles, and wire contracts only; muted-tone rendering and inline layout need human eyes at end-of-phase UAT"
metrics:
  duration: 10m
  completed: 2026-09-10
status: complete
actuals:
  tokens: 7700    # chars/4 over the realized 2-file diff (30653 chars); plan estimate 26000 — small because the row variants landed with plan 01's deviations
  tasks: 2
  commits: 3
---

# Phase 02 Plan 02: Node Outcome States + Serialized Save Flow Summary

**TREE-06/D-06 outcome rows pinned by dom tests (retry refetch, truncated notice outside the tree, rejected promises settle) and the FoldersEditor save flow hardened with a one-deep serialized PATCH queue whose failure revert re-derives from the live mirror, plus the full D-04 zero-include block (shake + warn line, whole-item counting, coded-envelope backstop).**

## Performance

- **Duration:** 10 min
- **Started:** 2026-09-10T14:17:59Z
- **Completed:** 2026-09-10T14:27:30Z
- **Tasks:** 2
- **Files modified:** 2 (`web/src/components/SelectionTree.dom.test.tsx`, `web/src/pages/Containers.tsx`)

## Accomplishments

- Every browse outcome now has a pinned, distinct render: restricted/missing/error show the scrubbed server text verbatim (or the `folder.couldNotRead` fallback) in a muted non-treeitem row with a neutral "Try again" Button that re-enters the spinner state through a genuine refetch (cache eviction proven by a second browse call); `truncated:true` renders the served children then exactly one non-interactive `folders.truncatedList` `<p>` excluded from `aria-setsize`; ok-empty renders `folder.none`; rejected promises settle with zero `.animate-spin` nodes in the document.
- The FoldersEditor save path is a one-deep serialized queue (RESEARCH Open Question 4): toggles during an in-flight save update the ref mirror and mark dirty; the drain on resolve sends the latest full flat list once; bursts collapse to one draining PATCH per in-flight save (T-02-08).
- Failure revert is re-derived — the failed mutation's set-difference inverse applied to the LIVE mirror — so a newer toggle always survives a slow or failing save (Pitfall 5 closed, pinned by the two-rapid-toggles-first-reply-fails dom test asserting the final mirror AND final PATCH body).
- D-04 complete: the zero-include toggle is blocked client-side before any request (no network call, mirror untouched, `glim-shake` replay via the nonce-in-key technique, `text-xs text-statusWarn` inline line naming Include in schedule); the count spans the whole item (a custom include keeps a last-mount uncheck legal); the server's `code:"empty-selection"` envelope is handled defensively — verbatim toast + revert — and stays unreachable.

## Task Commits

Each task was committed atomically:

1. **Task 1: TREE-06 + D-06 outcome rows pinned** - `2356de7d` (test)
2. **Task 2: serialized queue + D-04 extensions** - `a55ac49d` (test, RED: the two queue cases fail against the await-per-toggle path) → `3f35f872` (feat, GREEN)

**Plan metadata:** (this SUMMARY + state updates)

## Files Created/Modified

- `web/src/components/SelectionTree.dom.test.tsx` - +11 tests across two new describes (6 outcome-row pins, 5 queue/guard cases); harness gained `patchReplies` deferred-reply control over `setBackupPaths` and a `deferred()` helper
- `web/src/pages/Containers.tsx` - FoldersEditor: `SaveDesc` + queue refs (`mirrorRef`/`queueRef`/`pendingDescRef`/`pendingRowsRef`), `applyMirror` single mirror-write helper, `scheduleSave`/`attemptSave`/`revertFrom` replacing `persist`, sync ref-mirror `onToggle`, custom add/remove routed through the queue as structural saves

`web/src/components/SelectionTree.tsx` needed no changes — every row variant it renders landed with plan 01's documented deviations; this plan pinned them.

## Decisions Made

- **Ref mirror over state closures** — `onToggle` and the queue read `mirrorRef.current`, because two rapid toggles inside one React batch would otherwise see stale `includes` state and lose the first toggle's effect.
- **Drain sends the live mirror**, not the initiating toggle's sets; the desc (`pre`/`sent`/`node`) exists purely as the failure-revert recipe for whichever mutation started the attempt (the latest dirty one, for a drain).
- **Structural saves share the queue** — custom add/remove keep their toast-only failure path (no checkbox to restore) but now serialize behind/ahead of toggles, closing the last concurrent-PATCH race on this endpoint from this editor.
- **No vite build committed** — repo convention (`.gitignore` comment + plan-01 precedent): real dist artifacts stay ignored, the placeholder `web/dist/index.html` is the only tracked file and is untouched; CI's Docker web stage runs the real build. The production bundle was still verified locally (vite build exits 0; placeholder restored).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Custom add/remove routed through the serialized queue**
- **Found during:** Task 2 (queue implementation)
- **Issue:** The plan's queue text covers tree toggles; leaving `addCustom`/`removeCustomPath` on a direct fire-and-forget PATCH would have kept a concurrent-PATCH race on the same endpoint from the same editor — exactly the interleaving the queue exists to close (T-02-08, Pitfall 5).
- **Fix:** Both structural mutations call `scheduleSave` with `structural: true` (toast-only failure, no revert/shake — their historical semantics), so every `backupPaths` PATCH from this editor is serialized.
- **Files modified:** web/src/pages/Containers.tsx
- **Verification:** Full suite green (2155 tests); existing custom-path behavior unchanged (no test asserted the old concurrent shape).
- **Committed in:** 3f35f872 (part of the Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Contained hardening within the named files; no scope creep, no UI-visible change to custom-path handling.

## TDD Gate Compliance

Both tasks carried `tdd="true"`.

- **Task 2 followed full RED→GREEN:** `test(02-02)` commit `a55ac49d` contains the failing queue cases (verified failing against the await-per-toggle path — the burst case fails at "expected 2 to be 1" because the second PATCH fires concurrently today); `feat(02-02)` commit `3f35f872` turns them green. No REFACTOR commit needed — the GREEN implementation passed as written.
- **Task 1 could not produce a RED:** every behavior it pins landed with plan 01's checker-accepted deviations 1-2 (the plan itself anticipates this: "verify that holds"). The tests were committed as pins and **mutation-verified** — swapping the error fallback to `folder.none` text and disabling the truncated row fail 5 of the new cases, proving the pins bite. Also green-on-arrival in Task 2's RED commit: the blocked-shake, custom-include, and backstop cases (the minimal D-04 block from plan 01 already satisfies them; they pin the full contract).

## Issues Encountered

None. (Windows-host note, carried from plan 01: `npx`/`npm run` cmd shims are broken here, so `tsc`/`vitest`/`eslint` run via direct `node node_modules/...` invocations — equivalent commands, a tooling quirk, not a repo issue.)

## Interim Window Closed

The single window the plan-01 SUMMARY left open — rapid toggles using the simple await-per-save path (concurrent saves, snapshot reverts) — is closed by `3f35f872`: re-verified that a blocked last-include uncheck produces no network call, and the two-rapid-toggle cases pass.

## Verification

- `tsc --noEmit` — clean.
- `vitest run src/components/SelectionTree.dom.test.tsx` — 19/19 (8 from plan 01 unmodified + 11 new).
- Full suite — **2155 tests / 89 files passed** (was 2144; +11, 0 failures), including i18n parity across all 42 locales.
- `eslint src` — 0 errors; the 2 warnings (ActivityLog.tsx useMemo, Sidebar.tsx ref-cleanup) are pre-existing and out of scope.
- `vite build` — exits 0 (chunk-size warning pre-existing); tracked `web/dist/index.html` placeholder restored untouched per repo convention.
- Acceptance criteria walked item-by-item: status-color classes appear only on text spans (never checkbox/chevron/retry); notice rows carry no checkbox and no treeitem role; no await-per-toggle save path remains.

## Known Stubs

None.

## Next Phase Readiness

- Plan 02 closed the plan-01 interim window; Phase 2 success criterion 5 delivered in full.
- Ready for `02-03` (keyboard/a11y + geometry: full APG key map, `SelectionTree.keyboard.dom.test.tsx`, setsize/posinset polish — the notice rows are already structurally excluded from counts).
- INTEG-01 is also declared by `02-03` (shared-ID gate): it is already marked Complete from plan 01, so no gate action was needed beyond TREE-06/TREE-02.

## Self-Check: PASSED

- Commits found in git log: 2356de7d (test 02-02), a55ac49d (test 02-02 RED), 3f35f872 (feat 02-02).
- Modified files present on disk: web/src/components/SelectionTree.dom.test.tsx, web/src/pages/Containers.tsx.
- Full-suite verification green after the final commit (2155 passed / 89 files).

---
*Phase: 02-container-panel-tree-selection*
*Completed: 2026-09-10*
