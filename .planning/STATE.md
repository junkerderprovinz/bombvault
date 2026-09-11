---
gsd_state_version: 1.0
current_phase: 4
status: completed
stopped_at: Phase 4 complete — all phases complete
last_updated: "2026-09-11T05:37:36.657Z"
last_activity: 2026-09-11
last_activity_desc: Phase 4 complete
state_head: 0529320c91e9458b70fd647f3ef27f0aa536452f
progress:
  total_phases: 4
  completed_phases: 4
  total_plans: 15
  completed_plans: 15
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-10)

**Core value:** Every container, VM, and config on the host can be backed up consistently and restored completely — a dead server is rebuilt from the restic repo alone.
**Current focus:** Phase 4 — File Sets Parity

## Current Position

Phase: 4
Plan: Not started
Status: All phases complete
Last activity: 2026-09-11 — Phase 4 complete

Progress: [████████████████████] 11/11 plans ([████████░░] 75%)

## Performance Metrics

**Velocity:**

- Total plans completed: 18
- Average duration: —
- Total execution time: —

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 5 | - | - |
| 02 | 3 | ~71m | ~24m |
| 02 | 3 | - | - |
| 3 | 3 | - | - |
| 4 | 4 | - | - |

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
| Phase 01 P05 | 12min | 3 tasks | 8 files |
| Phase 02 P01 | 39m | 2 tasks | 47 files |
| Phase 02 P02 | 10m | 2 tasks | 2 files |
| Phase 02 P03 | 22m | 3 tasks | 6 files |
| Phase 03 P01 | 13min | 2 tasks | 9 files |
| Phase 03 P02 | 10min | 2 tasks | 46 files |
| Phase 3 P03 | 40m | 3 tasks | 48 files |
| Phase 04 P01 | 22min | 2 tasks | 10 files |
| Phase 4 P02 | 40min | 2 tasks | 4 files |
| Phase 4 P03 | 38 min | 3 tasks | 7 files |
| Phase 04 P04 | 8 min | 2 tasks | 3 files |

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
- [Phase 01]: Stored exclusion branches are enforced as restic --exclude on the backup argv at the single BackupDeps.Excludes site (WR-01 closed, user decision 2026-09-09); positionals stay the maximal-root includes — Content correctness accepted over metadata purity: derived exclude patterns land in the snapshot user-owned restic Excludes metadata; equality and orphan exclusions deliberately never emitted (pinned by TestExcludedBranches)
- [Phase 2]: Tree node states derive purely from (includes, exclusions) host-path sets (classifyNode/applyToggle), never from loaded children, so collapsed and never-loaded subtrees classify correctly (TREE-04)
- [Phase 2]: The exclusion list below a node IS the D-01 remembered-partial memory: unchecking a parent keeps strictly-below exclusions stored dormant; no second UI-side memory exists
- [Phase 2]: browseCache lives for the editor lifetime (useRef Map in FoldersEditor); rejected and ok:false browse responses are evicted so Try again genuinely refetches
- [Phase 2]: Minimal D-04 empty-selection block pulled forward from plan 02 (Rule 3): the i18n orphan test fails on any en key nothing renders
- [Phase 2]: FoldersEditor saves serialize through a one-deep queue over a ref mirror (inFlight/dirty + pendingDesc); the drain sends the LIVE mirror's latest full list once, so rapid toggles in one React batch each see their predecessor
- [Phase 2]: PATCH failure revert re-derives from the live mirror via set-difference inverse of the failed mutation (never a captured snapshot), so a newer toggle always survives a failing save (Pitfall 5 closed, plan-02)
- [Phase 2]: Custom add/remove ride the same serialized queue as structural saves (toast-only failure preserved) so no two backupPaths PATCHes from the editor are ever concurrent (T-02-08, Rule 2 deviation in 02-02)
- [Phase 2]: Plan-02 outcome-row tests are pins (rows landed with plan-01 deviations), mutation-verified - fallback/truncated mutations fail 5 of the new cases
- [Phase 2]: Keyboard acts on flatNodes (the render walk's flat output with parent links): focus order, geometry, and DOM can never diverge; stale focusPath falls back to firstRoot keeping exactly one tabbable treeitem
- [Phase 2]: aria-expanded on every expandable treeitem, honestly omitted on leaves (APG end-node rule, RESEARCH Pitfall 7) — both directions pinned in tests
- [Phase 2]: Sub-includes the server classifies as custom (exact-match rule, service.go) are absorbed under their reachable mount via partitionCustomPaths — presentation-only filter; addCustom duplicate guard keeps the raw list; one presentation per path (D-02/INTEG-01)
- [Phase 2]: Space routes through the exact onToggle pipeline clicks use (one D-04 guard, one save queue — T-02-10); inner native controls keep their own key semantics via the keydown target guard
- [Phase 03]: [Phase 3]: exclude-caches rides restic.Mode (Limits precedent) — the per-root CACHEDIR.TAG toggles compile at backup time into the item-level boolean union anyRootExcludeCaches, set from the UpsertTarget re-read (fresh per backup, literal A1 union over the stored map independent of selection inclusion); internal/backup stays untouched
- [Phase 03]: [Phase 3]: PATCH excludeCaches is a non-pointer map[string]bool — absent decodes to nil (untouched), explicit {} clears every root; keys validated through the toContainerPath containment discipline with atomic whole-save rejection plus a 64-entry cap
- [Phase 03]: Phase 3 plan 02: preview counts stored maximal includes at-or-under the root only (include above the root does not count) - exactly the per-root toFlatList membership, so the visible number equals the bare positionals the next backup hands restic
- [Phase 03]: Phase 3 plan 02: count and exclusions list are existence-unfiltered by design (A3/Pitfall 5) - stale/unreachable paths stay counted and listed; folders.notReachable/customMissing warn at row level
- [Phase 03]: Phase 3 plan 02: exclusions disclosure is a plain button (backupOrder precedent) tabbable outside the roving set; expansion is per-root component state deliberately NOT persisted (audit view, not navigation comfort); active and dormant roots render the identical section
- [Phase 3]: Reset selection rides the serialized queue as a no-source {backupPaths:[]} descriptor - the strictly tree-gated Phase 1 guard passes it by omission, making the confirmed reset the one sanctioned exit to auto-detection
- [Phase 3]: CACHEDIR flips are a second owed class in the one PATCH queue: the drain composes a single body from the owed classes (T-03-07), caches failure reverts only when the live map still equals the attempted flip
- [Phase 4]: Phase 4 plan 02: file-set PATCH selectedPaths is a three-state pointer field - absent = untouched, [] = coded empty-selection refusal (D-06), list = atomic validated normalized overwrite capped at 64 with per-entry containment via isStrictDescendant against the resolved root
- [Phase 4]: Phase 4 plan 02: a file-set path change clears the selection (to SQL NULL) with clear-wins precedence over same-request entries, compared on resolved roots - Pitfall 2 layer 1; the compile-time re-anchor stays layer 2
- [Phase 4]: Phase 4 plan 02: D-08 restore guard scoped to the in-place route with len(chosen.Paths) > 0 (Rule 1: the literal broader scope contradicted the plan's own acceptance criteria - TestRestoreFileSetToFolder and TestRestoreFileSetInPlaceConfirmed must stay green unmodified; Open Question 1 resolution honored)
- [Phase 4]: Files page tree reuses SelectionTree via additive-optional props only (blockedMessage routes refusal copy per domain); NULL selectedPaths seeds a synthetic root include with zero writes until first toggle; file-set PATCH body is exactly { selectedPaths } with no selectionSource
- [Phase 04]: Files-page exclusions audit list rides the shared SelectionTree disclosure (rootExclusions, Phase 3) that plan 03's mount already carries - Files.tsx documents the single implementation in the editor block comment instead of adding a second surface — Restating it per page would duplicate the audit surface and violate the phase zero-second-implementation lock; the Files dom pins lock this page's rendering instead
- [Phase 04]: FileSetDialog renders files.pathChangeHint unconditionally under the FolderBrowser - it states the consequence of the 04-02 PATCH-time clear rule, not a condition on stored selection — A conditional hint would hide the consequence exactly when the user is about to trigger it; UI-SPEC A3 disclosure

### Pending Todos

None yet.

### Blockers/Concerns

*(Resolved 2026-09-09: backup-time coverage diff deferred to v2 as SELECT-06 by user decision — the Phase 3 narrowing note under SELECT-03 remains the sole future-children mitigation for v1.)*

*(Resolved 2026-09-10: restic contract test TestPositionalExcludeAbsoluteSubdirPattern now proven — full suite green in the golang:1.26-bookworm + restic 0.17.3 container, docker-folders pushed to fork CatFoxVoyager/bombvault, CI lint.yml Test job green at HEAD c02149eb.)*

- ⚠️ [Phase 2] Pre-existing eslint warnings (warn-only, predate this milestone; `eslint src` reports 0 errors): `web/src/components/ActivityLog.tsx:234` exhaustive-deps (useMemo missing `resolveName`), `web/src/components/Sidebar.tsx:567` stale `seqRef.current` in effect cleanup — tracked in phase 02 `deferred-items.md` so they are not rediscovered per plan

None open besides the warn-only lint notes above.

## Deferred Items

Items acknowledged and deferred at milestone close, most recent first:

| Category | Item | Status | Deferred At | Milestone |
|----------|------|--------|-------------|-----------|
| v2 (REQUIREMENTS.md) | SELECT-06 backup-time coverage diff | Tracked in REQUIREMENTS.md v2 | 2026-09-09 | — |
| v2 (REQUIREMENTS.md) | TREE-07 search/filter, TREE-08 restore-side tree, SELECT-05 size hints | Tracked in REQUIREMENTS.md v2 | 2026-09-09 | — |

## Session Continuity

Last session: 2026-09-11T04:36:15.358Z
Stopped at: Phase 4 complete — all phases complete
Resume file: None
