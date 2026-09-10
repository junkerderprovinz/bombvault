# Phase 3: Selection Trust & Controls - Context

**Gathered:** 2026-09-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can see and control exactly what a selection will back up: per-mount effective-selection preview (with a narrowing note when a selection shrinks an item that already has snapshots), reviewable exclusions, defined UI semantics for deselecting everything (including the explicit exit back to auto-detection), and a per-root CACHEDIR.TAG toggle compiled to restic `--exclude-caches`. Requirements: SELECT-03, INTEG-03, INTEG-04, RESTIC-01. First phase of the milestone with a backend surface again (RESTIC-01: persistence + argv), on top of the Phase 1 engine contracts and the Phase 2 tree.

</domain>

<decisions>
## Implementation Decisions

### SELECT-03 — Aperçu de sélection effective
- **D-01:** L'aperçu « N chemins » est une ligne par racine de l'arbre, dérivée côté client du flat set live (le miroir de sauvegarde) : nombre d'includes maximaux relevant de cette racine. Zéro nouvel endpoint — le flat set EST la vérité positionnelle (les includes maximaux du flat set sont exactement les positionnels que la prochaine sauvegarde transmettra à restic, contrat Phase 1). Le comptage client doit reproduire la classification serveur (`SplitExclusion`/pruning — miroir dans `selectionTree.ts`), et le nombre affiché doit matcher ce que l'argv contiendra, pas un comptage de nœuds cochés à l'écran.
- **D-02:** Déclencheur de la note de rétrécissement : à chaque sauvegarde de sélection réussie où le nombre d'includes de l'item DIMINUE par rapport à la sélection précédente, ET l'item a ≥1 snapshot antérieur (signal « a déjà des snapshots » déjà servi au panel — le planner vérifie le champ exact : lastBackup/runs). La note dit que les snapshots futurs ne contiendront que les dossiers sélectionnés (c'est la communication de la sémantique future-children acceptée en Phase 1, prescrite par la note ROADMAP sous SELECT-03). Pas de comparaison au contenu des snapshots existants — c'est le coverage diff différé en v2 (SELECT-06).

### INTEG-03 — Exclusions reviewables
- **D-03:** Sous chaque racine qui porte des exclusions, une section repliable « N exclusions » liste les sous-branches désélectionnées (chemins relatifs à la racine, rendu muted cohérent avec le preview). Vue d'audit NON interactive : la mutation passe uniquement par l'arbre (un seul pipeline de toggle, thème D-02/T-02-10 de la Phase 2) — pas de re-check depuis la liste en v1.

### INTEG-04 — Sémantique UI du tout-décoché
- **D-04:** Une racine entièrement décochée (plus aucun include relevant, exclusions devenues orphelines-dormantes) rend comme une rangée racine en état « non sélectionné » avec compteur d'exclusions mémorisées — jamais masquée : les exclusions orphelines sont signifiantes (01-CONTEXT Q3, état anti-auto-détection), l'UI doit les montrer comme mémoire, pas les cacher.
- **D-05:** Sortie explicite vers l'auto-détection (clôture du deferred 01-CONTEXT Q4) : un contrôle « Réinitialiser la sélection » en pied de section envoie PATCH `backupPaths: []` avec `selectionSource` ≠ `"tree"` (franchit la garde Phase 1, qui ne refuse le `[]` nu que depuis la source arbre) → retour documenté à l'auto-détection, suppression des exclusions orphelines. Le message du blocage D-04 (zéro-include) est enrichi pour référencer ce contrôle. Le reset passe par la même queue sérialisée one-deep que les toggles.

### RESTIC-01 — Toggle CACHEDIR.TAG par racine
- **D-06:** Le toggle vit par racine (fidèle au requirement per-mount) mais se compile en flag item-level : dès qu'≥1 racine de l'item l'active, l'argv porte `--exclude-caches` (union). L'UI documente la portée réelle (tooltip : s'applique à toute la sauvegarde de l'item) — jamais de silence sur un effet qui déborde de la racine où il est actionné. Flag global restic placé avant les positionals (discipline argv), testé dans `restic_args_test.go`.
- **D-07:** Persistance : colonne append-only sur `targets` (JSON map racine→bool, owned setter sur le modèle `SetExcludes` — `internal/store/targets.go:489`), champ additif au corps PATCH `/api/containers/{name}` (aux côtés de `backupPaths`/`selectionSource`, `handlers.go:1112`). Jamais la settings row (per-container n'y vit pas ; MutateSettings deadlock pattern).

### Claude's Discretion
- Libellés i18n exacts et budget de nouvelles clés (à définir au planning ; parité 42 locales + tests orphans obligatoires, pattern Phase 2)
- Placement/rendu précis du toggle sous la racine et du contrôle « Réinitialiser » (tokens `carbon-*`/`status*` + classes `glim-*` ; couleurs de statut sur badges, jamais sur contrôles)
- Structure interne des composants preview/liste exclusions (suivre les précédents `ExcludesEditor`/`FolderBrowser`)
- Forme wire exacte du champ PATCH (nom, struct Go/TS) tant qu'elle est additive et validée au boundary

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spécification milestone & exigences
- `.planning/REQUIREMENTS.md` § Selection ↔ Persistence (SELECT-03), § Domain Integration (INTEG-03, INTEG-04 — backend landed Phase 1, UI semantics here), § Restic Engine (RESTIC-01) — verrouillés ; § Out of Scope (fanout ExcludesEditor, coverage diff)
- `.planning/ROADMAP.md` § Phase 3 — but, 4 critères de succès, notes (narrowing note = communication future-children ; fanout « exclude this subfolder instead » = follow-up hors v1)
- `.planning/research/SUMMARY.md` — spécification de référence du milestone (positions UI attendues du preview)
- `.planning/phases/01-selection-engine-restore-safety/01-CONTEXT.md` — décisions d'origine : encodage `!`, garde empty-selection (Q1-Q3), sémantique des exclusions orphelines, sortie reportée Q4 (clôturée ici par D-05)
- `.planning/phases/02-container-panel-tree-selection/02-CONTEXT.md` — D-01..D-06 acquis (live-save D-03, queue one-deep, garde D-04, expansion localStorage, cap 500/truncated)

### Backend RESTIC-01
- `internal/restic/restic.go:361` (`BackupArgs`) + `internal/restic/restic_args_test.go` — site argv du flag `--exclude-caches` et tables de test à étendre
- `internal/store/targets.go` — pattern colonne owned setter (`Excludes` JSON, `SetExcludes:489`) ; `internal/store/migrate.go` — étiquette migrations append-only (NUMBERING HAZARD)
- `internal/api/handlers.go:1112` — corps PATCH containers (`backupPaths`/`selectionSource`) à étendre additivement (decodeBody : DisallowUnknownFields, 1 MiB)
- `internal/api/service.go` — `SetBackupPaths`/`configuredBackupPaths` (~3816, frontière vide = auto-détection à préserver) ; chemin `BackupDeps.Excludes` du gap-closure 01-05 (le flag suivra la même filière)

### Frontend
- `web/src/pages/Containers.tsx` — panneau cible : arbre Phase 2, miroir live + queue (774, 846-873), garde D-04 (901-912), revert (860-864)
- `web/src/lib/selectionTree.ts` — classification locale (includes/exclusions) : source du comptage par racine D-01 et de la liste D-03
- `web/src/lib/api.ts` — types miroir à étendre (champ PATCH toggle, réponse mounts si le per-racine doit être servi)
- `web/src/components/ExcludesEditor.tsx` — voisin du panneau : styling cohérent INTEG-03, écriture additif du PATCH

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/api/selection.go` (Phase 1) — helpers purs de classification : la référence du comptage D-01 ; le client en est le miroir (`selectionTree.ts`)
- `selectionTree.ts` (Phase 2) — sets (includes, exclusions) déjà maintenus par l'arbre : compteur preview, liste exclusions et détection racine-tout-décochée s'y dérivent sans nouvelle source de vérité
- PATCH queue one-deep + miroir live (Containers.tsx) — reset D-05 et toggle CACHEDIR passent par la même sérialisation (jamais deux PATCH concurrents)
- `Badge`/`IncludeToggle`/`Button` (`glim-*`) — contrôles partagés

### Established Patterns
- i18n : toute chaîne utilisateur via `t()` (lint), parité 42 locales (tests i18n.parity/quality/orphans), `web/dist` rebuildé et committé après tout changement `web/`
- argv : flags globaux avant la sous-commande ; positionnels influencés après `--` ; `--exclude-caches` est un flag global, pas un pattern
- Migrations SQLite append-only (jamais éditer une migration livrée) ; settings row interdite pour l'état per-container

### Integration Points
- PATCH `/api/containers/{name}` — champ additif du toggle (D-07) ; garde empty-selection Phase 1 inchangée (le reset D-05 passe avec une source ≠ "tree")
- `GET /api/containers/{name}/mounts` — racines découvertes/custom : clés de la map per-racine D-07
- Chaîne `BackupArgs` : service → adapter `resticAdapter` → `restic.Backup` — même filière que les `--exclude` du 01-05
- Signal « l'item a des snapshots » : champ déjà servi au panel (dernier run/backup) — déclencheur D-02 ; le planner identifie le champ exact dans `api.ts`/la réponse liste conteneurs

</code_context>

<specifics>
## Specific Ideas

- Le nombre « N chemins » doit matcher EXACTEMENT les positionnels du prochain argv (critère de succès 1) — testable : le compteur dérive du même flat set que `toFlatList`
- La note de rétrécissement EST la communication de la sémantique future-children acceptée en Phase 1 (note ROADMAP Phase 3) — pas une nouvelle sémantique
- Le fanout « exclude this subfolder instead » routant vers l'ExcludesEditor existant est un follow-up, hors v1 (note ROADMAP) — ne pas l'ajouter
- `--exclude-caches` n'existe nulle part dans le code aujourd'hui (grep vide) — ajout net, aucune interaction avec les `--exclude` du 01-05

</specifics>

<deferred>
## Deferred Ideas

- Fanout « exclude this subfolder instead » vers l'ExcludesEditor — follow-up hors v1 (ROADMAP note)
- Coverage diff backup-time (siblings ni sélectionnés ni exclus) → v2 (SELECT-06, décision utilisateur 2026-09-09)
- File Sets : même preview/liste/toggle portés à la parité arbre — Phase 4 (INTEG-02)
- TREE-07 search/filter, SELECT-05 size hints → v2 (REQUIREMENTS.md)

</deferred>

---
*Phase: 3-Selection Trust & Controls*
*Context gathered: 2026-09-10*
