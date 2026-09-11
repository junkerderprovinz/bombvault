---
phase: 02-container-panel-tree-selection
fixed_at: 2026-09-10T15:33:44Z
review_path: .planning/phases/02-container-panel-tree-selection/02-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 02: Code Review Fix Report

**Fixed at:** 2026-09-10T15:33:44Z
**Source review:** .planning/phases/02-container-panel-tree-selection/02-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (1 critical, 4 warnings; fix_scope = critical_warning)
- Fixed: 5
- Skipped: 0
- Out of scope: IN-01 (Info tier — `aria-setsize` under truncation; review itself marks it acceptable as-is)

All fixes were applied in an isolated worktree (`gsd-reviewfix/02-<pid>` off `docker-folders`), committed per finding, then fast-forwarded onto `docker-folders`. The worktree, temp branch, and recovery sentinel were cleaned up transactionally.

## Fixed Issues

### CR-01: `applyToggle` "mixed" branch is a silent no-op for the carve-out flavor

**Files modified:** `web/src/lib/selectionTree.ts`, `web/src/pages/Containers.tsx`, `web/src/lib/selectionTree.test.ts`
**Commit:** 42af7305
**Status:** fixed: requires human verification (logic change — verified by execution, see below; the deselect direction was the review's primary choice but the review also blesses select-all as equally valid)
**Applied fix:**
- `selectionTree.ts` mixed case now tracks whether any strictly-below include was dropped; when none was (the carve-out flavor: an ancestor include applies, the exclusion sits strictly below, nothing of ours lives under the node), the click deselects the branch — `next.exclusions.add(node)`, identical in spirit to the "checked via ancestor only" case. The deeper exclusion stays stored (redundant under ours, preserved like every orphan E entry so the remembered partial wakes on re-include). The dispatch-table doc comment no longer asserts the false "assumption A2".
- `Containers.tsx onToggle` gained the defense-in-depth no-op guard: if both sets round-trip identical (equal sizes + one-way membership = set equality), return before the mirror apply, busy flag, and queue — a reducer no-op can never produce a PATCH or a "Saved" toast again.
- `selectionTree.test.ts` new table case pinning the flavor: `before {includes:["/a"], exclusions:["/a/b/c"]}, node "/a/b"` → `after {includes:["/a"], exclusions:["/a/b","/a/b/c"]}`.

**Verification:** beyond re-read, the pure reducer was compiled standalone and executed against all 9 toggle table cases (pre-existing semantics unchanged) plus the new carve-out case and its round-trip (re-include restores the remembered partial: node mixed again, E=["/a/b/c"]). All pass. The four Phase-02 test suites (94 tests) pass through the real editor in jsdom.

### WR-01: Non-treeitem children inside `role="tree"` / `role="group"` violate the APG tree structure

**Files modified:** `web/src/components/SelectionTree.tsx`
**Commit:** 9af93018
**Applied fix:** The blocked warn `<p>` and the four notice rows (loading, empty, error, truncated) each render inside a `role="presentation"` wrapper. Presentation removes the wrapper from the tree/group child contract while the notice text stays exposed to assistive tech. Inner elements keep their original tags/classes, so existing assertions (`tagName === "P"`, `closest('[role="treeitem"]') === null`, `.animate-spin` count) are unaffected — confirmed by the passing dom suites. A header note documents the pattern at the file top.

### WR-02: Mixed nodes announce contradictory state (input "checked" vs treeitem "mixed")

**Files modified:** `web/src/components/SelectionTree.tsx`, `web/src/components/SelectionTree.dom.test.tsx`, `web/src/pages/Containers.tree.dom.test.tsx`
**Commit:** 00e76da3
**Applied fix:** `aria-hidden="true"` added to the row checkbox input (`tabIndex={-1}` kept); the treeitem's `aria-checked` is the single selection announcement, and mouse users keep clicking the input. Because testing-library's `ByRole` filters aria-hidden elements by default, the 12 `getByRole("checkbox")` query sites in the two dom test files now pass `{ hidden: true }`, with a rationale comment at the first site of each file. All 94 tests pass.

### WR-03: `addCustom` composes an uncleaned host path — invisible, unreachable custom row

**Files modified:** `web/src/pages/Containers.tsx`
**Commit:** fab1b61c
**Applied fix:** `addCustom` now composes through the exported translator `browseRelToHost(raw, hostSourceRoot)` — the same function the tree uses for its children — so manually typed variants ("appdata/plex/", "appdata//plex", "a/../appdata/plex") are path.Clean-ed before entering `custom` and `includes`. This fixes the `partitionCustomPaths` cleaned-vs-raw membership mismatch (invisible selected include with no remove chip) and makes the duplicate guard catch spelling variants. Already-absolute paths still pass through untranslated (cleaned only), preserving the manual-fallback precedent. `browseRelToHost` added to the existing `selectionTree` import.

### WR-04: `addCustom` clears the staged pick before the duplicate early-return

**Files modified:** `web/src/pages/Containers.tsx`
**Commit:** 25389f2b
**Applied fix:** `setBrowseValue("")` moved after the duplicate guard, per the review's primary option: a duplicate add no longer destroys the staged pick — the text stays in the input as the feedback that the path is already present. The review's alternative (a "duplicate path" toast) was not taken: no suitable existing key exists and a new `folders.*` key would require parity across all 42 locales, churn judged disproportionate for a guard this rare. The trade-off is documented in a site comment.

## Verification

Gates ran in the MAIN checkout after the fast-forward (the isolated worktree has no `node_modules` by design, so a worktree-env run is not reproducible from there; the numbers below are reproducible from `docker-folders` at 25389f2b):

- `vitest run` on the four affected suites (`selectionTree.test.ts`, `SelectionTree.dom.test.tsx`, `SelectionTree.keyboard.dom.test.tsx`, `Containers.tree.dom.test.tsx`): **4 files, 94 tests, all pass**.
- `tsc --noEmit -p web/tsconfig.json`: clean.
- `eslint` on all 7 touched files: clean.
- Standalone execution of the compiled pure reducer (`selectionTree.ts` has zero dependencies): all 9 toggle table cases + new carve-out case + re-include round-trip pass.
- `web/dist` untouched (no build run; only the tracked placeholder remains), verified via `git status -- web` after the gates.

## Skipped Issues

None — all in-scope findings were fixed. IN-01 (Info) was outside the `critical_warning` fix scope.

---

_Fixed: 2026-09-10T15:33:44Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
