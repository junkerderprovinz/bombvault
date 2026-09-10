---
phase: 03-selection-trust-controls
plan: "02"
subsystem: ui
tags: [react, selection-tree, i18n, restic-positional-truth, treeview]

requires:
  - phase: 02-selection-tree
    provides: the SelectionTree component, the (includes, exclusions) host-path mirror, splitFlatSet/toFlatList wire forms
  - phase: 01-selection-foundation
    provides: the flat backupPaths encoding (bare includes + "!"-prefixed exclusions) whose bare entries are the restic positionals
provides:
  - rootIncludeCount(root, includes) pure helper — the per-root "{n} paths" preview over the flat-set positional truth (SELECT-03 first half, D-01)
  - rootExclusions(root, exclusions) pure helper — the per-root "{n} exclusions" review list, complete by construction incl. dormant entries (INTEG-03, D-03)
  - SelectionTree per-root preview sub-line and collapsible exclusions disclosure/list sub-rows (D-04 dormant rendering included)
  - i18n keys folders.previewPaths and folders.exclusions across all 42 locale tables
  - rebuilt, committed web/dist carrying the phase surfaces
affects: [03-03 (CACHEDIR toggle + reset UI rides the same root sub-row pattern), verify-work UAT]

actuals:
  tokens: 15500
  tasks: 2
  commits: 5

tech-stack:
  added: []
  patterns:
    - "Per-root derived-signal sub-rows: pure helpers over the stored mirror, never screen state (extends the Phase 2 classifyNode discipline to metadata surfaces)"
    - "Unpersisted disclosure expansion for audit views (contrast with D-05 tree-expansion comfort state)"

key-files:
  created: []
  modified:
    - web/src/lib/selectionTree.ts
    - web/src/lib/selectionTree.test.ts
    - web/src/components/SelectionTree.tsx
    - web/src/pages/Containers.tree.dom.test.tsx
    - web/src/lib/i18n.ts
    - web/src/lib/locales/*.ts (40 files)
    - web/dist

key-decisions:
  - "Preview counts stored maximal includes at-or-under the root only (an include above the root does not count) — the exact toFlatList membership per root, so the visible number equals the bare positionals the next backup hands restic"
  - "Count and list are existence-unfiltered by design (A3/Pitfall 5): stale/unreachable paths stay counted and stay listed; those cases are already row-level-warned"
  - "Exclusions disclosure is a plain button (backupOrder precedent), normally tabbable outside the roving set; expansion is per-root component state, deliberately NOT persisted (audit view, not navigation comfort)"
  - "The same exclusions section renders for active and dormant roots with no variant — the root checkbox above distinguishes them (D-04)"

patterns-established:
  - "Derived-trust surfaces: preview/list helpers live in selectionTree.ts beside the classifier and are table-tested against toFlatList agreement"
  - "Presentation-wrapped sub-rows between a treeitem and its group extend to interactive disclosures (aria-controls + reuse of the chevron triangle) without entering the treeitem set"

requirements-completed: [SELECT-03, INTEG-03, INTEG-04]

coverage:
  - id: D1
    description: "rootIncludeCount helper and the per-root '{n} paths' preview line inside every root treeitem label (mounts incl. unreachable, standalone customs, always rendered incl. the 0 case)"
    requirement: SELECT-03
    verification:
      - kind: unit
        ref: web/src/lib/selectionTree.test.ts#rootIncludeCount (D-01: the preview mirrors the flat-set positional truth)
        status: pass
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#per-root effective-selection preview (SELECT-03 first half, D-01)
        status: pass
    human_judgment: false
  - id: D2
    description: "rootExclusions helper and the collapsible '{n} exclusions' review list — relative mono ltr muted non-interactive rows, lexically sorted, title attribute per row"
    requirement: INTEG-03
    verification:
      - kind: unit
        ref: web/src/lib/selectionTree.test.ts#rootExclusions (D-03/D-04: the reviewable, complete-by-construction list)
        status: pass
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#per-root reviewable exclusions (INTEG-03, D-03, D-04)
        status: pass
    human_judgment: false
  - id: D3
    description: "D-04 dormant rendering: a fully-deselected root with remembered exclusions shows unchecked checkbox, '0 paths' preview and '{n} exclusions' section together, never hidden"
    requirement: INTEG-04
    verification:
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#D-04 dormant case: fully-deselected root renders unchecked box, '0 paths' and '{n} exclusions' together, never hidden
        status: pass
    human_judgment: false
  - id: D4
    description: "folders.previewPaths and folders.exclusions present in all 42 locale tables with the {n} placeholder intact and no em dashes"
    verification:
      - kind: unit
        ref: web/src/lib/i18n.parity.test.ts + i18n.orphans.test.ts + i18n.quality.test.ts (190/190 in the plan verify run)
        status: pass
    human_judgment: false
  - id: D5
    description: "Visual adequacy of the two new sub-row surfaces in the tree (muted register, indent 16 rhythm, chevron rotation, list readability under long paths)"
    verification: []
    human_judgment: true
    rationale: "Automated tests pin classes, roles, non-interactivity and text, not whether the surfaces read well beside the Phase 2 rows — end-of-phase UAT should eyeball the tree with real mounts."
  - id: D6
    description: "web/dist rebuilt and committed so the embedded SPA carries the phase surfaces"
    verification:
      - kind: other
        ref: "tsc --noEmit && vite build — clean; dist committed in a84c1601"
        status: pass
    human_judgment: false

duration: 10min
completed: 2026-09-10
status: complete
---

# Phase 3 Plan 02: Selection Trust — Preview & Reviewable Exclusions Summary

**Per-root "{n} paths" preview and collapsible "{n} exclusions" review list derived purely from the stored flat-selection mirror — zero new endpoints, zero new state sources (D-01, D-03, D-04).**

## Performance

- **Duration:** 10 min
- **Started:** 2026-09-10T19:21:47Z
- **Completed:** 2026-09-10T19:31:22Z
- **Tasks:** 2 (both TDD: RED + GREEN commits each)
- **Files modified:** 46

## Accomplishments
- `rootIncludeCount` + the muted "{n} paths" line in every root treeitem label — the count equals what `toFlatList` serializes as bare positionals (what the next backup hands restic), never checked nodes on screen, never existence-filtered (SELECT-03 first half, D-01)
- `rootExclusions` + the per-root collapsible "{n} exclusions" audit section: relative paths, mono ltr break-all with titles, lexically sorted, fully non-interactive, complete by construction incl. dormant entries, derived never from loaded children (INTEG-03, D-03)
- D-04 dormant rendering pinned: a fully-deselected root shows unchecked checkbox + "0 paths" + "{n} exclusions" together and is never filtered away (INTEG-04)
- 2 new i18n keys propagated with translated values to en+de inline tables and all 40 lazy locale tables; parity/orphans/quality green across 42 tables
- web/dist rebuilt and committed; full web suite 91 files / 2201 tests green; tsc, vite build, eslint (0 errors) clean

## Task Commits

Each task was committed atomically (TDD: failing test first, then implementation):

1. **Task 1: rootIncludeCount helper and per-root "{n} paths" preview (tracer)** - `5cb0115d` (test) + `5eca5c56` (feat)
2. **Task 2: rootExclusions helper, collapsible "{n} exclusions" list, D-04 dormant rendering, dist commit** - `90e5d63b` (test) + `a84c1601` (feat, includes web/dist)

**Plan metadata:** this docs commit

## Files Created/Modified
- `web/src/lib/selectionTree.ts` - `rootIncludeCount` and `rootExclusions` pure helpers with D-01/D-03/D-04 why-comments
- `web/src/lib/selectionTree.test.ts` - table tests for both helpers incl. the toFlatList agreement test and the existence-unfiltered pin
- `web/src/components/SelectionTree.tsx` - preview sub-line in mount and custom root labels; presentation-wrapped exclusions disclosure (plain button, reused chevron, per-root unpersisted expansion state) and non-interactive ul/li body
- `web/src/pages/Containers.tree.dom.test.tsx` - two new describes: preview contract (all root kinds, 0 case, carve-out vs narrowing) and exclusions contract (disclosure, zero-hidden, dormant D-04, non-interactive, never-expanded completeness)
- `web/src/lib/i18n.ts` - `folders.previewPaths` / `folders.exclusions` in en and de
- `web/src/lib/locales/*.ts` (40 files) - both keys with translated values, `{n}` intact
- `web/dist` - rebuilt and committed

## Decisions Made
- Preview counts stored maximal includes at-or-under the root only — an include above the root is not that root's entry to announce; this is exactly the per-root toFlatList membership, keeping the visible number equal to the argv truth
- Count and list stay existence-unfiltered (A3/Pitfall 5): stale/unreachable paths count and list; `folders.notReachable` / `folders.customMissing` already warn at row level
- Disclosure is a plain `<button>` (backupOrder precedent), normally tabbable outside the roving set; expansion state is keyed per root and deliberately NOT persisted — an audit view, unlike D-05 tree-expansion comfort state
- Active and dormant roots render the identical section; the checkbox distinguishes them (no second variant, no extra key)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Windows environment quirk (not a code issue): `npm run <script>` / `npx <bin>` fail with `'"node"' n'est pas reconnu` because npm's cmd shim cannot resolve the scoop-managed node from the Git Bash PATH. All verify commands were run with identical semantics by invoking the local bins directly through node (`node node_modules/vitest/dist/cli.js run ...`, `node node_modules/typescript/bin/tsc --noEmit`, `node node_modules/vite/bin/vite.js build`, `node node_modules/eslint/bin/eslint.js src`). Outputs matched the plan's expectations exactly.

## TDD Gate Compliance

Both tasks carried `tdd="true"`; the gate sequence is present in git log:
- RED: `test(03-02): add failing tests for per-root include-count preview` (5cb0115d) — 11 failures for the right reasons (missing export, absent rendering)
- GREEN: `feat(03-02): per-root include-count preview over the flat-set truth` (5eca5c56) — 76/76 pass
- RED: `test(03-02): add failing tests for per-root reviewable exclusions` (90e5d63b) — 10 failures for the right reasons
- GREEN: `feat(03-02): per-root collapsible exclusions review list and dist rebuild` (a84c1601) — 190/190 pass in the plan verify run

Tracer gate (Task 1, autonomous mode): the tracer's `<verify>` was re-run end-to-end at HEAD and passed before Task 2 started.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Ready for 03-03 (CACHEDIR toggle + reset control): the per-root sub-row pattern (presentation-wrapped, indent 16, py-1, sibling order treeitem → sub-rows → group) is established and dom-tested
- No blockers. Pre-existing warn-only eslint notes (ActivityLog.tsx:234, Sidebar.tsx:567) unchanged and tracked in phase 02 deferred-items

## Self-Check: PASSED

- All 7 sampled key files exist on disk (selectionTree.ts/.test.ts, SelectionTree.tsx, i18n.ts, dom test, fr/zh locales)
- All 4 production commits found in git log (5cb0115d, 5eca5c56, 90e5d63b, a84c1601)
- No uncommitted web/dist change after the task commit; no stray untracked build output
- Full web suite re-run green (91 files / 2201 tests); tsc + vite build + eslint 0 errors

---
*Phase: 03-selection-trust-controls*
*Completed: 2026-09-10*
