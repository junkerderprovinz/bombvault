# Milestones

## v1.0 Tree-Based Sub-Folder Backup Selection (Shipped: 2026-09-11)

**Phases completed:** 4 phases, 15 plans, 30 tasks

**Known verification overrides:** 2 newly acknowledged, 0 carried forward from a prior close (see STATE.md Deferred Items)

**Key accomplishments:**

- Flat "!"-prefixed selection encoding with per-class maximal-root pruning: mixed tree selections store canonically in the existing backupPaths set (zero migration), readers classify includes vs explicit-none, and backups hand restic maximal-include positionals with zero derived --exclude flags
- GET /api/browse hardened into the tree's node listing: os.Root containment behind the unchanged lexical reject, additive status trio (ok/missing/restricted/error), 500-entry cap with truncated flag, and a pinned ?hidden=1 opt-in — FolderBrowser contract byte-identical.
- Restore selectors now map onto the CHOSEN snapshot's recorded Paths (two-pass longest-prefix) with per-path skips recorded as scrubbed run notes, empty intersections aborting before any destructive teardown, and the restic 0.17 positional behaviors the design relies on contract-pinned for CI.
- Mounts endpoint renders stored exclusions as a first-class host-form `excluded` array (never stale custom paths), and a tree-source deselect-everything is refused at the PATCH boundary with a machine-routable `code:"empty-selection"` — the backend half of INTEG-04, plus the roadmap-prescribed Key Decisions bookkeeping
- Stored exclusion branches are now enforced as restic `--exclude` patterns on the backup argv (positionals stay maximal-root includes), closing the gap where the engine backed up branches the UI advertised as excluded — plus the four planning-doc drifts realigned.
- TREE-06/D-06 outcome rows pinned by dom tests (retry refetch, truncated notice outside the tree, rejected promises settle) and the FoldersEditor save flow hardened with a one-deep serialized PATCH queue whose failure revert re-derives from the live mirror, plus the full D-04 zero-include block (shake + warn line, whole-item counting, coded-envelope backstop).
- The selection tree is now fully keyboard-operable per the APG TreeView checkbox variant (roving tabindex over the same flat model that renders, Space riding the exact click pipeline) with uniform lazy-node aria geometry, and the INTEG-01 seam is closed: sub-includes the server classifies as custom rows are absorbed under their reachable mount — one presentation per path — with the wire contract, D-04 guard, reopen cache, and 64-cap pinned through the real FoldersEditor.
- Per-root exclude-caches toggle persisted as a targets JSON map (migration v100) and compiled at backup time into restic's constant --exclude-caches flag through Mode threading, with atomic PATCH validation and nil-safe mounts serving
- Per-root "{n} paths" preview and collapsible "{n} exclusions" review list derived purely from the stored flat-selection mirror — zero new endpoints, zero new state sources (D-01, D-03, D-04).
- Reset-selection exit to auto-detection, a lastBackup-gated narrowing note, and the per-root CACHEDIR.TAG switch — all three riding the one-deep serialized PATCH queue, now generalized to compose a single body from two owed mutation classes.
- A file set's tree selection now compiles at backup time into maximal-root restic positionals with derived excludes at the single site — while a set never touched by the tree backs up byte-identically to before (NULL-column legacy switch, pinned by the untouched TestBackupFileSet).
- A file set's tree selection is now writable through PATCH — validated per entry against the resolved root before any store write, normalized, capped, refused when empty (D-06) — and an in-place restore whose snapshot contains none of what the set selects now aborts synchronously before tearing anything down (D-08).
- The Phase 2 SelectionTree mounts on every file-set card behind a Choose folders disclosure - one synthetic root seeded honestly from the NULL column, one-deep serialized saves with D-06 refusal on both halves, and 4 new i18n keys across all 42 locales.
- The file-set dialog now warns that a path edit clears the ticked selection, the exclusions review list is pinned onto the Files page's shared tree, and the phase gate is green end to end.

---
