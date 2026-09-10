# Phase 3: Selection Trust & Controls - Research

**Researched:** 2026-09-10
**Domain:** Selection-trust UI over the Phase 1/2 selection contracts (Go backend: persistence + argv; React SPA: preview, exclusions list, deselect semantics, CACHEDIR.TAG toggle)
**Confidence:** HIGH (in-repo seams) / MEDIUM (restic flag semantics — official-docs cited)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

*(Copied verbatim from 03-CONTEXT.md — the French original is authoritative.)*

### SELECT-03 — Aperçu de sélection effective
- **D-01:** L'aperçu « N chemins » est une ligne par racine de l'arbre, dérivée côté client du flat set live (le miroir de sauvegarde) : nombre d'includes maximaux relevant de cette racine. Zéro nouvel endpoint — le flat set EST la vérité positionnelle (les includes maximaux du flat set sont exactement les positionnels que la prochaine sauvegarde transmettra à restic, contrat Phase 1). Le comptage client doit reproduire la classification serveur (`SplitExclusion`/pruning — miroir dans `selectionTree.ts`), et le nombre affiché doit matcher ce que l'argv contiendra, pas un comptage de nœuds cochés à l'écran.
- **D-02:** Déclencheur de la note de rétrécissement : à chaque sauvegarde de sélection réussie où le nombre d'includes de l'item DIMINUE par rapport à la sélection précédente, ET l'item a ≥1 snapshot antérieur (signal « a déjà des snapshots » déjà servi au panel — le planner vérifie le champ exact : lastBackup/runs). La note dit que les snapshots futurs ne contiendront que les dossiers sélectionnés (c'est la communication de la sémantique future-children acceptée en Phase 1, prescrite par la note ROADMAP sous SELECT-03). Pas de comparaison au contenu des snapshots existants — c'est le coverage diff différé en v2 (SELECT-06).

### INTEG-03 — Exclusions reviewables
- **D-03:** Sous chaque racine qui porte des exclusions, une section repliable « N exclusions » liste les sous-branches désélectionnées (chemins relatifs à la racine, rendu muted cohérent avec le preview). Vue d'audit NON interactive : la mutation passe uniquement par l'arbre (un seul pipeline de toggle, thème D-02/T-02-10 de la Phase 2) — pas de re-check depuis la liste en v1.

### INTEG-04 — Sémantique UI du tout-décoché
- **D-04:** Une racine entièrement décochée (plus aucun include relevant, exclusions devenues orphelines-dormantes) rend comme une rangée racine en état « non sélectionné » avec compteur d'exclusions mémorisées — jamais masquée : les exclusions orphelines sont signifiantes (01-CONTEXT Q3, état anti-auto-détection), l'UI doit les montrer comme mémoire, pas les cacher.
- **D-05:** Sortie explicite vers l'auto-détection (clôture du deferred 01-CONTEXT Q4) : un contrôle « Réinitialiser la sélection » en pied de section envoie PATCH `backupPaths: []` avec `selectionSource` ≠ `"tree"` (franchit la garde Phase 1, qui ne refuse le `[]` nu que depuis la source arbre) → retour documenté à l'auto-détection, suppression des exclusions orphelines. Le message du blocage D-04 (zéro-include) est enrichi pour référencer ce contrôle. Le reset passe par la même queue sérialisée one-deep que les toggles.

### RESTIC-01 — Toggle CACHEDIR.TAG par racine
- **D-06:** Le toggle vit par racine (fidèle au requirement per-mount) mais se compile en flag item-level : dès qu'≥1 racine de l'item l'active, l'argv porte `--exclude-caches` (union). L'UI documente la portée réelle (tooltip : s'applique à toute la sauvegarde de l'item) — jamais de silence sur un effet qui déborde de la racine où il est actionné. Flag global restic placé avant les positionnels (discipline argv), testé dans `restic_args_test.go`.
- **D-07:** Persistance : colonne append-only sur `targets` (JSON map racine→bool, owned setter sur le modèle `SetExcludes` — `internal/store/targets.go:489`), champ additif au corps PATCH `/api/containers/{name}` (aux côtés de `backupPaths`/`selectionSource`, `handlers.go:1112`). Jamais la settings row (per-container n'y vit pas ; MutateSettings deadlock pattern).

### Claude's Discretion
- Libellés i18n exacts et budget de nouvelles clés (à définir au planning ; parité 42 locales + tests orphans obligatoires, pattern Phase 2)
- Placement/rendu précis du toggle sous la racine et du contrôle « Réinitialiser » (tokens `carbon-*`/`status*` + classes `glim-*` ; couleurs de statut sur badges, jamais sur contrôles)
- Structure interne des composants preview/liste exclusions (suivre les précédents `ExcludesEditor`/`FolderBrowser`)
- Forme wire exacte du champ PATCH (nom, struct Go/TS) tant qu'elle est additive et validée au boundary

### Deferred Ideas (OUT OF SCOPE)
- Fanout « exclude this subfolder instead » vers l'ExcludesEditor — follow-up hors v1 (ROADMAP note)
- Coverage diff backup-time (siblings ni sélectionnés ni exclus) → v2 (SELECT-06, décision utilisateur 2026-09-09)
- File Sets : même preview/liste/toggle portés à la parité arbre — Phase 4 (INTEG-02)
- TREE-07 search/filter, SELECT-05 size hints → v2 (REQUIREMENTS.md)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SELECT-03 | Effective-selection preview — "N paths" per mount matching exactly the restic positionals + narrowing note | Client-side derivation from the existing `(includes, exclusions)` mirror (D-01); `lastBackup` is the "has snapshots" signal (verified below); note is event-driven at save time (D-02) — zero new endpoints |
| INTEG-03 | Exclusions reviewable after the fact — visible list near the mount, consistent with existing preview styling | Per-root derivation from the exclusions set already maintained in `selectionTree.ts`; non-interactive list; muted sub-label styling precedent verified in `SelectionTree.tsx` |
| INTEG-04 | Defined UI semantics for full deselect — never silently re-trigger auto-detection | Phase 1 backend guard verified (`errEmptySelection`, source-gated); reset control sends `backupPaths: []` without `selectionSource:"tree"` through the existing one-deep queue; `classifyNode` already renders a fully-deselected root "unchecked" |
| RESTIC-01 | Per-mount/root CACHEDIR.TAG toggle → restic `--exclude-caches` in `BackupArgs`, covered by `restic_args_test.go` | `--exclude-caches` verified against official restic docs (semantics + placement); persistence via append-only `targets` column + owned setter (v53/v100 template verified); two threading shapes analyzed with a recommendation |
</phase_requirements>

## Summary

Phase 3 is a trust-and-legibility layer over contracts that already exist: the Phase 1 flat-set encoding (`!` exclusions, `NormalizeSelection`, `includesOnly`, `excludedBranches`), the Phase 1 empty-selection guard (`errEmptySelection`, strictly gated on `selectionSource:"tree"`), the Phase 2 tree + `(includes, exclusions)` mirror + one-deep serialized PATCH queue in `FoldersEditor`, and the `BackupArgs` argv builder with its exclude loop. Three of the four requirements (SELECT-03, INTEG-03, INTEG-04) are **pure client derivations from state the panel already holds** — zero new endpoints, zero new state sources. Only RESTIC-01 touches the backend: one append-only SQLite column on `targets` (next migration number is **v100**), one owned setter, one additive PATCH field, one additive mounts-response field, one boolean threaded from `service.Backup` into `BackupArgs` as the constant flag `--exclude-caches`.

The two design points that need the planner's attention are (1) how the boolean reaches `BackupArgs` — the repo has a documented precedent (`Mode.Limits`, `restic.go:89-101`) for riding per-backup knobs on `restic.Mode` precisely to avoid widening the `backup.Restic` interface and every fake; the alternative (a `BackupDeps.ExcludeCaches` field + interface widening) is more explicit but touches 5+ interfaces/fakes. (2) The reset (D-05) must ride the existing PATCH queue but with a **different** `selectionSource` than the hardcoded `"tree"` the queue's drain sends today — `attemptSave` (Containers.tsx:851) needs a per-attempt source, and the narrowing-note trigger (D-02) must compare the attempted save against the **last-saved** include count (not the pre-mutation count) because the queue collapses bursts into one drain.

The i18n cost is the standing Phase 2 constraint: every new user-visible string × 42 locale tables (en + de inline in `i18n.ts` + 40 lazy `locales/*.ts`), gated by parity/quality/orphans tests. Budget at roughly 5–7 new keys plus one enrichment of `folders.emptySelectionBlocked`. `web/dist` must be rebuilt and committed after any `web/` change, and the full Go chain (`build/vet/gofmt/golangci-lint/test`) runs before push — golangci-lint and just are NOT on PATH on this Windows dev box, so the lint gate runs in CI (Phase 2 precedent).

**Primary recommendation:** Land RESTIC-01 backend first (migration v100 + setter + PATCH/mounts fields + `Mode.ExcludeCaches` → `BackupArgs` + `restic_args_test.go`), then the pure-frontend derivations (preview count, exclusions list, deselect rendering, reset through a per-attempt-source queue), then the CACHEDIR toggle UI, closing with i18n propagation across 42 locales + `web/dist` rebuild + full gate.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| "N paths" preview count (SELECT-03) | Browser / SPA | — | D-01 locks client-side derivation from the mirror; the flat set IS the positional truth (Phase 1 contract); zero new endpoints |
| Narrowing note (SELECT-03) | Browser / SPA | — | Event-driven at save time (include count ↓ vs last-saved + `lastBackup` signal); no server history of include counts exists, and none is needed |
| Reviewable exclusions list (INTEG-03) | Browser / SPA | — | Derived from the exclusions set the mirror already holds; read-only audit view |
| Deselect-all semantics + reset (INTEG-04) | Browser / SPA | API (existing guard) | The backend half landed in Phase 1 (`errEmptySelection` + coded envelope); the reset is a client-initiated PATCH `[]` without tree source |
| CACHEDIR.TAG toggle persistence (RESTIC-01) | API / Backend (store) | — | Per-container state lives in `targets` via an owned setter — never the settings row (D-07, deadlock pattern) |
| `--exclude-caches` argv emission (RESTIC-01) | API / Backend (restic engine) | — | Only `internal/restic` builds argv; flag placement after the `backup` verb, before `--` (verified against restic docs) |
| Union computation (per-root → item-level flag) | API / Backend (service) | — | `service.Backup` already computes the analogous `excludedBranches(tg.SelectedPaths)` union at the `BackupDeps` literal (service.go:4243) |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net/http` + `encoding/json` | Go 1.25 floor (CI 1.26) | PATCH field, mounts response field | House rule: no router/framework; `decodeBody` + `DisallowUnknownFields` already in place |
| `modernc.org/sqlite` via `internal/store` | v1.56.0 | Migration v100 + owned setter | Existing persistence layer; append-only migration discipline |
| React 19 + TypeScript 7 | ^19.2.7 / ^7.0.2 | All four UI features | Existing SPA; no state library — derivations from props/mirror |
| vitest + @testing-library/react | ^4.1.10 / ^16.3.2 | Unit + dom tests | Existing test stack (`selectionTree.test.ts`, `Containers.tree.dom.test.tsx` harness) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `Toggle.tsx` (in-repo) | — | CACHEDIR.TAG switch | If a switch is the chosen control; `role="switch"`, label always survives as accessible name |
| `Button.tsx` / `Badge.tsx` / `IconTipButton` / `InfoBubble` (in-repo) | — | Reset control, exclusions chevron, scope tooltip | Shared `glim-*` controls; tooltips for the item-wide-scope disclosure (D-06) |
| `useConfirm` (`web/src/lib/useConfirm.tsx`) | — | Optional reset confirmation | Reset deletes orphan exclusions too — destructive; discretion |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Client-derived preview count (D-01) | Server "effective selection" endpoint | Rejected by D-01 (locked): the flat set is already the positional truth; an endpoint would be a second source of truth that can drift |
| `restic.Mode` field for the flag | `BackupDeps.ExcludeCaches` + interface widening | Mode rides the documented Limits precedent (zero interface churn); BackupDeps is more explicit but touches `backup.Restic`, `ResticEngine`, `resticAdapter`, engine method, and every fake — see Pattern 4 |

**Installation:** none — zero new packages (npm or Go modules).

**Version verification:** No new packages to verify against registries; all versions above read from `web/package.json`, `go.mod`, and CLAUDE.md this session.

## Package Legitimacy Audit

No external packages are installed in this phase (locked by CLAUDE.md stack constraints and confirmed by the phase design — everything is stdlib + in-repo components).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| — (none) | — | — | — | — | — | N/A |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```text
 [SPA: FoldersEditor / SelectionTree]
   │  (includes, exclusions) mirror — HOST path space, loaded from GET mounts
   │
   ├─ D-01: per-root count = |{ i ∈ includes : i at-or-under root }|  ──► "N paths" line
   ├─ D-03: per-root list = { e ∈ exclusions : e strictly-under root } ──► collapsible muted list
   ├─ D-04: root with 0 relevant includes + N exclusions ──► unchecked row + remembered counter
   ├─ D-02: on save ok: attemptedCount < lastSavedCount AND container.lastBackup ≠ null
   │        ──► future-children note (transient, this-session)
   │
   ├─ toggle/save ──► one-deep queue ──► PATCH /api/containers/{name}
   │                                      body { backupPaths, selectionSource: "tree" }   (existing)
   │                                      body { backupPaths: [] }  ← D-05 reset (NO tree source)
   │                                      body { excludeCaches: {root: bool} } ← D-07 toggle (additive)
   │
   └─ GET /api/containers/{name}/mounts ──► + excludeCaches map (additive) ──► toggle state on load

 [API layer]
   ├─ handlePatchContainer: new pointer field → svc.SetExcludeCaches (boundary-validated)
   ├─ handleContainerMounts: serves the stored map additively
   └─ service.Backup (existing):
        tg (re-read, carries the new column)
        └─ union: any root true ──► mode.ExcludeCaches = true   (or BackupDeps field — planner)
                └─ resticAdapter.Backup(ctx, repo, paths, tags, mode, excludes...)
                        └─ restic.Backup → BackupArgs(...)
                                backup … --tag T… [--exclude-caches] --exclude E… -- <positionals>
                                                                  ▲ after the verb, before -- (argv discipline)
```

Trace of the primary use case (RESTIC-01): user flips the per-root toggle → PATCH `excludeCaches` → `targets.exclude_caches` (JSON map) via owned setter → next backup: `service.Backup` re-reads `tg`, computes the union, threads the boolean → `BackupArgs` appends `--exclude-caches` between the `--tag` block and the `--exclude` block → pinned by `TestBackupArgs*` in `restic_args_test.go`.

### Recommended Project Structure
```
internal/
├── restic/restic.go            # Mode.ExcludeCaches field (or param) + BackupArgs flag + doc comment
├── restic/restic_args_test.go  # TestBackupArgsExcludeCaches (+ absence pin)
├── store/migrate.go            # migration v100: target_exclude_caches (append at end)
├── store/targets.go            # Target field, 5 SQL lists, scanTarget, SetExcludeCaches
├── store/targets_test.go       # round-trip + Upsert-preserves test
├── api/handlers.go             # PATCH body field + mounts response field
└── api/service.go              # SetExcludeCaches svc method, ContainerMounts return, Backup union
web/src/
├── lib/selectionTree.ts        # pure helpers: rootIncludeCount / rootExclusions (+ tests)
├── lib/api.ts                  # wire types: PATCH field + mounts field (+ helper fn)
├── components/SelectionTree.tsx # per-root preview line, exclusions section, CACHEDIR toggle
├── pages/Containers.tsx        # FoldersEditor: per-attempt source queue, reset control, narrowing note
└── lib/i18n.ts + locales/*.ts  # ~5-7 new keys × 42 tables (+ enriched block message)
```

### Pattern 1: Per-root derivations are pure functions of the two sets (mirror D-01)
**What:** The preview count, the exclusions list, and the fully-deselected rendering derive ONLY from `(includes, exclusions)` — the same load-bearing rule as Phase 2's classifyNode. No new fetch, no server round-trip, no children.
**When to use:** All of SELECT-03/INTEG-03/INTEG-04 UI.
**Example:**
```typescript
// Derivation shape (mirror of server NormalizeSelection semantics — the stored
// includes are already maximal per class, so every include at-or-under a root
// IS one positional path):
//   count = [...includes].filter((p) => isAtOrUnder(p, root)).length
//   exclusions = [...exclusions].filter((p) => isStrictlyUnder(p, root))
//                 .map((p) => relative(p, root))          // strip the root prefix
// Both use the existing segment-aligned helpers in selectionTree.ts
// (isAtOrUnder/isStrictlyUnder, selectionTree.ts:69-82) — table-testable there.
```
Source: `web/src/lib/selectionTree.ts` (read this session; helpers quoted in Code Examples).

### Pattern 2: The narrowing note is event-driven, not state-derived (D-02)
**What:** Fire the note when a **successful** save's include count is lower than the **last successfully saved** count AND `container.lastBackup != null`.
**When to use:** SELECT-03's second half.
**Why last-saved, not pre-mutation:** the one-deep queue collapses bursts — the drain sends the LIVE mirror once with the latest desc. Comparing the attempted list to the count at last save handles stacked toggles correctly (5→3→4 nets to 5→4 = narrowing). Track a `lastSavedCount` ref initialized from the load-time mirror size and update it on every `r.ok`.
**Signal:** `Container.lastBackup` — verified verbatim below (handlers.go:531-533): non-null iff a successful backup run exists (orphan fallback: newest snapshot time). Thread it into `FoldersEditor` as a prop from `ContainerRow` (which already holds `container`, Containers.tsx:1922/2094).
**No persistence:** on reopen, "narrowing" is underivable (no server-side history of include counts) — the note is deliberately transient, matching "à chaque sauvegarde … où le nombre DIMINUE".

### Pattern 3: Reset through the queue with a per-attempt source (D-05)
**What:** The reset PATCH (`backupPaths: []`, NO `selectionSource:"tree"`) rides the SAME one-deep queue as toggles — but `attemptSave` hardcodes `selectionSource: "tree"` today.
**When to use:** INTEG-04 exit-to-auto-detection.
**Fix shape:** extend `SaveDesc` with the initiating mutation's selection source; the drain uses the latest desc's source. Toggle descs keep `"tree"` (guard live); the reset desc carries the non-tree source (absent field is fine — `setBackupPaths` in api.ts only includes `selectionSource` when truthy, api.ts:891-903, so calling it with no opts sends exactly `{backupPaths: []}`, which the Phase 1 guard passes by design). Edge cases resolve correctly: toggle stacked behind a reset → drain sends the non-empty live list with the toggle's `"tree"` source (guard only bites on empty); reset stacked behind a toggle → drain sends `[]` with the reset source.
**Server side (verified):** `SetBackupPaths` stores `[]` → `selected_paths = "[]"` → auto-detection restored, orphan exclusions gone with it.

### Pattern 4: Threading the CACHEDIR boolean to BackupArgs (planner decision, one recommended)
**What:** `--exclude-caches` is a constant flag (zero user input reaches argv), but the boolean must travel service → adapter → engine → builder.

**Option A (recommended): ride `restic.Mode`** — add `ExcludeCaches bool` to `Mode`; `service.Backup` sets it on the value copy it already builds per backup (`mode := s.primaryModeFor(...)`, service.go:4116 — a fresh value, safe to mutate); `BackupArgs` emits the flag when set. The repo's own documented precedent, verbatim from the `Limits` field comment (`internal/restic/restic.go:90-101`, quoted in Code Examples): a per-backup knob rides Mode *"because Mode is already threaded through every Backup/BackupStdin call site via the per-domain adapter … adding a new positional parameter would mean touching the backup.Restic interface and every Domain struct in internal/backup"*. Churn: `restic.go` (field + flag + tests) + one service line + fake-engine recording if a wiring test wants to observe mode. Zero changes to `internal/backup` — the ports-and-adapters seam stays untouched.

**Option B: `BackupDeps.ExcludeCaches bool` + widen `backup.Restic.Backup`** — more explicit dataflow, observable by orchestrator fakes, but touches: `backup.Restic` (orchestrator.go:98-117), every `internal/backup` fake implementing it, `resticAdapter.Backup` (service.go:6918), `ResticEngine.Backup` (service.go:73), `*restic.Restic.Backup` (restic.go:1688), `BackupArgs`, and `fakeResticEngine` (service_test.go:4312). This is exactly the churn the Limits comment documents avoiding.

**Either way:** the flag slots into `BackupArgs` beside the `--exclude` loop (after the `backup` verb, before `--`) — verified placement, see Code Examples. `BackupStdinArgs` (zvol path) is untouched: no folder structure, no CACHEDIR semantics.

### Pattern 5: Store column follows the SetExcludes template exactly (D-07)
**What:** JSON-text column on `targets`, owned setter, never in the ON CONFLICT update set.
**Template (v53, verbatim from migrate.go:496-501):**
```go
{
	// Per-container restic --exclude patterns applied to this container's backup.
	// JSON array; '[]' = none. Owned by SetExcludes (never reset by Upsert).
	version: 53, name: "target_excludes",
	sql: "ALTER TABLE targets ADD COLUMN excludes TEXT NOT NULL DEFAULT '[]';",
},
```
New migration: **version 100** (99 is the last, `file_sets_schedule_cadence`, migrate.go:1302-1305 — verified), name e.g. `target_exclude_caches`, `ALTER TABLE targets ADD COLUMN exclude_caches TEXT NOT NULL DEFAULT '{}';`. Append at the end, never renumber (NUMBERING HAZARD, migrate.go:50-67). `encoding/json` marshals Go maps with sorted keys, so `map[string]bool` stores deterministically — round-trip stable like the arrays.

**Every column addition touches FIVE positional SQL sites in `internal/store/targets.go`** (verified this session): the `Target` struct field, the INSERT column list in `UpsertTarget` (line 127 — add to columns+values, NOT to the ON CONFLICT SET), the three SELECT lists (`GetTargetByContainer` :146, `ListTargets` :154, `ListTargetsScheduleOrder` :196), and `scanTarget` (:542-565). Plus the round-trip test (`TestSetExcludesRoundTripAndUpsertPreserves` is the template, targets_test.go:59).

### Pattern 6: Boundary validation of the map keys (D-07 "validée au boundary")
**What:** The PATCH field decodes into `map[string]bool` (JSON rejects non-bool values at decode). Keys are host paths: validate each with the same discipline `SetBackupPaths` uses — split/translate via `toContainerPath` and reject the whole save if a key is not under the host mount (service.go:3839-3842 precedent), plus a small entry-count cap as defense-in-depth (1 MiB `MaxBytesReader` already bounds the body). Unknown-but-valid keys (a root that no longer exists) are harmless: they never match and never reach argv — only the boolean union does.
**Why strict:** the house rule is "Never trust a name past the handler"; path-shaped input gets path validation.

### Anti-Patterns to Avoid
- **Computing the preview from checked nodes on screen:** the count must come from the flat set (`toFlatList`'s input), not DOM/visual state — D-01's explicit "pas un comptage de nœuds cochés à l'écran". Collapsed and never-loaded subtrees classify from the sets.
- **Sending the reset around the queue** (a bare `setBackupPaths(name, [])` fire-and-forget): risks running concurrently with an in-flight toggle save — the exact T-02-08 class. D-05 locks queue serialization.
- **Interactive re-check in the exclusions list (D-03):** the list is an audit view; mutation goes only through the tree's toggle pipeline (one pipeline, T-02-10).
- **Hiding a fully-deselected root or its dormant exclusions (D-04):** orphan exclusions are the anti-auto-detection memory — render the counter, never filter it away.
- **Silent item-wide effect of the CACHEDIR toggle:** the tooltip disclosing "applies to the whole item's backup" is requirement-level (D-06), not polish.
- **Putting the per-root map in the settings row:** per-container state has never lived there; a store call inside a `MutateSettings` fn deadlocks on the single pooled connection (CLAUDE.md).
- **Editing/renumbering a shipped migration:** v100 appends; the `:latest` image publishes on every push to main (migrate.go:50-67).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Per-root classification/counting | New prefix-matching code | `isAtOrUnder`/`isStrictlyUnder`/`splitFlatSet` in `selectionTree.ts` | Segment-aligned, table-tested, the Phase 2 mirror of the server's `isStrictDescendant` |
| Serialized PATCH saves | A second ad-hoc queue | The existing one-deep queue (`scheduleSave`/`attemptSave`), extended with per-desc source | Proven against Pitfall 5 (revert never clobbers newer toggles); two queues would re-open concurrency |
| Toggle/switch control | Inline switch markup | `Toggle.tsx` (or the tree's checkbox idiom) | The one shared switch; focus-ring and label contracts already settled |
| Tooltip for scope disclosure | Custom hover div | `InfoBubble`/`IconTipButton` | Existing `glim-*` tooltip primitives |
| Confirmation for the reset | Custom modal | `useConfirm` | Existing confirm hook |
| New i18n mechanism | String concatenation / non-t() text | `t()` keys + the 42-table propagation pattern | Lint-enforced (`bombvault/*` rules: user text translated, no em dashes) |

**Key insight:** every UI value this phase shows is a pure function of state the panel already holds. The only new server state is one boolean per root, and only its UNION ever reaches argv.

## Runtime State Inventory

> Omitted — this phase adds a feature; it renames/migrates no existing strings or identities. The one schema change is an append-only additive column with a `'{}'` default (no data migration of existing rows: absence = "no root enabled" = today's behavior, byte-identical argv — pinned by the absence test).

## Common Pitfalls

### Pitfall 1: The i18n budget is 42 locales, not 2
**What goes wrong:** a key added to en+de only fails `i18n.parity.test.ts` ("has exactly the en key set" per locale); a key nothing renders fails `i18n.orphans.test.ts`.
**Why it happens:** en + de live inline in `i18n.ts`; the other 40 are lazy `locales/*.ts` files.
**How to avoid:** add keys to en + de inline, then propagate to all 40 locale files in the same task (Phase 2 precedent: one commit touched all 40 for 4 keys). Count: ~5-7 new keys (preview line with `{n}`, exclusions header with `{n}`, toggle label, scope tooltip, narrowing note, reset label) + enrichment of `folders.emptySelectionBlocked` (text change, all 42). No em dashes in user text (non-configurable lint).
**Warning signs:** parity test listing missing keys per locale; orphan test listing dead keys.

### Pitfall 2: The queue hardcodes `selectionSource: "tree"` — the reset would be refused
**What goes wrong:** wiring the reset through today's `attemptSave` sends `[]` with `"tree"` → the Phase 1 guard refuses it (`errEmptySelection`, service.go:3868-3872) → toast + revert, reset appears broken.
**Why it happens:** the drain body is built at Containers.tsx:851 with a fixed source.
**How to avoid:** per-desc source (Pattern 3). The api client already supports it: `setBackupPaths(name, [])` with no opts omits the field entirely.
**Warning signs:** dom test asserting the reset PATCH body carries no `selectionSource` (or a non-tree value).

### Pitfall 3: Narrowing-note trigger vs queue-collapsed saves
**What goes wrong:** comparing `desc.pre.includes.size` to `desc.sent.includes.size` mis-fires when bursts collapse (the drain sends the live mirror, not `desc.sent`), and misses net narrowing across stacked toggles.
**Why it happens:** the queue deliberately sends the latest list once.
**How to avoid:** compare the attempted (live) list size against a `lastSavedCount` ref updated on every successful save; init from the load-time mirror (Pattern 2).
**Warning signs:** note fires on a save that net-widened, or stays silent on a net-narrowing stack.

### Pitfall 4: `--exclude-caches` is a backup-subcommand flag, not a global one
**What goes wrong:** emitting it before the `backup` verb (with `--retry-lock`/`--limit-*`) — restic rejects unknown global flags / misplacement; existing tests pin the global-flag slot.
**Why it happens:** "flag global restic" in D-06 reads ambiguously; the operative phrase is "avant les positionnels".
**How to avoid:** append it in `BackupArgs` beside the `--exclude` loop (after `--json`/`--host`/`--tag`, before `--`) — the exact slot the existing tests pin for `--exclude` (restic_args_test.go:310-332). Verified against official docs (see Sources).
**Warning signs:** `TestBackupArgs*` wanting the flag anywhere left of `"backup"`.

### Pitfall 5: Forgetting that the preview must count STORED includes, not existing-on-disk ones
**What goes wrong:** trying to make the count match `onlyExistingPaths`-filtered argv exactly leads to client-side stat attempts or special cases.
**Why it happens:** success criterion 1 says "matches exactly the positional paths the next backup will hand restic", and the engine filters stale paths at run time (service.go:4144, `effectiveBackupPaths`).
**How to avoid:** D-01 locks the reading — the flat set IS the positional truth; count the stored/effective includes. The stale-path cases are already surfaced at row level (`folders.customMissing`, `folders.notReachable` labels). Planner should pin this in a test comment so the nuance is deliberate (see Open Questions Q3).
**Warning signs:** preview code reaching for `exists`/`reachable` flags to adjust counts.

### Pitfall 6: Concurrent PATCH fields from one editor
**What goes wrong:** the CACHEDIR toggle PATCH (`excludeCaches`) firing while a backupPaths save is in flight — the one-deep queue serializes only backupPaths PATCHes.
**Why it happens:** the toggle is a separate field on the same endpoint.
**How to avoid:** planner choice — either generalize the queue to serialize all container PATCHes from FoldersEditor, or route the toggle through its own one-deep serialized path. Server-side the fields are independent columns (no lost update between fields), but the house rule from Phase 2 is "jamais deux PATCH concurrents" from one editor (T-02-08).
**Warning signs:** two overlapping fetches to `/api/containers/{name}` in a dom test timeline.

### Pitfall 7: web/dist and the build chain
**What goes wrong:** `go build` embeds a stale SPA; CI/verify fails.
**How to avoid:** after any `web/` change: `cd web && npm ci && npm run build` (`tsc --noEmit && vite build`), commit `web/dist`. Full Go chain before push. Note: `gofmt`/`go vet`/`go test` run locally; **golangci-lint and just are NOT on PATH on this Windows box** (verified) — the lint gate lands in CI (Phase 2 precedent).
**Warning signs:** verify-work finding uncommitted dist.

### Pitfall 8: `DisallowUnknownFields` and the new PATCH field
**What goes wrong:** a new SPA field the server struct doesn't declare → every PATCH containing it rejected at the boundary.
**How to avoid:** declare the field in the `handlePatchContainer` body struct in the same change as the client (they ship in one binary, so skew is transient at worst — but tests decode real bodies). Pointer type (`*map[string]bool` or `map[string]bool` nil-check) so absent = untouched, matching every sibling field.

## Code Examples

### BackupArgs today — the insertion site for `--exclude-caches` (Go, pinned)
```go
// Source: internal/restic/restic.go:357-384 (read this session)
// BackupArgs returns the argv slice for `restic backup`.
// Tags are added with --tag; each exclude is added with --exclude (restic matches
// a bare name like ".git" by basename at any depth); paths are placed after --
// (arg-injection guard).
func BackupArgs(repo string, paths []string, tags []string, m Mode, excludes ...string) []string {
	args := repoFlag(repo)
	args = append(args, storageClassFlags(repo, m.StorageClass)...)
	args = append(args, retryLockFlags()...)
	args = append(args, limitFlags(m.Limits)...)
	args = append(args, "backup")
	if !m.Encrypted {
		args = append(args, insecureFlag)
	}
	args = append(args, "--json")
	// Pin a stable host (see backupHost) so a target's snapshots stay in one
	// restic group across container recreations; otherwise retention silently
	// stops collapsing snapshots after an update.
	args = append(args, "--host", backupHost)
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	for _, ex := range excludes {
		args = append(args, "--exclude", ex)
	}
	args = append(args, "--")
	args = append(args, paths...)
	return args
}
```
`--exclude-caches` slots between the `--tag` loop and the `--exclude` loop (or directly after it) — either is after the verb and before `--`; pick one and pin it byte-exactly in `TestBackupArgsExcludeCaches`, plus a zero-value absence pin (byte-identical to today, the `TestBackupArgsLimits` "zero Limits" precedent at restic_args_test.go:350-356).

### The Mode.Limits precedent for per-backup knobs (Go, pinned — the "why" for Pattern 4 Option A)
```go
// Source: internal/restic/restic.go:89-101 (excerpt)
	// Limits caps restic's transfer bandwidth (KiB/s each way) for a BACKUP run
	// against this repo — the primary-repo counterpart of CopyArgs' separate `lim
	// Limits` parameter. It travels on Mode (rather than as its own BackupArgs
	// parameter, CopyArgs' shape) because Mode is already threaded through every
	// Backup/BackupStdin call site via the per-domain adapter (internal/api's
	// resticAdapter/resticZvolAdapter); adding a new positional parameter would
	// mean touching the backup.Restic interface and every Domain struct in
	// internal/backup for a knob that is zero (unlimited, the default) unless
	// a domain's PRIMARY repo is both remote and has bandwidth limits configured
	// (see internal/api's primaryRemoteTarget). A zero Limits is a no-op, so every
	// local-primary and unconfigured-remote-primary backup is byte-identical to
	// before this field existed.
	Limits Limits
```

### The PATCH body struct to extend additively (Go, pinned)
```go
// Source: internal/api/handlers.go:1108-1128 (excerpt, read this session)
	var body struct {
		IncludeInSchedule *bool     `json:"includeInSchedule"`
		PreHook           *string   `json:"preHook"`
		PostHook          *string   `json:"postHook"`
		BackupPaths       *[]string `json:"backupPaths"`
		// SelectionSource is the optional intent carrier for a backupPaths save. …
		SelectionSource   *string   `json:"selectionSource"`
		StopContainers    *[]string `json:"stopContainers"`
		Excludes          *[]string `json:"excludes"`
		UpdateAfterBackup *bool     `json:"updateAfterBackup"`
		ScheduleCadence   *string   `json:"scheduleCadence"`
	}
```
Add e.g. `ExcludeCaches map[string]bool \`json:"excludeCaches"\`` (nil = untouched) with a why-comment; handle it beside the `body.Excludes` branch (handlers.go:1166-1171). D-07 names the exact placement "aux côtés de backupPaths/selectionSource".

### The empty-selection guard the reset must pass (and toggles must not) (Go, pinned)
```go
// Source: internal/api/service.go:3868-3873 (read this session)
	if selectionSource == "tree" && len(normalized) == 0 {
		if prior, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(prior.SelectedPaths) > 0 {
			return errEmptySelection
		}
	}
	return s.store.SetBackupPaths(name, normalized)
```
The refusal is strictly source-gated: a `[]` save with NO selectionSource (or any non-"tree" value) falls through to `store.SetBackupPaths(name, [])` → `selected_paths = "[]"` → auto-detection. That fall-through IS the reset (D-05) — no backend change needed for INTEG-04's exit.

### The mounts response to extend (Go, pinned)
```go
// Source: internal/api/handlers.go:1337-1343 (read this session)
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"mounts":         mounts,
		"custom":         custom,
		"excluded":       excluded,
		"hostMountRoot":  h.cfg.HostMountRoot,
		"hostSourceRoot": h.cfg.HostSourceRoot,
	}))
```
Add e.g. `"excludeCaches": <map[string]bool from the target row>` (nil-safe: serve `{}` when unset) — the toggle state arrives exactly when the tree renders. `ContainerMounts` (service.go:3722) already reads `tg` via `GetTargetByContainer`, so the map flows once `scanTarget` carries the column.

### The lastBackup signal for the narrowing note (Go, pinned)
```go
// Source: internal/api/handlers.go:531-534 (read this session)
		if run, _ := h.store.LastSuccessfulBackup(t.ID); run != nil {
			v.LastBackup = run.FinishedAt
			v.LastBackupStarted = &run.StartedAt
		}
```
Mirror type: `lastBackup: number | null` (`web/src/lib/api.ts:27`). `container.lastBackup != null` is the "≥1 prior snapshot" proxy D-02 names (orphan fallback adds newest-snapshot time, handlers.go:579-584 — even more snapshot-faithful).

### The client helpers every derivation reuses (TS, pinned)
```typescript
// Source: web/src/lib/selectionTree.ts:69-82 (read this session)
/** True when `path` equals or lives under `ancestor` (segment-aligned). */
export function isAtOrUnder(path: string, ancestor: string): boolean {
  const p = cleanPath(path);
  const a = cleanPath(ancestor);
  if (p === a) return true;
  const base = a === "/" ? "" : a;
  return p.startsWith(`${base}/`);
}

/** True when `path` lives STRICTLY under `ancestor` (segment-aligned). */
export function isStrictlyUnder(path: string, ancestor: string): boolean {
  const p = cleanPath(path);
  const a = cleanPath(ancestor);
  return p !== a && isAtOrUnder(p, a);
}
```
And the serialization the preview count must agree with (`toFlatList`, selectionTree.ts:129-139: sorted bare includes, then sorted `!`-prefixed exclusions — "the server's re-normalization is a no-op").

### The queue drain that needs a per-attempt source (TS, pinned)
```typescript
// Source: web/src/pages/Containers.tsx:845-851 (excerpt, read this session)
  async function attemptSave(desc: SaveDesc): Promise<void> {
    queueRef.current.inFlight = true;
    const rows = [...pendingRowsRef.current];
    pendingRowsRef.current.clear();
    try {
      const live = mirrorRef.current;
      const r = await setBackupPaths(name, toFlatList(live.inc, live.exc), { selectionSource: "tree" });
```
The hardcoded `{ selectionSource: "tree" }` is the exact line the reset (Pattern 3) must parameterize — carry the source on `SaveDesc`, default `"tree"`.

### restic's own words on --exclude-caches (external, cited)
> "Specify once to exclude a folder's content if it contains the special CACHEDIR.TAG file, but keep CACHEDIR.TAG."
> — restic docs, 040_backup.rst [CITED: restic.readthedocs.io/en/stable/040_backup.html]

And the interplay that makes it correct for positional roots: "Excludes do not apply to backup sources that were explicitly passed to the backup command… Content inside a directory you back up is still filtered by the given excludes." [CITED: same page] — so the item-level flag filters content WITHIN every positional root of the item's backup, exactly the union semantics D-06 documents.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Empty `backupPaths` = auto-detect, no guard (pre-Phase 1) | Source-gated refusal `code:"empty-selection"` + Phase 3 documented exit | Phase 1 → Phase 3 | INTEG-04 completes: the guard gains its UI-side counterpart (block message references reset; reset is the sanctioned exit) |
| Selection advertised exclusions but snapshots contained them (pre-01-05) | `excludedBranches` → `--exclude` on the argv (WR-01 closure) | Phase 1 plan 05 | Precedent channel for RESTIC-01's flag; `--exclude-caches` is a clean add with zero interaction (grep: flag absent from the codebase today — only an unrelated comment in `internal/api/cache.go:98`) |
| Mount rows flat checkboxes (pre-Phase 2) | Lazy tri-state tree over the flat set | Phase 2 | Phase 3's preview/list/toggle all hang off the tree's root rows and mirror |

**Deprecated/outdated:** nothing in-phase; no library churn.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Union reading of D-06: "≥1 racine active" = any TRUE entry in the stored map, regardless of whether that root is currently included in the selection | Pattern 4 / RESTIC-01 | If the intended reading is "union over currently-included roots only", the flag auto-disables when its root is deselected — different behavior in one edge (toggle on + root deselected + other roots still backed up). Planner should pin the reading in a service test; the pure-map union is simpler and matches the literal text |
| A2 | The toggle's PATCH may ride a separate serialized path OR the generalized queue (house rule read as per-editor serialization, not per-field) | Pitfall 6 | If the house rule demands a single serialized PATCH stream per editor, a separate un-serialized toggle PATCH would violate it — cheap to conform either way |
| A3 | Preview counts the stored/effective flat-set includes; run-time `onlyExistingPaths` filtering of stale paths is accepted as row-level-warned, not count-subtracted | Pitfall 5 / SELECT-03 | If the "matches exactly" criterion is read strictly against the existence filter, a phantom custom path would make the count off by one vs argv. D-01's locked text supports the flat-set reading; flag for planner confirmation |
| A4 | `--exclude-caches` placement beside `--exclude` (after verb, before `--`) is accepted by restic 0.17.x | Pattern 4 / Pitfall 4 | Docs (current stable) classify it as a backup option; the repo pins argv placement in unit tests regardless, and CI's restic 0.17.3 runs the contract tests. Residual risk small; a CI-green full suite closes it |
| A5 | Enriching `folders.emptySelectionBlocked` (existing key, text change) needs no new key; new strings budget ~5-7 keys | Pitfalls 1/2 | Under-budgeting stalls a task on parity churn; over-budgeting adds orphan-key debt. Planner sets the final list |

All restic-behavior claims are doc-cited (MEDIUM tier per classify-confidence for context7/webfetch) — the repo's own convention is to pin restic behaviors with contract tests where they matter; here the pinned unit test covers what BombVault controls (argv emission), and the flag's skipping behavior is restic's own.

## Open Questions

1. **Go threading shape for the flag (Option A vs B)**
   - What we know: both work; the repo's Limits precedent argues A; the CONTEXT's "même filière que les --exclude du 01-05" is satisfied by both (service → adapter → engine → BackupArgs).
   - What's unclear: whether the maintainer prefers the explicit `BackupDeps` dataflow over the Mode precedent.
   - Recommendation: Option A (Mode field) — documented precedent, minimal churn, `internal/backup` untouched. Planner decides; a `checkpoint` with the user is optional since either satisfies the locked decisions.

2. **Exact wire names (`excludeCaches` map vs list-of-enabled-roots)**
   - What we know: D-07 locks "JSON map racine→bool" for the COLUMN; the wire field name/shape is discretion ("forme wire exacte … tant qu'elle est additive et validée au boundary").
   - Recommendation: same map shape end-to-end (column ↔ PATCH ↔ mounts response) — one translation layer fewer, keys identical to the roots the panel renders.

3. **Preview vs run-time existence filter (A3)**
   - What we know: D-01 locks flat-set counting; the engine filters stale paths at run time; row-level warnings exist.
   - Recommendation: count the flat set; add a test comment naming the deliberate divergence for phantom roots; if the user wants strict argv equality, subtract `exists:false` custom rows and `reachable:false` mounts — a one-line filter, decided at planning.

4. **Reset confirmation UX**
   - What we know: the reset deletes remembered exclusions and returns to auto-detection (destructive-ish, but recoverable by re-selecting).
   - Recommendation: `useConfirm` dialog naming both consequences (auto-detection returns; remembered exclusions removed). Discretion.

5. **CACHEDIR toggle on custom-path roots**
   - What we know: D-06 says "par racine" — mounts are the named case; custom paths are also roots in the tree.
   - Recommendation: serve the toggle on every root row (mount + custom) — same map, same union; the tooltip already discloses item-level scope. Planner confirms.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node + npm | web build/tests | ✓ | 24.16.0 | — |
| web/node_modules | vitest/tsc/eslint | ✓ | installed (`web/node_modules/.bin/vitest` present) | — |
| Go toolchain | backend build/tests | ✓ | go1.25.0 windows/amd64 (floor 1.25.0; CI 1.26) | — |
| restic binary on PATH | restic-backed contract tests only | ✗ | — | Tests skip locally (repo-documented Windows caveat); CI installs 0.17.3. **This phase's Go tests are pure argv/unit/router tests — no local blocker** |
| golangci-lint | lint gate | ✗ (not on PATH) | — | CI Lint job gates it (Phase 2 precedent); `gofmt`/`go vet`/`go test` run locally |
| just | task runner | ✗ (not on PATH) | — | Run the underlying commands directly |
| hadolint | Dockerfile lint | ✗ | — | Not needed — no Dockerfile change this phase |

**Missing dependencies with no fallback:** none — every phase requirement is executable locally except restic-backed contract tests, which are not required by this phase's test map and run in CI.

**Missing dependencies with fallback:** golangci-lint/just (CI), restic (skip + CI).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `go test`; web: vitest ^4.1.10 + @testing-library/react ^16.3.2 (jsdom ^30.0.1) |
| Config file | Go: none (per-package); web: `web/vite.config.ts` (vitest config lives there) |
| Quick run command | `go test ./internal/restic/ -run TestBackupArgs -v` · `cd web && npx vitest run src/lib/selectionTree.test.ts` |
| Full suite command | `go test ./...` · `cd web && npm test` · `cd web && npm run build` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RESTIC-01 | `BackupArgs` emits `--exclude-caches` after the verb/before `--` when set; byte-identical absence when unset | unit | `go test ./internal/restic/ -run TestBackupArgs -v` | ✅ extend `restic_args_test.go` |
| RESTIC-01 | `SetExcludeCaches` round-trips; `UpsertTarget` never resets it | unit | `go test ./internal/store -run 'TestSetExcludeCaches' -v` | ❌ Wave 0 (template: targets_test.go:59) |
| RESTIC-01 | PATCH additive field persists + rejects out-of-mount keys; mounts response serves the map | integration (router) | `go test ./internal/api -run 'TestPatchContainer|TestContainerMounts' -v` | ✅ extend handlers tests |
| RESTIC-01 | Union → engine sees the flag (service wiring) | unit (fake engine records mode/args) | `go test ./internal/api -run 'TestBackupExcludeCaches' -v` | ❌ Wave 0 (fake extension) |
| SELECT-03 | Per-root include count derivation matches `toFlatList` membership | unit | `cd web && npx vitest run src/lib/selectionTree.test.ts` | ✅ extend (new pure helper + table) |
| SELECT-03 | Preview line renders per root; narrowing note fires on successful narrowing save only when `lastBackup` set | dom | `cd web && npx vitest run src/pages/Containers.tree.dom.test.tsx` | ✅ extend harness |
| INTEG-03 | Collapsible exclusions list under root, relative paths, muted, non-interactive | dom | `cd web && npx vitest run src/pages/Containers.tree.dom.test.tsx` | ✅ extend harness |
| INTEG-04 | Fully-deselected root renders unchecked + remembered-exclusions counter (never hidden) | dom | `cd web && npx vitest run src/pages/Containers.tree.dom.test.tsx` | ✅ extend harness |
| INTEG-04 | Reset control sends PATCH `[]` WITHOUT tree source through the queue; block message references reset | dom (+ Go guard already pinned) | `cd web && npx vitest run src/pages/Containers.tree.dom.test.tsx` | ✅ extend harness |
| i18n | New keys across 42 locales; placeholders kept; no orphans | unit | `cd web && npx vitest run src/lib/i18n.parity.test.ts src/lib/i18n.orphans.test.ts src/lib/i18n.quality.test.ts` | ✅ existing |

### Sampling Rate
- **Per task commit:** the quick commands above for the files touched + `gofmt -l .` + `go vet ./...` (Go tasks); `cd web && npm run build` when a web task touches code
- **Per wave merge:** `go build ./... && go vet ./... && go test ./...` + `cd web && npm ci && npm test && npm run build && npm run lint` + committed `web/dist`
- **Phase gate:** full Go chain + full web suite + tsc/eslint clean + `web/dist` rebuilt and committed before `/gsd-verify-work` (lint gate confirmed in CI where the local box lacks golangci-lint)

### Wave 0 Gaps
- [ ] `internal/store/targets_test.go` — `TestSetExcludeCachesRoundTripAndUpsertPreserves` (covers RESTIC-01 persistence)
- [ ] `internal/api` wiring test + minimal `fakeResticEngine` recording (mode or args) for the union (covers RESTIC-01 service seam)
- [ ] `web/src/lib/selectionTree.ts` — new pure helpers (`rootIncludeCount`, `rootExclusions`) + test tables (covers SELECT-03/INTEG-03 derivation)
- [ ] i18n keys: en + de inline, then all 40 `web/src/lib/locales/*.ts` (parity/quality/orphans gate) — same-task propagation
- [ ] No framework install needed — all infrastructure exists

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | Existing session middleware covers the reused endpoints |
| V3 Session Management | no (unchanged) | — |
| V4 Access Control | yes (inherited) | No new endpoints; the new field rides the existing authed/CSRF-protected `PATCH /api/containers/{name}` and `GET …/mounts` |
| V5 Input Validation | yes | `decodeBody`: 1 MiB `MaxBytesReader` + JSON-only + `DisallowUnknownFields` (declare the new field or bodies are rejected); map decodes as `map[string]bool` (non-bool values fail at unmarshal); keys are host paths validated with the `toContainerPath` containment discipline (service.go:3839-3842 precedent) + entry-count cap; whole save rejected atomically |
| V6 Cryptography | no | — |

### Known Threat Patterns for this phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path injection via map keys | Tampering | Keys are path-validated at the boundary (under host mount or the save is rejected); **no user-controlled string ever reaches argv** — only a boolean union emits the constant flag `--exclude-caches` (stronger than the `--exclude` channel, where patterns travel but are builder-typed) |
| Oversized map (storage abuse) | DoS | 1 MiB body cap bounds the payload; add a small entry-count cap as defense-in-depth; JSON-text column size stays trivial |
| Glob metacharacters in keys | Tampering | Moot for argv (keys never emitted); keys only ever equality-match rendered roots; strict containment validation anyway |
| XSS via rendered relative paths | Tampering | React text-node auto-escaping; render like every existing path (`dir="ltr"` mono spans); no `dangerouslySetInnerHTML` |
| Silent backup-scope change | Tampering (integrity of user intent) | The phase's whole purpose: preview must equal argv (D-01), narrowing is announced (D-02), deselect is explicit (D-04/D-05), item-wide flag effect is disclosed (D-06 tooltip) |
| CSRF on the new PATCH field | Spoofing | Existing CSRF middleware on the endpoint (unchanged); field is not a new route |

## Sources

### Primary (HIGH confidence)
- Codebase, read this session with `Read` (all quotes verbatim in Code Examples): `internal/api/selection.go` (full — SplitExclusion/NormalizeSelection/includesOnly/excludedBranches/mapRestorePaths), `internal/restic/restic.go` (Mode struct incl. Limits comment :89-101, BackupArgs :357-384, BackupStdinArgs, engine Backup :1688-1703), `internal/restic/restic_args_test.go` (TestBackupArgs* family :294-393), `internal/store/targets.go` (full — Target, UpsertTarget, 3 SELECTs, SetExcludes :489-512, SetBackupPaths, scanTarget), `internal/store/migrate.go` (numbering rules :10-67, v53 :496-501, v99 last :1302-1305, Migrate), `internal/api/handlers.go` (handlePatchContainer :1101-1192, handleContainerMounts :1311-1344, containerView lastBackup :531-534/:576-584), `internal/api/service.go` (ResticEngine :64-159, ContainerMounts :3722-3792, errEmptySelection+SetBackupPaths :3794-3874, configuredBackupPaths :3916-3922, Backup deps literal :4116/:4219-4248, resticAdapter.Backup :6918, SetExcludes :10102), `internal/backup/orchestrator.go` (Restic interface :97-117, BackupDeps.Excludes :222-224, call site :479), `internal/api/service_test.go` (fakeResticEngine.Backup :4312), `web/src/lib/selectionTree.ts` (full), `web/src/components/SelectionTree.tsx` (full), `web/src/pages/Containers.tsx` (FoldersEditor :610-1106 incl. queue :761-936, ExcludesEditor :1356+, section strip :1896-1921), `web/src/lib/api.ts` (Container :17-53, MountInfo/CustomPath/mounts response :856-903), `web/src/lib/i18n.orphans.test.ts` (full), `web/package.json` (scripts)
- `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` (Phase 3 verbatim), `.planning/research/SUMMARY.md`, `01-CONTEXT.md`, `02-CONTEXT.md`, `STATE.md` (Phase 1/2 decisions log)

### Secondary (MEDIUM confidence)
- restic official docs — 040_backup.rst, fetched this session via WebFetch: `--exclude-caches` semantics ("exclude a folder's content if it contains the special CACHEDIR.TAG file, but keep CACHEDIR.TAG"), excludes-vs-explicit-targets note, pattern rules [CITED: restic.readthedocs.io/en/stable/040_backup.html]
- Context7 `/restic/restic` + `/websites/restic_net` — `--exclude-caches`/`--exclude-if-present` introduced 0.7.2, backup-flag family (cross-check of the above)

### Tertiary (LOW confidence)
- None material; no claim in this document rests on unverified training knowledge. Design recommendations (Patterns 2/3 realizations, Option A) are analysis grounded in the quoted in-repo precedents and flagged as recommendations (A1-A5).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new packages; versions read from manifests this session
- Architecture: HIGH — every integration seam opened and quoted this session; wire contracts pinned by Phase 1/2 code; backend dataflow (store → service → adapter → engine → BackupArgs) traced end to end
- Pitfalls: HIGH for in-repo items (queue hardcoding, five SQL sites, i18n parity, guard interplay — all verified from source); MEDIUM for restic-behavior items (doc-cited; argv emission itself is unit-pinned, which is what RESTIC-01 requires)
- Union semantics reading (A1): MEDIUM — mechanics verified; the any-true-vs-included-roots-only reading is interpretation of D-06's literal text, flagged for a planner-pinned test

**Research date:** 2026-09-10
**Valid until:** 2026-10-10 (stable in-repo contracts; no upstream movement affects it — the only external dependency is restic's flag semantics, pinned by unit tests at implementation time)
