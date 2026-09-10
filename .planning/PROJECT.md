# BombVault

## What This Is

Backup and full disaster recovery for Unraid servers (Docker containers, KVM VMs, USB flash, appdata/config), also targeting TrueNAS Scale and generic Docker hosts. A Go backend drives restic as the storage engine behind a React/Vite SPA: discovery, one-click and scheduled backups, guided restores, retention, and off-site replication, shipped as a multi-arch Docker image.

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

### Active

<!-- Current scope. Building toward these. -->

- [ ] File Sets: same tree selection when choosing what a file set covers

### Out of Scope

- Sub-folder selection for VMs — a zvol is block storage; folder granularity has no meaning there
- Removing or replacing the restic ExcludesEditor — file/regex patterns and folder selection are complementary, not redundant
- Changing the `backupPaths` persistence format — existing deployments must keep working without migration
- Auto-detection of "junk" folders (heuristic cache/transcoding suggestions) — suggestions may come later; not part of this effort
- Per-subfolder selection for stacks' compose project dirs beyond what per-stack backup already does — covered by issue #189's per-stack design
- Fanout affordance from the tree into the ExcludesEditor ("exclude this subfolder instead") — noted as a follow-up at Phase 3 verification; the per-root exclusions disclosure is the v1 review surface, and the snapshot-content-comparison alternative is prohibited

## Context

- Brownfield repo; codebase map lives in `.planning/codebase/` (STACK, ARCHITECTURE, STRUCTURE, CONVENTIONS, TESTING, INTEGRATIONS, CONCERNS — refreshed 2026-09-09).
- The current container folder selector (`web/src/pages/Containers.tsx`, folder-picker component) works at mount granularity only: a bind mount is all-or-nothing, with custom paths as the only fine-grained escape hatch. Server-side discovery is `resolveAppdataPaths` (`internal/api/service.go:3550`); persistence is a flat list of host paths via `PATCH /api/containers/{name}` (`backupPaths`).
- restic already accepts per-backup `--exclude` args (`BackupArgs`, `internal/restic/restic.go:361`), but tree selection should flow through explicit path selection, keeping excludes for file/regex patterns.
- A server-side directory listing endpoint already exists (`GET /api/browse`, `internal/api/api.go:257`) with traversal guards — the lazy tree can build on it.
- Typical pain being solved: Plex-style appdata where `transcoding`/cache subfolders dwarf the real config; backups get hot and slow (#189 precedent), snapshots carry data nobody wants restored.
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
| Keep ExcludesEditor (restic patterns) alongside folder tree selection | Selection answers "which folders"; exclude patterns answer "which files/globs" — different questions | — Pending |
| Lazy-load tree children from a server directory listing (extend `/api/browse`) | Appdata trees can be huge; eager full-tree loads would stall the panel and the backend | Backend landed (Phase 1) |
| Selections compile to maximal-root restic positional targets; exclusions encode as `!`-prefixed entries in the same flat list, and exclusion branches under included roots are enforced as restic `--exclude` on the backup argv (gap closure 01-05, user decision 2026-09-09) | restic excludes DO filter content within positional sources (contract-proven by TestPositionalExcludesKeepSourceDir); the original claim was corrected as review finding WR-02. Maximal roots stay positionals so snapshot Paths keep stable restore selectors, and encoding stored exclusions as `--exclude` enforces the deselection content-wise — accepting that derived patterns land in the snapshot's restic Excludes metadata (the exclusions editor's user-owned surface) | Decided (Phase 1); exclusion enforcement amended by 01-05 |
| Future children of an included root are included (allowlist semantic) | narrowing selections communicate the future-children allowlist semantic; UI communication lands with Phase 3 SELECT-03 | Accepted (Phase 1) |
| Tree node states derive purely from the (includes, exclusions) host-path sets — never from loaded children | Collapsed and never-loaded subtrees classify correctly with zero browsing; the exclusion list below a node IS the remembered-partial memory, so no second UI-side memory can drift | Decided (Phase 2) |
| FoldersEditor saves serialize through a one-deep PATCH queue; revert re-derives from the live mirror by set-difference inverse | Rapid toggles collapse to one draining request carrying the latest full list; a newer toggle always survives a failing save (never a captured snapshot) | Decided (Phase 2) |
| Keyboard Space routes through the identical onToggle pipeline as checkbox clicks | One toggle semantics, one D-04 zero-include guard, one save queue — an alternate input path can never bypass a client-side guard | Decided (Phase 2) |
| Per-root CACHEDIR.TAG toggles compile to the item-level boolean union `anyRootExcludeCaches` → one constant `--exclude-caches` flag; map keys never reach argv | restic.Mode precedent (Limits); no user-controlled string crosses the argv boundary (toContainerPath containment + 64-entry cap on the keys); the container-wide scope is disclosed in the UI tooltip, not hidden | Decided (Phase 3) |
| Reset selection = no-source `{backupPaths: [], excludeCaches: {}}` through the serialized queue, fail-tone confirm naming both consequences | Passes the tree-gated Phase 1 guard by omission — the one sanctioned exit to auto-detection; a refused deselect never silently flips the item | Decided (Phase 3) |

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
*Last updated: 2026-09-10 after Phase 3*
