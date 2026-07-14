# Files Domain Design (Issue #62)

**Goal:** a fourth first-class backup domain `files` so BombVault backs up arbitrary
host paths (cache/array shares, single folders) and replaces a separate Backrest
instance. Requested by ptmorris1, seconded by manilx and BaukeZwart (#62).

**Decision (user, 2026-07-14):** full domain with its own tab and multiple named
path entries, NOT a minimal settings-only path list.

## Data model

- New domain id **`files`** joins the fixed domain set (`containers | vms | flash |
  config`) everywhere the set is enumerated (deploy targets, handlers, notify,
  schedule, dashboard, recovery/discover).
- Settings carry a list of **file sets** (the domain's "items", analogous to
  containers/VMs as items of their domains):
  ```
  FileSet { ID string; Name string; Path string; Excludes []string; Enabled bool }
  ```
  - `Name` is the user-visible label and the restic tag/item key (like a container
    name); `ID` is stable (slug/uuid) so renames do not orphan history.
  - `Path` is an absolute path as seen inside the container (Unraid: under `/mnt/...`).
    Validate existence at save and warn at backup time when missing.
  - `Excludes` are restic `--exclude` patterns, optional, per set.
- Repo layout mirrors the existing domains: local repo under the backup share
  (`<backup>/files`), off-site repo analogous (`offsiteRepoFor("files")`), defs dir
  `<repo>/def` NOT needed (no recreate-definition concept for plain files).

## Backup semantics

- One restic snapshot per enabled file set per run (item granularity like
  containers), tagged so snapshots list per set: reuse the existing item-tagging
  scheme the other domains use.
- The flash orchestrator already backs up a generic directory
  (`internal/backup/flash_orchestrator.go`); generalize or mirror that path for
  file sets (per-set source dir + excludes). No stop/start hooks (files have no
  runtime), no defs.
- Manual per-set "Backup now" + whole-domain backup, wired through the same
  single-flight/async machinery as the other domains (StartBackup family +
  useBackupWatch on the frontend).

## Integration surfaces (parity with existing domains)

- **Plans/Scheduler:** `files` is schedulable exactly like the other domains
  (include-all + per-set include, cadence via the shared CadenceBuilder).
- **Retention:** per-domain retention applies to `files` unchanged.
- **Off-site:** replication of the files repo like the others.
- **Drills:** local subset restore-drill AND off-site subset drill supported; files
  are DR-capable like flash (sandbox restore of a sample is cheap). The new
  off-site-subset dashboard badge (`1c2ea52`) applies automatically.
- **Notify:** files results appear in notifications (domain enum extended).
- **Dashboard:** a `files` protection card row with the same badges/buttons;
  VM-style DR refusal does NOT apply.
- **Recovery/Discover:** files snapshots are discoverable from a bare repo and
  restorable. Restore targets: original path (default) or a user-chosen directory;
  never silently overwrite — restore into the chosen directory as restic does.

## UI

- New **Files tab** alongside the existing per-domain tabs: list of file sets
  (name, path, excludes count, enabled, last backup + status, per-set Backup now),
  add/edit/remove via the standard card/dialog widgets already used by the app.
- Empty state explains the feature in one sentence (Backrest replacement).
- All new strings: key in `web/src/lib/i18n.ts` + real translations in ALL 26
  locale files, key-parity check must pass.

## Non-goals (this release)

- No per-set individual schedule (Plans cover the domain; per-set include flags).
- No recording of `/api/check` runs (unchanged, deliberate).
- No cross-server federation (that is #61's one-time restore, separate spec).

## Docs

- README: feature bullet + section update; CA template `<Overview>` and
  unraid-apps entry after release; note that sources must be mapped into the
  container (Unraid default `/mnt` mapping covers shares).
