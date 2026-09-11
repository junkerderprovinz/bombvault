# Phase 4: File Sets Parity - Pattern Map

**Mapped:** 2026-09-10
**Files analyzed:** 8 (all modifications except the migration entry + new tests)
**Analogs found:** 8 / 8 — this is a parity phase; every seam has a Phase 1–3 precedent in-repo

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/store/migrate.go` (v101 entry) | migration | batch | v99 `file_sets_schedule_cadence` (migrate.go:1302-1304) — same table, same ALTER shape | exact |
| `internal/store/filesets.go` (column + `SetFileSetSelectedPaths`) | model | CRUD | `SetFileSetScheduleCadence` (filesets.go:147-157) — owned-setter rationale verbatim in file | exact |
| `internal/backup/files_orchestrator.go` (additive `SourcePaths`) | service | request-response (subprocess) | its own `SourceDir` wrap at line 44 (`[]string{d.SourceDir}`) — additive field beside it | exact (self) |
| `internal/api/handlers.go` (`handlePatchFileSet` additive field) | controller | request-response | same handler's pointer-body + `ScheduleCadence` owned-field pattern (handlers.go:4498-4507) | exact (self) |
| `internal/api/service.go` (`BackupFileSet` compile + restore D-08) | service | transform → subprocess argv | container compile `service.go:4256` (`excludedBranches(tg.SelectedPaths)`) + container empty-selection guard (`service.go:3802, 3870-3875`) + container restore mapping guard (`service.go:5524-5527`) | exact |
| `web/src/lib/api.ts` (`FileSetView.selectedPaths` + patch field) | model (wire mirror) | request-response | existing `FileSetView`/`patchFileSet` shapes extended additively (api.ts:2264-2285, 2348-2364) | exact (self) |
| `web/src/pages/Files.tsx` (tree mount, mirror, queue, refusal, preview) | component | event-driven | `Containers.tsx` tree editor: serialized one-deep PATCH queue (`:816-851`), drain (`:944-1099`), `applyMirror` (`:885-889`) | exact |
| `web/src/components/SelectionTree.tsx` + `web/src/lib/selectionTree.ts` | component + utility | event-driven | reuse UNCHANGED (READ-ONLY) — at most additive optional props | n/a (source, not analog) |

## Pattern Assignments

### `internal/store/migrate.go` — v101 entry (migration, batch)

**Analog:** v99 `file_sets_schedule_cadence` (migrate.go:1302-1304) — same table.

```go
version: 99, name: "file_sets_schedule_cadence",
alreadySatisfied: columnPresent("file_sets", "schedule_cadence"),
sql:              "ALTER TABLE file_sets ADD COLUMN schedule_cadence TEXT NOT NULL DEFAULT '';",
```

**Differences for v101:**
- Nullable per D-03: `ALTER TABLE file_sets ADD COLUMN selected_paths TEXT;` — NO default, NO NOT NULL (the `received_repos.last_check_ok` nullable precedent, migrate.go:826, scanned via `sql.Null*`).
- **NO `alreadySatisfied` guard** — the guard exists ONLY for the contested 89/90/91 recovery (migrate.go NUMBERING HAZARD note, 50-67). Fresh number, unconditional body.
- Carry the load-bearing "why" comment: NULL = "never touched by the tree" = legacy single-positional argv; every existing row reads NULL after the ALTER.

### `internal/store/filesets.go` — column + owned setter (model, CRUD)

**Analog:** `SetFileSetScheduleCadence` (filesets.go:147-157); rationale comment at 139-146 applies word-for-word to the new setter.

```go
func (r *Repo) SetFileSetScheduleCadence(id, cadence string) error {
	res, err := r.db.Exec(
		`UPDATE file_sets SET schedule_cadence = ? WHERE id = ?`, cadence, id)
	if err != nil {
		return fmt.Errorf("SetFileSetScheduleCadence: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("SetFileSetScheduleCadence: no file set %q", id)
	}
	return nil
}
```

**Four-positional-lists hazard in miniature** — ALL FOUR touch together:
1. Three SELECTs (filesets.go:90, 111, 119): `SELECT id, name, path, excludes, enabled, schedule_cadence, created_at` → add `selected_paths`.
2. `scanFileSet` (:175-188) — scan the new column as `*string` / `sql.NullString` (NULL is common: every legacy row); NULL ⇒ nil `FileSet.SelectedPaths`. Current scan shape:

```go
func scanFileSet(s scanner) (FileSet, error) {
	var fs FileSet
	var exJSON string
	var enabled int
	err := s.Scan(&fs.ID, &fs.Name, &fs.Path, &exJSON, &enabled, &fs.ScheduleCadence, &fs.CreatedAt)
```

3. `UpdateFileSet` (:74-77) — **stays untouched**; its explicit column list is what protects the new column from being clobbered by name/path/excludes edits:
```go
UPDATE file_sets SET name = ?, path = ?, excludes = ?, enabled = ? WHERE id = ?
```
4. `CreateFileSet` INSERT (:53-57) — may omit the column (nullable ADD COLUMN without default accepts INSERTs omitting it ⇒ stores NULL).

Store treats `selected_paths` as an opaque string (selection.go file doc); encoding/decoding lives in the API tier. Setter does JSON encode/decode of `[]string` (nil ⇒ SQL NULL, not `'[]'` — the distinction IS the legacy switch).

### `internal/backup/files_orchestrator.go` — additive `SourcePaths` (service, subprocess)

**Analog:** the file's own single-path wrap (verified verbatim):

```go
type FilesRestic interface {
	Backup(ctx context.Context, repo string, paths, tags []string, excludes ...string) (Summary, error)
}
// FileSetBackupDeps:
	// SourceDir is the container-visible resolved path of the set's folder
	// (paths.Resolve(HostMountRoot, set.Path), e.g. /host/user/data/docs).
	SourceDir string
// BackupFileSetDir, line 44:
	summary, err := d.Restic.Backup(ctx, d.Repo, []string{d.SourceDir}, []string{"fileset:" + d.SetName}, d.Excludes...)
```

**Change:** additive `SourcePaths []string` on `FileSetBackupDeps` with a load-bearing comment (nil ⇒ legacy `[SourceDir]` byte-identical; CONTEXT D-01's "orchestrator unchanged" is interface-true, file-false). Line 44 becomes: use `d.SourcePaths` when non-nil, else `[]string{d.SourceDir}`. Interface `FilesRestic` untouched. Update fakes in `files_orchestrator_test.go`.

### `internal/api/handlers.go` — `handlePatchFileSet` additive field (controller, request-response)

**Analog:** the same handler's pointer-body + owned-field pattern (handlers.go:4498-4507, verified verbatim):

```go
var body struct {
	Name     *string   `json:"name"`
	Path     *string   `json:"path"`
	Excludes *[]string `json:"excludes"`
	Enabled  *bool     `json:"enabled"`
	// #199. Its own field rather than part of the set, because the store
	// setter is separate for the same reason: a form that does not know
	// about the cadence must not be able to clear one by omitting it.
	ScheduleCadence *string `json:"scheduleCadence"`
}
```

**Rules to carry:**
- `decodeBody` runs DisallowUnknownFields ⇒ the new field MUST be declared even though optional (container precedent rationale, handlers.go:1118-1122).
- `SelectedPaths *[]string`; nil = untouched (never call the setter). Present ⇒ validate every entry before ANY store write: `TrimSpace` + non-empty + containment under `paths.Resolve(HostMountRoot, mergedPath)` using segment-aligned `isAtOrUnder`/`paths.Within` (never raw prefix — `/a` must not match `/ab`) + 64-entry cap (excludeCaches precedent) + normalize via `NormalizeSelection` + zero-includes ⇒ refuse (D-06) via `codedFailEnvelope(err, "empty-selection")` (handlers.go:1160-1162 container precedent). One bad entry rejects the whole save atomically.
- **Pitfall-2 layer 1:** if `body.Path != nil` and it changes the resolved root, clear selected_paths in the same save (selection was expressed against the old root). Copy the existing comment style — the rename guard at 4533-4547 shows the house shape.
- `toContainerPath` does NOT apply here (that maps HostSourceRoot→HostMountRoot for containers; a file set's resolved root is already mount-root space — service.go:1132-1148).

### `internal/api/service.go` — compile step + restore mapping (service, transform)

**Analog A — compile (container precedent, service.go:4256 verbatim):**
```go
Excludes: append(s.resolveExcludePatterns(tg.Excludes, in), excludedBranches(tg.SelectedPaths)...),
```
Inside `BackupFileSet` (~8706, after the existing fresh read + resolve):
```go
set, err := s.store.GetFileSet(id)
// ...
if strings.TrimSpace(set.Path) == "" {
    return backup.Summary{}, fmt.Errorf("files backup: file set %q has no source path configured. Set a path before backing up", set.Name)
}
src, err := paths.Resolve(s.cfg.HostMountRoot, set.Path)
// ...
sum, err := backup.BackupFileSetDir(fctx, backup.FileSetBackupDeps{ SourceDir: src, ... })
```
Compile shape: NULL `SelectedPaths` ⇒ positionals `[src]` (legacy, pinned by `TestBackupFileSet`, service_test.go:293-295); else positionals = `includesOnly(set.SelectedPaths)` **re-anchored** — filter entries to at-or-under the freshly resolved `src` (Pitfall 2 layer 2, mandatory); empty after filtering ⇒ fall back to `[src]`, never an unanchored positional. Excludes = `set.Excludes` + `excludedBranches(set.SelectedPaths)...`. All triggers funnel here (StartBackupFileSet :4737, batch :4819, everything.go:429).

**Analog B — empty-selection refusal (adapt, do NOT copy — no selectionSource carrier, no prior-non-empty check needed for files):**
```go
var errEmptySelection = errors.New("an explicit empty selection would re-enable automatic appdata detection")
// handlers.go:
if errors.Is(err, errEmptySelection) {
    writeJSON(w, http.StatusOK, codedFailEnvelope(err, "empty-selection"))
```
Files message orients to DELETE the set (no auto-detection fallback to reset to — D-06).

**Analog C — restore mapping (D-08):** `mapRestorePaths` (selection.go:236-239, doc says "File Sets reuse it in Phase 4"):
```go
func mapRestorePaths(stored, snapshotPaths []string) (mapped, skipped []string)
// container guard shape, service.go:5524-5527:
mapped, skipped := mapRestorePaths(tg.AppdataPaths, chosen.Paths)
if len(tg.AppdataPaths) > 0 && len(mapped) == 0 {
    return containerRestorePlan{}, errors.New("nothing to restore for this item from this snapshot")
}
```
The stored list to map = the set's **compiled positional list** (legacy `[src]` when NULL, else re-anchored `includesOnly`) — NOT `set.Path` alone, NOT the raw flat set (Pitfall 7). Empty mapped ⇒ abort BEFORE any restore work, scrubbed message. Wire into in-place restore; leave to-folder whole-tree semantics unless the plan opts to rework (open question 1).

### `web/src/pages/Files.tsx` — tree mount + queue (component, event-driven)

**Analog:** `Containers.tsx` tree editor. Queue refs (816-851, verified verbatim):

```tsx
const mirrorRef = useRef<{ inc: Set<string>; exc: Set<string> }>({ inc: new Set(), exc: new Set() });
const queueRef = useRef<{ inFlight: boolean; dirty: boolean; reload: boolean }>({ inFlight: false, dirty: false, reload: false });
const owedRef = useRef<Set<"paths" | "caches">>(new Set());
const pendingRowsRef = useRef<Set<string>>(new Set());
```

Single mirror-write helper (885-889) — every mutation lands here so the ref the queue reads and the state the tree renders never drift:

```tsx
function applyMirror(inc: Set<string>, exc: Set<string>): void {
	mirrorRef.current = { inc, exc };
	setIncludes(inc);
	setExclusions(exc);
}
```

Drain sends the LIVE mirror's full list once (`body.backupPaths = toFlatList(live.inc, live.exc)` at :1021 — files mirror sends `selectedPaths`). Rules: rapid toggles never race; failure reverts by re-deriving from the live mirror; client-side zero-includes block before any PATCH (D-06); preview "N paths" = `rootIncludeCount` and exclusions list = `rootExclusions` — both from `selectionTree.ts`, never DOM/loaded-children counts (Pitfall 5; Phase 3 pinned rule: "the visible number equals the bare positionals the next backup hands restic"). Page uses PAGE_SHELL + `carbon-*`/`status*` tokens + `glim-*` classes; status colors on badges only; every user string via `t()`; no em dashes.

**SelectionTree mounting (Pitfall 6):** feed ONE root through existing shapes — a mount-like row whose `source` is the resolved set root; empty `excludeCaches` map + no-op `onToggleCaches` (or make those props optional — additive, non-semantic). Browse prefix is `hostMountRoot` (NOT `hostSourceRoot`); `Files.tsx:1289` already holds it. localStorage expansion key scoped by set id. No fork, no edits to `classifyNode`/`applyToggle`.

### `web/src/lib/api.ts` — wire mirror

**Analog:** existing shapes extended additively (api.ts:2264-2285 `FileSetView`; 2348-2364 `patchFileSet`). Add doc-commented `selectedPaths?: string[]` to both + `FileSetView.selectedPaths` served from the view (service.go:8799-8819 JSON tags; additive `json:"selectedPaths,omitempty"`). TS mirrors Go JSON field-for-field by hand (house rule).

## Shared Patterns

### Owned setter, never folded into the general update
**Source:** `internal/store/filesets.go:139-157` (rationale comment verbatim in file); same split in `targets.go`.
**Apply to:** `SetFileSetSelectedPaths` — a form that does not know about selection must never clear one by omitting it; `UpdateFileSet`'s column list stays untouched.

### Additive PATCH pointer field + boundary validation
**Source:** `handlers.go:4498-4507` (pointer body) + `handlers.go:1118-1122` (DisallowUnknownFields rationale) + Phase 3 `excludeCaches` cap precedent.
**Apply to:** the new `selectedPaths` field — per-entry containment, 64 cap, atomic rejection, D-06 refusal via `codedFailEnvelope(err, "empty-selection")`.

### Compile at a single site; excludes at the single enforcement site
**Source:** `service.go:4256` (`excludedBranches` on the argv tail) + `internal/api/selection.go` (`NormalizeSelection` :131 `includesOnly`, :185 `excludedBranches`, :239 `mapRestorePaths` — all READ-ONLY reuse).
**Apply to:** `BackupFileSet` compile; scheduler/batch/everything inherit for free. Never a second files-domain normalization or exclude-encoding.

### Serialized one-deep save queue
**Source:** `Containers.tsx:816-851` (refs), `:944-1099` (drain), `:885-889` (`applyMirror`).
**Apply to:** Files-page `selectedPaths` saves; last-toggle-wins, revert from live mirror, no concurrent PATCHes.

### Legacy byte-identity pinning
**Source:** `service_test.go:293-295` (`TestBackupFileSet` already asserts `lastPaths == [srcDir]`).
**Apply to:** every new test must leave this pin green untouched — NULL column ⇒ argv unchanged for sets never edited via the tree.

### Error handling / argv discipline (house-wide)
Scrub errors (paths → `[path]` first, ≤300-char restic reasons, `truncateErr` for runs); wrap with domain prefixes (`"files backup: "`, `"zvol restore: "` style); positionals after `--` via typed builders only; `paths.Resolve`/`paths.Within` for all containment; no shell anywhere.

## No Analog Found

None — every modified file has an exact in-repo precedent (parity phase). New test files (`Files.tree.dom.test.tsx`, selectedPaths PATCH tests) follow existing co-located `*.dom.test.tsx` / `service_test.go` patterns; no framework install.

## Metadata

**Analog search scope:** `internal/store`, `internal/api`, `internal/backup`, `web/src/pages`, `web/src/components`, `web/src/lib`
**Files read this session:** `files_orchestrator.go` (full), `filesets.go` (50-188), `handlers.go` (4448-4577), `migrate.go` (1260-1329), `Containers.tsx` (810-909), `SelectionTree.tsx` (60-169); all other excerpts verified verbatim in 04-RESEARCH.md with file:line citations
**Pattern extraction date:** 2026-09-10
