# Phase 2: Container Panel Tree Selection - Context

**Gathered:** 2026-09-10
**Status:** Ready for planning

<domain>
## Phase Boundary

The lazy tri-state tree lands in the container panel: users unfold any discovered mount or custom path, tick subfolders with cascade semantics, get mixed-state parents, full keyboard + ARIA access, and exact state reconstruction on reopen. Requirements: TREE-01, TREE-02, TREE-03, TREE-04, TREE-05, TREE-06, INTEG-01. Pure frontend over the Phase 1 contracts — the browse listing, the flat `backupPaths` encoding (bare + `!` exclusions), the PATCH with `selectionSource:"tree"`, and the empty-selection guard all landed in Phase 1; no backend change is expected in this phase.

</domain>

<decisions>
## Implementation Decisions

### Sémantique du clic sur checkbox mixed (TREE-02/TREE-03 — résout la note ROADMAP « spec the click-on-mixed behavior »)
- **D-01:** Cycle mémorisé à trois états : unchecked → clic → fill (tout cocher en dessous) ; checked-all → clic → tout décocher ; clic sur un parent mixed → décocher tout en dessous en mémorisant l'état partiel ; re-cocher une checkbox qui avait un état partiel mémorisé → restaurer exactement cet état partiel (TREE-03 verbatim : « re-activating a remembered-partial checkbox restores its prior partial state »). Le « fill » attendu par le research s'applique uniquement à la transition unchecked→checked ; un état mixte n'est jamais écrasé silencieusement.

### Intégration au panneau conteneur (INTEG-01)
- **D-02:** Évolutif — la liste mounts/custom-paths existante DEVIENT l'arbre : chaque rangée gagne un chevron d'expansion lazy et sa checkbox devient tri-state. Pas de modal, pas de bouton « sélection fine », pas de double présentation de la même donnée sur le même écran. Les checkboxes de mounts actuelles sont les racines de l'arbre.

### Flux de sauvegarde
- **D-03:** Live-save à chaque toggle : PATCH `/api/containers/{name}` avec le flat set (`backupPaths`) et `selectionSource:"tree"` immédiatement après chaque cochage/décochage — cohérent avec le live-save existant des checkboxes de mounts ; le serveur reste l'unique source de vérité, donc TREE-04 (reconstruction exacte à la réouverture) découle de la relecture serveur, pas d'un cache local de sélection.

### Garde empty-selection côté UI (avant la sémantique complète INTEG-04 en Phase 3)
- **D-04:** Blocage proactif : un toggle qui résulterait en zéro inclusion pour l'item est bloqué côté client avec un message inline traduit (en + de) routant vers la désactivation de l'item. Le PATCH refusé (code `"empty-selection"`, garde Phase 1) ne devrait jamais être atteignable depuis l'arbre. La sémantique complète de désélection totale reste en Phase 3 — ce blocage est le minimum pré-Phase 3, pas une anticipation.

### Expansion persistante & listings tronqués
- **D-05:** État d'expansion des nœuds persisté en localStorage `bv-*` par conteneur, plafonné (clé bornée, pas de croissance illimitée) ; la sélection, elle, est TOUJOURS reconstruite depuis le serveur (le localStorage ne porte que le confort visuel d'expansion).
- **D-06:** Listing tronqué (cap 500 + `truncated:true` du contrat Phase 1) rendu comme une ligne muted non interactive « N premières entrées affichées » en fin de liste du nœud — honnête sur le contenu, aucune pagination en v1.

### Claude's Discretion
- Style visuel exact des trois états de checkbox (suivre les tokens sémantiques `carbon-*`/`status*` et les `glim-*` engine classes — jamais de hex brut)
- Structure interne du composant (découpage hooks/fichiers, nom du composant arbre) — suivre les conventions PascalCase/components
- Détails d'implémentation APG au-delà des exigences TREE-05 (roving tabindex exact, gestion focus après fermeture) — suivre le pattern TreeView APG et les précédents keyboard (`DropdownListbox.keyboard.dom.test.tsx`)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spécification milestone & exigences
- `.planning/research/SUMMARY.md` — spécification de référence du milestone (sémantique cascade Veeam/Duplicati, positions UI attendues)
- `.planning/REQUIREMENTS.md` § Tree Component & Selection Semantics + § Domain Integration — TREE-01..06 et INTEG-01 verrouillés
- `.planning/ROADMAP.md` § Phase 2 — but, 5 critères de succès, notes (click-on-mixed à spécifier au planning — résolu par D-01 ; i18n les deux locales ; commit `web/dist`)

### Contrats backend Phase 1 à consommer (lecture seule pour cette phase)
- `internal/api/selection.go` — helpers purs de normalisation/classification (includes/exclusions, pruning par classe) : la définition de la forme flat set que l'UI doit produire et relire
- `internal/api/handlers.go` (handleBrowse + PATCH containers) — contrat de listing (`status` trio, cap 500 + `truncated`, `hidden=1`) et garde empty-selection (`selectionSource:"tree"` → code `"empty-selection"`)
- `.planning/phases/01-selection-engine-restore-safety/01-CONTEXT.md` — décisions d'origine d'encodage et de contrat de listing

### Frontend cible & précédents
- `web/src/pages/Containers.tsx` — panneau cible : sélecteur mounts actuel (checkboxes live-save), custom paths, ExcludesEditor voisin
- `web/src/lib/api.ts` — seul client API ; types miroir du JSON Go (réponse browse, corps PATCH) à étendre si besoin
- `web/src/components/FolderBrowser.tsx` — précédent SPA du contrat de listing (rendu statut/erreur scrubbed, cohérence hidden entries BROWSE-04)
- `web/src/components/SnapshotFileTree.tsx` — précédent SPA d'affichage arborescent (restore-side)
- `web/src/lib/displayPrefs.ts` — pattern localStorage `bv-*` pour D-05
- `web/src/lib/pageShell.ts` — PAGE_SHELL (racines de page)

### Standards d'accessibilité
- WAI-ARIA Authoring Practices, TreeView Pattern — https://www.w3.org/WAI/ARIA/apg/patterns/treeview/ — TREE-05 : arrows move/expand/collapse, Space toggle, `aria-expanded`, `aria-checked="mixed"`, `aria-level`/`setsize`/`posinset` sur nœuds lazy

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `GET /api/browse` (via `web/src/lib/api.ts`) — listing per-node avec `status`/`truncated`/`hidden` : l'arbre consomme ce contrat tel quel, aucun changement backend
- `FolderBrowser.tsx` / `SnapshotFileTree.tsx` — précédents de rendu listing/arborescence (gestion statut, styles)
- `IncludeToggle.tsx`, `Button.tsx`, `Badge.tsx` — contrôles partagés `glim-*` (les couleurs de statut vont aux badges/chips, jamais aux contrôles interactifs)
- vitest + @testing-library/react + jsdom — tests `.dom.test.tsx` co-localisés ; précédents keyboard : `DropdownListbox.keyboard.dom.test.tsx`, `ColorPickerPopover.keyboard.dom.test.tsx`

### Established Patterns
- Pas de state library : état local React + ref-mirroring (`xRef.current = x`) quand un callback doit lire l'état frais ; localStorage `bv-*` pour les préférences ; window CustomEvents
- Tailwind utility classes sur tokens sémantiques uniquement ; PAGE_SHELL sur la racine de page (lint `bombvault/page-uses-page-shell`)
- Toute chaîne utilisateur via `t()` de `useT()` (lint-enforced, en + de inline dans `i18n.ts`) ; pas d'em dash dans le texte utilisateur ; texte d'erreur backend affiché verbatim
- Named exports partout ; imports relatifs ; commentaires narratifs en tête de fichier non trivial

### Integration Points
- `GET /api/containers/{name}/mounts` — racines découvertes + chemins custom (traduction host/container déjà servie)
- `GET /api/browse?path=…` — enfants lazy par nœud (status trio, cap, hidden opt-in)
- `PATCH /api/containers/{name}` corps `{backupPaths, selectionSource:"tree"}` — live-save D-03 ; réponse d'erreur enveloppe maison avec `code`
- `web/dist` doit être rebuildé et committé après tout changement `web/` (SPA embarquée dans le binaire)

</code_context>

<specifics>
## Specific Ideas

- Le critère de succès 3 (« reopen = exactly what they left ») porte sur l'état de SÉLECTION reconstruit depuis le flat set serveur ; l'expansion visuelle est un confort distinct (D-05) et ne doit jamais devenir la source de vérité
- La note ROADMAP « Spec the click-on-mixed behavior (fill vs cycle) during planning » est résolue par D-01 (cycle mémorisé) — le planner n'a pas à la rouvrir
- i18n : les deux locales (en, de) doivent passer les tests de parité `i18n.parity/quality/orphans` ; `web/dist` committé

</specifics>

<deferred>
## Deferred Ideas

- « Charger plus » / pagination dans l'arbre au-delà du cap 500 (exigerait une pagination backend) — hors v1
- Sortie de l'état « tout décoché » / retour explicite à l'auto-détection → Phase 3 (INTEG-04 UI)
- Note de rétrécissement (SELECT-03) et liste reviewable d'exclusions (INTEG-03) → Phase 3
- Search/filter dans l'arbre (TREE-07), tailles de dossiers (SELECT-05) → v2 (déjà tracés dans REQUIREMENTS.md)

</deferred>

---
*Phase: 2-Container Panel Tree Selection*
*Context gathered: 2026-09-10*
