---
phase: 03-selection-trust-controls
plan: "03"
subsystem: ui
tags: [react, selection-tree, serialized-queue, cachedir-tag, i18n, phase-gate]

requires:
  - phase: 03-selection-trust-controls
    provides: "plan 01 — targets.exclude_caches wire field + wholesale-replace PATCH semantics, mounts-served excludeCaches object; plan 02 — per-root preview/exclusions sub-rows and the presentation-wrapper sub-row pattern"
  - phase: 02-selection-tree
    provides: "the one-deep serialized PATCH queue (attemptSave/revertFrom), the (includes, exclusions) host mirror, classifyNode"
  - phase: 01-selection-foundation
    provides: "the strictly selectionSource:'tree'-gated empty-selection guard the reset body must pass by omission"
provides:
  - Reset selection exit to auto-detection (D-05, INTEG-04) — confirmed, non-optimistic, sent as exactly {backupPaths: []} with NO selectionSource through the serialized queue, refetch-on-ok re-baselining mirror/custom/caches/lastSavedCount together
  - Narrowing note (D-02, SELECT-03) — event-driven role=status warn line when an acknowledged save's attempted include count drops below the last-saved count, gated on container.lastBackup !== null; transient editor-session state cleared on section close
  - Per-root CACHEDIR.TAG toggle (D-06, RESTIC-01) — hideLabel Toggle + caller-drawn 12px-register label + InfoBubble scope tooltip under every root (mounts and standalone customs), riding the same serialized queue as a second owed class
  - Two-class composed-body drain — setContainerTargets(name, {backupPaths?, selectionSource?, excludeCaches?}) sends exactly the owed classes per attempt; one fetchJSON per drain, never two concurrent container PATCHes (T-03-07), proven by a max-concurrency counter in the dom harness
  - i18n keys folders.resetSelection, folders.resetConfirm, folders.narrowedNote, folders.cachedirToggle, folders.cachedirScope + rewrites of folders.emptySelectionBlocked and folders.hint across all 42 locale tables
  - rebuilt, committed web/dist closing the phase gate
affects: [verify-work UAT (phase 3 surfaces complete), /gsd-ship (windows ledger entries 3-5 open)]

actuals:
  tokens: 50800   # chars/4 over the realized source diff (1177 insertions, dist excluded)
  tasks: 3
  commits: 6

tech-stack:
  added: []
  patterns:
    - "Discriminated-union save descriptors (cls: paths|caches) with latest-descriptor-wins per class — the queue composes one body from the owed classes and keeps only each class's latest failure-revert recipe"
    - "Per-class success feedback: paths saves toast Saved, caches-only saves stay quiet (the switch's own optimistic state is the feedback)"
    - "Sanctioned caller-drawn-label Toggle (hideLabel) when the internal text-sm caption would break a denser register — accessible name still via aria-label"

key-files:
  created: []
  modified:
    - web/src/pages/Containers.tsx
    - web/src/components/SelectionTree.tsx
    - web/src/lib/api.ts
    - web/src/lib/i18n.ts
    - web/src/lib/locales/*.ts (40 files)
    - web/src/pages/Containers.tree.dom.test.tsx
    - web/src/components/SelectionTree.dom.test.tsx
    - web/src/components/SelectionTree.keyboard.dom.test.tsx
    - web/dist

key-decisions:
  - "Reset rides the SAME one-deep queue as toggles (never fire-and-forget): its descriptor carries reset:true and NO source, so the drain sends exactly {backupPaths: []} — the strictly tree-source-gated Phase 1 guard passes it by omission, making the confirmed reset the one sanctioned exit to auto-detection (T-03-08)"
  - "Reset is non-optimistic: nothing is mutated locally until ok, so failure is toast + control shake with no mirror revert, and success replaces everything via setLoaded(false) refetch (remembered exclusions and custom rows gone, lastSavedCount re-initialized from the served state)"
  - "The narrowing gate compares the ACKNOWLEDGED attempt's include count against lastSavedCount (not a pre-mutation count), so a burst collapsed by the queue nets correctly, and it is gated on lastBackup — with no prior snapshot there is no captured scope to communicate about"
  - "CACHEDIR flips are a second owed class in the one queue rather than a parallel stream: a flip stacked behind an in-flight selection save drains as ONE further fetch carrying only excludeCaches, and the dom harness pins maxConcurrentPatches === 1 across the mixed timeline"
  - "Caches failure revert restores the flipped key's stored value only when the live map still equals the attempted next — a newer flip on the same key stacked behind the failed save survives (the Pitfall 5 newer-intent-survives discipline, one class over)"
  - "setBackupPaths was deleted, not kept: grep showed FoldersEditor was its only caller, so the queue's drain now funnels through the single composed-body setContainerTargets helper (ContainerTargetsBody mirrors the Go PATCH shape field-for-field)"

patterns-established:
  - "Generalized serialized queue: owedRef (Set of classes) + pendingDescsRef (latest desc per class) + one composed body per drain — extending the one-deep queue to N mutation classes without ever widening the fetch fanout"
  - "Derived view in test mocks: the retired setBackupPaths wire shape is projected out of the composed body inside the setContainerTargets mock, keeping Phase 2 assertions meaningful against the single new endpoint"

requirements-completed: [INTEG-04, SELECT-03, RESTIC-01]

coverage:
  - id: D1
    description: "Reset selection (D-05, INTEG-04): fail-tone confirm naming both consequences, serialized {backupPaths: []} with no selectionSource, refetch-on-ok re-rendering auto-detected selection with remembered exclusions gone, lastSavedCount re-initialized"
    requirement: INTEG-04
    verification:
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#reset selection (D-05, INTEG-04)
        status: pass
    human_judgment: false
  - id: D2
    description: "Narrowing note (D-02, SELECT-03): role=status text-statusWarn under the tree after a successful narrowing save gated on lastBackup; never on widening, never when lastBackup is null, never for the reset save; transient per editor session"
    requirement: SELECT-03
    verification:
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#narrowing note (D-02, SELECT-03)
        status: pass
    human_judgment: false
  - id: D3
    description: "Guard and hint copy: folders.emptySelectionBlocked references Reset selection, folders.hint no longer claims unticking everything reverts to the automatic default, reset button row with confirm dialog"
    requirement: INTEG-04
    verification:
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#empty-selection guard copy + reset button
        status: pass
      - kind: unit
        ref: web/src/lib/i18n.parity.test.ts + i18n.orphans.test.ts + i18n.quality.test.ts
        status: pass
    human_judgment: false
  - id: D4
    description: "Per-root CACHEDIR.TAG toggle (D-06, RESTIC-01): switch + caller-drawn label + scope InfoBubble under every root incl. customs, disabled on unreachable mounts, optimistic flip, quiet success, full-map PATCH never overlapping a backupPaths PATCH (T-03-07)"
    requirement: RESTIC-01
    verification:
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#per-root CACHEDIR.TAG toggle (D-06, RESTIC-01, T-03-07)
        status: pass
    human_judgment: false
  - id: D5
    description: "Failure semantics: failed caches PATCH reverts to the stored value (newer same-key flip survives), verbatim server error in a fail toast, shake on the switch row; failed reset shakes the control with no partial mutation"
    requirement: RESTIC-01
    verification:
      - kind: automated_ui
        ref: web/src/pages/Containers.tree.dom.test.tsx#failed caches PATCH / failed reset cases
        status: pass
    human_judgment: false
  - id: D6
    description: "i18n completeness: 5 new keys + 2 text changes across all 42 locale tables, placeholders intact, no em dashes, non-Latin locales carry real translations"
    requirement: SELECT-03
    verification:
      - kind: unit
        ref: web/src/lib/i18n.parity.test.ts + i18n.orphans.test.ts + i18n.quality.test.ts (all green over 42 tables)
        status: pass
    human_judgment: false
  - id: D7
    description: "Phase gate: full web suite, tsc, vite build, eslint 0 errors, rebuilt web/dist committed, full Go chain green with the embedded SPA current"
    requirement: INTEG-04
    verification:
      - kind: automated_ui
        ref: "web: 91 files / 2214 tests passed; node tsc --noEmit clean; vite build clean; eslint 0 errors (2 pre-existing warn-only items tracked)"
        status: pass
      - kind: command
        ref: "go build ./... && go vet ./... && gofmt -l . (empty) && go test ./... all exit 0"
        status: pass
    human_judgment: false

status: complete
completed: 2026-09-10
duration: 40m
---

# Phase 3 Plan 03: Selection Interactions — Reset, Narrowing Note, CACHEDIR Toggle Summary

**One-liner:** Reset-selection exit to auto-detection, a lastBackup-gated narrowing note, and the per-root CACHEDIR.TAG switch — all three riding the one-deep serialized PATCH queue, now generalized to compose a single body from two owed mutation classes.

## What Was Built

### Task 1 — Reset selection and narrowing note (TDD)

- `SaveDesc` gained `source?: "tree"` and `reset?: true`; the reset descriptor is the one that carries NO source, so its drain sends exactly `{backupPaths: []}` and the strictly tree-source-gated Phase 1 guard passes it by omission (the sanctioned exit to auto-detection, T-03-08).
- Reset button under the tree (shake-keyed, busy-disabled) behind a `useConfirm` fail-tone dialog naming both consequences; success is non-optimistic — `setLoaded(false)` refetch replaces mirror, custom rows, caches map and the `lastSavedCountRef` baseline together; failure is toast + control shake with zero local mutation.
- Narrowing note: `lastSavedCountRef` starts at the served include count and updates to each acknowledged attempt's attempted count; `narrowed` fires only when attempted < last-saved AND `container.lastBackup !== null` (threaded from `ContainerRow`); cleared on section close; never compares snapshot contents (SELECT-06 stays deferred).
- Guard/hint copy: `folders.emptySelectionBlocked` now teaches the reset exit; `folders.hint` no longer claims unticking everything reverts to the automatic default. Propagated to all 42 locale tables (exact per-locale tail swap + CJK-aware separators).

### Task 2 — Per-root CACHEDIR.TAG toggle (TDD)

- `api.ts`: `ContainerMountsResponse.excludeCaches?: Record<string, boolean>` (server always serves an object) and the new `setContainerTargets(name, body)` composed-body PATCH helper; `setBackupPaths` deleted (grep confirmed the editor was its only caller).
- Queue generalization: `owedRef: Set<"paths"|"caches">` + `pendingDescsRef` holding the LATEST descriptor per class; `attemptSave()` composes ONE `ContainerTargetsBody` carrying exactly the owed classes through a single `fetchJSON`. A caches flip stacked behind an in-flight selection save drains as one further fetch — `maxConcurrentPatches === 1` is pinned by the dom harness (T-03-07).
- `SelectionTree.tsx`: presentation-wrapped sub-row under EVERY root (mounts regardless of expand state, standalone customs alike): `Toggle` with `hideLabel` (the sanctioned caller-drawn-label shape — the internal text-sm caption would break the tree's 12px register), caller-drawn label in `text-xs text-carbon-textSub`, `InfoBubble` with the item-wide scope tooltip; disabled on unreachable mounts and while the root's save is in flight.
- Failure revert per class: caches restores the flipped key's stored value ONLY when the live map still equals the attempted next (a newer same-key flip survives); paths keeps the set-difference `revertFrom`; reset shakes the control with no mirror revert. Success feedback is per-class: paths toasts Saved, caches-only stays quiet.

### Task 3 — Phase gate

- Full web suite: 91 files / 2214 tests passed; `tsc --noEmit` clean; `vite build` clean; eslint 0 errors (the 2 pre-existing warn-only `exhaustive-deps` items in ActivityLog.tsx / Sidebar.tsx remain tracked, untouched files).
- Rebuilt `web/dist` committed (index.html is the only tracked dist file per the pre-Vite placeholder convention; hashed assets stay ignored).
- Go chain green: `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test ./...` all exit 0 with the rebuilt SPA embedded.

## TDD Gate Compliance

Both implementation tasks followed RED → GREEN with separate commits:

- Task 1: RED `83d51fd1` (9 failing dom tests: reset confirm/body/refetch/failure, narrowing gate truth table, copy changes) → GREEN `ad7b1e03` (all pass; suite re-run green).
- Task 2: RED `3961e672` (4 failing dom tests: switch rendering/disabled, caches-only body, stacked no-overlap timeline, failure revert+shake) → GREEN `1f074aa4` (all pass).
- One RED calibration fix during Task 2 GREEN: the reset-failure test expected `aria-checked "true"` where the Phase 2 classifier correctly reports `"mixed"` for a selected mount with a remembered exclusion (D-01) — the expectation was miscalibrated, not the implementation; fixed the test with an explanatory comment.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] SelectionTree sibling test harnesses still mocked the deleted setBackupPaths**
- **Found during:** Task 2 GREEN
- **Issue:** `api.ts` deleted `setBackupPaths`, which type-broke and silently mis-mocked `SelectionTree.dom.test.tsx` and `SelectionTree.keyboard.dom.test.tsx` (and the direct `<SelectionTree>` renders lacked the now-required `excludeCaches`/`onToggleCaches` props).
- **Fix:** Migrated both mocks to `setContainerTargets`, projecting the same flat `{name, paths, opts}` view the Phase 2 assertions pin out of the composed body; added the required props to every direct render (SelectionTree.dom.test.tsx TreeHarness, Containers.tree.dom.test.tsx cap test).
- **Files modified:** web/src/components/SelectionTree.dom.test.tsx, web/src/components/SelectionTree.keyboard.dom.test.tsx, web/src/pages/Containers.tree.dom.test.tsx
- **Commit:** 1f074aa4

**2. [Rule 1 - Bug] RED tests used jest-dom matchers this repo does not ship**
- **Found during:** Task 2 GREEN
- **Issue:** The Task 2 RED tests asserted via `toBeDisabled()`/`toBeEnabled()`; the repo deliberately has no `@testing-library/jest-dom` (see ColorPickerPopover.dom.test.tsx's header).
- **Fix:** Plain `HTMLButtonElement.disabled` property checks, the documented house style.
- **Files modified:** web/src/pages/Containers.tree.dom.test.tsx
- **Commit:** 1f074aa4

**3. [Rule 1 - Bug] Stale D-04 copy assertions in SelectionTree.dom.test.tsx**
- **Found during:** Task 2 GREEN (full-suite run)
- **Issue:** Task 1's `folders.emptySelectionBlocked` rewrite appended the reset sentence, but two assertions in the sibling suite still matched the old two-sentence text — missed because Task 1's verification ran the Containers tree suite, not this file.
- **Fix:** Updated both assertions to the current copy.
- **Files modified:** web/src/components/SelectionTree.dom.test.tsx
- **Commit:** 1f074aa4

**4. [Rule 1 - Bug] Shake asserted on the classless presentation wrapper**
- **Found during:** Task 2 GREEN
- **Issue:** The failure test asserted `glim-shake` on the sub-row's presentation wrapper; the shake rides the INNER flex div (the wrapper carries no class by design).
- **Fix:** Retargeted the assertion to `sub.firstElementChild`.
- **Files modified:** web/src/pages/Containers.tree.dom.test.tsx
- **Commit:** 1f074aa4

**5. [Rule 1 - Bug] TypeScript discriminant correlation in scheduleSave/revertFrom**
- **Found during:** Task 2 GREEN (tsc)
- **Issue:** `pendingDescsRef.current[desc.cls] = desc` loses the cls-to-shape correlation; `revertFrom` accepted the whole `SaveDesc` union while reading `sent`/`pre`.
- **Fix:** Discriminant-narrowed writes in `scheduleSave`; `revertFrom` signature narrowed to `Extract<SaveDesc, { cls: "paths" }>`.
- **Files modified:** web/src/pages/Containers.tsx
- **Commit:** 1f074aa4

### Platform Notes (Windows box; Phase 2 precedent)

- `npm run` / `npx` fail on this box — every script ran by invoking the local binaries directly (`node node_modules/vitest/dist/cli.js run ...`, `node node_modules/typescript/bin/tsc --noEmit`, `node node_modules/vite/bin/vite.js build`, `node node_modules/eslint/bin/eslint.js .`), which executes the exact same package-local code the scripts point at.
- `npm ci` (Task 3 action step 1) was skipped: no dependency changes in this plan and `web/package-lock.json` is untouched — `node_modules` is current, verified by the clean tsc/build/eslint/vitest runs.
- `golangci-lint` and `just` are not on PATH on this box; the CI Lint job remains the gate (Phase 2 precedent).

## Known Stubs

None — every surface ships wired: reset, narrowing note, and the CACHEDIR toggle all persist through the real PATCH endpoint, with i18n complete across 42 tables.

## Deferred Issues

- **Stacked-descriptor failure-revert window** (pre-existing plan 02 behavior, carried over by the two-class generalization): two same-class mutations stacked behind one failed drain revert only the latest's delta. Logged in `.planning/phases/03-selection-trust-controls/deferred-items.md` — needs a GSD decision (per-mutation delta journaling would be an architectural change to a shipped seam).
- Broken-windows ledger entries 3-5 (this plan's test-harness deviations) remain `open` in `.planning/WINDOWS.md` for `/gsd-ship` visibility.

## Threat Model Disposition

- **T-03-07 (concurrent PATCH lost-update): mitigated** — the generalized one-deep queue composes one body per drain; the dom harness pins `maxConcurrentPatches === 1` across a mixed paths+caches timeline.
- **T-03-08 (silent deselect into auto-detection): mitigated** — reset is explicit, confirmed with both consequences named, sent as the no-source body the guard passes by design; the guard message teaches it; narrowing is announced.
- **T-03-09 (CSRF on PATCH fields): accepted** — unchanged endpoint, existing CSRF middleware.
- **T-03-10 (PATCH failure text): accepted** — server errors arrive scrubbed; SPA shows them verbatim per house rule.
- **T-03-SC (package installs): accepted/avoided** — zero npm installs this plan.

No new security-relevant surface beyond the plan's threat model was introduced (no new routes; the composed body rides the existing PATCH endpoint).

## Self-Check: PASSED

- All 5 plan commits present on docker-folders: 83d51fd1, ad7b1e03, 3961e672, 1f074aa4, 27a29a55 (verified via git log).
- Acceptance greps: api.ts excludeCaches = 3 (≥2); SelectionTree.tsx cachedirToggle = 2 (≥1); cachedirScope across web/src/lib = 41 files (i18n.ts + 40 locales).
- Full web suite 2214/2214, Go chain exit 0, `git status` clean of tracked changes after the dist commit.
