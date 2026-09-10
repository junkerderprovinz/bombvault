---
status: testing
phase: 02-container-panel-tree-selection
source: [02-VERIFICATION.md]
started: 2026-09-10T15:50:13Z
updated: 2026-09-10T15:50:13Z
---

## Current Test

number: 1
name: Visual UAT in a real browser — container panel FoldersEditor tree states
expected: |
  Open the container panel FoldersEditor, unfold an appdata mount, toggle subfolders.
  Mixed parents show an indeterminate checkbox AND the treeitem announcing mixed
  (aria-checked="mixed"); excluded children render muted; notice rows (no-access +
  retry, truncated, empty) are visually distinct from selectable rows; the D-04
  blocked line (statusWarn) renders under the row; the tree scroll region is legible.
  jsdom asserts classes/roles/wire bodies only — this needs human eyes.
awaiting: user response

## Tests

### 1. Visual UAT in a real browser — tree states rendering
expected: Mixed parents: indeterminate checkbox + treeitem aria-checked="mixed"; excluded children muted; notice rows (no-access/retry, truncated, empty) visually distinct; statusWarn blocked line under the last-include row; scroll region legible.
result: [pending]

### 2. Real-browser keyboard walk — Tab into the tree, arrows/Enter/Space/Home/End
expected: Exactly one tabbable treeitem at every point; focus follows key moves with scrollIntoView; Space produces the byte-identical PATCH body as a checkbox click (same onToggle pipeline). Real-browser focus ordering is not observable in jsdom.
result: [pending]

### 3. Real-instance smoke (optional) — Unraid container panel against live mounts
expected: Unfold a real appdata mount, uncheck a subfolder, watch the network tab, close/reopen the section: exactly one browse per first expand, PATCH body is the canonical flat list with selectionSource "tree", reopen reconstructs exactly with no refetch. The mocked harness proves the contract, not the deployment.
result: [pending]

### 4. Review the 7 judgment-tier prohibitions (verifier verdicts non-authoritative per ADR-550 D4)
expected: Tree is not an FS browser; expansion never leaks into selection; no silent auto-exclusion; parent uncheck never deletes deeper exclusions; no second presentation of the same selection; zero-include never falls through; listing failure never blocks saving. The verifier's verdicts are all "holds" — a human confirms or corrects them.
result: [pending]

### 5. Decide the web/dist deviation (owner decision)
expected: Plan 03's artifact said "rebuilt embedded SPA committed" but only the tracked web/dist/index.html placeholder is committed; the ROADMAP note "committed in plan 03" is factually wrong. Shipped binaries are unaffected (Dockerfile web stage builds from source). Either accept the deviation (override YAML suggested in the VERIFICATION report) or require refreshing the tracked index.html + fixing the ROADMAP note. CLAUDE.md ("commit web/dist") and .gitignore lines 14-18 contradict each other — only the owner can settle which is canonical.
result: [pending]

## Summary

total: 5
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
