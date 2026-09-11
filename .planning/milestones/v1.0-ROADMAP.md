# Roadmap: BombVault — Tree-Based Sub-Folder Backup Selection

## Overview

This milestone replaces mount-granularity backup selection with tree-based per-folder selection over the same flat `backupPaths` set — no migration, two presentations of one persistence model. The order is risk-first MVP slicing: Phase 1 proves the keystone end-to-end at the engine level (lossless normalization to maximal roots handed to restic as positional targets, cheap containment-safe per-node listing, restores that survive selection changes) before any UI leans on it. Phase 2 lands the visible capability where the pain is — the container panel tree (Plex `transcoding`, caches, logs). Phase 3 makes selections legible and controllable: effective-selection preview, reviewable exclusions, defined empty-deselect semantics, and the CACHEDIR.TAG toggle. Phase 4 carries the same tree to File Sets. Every phase leaves existing deployments fully working.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Selection Engine & Restore Safety** - Normalize selections losslessly into the unchanged flat `backupPaths`, harden per-node listing, and make restores survive selection changes — proven end-to-end at the API/backup level before any UI is built on it. (completed 2026-09-10)
- [x] **Phase 2: Container Panel Tree Selection** - The lazy tri-state tree lands in the container panel: unfold a mount, tick subfolders, keyboard-accessible, state exact on reopen. (completed 2026-09-10)
- [x] **Phase 3: Selection Trust & Controls** - Users can see and control what will be backed up: effective-selection preview, reviewable exclusions, defined empty-deselect semantics, per-root CACHEDIR.TAG toggle. (completed 2026-09-10)
- [x] **Phase 4: File Sets Parity** - The same tree selection powers File Sets coverage. (completed 2026-09-11)

## Phase Details

### Phase 1: Selection Engine & Restore Safety

**Goal**: Selections made through the API normalize losslessly into the unchanged flat `backupPaths` set, node listings are cheap and containment-safe, and restores survive selection changes — the risky keystone (maximal-roots ↔ restic positional targets) proven end-to-end before any UI is built on it.
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: BROWSE-01, BROWSE-02, BROWSE-03, BROWSE-04, SELECT-01, SELECT-02, SELECT-04, RESTORE-01
**Success Criteria** (what must be TRUE):

  1. Saving a folder selection through the API stores the maximal-root form — ticked parents kept, redundant descendants dropped, unchecked sub-branches below included roots stored as exclusions — and reading it back reproduces exactly which branches are included, partial, and excluded (whitelist start-state included, no special-casing)
  2. A backup run after a narrowed selection hands restic exactly the maximal-root positional paths, and the resulting snapshot's `Paths` contain the selected folders and nothing more — positionals stay the maximal-root includes; stored exclusion branches are enforced as restic `--exclude` patterns on the backup argv (gap-closure amendment 2026-09-09, superseding the original parenthetical)
  3. Existing deployments' saved `backupPaths` keep working with zero migration, and an empty list still means exactly "auto-detection" at the persistence boundary — no silent semantic drift
  4. Listing a tree node's children is a cheap per-node call that distinguishes "empty directory" from "unreadable directory" (scrubbed messages), never escapes its root boundary even through symlinks, and agrees with the existing folder browser on hidden entries
  5. Restoring an older snapshot after the user reshaped their selection completes instead of aborting mid-restore after destructive teardown; restore maps by the chosen snapshot's recorded `Paths` (longest-prefix), not by the current selection

**Plans**: 5/5 plans executed (4/4 executed + 1 gap-closure)

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Selection encoding keystone: `selection.go` pure helpers + normalized `SetBackupPaths` + reader classification + maximal-include restic positionals (SELECT-01, SELECT-02, SELECT-04)
- [x] 01-02-PLAN.md — Browse node-listing contract: os.Root containment, status trio, cap+truncated, hidden opt-in (BROWSE-01..04)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-03-PLAN.md — Restore hardening: longest-prefix mapping against the chosen snapshot's Paths, pre-teardown abort, restic 0.17 spot-checks (RESTORE-01)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 01-04-PLAN.md — Exclusion visibility + empty-selection guard (INTEG-04 backend) + PROJECT.md Key Decisions

**Wave 4** *(gap closure — WR-01, user decision 2026-09-09 "Encode")*

- [x] 01-05-PLAN.md — WR-01 gap closure: enforce stored exclusion branches as restic `--exclude` on the backup argv (positionals stay maximal-root includes) + docs drift fixes (SELECT-01, SELECT-02, SELECT-04, BROWSE-01)

**Notes**: Research flag: none — fully specified by `.planning/research/SUMMARY.md` (its Phase 1–3 merged here). During this phase, log the positional-targets Key Decision and the future-children allowlist semantic in PROJECT.md. The empty-selection PATCH-boundary guard (backend half of INTEG-04) lands here; INTEG-04 completes with documented UI semantics in Phase 3.

### Phase 2: Container Panel Tree Selection

**Goal**: In the container panel, users can unfold any discovered mount or custom path and pick subfolders with cascade semantics, mixed-state parents, and full keyboard access — and what they see on reopen is exactly what they left.
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: TREE-01, TREE-02, TREE-03, TREE-04, TREE-05, TREE-06, INTEG-01
**Success Criteria** (what must be TRUE):

  1. User unfolds a discovered mount (e.g., Plex appdata) or a custom path in the container panel; children load on expand (never an eager full-tree load), and unchecking a volatile subfolder (transcoding/caches/logs) leaves the rest of the mount selected
  2. Checking a folder includes everything below it; partially-selected parents show an indeterminate state, and re-activating a remembered-partial checkbox restores its prior partial selection
  3. Closing and reopening the panel reconstructs the saved state exactly — fully-included, partially-included, and excluded sub-branches are all visually distinguishable
  4. The whole tree is operable by keyboard (arrows to move/expand/collapse, Space to toggle) with ARIA tree/checkbox roles: `aria-expanded` on parents, `aria-checked="mixed"` for partials, `aria-level`/`setsize`/`posinset` on lazy nodes
  5. An unreadable directory renders a muted "no access" row visually distinct from a plain empty directory; expansion never hangs on an infinite spinner or silently collapses

**Plans**: 3/3 plans executed

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — Tracer: the mounts/custom list becomes a lazy tri-state tree (pure selectionTree logic, wire types caught up, live-save with selectionSource "tree", exact reopen reconstruction, 4 i18n keys x 42 locales) (TREE-01, TREE-02, TREE-03, TREE-04, INTEG-01)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-02-PLAN.md — Node outcome states + save-flow guard: TREE-06 no-access/retry rows, D-06 truncated notice, D-04 pre-PATCH zero-include block, serialized one-deep PATCH queue (TREE-06, TREE-02, INTEG-01)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-03-PLAN.md — APG keyboard + aria geometry (TREE-05), sub-include absorption + expansion restore + integration test (INTEG-01), phase gate (full suite, lint, build, committed web/dist, Go regression)

**UI hint**: yes

**Notes**: Click-on-mixed behavior resolved by CONTEXT D-01 (remembered three-state cycle) — not reopened at planning. i18n: the parity test gates ALL 42 locale tables (en + de inline + 40 lazy chunks), not two; 4 new keys budgeted in plan 01. `web/dist` rebuilt and committed in plan 03 (task 3). Zero backend changes; zero npm installs.

### Phase 3: Selection Trust & Controls

**Goal**: Users can see and control exactly what a selection will back up — per-mount effective-selection preview (including a narrowing note when a selection shrinks an item with existing snapshots), reviewable exclusions, defined semantics for deselecting everything, and a per-root CACHEDIR.TAG toggle.
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: SELECT-03, INTEG-03, INTEG-04, RESTIC-01
**Success Criteria** (what must be TRUE):

  1. For each mount, the panel shows the effective selection ("N paths") that matches exactly the positional paths the next backup will hand restic — and when a new selection narrows an item that already has snapshots, the UI says so instead of silently changing what future snapshots contain
  2. Deselected sub-branches are reviewable after the fact — a visible list near the mount, styled consistently with the existing preview
  3. Unchecking the last checked folder in a mount produces an explicit, documented outcome — the item never silently flips to the empty-list auto-detection fallback
  4. A per-mount/root CACHEDIR.TAG toggle sits with the selection controls, and a backup of that item maps it to restic `--exclude-caches` in the built argv (covered by `restic_args_test.go`)

**Plans**: 3/3 plans executed

Plans:
**Wave 1**

- [x] 03-01-PLAN.md — RESTIC-01 backend: per-root CACHEDIR.TAG toggle persistence (migration v100 + owned SetExcludeCaches), additive PATCH/mounts fields, any-root-true union via Mode.ExcludeCaches into BackupArgs `--exclude-caches` (RESTIC-01)
- [x] 03-02-PLAN.md — Read-only trust layer: rootIncludeCount/rootExclusions pure helpers, per-root "{n} paths" preview line, collapsible reviewable exclusions list, D-04 deselected-root rendering, 2 i18n keys x 42 locales (SELECT-03, INTEG-03, INTEG-04)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03-03-PLAN.md — Interactions + gate: per-attempt queue source, Reset selection (PATCH [] without tree source, confirm, refetch-on-ok), narrowing note (lastSavedCount + lastBackup), CACHEDIR toggle UI over plan 01 fields, 5 i18n keys + 2 text changes x 42 locales, phase gate (INTEG-04, SELECT-03, RESTIC-01)

**UI hint**: yes

**Notes**: The future-children semantic accepted in Phase 1 is communicated here (narrowing note under SELECT-03); the fanout "exclude this subfolder instead" affordance routing to the existing ExcludesEditor is a follow-up, not v1 scope.

### Phase 4: File Sets Parity

**Goal**: Choosing what a File Set covers uses the same collapsible tree with the same cascade/mixed-state/persistence semantics — file sets gain sub-folder granularity with zero second implementation of selection.
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: INTEG-02
**Success Criteria** (what must be TRUE):

  1. On the File Sets page, the user picks what a file set covers through the same tree component — lazy expand, cascading checkboxes, mixed-state parents, exact reconstruction on reopen
  2. A file set's tree selection round-trips through the same flat persistence and normalization (no separate format), and the file set's next backup produces a snapshot whose `Paths` match the ticked roots

**Plans**: 4/4 plans executed

Plans:
**Wave 1**

- [x] 04-01-PLAN.md — Tracer: selection round-trip keystone — migration v101 (`selected_paths` nullable) + owned setter + orchestrator `SourcePaths` + `BackupFileSet` compile (re-anchor filter, derived excludes), legacy NULL argv pinned byte-identical (INTEG-02)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 04-02-PLAN.md — `selectedPaths` PATCH boundary (containment, 64 cap, atomic, D-06 refusal, path-change clear) + FileSetView/api.ts wire mirror + D-08 restore guard via the one `mapRestorePaths` (INTEG-02)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 04-03-PLAN.md — Files-page tree editor: optional CACHEDIR props, "Choose folders" disclosure, one-root SelectionTree, NULL synthetic-root seed, serialized one-deep queue, client refusal, 4 i18n keys x 42 locales (INTEG-02)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 04-04-PLAN.md — Audit surfaces: per-root exclusions review list (D-07) + dialog path-change hint (A3) + phase gate (INTEG-02)

**UI hint**: yes

**Notes**: Decide single-root picker vs multi-root selection during planning — research recommends multi-root (strict superset, matches the Active requirement); multi-root needs only an append-only SQLite migration (`selected_paths` nullable — never edit a shipped migration).

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Selection Engine & Restore Safety | 5/5 | Complete    | 2026-09-10 |
| 2. Container Panel Tree Selection | 3/3 | Complete    | 2026-09-10 |
| 3. Selection Trust & Controls | 3/3 | Complete    | 2026-09-10 |
| 4. File Sets Parity | 4/4 | Complete    | 2026-09-11 |
