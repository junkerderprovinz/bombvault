---
status: testing
phase: 01-selection-engine-restore-safety
source: [01-VERIFICATION.md]
started: 2026-09-09T22:38:24Z
updated: 2026-09-10T10:45:00Z
---

## Current Test

number: 1
name: Smoke Unraid réel — backup d'un conteneur avec sous-dossier exclu, inspection du snapshot, restore propre
expected: |
  Sur l'instance Unraid (192.168.31.6) : sélectionner un conteneur, exclure un sous-dossier de son appdata,
  lancer un backup, vérifier via restic ls que le snapshot contient les dossiers sélectionnés SANS la branche
  exclue (positionals = racines maximales inchangées, --exclude strictement descendants), puis restaurer et
  confirmer le contenu propre. L'intégration physique (FUSE Unraid, docker.sock, chemins hôtes) n'est couverte
  par aucun harnais de test.
awaiting: user response

## Tests

### 1. Smoke Unraid réel (192.168.31.6) — round-trip sélection / exclusion / restore sur l'instance physique
expected: Backup d'un conteneur avec sous-dossier exclu → `restic ls` montre la branche exclue absente du snapshot (positionals = racines maximales, excludes descendants après `--`) → restore propre, contenu conforme. Couvre FUSE Unraid, docker.sock et les chemins hôtes — hors de portée des harnais.
result: [pending]

### 2. Revue manuelle des 2 items edge-coverage (edge-coverage.json) : BROWSE-02 et SELECT-02 « unclassified »
expected: Un humain accepte l'interprétation planifiée (BROWSE-02 : sémantique du trio de statuts selon CONTEXT browse Q2 ; SELECT-02 : sémantique zéro-migration selon le texte de l'exigence + encodage Q1/Q3, couverte par les tests round-trip legacy `[]` et migrate.go intact) ou dépose une correction. Politique #1110 — jamais auto-résolus.
result: [pending]

### 3. Décision de format MVP — le but de phase n'est pas une User Story
expected: Soit accepter la base de vérification actuelle (les 5 critères numérotés du ROADMAP, tous de niveau moteur et testables — 10/10 vérifiés), soit exécuter `/gsd mvp-phase 1` pour poser un but en User Story et re-vérifier sous cette forme.
result: [pending]

### 4. Push docker-folders + jobs GitHub Actions Test/Lint verts au HEAD final (c02149eb)
expected: |
  La SUBSTANCE des deux jobs est déjà prouvée de première main au HEAD final : suite `go test ./...` intégrale
  verte (conteneur golang:1.26-bookworm + restic 0.17.3 SHA256-vérifié — contract tests positionals 3/3 dont
  TestPositionalExcludeAbsoluteSubdirPattern, TestBrowseSymlinkEscapeRejected, TestBrowseStatusRestricted
  exécuté en non-root uid 1000) et golangci-lint 0 issue dans l'image CI. Reste l'EXÉCUTION D'ENREGISTREMENT
  GitHub : push de docker-folders (403 pour CatFoxVoyager = accès READ — à pousser avec un compte autorisé ou
  après correction des droits) puis vérification des jobs Lint + Test sur ce HEAD.
result: passed
evidence: |
  Push effectué le 2026-09-10 vers le fork CatFoxVoyager/bombvault (créé depuis junkerdeprovinz/bombvault ;
  l'URL origin locale portait une faute de frappe junkerdeprovinz → junkerderprovinz, corrigée). Branche
  docker-folders poussée au commit c02149eb, hook pre-push (gofmt/hadolint/gitleaks) passé — aucun secret.
  AUCUNE PR vers le dépôt original (consigne utilisateur) ; PR interne au fork CatFoxVoyager/bombvault#1
  (base main ← head docker-folders) ouverte uniquement comme déclencheur CI — les workflows n'acceptent
  ni push sur docker-folders ni workflow_dispatch. Résultats au HEAD exact : lint.yml = web ✓ / go ✓ /
  test ✓ (installe restic), build.yml = Boot smoke test (amd64) ✓ / Build (amd64+arm64) ✓ — tout vert.
  Le workflow « Instant Repo-Watch Ping » échoue sur le fork (bot requérant des secrets absents) — non gateant.

## Summary

total: 4
passed: 1
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps

none — la re-vérification du 2026-09-10 ne rapporte aucun `gaps_found` : le gap closure 01-05 est FERMÉ et prouvé
(helper `excludedBranches` dans internal/api/selection.go + câblage unique `BackupDeps.Excludes` dans service.Backup,
contract tests 3/3 PASS sous restic réel, anti-contamination restore vérifiée par scope-guard diff vide), le Warning
de revue (mapped-restore-list) est corrigé au commit 04363500 (re-validation `paths.Within` de la liste mappée avant
la phase destructive) et confirmé par la suite intégrale verte au HEAD final. L'item 4 du précédent UAT (dérives de
documentation PROJECT.md/REQUIREMENTS.md/ROADMAP) est résolu par le commit a426ece5.
