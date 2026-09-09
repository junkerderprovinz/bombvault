---
phase: 01-selection-engine-restore-safety
plan: 01
subsystem: api
tags: [selection, backup-paths, restic, tdd, go, exclusions]

# Dependency graph
requires:
  - phase: 00-planning
    provides: RESEARCH/CONTEXT decisions on the flat "!"-prefixed selection encoding (R4, Q1-Q4) and locked restic argv positions L1/L14
provides:
  - internal/api/selection.go — pure selection-encoding primitives (ExclusionPrefix, SplitExclusion, PruneMaximal, NormalizeSelection) owning the meaning of "!"
  - service.SetBackupPaths normalizes mixed tree selections into the canonical flat stored set (dedupe, per-class maximal-root pruning, canonical order) with zero schema/wire change
  - Reader classification: configuredBackupPaths returns includes-only (explicit-vs-auto test stays on the raw list); storedDataIsGone classifies exclusions-only as explicit-none, never "data gone"
  - End-to-end proof that a narrowed selection reaches restic as maximal-include positionals with zero derived --exclude flags
affects: [01-04-container-mounts-class-split, restore-mapping, exclusion-assistant, tree-picker-ui]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 8470    # 33,880 diff chars over the 6 changed files (690 insertions, 9 deletions)
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Flat-set dual-class selection: bare entries = included roots, '!'-prefixed = deselected sub-branches, in ONE stored list; semantics owned by internal/api/selection.go, store treats entries as opaque strings"
    - "Per-class maximal-root pruning (includes and exclusions never prune each other); orphan exclusions preserved so explicit-none != [] = auto-detect"
    - "Reader classification at the raw/includes boundary: explicit-vs-auto decided on the RAW list, payload returned as includesOnly"

key-files:
  created:
    - internal/api/selection.go
    - internal/api/selection_test.go
    - internal/api/selection_readers_internal_test.go
  modified:
    - internal/api/service.go
    - internal/api/service_test.go
    - internal/store/targets.go

key-decisions:
  - "Selection encoding is a flat backupPaths set — bare + '!'-prefixed entries in one list; zero migration (SELECT-02), semantics centralized in internal/api/selection.go"
  - "Exclusions never become restic positionals and never derive --exclude flags (locked L1/L14): NormalizeSelection preserves them as a distinct class and readers split via SplitExclusion"
  - "configuredBackupPaths tests explicit-vs-auto on the RAW stored list but returns includes-only, so an exclusions-only selection stays an explicit choice while downstream consumers get a clean positional list"
  - "storedDataIsGone classifies explicit-none (non-empty raw, zero includes) as NOT gone, checked before any stat — a deliberate deselect is never refused as 'not reachable' (threat T-01-03)"

patterns-established:
  - "SplitExclusion as the single decode point: every reader that interprets a stored selection entry goes through it; nothing else may test strings.HasPrefix(\"!\")"
  - "White-box reader tests live in package api (*_internal_test.go) next to the helpers they reuse; pure encoding primitives get table tests in package api_test"

requirements-completed: [SELECT-01, SELECT-02, SELECT-04]

coverage:
  - id: D1
    description: "Pure selection-encoding primitives: SplitExclusion parses the '!' prefix; PruneMaximal drops strict descendants per class; NormalizeSelection canonicalizes (dedupe, per-class prune, includes-then-excludes order, idempotent, never nil)"
    requirement: SELECT-01
    verification:
      - kind: unit
        ref: "internal/api/selection_test.go#TestSplitExclusion"
        status: pass
      - kind: unit
        ref: "internal/api/selection_test.go#TestPruneMaximal"
        status: pass
      - kind: unit
        ref: "internal/api/selection_test.go#TestNormalizeSelection"
        status: pass
    human_judgment: false
  - id: D2
    description: "SetBackupPaths validates bare paths of both classes before translation (rejects bare '!'), then stores NormalizeSelection output; verified by a real-router PATCH round-trip asserting the stored normalized form via store.GetTargetByContainer, including legacy prefix-free and empty-list cases"
    requirement: SELECT-02
    verification:
      - kind: integration
        ref: "internal/api/service_test.go#TestSetBackupPathsMixedSelectionRoundTrip"
        status: pass
      - kind: integration
        ref: "internal/api/service_test.go#TestSetBackupPaths (legacy + empty cases)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Reader classification: configuredBackupPaths returns includes-only (raw-list explicit test); storedDataIsGone returns false for exclusions-only selections and unchanged legacy behavior for prefix-free lists"
    requirement: SELECT-04
    verification:
      - kind: unit
        ref: "internal/api/selection_readers_internal_test.go#TestConfiguredBackupPathsSplitsExclusions"
        status: pass
      - kind: unit
        ref: "internal/api/selection_readers_internal_test.go#TestStoredDataIsGoneClassifiesExplicitNone"
        status: pass
    human_judgment: false
  - id: D4
    description: "Backup compiles a mixed selection to maximal-include restic positionals with zero --exclude flags, and an exclusions-only container backs up (definition-only path) instead of being refused by the #181 guard"
    requirement: SELECT-04
    verification:
      - kind: integration
        ref: "internal/api/service_test.go#TestBackupNarrowedSelectionUsesMaximalIncludes"
        status: pass
      - kind: integration
        ref: "internal/api/service_test.go#TestBackupPathsExclusionsOnlyIsNotRefused"
        status: pass
    human_judgment: false

# Metrics
duration: 37min
completed: 2026-09-09
status: complete
---

# Phase 1 Plan 1: Selection Encoding Keystone Summary

**Flat "!"-prefixed selection encoding with per-class maximal-root pruning: mixed tree selections store canonically in the existing backupPaths set (zero migration), readers classify includes vs explicit-none, and backups hand restic maximal-include positionals with zero derived --exclude flags**

## Performance

- **Duration:** 37 min
- **Started:** 2026-09-09T15:48:49Z
- **Completed:** 2026-09-09T16:25:43Z
- **Tasks:** 2 (both TDD: RED and GREEN gates committed separately per task)
- **Files modified:** 6 (3 created, 3 modified)

## Accomplishments
- `internal/api/selection.go`: pure, table-tested encoding primitives — `ExclusionPrefix`, `SplitExclusion`, `PruneMaximal`, `NormalizeSelection` — that own the entire meaning of the "!" prefix; no store access, no receivers, POSIX-only path math mirroring internal/paths
- `service.SetBackupPaths` now normalizes any mixed selection into the canonical stored set (dedupe → per-class maximal-root pruning → includes-then-excludes order), validating bare paths of both classes before container translation — with zero schema, wire, or migration change (SELECT-02)
- Reader classification: `configuredBackupPaths` returns the includes half only while keeping its explicit-vs-auto test on the raw list; `storedDataIsGone` treats a non-empty selection with zero includes as a deliberate deselect, never "data gone" (threat T-01-03)
- End-to-end proof over `fakeResticEngine`: a narrowed selection sends restic exactly the maximal container-form include and an unchanged (empty) exclude list — locked argv positions L1/L14 hold

## Task Commits

Each task was committed atomically with separate TDD gates:

1. **Task 1 (tracer) RED: selection encoding tests** - `ee4bf937` (test)
2. **Task 1 (tracer) GREEN: normalize tree selections into the flat backupPaths set** - `8d4828e6` (feat)
3. **Task 2 RED: reader-classification and backup-positional tests** - `88de6f23` (test)
4. **Task 2 GREEN: classify prefixed selection readers; backups hand restic maximal includes** - `4bea7f3c` (feat)

**Tracer feedback gate:** after committing Task 1, its verify was re-run end-to-end (`TestSelect|TestSetBackupPaths|TestServiceContainerMountsAndSelection` + full internal/api and internal/store suites) and passed before any Task 2 work — expanding on a proven slice.

**Plan metadata:** pending (docs commit follows this SUMMARY)

_Note: TDD tasks carry two commits each (test → feat)._

## Files Created/Modified
- `internal/api/selection.go` (NEW) — the pure selection-encoding module; owns the "!" semantics
- `internal/api/selection_test.go` (NEW) — table tests for SplitExclusion, PruneMaximal, NormalizeSelection (incl. idempotency)
- `internal/api/selection_readers_internal_test.go` (NEW) — white-box pins for the unexported readers (package api)
- `internal/api/service.go` — SetBackupPaths normalize+validate path; configuredBackupPaths includes-only; storedDataIsGone explicit-none guard; #175/#181 doc paragraphs extended
- `internal/api/service_test.go` — real-router PATCH round-trip; narrowed-selection backup proof; exclusions-only backup refusal test
- `internal/store/targets.go` — SetBackupPaths doc comment points at internal/api/selection.go as the semantics owner (comment-only)

## Interim Window (intra-phase, by design)

Between this plan (wave 1) and plan 01-04 Task 1 (wave 3, splitting ContainerMounts classes), stored exclusions render in `GET /api/containers/{name}/mounts` as stale custom paths — the mounts reader does not yet decode the "!" prefix. This is known, accepted by the plan (`<action>` Task 2: "its current behavior with prefixed entries is interim intra-phase state resolved in plan 04"), and bounded to this phase. **Re-surface at the phase gate if plan 01-04 has not executed by then.**

## Decisions Made
- Flat-set encoding (no schema change): bare + "!"-prefixed entries coexist in one stored list; explicit-none is representable and distinct from `[]` = auto-detect (CONTEXT encoding Q1/Q3)
- Per-class pruning: an included root and an excluded branch deliberately coexist — that pair IS the "mount kept, volatile subfolder deselected" selection (Q4); includes and exclusions never prune each other
- Exclusions are preserved, never transformed: they are dropped at the reader boundary (includesOnly) and never derive restic `--exclude` flags, because restic excludes do not apply to positional sources (locked L1/L14)
- The #181 guard classifies explicit-none as "not gone" before any stat: the excluded branches never exist as literals, so onlyExistingPaths alone would prove nothing about the disk (T-01-03)
- Normalization re-attaches the prefix after container translation, so the store only ever sees translated, canonical, opaque strings

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added internal/api/selection_readers_internal_test.go (not in plan `<files>`)**
- **Found during:** Task 2 (RED)
- **Issue:** The plan's `<files>` lists only service.go + service_test.go, but `configuredBackupPaths` and `storedDataIsGone` are unexported — their required unit cases cannot assert call-by-call behavior from package `api_test`
- **Fix:** New white-box test file in package `api`, reusing `guardService`/`existingDir` from empty_backup_guard_internal_test.go per the existing internal-test convention
- **Files modified:** internal/api/selection_readers_internal_test.go (created)
- **Verification:** TestConfiguredBackupPathsSplitsExclusions + TestStoredDataIsGoneClassifiesExplicitNone green after GREEN
- **Committed in:** 88de6f23 (RED) / 4bea7f3c (GREEN path)

### Investigated, No Restructure

**TestBackupNarrowedSelectionUsesMaximalIncludes passed during RED (3 of 4 new tests failed as expected)**
- **Found during:** Task 2 (RED gate)
- **Observation:** Per the TDD fail-fast rule this was investigated, not ignored: after Task 1, `SetBackupPaths` stores exclusions as `"!"+path`, and such a literal never exists on disk — `effectiveBackupPaths`' existing `onlyExistingPaths` filter already dropped it, so positionals were accidentally correct at the engine-seam layer even before the Task 2 reader change
- **Resolution:** The discriminating RED signal for Task 2 is carried by the three reader-classification failures; the narrowed-selection test is kept unchanged as the end-to-end regression pin for the contract this plan locks (maximal includes in, zero excludes derived)

---

**Total deviations:** 1 auto-fixed (1 blocking — test placement only, no production-code deviation)
**Impact on plan:** The extra test file is the only way to reach the plan's own required unit cases; no scope creep, no behavior outside the plan.

## Issues Encountered
- The `newTestRouter` harness leaves `HostSourceRoot` empty, which makes `toContainerPath` reject all absolute host paths — the round-trip test builds its router inline with a populated cfg instead (mirroring TestServiceContainerMountsAndSelection). Resolved inside Task 1; harness left untouched for its existing users.
- Task 2's original single commit accidentally bundled RED tests with the GREEN implementation; the unpushed commit was split into the proper `test(01-01)` → `feat(01-01)` gate sequence (88de6f23 → 4bea7f3c) and the suite re-verified at final HEAD.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plans 01-02 (tree picker UI) and 01-03 can build directly on `SplitExclusion`/`NormalizeSelection`; the stored form is now canonical and idempotent, so re-saves cannot drift it
- Plan 01-04 owns the ContainerMounts class split (see Interim Window above); until then the mounts reader shows exclusions as stale custom paths
- Plan 03's restore mapping input stays clean: AppdataPaths recorded on the target is includes-only positional truth and never carries the prefix

## Self-Check: PASSED

All 7 files verified on disk; all 4 commit hashes verified in git log (ee4bf937, 8d4828e6, 88de6f23, 4bea7f3c). Full-repo gates green at final HEAD: `go test ./...` 22/22 packages ok, `go build ./...`, `go vet ./...`, `gofmt -l .` clean, `internal/store/migrate.go` untouched (SELECT-02).

---
*Phase: 01-selection-engine-restore-safety*
*Completed: 2026-09-09*
