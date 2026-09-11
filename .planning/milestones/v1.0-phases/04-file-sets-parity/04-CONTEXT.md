# Phase 4: File Sets Parity - Context

**Gathered:** 2026-09-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Choosing what a File Set covers uses the same collapsible tree as the container panel — same cascade/mixed-state/persistence semantics, lazy expand, exact reconstruction on reopen — with zero second implementation of selection (INTEG-02). A set's tree selection round-trips through the same flat normalization, and the set's next backup produces a snapshot whose `Paths` match the ticked roots. Single-root picker vs multi-root was decided here (multi-root, D-01/D-02). Create/Delete/scheduling of file sets are NOT touched.

</domain>

<decisions>
## Implementation Decisions

### Modèle de sélection — multi-root
- **D-01:** Multi-root (option recommandée par la note ROADMAP et le research) : l'utilisateur peut décocher la racine du set et cocher plusieurs sous-dossiers → plusieurs positionals restic. Strict superset du single-root (un seul enfant coché = cas particulier). `FilesRestic.Backup` prend déjà `paths []string` — l'orchestrateur `internal/backup/files_orchestrator.go` est inchangé.
- **D-02:** Périmètre de l'arbre : la racine visible unique est le `Path` résolu du set (`paths.Resolve(HostMountRoot, set.Path)`). La multi-root se joue entièrement SOUS cette racine. Pas de siblings hors `Path`, pas de browsing arbitraire (Out of Scope REQUIREMENTS) — `FolderBrowser` reste le boundary picker à la création ; le create est inchangé, l'arbre apparaît dès qu'un `Path` existe (les sets Discover path-less et disabled n'affichent pas d'arbre tant que le path n'est pas défini).

### Persistance
- **D-03:** Nouvelle migration append-only ajoutant une colonne nullable `selected_paths` TEXT sur `file_sets` (jamais éditer une migration livrée — NUMBERING HAZARD `internal/store/migrate.go`). Encodage = le MÊME flat set que `backupPaths` (entrées bare + `!`-préfixées, normalisation `internal/api/selection.go`, miroir client `selectionTree.ts`). NULL ou absent ⇒ comportement legacy byte-identique (positional unique `SourceDir`). Le champ `Path` existant n'est jamais réécrit par la sélection (zero impact sur les sets existants — philosophie SELECT-02 appliquée aux filesets).
- **D-04:** Surface wire : champ additif au corps PATCH `/api/files/sets/{id}` (pattern corps-à-pointeurs de `handlePatchFileSet`, decodeBody DisallowUnknownFields 1 MiB) — `selectedPaths` avec le nom/la struct exacts à la discrétion du planning tant qu'additif et validé au boundary : chaque entrée passe la discipline de confinement sous le mount root (pattern `toContainerPath` Phase 3), cap d'entrées (64, precedent excludeCaches), rejet atomique du save entier. Type miroir doc-commenté dans `web/src/lib/api.ts`, servi par la vue (extension de `FileSetView`).

### Compilation backup
- **D-05:** Le flat set se compile en includes maximaux sous `Path` = positionals restic ; les branches d'exclusion stockées sont enforcees en `--exclude` au site unique (discipline gap-closure 01-05). `selected_paths` NULL/absent ⇒ argv `[SourceDir]` byte-identique legacy. Critère de succès 2 : snapshot `Paths` = les racines cochées.
- **D-06:** Sémantique sélection vide : un set entièrement décoché est REFUSÉ client + serveur (même posture que la garde Phase 1/ton fail D-04 Phase 3). Le files domain n'a PAS de fallback auto-détection (le `Path` est explicite) — donc pas d'équivalent « Reset » : le message de refus oriente vers la suppression du set (DELETE existant), seule issue saine. Jamais un set vide silencieux (cardinal sin).

### Portée de la parité (surfaces de confiance)
- **D-07:** Portées avec l'arbre : le preview « N chemins » par racine (comptage client dérivé de la MÊME classification `selectionTree.ts` — doit matcher les positionals exacts, precedent D-01 Phase 3) et la liste reviewable des exclusions sous la racine (INTEG-03 pattern, vue audit non interactive — la mutation passe uniquement par l'arbre). Le toggle CACHEDIR.TAG est HORS SCOPE pour les file sets (RESTIC-01 est container-scoped) — deferred (voir <deferred>).

### Restauration / compat
- **D-08:** Les anciens snapshots mono-path d'un set devenu multi-root restent restaurables : réutiliser le mapping longest-prefix RESTORE-01 entre la path list du set et les snapshot `Paths` (intersection vide ⇒ abort AVANT teardown, message scrubbed). UNE implémentation du mapping, pas une seconde sémantique files-domain.

### Claude's Discretion
- Libellés i18n et budget de nouvelles clés (parité 42 locales + tests orphans, pattern Phase 2/3) ; em-dashes interdits dans le texte utilisateur
- Placement/rendu dans `Files.tsx` (PAGE_SHELL, tokens `carbon-*`/`status*`, classes `glim-*` ; couleurs de statut sur badges jamais sur contrôles)
- Structure interne : réutiliser `SelectionTree` tel quel avec une source de racines adaptée vs wrapper léger — au planning, à condition de n'ajouter AUCUNE seconde implémentation de la sémantique (toggles/classification/state dérivent des mêmes helpers)
- Forme wire exacte du champ `selectedPaths` (nom, struct Go/TS) tant qu'additive + validée au boundary
- Stratégie de tests : co-localisés `.test.ts`/`.dom.test.tsx`, `restic_args_test.go` si l'argv est touché, white-box `*_internal_test.go` pour helpers non exportés

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spécification milestone & exigences
- `.planning/REQUIREMENTS.md` § Domain Integration (INTEG-02) — le requirement de la phase ; § Selection ↔ Persistence (SELECT-01/SELECT-02) — l'encodage flat et la philosophie zero-impact ; § Out of Scope (browsing arbitraire, nested JSON, ExcludesEditor) — les garde-fous
- `.planning/ROADMAP.md` § Phase 4 — but, 2 critères de succès, note « single-root picker vs multi-root — research recommends multi-root; append-only SQLite migration (`selected_paths` nullable) »
- `.planning/research/SUMMARY.md` — « File Sets: multi-root needs only an append-only migration — FilesRestic.Backup already takes a []string » ; build order item 6 ; open question 2
- `.planning/PROJECT.md` — Active requirement + Key Decisions (compilation maximal-roots, futur-children allowlist)

### Décisions des phases précédentes (réutiliser, ne pas réinventer)
- `.planning/phases/01-selection-engine-restore-safety/01-CONTEXT.md` — encodage bare+`!`, garde empty-selection, enforcement `--exclude` au site unique (01-05), mapping longest-prefix RESTORE-01
- `.planning/phases/02-container-panel-tree-selection/02-CONTEXT.md` — D-01..D-06 acquis (arbre, queue one-deep, classification dérivée des sets)
- `.planning/phases/03-selection-trust-controls/03-CONTEXT.md` — D-01 preview « N chemins » matchant l'argv, D-03 liste exclusions non interactive, D-07 champ PATCH additif + validation boundary (pattern à répliquer pour `selectedPaths`)

### Code cible
- `internal/store/filesets.go` + `internal/store/migrate.go` — modèle FileSet (Path unique aujourd'hui), migration append-only à ajouter
- `internal/api/handlers.go` (`handlePatchFileSet`) — corps à pointeurs à étendre additivement
- `internal/api/service.go` — `BackupFileSet` (~8706, résolution SourceDir + refuse no-path), `validateFileSet` (~8871), `prepareRestoreFileSet` (~9021) pour D-08
- `internal/backup/files_orchestrator.go` — `FilesRestic.Backup(paths []string)` : inchangé (D-01)
- `web/src/pages/Files.tsx` — page cible (form d'édition, FolderBrowser, excludes)
- `web/src/components/SelectionTree.tsx` + `web/src/lib/selectionTree.ts` — LE composant et LA classification à réutiliser
- `web/src/lib/api.ts` — `FileSetView` (~2264), `createFileSet`/PATCH à étendre
- `internal/api/selection.go` — normalisation partagée (source de vérité)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `SelectionTree.tsx` + `selectionTree.ts` (Phase 2) — lazy expand, cascade, mixed-state, reconstruction, keyboard APG : le composant EST le requirement ; la classification (includes/exclusions par sets host-paths) fournit preview count et liste d'exclusions sans nouvelle source de vérité
- `internal/api/selection.go` — `NormalizeSelection`/`SplitExclusion` partagés : le flat set fileset passe par les MÊMES helpers (aucune seconde implémentation)
- `files_orchestrator.go` — signature multi-positional déjà en place : la multi-root ne touche que la résolution des paths côté service
- Garde « no source path configured » existante dans `BackupFileSet` — le refus sélection-vide (D-06) s'y ajoute naturellement

### Established Patterns
- Migrations SQLite append-only ; champ PATCH additif à pointeurs + validation boundary + cap (precedent excludeCaches Phase 3)
- i18n : toute chaîne via `t()`, parité 42 locales, `web/dist` rebuildé et committé après tout changement `web/`
- argv : positionals après `--`, exclusions au site unique 01-05 ; snapshot `Paths` = positionals (vérité restore)
- Avant push : `go build/vet/gofmt/golangci-lint/test` + `cd web && npm ci && npm run build`

### Integration Points
- PATCH `/api/files/sets/{id}` — champ `selectedPaths` additif (D-04)
- `GET /api/browse` (endpoint Phase 1 durci) — source du listing lazy de l'arbre sous le Path du set
- `BackupFileSet`/scheduler/everything — le compile step lit `selected_paths` au moment du backup (fresh read, precedent UpsertTarget re-read Phase 3)
- Restore fileset (original/chosen folder + files restore) — mapping longest-prefix D-08

</code_context>

<specifics>
## Specific Ideas

- Critère de succès 2 est testable directement : sauvegarder un set multi-root et lire les snapshot `Paths` = racines cochées (le round-trip complet, pas seulement l'UI)
- L'argv legacy (colonne NULL) doit rester byte-identique — pin de test sur l'absence de tout changement pour un set jamais édité via l'arbre
- Le refus sélection-vide cite la suppression du set comme seule issue (pas de « reset ») — le files domain n'a pas d'auto-détection vers laquelle revenir
- 03-CONTEXT deferred disait « même preview/liste portés à la parité » : les deux sont des dérivations gratuites de `selectionTree.ts` — ils viennent ; le toggle CACHEDIR non (voir deferred)

</specifics>

<deferred>
## Deferred Ideas

- Toggle CACHEDIR.TAG (`--exclude-caches`) pour les file sets — RESTIC-01 est container-scoped ; hors critères de succès phase 4. Candidat v2/milestone suivant si demandé
- TREE-07 search/filter, TREE-08 restore-side tree, SELECT-05 size hints, SELECT-06 coverage diff — déjà v2 (REQUIREMENTS.md)
- Fanout « exclude this subfolder instead » vers l'ExcludesEditor — hors v1 (Out of Scope PROJECT.md)

</deferred>

---

*Phase: 04-file-sets-parity*
*Context gathered: 2026-09-10*
