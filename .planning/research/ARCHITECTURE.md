# Architecture Research: Lazy-Loading Directory Tree Selection

**Domain:** Backup folder selection (containers + file sets) integrating into BombVault's existing ports-and-adapters architecture
**Researched:** 2026-09-09
**Confidence:** HIGH (all integration seams read directly from source; restic behavior confirmed against upstream docs at MEDIUM)

This is a brownfield milestone: the existing system is NOT re-researched (see `.planning/codebase/ARCHITECTURE.md`). This document answers one question — how a lazy-loading directory tree integrates with the existing HTTP → Service → orchestrator → engine pipeline — across the four sub-decisions: listing endpoint, selection→restic-paths translation, selection persistence, and path-namespace translation.

---

## Executive Answer

The tree is a **pure new view over existing data and endpoints**. No new persistence model, no new restic argv shape, and (for containers) no new endpoint are required:

1. **Listing:** extend `/api/browse` (one small opt-in addition — return hidden entries), never a per-container endpoint. The tree's browsable universe (paths under HostMountRoot) is by construction identical to the backable universe (paths `toContainerPath` accepts), so one generic endpoint serves every consumer.
2. **Selection model:** store the flat set of **maximal checked roots** in the existing `targets.selected_paths` via the existing `SetBackupPaths` contract. These compile verbatim to restic positional paths (the explicit-children model), NOT to `--exclude` patterns. Server-side normalization (drop any listed path with a listed ancestor) keeps snapshots canonical and prevents restic double-walks.
3. **Persistence:** nothing moves. The question's premise ("SQLite settings via MutateSettings") is slightly wrong for this data — selection lives in the `targets` table under its own owned-setter contract (`store.SetBackupPaths`), the targets-table analogue of `MutateSettings`. Zero schema change for containers.
4. **Path namespaces:** three namespaces exist (browse-relative / host / container-visible) and they share ONE relative subpath; all translation is prefix arithmetic with `HostSourceRoot`/`HostMountRoot`, both of which `GET /api/containers/{name}/mounts` already ships to the SPA.

The one genuine architectural risk this feature introduces is **selection churn over time colliding with restore**: restore replays the *last* run's stored path list against *whichever* snapshot the user picks, and fails fast mid-restore if a stored path is absent from that snapshot. A hardening step (intersect the stored list with the chosen snapshot's recorded `Paths`) should be part of this milestone's build order.

---

## System Overview: How the Tree Integrates

```
┌─────────────────────────────────────────────────────────────────────────┐
│ SPA (web/src)                                                           │
│                                                                         │
│  SelectionTree (NEW, lazy)          FoldersEditor (Containers.tsx)       │
│    │ on-expand                          │ owns save                     │
│    ├─ browse(sub) ──────────┐           │ toggle/add → persistPaths()   │
│    │                        │           └─→ PATCH /api/containers/{name}│
│    └─ getContainerMounts() ─┼─→ GET /api/containers/{name}/mounts        │
│                             │       {mounts[], custom[],                 │
│                             │        hostMountRoot, hostSourceRoot}      │
├─────────────────────────────┼───────────────────────────────────────────┤
│ HTTP layer (internal/api)   │                                           │
│  GET /api/browse (EXTEND:   │  PATCH handler (UNCHANGED wire format)    │
│   ?hidden=1 opt-in)         │  handlePatchContainer → svc.SetBackupPaths│
│  handleBrowse: paths.Resolve│  (body.backupPaths = HOST paths)          │
│  guard, os.ReadDir, dirs    │                                           │
├─────────────────────────────┴───────────────────────────────────────────┤
│ Service (internal/api/service.go)                                       │
│  SetBackupPaths:  host path → toContainerPath → NORMALIZE (NEW, pure:   │
│                   drop paths nested under other paths) → store setter   │
│  ContainerMounts: mounts + custom annotated for the tree (UNCHANGED)    │
│  Backup:          effectiveBackupPaths → BackupDeps.AppdataPaths        │
│  prepareRestoreForTarget: (HARDEN) intersect stored paths with chosen   │
│                   snapshot's recorded Paths                             │
├─────────────────────────────────────────────────────────────────────────┤
│ Store (internal/store/targets.go)  — UNCHANGED                          │
│  SetBackupPaths: owned setter, targets.selected_paths JSON              │
│  (container-visible paths; UpsertTarget never resets it)                │
├─────────────────────────────────────────────────────────────────────────┤
│ Orchestrator (internal/backup)  — UNCHANGED                             │
│  BackupContainer: Restic.Backup(d.AppdataPaths, tags, excludes)         │
├─────────────────────────────────────────────────────────────────────────┤
│ restic (internal/restic)  — UNCHANGED                                   │
│  BackupArgs: -- <paths...> positionals; per-line --exclude untouched    │
└─────────────────────────────────────────────────────────────────────────┘
```

**Direction of information flow:** downward for everything (SPA requests listings and pushes selections; service derives; store persists; orchestrator derives at backup time from store + live inspect). The tree introduces no new upward or lateral flow.

### Component Boundaries

| Component | Responsibility for this feature | Talks to | Changes? |
|-----------|--------------------------------|----------|----------|
| `SelectionTree` (new SPA component) | Lazy expand/collapse; tri-state checkboxes; produces the maximal-roots set in host paths | `browse()`, `getContainerMounts()` via `lib/api.ts`; parent owns save | NEW (`web/src/components/`, SnapshotFileTree as structural precedent — but lazy, not eager) |
| `FoldersEditor` (Containers.tsx) | Owns the checked set + live-save (optimistic toggle, revert+shake); roots = mounts + custom paths | `setBackupPaths` PATCH | EXTEND (swap mount-checkbox list for tree; same save funnel) |
| `GET /api/browse` | One-level directory listing under HostMountRoot, traversal-guarded | `paths.Resolve`, `os.ReadDir` | EXTEND (opt-in hidden entries; otherwise unchanged) |
| `handlePatchContainer` → `SetBackupPaths` | Boundary validation; host→container translation; dedupe | store setter | EXTEND (add pure normalization step; wire format unchanged) |
| `store.SetBackupPaths` | Owned partial-write of `targets.selected_paths`; empty = auto-detect | SQLite (single conn) | UNCHANGED |
| `Backup` / `effectiveBackupPaths` | Explicit selection else auto-discovery, existence-filtered | store, docker inspect, orchestrator | UNCHANGED |
| `backup.BackupContainer` | stop→snapshot→restart around `Restic.Backup(paths, tags, excludes)` | ports only | UNCHANGED |
| `restic.BackupArgs` | argv: `--tag` per identity tag, `--exclude` per user pattern, `-- <paths>` | restic binary | UNCHANGED |
| `prepareRestoreForTarget` / `RestorePaths` | Per-subtree restore of stored paths | store, restic engine | HARDEN (snapshot-Paths intersection) |
| File Sets storage (`file_sets.path`) | Root folder per set | store | DECISION POINT (single-root vs multi-root; see below) |

---

## The Three Path Namespaces (the load-bearing fact)

Every path in this feature exists in exactly three namespaces, and they **share the same relative suffix**:

| Namespace | Example (Unraid split-root default) | Where it appears |
|-----------|-------------------------------------|------------------|
| Browse-relative (under HostMountRoot) | `user/appdata/plex` | `GET /api/browse` requests and responses; `file_sets.path`; FolderBrowser values |
| HOST path | `/mnt/user/appdata/plex` | UI display (`MountInfo.Source`, `CustomPath.Path`); **PATCH `backupPaths` wire format** |
| Container-visible path | `/host/user/user/appdata/plex` | **`targets.selected_paths` (stored)**; restic positionals; `snapshot.Paths` |

(The apparent doubling — `/host/user/user/...` — is correct: host `/mnt` is bind-mounted at container `/host/user`, so host `/mnt/user/...` lands at `/host/user/user/...`.)

**Translation is pure prefix arithmetic with the same subpath:**

```
host            = HostSourceRoot + "/" + rel        (toHostPath inverse direction)
container       = HostMountRoot  + "/" + rel        (toContainerPath direction)
rel             = strip HostSourceRoot prefix from host path   (what FoldersEditor does today)
```

- `toContainerPath` / `toHostPath` (`internal/api/service.go:1137`, `:3684`) are the service-side seams — keep using them; do not re-implement prefix logic in handlers or the SPA's Go-free world.
- The SPA needs `hostSourceRoot` + `hostMountRoot` to convert between browse-relative and host forms — **`GET /api/containers/{name}/mounts` already returns both** (`internal/api/handlers.go:1301-1306`), so no new metadata endpoint.
- Consequence worth stating as an invariant: *a path is backable ⟺ it is under HostSourceRoot ⟺ its browse-relative form is listable by `/api/browse`*. The tree can never offer a path the backup engine cannot reach, and vice versa. This is the core argument for a single generic listing endpoint.
- All paths in this domain are POSIX (`internal/paths/paths.go` note) regardless of the dev machine's OS.

## Listing Endpoint Design: extend `/api/browse`, not per-container

**Recommendation: extend the existing generic endpoint** (already the pending decision in PROJECT.md). Reasons grounded in code:

1. **The tree content is container-independent.** What the tree shows is a function of HostMountRoot only. The container-specific data (which roots exist, what's checked, what's stale) lives in `GET /api/containers/{name}/mounts` and the stored selection. A per-container listing endpoint would duplicate the traversal guard, hidden-entry filter, and sorting for zero additional information.
2. **The guard chain is already right.** `handleBrowse` (`internal/api/handlers.go:4143`) resolves every request through `paths.Resolve` (rejects traversal and absolute subpaths, logs nothing user-controlled), returns the generic `{ok:false}` envelope on unreadable dirs (a per-node error state for the tree, not a broken panel), and is already behind `csrfGate`/`authGate` like everything under `/api`.
3. **Dirs-only is exactly right for folder-granularity selection — and matches restic.** `handleBrowse` skips non-dirs and dot-prefixed entries. restic archives symlinks as symlink nodes without following them (upstream docs, MEDIUM confidence), and `DirEntry.IsDir()` is false for symlinks — so the tree offering only real directories mirrors precisely what restic will traverse. Files are not selectable by scope; don't add them.
4. **Cost profile fits lazy loading.** One `os.ReadDir` per expand, bounded by immediate children; no recursion. This is the same property the excludes assistant relies on to avoid 504s on huge appdata trees (`internal/api/excludes_suggest.go`). No caching or streaming needed at LAN scale.

**The one extension the tree actually needs — hidden entries.** The uncheck-inside-checked-parent rewrite (below) replaces a stored parent with its children; `handleBrowse` currently hides dot-prefixed entries (`handlers.go:4182`), so a hidden child would be silently *deselected* by a rewrite the user experience says means "everything except the one I unchecked" — restic does back up dot-directories, so that would be a silent coverage gap. Add an opt-in query flag (e.g. `GET /api/browse?path=…&hidden=1`) that returns hidden dirs too; destination pickers keep the default. This is a two-line, additive, backward-compatible change and is the only server-side listing work this milestone needs.

**Client-side expand behavior:** render expand arrows optimistically (a dir may or may not have children), fetch children on first expand, cache per session, show the `{ok:false}` message inline in the node. No has-children hint is needed from the server.

## Selection → restic paths: explicit maximal roots (NOT parent + `--exclude`)

**Recommendation (matches the pending PROJECT.md decision, now with the mechanical justification):** the checked set compiles to explicit positional paths. Store and back up the set of **maximal checked roots**: path p ∈ S iff the user checked p (or an ancestor that implies it), and no ancestor of p is in S.

### The state model and its three interactions

- **Check a folder:** add it; drop any stored descendants (normalization). Ancestors stay implied.
- **Check a folder inside a checked parent:** parent is stored; the check is a no-op in S (already implied) — the tree only needs to visually reflect it.
- **Uncheck a folder p whose ancestor a is stored:** replace a with a's complete children minus p. The client can always do this exactly: p is only rendered after its parent expanded, and expansion already loaded the *complete* child list (browse returns all siblings, unpaginated). Hidden siblings must be included — hence the `?hidden=1` extension above.
- **Uncheck a checked or mixed node with no stored ancestor:** remove it and all stored descendants.

**Server-side normalization on save (new, pure, in `SetBackupPaths`):** after host→container translation and dedupe, drop any path that has another listed path as ancestor. This is the safety net that keeps S canonical even if a client bug or an old client sends nested entries. It matters for two verified reasons:
- restic walks every positional independently — a parent + child pair reads the child twice per run (storage dedupes; read cost doesn't — the exact #189 lesson, documented at `service.go:3581-3596`).
- restic's `--exclude` does **not** apply to positional backup sources themselves (upstream docs, MEDIUM confidence), so nested entries also can't be "fixed" with an exclude after the fact.

**Fanout escape hatch:** if a parent has enormous fanout (the pathological rewrite: hundreds of children replacing one stored path), don't automate a bad answer — the tree should offer "exclude this subfolder instead," which routes to the **existing per-container ExcludesEditor** (stored in `targets.excludes`, translated at backup time by `resolveExcludePatterns`). This preserves the deliberate feature separation (PROJECT.md: selection answers "which folders", patterns answer "which files/globs") and keeps `snapshot.Excludes` user-owned. The 1 MiB `decodeBody` cap comfortably holds thousands of paths if a big rewrite does happen.

### Why not parent + `--exclude` (snapshot-structure consequences, compared)

| Aspect | Explicit maximal roots (recommended) | Parent + exclude for unchecked children |
|---|---|---|
| `snapshot.Paths` | One entry per maximal root; changes when selection changes (normal restic; each entry independently addressable) | Frozen at `[parent]` — stable, but opaque: what the snapshot *contains* is only knowable by also reading `snapshot.Excludes` |
| `snapshot.Excludes` | Stays user-owned (file/regex patterns only) | Churns with selection edits; mixes user patterns with selection artifacts; ExcludesEditor UX polluted |
| Restore granularity | Per-subtree restore for free — the existing restore model (`restore <id>:<path> --target <path>`, one per stored path) already treats each path as its own subtree | Only the parent is a subtree; "restore just the config folder" needs `--include` paths derived from excludes — new logic |
| Feature separation | Clean: two features, two stores, two argv channels | Conflated: selection writes flow into the excludes channel and its mount-translation heuristics (`resolveExcludeLine`) |
| Known hazard | Selection churn (see restore coupling below) | **Hard error class:** excludes don't apply to positional sources — any path that is both stored and excluded silently wins as "backed up" |
| #91 retention invariant | Safe: identity comes from `--tag container:<ref>`, ungrouped; path-list changes don't touch it. Never add `--group-by paths` to "stabilize" anything | Same |

**Cost consequences of selection churn (either model, but explicit roots make it visible):** restic dedupes at the content-addressed blob level, so storage doesn't grow when the path list changes — but every positional is walked/hashed per run. Growing a selection re-reads newly included roots; shrinking is cheap (nothing to read for dropped roots, old blobs age out via retention). This is acceptable and is the same trade #189 already accepted for per-stack backups.

## Where selection state lives

**Containers: no schema change, no settings-row involvement.** The state already has a home with an enforced write contract:

- `targets.selected_paths` (JSON string array, migration `target_selected_paths`), **container-visible paths**.
- Written only by `store.SetBackupPaths` (`internal/store/targets.go:293`) — an *owned setter*, the targets-table analogue of `MutateSettings`: the column is never reset by `UpsertTarget` (round-trip pinned by `targets_test.go`), empty list explicitly means "clear selection → automatic appdata detection".
- The question's premise ("SQLite settings via MutateSettings") is the wrong table for this data: `MutateSettings` guards the singleton settings row; per-container selection is per-target state. Route new writes through the existing setter; a source-scan guard test protects the settings contract, and the same discipline (one owner per column) is what to preserve here.
- Semantics to preserve exactly: **empty selected_paths = automatic detection** (#181 reasoning at `service.go:3842-3888`). The tree must render the no-explicit-selection state as "Automatic — all discovered folders selected", not as "nothing selected", and unchecking the last folder of every mount must not be representable as "everything off" (it would silently mean auto). The existing FoldersEditor already models this; the tree inherits it.

**File Sets: one decision point.** `file_sets.path` holds one browse-relative root; `BackupFileSetDir` backs up `[]string{d.SourceDir}` — but `FilesRestic.Backup` already accepts a `paths []string` slice, so multi-root sets need **no orchestrator interface change**, only:
- a new append-only migration (next unused number; the ≥100 regime — never edit or renumber, the `:latest` hazard) adding e.g. `selected_paths TEXT` (nullable; NULL/empty = "whole root", preserving every existing set's meaning), with all four `settings.go`-style touchpoints if it were a settings column — here it's a per-row column, so accessor + INSERT/UPDATE/SELECT lists + round-trip test;
- resolution of each stored relative path through `paths.Resolve(HostMountRoot, …)` at backup time (the established file-set convention);
- the same normalization rule as containers.

Recommendation: the tree UI and normalization land first with containers; decide single-root ("tree improves root picking") vs multi-root ("partial selection inside a set's root") for file sets during phase planning — the architecture supports both, multi-root is a strict superset and is the reading of the Active requirement ("same tree selection when choosing what a file set covers").

## Data Flow

### Flow 1 — expand (lazy listing)

```
user expands node (mount-relative path p)
    → GET /api/browse?path=<p>[&hidden=1]
    → csrfGate/authGate → handleBrowse → paths.Resolve(HostMountRoot, p)
    → os.ReadDir (dirs only, sorted) → {ok:true, dirs:[{name,path:rel}]}
    → tree caches children under p
```
Failure: `{ok:false, error:"could not read directory"}` → inline node error; panel stays alive.

### Flow 2 — load roots + current selection

```
open container panel
    → GET /api/containers/{name}/mounts
    → {mounts:[{source(host), dest, selected, isAppdata, reachable}],
       custom:[{path(host), exists}], hostMountRoot, hostSourceRoot}
    → tree roots = reachable mounts + custom paths (stale ones flagged, #115)
    → node state derived client-side: checked (self or ancestor ∈ S),
      mixed (some stored descendant strictly below), unchecked
```

### Flow 3 — edit + save (live-save preserved)

```
toggle/add/remove in tree (optimistic UI)
    → build host-path list S (maximal roots; uncheck-inside-checked rewrite)
    → PATCH /api/containers/{name} {backupPaths: [host paths]}
    → handlePatchContainer → svc.SetBackupPaths
    → toContainerPath each (reject outside host mount) → dedupe → NORMALIZE (new)
    → store.SetBackupPaths (owned setter; empty = auto)
    → {ok:true} → toast; failure → revert + glim-shake (existing pattern)
```

### Flow 4 — backup time (unchanged mechanics, derived paths)

```
svc.Backup → effectiveBackupPaths (explicit S else auto; existence-filtered)
    → BackupDeps.AppdataPaths + definition JSON (AppdataPaths mirrored into the
      recreate recipe) + UpsertTarget(AppdataPaths: effective)
    → BackupContainer → Restic.Backup(positionals = AppdataPaths,
      tags = ["container:<ref>", "p1"], excludes = resolveExcludePatterns(...))
    → snapshot.Paths = the maximal roots; snapshot.Excludes = user patterns only
```

### Flow 5 — restore time (with the recommended hardening)

```
prepareRestoreForTarget: snaps already listed for tag ownership check
    → HARDEN: appdataForRestore = stored tg.AppdataPaths ∩ chosen snapshot.Paths
      (exact-string match; both are container-visible absolute paths)
    → RestorePaths: restore <id>:<path> --target <path> per entry (unchanged)
```

**Why the hardening matters:** restore replays the *last* run's stored `AppdataPaths` against *any* chosen snapshot. `RestorePaths` fails fast on the first absent path (`internal/api/service.go:6752`), and `VerifySnapshot` only proves the snapshot exists — not that each path is in it. Today the path list rarely changes; tree selection makes changing it a first-class user activity, so "restore an older snapshot taken under a different selection" becomes a routine case that currently aborts *after* destructive teardown (stop/remove already happened). The snapshot list (`snaps`, each carrying `Paths`) is already in hand at exactly the right place.

## Suggested Build Order (dependency-driven)

Each step is independently shippable; later steps depend on earlier ones only where noted.

1. **Selection normalization + namespace plumbing (Go, pure).** Normalization helper (drop nested entries) wired into `SetBackupPaths`; unit tests for maximal-roots invariants, host↔container translation edge cases (identity root, split root, non-`, trailing-slash, nested). *No behavior change for existing clients.* Everything else builds on the canonical form of S.
2. **`/api/browse?hidden=1` extension + contract test.** Additive; the uncheck-rewrite (step 3) needs it. Pin exact behavior: hidden only when flagged; dirs only; sorted; traversal guard unchanged.
3. **Restore hardening: snapshot-Paths intersection** in `prepareRestoreForTarget` (list already fetched there). Independent of steps 4–6; do it *before* the tree ships users into selection churn. Tests: stored-path-in-older-snapshot miss → skipped with a warning rather than a mid-restore abort; exact-match on container-visible strings.
4. **`SelectionTree` SPA component (pure + lazy).** Presentational tree + tri-state derivation + uncheck-rewrite logic, in `web/src/components/` (feature-scoped subfolder if it grows); unit + jsdom tests modeled on SnapshotFileTree/FolderBrowser precedents, but lazy (fetch on expand, cache, per-node error state). Depends on step 2 for exact rewrite semantics; can be developed against mocked browse.
5. **Wire into FoldersEditor (containers).** Replace the flat mount-checkbox list with the tree: roots = reachable mounts + custom paths (stale flagged), auto-state rendering, live-save via the existing single `persistPaths` funnel (optimistic + revert/shake precedent). Fanout escape hatch ("exclude instead") as a follow-up affordance, not a blocker.
6. **File Sets.** Decision point first (single-root tree picker vs multi-root selection). Multi-root: migration + accessor + resolve-multiple-paths at backup + service-layer restore enumeration. `FilesRestic.Backup` needs no signature change either way.
7. **Process tail (every web-touching step):** `en` + `de` i18n keys, `bombvault/*` lint rules respected (declare exceptions, never disable), `tsc --noEmit && vite build`, commit `web/dist`; both release-notes copies when shipping.

## Scaling Considerations

| Concern | Typical (appdata trees: tens of nodes/level) | Large (media roots, thousands of siblings) | Pathological |
|---------|----------------------------------------------|---------------------------------------------|--------------|
| Expand latency | One ReadDir + small JSON, imperceptible | One ReadDir still; JSON in the low MBs worst case — acceptable on LAN | Consider paging the listing (server-side change; don't preemptively build) |
| Stored selection size | 1–20 paths | Rewrite of a 500-child parent is bounded by the 1 MiB body cap (thousands of paths) | Fanout escape hatch: route to ExcludesEditor instead of storing hundreds of paths |
| restic read cost | Unchanged from today | Grows with maximal-root count on selection growth (#189: reads don't dedupe) | Selection churn every backup → schedule impact; normalization keeps the list minimal |
| Restore coupling | Intersection hardening makes old snapshots restorable under any later selection | Same | Same — the hardening is the fix, not a heuristic |

## Anti-Patterns (specific to this integration)

### Compiling selection into `--exclude` patterns
**What people do:** keep the mount root as the positional, generate excludes for unchecked children.
**Why it's wrong:** verified hard error — restic excludes don't apply to positional sources; it also conflates two deliberately separate features and pollutes `snapshot.Excludes` and the ExcludesEditor UX.
**Do this instead:** explicit maximal-roots positionals; user-owned patterns stay user-owned.

### Writing selection through `MutateSettings`
**What people do:** treat "settings via MutateSettings" as the generic persistence story and add per-container selection to the settings row.
**Why it's wrong:** wrong table and wrong contract — selection is per-target state with its own owned setter (`store.SetBackupPaths`); the settings row's guard test (`settings_writers_test.go`) exists for the singleton row, and a settings-row write inside any mutation fn deadlocks on the single pooled connection.
**Do this instead:** extend nothing; route through the existing setter.

### Re-implementing path translation in the SPA or handlers
**What people do:** string-prefix logic ad hoc at each call site (`/mnt` assumptions, missing `path.Clean`).
**Why it's wrong:** three namespaces with a split-root default (`/mnt` → `/host/user`) plus an identity-root mode; every ad hoc copy is a wrong-prefix bug waiting for a non-Unraid host.
**Do this instead:** service side uses `toContainerPath`/`toHostPath`; SPA uses the `hostMountRoot`/`hostSourceRoot` pair the mounts endpoint already returns; browse speaks mount-relative only.

### Sending nested paths to restic ("parent and its checked children")
**What people do:** accumulate checked paths without collapsing implied descendants.
**Why it's wrong:** restic walks every positional — nested entries re-read data (#189) and make `snapshot.Paths` non-canonical, which complicates restore enumeration.
**Do this instead:** normalize to maximal roots on save (server-side, pure, tested).

### Replacing the eager restore tree with the lazy one (or vice versa)
**What people do:** unify `SnapshotFileTree` (eager, over `restic ls` of a snapshot, fed by `LsStream`) with the new lazy browse tree.
**Why it's wrong:** different data sources (snapshot contents vs live host filesystem), different lifetimes, and the repo's documented rule that snapshot listings must use `LsStream` (memory).
**Do this instead:** share visual styling and tri-state logic if convenient; keep the two components and their data paths separate.

### Any `--group-by` "fix"
**What people do:** seeing `snapshot.Paths` change across runs and reaching for `--group-by paths` (or `tags`) to make listings tidy.
**Why it's wrong:** #91 (identity instability) and live-VM `live` tag pollution — both documented never-again events.
**Do this instead:** leave retention identity-stable (per-item tags, ungrouped); path-list variance is expected restic behavior.

## Integration Points

| Boundary | Communication | Notes |
|----------|---------------|-------|
| SPA ↔ `/api/browse` | same-origin JSON, csrfGate/authGate, `{ok}` envelope | Only change: `hidden` flag. Envelope convention: domain failures are HTTP 200 `{ok:false}` — the tree treats these as node states, not transport errors |
| SPA ↔ `/api/containers/{name}` PATCH | unchanged wire format (`backupPaths` = host paths) | 1 MiB body cap; `DisallowUnknownFields` — don't add fields casually |
| Service ↔ store | `SetBackupPaths` owned setter only | Single pooled SQLite connection; the setter's UPDATE-then-Upsert is existing behavior |
| Service ↔ orchestrator | `BackupDeps.AppdataPaths` (container-visible) | `internal/backup` never imports engines; ports stay as-is |
| Orchestrator ↔ restic | `BackupArgs` positionals after `--`; user-influenced paths stay argv-injection-safe | Arg pins live in `restic_args_test.go` — path-count changes are data, not argv-shape changes |
| Selection ↔ ExcludesEditor | None (deliberate) | They coexist on one backup: paths positionals + user `--exclude` patterns; suggestion flows read `configuredBackupPaths` and follow the tree automatically |

## Sources

**Local code (HIGH — primary, read directly):**
- `internal/api/handlers.go:4143-4205` — `handleBrowse` (traversal guard, dirs-only, hidden-skip, envelope); `:4207+` — `handleMkdir`; `:1090-1128` — `handlePatchContainer`; `:1280-1307` — `handleContainerMounts` (ships `hostMountRoot`/`hostSourceRoot`)
- `internal/api/service.go:1132-1148` — `toContainerPath`; `:3680-3695` — `toHostPath`; `:3511-3620` — `resolveAppdataPaths` (container-visible derivation, #189 read-cost note); `:3766-3792` — `SetBackupPaths` (host in → container stored); `:3816-3888` — configured/effective paths + #175/#181 empty-selection semantics; `:3957-4137` — `Backup` (deps assembly, excludes resolution); `:5242-5462` — restore plan (stored `AppdataPaths` replayed against chosen snapshot); `:6752-6759` — `RestorePaths` fail-fast
- `internal/store/targets.go:25-28, 293-317` — `selected_paths` + owned `SetBackupPaths`; `internal/store/filesets.go` — `file_sets.path` (browse-relative, resolved at backup time)
- `internal/backup/files_orchestrator.go` — `FilesRestic.Backup(paths []string…)`; `internal/backup/orchestrator.go:690-794` — per-subtree restore sequence
- `internal/restic/restic.go:357-384` — `BackupArgs` (positionals after `--`); `:478-511` — restore-subtree selectors from snapshot Paths
- `internal/paths/paths.go` — containment (`Resolve`, `Within`), POSIX-only note; `internal/api/api.go:164-186, 257-258, 300-317` — route table and literal-vs-param ordering
- `web/src/lib/api.ts:468-484, 1977-2005` — browse types + client; `web/src/pages/Containers.tsx:608-816` — FoldersEditor live-save, host-path translation, custom paths; `web/src/components/SnapshotFileTree.tsx`, `FolderBrowser.tsx` — UI precedents

**Upstream restic documentation (MEDIUM — via Context7, digests cached in the research store):**
- Snapshot JSON: `paths` = included backup paths, `excludes` = exclusion list; `snapshot:subfolder` restore selectors
- Exclude semantics: `filepath.Match` + `**`; **excludes do not apply to positional backup sources**
- Symlinks archived as links, never followed (matches dirs-only browse)
- Blob-level dedupe with per-path scan/read cost per run
