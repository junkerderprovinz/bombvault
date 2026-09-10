# Deferred Items — Phase 02

Out-of-scope discoveries logged during plan execution (pre-existing issues in
files this phase did not author). Not fixed here per the executor scope
boundary; recorded for a future phase or ad-hoc pass.

## From plan 02-03 (phase gate, 2026-09-10)

- **Pre-existing eslint warnings (2), unrelated to this phase's files:**
  - `web/src/components/ActivityLog.tsx:234` — `react-hooks/exhaustive-deps`
    warning: `useMemo` missing dependency `resolveName`.
  - `web/src/components/Sidebar.tsx:567` — `react-hooks/exhaustive-deps`
    warning: stale `seqRef.current` in effect cleanup.
  Both predate this phase (`eslint src` reports 0 errors; these are warnings
  only). The house convention treats `exhaustive-deps` as warn-only, so they
  are informational — flagged here so they are not rediscovered per plan.
