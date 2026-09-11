# BombVault

## What This Is

Backup and full disaster recovery for Unraid servers (Docker containers, KVM VMs, USB flash, appdata/config), also targeting TrueNAS Scale and generic Docker hosts. A Go backend drives restic as the storage engine behind a React/Vite SPA: discovery, one-click and scheduled backups, guided restores, retention, and off-site replication, shipped as a multi-arch Docker image. Container mounts and file sets both offer tree-based per-folder selection (lazy tri-state tree over the flat `backupPaths` set) so volatile subfolders like `transcoding` and caches can be excluded without dropping the mount.

## Core Value

Every container, VM, and config on the host can be backed up consistently and restored completely — a dead server is rebuilt from the restic repo alone.

## Requirements

### Validated

<!-- Inferred from the existing codebase (see .planning/codebase/); shipped and relied upon. -->

- ✓ Container discovery + consistent backup (stop → restic → restart, compose dependency ordering, health-gating, pre/post hooks, always-undo restart guarantee) — existing
- ✓ Automatic per-container data-path discovery (appdata-segment bind matching, named volumes, `bombvault.data` label, appdata fallback; compose project dir backed up once per stack, #189) — existing
- ✓ Per-container explicit folder selection: mount checkboxes + custom paths, live-saved (`backupPaths`) — existing
- ✓ Per-container restic exclude patterns with live preview and one-click suggestions (ExcludesEditor) — existing
- ✓ VM (zvol) backups incl. live snapshots (`live` tag), and USB flash backup — existing
- ✓ File Sets: arbitrary directory backups with root picker and exclude patterns — existing
- ✓ Self-config backup, restore guard chain (confirmation, snapshot-id validation, path traversal guards, IP/port conflict pre-flight, cross-instance remap) — existing
- ✓ Identity-stable per-item retention (tags `container:<ref>`/`vm:<name>`/`fileset:<name>`/`flash`/`config`, ungrouped) — existing
- ✓ Off-site replication (rclone/s3/sftp/…), append-only/immutable repos never pruned locally — existing
- ✓ Cron scheduler per domain + "Backup Everything", catch-up and due-gates — existing
- ✓ Live progress via SSE, notifications (webhook/Matrix/Healthchecks/Unraid/email) — existing
- ✓ React SPA: dashboard, per-domain pages, backup order, i18n, dark mode — existing
- ✓ Multi-arch Docker image (amd64+arm64) for Unraid / TrueNAS Scale / generic hosts, self-signed TLS, optional login — existing
- ✓ Selection engine & restore safety: lossless flat `backupPaths` encoding (bare + `!`-prefixed entries, zero migration), maximal-root restic positionals with descendant `--exclude` enforcement (WR-01 gap closure 01-05), hardened `/api/browse` listing (os.Root containment, status trio, cap 500 + truncated), restore longest-prefix mapping with pre-teardown abort — Phase 1
- ✓ Selection persists through the existing flat `backupPaths` set — the tree is a new view over the same data, no new persistence model — Phase 1
- ✓ Tree-based per-folder selection in the container panel: lazy-loading subtree per mount/root, tri-state checkboxes (mixed parents), remembered partial via dormant exclusions, unreadable-vs-empty per-node outcome rows, full APG keyboard operation — Phase 2
- ✓ Container panel volatile-subfolder deselection: unfold a mount, uncheck transcoding/caches/logs without dropping the rest — maximal-root includes with stored exclusions enforced on the backup argv (Phase 1 engine, one-deep serialized save queue client-side) — Phase 2
- ✓ Selection trust & controls: per-root "{n} paths" preview agreeing with the restic positionals + narrowing note on shrink (SELECT-03), per-root reviewable exclusions list (INTEG-03), defined empty-deselect semantics (client guard + fail-tone confirmed Reset as the one sanctioned exit, INTEG-04), per-root CACHEDIR.TAG toggle mapping to restic `--exclude-caches` (RESTIC-01) — Phase 3
- ✓ File Sets parity: the same `SelectionTree` component powers file-set coverage — NULL `selectedPaths` seeds a synthetic root include with zero writes, PATCH is a three-state pointer (64-cap, segment-aligned containment, empty-selection refusal), path edits clear the selection atomically (clear-wins), and in-place restore maps the compiled selection against snapshot Paths with a pre-teardown abort (D-08) — Phase 4

### Active

<!-- Current scope. Building toward these. -->

- [ ] SELECT-06: backup-time coverage diff (what changed since the last snapshot's selection)
- [ ] TREE-07: tree search/filter; TREE-08: restore-side tree picker; SELECT-05: size hints per node

### Out of Scope

- Sub-folder selection for VMs — a zvol is block storage; folder granularity has no meaning there
- Removing or replacing the restic ExcludesEditor — file/regex patterns and folder selection are complementary, not redundant
- Changing the `backupPaths` persistence format — existing deployments must keep working without migration
- Auto-detection of "junk" folders (heuristic cache/transcoding suggestions) — suggestions may come later; not part of this effort
- Per-subfolder selection for stacks' compose project dirs beyond what per-stack backup already does — covered by issue #189's per-stack design
- Fanout affordance from the tree into the ExcludesEditor ("exclude this subfolder instead") — noted as a follow-up at Phase 3 verification; the per-root exclusions disclosure is the v1 review surface, and the snapshot-content-comparison alternative is prohibited

## Context

- Brownfield repo; codebase map lives in `.planning/codebase/` (STACK, ARCHITECTURE, STRUCTURE, CONVENTIONS, TESTING, INTEGRATIONS, CONCERNS — refreshed 2026-09-09).
- Shipped v1.0 (2026-09-11): tree-based sub-folder selection is live on BOTH container mounts and file sets. One normalization contract (`internal/api/selection.go`) and one tree component (`web/src/components/SelectionTree.tsx` + `web/src/lib/selectionTree.ts`) serve both domains; selection compiles to maximal-root restic positionals with descendant branches enforced as `--exclude`, `--exclude-caches` rides the per-root CACHEDIR union, and restores map selections onto snapshot Paths (longest-prefix, pre-teardown abort).
- Typical pain solved: Plex-style appdata where `transcoding`/cache subfolders dwarf the real config — untick them per-node instead of dropping the mount; snapshots stop carrying data nobody wants restored.
- Known deferred debt: 2 pre-existing eslint warnings (ActivityLog.tsx:234, Sidebar.tsx:567, warn-only), the stacked-descriptor failure-revert window in the Containers save queue (narrow window, next successful save re-converges), and v2 requirements (SELECT-06, TREE-07/08, SELECT-05) tracked in REQUIREMENTS.md.
- Public repo (has always been public): no real user data or IPs anywhere, including examples.

## Constraints

- **Tech stack**: Go stdlib `net/http` only (no router/framework); SPA with no state library, no UI kit — the tree UI must be hand-rolled in line with existing components.
- **Storage engine**: restic only; `internal/backup` stays isolated from adapters (ports-and-adapters seam is enforced by review tests).
- **Retention identity**: per-item tags, ungrouped — NEVER reintroduce `--group-by paths` (#91) or global `--group-by tags` (live VM snapshots).
- **SQLite migrations**: append-only numbering; never edit a shipped migration (`:latest` publishes on every push to main).
- **Security discipline**: boundary validation (`resourceNameRe`, 1 MiB bodies, DisallowUnknownFields), error scrubbing (paths → `[path]` first), argv discipline (user-influenced positionals after `--`), credentials via env only.
- **Process**: `go build/vet/gofmt/golangci-lint/test` before every push; web build (`tsc --noEmit && vite build`, commit `web/dist`) required after any `web/` change since the SPA is embedded; async tests must wait for detached goroutines.
- **Releases**: SemVer 3-digit; never tag without explicit approval.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Tree selection = new view over the existing flat `backupPaths` set | Zero-migration compatibility with deployed instances; one persistence model, two presentations | Decided (Phase 1) |
| Keep ExcludesEditor (restic patterns) alongside folder tree selection | Selection answers "which folders"; exclude patterns answer "which files/globs" — different questions | ✓ Good (v1.0: the per-root exclusions disclosure is the review surface; fanout stays a follow-up) |
| Lazy-load tree children from a server directory listing (extend `/api/browse`) | Appdata trees can be huge; eager full-tree loads would stall the panel and the backend | ✓ Good (consumed by Phase 2 + Phase 4 trees; browse hardened Phase 1) |
| Selections compile to maximal-root restic positional targets; exclusions encode as `!`-prefixed entries in the same flat list, and exclusion branches under included roots are enforced as restic `--exclude` on the backup argv (gap closure 01-05, user decision 2026-09-09) | restic excludes DO filter content within positional sources (contract-proven by TestPositionalExcludesKeepSourceDir); the original claim was corrected as review finding WR-02. Maximal roots stay positionals so snapshot Paths keep stable restore selectors, and encoding stored exclusions as `--exclude` enforces the deselection content-wise — accepting that derived patterns land in the snapshot's restic Excludes metadata (the exclusions editor's user-owned surface) | Decided (Phase 1); exclusion enforcement amended by 01-05 |
| Future children of an included root are included (allowlist semantic) | narrowing selections communicate the future-children allowlist semantic; UI communication lands with Phase 3 SELECT-03 | Accepted (Phase 1) |
| Tree node states derive purely from the (includes, exclusions) host-path sets — never from loaded children | Collapsed and never-loaded subtrees classify correctly with zero browsing; the exclusion list below a node IS the remembered-partial memory, so no second UI-side memory can drift | Decided (Phase 2) |
| FoldersEditor saves serialize through a one-deep PATCH queue; revert re-derives from the live mirror by set-difference inverse | Rapid toggles collapse to one draining request carrying the latest full list; a newer toggle always survives a failing save (never a captured snapshot) | Decided (Phase 2) |
| Keyboard Space routes through the identical onToggle pipeline as checkbox clicks | One toggle semantics, one D-04 zero-include guard, one save queue — an alternate input path can never bypass a client-side guard | Decided (Phase 2) |
| Per-root CACHEDIR.TAG toggles compile to the item-level boolean union `anyRootExcludeCaches` → one constant `--exclude-caches` flag; map keys never reach argv | restic.Mode precedent (Limits); no user-controlled string crosses the argv boundary (toContainerPath containment + 64-entry cap on the keys); the container-wide scope is disclosed in the UI tooltip, not hidden | Decided (Phase 3) |
| Reset selection = no-source `{backupPaths: [], excludeCaches: {}}` through the serialized queue, fail-tone confirm naming both consequences | Passes the tree-gated Phase 1 guard by omission — the one sanctioned exit to auto-detection; a refused deselect never silently flips the item | ✓ Good (verified Phase 3) |
| File-set `selectedPaths` PATCH is a three-state pointer field (absent = untouched, list = atomic validated overwrite, 64-cap + containment) | Absent-untouched means untouched-until-toggle NULL-seeding never writes; explicit shape beats payload sniffing (INTEG-04 Q1 precedent) | ✓ Good (verified Phase 4, 04-UAT 22/22) |
| A file-set path edit clears the selection to SQL NULL (clear-wins, compared on resolved roots); compile re-anchors stale entries against the freshly resolved root as layer 2 | Silent backup-scope broadening (T-04-05) needs two independent layers; NULL keeps the legacy mono-SourceDir compile byte-identical | ✓ Good (verified Phase 4; post-review WR-01 made it one atomic statement) |
| Files-page tree reuses `SelectionTree` via additive-optional props only; the exclusions audit list rides the shared Phase 3 disclosure instead of a second surface | One tree, one exclusion review implementation; INTEG-02 satisfied by construction, not by convention | ✓ Good (integration checker: single implementation, no duplicated "!" logic in web/src) |

## Evolution

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-09-11 after v1.0 milestone*
