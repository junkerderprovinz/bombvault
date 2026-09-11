---
phase: 01-selection-engine-restore-safety
reviewed: 2026-09-10T12:00:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - internal/api/browse_contract_internal_test.go
  - internal/api/browse_contract_test.go
  - internal/api/foreign_internal_test.go
  - internal/api/handlers.go
  - internal/api/handlers_test.go
  - internal/api/panic_recovery_test.go
  - internal/api/restore_selection_test.go
  - internal/api/selection.go
  - internal/api/selection_internal_test.go
  - internal/api/selection_readers_internal_test.go
  - internal/api/selection_test.go
  - internal/api/service.go
  - internal/api/service_test.go
  - internal/api/stacks_test.go
  - internal/backup/orchestrator.go
  - internal/backup/orchestrator_test.go
  - internal/restic/restic_args_test.go
  - internal/restic/restic_positionals_contract_test.go
  - internal/store/targets.go
findings:
  critical: 0
  warning: 1
  info: 4
  total: 5
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-09-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Reviewed the phase 1 selection engine and restore-safety work: the `!`-prefixed
flat selection encoding (`internal/api/selection.go`), the save/read paths
(`service.SetBackupPaths`, `ContainerMounts`, the stored-data guards), the WR-01
gap closure (`excludedBranches` wired into `BackupDeps.Excludes` at
`service.go:4243`), the RESTORE-01 snapshot-path mapping (`mapRestorePaths` +
`chosenSnapshot` + `SkippedPaths` run note), and the additive `/api/browse`
extension (os.Root containment, status kinds, hidden opt-in, 500-entry cap).

The core invariants hold under trace: positionals are always the maximal-root
includes (`configuredBackupPaths` → `includesOnly`; lock L1/L14 respected — no
exclusion ever becomes a positional, restore argv carries no excludes); argv
discipline holds (`BackupArgs` puts `--exclude` before `--`, positionals after);
`UpsertTarget`'s re-read really does carry `SelectedPaths`, so the WR-01 wiring
is not silently dead; normalization is idempotent and order-canonical; the
empty-selection guard returns before any store write; the exclusions-only
explicit-none state is correctly excluded from the auto-detect fallback, the
#181 "gone" guard, and the empty-selection refusal. House rules are respected
in the tests (no new detached goroutines; POSIX-only fixtures skip loudly).
`gofmt`, `go build`, and the targeted pure-Go tests all pass locally.

One warning (a defense-in-depth asymmetry in the restore path) and four info
items. No critical findings.

## Warnings

### WR-01: Mapped restore selectors bypass the mount-root revalidation applied to the stored list

**File:** `internal/api/service.go:5496-5521`
**Issue:** `prepareRestoreForTarget` re-validates every stored path with
`paths.Within(s.cfg.HostMountRoot, p)` (lines 5496-5501, "defense-in-depth in
case the DB was tampered with"), but the list it actually hands to the
destructive phase is the MAPPED list (`appdataForRestore = mapped`, line 5521)
derived from `mapRestorePaths(tg.AppdataPaths, chosen.Paths)` — i.e. from
restic snapshot metadata, a different (and lower-trust) source than the DB row.
The mapped values are ancestors or descendants of validated stored paths, so
they are lexically contained in practice, and a `..`-bearing crafted value
fails closed at the orchestrator's SEC check (`orchestrator.go:712-716`) — but
the guarantee now rests on transitive reasoning instead of the explicit check
the design comment promises. Pass 2 can also emit a raw (uncleaned) snapshot
metadata string as the selector/target, and the mount root itself is reachable
as an ancestor of a validated path. Given the house posture is explicit
re-validation of everything crossing into a destructive restore, the output
list should get the same check its input gets.

**Fix:**
```go
chosen := chosenSnapshot(snaps, snapshotID)
mapped, skipped := mapRestorePaths(tg.AppdataPaths, chosen.Paths)
...
// Same defense-in-depth as the stored list above, now over the mapped
// selectors: they come from the snapshot's recorded Paths (repo metadata),
// not the DB row the loop above validated.
for _, q := range mapped {
    if !paths.Within(s.cfg.HostMountRoot, q) {
        return containerRestorePlan{}, errors.New("a mapped restore path is outside the host mount, so refusing to restore")
    }
}
appdataForRestore = mapped
```

## Info

### IN-01: Ancestor-fallback mapping restores more than the current selection, and only the under-restore is reported

**File:** `internal/api/selection.go:252-278`, `internal/backup/orchestrator.go:958-980`
**Issue:** When a stored path maps only via the pass-2 longest-ancestor
fallback, the restore replays the FULL ancestor subtree from the snapshot —
including branches the user has since deselected (e.g. a snapshot taken before
a narrowing brings the excluded branch back). The behavior is correct (a
snapshot restore restores that snapshot's content) and the mechanism is
documented, but reporting is asymmetric: `skippedPathsNote` tells the operator
what was left OUT, while nothing records that a path was restored via an
ancestor and therefore covered MORE than the stored selection claims. For a
DR-audit channel built precisely to prevent "success" being misread as
"everything as configured", this is a one-line doc/note gap.

**Fix:** Either extend the doc comment in `mapRestorePaths` to state the
over-restore consequence explicitly, or (better) tag pass-2 mappings and add
them to the run-record note, e.g. "...; N stored path(s) were restored via
their snapshot ancestor subtree, which may include more than the current
selection."

### IN-02: Legacy nested snapshot Paths can produce overlapping restore selectors

**File:** `internal/api/selection.go:243-251`
**Issue:** Pass 1 maps every snapshot path at-or-below a stored path, and
pre-normalization selections (stored before this phase; `SetBackupPaths` did
not prune maximal roots) could yield snapshots whose `Paths` are nested
(e.g. `["/a", "/a/b"]`). With a now-normalized stored `["/a"]`, both snapshot
paths map, so `RestorePaths` runs `restore <id>:/a --target /a` and then the
fully-overlapping `restore <id>:/a/b --target /a/b`. Content-wise idempotent
(same data rewritten), so no corruption — just redundant destructive-phase
work on old data. Self-heals as new snapshots replace old ones.

**Fix:** Optional: in pass 1, skip a snapshot path already covered by an
earlier mapped path (`isStrictDescendant(q, mappedSoFar)`), or note it as
accepted in the function doc.

### IN-03: Empty-selection guard fails open on a store read error

**File:** `internal/api/service.go:3868-3872`
**Issue:** The T-01-13 guard treats any `GetTargetByContainer` error
(identical to the no-rows case) as "nothing to protect" and lets the clear
proceed. For a genuine (transient) DB error this silently disables the
protection the feature exists to provide. Low impact — the subsequent
`SetBackupPaths` write would most likely fail on the same fault — but the
guard's contract ("a non-empty selection is stored to protect") is not
distinguishable from "read failed" in the current shape.

**Fix:** Distinguish `errors.Is(gErr, sql.ErrNoRows)` (proceed — fresh
container) from other errors (return the wrapped store error rather than
failing the guard open).

### IN-04: Exclusion branches under vanished includes emit no-op `--exclude` patterns into snapshot metadata

**File:** `internal/api/service.go:4243`, `internal/api/selection.go:185-206`
**Issue:** `excludedBranches` qualifies an exclusion against ALL includes in
the raw stored list, but the positionals come from
`onlyExistingPaths(includesOnly(...))`. When an include folder has vanished
from disk while its excluded sub-branch entry remains stored, the derived
pattern is emitted for a positional that no longer exists — a harmless no-op
to restic, but it lands in the snapshot's Excludes metadata (the same
user-owned surface the WR-01 tradeoff already accepted) as a machine-derived
pattern matching nothing. Cosmetic/metadata hygiene only.

**Fix:** Optional: intersect the include set with the `effective` list before
deriving (`excludedBranches` against only the includes that survived the
existence filter), or accept and document it next to the existing
Excludes-metadata tradeoff paragraph.

---

_Reviewed: 2026-09-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
