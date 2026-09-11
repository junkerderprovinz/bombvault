---
phase: 02-container-panel-tree-selection
plan: "03"
subsystem: web/spa-container-panel
tags: [keyboard-accessibility, treeview-apg, roving-tabindex, aria-geometry, sub-include-absorption, dom-tests]
requires:
  - "02-01 slice: selectionTree.ts (I,E) arithmetic + SelectionTree.tsx rendering + FoldersEditor wiring"
  - "02-02 slice: serialized one-deep PATCH queue, D-04 zero-include guard, per-node outcome rows"
  - "Phase 1 wire contract: flat includes/\"!\"-exclusions, ContainerMounts custom[] exact-match classification (service.go:3755-3782)"
provides:
  - "Pinned TREE-05: the full APG TreeView checkbox-variant key map (Right expand-then-descend, Left trio, Down/Up/Home/End, Enter, Space-only selection through the same onToggle pipeline as clicks — T-02-10), roving tabindex with exactly one tabbable treeitem, keyboard focus order driven by the same flat model walk that renders"
  - "Uniform lazy-node aria geometry (Pitfall 7): aria-level/setsize/posinset on every treeitem including deep lazy nodes, aria-checked-only selection (never aria-selected), notice rows outside the role and the counts, aria-expanded present on every expandable node and honestly omitted on end nodes"
  - "partitionCustomPaths + FoldersEditor absorption wiring (INTEG-01 closure, D-02, RESEARCH Q1): sub-includes the server serves as custom rows are absorbed under their reachable mount (whitelist start-state renders the mount mixed), standalone customs keep their level-1 lazy rows — one presentation per path"
  - "Integration pins: tree PATCH wire contract (bare mount + \"!\"+host subfolder, selectionSource tree), D-04 no-call through the real editor, close/reopen restores expansion from bv-tree-expanded with zero refetch, 64-entry cap proven through the component path"
affects:
  - web/src/components/SelectionTree.tsx
  - web/src/lib/selectionTree.ts
  - web/src/pages/Containers.tsx
  - Phase 4 File Sets (reuses SelectionTree — keyboard operability comes along)
tech-stack:
  added: []
  patterns:
    - "keyboard handler acts on a flat visible model built by the SAME recursive walk that renders (flatNodes + parent links) — focus order cannot diverge from DOM order, and a stale focusPath hidden by ancestor collapse falls back to firstRoot keeping the one-tabbable invariant"
    - "keydown target guard (tree or treeitem only) so inner native controls (checkbox, retry Button, remove chip) keep their own key semantics — prevents Space double-toggle on a focused checkbox"
    - "custom-path partition at the editor boundary: pure segment-aligned under-mount test in lib, presentation filter at the call site — mirrors the server's exact-match classification instead of duplicating it"
key-files:
  created:
    - web/src/components/SelectionTree.keyboard.dom.test.tsx
    - web/src/pages/Containers.tree.dom.test.tsx
  modified:
    - web/src/components/SelectionTree.tsx
    - web/src/lib/selectionTree.ts
    - web/src/lib/selectionTree.test.ts
    - web/src/pages/Containers.tsx
key-decisions:
  - "aria-expanded is rendered on every EXPANDABLE treeitem and omitted on honest leaves (unreachable mounts, custom paths outside hostSourceRoot) — APG end-node rule; the must-have's 'rendered on every treeitem' read as 'every expandable treeitem', both directions pinned in tests"
  - "Keyboard navigation acts on flatNodes (the render walk's output with parent links), never on a separate index: one source of truth for visual order, geometry, and focus"
  - "Right is a no-op while children load (the loading notice row is not focusable, so there is no child to descend to); Enter is the expansion default action on expandable nodes only"
  - "Space routes through the exact onToggle the checkbox click uses — one D-04 guard, one save queue, no key-only bypass (T-02-10); unreachable nodes and busy rows are skipped"
  - "Absorption filters only the RENDERED custom rows; addCustom's duplicate guard still checks the raw server list (a sub-include must not be re-addable as a duplicate custom row), and empty-state checks keep raw counts"
  - "Only reachable mounts absorb — an unreachable mount cannot be browsed, so its sub-includes keep standalone rows"
  - "Test-side (not component-side) fix for jsdom focus: .focus() must be act-wrapped so the onFocus state commit lands before the keydown reads it — in real browsers the discrete focus event always precedes the subsequent keydown"
  - "No rebuilt web/dist committed: repo convention keeps only the tracked web/dist/index.html placeholder (real artifacts gitignored); the production build was verified locally and the placeholder restored (plan-01/02 precedent)"
patterns-established:
  - "Flat-model-driven roving tabindex for any future composite widget (Phase 4 File Sets tree inherits keyboard operability for free)"
  - "Editor-boundary partition of server-classified rows (partitionCustomPaths) — pure lib function + presentation filter, testable in node env"
requirements-completed: [TREE-05, TREE-01, TREE-04, INTEG-01]
coverage:
  - id: D1
    description: "Full APG key map on the tree (TREE-05): Right expand-then-descend (focus stays on parent; no-op while loading), Left collapse/walk-up/closed-root-no-op, Down/Up never expand, Home/End across roots, Enter expansion default action, leaf no-ops, Space the only selection toggler riding the same PATCH pipeline as clicks with byte-identical round-trip lists"
    requirement: TREE-05
    verification:
      - kind: unit
        ref: "web/src/components/SelectionTree.keyboard.dom.test.tsx (describe 'SelectionTree keyboard map (TREE-05)', 7 key-map tests, jsdom + document.activeElement)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Roving tabindex (exactly one [tabindex=0] treeitem at every point) and uniform aria geometry: level/setsize/posinset on every node including the level-3 lazy node, checked-only selection (aria-selected never present), notice rows (loading/truncated/empty) outside the treeitem role and setsize counts, expanded-empty aria-expanded=true"
    requirement: TREE-05
    verification:
      - kind: unit
        ref: "web/src/components/SelectionTree.keyboard.dom.test.tsx (tabbable-count + uniform-geometry + notice-rows tests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "FoldersEditor integration (INTEG-01): subfolder uncheck PATCHes [mount, '!'+host subfolder] with selectionSource tree; last-include toggle fires no request and shows the D-04 warn line through the real editor; close/reopen restores expansion from bv-tree-expanded-{name} and children from the editor-lifetime cache with zero refetch; 66 expansions persist exactly the 64 most recent through real clicks"
    requirement: INTEG-01
    verification:
      - kind: unit
        ref: "web/src/pages/Containers.tree.dom.test.tsx (describe 'FoldersEditor tree integration (INTEG-01, D-02, D-04, D-05)', 6 tests)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Sub-include absorption (D-02/RESEARCH Q1): partitionCustomPaths segment-aligned tables (7 cases incl. plex2 sibling and exact-mount-root) + absorption through the editor — unselected mount renders mixed (whitelist start-state), no duplicate custom row for the sub-include, child renders checked exactly once"
    requirement: INTEG-01
    verification:
      - kind: unit
        ref: "web/src/lib/selectionTree.test.ts (describe 'partitionCustomPaths', 7 tables) + Containers.tree.dom.test.tsx absorption and standalone-custom tests"
        status: pass
    human_judgment: false
  - id: D5
    description: "Visual adequacy of keyboard focus movement (scrollIntoView nearest-row behavior) and the absorbed-mixed presentation in a real browser"
    verification: []
    human_judgment: true
    rationale: "jsdom asserts focus targets, roles, and wire contracts only; visible scrolling and mixed-parent visual reading need human eyes at end-of-phase UAT"
metrics:
  duration: 22m
  completed: 2026-09-10
status: complete
actuals:
  tokens: 12330    # chars/4 over the realized 6-file diff (49306 chars); plan estimate 30000 — tests are the bulk of the diff and they were written tight
  tasks: 3
  commits: 5
---

# Phase 02 Plan 03: Keyboard/ARIA Operability + Integration Closure Summary

**The selection tree is now fully keyboard-operable per the APG TreeView checkbox variant (roving tabindex over the same flat model that renders, Space riding the exact click pipeline) with uniform lazy-node aria geometry, and the INTEG-01 seam is closed: sub-includes the server classifies as custom rows are absorbed under their reachable mount — one presentation per path — with the wire contract, D-04 guard, reopen cache, and 64-cap pinned through the real FoldersEditor.**

## Performance

- **Duration:** 22 min
- **Started:** 2026-09-10T14:36:52Z
- **Completed:** 2026-09-10T14:59:20Z
- **Tasks:** 3
- **Files changed:** 6 (2 created, 4 modified; 1024 insertions, 8 deletions)

## Accomplishments

- SelectionTree grew the complete keyboard map (TREE-05): ArrowRight expands with focus staying on the parent and descends only on a second press (no-op while children load — the loading notice is not focusable); ArrowLeft collapses an open node, walks up from a closed child, and does nothing on a closed level-1 root; Down/Up/Home/End move focus in visual order without ever expanding; Enter is the expansion default action; Space is the only selection toggler and calls the same `onToggle` the checkbox click calls, so the D-04 zero-include guard and the serialized save queue apply identically to key users (T-02-10). Focus moves call `scrollIntoView({ block: "nearest" })`.
- The handler acts on `flatNodes` — the flat visible model built by the same recursive walk that renders, with parent links — so focus order can never diverge from DOM order, and a `focusPath` hidden by an ancestor collapse falls back to the first root, keeping exactly one tabbable treeitem at all times. A target guard lets inner native controls (checkbox, retry Button, remove chip) keep their own key semantics.
- Aria geometry is uniform (Pitfall 7): `aria-level`/`aria-setsize`/`aria-posinset` on every treeitem including deep lazy-loaded nodes, selection carried only by `aria-checked` (`aria-selected` never present), notice rows (loading/truncated/empty) rendered outside the treeitem role and the counts, an expanded empty directory honestly reporting `aria-expanded="true"` with zero child treeitems, and `aria-expanded` omitted on honest leaves (unreachable mounts, custom paths outside the served root).
- `partitionCustomPaths` (7 table cases) + the FoldersEditor wiring close INTEG-01's dual-presentation seam: a stored include strictly under a reachable mount — which the server serves as a custom row because its classification is exact-match-only — stays in the (I, E) mirror (rendering its mount mixed via the whitelist start-state and the sub-include checked once browsed) and is filtered from the rendered custom rows. Segment-aligned (`plex2` is a sibling), exact mount-root equality never a sub-include, only reachable mounts absorb.
- Six FoldersEditor integration tests pin the editor's end-to-end contract: the PATCH body shape (`[mount, "!"+host subfolder]`, `selectionSource: "tree"`), D-04's no-call through the real editor (not just the tree), absorption + standalone-custom lazy browsing, close/reopen restoring expansion from the localStorage key with children served by the editor-lifetime cache (zero refetch), and the 64-entry cap through 66 real clicks.
- Phase gate green: lockfile-fresh `npm ci`, full suite **2178 tests / 91 files** (was 2155; +23), `eslint src` 0 errors, `tsc --noEmit && vite build` exits 0, `go build ./... && go vet ./...` clean, `gofmt -l .` silent.

## Task Commits

Each task was committed atomically:

1. **Task 1: TREE-05 keyboard/ARIA (tdd)** - `c5de5c24` (test, RED: 10/10 failing) → `f47498e3` (feat, GREEN: 10/10 passing)
2. **Task 2: INTEG-01 absorption + integration pins (tdd)** - `c7c5fa9c` (test, RED: 8 failing — 7 partition tables + the absorption dom case) → `e2ce88eb` (feat, GREEN: 64/64 + 103/103 across the five tree suites)
3. **Task 3: phase gate** - `5ba49147` (fix: two unused bindings the gate's eslint run flagged in the Task 1 test file; everything else verified clean, no other tracked-file change)

**Plan metadata:** (this SUMMARY + state updates)

## Files Created/Modified

- `web/src/components/SelectionTree.keyboard.dom.test.tsx` - created, 10 tests: the full key map, tabbable-count invariant, uniform geometry loop over every treeitem, notice-row exclusion; harness = mock-api FoldersEditor with act-wrapped `focusItem` (jsdom focus commits must land before keydown) and deferred browse replies for loading states
- `web/src/pages/Containers.tree.dom.test.tsx` - created, 6 integration tests (wire contract, D-04 no-call, absorption, standalone customs, close/reopen cache, 64-cap via batched real clicks)
- `web/src/components/SelectionTree.tsx` - `useRef` tree/row refs, `FlatNode` + the `walk()` flat model, `focusNode`, `handleTreeKeyDown` (the APG map with target guard), per-row ref callbacks; header comment documents the map and the leaf-omission rationale
- `web/src/lib/selectionTree.ts` - `partitionCustomPaths` with the load-bearing comment tying it to service.go's exact-match rule, D-02, and segment alignment
- `web/src/lib/selectionTree.test.ts` - +7 `partitionCustomPaths` table cases (58 tests total in the file)
- `web/src/pages/Containers.tsx` - `partitionCustomPaths` call at the editor boundary; `customRows` (standalone only) passed to `SelectionTree`; duplicate guard and empty-state checks keep raw lists

## Decisions Made

- **aria-expanded on expandable nodes, omitted on leaves** — the must-have phrase "rendered on every treeitem" was read against the APG end-node rule it cites (RESEARCH Pitfall 7): an honest leaf must NOT carry aria-expanded. Both directions are pinned (geometry loop asserts presence on expandable rows; an explicit leaf assert pins absence).
- **One flat model for render, geometry, and focus** — `flatNodes` is built by the same `walk()` that produces the rows, so the three can never disagree.
- **Right-while-loading is a no-op by construction** — the loading notice row is outside the treeitem role and not focusable, so there is no child to descend to until the promise lands; the test proves it with a deferred reply.
- **Absorption filters presentation only** — the raw server `custom` list still drives `addCustom`'s duplicate guard and the empty-state checks; only the rows handed to `SelectionTree` are filtered, so re-adding an absorbed sub-include is still blocked as a duplicate.
- **act-wrapped focus in tests, component untouched** — the jsdom quirk (focus outside `act()` does not flush the onFocus state commit before a subsequent fireEvent keydown) is a test-environment ordering artifact; real browsers dispatch the discrete focus event before the next keydown, so the fix belongs in the harness.
- **web/dist not committed** — repo convention: only the tracked `web/dist/index.html` placeholder exists in git; the real bundle is gitignored and CI's Docker web stage builds it. Verified locally (`vite build` exits 0), placeholder restored untouched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two unused `root` bindings flagged by the phase-gate eslint run**
- **Found during:** Task 3 (gate)
- **Issue:** The Task 1 test file's act-wrapped `focusItem` refactor left `const root =` bindings read nowhere in two tests — eslint errors (0-errors gate would fail).
- **Fix:** Dropped the bindings; no assertion touched.
- **Files modified:** web/src/components/SelectionTree.keyboard.dom.test.tsx
- **Verification:** `eslint src` 0 errors; keyboard suite 10/10; full suite re-run green after the commit.
- **Committed in:** 5ba49147

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** None on behavior; the gate's own catch, fixed in place.

## TDD Gate Compliance

Both implementation tasks carried `tdd="true"`.

- **Task 1 followed full RED→GREEN:** `test(02-03)` `c5de5c24` failed 10/10 (verified before any implementation existed — including the Left-trio test, which needed a non-vacuous `focused()` assert to fail honestly: without a handler, focus never moves, so no-op assertions alone would pass vacuously). `feat(02-03)` `f47498e3` turned all 10 green. No REFACTOR commit needed.
- **Task 2 was RED-with-pins, the plan-02 Task 1 pattern:** `test(02-03)` `c7c5fa9c` contained 13 cases — 8 failing against the then-missing `partitionCustomPaths` (7 tables) and the dual-presentation bug the absorption fixes (the dom absorption test failed at "Found multiple elements" because the sub-include rendered twice) — plus 5 that passed on arrival because they pin behavior plans 01/02 already landed (wire contract, D-04 no-call, standalone customs, reopen cache, cap-through-component). Those pins are the integration net this plan exists to add; the failing 8 prove the net catches the seam it targets. `feat(02-03)` `e2ce88eb` turned everything green (64/64 on the plan's verify files; 103/103 across the five tree suites).
- Gate sequence in git log: `test` commits precede their `feat` commits for both tasks.

## Issues Encountered

- **Windows host (carried from plans 01/02):** `npx`/`npm run` cmd shims are broken here — `tsc`/`vitest`/`eslint`/`vite` run via direct `node node_modules/...` invocations. `npm ci` itself works. Tooling quirk, not a repo issue.
- **jsdom focus ordering (test-side):** first GREEN run failed 4 keyboard tests because unwrapped `.focus()` does not commit the onFocus state update before the next `fireEvent.keyDown` reads it — the Space PATCH body was the mount-root uncheck instead of the subfolder carve-out. Fixed in the harness (act-wrapped `focusItem`); the component is correct in real browsers.
- **Cap test timeout (test-side):** 66 sequential act-wrapped clicks exceeded the 5s default timeout; batching all clicks into ONE `await act(async () => {...})` keeps the functional reducer chaining correctly and the test at ~1s.
- **go test deliberately NOT run locally** (as the plan directs): restic is not on the Windows PATH; CI's Test job (which installs restic 0.17.3) is the arbiter. `go build ./...`, `go vet ./...`, and `gofmt -l .` all ran clean here; the Windows-host reduced suite caveat is documented in the repo guide.

## Verification

- `npm ci` — lockfile-fresh install OK.
- Full vitest suite — **2178 tests / 91 files passed** (was 2155; +23: 10 keyboard + 6 integration + 7 partition tables), re-run green after the final commit; includes i18n parity across all locales.
- `tsc --noEmit` — clean. `vite build` — exits 0 (chunk-size warning pre-existing/informational); tracked `web/dist/index.html` placeholder restored untouched per repo convention (`git status --porcelain web/dist` clean).
- `eslint src` — 0 errors; 2 warnings are pre-existing in `ActivityLog.tsx`/`Sidebar.tsx` (logged to `deferred-items.md`, out of scope).
- `go build ./... && go vet ./...` — clean; `gofmt -l .` — silent. `go test ./...` deferred to CI per plan.
- Acceptance walked: every key behavior + the three no-ops (loading-Right, closed-root-Left, leaf Enter/Right) pinned; Space round-trip returns the byte-identical prior flat list; exactly one tabbable treeitem before/after navigation; geometry uniform including the level-3 lazy node; notice rows outside role and counts; absorption leaves exactly one presentation per path.

## Known Stubs

None.

## Next Phase Readiness

- Phase 02 is complete: all three plans landed (01 tracer + rendering, 02 outcome rows + serialized queue, 03 keyboard/a11y + integration closure); TREE-01..TREE-06, INTEG-01 all delivered.
- TREE-01, TREE-04, INTEG-01 were already marked Complete from plan 01 (shared-ID); this plan re-pinned them at the integration level — `mark-complete` re-runs are idempotent.
- Phase 4 File Sets reuses `SelectionTree` and inherits keyboard operability and geometry for free.
- Remaining human-judgment surface (D5 here, D4 in plan 02): visual keyboard-focus scrolling and the absorbed-mixed presentation at end-of-phase UAT.

## Self-Check: PASSED

- Commits found in git log: c5de5c24, f47498e3, c7c5fa9c, e2ce88eb, 5ba49147 (all five on docker-folders).
- Created files present on disk: web/src/components/SelectionTree.keyboard.dom.test.tsx, web/src/pages/Containers.tree.dom.test.tsx.
- Modified files present on disk: web/src/components/SelectionTree.tsx, web/src/lib/selectionTree.ts, web/src/lib/selectionTree.test.ts, web/src/pages/Containers.tsx.
- Full suite re-run green after the final commit (2178 passed / 91 files).

---
*Phase: 02-container-panel-tree-selection*
*Completed: 2026-09-10*
