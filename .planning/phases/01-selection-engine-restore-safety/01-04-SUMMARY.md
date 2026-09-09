---
phase: 01-selection-engine-restore-safety
plan: 04
subsystem: api
tags: [selection, exclusions, empty-selection-guard, coded-envelope, mounts, restic, tdd, go]

# Dependency graph
requires:
  - phase: 01-selection-engine-restore-safety (plans 01-01/01-02/01-03, same phase)
    provides: selection.go primitives (SplitExclusion per D-02), the stored canonical flat set, os.Root browse contract, mapRestorePaths + RestoreDeps.SkippedPaths, and the router/test harness they established
provides:
  - GET /api/containers/{name}/mounts renders stored exclusions as a first-class top-level excluded array in HOST form, never as stale custom paths (closes the plan 01-01 interim window)
  - ContainerMounts class split: includes drive Selected/auto-fallback/custom loop; exclusions bypass them entirely; the auto-detection fallback stays keyed on the RAW list (exclusions-only keeps explicit-none semantics, L4)
  - PATCH empty-selection guard: tree-source [] over a stored non-empty selection is refused with errEmptySelection -> {ok:false, code:"empty-selection"} in the HTTP 200 envelope, prior state preserved (threats T-01-13/14)
  - Optional selectionSource wire field (pointer, only the literal "tree" carries meaning) + codedFailEnvelope helper beside failEnvelope
  - PROJECT.md Key Decisions: positional-targets + future-children allowlist rows logged with outcomes; tree-as-view -> Decided (Phase 1); lazy-load -> Backend landed (Phase 1)
affects: [phase-3-tree-ui, phase-4-file-sets, exclusion-review-ui, verify-work]

# Actuals (#2632) — pairs with the plan's `estimate` to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 7728    # 30,911 diff chars over the 5 changed files (460 insertions, 25 deletions)
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Coded failure envelope: codedFailEnvelope(err, code) adds a machine-routable code to the house HTTP 200 {ok:false, error} shape so a UI branches on failure KIND without parsing text"
    - "Strictly source-gated destructive-intent guard: the refusal keys on the literal selectionSource value only — never payload sniffing — keeping legacy and unknown sources byte-compat"
    - "Reader class split at the mounts boundary: SplitExclusion before any consumer; exclusions surface as reviewable state (excluded array, host form) instead of masquerading as stale custom paths"

key-files:
  created: []
  modified:
    - internal/api/service.go
    - internal/api/handlers.go
    - internal/api/handlers_test.go
    - internal/api/service_test.go
    - .planning/PROJECT.md

key-decisions:
  - "The empty-selection guard is strictly source-gated (literal \"tree\" only): no heuristic sniffing of tree-shaped payloads, so legacy clients and unknown future source values keep today's clears-to-auto-detect behavior byte-for-byte (RESEARCH Open Question 2)"
  - "The refusal returns BEFORE any store write, so a refused deselect-everything leaves the prior selection byte-identical (asserted in TestEmptySelectionGuard)"
  - "excluded is an empty slice (never null), nil-guarded in handleContainerMounts exactly like the existing mounts/custom guards; MountInfo.Selected stays include-only"
  - "selectionSource is a transient intent signal on the request, never persisted as a second representation of the selection (assumption delta: no-change to the one flat backupPaths set)"

patterns-established:
  - "codedFailEnvelope(err, code): the pattern for any future domain refusal that needs machine-routable guidance (File Sets reuse it in Phase 4)"
  - "Optional wire fields that gate behavior MUST still be declared on the body struct — decodeBody's DisallowUnknownFields would otherwise reject every payload that sends them"

requirements-completed: [INTEG-04]

coverage:
  - id: D1
    description: "ContainerMounts class split: exclusions render in the mounts endpoint's top-level excluded array in HOST form, never as stale custom paths; Selected computed from includes only; include-only responses unchanged apart from the additive empty (never null) excluded field"
    requirement: INTEG-04
    verification:
      - kind: integration
        ref: "internal/api/handlers_test.go#TestMountsExcluded (mixed / exclusions-only / include-only through the real router)"
        status: pass
      - kind: unit
        ref: "internal/api/service_test.go#TestContainerMountsNoPhantomAppdata + TestContainerMountsFlagsMissingCustomPath + TestServiceContainerMountsAndSelection (signature-change call sites, unmodified assertions)"
        status: pass
    human_judgment: false
  - id: D2
    description: "PATCH empty-selection guard: tree-source [] over a prior non-empty selection refused with code:\"empty-selection\" and prior state preserved; legacy [] clears byte-for-byte; exclusions-only tree save accepted; unknown source ignored; fresh container passes"
    requirement: INTEG-04
    verification:
      - kind: integration
        ref: "internal/api/handlers_test.go#TestEmptySelectionGuard (all five behavior cases through the real router)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Widened SetBackupPaths(ctx, name, hostPaths, selectionSource) keeps every plan-01 behavior identical when selectionSource is empty (normalization, exclusions-only explicit-none, [] clears to auto-detect)"
    requirement: INTEG-04
    verification:
      - kind: unit
        ref: "internal/api/service_test.go#TestSetBackupPathsLegacySourceKeepsPlan01Behavior"
        status: pass
      - kind: integration
        ref: "internal/api/service_test.go#TestSetBackupPathsMixedSelectionRoundTrip (unmodified, still green)"
        status: pass
    human_judgment: false
  - id: D4
    description: "PROJECT.md Key Decisions logs the positional-targets decision and the future-children allowlist semantic with outcomes recorded; tree-as-view -> Decided (Phase 1); lazy-load -> Backend landed (Phase 1); ExcludesEditor coexistence left pending"
    verification:
      - kind: other
        ref: "grep .planning/PROJECT.md (rows at PROJECT.md:71-75, outcomes filled)"
        status: pass
    human_judgment: false

# Metrics
duration: 17min
completed: 2026-09-09
status: complete
---

# Phase 1 Plan 4: Exclusion Visibility + Empty-Selection Guard Summary

**Mounts endpoint renders stored exclusions as a first-class host-form `excluded` array (never stale custom paths), and a tree-source deselect-everything is refused at the PATCH boundary with a machine-routable `code:"empty-selection"` — the backend half of INTEG-04, plus the roadmap-prescribed Key Decisions bookkeeping**

## Performance

- **Duration:** 17 min
- **Started:** 2026-09-09T18:32:24Z
- **Completed:** 2026-09-09T18:49:34Z
- **Tasks:** 2 (both TDD: separate RED and GREEN gate commits per task)
- **Files modified:** 5

## Accomplishments
- `ContainerMounts` splits the stored flat selection via plan 01's `SplitExclusion` (D-02 — no inline prefix logic): includes keep driving `Selected`, the auto-detection fallback and the custom loop; exclusions bypass the custom loop entirely and return as a new third value already translated to host form via `toHostPath`. The fallback stays keyed on the RAW list, so an exclusions-only selection keeps its explicit-none semantics (L4) instead of flipping to auto-detection
- `handleContainerMounts` adds the additive `excluded` field to the okEnvelope payload, nil-guarded to `[]` exactly like the existing mounts/custom guards — closing the plan 01-01 interim window where stored `!`-exclusions rendered as stale `Exists:false` custom paths (Pitfall 3)
- `service.SetBackupPaths` widens to `(ctx, name, hostPaths, selectionSource)`: after normalization, a literal `"tree"` source over an empty normalized result with a stored non-empty selection returns the new `errEmptySelection` sentinel BEFORE any store write; `handlePatchContainer` classifies via `errors.Is` and answers with the new `codedFailEnvelope(err, "empty-selection")` in the house HTTP 200 envelope. Legacy ("" source) clients, unknown source values, exclusions-only saves, and fresh containers all keep their specified behavior
- PROJECT.md Key Decisions: the positional-targets compilation decision and the future-children allowlist semantic are logged with outcomes; the tree-as-view row is now "Decided (Phase 1)" and lazy-load "Backend landed (Phase 1)"; the ExcludesEditor coexistence row stays pending (later-phase concern)

## Task Commits

Each task was committed atomically with separate TDD gates:

1. **Task 1 (tracer) RED: mounts excluded-array contract tests** - `148885ab` (test)
2. **Task 1 (tracer) GREEN: render stored exclusions as the mounts excluded array** - `56e01656` (feat)
3. **Task 2 RED: empty-selection guard contract tests** - `300c6663` (test)
4. **Task 2 GREEN: refuse tree-source deselect-everything with a coded envelope + Key Decisions** - `bd04241a` (feat)

**Tracer feedback gate:** after committing Task 1, its verify was re-run end-to-end (`go build ./... && go test ./internal/api/ -run 'TestContainerMounts|TestMountsExcluded' -count=1`, plus the full `internal/api` package suite) and passed before any Task 2 work — expanding on a proven slice.

**Plan metadata:** pending (docs commit follows this SUMMARY)

_Note: TDD tasks carry two commits each (test → feat). No REFACTOR commits were needed — the GREEN implementations landed clean._

## Files Created/Modified
- `internal/api/service.go` — `errEmptySelection` sentinel; `SetBackupPaths` selectionSource-aware guard; `ContainerMounts` class split returning exclusions host-form (3 slices + error)
- `internal/api/handlers.go` — optional `SelectionSource *string` body field (doc comment: only "tree" is meaningful, others ignored, SPA+server one binary); `codedFailEnvelope` beside `failEnvelope`; nil-guarded `excluded` in the mounts response
- `internal/api/handlers_test.go` — `TestMountsExcluded` (mixed/exclusions-only/include-only) and `TestEmptySelectionGuard` (five cases), both through the real router
- `internal/api/service_test.go` — four existing `ContainerMounts` call sites updated for the signature; five `SetBackupPaths` call sites pass the legacy `""`; new `TestSetBackupPathsLegacySourceKeepsPlan01Behavior`
- `.planning/PROJECT.md` — Key Decisions rows added/updated with outcomes

## Decisions Made
- Guard is strictly source-gated on the literal `"tree"`: no heuristic sniffing of tree-shaped payloads, per RESEARCH Open Question 2's resolved recommendation — the only alternative (payload sniffing) was locked out by CONTEXT INTEG-04 Q1
- The refusal fires before any persistence, making "prior state preserved" structural rather than best-effort
- `excluded` joins the envelope as an additive field with the existing nil-guard style; no other response field changed, so existing typed clients are untouched (no web/ changes in this plan, per phase scope)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- During Task 2's RED run, the "legacy empty list still clears to auto-detection" subtest passed while the four discriminating subtests failed — expected and intended: that subtest pins today's byte-compat behavior (the regression guarantee), it is not a guard behavior. The RED signal was carried by the four failures, all caused by `decodeBody`'s DisallowUnknownFields rejecting the then-undeclared `selectionSource` field — exactly the trap the plan's `<action>` calls out.
- `TestPatchContainer` (named in the task's verify command) has no matching tests in the package — Go reports "no tests to run" for that alternation and exits 0; the guard and signature coverage lives under `TestEmptySelectionGuard` / `TestSetBackupPaths*` as the plan's acceptance criteria specify.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 1 is now complete on the backend: the selection engine (encode, browse, restore, visibility, guard) is green end-to-end; the phase gate can verify the closed 01-01 interim window (exclusions no longer render as stale custom paths)
- Phase 2 (tree UI) builds on the `excluded` array, `Selected` include-only flags, and the `code:"empty-selection"` machine signal for routing guidance
- Phase 3 (INTEG-04 UI) documents the deselect-everything flow around the coded refusal; exit-from-all-deselected remains deferred there (D-12)
- File Sets (Phase 4) reuse the same `codedFailEnvelope` and class-split patterns via the generic selection helpers

## Self-Check: PASSED

All 5 modified files verified on disk; all 4 commit hashes verified in git log (148885ab, 56e01656, 300c6663, bd04241a). Plan-level gates green at final HEAD: `go build ./...`, full `go test ./...` (restic-dependent tests skip locally, prove on CI), `go vet ./...`, `gofmt -l .` prints nothing, `git status --porcelain web/` empty. Task acceptance criteria all traced PASS via source assertions + test runs.

---
*Phase: 01-selection-engine-restore-safety*
*Completed: 2026-09-09*
