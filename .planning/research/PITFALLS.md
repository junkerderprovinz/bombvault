# Pitfalls Research

**Domain:** Tree-based sub-folder backup selection (lazy collapsible tree, per-node checkboxes, mixed-state parents) for an existing Go + restic backup tool
**Researched:** 2026-09-09
**Confidence:** HIGH for code-grounded pitfalls (file:line cited); MEDIUM for restic/Go semantics (official docs via Context7); LOW for ARIA/UX pattern details (WAI-ARIA APG via web reader)

**Phase vocabulary used below** (roadmap may rename, keep the mapping):
- **P1 — Browse backend:** extend `GET /api/browse` (lazy listing, hardening)
- **P2 — Selection semantics:** how tree state maps to/from the flat `backupPaths` set
- **P3 — Tree UI:** the hand-rolled React tree component (container panel first)
- **P4 — Backup integration:** what restic is actually invoked with
- **P5 — Restore & coverage:** snapshot-structure expectations, restore path mapping

---

## Critical Pitfalls

### Pitfall 1: "Uncheck everything" silently resurrects auto-detection (empty-selection semantics inversion)

**What goes wrong:**
`configuredBackupPaths` (service.go:3826) treats an empty `SelectedPaths` as "no explicit selection" and falls back to `resolveAppdataPaths` auto-detection. Today that is fine: the only way to get an empty list is the explicit "clear selection" affordance, and users think of it as "go back to automatic" (#181's whole point). Under a tree, a user can *uncheck their way* to an empty set node-by-node — and the system will answer "back up nothing" with "back up all auto-detected appdata again". The user said no; the backup gets bigger. Worse, the empty-backup guard (#181) will *not* catch it when detection finds something, because the selection is no longer empty from the backup's point of view.

**Why it happens:**
The flat `backupPaths` list has exactly three expressible states — absent (auto), non-empty (explicit), and empty (auto, by the fallback above) — but a tree has four intents: auto / all / some / none. "None" and "auto" collapse to the same stored value. The persistence-format constraint (no `backupPaths` schema change) forbids adding a fourth state, so the collision must be resolved in the UI contract instead.

**How to avoid:**
- The tree UI refuses to *save* an all-unchecked tree for a container that auto-detection would cover: disable unchecking the last node with an explanation ("unchecking everything reverts this container to automatic appdata detection — deselect it from backups instead"), or route the intent to the existing explicit disable/deselect flow.
- If a "none" state is genuinely needed, decide it explicitly in P2 (e.g. store the mount root itself as a single path and exclude its children via the existing per-container excludes — NOT a persistence change), and document it; never let it fall out of list arithmetic by accident.
- Add a test at the `PATCH /api/containers/{name}` boundary: a tree save that produces `[]` must be either rejected with a `{ok:false}` message or provably intentional.

**Warning signs:**
A design doc or PR that says "unchecking all nodes just clears `backupPaths`" without mentioning auto-detection; QA note like "after unchecking everything the backup still runs and is bigger than expected."

**Phase to address:** P2 (semantics), with the guard rail enforced again in P3 (UI affordance).

---

### Pitfall 2: Decomposing a checked parent into the flat list permanently drops future children

**What goes wrong:**
Today `backupPaths = ["/host/user/appdata/plex"]` means "this mount and *everything ever created under it*" — restic walks the whole subtree each run. If the tree implements "uncheck `transcoding`" by expanding the parent into its children and storing `backupPaths = [child1, child2, ... childN]` (the only thing a flat selected-path list can express), then any folder created *after* the save — `cache`, `logs`, next year's plugin dir — is silently not backed up. Backups stay green, sizes look plausible, and the gap is discovered during a restore, which is the worst possible time.

**Why it happens:**
A selected-path list is an allowlist. "All except X" is a denylist. The constraint says the tree is "a new view over the same data" — but the two intents are not expressible in the same data. Decomposition is the naive mapping and it changes the *meaning* of an existing row without any migration.

**How to avoid:**
- Decide this in P2 before any UI exists, and write the decision into PROJECT.md's Key Decisions. The only decomposition-free encoding available under the "no persistence change" constraint is: keep the parent path in `backupPaths` when the user wants "the mount minus junk", and express the unchecked subtrees through the existing per-container restic excludes (which already have persistence, translation, and a preview editor). Tree node checked-at-parent → parent stays the stored path; unchecked subtree → one exclude line. Full sub-folder selection ("only config and plugins") is the case that must decompose — scope the decomposition behavior to it consciously.
- If decomposition is used anywhere, pair it with a backup-time coverage diff (list the stored paths' parents, warn when a sibling exists that is neither selected nor excluded) surfaced in the run report or the item card. Absence is invisible; make it visible once per run, not per restore.

**Warning signs:**
A `PATCH` payload where one mount root is replaced by 40 leaf paths; a user report much later of "new folder wasn't in the snapshot" with no error anywhere; tree code that calls a "list all descendants" endpoint.

**Phase to address:** P2 (decision + coverage warning), P4 (implement the coverage diff).

---

### Pitfall 3: restic cannot re-include inside an excluded directory — and the two workarounds produce *different snapshot layouts*

**What goes wrong:**
Official restic docs (040_backup.rst) are explicit: gitignore-style `!` negation exists, but "if a directory is excluded, it is impossible to include individual files contained within that directory," and there is no include filter for backup sources. So a mixed-depth selection ("all of `plex`, but only `config` and `plugins` under it, `transcoding` gone") cannot be expressed as `backup <parent> --exclude ...` when the kept and dropped branches interleave. The escape hatch is passing multiple path arguments — which changes what the snapshot looks like:

- `restic backup /host/user/appdata/plex` → snapshot contains `plex/config`, `plex/plugins`, ... (parent anchored).
- `restic backup /host/user/appdata/plex/config /host/user/appdata/plex/plugins` → snapshot root contains `config/` and `plugins/` *as siblings*; `plex` vanishes from the layout.

Consequences of switching an item between those shapes: restic identifies unmodified files by their in-snapshot path (docs, 040_backup.rst "Absolute and relative paths"), so the path change resets change detection and the parent-snapshot lineage — one full re-walk of the data (dedupe keeps stored bytes flat, but read/CPU cost happens once; the repo already documented exactly this class of surprise in issue #189 for compose dirs). Restore flows that assume the container's data lives under `<mount basename>/` in every snapshot will mis-map new snapshots, and `snapshots --path` filters (`HasPaths`) change behavior too. Untested edge: two selected paths with the *same basename* from different parents (`a/config`, `b/config`) both landing at the snapshot root.

**Why it happens:**
The feature is framed as "tree selection", and the natural mental model is "pass the selected leaves to restic". The layout/changelog consequences only appear when old and new snapshots of the *same item* coexist.

**How to avoid:**
- Pick **one** canonical invocation strategy per selection shape and document the snapshot layout it produces:
  - "Mount minus junk" → parent path + derived excludes (layout unchanged, future children covered per Pitfall 2's denylist framing).
  - True leaf selection → explicit path args, accepted as a layout change, with restore mapping that is structure-agnostic (see Pitfall 11).
- Keep `BackupArgs` argv construction in `internal/restic/restic_args_test.go` discipline: add cases for parent+excludes, multi-path, same-basename leaves, and a leaf nested under an excluded branch (proves the no-reinclude rule is respected rather than discovered).
- The container restore path must read `snapshot.paths` (it is in the snapshot JSON) and map onto mounts by longest-prefix against the *current* mount set — never by "first path component equals the mount name".

**Warning signs:**
Snapshot listing in the restore panel suddenly showing `config/` at the top level for new snapshots of an old item; a snapshot's `paths` array with 30 entries where it used to have 1; "restore this container" restoring half a mount because it keyed off the first path segment.

**Phase to address:** P2 (strategy decision), P4 (argv + tests), P5 (restore mapping).

---

### Pitfall 4: Lexical containment is not symlink containment in the listing endpoint

**What goes wrong:**
`paths.Resolve` (internal/paths/paths.go:33) is purely lexical — it cleans and prefix-checks strings. It cannot see that `appdata/plex` is a symlink to `/etc` or to somewhere outside `HostMountRoot`. `handleBrowse` (handlers.go:4143) resolves the subpath lexically and then `os.ReadDir` follows any symlink *in the path*, so `GET /api/browse?path=appdata/link-to-etc` happily lists a directory outside the mount root. The current code only *skips* symlink entries while listing a legitimate directory (`e.IsDir()` is false for symlink DirEntries), but the direct-address case — which the lazy tree will exercise constantly, and which any LAN client can hit directly in the default trusted-LAN no-session mode — escapes containment. Extending the endpoint (child counts, metadata, deeper listing) multiplies the surface. Additionally the current skip-entries behavior creates a *lie of omission*: a symlinked directory never appears in the tree, yet restic archives symlinks as symlink nodes without following them (official docs; internal/fs/node.go) — so what the tree shows and what a backup captures can already disagree.

**Why it happens:**
String-prefix validation feels like path validation. It answers "could this path escape if it existed as written?" and nothing about what the filesystem will actually resolve.

**How to avoid:**
- Move listing onto `os.Root` (repo is on go 1.25; `os.Root` landed in 1.24): "Methods on Root can only access files and directories beneath a root directory... symbolic links may not reference a location outside the root; symbolic links must not be absolute" (os/root.go). `os.OpenRoot(cfg.HostMountRoot)` once per request (or per service), then `Root.Open/Stat` per subpath; escapes come back as errors instead of listings.
- Keep `paths.Resolve` for the cheap reject (defense in depth), but stop treating it as the containment guarantee.
- Show symlinks in the tree as first-class nodes (link glyph + target tooltip), matching what restic will store — never silently hide them and never follow them in the *listing*.
- Preserve existing handler discipline on the extended endpoint: `{ok:false}` envelope, generic error strings, scrubbed logs (paths → `[path]`), `DisallowUnknownFields`, 1 MiB bodies, argv positionals after `--` if the endpoint ever grows flags.

**Warning signs:**
A test fixture with an escaping symlink under the temp root fails (or worse, is never written); review asks "what happens if a user symlinks appdata/foo → /"; tree screenshots where an entry that exists on disk is missing.

**Phase to address:** P1 — this is the phase's foundation; harden *before* the tree leans on the endpoint, not after.

---

### Pitfall 5: Host-path ↔ container-path translation mistakes across the tree

**What goes wrong:**
There are two roots and three vocabularies: the user speaks host paths (`/mnt/user/appdata/plex`), `backupPaths` and restic speak container paths under `HostMountRoot` (`/host/user/appdata/plex`), and mounts have a third coordinate (container Dest). Translation is symmetric but not idempotent: `toContainerPath` (service.go:1137) and `toHostPath` (service.go:3684) are prefix rewrites that differ between split-root Unraid (`/mnt` → `/host/user`) and identity-root TrueNAS/generic (both `/data`). Fresh tree code is a new place to get this wrong: displaying container paths as if they were host paths; computing tree-relative paths against the wrong root; offering checkboxes for mounts whose Source is not reachable under the mount (`MountInfo.Reachable`, service.go:3704) and storing paths `SetBackupPaths` will then reject; and the batch problem — `SetBackupPaths` rejects the *whole update* when one path is not under the host mount (service.go:3784), so a tree save of 200 leaf paths where one is stale (`/mnt/...` pasted form, or a custom path that drifted) fails everything with an error naming one path the user can't find in the tree.

**Why it happens:**
The strings look interchangeable and tests often run identity-root where the bug is invisible. Split-root Unraid is the deployment where it fires.

**How to avoid:**
- One rule, enforced in review: **host paths only ever exist inside the presentation layer.** Tree nodes carry container paths; `toHostPath` is applied at render time exactly where `MountInfo.Source` already is. Store nothing, compare nothing, send nothing to the API in host form except through the existing `SetBackupPaths` translation (which accepts host paths by contract).
- The tree save path should translate-and-validate *per node* and return a per-path result list (or pre-filter client-side using the `Reachable` flag), so one bad leaf cannot nuke a 200-node selection silently.
- Test the tree contract under *both* root configs — the codebase already does this pattern (`paths_test.go` split-root + identity-root table); mirror it for whatever new helper the tree adds.

**Warning signs:**
Works on TrueNAS/generic, wrong on Unraid (or vice versa); tree shows `/host/user/...` to the user; a save error quoting a path that appears nowhere in the UI.

**Phase to address:** P1 (endpoint contract), P2 (save semantics), P3 (display discipline).

---

### Pitfall 6: Huge directories, slow filesystems, and a listing endpoint with no cap or deadline

**What goes wrong:**
`handleBrowse` does `os.ReadDir(abs)` — all entries, no context, no cap, then sorts. The feature targets exactly the pathological case (Plex appdata). On Unraid, `HostMountRoot` sits on shfs/FUSE spanning the array: a listing can hit every disk, spin them up, and take seconds to minutes; the handler ignores `r.Context()`, so an abandoned expand keeps listing; the client has no timeout contract, so the tree spinner can hang forever; and a directory with tens of thousands of subdirs returns a multi-megabyte JSON the SPA must render. The obvious "improvements" — child counts, sizes, mtime badges — mean per-entry `Stat`, which on shfs is worse than the listing itself.

**Why it happens:**
The endpoint was built for picking one folder; the tree makes expand-by-expand latency and payload size user-facing.

**How to avoid:**
- Cap entries per response (e.g. first N dirs sorted + `"truncated": true` and let the client show "…N more"); never walk recursively server-side — laziness is the architecture.
- Plumb `r.Context()`: run the readdir in a goroutine, `select` on `ctx.Done()` so an aborted request stops waiting (bounded goroutine leak acceptable and consistent with the repo's async-cleanup test discipline — tests must wait for it).
- Client: fetch with timeout + explicit loading/error/empty states per node; cache expanded results per path with revalidation on re-expand (or a short TTL), because the tree will be reopened repeatedly in one session.
- Forbid child-count/size badges unless they come free; if a badge is wanted later, make it a separate opt-in endpoint and expect disk spin-up.

**Warning signs:**
Root expand of a big appdata share takes >2s or times out the dev proxy; memory spike from unsorted entry slices; an aborted UI navigation still visible as CPU in the container.

**Phase to address:** P1 (backend contract: cap, ctx, truncation flag), P3 (client timeouts/cache).

---

### Pitfall 7: Permission errors rendered as "empty" (the false-empty trap)

**What goes wrong:**
BombVault commonly runs as a non-root PUID/PGID. appdata dirs owned by container users (uid 999, the infamous Plex/sonarp ones) are frequently unreadable to it. `os.ReadDir` on such a directory errors — and the current handler collapses every failure into "could not read directory". The dangerous variant is one level down: an *unreadable* directory and an *empty* directory both render as a node with no children. A user who can't distinguish them will happily leave an unreadable (never-backed-up) branch selected, or deselect a healthy one; the first symptom is a restore that's missing everything under that branch, or a restic error mid-run. Also note `os.ReadDir` discards partial results on a mid-read error — a huge directory that fails partway looks identical to a failure at open.

**Why it happens:**
Read failure and empty success both produce "zero children" in the UI data model unless the distinction is carried explicitly.

**How to avoid:**
- The listing response should carry a per-node state: `ok` / `restricted` (EACCES-family) / `missing` (ENOENT) — Go's `*fs.PathError` + `errors.Is(err, fs.ErrPermission|fs.ErrNotExist)` makes this a five-line classifier. Render restricted as a locked node that is *selectable-but-warned* (restic runs under the same uid, so selection is honest but doomed) or disabled with a tooltip — decide once, in P3.
- Never collapse read-error into empty in any new code path (the client tree state machine should have no transition from "error" to "loaded: 0 children" without user action).
- Keep the existing generic error string discipline ("could not read directory", paths scrubbed) — the state flag carries the *kind*, not the path.

**Warning signs:**
QA on a PUID!=0 host shows empty folders that `ls` as root shows populated; restic logs permission errors for paths the UI showed as empty; a test fixture asserting `dirs: []` on a chmod-000 directory.

**Phase to address:** P1 (state flag in the response), P3 (locked-node rendering).

---

### Pitfall 8: Selection state divergence — stale tree vs configured vs effective

**What goes wrong:**
Three realities drift apart: what the tree shows (last listed), what is configured (`backupPaths`, which `SetBackupPaths` deliberately stores *without* existence checks — service_test.go:2173), and what a backup will actually take (`effectiveBackupPaths` = configured minus `onlyExistingPaths`, service.go:3838). Concrete drift the tree amplifies: a mount removed from the container leaves its node rendered checked; a renamed folder leaves a stale invisible entry; an unmounted array fails every `stat` — the exact #175 scenario the code comments warn turned "temporarily unreachable" into a confident wrong answer, and here it renders as an *empty tree*, inviting the user to "fix" a healthy selection (possibly clearing it into the Pitfall 1 trap). None of this errors today; it all silently narrows what gets backed up.

**Why it happens:**
The tree is a cache of the filesystem, and the store is a cache of intent; nothing forces them to reconcile, and the existing flat selector hid the problem by showing less.

**How to avoid:**
- The tree should render from `configuredBackupPaths` semantics (selection = intent, `CustomPath.Exists`-style flags for reality), not from the *effective* set — the distinction exists in code precisely so UI can answer from intent (see the configuredBackupPaths comment).
- Re-expand always refetches (Pitfall 6's cache rule) and stale-selection markers: a selected node whose path fails to list shows a warning glyph ("missing on disk"), never a silent uncheck. Silent repair of the stored list is forbidden — dropping entries is the backup engine's job at run time, not the UI's.
- Mount removed from the container → the node moves to the existing custom-paths section (that's what `CustomPath` is for), keeping its checked state visible.

**Warning signs:**
Users "cleaning up" selections that look empty; support question "why does my container show no folders" that is actually an unmounted array; a tree that stops showing a folder the user knows they selected.

**Phase to address:** P3 (rendering rules), with the intent-vs-reality vocabulary fixed in P2.

---

### Pitfall 9: Live-save races — the HTTP-200 `{ok:false}` envelope defeats naive optimistic UI, and PATCHes arrive out of order

**What goes wrong:**
Folder selection is live-saved per interaction via `PATCH /api/containers/{name}` (handlers.go:1101). Two existing frontend conventions coexist already: boolean toggles do optimistic flip + revert + `.glim-shake` on failure, structural add/remove deliberately do *not* revert (Containers.tsx:711-810 comments). A tree checkbox multiplies toggle frequency and list size, and inherits three concrete failure modes: (1) the envelope convention means *every* response is HTTP 200 — code that checks `res.ok` and treats non-OK as the error path will never see `{ok:false}` failures, so optimistic state sticks while the server kept the old list; (2) two rapid toggles = two in-flight PATCHes carrying *full replacement lists* — the slower stale one lands last and silently erases the newer toggle (classic lost update); (3) a decomposed selection of thousands of paths in one JSON body walks toward the 1 MiB `decodeBody` cap (handlers.go:375) — and DisallowUnknownFields means one shape mistake fails the whole save.

**Why it happens:**
"Save on every checkbox" feels idempotent, and the 200-always envelope is invisible until someone tests failure. The two-convention split exists for good reasons but gives the tree no default to copy.

**How to avoid:**
- Serialize saves per container: a single-flight queue (latest-wins or strict sequence) so two tree clicks can never be two concurrent full-list PATCHes; reconcile state from the response body, not from local assumption.
- A single shared save helper for the tree that (a) checks `envelope.ok`, not HTTP status, (b) reverts the optimistic checkbox on `{ok:false}` with the existing shake/toast language, (c) aborts superseded requests (`AbortController`) instead of letting them land late.
- Decide explicitly which convention a tree checkbox follows (it is a toggle: revert-on-failure), write it in the component comment like the existing ones do.
- Body size: log/monitor PATCH payload sizes in dev; if decomposed lists can plausibly approach the cap, that is a P2 smell (Pitfall 2), not a cap to raise.

**Warning signs:**
Checkbox flips back on its own seconds later; two quick clicks leave one selection in the UI and the other on the server; a test asserting `response.status === 200` proves nothing about success.

**Phase to address:** P3 (frontend), with the save contract pinned in P2.

---

### Pitfall 10: Mixed-state checkboxes derived from lazily-loaded children are wrong by construction

**What goes wrong:**
A parent's tri-state is a function of its children's states — but the tree is lazy, so the client usually *doesn't have* the children. Common shipped bugs: parent shows fully-checked because it was checked directly while collapsed, user expands, unchecks one loaded child, and the sibling set is unknown, so "derived mixed state" is computed over the loaded subset and lies; clicking a mixed parent clears everything when the user expected fill (or cycles); the indeterminate *visual* (HTML `indeterminate` property) is set while `checked`/`aria-checked` report a different state, so tests and screen readers disagree with the pixels; collapse/re-expand recomputes state from a refetched listing and visibly flips.

**Why it happens:**
Deriving state from children and deriving state from the flat selection set are two different models, and teams switch between them mid-implementation. With laziness, only the second is well-defined.

**How to avoid:**
- Define node state *only* from the flat `backupPaths` set, never from loaded siblings: a node is **checked** iff its exact path (or an ancestor) is a stored path; **mixed** iff it is not stored but has a stored descendant (prefix check against the selection set — cheap, no listing); **unchecked** otherwise. Unloaded children *inherit* their parent's state visually; no fetch is required to render correct state anywhere.
- Interaction rules per WAI-ARIA APG (aria-checked="mixed" on the parent; checking a parent checks all; unchecking clears all; Space toggles): checking a collapsed not-stored node stores that node's path (a whole-subtree claim — this is what keeps Pitfall 2's denylist option alive); *unchecking a child of a stored parent* is the one operation that needs the child list (that level's listing) — do it on expand-demand and show a spinner, don't guess.
- Build the tri-state as a real component with dom tests (the repo already tests DropdownListbox/keyboard interactions this way): assert `aria-checked` values, Space behavior, and the click-on-mixed behavior as a spec, not an accident.

**Warning signs:**
A `useMemo` somewhere computing checkedness by walking `children` arrays; tests that only exercise fully-loaded trees; a PR comment asking "what should the parent show if we haven't loaded its kids?"

**Phase to address:** P3, with the state model defined in P2 (it *is* the semantics).

---

### Pitfall 11: Snapshot-structure mixing erodes full-coverage history and breaks restore assumptions

**What goes wrong:**
Retention identity is safe — tags stay `container:<ref>`/`fileset:<name>`, selection changes don't fork the tag lineage, and nobody should "helpfully" add path-derived tags (identity-stable, ungrouped; NEVER `--group-by paths`, #91). The surprise is temporal, not identity: after a user narrows a selection, old snapshots hold the full mount and new ones hold the subset, aging out together under the same tag. Bump retention or prune aggressively, and the last full-coverage snapshot silently disappears while plenty of "recent, green" partial snapshots remain. Meanwhile every restore surface that assumed one shape per item (SnapshotFileTree, restore-to-original path mapping, "latest snapshot" shortcuts) now meets items whose snapshot layout changed mid-history (Pitfall 3), and `restic snapshots --path` filtering behaves differently across the boundary.

**Why it happens:**
Coverage is only visible per-snapshot; no surface summarizes "this item had full coverage until March". Restore code written when each item had one shape has no reason to handle two.

**How to avoid:**
- Restore path mapping keyed on `snapshot.paths` + longest-prefix against current mounts (Pitfall 3) — this alone de-fangs most of it.
- When a selection *narrows* an item that has existing snapshots, say so once in the UI (toast or item card note: "new snapshots will contain only the selected folders; older snapshots still hold everything") — turning a silent historical cliff into a communicated one.
- SnapshotFileTree and any `--include` filters at restore time must tolerate both shapes for the same item; test "restore oldest" and "restore newest" for an item whose selection changed.
- Leave retention arithmetic exactly as is (identity-stable); the fix is communication and restore robustness, not new tags.

**Warning signs:**
A user fileset/container where `restic ls` of successive snapshots shows different top-level entries; a restore of "everything for plex" that restores a subset with no error; prune math discussed in terms of paths.

**Phase to address:** P5 (restore mapping + messaging); the "don't add path tags" invariant belongs in P2's review checklist.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Deriving "unselected" as excludes inverted from a leaf list at backup time ("parent minus selected") | No UI/persistence thought needed | Breaks on restic's no-reinclude rule (Pitfall 3); recomputes differently as disk changes; undebuggable argv | Never — pick parent+excludes or explicit leaves deliberately |
| Client-side caching of listings for the session without revalidation | Snappy re-expands | Stale tree drives wrong selections (Pitfall 8) | Only with revalidate-on-expand or explicit refresh affordance |
| One "list subtree recursively" endpoint to power badges/expand-all | Feels complete fast | O(tree) latency on shfs, disk spin-up, re-creates the problem laziness exists to avoid | Never for the tree; maybe as an explicit, capped diagnostic |
| Storing selection in component state and syncing on unmount | Quick prototype | Live-save contract broken; refresh loses selection; races (Pitfall 9) | Never — live-save is the established contract |
| Skipping dom tests for tri-state because "it's just a checkbox" | Faster UI phase | ARIA/pixels/test divergence ships (Pitfall 10) | Never — repo already has the dom-test harness |
| Special-casing Unraid vs TrueNAS paths in the tree component | Quick fix for a reported bug | Forks the identity/split-root logic the paths package centralizes | Never — fix in `toContainerPath`/`toHostPath` consumers per Pitfall 5 |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| restic argv | Passing selected paths as flags or unquoted positionals before `--` | User-influenced positionals strictly after `--`; add argv unit cases (Pitfall 3) |
| restic excludes | Assuming exclude patterns match host paths | They must be container-visible anchored paths — reuse `resolveExcludeLine`'s translation, never hand-roll a second one |
| ExcludesEditor + tree | Treating them as competing systems, or double-excluding the same subtree and confusing the preview | Compose them in the preview: tree-derived excludes and user patterns coexist; mark which lines the tree owns so edits don't surprise |
| SQLite (single connection) | Doing DB work inside a MutateSettings fn or adding writes on the listing path | Listing is read-only and DB-free; any settings-adjacent write goes through MutateSettings with no inner DB calls (existing invariant) |
| Migrations | Sneaking a schema change in "while we're here" for selection metadata | Zero-migration is the stated constraint; if a real need appears, it must be an append-only migration and an explicit decision |
| Docker inspect (mounts) | Building tree roots from *current* mounts only | Orphaned selections belong to CustomPath rendering, not to the mount tree (Pitfall 8) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-entry `Stat` for badges/counts during listing | Expand takes seconds; array disks spin up | Listing returns names only; badges are opt-in/out of scope | Immediately on Unraid shfs |
| Unbounded `os.ReadDir` + full sort | Multi-MB JSON, memory spike, long handler | Cap N entries + `truncated` flag; sort only the capped slice | ~10k entries in one dir |
| Eager full-child fetch on container panel open ("just to compute states") | Panel load regresses to the pre-lazy problem | States derive from the flat set (Pitfall 10); fetch only on expand | First big appdata container |
| Re-listing on every checkbox interaction | Constant FUSE chatter, janky UI | Fetch per expand, cache with revalidation (Pitfall 6) | Normal use |
| Ignoring `r.Context()` on the listing handler | Zombie readdirs after navigation away | ctx-goroutine + select (Pitfall 6); async tests wait for the goroutine | Every abandoned expand |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Trusting `paths.Resolve` alone for the listing | Symlink path component escapes `HostMountRoot`; LAN-reachable in trusted-LAN mode | `os.Root` for actual resolution (Pitfall 4); Resolve stays as cheap first reject |
| Echoing server paths in errors to the client | Host layout disclosure; violates the scrubbing invariant | Scrub paths → `[path]` *first*, reuse existing helpers; generic strings on the wire |
| Raw user paths in log statements | Log injection / layout leak; repo already carries G706 nolint discipline for this | Same discipline on new handlers: no raw user bytes in format strings |
| New endpoint bypassing the middleware stack | Missing CSRF/cross-origin/auth guards on an endpoint that reads the whole share | Route through the same mux chain (`csrfGate` is safe-method-exempt by design; body-taking variants must use `decodeBody`) |
| Treating selected paths as trusted at backup time | A crafted PATCH could store a path later handed to restic outside expectations | Re-validate stored paths at argv build (same containment vocabulary as SetBackupPaths); `--` discipline |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Indeterminate checkbox that behaves differently from its appearance | Users stop trusting the control; misclicks cascade | Spec the click-on-mixed behavior (fill is the common expectation); ARIA `aria-checked="mixed"`; dom tests (Pitfall 10) |
| Silent narrowing of coverage (uncheck → future folders excluded) | Discovering gaps at restore time | Coverage warning at backup time (Pitfall 2); explicit note when a selection narrows (Pitfall 11) |
| "Empty" folder that is actually unreadable | False confidence; doomed selections | Locked/`restricted` node state (Pitfall 7) |
| Tree forgets expansion/scroll on panel re-entry | Re-navigation tax in a settings-heavy app | Preserve expansion state per container in the session; expansion is view state, selection is persisted state — keep them separate |
| Showing host paths inconsistently between tree nodes and the mount header | User can't match tree to their file manager | One rendering rule: host path display everywhere (Pitfall 5) |
| Excluding volatile folders still slow because selection forced full re-read once | "I excluded transcoding, why did this run take so long" | Explain the one-time re-walk in the run note (restic change detection reset, Pitfall 3); subsequent runs are incremental |

## "Looks Done But Isn't" Checklist

- [ ] **Listing endpoint:** returns `truncated` flag and honors client cancellation — verify with a >N-entry fixture and an aborted request
- [ ] **Symlinks:** escaping symlink fixture under the temp root; symlinked dir *renders* as a link node rather than vanishing — verify listing vs restic's stored node agree
- [ ] **Permissions:** chmod-000 child renders `restricted`, never empty — verify on a PUID!=0-equivalent setup
- [ ] **Save failure path:** `{ok:false}` (not HTTP status) reverts the optimistic checkbox with the established shake/toast — verify with a forced 400-envelope
- [ ] **Out-of-order saves:** two rapid toggles end in the second one's state — verify with delayed first response
- [ ] **Unreachable/removed mounts:** render via custom-paths semantics with state preserved — verify by removing a bind mount between renders
- [ ] **Empty-selection intent:** unchecking all nodes cannot silently fall back to auto-detection — verify at the PATCH boundary (Pitfall 1)
- [ ] **Snapshot layout change:** restore oldest vs newest for an item whose selection changed maps paths correctly via `snapshot.paths` — verify in the restore panel
- [ ] **Split-root parity:** tree contract tested under `/mnt`→`/host/user` and identity roots — verify via the paths_test.go table pattern
- [ ] **web/dist:** rebuilt and committed after any tree UI change (embedded SPA invariant)

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Future folders missed after decomposition (2) | LOW | Select the new folder (or revert the item to parent+excludes); next snapshot includes it; older snapshots untouched |
| Wrong snapshot layout shipped (3) | MEDIUM | Fix argv strategy; restore already-stored snapshots via `snapshot.paths` mapping; one extra full re-walk on next run |
| Symlink escape found in browse (4) | LOW | Ship `os.Root` hardening; no data loss — it was a read; audit logs for abuse |
| Truncated/timeout listing in the field (6) | LOW | Client shows truncated/error state with retry; server change is additive |
| Lost update from racing PATCHes (9) | LOW | User re-toggles after fix; add regression test with delayed responses |
| Uncheck-everything auto-detect surprise (1) | MEDIUM | Restore intent by re-selecting; if a big unwanted backup already ran, prune per existing retention; add UI guard |
| Full-coverage snapshot pruned after narrowing (11) | HIGH | Only recoverable off-site (replica/immutable repos are never pruned locally — this is what they are for); otherwise re-back-up what still exists on disk |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1 Empty-selection → auto-detect inversion | P2 (semantics), P3 (UI guard) | PATCH-boundary test: tree-produced `[]` rejected or provably intentional |
| 2 Decomposition drops future children | P2 (decision), P4 (coverage diff) | Key Decision logged in PROJECT.md; coverage-warning test with a post-selection folder |
| 3 restic no-reinclude + snapshot layout | P2 (strategy), P4 (argv tests), P5 (restore) | restic_args_test cases incl. same-basename leaves; restore oldest/newest mapping test |
| 4 Symlink escape in listing | P1 | Escaping-symlink fixture rejected by `os.Root`; symlink renders as link node |
| 5 Host↔container translation | P1, P2, P3 | Tree contract tests run under split-root AND identity-root tables |
| 6 Huge-dir listing latency/cancellation | P1, P3 | Truncation flag test; aborted-request test that waits for the goroutine |
| 7 Permission errors as false-empty | P1, P3 | chmod-000 fixture returns `restricted`; UI renders locked node |
| 8 Stale tree / configured vs effective | P2 (vocabulary), P3 | Missing-path node shows warning, never silent uncheck; removed mount lands in custom paths |
| 9 Live-save races + `{ok:false}` envelope | P2 (contract), P3 (helper) | Envelope-failure revert test; out-of-order response test |
| 10 Mixed-state derivation under laziness | P2 (model), P3 (component) | Dom tests assert `aria-checked`/Space/click-on-mixed from flat-set states only |
| 11 Snapshot mixing / restore expectations | P5 | Restore handles both layouts for one item; narrowing note shown once |

## Sources

- **Codebase (HIGH, file:line cited throughout):** `internal/api/handlers.go:4143` (handleBrowse, mkdir), `internal/api/service.go:1137/3684` (toContainerPath/toHostPath), `:3550` (resolveAppdataPaths), `:3766` (SetBackupPaths), `:3816-3840` (configured vs effective), `internal/paths/paths.go:33` (lexical Resolve), `internal/api/empty_backup_guard_internal_test.go` (#181 semantics), `internal/api/service_test.go:2125-2175` (rejection + no-existence-required), `web/src/pages/Containers.tsx:537-1051` (optimistic toggle vs no-revert conventions), `web/src/components/FolderBrowser.tsx`, `SnapshotFileTree.tsx` (existing precedents)
- **restic official docs + source (MEDIUM, via Context7; doc text quoted verbatim):** 040_backup.rst (exclude `!` negation with the explicit "impossible to include" rule; absolute/relative path change detection), internal/fs/node.go (symlinks saved, not followed), snapshot JSON `paths` field + `HasPaths` filter
- **Go stdlib (MEDIUM, via Context7):** `os.Root` documentation, src/os/root.go (symlink containment guarantees and limits), go.mod go 1.25.0
- **WAI-ARIA APG checkbox pattern (LOW, via web reader):** tri-state semantics, `aria-checked="mixed"`, check-all/clear-all activation rules
- **Derived analysis (MEDIUM):** empty-selection/auto-detect collision and future-children gap reasoned from the code citations above; no community post-mortems fetched for this specific combination

---
*Pitfalls research for: tree-based sub-folder backup selection on BombVault (Go/restic/Unraid)*
*Researched: 2026-09-09*
