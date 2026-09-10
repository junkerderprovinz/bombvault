# Deferred Items — Phase 03 (Selection Trust & Controls)

Out-of-scope discoveries logged during execution. Not fixed here; each needs a
GSD decision before any work happens.

## Stacked-descriptor failure-revert window (plan 02 queue, carried over by plan 03)

- **Found during:** 03-03 Task 2 (queue generalization to two classes)
- **What:** When two mutations of the SAME class stack behind an in-flight save,
  `pendingDescsRef` keeps only the LATEST descriptor per class
  (latest-descriptor-wins). If that drain fails, `revertFrom`/`revertCachesFrom`
  un-apply only the latest mutation's delta; the earlier stacked mutation's
  effect stays applied locally even though the server never acknowledged it.
- **Why deferred:** Pre-existing plan 02 behavior (single-class form), not
  introduced by plan 03; the window requires two same-class mutations inside one
  in-flight save that then fails, and the next successful save re-converges the
  mirror. Fixing it means per-mutation delta journals in the queue — an
  architectural change to a shipped, tested seam (GSD Rule 4 territory).
- **Where:** `web/src/pages/Containers.tsx` (`scheduleSave`/`attemptSave`/
  `revertFrom`/`revertCachesFrom`).
