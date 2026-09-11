---
phase: 01-selection-engine-restore-safety
verified: 2026-09-10T10:41:20Z
status: passed
score: 10/10 must-haves verified
behavior_unverified: 0 # les 2 vérités behavior-unverified de la vérification initiale sont désormais prouvées par exécution de première main (conteneur Linux + restic 0.17.3 réel) — voir la section Preuves d'exécution
overrides_applied: 0 # l'override initial (demi-contenu SC2 / WR-01) est FERMÉ par le gap closure 01-05 : le code satisfait désormais le must-have amendé, l'entrée est conservée ci-dessous à titre documentaire
overrides:

  - must_have: "The resulting snapshot's Paths contain the selected folders and nothing more (SC2 content half — mixed include+exclude selections)"
    reason: "FERMÉ le 2026-09-10 par le plan 01-05 (commits 8e3b86f3..11a1cf7d) : les branches d'exclusion stockées sont désormais encodées comme motifs restic --exclude sur l'argv de backup. Entrée d'origine (acceptation 2026-09-09) : WR-01 — une sélection mixte stockait et affichait la branche exclue, mais le moteur la sauvegardait quand même (positionals seuls) ; décision utilisateur explicite de router la fermeture vers un plan de gap closure « Encode ». Le must-have n'est plus dérogation : VERIFIED."
    accepted_by: "user (decision recorded in .planning/phases/01-selection-engine-restore-safety/01-REVIEW-FIX.md)"
    accepted_at: "2026-09-09T22:20:00Z"
re_verification:
  previous_status: human_needed
  previous_score: 8/10
  gaps_closed:

    - "SC2 demi-contenu (WR-01) : exclusions stockées → restic --exclude sur l'argv de backup (helper excludedBranches, câblé au seul site BackupDeps.Excludes, service.go:4243) — l'override devient inutile"
    - "SC4 moitié comportementale : TestBrowseSymlinkEscapeRejected exécuté et PASS sous Linux (conteneur golang:1.26-bookworm, HEAD 3fe492c5) ; TestBrowseStatusRestricted exécuté et PASS en utilisateur non-root"
    - "Contrats positionnels restic 0.17 : les 3 tests TestPositional* (dont le nouveau TestPositionalExcludeAbsoluteSubdirPattern du gap closure) exécutés et PASS contre restic 0.17.3 réel sous Linux au HEAD final"
    - "Les 4 dérives documentaires (PROJECT.md rationale, REQUIREMENTS.md INTEG-04 + BROWSE-01, ROADMAP SC2) réalignées par 01-05 Task 3 (commit a426ece5) — gates grep vérifiées"
    - "Warning de revue WR-01 (mapped restore selectors non re-validés) corrigé au commit 04363500 : boucle paths.Within sur la liste mappée dans prepareRestoreForTarget (service.go:5530-5535)"
  gaps_remaining: []
  regressions: []
unverified_prohibitions: 11 # verrous judgment-tier : verdicts LLM-judge non autoritaires per ADR-550 D4 — revue humaine recommandée (tableau dans le corps du rapport)
human_verification:

  - test: "Smoke sur l'instance Unraid réelle (<unraid-host>) : sauvegarder un conteneur avec un sous-dossier exclu (ex. appdata/plex avec !…/transcoding), vérifier le contenu du snapshot (restic ls : la branche exclue absente, le reste présent), puis restaurer proprement ce conteneur depuis ce snapshot"
    expected: "Le snapshot contient le montage moins la branche exclue (Paths = racine maximale, Excludes = le motif dérivé) ; le restore complète sans abort post-teardown ; l'état restauré correspond au snapshot, pas à la sélection courante"
    why_human: "Intégration réelle (chemins hôte Unraid/FUSE, docker.sock, restic réel sur l'array) qu'aucun harnais de test (memstore, fakes, t.TempDir) ne couvre ; les tests prouvent les contrats moteur, pas le comportement sur l'instance physique"

  - test: "Revue manuelle des 2 items edge-coverage délibérément non résolus (edge-coverage.json) : BROWSE-02 'unclassified' et SELECT-02 'unclassified'"
    expected: "Un humain accepte l'interprétation planifiée (BROWSE-02 : sémantique du trio de statuts per CONTEXT browse Q2 ; SELECT-02 : sémantique zéro-migration per texte de l'exigence + encodage Q1/Q3, couvert par les round-trips legacy/[] et migrate.go intact) ou dépose une correction"
    why_human: "Politique #1110 — hypothèses marquées 'never auto-resolved' ; la revue manuelle est la disposition mandate"

  - test: "Confirmer l'intention MVP-mode : le but de phase n'est pas au format User Story (user-story.validate → false), donc pas de table User Flow Coverage"
    expected: "Soit accepter la base de cette vérification (les 5 critères numérotés du ROADMAP, tous moteur et testables), soit exécuter /gsd mvp-phase 1 pour poser un but User Story et re-vérifier en forme MVP"
    why_human: "Décision de format appartenant à l'utilisateur"

  - test: "Pousser docker-folders sur origin et exiger les jobs Test + Lint verts sur le HEAD final (3fe492c5)"
    expected: "Actions GitHub verte (le job Test installe restic 0.17.3 en runner non-root — dernière combinaison jamais exécutée : TestBrowseStatusRestricted sur un vrai runner CI)"
    why_human: "La SUBSTANCE des deux jobs est déjà répliquée et vérifiée de première main au HEAD final (suite intégrale verte + golangci-lint 0 issue dans les conteneurs CI-équivalents — voir Preuves d'exécution) ; il ne reste que l'exécution d'enregistrement sur l'infrastructure GitHub, impossible depuis cette boîte"
---

# Phase 1 : Selection Engine & Restore Safety — Rapport de re-vérification

**But de la phase :** Les sélections faites via l'API se normalisent sans perte dans l'ensemble plat `backupPaths` inchangé, les listages de nœuds sont bon marché et confinés, et les restores survivent aux changements de sélection — la pierre angulaire risquée (maximal-roots ↔ cibles positionnelles restic) prouvée de bout en bout avant qu'une UI ne s'y appuie.
**Vérifié :** 2026-09-10T10:41:20Z
**Statut :** human_needed
**Re-vérification :** Oui — après gap closure 01-05 (WR-01) + fix de revue 04363500
**HEAD vérifié :** 3fe492c5 (delta code vs la réplication CI verte 11a1cf7d = uniquement les 15 lignes du fix de revue dans service.go, couvertes localement)
**Note de mode :** le ROADMAP marque la Phase 1 `mode: mvp`, mais le but n'est pas au format User Story — vérification goal-backward contre les 5 critères numérotés (voir item humain 3).

## Contexte de ce cycle

1. Exécution initiale 01-01..01-04 → première vérification (2026-09-09) : `human_needed`, 8/10, avec un écart fonctionnel « WR-01-vérif » (les exclusions stockées n'étaient pas traduites en `restic --exclude` au backup — scoré comme override approuvé). NB : cet objet est DISTINCT du finding « WR-01 » de la revue de code.
2. Gap closure 01-05 exécuté (commits 8e3b86f3..11a1cf7d) : helper `excludedBranches` (internal/api/selection.go), câblage au champ `BackupDeps.Excludes` dans `service.Backup` (service.go:4243), positionals inchangés = racines maximales, excludes strictement descendants, restore sans excludes.
3. Revue de code du delta de phase (01-REVIEW.md, commit 48168e90) : 0 Critical / 1 Warning / 4 Info. Le Warning (mapped-restore-list non re-validée) est corrigé au commit 04363500. Les 4 Info restent documentés, volontairement hors scope.
4. Réplication CI Docker locale (Linux, restic 0.17.3 réel) : suite intégrale verte rapportée au HEAD 11a1cf7d ; ce vérificateur a rejoué de première main les pièces critiques AU HEAD FINAL 3fe492c5 (voir Preuves d'exécution), y compris la suite intégrale complète.

## Goal Achievement

### Vérités observables

Fusion : 5 critères de succès du ROADMAP (le contrat) + must-haves des 5 PLANs. Les vérités du plan 01-05 sont intégrées dans SC2 ; les must-haves des plans 01-01..01-04 avaient été vérifiés à l'initiale et passent en contrôle de régression ici.

| # | Vérité | Statut | Preuve |
|---|-------|--------|--------|
| 1 | **SC1** — Sauver une sélection via l'API stocke la forme racines-maximales (parents gardés, descendants redondants éliminés, sous-branches décochées en exclusions) ; la relecture reproduit exactement inclus/partiel/exclus ; état-initial whitelist sans cas particulier | ✓ VERIFIED | `selection.go` NormalizeSelection/PruneMaximal purs par classe, exclusions orphelines conservées ; exécution locale 2026-09-10 : `TestNormalizeSelection`+`TestSplitExclusion`+`TestPruneMaximal`+`TestSetBackupPaths*` (batch régression `ok internal/api 1.888s`) ; visibilité read-back via `excluded` de mounts (`TestMountsExcluded` PASS) |
| 2 | **SC2 (moitié positionnelle)** — Un backup après sélection resserrée remet à restic exactement les chemins positionnels racines-maximales | ✓ VERIFIED | `TestBackupNarrowedSelectionUsesMaximalIncludes` PASS (exécuté localement au HEAD final) : `eng.lastPaths` == `[root/user/appdata/plex]` uniquement ; chaîne `effectiveBackupPaths` → `configuredBackupPaths` → `includesOnly` inchangée (grep : `excludedBranches` ne touche jamais les positionals) |
| 3 | **SC2 (moitié contenu, amendée 2026-09-09)** — Le snapshot contient les dossiers sélectionnés et rien de plus : les branches d'exclusion stockées sont des motifs `restic --exclude` sur l'argv de backup | ✓ VERIFIED (override FERMÉ) | Helper `excludedBranches` (selection.go:185-206, pur, 2 non-émissions délibérées : égalité + orpheline) câblé à l'UNIQUE site `BackupDeps.Excludes` (service.go:4243) : `append(resolveExcludePatterns(...), excludedBranches(tg.SelectedPaths)...)` — patterns utilisateur d'abord. Tests exécutés PASS localement : assertion `eng.lastExcludes == [root/…/transcoding]` + `AppdataPaths` sans préfixe `!` ; merge user-first (`TestBackupSelectionExcludesMergeAfterUserPatterns` : `["*.tmp", root/…/transcoding]`) ; table blanche `TestExcludedBranches` 7 cas. Preuve moteur réelle : `TestPositionalExcludeAbsoluteSubdirPattern` **PASS contre restic 0.17.3 réel sous Linux au HEAD final** (voir Preuves d'exécution) — le motif absolu-sous-répertoire exact que la dérivation émet filtre le sous-arbre SANS dropper le positional |
| 4 | **SC3** — Les `backupPaths` sauvegardés des déploiements existants continuent de fonctionner sans migration ; liste vide == exactement auto-détection à la frontière de persistance | ✓ VERIFIED | `git diff 121ee887..HEAD -- web/ internal/store/migrate.go` VIDE sur toute la phase ; round-trips legacy/`[]` PASS (batch régression) ; garde `errEmptySelection` intact (service.go:3794-3870) ; exclusions-seules = explicit-none accepté (`TestBackupPathsExclusionsOnlyIsNotRefused` PASS, test non modifié depuis 9792d927 — 0 ligne de diff le touche) |
| 5 | **SC4** — Le listage d'un nœud est un appel par-nœud bon marché, distingue vide d'illisible (messages scrubbed), n'échappe jamais à sa racine même via symlinks, et s'accorde avec le navigateur existé sur les entrées cachées | ✓ VERIFIED (comportement désormais prouvé) | **Exécution de première main au HEAD final, conteneur Linux** : `TestBrowseSymlinkEscapeRejected` **PASS** (os.Root rejette un symlink pointant hors racine) ; `TestBrowseStatusRestricted` **PASS en utilisateur non-root** (uid 1000) ; famille complète `TestBrowse*` PASS (cap, hidden, traversal, racine, sous-chemin). Classifieur unitaire PASS sur tout OS (local). La barrière behavior-unverified de la vérification initiale est levée |
| 6 | **SC5** — Restaurer un snapshot plus ancien après remodelage de la sélection complète au lieu d'avorter en plein restore après le teardown destructif ; mapping par les `Paths` du snapshot choisi (longest-prefix), pas par la sélection courante | ✓ VERIFIED | `mapRestorePaths` deux passes (selection.go:239-280) ; tests exécutés PASS localement AU HEAD FINAL (donc avec le fix de revue) : `TestMapRestorePaths` 8 sous-cas, `TestRestoreSelectionChange`, `TestRestoreSelectionChangePerPathSkip`, `TestRestoreSelectionChangeSkipOrder`, `TestRestoreSelectionChangeExplicitSnapshotID`, `TestRestoreEmptyIntersection` (aucun stop/remove Docker) ; `chosenSnapshot` (service.go:6552) ; `SkippedPaths: plan.skippedPaths` câblé (service.go:5663) |
| 7 | **INTEG-04 backend** — Désélection-totale source arbre refusée avec `code:"empty-selection"`, état antérieur préservé ; exclusions relisibles via `excluded` de mounts, jamais nul | ✓ VERIFIED | Régression PASS (batch) : `TestEmptySelectionGuard` 5 cas, `TestMountsExcluded` ; garde strictement source-gated AVANT toute écriture store (service.go:3868-3872) ; `SelectionSource *string` optionnel |
| 8 | **Key Decisions PROJECT.md** avec issues (positional-targets amendée, future-children, tree-as-view, lazy-load) | ✓ VERIFIED | PROJECT.md:71-75 ; ligne 74 amendée par 01-05 : rationale corrigé (restic excludes FILTrent le contenu dans les positionals — contract-proven), décision encode + tradeoff métadonnées datés 2026-09-09, outcome « amended by 01-05 » ; gates : `disqualified`/`excludes do not apply` = 0 occurrences |
| 9 | **Plomberie sécurité restore** — skips par-chemin (scrubbed, bornés, jamais d'abort global) ; intersection vide → abort AVANT teardown ; `RestoreDeps.SkippedPaths` byte-identique à zéro-valeur | ✓ VERIFIED | Tests PASS localement (voir #6) + `TestRestoreDepsSkippedPathsEmptyIsByteIdentical` PASS (`ok internal/backup 0.296s`) |
| 10 | **Contrats positionnels restic 0.17** — excludes ne droppent jamais une source positionnelle ; `Paths` absolus préservés verbatim ; motif absolu-sous-répertoire filtre le sous-arbre dans le positional | ✓ VERIFIED (comportement désormais prouvé) | **Exécution de première main au HEAD final** : `TestPositionalExcludesKeepSourceDir`, `TestPositionalAbsolutePathPreserved`, `TestPositionalExcludeAbsoluteSubdirPattern` — tous **PASS contre restic 0.17.3 réel (SHA256 vérifié) sous Linux** ; argv niveau : `TestBackupArgs*` 6 PASS localement |

**Score :** 10/10 vérités vérifiées (0 présente-comportement-non-vérifiée ; 0 override appliqué — l'override initial est fermé par le code)

### État du gap closure 01-05 (vérification complète des 7 must-haves du plan)

| Must-have 01-05 | Statut | Preuve |
|---|---|---|
| Sélection mixte → positionals racines-maximales ET branche exclue en `--exclude` | ✓ VERIFIED | Code service.go:4243 + tests PASS (voir vérité 3) |
| Moitié positionnelle du contrat 01-01 inchangée ; les exclusions ne deviennent jamais positionals | ✓ VERIFIED | Test révisé asserte `lastPaths` == includes seuls ; `includesOnly` inchangé (commentaire amendé uniquement) |
| Seuls les stricts descendants émis ; orphelines et égalité jamais | ✓ VERIFIED | `TestExcludedBranches` 7 cas PASS (les 2 non-émissions épinglées comme contrat) |
| Restore non affecté : `AppdataPaths` includes-only, aucun chemin restore ne consomme les excludes dérivés | ✓ VERIFIED | Énumération `excludedBranches` sur internal/ = exactement selection.go, service.go (unique site 4243), + 2 fichiers de test ; adaptateurs restore (`RestorePaths`/`RestoreSubtreeTo`/`RestoreSubtreeInclude`, service.go:6926-6936) ne prennent AUCUN exclude ; assertion `AppdataPaths` sans `ExclusionPrefix` |
| Exclusions-seules toujours definition-only, zéro appel restic | ✓ VERIFIED | Test PASS non modifié (0 diff depuis 9792d927) ; zéro includes → dérivation vide (cas épinglé) |
| Zéro migration, encodage wire inchangé, browse et web/ intacts | ✓ VERIFIED | `git diff 9792d927..HEAD -- internal/backup internal/store/migrate.go web` VIDE ; à l'échelle phase : web/ + migrate.go vides |
| Docs de planification alignées sur le comportement livré | ✓ VERIFIED | PROJECT.md:74 amendé ; REQUIREMENTS.md INTEG-04 `[ ]` + split backend/UI (lignes 38, 106) + BROWSE-01 texte du contrat livré (ligne 21, D-07 nommé) ; ROADMAP SC2 amendée (ligne 32) ; gates grep toutes à 0 |

### État post-fix de revue (commit 04363500)

- **WR-01 (revue) — re-validation des sélecteurs mappés** : ✓ CONFIRMÉ DANS LE CODE. Boucle `paths.Within(s.cfg.HostMountRoot, q)` sur la liste mappée dans `prepareRestoreForTarget` (service.go:5530-5535), placée entre le skip-logging et `appdataForRestore = mapped` — exactement où la phase destructive reçoit sa liste ; refus « a mapped restore path is outside the host mount » ; commentaire why citant la revue 01 (sémantique de bord : `Within` nettoie les deux côtés, la racine de montage elle-même échoue au strict-within = fail-closed voulu). Tous les tests restore PASS au HEAD final (le fix est couvert en chemin happy par `TestRestoreSelection*`).
  - ℹ️ Info : la branche de REFUS du nouveau garde (sélecteur mappé hors racine → refus) n'a pas de test négatif dédié — les fixtures existantes mappent toutes strictement à l'intérieur. Garde défense-en-profondeur simple, miroir du garde stored-list dix lignes au-dessus (lui non plus sans test négatif dédié). Suggestion : un futur test `TestRestoreMappedOutsideRootRefused`. N'affecte aucune vérité de phase.
- **IN-01..IN-04** : restent documentés dans 01-REVIEW.md, volontairement hors scope (Info). IN-04 (motifs no-op dans les métadonnées Excludes quand l'include a disparu du disque) est le seul qui touche le gap closure — cosmétique, métadonnées uniquement, tradeoff déjà accepté.

### Preuves d'exécution (première main, ce vérificateur, 2026-09-10)

| Preuve | Commande | Résultat |
|---|---|---|
| Suite intégrale au HEAD final, Linux + restic 0.17.3 réel | conteneur `golang:1.26-bookworm`, restic 0.17.3 SHA256-vérifié, `go test ./...` | **TOUS LES PAQUETS ok** (api 111s, restic 23s, backup 4s, store 5s, … — aucun FAIL) |
| Contrats positionnels réels (3 tests, dont le nouveau subdir du gap closure) | idem, `-run 'TestPositional' -v` | **3/3 PASS** |
| Containment symlink (SC4) | idem, `-run 'TestBrowse' -v` | **TestBrowseSymlinkEscapeRejected PASS** (+ 9 autres PASS) |
| Statut restricted (SC4/BROWSE-02) | idem, `-u 1000:1000`, `-run 'TestBrowseStatusRestricted' -v` | **PASS en non-root** |
| golangci-lint (job Lint) | conteneur `golangci/golangci-lint:latest`, `run ./...` | **0 issues** |
| Tests Go purs ciblés Windows (gap closure + restore + régression) | `go test ./internal/api/ -run …` local | 12/12 PASS + batch régression ok |
| Chaîne Go | `go build ./...`, `gofmt -l .`, `go vet ./internal/api/ ./internal/restic/` | OK / vide / propre |

Note : la réplication CI sous Docker tourne en root — `TestBrowseStatusRestricted` y skip par conception (commit 26e6fbf7) ; ce vérificateur l'a couvert séparément en non-root. Le job Test GitHub tourne en runner non-root : c'est la seule combinaison restée non exécutée sur l'infrastructure d'enregistrement (item humain 4).

### Artefacts requis

Tous présents, substantiels (aucun stub), câblés.

| Artefact | Attendu | Statut | Détails |
|---|---|---|---|
| `internal/api/selection.go` | Primitives d'encodage pures + `excludedBranches` + `mapRestorePaths` | ✓ VERIFIED | Lu en intégralité ; doc comments amendés (tradeoff métadonnées, décision 2026-09-09) |
| `internal/api/service.go` | SetBackupPaths normalisé, includesOnly readers, garde empty-selection, BackupDeps.Excludes câblé, mapping+abort prepare, re-validation mappée (fix revue) | ✓ VERIFIED | 3740-3872, 4243, 5496-5536, 5663, 6552 |
| `internal/api/service_test.go` | Tracer révisé (positionals+excludes+AppdataPaths) + merge user-first + exclusions-only | ✓ VERIFIED | 2298-2483 lus ; tous exécutés PASS |
| `internal/api/selection_internal_test.go` | Tables blanches `TestMapRestorePaths` + `TestExcludedBranches` | ✓ VERIFIED | 8 sous-cas + 7 cas PASS |
| `internal/api/handlers.go` | os.Root browse (trio statuts, cap 500, hidden), SelectionSource, codedFailEnvelope, excluded | ✓ VERIFIED | Inchangé depuis 11a1cf7d (couvert par la réplication verte + exécutions locales) |
| `internal/api/browse_contract_test.go` (+ internal) | Fixtures GOOS-guardées + table classifieur | ✓ VERIFIED | Symlink + restricted exécutés PASS sous Linux |
| `internal/api/restore_selection_test.go` | 5 cas RESTORE-01 | ✓ VERIFIED | Tous PASS au HEAD final |
| `internal/backup/orchestrator.go` (+ test) | `RestoreDeps.SkippedPaths` additif + note scrubbed | ✓ VERIFIED | Byte-identité PASS |
| `internal/restic/restic_positionals_contract_test.go` | 3 contract tests réels (LookPath skip) | ✓ VERIFIED | **Exécutés 3/3 PASS contre restic 0.17.3 réel** |
| `internal/store/targets.go` | Doc comment pointant selection.go ; zéro changement fonctionnel | ✓ VERIFIED | Vérification initiale inchangée |
| `.planning/PROJECT.md`, `REQUIREMENTS.md`, `ROADMAP.md` | 4 dérives réalignées | ✓ VERIFIED | Gates grep toutes vertes |
| `web/`, `internal/store/migrate.go` | Non touchés (par conception) | ✓ VERIFIED | Diff vide sur toute la phase |

### Key Links

| De | Vers | Via | Statut |
|---|---|---|---|
| `service.Backup` | `selection.excludedBranches` | `tg.SelectedPaths` → qualification strict-descendant → `append(resolveExcludePatterns(...), …)` → `BackupDeps.Excludes` → `BackupArgs` `--exclude` avant `--` | ✓ WIRED (service.go:4243) |
| `service.Backup` | positionals | `effectiveBackupPaths` → `BackupDeps.AppdataPaths` (jamais préfixés) | ✓ WIRED |
| `prepareRestoreForTarget` | `mapRestorePaths` + re-validation | stored ∩ snapshot Paths (longest-prefix) → `paths.Within` sur la liste mappée → phase destructive | ✓ WIRED (5502-5536) |
| `handlers.handleBrowse` | `os.OpenRoot`/`Root.Open` | containment noyau derrière le rejet lexical | ✓ WIRED (comportement prouvé Linux) |
| `handlers.handlePatchContainer` | `errEmptySelection` | `errors.Is` → enveloppe codée | ✓ WIRED |
| `service.ContainerMounts` | `excluded` mounts | `SplitExclusion` + toHostPath, nil-guardé | ✓ WIRED |

### Data-Flow (niveau 4)

Aucune valeur rendue ne termine en retour statique ou littéral dur sur les chemins touchés : positionals depuis `effectiveBackupPaths` (stat disque réel), excludes depuis `tg.SelectedPaths` + `tg.Excludes` (ligne store relue), mapping depuis snapshot Paths réel, browse depuis `ReadDir(-1)` réel. ✓ FLOWING partout.

### Couverture des exigences (REQUIREMENTS.md, Phase 1)

NB : la consigne citait « SELECT-01..06 … REST-01 » ; le contrat REQUIREMENTS.md mappe la Phase 1 à SELECT-01/02/04, BROWSE-01..04, RESTORE-01 (+ moitié backend INTEG-04). SELECT-03/INTEG-03/RESTIC-01 = Phase 3, SELECT-05/06 = v2, INTEG-01/02 = Phases 2/4 ; il n'existe pas d'exigence « REST-01 » (typo pour RESTORE-01). Vérifié selon le contrat.

| Exigence | Plan | Statut | Preuve |
|---|---|---|---|
| BROWSE-01 | 01-02 | ✓ SATISFIED | Listing par-nœud unique, cap 500+truncated, zéro probe (texte REQUIREMENTS.md maintenant aligné, D-07 nommé) |
| BROWSE-02 | 01-02 | ✓ SATISFIED | Trio de statuts ; restricted prouvé en non-root Linux, classifieur unitaire partout ; item edge-coverage 'unclassified' → item humain 2 |
| BROWSE-03 | 01-02 | ✓ SATISFIED | Lexical+os.Root ; symlink-escape **exécuté PASS** |
| BROWSE-04 | 01-02 | ✓ SATISFIED | TestBrowseHidden bidirectionnel + 4 tests legacy non modifiés |
| SELECT-01 | 01-01, 01-05 | ✓ SATISFIED | Normalisation + round-trip + désormais application contenu (WR-01 fermé) |
| SELECT-02 | 01-01 | ✓ SATISFIED | Zéro migration (migrate.go/web diff vides) ; item edge-coverage 'unclassified' → item humain 2 |
| SELECT-04 | 01-01 | ✓ SATISFIED | Whitelist passthrough + siblings (table) |
| RESTORE-01 | 01-03 | ✓ SATISFIED | 5 cas + byte-identité + re-validation mappée (fix revue) — tous PASS au HEAD final |
| INTEG-04 (moitié backend) | 01-04 | ✓ SATISFIED (backend) | Garde + enveloppe codée + excluded array ; sémantique UI documentée = Phase 3 (REQUIREMENTS.md maintenant aligné : `[ ]`, « Backend landed (Phase 1); UI semantics pending ») |

Exigences orphelines : aucune.

### Decision Coverage (porte non-bloquante)

16/16 décisions du CONTEXT honorées dans les artefacts livrés (encodage Q1-Q4 → selection.go + tests ; listing Q1-Q4 → handlers.go browse + tests contrat ; garde Q1-Q4 → PATCH/selectionSource/codedFailEnvelope/containers-uniquement, sortie d'état reportée Phase 3 documentée ; restore Q1-Q4 → prepareRestoreForTarget/SkippedPaths/abort-pre-teardown/contract tests). Aucune décision traduite n'a disparu à l'exécution.

### Prohibitions (judgment-tier — verdicts LLM-judge non autoritaires, revue humaine recommandée)

| # | Prohibition | Verdict | Preuve |
|---|---|---|---|
| 1 | Les positionals ne sont JAMAIS dérivés des exclusions (01-05, amendant le verrou 01-01) | holds | `includesOnly` inchangé ; tracer asserte positionals == includes seuls ; la dérivation alimente uniquement `Excludes` |
| 2 | Les excludes dérivés de la sélection n'atteignent JAMAIS le restore (01-05, NOUVEAU) | holds | Énumération : site unique service.go:4243 + tests ; adaptateurs restore sans excludes ; `AppdataPaths` épinglé sans `!` |
| 3 | Aucun changement schéma/wire/migrate.go/browse/web (01-05, NOUVEAU) | holds | Diff vide (9792d927..HEAD et à l'échelle phase) |
| 4 | Le chemin de sauvegarde ne répare/ne droppe jamais silencieusement | holds | Untranslatable → rejet du save (inchangé, régression verte) |
| 5 | `AppdataPaths` ne porte jamais d'entrée `!` | holds | Enregistré depuis `effective` (includes-only) + assertion test |
| 6 | Rejet traversal/absolu byte-identique, aucun nouveau champ | holds | Tests legacy non modifiés + PASS |
| 7 | Aucun hint hasChildren / probe d'emptiness | holds | grep aucun ; REQUIREMENTS.md purgé du texte |
| 8 | La résolution d'échec restore ne dépasse jamais la phase prepare synchrone | holds | Abort 5512-5514 + garde mappé 5530-5535 ; TestRestoreEmptyIntersection (aucun stop/remove) |
| 9 | Identité de rétention intacte dans tout argv touché | holds | Aucun `--group-by` dans les fichiers touchés |
| 10 | Le garde empty-selection ne se déclenche jamais pour les sources legacy/non-arbre | holds | Gate littérale `"tree"` + cas legacy/unknown verts |
| 11 | `selectionSource` reste optionnel ; son omission n'échoue jamais un save | holds | Pointeur + cas unknown-value vert |

### Anti-patterns trouvés

| Fichier | Ligne | Motif | Sévérité | Impact |
|---|---|---|---|---|
| internal/api/service.go | 7527 | `TODO: parse virsh dominfo output…` | ℹ️ Info | Préexistant à la base 121ee887 (noté 7504 à l'initiale, décalé par le fix de revue) — hors scope phase |
| internal/api/service.go | 5530-5535 | Branche de refus du garde mappé sans test négatif dédié | ℹ️ Info | Défense-en-profondeur, miroir du garde stored-list (aussi sans test négatif) ; suggestion de suivi, aucune vérité impactée |
| 01-REVIEW.md | — | IN-01..IN-04 documentés, hors scope délibéré | ℹ️ Info | Acceptés par le workflow de revue (scope critical+warning) |

Aucun marqueur TBD/FIXME/XXX introduit. Aucun blocker.

### Vérification humaine requise

Voir frontmatter `human_verification` — 4 items (l'instance étant moteur sans UI, la liste est délibérément minimale) :

1. **Smoke Unraid réel (<unraid-host>)** — backup conteneur avec sous-dossier exclu → contenu snapshot vérifié → restore propre. Alimente 01-UAT.md.
2. **Revue manuelle des 2 items edge-coverage** (BROWSE-02, SELECT-02 'unclassified') — politique #1110.
3. **Décision de format MVP** — but pas en User Story.
4. **Push docker-folders + Actions vertes** — la substance des jobs est déjà prouvée de première main au HEAD final (suite intégrale + lint) ; reste l'exécution d'enregistrement GitHub (et le runner non-root de CI pour TestBrowseStatusRestricted, couvert ici en non-root Docker).

Résumé des items humains de la vérification initiale : item 1 (CI) → substance close, résidu = item 4 ; item 4 (dérives docs) → FERMÉ par 01-05 Task 3 ; items 2 et 3 → inchangés (items 2 et 3 ci-dessus).

### Synthèse des écarts

Aucun écart structurel. Les 10 vérités du contrat sont vérifiées, dont les deux qui étaient behavior-unverified à l'initiale (containment symlink, contrats restic réels) désormais prouvées par exécution de première main au HEAD final, et l'override SC2-demi-contenu fermé par le gap closure 01-05 (code + tests unitaires + contract test réel vert). Le fix de revue WR-01 est confirmé dans le code et couvert par les suites restore au HEAD final. Les gardes de scope tiennent (web/, migrate.go, internal/backup intacts ; dérivation confinée au site de backup).

Le statut reste **human_needed** uniquement pour les 4 items humains ci-dessus — principalement le smoke sur l'instance réelle que ce vérificateur ne peut pas exécuter depuis cette boîte, et les deux revues politiques (#1110 / format MVP) qui appartiennent à l'utilisateur. Aucun `gaps_found` : rien à replanifier.

---

_Vérifié : 2026-09-10T10:41:20Z_
_Verifier : Claude (gsd-verifier)_
