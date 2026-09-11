---
phase: "02"
plan: "01"
subsystem: web/spa-container-panel
tags: [selection-tree, tri-state-checkbox, containers, i18n, lazy-loading]
requires:
  - "Phase 1 selection.go flat-list encoding (bare host paths = includes, '!'-prefixed = exclusions, canonical order, orphan exclusions preserved)"
  - "GET /api/browse serving browse-relative paths under the host source root (maxBrowseEntries=500, status/truncated on the wire)"
  - "GET /api/containers/{name}/mounts carrying selected/reachable/custom/excluded"
provides:
  - "web/src/lib/selectionTree.ts — pure (I,E) list arithmetic: EXCLUSION_PREFIX, isAtOrUnder/isStrictlyUnder (segment-aligned), classifyNode, applyToggle (D-01 remembered-partial cycle), splitFlatSet/toFlatList (canonical order), hostToBrowseRel/browseRelToHost, loadExpanded/saveExpanded (bv-tree-expanded-{name}, cap 64)"
  - "web/src/components/SelectionTree.tsx — reusable role=tree (components/, reusable for Phase 4 File Sets): tri-state checkboxes (aria-checked true/mixed/false), lazy fetch-on-expand through the editor-lifetime promise cache, loading/empty/error/truncated child rows, D-05 expansion persistence"
  - "FoldersEditor evolved per D-02: the mounts/custom list IS the tree; (includes, exclusions) mirror derived from the mounts response; every toggle optimistically applies and PATCHes toFlatList with selectionSource 'tree'"
  - "4 new folders.* i18n keys (treeLabel, truncatedList, retry, emptySelectionBlocked) translated in en/de inline tables plus all 40 locale files"
affects:
  - web/src/pages/Containers.tsx
  - web/src/lib/api.ts
tech-stack:
  added: []
  patterns:
    - "pure-module client mirror of selection.go invariants (lib/selectionTree.ts), table-tested against the Go contract"
    - "editor-lifetime promise cache (useRef Map owned by FoldersEditor, passed down) — rejections and ok:false responses evicted so retry refetches"
    - "DOM indeterminate property via callback ref (the app's first tri-state checkbox); roving tabindex on treeitems"
key-files:
  created:
    - web/src/lib/selectionTree.ts
    - web/src/lib/selectionTree.test.ts
    - web/src/components/SelectionTree.tsx
    - web/src/components/SelectionTree.dom.test.tsx
  modified:
    - web/src/pages/Containers.tsx
    - web/src/lib/api.ts
    - web/src/lib/i18n.ts
    - web/src/lib/locales/*.ts (40 locale files)
decisions:
  - "Node states derive purely from the (includes, exclusions) host-path sets via classifyNode — never from loaded children — so collapsed and never-loaded subtrees classify correctly (TREE-04)"
  - "The exclusion list below a node IS the D-01 remembered-partial memory: unchecking a parent keeps strictly-below exclusions stored dormant; there is no second UI-side memory"
  - "browseCache lives for the editor (panel) lifetime, not the page: survives section close/reopen, dies with the page; failed or refused reads are evicted so 'Try again' genuinely refetches"
requirements-completed: [TREE-01, TREE-02, TREE-03, TREE-04, INTEG-01]
coverage:
  - id: D1
    description: "Pure (I,E) selection arithmetic — classifyNode, applyToggle D-01 remembered-partial cycle, flat-set round-trip, path translation, capped expansion persistence"
    requirement: TREE-02
    verification:
      - kind: unit
        ref: web/src/lib/selectionTree.test.ts
        status: pass
    human_judgment: false
  - id: D2
    description: "SelectionTree role=tree component — lazy fetch-on-expand (TREE-01), editor-lifetime cache, loading/empty/error/truncated rows (TREE-06), tri-state aria-checked"
    requirement: TREE-01
    verification:
      - kind: unit
        ref: web/src/components/SelectionTree.dom.test.tsx
        status: pass
    human_judgment: false
  - id: D3
    description: "FoldersEditor tree integration — live-save PATCH wire contract with selectionSource 'tree', D-04 last-include block, TREE-03 mixed-state derivation and TREE-04 reopen reconstruction"
    requirement: INTEG-01
    verification:
      - kind: integration
        ref: "web/src/components/SelectionTree.dom.test.tsx (PATCH body, D-04 no-call, remount zero-refetch cases)"
        status: pass
    human_judgment: false
  - id: D4
    description: "i18n — 4 new folders.* keys (treeLabel, truncatedList, retry, emptySelectionBlocked) propagated to en/de inline tables plus all 40 locale files"
    verification:
      - kind: unit
        ref: web/src/lib/i18n.parity.test.ts
        status: pass
    human_judgment: false
  - id: D5
    description: "Visual adequacy of the tri-state tree UX (chevron affordances, mixed-state checkbox rendering, muted notice rows) in a real browser"
    verification: []
    human_judgment: true
    rationale: "jsdom cannot assert visual layout or interactive feel; automated tests cover state and wire contracts only"
metrics:
  duration: 39m
  completed: 2026-09-10
status: complete
actuals:
  tokens: 30375   # chars/4 over the realized 3-commit diff (121499 chars); plan estimate 34000
  tasks: 2
  commits: 3
---

# Phase 02 Plan 01: Container Panel Tree Selection — Tracer Summary

Lazy tri-state selection tree inside the container panel's FoldersEditor: every node's checked/mixed/excluded state is pure arithmetic over Phase 1's flat (includes, exclusions) wire list, children fetch exactly once per node on first expand through an editor-lifetime cache, and every toggle live-PATCHes the canonical flat list with `selectionSource: "tree"`.

## Tasks Completed

| # | Task | Commit | Key files |
|---|------|--------|-----------|
| 1 | Tracer slice: pure selection logic + SelectionTree + FoldersEditor wiring + i18n (TDD) | `3e1bc7a8` (RED) → `d5fc2a08` (GREEN) | selectionTree.ts, selectionTree.test.ts, SelectionTree.tsx, SelectionTree.dom.test.tsx, Containers.tsx, api.ts, i18n.ts, 40 locales |
| 2 | Pin determinism, permutation insensitivity, persistence bounds | `673193fa` | selectionTree.test.ts |

Task 1 was the `type="tracer"` slice: 45 pure tests + 8 jsdom tests written RED first, then the real implementation (never a throwaway). Tracer feedback gate re-ran end-to-end verification after the GREEN commit (tsc clean, 140 tests across the three verify files green): `⚡ Tracer verified end-to-end — expanding`.

## Test Coverage

- `web/src/lib/selectionTree.test.ts` — 51 node-env table tests: segment-aligned prefix arithmetic (the `/c/plex` vs `/c/plex2` trap), all four classifyNode states including equal-pair exclusion dominance (total classifier), 8 applyToggle transitions plus the full D-01 cycle (check → carve out → uncheck parent → re-check restores the EXACT partial), splitFlatSet/toFlatList canonical order + insertion-order insensitivity + round-trip + explicit-none carrier, host↔browse-rel translation, expansion persistence (per-container scoping, cap 64 with recency eviction, corrupted JSON → [], throwing storage → [] both directions), toggle-sequence determinism (byte-identical lists, pinned end state), permutation insensitivity (all 24 orderings of a 4-entry list over a 7-node corpus).
- `web/src/components/SelectionTree.dom.test.tsx` — 8 jsdom tests: zero browse calls on editor open / exactly one on first expand with the browse-relative path (`user/appdata/plex`) / cache hit on re-expand; child uncheck PATCHes `{backupPaths: [mount, "!"+mount+"/transcoding"], selectionSource: "tree"}` with parent aria-checked="mixed" and the excluded child muted; remount with pre-seeded localStorage + shared cache reconstructs states with zero new browse calls (TREE-04); exclusions-only stored list renders all-unchecked and PATCHes nothing; empty listing → `folder.none`; pending promise → loading row; ok:false error → server text verbatim + "Try again" refetches (cache eviction pinned); D-04 block on last-include uncheck.

## Verification

All plan-level gates green (run via `node node_modules/...` invocations — the `npx`/`npm run` cmd shims are broken on this Windows host, a tooling quirk, not a repo issue):

- `tsc --noEmit` — clean.
- Full vitest suite — **2144 tests / 89 files passed** (was 2138 before this plan; +59 tests, 0 failures), including i18n parity/orphans/quality across all 42 locales.
- `eslint` — 0 errors; the 2 warnings (ActivityLog.tsx useMemo, Sidebar.tsx ref-cleanup) are pre-existing and out of scope.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Pulled the minimal D-04 empty-selection block forward from plan 02**
- **Found during:** Task 1 (i18n step)
- **Issue:** The plan's i18n task adds `folders.emptySelectionBlocked` in all 42 locales, but the en-key orphan test fails on any key no component renders — the D-04 inline warn line was scheduled for plan 02, which would have left this plan's verify red.
- **Fix:** Landed the minimal D-04 client block now: `FoldersEditor.onToggle` refuses a toggle that would empty the includes set (sets `blockedPath`, shakes the row, no PATCH), and `SelectionTree` renders the `folders.emptySelectionBlocked` warn line under the blocked row. The full D-04 treatment (copy review, edge polish) still lands in plan 02.
- **Files modified:** web/src/pages/Containers.tsx, web/src/components/SelectionTree.tsx
- **Commit:** `d5fc2a08`

**2. [Rule 3 - Blocking] Rendered the truncated-list notice and error retry affordance this plan**
- **Found during:** Task 1 (SelectionTree implementation)
- **Issue:** Same orphan-test coupling: `folders.truncatedList` and `folders.retry` keys were in this plan's i18n budget, so their rendering sites had to exist here too.
- **Fix:** The truncated listing row and the per-node error row with a "Try again" Button are part of SelectionTree (both were specced in the plan's row semantics anyway; only the copy was budgeted here).
- **Files modified:** web/src/components/SelectionTree.tsx
- **Commit:** `d5fc2a08`

**3. [Rule 3 - Blocking] Exported FoldersEditor from Containers.tsx**
- **Found during:** Task 1 (dom harness)
- **Issue:** The dom test plan requires mounting FoldersEditor in isolation; it was a module-private function.
- **Fix:** `export function FoldersEditor` — same precedent as the existing `ExcludesEditor` export in that file.
- **Files modified:** web/src/pages/Containers.tsx
- **Commit:** `d5fc2a08`

## Interim window

Checker-accepted, wave-ordered gap between this plan and plan 02 Task 2:

- **Rapid toggles use the simple await-per-save path.** Each toggle optimistically updates the mirror and awaits its own PATCH; a burst of clicks on different rows issues concurrent saves and an optimistic revert on failure. The serialized one-deep save queue (drop stale in-flight results, coalesce to latest) lands in plan 02 Task 2. This is a UI-only window over correct server semantics — the server remains the only selection truth and every PATCH carries the full canonical list.
- (The plan's other anticipated window — the last-include uncheck not being blocked client-side — was closed early by deviation 1 above, since the i18n orphan test forced the minimal D-04 block into this plan.)

## TDD Gate Compliance

Both tasks carried `tdd="true"`; gate sequence verified in git log: `test(02-01)` RED commit `3e1bc7a8` (failing tests confirmed failing before implementation) precedes `feat(02-01)` GREEN commit `d5fc2a08` (full suite green), and the Task 2 pin commit `673193fa` follows. No REFACTOR commit was needed — the GREEN implementation passed as written.

## Known Stubs

None. No placeholder data, no unwired props, no skipped tests, no unrun verification steps.

## Self-Check: PASSED

- Commits found in git log: 3e1bc7a8, d5fc2a08, 673193fa.
- Created files present on disk: web/src/lib/selectionTree.ts, web/src/lib/selectionTree.test.ts, web/src/components/SelectionTree.tsx, web/src/components/SelectionTree.dom.test.tsx.
- Full-suite verification rerun green after the final commit (2144 passed / 89 files).
