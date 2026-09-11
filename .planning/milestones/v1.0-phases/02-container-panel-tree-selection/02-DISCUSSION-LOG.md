# Phase 2: Container Panel Tree Selection - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-10
**Phase:** 2-Container Panel Tree Selection
**Areas discussed:** Sémantique du clic sur checkbox mixed, Intégration au panneau conteneur, Flux de sauvegarde, Réaction au refus empty-selection, Expansion persistante + listings tronqués
**Mode:** --auto (sélection et réponses auto-resolves sur l'option recommandée ; chaque choix journalisé pour revue utilisateur)

---

## Sémantique du clic sur checkbox mixed

| Option | Description | Selected |
|--------|-------------|----------|
| Cycle mémorisé | unchecked→clic→fill tout coché ; checked-all→clic→tout décoché ; mixed→clic→décoche en mémorisant le partiel ; re-cocher un ancien partiel→restaure l'état partiel | ✓ |
| Fill pur | mixed→clic→tout cocher (écrase l'état partiel) | |
| Cycle complet | unchecked→mixed→checked→unchecked à chaque clic (sans mémoire) | |

**User's choice:** [auto] Cycle mémorisé (recommended default)
**Notes:** TREE-03 exige littéralement la restauration de l'état partiel ; le « fill » du research s'applique à unchecked→checked uniquement. Résout la note ROADMAP « Spec the click-on-mixed behavior (fill vs cycle) during planning ».

---

## Intégration au panneau conteneur

| Option | Description | Selected |
|--------|-------------|----------|
| Évolutif — la liste devient l'arbre | Chaque rangée mount/custom-path gagne un chevron lazy et sa checkbox devient tri-state ; une seule vue | ✓ |
| Complémentaire — vue actuelle + « sélection fine » | Garder l'UI mounts existante + un bouton ouvrant un panneau/modal arbre | |

**User's choice:** [auto] Évolutif (recommended default)
**Notes:** INTEG-01 demande de déplier un mount DANS le panneau ; deux présentations concurrentes de la même donnée sur le même écran seraient de la confusion.

---

## Flux de sauvegarde

| Option | Description | Selected |
|--------|-------------|----------|
| Live-save à chaque toggle | PATCH immédiat avec `selectionSource:"tree"` après chaque cochage/décochage | ✓ |
| Édition locale + bouton Save | Moins de PATCH ; introduit un état d'édition annulable inédit dans ce panneau | |

**User's choice:** [auto] Live-save (recommended default)
**Notes:** Cohérent avec le live-save existant des checkboxes de mounts ; le serveur reste l'unique source de vérité (TREE-4 naturel) ; le garde empty-selection protège chaque pas.

---

## Réaction au refus empty-selection

| Option | Description | Selected |
|--------|-------------|----------|
| Blocage proactif + message inline | La dernière inclusion d'un item ne peut pas être décochée depuis l'arbre ; message traduit routant vers la désactivation de l'item | ✓ |
| PATCH puis toast + rollback | Laisser partir la requête, afficher l'erreur backend (code "empty-selection"), rollback visuel | |
| Toast seul | Afficher l'erreur sans rollback visuel | |

**User's choice:** [auto] Blocage proactif (recommended default)
**Notes:** Minimum pré-Phase 3 ; la sémantique complète INTEG-04 (sortie de l'état « tout décoché ») reste en Phase 3 — pas de scope creep.

---

## Expansion persistante + listings tronqués

| Option | Description | Selected |
|--------|-------------|----------|
| localStorage `bv-*` plafonné | État d'expansion par conteneur survit aux sessions ; clé bornée | ✓ |
| Toujours replié au chargement | Reconstruction de la sélection uniquement | |
| Ligne muted non interactive | « N premières entrées affichées » en fin de liste d'un nœud tronqué | ✓ |
| Bouton « charger plus » | Pagination dans l'arbre (exigerait pagination backend) | |

**User's choice:** [auto] localStorage plafonné + ligne muted (recommended defaults)
**Notes:** Le pattern display-prefs existe déjà ; « charger plus » est une nouvelle capacité — notée en deferred.

---

## Claude's Discretion

- Style visuel des trois états (tokens sémantiques `carbon-*`/`status*`)
- Structure interne du composant (découpage, nommage)
- Détails APG au-delà de TREE-05 (roving tabindex, gestion du focus)

## Deferred Ideas

- « Charger plus » / pagination dans l'arbre au-delà du cap 500 — hors v1
- Sortie de l'état « tout décoché » → Phase 3 (INTEG-04 UI)
- SELECT-03 / INTEG-03 → Phase 3 ; TREE-07 / SELECT-05 → v2 (déjà tracés)
