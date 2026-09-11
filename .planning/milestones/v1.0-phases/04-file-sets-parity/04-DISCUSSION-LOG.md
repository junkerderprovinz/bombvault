# Phase 4: File Sets Parity - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-10
**Phase:** 04-file-sets-parity
**Areas discussed:** Selection model, Persistence, Tree scope, Backup compile, Empty selection, Parity scope, Restore compat
**Mode:** --auto (all areas auto-selected on their recommended defaults, per /gsd-autonomous)

---

## Selection model — single-root vs multi-root

| Option | Description | Selected |
|--------|-------------|----------|
| Single-root | The tree only refines inside the picked root; root stays the lone positional | |
| Multi-root (strict superset) | Uncheck the root, tick several children → several restic positionals | ✓ |
| Exclusion-only | Keep root positional, tree selection only carves exclusions | |

**User's choice:** auto-selected (recommended default)
**Notes:** ROADMAP note + research/SUMMARY both recommend multi-root; `FilesRestic.Backup` already takes `paths []string` so the orchestrator is untouched.

---

## Persistence

| Option | Description | Selected |
|--------|-------------|----------|
| Nullable selected_paths column | Append-only migration on file_sets, same bare+`!` flat encoding; NULL = legacy | ✓ |
| Reuse Path field | Encode selection into the existing single Path string | |
| New JSON nested-tree column | Store the tree shape directly (banned by Out of Scope) | |

**User's choice:** auto-selected (recommended default)
**Notes:** research recommendation; SELECT-02 zero-impact philosophy applied to filesets — existing sets behave byte-identically while the column stays NULL.

---

## Tree scope / roots

| Option | Description | Selected |
|--------|-------------|----------|
| Root = set's resolved Path | Single visible root; multi-root selection lives under it; no browsing outside | ✓ |
| Whole host mount root | Tree shows the mount root with the set's Path ticked; siblings selectable | |
| Multiple custom roots | Allow adding arbitrary roots like container custom paths | |

**User's choice:** auto-selected (recommended default)
**Notes:** Out of Scope bans arbitrary-path browsing; FolderBrowser stays the boundary picker at create; path-less Discover sets show no tree until a path exists.

---

## Backup compile

| Option | Description | Selected |
|--------|-------------|----------|
| Maximal includes → positionals; exclusions → --exclude | Same engine contract as containers (01-05 discipline) | ✓ |
| Flatten to explicit leaf positionals | One positional per leaf folder | |

**User's choice:** auto-selected (recommended default)
**Notes:** NULL column ⇒ argv `[SourceDir]` byte-identical (pinned test). Snapshot `Paths` must equal the ticked roots (success criterion 2).

---

## Empty selection semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Refuse client+server; exit = delete the set | Same posture as the Phase 1 guard; no auto-detect fallback exists in files domain | ✓ |
| Allow empty + skip backups silently | Silent non-backup — cardinal sin | |
| Allow empty + auto-restore to full root | Silent re-widening contradicts explicit deselection | |

**User's choice:** auto-selected (recommended default)
**Notes:** INTEG-04 analog without the Reset exit — there is no auto-detection to reset to when Path is explicit.

---

## Parity scope (which Phase 3 trust surfaces come along)

| Option | Description | Selected |
|--------|-------------|----------|
| Tree + preview count + exclusions list | Client-side derivations from the same classification; CACHEDIR toggle excluded | ✓ |
| Tree only | Minimal INTEG-02; no preview, no list | |
| Full parity incl. CACHEDIR toggle | Adds store field + argv union + tests beyond the success criteria | |

**User's choice:** auto-selected (recommended default)
**Notes:** goal-backward to success criteria 1-2; 03-CONTEXT deferred line promised "même preview/liste". CACHEDIR for file sets recorded as deferred (RESTIC-01 is container-scoped).

---

## Restore compat (old single-root snapshots of a now multi-root set)

| Option | Description | Selected |
|--------|-------------|----------|
| RESTORE-01 longest-prefix mapping reused | Intersection non-empty restores; empty aborts pre-teardown | ✓ |
| Per-domain files-specific mapping | Second implementation of the same semantics | |
| Refuse restore after shape change | Breaks existing restores unnecessarily | |

**User's choice:** auto-selected (recommended default)
**Notes:** one mapping implementation, no second semantics.

---

## Claude's Discretion

- i18n labels + 42-locale parity budget; placement/render inside Files.tsx (PAGE_SHELL, tokens, glim-* classes)
- Reuse SelectionTree as-is with an adapted root source vs a thin wrapper — planning's call, provided no second implementation of selection semantics
- Exact wire shape of the `selectedPaths` PATCH field (additive + boundary-validated)
- Test strategy (co-located .test.ts/.dom.test.tsx, restic_args_test.go pins, white-box internal tests)

## Deferred Ideas

- CACHEDIR.TAG toggle for file sets (v2 candidate / next milestone)
- TREE-07/08, SELECT-05/06 — already v2 in REQUIREMENTS.md
- Fanout "exclude this subfolder instead" into ExcludesEditor — out of v1

---

*Phase: 04-file-sets-parity*
*Discussion log generated: 2026-09-10*
