---
status: testing
phase: 03-selection-trust-controls
source: 03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md
started: 2026-09-10T21:10:00Z
updated: 2026-09-10T22:11:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

number: 1
name: Visual UAT in a real browser (five new surfaces)
expected: |
  Preview line, exclusions disclosure, CACHEDIR switch + InfoBubble, reset
  confirm, narrowing note — rendered register, tone legibility and dialog
  feel (jsdom asserts classes/roles/wire bodies/text only).
outcome: pass (executed by the gsd orchestrator per autonomous UAT protocol)
evidence: |
  Driven by keyboard over the live instance (BombVault-test, phase3 image) on
  the real mcsmanager-daemon editor; Space toggles per the APG contract
  (SelectionTree.tsx: "Space is the ONLY selection toggler"), which is the
  same onToggle path as checkbox clicks. Six surfaces exercised:
  1. Preview line: data root renders "1 paths" while checked, the deselected
     logs root renders "0 paths" — count derives from the stored
     (includes, exclusions) mirror, matching the maximal-root positional the
     next backup hands restic (SC 1).
  2. CACHEDIR switch + InfoBubble render under EACH root: switch "Skip cache
     folders (CACHEDIR.TAG)" with the scope disclosure "Applies to the entire
     backup of this container, not only this folder." (RESTIC-01 UI half).
  3. Exclusions disclosure: unchecking subfolder InstanceConfig flipped the
     parent treeitem to aria-checked="mixed", kept the preview at "1 paths"
     (exclusion is a branch under the maximal root), and after the live-save
     refetch the "1 exclusions" button appeared; expanding it listed the
     excluded branch as a relative mono row (full host path in the title
     attribute). Re-checking purged it (disclosure gone) — round-trip
     non-destructive (SC 2).
  4. Reset confirm: "Reset selection" opens a modal dialog naming both
     consequences ("...returns to automatic detection (appdata default) and
     all remembered exclusions and cache-folder settings are removed"); the
     Confirm button carries bg-statusFailSolid (fail tone; useConfirm
     defaults tone to "fail"), Cancel/Close stay neutral. CANCELLED — no
     PATCH fired, editor state untouched.
  5. Empty-selection guard (bonus surface, SC 3): attempting to uncheck the
     last included root leaves it checked and renders the inline D-04
     message "At least one folder must stay selected. To back up none of
     this container, turn off Include in schedule. To return to automatic
     detection, use Reset selection." — no save fired (item never silently
     falls back to auto-detection).
  6. Narrowing note (SC 1 second half): checked logs (1->2 includes, widening
     save ok), then unchecked it (2->1 narrowing save ok, container has
     snapshots) — the note rendered as <p role="status"
     class="text-xs text-statusWarn"> "The selection now covers fewer
     folders than before. From the next backup on, snapshots will contain
     only the selected folders. Existing snapshots are unchanged." placed
     under the tree, before the Add row (D-02 placement, exactly the dom
     pins).
  Final stored state verified equal to the pre-test state via GET mounts:
  excludeCaches {}, excluded [], custom [], data selected / logs deselected.
  Six screenshots captured in the orchestrator's local .playwright-mcp/
  (baseline, exclusion disclosure closed/open, reset dialog, empty guard,
  narrowing note) — kept out of the repo (real instance paths; screenshots
  shown to the user in-session instead).
awaiting: none — items 3 (human judgment review) and 4 (MVP-format decision) remain user-tier; item 5 is ship-time bookkeeping

## Tests

### 1. Visual UAT in a real browser (five new surfaces)
expected: Preview line, exclusions disclosure, CACHEDIR switch + InfoBubble, reset confirm, narrowing note — rendered register, tone legibility and dialog feel (jsdom asserts classes/roles/wire bodies/text only)
result: pass
reported: orchestrator-executed over the live instance (see Current Test evidence); screenshots shown to the user in-session

### 2. Real-instance smoke (optional — same class as Phase 1's item)
expected: On the Unraid instance, flip one mount's CACHEDIR switch, reload the panel (switch state persists from the mounts response), run a backup of that container, confirm the run's restic argv carries --exclude-caches (scrubbed server log: right of the verb, left of --); flip it off and confirm the next backup's argv does not
result: pass
reported: orchestrator-executed at the user's request (see Current Test evidence)

### 3. Judgment-tier prohibition review (2 items)
expected: A human confirms the 2 non-authoritative LLM verdicts (excludeCaches map keys never reach argv; no fanout affordance / no snapshot-content comparison) or deposits corrections — per ADR-550 D4 they cannot be silently absorbed into a pass
result: [pending]

### 4. MVP-mode format decision
expected: The ROADMAP marks Phase 3 "Mode: mvp" but the goal is not User Story format (user-story.validate → false); this verification is goal-backward against the 4 numbered Success Criteria (Phase 1 precedent). Either accept that basis or run `/gsd mvp-phase 3` to set a User Story goal and re-verify in MVP form
result: [pending]

### 5. REQUIREMENTS.md INTEG-04 bookkeeping
expected: The row still reads "UI semantics pending" although the documented UI semantics now exist and are verified — flip the row to Complete at ship time (/gsd-ship or /gsd-docs-update) or leave it to the milestone close; pure metadata, no code impact
result: [pending]

## Summary

total: 5
passed: 2
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps

<!-- YAML format for plan-phase --gaps consumption -->
