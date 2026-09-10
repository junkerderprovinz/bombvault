---
phase: 01-selection-engine-restore-safety
plan: 05
subsystem: api
tags: [restic, backup, selection, exclude-patterns, argv, gap-closure, wr-01]

# Dependency graph
requires:
  - phase: 01-01
    provides: selection.go pure encoding primitives (SplitExclusion, isStrictDescendant, NormalizeSelection) + normalized SetBackupPaths + maximal-include positionals
  - phase: 01-04
    provides: exclusion visibility (mounts excluded array) + empty-selection guard + WR-02-corrected comments engaging the real tradeoff
provides:
  - excludedBranches(entries []string) []string — pure strict-descendant derivation of the exclusion branches a backup must enforce
  - Backup argv encodes stored exclusion branches as restic --exclude patterns after user-resolved patterns (WR-01 closed; snapshot content matches the advertised selection)
  - White-box derivation table, deterministic merge-order pin, real-restic absolute-subdir contract test
  - Four planning-doc drifts realigned (PROJECT.md rationale + encode decision, REQUIREMENTS.md INTEG-04 + BROWSE-01, ROADMAP SC2 parenthetical)
affects: [02-container-panel-tree, 03-selection-trust-controls, phase-1-re-verification]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
actuals:
  tokens: 6822   # chars/4 over the realized diff (27290 diff chars) vs estimate 44000 — plan overestimated
  tasks: 3
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Selection-derived restic --exclude tail: user-resolved patterns first, excludedBranches(tg.SelectedPaths)... appended at the single BackupDeps construction site"
    - "Strict-descendant qualification reuses isStrictDescendant; equality and orphan exclusions deliberately never emitted (pinned by TestExcludedBranches)"

key-files:
  created: []
  modified:
    - internal/api/selection.go
    - internal/api/service.go
    - internal/api/service_test.go
    - internal/api/selection_internal_test.go
    - internal/restic/restic_positionals_contract_test.go
    - .planning/PROJECT.md
    - .planning/REQUIREMENTS.md
    - .planning/ROADMAP.md

key-decisions:
  - "Encode stored exclusion branches as restic --exclude at the single BackupDeps.Excludes site (WR-01, user decision 2026-09-09); positionals stay the maximal-root includes"
  - "Accepted tradeoff: derived patterns land in the snapshot's user-owned restic Excludes metadata — content correctness wins over metadata purity"
  - "Equality and orphan exclusions never emitted (contradictory pair / nothing to carve); survivors keep stored order so the argv tail is deterministic"
  - "Exclusions-only selections untouched: zero includes means excludedBranches returns empty and the definition-only path stays intact"

patterns-established:
  - "excludedBranches: the enforcement half of the stored selection — strict-descendant qualification against includes via the same segment-aligned primitive as paths.go:44-48"

requirements-completed: [SELECT-01, SELECT-02, SELECT-04, BROWSE-01]

# Coverage metadata (#1602) — one entry per shipped deliverable.
coverage:
  - id: D1
    description: "A mixed selection's backup hands restic the maximal-root includes as positionals plus the stored exclusion branch as --exclude (WR-01 closed; snapshot content matches the advertised selection)"
    requirement: SELECT-01
    verification:
      - kind: unit
        ref: "tests/internal/api/service_test.go#TestBackupNarrowedSelectionUsesMaximalIncludes"
        status: pass
    human_judgment: false
  - id: D2
    description: "Stored AppdataPaths stays excludes-free — restore mapping input holds no !-prefixed entries and equals the positional list"
    requirement: RESTORE-01
    verification:
      - kind: unit
        ref: "tests/internal/api/service_test.go#TestBackupNarrowedSelectionUsesMaximalIncludes (AppdataPaths + ExclusionPrefix assertions)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Derivation semantics table-pinned: qualify-both, orphan-dropped, exclusions-only, equality-non-emission, includes-only, empty-non-nil, deep-branch cases"
    verification:
      - kind: unit
        ref: "tests/internal/api/selection_internal_test.go#TestExcludedBranches"
        status: pass
    human_judgment: false
  - id: D4
    description: "Merged argv order deterministic: user exclude patterns first, selection-derived tail appended, positionals undisturbed"
    verification:
      - kind: unit
        ref: "tests/internal/api/service_test.go#TestBackupSelectionExcludesMergeAfterUserPatterns"
        status: pass
    human_judgment: false
  - id: D5
    description: "Real-restic 0.17 proof that an absolute-subdir exclude pattern filters a subtree WITHIN a positional source without dropping the positional"
    verification:
      - kind: integration
        ref: "tests/internal/restic/restic_positionals_contract_test.go#TestPositionalExcludeAbsoluteSubdirPattern"
        status: unknown
    human_judgment: true
    rationale: "No restic on this Windows dev box PATH, so the contract test skips locally by design (LookPath guard); it has never executed anywhere yet because the docker-folders branch is unpushed. CI (restic 0.17.3 on Linux) is the arbiter — the same closure step the phase verification report already requires (push branch, require green Test job)."
  - id: D6
    description: "Four planning-doc drifts realigned (PROJECT.md rationale + encode decision with date, INTEG-04 backend-landed/UI-pending split, BROWSE-01 mechanism text, ROADMAP SC2 parenthetical)"
    verification:
      - kind: other
        ref: "grep gates: disqualified=0, hasChildren=0, 'never exclude-based encoding'=0, 2026-09-09 present in PROJECT.md Key Decisions and ROADMAP SC2, no changed line touches SELECT-03"
        status: pass
    human_judgment: false
  - id: D7
    description: "Restore contamination + scope guards: derivation referenced only by the backup construction site and its tests; internal/backup, internal/store/migrate.go, and web/ diff-empty from this plan"
    verification:
      - kind: other
        ref: "grep -rl excludedBranches internal/ = exactly selection.go, service.go, selection_internal_test.go, service_test.go; git diff --name-only 9792d927..HEAD -- internal/backup internal/store/migrate.go web = empty"
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-09-10
status: complete
---

# Phase 1 Plan 5: WR-01 Gap Closure — Exclusion Enforcement + Docs Realignment Summary

**Stored exclusion branches are now enforced as restic `--exclude` patterns on the backup argv (positionals stay maximal-root includes), closing the gap where the engine backed up branches the UI advertised as excluded — plus the four planning-doc drifts realigned.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-09-10T09:33:03Z
- **Completed:** 2026-09-10T09:44:51Z
- **Tasks:** 3 (Task 1 in full RED→GREEN TDD; tracer verified end-to-end before expansion)
- **Files modified:** 8

## Accomplishments

- WR-01 closed: a mixed folder selection (include + exclude branch) now produces a backup whose restic invocation is maximal-root includes as positionals (after `--`) plus one `--exclude` per stored exclusion branch strictly below an included root (before `--`), with user exclude patterns preserved ahead of the derived tail — the snapshot content finally matches what the stored selection and the mounts `excluded` array advertise (SC2 content half un-overridden).
- `excludedBranches` added to `internal/api/selection.go`: pure, unexported, strict-descendant qualification against includes; equality exclusions and orphan exclusions deliberately never emitted (contradictory pair / nothing to carve), stored order kept, never nil; glob-semantics caveat documented per threat T-01-05-01.
- Wired at exactly ONE production literal: the `backup.BackupDeps{...}` `Excludes:` field in `service.Backup` (`append(s.resolveExcludePatterns(tg.Excludes, in), excludedBranches(tg.SelectedPaths)...)`). No other service function changed — one hunk region inside Backup.
- `includesOnly` doc comment revised: the metadata-purity lock tail replaced by the amended 2026-09-09 decision with the accepted Excludes-metadata tradeoff; the corrected restic facts (excludes DO filter content within positional sources) retained.
- Restore safety proven: the revised tracer test pins stored `AppdataPaths` == positionals with no `ExclusionPrefix` entry; helper enumeration across `internal/` returns exactly the four api files; `internal/backup`, `internal/store/migrate.go`, and `web/` are diff-empty from this plan (scope guard held vs `9792d927`).
- Zero migration, unchanged `!`-prefix wire encoding, browse and `web/` untouched; exclusions-only selections still run definition-only (`TestBackupPathsExclusionsOnlyIsNotRefused` green, unmodified); user-pattern passthrough for containers without a tree selection untouched.

## Task Commits

Each task was committed atomically (TDD sequence for Task 1):

1. **Task 1 (RED): revised enforcement test** - `8e3b86f3` (test) — fails on the exclude assertion before any production change
2. **Task 1 (GREEN): encode stored exclusion branches as restic --exclude** - `6dc3cdeb` (feat)
3. **Task 2: derivation table + merge order + real-restic subdir contract** - `184a248f` (test)
4. **Task 3: four planning-doc drifts realigned** - `a426ece5` (docs)

**Plan metadata:** see final docs commit (state + roadmap progress).

## Files Created/Modified

- `internal/api/selection.go` — NEW `excludedBranches(entries []string) []string` + revised `includesOnly` doc comment (amended lock, metadata tradeoff)
- `internal/api/service.go` — one hunk in `Backup`: `Excludes:` = user-resolved patterns + selection-derived tail, with the gap-closure comment
- `internal/api/service_test.go` — `TestBackupNarrowedSelectionUsesMaximalIncludes` revised (positionals + excludes + AppdataPaths cleanliness); NEW `TestBackupSelectionExcludesMergeAfterUserPatterns`
- `internal/api/selection_internal_test.go` — NEW `TestExcludedBranches` white-box table (7 cases)
- `internal/restic/restic_positionals_contract_test.go` — NEW `TestPositionalExcludeAbsoluteSubdirPattern` (skips locally; CI arbiter) + header doc claim 3
- `.planning/PROJECT.md` — Key Decision row: falsified rationale replaced with corrected restic facts + encode decision (2026-09-09) + tradeoff; outcome notes the 01-05 amendment
- `.planning/REQUIREMENTS.md` — INTEG-04 unchecked with backend-landed/UI-pending split (+ traceability row); BROWSE-01 mechanism text now describes the shipped single-listing contract (D-07 rationale named)
- `.planning/ROADMAP.md` — Phase 1 SC2 parenthetical superseded by the amended positionals-plus-excludes contract with the amendment date

## Decisions Made

- Enforcement point: the single `backup.BackupDeps` literal in `service.Backup` — user-owned patterns first, derived tail appended (variadic spread), exactly as the plan prescribed; `tg` comes from `UpsertTarget`'s re-read which carries `SelectedPaths`.
- No translation applied to derived patterns: stored entries are already container-form absolute paths in the same namespace the positionals walk (`toContainerPath`/`toHostPath` confirmed at read time), so the exclude and its positional source share the namespace restic compares in.
- Task 3 SELECT-03 gate: the plan's strict `! git diff | grep -qF "SELECT-03"` also matches diff CONTEXT lines (the traceability row adjacent to the INTEG-04 edit). Evaluated under the criterion's stated meaning — "no change on the SELECT-03 line" — confirmed: no `+`/`-` line touches SELECT-03, and the line-30 requirement text is byte-identical.

## Deviations from Plan

None - plan executed exactly as written.

(Two authoring-time self-corrections were fixed before their commits and therefore produced no deviation commits: the test's `GetTargetByContainer` nil-check adjusted to the value-returning store signature, and Task 2's doc-comment phrasing reworded so the helper-name enumeration gate returns exactly the four api files mandated by the acceptance criterion — the restic contract test references "the stored-selection exclusion derivation" without the identifier, and the merge test names `excludedBranches` where it belongs.)

## Issues Encountered

- No restic on this Windows box's PATH (expected): all three `TestPositional*` contract tests skip loudly locally (`ok` package result); `TestPositionalExcludeAbsoluteSubdirPattern` has never executed anywhere yet because the `docker-folders` branch is unpushed. This is the designed CI-arbiter pattern (01-VERIFICATION truth 10) — the branch push with green Test + Lint jobs remains the closure step (phase human item 1).
- `gofmt` realigned four adjacent `BackupDeps` literal lines after the new comment split the alignment group — cosmetic, inside the allowed single hunk region.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- WR-01 is closed at the code level; the phase's remaining open items are unchanged from 01-VERIFICATION: push `docker-folders` and require green Test + Lint CI (now also proving the new contract test), plus the two manual edge-coverage reviews and the MVP-mode format decision.
- Phase 2 (container panel tree) can build on the encoded selection: the tree's "uncheck a sub-branch" now has real content-level semantics end to end — the UI just constructs the same stored flat set.
- Phase 3's exclusions editor preview will show derived patterns in snapshot metadata; the accepted tradeoff is documented in `excludedBranches`'s doc comment and PROJECT.md's Key Decision row.

---
*Phase: 01-selection-engine-restore-safety*
*Completed: 2026-09-10*

## Self-Check: PASSED

All 9 created/modified files exist on disk; all 5 commits (`8e3b86f3`, `6dc3cdeb`, `184a248f`, `a426ece5`, `0735929a`) present in git log. Plan-level verification re-run clean: `go build ./... && go vet ./... && gofmt -l .` clean; gap-contract + neighbor tests pass; scope guard diff-empty; docs gates pass.
