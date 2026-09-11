# Phase 4: File Sets Parity - Research

**Researched:** 2026-09-10
**Domain:** Brownfield feature wiring — extending the Phase 2 selection tree + Phase 1 flat-selection engine to the files domain (Go `internal/api`/`internal/store`/`internal/backup` + React SPA `Files.tsx`)
**Confidence:** HIGH (codebase-grounded; every integration seam read from source this session)

## Summary

Phase 4 is a parity phase, not a greenfield one: every hard problem (cascade semantics, mixed-state classification, flat-set normalization, empty-selection refusal, longest-prefix restore mapping, containment discipline) was solved in Phases 1–3 and is unit-testable in isolation. The phase reduces to five additive seams: (1) one append-only SQLite migration adding a **nullable** `selected_paths` column to `file_sets`; (2) one owned store setter beside the `SetFileSetScheduleCadence` precedent; (3) one compile step inside `BackupFileSet` that turns the flat set into restic positionals + derived `--exclude` patterns, mirroring the container line at `service.go:4256`; (4) one additive pointer field on the PATCH `/api/files/sets/{id}` body plus an additive `FileSetView` field; (5) mounting `SelectionTree` on the Files page with a single root (the set's resolved `Path`) and the set's flat set as the (I, E) mirror. `FilesRestic.Backup` already accepts `paths []string`, and `GET /api/browse` already serves mount-root-relative listings — **neither the restic interface nor the browse endpoint changes**.

One CONTEXT correction surfaced by reading the code: CONTEXT D-01 says the orchestrator is "unchanged" — that is true of the `FilesRestic` **interface** but false of the **file**. `BackupFileSetDir` wraps a single `SourceDir string` into `[]string{d.SourceDir}` at `files_orchestrator.go:44`, so multi-root positionals require a minimal additive deps field (recommended: `SourcePaths []string`, nil ⇒ legacy byte-identical `[SourceDir]`). This is a three-line change plus test-fake updates, but the planner must plan it or criterion 2 is unreachable.

The two genuine design hazards are (a) **set `Path` edits orphaning `selected_paths`** — a stale entry compiles into a positional *outside* the set's current root, silently broadening backup scope — and (b) the **empty-selection refusal shape**, which differs from the container guard because the files domain has no auto-detection fallback to protect. Both are flagged below with recommendations; neither has a precedent conflict.

**Primary recommendation:** Reuse, do not port. The flat set for a file set stores absolute mount-root-space paths under the set's resolved root (the same space `SourceDir` already occupies), passes through the **same** `NormalizeSelection`/`includesOnly`/`excludedBranches` helpers, mounts `SelectionTree` with one root and `hostMountRoot` as the browse prefix, and pins the legacy NULL-column argv byte-identical with the existing `TestBackupFileSet`.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Modèle de sélection — multi-root**
- **D-01:** Multi-root (option recommandée par la note ROADMAP et le research) : l'utilisateur peut décocher la racine du set et cocher plusieurs sous-dossiers → plusieurs positionals restic. Strict superset du single-root (un seul enfant coché = cas particulier). `FilesRestic.Backup` prend déjà `paths []string` — l'orchestrateur `internal/backup/files_orchestrator.go` est inchangé.
- **D-02:** Périmètre de l'arbre : la racine visible unique est le `Path` résolu du set (`paths.Resolve(HostMountRoot, set.Path)`). La multi-root se joue entièrement SOUS cette racine. Pas de siblings hors `Path`, pas de browsing arbitraire (Out of Scope REQUIREMENTS) — `FolderBrowser` reste le boundary picker à la création ; le create est inchangé, l'arbre apparaît dès qu'un `Path` existe (les sets Discover path-less et disabled n'affichent pas d'arbre tant que le path n'est pas défini).

**Persistance**
- **D-03:** Nouvelle migration append-only ajoutant une colonne nullable `selected_paths` TEXT sur `file_sets` (jamais éditer une migration livrée — NUMBERING HAZARD `internal/store/migrate.go`). Encodage = le MÊME flat set que `backupPaths` (entrées bare + `!`-préfixées, normalisation `internal/api/selection.go`, miroir client `selectionTree.ts`). NULL ou absent ⇒ comportement legacy byte-identique (positional unique `SourceDir`). Le champ `Path` existant n'est jamais réécrit par la sélection (zero impact sur les sets existants — philosophie SELECT-02 appliquée aux filesets).
- **D-04:** Surface wire : champ additif au corps PATCH `/api/files/sets/{id}` (pattern corps-à-pointeurs de `handlePatchFileSet`, decodeBody DisallowUnknownFields 1 MiB) — `selectedPaths` avec le nom/la struct exacts à la discrétion du planning tant qu'additif et validé au boundary : chaque entrée passe la discipline de confinement sous le mount root (pattern `toContainerPath` Phase 3), cap d'entrées (64, precedent excludeCaches), rejet atomique du save entier. Type miroir doc-commenté dans `web/src/lib/api.ts`, servi par la vue (extension de `FileSetView`).

**Compilation backup**
- **D-05:** Le flat set se compile en includes maximaux sous `Path` = positionals restic ; les branches d'exclusion stockées sont enforcees en `--exclude` au site unique (discipline gap-closure 01-05). `selected_paths` NULL/absent ⇒ argv `[SourceDir]` byte-identique legacy. Critère de succès 2 : snapshot `Paths` = les racines cochées.
- **D-06:** Sémantique sélection vide : un set entièrement décoché est REFUSÉ client + serveur (même posture que la garde Phase 1/ton fail D-04 Phase 3). Le files domain n'a PAS de fallback auto-détection (le `Path` est explicite) — donc pas d'équivalent « Reset » : le message de refus oriente vers la suppression du set (DELETE existant), seule issue saine. Jamais un set vide silencieux (cardinal sin).

**Portée de la parité (surfaces de confiance)**
- **D-07:** Portées avec l'arbre : le preview « N chemins » par racine (comptage client dérivé de la MÊME classification `selectionTree.ts` — doit matcher les positionals exacts, precedent D-01 Phase 3) et la liste reviewable des exclusions sous la racine (INTEG-03 pattern, vue audit non interactive — la mutation passe uniquement par l'arbre). Le toggle CACHEDIR.TAG est HORS SCOPE pour les file sets (RESTIC-01 est container-scoped) — deferred (voir deferred).

**Restauration / compat**
- **D-08:** Les anciens snapshots mono-path d'un set devenu multi-root restent restaurables : réutiliser le mapping longest-prefix RESTORE-01 entre la path list du set et les snapshot `Paths` (intersection vide ⇒ abort AVANT teardown, message scrubbed). UNE implémentation du mapping, pas une seconde sémantique files-domain.

### Claude's Discretion
- Libellés i18n et budget de nouvelles clés (parité 42 locales + tests orphans, pattern Phase 2/3) ; em-dashes interdits dans le texte utilisateur
- Placement/rendu dans `Files.tsx` (PAGE_SHELL, tokens `carbon-*`/`status*`, classes `glim-*` ; couleurs de statut sur badges jamais sur contrôles)
- Structure interne : réutiliser `SelectionTree` tel quel avec une source de racines adaptée vs wrapper léger — au planning, à condition de n'ajouter AUCUNE seconde implémentation de la sémantique (toggles/classification/state dérivent des mêmes helpers)
- Forme wire exacte du champ `selectedPaths` (nom, struct Go/TS) tant qu'additive + validée au boundary
- Stratégie de tests : co-localisés `.test.ts`/`.dom.test.tsx`, `restic_args_test.go` si l'argv est touché, white-box `*_internal_test.go` pour helpers non exportés

### Deferred Ideas (OUT OF SCOPE)
- Toggle CACHEDIR.TAG (`--exclude-caches`) pour les file sets — RESTIC-01 est container-scoped ; hors critères de succès phase 4. Candidat v2/milestone suivant si demandé
- TREE-07 search/filter, TREE-08 restore-side tree, SELECT-05 size hints, SELECT-06 coverage diff — déjà v2 (REQUIREMENTS.md)
- Fanout « exclude this subfolder instead » vers l'ExcludesEditor — hors v1 (Out of Scope PROJECT.md)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INTEG-02 | File Sets page — the same tree component is used when choosing what a file set covers | `SelectionTree.tsx` contract verified reusable (props at `SelectionTree.tsx:76-108`); `selectionTree.ts` semantics helpers are component-independent; `GET /api/browse` already serves the lazy listing under the set's Path (`handlers.go:4244-4356`); success criterion 1 = mount the tree with one root per D-02; criterion 2 = the compile step (F3) + orchestrator field (F1) + migration (F2) round-trip verified by `TestBackupFileSet` extension |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Tree selection UI (lazy expand, cascade, mixed-state) | SPA (`web/src/pages/Files.tsx` + `components/SelectionTree.tsx`) | — | The component already owns this; files page only supplies roots + mirror (D-02 discretion) |
| Selection normalization / persistence encoding | API tier (`internal/api/selection.go`, `handlers.go` boundary) | Store (opaque JSON string) | `selection.go` is the single semantic owner; `internal/store` never interprets entries (its file doc says so) |
| Boundary validation of `selectedPaths` | API tier (`handlePatchFileSet` → service validator) | — | decodeBody + per-entry containment + 64 cap happen before any store write |
| Compile selection → restic argv | API tier (`BackupFileSet`, `service.go:8706`) | Orchestrator (`internal/backup/files_orchestrator.go` carries the list) | Single compile site; scheduler/batch/everything callers inherit it; `internal/backup` stays adapter-free |
| Flat-set storage | Store tier (`internal/store/filesets.go` + migration v101) | — | Owned setter; SELECT/scan lists extended |
| Lazy child listings | API tier (`GET /api/browse`) | — | Zero change; already mount-root-relative with status trio + cap 500 |
| Restore mapping (D-08) | API tier (`prepareRestoreFileSet`/`runRestoreFileSet` + `mapRestorePaths`) | — | `mapRestorePaths` doc already names Phase 4 reuse; one implementation, no files-domain second semantic |
| Preview count + exclusions review list | SPA (derives from `selectionTree.ts` helpers) | — | D-07: client count must equal the positionals; helpers guarantee it |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| (none new — Go stdlib + existing SPA deps) | — | This phase installs **zero** new dependencies | House constraint: hand-rolled tree, stdlib `net/http`, no state library; Phases 1–3 shipped with zero installs |

### Supporting (already in repo, touched by this phase)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `modernc.org/sqlite` | v1.56.0 | The `selected_paths` migration + owned setter | v101 append-only ALTER |
| `internal/api/selection.go` (in-repo) | — | `NormalizeSelection`, `SplitExclusion`, `includesOnly`, `excludedBranches`, `mapRestorePaths` | ALL encoding/classification/restore-mapping — never reimplemented |
| `web/src/lib/selectionTree.ts` (in-repo) | — | `splitFlatSet`, `toFlatList`, `classifyNode`, `applyToggle`, `rootIncludeCount`, `rootExclusions` | Client mirror of the same semantics |
| `web/src/components/SelectionTree.tsx` (in-repo) | — | The tree component (TREE-01..05 already met) | Mounted with a single root (D-02) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Additive `SourcePaths []string` on `FileSetBackupDeps` | Reuse `SourceDir` with a sentinel separator | Sentinel-string hacks break the typed-builder argv discipline; additive field keeps `nil ⇒ legacy` explicit and testable |
| JSON column `'[]'` NOT NULL DEFAULT (container style) | Nullable column (D-03, prescribed) | For containers, `'[]'` means "auto-detect" — a state that does not exist for file sets (D-06 refuses empty). NULL cleanly separates "never touched by the tree" (legacy argv) from any tree write; NOT NULL would force a meaningless sentinel |
| Mounting `SelectionTree` unmodified with adapter props | Wrapping it in a files-specific component | Either is sanctioned (CONTEXT discretion); the lock is that `(I, E)` classification/toggle helpers stay the shared ones — no fork |

**Installation:**
```bash
# None. Zero npm installs, zero go get. (Precedent: Phase 2 note "zero npm installs".)
```

## Package Legitimacy Audit

No external packages are installed by this phase (Go stdlib + existing `web/package.json` dependencies only; verified: no new imports required by any seam above). No legitimacy checks were therefore required. If planning introduces an unexpected dependency, run the Package Legitimacy Gate before adding it.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```text
                       Files.tsx (SPA)                                     Go backend
  ┌─────────────────────────────────────────────┐      ┌──────────────────────────────────────────────────┐
  │ FileSetRow / editor                          │      │                                                  │
  │   │ GET /api/files  → FileSetView.selectedPaths     │ handleListFileSets → ListFileSetViews            │
  │   ▼                                          │      │   (serves selectedPaths + existing fields)       │
  │ splitFlatSet → (includes, exclusions) mirror │      │                                                  │
  │   │                                          │      │                                                  │
  │   ▼                                          │      │                                                  │
  │ SelectionTree (single root = set.Path)       │      │ GET /api/browse?path=<set.Path>/<sub>            │
  │   lazy expand ───────────────────────────────┼──────┼─▶ os.OpenRoot(HostMountRoot) + paths.Resolve     │
  │   toggle → applyToggle → mirrorRef           │      │   (status trio, cap 500 + truncated)             │
  │   │ zero includes? ──▶ client refusal (D-06) │      │                                                  │
  │   ▼                                          │      │                                                  │
  │ serialized PATCH queue ──────────────────────┼──────┼─▶ PATCH /api/files/sets/{id}                     │
  │   body {selectedPaths: toFlatList(I,E)}      │      │   handlePatchFileSet: pointer body               │
  │                                              │      │   → per-entry containment under resolved root    │
  │                                              │      │   → 64 cap → NormalizeSelection →               │
  │                                              │      │   → zero includes ⇒ REFUSE (D-06)                │
  │                                              │      │   → SetFileSetSelectedPaths (owned setter)       │
  │                                              │      │                                                  │
  │ preview "{n} paths" = rootIncludeCount       │      │ SQLite file_sets.selected_paths (nullable)       │
  │ exclusions list = rootExclusions (D-07)      │      │   NULL ⇒ legacy                                  │
  └─────────────────────────────────────────────┘      └──────────────────────────────────────────────────┘
                                                              │
   Backup trigger (button / scheduler / batch / everything)   ▼
                                              ┌──────────────────────────────────────────────────┐
                                              │ BackupFileSet (service.go:8706)  — single site   │
                                              │  GetFileSet (fresh read)                         │
                                              │  src = paths.Resolve(HostMountRoot, set.Path)    │
                                              │  selected_paths NULL?                            │
                                              │     ├─ yes → positionals = [src]   (LEGACY argv) │
                                              │     └─ no  → positionals = includesOnly(...)     │
                                              │              excludes = set.Excludes             │
                                              │                        + excludedBranches(...)   │
                                              └──────────────────────────────────────────────────┘
                                                              ▼
                                              ┌──────────────────────────────────────────────────┐
                                              │ BackupFileSetDir(files_orchestrator.go)          │
                                              │  FileSetBackupDeps{SourcePaths (NEW, additive)}  │
                                              │  → restic backup <repo> … -- <positional roots>  │
                                              │  snapshot.Paths == ticked roots (criterion 2)    │
                                              └──────────────────────────────────────────────────┘
                                                              ▼
                                              ┌──────────────────────────────────────────────────┐
                                              │ Restore (D-08): prepareRestoreFileSet            │
                                              │  stored list = compiled positionals (or [src])   │
                                              │  mapRestorePaths(stored, chosen.Paths)           │
                                              │  empty intersection ⇒ abort BEFORE restore work  │
                                              └──────────────────────────────────────────────────┘
```

### Recommended Project Structure
All files exist; this phase edits them, creates none except the migration entry and tests:
```text
internal/store/
├── migrate.go              # v101 appended LAST (after v100) — never renumber
└── filesets.go             # FileSet.SelectedPaths + SetFileSetSelectedPaths + SELECT/scan lists
internal/api/
├── handlers.go             # handlePatchFileSet: additive SelectedPaths *[]string
├── service.go              # BackupFileSet compile; prepareRestoreFileSet/runRestoreFileSet (D-08)
└── selection.go            # READ-ONLY reuse — helpers already exist (mapRestorePaths doc names Phase 4)
internal/backup/
└── files_orchestrator.go   # FileSetBackupDeps.SourcePaths []string additive (see F1)
web/src/
├── pages/Files.tsx         # tree mount point, mirror, queue, refusal, preview/exclusions
├── components/SelectionTree.tsx  # reuse; at most additive optional props (no semantic fork)
├── lib/selectionTree.ts    # READ-ONLY reuse
└── lib/api.ts              # FileSetView.selectedPaths + patchFileSet field (doc-commented mirror)
```

### Pattern 1: The compile step (mirrors the container WR-01 enforcement)
**What:** At backup time, turn the stored flat set into maximal-root positionals plus derived `--exclude` patterns.
**When to use:** Inside `BackupFileSet` only — the single production compile site for the domain.
**Example:**
```go
// Source: internal/api/service.go:4256 (the container precedent, verbatim shape):
//   Excludes: append(s.resolveExcludePatterns(tg.Excludes, in), excludedBranches(tg.SelectedPaths)...),
// and internal/api/selection.go:236-237 (verbatim):
//   "Containers call it today;
//    File Sets reuse it in Phase 4 (01-CONTEXT.md restore Q1/D-13)."
//
// File-set compile (recommended shape):
var positionals []string
excludes := set.Excludes
if set.SelectedPaths == nil {
    positionals = []string{src} // legacy byte-identical (pinned by TestBackupFileSet)
} else {
    positionals = includesOnly(set.SelectedPaths)             // includesOnly: selection.go:131
    excludes = append(append([]string{}, set.Excludes...),
        excludedBranches(set.SelectedPaths)...)               // excludedBranches: selection.go:185
}
```

### Pattern 2: Owned store setter (never fold into UpdateFileSet)
**What:** `SetFileSetSelectedPaths(id string, paths []string)` writes only its column, JSON-encoded, `nil`⇒`"null"` or clear-to-NULL.
**When to use:** The tree PATCH path. `UpdateFileSet` keeps its explicit column list, so name/path/excludes/enabled edits can never clobber a selection (the #199 cadence rationale, `filesets.go:139-146` verbatim rationale applies word-for-word).
**Example:**
```go
// Source: internal/store/filesets.go:147-157 (SetFileSetScheduleCadence — the shape to mirror)
// and internal/store/targets.go:529-547 (SetExcludeCaches — the JSON-column shape).
// The existing UPDATE already protects the new column (verbatim, filesets.go:75):
//   UPDATE file_sets SET name = ?, path = ?, excludes = ?, enabled = ? WHERE id = ?
```

### Pattern 3: Additive PATCH pointer field + boundary validation
**What:** `SelectedPaths *[]string` in `handlePatchFileSet`'s anonymous body struct; nil = untouched; when present, every entry validated, capped, normalized, refused-when-empty — atomically (one bad entry rejects the whole save).
**Example:**
```go
// Source: internal/api/handlers.go:4498-4507 (existing pointer body, verbatim fields):
//   Name *string   `json:"name"`
//   Path *string   `json:"path"`
//   Excludes *[]string `json:"excludes"`
//   Enabled *bool   `json:"enabled"`
//   ScheduleCadence *string `json:"scheduleCadence"`
// DisallowUnknownFields means the field MUST be declared even though optional
// (handlers.go:1118-1122 container precedent, verbatim rationale:
//  "decodeBody runs DisallowUnknownFields, so an undeclared
//   selectionSource would reject every tree save at the boundary.").
```

### Pattern 4: The serialized save queue (client)
**What:** All `selectedPaths` PATCHes from the Files page funnel through a one-deep queue over a ref mirror; a drain sends the LIVE mirror's latest full list once.
**When to use:** Always — the Files page has no queue today; the container panel precedent is the required shape (STATE.md Phase 2/3 decisions: rapid toggles must never race; revert re-derives from the live mirror).
**Example:** `Containers.tsx:816-851` (`queueRef`, `mirrorRef`, `owedRef`, `pendingRowsRef`) and the drain at `:944-1099` (`body.backupPaths = toFlatList(live.inc, live.exc)` at `:1021`).

### Anti-Patterns to Avoid
- **Second classification implementation:** never compute node state or counts in Files.tsx with new prefix logic — `classifyNode`/`applyToggle`/`rootIncludeCount`/`rootExclusions` are the only semantics (CONTEXT discretion lock).
- **Interpreting entries in the store:** `internal/store` treats selected_paths as an opaque string (selection.go file doc: "the store treats entries as opaque strings").
- **Storing relative paths in selected_paths:** positionals must equal `includesOnly` output directly; storing mount-root-relative entries would force a re-resolution step and break the "flat set IS the positional truth" invariant.
- **Editing/renumbering any migration:** v101 appended after v100, fresh number, unconditional body (the alreadySatisfied guard exists ONLY for the contested 89/90/91 recovery — do not reach for it; migrate.go:50-67).
- **Silent empty selection:** never persist (or accept) a tree-written selected_paths with zero includes (D-06).
- **`--exclude` as selection encoding:** exclusions from the tree ride `excludedBranches` on the argv tail at the single site, exactly like containers; never per-root exclude patterns as storage.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cascade/mixed-state/exclusion semantics | Files-specific toggle logic | `selectionTree.ts` `applyToggle`/`classifyNode` | Remembered-partial cycle (Phase 2 D-01) is subtle and test-pinned; a fork drifts |
| Flat-set normalize + prune + canonical order | New normalize for files | `internal/api/selection.go` `NormalizeSelection` | Per-class pruning, orphan preservation, byte-identical canonical order — all pinned by tests |
| Exclusion enforcement on argv | String-munging `--exclude` patterns ad hoc | `excludedBranches` | Equals-pair and orphan-exclusion shapes deliberately not emitted (pinned by `TestExcludedBranches`) |
| Restore mapping | First-path-component matching | `mapRestorePaths` | Two-pass longest-prefix, skip report, empty-intersection contract — RESTORE-01 |
| Directory listing for the tree | New files-domain listing endpoint | `GET /api/browse` | os.Root containment, status trio, cap 500 already hardened (Phase 1); one browsable universe |
| Per-entry containment check | Hand-written prefix compare | `paths.Resolve` + segment-aligned `isAtOrUnder`/`paths.Within` | `"/a"` never matches `"/ab"`; the strict-prefix primitive is easy to get subtly wrong |
| Save racing | Ad-hoc debounce | The one-deep queue pattern | Last-toggle-wins semantics with revert derived from the live mirror (Phase 2 Pitfall 5 closure) |

**Key insight:** This phase's risk is *not* in any algorithm — it is in accidentally creating a second copy of a semantic that already exists. The repo's own architecture (ports-and-adapters, single normalization owner, mirrored TS helpers) makes the reuse path the *shortest* path.

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `file_sets` rows (existing deployments): no `selected_paths` until migration v101 runs; after it, every existing row reads NULL = legacy argv. No data backfill needed or wanted. `runs`/`targets` untouched. | Migration v101 only (code edit, no data migration) |
| Live service config | None — file sets live entirely in SQLite; no external service holds file-set config. | None — verified by reading `internal/store/filesets.go` (no external sync exists) |
| OS-registered state | None — file sets are not OS-registered (scheduler reads SQLite at reload; `SetFileSetScheduleCadence`'s `reloadScheduler` pattern in `handlePatchFileSet:4556-4568` exists for cadence only and is NOT triggered by selection writes). | None — verified |
| Secrets/env vars | None — selection adds no env vars or secret keys. | None |
| Build artifacts | `web/dist` is embedded in the binary — any `web/` change requires `cd web && npm ci && npm run build` and committing `web/dist`, or the binary ships the stale SPA. SQLite WAL note: migrations run at boot before store open (`cmd/bombvault` startup order) — no manual step. | Commit `web/dist` in the web plan's final task |

## Common Pitfalls

### Pitfall 1: The orchestrator single-path wrap (CONTEXT D-01 literal claim is false)
**What goes wrong:** Planning assumes `internal/backup/files_orchestrator.go` needs no change and discovers at execution that criterion 2 is unreachable — `FileSetBackupDeps.SourceDir string` + `[]string{d.SourceDir}` caps positionals at one.
**Why it happens:** D-01 correctly notes `FilesRestic.Backup` takes `paths []string` (the interface) but the deps struct funnels a single string.
**How to avoid:** Additive field `SourcePaths []string`; orchestrator uses it when non-nil, else legacy `[]string{SourceDir}`. Update `files_orchestrator_test.go` fakes. Interface untouched (no adapter changes beyond the struct literal at `service.go:8766`).
**Warning signs:** Any plan task list with no `internal/backup` edit while claiming multi-root backups.

### Pitfall 2: set `Path` edits orphaning `selected_paths` (silent scope broadening)
**What goes wrong:** User saves a tree selection under `Path=A`, later edits `Path` to `B` (allowed — only *rename* is refused once backups exist, `handlers.go:4533-4547`). The stored entries now sit outside the new root; a naive compile hands restic positionals outside the set's declared scope — the backup silently covers folders the set no longer names.
**Why it happens:** selected_paths is stored absolute (mount-root space); the anchor it was validated against is mutable.
**How to avoid:** Two layers. (1) PATCH-time rule (recommended): when `body.Path != nil` and it changes the resolved root, clear selected_paths in the same save (the selection was expressed against the old root and no longer means anything). (2) Compile-time defense (mandatory regardless): filter entries to those at-or-under the freshly resolved `src`; if filtering empties the list, fall back to legacy `[src]` — never emit an unanchored positional. Decide and document the UX note (a path edit discards the sub-folder selection).
**Warning signs:** A compile that trusts stored entries without re-anchoring to the current `set.Path`.

### Pitfall 3: Empty-selection guard shape differs from containers
**What goes wrong:** Copying the container guard (`selectionSource == "tree"` + prior-non-empty + `errEmptySelection`) verbatim. For file sets the semantics are wrong in two ways: there is no auto-detection to fall back to (the `Path` is explicit), and the tree field only exists when the tree wrote it — no source carrier is needed.
**Why it happens:** Superficial similarity to `SetBackupPaths` (`service.go:3819-3876`).
**How to avoid:** At the files PATCH boundary: if `SelectedPaths` present and `includesOnly(NormalizeSelection(...))` is empty ⇒ refuse (client pre-PATCH block + server refusal), message orienting to DELETE (D-06). A coded envelope (`codedFailEnvelope(err, "empty-selection")`, `handlers.go:1161` precedent) keeps UI routing uniform.
**Warning signs:** A `selectionSource` field added to the files PATCH body.

### Pitfall 4: Nullable column handling in the three SELECTs + scan
**What goes wrong:** Adding the column to the SELECT lists but scanning it as `string` — every legacy row (NULL) fails the scan, taking down `ListFileSets`/`GetFileSet` at runtime.
**Why it happens:** Every existing `file_sets` column is NOT NULL with a default; there is no nullable-scan precedent in this file (nullable precedent exists elsewhere: `received_repos.last_check_ok INTEGER` nullable, migrate.go:826, scanned via sql.Null types).
**How to avoid:** Scan into `*string`/`sql.NullString` (or `COALESCE`); map NULL ⇒ nil slice on `FileSet.SelectedPaths`. Touch ALL THREE SELECTs (`filesets.go:90, 111, 119`) + `scanFileSet` (`:175-188`) — the settings.go "four positional lists" hazard in miniature. `CreateFileSet`'s INSERT (`:54`) can omit the column (stores NULL — verified: nullable ADD COLUMN without default accepts INSERTs omitting it).
**Warning signs:** `SELECT` lists updated but `scanFileSet` unchanged, or vice versa.

### Pitfall 5: Preview count drifting from argv
**What goes wrong:** The "{n} paths" preview counts checked nodes on screen instead of stored maximal includes; the number then disagrees with the positionals (success-criterion 1/SELECT-03 regression the Phase 3 plan pinned for containers).
**How to avoid:** Count via `rootIncludeCount(root, includes)` — the same classification the compile consumes. The pinned Phase 3 rule transfers verbatim: "the visible number equals the bare positionals the next backup hands restic" (STATE.md Phase 3).
**Warning signs:** Any count derived from loaded children or DOM state.

### Pitfall 6: SelectionTree prop-shape frictions treated as a rewrite trigger
**What goes wrong:** The component's root types are container-shaped (`mounts: MountInfo[]`, `customPaths: CustomPath[]`, `hostSourceRoot` prefix, `containerName` localStorage scope, required `excludeCaches`/`onToggleCaches` props). A planner might conclude the component must be forked — violating the "zero second implementation" lock.
**How to avoid:** Verify against the verified props (`SelectionTree.tsx:76-108`): either (a) feed one root through the existing shapes (a mount-like row whose `source` is the resolved set root), passing an empty `excludeCaches` map + no-op `onToggleCaches` (RESTIC-01 is container-scoped and out of scope — D-07), or (b) make `excludeCaches`/`onToggleCaches` (and possibly `onRemoveCustom`) optional — additive, non-semantic. Browse translation prefix is `hostMountRoot` for files (`browse()` takes mount-root-relative paths, `api.ts:2024-2027`; `Files.tsx:1289` already holds `hostMountRoot`), NOT `hostSourceRoot`. localStorage key: scope by set id (comfort state only, D-05 Phase 2).
**Warning signs:** A new tree component file, or edits to `classifyNode`/`applyToggle`.

### Pitfall 7: Restore mapping applied to the wrong "stored list"
**What goes wrong:** D-08 says map "la path list du set" — using `set.Path` (one entry) makes the mapping trivially map to `Paths[0]` and hides multi-root snapshots; using the raw flat set feeds exclusion-prefixed entries into a selector list.
**How to avoid:** The list to map is the set's **compiled positional list** — legacy `[src]` when NULL, else `includesOnly(selected_paths)` re-anchored per Pitfall 2 — matched against `chosen.Paths` through `mapRestorePaths` (`selection.go:239`), with the container guard shape (`service.go:5515-5548`): empty mapped ⇒ abort before work with a scrubbed message; `paths.Within` re-check over mapped selectors.
**Warning signs:** `snapshotSubtree` (Paths[0], `service.go:6580-6590`) still being the sole multi-root restore selector.

## Code Examples

### Verbatim facts every plan task should cite

**The orchestrator seam (internal/backup/files_orchestrator.go:14-16, 20-22, 44):**
```go
// Source: internal/backup/files_orchestrator.go (read this session)
type FilesRestic interface {
    Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error)
}
// ...
// SourceDir is the container-visible resolved path of the set's folder
// (paths.Resolve(HostMountRoot, set.Path), e.g. /host/user/data/docs).
SourceDir string
// ...
summary, err := d.Restic.Backup(ctx, d.Repo, []string{d.SourceDir}, []string{"fileset:" + d.SetName}, d.Excludes...)
```

**The latest migration number and the file_sets column precedent (internal/store/migrate.go:1302-1315):**
```go
// Source: internal/store/migrate.go (read this session)
version: 99, name: "file_sets_schedule_cadence",
alreadySatisfied: columnPresent("file_sets", "schedule_cadence"),
sql:              "ALTER TABLE file_sets ADD COLUMN schedule_cadence TEXT NOT NULL DEFAULT '';",
// ...
version: 100, name: "target_exclude_caches",
sql: "ALTER TABLE targets ADD COLUMN exclude_caches TEXT NOT NULL DEFAULT '{}';",
// → Phase 4 appends v101. file_sets was created at v64 (migrate.go:566-574):
//   CREATE TABLE file_sets (
//     id         TEXT    PRIMARY KEY,
//     name       TEXT    NOT NULL UNIQUE,
//     path       TEXT    NOT NULL,
//     excludes   TEXT    NOT NULL DEFAULT '[]',
//     enabled    INTEGER NOT NULL DEFAULT 1,
//     created_at INTEGER NOT NULL
//   );
```
Note on the guard: v99 carries `alreadySatisfied` because its body shipped under review contention; a normal new migration "gets a fresh number and runs unconditionally" (migrate.go:22-23) — v101 needs **no** guard.

**The store SELECT lists that must all gain the column (internal/store/filesets.go:90, 111-112, 119-120):**
```sql
-- Source: internal/store/filesets.go (read this session; all three identical column lists)
SELECT id, name, path, excludes, enabled, schedule_cadence, created_at
FROM file_sets ORDER BY name
```
```go
// scanFileSet signature site: filesets.go:175-188 — Scan of exactly those 7 columns.
// UpdateFileSet never touches the new column (verbatim, filesets.go:75):
//   UPDATE file_sets SET name = ?, path = ?, excludes = ?, enabled = ? WHERE id = ?
```

**The backup compile site and its fresh read (internal/api/service.go:8718, 8725-8731, 8766-8774):**
```go
// Source: internal/api/service.go (read this session)
set, err := s.store.GetFileSet(id)
// ...
if strings.TrimSpace(set.Path) == "" {
    return backup.Summary{}, fmt.Errorf("files backup: file set %q has no source path configured. Set a path before backing up", set.Name)
}
src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
// ...
sum, err := backup.BackupFileSetDir(fctx, backup.FileSetBackupDeps{
    SourceDir: src,
    Repo:      repo,
    TargetID:  set.ID,
    SetName:   set.Name,
    Excludes:  set.Excludes,
    Restic:    &resticAdapter{engine: s.engine, mode: mode},
    Runs:      runsAdapter{st: s.store, ctx: ctx, svc: s},
})
```
All backup triggers funnel here (verified call sites): manual `StartBackupFileSet` (`service.go:4737`), batch `backupFileSetOneForBatch` (`:4819`), Backup Everything (`everything.go:429`) — the compile inherits them all.

**The container exclusion-enforcement line to mirror (internal/api/service.go:4256):**
```go
// Source: internal/api/service.go (read this session)
Excludes: append(s.resolveExcludePatterns(tg.Excludes, in), excludedBranches(tg.SelectedPaths)...),
```

**The container empty-selection refusal (the shape D-06 adapts, NOT copies — internal/api/service.go:3802, 3870-3875; handlers.go:1160-1162):**
```go
// Source: internal/api/service.go, handlers.go (read this session)
var errEmptySelection = errors.New("an explicit empty selection would re-enable automatic appdata detection")
// ...
if selectionSource == "tree" && len(normalized) == 0 {
    if prior, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(prior.SelectedPaths) > 0 {
        return errEmptySelection
    }
}
// handlers.go:
if errors.Is(err, errEmptySelection) {
    writeJSON(w, http.StatusOK, codedFailEnvelope(err, "empty-selection"))
```

**The browse contract (internal/api/handlers.go:3004, 4245-4249, 4276, 4331, 4350):**
```go
// Source: internal/api/handlers.go (read this session)
const maxBrowseEntries = 500
// request:  ?path=<relative-under-HostMountRoot>  (?hidden=1 opt-in)
root, err := os.OpenRoot(h.cfg.HostMountRoot)
// child paths are built mount-root-relative: entryPath = subpath + "/" + name
// response: {"ok":true, "root": <HostMountRoot>, "path": <subpath>, "dirs":[{name, path}], "status":"ok", "truncated":bool}
```

**The restore mapping to reuse (internal/api/selection.go:236-239, 245-249; service.go:5524-5527):**
```go
// Source: internal/api/selection.go (read this session) — doc comment verbatim:
// "Containers call it today;
//  File Sets reuse it in Phase 4 (01-CONTEXT.md restore Q1/D-13)."
func mapRestorePaths(stored, snapshotPaths []string) (mapped, skipped []string)
// Container guard shape (service.go:5524-5527):
mapped, skipped := mapRestorePaths(tg.AppdataPaths, chosen.Paths)
if len(tg.AppdataPaths) > 0 && len(mapped) == 0 {
    return containerRestorePlan{}, errors.New("nothing to restore for this item from this snapshot")
}
```
Current file-set restore for contrast (service.go:9108-9115): in-place = `s.engine.RestorePath(ctx, plan.repo, plan.snapshotID, plan.inPlace, plan.mode)`; to-folder = `RestoreSubtreeTo(..., plan.subtree, ...)` where `plan.subtree = snapshotSubtree(snaps, snapshotID)` = `sn.Paths[0]` (`service.go:6583-6584`). Neither intersects a stored list today — D-08 is new wiring, not a refactor of existing mapping.

**The wire surface (internal/api/service.go:8799-8819 JSON tags; web/src/lib/api.ts:2264-2285, 2348-2364):**
```go
// Source: internal/api/service.go — FileSetView fields (read this session):
// ID `json:"id"`, Name `json:"name"`, Path `json:"path"`, Excludes `json:"excludes"`,
// Enabled `json:"enabled"`, LastBackup `json:"lastBackup"`, PathExists `json:"pathExists"`,
// ScheduleCadence `json:"scheduleCadence"`, EffectiveSchedule `json:"effectiveSchedule"`
// → additive: SelectedPaths []string `json:"selectedPaths,omitempty"` (planner names it; D-04)
```
```ts
// Source: web/src/lib/api.ts (read this session) — patchFileSet patch shape:
// patch: { name?: string; path?: string; excludes?: string[]; enabled?: boolean;
//          scheduleCadence?: string }
// → additive: selectedPaths?: string[] (doc-commented mirror, D-04)
// FileSetView at api.ts:2264: id, name, path, excludes, enabled, lastBackup,
// scheduleCadence?, effectiveSchedule?, pathExists
```

**The tree component contract (web/src/components/SelectionTree.tsx:76-108, condensed verbatim):**
```ts
// Source: web/src/components/SelectionTree.tsx (read this session)
export interface SelectionTreeProps {
  mounts: MountInfo[];            // level-1 roots (container-shaped)
  customPaths: CustomPath[];      // trailing level-1 treeitems
  includes: ReadonlySet<string>;  // HOST path space — the server-truth mirror
  exclusions: ReadonlySet<string>;// "!" stripped — dormant entries included
  hostSourceRoot: string;         // the browse translation prefix
  containerName: string;          // scopes bv-tree-expanded-{name} (D-05)
  browseCache: Map<string, Promise<BrowseResponse>>; // editor-lifetime
  onToggle: (hostPath: string) => void;
  onRemoveCustom: (hostPath: string) => void;
  excludeCaches: Readonly<Record<string, boolean>>;  // RESTIC-01 — files: n/a
  onToggleCaches: (hostPath: string, next: boolean) => void;
  busyPaths?: ReadonlySet<string>;
  shakeCounts?: Readonly<Record<string, number>>;
  blockedPath?: string | null;
}
```

**Shared client semantics helpers (web/src/lib/selectionTree.ts — exports verified this session):**
`EXCLUSION_PREFIX ("!")`, `NodeState`, `FlatSets`, `isAtOrUnder`, `isStrictlyUnder`, `hostToBrowseRel`, `browseRelToHost`, `splitFlatSet`, `toFlatList`, `partitionCustomPaths`, `rootIncludeCount`, `rootExclusions`, `classifyNode`, `applyToggle`, `loadExpanded`, `saveExpanded`.

**Namespace translation facts (internal/config/config.go:83-84; internal/api/service.go:1132-1148; internal/paths/paths.go:33, 58):**
```go
// Source: internal/config/config.go (read this session)
HostMountRoot:    stringOr(env["HOST_MOUNT_ROOT"], "/host/user"),
HostSourceRoot:   stringOr(env["HOST_SOURCE_ROOT"], "/mnt"),
// Source: internal/api/service.go:1132-1136 (doc comment verbatim):
// "toContainerPath translates a HOST path under HostSourceRoot to its
//  container-visible equivalent under HostMountRoot (the broad Host Data mount,
//  e.g. /mnt → /host/user). Returns ("", false) when the host path is not
//  reachable through the mount."
// → A file set's resolved root is ALREADY mount-root space; toContainerPath does
//   NOT apply to file-set entries. Containment = at-or-under the resolved root.
// Source: internal/paths/paths.go: `func Resolve(root, sub string) (string, error)` (strict
//   containment, ErrTraversal/ErrAbsoluteSub), `func Within(root, absPath string) bool`.
```

**The legacy argv pin (internal/api/service_test.go:293-295):**
```go
// Source: internal/api/service_test.go (read this session)
if len(eng.lastPaths) != 1 || eng.lastPaths[0] != srcDir {
    t.Fatalf("expected the set's resolved source dir, got %v", eng.lastPaths)
}
```

**i18n scale (web/src/lib/i18n.ts:1-37; directory count verified):** header says "42 locales"; `en` + `de` inline as source of truth + **40** lazy chunk files in `web/src/lib/locales/` (counted 40). Gates: `i18n.parity.test.ts`, `i18n.quality.test.ts`, `i18n.orphans.test.ts`. Existing `folders.*` keys (e.g. `folders.previewPaths`, `folders.exclusions`, `folders.emptySelectionBlocked`, `folders.truncatedList`, `folders.retry`) are candidates for reuse where the wording is not container-specific.

## State of the Art

Nothing in this phase's stack moved since the milestone research (2026-09-09) — restic 0.17.3 and rclone 1.74.2 are checksum-pinned in the Dockerfile; Go/React/Vite are lockfile-pinned and Renovate-managed. The relevant "current approach" is the milestone's own: maximal-root positionals + `excludedBranches` enforcement (WR-01 gap closure, 2026-09-09) — treat any restic-exclude-based selection encoding as superseded/wrong (restic excludes not applying to positional *targets* is the original disqualifier; within-source exclusion enforcement is the settled hybrid).

**Deprecated/outdated:**
- `--group-by paths`/`--group-by tags` for retention: permanently banned (#91, live `tag` discipline) — not touched here, listed so no task "helpfully" adds grouping.
- `UpdateSettings`-style full-row writes: production-forbidden; the owned-setter pattern is the current rule.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | CONTEXT D-01's "orchestrateur inchangé" is read as interface-unchanged; the file needs an additive deps field (`SourcePaths []string`) | Summary, Pitfall 1, Code Examples | If the user truly wants zero `internal/backup` edits, multi-root positionals are impossible without an alternate orchestrator path (a second implementation — worse). Confirm at planning if contested |
| A2 | selected_paths stores **absolute mount-root-space** paths (same space as `SourceDir`), not set-root-relative | Architecture Patterns, Pitfall 2 | If relative was intended, compile needs a Resolve step and D-05's "flat set = positionals" reading changes; wire shape unchanged |
| A3 | Recommended rule: PATCH `path` change clears selected_paths (Pitfall 2 layer 1) | Pitfalls | If the user prefers keep-and-refuse-at-backup or keep-and-clamp, only the PATCH rule and its UI note change; compile-time defense (layer 2) is needed under every variant |
| A4 | Files-page tree reuses `SelectionTree` with additive optional props / adapter props rather than a fork | Pitfall 6, Standard Stack | If a fork is chosen, the CONTEXT "zero second implementation" lock is at risk — flagged as discretion but constrained |
| A5 | Expansion localStorage for files is keyed per set id (comfort state, D-05 analogy) | Pitfall 6 | Cosmetic; wrong keying only affects expansion restore |
| A6 | The tree renders even for sets whose root is currently missing (`pathExists:false`), classifying from (I, E) with browse rendering `missing` status rows | Architecture | If undesired, gate rendering on `pathExists` — one-line planner call |

## Open Questions (RESOLVED)

All three recommendations below were adopted during planning; the resolving plan/task is cited per question.

1. **Restore wiring depth for D-08** (in-place only vs also to-folder)
   - What we know: in-place restore (`RestorePath`) restores the whole snapshot; to-folder uses `Paths[0]` via `snapshotSubtree`. `mapRestorePaths` returns per-selector lists; the adapter's `RestorePaths` loops `RestorePath` per selector (`service.go:6939-6946`).
   - What's unclear: whether the planner wires mapping into the in-place path only (minimum D-08) or also reworks to-folder's single-subtree assumption for multi-root snapshots.
   - Recommendation: wire mapping into in-place restore (the destructive-overwrite case D-08 names); leave to-folder whole-tree semantics unless multi-root snapshots demonstrably break it — document the choice in the plan.
   - **RESOLVED → 04-02 Task 2:** adopted. The guard computes the compiled list in `prepareRestoreFileSet` and aborts pre-teardown on empty mapping; to-folder `snapshotSubtree` whole-tree semantics deliberately untouched, with the choice recorded in a why-comment at the guard.

2. **Empty-selection refusal code + message routing**
   - What we know: container refusal uses `code:"empty-selection"`; D-06 prescribes client + server refusal with a DELETE-orienting message.
   - What's unclear: reuse `"empty-selection"` (uniform UI routing) vs a files-specific code (cleaner telemetry).
   - Recommendation: reuse `"empty-selection"` — the Files page can share the Phase 2/3 guard plumbing and wording pattern.
   - **RESOLVED → 04-02 Task 1:** adopted. New files-specific sentinel (`errFileSetEmptySelection`, DELETE-orienting message per D-06) routed through `codedFailEnvelope(err, "empty-selection")` so the SPA keeps one refusal code.

3. **Where the Files-page tree lives visually** (row-expansion like Containers vs inside `FileSetDialog`)
   - What we know: D-02 says the tree appears whenever a `Path` exists; the dialog owns name/path/excludes editing; live-save toggles favor a row-level editor with the serialized queue (dialog PATCHes are full-set saves — mixing a live mirror into the dialog's `patchFileSet(initial.id, {name, path, excludes, enabled})` at `Files.tsx:911` would race).
   - Recommendation: row-level expansion (FoldersEditor precedent) with a dedicated selection queue; dialog stays as-is.
   - **RESOLVED → 04-03 Task 2:** adopted. Row-level "Choose folders" disclosure with the dedicated one-deep queue; `FileSetDialog` keeps its full-set PATCH (04-04 adds only the `files.pathChangeHint` caption under the FolderBrowser, no selection surface in the dialog).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | `go build/vet/test` | ✓ | go1.25.0 (go.mod floor) | CI runs 1.26 |
| Node | `cd web && npm run build` (tsc + vite), vitest | ✓ | v24.16.0 | — |
| restic >= 0.17 | `go test ./...` (contract tests) | ✗ (not on PATH) | — | Windows dev box: POSIX tests skip by design (CLAUDE.md caveat); full suite proven in golang:1.26 + restic 0.17.3 container (STATE.md 2026-09-10) and CI Test job installs 0.17.3 |
| golangci-lint | lint gate | ✗ (not on PATH) | — | CI Lint job is the gate; `just` also absent locally |
| npm ci artifacts | vitest run | ✓ (node_modules present) | — | `npm ci` if clean checkout |

**Missing dependencies with no fallback:** none blocking (restic/lint gaps are CI-covered; locally the suite runs reduced — an accepted, documented dev-box state).
**Missing dependencies with fallback:** restic + golangci-lint + just → CI.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `go test` (no testify); vitest ^4.1.10 + @testing-library/react ^16.3.2 + jsdom ^30.0.1 |
| Config file | none for Go (stdlib); `web/vite.config.ts` + `web/tsconfig.json` for web |
| Quick run command | `go test ./internal/api/ -run 'TestBackupFileSet|TestFileSet' && go test ./internal/store/ -run FileSet` |
| Full suite command | `go test ./...` (restic >= 0.17 needed for contract tests — CI); `cd web && npx vitest run` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INTEG-02 (crit. 1) | Tree selection on Files page: lazy expand, cascade, mixed, exact reopen | dom (vitest+RTL) | `cd web && npx vitest run src/pages/Files.tree.dom.test.tsx` | ❌ Wave 0 (precedent: `Containers.tree.dom.test.tsx`) |
| INTEG-02 (crit. 2a) | Save round-trips through the flat normalization | unit (Go) | `go test ./internal/api/ -run TestPatchFileSetSelectedPaths` | ❌ Wave 0 (extend handlers/service tests) |
| INTEG-02 (crit. 2b) | Next backup snapshot `Paths` == ticked roots | unit (Go, fake engine capture) | `go test ./internal/api/ -run TestBackupFileSet` (new subtests) | ✅ extend `service_test.go:262` |
| INTEG-02 (legacy) | NULL selected_paths ⇒ argv `[src]` byte-identical; `TestBackupFileSet` stays green untouched | unit (Go) | `go test ./internal/api/ -run TestBackupFileSet$` | ✅ (already pins it) |
| INTEG-02 (D-06) | Empty selection refused client + server | unit + dom | `go test ./internal/api/ -run TestFileSetEmptySelection`; vitest dom | ❌ Wave 0 |
| INTEG-02 (D-07) | Preview count == positionals; exclusions list | unit (vitest node-env) | `cd web && npx vitest run src/lib/selectionTree.test.ts` (+ new file-set cases) | ✅ extend |
| INTEG-02 (D-08) | Restore maps longest-prefix; empty intersection aborts pre-work | unit (Go) | `go test ./internal/api/ -run TestRestoreFileSet` (+ new mapping cases); `mapRestorePaths` cases already in `selection_internal_test.go` | ✅ extend |
| INTEG-02 (D-03) | Migration v101 applies; legacy rows read NULL; round-trip | unit (Go) | `go test ./internal/store/ -run FileSet` | ✅ extend (`migrate_upgrade_internal_test.go` pattern) |
| INTEG-02 (argv) | Orchestrator passes `SourcePaths` through; nil ⇒ `[SourceDir]` | unit (Go) | `go test ./internal/backup/ -run TestBackupFileSetDir` | ✅ extend `files_orchestrator_test.go` |
| INTEG-02 (i18n) | New keys × 42 locales parity/quality/orphans | unit (vitest) | `cd web && npx vitest run src/lib/locales` | ✅ |
| INTEG-02 (real restic, optional) | Multi-positional snapshot Paths at 0.17.3 | contract (skips without restic) | `go test ./internal/restic/ -run Positionals` | ✅ extend `restic_positionals_contract_test.go` pattern |

### Sampling Rate
- **Per task commit:** domain-scoped `go test` + touched vitest files
- **Per wave merge:** `go test ./...` + `cd web && npx vitest run` + `go build ./... && go vet ./... && gofmt -l .`
- **Phase gate:** full suite + `cd web && npm ci && npm run build` (commit `web/dist`) before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/src/pages/Files.tree.dom.test.tsx` — Files-page tree dom harness (no Files page test exists today)
- [ ] `internal/api` tests for the selectedPaths PATCH boundary (extend existing file-set handler/service test files)
- [ ] No framework install needed (both toolchains present)

## Security Domain

security_enforcement is enabled; ASVS level 1 applies. The phase adds user-influenced path data crossing three trust boundaries; the categories below are the relevant ones.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | Existing session middleware; no new endpoint |
| V3 Session Management | no (unchanged) | — |
| V4 Access Control | no (unchanged) | All `/api/files/*` routes already behind the same auth gate (`api.go` route table) |
| V5 Input Validation | **yes** | `decodeBody` (1 MiB MaxBytesReader + DisallowUnknownFields + JSON-only) already wraps the PATCH; NEW: per-entry `TrimSpace` + non-empty + segment-aligned containment under `paths.Resolve(HostMountRoot, mergedPath)` + 64-entry cap + atomic whole-save rejection |
| V6 Cryptography | no (unchanged) | — |
| V14 Config/argv | **yes** | Positionals only via typed builder args after `--`; entries never enter flags; derived excludes ride the existing variadic (no shell anywhere) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via selectedPaths entries (`..`, absolute escapes) | Elevation/Tampering | `paths.Resolve` (ErrTraversal/ErrAbsoluteSub) on the merged root + at-or-under check per entry (segment-aligned; `/a` never matches `/ab`) before any store write |
| Stale-entry scope broadening after a `Path` edit | Tampering | Compile-time re-anchor (Pitfall 2 layer 2) + PATCH-time clear rule (layer 3 recommendation) |
| Glob metacharacters in stored entries reaching derived `--exclude` patterns | Tampering (accepted, bounded) | Precedent T-01-05-01 (`selection.go:179-184`): same semantics user-written excludes already have on this authenticated single-admin surface; blast radius bounded by positional roots — carry the same acceptance comment forward |
| Silent empty/empty-looking selection (data-loss class) | Denial of service (backup misses data) | D-06 refusal client + server; never a silent no-op |
| argv injection via paths | Tampering | No shell; restic exec is argv-built by typed builders; user-influenced positionals after `--` (house discipline, unchanged) |
| Error-message path leakage | Information disclosure | Scrubbed failures: paths → `[path]` first (house order), ≤300-char restic reasons, `truncateErr` for runs — existing helpers, reuse as-is |

## Sources

### Primary (HIGH confidence)
- Codebase, read this session with line ranges: `internal/backup/files_orchestrator.go` (14-16, 19-34, 39-53); `internal/store/filesets.go` (10-34, 53-57, 74-85, 88-122, 147-157, 175-188); `internal/store/migrate.go` (50-67, 566-574, 1302-1315); `internal/api/selection.go` (full file); `internal/api/service.go` (1125-1148, 3790-3885, 4244-4261, 5505-5550, 6931-6960, 8700-8889, 8994-9133, 6580-6590); `internal/api/handlers.go` (66, 1105-1170, 3004, 4244-4356, 4448-4570); `internal/paths/paths.go` (11-15, 33, 58); `internal/config/config.go` (22-23, 83-84); `internal/api/service_test.go` (257-309); `web/src/lib/selectionTree.ts` (full file); `web/src/components/SelectionTree.tsx` (1-150); `web/src/lib/api.ts` (460-497, 857-899, 2024-2048, 2259-2464); `web/src/pages/Files.tsx` (1-63, 863-995, 1279-1316); `web/src/pages/Containers.tsx` (760-910); `web/src/lib/i18n.ts` (1-37)
- restic official docs via Context7 (`/restic/restic`, 040_backup.rst / 075_scripting.rst): snapshot `paths` = "List of paths included in the backup"; `ls` header `snapshot <id> of [<paths>]`; absolute positionals preserved — cross-checked against in-repo real-restic 0.17.3 contract tests (`internal/restic/restic_positionals_contract_test.go`, `TestPositionalExcludesKeepSourceDir`, `TestPositionalExcludeAbsoluteSubdirPattern`)

### Secondary (MEDIUM confidence)
- SQLite ALTER TABLE ADD COLUMN semantics via Context7 (`/sqlite/sqlite`, `src/alter.c`): nullable column without DEFAULT is legal; NOT NULL-without-default is rejected; existing rows report NULL — cross-checked against in-repo nullable-column precedent (`received_repos.last_check_ok`, migrate.go:826)

### Tertiary (LOW confidence)
- None — no claim in this document rests on training memory alone

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; everything is in-repo and was read this session
- Architecture: HIGH — every seam (orchestrator wrap, compile site, browse contract, restore path, PATCH body, SELECT lists, migration number) verified with file:line and verbatim quotes
- Pitfalls: HIGH — all seven are grounded in read code (two — orchestrator wrap, restore-mapping absence — are direct corrections/clarifications to CONTEXT assumptions); the two restic/SQLite external behavior claims are doc-cited and contract-test-crossed

**Research date:** 2026-09-10
**Valid until:** 2026-10-10 (stable: brownfield phase, pinned toolchain, no external deps)
