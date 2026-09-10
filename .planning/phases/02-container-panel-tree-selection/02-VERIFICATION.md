---
phase: 02-container-panel-tree-selection
verified: 2026-09-10T15:46:21Z
status: passed
score: 17/17 must-haves verified
behavior_unverified: 0 # every behavior-dependent truth (queue serialization, lazy loading, reconstruction, keyboard map, guards) is exercised by a passing behavioral test — no presence-only truths remain
overrides_applied: 0
unverified_prohibitions: 7 # judgment-tier locks: non-authoritative LLM-judge verdicts per ADR-550 D4 — all read as holding, human review recommended (table below)
re_verification:
  previous_status: none
  previous_score: none
  gaps_closed: []
  gaps_remaining: []
  regressions: []
deferred: # items not met at phase level but explicitly addressed in later phases — informational, no closure plan needed

  - truth: "Structural remove of the item's last include (remove-custom chip) bypasses the D-04 zero-include client floor and can PATCH an exclusions-only list (server accepts as explicit-none per Phase 1) or a fully-empty list (server refuses with code empty-selection; toast-only structural failure leaves the row gone until reload)"
    addressed_in: "Phase 3"
    evidence: "ROADMAP Phase 3 goal: 'defined empty-deselect semantics'; REQUIREMENTS.md INTEG-04: 'Fully deselecting ... has defined, UI-documented semantics ... completes with the documented UI semantics in Phase 3 per ROADMAP'. Pre-phase-2 behavior was strictly worse (empty list silently meant auto-detection); plan 02 scoped D-04 to the toggle path only; no test pins removal."
human_verification:

  - test: "Visual UAT in a real browser: open the container panel FoldersEditor, unfold an appdata mount, toggle subfolders — verify the three node tones (checked/mixed text-carbon-text, unchecked text-carbon-textSub, excluded/unreachable text-carbon-textMuted), muted notice rows (no-access + retry, truncated, empty), the text-statusWarn blocked line under the row, and the h-[clamp(12rem,55vh,32rem)] scroll region"
    expected: "Mixed parents show an indeterminate checkbox with the treeitem announcing mixed; excluded children render muted; notice rows are visually distinct from real nodes; no status color on any control"
    why_human: "jsdom asserts classes/roles/wire bodies only; rendered layout, tone legibility, and interactive feel need human eyes (the plans' own D4/D5 human-judgment coverage items)"

  - test: "Real-browser keyboard walk: Tab into the tree, walk with Arrow keys, expand/collapse (Right/Left/Enter), jump Home/End, toggle with Space"
    expected: "Focus moves with scrollIntoView({block:'nearest'}) keeping the row visible; exactly one tabbable treeitem; Space produces the same PATCH as a click; inner controls (checkbox, retry, remove chip) keep their own key semantics"
    why_human: "jsdom focus events needed act()-wrapping in the harness (documented test-env ordering quirk); real-browser focus ordering and visible scroll behavior are not observable in jsdom"

  - test: "Real-instance smoke (optional, same class as Phase 1 item 1): on the Unraid instance, unfold a real Plex appdata mount, uncheck transcoding, observe the PATCH (flat list, selectionSource tree) in the network tab, close and reopen the section"
    expected: "One browse call per node expand; PATCH body is the canonical flat list; reopen reconstructs checked/mixed/excluded from the mounts response with expansion restored from localStorage and no refetch"
    why_human: "Real host paths (FUSE), real server normalization, real localStorage — the mocked-api harness proves the contract, not the deployment"

  - test: "Review the 7 judgment-tier prohibitions below (LLM-judge verdicts are non-authoritative)"
    expected: "A human confirms the holds verdicts or deposits corrections"
    why_human: "ADR-550 D4: judgment-tier prohibitions cannot be silently absorbed into a pass"

  - test: "Decide the web/dist deviation: plan 03's artifact said 'rebuilt embedded SPA committed'; the executor did NOT commit a rebuilt dist (only the tracked placeholder index.html remains, now referencing pre-phase asset hashes), citing .gitignore lines 14-18 ('real build artifacts under web/dist stay ignored')"
    expected: "Either accept the deviation (add the override below — the shipped binary is unaffected because the Dockerfile web stage builds the SPA from source and stage 2 copies --from=web /src/web/dist, verified in Dockerfile lines 20-26 and 42; vite build exits 0 at HEAD), or require refreshing the tracked web/dist/index.html per prior practice (commits f8a6e327 et al. did update it) and fix the ROADMAP note 'web/dist rebuilt and committed in plan 03' which is now factually wrong"
    why_human: "Accepting a documented deviation from a plan artifact is a developer decision; repo docs (CLAUDE.md 'commit web/dist') and .gitignore contradict each other and only the owner can settle which is canonical"
addressed_in: Phase 3
evidence: "ROADMAP Phase 3 goal: 'defined empty-deselect semantics'; REQUIREMENTS.md INTEG-04: 'Fully deselecting ... has defined, UI-documented semantics ... completes with the documented UI semantics in Phase 3 per ROADMAP'. Pre-phase-2 behavior was strictly worse (empty list silently meant auto-detection); plan 02 scoped D-04 to the toggle path only; no test pins removal."
---

# Phase 2: Container Panel Tree Selection — Verification Report

**Phase Goal:** In the container panel, users can unfold any discovered mount or custom path and pick subfolders with cascade semantics, mixed-state parents, and full keyboard access — and what they see on reopen is exactly what they left.
**Verified:** 2026-09-10T15:46:21Z
**Status:** human_needed
**Re-verification:** No — initial verification
**Verified HEAD:** 97858a27 (includes the five review-fix commits 42af7305..25389f2b — the fixes are part of the delivered phase per instruction)

## Verification Method

All claims were re-derived from the current codebase (files read in full: `selectionTree.ts`, `SelectionTree.tsx`, `FoldersEditor` in `Containers.tsx`, all four test files) and re-executed first-hand on this host (direct `node node_modules/...` invocations per the Windows caveat):

- Four phase suites + i18n parity: **5 files / 181 tests, all pass** (4.9s).
- Full web suite, run once: **91 files / 2179 tests, all pass** (summary claimed 2178 after plan 03; +1 is the CR-01 carve-out table case from the review fix — consistent).
- `tsc --noEmit`: clean. `eslint src`: 0 errors / 2 warnings (both pre-existing, logged in deferred-items.md).
- `vite build`: exit 0 (chunk-size warning pre-existing); tracked `web/dist/index.html` placeholder restored after the check (tree clean).
- `go build ./...` + `go vet ./...`: clean (embed proof; `go test` deferred to CI per the documented restic-on-PATH caveat — zero Go files changed this phase, confirmed by empty `git diff 3e1bc7a8^..HEAD -- internal/ cmd/`).
- `git log` confirms all 11 plan commits + 5 review-fix commits exist on docker-folders.

## Goal Achievement

### Observable Truths

Merged set: 5 ROADMAP success criteria (the contract) + plan must-have truths consolidated under them. Every behavior-dependent truth has a passing behavioral test (Step 7b single-suite runs above), so none are left PRESENT_BEHAVIOR_UNVERIFIED.

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | **SC1a / TREE-01** — unfolding a discovered mount or custom path loads children lazily: zero browse calls on open, exactly one per node on first expand, cache reuse on re-expand, never eager | ✓ VERIFIED | `SelectionTree.dom.test.tsx` "lazy expansion": asserts `browseCalls == []` on open, `["user/appdata/plex"]` after expand, unchanged after collapse+re-expand; integration test pins custom roots browse too (`user/backups`). Code: `fetchListing` through the `browseCache` prop, rejections and ok:false evicted |
| 2 | **SC1b / TREE-02** — unchecking a volatile subfolder PATCHes the flat set with the mount still bare and the subfolder as `!`-prefixed exclusion; the rest of the mount stays selected | ✓ VERIFIED | Integration test asserts exact body `{paths: ["/mnt/user/appdata/plex", "!/mnt/user/appdata/plex/transcoding"], opts:{selectionSource:"tree"}}`, root goes `aria-checked="mixed"`, `library` stays `"true"`; `applyToggle` "checked-via-ancestor" carve-out table |
| 3 | **SC2 / TREE-03** — cascade include; mixed parents (`aria-checked="mixed"`, DOM indeterminate); re-activating a remembered-partial checkbox restores its exact prior partial | ✓ VERIFIED | D-01 cycle test (check → carve out → uncheck parent → re-check restores the EXACT partial, exclusions dormant throughout); mixed via callback-ref indeterminate + treeitem aria-checked; remount mixed test; carve-out flavor fixed by CR-01 and table-pinned |
| 4 | **SC3 / TREE-04** — closing and reopening reconstructs the saved state exactly (fully/partially/excluded distinguishable) from the mounts response alone | ✓ VERIFIED | Remount test: zero new browse calls, mixed/excluded/checked re-derived; `Containers.tree.dom.test.tsx` close/reopen test (expansion from `bv-tree-expanded-{name}`, children from editor-lifetime cache, zero refetch); permutation-insensitivity table (all 24 orderings over a 7-node corpus); exclusions-only list rendered all-unchecked, never mutated, zero PATCHes |
| 5 | **SC4 / TREE-05** — fully keyboard-operable (arrows/Enter/Space/Home/End) with ARIA tree/checkbox roles, aria-expanded on parents, mixed on partials, level/setsize/posinset on lazy nodes | ✓ VERIFIED | `SelectionTree.keyboard.dom.test.tsx` 10 tests with `document.activeElement` asserts: Right expand-then-descend + loading no-op, Left trio, Down/Up never expand, Home/End across roots, Enter expansion, Space-only selection through the identical onToggle pipeline (PATCH body asserted, byte-identical on double-toggle), one tabbable treeitem invariant, geometry uniform incl. the level-3 lazy node, `aria-selected` never present, notice rows outside role and counts. Note: aria-expanded is on every EXPANDABLE treeitem and honestly omitted on leaves (unreachable mounts, outside-root customs) — the APG-correct reading of the plan wording, pinned in both directions; SC4 itself says "on parents" |
| 6 | **SC5 / TREE-06** — unreadable vs empty distinguished per node (muted no-access row + retry vs plain empty); no infinite spinners, no silent collapses | ✓ VERIFIED | Outcome-row tests: verbatim server text / couldNotRead fallback, retry re-enters spinner and genuinely refetches (cache eviction proven by second browse call), truncated notice non-treeitem and outside setsize, three outcomes distinct in one tree, rejected promise settles with `document.querySelectorAll(".animate-spin").length === 0`, listing failure never blocks a toggle's PATCH |
| 7 | **D-03** — every toggle live-saves the full canonical flat list with `selectionSource:"tree"`; server is the only selection truth; reconstruction never reads a local selection cache | ✓ VERIFIED | `attemptSave` sends `toFlatList(mirror)` on every mutation; tests assert bodies and selectionSource; selection state derives from `getContainerMounts` only; localStorage carries expansion exclusively (SelectionTree reads it solely for `expandedOrder`) |
| 8 | **D-04** — zero-include toggle blocked client-side before any request (whole-item counting), inline text-statusWarn line + shake; coded empty-selection envelope handled defensively | ✓ VERIFIED | `onToggle`: `next.includes.size === 0` → blockedPath + shake + return before queue; tests: no PATCH, mirror untouched, warn line classes; custom-include-present case PATCHes (count spans whole item); backstop case toasts verbatim + reverts. Plus CR-01's no-op guard (identical sets → no save, no toast) |
| 9 | **Queue (plan 02)** — one-deep serialized PATCH queue: bursts collapse to one drain, latest list wins, failure revert re-derived from the live mirror so a newer toggle survives | ✓ VERIFIED | Deferred-promise tests: two rapid toggles with first reply failing → final mirror = two-toggle result, final body exact, verbatim toast; burst case → exactly one draining PATCH; custom add/remove ride the same queue (structural, toast-only) |
| 10 | **D-06** — truncated listing renders served children then one non-interactive notice row, outside treeitem counts | ✓ VERIFIED | Test: children render, `aria-setsize == "2"`, one `<p>` notice, no treeitem/checkbox inside, `closest('[role="treeitem"]')` null |
| 11 | **D-05** — expansion persisted per container in `bv-tree-expanded-{name}`, capped 64 evicting oldest, throw-safe both directions, never consulted for selection | ✓ VERIFIED | Tables: cap/recency eviction, corrupted JSON → [], throwing storage → [] both directions; component-path test: 66 real clicks persist exactly the 64 most recent; close/reopen test reads the key |
| 12 | **INTEG-01 / D-02 absorption** — every discovered mount and custom path unfolds as a lazy root; sub-includes under a reachable mount absorbed (mount mixed, no duplicate custom row); no second presentation | ✓ VERIFIED | `partitionCustomPaths` 7 tables (segment-aligned plex2 sibling, exact-root non-match); integration test: unselected mount + sub-include custom row → mount mixed, exactly one presentation, Media checked once; standalone customs keep level-1 lazy rows |
| 13 | Edge lists render safely: empty list (explicit-none legacy displayed, never mutated), single include (subtree checked), nullish browse listings never crash | ✓ VERIFIED | splitFlatSet exclusions-only table; dom exclusions-only test; empty/`dirs:[]` handling pinned; `res.dirs ?? []` and `res.truncated === true` guards in code |
| 14 | Determinism: identical toggle sequences → byte-identical canonical lists; permutation-insensitive classification | ✓ VERIFIED | Toggle-sequence determinism table (deep-equal repeat + pinned end state); 24-permutation table over fixed corpus |
| 15 | i18n: 4 new `folders.*` keys in en + de inline tables and all 40 locale files; parity test green; no em dashes | ✓ VERIFIED | First-hand grep: 40/40 files × 4 keys; en/de blocks present; i18n.parity.test.ts green (exact en key-set equality per locale); em-dash grep on the new keys clean |
| 16 | Wire types caught up to the Phase 1 backend: `BrowseResponse.status`/`truncated`, `ContainerMountsResponse.excluded`, `setBackupPaths` opts with selectionSource, `OkEnvelope.code` | ✓ VERIFIED | api.ts lines 6-14, 481-497, 871-903 read directly; consumed by component/tests |
| 17 | Zero mounts render the existing `folders.empty`; row styling/status text preserved; no new singular/plural copy | ✓ VERIFIED | Containers.tsx 1036-1038 (raw-count empty check); mount row markup preserved inside SelectionTree roots (`folders.appdataDefault`, `folders.notReachable` status lines); only the 4 budgeted keys added |

**Score:** 17/17 truths verified (0 present-behavior-unverified)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | `removeCustomPath` (structural remove chip) bypasses the D-04 floor: removing the item's last include can PATCH a fully-empty list (server refuses, coded empty-selection, toast-only structural failure leaves the row gone until reload) or an exclusions-only list (server accepts as Phase 1's explicit-none) | Phase 3 | ROADMAP Phase 3 goal "defined empty-deselect semantics"; REQUIREMENTS.md INTEG-04 "completes with the documented UI semantics in Phase 3 per ROADMAP". Plan 02 scoped D-04 to the toggle path explicitly; pre-phase-2 behavior was strictly worse (empty list silently = auto-detection) — not a regression |

### Required Artifacts

All present, substantive (no stubs), wired, data flowing.

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/src/lib/selectionTree.ts` | Pure (I,E) arithmetic: EXCLUSION_PREFIX, NodeState, isAtOrUnder/isStrictlyUnder, hostToBrowseRel/browseRelToHost, splitFlatSet, toFlatList, classifyNode, applyToggle (D-01 + CR-01 carve-out), loadExpanded/saveExpanded, partitionCustomPaths | ✓ VERIFIED | 341 lines, zero React imports, narrative header; every export present and table-tested |
| `web/src/lib/selectionTree.test.ts` | Tables for all four states, segment alignment, D-01 cycle, canonical order, permutations, persistence bounds, partition | ✓ VERIFIED | 584 lines, value-level assertions, no skips, no circular patterns; passes |
| `web/src/components/SelectionTree.tsx` | role=tree component: tri-state checkboxes, lazy fetch through editor-lifetime cache, outcome rows, keyboard map, geometry | ✓ VERIFIED | 541 lines; WR-01 presentation wrappers + WR-02 aria-hidden fix in place; scroll region + indent + min-height rules present |
| `web/src/components/SelectionTree.dom.test.tsx` | Lazy expansion, PATCH body, remount, outcome rows, queue, D-04 | ✓ VERIFIED | 630 lines, 19 tests, deferred-reply harness; passes |
| `web/src/components/SelectionTree.keyboard.dom.test.tsx` | Full APG key map, roving tabindex, geometry, notice-row exclusion | ✓ VERIFIED | 447 lines, 10 tests, scrollIntoView stub, activeElement asserts; passes |
| `web/src/pages/Containers.tree.dom.test.tsx` | Integration: wire contract, D-04 no-call, absorption, reopen, 64-cap | ✓ VERIFIED | 307 lines, 6 tests; passes |
| `web/src/lib/api.ts` | Wire types + setBackupPaths opts | ✓ VERIFIED | Read directly (see truth 16) |
| `web/src/pages/Containers.tsx` (FoldersEditor) | Mirror above null return, queue refs, D-04, absorption wiring | ✓ VERIFIED | Lines 610-1100 read; exported for harness; wired into ContainerRow at 2094 |
| `web/src/lib/i18n.ts` + 40 locales | 4 keys × 42 tables | ✓ VERIFIED | First-hand grep 40/40 + en/de; parity green |
| `web/dist` | "Rebuilt embedded SPA committed" | ⚠️ DEVIATION (documented) | NOT committed — only the tracked `web/dist/index.html` placeholder remains (untouched, now stale hash refs). .gitignore lines 14-18 explicitly ignore real artifacts; Dockerfile stage 1 builds the SPA from source and stage 2 copies it (lines 20-26, 42), so every shipped binary embeds a fresh SPA; `vite build` exits 0 at HEAD. ROADMAP note "committed in plan 03" is now factually wrong. → Human decision item 5; override suggested there |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| Containers.tsx load effect | api.ts getContainerMounts | includes = selected+reachable mounts ∪ custom; exclusions = excluded[] | ✓ WIRED (line 796-809) | Pattern `getContainerMounts(` present; `selected && m.reachable` filter is pre-phase-2 semantics carried over verbatim (verified against 3e1bc7a8^) |
| SelectionTree expand handler | api.ts browse | one call per node via browseCache, browse-relative path | ✓ WIRED (line 152) | No hidden opt-in possible (`browse(path)` single-arg) — BROWSE-04 consistent |
| Containers.tsx persist | api.ts setBackupPaths | toFlatList + selectionSource "tree" through the queue | ✓ WIRED (line 851) | Single funnel: attemptSave |
| SelectionTree key handler | selectionTree applyToggle via onToggle | Space routes through the click pipeline | ✓ WIRED (line 368) | One guard, one queue — T-02-10 closed |
| Containers render | partitionCustomPaths | standalone customs rendered, under-mount absorbed | ✓ WIRED (line 1014-1019) | Raw list keeps duplicate guard + empty checks |
| web/src | web/dist | vite build | ⚠️ PARTIAL (deviation) | Build verified green locally; commit skipped per .gitignore; shipped-binary path covered by CI Docker stage — see artifact row and human item 5 |

### Data-Flow Trace (Level 4)

Every rendered value chains to a real data source: node states ← classifyNode over the (I,E) mirror ← getContainerMounts response; children ← browse responses ← listings map; PATCH bodies ← toFlatList(live mirror). No static returns, hardcoded lists, or mocks in production paths. ✓ FLOWING everywhere.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Four phase suites + i18n parity | `node node_modules/vitest/vitest.mjs run <5 files>` | 5 files / 181 tests passed | ✓ PASS |
| Full web suite (run once) | `node node_modules/vitest/vitest.mjs run` | 91 files / 2179 tests passed | ✓ PASS |
| Strict typecheck | `node node_modules/typescript/bin/tsc --noEmit` | clean | ✓ PASS |
| Lint (house rules) | `node node_modules/eslint/bin/eslint.js src` | 0 errors, 2 pre-existing warnings | ✓ PASS |
| Production build | `node node_modules/vite/bin/vite.js build` | exit 0 (placeholder restored after) | ✓ PASS |
| Go chain (embed proof) | `go build ./... && go vet ./...` | clean; zero Go files changed in phase | ✓ PASS |

### Probe Execution

Step 7c: SKIPPED — no probes declared in PLAN/SUMMARY and no `scripts/*/tests/probe-*.sh` exist in the repo.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|---------------------|----------|
| TREE-01 | 02-01, 02-03 | Lazy tree, children on expand, no eager loads | ✓ SATISFIED | Truth 1 |
| TREE-02 | 02-01, 02-02 | Cascade checkbox, child uncheck carves exclusion | ✓ SATISFIED | Truths 2, 8 |
| TREE-03 | 02-01 | Mixed state + remembered-partial restore | ✓ SATISFIED | Truth 3 |
| TREE-04 | 02-01, 02-03 | Reopen reconstruction from backupPaths | ✓ SATISFIED | Truth 4 |
| TREE-05 | 02-03 | Full keyboard + ARIA roles | ✓ SATISFIED | Truth 5 |
| TREE-06 | 02-02 | Unreadable vs empty distinct, no spinners | ✓ SATISFIED | Truth 6 |
| INTEG-01 | 02-01, 02-02, 02-03 | Container panel unfold + subfolder selection without dropping the mount | ✓ SATISFIED | Truths 1, 7, 12 |

Orphaned requirements: none — REQUIREMENTS.md maps exactly these 7 IDs to Phase 2 and every one is claimed by at least one plan. Traceability rows already marked Complete.

### Review Fix Verification (02-REVIEW.md → 02-REVIEW-FIX.md)

All five in-scope findings verified fixed in the current code, not just claimed:

| Finding | Fix in code | Test pin | Status |
|---------|-------------|----------|--------|
| CR-01 applyToggle carve-out no-op | `selectionTree.ts` mixed branch drops-below check + `!`-branch deselect; `onToggle` unchanged-set guard before any save | New table "carve-out mixed node deselects its branch" + suite green | ✓ VERIFIED |
| WR-01 non-treeitem children | role="presentation" wrappers on blocked line + all four notice rows | dom assertions unaffected, suites green | ✓ VERIFIED |
| WR-02 contradictory announcement | `aria-hidden="true"` on the row input (tabIndex -1 kept) | 12 query sites moved to `{hidden:true}` | ✓ VERIFIED |
| WR-03 uncleaned addCustom path | `browseRelToHost(raw, hostSourceRoot)` composition (Containers.tsx 959) | duplicate guard + partition membership now see cleaned paths | ✓ VERIFIED |
| WR-04 staged pick destroyed | `setBrowseValue("")` moved after the duplicate guard (971) | site comment documents the tradeoff | ✓ VERIFIED |

IN-01 (aria-setsize capped count under truncation) remains Info-tier/out of fix scope — acceptable per the review itself.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| web/src/pages/Containers.tsx | 331 | `TODO(#follow-up)` stake-detail copy | ℹ️ Info | Pre-existing (commit b98c992f, 2026-08-19 — verified predates phase 2); out of scope |
| SelectionTree.tsx | 212 | aria-setsize reports capped count under truncation (IN-01) | ℹ️ Info | Announcement inaccuracy only; truncated notice conveys the cap; review marked acceptable |
| ActivityLog.tsx / Sidebar.tsx | 234 / 567 | eslint exhaustive-deps warnings | ℹ️ Info | Pre-existing, recorded in deferred-items.md |

No TBD/FIXME/XXX introduced by this phase. No disabled tests. No circular test patterns. No stub implementations.

### Test Quality Audit

| Test File | Linked Req | Active | Skipped | Circular | Assertion Level | Verdict |
|-----------|-----------|--------|---------|----------|-----------------|---------|
| selectionTree.test.ts | TREE-02/03/04, D-05 | 58+ | 0 | 0 | Value (exact sets/lists) | OK |
| SelectionTree.dom.test.tsx | TREE-01/02/03/04/06, D-03/D-04, queue | 19 | 0 | 0 | Behavioral (exact PATCH bodies, aria states, deferred timing) | OK |
| SelectionTree.keyboard.dom.test.tsx | TREE-05 | 10 | 0 | 0 | Behavioral (activeElement, PATCH bodies, geometry loops) | OK |
| Containers.tree.dom.test.tsx | INTEG-01, D-02/D-04/D-05 | 6 | 0 | 0 | Behavioral (wire contract, no-call guard, reopen, cap) | OK |

Disabled tests on requirements: 0. Circular patterns: 0. Insufficient assertions: 0.

### Decision Coverage

All trackable CONTEXT.md decisions are honored by shipped artifacts (gate result: 6/6 honored — D-01 applyToggle cycle, D-02 single-tree/absorption, D-03 live-save, D-04 client block, D-05 capped expansion persistence, D-06 truncated notice). Non-blocking; no drift.

### Prohibitions (judgment-tier — non-authoritative verdicts, human review recommended)

| # | Prohibition | Verdict | Evidence |
|---|-------------|---------|----------|
| 1 | Tree never an arbitrary FS browser; every browsable path descends from a mount/custom root; no free-text entry in the tree | holds | All node paths derive from mounts response or browse dirs; addCustom is the pre-existing explicit path, outside the tree; browse path is prefix-swapped arithmetic |
| 2 | Expansion never leaks into persisted selection | holds | localStorage read only in SelectionTree expandedOrder; PATCH bodies from toFlatList(mirror) exclusively; tests pin reopen re-derivation |
| 3 | No silent auto-exclusion — every exclusion from an explicit toggle | holds | Exclusions change only via applyToggle in onToggle; no heuristic code path touches E |
| 4 | Unchecking a parent never deletes exclusions strictly below | holds | applyToggle own-include branch leaves E untouched; cycle test pins dormancy |
| 5 | No second presentation of the same selection on one screen | holds | Absorption test: exactly one presentation per path; no modal/fine-selection button added |
| 6 | Zero-include toggle never falls through to the server fallback | holds | Client block before queue; backstop handled; test pins no-call |
| 7 | Listing failures never block saving | holds | Test: node with failed listing still toggles + PATCHes |

### Human Verification Required

See frontmatter `human_verification` — 5 items: (1) visual UAT of the tree states/notice rows/warn line in a real browser, (2) real-browser keyboard walk, (3) optional real-instance smoke, (4) prohibition-judgment review, (5) the web/dist deviation decision (accept-with-override vs. refresh tracked index.html + fix the ROADMAP note).

### Gaps Summary

No structural gaps. All 5 ROADMAP success criteria are verified against the current codebase with first-hand test executions (181 targeted + 2179 full-suite, tsc/eslint/vite/go chains green), the review's five findings are confirmed fixed in code, decision coverage is 6/6, and requirement traceability is complete (7/7, no orphans). Two items are routed rather than scored: the removeCustomPath zero-include edge is explicitly Phase 3 work (INTEG-04) and recorded as deferred; the web/dist "committed" artifact was deliberately not delivered as written (repo .gitignore forbids it; the CI Docker build covers the shipped-binary intent) and needs a developer decision, with an override suggested in human item 5. The status is human_needed solely for the human surface above — chiefly real-browser visual/keyboard UAT that jsdom cannot observe.

---

_Verified: 2026-09-10T15:46:21Z_
_Verifier: Claude (gsd-verifier)_
