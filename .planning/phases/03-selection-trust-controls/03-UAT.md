---
status: testing
phase: 03-selection-trust-controls
source: 03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md
started: 2026-09-10T21:10:00Z
updated: 2026-09-10T22:02:00Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

number: 2
name: Real-instance smoke (optional — same class as Phase 1's item)
expected: |
  On the Unraid instance, flip one mount's CACHEDIR switch, reload the panel
  (switch state persists from the mounts response), run a backup of that
  container, confirm the run's restic argv carries --exclude-caches (scrubbed
  server log: right of the verb, left of --); flip it off and confirm the next
  backup's argv does not.
outcome: pass (executed by the gsd orchestrator at the user's request)
evidence: |
  Instance: BombVault-test on Unraid, redeployed from image bombvault:phase3-788d018b
  (HEAD 788d018b built on the host from the committed tree; APP_KEY and volumes
  preserved; the four stored repo paths had a stray "user/" prefix that broke
  repo resolution — corrected to bombvault/{container,vms,config,files} against
  hostSourceRoot /mnt). Target container: mcsmanager-daemon (two mounts; data
  selected, logs deselected).
  1. Flip ON via PATCH /api/containers/mcsmanager-daemon
     {"excludeCaches":{"/mnt/user/appdata/minecraft/daemon/data":true}} -> {"ok":true};
     GET mounts reload shows "excludeCaches":{"/mnt/user/appdata/minecraft/daemon/data":true}.
  2. Backup run 1 success (7s, snapshot 90dc9428..., 236,618,861 bytes). Live restic
     argv captured from /proc/<pid>/cmdline during the run:
     restic -r /host/user/bombvault/container --retry-lock 5m backup --json
     --host bombvault --tag container:mcsmanager-daemon --tag p1 --exclude-caches --
     /host/user/user/appdata/minecraft/daemon/data
     --exclude-caches sits right of the verb, left of --, between the --tag loop and
     the positionals (same slot TestBackupArgsExcludeCaches pins).
  3. Flip OFF via PATCH {"excludeCaches":{}} -> {"ok":true}; GET mounts shows {}.
  4. Backup run 2 success (4s, snapshot 7c013d45..., 8,687 bytes deduped). Live argv:
     restic -r /host/user/bombvault/container --retry-lock 5m backup --json
     --host bombvault --tag container:mcsmanager-daemon --tag p1 --
     /host/user/user/appdata/minecraft/daemon/data
     No --exclude-caches — byte-identical to the pre-feature shape (zero-value pin).
  Final stored state equals the pre-test state (no toggles). Self-backup of
  BombVault-test is refused by design, so the smoke exercised a real user container.
awaiting: none — next: item 1 (visual UAT)

## Tests

### 1. Visual UAT in a real browser (five new surfaces)
expected: Preview line, exclusions disclosure, CACHEDIR switch + InfoBubble, reset confirm, narrowing note — rendered register, tone legibility and dialog feel (jsdom asserts classes/roles/wire bodies/text only)
result: [pending]

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
passed: 1
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps

<!-- YAML format for plan-phase --gaps consumption -->
