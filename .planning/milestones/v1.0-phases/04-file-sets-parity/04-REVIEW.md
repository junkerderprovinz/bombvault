---
phase: 04-file-sets-parity
reviewed: 2026-09-11T00:00:00Z
depth: standard
files_reviewed: 59
files_reviewed_list:
  - internal/api/files_internal_test.go
  - internal/api/handlers.go
  - internal/api/handlers_test.go
  - internal/api/selection.go
  - internal/api/service.go
  - internal/api/service_test.go
  - internal/backup/files_orchestrator.go
  - internal/backup/files_orchestrator_test.go
  - internal/restic/restic_positionals_contract_test.go
  - internal/store/filesets.go
  - internal/store/filesets_test.go
  - internal/store/migrate.go
  - web/dist/index.html
  - web/src/components/SelectionTree.tsx
  - web/src/lib/api.ts
  - web/src/lib/i18n.filesSelection.test.ts
  - web/src/lib/i18n.ts
  - web/src/lib/locales/ar.ts
  - web/src/lib/locales/bg.ts
  - web/src/lib/locales/ca.ts
  - web/src/lib/locales/cs.ts
  - web/src/lib/locales/da.ts
  - web/src/lib/locales/el.ts
  - web/src/lib/locales/es.ts
  - web/src/lib/locales/et.ts
  - web/src/lib/locales/eu.ts
  - web/src/lib/locales/fa.ts
  - web/src/lib/locales/fi.ts
  - web/src/lib/locales/fr.ts
  - web/src/lib/locales/gl.ts
  - web/src/lib/locales/he.ts
  - web/src/lib/locales/hi.ts
  - web/src/lib/locales/hr.ts
  - web/src/lib/locales/hu.ts
  - web/src/lib/locales/id.ts
  - web/src/lib/locales/is.ts
  - web/src/lib/locales/it.ts
  - web/src/lib/locales/ja.ts
  - web/src/lib/locales/ko.ts
  - web/src/lib/locales/lt.ts
  - web/src/lib/locales/lv.ts
  - web/src/lib/locales/ms.ts
  - web/src/lib/locales/nl.ts
  - web/src/lib/locales/no.ts
  - web/src/lib/locales/pl.ts
  - web/src/lib/locales/pt.ts
  - web/src/lib/locales/ro.ts
  - web/src/lib/locales/ru.ts
  - web/src/lib/locales/sk.ts
  - web/src/lib/locales/sl.ts
  - web/src/lib/locales/sr.ts
  - web/src/lib/locales/sv.ts
  - web/src/lib/locales/th.ts
  - web/src/lib/locales/tr.ts
  - web/src/lib/locales/uk.ts
  - web/src/lib/locales/vi.ts
  - web/src/lib/locales/zh.ts
  - web/src/pages/Files.tree.dom.test.tsx
  - web/src/pages/Files.tsx
findings:
  critical: 1
  warning: 1
  info: 4
  total: 6
status: issues_found
---

# Phase 4: Code Review Report

**Reviewed:** 2026-09-11
**Depth:** standard
**Files Reviewed:** 59
**Status:** issues_found

## Summary

Reviewed the full Phase 4 tracer: v101 migration + nullable `selected_paths` column, the owned store setter, `fileSetPositionals`/compile wiring, the PATCH boundary (`selectedPaths` pointer, D-06 refusal, clear-on-path-change), the D-08 restore guard, the Files-page `FileSetFoldersEditor` (SelectionTree reuse + serialized PATCH queue), the i18n fan-out, and the rolled `web/dist/index.html`.

The backend is in good shape: migration v101 is append-only with the NUMBERING HAZARD comments preserved; the nullable scan into `*string` is correct; the NULL-vs-`'[]'` discipline holds end to end (single writer refuses zero-include saves before any write; the clear writes SQL NULL); per-entry boundary validation (trim/split/`path.Clean`/segment-aligned containment against the resolved root, 64-entry cap, atomic whole-save rejection) is sound — `path.Clean` resolves `..` before the containment check, so traversal under a sibling-prefix root (`/data/doc` vs `/data/docs`) is impossible on both the write side and the compile-side re-anchor; argv discipline holds (positionals via typed builders after `--`, derived excludes ahead of it, no shell); new error text crosses to the UI through `failEnvelope`/`codedFailEnvelope` scrubbing; the D-08 guard sits synchronously before anything destructive and its `mapRestorePaths` ancestor-fallback is consistent with `RestorePath`'s `<id>:<path>` subtree selector semantics. The zero-second-implementation lock holds for selection semantics (Files.tsx consumes `selectionTree.ts` + `SelectionTree`; `NormalizeSelection` remains the only prune). i18n parity is exact: all 4 new keys in en + de + all 40 chunks, em-dash ban held. `web/dist/index.html` is a coherent build artifact (hashed bundle refs, theme bootstrap, no secrets or paths). `gofmt -l` on the touched tree is silent; every behavior the summaries claim is pinned by a test that exists and reads correctly.

The one serious defect is client-side: `FileSetFoldersEditor` seeds its selection mirror once at mount and never re-syncs with the `set` prop, which defeats the phase's own A3 path-change-clear rule from the UI side and can wedge the editor until a page reload. One backend robustness warning (non-atomic path-update + selection-clear pair) and four info-level items round it out.

## Critical Issues

### CR-01: FileSetFoldersEditor mirror never re-syncs — a path edit leaves a stale/wedged editor and can silently resurrect a cleared selection

**File:** `web/src/pages/Files.tsx:1130-1136` (seed + `useState`/`useRef` from mount-time `set`); call site `web/src/pages/Files.tsx:1561`; card keying `web/src/pages/Files.tsx:1896-1898`; silent refetch `web/src/pages/Files.tsx:1619-1630`

**Issue:** The editor computes `seed = splitFlatSet(noPath ? [] : (set.selectedPaths ?? [root]))` and copies it into `useState`/`useRef` initializers — read exactly once at mount. The component is rendered per card without a key tied to the selection anchor (`{!noPath && <FileSetFoldersEditor set={set} ...>}`), rows are keyed by `s.id` (stable across refetches), and `loadSets()` never toggles `loading`, so nothing remounts the editor when `set` changes. The in-code justification ("rows are keyed by set id, so instance identity IS set identity and no reseed path exists") conflates set id with set content — and Phase 4 itself introduced the mutation that breaks it: editing the set's folder through `FileSetDialog` triggers the server-side A3 clear (`selected_paths = NULL`), after which the still-mounted editor keeps the OLD anchor's mirror. Three consequences:

1. **Dishonest render:** a just-cleared (NULL) set renders its stale include instead of the honest NULL seed (root CHECKED at "1 paths", UI-SPEC item 9) — the tree shows a selection the server no longer has.
2. **Wedged editor:** the next toggle PATCHes the full flat list *including the stale old-root entries*; `Service.SetFileSetSelectedPaths` validates per-entry against the NEW resolved root and rejects atomically ("selected path %q is not under the set's source folder"). Every subsequent toggle re-sends the same stale list and is refused the same way. There is no Reset and the stale entry is not rendered anywhere under the new root, so the user cannot fix it — the editor is bricked until a full page reload.
3. **Silent resurrection:** when the path moves *deeper* (e.g. `data` → `data/docs`), stale entries that happen to fall under the new root pass containment, so the same PATCH stores a selection the A3 rule just deliberately cleared — the "clear-wins" guarantee exists only for clients that re-sync.

**Fix:** Remount the editor when the anchor (or served selection) changes — the cheapest correct fix is at the call site:

```tsx
{!noPath && (
  <FileSetFoldersEditor
    key={`${set.id}:${set.path}:${set.selectedPaths ? "set" : "null"}`}
    set={set}
    hostMountRoot={hostMountRoot}
    t={t}
  />
)}
```

or, if remount cost matters, reseed via an effect that compares `set.path`/`set.selectedPaths` against the seed inputs and resets mirror state when they diverge (mirroring the "instance identity IS set identity" comment with an actual mechanism). Add a dom pin: "path change via the dialog clears the editor's mirror without a remount".

## Warnings

### WR-01: Path update and selection clear are two independent writes — the A3 clear is best-effort, not atomic

**File:** `internal/api/handlers.go:4561` (UpdateFileSet), `internal/api/handlers.go:4579-4597` (clear/save switch)

**Issue:** `handlePatchFileSet` persists the merged set with `h.store.UpdateFileSet(fs)` and only afterwards runs the clear-on-path-change branch (`h.store.SetFileSetSelectedPaths(id, nil)`). If the clear fails (store error, lock contention), the handler returns a fail envelope while the path has ALREADY changed — the client is told the save failed, but the new path is live and the stored selection remains anchored to the old root. The compile-time re-anchor in `fileSetPositionals` prevents any backup-scope leak from this torn state (verified: stale entries filter against the fresh root, filtered-empty falls back to `[src]`), so this is a robustness gap, not a safety hole. Additionally, because the switch sits before the `ScheduleCadence` handling, a selection-write failure also silently skips a cadence change sent in the same request.

**Fix:** Collapse the pair into one store statement so the invariant "a path change never coexists with an old-anchor selection" holds at the storage layer, e.g. a `Repo` method used when `pathChanged`:

```go
// UPDATE file_sets SET name=?, path=?, excludes=?, enabled=?, selected_paths=NULL WHERE id=?
res, err := r.db.Exec(`UPDATE file_sets SET name=?, path=?, excludes=?, enabled=?, selected_paths=NULL WHERE id=?`, ...)
```

(`file_sets` gains no new column semantics; the nullable column already accepts NULL in the same row write.) If a transactional setter is out of scope for the fix pass, at minimum order the clear BEFORE `UpdateFileSet` so the failure mode degrades to "path unchanged, selection cleared" and document the residual torn state at the switch.

## Info

### IN-01: Locale fan-out glued two object properties onto one physical line in 26 of 40 chunks

**File:** `web/src/lib/locales/ar.ts:1184` (and 25 siblings: bg, ca, cs, da, el, es, et, eu, fa, fi, fr, gl, he, hi, hr, hu, id, is, it, ja, ko, lt, lv, ms, nl, no, pl, pt, ro, ru, sk, sl, sr, sv, th, tr, uk, vi, zh — whichever carry the pattern)

**Issue:** The scripted insertion appended `"files.pathChangeHint": "...",  "settings.filesEnabled": "..."` onto a single line (the removed `settings.filesEnabled` line was re-added glued to the new key). Valid TypeScript, invisible to `tsc --noEmit`/eslint, but it breaks the file's one-key-per-line formatting and will garble the next diff that touches `settings.filesEnabled` in those files.

**Fix:** One formatting pass splitting the glued lines (e.g. targeted `sed`/prettier over the 26 files); no semantic change.

### IN-02: Compiled file-set positionals are not existence-filtered — containers filter, files let restic skip with per-run log noise

**File:** `internal/api/service.go:8779` (compile site), `internal/api/selection.go:240-254` (pure helper — correctly cannot stat), contrast `internal/api/service.go:3931` (`onlyExistingPaths` for containers)

**Issue:** The container pipeline filters vanished paths at backup time (`effectiveBackupPaths` → `onlyExistingPaths`, with a documented #175 rationale distinguishing "deselected" from "share unmounted"). The file-set compile emits stale positionals verbatim: a selected subfolder deleted after the save makes restic log a read error and exit 3 on every run — which `Restic.Backup` correctly maps to success-with-skips (`internal/restic/restic.go:1717-1720`), so runs succeed, but each one logs skip noise and nothing on the Files card surfaces the stale entry (containers render `folders.notReachable`/`customMissing` row-level warnings). The root-level case is properly guarded by the `os.Stat(src)` pre-check, so only children are affected.

**Fix:** Apply `onlyExistingPaths` to the re-anchored positionals at the `BackupFileSet` compile site (the helper stays pure), and/or surface a stale-entry badge via `pathExists`-style data in `FileSetView`. Low priority — outcome parity with containers already holds.

### IN-03: Serialized PATCH queue is a second hand-copy of the containers' Pattern-4 machinery

**File:** `web/src/pages/Files.tsx:1139-1249` (`scheduleSave`/`attemptSave`/`revertFrom`)

**Issue:** The phase's zero-second-implementation lock holds for selection semantics (`applyToggle`/`splitFlatSet`/`toFlatList`/`SelectionTree` are shared — verified), but the one-deep serialized save queue with delta-revert now exists as two subtly divergent copies (Containers.tsx's queue with its reset/caches owed classes vs Files.tsx's narrowed variant whose `revertFrom` is the newest mutation of the algorithm). The divergence is documented, but the next queue fix must be applied twice and the delta-revert subtleties (Pitfall 5) are exactly the kind of logic that drifts.

**Fix:** Extract a `useSerializedSaveQueue` hook (mirror ref + `scheduleSave`/`attemptSave`/`revertFrom`, parameterized by the PATCH call and the owed classes) once both call sites are stable. Not blocking.

### IN-04: Disclosure region announced with the container domain's title

**File:** `web/src/pages/Files.tsx:1309`

**Issue:** `role="region" aria-label={t("folders.title")}` labels the file-set card's tree region with the Folders/containers domain heading, which screen readers announce on a Files-page card. Every other copy surface in this phase was carefully domain-routed (`blockedMessage`, the `files.*` keys), so this one label is off-domain.

**Fix:** Point the label at a files-domain key (reuse `t("files.foldersToggle")` if no dedicated region title exists).

---

**Verified clean (notable):** v101 append-only with prior migrations byte-identical; nullable `*string` scan; NULL never `'[]'` (single validated writer + NULL clear); per-entry containment is segment-aligned after `path.Clean` (sibling-prefix and `..` traps closed at both write and compile sites); 64-entry cap enforced pre-read; `[]`/exclusions-only refused via coded `empty-selection` envelope with prior selection untouched; derived-exclude tail mirrors the container compile line and is stale-root-harmless; D-08 guard scoped to in-place with `len(Paths) > 0` and consistent with `RestorePath`'s selector semantics; client D-06 block fires before any request; queue never exceeds one concurrent PATCH (pinned); new i18n keys present in en + de + all 40 locale chunks with no em dashes; `web/dist/index.html` is a clean build artifact (no secrets, no absolute paths); `gofmt -l` silent on the touched tree.

---

_Reviewed: 2026-09-11_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
