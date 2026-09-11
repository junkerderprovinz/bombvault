---
phase: 01-selection-engine-restore-safety
fixed_at: 2026-09-10T10:35:00Z
review_path: .planning/phases/01-selection-engine-restore-safety/01-REVIEW.md
iteration: 1
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 01: Code Review Fix Report

**Fixed at:** 2026-09-10T10:35:00Z
**Source review:** .planning/phases/01-selection-engine-restore-safety/01-REVIEW.md (commit 48168e90)
**Iteration:** 1

**Summary:**
- Findings in scope (critical + warning): 1
- Fixed: 1
- Skipped: 0
- Out of scope (Info, documented only): IN-01, IN-02, IN-03, IN-04

This report replaces the stale REVIEW-FIX.md from the previous review cycle
(WR-02/IN-01/IN-02), which described a different REVIEW.md and was left on disk.

Operational note: on startup this fixer run detected and completed the recovery
of a prior interrupted run (orphan worktree `rf-01-112155-...` and branch
`gsd-reviewfix/01-112155` removed via the recovery sentinel) before starting.

## Fixed Issues

### WR-01: Mapped restore selectors bypass the mount-root revalidation applied to the stored list

**Files modified:** `internal/api/service.go`
**Commit:** 04363500
**Applied fix:** Added an explicit `paths.Within(s.cfg.HostMountRoot, q)`
revalidation loop over the MAPPED selector list in `prepareRestoreForTarget`
(`internal/api/service.go:5530-5535`), placed between the per-path skip logging
and the `appdataForRestore = mapped` assignment — exactly where the destructive
phase receives its list. A mapped selector outside the host mount root now
refuses the restore with "a mapped restore path is outside the host mount, so
refusing to restore", mirroring the stored-list check ten lines above
(same error shape, same `%q`-quoted server-side log line with the house
`//nolint:gosec // G706` justification). A "why" comment cites the phase 01
review WR-01 and records the deliberate edge semantics: `Within` cleans both
sides (so uncleaned snapshot metadata strings are judged on their cleaned
form), and the mount root itself fails the strict-within check — the intended
fail-closed outcome for a pass-2 ancestor selector that would otherwise replay
the whole root subtree in a single-container restore.

Verification (all run in the isolated review-fix worktree, same commit content
that was fast-forwarded to `docker-folders` — reproducible from the main
checkout after teardown):

- `gofmt -l internal/api/service.go` — clean (no output)
- `go build ./...` — pass
- `go vet ./internal/api/` — pass
- `go test ./internal/api/ -run 'TestRestoreSelection|TestPrepareRestoreSeam|TestMapRestorePaths|TestForeignContainerRestore|TestLocalContainerRestore'` — pass

Existing fixtures were traced before applying the fix: every restore-selection
and foreign-container test maps to a selector strictly inside the fixture
`HostMountRoot` (`/host/user`), and the empty-intersection case aborts with
"nothing to restore" before the new loop matters, so no test relied on the
removed asymmetry.

## Skipped Issues

None — the single in-scope finding was fixed. The four Info findings (IN-01
through IN-04) are outside the `critical_warning` fix scope and remain
documented in 01-REVIEW.md for a later cycle.

---

_Fixed: 2026-09-10T10:35:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
