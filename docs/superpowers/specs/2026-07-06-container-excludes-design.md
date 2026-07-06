# Per-container exclude patterns (#36) — design

**Issue #36 (strese):** back up a container volume but exclude subdirectories (e.g. Plex `/config` minus
`.../Plex Media Server/{Cache,Media,Metadata}`), passed to restic as `--exclude`. A one-pattern-per-line
textarea per container.

**Scope (owner-confirmed):** **containers only.** VMs back up opaque disk-image files (restic never walks the
guest FS, so a subdir exclude is meaningless) and config is BombVault's own recovery bundle — both excluded
from this feature. The variadic `excludes ...string` plumbing already spans all domains, so VMs/config/flash
can adopt it later trivially, but we build **no** VM/config exclude UI now.

## The one hard problem: what restic actually stores

restic records the **absolute source paths BombVault hands it**, which are the container's host-mount view,
rebased from `HostSourceRoot`→`HostMountRoot` by `toContainerPath` (e.g. host `/mnt/user/appdata/plex/…` →
stored `/host/user/user/appdata/plex/…`, note the doubled `user`). So a user-typed **container** path like
`/config/Library/…/Cache` appears in **no** stored path and a raw `--exclude /config/…` matches nothing.

restic `--exclude` semantics: matched against each stored absolute path; a pattern with **no slash** matches a
basename at **any depth** (this is why the flash `.git` exclude works); a pattern **with** a slash is matched
against the path; a leading `/` anchors it. `filepath.Match` globs: `*` within one segment, `**` across `/`.
Brace lists `{a,b}` are **not** supported.

## Path semantics (owner-confirmed): forgiving hybrid, translate at backup time

For each raw line (source of truth = the human-readable text the user typed, stored verbatim per target):

1. **No slash** (e.g. `.git`, `*.tmp`) → pass through verbatim (restic basename-at-any-depth). status `basename`.
2. **Starts with a container mount `Destination`** (longest match; e.g. `/config`) → take that mount's `Source`
   + the remainder, run through `toContainerPath` → the exact anchored `/host/user/…` path restic stored →
   emit as `--exclude`. status `translated`.
3. **Otherwise** (a slash but no mount prefix, or untranslatable) → pass through verbatim (advanced/host/glob
   users), never silently dropped. status `passthrough`.

Translation happens at **backup time** (re-resolved every run from the live inspect), so patterns survive
container recreation/remounts and stay readable in the DB and UI. The raw text round-trips in the textarea.

## No-match feedback (owner-confirmed): resolved-pattern preview + warning

A stateless preview endpoint resolves candidate lines against the live container inspect and returns, per line,
`{raw, resolved, status, matches}`:
- `basename` → `matches=true` (applies at any depth).
- `translated` → `matches` = the resolved path is under one of the container's actually-backed-up volumes
  (`effective`); a translated path under a volume that isn't selected for backup → `matches=false` + warning.
- `passthrough` → `matches=false` + a soft note "passed to restic as-is (not a recognized container path)".

The editor shows each line's resolved `--exclude` and flags lines that would exclude nothing, so a user can
trust the exclude took effect (guards the "silent no-match → false confidence" failure).

## Architecture

Reuses the existing exclude plumbing end-to-end; only the container path feeds it now.

- **Store** (`internal/store/targets.go`, `migrate.go`): new `Target.Excludes []string`, migration v53
  `ALTER TABLE targets ADD COLUMN excludes TEXT NOT NULL DEFAULT '[]'`, wired into UpsertTarget INSERT +
  scanTarget + both SELECTs, **omitted** from `ON CONFLICT DO UPDATE` (setter-owned, like SelectedPaths/
  StopContainers). New `SetExcludes` repo setter (mirror `SetStopContainers`).
- **Service** (`internal/api/service.go`): `SetExcludes` (trim/dedupe/drop-blank); `resolveExcludeLine` +
  `resolveExcludePatterns(raw, in)` (the hybrid translation) feeding `BackupDeps.Excludes` at the container
  backup; `previewExcludes(raw, in, effective)` for the endpoint.
- **Orchestrator** (`internal/backup/orchestrator.go`): `BackupDeps.Excludes []string`; pass `d.Excludes...`
  into `d.Restic.Backup` at the container backup call (~:364). No change below the interface.
- **API** (`internal/api/handlers.go`, `api.go`): PATCH container body gains `Excludes *[]string`; `containerView`
  gains `Excludes []string`; new `POST /api/containers/{name}/excludes/preview` → `[]ExcludePreview`.
- **Frontend** (`web/src/pages/Containers.tsx`, `lib/api.ts`): `ExcludesEditor` (clone `StopContainersEditor`)
  with a debounced preview call rendering resolved patterns + no-match warnings; `setContainerExcludes` +
  `previewExcludes` clients; `excludes: string[]` on the `Container` type.
- **i18n**: new `excludes.*` keys inline en+de + all 24 `locales/*.ts` in one pass.
- **Tests**: store round-trip + Upsert-no-clobber; service translation (`/config/…` → `/host/user/user/…`
  incl. doubled `user`, basename passthrough, no-match); orchestrator/argv test that container excludes reach
  `--exclude`.

## Risks / notes
- Brace `{a,b}` unsupported → help text says "one line each".
- Exact-prefix fragility → always route through `toContainerPath` (never hand-build the anchored path).
- Changing excludes only affects **future** snapshots; already-captured data stays until prune.
- Upsert-clobber trap → keep `excludes` out of `ON CONFLICT DO UPDATE SET`.
