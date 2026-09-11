# Phase 3: Selection Trust & Controls - Pattern Map

**Mapped:** 2026-09-10
**Files analyzed:** 14 (all modifications/extensions of existing files; zero brand-new files)
**Analogs found:** 14 / 14

Verified against source this session (not just RESEARCH.md): `internal/store/targets.go`, `internal/restic/restic.go`, `internal/api/handlers.go`, `web/src/pages/Containers.tsx`, `web/src/lib/selectionTree.ts`. Line numbers below are current-branch accurate.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/store/migrate.go` | migration | transform | v53 `target_excludes` migration (migrate.go:496-501) | exact (same table, same JSON-column shape) |
| `internal/store/targets.go` | model | CRUD | `SetExcludes` + the `excludes` column (targets.go:489-512, 115-118, 542-565) | exact |
| `internal/store/targets_test.go` | test | CRUD | `TestSetExcludesRoundTripAndUpsertPreserves` (targets_test.go:59) | exact |
| `internal/api/handlers.go` | controller | request-response | `handlePatchContainer` body.Excludes branch (handlers.go:1108-1171) | exact (same endpoint, same field-addition pattern) |
| `internal/api/service.go` | service | request-response | `SetExcludes` svc (:10102) + `SetBackupPaths` guard (:3868-3873) + Backup deps literal union (:4219-4248) | exact |
| `internal/restic/restic.go` | utility (argv builder) | transform | `BackupArgs` + `Mode.Limits` knob precedent (restic.go:357-384, 89-101) | exact |
| `internal/restic/restic_args_test.go` | test | transform | `TestBackupArgs*` family incl. zero-value absence pin (:294-393) | exact |
| `web/src/lib/selectionTree.ts` | utility | transform | its own `isAtOrUnder`/`isStrictlyUnder`/`splitFlatSet`/`toFlatList` (:68-139) | exact (extend in place) |
| `web/src/lib/selectionTree.test.ts` | test | transform | existing table tests in the same file | exact |
| `web/src/lib/api.ts` | utility (wire types) | request-response | `setBackupPaths` opts pattern (api.ts:891-903), `Container.lastBackup` (:27) | exact |
| `web/src/components/SelectionTree.tsx` | component | CRUD (UI state) | its own root-row rendering; muted sub-label precedent (verified in Phase 2) | exact |
| `web/src/pages/Containers.tsx` | component | request-response (serialized PATCH queue) | `FoldersEditor` queue `attemptSave`/`scheduleSave` (Containers.tsx:845-936) | exact |
| `web/src/pages/Containers.tree.dom.test.tsx` | test | request-response | existing Phase 2 dom harness | exact |
| `web/src/lib/i18n.ts` + `web/src/lib/locales/*.ts` (×40) | config (i18n tables) | transform | Phase 2 key propagation (en+de inline, 40 lazy locales) | exact |

## Pattern Assignments

### `internal/store/targets.go` + `migrate.go` (RESTIC-01 persistence, D-07)

**Analog:** the `excludes` column lifecycle — v53 migration, owned setter, five positional SQL sites.

**Migration template** (migrate.go:496-501 — copy verbatim, bump to version 100, last is v99 at :1302-1305):
```go
{
	// Per-container restic --exclude patterns applied to this container's backup.
	// JSON array; '[]' = none. Owned by SetExcludes (never reset by Upsert).
	version: 53, name: "target_excludes",
	sql: "ALTER TABLE targets ADD COLUMN excludes TEXT NOT NULL DEFAULT '[]';",
},
```
New: `version: 100, name: "target_exclude_caches"`, `DEFAULT '{}'`. Append at end only (NUMBERING HAZARD, migrate.go:50-67). `map[string]bool` marshals with sorted keys — round-trip stable.

**Owned setter template** (targets.go:489-512 — copy structure exactly, including the create-on-missing-row fallback):
```go
func (r *Repo) SetExcludes(containerName string, excludes []string) error {
	if excludes == nil {
		excludes = []string{}
	}
	exJSON, err := json.Marshal(excludes)
	if err != nil {
		return fmt.Errorf("SetExcludes marshal: %w", err)
	}
	res, err := r.db.Exec(
		`UPDATE targets SET excludes = ? WHERE container_name = ?`,
		string(exJSON), containerName)
	if err != nil {
		return fmt.Errorf("SetExcludes: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := r.UpsertTarget(Target{ContainerName: containerName, Excludes: excludes}); err != nil {
			return fmt.Errorf("SetExcludes create target: %w", err)
		}
	}
	return nil
}
```

**FIVE positional SQL sites** must change together (verified): `Target` struct field; INSERT columns/values in `UpsertTarget` (:127-134 — add to the INSERT only, NEVER to the ON CONFLICT SET, per the ownership comment :120-125); the three SELECT lists (:146, :154, :196); `scanTarget` (:542-565). Error wrapping: `fmt.Errorf("MethodName: %w", err)`.

**Test template:** `TestSetExcludesRoundTripAndUpsertPreserves` (targets_test.go:59).

---

### `internal/api/handlers.go` (PATCH field + mounts response, D-07)

**Analog:** `body.Excludes` in `handlePatchContainer` (handlers.go:1108-1171 — verified this session).

Body struct addition (pointer = nil-means-untouched, mandatory under `DisallowUnknownFields`):
```go
var body struct {
	...
	Excludes          *[]string `json:"excludes"`
	// ExcludeCaches: per-mount-root CACHEDIR.TAG toggle (RESTIC-01). Map root→bool;
	// nil = untouched. Only the boolean UNION ever reaches argv (D-06).
	ExcludeCaches     map[string]bool `json:"excludeCaches"`
	...
}
```
Handler branch — copy the `body.Excludes` branch shape (handlers.go:1166-1171): nil-check → `h.svc.SetExcludeCaches(ctx, name, map)` → `writeJSON(w, http.StatusOK, failEnvelope(err))` on failure. Errors stay HTTP 200 `{ok:false}`.

**Mounts response** (handlers.go:1337-1343): add `"excludeCaches": <map or {} if unset>` to the existing `map[string]any`.

**Boundary validation:** map keys are host paths — apply the same containment discipline as `SetBackupPaths` (service.go:3839-3842 `toContainerPath` reject-if-not-under-mount precedent) plus a small entry-count cap.

---

### `internal/api/service.go` (svc method + union threading, D-06/D-07)

**Analog A — svc setter:** `SetExcludes` at service.go:10102 (thin store delegate).

**Analog B — union at the Backup deps literal:** `excludedBranches(tg.SelectedPaths)` computed at service.go:4219-4248 — the new boolean union (`any root true`) computes at the same site from `tg.ExcludeCaches` re-read via `GetTargetByContainer`.

**Analog C — Mode threading (recommended Option A):** the `Mode.Limits` why-comment (restic.go:89-101) is the documented precedent: per-backup knobs ride `restic.Mode` because it is already threaded through every adapter call site; a new parameter would touch `backup.Restic` + every fake. `service.Backup` already builds a fresh `mode := s.primaryModeFor(...)` (service.go:4116) — set `mode.ExcludeCaches = true` on that copy. Zero changes to `internal/backup`.

**The guard the reset must pass (INFORMATIONAL — unchanged this phase)** (service.go:3868-3873):
```go
if selectionSource == "tree" && len(normalized) == 0 {
	if prior, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(prior.SelectedPaths) > 0 {
		return errEmptySelection
	}
}
return s.store.SetBackupPaths(name, normalized)
```
A `[]` save with no `selectionSource` falls through to auto-detection — that fall-through IS the D-05 reset. No backend change.

---

### `internal/restic/restic.go` + `restic_args_test.go` (argv, D-06)

**Analog:** `BackupArgs` (restic.go:357-384, verified verbatim this session). `--exclude-caches` slots between the `--tag` loop (:375-377) and the `--exclude` loop (:378-380) — after the `backup` verb (:366), before `--` (:381). Never left of `"backup"` (Pitfall 4).

```go
for _, tag := range tags {
	args = append(args, "--tag", tag)
}
for _, ex := range excludes {
	args = append(args, "--exclude", ex)
}
args = append(args, "--")
```
Add: `if m.ExcludeCaches { args = append(args, "--exclude-caches") }` with a why-comment citing RESTIC-01/D-06 (constant flag, no user input reaches argv; item-level union of per-root toggles).

**Tests:** extend the `TestBackupArgs*` family (:294-393): (1) presence pin byte-exact at the chosen slot; (2) zero-value absence pin — byte-identical argv when unset, copying the `TestBackupArgsLimits` "zero Limits" precedent (:350-356). `BackupStdinArgs` untouched.

---

### `web/src/lib/selectionTree.ts` (pure derivations, D-01/D-03/D-04)

**Analog:** its own helpers (verified :68-139). Everything derives from `(includes, exclusions)` via existing segment-aligned predicates — write NO new prefix-matching code:

```typescript
// count  = [...includes].filter((p) => isAtOrUnder(p, root)).length
// list   = [...exclusions].filter((p) => isStrictlyUnder(p, root)).map((p) => relative(p, root))
```
New pure exports (e.g. `rootIncludeCount(root, includes)`, `rootExclusions(root, exclusions)`) must agree with `toFlatList` (:129-139) membership — the count mirrors what argv will carry (D-01: stored/effective flat set, NOT screen-checked nodes, NOT existence-filtered — Pitfall 5). Table-test in `selectionTree.test.ts`. A fully-deselected root = `rootIncludeCount === 0` → render unchecked + `rootExclusions().length` counter (D-04).

---

### `web/src/pages/Containers.tsx` (queue source, reset, narrowing note — D-02/D-05)

**Analog:** the FoldersEditor one-deep queue (verified :845-936).

**The exact line to parameterize** (Containers.tsx:851):
```typescript
const r = await setBackupPaths(name, toFlatList(live.inc, live.exc), { selectionSource: "tree" });
```
Extend `SaveDesc` with the initiating mutation's source; drain uses the latest desc's source. Toggle descs keep `"tree"` (guard live); the reset desc omits the field — `setBackupPaths(name, [])` with no opts already sends exactly `{backupPaths: []}` (api.ts:891-903 includes `selectionSource` only when truthy), which the Phase 1 guard passes. Never send the reset outside the queue (anti-pattern: concurrent PATCHes, T-02-08).

**Narrowing note (D-02):** event-driven — on `r.ok`, compare attempted (live) include count to a `lastSavedCount` ref (init from load-time mirror size, update on every success) — NOT `desc.pre` vs `desc.sent` (queue collapses bursts, Pitfall 3). Gate on `container.lastBackup != null` (wire type `lastBackup: number | null`, api.ts:27; served at handlers.go:531-534). Transient, this-session only. The D-04 zero-include block (onToggle :921-925) gets its message enriched to reference the reset control.

**CACHEDIR toggle PATCH serialization (Pitfall 6, planner choice):** either generalize the queue to all container PATCHes from FoldersEditor, or give the toggle its own one-deep serialized path — never two overlapping fetches to `/api/containers/{name}`.

---

### `web/src/components/SelectionTree.tsx` + `web/src/lib/api.ts` (UI surface)

**Analog:** existing root rows + `FolderBrowser`/`ExcludesEditor` styling precedents; shared controls `Toggle.tsx`, `Badge`, `Button`, `InfoBubble`/`IconTipButton` (`glim-*` classes, `carbon-*`/`status*` tokens — status colors on badges only, never controls). Reset confirmation via `useConfirm` (names both consequences). Wire types added additively in `api.ts` mirroring the Go JSON shape field-for-field with doc comments.

**i18n (42 locales):** every new string via `t()`; add to en+de inline in `i18n.ts`, then propagate to all 40 `locales/*.ts` in the SAME task; ~5-7 keys + enrichment of `folders.emptySelectionBlocked`; no em dashes; parity/quality/orphans tests gate it.

## Shared Patterns

### Error envelope + scrubbing
**Source:** `internal/api/handlers.go` (`failEnvelope`/`codedFailEnvelope` :39+, empty-selection code at :1152-1154)
**Apply to:** the new PATCH field branch — plain `failEnvelope`; HTTP 200 `{ok:false}`; server text shown verbatim in the SPA toast (`push(r.error ?? ...)` pattern, Containers.tsx:859).

### Store error wrapping
**Source:** `internal/store/targets.go` — every method wraps with its own name: `fmt.Errorf("SetExcludes: %w", err)`.
**Apply to:** `SetExcludeCaches` and scan/unmarshal sites.

### Boundary validation
**Source:** `decodeBody` (1 MiB, JSON-only, DisallowUnknownFields) + `toContainerPath` containment (service.go:3839-3842).
**Apply to:** the `excludeCaches` map keys (paths) + entry-count cap; declare the field in the body struct in the same change as the client (Pitfall 8).

### Serialized PATCH queue
**Source:** `scheduleSave`/`attemptSave` (Containers.tsx:761-936).
**Apply to:** reset (D-05) and the CACHEDIR toggle PATCH (Pitfall 6) — one serialized stream per editor, never concurrent.

### Build gates
**Apply to:** all tasks — `web/dist` rebuilt + committed after any `web/` change; `go build/vet/gofmt/test` locally; golangci-lint + just land in CI (not on this Windows box's PATH).

## No Analog Found

None — every file in this phase extends an existing, in-repo pattern. The genuinely new logic (per-root pure helpers, narrowing-note trigger, per-attempt queue source) is fully specified in RESEARCH.md Patterns 1-3 with table-testable shapes.

## Metadata

**Analog search scope:** `internal/{store,api,restic,backup}`, `web/src/{lib,components,pages}` — all named in 03-CONTEXT.md canonical_refs and verified.
**Files scanned:** 11 source/analog files (5 re-verified against source this session; remainder trusted from RESEARCH.md's same-day verbatim reads).
**Pattern extraction date:** 2026-09-10
