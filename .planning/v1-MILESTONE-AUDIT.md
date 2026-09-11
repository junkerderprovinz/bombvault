---
milestone: v1 — Tree-Based Sub-Folder Backup Selection
audited: 2026-09-11
status: passed
scores:
  requirements: 20/20 satisfied
  phases: 4/4 verified passed (53/53 must-haves)
  integration: PASS_WITH_NOTES (9/9 seams WIRED, 0 critical, 1 by-design warning, 2 info)
  flows: 2/2 E2E HOPS VERIFIED
gaps: []
tech_debt:
  - category: lint
    item: 2 pre-existing eslint warnings (web/src/components/ActivityLog.tsx:234 exhaustive-deps, web/src/components/Sidebar.tsx:567 stale seqRef in cleanup) — predates this milestone, 0 errors, tracked in phase-02 deferred-items.md
    status: deferred
  - category: nyquist
    item: all 4 VALIDATION.md remain status: draft / nyquist_compliant: false (seeded by plan-phase, never reconciled by validate-phase) — NOT-VALIDATED is coverage TODO per #2117, not a conformance failure; real coverage was delivered via VERIFICATION.md (53/53) + UAT (04: 22/22, Phases 1-3 precedents)
    status: tracked
  - category: design-note
    item: SELECT-03 preview is existence-unfiltered by design (03-VERIFICATION A3/Pitfall 5) — a stale include is counted in "{n} paths" but stat-filtered out of the actual argv (service.go:3928-3931); row-level folders.notReachable warns; checker suggested a visible qualifier or stat-mirroring as future option
    status: by-design, noted
  - category: future-surface
    item: BROWSE-04 hidden=1 opt-in has no frontend consumer yet (handlers.go:4237-4239) — additive API surface, both tree consumers agree with the default
    status: future
nyquist:
  compliant_phases: []
  partial_phases: []
  not_validated_phases: [1, 2, 3, 4]
  missing_phases: []
  overall: not_validated
---

# Milestone Audit — v1: Tree-Based Sub-Folder Backup Selection

Audité le 2026-09-11. 4 phases complètes, 15 plans exécutés, toutes les vérifications passed, UAT phase 4 à 22/22.

## Requirements matrix (3-source cross-reference)

Sources : (a) `.planning/REQUIREMENTS.md` traceability ; (b) tables requirements des 4 `*-VERIFICATION.md` ; (c) frontmatter `requirements-completed` des 15 `*-SUMMARY.md`.

| Requirement | Phase | Traceability | VERIFICATION | SUMMARY plans | Verdict |
|-------------|-------|--------------|--------------|---------------|---------|
| BROWSE-01 | 1 | Complete | 01 ✓ SATISFIED | 01-02, 01-05 | satisfied |
| BROWSE-02 | 1 | Complete | 01 ✓ SATISFIED | 01-02 | satisfied |
| BROWSE-03 | 1 | Complete | 01 ✓ SATISFIED | 01-02 | satisfied |
| BROWSE-04 | 1 | Complete | 01 ✓ SATISFIED | 01-02 | satisfied |
| SELECT-01 | 1 | Complete | 01 ✓ SATISFIED | 01-01, 01-05 | satisfied |
| SELECT-02 | 1 | Complete | 01 ✓ SATISFIED | 01-01, 01-05 | satisfied |
| SELECT-04 | 1 | Complete | 01 ✓ SATISFIED | 01-01, 01-05 | satisfied |
| RESTORE-01 | 1 | Complete | 01 ✓ SATISFIED | 01-03 | satisfied |
| TREE-01 | 2 | Complete | 02 ✓ SATISFIED | 02-01, 02-03 | satisfied |
| TREE-02 | 2 | Complete | 02 ✓ SATISFIED | 02-01, 02-02 | satisfied |
| TREE-03 | 2 | Complete | 02 ✓ SATISFIED | 02-01 | satisfied |
| TREE-04 | 2 | Complete | 02 ✓ SATISFIED | 02-01, 02-03 | satisfied |
| TREE-05 | 2 | Complete | 02 ✓ SATISFIED | 02-03 | satisfied |
| TREE-06 | 2 | Complete | 02 ✓ SATISFIED | 02-02 | satisfied |
| INTEG-01 | 2 | Complete | 02 ✓ SATISFIED | 02-01, 02-02, 02-03 | satisfied |
| SELECT-03 | 3 | Complete | 03 ✓ SATISFIED | 03-02, 03-03 | satisfied |
| INTEG-03 | 3 | Complete | 03 ✓ SATISFIED | 03-02 | satisfied |
| INTEG-04 | 1+3 | **Complete (checkbox mis à jour cet audit)** | 01 ✓ (backend), 03 ✓ SATISFIED (code, « bookkeeping only ») | 01-04, 03-02, 03-03 | satisfied (update checkbox) |
| RESTIC-01 | 3 | Complete | 03 ✓ SATISFIED | 03-01, 03-03 | satisfied |
| INTEG-02 | 4 | Complete | 04 ✓ SATISFIED | 04-01, 04-02, 04-04 | satisfied |

- **20/20 satisfied** ; orphelins : aucun ; unsatisfied : aucun → FAIL gate non déclenché.
- INTEG-04 : la seule ligne non-« Complete » de la traceability. Les trois sources convergent (garde backend Phase 1 + sémantique UI Phase 3, 03-VERIFICATION notait déjà « bookkeeping only ») → ligne 38 `[ ]`→`[x]` et traceability → Complete, appliqués dans cet audit.

## Phases

| Phase | Nom | Statut | Vérification | Security | UAT |
|-------|-----|--------|--------------|----------|-----|
| 1 | Selection Engine & Restore Safety | completed 2026-09-10 | passed 10/10 | verified | precedent engine-level + real-restic proof |
| 2 | Container Panel Tree Selection | completed 2026-09-10 | passed 17/17 | verified | precedent interactive |
| 3 | Selection Trust & Controls | completed 2026-09-10 | passed 14/14 | verified | precedent interactive |
| 4 | File Sets Parity | completed 2026-09-11 | passed 12/12 | verified (threats_open 0, 16 menaces closed) | 22/22 pass, 0 issues (04-UAT.md) |

## Integration (gsd-integration-checker, PASS_WITH_NOTES)

9 seams vérifiés WIRED, chaque finding mappé aux requirement IDs :

1. Contrat de normalisation unique — `internal/api/selection.go` (SELECT-01/02/04) : tous les readers backend décodent par lui ; aucun second site de pruning ; `internal/store` n'interprète jamais les entrées.
2. Réutilisation composant (INTEG-02) — `SelectionTree.tsx` importé par Containers.tsx:5 ET Files.tsx:32 ; arithmétique pure partagée `selectionTree.ts` ; aucune logique « ! » dupliquée dans web/src.
3. Contrat browse (BROWSE-01..04) — cap 500 + truncated, statuts KIND only, containment os.Root, hidden opt-in ; consommé par api.ts:2024 et SelectionTree.tsx:186.
4. Garde empty-selection (INTEG-04) — containers tree-gated (service.go:3870), Files inconditionnel (service.go:8991) ; asymétrie délibérée et documentée (un set se supprime, pas un conteneur).
5. CACHEDIR → argv (RESTIC-01) — union fraîche par backup (service.go:4187), `--exclude-caches` (restic.go:397), pin TestBackupArgsExcludeCaches.
6. Exclusions jamais positionnels (SELECT-04) — includesOnly partout ; branches exclues en `--exclude` typés (service.go:4256, 8787).
7. Mapping restore (RESTORE-01) — mapRestorePaths synchrone pre-teardown, abort intersection vide en prepare (service.go:5524-5528) ; D-08 files (service.go:9204-9209).
8. Compile file-set → orchestrateur — fileSetPositionals (nil-seed, re-anchor, fallback) ; SourcePaths passthrough (files_orchestrator.go:62-66) ; cap 64 + containment au PATCH.
9. Miroirs wire — backupPaths/selectionSource/excludeCaches (api.ts:906-908), selectedPaths three-state pointer (api.ts:2369-2374 ↔ handlers.go:4598), browse truncated, code?: string.

**Findings** : 1 WARNING (SELECT-03 — aperçu existence-unfiltered vs argv stat-filtré ; BY-DESIGN, déjà documenté en 03-VERIFICATION A3/Pitfall 5 avec warns `folders.notReachable` au niveau row) ; 2 INFO (BROWSE-04 hidden sans consommateur ; métadonnées Excludes du snapshot portant les patterns dérivés — tradeoff accepté 2026-09-09).

## E2E flows

- **Container backup** : toggle arbre → PATCH backupPaths+selectionSource → compile includesOnly positionnels + excludes dérivés + union exclude-caches → snapshot Paths forme maximal-root → restore mapping longest-prefix avec abort prepare. **HOPS VERIFIED** (hop preview : warning by-design ci-dessus).
- **File set** : NULL selectedPaths → seed racine synthétique sans écriture → PATCH 64-cap/containment → compile re-anchor/fallback → SourcePaths passthrough → snapshot Paths → garde D-08 restore + refusal empty-selection codé. **HOPS VERIFIED** (preuve live UAT : 04-UAT tests 1-5).

## Nyquist validation

Hook validate-phase actif mais jamais exécuté : les 4 VALIDATION.md sont restés en `status: draft` (seedés par plan-phase). Classification : 4× NOT-VALIDATED — coverage TODO per #2117, pas un échec de conformité. La couverture réelle a été livrée par les VERIFICATION.md goal-backward (53/53 must-haves, preuves first-hand dont restic 0.17.3 réel) et les UAT (Phase 4 : 22/22 incl. 10 checkpoints UI navigateur réel et smoke API live sur instance Unraid).

## Technical debt & deferred

| Catégorie | Item | Statut |
|-----------|------|--------|
| lint | 2 eslint warnings pre-existing (ActivityLog.tsx:234, Sidebar.tsx:567) — prédatent le milestone, 0 erreurs | deferred (phase-02 deferred-items.md) |
| nyquist | 4 VALIDATION.md en draft, jamais réconciliés | tracked (coverage assurée par VERIFICATION+UAT) |
| design-note | SELECT-03 aperçu existence-unfiltered (argv stat-filtré) | by-design, noté |
| future-surface | BROWSE-04 hidden=1 sans consommateur frontend | future |
| v2 (REQUIREMENTS.md) | SELECT-06 backup-time coverage diff ; TREE-07/08, SELECT-05 | deferred v2, 2026-09-09 |

## Verdict

**PASSED** — aucun gap, aucune exigence unsatisfied, intégration cross-phase propre. Le milestone v1 peut être complété.
