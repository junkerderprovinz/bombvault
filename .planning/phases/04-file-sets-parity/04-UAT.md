---
status: complete
phase: 04-file-sets-parity
source: [04-01-SUMMARY.md, 04-02-SUMMARY.md, 04-03-SUMMARY.md, 04-04-SUMMARY.md]
started: 2026-09-11T05:57:31Z
updated: 2026-09-11T06:30:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

number: none
name: Session complete — 22/22 pass, 0 issues, 0 gaps
awaiting: nothing

## Tests

### 1. Preuve moteur — snapshot Paths == positionnels compilés (restic 0.17.3 réel)
expected: Dans le conteneur golang:1.26-bookworm avec restic 0.17.3 (SHA256 vérifié), TestMultiPositionalPathsMirrorSelection passe : Paths du snapshot deep-equal à la liste positionnelle compilée, ordre préservé, branche exclue filtrée.
result: pass
reported: |
  Re-run UAT-frais à HEAD ad1c12a5 : conteneur golang:1.26-bookworm, restic 0.17.3
  linux/amd64 téléchargé et SHA256-vérifié contre l'ARG du Dockerfile
  (5097faed…d4d3, "…/tmp/restic.bz2: OK"), `restic version` → 0.17.3.
  `go test ./internal/restic/ -run TestMultiPositionalPathsMirrorSelection -v`
  → PASS en 4.91s. Cohérent avec la preuve de verification (4.97s à 0529320c).

### 2. UAT visuelle navigateur réel — surfaces Files de la phase
expected: Sur la page Files d'une instance réelle : la divulgation « Choose folders » d'une carte de file set ouvre l'arbre à UNE racine (label mono en chemin brut, sans flèche dest), l'aperçu « N paths » dans le label racine (dont « 1 path » honnête sur un set jamais édité — seed NULL), la divulgation « {n} exclusions » avec lignes relatives mono ltr muted, la ligne de refus text-statusWarn après refus du dernier untick, la région scrollable clamp(12rem,55vh,32rem). Aucun switch CACHEDIR, aucun Reset, aucune custom-path row sur la carte ; le dialog affiche la caption de changement de dossier sous le FolderBrowser. Rythme identique à l'arbre conteneurs de la Phase 2 (composant partagé).
result: pass
reported: |
  Instance : BombVault-test redéployée sur bombvault:phase4-ad1c12a5 (healthy,
  /api/health ok) ; navigateur playwright sur http://192.168.31.6:13000/files.
  Set créé pour l'UAT (« uat-test », planification décochée). Vérifié :
  - Dialog « Add folder set » : caption « Changing the folder clears the ticked
    sub-folder selection. » rendue sous le contrôle Folder/Browse, avant le hint
    générique, inconditionnelle (04-04 D2).
  - Disclosure « Choose folders » : arbre à UNE racine (aria-setsize="1",
    aria-posinset="1", aria-level="1"), label chemin brut mono SANS flèche dest,
    seed NULL honnête → racine CHECKED « 1 paths » avant tout toggle (le « 1 paths »
    non-pluralisé est le rendu VOULU : folders.previewPaths = clé {n} invariante,
    i18n.ts:744-746, pin dom « byte-identical, no pluralization »).
  - Région scrollable : classe h-[clamp(12rem,55vh,32rem)] overflow-y-auto,
    hauteur calculée 473,5px (=55vh) (04-03).
  - Après untick de « cache » : bouton « 1 exclusions » ; contenu = <li> « cache »
    en chemin RELATIF, font-mono text-carbon-textMuted, zéro contrôle interactif
    (04-04 D1) ; disclosure effondré à chaque réouverture.
  - Refus du dernier untick (racine) : ligne « A set needs at least one folder, so
    the last tick cannot be removed. Use Remove set if you no longer want this
    set. » en text-xs text-statusWarn, état racine inchangé (mixed, 1 paths).
  - Aucun switch CACHEDIR, aucun bouton Reset, aucune custom-path row dans la
    carte ; le seul switch est « Include in schedule » (légitime).
  Screenshots : uat-p4-test2-null-seed.png, uat-p4-test2-exclusions-d06.png.
  UI checkpoints: 10 auto-verified, 0 queued for manual review.

### 3. Pipeline live-save + exact reopen (navigateur réel)
expected: Cocher/décocher un sous-dossier n'exige aucun bouton Save — chaque toggle part en PATCH sérialisé ; l'aperçu « N paths » suit exactement les inclusions (égal au nombre de positionnels que le prochain backup remettra à restic) ; après rechargement de page la sélection est reconstruite à l'identique (exact reopen, zéro refetch visible) et les états mixed/cascade restent corrects.
result: pass
reported: |
  Untick de « cache » (case cochée directement) : effet immédiat sans aucun bouton
  Save — cascade « cache/restic » décochée, ssh/templates intactes, racine passée
  en checked=mixed, aperçu « 1 paths » inchangé (= includes = positionnels du
  prochain backup, confirmé ensuite par le snapshot réel du test 5b). Rechargement
  de page : arbre reconstruit à l'identique (cache+restic untickés, ssh/templates
  cochées, racine mixed « 1 paths », disclosure exclusions présent mais effondré),
  aucune erreur réseau visible.

### 4. Click-through CR-01 — édition du dossier via le dialog puis toggle dans l'arbre de la carte
expected: Éditer le dossier d'un set dans le dialog, sauvegarder : la carte se remonte proprement sur le seed honnête post-clear (racine CHECKED, « 1 path » — sélection purgée côté serveur, jamais l'ancienne sélection résurrectrice) ; déplier ensuite « Choose folders » et cocher un sous-dossier du NOUVEAU dossier fonctionne sans refus spurieux « not under the set's source folder » et sans éditeur wedgé (fix CR-01 d8fd491e, remontage par fileSetEditorKey).
result: pass
reported: |
  Dialog → Folder changé vers user/appdata/BombVault-test/cache → Save. La carte
  se remonte sur le nouveau chemin avec « Choose folders » EFFONDRÉ (nouveau
  montage, zéro état périmé). À l'ouverture : racine = NOUVEAU dossier CHECKED
  « 1 paths », enfant restic coché par cascade, AUCUN bouton d'exclusions
  résurrecteur (sélection bien purgée côté serveur, confirmé aussi par l'API :
  selectedPaths omis du wire après clear). Untick de restic ensuite : PATCH parti
  normalement, racine mixed « 1 paths », « 1 exclusions » apparue, AUCUNE ligne
  statusWarn de refus spurieux, éditeur non wedgé. Screenshot :
  uat-p4-test4-cr01-remount.png.

### 5. Smoke instance réelle — argv/snapshot vs sélection (optionnel, même classe que Phases 1-3)
expected: Sur l'instance Unraid : (a) un file set jamais édité (selected_paths NULL) sauvegarde exactement comme avant la phase — snapshot Paths == [SourceDir] ; (b) cocher un sous-dossier puis lancer le backup du set produit un snapshot dont Paths == aux racines cochées ; (c) après l'édition de dossier du test 4 (sélection purgée), le backup suivant redevient [nouveau SourceDir].
result: pass
reported: |
  API de l'instance de test (SSH + curl sur 13000), set id 9426c901….
  (b) Sélection [racine cache cochée, restic exclue] (serve-back : liste plate
  ["/host/user/.../cache", "!/host/user/.../cache/restic"]) → POST backup →
  snapshot paths == ["/host/user/user/appdata/BombVault-test/cache"], tag
  fileset:uat-test : EXACTEMENT 1 positionnel = la racine cochée, branche restic
  filtrée (passée en --exclude). L'aperçu « 1 paths » == le Paths réel.
  (c)+(a) PATCH {path: "user/appdata/BombVault-test"} → {ok:true} ; le serve-back
  suivant omet selectedPaths ENTIÈREMENT (preuve directe du contrat 04-02 D5 :
  clear vers SQL NULL, jamais "[]") ; backup suivant → snapshot paths ==
  ["/host/user/user/appdata/BombVault-test"] : mono SourceDir legacy
  byte-identical. Le seed NULL observé au test 2 (aperçu « 1 paths » racine
  CHECKED avant tout toggle) complète (a).

### 6. Revue des 12 prohibitions judgment-tier (ADR-550 D4)
expected: Un humain confirme les 12 verdicts « holds » non autoritatifs (verdicts LLM/grep — ne peuvent être absorbés silencieusement dans un pass) ou dépose des corrections. Liste : (1) aucune migration livrée éditée/renumérotée ; (2) identité retention jamais regroupée ; (3) internal/store n'interprète jamais selected_paths ; (4) aucune seconde implémentation de normalisation/compile ; (5) positionnels influencés par l'utilisateur uniquement après -- via builders typés ; (6) aucun carrier selectionSource sur le PATCH files ; (7) aucune sélection vide persistée silencieusement côté serveur ; (8) aucune seconde sémantique de mapping restore files-domain ; (9) toContainerPath jamais appliqué aux entrées file-set ; (10) UpdateFileSet n'écrit jamais selected_paths (owned setter seul + clear writer nommé) ; (11) l'arbre Files ne rend jamais switch CACHEDIR/custom-path rows/Reset, la liste d'exclusions n'est jamais une surface de mutation, la sélection jamais dans localStorage ; (12) web/dist : seul le index.html tracké est commité.
result: pass
reported: |
  Utilisateur : « Confirmé — tout tient » (AskUserQuestion, 2026-09-11). Les 12
  verdicts « holds » de 04-VERIFICATION.md sont confirmés sans correction ; la
  confirmation visuelle playwright du test 2 corrobore (11) (aucun switch
  CACHEDIR, aucun Reset, aucune custom-path row, exclusions non interactives).

### 7. Décision format MVP-mode
expected: ROADMAP marque la Phase 4 « Mode: mvp » mais le goal n'est pas en format User Story (user-story.validate → false) ; la vérification est goal-backward contre les 2 Success Criteria numérotés (precedent Phases 1/3). Soit ce fondement est accepté, soit /gsd mvp-phase 4 définit un goal User Story et re-vérifie en forme MVP.
result: pass
reported: |
  Utilisateur : « Accepté (recommandé) » (AskUserQuestion, 2026-09-11). Le
  fondement goal-backward contre les 2 Success Criteria est accepté (precedent
  Phases 1/3) ; aucune re-planification MVP requise. Cohérent avec le
  section_manifest de session qui exclut mvp-uat-framing.

### 8. [04-01 D1] Migration v101 file_set_selected_paths
expected: Migration v101 : colonne nullable TEXT ajoutée inconditionnellement après v100, sans garde alreadySatisfied ; les lignes existantes lisent NULL.
result: pass
source: automated
coverage_id: 04-01-D1

### 9. [04-01 D2] Owned setter SetFileSetSelectedPaths
expected: La sélection écrite round-trippe par les trois chemins de lecture ; nil stocke SQL NULL (jamais '[]') ; UpdateFileSet n'écrase jamais une sélection.
result: pass
source: automated
coverage_id: 04-01-D2

### 10. [04-01 D3] Passthrough SourcePaths de l'orchestrateur
expected: Passthrough additif avec fallback legacy nil/vide ; tags et excludes inchangés par le compile multi-racines.
result: pass
source: automated
coverage_id: 04-01-D3

### 11. [04-01 D4] Helper de compile fileSetPositionals
expected: Legacy nil, filtre re-anchor (segment-aligned, sibling-prefix safe), fallback filtered-empty, ordre canonique déterministe.
result: pass
source: automated
coverage_id: 04-01-D4

### 12. [04-01 D5] Site de compile unique BackupFileSet
expected: Positionnels maximal-root + derives excludes après les patterns de l'utilisateur ; colonne NULL compile byte-identiquement (pin legacy intact et vert).
result: pass
source: automated
coverage_id: 04-01-D5

### 13. [04-02 D1] Boundary pointeur PATCH selectedPaths
expected: absent = intact (ne clear jamais), déclaré explicitement pour DisallowUnknownFields.
result: pass
source: automated
coverage_id: 04-02-D1

### 14. [04-02 D2] Validation par entrée avant TOUTE écriture
expected: Rejet atomique (incl. piège sibling-prefix), cap 64 entrées, stockage canonique normalisé, serve-back de la vue.
result: pass
source: automated
coverage_id: 04-02-D2

### 15. [04-02 D3] D-06 refus sélection vide
expected: Sélection à zéro includes refusée avec code empty-selection + message Remove-set ; sélection prioritaire byte-identique après un refus.
result: pass
source: automated
coverage_id: 04-02-D3

### 16. [04-02 D4] Clear au changement de dossier
expected: Clear (vers SQL NULL legacy) avec précédence clear-wins sur les entrées de la même requête ; comparaison sur racines résolues.
result: pass
source: automated
coverage_id: 04-02-D4

### 17. [04-02 D5] Miroir wire
expected: Set NULL omet selectedPaths entièrement ; set édité renvoie la forme stockée ; miroir TS FileSetView + patchFileSet sur les deux points.
result: pass
source: automated
coverage_id: 04-02-D5

### 18. [04-02 D6] D-08 garde restore
expected: Le restore in-place mappe la sélection compilée contre les Paths du snapshot et abandonne synchronement avec « nothing to restore » sur intersection vide ; snapshots legacy mono-path restaurables ; routes to-folder et snapshots sans paths intactes.
result: pass
source: automated
coverage_id: 04-02-D6

### 19. [04-04 D1] Liste d'audit des exclusions côté Files
expected: Ligne de compte folders.exclusions et chemins relatifs mono muted, effondrée à chaque réouverture, identique pour racines actives et dormantes, non interactive.
result: pass
source: automated
coverage_id: 04-04-D1

### 20. [04-04 D2] Caption pathChangeHint du FileSetDialog
expected: Divulgation de la conséquence du changement de dossier sous le FolderBrowser, inconditionnelle, hint de path existant intact.
result: pass
source: automated
coverage_id: 04-04-D2

### 21. [04-04 D3] Phase gate vert
expected: go build/vet/gofmt clean, go test ./... ok, suite web complète 93 fichiers, npm ci && npm run build verts.
result: pass
source: automated
coverage_id: 04-04-D3

### 22. [04-04 D4] Roll du bundle web/dist tracké
expected: Le hash de bundle rollé est commité afin que la SPA embarquée corresponde à la source (assets gitignored).
result: pass
source: automated
coverage_id: 04-04-D4

## Summary

total: 22
passed: 22
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

<!-- YAML format for plan-phase --gaps consumption -->

- none: all 22 tests pass — 15 automated (coverage) + 7 orchestrator-executed/user-confirmed (engine proof, visual UAT over the live phase-4 instance, live-save/reopen, CR-01 click-through, real-instance smoke, 12 prohibitions confirmed, MVP-format accepted)
