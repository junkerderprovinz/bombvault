---
phase: 01-selection-engine-restore-safety
reviewed: 2026-09-09T19:21:36Z
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
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-09-09T19:21:36Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Narrative Findings (AI reviewer)

### Summary

Reviewed the full Phase 01 diff (base `121ee887`): the selection encoding keystone (`selection.go` + normalized `SetBackupPaths` + reader classification), the os.Root browse contract, RESTORE-01 longest-prefix mapping with pre-teardown abort, exclusion visibility + empty-selection guard, and the additive `RestoreDeps.SkippedPaths` plumbing — 19 source files at standard depth, cross-referenced against the four plans, CONTEXT/RESEARCH, the milestone ROADMAP success criteria, and the research SUMMARY's UI-rewrite rule.

The implementation is solid where it is riskiest. `mapRestorePaths` is pure, deterministic, and its two-pass longest-ancestor semantics are table-proven; the empty-intersection abort provably fires before any destructive teardown (`TestRestoreEmptyIntersection` inspects the docker call log). The os.Root browse path is genuinely containment-safe (`os.OpenRoot` + `Root.Open` + `ReadDir(-1)`), the traversal-rejection response stays byte-identical to the legacy shape, and the cap/truncated/hidden trio matches BROWSE-01..04. The restic positional contract tests correctly prove engine behavior (not just argv shape) against real restic 0.17.3 on CI and skip locally, per house convention. The empty-selection guard is correctly source-gated on `selectionSource:"tree"` and preserves the `[]` = auto-detection byte-for-byte. I traced and dropped several candidate findings after verification: `chosenSnapshot` first-match is safe because `resticAdapter.VerifySnapshot` rejects ambiguous prefix ids pre-teardown; `Root.Open` accepts trailing slashes and `./` components (probed empirically), so the legacy `?path=x/` form does not regress; success run rows carrying the skip note in `runs.error` follow the existing `finishRestoreRunWarn` precedent; and mapped restore paths cannot escape containment (segment-aligned strict prefixes of already-validated stored paths, plus the orchestrator SEC guard).

Two warnings remain. The headline one (WR-01) is a real semantic gap at the heart of the phase: a mixed include+exclude selection is accepted, stored, and advertised as `excluded` by the mounts API, yet the next backup silently backs the "excluded" branch up. It is the deliberate locked L1/L14 design, but the locked position is justified by a factually wrong restic claim (WR-02), and nothing currently prevents Phase 2 from building UI on top of an API whose advertised semantics the engine does not deliver. Both should be resolved — by enforcement, by exclude-encoding, or by explicit documentation — before Phase 2 lands.

## Warnings

### WR-01: Mixed include+exclude selections are stored and advertised as exclusions, but the engine silently backs the "excluded" branch up

**File:** `internal/api/selection.go:118-124` (with `internal/api/service.go:3817-3874`, `internal/api/service.go:3783-3795`, `internal/api/service.go:3916-3922`)
**Classification:** WARNING
**Issue:** A client can store the selection `["/x", "!/x/branch"]`. It passes validation, survives `PruneMaximal` per-class pruning (`selection.go:53-60`: "an included root and an excluded branch deliberately coexist"), and persists via `SetBackupPaths` (`service.go:3817-3874`). The read-back paths then advertise semantics the engine never enforces: `GET /api/containers/{name}/mounts` renders `excluded:["branch"]` (`service.go:3787`), and the stored pair round-trips as "this branch is deselected." But the backup path compiles the selection through `includesOnly` (`selection.go:124`, consumed at `service.go:3919`), so the next run hands restic the positional `["/x"]` with zero derived excludes (L14) — and restic backs up **all** of `/x`, `branch` included. Snapshot `Paths` will read `[<mount root>]`, so even the Roadmap's own success criterion 2 ("the resulting snapshot's `Paths` contain the selected folders and nothing more") is only half-met: positionally yes, content-wise no. For a backup product this is a silent contract violation with user-visible consequences: a user who believes `transcoding` is excluded accumulates gigabytes of unwanted data and, worse, a restore will resurrect data they believed was deselected. This matches the hard-error class `.planning/research/ARCHITECTURE.md:146` describes ("silently wins as backed up"). The mitigation "the UI decomposes before sending" (research SUMMARY.md:43 rewrite rule) does not hold at the API boundary: the engine accepts mixed pairs from any client, stores them, and *renders them as truth* in `mounts.excluded` between now and whenever Phase 3's trust/controls land.

**Fix:** Close the gap one of three ways, and do it before Phase 2 builds on this API:
1. **Enforce:** in `NormalizeSelection` or `SetBackupPaths`, reject (or auto-decompose) an include that carries a stored excluded descendant, so the mixed pair can never persist — the stored form then always matches what the engine will do.
2. **Encode:** append the bare halves of stored exclusion entries as restic `--exclude` patterns while keeping maximal roots as positionals. This is *not* the no-op L14 claims (see WR-02): the phase's own contract test `TestPositionalExcludesKeepSourceDir` proves excludes filter content *within* positional sources while `Paths` keep the positional verbatim — which would satisfy criterion 2's "nothing more" exactly. Caveat to design around: derived patterns would land in the snapshot's `Excludes` metadata, which is currently user-owned (STACK.md:78).
3. **Document honestly:** if neither is wanted in Phase 1, change `mounts.excluded` to distinguish "recorded deselection" from "will not be backed up" (or omit entries under included roots), and write the deliberate over-capture into the phase summary and PROJECT.md Key Decisions so Phase 2/3 inherit an accurate contract.

### WR-02: Locked position L14 is justified by a factually wrong restic claim (in production and test comments)

**File:** `internal/api/selection.go:118-122`, `internal/api/service_test.go:2298-2304`
**Classification:** WARNING
**Issue:** Both comments state that "restic excludes do not apply to positional sources, so exclude-encoding a deselection would silently no-op / silently back the branch up anyway." That is not restic's behavior: per restic's documentation, excludes do not drop a positional *source directory*, but "content within those directories remains subject to filtering." The phase's own contract test proves it — `internal/restic/restic_positionals_contract_test.go:86-107` asserts `ex.txt` is genuinely filtered out of the snapshot taken with positional `srcDir` and exclude `ex.txt`. This matters beyond pedantry: these are load-bearing "why" comments (the house convention) justifying a *locked design decision* (L14, no `--exclude` derived from selection). A future maintainer reading them will conclude exclude-encoding is technically impossible (it is not — it is a metadata-ownership tradeoff) and may either wrongly "fix" WR-01 by exclude-encoding without considering the snapshot `Excludes` pollution, or leave WR-01 unfixable-looking. The true rationale for L14 is product-level: snapshot `Excludes` metadata is user-owned, and selection-derived patterns would corrupt round-tripping of the stored selection.

**Fix:** Reword both comments to the accurate tradeoff, e.g. for `selection.go:118-122`:

```go
// includesOnly returns the bare (included) half of a stored flat selection —
// the list a backup is actually built from. Exclusion entries are dropped, not
// transformed (locked positions L1/L14). Note this is a PRODUCT choice, not a
// restic limitation: restic excludes DO filter content within positional
// sources (restic_positionals_contract_test.go), so encoding "!" entries as
// --exclude would enforce the deselection content-wise. We do not, because
// snapshot.Excludes metadata is user-owned (STACK.md:78) and selection-derived
// patterns would pollute it; content-level narrowing is delegated to the UI
// decomposition rule (research SUMMARY.md:43) until Phase 3 decides otherwise.
```

(And the matching correction at `service_test.go:2302-2304`.)

## Info

### IN-01: `storedDataIsGone` stats the raw stored list, including `!`-prefixed entries that can never exist

**File:** `internal/api/service.go:3988-3992`
**Classification:** INFO
**Issue:** `storedDataIsGone` correctly early-returns for the explicit-none case (`len(existing.SelectedPaths) > 0 && len(includesOnly(...)) == 0`, `service.go:3972-3993`), but the subsequent stat loop iterates the RAW `existing.SelectedPaths`, so `!`-prefixed entries are stat'd as literal paths. Traced harmless in practice: `stat("!/host/...")` resolves relative to cwd and virtually never exists, so such entries only ever vote "gone" — which cannot flip the outcome, because the vote only matters when every include has already failed its stat (in which case the result is "gone" with or without them). The only theoretical inversion (a file literally named `!`-leading in cwd) is pathological. Still, iterating the wrong list is a latent trap: a future refactor of this guard could be bitten by entries that are classes, not paths.

**Fix:** Stat `includesOnly(existing.SelectedPaths)` in the loop, matching the reader convention every other consumer uses (`service.go:3919`, `service.go:3722-3799`).

### IN-02: Inaccurate `//nolint:gosec // G706` justifications on browse-path log lines

**File:** `internal/api/handlers.go:4276`, `internal/api/handlers.go:4288`
**Classification:** INFO
**Issue:** Both nolint justifications claim "no raw user bytes reach the log formatter." That is not true: `rel` *is* client-influenced data (the request's `path` query parameter after `paths.Resolve` validation) and it does reach the `%q` verb. The code is in fact safe, but for a different reason than stated: `%q` escapes quotes, backslashes, newlines, and control characters, and `paths.Resolve` rejects traversal — that combination is what removes the log-injection/format surface G706 targets. House convention requires nolint justifications to name the *actual* reason (the model comment at `internal/restic/proc_unix.go:26-41`); a justification citing a property the code does not have invites a future edit that preserves the (wrong) stated rationale while dropping the (load-bearing) escaping.

**Fix:** Reword both to, e.g.: `//nolint:gosec // G706: rel is user-influenced but paths.Resolve-validated and rendered via %q, which escapes quotes/newlines/control chars — no log-injection or format surface.`

---

_Reviewed: 2026-09-09T19:21:36Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
