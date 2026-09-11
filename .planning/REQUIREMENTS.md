# Requirements: BombVault — Tree-Based Sub-Folder Backup Selection

**Defined:** 2026-09-09
**Core Value:** Every container, VM, and config on the host can be backed up consistently and restored completely — a dead server is rebuilt from the restic repo alone.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Tree Component & Selection Semantics

- [x] **TREE-01**: Collapsible lazy-loading tree per mount/root — children load on expand; no eager full-tree loads (appdata trees can hold thousands of entries)
- [x] **TREE-02**: Checkbox at every level; checking a folder includes everything below it; unchecking a child carves it out as an exclusion (Veeam/Duplicati cascade model)
- [x] **TREE-03**: Partially-selected parents display a mixed/indeterminate state (`aria-checked="mixed"`, derived from children); re-activating a remembered-partial checkbox restores its prior partial state
- [x] **TREE-04**: On reopen, selection state is reconstructed from the saved `backupPaths` set — fully-included, partially-included, and excluded sub-branches are all distinguishable
- [x] **TREE-05**: Full keyboard navigation + ARIA tree/checkbox roles (APG patterns: arrow expand/collapse/move, Space toggle, `aria-expanded` on parents, `aria-level`/`setsize`/`posinset` for lazy nodes)
- [x] **TREE-06**: Unreadable vs empty directories are distinguished per node — muted "no access" row vs plain empty; no infinite spinners, no silent collapses

### Browse Backend

- [x] **BROWSE-01**: Listing children of a tree node is cheap and per-node (single per-node directory listing, no eager recursion, every directory expandable, an empty directory returns ok with no children — a probe-based child-count hint was rejected as an N+1 latency multiplier per D-07), suitable for lazy expansion of huge appdata trees
- [x] **BROWSE-02**: Listing responses distinguish "empty directory" from "error reading directory", with scrubbed messages (paths → `[path]` first)
- [x] **BROWSE-03**: Listing is containment-safe — lexical/symlink-safe (`os.Root`) and never lists outside the discovered/custom root boundary; the tree must not become an arbitrary filesystem probe
- [x] **BROWSE-04**: Hidden-entry visibility is consistent between the tree and the existing folder browser, so both views agree on what they list

### Selection ↔ Persistence

- [x] **SELECT-01**: Tree state normalizes to/from the existing flat `backupPaths` set — store maximal included paths (highest ticked node) + exclusion sub-paths below included roots; drop redundant descendants
- [x] **SELECT-02**: Persistence format unchanged — zero migration; existing deployments keep their saved `backupPaths` selections working without action
- [x] **SELECT-03**: Effective-selection preview — the panel shows what will actually be backed up per mount ("N paths"), matching exactly the positional paths handed to restic (explicit sources bypass excludes, so the effective set must equal the ticked paths)
- [x] **SELECT-04**: Whitelist start-state — starting from nothing checked and ticking keep-lists works through the same normalization (no special-casing)

### Domain Integration

- [x] **INTEG-01**: Container panel — unfold any discovered mount or custom path and select subfolders (Plex `transcoding`, caches, logs) without dropping the rest of the mount
- [x] **INTEG-02**: File Sets page — the same tree component is used when choosing what a file set covers
- [x] **INTEG-03**: Exclusions are reviewable after the fact — deselected sub-branches render as a visible list near the mount, consistent with existing preview styling
- [x] **INTEG-04**: Fully deselecting a mount's tree has defined, UI-documented semantics that never silently re-trigger the empty-list auto-detection fallback (`configuredBackupPaths` treats an empty list as "no explicit selection") — backend PATCH guard landed in Phase 1 (coded empty-selection refusal, prior state preserved); completes with the documented UI semantics in Phase 3 per ROADMAP

### Restore Robustness

<!-- Added during roadmap creation (2026-09-09) from research/PITFALLS: the tree makes selection churn first-class, and today a changed path list aborts a restore mid-flight after destructive teardown. Restore hardening must precede tree-wide availability. -->

- [x] **RESTORE-01**: Restore survives selection changes — restoring a chosen snapshot intersects the stored path list with that snapshot's recorded `Paths` (longest-prefix mapping, never first-path-component), so narrowing or reshaping a selection never aborts a restore mid-flight after destructive teardown

### Restic Engine

- [x] **RESTIC-01**: Per-mount/root `CACHEDIR.TAG` toggle maps to restic `--exclude-caches` in `BackupArgs` (argv change covered by `restic_args_test.go`)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Tree Enhancements

- **TREE-07**: Search/filter within large trees (trigger: users report navigation pain on big mounts)
- **TREE-08**: Restore-side tree reusing the component over snapshot listings (different data source: snapshot `ls` vs live FS; keep component state model portable)

### Selection Enhancements

- **SELECT-05**: Per-folder size hints — on-demand "measure this folder" with cached results (background job + SQLite cache); never eager on expand
- **SELECT-06**: Backup-time coverage diff — warn when a sibling of a stored path is neither selected nor excluded (research recommendation; deferred 2026-09-09 by user decision — the Phase 3 narrowing note under SELECT-03 remains the sole future-children mitigation for v1)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Sub-folder selection for VMs | A zvol is block storage; folder granularity has no meaning there (PROJECT.md) |
| Replacing the restic ExcludesEditor | Patterns answer "which files/globs"; the tree answers "which folders" — complementary, not redundant (PROJECT.md) |
| Changing the `backupPaths` persistence format | Existing deployments must keep working without migration (PROJECT.md) |
| Auto-detection of junk folders / silent auto-exclusion | Silent non-backup is the cardinal sin of backup tools — users discover missing restores at disaster time; confirm-first suggestions may be a later effort (PROJECT.md) |
| Junk suggestion chips | Candidate for a later effort once v1 selection UX proves the pattern (PROJECT.md) |
| File-type/extension masks inside the tree | Different question (which files vs which folders); mixing them creates precedence confusion — masks stay in ExcludesEditor |
| Eager size computation on tree expand | Stalls the panel and hammers the disk on multi-GB appdata; only on-demand measurement survives (SELECT-05) |
| Nested-tree JSON persistence in SQLite | Mirrors the UI but violates the locked flat-set decision; two persistence models drift |
| Per-subfolder retention or schedules | Fragments restic snapshot identity; retention is per-item tag and must never regroup (#91, live `tag` discipline) |
| Arbitrary-path browsing from the tree | Defeats boundary validation and error-scrubbing discipline; expands `/api/browse` attack surface — custom paths remain an explicit add |
| Per-subfolder "last backed up" indicators | Snapshots aren't cheaply path-indexed; a folder "backed up" at snapshot time may have been excluded — misleading |
| Compose project dir sub-folder selection beyond per-stack backup | Covered by issue #189's per-stack design (PROJECT.md) |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| BROWSE-01 | Phase 1 | Complete |
| BROWSE-02 | Phase 1 | Complete |
| BROWSE-03 | Phase 1 | Complete |
| BROWSE-04 | Phase 1 | Complete |
| SELECT-01 | Phase 1 | Complete |
| SELECT-02 | Phase 1 | Complete |
| SELECT-04 | Phase 1 | Complete |
| RESTORE-01 | Phase 1 | Complete |
| TREE-01 | Phase 2 | Complete |
| TREE-02 | Phase 2 | Complete |
| TREE-03 | Phase 2 | Complete |
| TREE-04 | Phase 2 | Complete |
| TREE-05 | Phase 2 | Complete |
| TREE-06 | Phase 2 | Complete |
| INTEG-01 | Phase 2 | Complete |
| SELECT-03 | Phase 3 | Complete |
| INTEG-03 | Phase 3 | Complete |
| INTEG-04 | Phase 3 | Complete |
| RESTIC-01 | Phase 3 | Complete |
| INTEG-02 | Phase 4 | Complete |

**Coverage:**

- v1 requirements: 20 total (19 original + RESTORE-01 added during roadmap creation)
- Mapped to phases: 20
- Unmapped: 0 ✓

---
*Requirements defined: 2026-09-09*
*Last updated: 2026-09-09 — roadmap created; RESTORE-01 added from research; traceability populated*
