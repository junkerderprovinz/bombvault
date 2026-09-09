---
status: testing
phase: 01-selection-engine-restore-safety
source: [01-VERIFICATION.md]
started: 2026-09-09T22:38:24Z
updated: 2026-09-09T22:38:24Z
---

## Current Test

number: 1
name: Push docker-folders to origin and require green Test + Lint CI jobs on the phase HEAD (e688dd94)
expected: |
  Test job (installs restic 0.17.3) passes TestPositionalExcludesKeepSourceDir, TestPositionalAbsolutePathPreserved, TestBrowseStatusRestricted, TestBrowseSymlinkEscapeRejected plus the full suite; Lint job passes golangci-lint (never run on these commits).
awaiting: user response

## Tests

### 1. Push docker-folders to origin and require green Test + Lint CI jobs on the phase HEAD (e688dd94)
expected: Test job (installs restic 0.17.3) passes TestPositionalExcludesKeepSourceDir, TestPositionalAbsolutePathPreserved, TestBrowseStatusRestricted, TestBrowseSymlinkEscapeRejected plus the full suite; Lint job passes golangci-lint (never run on these commits). Both CI jobs have never seen this branch (no upstream on origin).
result: [pending]

### 2. Manually review the 2 edge-coverage items deliberately left unresolved (edge-coverage.json): BROWSE-02 'unclassified' and SELECT-02 'unclassified'
expected: A human either accepts the planned interpretation (BROWSE-02: status-trio semantics per CONTEXT browse Q2; SELECT-02: zero-migration semantics per requirement text + encoding Q1/Q3, covered by the legacy/[] round-trip tests and untouched migrate.go) or files a correction.
result: [pending]

### 3. Confirm MVP-mode intent: the phase goal is not in User Story form (gsd user-story.validate returned false), so no User Flow Coverage table was produced
expected: Either accept this verification's basis (the 5 numbered ROADMAP success criteria, all engine-level and testable) or run /gsd mvp-phase 1 to set a User Story goal and re-verify in MVP form.
result: [pending]

### 4. Bookkeeping review of three documentation drifts found during verification
expected: |
  (a) PROJECT.md Key Decision row still carries WR-02's falsified rationale ('restic excludes do not apply to positional sources — exclude-based encoding disqualified') and now contradicts both the corrected code comments and the gap-closure decision; (b) REQUIREMENTS.md marks INTEG-04 [x]/'Complete' while ROADMAP says INTEG-04 completes with UI semantics in Phase 3; (c) REQUIREMENTS.md BROWSE-01 text still names the 'hasChildren hint via emptiness probe' mechanism the phase deliberately rejected (D-07: N+1 latency multiplier). Doc updates queued with the gap-closure plan or applied directly; none affects shipped code behavior.
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
