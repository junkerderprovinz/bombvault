---
status: testing
phase: 03-selection-trust-controls
source: 03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md
started: 2026-09-10T21:10:00Z
updated: 2026-09-10T21:10:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

number: 1
name: Visual UAT in a real browser (five new surfaces)
expected: |
  Open the container panel FoldersEditor with a multi-mount container. Verify:
  the muted "{n} paths" line inside every root label (including "0 paths" on a
  deselected mount); expanding an "{n} exclusions" disclosure (chevron rotation,
  mono ltr break-all relative rows with hover titles); the "Skip cache folders
  (CACHEDIR.TAG)" switch row with its InfoBubble under every root; the Reset
  selection button with its fail-tone confirm dialog; and (after unchecking a
  mount on a backed-up container) the role=status text-statusWarn narrowing
  note under the tree. Sub-rows sit on the indent-16 py-1 rhythm consistent
  with the Phase 2 rows; the switch label reads at the 12px register; the
  confirm dialog carries the destructive treatment; the note renders between
  the tree and the Add row.
awaiting: user response

## Tests

### 1. Visual UAT in a real browser (five new surfaces)
expected: Preview line, exclusions disclosure, CACHEDIR switch + InfoBubble, reset confirm, narrowing note — rendered register, tone legibility and dialog feel (jsdom asserts classes/roles/wire bodies/text only)
result: [pending]

### 2. Real-instance smoke (optional — same class as Phase 1's item)
expected: On the Unraid instance, flip one mount's CACHEDIR switch, reload the panel (switch state persists from the mounts response), run a backup of that container, confirm the run's restic argv carries --exclude-caches (scrubbed server log: right of the verb, left of --); flip it off and confirm the next backup's argv does not
result: [pending]

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
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps

<!-- YAML format for plan-phase --gaps consumption -->
