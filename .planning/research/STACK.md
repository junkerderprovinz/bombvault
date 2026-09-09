# Stack Research

**Domain:** Tree-based sub-folder backup selection (lazy directory tree UI + restic path translation) for BombVault
**Researched:** 2026-09-09
**Confidence:** HIGH overall (core restic/Go facts cross-checked against official docs; UI patterns from official React/W3C sources)

## Verdict First

**Zero new dependencies.** Everything this feature needs already exists in the stack: extend the existing `GET /api/browse` endpoint for lazy child listings, hand-roll the collapsible tree with native checkboxes in the existing SPA idiom, and translate selections into **explicit restic target paths** through the unchanged `BackupArgs` builder. The tree is a view over the flat `backupPaths` set, exactly as PROJECT.md decided. The only new API surface is one optional field on the browse response and one save-time normalization rule.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `GET /api/browse` (extended) | existing (api.go:257, handlers.go:4143) | Lazy child listing for tree expansion | Already exists with traversal guards, relative-to-`HostMountRoot` contract, dirs-only filtering, hidden-entry skip, sorted output, `ok:false` error shape. The tree needs **one endpoint, called N times** — one listing per expansion. Do not create a second browse route. Confidence: HIGH (read the code). |
| Explicit restic target paths (unchanged `BackupArgs`) | restic 0.17 floor (restic.go:361) | Selection → backup content | restic preserves the **full absolute path** of each explicitly passed target inside the snapshot (verified: `restic backup /home/user/work.txt` → snapshot contains `/home`, `/home/user`, `/home/user/work.txt`). So passing `/mnt/user/appdata/plex/config` and `/mnt/user/appdata/plex/metadata` as targets yields a snapshot browsable at the real paths, merged at shared prefixes — no basename flattening, no restore-layout surprise. `backupPaths` is *already* this list; `effectiveBackupPaths` already feeds it to restic. The translation layer is near-zero. Confidence: HIGH (restic stable docs + Context7 corpus). |
| Go `os.ReadDir` + `DirEntry` | Go stdlib (1.25 floor) | Directory listing without per-entry stat | `os.ReadDir` returns sorted `DirEntry` values; `IsDir()`/`Type()` come from the dirent (getdents) data — no `stat` syscall per entry. The existing handler already uses it; keep it. For a bounded emptiness probe (if ever needed): `f.ReadDir(1)` reads at most one entry and returns `io.EOF` at directory end — but see the `hasChildren` decision below; we recommend **not** probing. Confidence: HIGH. |
| Go `os.Root` (`os.OpenRoot`) | Go 1.24 core API, Go 1.25 expanded methods | Symlink-safe listing inside `HostMountRoot` | `os.Root` methods resolve names only within the opened root and **refuse symlinks that resolve outside it**; it is concurrency-safe and holds a directory fd. This closes a real gap: the current handler's `paths.Resolve` guard is purely *lexical* — a symlink inside appdata pointing to `/etc` would be followed by `os.ReadDir` and its contents listed. Fallback that works on any Go ≥ 1.24: `root.Open(rel).ReadDir(-1)`. Confidence: HIGH on API behavior (Go source + API diffs); MEDIUM that the current lexical guard is exploitable in practice (reasoned from code + stdlib semantics, not exploited). |
| Native `<input type="checkbox">` + `indeterminate` DOM property | HTML / React 19 | Per-node tri-state checkbox | The mixed (partial) state is the DOM `indeterminate` *property* — it is not an HTML attribute and has no JSX prop; set it with a callback ref (`(el) => { if (el) el.indeterminate = isMixed; }`). Semantics match the W3C APG tri-state checkbox: all children checked → checked, none → unchecked, some → mixed. Announced correctly by screen readers on native checkboxes with zero extra ARIA work. Confidence: HIGH. |
| Fetch-on-expand with a module-level promise cache | React 19 pattern | Lazy children without a data library | React's own guidance for on-demand data is: fetch in the event handler, cache the promise outside React state (`Map` keyed by path), render from local state; `useSyncExternalStore` is only needed if the cache must be shared across components reactively. For one panel, a small `treeCache.ts` module (`Map<string, Promise<Entry[]>>` + a loaded-children `Map`) with `useState` is the whole data layer. Confidence: HIGH (react.dev patterns). |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| None | — | — | Genuinely nothing. Every candidate (TanStack Query, react-arborist, react-window) is either banned by the zero-dep constraint or solves a problem this feature does not have. See "What NOT to Use". |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| restic CLI spot-check (already on dev PATH) | Verify two documented behaviors against the pinned 0.17 floor | Two commands, 30 seconds, run once during the phase: (1) `restic backup <abs path>` + `restic ls <snap>` confirms full absolute path preservation; (2) an `--exclude` pattern matching an explicit target confirms "excludes never drop explicitly passed targets". Docs verified against restic **0.19.1 stable**; both behaviors are long-standing but the project floor is 0.17 — check, don't assume. Confidence: MEDIUM until spot-checked. |
| `go test` + `restic_args_test.go` idiom | Argv-level tests for the normalization + any new browse handler logic | The repo already unit-tests argv construction; add the same for: ancestry pruning of the target list, and a handler test for symlink-outside-root rejection once `os.Root` lands. |

## The Two Design Cores (prescriptive)

### 1. API pattern for the lazy tree

Keep `/api/browse` as a **shallow, per-directory listing** and build the tree entirely client-side:

- **Request:** `GET /api/browse?path=<relative>` — unchanged contract (relative to `HostMountRoot`, `paths.Resolve` containment, generic error message on traversal attempts).
- **Response:** current `{ok, root, path, dirs: [{name, path}]}` — keep dirs-only (backup selection is folder granularity; files are the ExcludesEditor's job per the Key Decision in PROJECT.md).
- **`hasChildren` field: do not add a server-side probe.** Decide that **every listed dir is expandable**. On expand, an empty dir just renders nothing under it (the VS Code remote-explorer / Ant Design `loadData` behavior). Rationale: on Unraid, appdata lives on SHFS (FUSE) — a `Readdirnames(1)` emptiness probe per listed subdir turns one listing round-trip into N+1 FUSE ops, doubling the latency of every expansion for a cosmetic expander glyph. The dominant real subtrees (Plex-style appdata) are never empty at the levels users expand.
- **Caching:** client-side only, keyed by relative path, lifetime = panel mount. No server cache, no ETags. Listings are cheap; stale-tree confusion costs more than re-fetching.
- **Hidden entries:** keep skipping dot-prefixed dirs (existing behavior). Document in the UI hint that hidden dirs are not shown/selectable even though restic would back them up if inside a selected folder — consistent with the current picker.

### 2. Selection model → restic translation

UI state = `Set<string>` of explicitly checked paths (a mirror of `backupPaths`). Everything else is **derived**, never stored:

- **Ancestor-dominance display rule:** a node renders *checked* if its path or any ancestor is in the set; *mixed* if no ancestor is in the set and at least one loaded descendant is in the set; *unchecked* otherwise. No expansion is ever triggered just to compute display state.
- **The one hard rule — materialize on uncheck:** unchecking a descendant of a checked parent cannot be expressed in a flat path set, so replace the parent with its **immediate children minus the unchecked one** (one server listing, usually already loaded because the node was visible/expanded). Never recursively materialize deeper levels; the flat set stays minimal and the restic target list stays small. This is the standard resolution for flat-persistence + lazy-tree (Vorta's include list and Kopia's policy `include` lists resolve identically). Confidence: HIGH on the mechanics; MEDIUM on "everyone does it this way" (synthesized from the backup-UI landscape, not a single citable doc).
- **Check a mixed/checked parent = re-collapse:** add the parent path and prune stored descendants (ancestry pruning, below).
- **Save-time ancestry pruning (make it a server invariant):** in/near `SetBackupPaths`, drop any stored path that has an ancestor also stored. Three reasons: (a) it is the normalization restic wants — never pass `/mnt/user/appdata/plex` and `/mnt/user/appdata/plex/config` as targets together (overlapping-target behavior is not crisply documented; pruning makes the question moot); (b) it keeps the snapshot `paths` metadata canonical, and restic picks its incremental **parent by `host,paths`** — a canonical list keeps parent selection stable between runs; (c) it also fixes legacy duplicates from custom-path entry, protecting old deployments. Confidence: MEDIUM on restic overlap behavior; HIGH that pruning is correct regardless.
- **Restic invocation:** unchanged `BackupArgs(repo, prunedPaths, tags, mode, excludes...)`. Do not add `--parent` management; accept the one parentless full-rescan after a selection edit (restic groups parents by `host,paths`, so a changed list = no parent for exactly one run; dedup means repo growth stays ~zero — only scan IO is paid once).
- **Division of labor with excludes is real and verified:** restic docs state excludes **do not apply to targets explicitly passed** (a user's `*.log`-style pattern cannot silently kill a checked folder), but content *inside* a selected dir is still filtered by excludes. That is precisely the tree=folders / ExcludesEditor=files-and-globs split PROJECT.md decided on. Generating `--exclude` patterns from unchecked folders would invert the persistence model, and restic's negation rules make exclude-based "select" one-way: *once a directory is excluded, descendants cannot be re-included*. Confidence: HIGH (verbatim from stable docs).

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| Extend `/api/browse` | New `/api/tree` endpoint returning nested nodes | Never — two endpoints for one listing, plus a nested-response shape invites eager recursion bugs. |
| Explicit target paths | Mount root as single target + generated `--exclude` per unchecked folder | Only if snapshot layout ever had to stay byte-identical across selection changes; it doesn't (paths are absolute → layout is stable per selection), and excludes lose the ability to re-include descendants. |
| Always-expandable dirs (no `hasChildren`) | Server probes emptiness per listed dir (`Readdirnames(1)`) | Only if the browse endpoint moves off FUSE-hosted paths (e.g. local-disk file sets on generic hosts) AND users complain about empty-dir spinner flicker. Cheap to add later as `hasChildren?: boolean` — absent = expandable, so it's non-breaking. |
| Client-side promise cache | TanStack Query / SWR | Never here — one endpoint, panel-lifetime cache, manual invalidation; a library is 40 kB of dependency for a `Map`. |
| Recursive React component rows | Full APG `role="tree"` with roving tabindex | Phase 2 if keyboard-tree navigation is requested. V1: plain collapsible rows with natively focusable buttons + checkboxes (matches the existing folder-picker idiom and i18n/dark-mode plumbing); add `role="tree"/treeitem/group`, `aria-expanded`, `aria-checked="mixed"` if audit demands. |
| `os.Root` for listing | Keep lexical `paths.Resolve` only | If Go floor ever drops below 1.24. Keep `paths.Resolve` anyway as defense-in-depth — they compose. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| TanStack Query, SWR, RTK Query | New dependency; violates the explicit no-state-library constraint; cache/invalidation needs here are one `Map` and a panel lifetime | `treeCache.ts` promise cache + `useState` (react.dev's documented fetch-in-event-handler pattern) |
| react-arborist, react-complex-tree, Ant Design Tree, MUI X TreeView | UI-kit territory (banned); none natively models *lazy load + tri-state checkboxes + flat-path selection with ancestor dominance* — all three would still be hand-written; styling fights Tailwind-4 dark mode; react-arborist's maintenance cadence is a risk | ~200–300 lines of recursive rows; the hard part (selection semantics) is independent of rendering anyway |
| react-window / react-virtualized | Premature: only expanded paths render; dirs-only listings keep sibling counts modest. Plex cache dirs can have many *entries* but most are files, which the tree doesn't list | If a pathological dir ever hurts, hand-roll windowing in that one component — do not take a dependency |
| Translating unchecked folders into `--exclude`/`--iexclude` patterns | Inverts the persistence model (persistence is include-paths); restic negation is one-way (excluded dir = descendants permanently un-includable); would poison the ExcludesEditor live preview with machine-generated patterns users didn't write; excludes don't stop restic from walking *to* the exclusion points when the parent is a target, so IO savings are partial anyway | Explicit target paths + save-time ancestry pruning |
| `--files-from` / `--files-from-verbatim` for the target list | `--files-from` **expands glob patterns** in the file — literal paths containing `*?[` would silently reinterpret; adds a temp-file round-trip for no gain | Positional targets after `--` in `BackupArgs` — already injection-safe and already unit-tested |
| Negative entries ("all except X") in `backupPaths` | Changes the persistence format — explicitly out of scope (PROJECT.md); old deployments must keep working | Materialize-on-uncheck (parent → immediate children minus unchecked) |
| Eager full-tree endpoint (one `GET /api/tree?root=appdata/plex`) | Appdata trees are exactly the huge, slow case (Plex `transcoding` subtrees); eager recursion stalls both the panel and the SHFS backend — this was the pre-existing decision in PROJECT.md | Shallow per-directory listing, fetch on expand |
| Storing the *expanded* set server-side | Expansion is presentation, not selection; server-side expansion state would leak UI concerns into `backupPaths` | Keep `expanded` + `loaded` sets in component state |

## Stack Patterns by Variant

**If the target is a container mount (bind or named volume) under appdata:**
- Tree roots at the mount's host path from existing discovery (`resolveAppdataPaths`); browse relative to `HostMountRoot` as today.
- Expect FUSE latency — show per-node spinners on expand; do not prefetch children on hover (SHFS makes hover-prefetch a cost amplifier, unlike the react.dev hover-preload example, which assumes cheap fetches).

**If the target is a File Set root (arbitrary host dir, may be local disk):**
- Same component, same endpoint, same semantics — only the initial path differs. No special-casing; this is the payoff of reusing `/api/browse`.

**If a listed dir disappears mid-edit (container stopped, mount gone):**
- `ok:false` listing → render the node with a retry affordance, leave selection untouched. `SetBackupPaths` already tolerates non-existent paths (existence is not required — verified in service tests), so a vanished folder never blocks saving.

**If the user checks a parent and some children were already individually stored:**
- Ancestry pruning at save handles it (descendants dropped when ancestor present). Do this server-side so it is an invariant, not a UI promise.

## Version Compatibility

| Component | Compatible With | Notes |
|-----------|-----------------|-------|
| restic behaviors relied on | 0.17 floor (docs verified at 0.19.1 stable) | Spot-check two behaviors against 0.17 CLI during the phase: absolute-path preservation in snapshots; excludes not dropping explicitly passed targets. Both are long-standing; the check is two commands. |
| `os.Root` | Go ≥ 1.24 (project floor 1.25 — fine) | Go 1.24: core methods; Go 1.25: expanded (Chmod/Chown/MkdirAll/ReadFile/Rename, per `api/go1.25.txt`). Use `root.Open(rel).ReadDir(-1)` for listing — works across both. Keep `paths.Resolve` for the pre-existing error messages/tests. |
| React patterns | React 19.x (19.2 current) | Event-handler fetch + external promise cache; `indeterminate` via callback ref. No Suspense/`use()` needed for a settings panel — do not introduce Suspense boundaries around form rows. |
| Existing SPA plumbing | React 19 + TS + Vite + Tailwind 4, embedded `web/dist` | Any `web/` change requires the frontend build committed per repo convention. |

## Sources

- restic official docs, "Backing up" (readthedocs stable 0.19.1) — absolute/relative path snapshot layout, parent selection groups by `host,paths`, exclude pattern grammar (`filepath.Match` + `**`, leading-`/` anchor, `!` negation limits), "excludes do not apply to explicitly passed backup sources", symlinks not followed. Fetched verbatim this session. **HIGH** (cross-checked with Context7 `/restic/restic` corpus of the same docs).
- Context7 `/restic/restic` — snapshot metadata (`paths`, `excludes` fields), symlink node structure, snapshot filter semantics. **MEDIUM** (seam tier; corroborates the readthedocs text).
- Context7 `/golang/go` — `os/root.go` doc comment (symlinks may not resolve outside root; concurrency-safe; fd-backed), `api/go1.24.txt` (`os.OpenRoot`), `api/go1.25.txt` (expanded method set), `os/dir.go` (`File.ReadDir(n)` bounded batches). **HIGH** for behavior, from Go source; **MEDIUM** for the exact 1.25 method list (corpus truncation — hence the `Root.Open().ReadDir` fallback).
- Context7 `/reactjs/react.dev` — "You Might Not Need an Effect" (external store subscription), `useSyncExternalStore`, `use()`/Suspense cached-promise pattern, "Choosing the State Structure" (Set-based selection). **MEDIUM** (seam tier) but these are official React docs.
- Context7 `/w3c/wai-aria-practices` — APG Checkbox pattern (tri-state/mixed semantics) and Tree View pattern (`role=tree/treeitem/group`, `aria-expanded`). **HIGH** (W3C examples, fetched verbatim).
- Codebase seams (read this session): `internal/api/api.go:257`, `internal/api/handlers.go:4143` (`handleBrowse`), `internal/restic/restic.go:361` (`BackupArgs`), `internal/api/service.go:3771` (`SetBackupPaths`), `web/src/lib/api.ts:468` (browse client types). **HIGH** (primary source).
- Overlapping-restic-target behavior and backup-UI landscape (Vorta/Kopia materialization analogy): **LOW–MEDIUM** — synthesized; no single authoritative source. This is why save-time ancestry pruning is recommended unconditionally rather than relying on restic deduplicating overlapping targets.

---
*Stack research for: BombVault tree-based sub-folder backup selection*
*Researched: 2026-09-09*
