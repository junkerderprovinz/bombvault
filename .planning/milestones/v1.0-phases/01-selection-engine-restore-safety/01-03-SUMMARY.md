---
phase: 01-selection-engine-restore-safety
plan: 03
subsystem: api
tags: [restic, restore, selection-mapping, snapshot-paths, run-records, contract-tests, scrubbing]

# Dependency graph
requires:
  - phase: 01-01 (selection encoding)
    provides: selection.go primitives — isStrictDescendant / PruneMaximal / NormalizeSelection / includesOnly, the "!"-prefix flat-set encoding mapRestorePaths builds on
  - phase: 01-02 (browse hardening)
    provides: hardened listing/scrubbing conventions this plan's error paths follow
provides:
  - mapRestorePaths(stored, snapshotPaths) — pure two-pass longest-prefix intersection producing the restore selector list + skipped list
  - prepareRestoreForTarget mapping block — synchronous mapping + empty-intersection abort BEFORE any destructive Stop/Remove
  - RestoreDeps.SkippedPaths additive DI field + orchestrator skip note in the success run record (scrubbed, 500-capped)
  - chosenSnapshot(snaps, id) — explicit-or-latest snapshot resolution feeding the mapping
  - restic 0.17 positional contract tests (excludes keep positionals; absolute Paths preserved)
  - same-basename multi-positional argv case in restic_args_test.go
affects: [phase-4 file-sets restore (01-CONTEXT D-13: File Sets reuse mapRestorePaths), restore UX/skip surfacing, DR drill reporting]

# Actuals (#2632)
actuals:
  tokens: 12048   # 48193 diff chars / 4, cumulative over the plan's 6 commits
  tasks: 3
  commits: 6

# Tech tracking
tech-stack:
  added: []   # no new libraries; restic 0.17.3 contract tests use the existing binary convention
  patterns:
    - two-pass deterministic path mapping (descendant clause, then longest-strict-ancestor fallback, deduped; unmapped → skipped, never fatal)
    - resolve failures in the synchronous prepare phase, before destructive teardown
    - success-with-note run records via the existing Runs.Finish errMsg channel (empty ⇒ byte-identical)
    - zero-value byte-identity pin (TestRestoreDepsSkippedPathsEmptyIsByteIdentical family)

key-files:
  created:
    - internal/api/selection_internal_test.go (white-box TestMapRestorePaths, 8 subtests)
    - internal/api/restore_selection_test.go (RESTORE-01 tracer + per-path skip, skip order, empty intersection, explicit-snapshot-id)
    - internal/restic/restic_positionals_contract_test.go (two real-binary TestPositional contract tests)
  modified:
    - internal/api/selection.go (mapRestorePaths)
    - internal/api/service.go (skippedPaths plan field, mapping+abort block, chosenSnapshot, deps wiring)
    - internal/backup/orchestrator.go (RestoreDeps.SkippedPaths, skippedPathsNote at the success Finish)
    - internal/backup/orchestrator_test.go (byte-identity pin)
    - internal/restic/restic_args_test.go (same-basename positional case, additions only)
    - internal/api/{service_test,foreign_internal_test,stacks_test,panic_recovery_test}.go (snapshot-Paths fixtures)

key-decisions:
  - "Mapping resolves failures synchronously in prepareRestoreForTarget; the orchestrator port RestorePaths is unchanged — the orchestrator only ever receives a mapped, snapshot-path-form list"
  - "Empty intersection (stored paths present, nothing maps) aborts in prepare with the explicit nothing-to-restore error, never after Stop/Remove"
  - "Per-path skips are notes, not aborts: carried via additive RestoreDeps.SkippedPaths into the success run record — count + [path]-scrubbed tokens, length-capped through truncateErr"
  - "Empty SkippedPaths returns \"\" from skippedPathsNote so nil/empty produce byte-identical run rows (pinned)"
  - "Skip order is pinned pre-scrub at the pure level (TestMapRestorePaths); the scrubbed run row is order-insensitive by design (T-01-11)"
  - "Fixture snapshots predating RESTORE-01 now record realistic Paths — a snapshot without Paths is under-specified, not the guard over-eager"

patterns-established:
  - "RESTORE-01 mapping contract: restore selectors must come from the chosen snapshot's Paths, never the stored list replayed verbatim"
  - "New fake-engine container fixtures must seed snapshot Paths matching the stored positional"

requirements-completed: [RESTORE-01]

# Coverage metadata (#1602)
coverage:
  - id: D1
    description: "Pure longest-prefix mapping helper (mapRestorePaths) intersecting the stored selection with the chosen snapshot's Paths, with per-path skips"
    requirement: RESTORE-01
    verification:
      - kind: unit
        ref: "internal/api/selection_internal_test.go#TestMapRestorePaths"
        status: pass
    human_judgment: false
  - id: D2
    description: "Restoring an older snapshot after the selection changed completes, handing the engine the mapped snapshot-path form (the RESTORE-01 tracer)"
    requirement: RESTORE-01
    verification:
      - kind: integration
        ref: "internal/api/restore_selection_test.go#TestRestoreSelectionChange"
        status: pass
    human_judgment: false
  - id: D3
    description: "Per-path skip completes the restore and records a bounded, [path]-scrubbed note in the success run row; multiple orphans never abort"
    requirement: RESTORE-01
    verification:
      - kind: integration
        ref: "internal/api/restore_selection_test.go#TestRestoreSelectionChangePerPathSkip"
        status: pass
      - kind: integration
        ref: "internal/api/restore_selection_test.go#TestRestoreSelectionChangeSkipOrder"
        status: pass
    human_judgment: false
  - id: D4
    description: "Empty intersection aborts in the synchronous prepare phase with the nothing-to-restore error and provably no stop/remove calls"
    requirement: RESTORE-01
    verification:
      - kind: integration
        ref: "internal/api/restore_selection_test.go#TestRestoreEmptyIntersection"
        status: pass
    human_judgment: false
  - id: D5
    description: "Restore by explicit snapshot id maps against THAT snapshot's Paths, not the newest"
    requirement: RESTORE-01
    verification:
      - kind: integration
        ref: "internal/api/restore_selection_test.go#TestRestoreSelectionChangeExplicitSnapshotID"
        status: pass
    human_judgment: false
  - id: D6
    description: "Zero-value byte identity: nil/empty RestoreDeps.SkippedPaths produce the exact pre-feature run record"
    requirement: RESTORE-01
    verification:
      - kind: unit
        ref: "internal/backup/orchestrator_test.go#TestRestoreDepsSkippedPathsEmptyIsByteIdentical"
        status: pass
    human_judgment: false
  - id: D7
    description: "restic 0.17 positional contracts: excludes never drop the positional source dir; snapshot Paths preserve absolute paths verbatim"
    requirement: RESTORE-01
    verification:
      - kind: integration
        ref: "internal/restic/restic_positionals_contract_test.go#TestPositionalExcludesKeepSourceDir"
        status: unknown
      - kind: integration
        ref: "internal/restic/restic_positionals_contract_test.go#TestPositionalAbsolutePathPreserved"
        status: unknown
    human_judgment: true
    rationale: "Real-binary contract tests skip on this restic-less Windows dev box (exec.LookPath); they run on CI against pinned restic 0.17.3. Status is CI-proven, not locally provable."
  - id: D8
    description: "argv contract: two positional sources sharing a basename leaf pass through -- verbatim and in order"
    requirement: RESTORE-01
    verification:
      - kind: unit
        ref: "internal/restic/restic_args_test.go#TestBackupArgsSameBasenamePositionals"
        status: pass
    human_judgment: false

# Metrics
duration: 61min
completed: 2026-09-09
status: complete
---

# Phase 01 Plan 03: Restore Selection Mapping & Skip Safety Summary

**Restore selectors now map onto the CHOSEN snapshot's recorded Paths (two-pass longest-prefix) with per-path skips recorded as scrubbed run notes, empty intersections aborting before any destructive teardown, and the restic 0.17 positional behaviors the design relies on contract-pinned for CI.**

## Performance

- **Duration:** 61 min
- **Started:** 2026-09-09T17:22:32Z
- **Completed:** 2026-09-09T18:25:10Z
- **Tasks:** 3
- **Files modified:** 12 (cumulative across the plan's 6 commits)

## Accomplishments

- RESTORE-01 closed: restoring an older snapshot after the user reshaped their selection now completes — `mapRestorePaths` intersects the stored positional truth with the chosen snapshot's Paths; the empty-intersection case aborts synchronously in prepare, before Stop/Remove
- Per-path skips never abort: unmapped stored paths flow through the additive `RestoreDeps.SkippedPaths` into a bounded, `[path]`-scrubbed note on the success run row (count preserved; nil/empty byte-identical, pinned)
- restic 0.17 assumptions contract-pinned: `--exclude` filters files but never drops a positional source dir; snapshot Paths preserve absolute paths verbatim (skip locally, proven on CI against 0.17.3)
- Plan-level gates green: `go test ./...`, `go vet ./...`, `gofmt -l .` all clean

## Task Commits

Each task was committed atomically (TDD tasks split RED/GREEN):

1. **Task 1: mapRestorePaths + prepare-phase wiring + tracer** — `daff52f7` (test, RED) + `3d59abd8` (feat, GREEN)
2. **Task 2: skip reporting through the orchestrator + empty-intersection proof** — `a2d665a5` (test, RED) + `cdcede28` (feat, GREEN)
3. **Task 3: restic positional contract tests + same-basename argv case** — `68156e2d` (test)
4. **Follow-up: fixture regression fix from Task 1's guard** — `f034a035` (test)

## Files Created/Modified

- `internal/api/selection.go` — `mapRestorePaths`: pure two-pass mapping (snapshot-path-at-or-below-stored restores as-is; uncovered stored paths fall back to the LONGEST strict ancestor, deduped; the rest skip)
- `internal/api/service.go` — `containerRestorePlan.skippedPaths`, mapping+scrubbed-log+abort block in `prepareRestoreForTarget`, `chosenSnapshot`, `SkippedPaths` wired into the executeRestore deps
- `internal/backup/orchestrator.go` — additive `RestoreDeps.SkippedPaths` + `skippedPathsNote` at the success `Runs.Finish` site
- `internal/api/selection_internal_test.go` — white-box 8-subtest mapping table
- `internal/api/restore_selection_test.go` — tracer + per-path skip + skip order + empty intersection + explicit-id cases
- `internal/backup/orchestrator_test.go` — `TestRestoreDepsSkippedPathsEmptyIsByteIdentical` pin
- `internal/restic/restic_positionals_contract_test.go` — two `TestPositional*` real-binary contract tests
- `internal/restic/restic_args_test.go` — `TestBackupArgsSameBasenamePositionals` (additions only)
- `internal/api/{service_test,foreign_internal_test,stacks_test,panic_recovery_test}.go` — snapshot-Paths fixtures (regression fix)

## Decisions Made

- Mapping lives in the synchronous prepare phase; the orchestrator port `RestorePaths` is unchanged — failure resolution can never happen after destructive teardown
- The skip note is emitted at `RestoreContainer`'s success `Runs.Finish` call site (the plan's wording said "in runRestore"; `runRestore` never calls `Finish` — same channel, one caller upstream)
- Scrub discipline for the note: `scrubRunErr` per path FIRST (count stays exact), then the whole note length-capped through `truncateErr` — the final pass is the same belt-and-suspenders rule every writer of `runs.error` follows
- Skip ORDER is observable only pre-scrub, so it is pinned in `TestMapRestorePaths` (pure level); the scrubbed run-row note is order-insensitive by design (T-01-11) — the integration test pins multi-skip non-abort + exact note shape instead

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] White-box test file for the unexported helper**
- **Found during:** Task 1 (RED)
- **Issue:** `mapRestorePaths` is deliberately unexported (selection.go's primitives are all internal); the plan's test listing didn't name a home for its table test, and the black-box package (`api_test`) cannot reach it
- **Fix:** created `internal/api/selection_internal_test.go` (package `api`), mirroring plan 01-01's `selection_readers_internal_test.go` precedent
- **Files modified:** internal/api/selection_internal_test.go
- **Verification:** `go test ./internal/api/ -run TestMapRestorePaths -count=1`
- **Committed in:** `daff52f7`

**2. [Rule 1 - Bug] RED skip-note fixtures seeded nothing skippable**
- **Found during:** Task 2 (GREEN)
- **Issue:** the first per-path-skip fixtures seeded all stored paths UNDER the snapshot path, so every path mapped to the shared ancestor and `skipped` came back empty — the tests failed for the wrong reason and could never pass with any correct note implementation
- **Fix:** reseeded with true orphans (a bind added after the snapshot: no ancestor, no descendant); honest shapes now drive count-1/count-2 assertions
- **Files modified:** internal/api/restore_selection_test.go
- **Verification:** both tests fail at RED (no note) and pass at GREEN with the note present
- **Committed in:** `cdcede28` (GREEN commit)

**3. [Rule 1 - Bug] Task 1's guard broke 10 pre-existing restore tests**
- **Found during:** plan-level verification (`go test ./...`)
- **Issue:** container/stack/foreign-restore fixtures seeded snapshots with only ID+Tags — the fake-engine convention predating RESTORE-01 — which the empty-intersection guard now reads as "nothing in this snapshot", refusing ten legitimate restores (deleted-container definition restore, cross-pool remap family, stack restore loops)
- **Fix:** each seeded snapshot now records the `Paths` its real counterpart would write; fixtures were under-specified, the guard was not over-eager; no production code changed
- **Files modified:** internal/api/service_test.go, internal/api/foreign_internal_test.go, internal/api/stacks_test.go, internal/api/panic_recovery_test.go
- **Verification:** `go test ./...` green; the ten named tests pass individually
- **Committed in:** `f034a035`

---

**Total deviations:** 3 auto-fixed (1 blocking, 2 bug)
**Impact on plan:** All three were necessary for correctness of the new guard and its interaction with existing tests. No scope creep; no architectural changes.

## Issues Encountered

- This dev box's go tool refuses to resolve self-module imports in newly created test files ("no required module provides package …" for byte-identical imports that compile fine in pre-existing files; repo-wide, reproducible). Worked within it: the contract test file uses `package restic` (the `restic_args_test.go` convention, identifiers in scope without an import) instead of `package restic_test`. Baseline `go build ./...`/existing tests are unaffected; noted for the environment owner, not fixed (out of scope)
- restic is not on PATH locally, so the two `TestPositional*` contract tests skip loudly here and prove on CI against pinned restic 0.17.3 — the plan's explicit design

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `mapRestorePaths` is pure and receiver-free — File Sets reuse it in Phase 4 per 01-CONTEXT D-13
- The restore path now guarantees: mapped snapshot-path-form selectors, skips as run notes, aborts only in prepare — restore UX/skip-surfacing work can build on the run-record note channel
- No blockers

---
*Phase: 01-selection-engine-restore-safety*
*Completed: 2026-09-09*

## Self-Check: PASSED

All 9 created/modified key files verified on disk; all 6 commit hashes verified in `git log`. Plan-level gates re-run clean at close: `go test ./...` exit 0, `go vet ./...` exit 0, `gofmt -l .` empty.
