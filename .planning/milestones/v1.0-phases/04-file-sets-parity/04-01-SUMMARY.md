---
phase: 04-file-sets-parity
plan: 01
subsystem: backup
tags: [restic, sqlite, file-sets, selection, go, migration]

# Dependency graph
requires:
  - phase: 01-selection-engine-restore-safety
    provides: internal/api/selection.go flat-set semantics (NormalizeSelection/includesOnly/excludedBranches), restic positional contract pins
  - phase: 03-selection-trust-controls
    provides: owned-setter and additive-PATCH-field precedents (SetExcludeCaches shape, #199 cadence rationale)
provides:
  - Migration v101 `file_set_selected_paths` (nullable TEXT on file_sets) + FileSet.SelectedPaths with nullable scan in all three SELECT paths
  - Owned store setter SetFileSetSelectedPaths (nil stores SQL NULL, never '[]'); UpdateFileSet/CreateFileSet untouched so full-row saves never clobber a selection
  - FileSetBackupDeps.SourcePaths []string additive passthrough (nil/empty keeps legacy []string{SourceDir} byte-identical); FilesRestic interface unchanged
  - fileSetPositionals compile helper in selection.go (legacy nil, shared maximal prune, re-anchor filter, filtered-empty fallback)
  - BackupFileSet single compile-site wiring: positionals + excludedBranches tail mirroring the container compile line; manual/batch/Everything inherit
affects: [04-02 PATCH boundary and empty-selection refusal, 04-03 Files page tree, 04-04 restore mapping]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 9570    # 38283 chars of realized internal/ diff / 4 (plan estimated 55000)
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []   # zero installs, per research (Go stdlib + existing deps only)
  patterns:
    - "Nullable-column legacy switch: NULL column = 'never touched by the tree' = byte-identical legacy argv; owned setter keeps NULL and '[]' distinct"
    - "Compile-time re-anchor: stored selection entries are re-filtered against the freshly resolved root every backup (the anchor is user-mutable)"
    - "Single compile site per domain: BackupFileSet is the only place a file-set selection becomes restic argv"

key-files:
  created:
    - none (parity plan — every file already existed; new tests are appended to existing files)
  modified:
    - internal/store/migrate.go (v101 appended; all prior migrations byte-identical, zero removed lines)
    - internal/store/filesets.go (SelectedPaths field, three SELECT lists + nullable scan, SetFileSetSelectedPaths)
    - internal/store/filesets_test.go (TestFileSetSelectedPathsRoundTrip)
    - internal/backup/files_orchestrator.go (SourcePaths field + wrap)
    - internal/backup/files_orchestrator_test.go (TestBackupFileSetDirSourcePaths)
    - internal/api/selection.go (fileSetPositionals)
    - internal/api/service.go (BackupFileSet compile wiring)
    - internal/api/files_internal_test.go (TestFileSetPositionals white-box table)
    - internal/api/service_test.go (TestBackupFileSetSelectedPaths subtests)
    - internal/restic/restic_positionals_contract_test.go (TestMultiPositionalPathsMirrorSelection)

key-decisions:
  - "Compile re-runs the stored list through the shared NormalizeSelection before the anchor filter: the plan action's shorthand (includesOnly + filter) could not satisfy the plan's own behavior spec ([srcRoot, srcRoot/child] must compile to [srcRoot]) because includesOnly does not prune; NormalizeSelection is the existing prune owner, so there is still no second pruning site"
  - "Orchestrator wrap guards on len(d.SourcePaths) > 0 rather than non-nil, so a hypothetically empty compiled list can never hand restic zero positional sources"
  - "Tracer landed as two bounded commits (store leg standalone green; backup/api leg) per the plan's sub-commit discipline, never one mega-commit across three packages"
  - "Real-restic case uses two disjoint branches (shallower + deeper), the shape fileSetPositionals actually emits; a nested positional pair is unconstructable by the compile and would have proven a shape production cannot produce"

patterns-established:
  - "Nullable selection column: NULL is the legacy switch; '[]' is never stored for it — the distinction is load-bearing at the compile"
  - "Re-anchor on every compile: stored entries are never trusted stale against a mutable root; filtered-empty falls back to the root, an unanchored positional is never emitted"

requirements-completed: [INTEG-02]  # copied verbatim from plan frontmatter; see requirement note below — INTEG-02 is shared by all four phase plans, so REQUIREMENTS.md marking is gate-blocked until their summaries exist

# Coverage metadata (#1602) — one entry per shipped deliverable.
coverage:
  - id: D1
    description: "Migration v101 file_set_selected_paths: nullable TEXT column appended unconditionally after v100, no alreadySatisfied guard; existing rows read NULL"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/store/filesets_test.go#TestFileSetSelectedPathsRoundTrip"
        status: pass
      - kind: other
        ref: "grep -c 'version: 101' internal/store/migrate.go == 1; git diff shows zero removed migrate.go lines"
        status: pass
    human_judgment: false
  - id: D2
    description: "Owned setter SetFileSetSelectedPaths: written selection round-trips through all three read paths; nil stores SQL NULL (never '[]'); UpdateFileSet never clobbers a selection"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/store/filesets_test.go#TestFileSetSelectedPathsRoundTrip"
        status: pass
    human_judgment: false
  - id: D3
    description: "Orchestrator SourcePaths passthrough with nil/empty legacy fallback; tags and excludes unchanged by the multi-root compile"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/backup/files_orchestrator_test.go#TestBackupFileSetDirSourcePaths"
        status: pass
      - kind: unit
        ref: "tests/internal/backup/files_orchestrator_test.go#TestBackupFileSetDir (pre-existing nil-legacy pin, untouched)"
        status: pass
    human_judgment: false
  - id: D4
    description: "fileSetPositionals compile helper: legacy nil, re-anchor filter (segment-aligned, sibling-prefix safe), filtered-empty fallback, determinism/canonical order"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/files_internal_test.go#TestFileSetPositionals"
        status: pass
    human_judgment: false
  - id: D5
    description: "BackupFileSet single compile site: maximal-root positionals + derived excludes after the user's own patterns; NULL column compiles byte-identically (legacy pin untouched and green)"
    requirement: INTEG-02
    verification:
      - kind: unit
        ref: "tests/internal/api/service_test.go#TestBackupFileSetSelectedPaths"
        status: pass
      - kind: unit
        ref: "tests/internal/api/service_test.go#TestBackupFileSet (pre-existing NULL-column pin, untouched)"
        status: pass
    human_judgment: false
  - id: D6
    description: "Engine-floor proof: multi-positional backup records snapshot Paths exactly equal to the positional list (order preserved) with the excluded branch filtered (restic 0.17.3 contract)"
    requirement: INTEG-02
    verification:
      - kind: integration
        ref: "tests/internal/restic/restic_positionals_contract_test.go#TestMultiPositionalPathsMirrorSelection"
        status: unknown   # skips locally (no restic on PATH, per plan design); CI Test job (restic 0.17.3) is the gate
    human_judgment: true
    rationale: "The contract test carries the file's exec.LookPath skip guard and cannot execute on the Windows dev box; the plan explicitly assigns CI (restic 0.17.3) as its proof. Everything provable locally (argv shape, compile semantics, store contract) is proven above."

# Metrics
duration: 22min
completed: 2026-09-11
status: complete
---

# Phase 4 Plan 1: Selection Round-Trip Keystone Summary

**A file set's tree selection now compiles at backup time into maximal-root restic positionals with derived excludes at the single site — while a set never touched by the tree backs up byte-identically to before (NULL-column legacy switch, pinned by the untouched TestBackupFileSet).**

## Performance

- **Duration:** 22 min
- **Started:** 2026-09-11T01:52:45Z
- **Completed:** 2026-09-11T02:15:32Z
- **Tasks:** 2 (Task 1 landed as two sub-commits per the plan's sub-commit discipline)
- **Files modified:** 10 code files (zero new files; all tests appended to existing files)

## Accomplishments
- Migration v101 `file_set_selected_paths` appended (nullable TEXT, no default, no alreadySatisfied guard); every existing row reads NULL = legacy switch; all prior migrations byte-identical (zero removed lines)
- Owned setter `SetFileSetSelectedPaths` (nil stores SQL NULL, never `'[]'`); `UpdateFileSet`'s explicit column list untouched, so name/path/excludes/enabled saves can never clear a stored selection (pinned across three consecutive saves)
- `FileSetBackupDeps.SourcePaths []string` additive passthrough with nil/empty legacy fallback; `FilesRestic` interface untouched (RESEARCH Pitfall 1 landed exactly as planned)
- `fileSetPositionals` compile helper: legacy nil, shared maximal prune, segment-aligned re-anchor against the freshly resolved root, filtered-empty fallback — an unanchored positional is never emitted
- `BackupFileSet` single compile-site wiring with `excludedBranches` tail mirroring the container line; manual, batch, and Backup Everything all inherit the compile
- The compile's hard invariants are pinned to fail loudly: a mutation disabling the anchor filter fails 10 pins (verified, then reverted byte-identical)

## Task Commits

Each task was committed atomically:

1. **Task 1 (store leg): Selection round-trip keystone — v101 column, owned setter** - `3b42ef05` (feat)
2. **Task 1 (backup/api leg): SourcePaths passthrough and file-set compile** - `9dd2f313` (feat)
3. **Task 2: Compile hardening pins — re-anchor/determinism/stale-root/real-restic** - `2776eb4a` (test)

## Files Created/Modified
- `internal/store/migrate.go` — v101 entry only (append-only; NUMBERING HAZARD comments preserved verbatim)
- `internal/store/filesets.go` — SelectedPaths field + nullable scan in all three SELECT paths + owned setter
- `internal/store/filesets_test.go` — TestFileSetSelectedPathsRoundTrip (NULL-on-create, round-trip, preserve ×3, clear-to-NULL, unknown-id)
- `internal/backup/files_orchestrator.go` — SourcePaths deps field + compile-honoring wrap
- `internal/backup/files_orchestrator_test.go` — TestBackupFileSetDirSourcePaths (verbatim passthrough + nil-legacy)
- `internal/api/selection.go` — fileSetPositionals (the single file-set compile helper)
- `internal/api/service.go` — BackupFileSet compile wiring (single site)
- `internal/api/files_internal_test.go` — TestFileSetPositionals white-box table + determinism
- `internal/api/service_test.go` — TestBackupFileSetSelectedPaths (maximal prune, canonical order, derived excludes, stale-root)
- `internal/restic/restic_positionals_contract_test.go` — TestMultiPositionalPathsMirrorSelection (CI-gated engine proof)

## Decisions Made
- Compile re-runs stored entries through the shared `NormalizeSelection` before the anchor filter — see Deviations item 1; the behavior spec's maximal-prune requirement wins over the action text's `includesOnly` shorthand, and the prune stays the shared implementation's job
- Orchestrator guard is `len(d.SourcePaths) > 0` (not just non-nil) so an empty list can never zero restic's positionals
- Real-restic case backs up two disjoint branches (the shape the compile actually emits) rather than the action text's literal "parent dir and a subdirectory under it" — a nested positional pair is unconstructable by `fileSetPositionals`, so proving it would have pinned a shape production cannot produce

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan action/behavior conflict in the compile helper: prune source**
- **Found during:** Task 1, step 5 (selection.go)
- **Issue:** The action text said `positionals = includesOnly(selected) filtered to entries at-or-under src`, but `includesOnly` only extracts bare entries — it does not prune. The plan's own behavior spec (multi-root subtest: stored `[srcRoot, srcRoot/child]` must compile to exactly `[srcRoot]`) therefore required pruning at compile time for any stored list that bypasses normalization.
- **Fix:** `fileSetPositionals` routes the stored list through the existing `NormalizeSelection` (canonical order + dedupe + per-class maximal prune) before the anchor filter. No second pruning site exists — the pruning is the shared implementation's, matching the must_haves truth "the maximal prune happens through includesOnly, no second pruning site" in the only way that actually prunes.
- **Files modified:** internal/api/selection.go
- **Verification:** TestFileSetPositionals "redundant descendant collapses to the maximal root" and TestBackupFileSetSelectedPaths "multi-root selection compiles to the maximal root only" pass
- **Committed in:** 9dd2f313 (part of the backup/api leg commit)

**2. [Rule 1 - Bug] Determinism pin's own expectations were wrong (test bug, fixed before commit)**
- **Found during:** Task 2
- **Issue:** The determinism subtest first expected collapsed roots despite a bare root in the stored set, then expected a rewritten path (`/alpha` from `/alpha/inner`) that nothing in the pipeline produces — both were pin-authoring errors, not implementation defects. The compile was behaving per contract both times.
- **Fix:** Corrected the fixture to disjoint survivors (`[src/zeta, src/alpha/inner, /elsewhere/filtered-out]` → `[src/alpha/inner, src/zeta]`, sorted, stable across calls).
- **Files modified:** internal/api/files_internal_test.go
- **Verification:** TestFileSetPositionals fully green; the "fix the code, not the pin" rule was checked and did not trigger — no production defect existed
- **Committed in:** 2776eb4a

---

**Total deviations:** 2 auto-fixed (2 × Rule 1: one plan-internal inconsistency resolved in favor of the behavior spec, one pin-authoring error corrected pre-commit).
**Impact on plan:** Neither expands scope; both land the plan's stated truths more faithfully than the literal action text would have.

## Issues Encountered
- None blocking. Note: `go test ./internal/restic/` contract cases (including the new `TestMultiPositionalPathsMirrorSelection`) skip on this Windows dev box — no restic on PATH; this is the plan's explicit design and the CI Test job (restic 0.17.3) is their gate, the same accepted dev-box state recorded in STATE.md since Phase 1.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Ready for 04-02: the column, owned setter, and compile the PATCH boundary writes through all exist and are pinned; the store-side state the PATCH-time rules need (clear-on-path-edit layer, empty-selection refusal, 64-entry cap) has its read-side defense already in place (stale-root re-anchor + filtered-empty fallback)
- Ready for 04-03: `FileSet.SelectedPaths` round-trips through all three read paths for the view serving
- Ready for 04-04: the compile produces exactly the stored positional list `mapRestorePaths` will consume
- `TestBackupFileSet` (service_test.go) — the pre-phase legacy pin — was not edited and stays green; the acceptance criterion "grep of the diff for group-by returns nothing" holds (0 matches)

---
*Phase: 04-file-sets-parity*
*Completed: 2026-09-11*

## Self-Check: PASSED

All 10 modified code files exist on disk; all 3 task commits (`3b42ef05`, `9dd2f313`, `2776eb4a`) exist in git log; SUMMARY.md present. Plan-level verification re-run at close: `go build ./...`, `go vet ./...`, `gofmt -l .` (silent), and `go test ./internal/store/ ./internal/backup/ ./internal/api/ ./internal/restic/` all green, including the untouched `TestBackupFileSet` legacy pin. Diff scope confirmed: zero removed lines in migrate.go, zero touches to handlers.go or web/, zero `group-by` matches.
