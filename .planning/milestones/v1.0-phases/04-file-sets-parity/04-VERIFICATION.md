---
phase: 04-file-sets-parity
verified: 2026-09-11T05:33:58Z
status: passed
score: 12/12 must-haves verified
behavior_unverified: 0 # every behavior-dependent truth (lazy expand, cascade/mixed-state, exact reopen, queue serialization, both D-06 halves, path-change clear, restore guard, snapshot-Paths engine floor) is exercised by a passing behavioral test — the engine floor was re-proven first-hand against real restic 0.17.3 under Linux at HEAD
overrides_applied: 0
unverified_prohibitions: 12 # judgment-tier locks: non-authoritative LLM-judge verdicts per ADR-550 D4 — all grep/diff/read verdicts below read as holding, human review recommended (table in body)
re_verification:
  previous_status: none
  previous_score: none
  gaps_closed: []
  gaps_remaining: []
  regressions: []
human_verification: # UAT-class, NON-BLOCKING per the Phase 2/3 precedent (both sealed `passed` with these same item classes routed to UAT.md)
  - test: "Visual UAT in a real browser: open the Files page, expand a set card's 'Choose folders' disclosure, verify the one-root tree (mono bare-path label, no dest arrow), the 'N paths' preview inside the root label (including '0 paths' after carving out children), an expanded '{n} exclusions' disclosure with muted mono ltr relative rows, the text-statusWarn refusal line under a row after refusing the last untick, and the clamp(12rem,55vh,32rem) scroll region"
    expected: "Rendering matches the Phase 2 container tree rhythm (shared component); no CACHEDIR switch, no Reset, no custom-path rows anywhere on the card; the dialog shows the path-change caption under the FolderBrowser"
    why_human: "jsdom asserts classes/roles/wire bodies only; tone legibility and layout need human eyes (same class as the 02/03 item-1 UAT)"
  - test: "Real-instance smoke (optional, same class as Phases 1-3): on the Unraid instance, tick a subfolder of a file set, run that set's backup, and compare the snapshot's restic Paths to the ticked roots; then edit the set's folder in the dialog, save, reopen the card, and confirm the tree shows the honest NULL seed (root CHECKED at '1 paths') rather than the old anchor's selection"
    expected: "Snapshot Paths equal the compiled positionals; the path edit clears the selection server-side and the card remounts to the post-clear seed; a NULL (never-edited) set backs up [SourceDir] exactly as before the phase"
    why_human: "Real FUSE paths, real SQLite, real restic child — the harnesses prove the contracts, not the deployment"
  - test: "Review the 12 judgment-tier prohibitions below (LLM/grep verdicts are non-authoritative per ADR-550 D4)"
    expected: "A human confirms the holds verdicts or deposits corrections"
    why_human: "ADR-550 D4: judgment-tier prohibitions cannot be silently absorbed into a pass"
  - test: "Confirm the MVP-mode format decision: ROADMAP marks Phase 4 'Mode: mvp' but the goal is not in User Story format (user-story.validate -> false: 'Choosing what a File Set covers uses...' has no 'As a ... I want to ... so that' shape), so this verification is goal-backward against the 2 numbered Success Criteria (Phase 1/3 precedent)"
    expected: "Either accept that basis or run /gsd mvp-phase 4 to set a User Story goal and re-verify in MVP form"
    why_human: "Format decision belonging to the user"
---

# Phase 4: File Sets Parity — Verification Report

**Phase Goal:** Choosing what a File Set covers uses the same collapsible tree with the same cascade/mixed-state/persistence semantics — file sets gain sub-folder granularity with zero second implementation of selection.
**Verified:** 2026-09-11T05:33:58Z
**Status:** passed
**Re-verification:** No — initial verification
**HEAD verified:** 0529320c (branch `docker-folders`; includes both post-review fixes d8fd491e (CR-01) and 0529320c (WR-01))
**Mode note:** ROADMAP marks Phase 4 `mode: mvp`, but the goal is not in User Story format — verified goal-backward against the 2 numbered Success Criteria (Phase 1/3 precedent; human item 4).

## Goal Achievement

### User Flow Coverage (MVP-mode narrowing, goal not in User Story form — success-criterion basis)

User story outcome clause, restated from SC1/SC2: a user picks a file set's coverage on the Files page through the same tree, and the next snapshot contains exactly the ticked roots.

| Step | Expected | Evidence | Status |
|------|----------|----------|--------|
| Open a set card | "Choose folders" disclosure below the actions row; no disclosure for a path-less set | `web/src/pages/Files.tsx:1312-1332` (disclosure, aria-expanded/controls), `:1306` + `:1589` (no-Path gating, D-02) | ✓ |
| Expand | Exactly ONE level-1 treeitem — the set's resolved path, aria-setsize=1, mono, no dest arrow | `Files.tsx:1343-1344` (one synthetic root, `customPaths={[]}`); dom pin "discloses exactly ONE level-1 treeitem" (Files.tree.dom.test.tsx:200) | ✓ |
| Expand a folder | Children lazy-load via GET /api/browse through the hostMountRoot prefix, cached for the editor lifetime | `Files.tsx:1171` (editor-lifetime browseCache), SelectionTree browse path; dom pin Files.tree.dom.test.tsx:273 | ✓ |
| Tick/untick | Cascade via the shared `applyToggle`; partial parents render mixed; last-untick refused before the wire | `Files.tsx:1276-1300` (onToggle → applyToggle only), `:1286-1290` (client D-06); dom pins :400 (burst), :430 (refusal), :235 (mixed) | ✓ |
| Close & reopen | Exact reconstruction from the served `selectedPaths`, zero refetch | dom pin "reopens exactly…" Files.tree.dom.test.tsx:537 | ✓ |
| Run the set's backup | Snapshot `Paths` == compiled positionals == ticked roots | `service.go:8779-8787` (compile), `TestMultiPositionalPathsMirrorSelection` **PASS first-hand vs real restic 0.17.3 under Linux at HEAD** (see Execution Evidence) | ✓ |

### Observable Truths

Fusion: the 2 ROADMAP Success Criteria (the contract) + plan-level must-haves from 04-01..04-04 that add verifiable detail.

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | **SC1** — On the Files page, coverage is picked through the SAME tree component: lazy expand, cascading checkboxes, mixed-state parents, exact reconstruction on reopen | ✓ VERIFIED | `FileSetFoldersEditor` mounts the Phase 2 `SelectionTree` verbatim (`Files.tsx:1343-1362`: one synthetic root, `customPaths:[]`, `containerName=fileset-{id}`, `blockedMessage=files.emptySelectionBlocked`); `web/src/lib/selectionTree.ts` diff across the phase is EMPTY (zero second implementation); toggle goes only through shared `applyToggle` (`Files.tsx:1278`). Behavioral: `Files.tree.dom.test.tsx` 19/19 PASS (lazy expand + cache :273, stored-partial mixed state :235, exact reopen :537, Space-routing :594) + `selectionTree.test.ts` 74/74 PASS (cascade/classify/normalize owners) |
| 2 | **SC2 (persistence half)** — The selection round-trips through the same flat persistence and normalization, no separate format | ✓ VERIFIED | Same flat encoding as `backupPaths` (bare + `!`-prefixed), own nullable column v101; write path normalizes through the SHARED `NormalizeSelection` (`service.go:8990`), read path serves it back verbatim with omitempty NULL switch (`service.go:8879-8883`, `FileSetView.SelectedPaths` :8837); store scan nullable in all three SELECTs (`filesets.go:143/164/171`, `scanFileSet:272-284`); round-trip `TestFileSetSelectedPathsRoundTrip` PASS, boundary `TestPatchFileSetSelectedPaths` 7/7 PASS, view-shape test PASS; TS mirror field-for-field (`api.ts:2291/2370`) |
| 3 | **SC2 (engine half)** — The set's next backup produces a snapshot whose `Paths` match the ticked roots | ✓ VERIFIED (first-hand engine proof) | Single compile site `service.go:8779`: `fileSetPositionals(set.SelectedPaths, src)` → `FileSetBackupDeps.SourcePaths`; argv pins `TestBackupFileSetSelectedPaths` 4/4 PASS (maximal prune, canonical order, derived excludes, stale-root) + `TestBackupFileSetDirSourcePaths` PASS (verbatim passthrough, nil-legacy). Engine floor: `TestMultiPositionalPathsMirrorSelection` asserts `snaps[0].Paths` deep-equal to the positional list (verbatim, absolute, order-preserved) with the excluded branch filtered — **run by this verifier at HEAD 0529320c in golang:1.26-bookworm against SHA256-verified restic 0.17.3 → PASS (4.97s)** |
| 4 | Legacy NULL compat — a never-tree-edited set compiles/renders/backups byte-identically to pre-phase behavior | ✓ VERIFIED | `fileSetPositionals(nil, src) → []string{src}` (`selection.go:240-243`); pre-phase pin `TestBackupFileSet` **byte-identical** (awk-extracted function diff `c6b4c5c6..HEAD` = empty) and PASS — positionals exactly `[srcDir]`, no derived excludes; `CreateFileSet` INSERT omits the column (stores NULL), `UpdateFileSet` column list never touches it; UI: NULL seeds the synthetic root include, renders CHECKED at "1 paths", NO write until first toggle (`Files.tsx:1155`, dom pin :220 "NULL seed … no write happens") |
| 5 | Compile hardening — re-anchor against the freshly resolved root every backup, filtered-empty falls back to the root (never an unanchored positional), deterministic canonical order, segment-aligned sibling-prefix safety | ✓ VERIFIED | `selection.go:240-254` (shared `NormalizeSelection` prune, `p == src || isStrictDescendant(p, src)` filter, empty→`[src]` fallback); white-box `TestFileSetPositionals` 11/11 PASS incl. "/data/doc vs /data/docs" trap, all-outside fallback, determinism; stale-root service subtest PASS (entries under an edited-away root never emitted) |
| 6 | PATCH boundary — `selectedPaths` pointer (absent = untouched), per-entry containment before ANY write, 64-entry cap, atomic whole-save rejection, scrubbed errors | ✓ VERIFIED | `handlers.go:4515` (`*[]string`, declared for DisallowUnknownFields); `service.go:8959-8994` (trim/split/`path.Clean`/equality-or-`isStrictDescendant` vs resolved root, cap :8960, normalize, refusal); `TestPatchFileSetSelectedPaths` subtests PASS: atomic rejection keeps prior selection byte-identical, 64 accepted / 65 refused, absent key untouched, redundant descendant collapses |
| 7 | D-06 — an empty (zero-include) selection is refused on BOTH halves, prior state structurally untouched | ✓ VERIFIED | Server: `errFileSetEmptySelection` (`service.go:8814`) → `codedFailEnvelope(err, "empty-selection")` (`handlers.go:4604-4609`); `TestFileSetEmptySelection` 3/3 PASS (exclusions-only, empty list, refusal never clears). Client: block before any request (`Files.tsx:1286-1290`), dom pins :430 (zero PATCHes) and :454 (server-coded refusal → warn line + fail toast, live-mirror revert keeps a stacked toggle) |
| 8 | Path-change clear — a PATCH that changes the set's resolved Path clears the selection in the SAME save, clear-wins over same-request entries, atomic at the storage layer | ✓ VERIFIED | Resolved-root comparison `handlers.go:4572-4577`; single-statement `UpdateFileSetClearingSelection` (`filesets.go:118-138`, sets `selected_paths = NULL` in the row UPDATE — WR-01 fix) wired at `handlers.go:4584-4585`; switch order enforces clear-wins (`:4597-4602`); `TestUpdateFileSetClearingSelection` PASS (store pin) + Go subtests "changing_the_set_path_clears" / "a_path_change_wins_over_entries" PASS + dom remount pins :308/:351 (CR-01 fix: `fileSetEditorKey` at `Files.tsx:1125-1127/1590` reseeds to the honest post-clear NULL seed without wedging) |
| 9 | D-08 — an in-place restore whose snapshot shares nothing with the set's compiled coverage aborts synchronously BEFORE destructive work; old mono-path snapshots still restore; one mapping implementation | ✓ VERIFIED | `service.go:9202-9214`: guard scoped to `plan.inPlace != ""` + `len(chosen.Paths) > 0`, compiles via the SAME `fileSetPositionals`, maps via the one `mapRestorePaths` (exactly 2 call sites in service.go), aborts with "nothing to restore for this set from this snapshot" before any engine call; `TestRestoreFileSetSelectionGuard` 4/4 PASS (disjoint aborts, mono-path restores, to-folder untouched, NULL-selection legacy mapping) |
| 10 | Zero-second-implementation lock — selection semantics live only in SelectionTree/applyToggle/classifyNode + selection.go | ✓ VERIFIED | `git diff c6b4c5c6..HEAD -- web/src/lib/selectionTree.ts` EMPTY; `Files.tsx` imports the shared helpers (`:31`) and contains no hand-rolled prefix logic (grep startsWith/endsWith = 0 on paths) and no `selectionSource` (=0); `fileSetPositionals` reuses NormalizeSelection/includesOnly/isStrictDescendant — no `strings.HasPrefix` path comparison outside `isStrictDescendant` itself (`selection.go:50`, `a+"/"` segment-aligned); one `mapRestorePaths`; exclusions list is the shared SelectionTree disclosure, not restated (04-04 documented decision) |
| 11 | 4 new i18n keys x 42 locales, gates green, no em dashes | ✓ VERIFIED | `files.foldersToggle/foldersHint/emptySelectionBlocked/pathChangeHint` in en+de inline tables (`i18n.ts` grep = 2) and all 40 lazy chunks (grep -rl = 40); copy contract pinned by `i18n.filesSelection.test.ts` (created per 04-03 deviation 1 so the orphan gate stays honest); full vitest suite green at HEAD (93 files / 2241 tests, incl. the three i18n gates) |
| 12 | Audit surfaces — per-root reviewable exclusions list (display-only, collapsed on reopen, dormant parity, existence-unfiltered) + dialog path-change disclosure (A3) | ✓ VERIFIED | List rides the shared `SelectionTree` `rootExclusions` disclosure (SelectionTree.tsx:442/544-596) that the Files mount already renders; `Files.tsx` documents the single implementation (grep rootExclusions = 1); dialog caption `files.pathChangeHint` under the FolderBrowser, unconditional (`Files.tsx:1005`); dom pins :632 (count + mono rows + ZERO interactive controls), :668 (collapsed on every reopen, no persistence), :695 (dormant parity + existence-unfiltered), :732/:751 (dialog disclosure) |

**Score:** 12/12 truths verified (0 present-behavior-unverified; 0 overrides applied)

### Execution Evidence (first-hand, this verifier, 2026-09-11)

| Proof | Command | Result |
|---|---|---|
| Store round-trip + WR-01 atomic clear | `go test ./internal/store/ -run "TestFileSetSelectedPathsRoundTrip\|TestUpdateFileSetClearingSelection" -v` | **2/2 PASS** (0.54s) |
| Orchestrator SourcePaths passthrough | `go test ./internal/backup/ -run TestBackupFileSetDir -v` | **PASS** (nil-legacy + verbatim subtests) |
| Compile + boundary + refusal + view + restore guard + legacy pin | `go test ./internal/api/ -run "TestFileSetPositionals\|TestBackupFileSet\|TestPatchFileSetSelectedPaths\|TestFileSetEmptySelection\|TestFileSetSelectedPathsViewShape\|TestRestoreFileSetSelectionGuard" -v` | **ALL PASS** (26 subtests across 9 functions, 0.96s) — incl. the untouched legacy `TestBackupFileSet` |
| Engine floor: snapshot Paths == positional list | container `golang:1.26-bookworm`, restic 0.17.3 bz2 **SHA256-verified** (`5097faed…` from the repo Dockerfile), `go test ./internal/restic/ -run TestMultiPositionalPathsMirrorSelection -v -count=1` at HEAD 0529320c | **PASS (4.97s)** — Paths deep-equal positionals verbatim/ordered, both selectors live, excluded branch's file filtered |
| Files editor + shared selection semantics | `npx vitest run src/pages/Files.tree.dom.test.tsx src/lib/selectionTree.test.ts` | **93/93 PASS** (3.14s) |
| Legacy pin immutability | `awk`-extract `TestBackupFileSet` at `c6b4c5c6` vs HEAD, diff | **byte-identical** |
| Migration append-only | `git diff c6b4c5c6..HEAD -- internal/store/migrate.go` | **26 insertions, 0 deletions** (v101 only; NUMBERING HAZARD comments intact) |
| Regression gates at HEAD (fixer-reported, consistent with the above) | gofmt / go vet / go test ./... (22 pkgs, Windows-reduced) / eslint / full vitest | clean / exit 0 / ok / 0 errors / 93 files 2241 tests |

Note: the Windows-local `go test` runs skip POSIX-only and restic-dependent tests by repo-documented design; the restic contract row above was closed first-hand in the Linux container, so no behavior-dependent truth rests on CI alone.

### Required Artifacts

All present, substantive, wired. No stubs.

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `internal/store/migrate.go` | v101 `file_set_selected_paths` nullable TEXT, append-only | ✓ VERIFIED | Entry at :1339-1341 directly after v100; no default, no alreadySatisfied guard; 0 removed lines |
| `internal/store/filesets.go` | `SelectedPaths` field, nullable scan (3 SELECTs), owned setter, atomic clear writer | ✓ VERIFIED | Field :46 with load-bearing D-03/D-05 comment; scan :272-284 (*string, NULL⇒nil); `SetFileSetSelectedPaths` :225-243 (nil⇒SQL NULL, never `'[]'`); `UpdateFileSetClearingSelection` :118-138; `UpdateFileSet`/`CreateFileSet` untouched (selection never clobbered) |
| `internal/backup/files_orchestrator.go` | `SourcePaths []string` additive deps field + honoring wrap | ✓ VERIFIED | Field :36, wrap :63-64 (`len(d.SourcePaths) > 0` — empty list can never zero positionals); `FilesRestic` interface unchanged |
| `internal/api/selection.go` | `fileSetPositionals` — the single files-domain compile helper | ✓ VERIFIED | :240-254, pure, shared-prune, re-anchoring, fallback; consumed only by BackupFileSet + the D-08 guard |
| `internal/api/service.go` | Compile wiring, PATCH validation + D-06 sentinel, view field, D-08 guard | ✓ VERIFIED | :8779-8787, :8931-8994, :8837/:8883, :9202-9214 |
| `internal/api/handlers.go` | PATCH pointer field, oldPath capture, clear/selection switch, coded envelope | ✓ VERIFIED | :4515, :4529, :4572-4614 |
| `web/src/lib/api.ts` | TS mirror `selectedPaths` on FileSetView + patchFileSet | ✓ VERIFIED | :2291, :2370 |
| `web/src/components/SelectionTree.tsx` | Optional CACHEDIR props + optional blockedMessage, additive only | ✓ VERIFIED | :104/:108/:121; container call sites source-compatible (Containers dom suite unmodified and green in the full run) |
| `web/src/pages/Files.tsx` | Disclosure, one-root editor, NULL seed, queue, refusals, remount key, dialog hint | ✓ VERIFIED | :1104-1367 (editor), :1110-1127 (CR-01 key), :1590 (keyed mount), :1005 (hint) |
| `web/src/pages/Files.tree.dom.test.tsx` | The page's first dom harness | ✓ VERIFIED | 19 tests, all PASS first-hand |
| `web/dist/index.html` | Rolled so the embedded SPA matches source | ✓ VERIFIED | Roll committed IN `d8fd491e` itself (the last web/src change); HEAD is Go-only after it — embed is fresh |
| Tests (Go) | filesets/filesets_test, files_orchestrator(_test), files_internal_test, service_test, handlers_test, restic contract | ✓ VERIFIED | All enumerated and exercised above |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| FileSetView.selectedPaths (absent = NULL) | splitFlatSet seed | `Files.tsx:1155` `set.selectedPaths ?? [root]` | ✓ WIRED | NULL ⇒ synthetic root include (UI-SPEC item 9) |
| Tree toggle | shared applyToggle → mirror → queue | `Files.tsx:1276-1300` → `scheduleSave/attemptSave` | ✓ WIRED | Only mutation path; `maxConcurrentPatches === 1` pinned |
| Queue drain | plan-02 boundary | `patchFileSet(set.id, { selectedPaths: toFlatList(live.inc, live.exc) })` (`Files.tsx:1218`) | ✓ WIRED | No selectionSource in the body (grep = 0) |
| PATCH body selectedPaths | `Service.SetFileSetSelectedPaths` → store setter → column | `handlers.go:4602-4613` → `service.go:8959` → `filesets.go:225` | ✓ WIRED | Validated → normalized → stored; coded refusal branch present |
| BackupFileSet | fileSetPositionals → SourcePaths → restic positionals after `--` | `service.go:8779-8782` → deps → `files_orchestrator.go:63-64` | ✓ WIRED | Manual/batch/Everything funnel through the one site |
| excludedBranches(set.SelectedPaths) | deps Excludes → `--exclude` tail | `service.go:8786-8787` (mirrors the container compile line) | ✓ WIRED | `excludedBranches(nil)` empty ⇒ legacy-safe |
| prepareRestoreFileSet | fileSetPositionals → mapRestorePaths → pre-teardown abort | `service.go:9202-9214` | ✓ WIRED | Same helper as backup — guard and backup cannot disagree |
| browse(hostToBrowseRel(path, hostMountRoot)) | GET /api/browse | SelectionTree with `hostSourceRoot={hostMountRoot}` (`Files.tsx:1348`) | ✓ WIRED | Zero backend change; lazy per-node (Phase 1 contract) |

### Data-Flow Trace (Level 4)

No rendered value terminates in a static return or hardcoded literal on the touched paths: the tree seed comes from the served `FileSetView.selectedPaths` (store read, omitempty-verified by `TestFileSetSelectedPathsViewShape`); preview counts and the audit list derive from the same (includes, exclusions) sets the compile consumes (`rootIncludeCount`/`rootExclusions` over `toFlatList` membership — pinned in the harness); backup positionals come from a fresh `GetFileSet` read each run; browse listings come from live `ReadDir` behind the Phase 1 endpoint. ✓ FLOWING throughout.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| NULL selection compiles to legacy `[SourceDir]` | `go test ./internal/api/ -run "TestBackupFileSet$"` | PASS (pin byte-identical to pre-phase) | ✓ PASS |
| Multi-root compile → maximal positionals + derived excludes | `-run TestBackupFileSetSelectedPaths` | 4/4 subtests PASS | ✓ PASS |
| Snapshot Paths == ticked roots (engine floor) | container run vs restic 0.17.3 | PASS | ✓ PASS |
| Empty selection refused, prior state intact | `-run TestFileSetEmptySelection` | 3/3 PASS | ✓ PASS |
| Disjoint snapshot aborts pre-teardown | `-run TestRestoreFileSetSelectionGuard` | 4/4 PASS | ✓ PASS |
| Path-change clear + clear-wins + atomic | `-run TestPatchFileSetSelectedPaths` + `TestUpdateFileSetClearingSelection` | 7/7 + PASS | ✓ PASS |
| Lazy expand / mixed state / exact reopen / queue / refusals / CR-01 remount / audit list / dialog hint | `npx vitest run src/pages/Files.tree.dom.test.tsx` | 19/19 PASS | ✓ PASS |
| Cascade + classification owners | `npx vitest run src/lib/selectionTree.test.ts` | 74/74 PASS | ✓ PASS |

### Probe Execution

SKIPPED — no `scripts/*/tests/probe-*.sh` exist and no plan declares probes; the phase's runnable checks are the Go/vitest suites above.

### Requirements Coverage (REQUIREMENTS.md, Phase 4)

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| INTEG-02 | 04-01..04-04 | File Sets page — the same tree component is used when choosing what a file set covers | ✓ SATISFIED | Criterion 1 by plans 03-04 (truth 1), criterion 2 by plans 01-02 (truths 2-3, incl. first-hand engine proof); REQUIREMENTS.md already marked `[x]` / traceability "Complete" (04-04 Task) |

Orphaned requirements: none — INTEG-02 is the only requirement mapped to Phase 4, and it is claimed by all four plans.

### Decision Coverage (non-blocking gate)

8/8 CONTEXT decisions honored in the shipped artifacts: D-01 multi-root positionals (`SourcePaths`, disjoint-branches test), D-02 one root = resolved Path / no arbitrary browsing (one-treeitem pin, noPathHint gating), D-03 v101 nullable + flat encoding + NULL legacy (store leg + pins), D-04 pointer PATCH + containment + 64 cap + atomic (boundary tests), D-05 compile + single-site derived excludes + NULL legacy argv (compile tests + legacy pin), D-06 client+server refusal oriented to Remove set (both halves tested), D-07 preview count + reviewable exclusions + CACHEDIR out of scope (mount pins + no-switch pin), D-08 one mapping + pre-teardown abort (guard tests). No translated decision vanished at execution.

### Prohibitions (judgment-tier — non-authoritative verdicts, human review recommended)

| # | Prohibition | Verdict | Evidence |
|---|---|---|---|
| 1 | No shipped migration edited/renumbered; v101 unconditional, no alreadySatisfied guard | holds | migrate.go diff 0 removed lines; entry :1339-1341 |
| 2 | Retention identity never regrouped (no `--group-by paths` / global `--group-by tags`) | holds | `git diff` phase grep for group-by = 0 |
| 3 | `internal/store` never interprets selected_paths (opaque blob) | holds | filesets.go holds only json.Marshal/Unmarshal + SQL; semantics live in api tier / web mirror |
| 4 | No second normalization/compile implementation | holds | `fileSetPositionals` = shared helpers only; selectionTree.ts diff empty; no prefix logic in Files.tsx |
| 5 | User-influenced positionals only after `--` via typed builders; no shell | holds | SourcePaths rides the existing `FilesRestic.Backup` builder; contract test asserts the positional slot and Paths verbatim |
| 6 | No selectionSource carrier on the files PATCH (presence-gated guard) | holds | Files PATCH body `handlers.go:4499-4516` has no such field; the 2 handler-grep hits are the pre-existing container handler (:1120-1123); Files.tsx grep = 0 |
| 7 | No silent empty selection ever persisted server-side | holds | `TestFileSetEmptySelection` (refusal + prior state) + `[]`/exclusions-only subtests PASS |
| 8 | No second files-domain restore-mapping semantic | holds | exactly 2 `mapRestorePaths(` sites in service.go (container + files guard) |
| 9 | `toContainerPath` never applies to file-set entries | holds | grep of the files validation path = 0; entries stay mount-root absolute |
| 10 | `UpdateFileSet` never writes selected_paths; owned setter is the only write path (plus the named clear writer) | holds | UPDATE column lists at `filesets.go:88-89` / :126-128; preserve rows in the round-trip test PASS |
| 11 | Files tree never renders CACHEDIR switch / custom-path rows / Reset; exclusions list never a mutation surface; selection never in localStorage | holds | dom pins :632/:668 (zero controls, no persistence leak); `mounts=[one root]`, `customPaths=[]`, grep narrowedNote/resetSelection/excludeCaches= in Files.tsx = 0 |
| 12 | web/dist artifacts: only the tracked index.html placeholder is committed | holds | Assets gitignored; the tracked file's roll committed in d8fd491e itself, working tree clean at HEAD |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| internal/api/service.go | — | Compiled file-set positionals not existence-filtered (containers use `onlyExistingPaths`) — deleted subfolders yield per-run restic skip noise, mapped to success-with-skips | ℹ️ Info | Review IN-02, deliberately out of scope; root-level case guarded by the `os.Stat` pre-check; outcome parity with containers already holds |
| web/src/pages/Files.tsx | 1139-1249 | Serialized save queue is a second hand-copy of the containers' Pattern-4 machinery | ℹ️ Info | Review IN-03; divergence documented; hook extraction suggested for when both call sites are stable |
| web/src/pages/Files.tsx | 1334 | Tree region aria-label uses the container domain's `folders.title` | ℹ️ Info | Review IN-04; single off-domain label, screen-reader polish |
| web/src/lib/locales/*.ts | — | 26 chunks carry two glued object properties on one physical line | ℹ️ Info | Review IN-01; valid TS, invisible to gates; formatting-only |

No TBD/FIXME/XXX/debt markers introduced by the phase (grep clean on all phase files). No blocker anti-patterns. The review's 1 Critical (CR-01) and 1 Warning (WR-01) are both verified FIXED in code at HEAD: CR-01 via `fileSetEditorKey` remount (`Files.tsx:1125-1127`, wired `:1590`, pinned by dom tests :308/:351) and WR-01 via the atomic `UpdateFileSetClearingSelection` (`filesets.go:118-138`, wired `handlers.go:4584-4585`, store pin PASS).

### Human Verification Required

See frontmatter `human_verification` — 4 UAT-class items, NON-BLOCKING per the Phase 2/3 precedent (both UI phases sealed `passed` with the same item classes routed to UAT):

1. **Visual UAT in a real browser** — the Files-page tree/audit/disclosure rendering (tones, register, scroll region).
2. **Real-instance smoke** — tick → backup → snapshot Paths vs ticked roots; dialog path edit → honest NULL reseed.
3. **Judgment-tier prohibition review** — the 12 holds verdicts above.
4. **MVP-mode format decision** — goal is not in User Story form; accept the success-criterion basis or run `/gsd mvp-phase 4`.

The fixer's conditional routing note (CR-01 touches optimistic-UI remount semantics; route to human only if a click-through-only gap exists) does not fire: the exact scenario — dialog path edit → refetch → remount to the honest post-clear seed without wedging, and the same-anchor refetch keeping the editor mounted — is exercised by two dedicated dom tests (`Files.tree.dom.test.tsx:308`, `:351`) that pass against the real component.

### Gaps Summary

None. Both Success Criteria are verified TRUE in code with named first-hand evidence: the Files page picks file-set coverage through the unmodified Phase 2 SelectionTree with lazy/cascade/mixed/exact-reopen semantics behaviorally pinned (19 dom + 74 shared tests), and the selection round-trips through the same flat persistence and shared normalization into a backup whose snapshot Paths were proven equal to the compiled ticked roots against real restic 0.17.3 under Linux at HEAD. Legacy NULL behavior is byte-identical (untouched pre-phase pin). Both review findings are fixed and pinned. The four Info items remain documented, deliberately out of scope.

Residual bookkeeping for the orchestrator (not code gaps): ROADMAP.md still shows Phase 4 unchecked / "In Progress" in the Progress table, and STATE.md close-out — updated during phase sealing.

---

_Verified: 2026-09-11T05:33:58Z_
_Verifier: Claude (gsd-verifier)_
