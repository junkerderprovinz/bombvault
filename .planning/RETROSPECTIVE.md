# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Tree-Based Sub-Folder Backup Selection

**Shipped:** 2026-09-11
**Phases:** 4 | **Plans:** 15 | **Sessions:** ~15 plan executions (orchestrator + subagent per plan)

### What Was Built
- One normalization contract (`internal/api/selection.go`) over the unchanged flat `backupPaths` set — bare + `!`-prefixed entries, maximal-root positionals, descendant exclusions enforced as restic `--exclude` at the single `BackupDeps.Excludes` site; zero migration for deployed instances
- One tree component (`SelectionTree.tsx` + `selectionTree.ts`) powering container mounts AND file sets — lazy tri-state nodes derived purely from the (includes, exclusions) sets, full APG keyboard operation, D-04/D-06 zero-include guards, one-deep serialized PATCH queue with live-mirror revert
- `/api/browse` hardened into the tree's node listing: os.Root containment, status trio (ok/missing/restricted/error), 500-entry cap + truncated flag, pinned `?hidden=1`
- Restore safety: selectors map onto the chosen snapshot's recorded Paths (two-pass longest-prefix), empty intersections abort before destructive teardown, per-path skips as scrubbed run notes; restic 0.17 positional behaviors contract-pinned for CI
- Trust controls: per-root "{n} paths" preview agreeing with restic positionals, reviewable exclusions disclosure, fail-tone confirmed Reset exit, per-root CACHEDIR.TAG toggle → `--exclude-caches` (migration v100)
- File sets parity: three-state pointer PATCH (absent/[]/list, 64-cap, containment), path-edit clear-wins to SQL NULL, NULL-seeded synthetic root with zero writes, D-08 in-place restore guard

### What Worked
- Keystone-first ordering: Phase 1 proved maximal-roots ↔ restic positional targets end-to-end (engine + tests, no UI) before any component existed — Phases 2-4 became assembly, never re-litigation
- The zero-second-implementation lock held by construction: additive-optional props on `SelectionTree` let Phase 4 mount the exact Phase 2/3 component; the integration checker found no duplicated "!" logic in `web/src`
- Locked decisions written once (CONTEXT.md → PROJECT.md Key Decisions) — plans across 4 phases never re-argued the encoding, the queue, or the persistence format
- Contract-pinning hostile behaviors (restic excludes-with-positionals, os.Root escapes) as tests that skip locally and prove on CI kept Windows dev honest without weakening Linux CI

### What Was Inefficient
- GSD frontmatter field-name drift: `requirements-completed:` (dash, not underscore) defeated repeated `summary-extract` queries before a direct grep settled it
- The deferred-items CLI writer rejects this repo's heading-delimited shape (#3457): two failed acknowledge attempts before reading the scanner source and editing the files directly (bare `Status:` value + HTML-comment annotation)
- The milestone-complete CLI archives an audit named `v{version}-MILESTONE-AUDIT.md`; the file was first written as `v1-MILESTONE-AUDIT.md` and needed a manual `mv`
- All four phase VALIDATION.md files closed as nyquist `draft` (tracked as tooling debt, not phase failures) — nothing during execution forces the nyquist pass until audit time

### Patterns Established
- Three-state pointer fields for selection PATCH bodies: absent = untouched, `[]` = coded refusal, list = atomic validated overwrite — explicit shape beats payload sniffing
- One-deep serialized save queue per editor; failure reverts re-derive from the live mirror by set-difference inverse, never a captured snapshot
- Node classification from stored sets only — collapsed/never-loaded subtrees classify correctly with zero browsing
- Exclusions are a distinct class in the flat set: never positionals, never derived flags except at the single `BackupDeps.Excludes` enforcement site
- deferred-items entries carry a bare `Status:` value (strict equality in the scanner) with explanations in HTML comments

### Key Lessons
1. Proving the riskiest seam first (selection encoding ↔ restic argv) without UI cost the least and de-risked everything after — the tracer-first instinct is correct for contract-heavy features
2. Compatibility locks (zero migration, flat set) paid for themselves: every phase was additive because the storage format never moved
3. When tooling rejects a file shape, read the scanner/parser source — the canonical format lives in code, not docs
4. UI semantics that document a backend guard (INTEG-04) belong on the same phase boundary as the guard; splitting them across phases leaves a requirement artificially "partial" at every intermediate verification

### Cost Observations
- Model mix: not tracked (executions dispatched to subagents with inherited model)
- Sessions: ~15 plan executions + audit + closeout
- Notable: keystone-first kept re-planning at zero; the only rework in the milestone was review-driven gap closure (01-05 exclusion enforcement), absorbed inside its phase

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | ~15 | 4 | Keystone-first decomposition; locked-decision docs reused across phases without re-litigation |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|-------------------|
| v1.0 | — (not aggregated; all phases green incl. 04-UAT 22/22, 53/53 requirement checks) | — | 0 (stdlib + existing toolchain only) |

### Top Lessons (Verified Across Milestones)

1. (First milestone on record — lessons above become the baseline to verify against.)
