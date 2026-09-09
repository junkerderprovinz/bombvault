# Research Summary — Tree-Based Sub-Folder Backup Selection

**Project:** BombVault tree-based sub-folder backup selection (lazy directory tree UI + restic path translation)
**Synthesized:** 2026-09-09
**Research files:** STACK.md, FEATURES.md, ARCHITECTURE.md, PITFALLS.md

---

## Executive Summary

Tree-based sub-folder selection for BombVault is a brownfield feature that, per all four research files, is a **pure new view over existing data**: extend the existing `GET /api/browse` endpoint (one small opt-in addition), hand-roll a lazy tri-state checkbox tree in the existing SPA idiom, and compile selections into **explicit restic positional target paths** through the unchanged `BackupArgs` builder. The stack needs **zero new dependencies** — TanStack Query, react-arborist, and friends are all either banned by constraint or solve problems this feature does not have. The tree's browsable universe (paths under `HostMountRoot`) is by construction identical to the backable universe, so one generic endpoint serves every consumer and the tree can never offer a path the engine cannot reach.

The central technical decision — **resolved here, because STACK and ARCHITECTURE align and PITFALLS' alternative is technically broken** — is: selections compile to *maximal checked roots* passed as restic positional paths, never to parent-plus-derived-`--exclude`. This is not a style preference: restic's excludes **do not apply to positional backup sources** (verified against official docs), so the parent+exclude hybrid is an error class, not a design option; restic's `!` negation also cannot re-include inside an excluded directory. PITFALLS raises one legitimate concern against pure decomposition (Pitfall 2): replacing a stored parent with its children is an allowlist — folders created *after* the save under that parent are silently not backed up, and the flat set cannot express "all except X". The synthesis: **accept explicit-positional-targets as the engine model; surface the future-children semantic as a deliberate, communicated consequence** — a narrowing-selection note in the UI ("new snapshots will contain only the selected folders") plus a backup-time coverage warning — rather than solving it with exclude semantics that restic does not support. The fanout escape hatch ("exclude this subfolder instead" routing to the existing ExcludesEditor) covers the "mount minus junk" case where the denylist instinct is strongest.

The key risks are UI-semantics traps, not engine work: unchecking everything silently resurrects auto-detection (empty-selection inversion); mixed-state derivation must come from the flat stored set, never from lazily-loaded children; the listing endpoint's lexical containment guard is not symlink containment (`os.Root` hardening); and selection churn makes restore coupling first-class — restore replays the last run's stored path list against an older snapshot that may not contain those paths, which aborts mid-restore *after* destructive teardown. That restore hardening (intersect stored paths with the chosen snapshot's recorded `Paths`) must ship **before** the tree does.

---

## Key Findings

### From STACK.md (confidence: HIGH)

- **Zero new dependencies.** Extend `GET /api/browse` (api.go:257, handlers.go:4143) for lazy child listings; hand-roll the tree (~200-300 lines of recursive rows); translation layer is near-zero since `backupPaths` already feeds `BackupArgs` positionals.
- **restic preserves full absolute paths** of explicitly passed targets in snapshots — merged at shared prefixes, no basename flattening. Snapshot layout is stable per selection.
- **Excludes do not apply to explicitly passed targets** (HIGH, verbatim docs) — which is exactly the desired tree=folders / ExcludesEditor=files-and-globs split, and why exclude-based "select" is disqualified.
- **Save-time ancestry pruning as a server invariant** in/near `SetBackupPaths`: drop any stored path with a stored ancestor. Canonical lists keep restic's parent-snapshot selection (by `host,paths`) stable; also fixes legacy duplicates.
- **No `hasChildren` server probe** — SHFS/FUSE makes emptiness probes an N+1 latency multiplier; every dir is expandable, empty renders nothing.
- **`os.Root` (Go >=1.24, repo floor 1.25)** for symlink-safe listing; keep `paths.Resolve` as defense-in-depth.
- Native checkboxes with the `indeterminate` DOM property set via callback ref; module-level promise cache instead of a data library.

### From FEATURES.md (confidence: HIGH core UX; LOW vendor specifics)

- **Demand proof:** Unraid's "Plex Backup Fine Tuning" guide exists *because* subfolder selection is missing — users physically move folders and hand-write cron scripts to work around all-or-nothing appdata backups. **No restic frontend offers tree selection** (Backrest/Vorta stop at text paths + globs); BombVault would be first.
- **Table stakes (P1):** lazy collapsible tree, cascade semantics (tick includes subtree), mixed-state parents, normalization to/from flat `backupPaths`, state reconstruction on reopen + reviewable exclusions, both Container panel and File Sets, keyboard/a11y, empty-vs-unreadable handling.
- **Differentiators (v1.x):** confirm-first junk-folder suggestions (Plex `transcoding`, `node_modules`, `@eaDir`), effective-selection preview, tree search/filter, `CACHEDIR.TAG` toggle (argv-only, ships anytime).
- **Anti-features (do not build):** silent auto-exclusion, file-type masks inside the tree, eager size computation, nested-tree JSON persistence, per-subfolder retention/schedules, arbitrary-path browsing, per-subfolder recency, virtualized mega-tree.
- **Normalization is the keystone** — cascade + mixed state + flat persistence meet in one function; get it first.

### From ARCHITECTURE.md (confidence: HIGH, all seams read from source)

- **Three path namespaces, one relative suffix:** browse-relative / host (PATCH wire format, UI display) / container-visible (`targets.selected_paths`, restic positionals, `snapshot.Paths`). Translation is pure prefix arithmetic with `HostSourceRoot`/`HostMountRoot`, both already shipped by `GET /api/containers/{name}/mounts`.
- **Persistence: nothing moves.** Selection lives in `targets.selected_paths` via the owned setter `store.SetBackupPaths` — not the settings row (a settings-row write inside a mutation fn deadlocks on the single pooled connection). Empty list = auto-detection; preserve exactly.
- **One listing extension needed: `?hidden=1`.** The uncheck-inside-checked-parent rewrite replaces a parent with its complete child list; hidden dot-dirs must be included or they get silently deselected (restic does back up dot-directories).
- **Restore hardening (recommended milestone work):** in `prepareRestoreForTarget`, intersect stored `AppdataPaths` with the chosen snapshot's `Paths`; today a changed path list aborts mid-restore after stop/remove.
- **Build order:** (1) normalization + namespace plumbing (pure Go), (2) `?hidden=1` + contract test, (3) restore hardening, (4) `SelectionTree` component, (5) wire into FoldersEditor, (6) File Sets (decision point: single vs multi-root), (7) i18n/web-dist tail.
- **File Sets:** multi-root needs only an append-only migration (`selected_paths` nullable) — `FilesRestic.Backup` already takes a `[]string`.

### From PITFALLS.md (confidence: HIGH code-grounded)

Top pitfalls, mapped to phases:

1. **Empty-selection -> auto-detect inversion (CRITICAL, P2/P3):** "uncheck everything" collides with empty = auto. Guard at the PATCH boundary; disable/redirect the last uncheck with an explanation.
2. **Decomposition drops future children (CRITICAL, P2 decision):** the allowlist/denylist gap. **Resolution in this synthesis:** the denylist encoding (parent + generated excludes) is technically broken (see tension resolution below), so this becomes a *communicated semantic* — narrowing note in UI + backup-time coverage diff — not an engine workaround.
3. **restic no-reinclude + snapshot-layout mixing (CRITICAL, P2/P4/P5):** excluded dir = descendants permanently un-includable; passing leaf paths vs parent changes snapshot shape and resets restic change detection (one full re-walk, #189 class). Restore must map by `snapshot.paths` longest-prefix, never first path component.
4. **Symlink escape in listing (P1):** `paths.Resolve` is lexical only; direct-address `GET /api/browse?path=appdata/link-to-etc` lists outside the mount. Harden with `os.Root` *before* the tree leans on the endpoint.
5. **False-empty trap (P1/P3):** permission-denied must render `restricted`, never empty — first symptom is otherwise a restore missing a whole branch (PUID/PGID reality).
6. **Save races + `{ok:false}` envelope (P3):** HTTP is always 200; check the envelope, single-flight the PATCH queue, abort superseded requests.
7. **Mixed-state under laziness (P2/P3):** node state derives *only* from the flat stored set (prefix checks); never from loaded siblings; the uncheck-rewrite is the one operation that needs the child list.
8. **Snapshot-structure mixing erodes full-coverage history (P5):** retention identity is safe (never `--group-by paths`, #91); the fix is restore robustness + one communicated narrowing note.

---

## Resolved Cross-Dimension Tension: selection -> restic encoding

**STACK and ARCHITECTURE agree; PITFALLS' floated alternative (parent + tree-derived `--exclude`) is overruled on technical grounds, not averaged away.**

- **Positional maximal-roots wins** because restic's excludes **do not apply to positional backup sources** — a path that is both a stored target and "excluded" silently gets backed up anyway. The hybrid is not less elegant; it is a hard error class, verified against official restic docs (HIGH confidence). It also pollutes `snapshot.Excludes` and the ExcludesEditor UX with machine-generated patterns, conflates two deliberately separate features, and breaks restore granularity (only the parent is a subtree).
- **Pitfalls' legitimate concern is preserved as a product requirement, not an engine change:** decomposing a partially-unchecked parent into child paths means future folders created under that parent are silently not backed up (the flat set is an allowlist and cannot say "all except X"). Since the broken encoding is off the table, this becomes a **deliberate, documented semantic of partial selection**, addressed in three places:
  1. **UI (P3):** when a selection narrows an item with existing snapshots, show a one-time note ("new snapshots will contain only the selected folders; older snapshots still hold everything").
  2. **Backup time (P4):** coverage diff — list the stored paths' parents and warn when a sibling exists that is neither selected nor excluded; make absence visible once per run, not per restore.
  3. **Escape hatch (P3, follow-up):** the fanout affordance "exclude this subfolder instead" routes to the existing per-container ExcludesEditor — the *user-owned* exclude channel is the correct denylist mechanism when the user consciously wants "mount minus junk" with future-children coverage.
- **Log this in PROJECT.md Key Decisions during the semantics phase**, as PITFALLS prescribes.

---

## Implications for Roadmap

PITFALLS' phase vocabulary (P1-P5) is adopted as the roadmap skeleton, merged with ARCHITECTURE's build order. Suggested 6 phases:

### Phase 1 — Browse backend hardening & contract (Go)
- `os.Root` symlink-safe listing (Pitfall 4 — foundational), `?hidden=1` opt-in flag, entry cap + `truncated` flag, `r.Context()` cancellation, per-node state classifier (`ok`/`restricted`/`missing`, Pitfall 7). Contract tests: traversal guard, escaping-symlink fixture, split-root + identity-root tables.
- **Rationale:** everything else depends on the listing contract; harden before the tree leans on it. No behavior change for existing clients.
- Research flag: **none needed** — fully specified by this research (skip `--research-phase`).

### Phase 2 — Selection semantics & normalization (pure Go, decisions)
- Server-side ancestry pruning in `SetBackupPaths` (maximal-roots invariant, pure, tested); uncheck-rewrite rules; empty-selection guard at PATCH boundary (Pitfall 1); host<->container translation discipline; **log the positional-targets Key Decision + future-children semantic in PROJECT.md**; pin the save contract (envelope, single-flight) and the flat-set node-state model.
- **Rationale:** the keystone — every later phase consumes these semantics.
- Research flag: **none needed** — decisions are made; this is specification + pure-function implementation.

### Phase 3 — Restore hardening (Go)
- Intersect stored `AppdataPaths` with chosen snapshot's `Paths` in `prepareRestoreForTarget`; restore mapping by `snapshot.paths` longest-prefix (structure-agnostic); tests for restore-oldest/newest across a selection change.
- **Rationale:** must precede tree-wide availability — the tree makes selection churn first-class, and today's restore aborts mid-restore *after* destructive teardown. Independent of Phases 4-5; can be built in parallel.
- Research flag: **none needed.**

### Phase 4 — Tree UI (SPA, containers)
- `SelectionTree` component (lazy fetch-on-expand, promise cache, tri-state from the flat set, ARIA mixed-state, dom tests); wire into FoldersEditor with live-save via single-flight envelope-checked helper; locked/restricted nodes; narrowing note; fanout escape hatch as follow-up affordance. Both i18n locales; commit `web/dist`.
- **Rationale:** depends on Phases 1-2; the component is the feature.
- Research flag: **consider `--research-phase` only if the implementer wants deeper a11y/ARIA-tree detail** — otherwise STACK.md's APG guidance suffices.

### Phase 5 — Backup-time visibility & coverage (Go + light UI)
- Coverage diff (warn when a sibling of a stored path is neither selected nor excluded — the future-children mitigation); argv tests in `restic_args_test.go` (multi-path, same-basename leaves, nested-under-checked); `CACHEDIR.TAG` toggle if desired (argv-only).
- **Rationale:** converts the accepted semantic gap into a visible, per-run warning.
- Research flag: **none.**

### Phase 6 — File Sets parity
- Decide single-root tree picker vs multi-root selection (recommend multi-root — strict superset, matches the Active requirement); append-only migration + accessor + resolve-at-backup + restore enumeration; reuse the tree component.
- **Rationale:** architecture supports both; the component lands first with containers, file sets follow cheaply.
- Research flag: **decide the single/multi-root question during planning**, not research.

**Later milestones (from FEATURES.md, out of this roadmap):** junk-folder suggestion chips (confirm-first), effective-selection preview, tree search/filter, whitelist mode, per-folder size hints (needs background-job machinery), restore-side tree over snapshot listings.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | restic/Go facts cross-checked against official docs and source; only NEW gap is restic overlapping-target behavior (LOW-MEDIUM) — moot because ancestry pruning is recommended unconditionally |
| Features | HIGH | Core UX from canonical vendor docs (Duplicati, Veeam, W3C APG); Synology/Backblaze specifics LOW (bot-blocked) and not load-bearing |
| Architecture | HIGH | Every integration seam read from source with file:line; restic doc claims at MEDIUM (spot-check two behaviors against the 0.17 floor during Phase 5) |
| Pitfalls | HIGH for code-grounded items | MEDIUM for restic/Go semantics; the empty-selection collision and future-children gap are reasoned analysis, not post-mortems — verify with the prescribed boundary tests |

### Gaps to Address During Planning

1. **restic 0.17 spot-check** (two commands): absolute-path preservation; excludes-not-dropping-explicit-targets. Docs verified at 0.19.1; floor is 0.17.
2. **File Sets: single-root vs multi-root** — decide during Phase 6 planning (recommend multi-root).
3. **Click-on-mixed behavior** (fill vs cycle) — spec it in Phase 4; APG allows either, fill is the common expectation.
4. **Coverage-diff UX placement** (run report vs item card) — decide in Phase 5.
5. **Overlapping-restic-target behavior** — unresolved upstream; neutralized by pruning, but don't rely on restic deduplicating overlapping positionals.

---

## Sources

Aggregated from the four research files (see each file for full detail):

- **Codebase (HIGH, primary):** `handlers.go:4143` (`handleBrowse`), `service.go` (`toContainerPath` :1137, `toHostPath` :3684, `SetBackupPaths` :3766, configured/effective :3816-3888, restore :5242-5462, `RestorePaths` :6752), `store/targets.go` (`selected_paths`, owned setter), `restic.go:357-384` (`BackupArgs`), `paths/paths.go` (`Resolve`), `Containers.tsx` (FoldersEditor, live-save conventions), `SnapshotFileTree.tsx`/`FolderBrowser.tsx`.
- **restic official docs (HIGH-MEDIUM):** 040_backup.rst — positional sources bypass excludes; `!` negation cannot re-include in excluded dirs; absolute/relative path change detection; snapshot `paths`/`excludes`; parent selection by `host,paths`; symlinks stored not followed.
- **Go stdlib (HIGH):** `os.Root` (1.24+), `os.ReadDir`/`DirEntry`, go 1.25 floor.
- **React/W3C (HIGH-MEDIUM):** react.dev fetch-in-event-handler + Set-state patterns; W3C APG checkbox (mixed) and treeview patterns.
- **Competitor docs (HIGH for Duplicati/Veeam/Kopia/Backrest/restic; LOW for Synology/Backblaze/CrashPlan — bot-blocked):** tree+tri-state UX model, junk-filter precedents, and the restic-frontend tree-selection gap; Unraid community Plex guide as demand proof.
- **Derived analysis (MEDIUM):** empty-selection/auto-detect collision, future-children gap, coverage-diff design.

---
*Synthesis of research for: BombVault tree-based sub-folder backup selection*
