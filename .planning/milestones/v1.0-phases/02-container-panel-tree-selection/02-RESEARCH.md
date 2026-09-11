# Phase 2: Container Panel Tree Selection - Research

**Researched:** 2026-09-10
**Domain:** Lazy tri-state checkbox tree (SPA) over the Phase 1 selection/browse backend contracts
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**D-01 (mixed-click semantics — resolves the ROADMAP "spec the click-on-mixed" note):** Cycle mémorisé à trois états : unchecked → clic → fill (tout cocher en dessous) ; checked-all → clic → tout décocher ; clic sur un parent mixed → décocher tout en dessous en mémorisant l'état partiel ; re-cocher une checkbox qui avait un état partiel mémorisé → restaurer exactement cet état partiel (TREE-03 verbatim). Le « fill » attendu par le research s'applique uniquement à la transition unchecked→checked ; un état mixte n'est jamais écrasé silencieusement.

**D-02 (INTEG-01 integration):** Évolutif — la liste mounts/custom-paths existante DEVIENT l'arbre : chaque rangée gagne un chevron d'expansion lazy et sa checkbox devient tri-state. Pas de modal, pas de bouton « sélection fine », pas de double présentation de la même donnée sur le même écran. Les checkboxes de mounts actuelles sont les racines de l'arbre.

**D-03 (save flow):** Live-save à chaque toggle : PATCH `/api/containers/{name}` avec le flat set (`backupPaths`) et `selectionSource:"tree"` immédiatement après chaque cochage/décochage — le serveur reste l'unique source de vérité ; TREE-04 découle de la relecture serveur, pas d'un cache local de sélection.

**D-04 (pre-Phase-3 empty guard, UI side):** Blocage proactif : un toggle qui résulterait en zéro inclusion pour l'item est bloqué côté client avec un message inline traduit (en + de) routant vers la désactivation de l'item. Le PATCH refusé (code `"empty-selection"`) ne devrait jamais être atteignable depuis l'arbre. La sémantique complète de désélection totale reste en Phase 3.

**D-05 (expansion persistence):** État d'expansion des nœuds persisté en localStorage `bv-*` par conteneur, plafonné (clé bornée, pas de croissance illimitée) ; la sélection est TOUJOURS reconstruite depuis le serveur (le localStorage ne porte que le confort visuel d'expansion).

**D-06 (truncated listings):** Listing tronqué (cap + `truncated:true` du contrat Phase 1) rendu comme une ligne muted non interactive « N premières entrées affichées » en fin de liste du nœud — honnête sur le contenu, aucune pagination en v1.

### Claude's Discretion
- Style visuel exact des trois états de checkbox (tokens sémantiques `carbon-*`/`status*` et classes `glim-*` — jamais de hex brut)
- Structure interne du composant (découpage hooks/fichiers, nom du composant arbre) — conventions PascalCase/components
- Détails d'implémentation APG au-delà des exigences TREE-05 (roving tabindex exact, gestion focus après fermeture) — pattern TreeView APG et précédents keyboard (`DropdownListbox.keyboard.dom.test.tsx`)

### Deferred Ideas (OUT OF SCOPE)
- « Charger plus » / pagination dans l'arbre au-delà du cap (exigerait une pagination backend) — hors v1
- Sortie de l'état « tout décoché » / retour explicite à l'auto-détection → Phase 3 (INTEG-04 UI)
- Note de rétrécissement (SELECT-03) et liste reviewable d'exclusions (INTEG-03) → Phase 3
- Search/filter dans l'arbre (TREE-07), tailles de dossiers (SELECT-05) → v2
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TREE-01 | Collapsible lazy-loading tree per mount/root — children load on expand; no eager full-tree loads | Browse contract pinned (status/truncated/cap 500, dirs-only); fetch-on-expand + panel-lifetime promise cache design; roots come from the mounts endpoint with zero browse calls at mount |
| TREE-02 | Checkbox at every level; checking includes everything below; unchecking a child carves it out as an exclusion | Toggle→flat-set reducer over the Phase 1 `!`-encoding; per-node state derives from (includes, exclusions) list arithmetic only |
| TREE-03 | Mixed-state parents (`aria-checked="mixed"`, derived); re-activating a remembered-partial checkbox restores its prior partial state | D-01 realized naturally by the encoding: the exclusion list below a node IS the remembered partial state (dormant exclusions survive the round trip — normalization preserves orphans) |
| TREE-04 | On reopen, selection state is reconstructed from the saved flat set — fully/partially/excluded branches all distinguishable | Full (I, E) reconstruction from `GET /api/containers/{name}/mounts` (`selected` flags + `custom` + `excluded`); pure classifier, no loading needed |
| TREE-05 | Full keyboard + ARIA tree/checkbox roles (arrows, Space, `aria-expanded`, `aria-checked="mixed"`, `aria-level`/`setsize`/`posinset` on lazy nodes) | APG TreeView pattern fetched (canonical W3C page); keyboard table, roving tabindex vs aria-activedescendant, leaf-aria-expanded omission rule; house keyboard-test precedent |
| TREE-06 | Unreadable vs empty directories distinguished per node — muted "no access" row; no infinite spinners, no silent collapses | `status` trio on the browse response (`restricted`/`missing`/`error`) + error text; per-node error state with retry, distinct from `ok`+empty |
| INTEG-01 | Container panel — unfold any discovered mount or custom path, pick subfolders without dropping the rest of the mount | FoldersEditor integration map (D-02 evolution path), live-save PATCH shape with `selectionSource:"tree"` (handler accepts it today), client-side D-04 block |
</phase_requirements>

## Summary

Phase 2 is a pure-frontend phase: every backend contract it needs landed in Phase 1 and is pinned in this research with file:line evidence. The tree consumes `GET /api/browse?path=<rel>` (lazy, dirs-only, `status`/`truncated`, cap 500), gets its roots and full selection state from `GET /api/containers/{name}/mounts` (which since Phase 1 also serves the stored exclusions in `excluded[]`), and live-saves every toggle through `PATCH /api/containers/{name}` with `selectionSource:"tree"` — a body field the Go handler already parses and gates the empty-selection refusal on. No Go changes are expected; the SPA must first close a small wire-type gap (`web/src/lib/api.ts` lags the Phase 1 response shapes).

The keystone insight that makes TREE-02/03/04 clean: **per-node state is pure list arithmetic over the two entry classes and never depends on lazily-loaded children.** With Phase 1's `!`-encoding, the client holds includes `I` and exclusions `E` (both derivable from the mounts response), and classifies any node without a single network call: excluded if at/under an `E` entry; checked if at/under an `I` entry with no `E` strictly below; mixed if it has an include at/under it AND an exclusion strictly below it, or an include strictly below it (whitelist start-state). D-01's "remembered partial" is then realized by the encoding itself — unchecking a mixed/checked parent removes only the include; the exclusions below go dormant (normalization preserves orphan exclusions by design), and re-checking the parent wakes them up, restoring the exact prior partial state with zero UI-side memory. The laziness only affects which rows render, never what state they show.

The two findings the planner most needs before anything else: (1) **the i18n note in ROADMAP/CONTEXT ("both locales, en + de") is wrong for this codebase** — `en` and `de` are inline in `i18n.ts`, but the parity test requires EVERY new key in all 42 locale tables (40 files under `web/src/lib/locales/`); every recent feature commit touched all of them; (2) `web/node_modules` is currently absent, so `npm ci` is a Wave 0 step before any vitest/tsc run. The APG TreeView pattern (fetched from the canonical W3C page) prescribes the exact keyboard map, `aria-expanded`-only-on-parents rule, and the `aria-level`/`setsize`/`posinset` requirement for dynamically loaded nodes that TREE-05 already names.

**Primary recommendation:** Build `web/src/lib/selectionTree.ts` (pure: path translation, node classifier, toggle→flat-set reducer, expansion persistence) + `web/src/components/SelectionTree.tsx` (single `role="tree"` whose level-1 items are the existing mount/custom rows, roving tabindex, fetch-on-expand with an editor-lifetime promise cache), wire it into `FoldersEditor` per D-02, extend `api.ts` types first, and budget i18n keys × 42 locales.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Lazy child listing | API / Backend (existing `GET /api/browse`) | Browser (promise cache) | Server owns containment (os.Root + paths.Resolve, Phase 1); client caches per panel lifetime only |
| Tree expansion state (visual comfort) | Browser (localStorage `bv-*`, D-05) | — | Presentation, deliberately never server state (STACK: expansion is not selection) |
| Selection truth | API / Backend (`selected_paths` via PATCH) | Browser (optimistic mirror + revert) | D-03: server is the single source of truth; UI mirror equals it because every toggle round-trips |
| Node tri-state derivation | Browser (pure classifier over I/E) | — | TREE-03/TREE-04 require display state without loading; server stores only the flat set |
| Toggle→flat-set compilation | Browser (reducer produces `!`-prefixed host paths) | API (re-normalizes as invariant) | Server NormalizeSelection is the invariant backstop; client sends host-form entries |
| Empty-selection blocking (D-04) | Browser (proactive, pre-PATCH) | API (last-resort coded refusal) | Server guard only refuses a fully-empty list from source `"tree"`; exclusions-only saves pass — the client block is the only guard on that path |
| Keyboard/ARIA semantics | Browser (roving tabindex + aria on treeitems) | — | APG TreeView pattern; no server involvement |
| Restic argv impact | API / Backend (unchanged Phase 1 code) | — | Positional includes + derived `--exclude` for exclusion branches already wired (`excludedBranches`); zero changes this phase |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | ^19.2.7 | UI runtime (existing) | House stack; StrictMode; no state library [VERIFIED: web/package.json:17] |
| react-dom | ^19.2.7 | Render (existing) | — [VERIFIED: web/package.json:18] |
| TypeScript (tsgo) | ^7.0.2 | Typecheck gate `tsc --noEmit` | strict + noUnusedLocals + noUnusedParameters [VERIFIED: web/package.json:35, CLAUDE.md] |
| vitest | ^4.1.10 | Test runner (`npm test` = `vitest run`) | House runner; no vitest config file — per-file `// @vitest-environment jsdom` pragma for dom tests [VERIFIED: web/package.json:12,38 + web/vite.config.ts (no test block) + Containers.excludesAssistant.dom.test.tsx:1] |
| @testing-library/react | ^16.3.2 | Dom tests (render/fireEvent/act) | House pattern [VERIFIED: web/package.json:25] |
| jsdom | ^30.0.1 | Dom environment | — [VERIFIED: web/package.json:32] |
| Tailwind CSS | ^4.3.3 | Styling via semantic tokens only | `carbon-*`/`status*`/`glim-*`; never raw hex on controls [VERIFIED: web/package.json:33 + CLAUDE.md] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @testing-library/dom | ^10.4.1 | Queries/focus assertions | `document.activeElement` assertions in keyboard tests [VERIFIED: web/package.json:24] |
| eslint + bombvault-lint-ts | ^10.8.1 / file:lint-ts | 8 house `bombvault/*` rules as errors | Every new component; exceptions declared in eslint.config.js, never disable comments [VERIFIED: web/eslint.config.js:104-116] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled recursive rows | react-arborist / react-complex-tree / Ant Tree / MUI X TreeView | BANNED (no UI kit, CLAUDE.md); none natively model lazy + tri-state + flat-set-with-ancestor-dominance; styling fights Tailwind-4 dark mode [CITED: .planning/research/STACK.md alternatives] |
| Editor-lifetime promise cache (Map) | TanStack Query / SWR | BANNED (no state library, CLAUDE.md); needs here are one Map, panel lifetime |
| Roving tabindex on treeitems | aria-activedescendant on the tree element | APG explicitly allows both; roving tabindex is testable via `document.activeElement` in jsdom and matches the DropdownListbox keyboard precedent [CITED: W3C APG TreeView] |

**Installation:** NONE. This phase adds zero npm dependencies (constraint: SPA with no UI kit, no state library). [VERIFIED: constraint in D:\code\bombvault\CLAUDE.md + .claude\CLAUDE.md "Tech stack" section]

**Version verification:** Existing versions read directly from `web/package.json` this session (quoted above); no new packages to verify against the registry.

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none) | — | — | — | — | — | No external packages installed this phase |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 SPA (Containers page → ContainerRow → FoldersEditor)
 │
 ├─ mount/open ───────────► GET /api/containers/{name}/mounts
 │                          └─► returns: mounts[] (source/dest/selected/isAppdata/reachable),
 │                              custom[] (includes with no matching mount, host form),
 │                              excluded[] (stored "!" branches, host form),        ← Phase 1
 │                              hostMountRoot, hostSourceRoot
 │                              └─► client derives I (includes) + E (exclusions) in HOST space
 │
 ├─ expand node (chevron / Right Arrow)
 │   └─► cache hit? ── no ──► GET /api/browse?path=<rel>   (lazy, one call per node)
 │   │                          └─► {ok, dirs:[{name,path}], status:"ok", truncated}  ← Phase 1
 │   │                          └─► {ok:false, error, status:"restricted"|"missing"|"error"}
 │   └─ render children rows; truncated → muted non-interactive notice row (D-06)
 │       restricted/missing/error → muted "no access" row + retry (TREE-06)
 │
 ├─ toggle checkbox (Space / click) ──► pure reducer: (I, E, node, action) → next flat set
 │       check      = add node to I (dormant E below wakes on re-check → remembered partial, D-01)
 │       uncheck    = remove node from I (E below stays, dormant)
 │       exclude    = add "!"+node to E (parent stays in I)
 │       re-include = remove node from E
 │       └─► if next set would leave ZERO includes for the item → BLOCK inline (D-04), no PATCH
 │
 ├─ live-save (D-03) ──► PATCH /api/containers/{name}
 │       body {backupPaths: [host paths..., "!"+host paths...], selectionSource:"tree"}
 │       └─► Go: SplitExclusion → toContainerPath (rejects unreachable) → NormalizeSelection
 │              → empty-guard (source-gated) → store.SetBackupPaths → SQLite selected_paths
 │       └─► {ok:false, code:"empty-selection"} only if client guard was bypassed
 │
 └─ close/reopen panel ──► FoldersEditor state survives (mounted, renders null when closed);
     │                      tree rows re-derive visual state from (I, E); expansion restored
     └─                     from localStorage bv-* (D-05); listings from editor-lifetime cache

 Visual state per node (PURE, no loading):
   excluded  ⇔ node at/under an E entry
   checked   ⇔ node at/under an I entry AND not excluded AND no E strictly below
   mixed     ⇔ (at/under an I entry AND some E strictly below) OR (some I strictly below node)
   unchecked ⇔ otherwise
```

### Recommended Project Structure
```
web/src/
├── lib/
│   ├── selectionTree.ts            # NEW pure logic: host↔browse path translation,
│   │                               #   node classifier, toggle→flat-set reducer,
│   │                               #   expansion persistence (localStorage, capped)
│   └── selectionTree.test.ts       # NEW pure unit tests (node env, no jsdom pragma)
├── components/
│   ├── SelectionTree.tsx           # NEW tree component (reused by Phase 4 File Sets →
│   │                               #   lives in components/, NOT inside pages/)
│   ├── SelectionTree.dom.test.tsx  # NEW render/state/interaction tests (jsdom pragma)
│   └── SelectionTree.keyboard.dom.test.tsx  # NEW keyboard/ARIA tests (jsdom pragma)
├── pages/
│   └── Containers.tsx              # MODIFIED: FoldersEditor rows become tree roots (D-02)
└── lib/
    ├── api.ts                      # MODIFIED (Wave 0): BrowseResponse +status/+truncated,
    │                               #   ContainerMountsResponse +excluded, setBackupPaths
    │                               #   +selectionSource param (and optional +code on envelope)
    └── i18n.ts                     # MODIFIED: new keys in en + de inline tables
        …/locales/*.ts (40 files)   # MODIFIED: same new keys in EVERY locale (parity test)
```

### Pattern 1: Per-node state from the two flat lists (never from loaded children)
**What:** A node's checked/mixed/excluded state is computed by prefix arithmetic over `I` (includes) and `E` (exclusions) — both fully known, both small. Laziness affects only which rows render.
**When to use:** Always, for every node at every depth, including never-loaded subtrees.
**Why:** TREE-04 requires exact reconstruction on reopen without expanding anything; PITFALLS #7 (mixed-state under laziness) is voided by construction.
**Example:**
```typescript
// Sketch — final shape is the planner's; semantics are pinned by Phase 1 (selection.go).
// A node is EXCLUDED if it is at/under an exclusion entry;
// CHECKED if at/under an include with no exclusion strictly below it;
// MIXED if an include applies AND an exclusion lies strictly below (carve-out),
//    or an include lies strictly below the node (whitelist start-state).
type NodeState = "checked" | "mixed" | "excluded" | "unchecked";
function classify(nodeHostPath: string, includes: Set<string>, exclusions: Set<string>): NodeState {
  const underExcl = [...exclusions].some((e) => atOrUnder(nodeHostPath, e));
  if (underExcl) return "excluded";
  const incApplies = [...includes].some((i) => atOrUnder(nodeHostPath, i));
  const exclBelow = [...exclusions].some((e) => strictlyUnder(e, nodeHostPath));
  const incBelow = [...includes].some((i) => strictlyUnder(i, nodeHostPath));
  if (incApplies && exclBelow) return "mixed";
  if (incBelow) return "mixed";
  if (incApplies) return "checked";
  return "unchecked";
}
```
Strict segment-aligned prefix tests (`"/c/plex"` never matches `"/c/plex2/x"`) mirror `isStrictDescendant` in selection.go:45-51 — the classifier's semantics must match the Go pruning or TREE-04 drifts.

### Pattern 2: D-01 remembered-partial = dormant exclusions
**What:** Toggling a checked/mixed parent OFF removes only its include; exclusions strictly below stay stored (dormant). Toggling the parent ON again re-adds the include and the dormant exclusions immediately apply again — the exact prior partial state, with no UI-side memory.
**Why it round-trips:** Phase 1 normalization preserves orphan exclusions by design — `NormalizeSelection` doc: "An orphan exclusion (no included ancestor) is PRESERVED, not dropped" [VERIFIED: internal/api/selection.go:85-88]; `excludedBranches` only emits exclusions that are strict descendants of an include [VERIFIED: internal/api/selection.go:185-206], so dormant branches add no argv noise; the empty-selection guard fires only on a fully-empty normalized list [VERIFIED: internal/api/service.go:3868-3872].
**Consequence the planner must handle:** unchecking the last include of an item leaves an exclusions-only list — the server ACCEPTS it (it is the defined explicit-none state). The D-04 client-side block is therefore the only guard on that path; it must count includes for the whole item (all mounts + custom), not just the toggled node.

### Pattern 3: One tree, roots are the existing rows (D-02)
**What:** A single `role="tree"` per FoldersEditor; level-1 treeitems are the existing mount rows (each gaining a chevron and a tri-state checkbox) followed by custom-path rows. Mounts keep their live-save checkbox semantics — now tri-state.
**When to use:** Always (D-02 forbids modals/second presentations).
**Why one tree, not one tree per mount:** APG "root nodes are children of `tree`" — sibling roots in one tree keep arrow navigation seamless across mounts [CITED: W3C APG TreeView].

### Pattern 4: Keyboard + ARIA (TREE-05) — APG TreeView, checkbox variant
**What:** Roving tabindex (one treeitem `tabindex={0}`, all others `-1`); Up/Down move focus without expanding; Right = expand closed node WITHOUT moving focus, or move to first child if open; Left = collapse open node, or move to parent from a child; Home/End = first/last focusable; Enter = default action (toggle expansion on parents); Space = toggle the focused node's checkbox. `aria-expanded` on every expandable node, `aria-checked="true"|"mixed"|"false"` for selection, `aria-level`/`aria-setsize`/`aria-posinset` on nodes. The `tree` element needs an accessible name (`aria-label` via `t()`).
**APG details that are easy to get wrong** [CITED: https://www.w3.org/WAI/ARIA/apg/patterns/treeview/]:
- `aria-expanded` must be OMITTED on end nodes — otherwise AT misreports them as parents. Since Phase 1 decided every directory is expandable (no emptiness probe), every tree node here IS a potential parent: render `aria-expanded` on all of them (an expanded empty dir honestly reports `true` with zero children).
- Express selection with `aria-checked` on treeitems; do NOT also set `aria-selected` (APG: never mix; `aria-checked` is the recommended convention for multi-select trees).
- Dynamically loaded nodes REQUIRE `aria-level`, `aria-setsize`, `aria-posinset` — set them uniformly on every node, not only post-load.
- Focus stays on the parent when Right Arrow expands it; focus moves to the first child only on the SECOND Right press. With async children this needs an "expanded but loading" state where a second Right is a no-op (or moves once loaded).
- Type-ahead (printable chars) is optional/recommended for trees with >7 root nodes — NOT required by TREE-05; recommend deferring to keep scope tight.
- Selection is always independent of focus in multi-select trees (Space toggles only the focused node; arrow moves never toggle).
**Test precedent:** `web/src/components/DropdownListbox.keyboard.dom.test.tsx` (focus placement, arrow walking with wrap, Home/End, focus return on close) — same file-per-concern naming: `SelectionTree.keyboard.dom.test.tsx`.

### Pattern 5: Tri-state checkbox rendering (native input + indeterminate)
**What:** Native `<input type="checkbox">` per row; checked state via the `checked` prop; the mixed state via the DOM `indeterminate` property, which React does NOT expose as a prop — set it in a callback ref or effect keyed on the mixed flag.
**When to use:** Every node row. No `role="checkbox"` div needed: a native input inside a treeitem already exposes checked semantics; if the treeitem itself carries `aria-checked`, avoid duplicating the state on both the treeitem and the input (put `aria-checked` on the treeitem and keep the input as the visible control, or put the checkbox semantics on the input only — pick one consistently; APG shows both, mixing states on two elements confuses AT) [CITED: W3C APG checkbox + treeview].
**Precedent gap:** no code in `web/src` sets `indeterminate` today (grep: only ProgressBar's CSS keyframe of the same name) — this is the app's first tri-state checkbox.

### Pattern 6: Live-save with revert+shake (existing house pattern, extended)
**What:** Every toggle optimistically updates the (I, E) mirror, immediately PATCHes the full flat list, and on failure reverts and replays `.glim-shake` keyed by row — exactly `FoldersEditor.toggle` today, plus `selectionSource:"tree"`.
**Why single-flight matters here:** tree toggles can be rapid; each PATCH carries the complete list so superseded saves are harmless content-wise, but the planner should serialize in-flight PATCHes (queue/chain; PITFALLS #6: single-flight the PATCH queue, abort superseded requests) so a slow save cannot revert a newer toggle. [CITED: .planning/research/PITFALLS.md #6]

### Anti-Patterns to Avoid
- **Deriving mixed state from loaded siblings** (e.g. "parent is mixed because some rendered child is checked"): breaks TREE-04 for collapsed branches and never-loaded deep subtrees. Derive from (I, E) only.
- **Deleting exclusions when the parent include is removed:** destroys D-01's remembered-partial; the encoding requires them to stay dormant.
- **Passing `hidden=1` to browse from the tree:** BROWSE-04 pins consistency with FolderBrowser, which does not opt in [VERIFIED: web/src/components/FolderBrowser.tsx:92-111 — `browse(path)` with no hidden param].
- **Copying SnapshotFileTree's a11y model:** it has hard-coded untranslated aria-labels (`aria-label={expanded ? "collapse" : "expand"}`) [VERIFIED: web/src/components/SnapshotFileTree.tsx:123] — a lint violation by today's rules; the new tree routes every string through `t()`.
- **Status colors on the checkbox/chevron controls:** banned by `bombvault/no-status-color-on-control`; the "no access" row is muted TEXT (`text-carbon-textMuted`), status hues live on badges only.
- **A second presentation of the same selection** (modal, "fine selection" button): D-02 forbids it.
- **Storing expansion server-side or inside `backupPaths`:** presentation ≠ selection (STACK).
- **Re-fetching children on every re-expand:** cache per panel/editor lifetime; SHFS/FUSE latency makes refetch-forever a cost amplifier.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Child listing + containment | New tree/browse endpoints, nested-tree JSON | Existing `GET /api/browse` per node | One endpoint called N times; nested shapes invite eager recursion (STACK; already-landed Phase 1 contract) |
| Selection persistence | Client-side selection cache as truth | Server `selected_paths` via PATCH + derive display from (I, E) | D-3: server is the single source of truth; two truths drift |
| Server cache/ETag for listings | — | Client panel-lifetime Map cache | Listings are cheap; stale trees cost more (STACK) |
| Virtualization | Windowed rendering | Render only expanded paths; dirs-only listings keep sibling counts modest | Premature; PATHOLOGICAL dirs can be handled later in this one component |
| Emptiness probes | `hasChildren` server field | Every dir expandable; empty renders nothing | N+1 FUSE ops per expansion (BROWSE-01 decision, Phase 1) |
| Pagination beyond cap | "Load more" | D-06 muted notice row | Deferred explicitly (backend pagination out of v1) |

**Key insight:** the hard part of this phase is selection semantics, and Phase 1 already owns them server-side (`selection.go`). The client only ever needs list arithmetic, one reducer, and honest rendering — which is why zero new dependencies are needed.

## Common Pitfalls

### Pitfall 1: The i18n budget is 42 locales, not 2
**What goes wrong:** ROADMAP/CONTEXT note says "Both i18n locales" / "les deux locales (en, de)". The parity test gates ALL locale tables against the en key set: every new `t()` key must be added to `en` and `de` inline in `i18n.ts` AND to all 40 files in `web/src/lib/locales/` or `npm test` fails.
**Why it happens:** en/de are the inline "source of truth" tables; the other 40 are lazy-loaded chunks but still parity-enforced.
**How to avoid:** Budget one task (or task step) for locale propagation. Precedent: commit fb6400ce added 2 keys and touched `i18n.ts` + all 40 locale files.
**Warning signs:** `i18n.parity.test.ts` failure "locale X diverges from en: 1 missing".
[VERIFIED: web/src/lib/i18n.parity.test.ts:40-53 — "has exactly the en key set"; web/src/lib/localesForTests.ts:20-21 — `/** code -> table, for all 42. */`]

### Pitfall 2: api.ts wire types lag the Phase 1 contract (Wave 0)
**What goes wrong:** `BrowseResponse` has no `status`/`truncated`, `ContainerMountsResponse` has no `excluded`, `setBackupPaths` sends no `selectionSource` — the tree cannot read TREE-06's status or D-03's source flag with the types as-is.
**Why it happens:** Phase 1 was backend-only; the SPA was never extended.
**How to avoid:** First code task extends `web/src/lib/api.ts`. Verbatim current shapes [VERIFIED: web/src/lib/api.ts:475-484]:
```typescript
export interface BrowseResponse {
  ok: boolean;
  root?: string;
  path?: string;
  dirs?: BrowseDirEntry[];
  error?: string;
}
```
and [VERIFIED: web/src/lib/api.ts:858-876]:
```typescript
export interface ContainerMountsResponse extends OkEnvelope {
  mounts?: MountInfo[];
  custom?: CustomPath[];
  hostMountRoot?: string;
  hostSourceRoot?: string;
}
// ...
export function setBackupPaths(name: string, backupPaths: string[]): Promise<OkEnvelope> {
  return fetchJSON(`/api/containers/${encodeURIComponent(name)}`, {
    method: "PATCH",
    body: JSON.stringify({ backupPaths }),
  });
}
```
Required additions (backend already serves/parses all of them): `status?: "ok" | "restricted" | "missing" | "error"` and `truncated?: boolean` on BrowseResponse; `excluded?: string[]` on ContainerMountsResponse; a `selectionSource` option on setBackupPaths; optionally `code?: string` on the envelope type for defensive `empty-selection` handling.
**Warning signs:** tree code casting responses to `any`, or silently never reading `status`.

### Pitfall 3: Losing tree state on panel close (TREE-04)
**What goes wrong:** `FoldersEditor` renders `null` when its section closes (`if (!open) return null;` [VERIFIED: web/src/pages/Containers.tsx:818]) — the tree component unmounts and loses loaded listings and expansion state; only FoldersEditor-level state survives.
**Why it happens:** One-section-at-a-time Selector strip (`toggleSection` resets the Set) [VERIFIED: Containers.tsx:1714-1717].
**How to avoid:** Hold (I, E), the listings cache, and translation roots in FoldersEditor-level state (survives close/reopen, dies on page navigation — exactly the "panel lifetime" STACK prescribes); restore expansion from the D-05 localStorage key on tree mount.
**Warning signs:** every reopen re-fetches root listings or shows all rows collapsed.

### Pitfall 4: Dormant exclusions reach the server as "explicit none"
**What goes wrong:** Unchecking the item's LAST include leaves an exclusions-only list; the server's empty-selection guard does NOT refuse it (it fires only on a fully-empty normalized list) — the item silently enters the explicit-none state D-04 says the client must block.
**Why it happens:** The guard is `selectionSource == "tree" && len(normalized) == 0` [VERIFIED: internal/api/service.go:3868]; orphan exclusions are deliberately preserved (explicit-none carrier).
**How to avoid:** The D-04 client block counts includes across the whole item (mounts + custom) BEFORE building the PATCH; zero includes → inline translated block, no request. The `code:"empty-selection"` envelope stays an unreachable backstop.
**Warning signs:** a test where the last mount include is unchecked and a PATCH goes out.

### Pitfall 5: Rapid toggles vs in-flight saves (revert clobbers newer state)
**What goes wrong:** Two quick toggles: save A in flight, toggle B applies optimistically, save A fails and the revert handler restores pre-A state — wiping B.
**Why it happens:** The existing revert pattern (`toggle`'s failure path) assumes one toggle per save.
**How to avoid:** Serialize saves (chain each PATCH after the previous resolves; only the final full-list state is sent), and make revert re-derive from the latest mirror rather than a captured snapshot.
**Warning signs:** dom test firing two toggles back-to-back with the first reply `{ok:false}`.

### Pitfall 6: Infinite spinner / silent collapse on listing failure (TREE-06)
**What goes wrong:** An unreadable directory (PUID/PGID reality) yields `{ok:false, status:"restricted"}` — if the tree treats `ok:false` as "empty" the branch silently disappears; if the fetch promise never settles in the UI, the spinner never clears.
**Why it happens:** Empty is `ok:true` + zero dirs; error is a different shape — the distinction is exactly the Phase 1 status trio.
**How to avoid:** Three per-node render states: loading (spinner), error (muted "no access" row + retry affordance, selection untouched — STACK: a vanished/unreadable folder never blocks saving), empty (nothing under the row). The browse error text is server-scrubbed and shown verbatim per house rule.
**Warning signs:** any code path where `dirs ?? []` swallows an `ok:false`.

### Pitfall 7: aria-expanded on leaves / missing setsize on lazy nodes
**What goes wrong:** Screen readers announce expandable leaves as parents, or lose position context in lazily loaded branches.
**How to avoid:** APG rules above (Pattern 4). Note the truncated notice row (D-06) is NOT a treeitem: render it outside the treeitem count (plain muted text row) so `setsize`/`posinset` count only real nodes.
**Warning signs:** axe/role queries finding `treeitem` on the notice row.

### Pitfall 8: Forgetting web/dist and the build chain
**What goes wrong:** `go build` embeds a stale SPA; CI/push gates fail.
**How to avoid:** Every web change ends with `cd web && npm ci && npm run build` (tsc --noEmit && vite build) and a commit of `web/dist` (CLAUDE.md rule). Note `web/node_modules` is CURRENTLY ABSENT on this machine — `npm ci` is a Wave 0 step.
**Warning signs:** `git status` clean but `web/dist` older than `web/src`.

## Code Examples

### The browse contract as the tree consumes it (Go, pinned)
```go
// Source: internal/api/handlers.go:4328-4335 (success) — fields verbatim
writeJSON(w, http.StatusOK, map[string]any{
	"ok":        true,
	"root":      h.cfg.HostMountRoot,
	"path":      subpath,
	"dirs":      dirs,
	"status":    "ok",
	"truncated": truncated,
})
```
```go
// Source: internal/api/handlers.go:4191-4200 — status kinds verbatim
func classifyReadDirError(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "restricted"
	case errors.Is(err, fs.ErrNotExist):
		return "missing"
	default:
		return "error"
	}
}
```
```go
// Source: internal/api/handlers.go:2984 — verbatim
const maxBrowseEntries = 500
```

### The PATCH body and the empty-selection guard (Go, pinned)
```go
// Source: internal/api/handlers.go:1112-1123 — body fields verbatim
	var body struct {
		IncludeInSchedule *bool     `json:"includeInSchedule"`
		PreHook           *string   `json:"preHook"`
		PostHook          *string   `json:"postHook"`
		BackupPaths       *[]string `json:"backupPaths"`
		// ... SelectionSource *string `json:"selectionSource"` (declared so
		// DisallowUnknownFields does not reject tree saves)
```
```go
// Source: internal/api/handlers.go:1152-1154 — coded refusal verbatim
			if errors.Is(err, errEmptySelection) {
				writeJSON(w, http.StatusOK, codedFailEnvelope(err, "empty-selection"))
				return
			}
```
```go
// Source: internal/api/service.go:3868-3872 — guard conditions verbatim
	if selectionSource == "tree" && len(normalized) == 0 {
		if prior, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(prior.SelectedPaths) > 0 {
			return errEmptySelection
		}
	}
```
Input paths are HOST paths; the "!" prefix is split BEFORE translation [VERIFIED: service.go:3817-3850 — doc: "An entry prefixed with \"!\" (the tree selector's excluded branch...) is translated and contained on its BARE path, then stored prefixed"]. So the tree PATCHes `"/mnt/user/appdata/plex"` and `"!/mnt/user/appdata/plex/transcoding"` (host form).

### The mounts response gives the tree everything except the raw list (Go, pinned)
```go
// Source: internal/api/handlers.go:1337-1343 — response fields verbatim
	writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
		"mounts":         mounts,
		"custom":         custom,
		"excluded":       excluded,
		"hostMountRoot":  h.cfg.HostMountRoot,
		"hostSourceRoot": h.cfg.HostSourceRoot,
	}))
```
```go
// Source: internal/api/service.go:3699-3705 — MountInfo verbatim
type MountInfo struct {
	Source    string `json:"source"`    // host path (shown to the user)
	Dest      string `json:"dest"`      // in-container mount point
	Selected  bool   `json:"selected"`  // currently included in the backup
	IsAppdata bool   `json:"isAppdata"` // auto-detected appdata default
	Reachable bool   `json:"reachable"` // reachable under the host mount (backable)
}
```
Client-side reconstruction: `I = {m.source | m.selected} ∪ {c.path | c ∈ custom}` (host form) and `E = excluded` (host form). Caveat: `custom` holds every include that is not exactly a mount root — including includes that lie UNDER a reachable mount (sub-includes surface there today) [VERIFIED: service.go:3755-3782 — `matched[cp] = true` only on exact mount-root match]. See Open Questions Q1.

### Path translation is prefix arithmetic with the two served roots
```typescript
// Sketch — existing precedent at Containers.tsx:802 (verbatim):
//   const p = raw.startsWith("/") ? raw : `${hostSourceRoot}/${raw}`;
// browse-relative ↔ host: swap the hostSourceRoot prefix.
// Go twin for reference (service.go:1137-1148): toContainerPath swaps
// hostSourceRoot → hostMountRoot ("e.g. /mnt → /host/user").
function hostToBrowseRel(host: string, hostSourceRoot: string): string {
  const p = host.startsWith(hostSourceRoot + "/") ? host.slice(hostSourceRoot.length + 1) : host;
  return p;
}
function browseRelToHost(rel: string, hostSourceRoot: string): string {
  return `${hostSourceRoot}/${rel}`;
}
```
The tree works in ONE canonical space (recommend host paths — the space mounts/custom/excluded already speak) and translates only at the browse-call boundary. Pure functions, table-tested.

### The dom-test harness pattern to follow
```typescript
// Source pattern: web/src/pages/Containers.excludesAssistant.dom.test.tsx:1,49-63 (verbatim shape)
// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getContainerMounts: () => Promise.resolve({ /* mounts, custom, excluded, roots */ }),
    browse: (path: string) => Promise.resolve(browseReplies.shift() ?? { ok: true, dirs: [], status: "ok", truncated: false }),
    setBackupPaths: (_n: string, paths: string[], opts?: { selectionSource?: string }) => { patchBodies.push({ paths, opts }); return Promise.resolve({ ok: true }); },
  };
});
const { FoldersEditor } = await import("./Containers");  // imported AFTER vi.mock
```
Keyboard tests follow `DropdownListbox.keyboard.dom.test.tsx` (jsdom needs `Element.prototype.scrollIntoView` stubbed when positioning code runs — see its beforeEach).

### APG TreeView keyboard map (reference table for the implementation)
[CITED: https://www.w3.org/WAI/ARIA/apg/patterns/treeview/]
| Key | Action |
|-----|--------|
| Right Arrow | closed → expand, focus stays; open → focus first child; end node → nothing |
| Left Arrow | open → collapse; closed child → focus parent; closed root → nothing |
| Down / Up Arrow | focus next/previous focusable node, never expands/collapses |
| Home / End | first / last focusable node |
| Enter | default action (toggle expansion on parents) |
| Space | toggle the focused node's checkbox |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| STACK.md draft model: uncheck-child materializes parent into immediate children ("materialize-on-uncheck") | Phase 1 `!`-exclusion encoding: parent stays an include, child stored as `"!"`-prefixed exclusion | Landed 2026-09-10 (Phase 1) | No child-list rewrite ever needed; remembered-partial comes free; tree never needs the listing to compute a toggle |
| Mount checkbox boolean selection | Tri-state tree rows over the same flat set | This phase | Selection semantics unchanged at the persistence boundary |
| Plain collapsible rows "add ARIA if audit demands" (STACK early idea) | Full APG treeview mandated by TREE-05 | Roadmap creation | role=tree/treeitem/group + roving tabindex + lazy-node geometry are requirements, not polish |

**Deprecated/outdated:**
- Treating `custom[]` as the only place sub-mount includes appear — with the tree, sub-includes under a reachable mount belong in the tree state (see Open Questions Q1).
- SnapshotFileTree's aria approach (untranslated hard-coded aria-labels) — do not replicate.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | No Go changes are needed this phase (all backend contracts sufficient) | Summary, Architecture | Low — verified against handlers/service/selection this session; only conceivable addition would be serving the raw flat list in one array, which the mounts response makes unnecessary |
| A2 | Dormant exclusions (orphaned by parent uncheck) are an acceptable stored state for the UI to produce | Pattern 2 | Low/Medium — server semantics verified (preserved, not emitted on argv, guard passes); but whether product WANTS the stored list to accumulate dormant entries is a D-01 reading; planner should confirm the cycle behavior matches D-01 exactly |
| A3 | Sub-includes under a reachable mount should render as tree state under the mount row (absorbed from the custom list) | Open Questions Q1 | Medium — if wrong, dual presentation appears (D-02 violation) or whitelist selections render oddly |
| A4 | Single `role="tree"` containing mount+custom roots (vs per-mount trees) satisfies TREE-05 | Pattern 3 | Low — APG-conformant either way; affects aria-level numbering only |
| A5 | Focus stays workable without aria-activedescendant (roving tabindex chosen) | Pattern 4 | Low — APG allows both; jsdom testing favors roving tabindex |
| A6 | Expansion localStorage key format `bv-*` per container with a hard entry cap satisfies D-05 | Pattern/Structure | Low — exact key name/cap number is planner discretion |

## Open Questions (RESOLVED)

*All four resolved at plan time (2026-09-10) — dispositions inline below; verified by gsd-plan-checker (Warning 1, dimension 11).*

1. **Where do sub-mount includes render — tree state or the custom list?**
   - What we know: includes that are not exactly a mount root land in `custom[]` today (service.go:3776-3782), including paths UNDER a reachable mount.
   - What's unclear: D-02 forbids double presentation; should the FoldersEditor filter custom entries that lie under a reachable mount and fold them into that mount's tree state (recommended — that IS TREE-04's "partial mount"), and if so does the custom row list change for pre-existing deployments?
   - Recommendation: absorb them into the tree; filter them from the custom row list by prefix test against reachable mount sources.
   - **Resolution (plans):** adopted — absorbed into the tree, custom rows filtered by prefix test (02-01 tracer + 02-03 Task 2 sub-include absorption; surfaced as a flagged assumption in 02-03).

2. **Does reopen re-fetch from the server?**
   - What we know: D-03 says the server is the source of truth and TREE-4 "découle de la relecture serveur"; but the existing editors keep a component-persistent mirror loaded once (`loaded` guard, Containers.tsx:721-744), and every tree toggle round-trips, so the mirror equals the server within a session.
   - What's unclear: whether the planner should force a mounts re-fetch on editor reopen (stricter reading) or keep component persistence (consistent with all sibling editors).
   - Recommendation: keep component persistence (state survives close because FoldersEditor stays mounted); a page reload refetches anyway. Cross-session drift matches the behavior of every other editor today.
   - **Resolution (plans):** adopted — component persistence kept, editor-level state (02-03 flagged assumption; mirror at FoldersEditor level per Pitfall 3).

3. **Enter key on leaf nodes — any action?**
   - What we know: APG defines Enter as the node's default action; for a checkbox tree on a leaf, Space already toggles.
   - Recommendation: Enter toggles expansion on parents, does nothing extra on leaves (Space is the only toggler). Planner's discretion per CONTEXT.
   - **Resolution (plans):** adopted verbatim — Enter expands/collapses parents, no extra leaf action (02-03 Task 1 keyboard map).

4. **How aggressively to serialize PATCHes (Pitfall 5)?**
   - What we know: house live-save awaits per toggle with revert; rapid tree toggles make that insufficient.
   - Recommendation: a one-deep save queue — while a PATCH is in flight, the next toggle updates the mirror and marks dirty; on resolve, send the latest full list once. Keeps ordering, collapses bursts, keeps revert logic simple.
   - **Resolution (plans):** adopted verbatim — serialized one-deep PATCH queue (02-02 Task 2).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | web build/tests | ✓ | v24.16.0 | — |
| npm | package install | ✓ | 11.13.0 | — |
| `web/node_modules` | vitest/tsc/eslint runs | ✗ MISSING | — | Wave 0: `cd web && npm ci` |
| Go toolchain | regression check only | ✓ | go1.25.0 (windows/amd64) | — |
| restic ≥ 0.17 on PATH | full `go test ./...` | ✗ not on Windows PATH | — | Reduced local suite (POSIX tests skip on Windows — documented repo caveat); CI Test job is the arbiter (also the open Broken Window #2) |
| `web/dist/index.html` placeholder | `go build` pre-Vite | ✓ present | — | — |

**Missing dependencies with no fallback:** none blocking (npm ci resolves the node_modules gap; no Go changes expected so the local restic gap does not bite).
**Missing dependencies with fallback:** restic (CI runs the full suite); none otherwise.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | vitest ^4.1.10 + @testing-library/react ^16.3.2 + jsdom ^30.0.1 |
| Config file | none — `web/vite.config.ts` has no test block; dom tests opt in per file via `// @vitest-environment jsdom` first line |
| Quick run command | `cd web && npx vitest run src/lib/selectionTree.test.ts` (pure) / `src/components/SelectionTree.dom.test.tsx` (dom) |
| Full suite command | `cd web && npm test` (vitest run) + `cd web && npm run build` (tsc --noEmit && vite build) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TREE-01 | children fetch on expand only; never at mount; cache prevents refetch on re-expand | unit (dom) | `cd web && npx vitest run src/components/SelectionTree.dom.test.tsx` | ❌ Wave 0 |
| TREE-02 | check includes subtree; uncheck child under checked parent adds `!` entry, parent stays include | unit | `cd web && npx vitest run src/lib/selectionTree.test.ts` | ❌ Wave 0 |
| TREE-03 | mixed derivation from (I, E); uncheck mixed parent keeps exclusions; re-check restores exact partial | unit + dom | both commands above | ❌ Wave 0 |
| TREE-04 | remount with same (I, E) reproduces identical checked/mixed/excluded rendering and aria state | dom | `npx vitest run src/components/SelectionTree.dom.test.tsx` | ❌ Wave 0 |
| TREE-05 | arrows/Space/Enter/Home/End; aria-expanded/checked=mixed/level/setsize/posinset; roving tabindex | dom (keyboard) | `cd web && npx vitest run src/components/SelectionTree.keyboard.dom.test.tsx` | ❌ Wave 0 |
| TREE-06 | status restricted/missing/error → muted no-access row + retry, distinct from ok+empty; spinner always clears | dom | `npx vitest run src/components/SelectionTree.dom.test.tsx` | ❌ Wave 0 |
| INTEG-01 | mount/custom roots expandable; PATCH body carries `!`-prefixed host paths + `selectionSource:"tree"`; D-04 block fires before PATCH on zero-includes | dom (integration in FoldersEditor harness) | `cd web && npx vitest run src/pages/Containers.tree.dom.test.tsx` (name per planner) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd web && npm test` (fast; whole vitest suite) — plus `npm run build` when the task touches code
- **Per wave merge:** `cd web && npm ci && npm run build` + `go build ./... && go vet ./...` (regression) + committed `web/dist`
- **Phase gate:** full vitest suite green + tsc clean + eslint clean (`cd web && npm run lint`) + `web/dist` rebuilt and committed before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `cd web && npm ci` — node_modules currently absent (blocking every other check)
- [ ] `web/src/lib/api.ts` — extend `BrowseResponse` (+`status`, +`truncated`), `ContainerMountsResponse` (+`excluded`), `setBackupPaths` (+`selectionSource`), envelope (+`code?`) — covers REQ TREE-06/D-03 plumbing
- [ ] `web/src/lib/selectionTree.ts` + `selectionTree.test.ts` — pure classifier/reducer/translation/persistence
- [ ] `web/src/components/SelectionTree.tsx` + `.dom.test.tsx` + `.keyboard.dom.test.tsx`
- [ ] FoldersEditor integration test file (mocked api module harness)
- [ ] i18n keys: en + de inline, then all 40 `web/src/lib/locales/*.ts` files (parity/quality/orphans tests gate them)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | Existing session middleware covers the reused endpoints |
| V3 Session Management | no (unchanged) | — |
| V4 Access Control | yes (inherited) | No new endpoints; tree reads/writes ride the existing authed/CSRF-protected `/api/containers/*` + `/api/browse` routes |
| V5 Input Validation | yes (inherited, defense-in-depth) | Server re-validates every PATCH entry: `SplitExclusion` → `toContainerPath` strict prefix check rejects unreachable paths; whole save rejected atomically [VERIFIED: internal/api/service.go:3817-3850]. Browse containment is os.Root + paths.Resolve (Phase 1, BROWSE-03). Client sends only paths derived from server listings |
| V6 Cryptography | no | — |

### Known Threat Patterns for SPA tree over browse/PATCH

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal via crafted node paths | Tampering / Info disclosure | Tree never constructs paths from user text — only from listing responses; server re-validates (toContainerPath) and contains listings (os.Root); already landed Phase 1 |
| XSS via directory names | Tampering | React text-node rendering auto-escapes; never `dangerouslySetInnerHTML`; names render like every other path today (`dir="ltr"` mono spans) |
| Attack-surface expansion (arbitrary browsing) | Info disclosure | Out of scope by REQUIREMENTS: the tree offers only discovered mounts/custom paths; `hidden=1` NOT used (BROWSE-04 consistency) |
| Oversized PATCH bodies | DoS | 1 MiB `MaxBytesReader` + `DisallowUnknownFields` at the boundary (unchanged); tree lists are bounded by cap-500 listings |

## Sources

### Primary (HIGH confidence)
- Codebase (read this session with Read): `internal/api/selection.go` (full), `internal/api/handlers.go` (handleBrowse, handlePatchContainer, handleContainerMounts, classifyReadDirError, maxBrowseEntries), `internal/api/service.go` (toContainerPath/toHostPath, ContainerMounts, SetBackupPaths, errEmptySelection, MountInfo/CustomPath), `web/src/lib/api.ts` (browse types, mounts types, setBackupPaths, fetchJSON, OkEnvelope), `web/src/pages/Containers.tsx` (FoldersEditor complete, section strip, UpdateAfterBackupRow), `web/src/components/FolderBrowser.tsx` (full), `web/src/components/SnapshotFileTree.tsx` (full), `web/src/components/DropdownListbox.keyboard.dom.test.tsx`, `web/src/pages/Containers.excludesAssistant.dom.test.tsx`, `web/src/lib/i18n.ts` (+ LANGUAGES, loadLocale), `web/src/lib/localesForTests.ts`, `web/src/lib/i18n.parity.test.ts`, `web/src/lib/displayPrefs.ts`, `web/eslint.config.js`, `web/package.json`, `web/vite.config.ts`
- `.planning/research/STACK.md`, `SUMMARY.md` — milestone research (code-grounded, HIGH per its own assessment)
- Git history: commit fb6400ce (locale propagation evidence — all 40 locale files touched for 2 keys)

### Secondary (MEDIUM confidence)
- W3C WAI-ARIA APG TreeView pattern — https://www.w3.org/WAI/ARIA/apg/patterns/treeview/ (fetched this session via WebFetch + Context7 `/w3c/wai-aria-practices`)
- W3C WAI-ARIA APG Checkbox (mixed-state) — https://www.w3.org/WAI/ARIA/apg/patterns/checkbox/ (Context7)

### Tertiary (LOW confidence)
- None material; no claims in this document rest on unverified training knowledge. Design recommendations (Patterns 1-6 realization) are analysis, flagged as recommendations rather than verified facts.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new packages; every existing version read from package.json
- Architecture: HIGH — every integration seam opened and quoted this session; the wire contracts are pinned by Phase 1 code
- Pitfalls: HIGH — i18n parity, wire-type lag, unmount lifecycle, and guard interplay all verified from source/tests; APG pitfalls cited from the canonical W3C page
- D-01 realization (dormant exclusions): MEDIUM — mechanics verified in code; the interpretation that this matches D-01's intent is a recommendation (A2)

**Research date:** 2026-09-10
**Valid until:** 2026-10-10 (stable, in-repo contracts; re-verify only if Phase 3 backend work lands first)
