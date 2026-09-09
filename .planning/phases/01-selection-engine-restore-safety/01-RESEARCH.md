# Phase 1: Selection Engine & Restore Safety - Research

**Researched:** 2026-09-09
**Domain:** Go backend — selection normalization (`backupPaths` flat set), browse-endpoint hardening (`/api/browse`), restic positional targets, restore hardening (`prepareRestoreForTarget`)
**Confidence:** HIGH — every integration seam read from source this session; restic/os.Root facts verified against official sources via Context7; all positions pre-locked by `.planning/research/SUMMARY.md`

<user_constraints>
## User Constraints (from CONTEXT.md)

### Implementation Decisions

### Encodage des exclusions dans le flat set
- Q1: Sous-branches exclues encodées par préfixe `!` dans la MÊME liste plate (`!/mnt/user/appdata/plex/transcoding`) — zéro migration, zéro changement wire, une seule structure ; les chemins absolus commençant par `/` rendent le préfixe non ambigu
- Q2: Sémantique du préfixe dans des helpers purs d'un nouveau fichier `internal/api/selection.go` (normalisation, pruning d'ancêtres, classification inclus/exclu) appelés par `SetBackupPaths` et les readers — testable en table, pas de logique inline dans `internal/store/targets.go`
- Q3: Exclusion orpheline (sans ancêtre inclus) CONSERVÉE et signifiante : exclusions seules = « item explicitement désélectionné » — c'est l'état qui porte le garde anti-auto-détection (et le distingue d'une liste vide = auto-détection)
- Q4: Pruning d'ancêtres PAR CLASSE : une inclusion élide les inclusions strictement en dessous ; une exclusion élide les exclusions en dessous ; inclusion et exclusion coexistent volontairement (racine incluse + branche exclue)

### Contrat de listing des nœuds
- Q1: Étendre `GET /api/browse` additivement (`?hidden=1` + champs de réponse nouveaux) — un seul univers browsable, pas d'endpoint dupliqué ; FolderBrowser existant inchangé
- Q2: Champ additif `status` par listing : `ok` / `restricted` (permission-denied) / `missing`, message scrubbed ; liste vide + `status:"ok"` = vide réel — reste dans l'enveloppe HTTP 200
- Q3: PAS de hint `hasChildren` (pas de probe d'emptiness) — chaque répertoire est expandable (dérivé de `DirEntry.IsDir()`), listing d'un dossier vide rend 0 enfants ; BROWSE-01 reste satisfait (cheap, per-node)
- Q4: Cap d'entrées constant (ordre ~500, nombre exact à la discrétion du planning) + booléen `truncated:true` ; tri lexical comme aujourd'hui

### Garde anti-auto-détection au PATCH (backend INTEG-04)
- Q1: Aucun nouveau champ wire OBLIGATOIRE : l'état « item explicitement tout décoché » = exclusions orphelines persistées (≠ `[]`) ; champ optionnel `selectionSource:"tree"` dans le corps PATCH porte l'intention ; refus du `[]` nu venant d'une sélection antérieure non vide via la source arbre ; clients existants inchangés
- Q2: Refus en enveloppe maison : HTTP 200 + `{ok:false, error:<scrubbed>, code:"empty-selection"}` — le `code` machine permet à l'UI de Phase 3 de router la guidance
- Q3: Helpers génériques + wiring containers uniquement en Phase 1 ; File Sets suivront en Phase 4 via le même helper
- Q4: Sortie de l'état « tout décoché » (revenir à l'auto-détection) REPORTÉE en Phase 3 (INTEG-04 complète avec l'UI) — noté en deferred

### Durcissement restore — cas limites (RESTORE-01)
- Q1: Conteneurs uniquement (`AppdataPaths` / `prepareRestoreForTarget`) ; helpers génériques pour réutilisation File Sets en Phase 4
- Q2: Chemin du snapshot sans correspondance courante → skip + raison loguée scrubbed + skips signalés dans le résultat du restore (jamais d'abort global pour un chemin orphelin)
- Q3: Intersection vide (snapshot ne contient aucun chemin stocké) → abort propre AVANT tout teardown destructif, erreur explicite « rien à restaurer pour cet item depuis ce snapshot »
- Q4: Spot-check restic 0.17 (préservation chemins absolus ; excludes ne s'appliquant pas aux positionals) en contract tests dans la suite existante (restic ≥ 0.17 déjà requis sur PATH)

### Deferred Ideas (OUT OF SCOPE)
- Sortie de l'état « tout décoché » / retour explicite à l'auto-détection → Phase 3 (INTEG-04 UI)
- Note de rétrécissement UI (« new snapshots will contain only the selected folders ») → Phase 3 (SELECT-03)
- Coverage diff backup-time (siblings ni sélectionnés ni exclus) → v2 (SELECT-06, décision utilisateur 2026-09-09)
- Fanout « exclude this subfolder instead » vers ExcludesEditor → follow-up, hors v1
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BROWSE-01 | Listing children of a tree node is cheap and per-node | `handleBrowse` is already one `ReadDir` per request (handlers.go:4166); cap + truncation added (R1); **no `hasChildren` probe — locked by CONTEXT browse Q3, superseding the REQUIREMENTS.md parenthetical "child list + `hasChildren` hint via emptiness probe"** |
| BROWSE-02 | Distinguish "empty directory" from "error reading directory", scrubbed messages | Additive `status` field: `ok`/`restricted`/`missing` via `errors.Is` on `*fs.PathError` (R2); error strings stay generic |
| BROWSE-03 | Containment-safe listing (`os.Root`), never escapes root boundary | `os.OpenRoot` + `Root.Open` verified (Context7 golang/go); `paths.Resolve` kept as first reject; direct-address symlink escape closed (R6 anchor handlers.go:4154-4166) |
| BROWSE-04 | Hidden-entry visibility consistent between tree and folder browser | `?hidden=1` opt-in; default byte-identical; contract test pins the delta both ways (R8); FolderBrowser unchanged (locked Q1) |
| SELECT-01 | Tree state normalizes to/from flat `backupPaths` (maximal roots + exclusions) | `internal/api/selection.go` pure helpers (R3); wired into `service.SetBackupPaths` (service.go:3771-3792) and readers |
| SELECT-02 | Persistence format unchanged — zero migration | `store.SetBackupPaths` (targets.go:297-317) and `targets.selected_paths` JSON untouched; `!` entries are string data in the same list; `UpsertTarget` ON CONFLICT never resets the column (targets.go:129-131) |
| SELECT-04 | Whitelist start-state through the same normalization | Normalizer is class-symmetric: a pure include list passes through `PruneMaximal` unchanged in meaning (R3, test table) |
| RESTORE-01 | Restore survives selection changes — longest-prefix mapping against chosen snapshot's `Paths` | `prepareRestoreForTarget` (service.go:5332-5463) intersects/mmaps stored `AppdataPaths` with `chosen.Paths` from `snaps` already in hand (:5344); skip + empty-intersection abort semantics (R5); fail-fast loop at service.go:6752-6759 + post-teardown call site orchestrator.go:774 is the exact bug being fixed |
</phase_requirements>

## Summary

This phase is the brownfield keystone of the tree-selection milestone: it makes the flat `backupPaths` set losslessly express sub-folder selections (maximal includes + `!`-prefixed exclusions in the SAME list, zero schema or wire change), hardens `GET /api/browse` into a cheap, capped, symlink-safe, status-carrying node-listing contract, and fixes the one destructive bug selection churn exposes — a restore that replays the last run's stored path list against an older snapshot and fails mid-loop *after* the container has been stopped and removed. All of this was specified by `.planning/research/SUMMARY.md` + ARCHITECTURE.md + PITFALLS.md; this document verifies every seam against the current code with file:line anchors and resolves the items the milestone research left open for planning.

The critical discovery of this verification pass: **the `!`-prefix interacts with six existing readers in ways that would silently misbehave if only `SetBackupPaths` were changed.** `toContainerPath` would reject a `!`-prefixed host path outright (whole-save failure), `storedDataIsGone` would stat `!/host/...` paths and refuse backups as "not reachable" (the exact opposite of the locked exclusions-only = explicit-none semantic), and `ContainerMounts` would render every exclusion as a stale "custom path". Each is addressed in "Open Items Resolved" with a concrete seam. None of these are blockers; all are additive edits at verified anchors.

The phase needs **zero new dependencies** (Go stdlib only — `os.Root` is in the 1.24+ stdlib, repo floor/go version is 1.25.0) and **no web/ changes** (the additive JSON fields are invisible to the existing typed client; defer `api.ts` type updates to Phase 2 when a consumer exists — this keeps Phase 1 pure-Go and avoids a `web/dist` rebuild).

**Primary recommendation:** Build in the milestone's order — (1) `internal/api/selection.go` pure helpers + `SetBackupPaths` normalization + reader classification, (2) browse contract (`?hidden=1`, cap+`truncated`, `status`, `os.Root`), (3) PATCH empty-selection guard, (4) restore mapping in `prepareRestoreForTarget`, (5) restic 0.17 contract tests — with each step pinned by the test layer mapped in Validation Architecture.

## Locked Positions (constraints — do not re-derive)

| # | Position | Source |
|---|----------|--------|
| L1 | Selections compile to **maximal-root restic positional targets**; exclude-based encoding is DISQUALIFIED — restic excludes do not apply to positional sources (verified verbatim this session, see Sources) | SUMMARY.md tension resolution; RESEARCH.md Sources |
| L2 | Exclusions encode as `!` prefix in the SAME flat list; absolute paths make the prefix unambiguous; zero migration, zero wire change | CONTEXT.md encoding Q1 |
| L3 | Prefix semantics live as pure helpers in `internal/api/selection.go`, called by `SetBackupPaths` and readers — NOT inline in `internal/store/targets.go` | CONTEXT.md encoding Q2 |
| L4 | Orphan exclusions preserved and meaningful: exclusions-only = "explicitly deselected" ≠ `[]` = auto-detection | CONTEXT.md encoding Q3 |
| L5 | Per-class ancestor pruning: an include elides strictly-lower includes; an exclusion elides strictly-lower exclusions; include+exclude coexist (included root + excluded branch) | CONTEXT.md encoding Q4 |
| L6 | `GET /api/browse` extended additively only (`?hidden=1` + new response fields); no duplicate endpoint; FolderBrowser unchanged | CONTEXT.md browse Q1 |
| L7 | Per-listing `status`: `ok`/`restricted`/`missing`, scrubbed message, HTTP-200 envelope; empty + `status:"ok"` = genuinely empty | CONTEXT.md browse Q2 |
| L8 | NO `hasChildren` hint / no emptiness probe — every directory expandable | CONTEXT.md browse Q3 |
| L9 | Constant entry cap (~500 order) + `truncated:true`; lexical sort as today | CONTEXT.md browse Q4 |
| L10 | Empty-selection guard: no mandatory new wire field; optional `selectionSource:"tree"`; refuse bare `[]` only from tree source over a previously non-empty selection; `code:"empty-selection"` envelope | CONTEXT.md INTEG-04 Q1/Q2 |
| L11 | Restore: containers only; per-path skip (scrubbed log + skips in restore result), empty-intersection clean abort BEFORE destructive teardown | CONTEXT.md restore Q1/Q2/Q3 |
| L12 | restic 0.17 spot-checks (absolute-path preservation; excludes don't drop positionals) ship as contract tests in the existing suite | CONTEXT.md restore Q4 |
| L13 | Retention identity untouched: per-item tags, ungrouped — never `--group-by paths` (#91) or global `--group-by tags` | CLAUDE.md; PITFALLS #11 |
| L14 | No `--exclude` flag is ever derived from the selection; `snapshot.Excludes` stays user-owned | SUMMARY.md; CONTEXT Q1 encoding |

## Verified Integration Anchors

All read this session. Verbatim quotes beside load-bearing discrete values.

### HTTP layer

| Anchor | Verified |
|--------|----------|
| `internal/api/api.go:257` | `mux.HandleFunc("GET /api/browse", h.handleBrowse)` — the route the tree extends |
| `internal/api/api.go:258` | `mux.HandleFunc("POST /api/browse/mkdir", h.handleMkdir)` |
| `internal/api/api.go:186` | `mux.HandleFunc("PATCH /api/containers/{name}", h.handlePatchContainer)` |
| `internal/api/handlers.go:4143-4205` | `handleBrowse`: query `path` (4144); empty subpath lists root directly (4150-4151); `paths.Resolve(h.cfg.HostMountRoot, subpath)` (4154); `os.ReadDir(abs)` (4166); dirs-only `!e.IsDir()` skip (4178-4180); hidden skip `strings.HasPrefix(name, ".")` (4182-4184); rel path built from `subpath+"/"+name` (4186-4191); explicit lexical sort (4197); success payload `{ok:true, root, path, dirs}` (4199-4204) |
| `internal/api/handlers.go:4158-4161` | Traversal rejection: `{"ok": false, "error": "invalid path: must be a relative subpath under the mount root"}` — byte-identical response required going forward |
| `internal/api/handlers.go:4169-4172` | Read failure: `{"ok": false, "error": "could not read directory"}` — today ENOENT and EACCES collapse into this one string (the BROWSE-02 gap) |
| `internal/api/handlers.go:2935-2938` | `type browseDirEntry struct { Name string; Path string }` — entry shape unchanged |
| `internal/api/handlers.go:585` | `resourceNameRe = regexp.MustCompile(\`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$\`)` — guards the `{name}` path param only. **It is NOT applied to `backupPaths` elements** — no collision with `!`-prefixed entries at this gate |
| `internal/api/handlers.go:1090-1161` | `handlePatchContainer`: pointer-fields body struct, `BackupPaths *[]string \`json:"backupPaths"\`` (1101) → `h.svc.SetBackupPaths(r.Context(), name, *body.BackupPaths)` (1123-1128) → `failEnvelope(err)` on error (1125). The `selectionSource` field and the coded envelope land here |
| `internal/api/handlers.go:365-383` | `decodeBody`: `crossOriginGuard` (365-368), `http.MaxBytesReader(w, r.Body, 1<<20)` // 1 MiB (375), `dec.DisallowUnknownFields()` (377). Adding `selectionSource` to the body struct is mandatory (unknown fields are rejected); omitting it stays legal (pointer field) |
| `internal/api/handlers.go:1283-1307` | `handleContainerMounts` response: `{mounts, custom, hostMountRoot, hostSourceRoot}` inside `okEnvelope` (1301-1306) — the additive `excluded` field lands here |

### Path containment

| Anchor | Verified |
|--------|----------|
| `internal/paths/paths.go:33-51` | `Resolve(root, sub)`: rejects absolute sub (35-37), joins+cleans, strict-prefix check `strings.HasPrefix(cleaned, cleanRoot+"/")` (45-48). Purely lexical |
| `internal/paths/paths.go:58-65` | `Within(root, absPath)` — restore-side re-validation of stored absolute paths |
| `internal/paths/paths.go:31-32` | POSIX-only note: "Paths here are always Linux paths (container-internal)" — `!` prefix and `/`-prefix tests are safe on all build OSes |
| `internal/paths/paths.go:12,15` | Sentinels `ErrTraversal`, `ErrAbsoluteSub` |

### Service layer — selection persistence and readers

| Anchor | Verified |
|--------|----------|
| `internal/api/service.go:1137-1148` | `toContainerPath(host)`: prefix arithmetic vs `HostSourceRoot`/`HostMountRoot`; returns `("", false)` when not reachable. **A `!`-prefixed host path fails `strings.TrimPrefix(p, srcRoot+"/")` → `("", false)` → the whole save is rejected** (verified logic) — the prefix must be parsed before translation |
| `internal/api/service.go:3684-3695` | `toHostPath(cp)`: inverse; returns input unchanged when not under mount root (so a stored `!/host/...` would round-trip out still `!`-prefixed and wrong-rooted — readers must decode first) |
| `internal/api/service.go:3771-3792` | `SetBackupPaths(_ ctx, name, hostPaths)`: TrimSpace (3775), skip empty (3776-3778), `toContainerPath` per element with whole-update rejection `path %q is not under the host mount and can't be backed up` (3784), dedupe on container path (3786-3789) → `s.store.SetBackupPaths(name, cps)` (3791). **The normalization + `!` parse/re-attach + empty-tree guard insert here** |
| `internal/api/service.go:3826-3832` | `configuredBackupPaths`: `if existing, gErr := s.store.GetTargetByContainer(name); gErr == nil && len(existing.SelectedPaths) > 0 { chosen = existing.SelectedPaths }` — the explicit-vs-auto test on the RAW list (exclusions-only is correctly non-empty = explicit). Returns must become includes-only for downstream consumers |
| `internal/api/service.go:3838-3840` | `effectiveBackupPaths` = `onlyExistingPaths(configuredBackupPaths(...))` — the existence filter that becomes positionals |
| `internal/api/service.go:3864-3888` | `emptyBackupIsUnreachable` = `len(effective) == 0 && s.storedDataIsGone(name)`; `storedDataIsGone` stats `existing.SelectedPaths` (3883-3887). **With `!` entries stored raw, every stat fails → returns true → backup refused as "not reachable" — wrong for the explicit-none state; must classify (R4)** |
| `internal/api/service.go:3722-3764` | `ContainerMounts`: `effective := tg.SelectedPaths; if len(effective) == 0 { effective = auto }` (3730-3733); `selSet[cp]` exact-match drives `MountInfo.Selected` (3746); unmatched selected paths → `CustomPath` with `os.Stat` existence flag (3757-3761). **Raw `!` entries would become stale custom paths; must split classes (R2)** |
| `internal/api/excludes_suggest.go:1007-1013` | `SuggestExcludes`: `configured := s.configuredBackupPaths(name, in)`; `roots := onlyExistingPaths(configured)` — inherits the includes-only fix automatically once `configuredBackupPaths` splits |

### Service layer — backup

| Anchor | Verified |
|--------|----------|
| `internal/api/service.go:3957-4055` | `Backup`: `effective := s.effectiveBackupPaths(name, in)` (4036); #181 guard `if s.emptyBackupIsUnreachable(name, effective)` → refuse (4051-4055); `UpsertTarget(store.Target{..., AppdataPaths: effective, ...})` (4063); `BackupDeps{AppdataPaths: effective, ...}` (4111-4115) |
| `internal/api/service.go:3550-3619` | `resolveAppdataPaths` — the auto-detection fallback; unchanged by this phase |
| `internal/backup/orchestrator.go:378` | `tags := []string{"container:" + d.ContainerRef, "p1"}` — identity-stable per-item tags (L13) |
| `internal/backup/orchestrator.go:466-472` | `if len(d.AppdataPaths) > 0 { summary, backupErr = d.Restic.Backup(ctx, d.RepoPath, d.AppdataPaths, tags, d.Excludes...) }` — **an empty positional list already yields a definition-only backup (restic skipped)**; the explicit-none state reuses this path once the #181 guard classifies correctly |
| `internal/restic/restic.go:361-384` | `BackupArgs`: `--exclude` per pattern (378-380), then `args = append(args, "--"); args = append(args, paths...)` (381-382) — positionals after `--`, argv shape unchanged by this phase (path list is data, not argv shape) |

### Service layer — restore (the RESTORE-01 bug site)

| Anchor | Verified |
|--------|----------|
| `internal/api/service.go:5225-5235` | `containerRestorePlan` fields: `repo, mode, targetID, snapshotID, recreateOnly, appdataPaths, restoreDirs, inspect, templateXML` — the `skippedPaths` field is added here |
| `internal/api/service.go:5266-5287` | `prepareRestore`: confirm/name/snapshot-id guards first, repo resolution, → `prepareRestoreIn` |
| `internal/api/service.go:5295-5322` | `prepareRestoreIn`: store lookup `tg` → `prepareRestoreForTarget(ctx, ref, name, snapshotID, tg, "", false)` |
| `internal/api/service.go:5332-5463` | `prepareRestoreForTarget`: `snaps, snapErr := s.snapshotsForTag(ctx, ref.repo, ref.mode, "container:"+name)` (5344) — **the snapshot list, each entry carrying `Paths`, is already in hand at exactly the right place**; latest resolution `snaps[len(snaps)-1].ID` (5355); `appdataForRestore := tg.AppdataPaths` (5366); per-path `paths.Within` re-validation (5375-5380); cross-pool remap consumes `appdataForRestore` (5392-5417); plan built at 5452-5462. **The mapping inserts between snapshot resolution and the remap block** |
| `internal/api/service.go:5468-5520` | `executeRestore`: `RestoreDeps{..., AppdataPaths: plan.appdataPaths, RestoreDirs: plan.restoreDirs, ...}` (5496-5514) |
| `internal/api/service.go:6752-6759` | `resticAdapter.RestorePaths`: `for _, p := range paths { if err := a.engine.RestorePath(...); err != nil { return err } }` — **fail-fast on first missing path** |
| `internal/restic/restic.go:489-511` | `RestoreSubtreeToArgs` (`restore <id>:<subtreePath> --target <target>`; selector after `--`, :500) and `RestorePathArgs` (= special case target==subtree). Doc comment :485-487: "callers take it from the SNAPSHOT's Paths (not a recomputed value)" — the mapped restore list must be snapshot-path-form |
| `internal/restic/restic.go:165-172` | `type Snapshot struct { ID string; Time string; Paths []string \`json:"paths"\`; Tags []string; Hostname string; Original string }` — `Paths` is the mapping source |
| `internal/backup/orchestrator.go:691-794` | `runRestore`: `VerifySnapshot` pre-flight (735 — proves existence only, not per-path presence) → `Pull` (744) → **`Stop` (749) + `Remove` (754) — destructive teardown** → `d.Restic.RestorePaths(ctx, d.RepoPath, d.SnapshotID, d.AppdataPaths)` (774). A missing path aborts HERE, after teardown — the exact bug RESTORE-01 fixes |
| `internal/backup/orchestrator.go:103-105` | `Restic` port: `RestorePaths(ctx, repo, snapshotID string, paths []string) error` — unchanged; hardening happens in the service before the orchestrator ever runs |

### Store (unchanged by this phase)

| Anchor | Verified |
|--------|----------|
| `internal/store/targets.go:25-28` | `SelectedPaths []string` — "Empty means 'use the automatic appdata detection'. Owned by SetBackupPaths (never reset by Upsert)" |
| `internal/store/targets.go:297-317` | `SetBackupPaths`: `if selected == nil { selected = []string{} }` (298-300); UPDATE-then-Upsert-on-zero-rows; `[]` persists as `[]` = auto-detect (SELECT-02 boundary preserved byte-for-byte) |
| `internal/store/targets.go:126-131` | `UpsertTarget` ON CONFLICT updates ONLY `appdata_paths` and `definition` — `selected_paths` never clobbered by a backup |
| `internal/store/targets.go:539-562` | `scanTarget` — plain JSON unmarshal; `!` entries are opaque strings to the store |

### Test infrastructure (existing, reusable)

| Anchor | Verified |
|--------|----------|
| `internal/api/handlers_test.go:29-65` | `newTestRouter` / `newTestRouterSvc` / `newTestRouterSvcDir` (identity-root `HostMountRoot: dir`, seeds `appdata/plex`); `doJSON` helper at :82-98 |
| `internal/api/handlers_test.go:1523-1531` | `newBrowseRouter(t, mountRoot)` — purpose-built harness for browse tests |
| `internal/api/handlers_test.go:1533-1628` | Existing browse contract tests: `TestBrowseListsMountRoot` (1533; asserts file + hidden dir excluded), `TestBrowseListsSubpath` (1574), `TestBrowseRejectsTraversal` (1602), `TestBrowseRejectsAbsolutePath` (1617) — **must stay green byte-identically** |
| `internal/api/service_test.go:3794-3824` | `fakeResticEngine`: `lastPaths []string` (3797, backup positionals), `restored []string` (3800), `snaps []restic.Snapshot` (3809, seedable snapshot Paths) — success criteria 2 and 5 fully testable with existing fakes |
| `internal/api/service_test.go:2124-2147` | Existing `SetBackupPaths` + backup flow test (host-path input, unreachable rejection, `eng.lastPaths` assertion) — the pattern to extend |
| `internal/restic/restic_args_test.go` | Argv table discipline: `TestBackupArgs` (294), `TestBackupArgsExcludes` (310), `TestBackupArgsContainerExcludes` (320), `TestRestorePathArgs` (501) |
| `internal/restic/restic_roundtrip_test.go:16-22` | Real-restic test pattern: `if _, err := exec.LookPath("restic"); err != nil { t.Skip("no restic") }` — "runs in CI where restic is installed" |
| `.github/workflows/lint.yml:49-61` | Test job installs `RESTIC_VERSION=0.17.3` (SHA256-pinned) before `go test ./...` — the proving ground for the L12 spot-checks |

### PROJECT.md (phase bookkeeping)

| Anchor | Verified |
|--------|----------|
| `.planning/PROJECT.md:67-74` | Key Decisions table — 3 rows currently `— Pending`. During this phase: log the positional-targets decision and the future-children allowlist semantic (prescribed by SUMMARY.md; phase Notes require it) |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os` (`os.OpenRoot`, `Root.Open`, `File.ReadDir`) | go1.25.0 (go.mod floor `go 1.25.0`; toolchain go1.25.0 windows/amd64 verified) | Symlink-contained directory listing | `os.Root` landed in Go 1.24 stdlib; containment guarantee is documented stdlib behavior — no third-party dependency is warranted or allowed (constraint: stdlib `net/http` only, no new deps) |
| `internal/paths` | in-repo | Lexical first-reject (`Resolve`), restore re-validation (`Within`) | Existing, tested, split-root/identity-root table-tested |
| restic binary | 0.17.3 (CI-pinned, lint.yml:51) | Storage engine; positionals + user excludes only | Locked engine; the ONLY touching code is `internal/restic` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `go test` + `httptest` | go1.25.0 | All test layers | Every phase test; house style |
| `errors.Is` + `fs.ErrPermission`/`fs.ErrNotExist` | stdlib | `status` classifier on `*fs.PathError` | R2 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.Root` listing | Keep `paths.Resolve` + `os.ReadDir` | Resolve is lexical only — direct-address symlink escape remains (PITFALLS #4). Root is stdlib and cheap; Resolve stays as defense-in-depth |
| `filepath.WalkDir`-style recursion for hasChildren | — | Locked OUT (L8): probes are an N+1 latency multiplier on shfs/FUSE |
| Wire `backupPaths` as structured objects `{path, excluded}` | `!` prefix in flat strings | Locked OUT (L2): zero wire change, existing clients keep working |
| Deriving `--exclude` flags from the selection | Maximal-root positionals | Locked OUT (L1): hard error class — restic ignores excludes that match positional sources |

**Installation:** None. Zero new Go modules, zero npm packages, no migrations, no Dockerfile changes.

**Version verification:** `go version` → go1.25.0 windows/amd64 (matches floor; `os.Root` available). `restic` NOT on PATH locally — real-restic tests skip locally and run in CI (0.17.3, verified lint.yml:49-61).

## Package Legitimacy Audit

No external packages are installed by this phase (Go stdlib + in-repo code only). Nothing to audit; no `[ASSUMED]` package names anywhere in this research.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Open Items Resolved (planning inputs)

### R1. Listing cap — use 500
`const maxBrowseEntries = 500` (handlers.go, near `browseDirEntry`). Rationale: CONTEXT Q4 says "ordre ~500, nombre exact à la discrétion du planning"; 500 entries ≈ tens of KB of JSON (fine on LAN) and matches the PITFALLS #6 "first N sorted + truncated" design. Order of operations: `ReadDir(-1)` → filter (dirs, hidden per flag) → sort lexically (as today) → truncate to 500 → `truncated = len(kept) > 500`. Deterministic (lexically-first 500) and the sort stays on the filtered slice only.

### R2. Additive response shapes
- **Browse success** (both flag states): existing fields byte-identical (`ok`, `root`, `path`, `dirs[{name,path}]`); ADD top-level `"status":"ok"` and `"truncated":bool`. No `hasChildren` (L8). Hidden flag: `?path=<p>&hidden=1` — when absent, dot-dir skip is unchanged (L6).
- **Browse read failure** (the `os.ReadDir` error branch, handlers.go:4167-4173): keep `"error":"could not read directory"` verbatim (scrubbed, generic — the state carries the kind, not the path) and ADD `"status"` classified from the `*fs.PathError`: `errors.Is(err, fs.ErrPermission)` → `"restricted"`; `errors.Is(err, fs.ErrNotExist)` → `"missing"`; anything else (including an `os.Root` escape rejection) → `"error"`.
- **Browse traversal/absolute rejection** (4158-4161): byte-identical to today, NO status field — existing contract tests stay untouched.
- **Mounts endpoint** (handlers.go:1301-1306): ADD `"excluded":[]` — the decoded exclusion entries in HOST form (`toHostPath` of the bare path), never mixed into `custom`; `Selected` flags computed from includes only. Empty slice (not null) when no exclusions, matching the existing `mounts`/`custom` nil-guard style (1293-1298).

### R3. `internal/api/selection.go` — pure helpers (no store, no receiver, table-testable)
```go
// ExclusionPrefix marks a deselected sub-branch in the flat backupPaths set.
// Absolute POSIX paths make the prefix unambiguous (CONTEXT 2026-09-09, encoding Q1).
const ExclusionPrefix = "!"

// SplitExclusion parses one flat-set entry: "!/mnt/x" → ("/mnt/x", true);
// "/mnt/x" → ("/mnt/x", false). A bare "!" (empty path) is the caller's reject.
func SplitExclusion(entry string) (bare string, excluded bool)

// PruneMaximal drops any path that has another path in the list as a strict
// ancestor (same-class pruning, CONTEXT encoding Q4). Strictness test:
// strings.HasPrefix(child, ancestor+"/") — POSIX, per internal/paths note.
func PruneMaximal(paths []string) []string

// NormalizeSelection canonicalizes a mixed, already-container-translated entry
// list: split classes → per-class PruneMaximal → dedupe → stable canonical
// order = includes sorted lexically, then exclusions sorted lexically, each
// exclusion stored as "!" + container path.
func NormalizeSelection(entries []string) []string
```
Call sites: `service.SetBackupPaths` (parse `!` → translate bare via `toContainerPath` → re-attach → `NormalizeSelection` → store), `configuredBackupPaths`/`ContainerMounts`/`storedDataIsGone` (split via `SplitExclusion`). A table-driven `selection_test.go` covers: maximal-roots invariants, per-class pruning (L5: include `/x` + exclude `/x/y` both survive; exclude `/x` elides exclude `/x/y`), whitelist start-state (SELECT-04: pure include list → `PruneMaximal` only), orphan exclusions preserved (L4), split-root AND identity-root translation (mirror `paths_test.go`'s table pattern).

### R4. Backup-time and reader semantics for `!` entries (the discovery set)
- `service.SetBackupPaths`: parse prefix BEFORE `toContainerPath` (else whole-save rejection, verified above); validate bare path of BOTH classes identically (under host mount); store `!`+containerVisible.
- `configuredBackupPaths` (service.go:3826): keep the explicit test on the RAW list (`len(existing.SelectedPaths) > 0`), return includes-only. Downstream (`effectiveBackupPaths`, `SuggestExcludes`) inherit correct behavior.
- `storedDataIsGone` (service.go:3874-3888): if raw `SelectedPaths` non-empty but its includes are empty → explicit-none → return `false` (never "data gone"); otherwise measure includes as today. Without this, a legitimate explicitly-deselected container is refused as "not reachable" (verified stat-failure logic).
- `Backup` (service.go:4036+): positionals = includes (existence-filtered); explicit-none → empty positional list → definition-only backup via the EXISTING orchestrator skip (orchestrator.go:466-472) — no new backup mode.
- `ContainerMounts` (service.go:3722-3764): `effective` split; includes drive `selSet`/`matched`/custom loop; exclusions → new `excluded` response field (R2). Without this, every exclusion renders as a stale "custom path" with `Exists:false`.
- `AppdataPaths` (recorded at backup, service.go:4063) stays positional-truth (includes only) — never carries `!`; restore mapping input is clean.

### R5. Restore mapping — precise semantics (RESTORE-01)
Insert in `prepareRestoreForTarget` between snapshot resolution (service.go:5348-5361) and the `destBase` remap block (:5392), so cross-pool remaps operate on mapped paths:
- Input: stored `tg.AppdataPaths` (positional truth) ∩ chosen snapshot's `Paths` (the `*restic.Snapshot` resolved from `snaps` — in hand at :5344/:5355).
- Clause 1 (descendant/direct): every snapshot path `q` equal to or strictly below some stored `p` is restored as-is (`q` is a valid `<id>:<q>` selector and within stored intent).
- Clause 2 (longest-prefix ancestor): a stored `p` with no snapshot path at-or-below it falls back to the LONGEST `q ∈ snapshot.Paths` that is an ancestor of `p` (`HasPrefix(p, q+"/")`) — never first-path-component (RESTORE-01 wording); restoring `q`'s subtree covers `p`.
- Clause 3 (skip): a stored `p` matching neither clause is SKIPPED — scrubbed log (`paths → [path]` first) + recorded in new plan field `skippedPaths []string` (add to `containerRestorePlan`, service.go:5225-5235, and mirror as `SkippedPaths []string` on `backup.RestoreDeps` so the orchestrator's run record can carry a bounded, scrubbed skip note — the "skips signalés dans le résultat du restore" of CONTEXT restore Q2). Never a global abort for an orphan path.
- Abort: if stored paths existed and the mapped result is empty → return a clean error ("nothing to restore for this item from this snapshot") from `prepareRestoreForTarget` — i.e. BEFORE `executeRestore`'s Stop/Remove (orchestrator.go:749-754). This is the entire safety property: all failure resolution happens in the synchronous prepare phase.
- The plan's `appdataPaths` field carries the MAPPED list in snapshot-path form (matches `RestoreSubtreeToArgs` doc, restic.go:485-487). `VerifySnapshot` pre-flight (orchestrator.go:735) stays. Recreate-only path untouched. `runRestore`'s SEC per-path guard (orchestrator.go:702-706) is shape-compatible (mapped paths are absolute POSIX).

### R6. `os.Root` integration
Per-request `os.OpenRoot(h.cfg.HostMountRoot)` + `defer root.Close()` (one fd, no long-lived handle; goroutine-safe per stdlib docs). Empty `subpath` → open `"."` (Root names may reference the directory itself — stdlib contract). Listing: `root.Open(rel)` → `f.ReadDir(-1)` (no `Root.ReadDir` in the verified 1.24-1.25 API surface) → existing explicit sort. `paths.Resolve` stays as the cheap first reject producing today's byte-identical "invalid path" response. Effect: direct-address `?path=appdata/link-to-etc` now errors instead of listing `/etc` (PITFALLS #4 closed). Listed entries that are symlinks still skip via `e.IsDir()==false` — the listing never offers a path Root would reject; restic's symlink-node-vs-listing nuance is documented, not changed (rendering choice belongs to Phase 2).

### R7. restic 0.17 spot-check contract tests (L12)
Two real-restic tests in the existing suite, using the `exec.LookPath("restic") → t.Skip` pattern (restic_roundtrip_test.go:19-22); they run in CI against 0.17.3 and skip on this dev box (restic absent — verified):
1. **Excludes don't drop positionals**: init → seed `src/dir/{file.txt,ex.txt}` → `Backup(repo, []string{srcDir}, tags, mode, "ex.txt")` → assert snapshot `Paths == [srcDir]` AND `file.txt` present AND `ex.txt` filtered (proves both halves of the doc claim).
2. **Absolute-path preservation**: `Backup(repo, []string{absDir}, ...)` → snapshot `Paths == [absDir]` verbatim (the snapshot-layout stability the maximal-roots form relies on).

### R8. BROWSE-04 hidden consistency — what Phase 1 actually proves
Contract test pair on one fixture: no flag → dot-dir absent; `hidden=1` → dot-dir present, all other entries identical and identically sorted. "Consistency" at this phase = one endpoint, default behavior byte-identical (FolderBrowser and every destination picker keep agreeing with today), the tree's future opt-in is additive and pinned. Whether the tree *shows* hidden entries and how FolderBrowser should treat them is a Phase 2/3 UI decision using this same tested contract.

## Architecture Patterns

### System Architecture Diagram

```
PATCH /api/containers/{name} {backupPaths: [host paths, "!"=excluded], selectionSource?}
  │
  ▼
handlePatchContainer (handlers.go:1090)
  │ decodeBody: 1 MiB + DisallowUnknownFields        │ empty + source=="tree" + prior non-empty
  ▼                                                  ▼ → {ok:false, code:"empty-selection"}
service.SetBackupPaths (service.go:3771)
  │ parse "!" per entry → toContainerPath(bare) → re-attach
  │ NormalizeSelection (NEW selection.go: per-class prune, canonical order)
  ▼
store.SetBackupPaths (targets.go:297) ── targets.selected_paths JSON (UNCHANGED, zero migration)

GET /api/browse?path=<rel>[&hidden=1]
  │
  ▼
handleBrowse (handlers.go:4143)
  │ paths.Resolve (lexical reject, unchanged response)
  │ os.OpenRoot(HostMountRoot) → root.Open(rel) → ReadDir(-1)   [NEW containment]
  │ filter dirs (/+hidden unless flag) → sort → cap 500 → truncated
  │ classify error: restricted / missing / error               [NEW status]
  ▼
{ok, root, path, dirs, status, truncated}

svc.Backup (service.go:3957)
  │ configuredBackupPaths → split(!): includes-only (NEW)
  │ effectiveBackupPaths → onlyExistingPaths(includes)
  │ storedDataIsGone: explicit-none ⇒ never "gone"          [NEW classification]
  ▼
BackupDeps.AppdataPaths = maximal includes ──► restic backup -- <positionals>  (orchestrator.go:472)
                                             snapshot.Paths == positionals (verbatim absolute)

svc.Restore (service.go:5242)
  │
  ▼
prepareRestoreForTarget (service.go:5332)
  │ snaps = snapshotsForTag("container:"+name)              (:5344, already in hand)
  │ chosen = resolved snapshot ──► chosen.Paths
  │ mapRestorePaths(tg.AppdataPaths, chosen.Paths)           [NEW: longest-prefix, skips]
  │ empty mapped + stored non-empty ⇒ clean abort            [NEW: BEFORE teardown]
  ▼
executeRestore → runRestore (orchestrator.go:691)
  VerifySnapshot → Pull → Stop → Remove → RestorePaths(mapped, no fail-fast trap)
```

### Recommended Project Structure
```
internal/api/
├── selection.go            # NEW: ExclusionPrefix, SplitExclusion, PruneMaximal, NormalizeSelection
├── selection_test.go        # NEW: table-driven normalization/pruning/classification + both root configs
├── handlers.go              # EDIT: handleBrowse (Root, hidden, cap, status), handlePatchContainer (selectionSource + coded envelope), handleContainerMounts (excluded)
├── service.go               # EDIT: SetBackupPaths (parse/normalize/guard hook), configuredBackupPaths + storedDataIsGone + ContainerMounts (split), prepareRestoreForTarget (mapping)
└── *_test.go                # EDIT/NEW: browse contract, PATCH guard, restore hardening tests
internal/backup/
└── orchestrator.go          # EDIT (additive only): RestoreDeps.SkippedPaths + run record note
internal/restic/
├── restic_args_test.go      # EDIT: argv cases (multi-path positionals, same-basename leaves) if not already covered
└── restic_positionals_contract_test.go  # NEW: real-restic 0.17 spot-checks (skip w/o binary)
```

### Anti-Patterns to Avoid
- **Storing `!` entries raw and only fixing the setter:** six readers misbehave (R4 list). Fix every reader in the same phase, or the explicitly-deselected state bricks backups.
- **Deriving `--exclude` from the selection:** hard error class (L1) — restic silently backs up positionals that match excludes.
- **Mapping restore paths by first path component or by current selection:** RESTORE-01 names longest-prefix against the CHOSEN snapshot's `Paths` explicitly.
- **Fixing `hasChildren` with an emptiness probe:** shfs/FUSE N+1 (L8, PITFALLS #6).
- **Special-casing the whitelist start-state:** one normalizer, class-symmetric (SELECT-04).
- **Silent repair of the stored list:** dropping stale entries is the engine's job at run time (`onlyExistingPaths`), never the save path's.
- **Collapsing the HTTP-200 envelope:** tests must assert `ok`, never status codes (PITFALLS #9).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Symlink-safe containment | Custom resolve-and-lstat walk | `os.Root` (stdlib, go1.24+) | Kernel-enforced containment; custom walks have TOCTOU races |
| Path containment pre-check | New prefix validator | `internal/paths` (`Resolve`/`Within`) | Already split-root/identity-root table-tested |
| Ancestor/strict-descendant test | Path-segment parser | `strings.HasPrefix(child, parent+"/")` on cleaned POSIX paths | All domain paths are POSIX by convention (paths.go:31-32); a segment parser is surplus machinery |
| Error-kind classification | String-matching on error text | `errors.Is(err, fs.ErrPermission / fs.ErrNotExist)` on `*fs.PathError` | stdlib contract; text matching breaks across platforms/locales |
| Maximal-root pruning at save | Client-side trust | Server-side `NormalizeSelection` invariant | Server is the safety net against buggy/old clients (ARCHITECTURE) |
| restic behavior proofs | Reasoning from docs alone | Real-restic contract tests (L12) | The 0.17 floor predates the doc page verified (master); CI is the arbiter |

**Key insight:** everything risky here (containment, change detection, restore selectors) already has a stdlib or in-repo primitive; the phase is disciplined wiring plus tests, not invention.

## Runtime State Inventory

Behavior-compatibility phase (no rename/migration), but existing persisted state is the compatibility contract:

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | Existing `targets.selected_paths` rows: JSON string arrays of bare container-visible paths (never `!`-prefixed today — the prefix does not exist yet). Migration 0009-family `target_selected_paths` schema untouched | None — readers must treat a prefix-free list as all-includes (the `SplitExclusion` default). No data migration |
| Live service config | None — no external service stores these strings | None — verified: selection lives only in SQLite |
| OS-registered state | None | None |
| Secrets/env vars | None — no env names change | None |
| Build artifacts | `web/dist` embed: NOT touched this phase (no web/ changes recommended). `go build` chain unaffected | None — do not rebuild `web/dist` |

## Common Pitfalls

### Pitfall 1: `toContainerPath` rejects `!`-prefixed host paths (whole-save failure)
**What goes wrong:** the PATCH wire format is host paths; `toContainerPath("!/mnt/...")` fails its `TrimPrefix(p, srcRoot+"/")` test → `("", false)` → `SetBackupPaths` rejects the entire update with "path %q is not under the host mount".
**Why it happens:** the prefix parse was forgotten before translation.
**How to avoid:** `SplitExclusion` FIRST, translate the bare path, re-attach the prefix to the container form (R4).
**Warning signs:** first tree save with an exclusion fails with "not under the host mount".

### Pitfall 2: Explicit-none containers get refused as "not reachable" (#181 guard inversion)
**What goes wrong:** `storedDataIsGone` stats raw `SelectedPaths`; `!`-prefixed entries never stat → all-failed → `emptyBackupIsUnreachable` refuses the backup.
**Why it happens:** the guard predates exclusions; its stat loop assumes bare paths.
**How to avoid:** classify in `storedDataIsGone` — non-empty raw list with empty includes ⇒ explicit-none ⇒ `false` (R4).
**Warning signs:** a container whose user deselected everything can never back up again, error mentions "appdata share mounted".

### Pitfall 3: Exclusions rendered as stale "custom paths"
**What goes wrong:** `ContainerMounts` treats every unmatched stored path as `CustomPath`, stats it (`Exists:false`), and `toHostPath` passes the `!`-prefixed container path through unchanged — the UI would show a phantom `/host/...` entry.
**How to avoid:** split classes before the custom loop; expose exclusions via the new `excluded` field (R2).

### Pitfall 4: Restore still aborts after teardown for a path the mapping could have skipped
**What goes wrong:** hardening added in `executeRestore` or the adapter instead of the prepare phase; or the empty-intersection check placed after `UpsertTarget`/`guardContainerRestoreDestination` side effects.
**How to avoid:** all mapping/decisions in `prepareRestoreForTarget` (synchronous, pre-teardown by construction); the orchestrator only ever receives a mapped, non-empty-when-possible list (R5).
**Warning signs:** a test where `restored` (fake engine) is partially populated and the run still ends "failed".

### Pitfall 5: Windows dev hides the symlink fixture
**What goes wrong:** the escaping-symlink test can't create symlinks on Windows without privileges → fixture silently skipped → containment untested locally (it still runs on Linux CI).
**How to avoid:** explicit `runtime.GOOS` skip with a loud message (mirrors the repo's POSIX-only test convention); assert the `os.Root` error path in a unit seam where possible.

### Pitfall 6: Contract drift on the untouched branches
**What goes wrong:** refactoring `handleBrowse` changes the no-flag response shape (field order aside, presence/absence) and breaks FolderBrowser and the four existing browse tests.
**How to avoid:** existing tests (`TestBrowseListsMountRoot` etc.) must pass UNMODIFIED; new assertions only in new tests. The traversal response stays byte-identical (no status field).

### Pitfall 7: `selectionSource` breaks the PATCH for old clients
**What goes wrong:** making the field mandatory, or rejecting unknown values so hard that a future source value fails a save.
**How to avoid:** pointer field (optional), only `"tree"` carries meaning, any other value ignored (treated as absent). SPA and server ship in one binary (embedded `web/dist`) so version skew is a non-issue — note it in the field's comment.

## Code Examples

### Normalization invariant (selection.go target behavior)
```go
// Source: CONTEXT.md 2026-09-09 (encoding Q1/Q4) — house style: table test in selection_test.go
input:  ["/c/appdata/plex", "/c/appdata/plex/config", "!/c/appdata/plex/transcoding", "!/c/appdata/plex/transcoding/cache"]
output: ["/c/appdata/plex", "!/c/appdata/plex/transcoding"]
// include elides include-descendant; exclusion elides exclusion-descendant; classes coexist.

input:  ["!/c/appdata/plex/transcoding"]                // orphan exclusion, no included ancestor
output: ["!/c/appdata/plex/transcoding"]                 // preserved — explicit-none carrier (L4); [] stays auto-detect

input:  ["/c/appdata/plex/config", "/c/appdata/plex"]    // whitelist start-state (SELECT-04)
output: ["/c/appdata/plex"]                              // same normalizer, no special case
```

### SetBackupPaths insertion (service.go:3771 shape)
```go
// Source: verified anchor service.go:3771-3792; prefix parse BEFORE translation
for _, hp := range hostPaths {
    bare, excluded := SplitExclusion(strings.TrimSpace(hp))
    if bare == "" && excluded { return fmt.Errorf("empty excluded path") }
    if bare == "" { continue }
    cp, ok := s.toContainerPath(bare)   // unchanged translation + containment
    if !ok { return fmt.Errorf("path %q is not under the host mount and can't be backed up", bare) }
    if excluded { cp = ExclusionPrefix + cp }
    // ... dedupe, then NormalizeSelection(cps) before store.SetBackupPaths
}
```

### Restore mapping (prepareRestoreForTarget insertion)
```go
// Source: verified anchors service.go:5344 (snaps), :5366 (stored), restic.go:485-487 (selector rule)
chosen := resolvedSnapshotFrom(snaps, snapshotID)        // already resolved at :5348-5361
mapped, skipped := mapRestorePaths(tg.AppdataPaths, chosen.Paths)  // NEW pure helper (selection.go-adjacent)
if len(tg.AppdataPaths) > 0 && len(mapped) == 0 {
    return containerRestorePlan{}, errors.New("nothing to restore for this item from this snapshot")
}
// mapped → plan.appdataPaths (snapshot-path form); skipped → plan.skippedPaths (scrubbed at record time)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `paths.Resolve` treated as containment guarantee | Lexical pre-check + `os.Root` enforced containment | `os.Root` shipped in Go 1.24 (repo floor 1.25.0) | Direct-address symlink escape closed |
| Restore replays stored path list fail-fast per path | Longest-prefix mapping against chosen snapshot's `Paths`, skip+abort semantics | This phase | RESTORE-01 |
| `backupPaths` = flat include list only | Same list, `!`-prefixed exclusions expressible | This phase (locked 2026-09-09) | Zero-migration expressiveness |

**Deprecated/outdated:** REQUIREMENTS.md BROWSE-01's "`hasChildren` hint via emptiness probe" parenthetical — superseded by CONTEXT browse Q3 (no probe). Planner should treat BROWSE-01 as "cheap, per-node" only.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Exact cap value 500 (CONTEXT left the number to planning; "ordre ~500" is the only locked part) | R1 | Trivially tunable constant; low risk |
| A2 | `os.Root.Open` + `File.ReadDir(-1)` is sufficient (no `Root.ReadDir` needed) — based on the verified 1.24 API surface; Go 1.25 may have added more methods | R6 | None — `Root.Open`+`ReadDir` is verified-available and portable |
| A3 | Canonical stored order (sorted includes then sorted excludes) is consumer-safe — no reader depends on `SelectedPaths` order (verified: readers build sets or stat-loop; only `ContainerMounts` custom-list order changes, cosmetic) | R3 | Low; would only affect display order |
| A4 | `snapshot.Paths` entries are verbatim positionals for both same-shape and legacy snapshots (restic doc-verified; 0.17-specific confirmation delegated to the L12 contract tests on CI) | R5, R7 | If 0.17 deviated, mapping tests on CI catch it before any release; mapping itself is data-driven so no code change |
| A5 | `Run` record can carry a bounded skip note via the existing `Runs.Finish` error-text parameter (truncateErr-bounded) without schema change | R5 | If the planner prefers log-only skip reporting (CONTEXT Q2's minimum), the deps field shrinks to logging — behavior-safe either way |

## Open Questions (RESOLVED — both adopted verbatim by the phase plans)

1. **Where exactly should `SkippedPaths` surface in the run record?**
   - What we know: CONTEXT Q2 requires "skips signalés dans le résultat du restore"; the run row is written by the orchestrator (`Runs.Start/Finish`), the skips are known in the service.
   - What's unclear: run-error-field note (A5) vs a dedicated display channel later.
   - Recommendation: pass `SkippedPaths` through `RestoreDeps` and append a bounded scrubbed summary to the success run's note text; refine display in Phase 3.
   - **RESOLVED:** adopted as recommended — plan 01-03 Task 2 ("Skip reporting through the orchestrator"): additive `RestoreDeps.SkippedPaths` field + bounded scrubbed skip note on the success run record (byte-identity pin when the field is empty); display refinement stays Phase 3.

2. **Should the empty-selection guard also fire when `selectionSource` is absent but the save comes from a tree-shaped payload?**
   - What we know: CONTEXT Q1 locks the guard to tree-source saves only; legacy clients must keep today's `[]`-clears behavior byte-for-byte.
   - Recommendation: strictly source-gated, as locked. No heuristic sniffing.
   - **RESOLVED:** adopted as recommended — plan 01-04 Task 2 ("PATCH empty-selection guard"): the guard fires only when `selectionSource == "tree"`; legacy and unknown sources keep today's clears-to-auto-detect behavior byte-for-byte, asserted in `TestEmptySelectionGuard`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All build/test | ✓ | go1.25.0 windows/amd64 (`os.Root` floor met) | — |
| restic ≥ 0.17 | `go test` real-restic contract tests (L12); repo-wide suite requirement | ✗ locally | CI: 0.17.3 (lint.yml:51, SHA256-pinned) | Real-restic tests `t.Skip("no restic")` locally (house pattern restic_roundtrip_test.go:20); full coverage on CI |
| golangci-lint | Pre-push gate | not verified this session | CI runs it | `just check` / CI gate |
| Node/npm | NOT needed (no web/ changes planned) | ✓ | v24.16.0 | — |

**Missing dependencies with no fallback:** none blocking — restic's absence only defers two contract tests to CI.
**Missing dependencies with fallback:** restic (above).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `go test` (no testify), `httptest` router harness |
| Config file | none — stdlib convention (`*_test.go` beside code); `just check` runs the Go chain |
| Quick run command | `go test ./internal/api/ -run 'TestSelect|TestBrowse|TestRestore|TestBackupPaths' -count=1` |
| Full suite command | `go test ./...` (restic-dependent tests skip locally; run fully on CI) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| SELECT-01 | Normalization: maximal roots, per-class pruning, `!` round-trip, canonical order | unit (table) | `go test ./internal/api/ -run TestNormalizeSelection -count=1` | ❌ Wave 0 (`selection_test.go`) |
| SELECT-01 | Node classification from stored set: included / partial (stored descendant) / excluded / unchecked | unit | `go test ./internal/api/ -run TestSplitSelection -count=1` | ❌ Wave 0 (`selection_test.go`) |
| SELECT-02 | Zero-migration compatibility: prefix-free legacy list reads as all-includes; `[]` persists and reads as auto-detect; `UpsertTarget` never resets selection | unit + integration | `go test ./internal/store/ ./internal/api/ -run 'TestSetBackupPaths|TestUpsertTarget' -count=1` | ✅ extend existing (service_test.go:2124ff, targets_test.go) |
| SELECT-04 | Whitelist start-state through the same normalizer (pure include list) | unit | `go test ./internal/api/ -run TestNormalizeSelection -count=1` | ❌ Wave 0 (table case) |
| BROWSE-01 | Cheap per-node listing, cap 500 + `truncated`, no hasChildren | handler/contract | `go test ./internal/api/ -run TestBrowse -count=1` | ✅ harness (newTestRouter/newBrowseRouter) + ❌ new cap test |
| BROWSE-02 | `status` classification: empty+`ok` vs `restricted` (chmod-000 fixture) vs `missing` (ENOENT) | handler/contract | `go test ./internal/api/ -run TestBrowseStatus -count=1` | ❌ Wave 0 |
| BROWSE-03 | `os.Root` containment: escaping-symlink fixture rejected (skips on Windows); traversal/absolute responses byte-identical | contract + unit | `go test ./internal/api/ -run 'TestBrowse' -count=1` | ✅ traversal/absolute tests exist (must stay green) + ❌ symlink fixture |
| BROWSE-04 | Hidden-entry delta: default excludes dot-dirs, `hidden=1` includes them, otherwise identical | contract | `go test ./internal/api/ -run TestBrowseHidden -count=1` | ❌ Wave 0 |
| SELECT-01 (wire) | PATCH round-trip through the real router: save mixed list → mounts endpoint returns includes in `Selected` + exclusions in `excluded`, host-form | handler/contract | `go test ./internal/api/ -run TestPatchBackupPaths -count=1` | ❌ Wave 0 |
| INTEG-04 (backend) | Empty-selection guard: tree-source `[]` over prior non-empty → `{ok:false, code:"empty-selection"}`; legacy `[]` unchanged; exclusions-only accepted | handler/contract | `go test ./internal/api/ -run TestEmptySelectionGuard -count=1` | ❌ Wave 0 |
| (criterion 2) | Backup after narrowing hands restic exactly the maximal-root positionals | integration (fake engine) | `go test ./internal/api/ -run TestBackupNarrowed -count=1` — assert `eng.lastPaths` | ❌ Wave 0 (pattern: service_test.go:2140-2146) |
| RESTORE-01 | Restore older snapshot after selection change: descendant clause, longest-prefix ancestor clause, per-path skip, empty-intersection pre-teardown abort, skips recorded | integration (fake engine, seeded `snaps`) | `go test ./internal/api/ -run TestRestoreSelectionChange -count=1` — assert `restored` + error/skip outcomes | ❌ Wave 0 |
| L12 (restore Q4) | restic 0.17: excludes don't drop positionals; absolute `Paths` preservation | contract (real binary) | `go test ./internal/restic/ -run TestPositional -count=1` (skips w/o restic; green on CI) | ❌ Wave 0 (pattern: restic_roundtrip_test.go:19) |
| (argv discipline) | `BackupArgs` multi-path / same-basename-leaves positionals after `--` | unit (argv table) | `go test ./internal/restic/ -run TestBackupArgs -count=1` | ✅ extend restic_args_test.go:294ff |

**Evidence per success criterion:** (1) selection round-trip table + PATCH contract test — read-back reproduces included/partial/excluded for all start states; (2) `eng.lastPaths` assertion equals container-form maximal roots and excludes nothing via `--exclude` (`lastExcludes` unchanged); (3) legacy-list + `[]` store round-trip tests, no migration file added (`git diff internal/store/migrate.go` empty); (4) browse contract tests (cap/truncated, status trio, hidden delta, symlink rejection); (5) restore-selection-change test asserting `restored` contents, skip records, and that the empty-intersection case errors BEFORE any `Docker.Stop`/`Remove` call on the fake.

### Sampling Rate
- **Per task commit:** quick run command above (target < 30 s — fakes only)
- **Per wave merge:** `go test ./...` + `go vet ./...` + `gofmt -l .` (must print nothing) + `golangci-lint run ./...`
- **Phase gate:** full suite green (CI: restic 0.17.3 installed) before `/gsd-verify-work`; `hadolint Dockerfile` unaffected (no Dockerfile change)

### Wave 0 Gaps
- [ ] `internal/api/selection_test.go` — normalization/pruning/classification tables (SELECT-01/04, both root configs)
- [ ] Browse contract additions in `internal/api/handlers_test.go` (or a new `browse_contract_test.go`): cap+truncated, status trio, hidden delta, escaping-symlink fixture (GOOS-guarded) — BROWSE-01..04
- [ ] `internal/api` restore-hardening tests (new file, e.g. `restore_selection_test.go`): the five RESTORE-01 cases, seeded `fakeResticEngine.snaps`
- [ ] Empty-selection guard test: PATCH boundary, coded envelope + legacy-compat cases (INTEG-04 backend)
- [ ] `internal/restic` positional contract test file (real binary, skip pattern)
- No framework/config installs needed — stdlib infra complete

## Security Domain

`security_enforcement: true`, ASVS L1 (config.json). Phase touches: one read endpoint (extended), one write endpoint (body field + guard), restore/write-path logic.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (unchanged) | Existing `authGate` session gate on all /api routes |
| V3 Session Management | no (unchanged) | Existing session cookie machinery |
| V4 Access Control | yes (verify only) | `/api/browse` extension stays behind the SAME mux chain (`csrfGate`/`authGate`) — no new route; PATCH already gated |
| V5 Input Validation | yes | `decodeBody` (1 MiB MaxBytesReader + DisallowUnknownFields, handlers.go:375-377); `paths.Resolve` first reject + `os.Root` enforced containment; per-element validation of `backupPaths` entries (TrimSpace, prefix parse, `toContainerPath` containment, reject bare `"!"`); `selectionSource` value-checked (`"tree"` honored, anything else ignored); `nameParam`/`resourceNameRe` unchanged (handlers.go:585: `^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`) |
| V6 Cryptography | no | No crypto in this phase; restic handles repo encryption |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Symlink escape via direct-address browse (`?path=appdata/link-to-etc`) | Information Disclosure | `os.Root` (kernel-enforced); `paths.Resolve` retained as first reject; escaping-symlink fixture test |
| Arbitrary filesystem probe through the tree | Information Disclosure | Browsable universe = under `HostMountRoot` only; cap+truncation; no recursion; locked Out-of-Scope item ("arbitrary-path browsing") |
| Log injection / host-layout leak via user paths | Information Disclosure | House discipline: no raw user bytes in log format strings (`//nolint:gosec // G706` only with justification); log bare/scrubbed forms; generic error strings on the wire ("could not read directory" carries no path) |
| Error message path disclosure | Information Disclosure | `scrubError`/`scrubSecrets` order (paths → `[path]` FIRST); `status` field carries the error KIND, never the path |
| crafted PATCH storing a path later handed to restic | Tampering | Containment re-checked at save (`toContainerPath`) and restore (`paths.Within`, service.go:5375-5380; orchestrator SEC guard orchestrator.go:702-706); argv positionals strictly after `--` (restic.go:381-382) |
| Selection-derived `--exclude` injection | Tampering | Locked out entirely (L1/L14) — excludes are never derived from selection |
| Empty-selection auto-detect inversion (data-integrity) | Tampering (user intent) | PATCH guard with `code:"empty-selection"` (L10) + explicit-none classification at backup (R4) |
| Huge-payload DoS on listing | Denial of Service | 500-entry cap + `truncated`; `r.Context()` cancellation recommended while touching the handler (PITFALLS #6); no recursion |

## Sources

### Primary (HIGH confidence)
- Codebase, read this session: every anchor in "Verified Integration Anchors" (handlers.go, api.go, service.go, targets.go, orchestrator.go, restic.go, paths.go, excludes_suggest.go, test files, lint.yml, PROJECT.md)
- Context7 `/golang/go` (src/os/root.go, api/go1.24.txt, src/os/dir.go): `os.OpenRoot`/`Root` containment contract verbatim — "Methods on Root can only access files and directories beneath a root directory… symbolic links may not reference a location outside the root. Symbolic links must not be absolute."; `File.ReadDir(n)` semantics; Windows reserved-name caveat; goroutine safety
- Context7 `/restic/restic` (doc/040_backup.rst): verbatim — "excludes do not apply to backup sources explicitly passed as arguments to the command, though content within those directories remains subject to filtering"; "if a directory is excluded, it is impossible to include individual files contained within that directory"; absolute-path snapshot layout (`restic ls` shows `/home/user/work.txt` verbatim) and change-detection reset on path changes

### Secondary (MEDIUM confidence)
- `.planning/research/SUMMARY.md`, `ARCHITECTURE.md`, `PITFALLS.md` — milestone-level positions (all re-verified against source here; restic claims cross-checked above)
- restic 0.17-specific behavior: doc text verified at master; the 0.17 floor is proven by the L12 contract tests on CI (0.17.3, lint.yml:49-61) rather than in-session execution (restic not installed locally)

### Tertiary (LOW confidence)
- None — no claim in this document rests on uncorroborated web search.

## Metadata

**Confidence breakdown:**
- Integration anchors: HIGH — every file:line read this session with verbatim quotes for discrete values
- Locked positions: HIGH — CONTEXT.md (user decisions) + milestone research, restated not re-derived
- Open-item resolutions (cap, shapes, signatures, restore semantics): HIGH for mechanics (anchor-verified), MEDIUM for judgment calls explicitly delegated by CONTEXT (cap value, skip-reporting channel)
- restic/os.Root external facts: HIGH (official sources via Context7; 0.17-specific proof delegated to CI contract tests by locked decision L12)

**Research date:** 2026-09-09
**Valid until:** 2026-10-09 (stable: brownfield code verified at fixed commit; no fast-moving externals)
