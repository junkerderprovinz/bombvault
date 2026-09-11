---
phase: 04-file-sets-parity
plan: 03
subsystem: web-files-page
tags: [selection-tree, file-sets, i18n, live-save, d06-refusal]
requires:
  - "04-02: selectedPaths PATCH boundary + restore guard (server refuses empty-selection, wire type FileSetView.selectedPaths)"
  - "Phase 2: SelectionTree component + selectionTree.ts (splitFlatSet/applyToggle/toFlatList/rootIncludeCount)"
  - "04-01: hostMountRoot prop on Files.tsx (resolved root computation)"
provides:
  - "FileSetFoldersEditor: per-set Choose folders disclosure mounting the Phase 2 SelectionTree over one synthetic root"
  - "One-deep serialized PATCH queue for file-set selection (Containers Pattern 4, single owed class)"
  - "SelectionTreeProps.blockedMessage: caller-routed refusal copy (default keeps folders key; containers byte-identical)"
  - "4 files.* i18n keys across en + de + 40 locale chunks (files.foldersToggle, files.foldersHint, files.emptySelectionBlocked, files.pathChangeHint)"
  - "Files.tree.dom.test.tsx: the Files page's first DOM harness (12 tests)"
affects:
  - "04-04: file-set dialog reuses FileSetFoldersEditor's mount contract and the same i18n keys"
tech-stack:
  added: []
  patterns:
    - "Containers.tsx Pattern 4 live-save queue narrowed to ONE owed class (no reset stickiness)"
    - "Keyed-Fragment shake nonce (row remount replays .glim-shake)"
key-files:
  created:
    - web/src/pages/Files.tree.dom.test.tsx
    - web/src/lib/i18n.filesSelection.test.ts
  modified:
    - web/src/lib/i18n.ts
    - web/src/lib/locales/*.ts (40 chunks)
    - web/src/components/SelectionTree.tsx
    - web/src/pages/Files.tsx
    - web/dist/index.html (bundle hash roll)
decisions:
  - "Reuse contract via additive-optional props only: excludeCaches/onToggleCaches optional (CACHEDIR sub-row renders only when both present), dest==='' label branch (bare path, no arrow), blockedMessage optional (default folders key) - container panel byte-identical, all 107 container/lib tests green unmodified"
  - "NULL mirror seed honesty (UI-SPEC item 9): splitFlatSet over set.selectedPaths ?? [root] renders the root CHECKED at '1 paths' and fires nothing until the first toggle - NULL stays NULL in storage"
  - "Refusal copy routed per domain: SelectionTree renders blockedMessage ?? t('folders.emptySelectionBlocked'); Files passes t('files.emptySelectionBlocked') which orients to Remove set (the files domain has no Reset)"
  - "i18n orphan-gate honesty: keys land one task ahead of their last renderer, so i18n.filesSelection.test.ts pins the exact en copy as the contract reference (Rule 3)"
  - "Files PATCH carries no selectionSource field (presence-gated plan 02 boundary); body is exactly { selectedPaths: toFlatList(live.inc, live.exc) }"
metrics:
  duration: 38 min
  completed: 2026-09-11
  tasks: 3
  commits: 6
actuals:
  tokens: 30182
  tasks: 3
  commits: 6
status: complete
---

# Phase 4 Plan 03: Files Page Selection Tree Summary

**One-liner:** The Phase 2 SelectionTree mounts on every file-set card behind a Choose folders disclosure - one synthetic root seeded honestly from the NULL column, one-deep serialized saves with D-06 refusal on both halves, and 4 new i18n keys across all 42 locales.

## What Was Built

- **Task 1 (i18n interface-first):** `files.foldersToggle` / `files.foldersHint` / `files.emptySelectionBlocked` / `files.pathChangeHint` in the en table (UI-SPEC copy verbatim), de table (du-form; each locale's refusal embeds its own `files.deleteSet` label), and all 40 lazy locale chunks. Parity, quality, and orphan gates pass.
- **Task 2 (mount contract, TDD):** `FileSetFoldersEditor` in Files.tsx - row-level disclosure (`aria-expanded`/`aria-controls`, border-t placement per UI-SPEC item 1), exactly one level-1 treeitem (`aria-setsize=1`, mono bare-path label, no dest-arrow), NULL seed rendering the root CHECKED at "1 paths" with zero writes, argv-matching preview count via `rootIncludeCount`, no-Path gating (no disclosure; `files.noPathHint` stands in), lazy browse through the hostMountRoot prefix with an editor-lifetime cache, per-set expansion memory (`bv-tree-expanded-fileset-{id}`).
- **Task 3 (live-save pipeline, TDD):** one-deep serialized PATCH queue over a ref mirror (burst collapses to `maxConcurrentPatches === 1`; the drain sends the LIVE mirror's latest full list), client D-06 zero-include block before any request, server `code:"empty-selection"` refusal routed to the same inline warn line plus the verbatim fail toast, revert-from-live-mirror by set-difference inverse (a toggle stacked behind a failing save survives), busy/shake row wiring, exact-reopen (zero refetch; selection reconstructs from the (I, E) mirror and from the saved FileSetView identically), Space routed through the same onToggle pipeline.

## Verification Results

- `npx vitest run src/pages/Files.tree.dom.test.tsx` - 12/12 green (5 mount + 7 pipeline pins, including the `maxConcurrentPatches === 1` pin, the refusal no-call pin, and the exact-reopen pin)
- `npx tsc --noEmit` - exits 0; `npx eslint src/pages/Files.tsx src/components/SelectionTree.tsx` - 0 errors
- `npx vitest run` (full suite) - 93 files / 2234 tests green, including Containers.tree.dom.test.tsx (unmodified) and selectionTree.test.ts
- `git diff web/src/lib/selectionTree.ts` across the plan - zero changes (byte-untouched)
- Acceptance greps: `applyToggle` in Files.tsx = 4; `selectionSource` = 0; `files.emptySelectionBlocked` = 1; `excludeCaches?` in SelectionTree.tsx = 1; `rootIncludeCount` = 1
- `npm ci && npm run build` - clean; web/dist/index.html bundle-hash roll committed (assets gitignored per repo convention)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Copy-contract test file created ahead of the renderers**
- **Found during:** Task 1
- **Issue:** The i18n orphan gate refuses any en key nothing in src names; the plan ordered the 4 keys a task (and plan 04-04) ahead of their renderers, so the fanout commit alone would fail the gate.
- **Fix:** Created `web/src/lib/i18n.filesSelection.test.ts` - pins the exact en copy of all 4 keys (including the D-06 "must contain files.deleteSet" relation and the em-dash ban) as the honest reference the gate counts as a use.
- **Files modified:** web/src/lib/i18n.filesSelection.test.ts
- **Commit:** 382103cc

**2. [Rule 1 - Bug] Test harness correctness fixes while driving the pipeline pins green**
- **Found during:** Task 3 GREEN
- **Issue:** Three harness assumptions were wrong, not the implementation: (a) child treeitems are DOM siblings of the root treeitem (role="group" wrapper), so `within(root)` queries find nothing; (b) the mocked browse's default reply is an EMPTY listing, so tests expanding the root must seed `browseReplies`; (c) the shake nonce remounts rows via the keyed Fragment, so DOM references captured before a block/failure go stale; the tree's key map acts on the roving-focus node, so the Space pin must focus the row first and keyDown on the tree element.
- **Fix:** Screen-level child queries, seeded listings (including a second listing for the exact-reopen remount - whose root auto-expands from localStorage, so the manual re-expansion click was removed), post-shake re-queries, and the component-harness focus-then-press pattern for Space. The Space scenario was also corrected to seed a genuinely excluded child ([root, !sub]) - under a NULL seed the child is covered, and Space there (correctly) excludes it.
- **Files modified:** web/src/pages/Files.tree.dom.test.tsx
- **Commit:** 822394a8

**3. [Rule 1 - Bug] Server-refusal test did not exercise what its name claimed**
- **Found during:** Task 3 GREEN
- **Issue:** The "stacked toggle survives a server refusal" scenario toggled only once, so the live-mirror revert had nothing newer to preserve.
- **Fix:** The first PATCH is now held in flight (deferred reply), a second toggle stacks behind it, and the refusal then reverts ONLY the failed delta - media returns covered while sub's carve-out survives, and the drain re-sends exactly [ROOT, !sub].
- **Files modified:** web/src/pages/Files.tree.dom.test.tsx
- **Commit:** 822394a8

**Task-staging note (not a deviation):** Task 2's GREEN initially tripped `noUnusedLocals` because rowBusy/rowShake/blockedPath have no reader until the Task 3 queue exists; the three useState declarations and their optional SelectionTree props moved to the Task 3 commit (recorded in 1f305d9f's message).

## Requirements Completed

- **INTEG-02** (criterion 1): the Files page renders the shared selection tree for file sets with exact reopen. Shared with plans 04-01/04-02/04-04 - the requirement is only fully satisfiable when 04-04's dialog work lands; mark-complete is gated on the phase's last summary.

## Authentication Gates

None.

## Known Stubs

None. Every rendered string routes through a wired i18n key; no placeholder data sources.

## Threat Flags

None. No new network endpoints, auth paths, or trust-boundary surfaces; the editor PATCHes through the existing plan-02 boundary (`patchFileSet`), whose server-side empty-selection refusal (T-04-13) this plan now exercises from the client.

## Self-Check: PASSED

- 382103cc (Task 1 feat) - FOUND in git log
- 3629a451 (Task 2 RED) - FOUND
- 1f305d9f (Task 2 GREEN) - FOUND
- 4f116c75 (Task 3 RED) - FOUND
- 822394a8 (Task 3 GREEN) - FOUND
- b6229e98 (dist roll) - FOUND
- web/src/pages/Files.tree.dom.test.tsx - FOUND on disk
- web/src/lib/i18n.filesSelection.test.ts - FOUND on disk
- .planning/phases/04-file-sets-parity/04-03-SUMMARY.md - FOUND on disk (this file)
