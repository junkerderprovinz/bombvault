# Feature Research

**Domain:** Tree-based sub-folder backup selection (backup tool selection UX)
**Researched:** 2026-09-09
**Confidence:** HIGH for core UX patterns (multiple canonical vendor docs + W3C specs cross-checked); LOW for Synology/Backblaze/CrashPlan specifics (sources bot-blocked, noted inline)

## Feature Landscape

### How comparable products do folder selection (competitor scan)

| Product | Selection model | Subfolder granularity | Junk/cache handling | Tri-state parents |
|---------|----------------|----------------------|--------------------|-------------------|
| **Duplicati 2** | File picker in wizard step 3 + text-path box for network shares | Yes — deselect any file/folder in the picker; deselections surface as red-cross exclusions in the Filters list | OS-specific default exclusion lists (temp, pagefile, hibernation) + built-in filter **groups**: `SystemFiles`, `OperatingSystem`, `CacheFiles`, `TemporaryFiles`, `Applications`, `DefaultExcludes` meta-group | Yes — in restore browser, partially-selected folders show a **green square**; live filter-preview icons (green check = folder traversed, children may still be filtered) |
| **Kopia** | One directory = one snapshot source; policies scoped global → user@host → path with visible inheritance | Only via policy ignore rules / `.kopiaignore` files, never a tree | Canonical ignore examples are literally `dist/`, `node_modules/`, `public/`; per-directory `.kopiaignore` picked up during scan | No tree at all |
| **Backrest (restic frontend)** | Backup plan paths = plain text list ("Directories/files to backup") | Only via exclude patterns (example in docs: `*node_modules*`) | Pattern-based only | No tree |
| **Vorta (borg GUI)** | Profile with source folder list (native file dialog multi-select) | Exclude text patterns only | Pattern-based only | No tree |
| **Veeam Agent for Windows** | "The following file system objects" checkbox tree in the Files wizard step | Yes — check a folder, subfolders auto-include; **exclude by unchecking children**. Checking a whole volume shows a volume mark; unchecking any folder downgrades it to a file-level mark that enables filters | Always-excluded list (temp, Recycle Bin, pagefile, hibernate file, VSS metadata, EFS); include/exclude masks where **exclude masks outrank include masks**; whole-volume selection disables file-type filters | Implicitly (folder marks) |
| **restic (engine)** | Path arguments + `--files-from`; **no `--include` flag** | Exclusion only; excluded directories are not traversed; `!` negation cannot re-include inside an excluded dir; **explicitly named sources bypass excludes entirely** | Opt-in `--exclude-caches` (honors `CACHEDIR.TAG`), `--exclude-if-present`, `--exclude-larger-than`; no default excludes | n/a |
| **Unraid CA Backup / Appdata.Backup** | Whole-appdata tar per container; no subfolder selection; plugin now feature-frozen | None | None — see demand proof below | n/a |
| **Synology ABB / Hyper Backup** | Checkbox folder trees in wizards *(specifics unverifiable — KB is JS-shelled; LOW confidence)* | Yes per community usage | NAS junk like `@eaDir` thumbnails is the canonical exclusion example — documented verbatim in Duplicati's filter use cases | Yes per product screenshots (LOW confidence) |

**Demand proof (the strongest single finding):** Unraid's community guide "Plex Backup Fine Tuning" exists *because* subfolder selection is missing. Its recipe: physically **move** `Cache/Transcode/Sync+`, `Cache/PhotoTranscoder`, `Media`, `Metadata` out of the Plex appdata folder, add four extra container bind-mounts so the container still finds them, then hand-write two cron'd scripts (critical DBs daily, volatile media 2×/week). Users are restructuring their containers to work around all-or-nothing appdata backups — exactly the pain BombVault's tree selector removes.

**Gap analysis:** No restic frontend (Backrest, and per its docs none other) offers tree-based folder selection — they all stop at text paths + glob excludes. Duplicati and Veeam prove the tree+tri-state model is the mature UX at the high end. BombVault can be the first restic ecosystem tool with a first-class selection tree while keeping patterns in the (already built) ExcludesEditor.

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = feature feels broken.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Collapsible lazy-loading subtree per mount/root | Appdata trees have thousands of entries; every serious tool (Duplicati, Veeam, Synology) expands on demand. PROJECT.md decision: lazy-load from server listing | MEDIUM | `/api/browse` already exists with traversal guards; needs child-listing per expand, `hasChildren` hints, and pagination for huge dirs. Hand-rolled React component; no UI kit per constraints |
| Checkbox at every level; checking a folder includes everything below | The Veeam/Duplicati/Synology model. "Selecting a folder automatically includes its subfolders; to exclude one, clear its checkbox" (Veeam docs) | LOW | Pure selection-model logic in the client |
| Mixed-state (indeterminate) parent for partial selections | Duplicati's green square; W3C ARIA `aria-checked="mixed"`; native DOM `input.indeterminate` property | LOW | Derive parent state from children; hand-rolled checkbox just sets the DOM property + `aria-checked` |
| State reconstructed from the saved selection on reopen | Users must see what they chose last time; selection is the persistent artifact, the tree is a view | MEDIUM | Read saved flat `backupPaths`, re-derive: node included if it or an ancestor is in the set, excluded sub-branches marked; requires a normalization function (see Dependencies) |
| Selection persists through the existing flat `backupPaths` set | Zero-migration requirement already locked in PROJECT.md; every tool stores a flat effective set (Duplicati: source + filter lists) | MEDIUM | Normalization: store **maximal included paths** (highest ticked node) + exclusion sub-paths below included roots; drop redundant descendants |
| Exclusions visible and reviewable after the fact | Duplicati shows deselected folders in the Filters list; trust in backup tools requires seeing what will NOT be backed up | LOW | Render exclusion sub-paths as a summary row/list near the mount; reuse existing preview styling conventions |
| Keyboard navigation + accessibility | W3C APG treeview pattern: arrows expand/collapse/move, Space toggles, asterisk expands siblings, type-ahead for >7 root nodes; `aria-expanded` on parents only, `aria-level/setsize/posinset` for lazy nodes | MEDIUM | Cost is in the hand-rolled component, not the concept. Without it the feature is keyboard-hostile (today's SPA presumably has baseline a11y) |
| Paths stay inside the mount/root boundary | Security discipline: path traversal guards already surround `/api/browse`; the tree must not become an arbitrary filesystem probe | LOW | Reuse existing guards; never list outside the discovered/custom roots |
| Graceful handling of unreadable/empty dirs | Appdata has permission-denied and empty dirs; a spinner that never ends or a silent collapse is confusing | LOW | Distinguish empty vs error per node; show a muted "no access" row |

### Differentiators (Competitive Advantage)

Features that set BombVault apart. Not expected from a restic frontend, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| First-class tree selection in the restic ecosystem | Backrest/Vorta/resticprofile all stop at text lists; Duplicati/Veeam prove the UX but don't drive restic. Differentiation is "appdata-native, tree-first" | — (is the feature) | Lead the milestone messaging with this |
| One-click junk-folder suggestions (Plex `transcoding`, `Cache`, `node_modules`, `@eaDir`) | The exact pain from issue #189: Plex appdata where transcoding dwarfs config. Duplicati's `CacheFiles`/`DefaultExcludes` groups prove mature tools ship curated lists; Kopia's docs literally use `node_modules/` as the canonical example | MEDIUM | **Suggestions, never silent auto-exclusion** — must be an explicit user confirmation (backup trust rule). PROJECT.md defers auto-*detection* out of this milestone; ship detection as suggestion chips in a follow-up. Cross-check against discovered paths (name + relative-path match under the mount) |
| On-demand per-folder size hints | Users pick what to exclude by size ("transcoding = 212 GB → untick"). No comparable selection UI shows this; Veeam shows only volume sizes | HIGH | `du` walks are expensive on big appdata; must be an explicit "compute" affordance per node (or background job + SQLite cache), never eager on expand. Strong candidate for a later phase, not v1 |
| "Everything except" mode (whitelist start state) | Duplicati codified this: if all rules are includes, everything else is auto-excluded. In a tree this falls out naturally: start unchecked, tick what to keep | LOW | The tree semantics ARE whitelist mode; the work is a mode affordance ("include all by default" vs "include nothing by default") plus normalization storing the maximal included set. Cheap — consider v1 if the selection model supports it |
| Effective-selection preview ("what will actually be backed up") | Duplicati's live icons (green check = traversed, children may still be filtered) prevent surprise; BombVault already has a live-preview precedent in ExcludesEditor | MEDIUM | Show computed effective path list/count for the mount on save; later extend to size. Also guards the restic pitfall: explicit source paths bypass excludes, so the effective set must be exactly the ticked paths |
| `CACHEDIR.TAG` toggle per mount/root | restic ships `--exclude-caches`; exposing it is a one-checkbox standards-based cache exclusion that pattern tools bury in docs | LOW | Argv change in `BackupArgs` + checkbox; pair it with the tree as "the standards-compliant junk exclude" |
| Reuse the same tree component for restore browsing | Duplicati's restore browser is the gold standard: click folder selects it + everything below, tri-state green square, search highlights matches. Restore-side tree over snapshot listings shares ~all component machinery | HIGH | Different milestone — snapshot `ls` data model differs from live FS listing. Keep component state model portable so this is cheap later |

### Anti-Features (Commonly Requested, Often Problematic)

Features to deliberately NOT build.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Auto-excluding "junk" folders silently | Users want zero-config fast backups | Silent non-backup of data is the cardinal sin of backup tools; users discover missing restores at disaster time. Duplicati makes its defaults an explicit, visible list; PROJECT.md already rules out auto-detection for this effort | Suggestion chips the user confirms, with the exclusion visible in the reviewable exclusion list |
| File-type/extension masks inside the tree | Veeam's include/exclude masks live next to its tree, so users ask for the same | Different question (which files vs which folders) — mixing them creates precedence confusion Veeam itself papers over with "exclude masks outrank includes" and "filters disabled on whole volumes". PROJECT.md decision: ExcludesEditor and tree selection are complementary | Keep masks in ExcludesEditor; tree handles folders only. Cross-link the two ("exclude file patterns →" ) |
| Eager size computation on tree expand | "Show me the big folders" | `du` on multi-GB appdata per expand = stalls the panel and hammers the disk; appdata may be on spinning arrays | On-demand "measure this folder" + cached results (see Differentiators) |
| Storing selection as a nested tree JSON in SQLite | It mirrors the UI | Violates the locked decision (flat `backupPaths`, zero migration); two persistence models drift; existing deployments break | Flat maximal-path set; the tree is a pure view |
| Per-subfolder retention or schedules | "Back up databases nightly, media weekly" (real Plex-guide desire) | Fragments restic snapshot identity; retention is per-item tag and must never regroup (#91, #24 discipline); multi-schedule multi-path backups per container multiply stop/restart cycles users must survive | Time-based split belongs at the file-set/container level today; keep one snapshot per item per run |
| Arbitrary-path browsing from the tree (type any host path) | Power users want full-filesystem trees | Defeats boundary validation and error-scrubbing discipline; expands the attack surface of `/api/browse` beyond discovered roots | Custom paths remain an explicit add (existing UX), then appear as tree roots |
| Per-subfolder "last backed up" indicators | Nice-looking, seemingly useful | restic snapshots aren't path-indexed per subfolder cheaply; per-path recency requires scanning snapshot listings per folder — expensive and misleading (a folder "backed up" at snapshot time may have been excluded) | Recency at item level already exists (dashboard/panels); folder recency is a restore-browser feature for a later milestone |
| Virtualized mega-tree showing everything at once | "I want to see the whole tree" | Kills performance for huge appdata and defeats lazy loading; ARIA guidance itself pushes progressive disclosure | Lazy expand + search/filter box; virtualization only if a real tree grows unwieldy |

## Feature Dependencies

```
[Extended /api/browse listing (children, hasChildren, error/empty states)]
    └──requires──> [Lazy tree component (expand/collapse, a11y keyboard)]
                       └──requires──> [Selection model: cascade + mixed state]
                                          └──requires──> [Normalization <-> flat backupPaths]
                                                             └──requires──> [State reconstruction on reopen]
                                                 [Reviewable exclusion list] ──enhances──> [Normalization]

[Junk suggestions] ──requires──> [Selection model]  (operate on tree nodes; not on raw paths)
[Effective-selection preview] ──requires──> [Normalization]  (must compute the exact effective set)
[Whitelist "everything except" mode] ──requires──> [Normalization storing maximal included set]
[CACHEDIR.TAG toggle] ──independent──> (argv-only; ships anytime, pairs well with tree)
[Per-folder size hints] ──requires──> [Lazy tree] + [background du job + cache] ──conflicts──> [eager expand]
[Restore-side tree] ──enhances──> [tree component reuse] ──conflicts──> [this milestone's scope if started now]
```

### Dependency Notes

- **Normalization is the keystone.** Cascade + mixed state + flat persistence all meet in one function: `tick(untick)` over a path set. Get it right first — every table-stakes row above depends on it, and the restic "explicit sources bypass excludes" pitfall makes a wrong effective set a silent backup hole.
- **`/api/browse` extension precedes UI.** Lazy loading needs cheap per-node child listing with `hasChildren` hints (a `Readdirnames(1)`-style emptiness probe) and consistent error vs empty reporting; the UI component cannot be meaningfully built against the current one-shot listing.
- **Suggestions/preview enhance, never gate, v1.** Both require the selection model but nothing else blocks them; they can land in a later phase without rework.
- **Size hints conflict with eager expand.** Any size work must be async/cached or it reintroduces the stall the lazy design exists to prevent.

## MVP Definition

### Launch With (v1)

- [ ] Lazy-loading collapsible tree per mount/root with checkbox per level — the feature itself
- [ ] Cascade semantics: tick includes subtree, unchecking carves exclusions — the universal model (Veeam/Duplicati/Synology)
- [ ] Mixed-state parents (indeterminate, ARIA `mixed`) — table stakes visual contract
- [ ] Normalization to/from flat `backupPaths` (maximal included set + exclusion sub-paths) — zero-migration persistence
- [ ] State reconstruction on reopen + reviewable exclusion list — trust
- [ ] Works in both Container panel and File Sets — same component, both selectors
- [ ] Keyboard navigation + ARIA tree/checkbox roles — APG patterns
- [ ] Empty vs unreadable dir handling, boundary-safe listing — operational reality of appdata

### Add After Validation (v1.x)

- [ ] Junk-folder suggestions (one-click, confirm-first; Plex transcoding/Cache, `node_modules`, `@eaDir` name+path match) — trigger: v1 shipped, users selecting trees manually
- [ ] Effective-selection preview row ("N paths will be backed up for this mount") — trigger: normalization stable
- [ ] Search/filter within the tree — trigger: users report navigation pain on big mounts
- [ ] `CACHEDIR.TAG` exclude toggle — trigger: anytime; small argv change
- [ ] Whitelist start-state mode — trigger: demand for keep-list use cases

### Future Consideration (v2+)

- [ ] Per-folder size hints (on-demand measure + cached) — defer: expensive, needs background-job machinery and cache design
- [ ] Restore-side tree reusing the component over snapshot listings — defer: different data source (snapshot `ls`), big value, separate milestone
- [ ] Auto-detection of junk folders — stays out per PROJECT.md until suggestions prove the pattern

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Lazy tree + cascade + mixed state + flat-set normalization | HIGH | MEDIUM | P1 |
| State reconstruction + reviewable exclusions | HIGH | LOW | P1 |
| A11y keyboard support | MEDIUM (HIGH for long-term quality) | MEDIUM | P1 |
| File Sets parity (same component) | HIGH | LOW (after container panel) | P1 |
| Effective-selection preview | MEDIUM-HIGH | MEDIUM | P2 |
| Junk suggestions (confirm-first) | HIGH (the #189 pain directly) | MEDIUM | P2 |
| Search/filter in tree | MEDIUM | MEDIUM | P2 |
| CACHEDIR.TAG toggle | MEDIUM | LOW | P2 |
| Whitelist start-state mode | MEDIUM | LOW | P3 |
| Per-folder size hints | HIGH | HIGH | P3 |
| Restore-side tree reuse | HIGH | HIGH | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

## Competitor Feature Analysis (what BombVault should do differently)

| Feature | Duplicati | Veeam Agent | restic frontends (Backrest/Vorta) | BombVault plan |
|---------|-----------|-------------|-----------------------------------|----------------|
| Selection surface | File picker + filter lists | Checkbox tree in wizard | Text path lists + glob excludes | Lazy checkbox tree over real mount/root, persisted as flat path set |
| Partial-selection display | Green square (restore), red-cross exclusions (config) | Implicit volume/file marks | n/a | Indeterminate checkbox + reviewable exclusion list |
| Junk handling | Curated filter groups, OS defaults | Always-excluded list | User-written globs (`*node_modules*`) | Confirm-first suggestions tuned to NAS/container junk (transcoding, `@eaDir`, `node_modules`) + optional `CACHEDIR.TAG` |
| Whitelist mode | All-include-rules semantics | Include masks (file-level only) | `--files-from` hand-built | Tree start-state mode backed by maximal-path normalization |
| Preview of effective selection | Live filter icons | Partial (filters vs volumes rules) | None | Effective path count/list on save |
| Engine fidelity risk | Own engine | Own engine | Thin CLI wrapper | Explicit-source-paths-bypass-excludes pitfall handled by construction: effective set = ticked paths |

## Sources

- Duplicati 2 official docs (GUI chapter + Filters appendix, via DuplicatiDocs repo / readthedocs) — HIGH: folder picker, red-cross exclusions, `CacheFiles`/`DefaultExcludes` groups, first-match rules, whitelist semantics, restore tri-state green square, `@eaDir` example
- Veeam Agent for Microsoft Windows official helpcenter (`restore_file.html`, `backup_job_folders.html`, `howto_select_backup_items.html`) — HIGH: checkbox tree, exclude-by-uncheck, volume/file-level marks, include/exclude masks precedence, always-excluded list, mount-point/dedup caveats
- restic official backup docs (restic.readthedocs.io 040_backup.html) — HIGH: exclude pattern semantics, traversal skip, negation limits, explicit sources bypass excludes, `--exclude-caches`
- W3C ARIA Authoring Practices Guide — checkbox pattern (`aria-checked="mixed"`, activation restores remembered partial state) and treeview pattern (roles, `aria-expanded`, dynamic-node attributes, keyboard model) — HIGH
- Kopia official docs via Context7 (`/websites/kopia_io`) — HIGH-MEDIUM: policy ignore rules, `.kopiaignore`, scope inheritance, `node_modules/` as canonical example
- Backrest official getting-started docs — HIGH: text-list paths, glob excludes, no tree (gap confirmation)
- Vorta README (borgbase/vorta) — MEDIUM: profile-based source folder lists, no tree
- Commifreak/unraid-appdata.backup README — MEDIUM: whole-appdata model, feature-frozen state
- Glasti1/unraid-plex-backup-fine-tuning GUIDE.md — HIGH: community workaround (moving transcoding folders out of appdata + hand-written scripts) proving subfolder-selection demand
- Synology ABB/Hyper Backup specifics — LOW (kb.synology.com not fetchable; `@eaDir` pain corroborated via Duplicati docs instead)
- Backblaze/CrashPlan default exclusion lists — LOW (bot-blocked; claims above deliberately avoid citing them)

---
*Feature research for: BombVault tree-based sub-folder backup selection*
*Researched: 2026-09-09*
