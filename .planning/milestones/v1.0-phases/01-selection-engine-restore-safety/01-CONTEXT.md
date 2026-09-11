# Phase 1: Selection Engine & Restore Safety - Context

**Gathered:** 2026-09-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Selections made through the API normalize losslessly into the unchanged flat `backupPaths` set, node listings are cheap and containment-safe, and restores survive selection changes — the risky keystone (maximal-roots ↔ restic positional targets) proven end-to-end at the API/backup level before any UI is built on it. Requirements: BROWSE-01..04, SELECT-01, SELECT-02, SELECT-04, RESTORE-01 (+ backend half of INTEG-04). No UI work in this phase.

</domain>

<decisions>
## Implementation Decisions

### Encodage des exclusions dans le flat set
- Q1: Sous-branches exclues encodées par préfixe `!` dans la MÊME liste plate (`!/mnt/user/appdata/plex/transcoding`) — zéro migration, zéro changement wire, une seule structure ; les chemins absolus commençant par `/` rendent le préfixe non ambigu
- Q2: Sémantique du préfixe dans des helpers purs d'un nouveau fichier `internal/api/selection.go` (normalisation, pruning d'ancêtres, classification inclus/exclu) appelés par `SetBackupPaths` et les readers — testable en table, pas de logique inline dans `internal/store/targets.go`
- Q3: Exclusion orpheline (sans ancêtre inclus) CONSERVÉE et signifiante : exclusions seules = « item explicitement désélectionné » — c'est l'état qui porte le garde anti-auto-détection (et le distingue d'une liste vide = auto-détection)
- Q4: Pruning d'ancêtres PAR CLASSE : une inclusion élide les inclusions strictement en dessous ; une exclusion élide les exclusions en dessous ; inclusion et exclusion coexistent volontairement (racine incluse + branche exclue)

### Contrat de listing des nœuds
- Q1: Étendre `GET /api/browse` additivement (`?hidden=1` + champs de réponse nouveaux) — un seul univers browsable, pas d'endpoint dupliqué ; FolderBrowser existant inchangé
- Q2: Champ additif `status` par listing : `ok` / `restricted` (permission-denied) / `missing`, message scrubbed ; liste vide + `status:"ok"` = vide réel — reste dans l'enveloppe HTTP 200
- Q3: PAS de hint `hasChildren` (pas de probe d'emptiness) — chaque répertoire est expandable (dérivé de `DirEntry.IsDir()`), listing d'un dossier vide rend 0 enfants ; BROWSE-01 reste satisfait (cheap, per-node)
- Q4: Cap d'entrées constant (ordre ~500, nombre exact à la discrétion du planning) + booléen `truncated:true` ; tri lexical comme aujourd'hui

### Garde anti-auto-détection au PATCH (backend INTEG-04)
- Q1: Aucun nouveau champ wire OBLIGATOIRE : l'état « item explicitement tout décoché » = exclusions orphelines persistées (≠ `[]`) ; champ optionnel `selectionSource:"tree"` dans le corps PATCH porte l'intention ; refus du `[]` nu venant d'une sélection antérieure non vide via la source arbre ; clients existants inchangés
- Q2: Refus en enveloppe maison : HTTP 200 + `{ok:false, error:<scrubbed>, code:"empty-selection"}` — le `code` machine permet à l'UI de Phase 3 de router la guidance
- Q3: Helpers génériques + wiring containers uniquement en Phase 1 ; File Sets suivront en Phase 4 via le même helper
- Q4: Sortie de l'état « tout décoché » (revenir à l'auto-détection) REPORTÉE en Phase 3 (INTEG-04 complète avec l'UI) — noté en deferred

### Durcissement restore — cas limites (RESTORE-01)
- Q1: Conteneurs uniquement (`AppdataPaths` / `prepareRestoreForTarget`) ; helpers génériques pour réutilisation File Sets en Phase 4
- Q2: Chemin du snapshot sans correspondance courante → skip + raison loguée scrubbed + skips signalés dans le résultat du restore (jamais d'abort global pour un chemin orphelin)
- Q3: Intersection vide (snapshot ne contient aucun chemin stocké) → abort propre AVANT tout teardown destructif, erreur explicite « rien à restaurer pour cet item depuis ce snapshot »
- Q4: Spot-check restic 0.17 (préservation chemins absolus ; excludes ne s'appliquant pas aux positionals) en contract tests dans la suite existante (restic ≥ 0.17 déjà requis sur PATH)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `GET /api/browse` (`internal/api/handlers.go:4143`, route `internal/api/api.go:257`) — listing existant avec gardes traversal ; base de l'extension additive
- `internal/paths` (`paths.Resolve`, `ErrTraversal`) — containment lexical ; `os.Root` (Go ≥ 1.24, floor repo 1.25) pour la sûreté symlink en défense additionnelle
- `store.SetBackupPaths` (`internal/store/targets.go`, owned setter sur `targets.selected_paths`) — point d'entrée unique de la persistance sélection ; PAS la settings row (deadlock single-connection)
- `toContainerPath` / `toHostPath` (`internal/api/service.go:1137` / `:3684`) — traduction des 3 namespaces (browse-relative / host / container-visible), déjà servis par `GET /api/containers/{name}/mounts`
- `BackupArgs` (`internal/restic/restic.go:361`) — `backupPaths` alimente déjà les positionals ; translation layer quasi nulle
- `prepareRestoreForTarget` (`internal/api/service.go` ~5242-5462) — site du durcissement restore
- FolderBrowser / SnapshotFileTree (SPA) — référence du contrat de listing actuel (cohérence hidden entries, BROWSE-04)

### Established Patterns
- Enveloppe HTTP 200 `{ok:false, error}` + `writeJSON`/`okEnvelope`/`failEnvelope` ; validation au boundary (`resourceNameRe`, 1 MiB, DisallowUnknownFields) ; scrubbing paths→`[path]` d'abord
- argv discipline : positionals influencés par l'utilisateur après `--` ; credentials par env ; pas de `--exclude` dérivé de la sélection (positionals explicites uniquement)
- Tests : `go test` stdlib, tables d'args dans `restic_args_test.go`, handler tests via routeur réel (`newTestRouter`), restic ≥ 0.17 requis sur PATH
- Commentaires « why » avec numéros d'issue ; helpers purs testables ; DI par deps structs

### Integration Points
- PATCH `/api/containers/{name}` (corps `backupPaths`) — la sélection arbre passera par là (champ optionnel `selectionSource` additif)
- `configuredBackupPaths` / effective paths (`service.go` ~3816-3888) — la frontière « vide = auto-détection » à préserver exactement
- Restic snapshot `Paths` — source du mapping longest-prefix du restore

</code_context>

<specifics>
## Specific Ideas

- Research `.planning/research/SUMMARY.md` est la spécification de référence de la phase (research flag : none) — positions maximal-roots VERROUILLÉES (exclude-based disqualifié : restic excludes ne s'appliquent pas aux positionals)
- À logger dans PROJECT.md Key Decisions pendant la phase : décision positional-targets + sémantique future-children allowlist (prescrit par le research)
- Snapshot layout : restic préserve les chemins absolus des cibles explicites — le selection set maximal-root garantit une forme de snapshot stable par sélection

</specifics>

<deferred>
## Deferred Ideas

- Sortie de l'état « tout décoché » / retour explicite à l'auto-détection → Phase 3 (INTEG-04 UI)
- Note de rétrécissement UI (« new snapshots will contain only the selected folders ») → Phase 3 (SELECT-03)
- Coverage diff backup-time (siblings ni sélectionnés ni exclus) → v2 (SELECT-06, décision utilisateur 2026-09-09)
- Fanout « exclude this subfolder instead » vers ExcludesEditor → follow-up, hors v1
</deferred>
