# Phase 1: Selection Engine & Restore Safety - Pattern Map

**Mapped:** 2026-09-09
**Files analyzed:** 9 (3 new, 6 modified/extended)
**Analogs found:** 9 / 9

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/api/selection.go` (NEW) | utility (pure path semantics) | transform | `internal/paths/paths.go` (pure helpers, POSIX note, sentinel errors) | role-match |
| `internal/api/selection_test.go` (NEW) | test | unit/table | `internal/restic/restic_args_test.go` (table discipline) + `internal/paths` test style | role-match |
| `internal/api/handlers.go` — `handleBrowse` extension | handler | request-response | `handleBrowse` itself (handlers.go:4143-4205) + `handleContainerMounts` envelope (1283-1307) | exact |
| `internal/api/handlers.go` — `handlePatchContainer` guard | handler | request-response | `handlePatchContainer` itself (handlers.go:1090-1161) + `decodeBody` (365-383) | exact |
| `internal/api/service.go` — `SetBackupPaths` + readers | service | CRUD | same functions (service.go:3771-3792, 3826-3832, 3864-3888, 3722-3764) | exact |
| `internal/api/service.go` — `prepareRestoreForTarget` | service | batch/transform | same function (service.go:5332-5462) + plan struct (5225-5235) | exact |
| `internal/store/targets.go` — `SetBackupPaths` | model | CRUD | same function (targets.go:297-317) — unchanged; normalization happens UPSTREAM in service | exact |
| `internal/restic/restic_args_test.go` (extend) | test | unit/table | same file (TestBackupArgs family, 294-349) | exact |
| `internal/restic/restic_positionals_contract_test.go` (NEW) | test (real binary) | batch | `internal/restic/restic_roundtrip_test.go:16-40` (`exec.LookPath` skip pattern) | exact |

## Pattern Assignments

### `internal/api/selection.go` (utility, transform) — NEW

**Analog:** `internal/paths/paths.go` (whole file — the house style for pure path helpers)

**Style to copy** (paths.go:1-15, 31-51): package doc sentence, sentinel errors as exported vars, "why" paragraphs on every non-obvious choice, POSIX-only note making Windows builds safe:

```go
// Package paths provides in-app path containment under the host mount root.
// ...
// ErrTraversal is returned when a sub path would escape the root.
var ErrTraversal = errors.New("paths: sub path escapes the root (traversal)")
```

**The strict-ancestor test to reuse verbatim** (paths.go:44-48) — this IS the PruneMaximal strictness primitive (`/host/user` must not match `/host/user2/foo`):

```go
// Append "/" to cleanRoot so /host/user never matches /host/user2/foo.
prefix := cleanRoot + "/"
if !strings.HasPrefix(cleaned, prefix) {
    return "", ErrTraversal
}
```

**POSIX note to mirror** (paths.go:31-32) — justifies using `path`/`strings` and not `filepath`, which is what makes `!`-prefix tests safe on the Windows dev box:

```go
// Paths here are always Linux paths (container-internal), so the path package
// (always slash-separated) is correct regardless of the build OS.
```

**Planner note:** `selection.go` gets NO receiver, NO store access, NO cfg — pure funcs on `[]string` per CONTEXT encoding Q2. Export surface per RESEARCH R3: `ExclusionPrefix = "!"`, `SplitExclusion(entry) (bare string, excluded bool)`, `PruneMaximal(paths []string) []string`, `NormalizeSelection(entries []string) []string`.

---

### `internal/api/selection_test.go` (test, unit/table) — NEW

**Analog:** `internal/restic/restic_args_test.go:294-332` — exact-match table style: build input, single `reflect.DeepEqual` against a fully-literal `want`, comment citing the issue number:

```go
func TestBackupArgs(t *testing.T) {
	got := BackupArgs("/repo", []string{"-weird", "/p"}, []string{"container:plex"}, Mode{Encrypted: true})
	want := []string{"-r", "/repo", "--retry-lock", "5m", "backup", "--json", "--host", "bombvault", "--tag", "container:plex", "--", "-weird", "/p"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
```

Table cases required by CONTEXT (encoding Q3/Q4, SELECT-04): include elides include-descendant; exclusion elides exclusion-descendant; include+exclude coexist (`/x` + `!/x/y` both survive); orphan exclusion preserved; pure include list passes through unchanged; canonical order (sorted includes then sorted excludes, `!` re-attached); empty input → empty output; bare `"!"` rejected by caller. Test both root configs (split-root and identity-root) where translation is involved — mirror `paths.go:22-29`'s dual-root doc framing.

---

### `internal/api/handlers.go` — `handleBrowse` extension (handler, request-response)

**Analog:** itself, `handleBrowse` (handlers.go:4143-4205). The edit is in-place; existing branches are CONTRACT:

**Traversal rejection — keep byte-identical, NO `status` field** (handlers.go:4156-4162):

```go
// paths.Resolve returns ErrTraversal or ErrAbsoluteSub — neither
// leaks host paths; report a generic message for defense-in-depth.
writeJSON(w, http.StatusOK, map[string]any{
	"ok":    false,
	"error": "invalid path: must be a relative subpath under the mount root",
})
```

**Read failure — keep the generic string, ADD `status`** (handlers.go:4168-4173). The classification comes from `errors.Is` on the `*fs.PathError` (`fs.ErrPermission` → `"restricted"`, `fs.ErrNotExist` → `"missing"`, else `"error"`). Note the `//nolint:gosec // G706` justification-comment style — copy it for any new log line:

```go
log.Printf("api: browse: ReadDir %q: %v", abs, err) //nolint:gosec // G706: abs is always either cfg.HostMountRoot or a Resolve-validated child path; no raw user bytes reach the log formatter
writeJSON(w, http.StatusOK, map[string]any{
	"ok":    false,
	"error": "could not read directory",
})
```

**Success payload — add `status:"ok"` + `truncated:bool`, never `hasChildren`** (handlers.go:4199-4204):

```go
writeJSON(w, http.StatusOK, map[string]any{
	"ok":   true,
	"root": h.cfg.HostMountRoot,
	"path": subpath,
	"dirs": dirs,
})
```

**os.Root seam** (replaces `os.ReadDir(abs)` at 4166): `root, err := os.OpenRoot(h.cfg.HostMountRoot)` + `defer root.Close()`; `f, err := root.Open(rel)` (empty subpath → `"."`, stdlib contract) → `f.ReadDir(-1)`. `paths.Resolve` STAYS as the first reject so the traversal response is untouched. New query param: `hidden := r.URL.Query().Get("hidden")` — dot-dir skip at 4182-4184 becomes conditional. New constant near `browseDirEntry` (handlers.go:2935): `const maxBrowseEntries = 500`; order = ReadDir(-1) → filter → sort (4197, keep) → truncate → `truncated = len(kept) > maxBrowseEntries`.

---

### `internal/api/handlers.go` — `handlePatchContainer` guard (handler, request-response)

**Analog:** itself (handlers.go:1090-1161).

**Body struct — pointer fields so absent fields never reset** (handlers.go:1095-1106); add `SelectionSource *string \`json:"selectionSource"\`` — MUST be added because `decodeBody` rejects unknown fields:

```go
// Pointers so a hooks-only PATCH doesn't reset the schedule flag (and vice
// versa) — only the fields actually sent are applied.
```

**The guard call site** (handlers.go:1123-1128) — `SetBackupPaths` errors already flow through `failEnvelope`; the coded envelope is the only new shape here:

```go
if body.BackupPaths != nil {
	if err := h.svc.SetBackupPaths(r.Context(), name, *body.BackupPaths); err != nil {
		writeJSON(w, http.StatusOK, failEnvelope(err))
		return
	}
}
```

**Coded envelope for the empty-selection refusal** — new small helper next to `failEnvelope` (handlers.go:56-61), same scrub discipline:

```go
// failEnvelope returns a graceful failure envelope. The error is scrubbed so no
// repo path or secret leaks to the client ...
func failEnvelope(err error) map[string]any {
	return map[string]any{"ok": false, "error": scrubError(err)}
}
// NEW shape: {"ok":false, "error":<scrubbed>, "code":"empty-selection"} — the
// machine code lets the Phase 3 UI route guidance (CONTEXT INTEG-04 Q2).
```

**Boundary validation — already in place, do not duplicate** (handlers.go:365-383): `crossOriginGuard` → `MaxBytesReader` 1 MiB → `DisallowUnknownFields`. `resourceNameRe` (585) guards `{name}` only and does NOT touch `backupPaths` elements — the `!` prefix must not be rejected at any handler gate; per-element validation lives in `service.SetBackupPaths`.

---

### `internal/api/service.go` — selection setter + readers (service, CRUD)

**Analog:** the functions being edited; the edits are insertions at verified anchors.

**`SetBackupPaths` — prefix parse goes BEFORE translation** (service.go:3771-3792). The existing loop shape is kept; insert `SplitExclusion` first, re-attach after:

```go
func (s *Service) SetBackupPaths(_ context.Context, name string, hostPaths []string) error {
	var cps []string
	seen := map[string]bool{}
	for _, hp := range hostPaths {
		hp = strings.TrimSpace(hp)
		if hp == "" {
			continue
		}
		// toContainerPath path.Cleans the input first (resolving any ".."), then
		// requires the host-source-root prefix, so its result is guaranteed to sit
		// under the mount root — no separate containment check needed.
		cp, ok := s.toContainerPath(hp)
		if !ok {
			return fmt.Errorf("path %q is not under the host mount and can't be backed up", hp)
		}
		...
	}
	return s.store.SetBackupPaths(name, cps)
}
```

Why order matters (RESEARCH Pitfall 1): `toContainerPath` (service.go:1137-1148) does `strings.TrimPrefix(p, srcRoot+"/")` — a raw `!/mnt/...` fails it and the WHOLE save is rejected. Parse `!` → translate bare → `cp = ExclusionPrefix + cp`. Then `NormalizeSelection(cps)` before the store call. The empty-selection guard hook (tree-source `[]` over prior non-empty) inserts here per L10.

**`configuredBackupPaths` — keep the explicit test on the RAW list, return includes-only** (service.go:3826-3832); preserve its #175 "why" paragraph verbatim:

```go
func (s *Service) configuredBackupPaths(name string, in model.Inspect) []string {
	chosen := s.resolveAppdataPaths(name, in)
	if existing, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(existing.SelectedPaths) > 0 {
		chosen = existing.SelectedPaths
	}
	return chosen
}
```

**`storedDataIsGone` — classify explicit-none as NOT gone** (service.go:3874-3888). Current tail measures `stored` raw; a `!`-prefixed list stats-fails everything → backup refused ("not reachable") — the exact inversion Pitfall 2. Insert: non-empty raw list whose includes are empty → `return false`. Keep the full #181 doc paragraph (3842-3863) — it documents the three-state decision this edit extends.

**`ContainerMounts` — split classes before the custom loop** (service.go:3722-3764). `effective` split via `SplitExclusion`: includes drive `selSet`/`matched`/the custom loop (3757-3762); exclusions become a new return/render as the `excluded` response field. Note the existing nolint-with-reason style at 3759.

**`handleContainerMounts` additive field** (handlers.go:1293-1306) — nil-guard pattern to copy for the new `excluded` slice:

```go
if custom == nil {
	custom = []CustomPath{}
}
writeJSON(w, http.StatusOK, okEnvelope(map[string]any{
	"mounts":         mounts,
	"custom":         custom,
	"hostMountRoot":  h.cfg.HostMountRoot,
	"hostSourceRoot": h.cfg.HostSourceRoot,
}))
```

---

### `internal/api/service.go` — restore hardening (service, batch)

**Analog:** `prepareRestoreForTarget` (service.go:5332-5462) + plan struct (5225-5235).

**Plan struct — add `skippedPaths`** (service.go:5225-5235, field-comment style):

```go
type containerRestorePlan struct {
	repo         string
	mode         restic.Mode
	targetID     string
	snapshotID   string
	recreateOnly bool
	appdataPaths []string            // restored per-path back to origin (nil = recreate-only)
	restoreDirs  []backup.RestoreDir // cross-pool remap: Subtree->Target; empty = in-place via appdataPaths
	inspect      model.Inspect
	templateXML  string
}
```

**Snapshot list is already in hand** — mapping inserts between :5344/:5355 and the remap block at :5392:

```go
snaps, snapErr := s.snapshotsForTag(ctx, ref.repo, ref.mode, "container:"+name)
...
case len(snaps) > 0:
	snapshotID = snaps[len(snaps)-1].ID
```

The chosen snapshot's `Paths []string` (`restic.Snapshot`, `internal/restic/restic.go:165-172`) is the mapping source. `paths.Within` re-validation loop (5375-5380) stays BEFORE the mapping (defense-in-depth on stored paths). Per RESEARCH R5: clause 1 descendant/direct (`q` == or strictly below stored `p`), clause 2 longest-prefix ancestor (`strings.HasPrefix(p, q+"/")` — the `paths.go:44-48` primitive), clause 3 skip with scrubbed log + `skippedPaths`; empty mapped list over non-empty stored → `return containerRestorePlan{}, errors.New("nothing to restore for this item from this snapshot")` — this fires synchronously in prepare, BEFORE `executeRestore`'s Stop/Remove (`internal/backup/orchestrator.go:749-754`), which is the whole safety property.

**Mapped list must be snapshot-path form** — `internal/restic/restic.go:485-500` doc: "callers take it from the SNAPSHOT's Paths (not a recomputed value)"; selector goes after `--`.

`RestoreDeps` (`internal/backup/orchestrator.go`) gains additive `SkippedPaths []string` (DI deps-struct pattern; never a global).

---

### `internal/store/targets.go` — `SetBackupPaths` (model, CRUD)

**Analog:** itself (targets.go:293-317). NO semantic change — the `!` prefix is opaque string data in `selected_paths` JSON (SELECT-02, zero migration). Keep the doc comment and the nil→`[]string{}` guard:

```go
// SetBackupPaths sets the explicit backup-folder selection (container-translated
// paths) for a container, creating the target row if it does not exist yet. An
// empty slice clears the selection so backups fall back to automatic appdata
// detection. Owned by this setter; never reset by UpsertTarget.
func (r *Repo) SetBackupPaths(containerName string, selected []string) error {
	if selected == nil {
		selected = []string{}
	}
	...
```

The only permitted edit is a doc-comment sentence noting entries may carry the `!` exclusion prefix (semantics owned by `internal/api/selection.go`, not the store). `UpsertTarget` (126-131) never resets `selected_paths` — leave untouched.

---

### `internal/restic/restic_args_test.go` (extend) and new contract test

**Analog (argv table):** `restic_args_test.go:294-332` (shown above). Extend only if multi-path/same-basename positional cases are missing; never change existing `want` slices.

**Analog (real binary):** `restic_roundtrip_test.go:16-40` — the exact skip-and-setup pattern for the L12 spot-checks in `restic_positionals_contract_test.go`:

```go
// TestRoundtrip exercises a full init → backup → restore cycle using the real
// restic binary.  It is skipped when restic is not on PATH (local dev) and
// runs in CI where restic is installed by the workflow.
func TestRoundtrip(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	...
	r := restic.Restic{Bin: "restic"}
	m := restic.Mode{Encrypted: false}
	if err := r.Init(ctx, repo, m); err != nil { ... }
```

**Argv shape the contract tests must not disturb** (`internal/restic/restic.go:361-384`): tags via `--tag`, excludes via `--exclude`, positionals strictly after `--`. NO selection-derived `--exclude` is ever added (L1/L14).

**Test-harness analogs (api package):**
- `newTestRouter` / `newTestRouterSvcDir` (`handlers_test.go:29-65`) — identity-root `HostMountRoot: dir`, seeds `appdata/plex`; use for PATCH-guard and restore-hardening tests.
- `newBrowseRouter(t, mountRoot)` (`handlers_test.go:1521-1531`) — purpose-built browse harness; use for cap/status/hidden/symlink tests.
- `doJSON` (`handlers_test.go:82-98`) — request + envelope decode; assert `m["ok"]`, never status codes (Pitfall: collapsing the 200 envelope).
- `fakeResticEngine` (`service_test.go:3794-3834`) — `lastPaths` (backup positionals), `restored`, `lastExcludes`, seedable `snaps []restic.Snapshot` — everything RESTORE-01 and criterion-2 assertions need with zero new fakes.
- Existing browse tests (`handlers_test.go:1533-1628`) MUST pass unmodified — new assertions go in new tests only.
- Symlink-escape fixture: guard with an explicit `runtime.GOOS` check + loud message (Windows dev cannot create symlinks; repo POSIX-skip convention).

## Shared Patterns

### HTTP 200 envelope
**Source:** `internal/api/handlers.go:38-61`
**Apply to:** every new/changed response in this phase (`status`, `truncated`, `excluded`, `code:"empty-selection"`)
```go
func writeJSON(w http.ResponseWriter, status int, v any) { ... }        // always 200 for domain outcomes
func okEnvelope(extra map[string]any) map[string]any { ... }            // merge pattern for additive fields
func failEnvelope(err error) map[string]any { return ...scrubError(err) }
```
Tests assert `m["ok"]`, never `w.Code` (except the trivial `== http.StatusOK`).

### Error scrubbing
**Source:** `internal/api/handlers.go:63-131` (`scrubSecrets` — paths→`[path]` FIRST, then credentials; order documented as load-bearing)
**Apply to:** the coded envelope, skip-path records, `status` messages (which carry the KIND, never the path). Any new log line with a path gets `//nolint:gosec // G706` + justification.

### Boundary validation
**Source:** `internal/api/handlers.go:365-383` (`decodeBody`), `:585` (`resourceNameRe`)
**Apply to:** `handlePatchContainer` (new `selectionSource` pointer field mandatory in the struct — DisallowUnknownFields rejects undeclared JSON). Per-element `backupPaths` validation lives in `service.SetBackupPaths` (TrimSpace, split, translate, reject bare `"!"`), not in the handler.

### Pure helper + load-bearing "why" comment
**Source:** `internal/paths/paths.go` (whole file), `service.go:3816-3863` (#175/#181 paragraphs)
**Apply to:** `selection.go`, `mapRestorePaths`, every edit paragraph. A tricky choice without an issue-citing paragraph is off-style (house rule).

### DI via deps structs
**Source:** `internal/backup/orchestrator.go` (`RestoreDeps`), `internal/api/service.go` engine interface
**Apply to:** `RestoreDeps.SkippedPaths` (additive field, no globals); restore hardening happens entirely in the service prepare phase — the orchestrator port `RestorePaths(ctx, repo, snapshotID, paths []string) error` is unchanged.

### Async-test hygiene / real-binary skip
**Source:** `handlers_test.go:70-80` (`waitForBackupDone`), `restic_roundtrip_test.go:19-22`
**Apply to:** any test that triggers a backup goroutine; the two real-restic contract tests (skip locally, prove on CI 0.17.3).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | — | — | Every phase file has a verified in-repo analog; `os.Root` is new API usage but the containment *pattern* (paths.Resolve first-reject) is the analog to layer it onto |

## Metadata

**Analog search scope:** `internal/api/` (handlers.go, service.go, handlers_test.go, service_test.go), `internal/paths/`, `internal/store/`, `internal/restic/`
**Files read for excerpts:** 11
**Pattern extraction date:** 2026-09-09
