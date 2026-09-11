---
phase: 01-selection-engine-restore-safety
plan: 02
subsystem: api
tags: [go, net-http, os-root, browse, containment, directory-listing, symlink-safety]

# Dependency graph
requires:
  - phase: 01-selection-engine-restore-safety (plan 01-01)
    provides: selection encoding in internal/api/selection.go and the normalized flat backupPaths set (adjacent seam; this plan did not need to touch it)
provides:
  - GET /api/browse hardened into the Phase 2 tree's node-listing contract: os.Root kernel-enforced containment layered behind the unchanged paths.Resolve first reject
  - Per-listing additive status field (ok/restricted/missing/error) distinguishing empty from unreadable, error KIND only, generic scrubbed message verbatim
  - 500-entry cap with truncated flag, sort-then-truncate on the filtered slice (deterministic lexically-first page)
  - ?hidden=1 opt-in for dot-prefixed entries, default byte-identical, delta pinned both ways by contract tests
  - browse_contract_test.go + browse_contract_internal_test.go contract test layer (status trio, cap, hidden delta, GOOS-guarded escape/restricted fixtures, classifier table)
affects: [02 (tree UI consumes the listing contract), 01-03, 01-04, restore-safety, FolderBrowser consumers]

# Actuals (#2632) — pairs with the plan's estimate to calibrate future estimates.
# Same estimateTokens scale (chars/4 over the realized diff), never a harness token count.
actuals:
  tokens: 6800
  tasks: 2
  commits: 4

# Tech tracking
tech-stack:
  added: [] # os.Root is Go 1.24+ stdlib (repo floor 1.25.0) — zero new dependencies, per RESEARCH
  patterns:
    - "Layered containment: cheap lexical first-reject (paths.Resolve, byte-identical rejection response) + kernel-enforced os.Root behind it"
    - "Error-KIND classification on the wire: errors.Is on *fs.PathError -> status string; the status never carries the path"
    - "White-box/contract test split: router-harness contract tests in package api_test, unexported-helper tables in *_internal_test.go (package api)"

key-files:
  created:
    - internal/api/browse_contract_test.go
    - internal/api/browse_contract_internal_test.go
  modified:
    - internal/api/handlers.go

key-decisions:
  - "handleBrowse containment is two-layered: the paths.Resolve rejection response stays byte-identical with NO status field (FolderBrowser contract, D-05), while every other read outcome lands in the HTTP 200 envelope carrying a status kind"
  - "classifyReadDirError maps via errors.Is on *fs.PathError (restricted/missing/error); an os.Root escape rejection deliberately lands in the opaque error bucket so an escape attempt never announces itself on the wire (threat T-01-08)"
  - "Listing cap locked at exactly 500 with truncation AFTER the lexical sort of the filtered slice, so a capped listing is a deterministic prefix of the full sorted listing (D-08 / R1)"
  - "hidden opt-in reads only the literal '1' (?hidden=0 behaves as absent); the flag filters, never reorders or fabricates (BROWSE-04)"
  - "Classifier table lives in a sibling white-box file (browse_contract_internal_test.go, package api): the router harness is in package api_test and Go cannot mix both packages in one file — follows the repo's *_internal_test.go convention"

patterns-established:
  - "Status-kind envelope: additive per-listing status field distinguishing ok/missing/restricted/error without changing the generic scrubbed error string"
  - "GOOS-guarded fixture + all-OS unit table pairing: POSIX-only endpoint fixtures skip loudly on Windows dev while the unexported classifier mapping is pinned by a constructed-*fs.PathError table that runs everywhere"

requirements-completed: [BROWSE-01, BROWSE-02, BROWSE-03, BROWSE-04]

# Coverage metadata (#1602) — one entry per shipped deliverable.
coverage:
  - id: D1
    description: "Per-listing status trio: empty dir returns entries:[] + status:'ok'; missing path returns the verbatim generic error + status:'missing'; permission-denied returns the same generic error + status:'restricted' — all in the HTTP 200 envelope"
    requirement: BROWSE-02
    verification:
      - kind: unit
        ref: "tests/internal/api/browse_contract_test.go#TestBrowseStatusOkOnEmpty"
        status: pass
      - kind: unit
        ref: "tests/internal/api/browse_contract_test.go#TestBrowseStatusMissing"
        status: pass
      - kind: unit
        ref: "tests/internal/api/browse_contract_internal_test.go#TestClassifyReadDirError"
        status: pass
      - kind: integration
        ref: "tests/internal/api/browse_contract_test.go#TestBrowseStatusRestricted"
        status: unknown
    human_judgment: true
    rationale: "The endpoint-level permission-denied fixture needs a POSIX filesystem (chmod 000 is a no-op on Windows dev) so it skips locally and proves on Linux CI; the KIND mapping itself is fully proven on every OS by TestClassifyReadDirError over constructed *fs.PathError values"
  - id: D2
    description: "Symlink-safe containment: a symlink inside the mount root pointing outside it is rejected by os.Root instead of listed; the rejection is indistinguishable from any other read failure on the wire"
    requirement: BROWSE-03
    verification:
      - kind: unit
        ref: "tests/internal/api/browse_contract_internal_test.go#TestClassifyReadDirError (os.Root escape rejection stays opaque bucket)"
        status: pass
      - kind: integration
        ref: "tests/internal/api/browse_contract_test.go#TestBrowseSymlinkEscapeRejected"
        status: unknown
    human_judgment: true
    rationale: "The escaping-symlink fixture needs symlink creation (privileged on Windows dev) so it skips locally and proves on Linux CI per the plan's explicit GOOS-guard design; the escape-rejection classification is unit-proven on every OS"
  - id: D3
    description: "Cheap capped per-node listing: exactly 500 entries returns truncated:false, 501+ returns the lexically-first 500 with truncated:true; single ReadDir per request, no recursion, no hasChildren emptiness probe"
    requirement: BROWSE-01
    verification:
      - kind: unit
        ref: "tests/internal/api/browse_contract_test.go#TestBrowseCap"
        status: pass
    human_judgment: false
  - id: D4
    description: "Hidden-entry opt-in delta pinned both ways: default excludes dot-prefixed entries byte-identically, hidden=1 includes them at root and depth (hidden=0 behaves as absent), otherwise identical and identically sorted; mid-dot names never hidden; empty dir stays [] with the flag"
    requirement: BROWSE-04
    verification:
      - kind: unit
        ref: "tests/internal/api/browse_contract_test.go#TestBrowseHidden"
        status: pass
    human_judgment: false
  - id: D5
    description: "FolderBrowser contract preserved: the four pre-existing browse tests pass unmodified and the traversal/absolute-path rejection responses gain no new fields (no status on those branches)"
    requirement: BROWSE-01
    verification:
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestBrowseListsMountRoot"
        status: pass
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestBrowseListsSubpath"
        status: pass
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestBrowseRejectsTraversal"
        status: pass
      - kind: unit
        ref: "tests/internal/api/handlers_test.go#TestBrowseRejectsAbsolutePath"
        status: pass
    human_judgment: false

# Metrics
duration: 19 min
completed: 2026-09-09
status: complete
---

# Phase 01 Plan 02: Browse Node-Listing Contract Summary

**GET /api/browse hardened into the tree's node listing: os.Root containment behind the unchanged lexical reject, additive status trio (ok/missing/restricted/error), 500-entry cap with truncated flag, and a pinned ?hidden=1 opt-in — FolderBrowser contract byte-identical.**

## Performance

- **Duration:** 19 min
- **Started:** 2026-09-09T16:34:21Z
- **Completed:** 2026-09-09T16:53:11Z
- **Tasks:** 2 (both tdd="true" — RED/GREEN each)
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments

- `handleBrowse` now lists through `os.OpenRoot(HostMountRoot)` + `Root.Open` + `ReadDir(-1)`: a symlink planted inside the mount root can no longer be listed through to a location outside it (BROWSE-03, threat T-01-05), while the cheap `paths.Resolve` first reject keeps the traversal/absolute-path rejection response byte-identical (no status field — the four pre-existing contract tests pass unmodified).
- Every read outcome lands in the HTTP 200 envelope with an additive `status` kind via `classifyReadDirError` (`errors.Is` on `*fs.PathError`): `restricted` / `missing` / opaque `error`; empty + `status:"ok"` is the real-empty signal; the "could not read directory" string stays verbatim and the status never carries the path (BROWSE-02, threat T-01-08).
- Listings are capped at `maxBrowseEntries = 500` with `truncated:true` on overflow, truncated AFTER the lexical sort of the filtered slice so a capped page is a deterministic prefix (BROWSE-01, threat T-01-07); no `hasChildren`, no emptiness probe, no recursion (D-07).
- `?hidden=1` opt-in includes dot-prefixed entries at the root and at every depth; absence or any other value (incl. `hidden=0`) keeps today's byte-identical default; the flag filters, never reorders or fabricates (BROWSE-04).
- New contract test layer: 6 endpoint tests in `browse_contract_test.go` + the all-OS `classifyReadDirError` table in `browse_contract_internal_test.go`.

## Task Commits

Each task followed the TDD RED -> GREEN cycle (both tasks carry `tdd="true"`):

1. **Task 1 RED: failing status-trio + containment contract tests** - `6a58fb32` (test)
2. **Task 1 GREEN: os.Root containment + per-listing status** - `dcc625da` (feat)
3. **Task 2 RED: failing listing-cap + hidden opt-in tests** - `7b0c1c42` (test)
4. **Task 2 GREEN: cap + truncated flag + hidden opt-in** - `bf28fe57` (feat)

**Plan metadata:** (see final docs commit)

## Files Created/Modified

- `internal/api/browse_contract_test.go` (created) — endpoint contract tests: status ok-on-empty/missing, GOOS-guarded restricted + escaping-symlink fixtures, hidden delta, cap/truncated
- `internal/api/browse_contract_internal_test.go` (created) — white-box `TestClassifyReadDirError` table over constructed `*fs.PathError` values (runs on every OS)
- `internal/api/handlers.go` (modified) — `handleBrowse` hardened (os.Root, status, hidden, cap, truncated), `classifyReadDirError`, `maxBrowseEntries`; doc comment relocated from its drifted spot onto the function

## Decisions Made

- Containment is layered: `paths.Resolve` stays the cheap FIRST reject with its response byte-identical (no status field — pinned by the untouched existing tests); `os.Root` adds kernel-enforced symlink containment behind it (D-05/D-06).
- An `os.Root` escape rejection lands in the opaque `error` bucket on purpose: an escape attempt must be indistinguishable from any other failure on the wire.
- Truncation happens after the sort on the filtered slice (R1 order), making the cap deterministic rather than directory-order-dependent.
- The hidden opt-in is the literal `"1"` only — a defensive reading so empty/mistyped values cannot silently widen visibility.
- The classifier table lives in a sibling white-box file (package `api`) because the router harness is in package `api_test`; Go cannot mix both in one file, and `*_internal_test.go` is the established repo convention for white-box tests.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Classifier table moved to a sibling white-box test file**
- **Found during:** Task 1 (test layout)
- **Issue:** The plan places ALL Task 1 tests in one new `browse_contract_test.go`, but Go physically cannot mix packages in one file: the plan's router-harness requirement (`newBrowseRouter` + `doJSON`) lives in `package api_test`, while the classifier table targets the unexported `classifyReadDirError` (only reachable from `package api`).
- **Fix:** Endpoint contract tests stayed in `browse_contract_test.go` (package `api_test`, harness reuse as planned); the classifier table went to `browse_contract_internal_test.go` (package `api`), matching the repo's `*_internal_test.go` white-box convention (same split plan 01-01 used for `selection_readers_internal_test.go`).
- **Files modified:** internal/api/browse_contract_internal_test.go (created in addition to the planned file)
- **Verification:** `go test ./internal/api/ -run 'TestClassifyReadDirError' -count=1` green on Windows dev (the OS where the endpoint-level restricted fixture cannot run)
- **Committed in:** 6a58fb32 (Task 1 RED commit)

---

**Total deviations:** 1 auto-fixed (1 blocking/structural, Go test-package mechanics)
**Impact on plan:** None on behavior or contract; one additional test file beyond the plan's `files_modified` list, justified by the language's package rules and consistent with existing repo convention.

## Issues Encountered

- The initial `TestBrowseHidden` fixture expectations omitted the visible `emptydir` entry from the want lists (test bug, not a product bug); fixed before the RED commit so the observed RED failed on the actual missing feature (hidden=1 delta, cap). Never landed in history.
- Environment note (by design, not a defect): `TestBrowseStatusRestricted` and `TestBrowseSymlinkEscapeRejected` skip loudly on this Windows dev box (chmod is a no-op; symlinks need privileges) and prove on Linux CI, per the plan's explicit GOOS-guard design (RESEARCH Pitfall 5). The KIND mapping they depend on is proven on every OS by the `TestClassifyReadDirError` table.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Ready for 01-03 (restore hardening) and 01-04 (PATCH empty-selection guard): this plan touched only `handleBrowse` + its tests; the selection seam from 01-01 is untouched and committed as-is.
- The Phase 2 tree UI can build directly on this contract: expandable-always nodes (no probe), `status` for empty/missing/restricted rendering, `truncated` for paging, `?hidden=1` for show-hidden toggles. `web/` types intentionally unchanged (api.ts updates land in Phase 2 when a consumer exists); `git status --porcelain web/` empty.

---
*Phase: 01-selection-engine-restore-safety*
*Completed: 2026-09-09*

## Self-Check: PASSED

- All created/modified files exist on disk (browse_contract_test.go, browse_contract_internal_test.go, handlers.go).
- All 4 task commits found in git log: 6a58fb32 (test), dcc625da (feat), 7b0c1c42 (test), bf28fe57 (feat).
- Plan verification re-run at close: `go build ./...` clean, `go vet ./...` clean, `gofmt -l .` silent, `go test ./...` full suite green, `git status --porcelain web/` empty, handlers_test.go unmodified.
