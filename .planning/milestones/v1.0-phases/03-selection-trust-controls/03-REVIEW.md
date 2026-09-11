---
phase: 03-selection-trust-controls
reviewed: 2026-09-10T12:00:00Z
depth: standard
files_reviewed: 59
files_reviewed_list:
  - internal/api/handlers.go
  - internal/api/handlers_test.go
  - internal/api/service.go
  - internal/api/service_test.go
  - internal/restic/restic.go
  - internal/restic/restic_args_test.go
  - internal/store/migrate.go
  - internal/store/targets.go
  - internal/store/targets_test.go
  - web/dist/index.html
  - web/src/components/SelectionTree.dom.test.tsx
  - web/src/components/SelectionTree.keyboard.dom.test.tsx
  - web/src/components/SelectionTree.tsx
  - web/src/lib/api.ts
  - web/src/lib/i18n.ts
  - web/src/lib/locales/ar.ts
  - web/src/lib/locales/bg.ts
  - web/src/lib/locales/ca.ts
  - web/src/lib/locales/cs.ts
  - web/src/lib/locales/da.ts
  - web/src/lib/locales/el.ts
  - web/src/lib/locales/es.ts
  - web/src/lib/locales/et.ts
  - web/src/lib/locales/eu.ts
  - web/src/lib/locales/fa.ts
  - web/src/lib/locales/fi.ts
  - web/src/lib/locales/fr.ts
  - web/src/lib/locales/gl.ts
  - web/src/lib/locales/he.ts
  - web/src/lib/locales/hi.ts
  - web/src/lib/locales/hr.ts
  - web/src/lib/locales/hu.ts
  - web/src/lib/locales/id.ts
  - web/src/lib/locales/is.ts
  - web/src/lib/locales/it.ts
  - web/src/lib/locales/ja.ts
  - web/src/lib/locales/ko.ts
  - web/src/lib/locales/lt.ts
  - web/src/lib/locales/lv.ts
  - web/src/lib/locales/ms.ts
  - web/src/lib/locales/nl.ts
  - web/src/lib/locales/no.ts
  - web/src/lib/locales/pl.ts
  - web/src/lib/locales/pt.ts
  - web/src/lib/locales/ro.ts
  - web/src/lib/locales/ru.ts
  - web/src/lib/locales/sk.ts
  - web/src/lib/locales/sl.ts
  - web/src/lib/locales/sr.ts
  - web/src/lib/locales/sv.ts
  - web/src/lib/locales/th.ts
  - web/src/lib/locales/tr.ts
  - web/src/lib/locales/uk.ts
  - web/src/lib/locales/vi.ts
  - web/src/lib/locales/zh.ts
  - web/src/lib/selectionTree.test.ts
  - web/src/lib/selectionTree.ts
  - web/src/pages/Containers.tree.dom.test.tsx
  - web/src/pages/Containers.tsx
findings:
  critical: 0
  warning: 4
  info: 4
  total: 8
status: issues_found
---

# Phase 3: Code Review Report

**Reviewed:** 2026-09-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 59
**Status:** issues_found

## Summary

Reviewed the phase 3 (Selection Trust & Controls) changes: the backend excludeCaches pipeline (migration v100, `SetExcludeCaches` owned setter, additive PATCH field, backup-time `anyRootExcludeCaches` union into `restic.Mode`), and the frontend selection-trust work (per-root preview counts, reviewable exclusions, reset selection, narrowing note, per-root CACHEDIR.TAG toggle, 42-locale i18n propagation).

The backend work is solid: migration numbering is append-only (v100 after v99), argv placement of `--exclude-caches` is correct (after the verb, before `--`, pinned by test), all container backup entry points funnel through `Service.Backup` where the union is compiled from the authoritative post-upsert re-read, boundary validation reuses `toContainerPath` containment (same discipline as `SetBackupPaths`), the refusal error is scrubbed through `failEnvelope`, and no user-controlled string reaches argv. The store round-trip, ON CONFLICT ownership, and wire nil-normalization are all correct and tested. Locale parity is complete (8 keys present in all 40 locale files plus en/de inline; `{n}` placeholders preserved; no em dashes in user text).

The findings below are all in the frontend save-queue orchestration around the new reset/caches features. None are blockers, but WR-01/WR-02/WR-04 all concern the reset flow silently losing or misrepresenting state, and WR-04 has a user-visible data-skipping consequence.

## Structural Findings (fallow)

No structural pre-pass was provided with this review.

## Warnings

### WR-01: A confirmed reset can be silently dropped when another tree toggle stacks behind it

**File:** `web/src/pages/Containers.tsx:933-946` (with `web/src/pages/Containers.tsx:967-980`)
**Issue:** `scheduleSave`'s in-flight branch overwrites the pending paths-class descriptor unconditionally (`pendingDescsRef.current.paths = desc`, line 939). A reset desc is therefore discarded if any later paths mutation lands while the reset is waiting for its drain. Sequence: toggle A goes in flight → user confirms the reset dialog → `scheduleSave(reset)` records the reset desc → user toggles B → `scheduleSave(toggleB)` overwrites the reset desc → the drain sends the live pre-reset mirror under `selectionSource: "tree"` instead of `{backupPaths: []}`. The confirmed destructive reset never reaches the server, no failure signal fires (the user sees a normal "Saved" toast for the toggle drain), and the remembered exclusions the confirm dialog promised to remove survive. The existing comment documents "latest-intent-wins" for a toggle stacked behind a reset, but the actual payload sent is the OLD explicit selection plus/minus the toggles — not auto-detection plus the new toggle — so even the sanctioned stacking case contradicts the reset contract. There is also no test covering any toggle-stacked-behind-reset path.
**Fix:** Make the reset desc sticky for the paths class: in `scheduleSave`, do not overwrite a pending `reset: true` desc with a non-reset desc (or drain the reset first and queue later toggles behind it). At minimum, when a reset desc is displaced, surface it (toast + shake on the reset control) instead of silently dropping it:

```ts
if (queueRef.current.inFlight) {
  queueRef.current.dirty = true;
  owedRef.current.add(desc.cls);
  if (desc.cls === "paths") {
    // A confirmed reset must never be displaced by a later toggle: the
    // toggle's effect is defined relative to the post-reset (auto) state.
    if (!pendingDescsRef.current.paths?.reset) pendingDescsRef.current.paths = desc;
  } else {
    pendingDescsRef.current.caches = desc;
  }
  return;
}
```

### WR-02: The post-reset refetch is not serialized with the save queue and can clobber a stacked mutation

**File:** `web/src/pages/Containers.tsx:1002` (with `web/src/pages/Containers.tsx:889-926`, `web/src/pages/Containers.tsx:1049-1052`)
**Issue:** On reset success, `setLoaded(false)` triggers the load effect's `getContainerMounts` GET, while the `finally` block simultaneously starts the next drain for any mutation stacked during the reset PATCH's flight. The refetch's `applyMirror`/`applyCaches`/`setCustom` application is not coordinated with that drain: if the GET response reflects pre-PATCH server state (the PATCH was issued first in program order, but connection-level reordering on separate keep-alive connections can deliver it first), the served state overwrites the stacked toggle/flip locally. The user's click is then silently lost (UI shows the stale value; the server persisted the click; the next save of that class entrenches whichever side the UI holds). Same window applies to a CACHEDIR flip stacked behind the reset. The test suite never exercises this because every reset test resolves the PATCH with nothing stacked behind it.
**Fix:** Route the reload through the queue: add a `reload` owed class (or a `reloading` flag) that the `finally` chain performs only when `owedRef` is otherwise empty, so the GET is issued after all stacked drains settle; or guard the load-apply with an epoch counter that skips applying the response when the queue went non-idle after the fetch started.

### WR-03: Exclusions-disclosure DOM id can collide between roots

**File:** `web/src/components/SelectionTree.tsx:422`
**Issue:** `exclListId` sanitizes the root path with `spec.path.replace(/[^a-zA-Z0-9]/g, "-")`. Two depth-0 roots whose paths differ only in non-ASCII-alphanumeric characters produce the same id: `/mnt/user/app-data` vs `/mnt/user/app_data` both yield `«useId»excl--mnt-user-app-data`, and any two CJK-named segments (`/mnt/user/東京` vs `/mnt/user/大阪`) collapse to identical ids. When both disclosures are open, the `ul` elements carry duplicate `id`s (invalid HTML) and both buttons' `aria-controls` resolve to the first list, breaking the button-to-list association for screen readers. This is realistic for a UI shipped with 42 locales.
**Fix:** Key the id off something collision-free — the root's position (`${exclIdPrefix}excl-${spec.posInSet}`), or a small per-row child component calling `useId()` itself, or `encodeURIComponent(spec.path)` instead of character stripping.

### WR-04: Reset leaves orphaned excludeCaches keys with no UI to turn them off (and the dom test masks this)

**File:** `web/src/pages/Containers.tsx:1196-1220` (with `internal/api/service.go:10164-10170`, `web/src/pages/Containers.tree.dom.test.tsx:597`)
**Issue:** The reset PATCH sends exactly `{backupPaths: []}` — `exclude_caches` is untouched, so a `true` entry keyed by a root that disappears in the reset (a standalone custom path row; custom rows are gone post-reset since the selection becomes auto-detected) survives invisibly. `anyRootExcludeCaches` runs over the stored map regardless of whether the root is still rendered (the literal A1 reading), so `--exclude-caches` keeps firing for every future backup while the only control bound to that key — the per-root switch in `SelectionTree` — no longer exists anywhere in the UI. The user cannot turn it off without re-adding the exact path as a custom row. The confirm copy (`folders.resetConfirm`, `i18n.ts:755-756`) names the exclusions consequence but is silent about cache toggles. The dom tests mask the behavior: the post-reset fixture at `Containers.tree.dom.test.tsx:597` resets `excludeCaches` to `{}` even though the real server keeps the stored map.
**Fix:** Compose the reset body with `excludeCaches: {}` as well (the empty-selection guard only gates on `backupPaths` + `selectionSource`, so adding the cleared map to the reset body keeps the sanctioned shape), or render orphaned stored keys under a fallback row; either way, update the reset confirm copy and the post-reset test fixture to model the real server state.

## Info

### IN-01: `removeCustomPath` bypasses the empty-selection guard, leaving UI/server divergence on refusal

**File:** `web/src/pages/Containers.tsx:1179-1194`
**Issue:** The D-04 client-side guard (`next.includes.size === 0` → blocked + shake) exists only in `onToggle`. Removing the last custom row of a container with no selected mounts sends a tree-sourced empty save; the server refuses with `errEmptySelection`, and because the remove is `structural` (no revert), the row is already gone locally while the server keeps it — divergence until the next reload, with only the refusal toast as a signal. Pre-existing phase-2 shape, but the functions were touched in this phase and the updated `folders.hint` copy now asserts "Unticking everything is blocked", which this path contradicts.
**Fix:** Apply the same zero-include check in `removeCustomPath` (block + shake + warn line) instead of relying on the server refusal.

### IN-02: Composed PATCH is not transactional across classes; a caches failure reverts paths that already persisted

**File:** `internal/api/handlers.go:1153-1185` (with `web/src/pages/Containers.tsx:1023-1032`)
**Issue:** When one drain carries both `backupPaths` and `excludeCaches` and only the caches validation fails (e.g. the host mount config changed since load), the server has already persisted the paths save, but the client treats the whole attempt as failed and `revertFrom(pathsDesc)` restores the pre-toggle mirror — the UI now disagrees with persisted server state. Narrow (UI keys are pre-validated at serve time) and rooted in the pre-existing one-setter-per-field PATCH shape, but the two-class composition makes it newly reachable.
**Fix:** Either order the fields so cheap validation (excludeCaches containment) runs before any store write in the handler, or note the partial-application semantics at the client revert site so a future fix doesn't have to rediscover them.

### IN-03: Finally-chained drains read `lastBackup` and `t` from a stale closure

**File:** `web/src/pages/Containers.tsx:994,1012` (chained at `web/src/pages/Containers.tsx:1051`)
**Issue:** The `finally` block chains `void attemptSave()` using the `attemptSave` binding of the render in which the completing attempt was created, so a drain started from the chain reads the `lastBackup` prop and `t` captured at that older render. A containers refresh or locale switch during an in-flight save can make the narrowing gate consult a stale `lastBackup` (wrongly suppressing or firing the note). This is exactly the situation the house `xRef.current = x` ref-mirroring convention exists for; the queue otherwise reads live state through refs.
**Fix:** Mirror `lastBackup` (and optionally `t`) into refs read inside `attemptSave`, matching the `mirrorRef`/`cachesRef` pattern.

### IN-04: The 64-entry excludeCaches cap can reject legitimate wide maps with no UI guard or direct test

**File:** `internal/api/service.go:10134-10150`
**Issue:** `maxExcludeCachesEntries = 64` counts all keys, including explicit `false` values the UI round-trips. A container with more than 64 bind mounts where the user toggles the switch on many roots gets a whole-map rejection (toast + revert) with nothing in the UI counting entries. The cap also has no direct test (`TestPatchContainerExcludeCaches` covers containment, decode, and atomicity, not the limit).
**Fix:** Count only `true` values toward the cap (the union is all that reaches argv), or raise the limit above realistic mount counts; add a boundary test at 64/65 entries either way.

---

_Reviewed: 2026-09-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
