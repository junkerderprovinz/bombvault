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

### Active

<!-- Current scope. Building toward these. -->

- [ ] Tree-based per-folder backup selection: collapsible lazy-loading subtree per mount/root, checkboxes at every level, mixed-state parent for partial selections
- [ ] Container panel: unfold a mount, uncheck volatile subfolders (transcoding, caches, logs) without dropping the rest of the mount
- [ ] File Sets: same tree selection when choosing what a file set covers
- [ ] Selection persists through the existing flat `backupPaths` set — the tree is a new view over the same data, no new persistence model

### Out of Scope

- Sub-folder selection for VMs — a zvol is block storage; folder granularity has no meaning there
- Removing or replacing the restic ExcludesEditor — file/regex patterns and folder selection are complementary, not redundant
- Changing the `backupPaths` persistence format — existing deployments must keep working without migration
- Auto-detection of "junk" folders (heuristic cache/transcoding suggestions) — suggestions may come later; not part of this effort
- Per-subfolder selection for stacks' compose project dirs beyond what per-stack backup already does — covered by issue #189's per-stack design

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
*Last updated: 2026-09-09 after initialization*
