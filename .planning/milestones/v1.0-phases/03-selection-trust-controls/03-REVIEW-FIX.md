---
phase: 03-selection-trust-controls
fixed_at: 2026-09-10T17:00:00Z
review_path: .planning/phases/03-selection-trust-controls/03-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 4
status: all_fixed
---

# Phase 3: Code Review Fix Report

**Fixed at:** 2026-09-10T17:00:00Z
**Source review:** `.planning/phases/03-selection-trust-controls/03-REVIEW.md`
**Iteration:** 1
**Branch:** `docker-folders` (workflow.use_worktrees=false — fixed and committed directly in the main checkout)

**Summary:**
- Findings in scope (fix_scope=critical_warning): 4 (WR-01..WR-04)
- Fixed: 4
- Skipped: 4 (all Info findings — IN-01..IN-04, out of fix scope)

## Fixed Issues

### WR-01: A confirmed reset can be silently dropped when another tree toggle stacks behind it

**Verdict:** fixed
**Commit:** `6aa9f24f`
**Files modified:** `web/src/pages/Containers.tsx`, `web/src/pages/Containers.tree.dom.test.tsx`
**What changed:** `scheduleSave`'s in-flight branch no longer overwrites a pending `reset: true` paths descriptor with a later toggle's descriptor. The stacked toggle still marks the class owed (the drain must fire) and still flips the mirror optimistically, but the reset body is what drains; the ok refetch then re-derives the editor from the served auto-detected state, so the toggle's optimistic flip reverts (its effect is defined relative to the post-reset state). A reset desc can still replace a pending reset. The stacking-window contract comments (`SaveDesc` doc, `attemptSave` reset branch) now state the split semantics: sticky while pending; latest-intent-wins once the reset's own drain has started (that window is unchanged, documented in code — the review itself noted the sanctioned-stacking contradiction; supersession semantics are now uniform). New dom test pins: toggle in flight, reset confirmed behind it, second toggle stacked, drain body = the reset PATCH (not the live pre-reset list), reload re-derives. Verified RED against the unconditional overwrite before committing the fix.

### WR-02: The post-reset refetch is not serialized with the save queue and can clobber a stacked mutation

**Verdict:** fixed
**Commit:** `a5be53f2`
**Files modified:** `web/src/pages/Containers.tsx`, `web/src/pages/Containers.tree.dom.test.tsx`
**What changed:** The reset-ok branch no longer calls `setLoaded(false)` immediately; it sets `queueRef.current.reload` (new flag on the queue state), and `attemptSave`'s `finally` chain issues the GET only when it did NOT chain a drain and `owedRef` is empty — i.e. the refetch always starts with the queue idle, after every stacked drain settled, so its apply can only reflect final server state. The chained-drain decision is captured in a `chained` local BEFORE `void attemptSave()` (whose first synchronous step consumes `owedRef` — checking it after would always read empty and re-introduce the race). New dom test holds the reset PATCH, stacks a CACHEDIR flip during its flight, and pins that `mountsCalls` stays 1 while the flip's drain is held (the GET leaves only after that drain settles, and the served post-drain map re-seeds the switch ON). Verified RED against the immediate `setLoaded(false)`. Residual window, documented rather than changed (pre-existing phase-2 shape, outside the finding): while a load GET is itself in flight the tree is unmounted (loading gate) and only the Add row is an exposed mutation surface; epoch-guarding the load apply was considered and rejected because a skipped apply leaves the editor un-loaded with no sanctioned re-trigger.

### WR-03: Exclusions-disclosure DOM id can collide between roots

**Verdict:** fixed
**Commit:** `daee9390`
**Files modified:** `web/src/components/SelectionTree.tsx`, `web/src/components/SelectionTree.dom.test.tsx`
**What changed:** `exclListId` now derives from `encodeURIComponent(spec.path)` instead of stripping non-alphanumerics. The encoding is injective on the path, so `/mnt/user/app-data` vs `/mnt/user/app_data` (and any two CJK-named segments) can no longer collapse to one id; the per-instance `useId()` prefix keeps separate trees apart as before. New dom test renders two colliding roots directly against `SelectionTree`, asserts distinct `aria-controls` values and that each button resolves its own list when both disclosures are open. Verified RED against the stripped derivation.

### WR-04: Reset leaves orphaned excludeCaches keys with no UI to turn them off (and the dom test masks this)

**Verdict:** fixed
**Commit:** `4976fa07`
**Files modified:** `web/src/pages/Containers.tsx`, `web/src/lib/api.ts`, `web/src/lib/i18n.ts`, `web/src/lib/locales/*.ts` (40 files), `web/src/pages/Containers.tree.dom.test.tsx`
**What changed:**
- The reset drain now composes `excludeCaches: {}` into the same serialized PATCH body (the empty-selection guard is strictly gated on the literal `"tree"` source, which the reset never sends — the sanctioned no-source shape is unchanged, verified against `internal/api/handlers.go` and `SetExcludeCaches`, where an empty map clears every toggle). A caches flip stacked behind a reset drain is superseded (the reset's `{}` stands; the ok reload re-seeds from the cleared map); on a failed combined drain the flip's revert recipe still runs and lands on server truth, per the comment at the suppression site.
- The confirm copy now names the caches consequence and was propagated to all 42 locales (en/de inline plus the 40 locale files), each reusing its own `folders.cachedirToggle` vocabulary; no em dashes introduced; parity/quality/orphans i18n tests green.
- The main reset dom test now models the orphan scenario: the fixture carries a `true` caches entry keyed by a standalone CUSTOM root (the row class the reset removes), pins the composed body `{backupPaths: [], excludeCaches: {}}`, and asserts post-reset that the custom root is gone and no switch renders ON. The post-reset fixture models the FIXED server (map cleared by the reset PATCH) rather than the pre-fix server that kept it. Verified RED against the old body.
- `ContainerTargetsBody`'s doc comment in `api.ts` documents the reset shape.

## Skipped Issues

### IN-01: `removeCustomPath` bypasses the empty-selection guard, leaving UI/server divergence on refusal

**Verdict:** skipped
**Reason:** info-level, out of fix scope (fix_scope=critical_warning)
**Original issue:** Removing the last custom row of a container with no selected mounts sends a tree-sourced empty save; the server refuses and the structural remove leaves UI/server divergent until reload.

### IN-02: Composed PATCH is not transactional across classes; a caches failure reverts paths that already persisted

**Verdict:** skipped
**Reason:** info-level, out of fix scope (fix_scope=critical_warning)
**Original issue:** When one drain carries both classes and only the caches validation fails, the server already persisted the paths save while the client reverts the whole attempt.

### IN-03: Finally-chained drains read `lastBackup` and `t` from a stale closure

**Verdict:** skipped
**Reason:** info-level, out of fix scope (fix_scope=critical_warning)
**Original issue:** A drain chained from `finally` reads the `lastBackup` prop and `t` captured at an older render; the house ref-mirroring convention (`lastBackupRef`) would fix it.

### IN-04: The 64-entry excludeCaches cap can reject legitimate wide maps with no UI guard or direct test

**Verdict:** skipped
**Reason:** info-level, out of fix scope (fix_scope=critical_warning)
**Original issue:** `maxExcludeCachesEntries = 64` counts explicit `false` values too, and has no boundary test at 64/65 entries.

## Verification

**Where the gates ran:** the main checkout on `docker-folders` (workflow.use_worktrees=false — no isolated worktree was created; all four commits landed directly on the branch). The numbers below are reproducible from this tree.

- `node node_modules/vitest/vitest.mjs run` (full web suite, invoked directly — `npx`/`npm run` are broken in this environment): **91 files / 2217 tests passed**, including the three new tests added by these fixes.
- `node node_modules/typescript/bin/tsc --noEmit`: clean.
- `node node_modules/eslint/bin/eslint.js src`: 0 errors; 2 pre-existing `react-hooks/exhaustive-deps` warnings in `ActivityLog.tsx` and `Sidebar.tsx` (files untouched by these fixes; the rule is warn-only by house convention).
- RED/GREEN discipline: each fix's pinning test was run against the pre-fix source (via a temporary revert) and failed exactly on the new assertion before the fix was committed.
- Go chain (`go build/vet/gofmt/golangci-lint/test`): not run — no files under `internal/` were modified (the WR-04 backend references in the review were read for verification only; the fix is client-side + copy). The orchestrator's post-fix re-run covers it.
- `web/dist` was NOT rebuilt and NOT committed, per instructions — the orchestrator rebuilds and commits it after these fixes (the SPA is embedded; a rebuild is required before shipping).

---

_Fixed: 2026-09-10T17:00:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
