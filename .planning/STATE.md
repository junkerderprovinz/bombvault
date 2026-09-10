---
gsd_state_version: 1.0
current_phase: 1
current_phase_name: selection-engine-restore-safety
status: executing
stopped_at: Completed 01-04-PLAN.md
last_updated: "2026-09-10T09:23:13.135Z"
last_activity: 2026-09-09
last_activity_desc: Phase 01 execution started
state_head: edc5484551c59c2c1985204f98205357f52e9c90
progress:
  total_phases: 4
  completed_phases: 0
  total_plans: 5
  completed_plans: 4
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-09)

**Core value:** Every container, VM, and config on the host can be backed up consistently and restored completely — a dead server is rebuilt from the restic repo alone.
**Current focus:** Phase 01 — Selection Engine & Restore Safety

## Current Position

Phase: 1 (selection-engine-restore-safety) — READY TO EXECUTE
Plan: 4 of 4
Status: Ready to execute
Last activity: 2026-09-09 — Phase 01 execution started

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

**Recent Trend:**

- Last 5 plans: —
- Trend: —

*Updated after each plan completion*
**Per-Plan Metrics:**

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 01 P01-01 | 37min | 2 tasks | 6 files |
| Phase 01 P01-02 | 19 min | 2 tasks | 3 files |
| Phase 01 P01-03 | 61min | 3 tasks | 12 files |
| Phase 01 P01-04 | 17min | 2 tasks | 5 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Roadmap: selection compiles to maximal-root restic positional targets — exclude-based encoding disqualified (restic excludes do not apply to positional sources); log formally in PROJECT.md during Phase 1
- Roadmap: tree is a new view over the unchanged flat `backupPaths` — zero migration; narrowing selections communicate the future-children allowlist semantic (note under SELECT-03, Phase 3)
- Roadmap: RESTORE-01 added during roadmap creation from research (PITFALLS #3) — restore hardening precedes tree-wide availability
- [Phase 01]: Selection encoding is a flat backupPaths set - bare + "!"-prefixed entries in one list; zero migration (SELECT-02), semantics centralized in internal/api/selection.go
- [Phase 01]: Exclusions never become restic positionals and never derive --exclude flags (locked L1/L14): NormalizeSelection preserves them as a distinct class and readers split via SplitExclusion
- [Phase 01]: configuredBackupPaths tests explicit-vs-auto on the RAW stored list but returns includes-only, so an exclusions-only selection stays an explicit choice while downstream consumers get a clean positional list
- [Phase 01]: storedDataIsGone classifies explicit-none (non-empty raw, zero includes) as NOT gone, checked before any stat - a deliberate deselect is never refused as "not reachable" (threat T-01-03)
- [Phase 01]: Phase 01 plan 02: handleBrowse containment is two-layered - byte-identical paths.Resolve first reject (no status field on that branch) + os.Root kernel-enforced containment behind it; every other read outcome lands in the HTTP 200 envelope carrying a status kind — Phase 01 plan 02: handleBrowse containment is two-layered - byte-identical paths.Resolve first reject (no status field on that branch) + os.Root kernel-enforced containment behind it; every other read outcome lands in the HTTP 200 envelope carrying a status kind
- [Phase 01]: Phase 01 plan 02: browse status carries the error KIND only (restricted/missing/error via errors.Is on *fs.PathError); an os.Root escape rejection deliberately lands in the opaque error bucket so an escape attempt never announces itself on the wire — Phase 01 plan 02: browse status carries the error KIND only (restricted/missing/error via errors.Is on *fs.PathError); an os.Root escape rejection deliberately lands in the opaque error bucket so an escape attempt never announces itself on the wire
- [Phase 01]: Phase 01 plan 02: listing contract = cap 500 + truncated flag with sort-then-truncate on the filtered slice (deterministic lexically-first page); hidden visibility is a literal hidden=1 opt-in with byte-identical default (flag filters, never reorders or fabricates) — Phase 01 plan 02: listing contract = cap 500 + truncated flag with sort-then-truncate on the filtered slice (deterministic lexically-first page); hidden visibility is a literal hidden=1 opt-in with byte-identical default (flag filters, never reorders or fabricates)
- [Phase 01]: Phase 01 plan 02: white-box tables for unexported helpers live in *_internal_test.go (package api) beside the router-harness contract tests in package api_test - Go cannot mix packages per file (browse_contract_internal_test.go precedent) — Phase 01 plan 02: white-box tables for unexported helpers live in *_internal_test.go (package api) beside the router-harness contract tests in package api_test - Go cannot mix packages per file (browse_contract_internal_test.go precedent)
- [Phase 01]: Restore selectors map onto the chosen snapshot's Paths (two-pass longest-prefix); empty intersection aborts in prepare before teardown; per-path skips become scrubbed success-run notes via RestoreDeps.SkippedPaths (nil/empty byte-identical, pinned)
- [Phase 01]: Fixtures predating RESTORE-01 now seed snapshot Paths — fake-engine container snapshots must model the positional truth the mapping reads
- [Phase 01]: restic 0.17 positional behaviors (excludes keep positionals; absolute Paths preserved) pinned as contract tests that skip locally and prove on CI
- [Phase 01]: Empty-selection guard is strictly source-gated on the literal selectionSource:"tree" - refusal (errEmptySelection -> code:"empty-selection" envelope) fires before any store write; legacy and unknown sources keep byte-compat clears — RESEARCH Open Question 2 resolved: no payload sniffing (CONTEXT INTEG-04 Q1); a refused deselect leaves prior state structurally untouched, not best-effort

### Pending Todos

None yet.

### Blockers/Concerns

None.

*(Resolved 2026-09-09: backup-time coverage diff deferred to v2 as SELECT-06 by user decision — the Phase 3 narrowing note under SELECT-03 remains the sole future-children mitigation for v1.)*

## Deferred Items

Items acknowledged and deferred at milestone close, most recent first:

| Category | Item | Status | Deferred At | Milestone |
|----------|------|--------|-------------|-----------|
| v2 (REQUIREMENTS.md) | SELECT-06 backup-time coverage diff | Tracked in REQUIREMENTS.md v2 | 2026-09-09 | — |
| v2 (REQUIREMENTS.md) | TREE-07 search/filter, TREE-08 restore-side tree, SELECT-05 size hints | Tracked in REQUIREMENTS.md v2 | 2026-09-09 | — |

## Session Continuity

Last session: 2026-09-09T18:52:15.816Z
Stopped at: Completed 01-04-PLAN.md
Resume file: None
