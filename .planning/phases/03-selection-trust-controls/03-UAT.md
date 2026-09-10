---
status: complete
phase: 03-selection-trust-controls
source: 03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md
started: 2026-09-10T21:10:00Z
updated: 2026-09-10T23:22:07Z
---

## Current Test

[testing complete]

## Tests

### 1. Visual UAT in a real browser (five new surfaces)
expected: Preview line, exclusions disclosure, CACHEDIR switch + InfoBubble, reset confirm, narrowing note — rendered register, tone legibility and dialog feel (jsdom asserts classes/roles/wire bodies/text only)
result: pass
reported: orchestrator-executed over the live instance (see Current Test evidence); screenshots shown to the user in-session. POST-UAT finding from the user: the CACHEDIR sub-row (indent 16 = the depth-1 child indent) rendered between an expanded root and its first child, reading as that subtree's first subfolder. Fixed in ec4d4044: the row now renders AFTER the expanded children group (option chosen by the user over outdenting or hoisting out of the tree); dom pin relaxed from immediate-sibling to own-fragment scan; redeployed to the test instance (phase3-9d53ef7a) and re-verified live — root + subfolders contiguous, switch below the branch, collapsed roots unchanged (switch directly beneath)

### 2. Real-instance smoke (optional — same class as Phase 1's item)
expected: On the Unraid instance, flip one mount's CACHEDIR switch, reload the panel (switch state persists from the mounts response), run a backup of that container, confirm the run's restic argv carries --exclude-caches (scrubbed server log: right of the verb, left of --); flip it off and confirm the next backup's argv does not
result: pass
reported: orchestrator-executed at the user's request (see Current Test evidence)

### 3. Judgment-tier prohibition review (2 items)
expected: A human confirms the 2 non-authoritative LLM verdicts (excludeCaches map keys never reach argv; no fanout affordance / no snapshot-content comparison) or deposits corrections — per ADR-550 D4 they cannot be silently absorbed into a pass
result: pass
reported: user confirmed both verdicts verbatim ("Oui confirmé pour tout", 2026-09-10) — map keys never reach argv (containment validation + boolean union only) and the fanout/snapshot-comparison prohibitions hold as documented

### 4. MVP-mode format decision
expected: The ROADMAP marks Phase 3 "Mode: mvp" but the goal is not User Story format (user-story.validate → false); this verification is goal-backward against the 4 numbered Success Criteria (Phase 1 precedent). Either accept that basis or run `/gsd mvp-phase 3` to set a User Story goal and re-verify in MVP form
result: pass
reported: user accepted the goal-backward basis against the 4 numbered Success Criteria (the recommended option, Phase 1 precedent) — no MVP re-plan needed

### 5. REQUIREMENTS.md INTEG-04 bookkeeping
expected: The row still reads "UI semantics pending" although the documented UI semantics now exist and are verified — flip the row to Complete at ship time (/gsd-ship or /gsd-docs-update) or leave it to the milestone close; pure metadata, no code impact
result: pass
reported: decision made and confirmed — the row flip is left to the milestone close (a sanctioned disposition per this item's own expected text); the REQUIREMENTS.md edit itself is ship-time work, not phase-3 scope

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

<!-- YAML format for plan-phase --gaps consumption -->
