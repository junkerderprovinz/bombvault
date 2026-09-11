---
phase: 04-file-sets-parity
plan: 02
subsystem: backup
tags: [file-sets, selection, patch-boundary, restore-guard, http-api, typescript, tdd]

# Dependency graph
requires:
  - phase: 04-file-sets-parity
    plan: 01
    provides: v101 selected_paths column + owned store setter, fileSetPositionals compile helper, NormalizeSelection/SplitExclusion/isStrictDescendant flat-set semantics
  - phase: 01-selection-engine-restore-safety
    provides: mapRestorePaths two-pass longest-prefix mapping, chosenSnapshot, coded empty-selection envelope precedent
provides:
  - PATCH /api/files/sets/{id} selectedPaths boundary: *[]string pointer semantics (absent = untouched, [] = coded refusal, list = atomic validated overwrite), 64-entry cap, per-entry containment against the set's resolved root, NormalizeSelection before store
  - errFileSetEmptySelection sentinel + codedFailEnvelope(err, "empty-selection") routing; a refused deselect leaves the prior selection structurally untouched
  - Path-change clear with clear-wins precedence over same-request entries (Pitfall 2 layer 1 / A3), compared on resolved roots
  - FileSetView.SelectedPaths (omitempty NULL legacy switch) served back verbatim + hand-synced TS mirror (FileSetView + patchFileSet)
  - D-08 restore selection guard in prepareRestoreFileSet: compiled via fileSetPositionals, mapped via mapRestorePaths (2nd call site), synchronous "nothing to restore" abort before destructive work, scoped to the in-place route
affects: [04-03 Files page tree editor (the write/read surface it calls), 04-04 restore-side parity leftovers]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 8814    # 35255 chars of realized internal/ + web/src diff / 4 (plan estimated 40000)
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []   # zero installs — Go stdlib + existing selection/paths helpers only
  patterns:
    - "Pointer PATCH field with three-state semantics: absent = untouched, empty list = coded refusal, list = atomic validated overwrite (DisallowUnknownFields forces the explicit declaration)"
    - "Guard/backup symmetry: the restore guard compiles through the SAME helper the backup ran (fileSetPositionals), so the two can never disagree about what a set selects"
    - "Clear-wins precedence as structure: a path change and a selection list in one request are resolved by a switch order, never by validation order"

key-files:
  created:
    - none (parity plan — tests appended to existing files; no new source files)
  modified:
    - internal/api/handlers_test.go (TestPatchFileSetSelectedPaths, TestFileSetEmptySelection, TestFileSetSelectedPathsViewShape, TestRestoreFileSetSelectionGuard + helpers patchFileSet/newDocsSet/storedFileSetSelection; fileSetRow mirror field)
    - internal/api/service.go (SetFileSetSelectedPaths + maxFileSetSelectedPaths + errFileSetEmptySelection; FileSetView.SelectedPaths + ListFileSetViews population; D-08 guard in prepareRestoreFileSet)
    - internal/api/handlers.go (handlePatchFileSet: selectedPaths pointer field, oldPath capture, clear/selection switch, coded empty-selection envelope)
    - web/src/lib/api.ts (FileSetView.selectedPaths + patchFileSet.selectedPaths, doc-commented hand-synced mirror)

key-decisions:
  - "Boundary containment primitive is isStrictDescendant (segment-aligned, equality allowed), not paths.Within: entries live in mount-root space, which is a drive path on the Windows dev box where Within (leading-/ requirement) would wrongly refuse every valid entry; the compile-side re-anchor uses the same primitive, so write-side and read-side agree on one definition of 'under the root'"
  - "D-06 refusal is PRESENCE-gated with no selectionSource carrier: the file-set PATCH field did not exist before this plan, so there is no legacy client to protect — zero includes after normalization is always refused (exclusions-only included), and the message orients to Remove set because the tree cannot express 'back up nothing' for a set"
  - "Clear-wins precedence is structural (switch order: pathChanged clears, else entries are saved), so entries in a path-changing request are never validated or stored — honoring both would silently rewrite the selection's meaning under the new anchor"
  - "mapRestorePaths' skipped list is deliberately unreported on this route (minimum D-08): the restore hands restic the whole-snapshot path, so there is no per-path skip channel; the guard's only job is the empty-intersection abort"

patterns-established:
  - "File-set PATCH selection contract: absent key untouched, [] refused with code empty-selection, list = atomic validated normalized overwrite"
  - "Restore-side safety reuses the backup-time compile, never a second interpretation of the stored selection"

requirements-completed: [INTEG-02]  # copied verbatim from plan frontmatter; INTEG-02 is shared by plans 04-01..04-04, so REQUIREMENTS.md marking stays gate-blocked until 04-03/04-04 summaries exist (same note as 04-01)

# Coverage metadata (#1602) — one entry per shipped deliverable.
coverage:
  - id: D1
    description: "PATCH selectedPaths pointer boundary: absent = untouched (never clears), declared explicitly for DisallowUnknownFields"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestPatchFileSetSelectedPaths/an_absent_selectedPaths_key_leaves_the_stored_selection_untouched"
        status: pass
    human_judgment: false
  - id: D2
    description: "Per-entry validation before ANY store write (atomic rejection incl. sibling-prefix trap), 64-entry cap, normalized canonical storage, view serve-back"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestPatchFileSetSelectedPaths (atomic rejection / 64 cap / canonical form subtests)"
        status: pass
    human_judgment: false
  - id: D3
    description: "D-06: zero-includes selection refused with code empty-selection + Remove-set message; prior selection byte-identical after a refusal"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestFileSetEmptySelection"
        status: pass
    human_judgment: false
  - id: D4
    description: "Path-change clear (to SQL NULL legacy state) with clear-wins precedence over same-request entries; comparison on resolved roots"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestPatchFileSetSelectedPaths/changing_the_set_path_clears_the_stored_selection"
        status: pass
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestPatchFileSetSelectedPaths/a_path_change_wins_over_entries_in_the_same_request"
        status: pass
    human_judgment: false
  - id: D5
    description: "Wire mirror: NULL set omits selectedPaths entirely; edited set serves the stored form back; TS FileSetView + patchFileSet mirror both spots"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestFileSetSelectedPathsViewShape"
        status: pass
      - kind: other
        ref: "cd web && npx tsc --noEmit (clean); grep -c selectedPaths web/src/lib/api.ts == 2"
        status: pass
    human_judgment: false
  - id: D6
    description: "D-08 restore guard: in-place restore maps the compiled selection against the snapshot's Paths and aborts synchronously with 'nothing to restore' on empty intersection; mono-path legacy snapshots still restore; to-folder route and path-less snapshots untouched; pre-existing restore tests green unmodified"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestRestoreFileSetSelectionGuard"
        status: pass
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestRestoreFileSetInPlaceConfirmed + TestRestoreFileSetToFolder (pre-existing pins, diff vs RED commit empty)"
        status: pass
      - kind: other
        ref: "grep -c 'mapRestorePaths(' internal/api/service.go == 2 (baseline 1, grew by exactly 1)"
        status: pass
    human_judgment: false

# Metrics
duration: 40min
completed: 2026-09-11
status: complete
---

# Phase 4 Plan 2: SelectedPaths Boundary and Restore Guard Summary

**A file set's tree selection is now writable through PATCH — validated per entry against the resolved root before any store write, normalized, capped, refused when empty (D-06) — and an in-place restore whose snapshot contains none of what the set selects now aborts synchronously before tearing anything down (D-08).**

## Performance

- **Duration:** 40 min
- **Started:** 2026-09-11T02:24:37Z
- **Completed:** 2026-09-11T03:04:37Z
- **Tasks:** 2 (each landed as a RED test commit + GREEN implementation commit)
- **Files modified:** 4 (3 Go + 1 TS; no new files)

## Accomplishments
- PATCH `selectedPaths` is a `*[]string` pointer: absent = untouched (an ordinary name/path edit can never clear a selection), `[]` decodes non-nil and is refused (D-06), a list is an atomic validated overwrite; the field is declared explicitly because `decodeBody` runs DisallowUnknownFields
- `Service.SetFileSetSelectedPaths`: 64-entry cap (the SetExcludeCaches ceiling), per-entry trim/split/clean/containment (equality or `isStrictDescendant` under the set's resolved root, segment-aligned, sibling-prefix safe) BEFORE any store write, then shared `NormalizeSelection`; entries stay in mount-root absolute space, never container-translated
- D-06: zero-includes selections (exclusions-only and `[]` alike) refused with `errFileSetEmptySelection`, routed to the machine-routable `empty-selection` envelope code; the message orients to Remove set; refusals leave the prior selection byte-identical
- Path-change clear with clear-wins precedence over same-request entries (Pitfall 2 layer 1 / A3), compared on RESOLVED roots so re-sending the identical path is not a change; the clear stores SQL NULL (the legacy switch), never `[]`
- `FileSetView.SelectedPaths` (omitempty) served back verbatim by ListFileSetViews; hand-synced TS mirror in `FileSetView` + `patchFileSet` with the absent/empty semantics documented; `tsc --noEmit` clean
- D-08: `prepareRestoreFileSet` compiles the set's selection through `fileSetPositionals` (the backup's own compile) and maps it against the chosen snapshot's Paths via `mapRestorePaths` (exactly one new call site); empty intersection aborts synchronously with "nothing to restore for this set from this snapshot" — zero restic calls, nothing torn down

## Task Commits

Each task followed the full RED → GREEN TDD cycle with separate commits:

1. **Task 1 RED: failing selectedPaths boundary + empty-selection tests** - `87e1346c` (test)
2. **Task 1 GREEN: PATCH boundary, D-06 refusal, path-change clear, view mirror** - `ac01ffa0` (feat)
3. **Task 2 RED: failing restore selection guard test** - `c4c495bb` (test)
4. **Task 2 GREEN: D-08 guard in prepareRestoreFileSet** - `77ddb495` (feat)

## Files Created/Modified
- `internal/api/handlers_test.go` — 4 new test functions (11+4 subtests) + `patchFileSet`/`newDocsSet`/`storedFileSetSelection` helpers + `fileSetRow` mirror field
- `internal/api/service.go` — `SetFileSetSelectedPaths`, `maxFileSetSelectedPaths`, `errFileSetEmptySelection`, view field + population, D-08 guard
- `internal/api/handlers.go` — PATCH body field, oldPath capture, clear/selection switch, coded envelope branch
- `web/src/lib/api.ts` — `selectedPaths` in `FileSetView` + `patchFileSet`, both doc-commented

## Decisions Made
- Boundary containment uses `isStrictDescendant` (the compile's own primitive), not `paths.Within` — see Deviations-adjacent rationale: entries are mount-root-space paths that are drive paths on non-POSIX dev boxes, where `Within`'s leading-"/" requirement would refuse every valid entry; one primitive across write-side validation and read-side re-anchor
- D-06 is presence-gated with no `selectionSource` carrier — the file-set PATCH field did not exist before this plan, so there is no legacy-client shape to protect (the container guard's source gate has no analog here, per RESEARCH Pitfall 3's prohibition on inventing one)
- Clear-wins precedence is a structural switch, not validation order: a path change plus entries in one request stores NULL and never validates the entries
- `skipped` from `mapRestorePaths` is deliberately unreported (minimum D-08) — the whole-snapshot route has no per-path skip channel

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan-internal conflict: the guard's implied scope contradicted the plan's own acceptance criteria**
- **Found during:** Task 2, RED-phase design
- **Issue:** The action text implies mapping the restore paths for the restore; applied beyond the in-place route it breaks two of the plan's own must-pass tests: `TestRestoreFileSetToFolder` (cross-root snapshot Paths, NULL-compatible selection ⇒ empty mapping ⇒ abort, but the test expects success) and `TestRestoreFileSetInPlaceConfirmed` (snapshot with NO Paths ⇒ empty mapping ⇒ abort, test expects success). RESEARCH Open Question 1's adopted resolution already says to-folder whole-tree semantics stay deliberately untouched.
- **Fix:** The guard is scoped to `plan.inPlace != ""` and additionally requires `len(chosen.Paths) > 0` (path-less snapshots have nothing to map against). Both boundaries carry why-comments at the site and are pinned by `TestRestoreFileSetSelectionGuard`'s to-folder subtest plus the untouched pre-existing pins (diff vs the RED commit is empty).
- **Files modified:** internal/api/service.go
- **Verification:** full `internal/api` package green (55s run), including all six pre-existing file-set restore tests unmodified
- **Committed in:** 77ddb495

---

**Total deviations:** 1 auto-fixed (Rule 1, plan-internal inconsistency resolved in favor of the behavior spec / acceptance criteria — the same class of resolution as 04-01's deviation 1).
**Impact on plan:** None on scope; the guard lands exactly on the must_haves truth ("restore guard aborts pre-teardown, legacy mono-path snapshot still restores") — the literal broader scope would have violated it.

## Issues Encountered
- None blocking. `npx` needed `node` added to PATH on this box (Git Bash session); no repo impact.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Ready for 04-03: the view serves `selectedPaths` back (with the NULL omission the tree editor reads as "never edited"), and the PATCH accepts the full-list save the editor sends — including the coded `empty-selection` refusal the editor must handle by keeping its local state
- Ready for 04-04: the D-08 guard and its mapping are in place; any restore-side tree work starts from a route whose destructive path already refuses unmappable snapshots
- `TestBackupFileSet`/`TestBackupFileSetSelectedPaths` (plan-01 pins) and all pre-phase file-set restore tests remain green and untouched

---
*Phase: 04-file-sets-parity*
*Completed: 2026-09-11*

## Self-Check: PASSED

All 4 modified files exist on disk and compile (`go build ./...`, `go vet ./...`, `gofmt -l .` silent, `tsc --noEmit` clean). All 4 task commits verified in `git log f69be1be..HEAD`: `87e1346c`, `ac01ffa0`, `c4c495bb`, `77ddb495`. Targeted suites green (`go test ./internal/api/ ./internal/store/ ./internal/backup/`), every commit carries exactly one Co-Authored-By trailer, and no forbidden path (.gsd/, .planning/milestone.lock, .playwright-mcp/, uat-item1-baseline.png) was committed.
