# Roadmap: BombVault

## Milestones

- ✅ **v1.0 Tree-Based Sub-Folder Backup Selection** — Phases 1-4 (shipped 2026-09-11)

## Phases

<details>
<summary>✅ v1.0 (Phases 1-4) — SHIPPED 2026-09-11</summary>

- [x] **Phase 1: Selection Engine & Restore Safety** (5 plans) — Selections normalize losslessly into the unchanged flat `backupPaths` set, node listings are cheap and containment-safe, and restores survive selection changes — the risky keystone (maximal-roots ↔ restic positional targets) proven end-to-end before any UI. Full details: `.planning/milestones/v1.0-phases/01-selection-engine-restore-safety/`
- [x] **Phase 2: Container Panel Tree Selection** (3 plans) — In the container panel, users can unfold any discovered mount or custom path and pick subfolders with cascade semantics, mixed-state parents, and full keyboard access — and what they see on reopen is exactly what they left. Full details: `.planning/milestones/v1.0-phases/02-container-panel-tree-selection/`
- [x] **Phase 3: Selection Trust & Controls** (3 plans) — Users can see and control exactly what a selection will back up — per-mount effective-selection preview with narrowing note, reviewable exclusions, defined deselect-everything semantics, and a per-root CACHEDIR.TAG toggle. Full details: `.planning/milestones/v1.0-phases/03-selection-trust-controls/`
- [x] **Phase 4: File Sets Parity** (4 plans) — Choosing what a File Set covers uses the same collapsible tree with the same cascade/mixed-state/persistence semantics — file sets gain sub-folder granularity with zero second implementation of selection. Full details: `.planning/milestones/v1.0-phases/04-file-sets-parity/`

</details>
