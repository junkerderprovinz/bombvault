---
status: complete
phase: 02-container-panel-tree-selection
source: [02-VERIFICATION.md]
started: 2026-09-10T15:50:13Z
updated: 2026-09-10T17:05:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Visual UAT in a real browser — tree states rendering
expected: Mixed parents: indeterminate checkbox + treeitem aria-checked="mixed"; excluded children muted; notice rows (no-access/retry, truncated, empty) visually distinct; statusWarn blocked line under the last-include row; scroll region legible.
result: pass (2026-09-10: verified live on the Unraid test instance by user + Playwright — tree renders after switching sidebar off "Vue simple"; BombVault-test shows /config appdata checked-by-default, /host/user mixed aria-checked, /host/boot + docker.sock disabled with blocked message, /backup expandable; empty JS console)

### 2. Real-browser keyboard walk — Tab into the tree, arrows/Enter/Space/Home/End
expected: Exactly one tabbable treeitem at every point; focus follows key moves with scrollIntoView; Space produces the byte-identical PATCH body as a checkbox click (same onToggle pipeline). Real-browser focus ordering is not observable in jsdom.
result: pass (2026-09-10: user confirmed keyboard walk in real browser)

### 3. Real-instance smoke (optional) — Unraid container panel against live mounts
expected: Unfold a real appdata mount, uncheck a subfolder, watch the network tab, close/reopen the section: exactly one browse per first expand, PATCH body is the canonical flat list with selectionSource "tree", reopen reconstructs exactly with no refetch. The mocked harness proves the contract, not the deployment.
result: pass (2026-09-10: executed by Claude via Playwright on the live Unraid instance, BombVault-test — one browse per first expand (reqs 19/20, no duplicates); toggle of /mnt/remotes produced PATCH /api/containers/BombVault-test with body {"backupPaths":["/mnt/remotes","/mnt/user/appdata/BombVault-test"],"selectionSource":"tree"} (debounced); close+reopen of the section rebuilt the tree expanded+mixed state with ZERO new network requests; BombVault-test is self-excluded from backups so the stored selection has no side effects)

### 4. Review the 7 judgment-tier prohibitions (verifier verdicts non-authoritative per ADR-550 D4)
expected: Tree is not an FS browser; expansion never leaks into selection; no silent auto-exclusion; parent uncheck never deletes deeper exclusions; no second presentation of the same selection; zero-include never falls through; listing failure never blocks saving. The verifier's verdicts are all "holds" — a human confirms or corrects them.
result: pass (2026-09-10: user confirmed all 7 prohibitions hold)

### 5. Decide the web/dist deviation (owner decision)
expected: Plan 03's artifact said "rebuilt embedded SPA committed" but only the tracked web/dist/index.html placeholder is committed; the ROADMAP note "committed in plan 03" is factually wrong. Shipped binaries are unaffected (Dockerfile web stage builds from source). Either accept the deviation (override YAML suggested in the VERIFICATION report) or require refreshing the tracked index.html + fixing the ROADMAP note. CLAUDE.md ("commit web/dist") and .gitignore lines 14-18 contradict each other — only the owner can settle which is canonical.
result: pass (2026-09-10: owner decision — deviation ACCEPTED. Placeholder-only web/dist stays canonical; Dockerfile builds from source so shipped binaries are unaffected; override recorded here per the VERIFICATION report suggestion. No ROADMAP/index.html change required.)

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps
