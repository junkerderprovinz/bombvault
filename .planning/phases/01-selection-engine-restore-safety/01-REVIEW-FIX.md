---
phase: 01-selection-engine-restore-safety
fixed_at: 2026-09-09T22:20:00Z
review_path: D:/code/bombvault/.planning/phases/01-selection-engine-restore-safety/01-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 1: Code Review Fix Report

**Fixed at:** 2026-09-09T22:20:00Z
**Source review:** D:/code/bombvault/.planning/phases/01-selection-engine-restore-safety/01-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (per fix directive: WR-02, IN-01, IN-02)
- Fixed: 3
- Skipped: 0

## Fixed Issues

### WR-02: Locked position L14 is justified by a factually wrong restic claim

**Files modified:** `internal/api/selection.go`, `internal/api/service_test.go`
**Commit:** 247e81af
**Applied fix:** Comments only — no behavior change. Both comments (the `includesOnly` doc comment in `selection.go:118-131` and the `TestBackupNarrowedSelectionUsesMaximalIncludes` doc comment in `service_test.go:2298-2306`) claimed "restic excludes do not apply to positional sources," which the phase's own contract test `TestPositionalExcludesKeepSourceDir` disproves (excludes filter content WITHIN positional sources while `Paths` keep the positional verbatim). Both now state the real product-level rationale for the L14 lock: restic excludes DO filter content within positional sources, but engine-derived `--exclude` patterns would land in the snapshot's restic `Excludes` metadata — user-owned surface managed through the exclusions editor (STACK.md "What NOT to Use") — so selection-derived patterns are deliberately kept out of it.

### IN-01: `storedDataIsGone` stats the raw stored list, including `!`-prefixed entries that can never exist

**Files modified:** `internal/api/service.go`, `internal/api/selection_readers_internal_test.go`
**Commit:** 7d75a124
**Applied fix:** The stat loop in `storedDataIsGone` (`service.go:3972-3996`) now measures `includesOnly(existing.SelectedPaths)` instead of the raw list, matching the reader convention every other consumer uses; the `AppdataPaths` fallback is unchanged for an empty stored selection. Explicit-none classification semantics preserved exactly as recorded in STATE.md: non-empty raw + zero includes = NOT gone, checked before any stat.

**Process note (TDD-light, as directed):** the behavior fix and the test extension were committed together in one commit (`7d75a124`) — acceptable per the directive since this finding is a refinement. Both new test cases were traced against the OLD code before the fix to guarantee PASS/FAIL outcomes are preserved: (1) an exclusions-only selection whose captured `AppdataPaths` has also vanished stays NOT gone — under the old code the explicit-none early return already fired before any stat; (2) a mixed selection whose include vanished IS gone — under the old code the `"!"`-literal entries only ever voted "gone", so the outcome is identical. All three pre-existing case outcomes in `TestStoredDataIsGoneClassifiesExplicitNone` (exclusions-only → not gone; mixed with existing includes → not gone; vanished prefix-free → gone) are unchanged and verified passing.

### IN-02: Inaccurate `//nolint:gosec // G706` justifications on browse-path log lines

**Files modified:** `internal/api/handlers.go`
**Commit:** 6ba0431a
**Applied fix:** Comments only — no code change. Both justifications (`handlers.go:4276` and `:4288`) claimed "no raw user bytes reach the log formatter," which is false: `rel` is client-influenced (the request's `path` query parameter). Both now credit the actual safety mechanism: `rel` is client-influenced but `paths.Resolve`-validated upstream, and `%q` escapes quotes/backslashes/newlines/control chars — no log-injection or format-string surface. This prevents a future edit from preserving the (wrong) stated rationale while dropping the (load-bearing) escaping.

## Deferred Issues

### WR-01: Mixed include+exclude selections are stored and advertised as exclusions, but the engine silently backs the "excluded" branch up

**File:** `internal/api/selection.go:118-124`
**Status:** DEFERRED — routed by explicit user decision to the planned gap-closure plan (encode stored exclusions as restic `--exclude` patterns).
**Rationale:** Fixing WR-01 inline would contradict that user decision. The plan's chosen approach (option 2 "Encode" from the review) is viable precisely because WR-02's false claim is now corrected in the code comments: restic excludes demonstrably filter content within positional sources (`TestPositionalExcludesKeepSourceDir`), so encoding stored exclusions as `--exclude` patterns enforces the deselection content-wise. The design caveat the review flags — derived patterns land in the snapshot's user-owned `Excludes` metadata — is now accurately documented at the L14 lock site (`selection.go`) so gap closure engages with the real tradeoff.

## Verification

**Where the gates ran:** in the isolated review-fix worktree `D:/code/bombvault/.claude/worktrees/rf-01-67144-1788991848` (Go-only gates; no node_modules required — numbers are reproducible from any checkout at the same commits).

- `go build ./...` — OK
- `go vet ./internal/api/` — OK
- `go test ./internal/api/ -count=1` — PASS (ok, 44.8s; includes the extended `TestStoredDataIsGoneClassifiesExplicitNone` and all pre-existing guard/reader tests)
- `gofmt -l .` — prints nothing

Per-fix verification: WR-02 and IN-02 are comment-only changes verified by re-read + gofmt; IN-01 additionally verified by targeted test runs (`TestStoredDataIsGoneClassifiesExplicitNone`, `TestConfiguredBackupPathsSplitsExclusions`, all `TestEmptyBackup*`) before its commit.

**Commits (on `gsd-reviewfix/01-67144`, fast-forwarded to `docker-folders` by cleanup):**
- `247e81af` fix(01): WR-02 correct factually wrong restic claim in locked L14 comments
- `7d75a124` fix(01): IN-01 stat the includes-only view in storedDataIsGone
- `6ba0431a` fix(01): IN-02 correct G706 nolint justifications to name the real safety mechanism

---

_Fixed: 2026-09-09T22:20:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
