# Roadmap: BombVault — Tree-Based Sub-Folder Backup Selection

## Overview

This milestone replaces mount-granularity backup selection with tree-based per-folder selection over the same flat `backupPaths` set — no migration, two presentations of one persistence model. The order is risk-first MVP slicing: Phase 1 proves the keystone end-to-end at the engine level (lossless normalization to maximal roots handed to restic as positional targets, cheap containment-safe per-node listing, restores that survive selection changes) before any UI leans on it. Phase 2 lands the visible capability where the pain is — the container panel tree (Plex `transcoding`, caches, logs). Phase 3 makes selections legible and controllable: effective-selection preview, reviewable exclusions, defined empty-deselect semantics, and the CACHEDIR.TAG toggle. Phase 4 carries the same tree to File Sets. Every phase leaves existing deployments fully working.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Selection Engine & Restore Safety** - Normalize selections losslessly into the unchanged flat `backupPaths`, harden per-node listing, and make restores survive selection changes — proven end-to-end at the API/backup level before any UI is built on it.
- [ ] **Phase 2: Container Panel Tree Selection** - The lazy tri-state tree lands in the container panel: unfold a mount, tick subfolders, keyboard-accessible, state exact on reopen.
- [ ] **Phase 3: Selection Trust & Controls** - Users can see and control what will be backed up: effective-selection preview, reviewable exclusions, defined empty-deselect semantics, per-root CACHEDIR.TAG toggle.
- [ ] **Phase 4: File Sets Parity** - The same tree selection powers File Sets coverage.

## Phase Details

### Phase 1: Selection Engine & Restore Safety

**Goal**: Selections made through the API normalize losslessly into the unchanged flat `backupPaths` set, node listings are cheap and containment-safe, and restores survive selection changes — the risky keystone (maximal-roots ↔ restic positional targets) proven end-to-end before any UI is built on it.
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: BROWSE-01, BROWSE-02, BROWSE-03, BROWSE-04, SELECT-01, SELECT-02, SELECT-04, RESTORE-01
**Success Criteria** (what must be TRUE):

  1. Saving a folder selection through the API stores the maximal-root form — ticked parents kept, redundant descendants dropped, unchecked sub-branches below included roots stored as exclusions — and reading it back reproduces exactly which branches are included, partial, and excluded (whitelist start-state included, no special-casing)
  2. A backup run after a narrowed selection hands restic exactly the maximal-root positional paths, and the resulting snapshot's `Paths` contain the selected folders and nothing more (explicit targets only — never exclude-based encoding)
  3. Existing deployments' saved `backupPaths` keep working with zero migration, and an empty list still means exactly "auto-detection" at the persistence boundary — no silent semantic drift
  4. Listing a tree node's children is a cheap per-node call that distinguishes "empty directory" from "unreadable directory" (scrubbed messages), never escapes its root boundary even through symlinks, and agrees with the existing folder browser on hidden entries
  5. Restoring an older snapshot after the user reshaped their selection completes instead of aborting mid-restore after destructive teardown; restore maps by the chosen snapshot's recorded `Paths` (longest-prefix), not by the current selection

**Plans**: 3/4 plans executed

Plans:
**Wave 1**

- [x] 01-01-PLAN.md — Selection encoding keystone: `selection.go` pure helpers + normalized `SetBackupPaths` + reader classification + maximal-include restic positionals (SELECT-01, SELECT-02, SELECT-04)
- [x] 01-02-PLAN.md — Browse node-listing contract: os.Root containment, status trio, cap+truncated, hidden opt-in (BROWSE-01..04)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 01-03-PLAN.md — Restore hardening: longest-prefix mapping against the chosen snapshot's Paths, pre-teardown abort, restic 0.17 spot-checks (RESTORE-01)

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 01-04-PLAN.md — Exclusion visibility + empty-selection guard (INTEG-04 backend) + PROJECT.md Key Decisions

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

**Plans**: TBD
**UI hint**: yes

**Notes**: Spec the click-on-mixed behavior (fill vs cycle) during planning — APG allows either; research expects fill. `--research-phase` only if deeper ARIA-tree detail is wanted; otherwise STACK.md/APG guidance suffices. Both i18n locales; commit `web/dist`.

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

**Plans**: TBD
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

**Plans**: TBD
**UI hint**: yes

**Notes**: Decide single-root picker vs multi-root selection during planning — research recommends multi-root (strict superset, matches the Active requirement); multi-root needs only an append-only SQLite migration (`selected_paths` nullable — never edit a shipped migration).

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Selection Engine & Restore Safety | 3/4 | In Progress|  |
| 2. Container Panel Tree Selection | 0/TBD | Not started | - |
| 3. Selection Trust & Controls | 0/TBD | Not started | - |
| 4. File Sets Parity | 0/TBD | Not started | - |
