# Codebase Structure

**Analysis Date:** 2026-09-09

## Directory Layout

```
bombvault/
├── cmd/bombvault/         # composition root: main.go (wiring), healthcheck, tests
├── internal/
│   ├── api/               # HTTP layer + ALL domain logic (45 files ~28.5k lines + 129 test files)
│   ├── backup/            # per-domain orchestrators, ports & adapters (pure, zero third-party)
│   ├── restic/            # restic argv builders + execution engine (ONLY restic touchpoint)
│   ├── store/             # SQLite settings/state (migrate.go v1..v99, accessor per table)
│   ├── schedule/          # per-domain cron scheduler (robfig/cron), catch-up, due-gates
│   ├── secret/            # AES-256-GCM, Argon2id, session tokens, hand-written TOTP
│   ├── restickey/         # repo password derivation (HMAC-SHA256 from APP_KEY)
│   ├── ageseal/           # age encryption for exports
│   ├── notify/            # webhook/Matrix/SMTP/Apprise/Healthchecks (best-effort)
│   ├── progress/          # context sinks + pub/sub store for SSE percentages
│   ├── dockercli/         # Docker SDK wrapper (interface for DI)
│   ├── virshcli/          # virsh CLI wrapper (VM lifecycle, credential scrubbing)
│   ├── sshconn/           # SSH to libvirt host, key generation, ssh config
│   ├── compose/           # compose labels + topological depends_on ordering (shared)
│   ├── model/             # behavior-free container types crossing the DI seam
│   ├── config/            # env-var config load/validate, DBPath derivation
│   ├── paths/             # path containment under host mount root (ErrTraversal)
│   ├── platform/          # Unraid/TrueNAS/Generic seam + detection
│   ├── template/          # Unraid XML template path rewriting
│   ├── selfrestore/       # staged config restore at boot (before store.Open)
│   ├── spike/             # DI host-integration probes
│   └── releasenotes/      # //go:embed notes/*.md + sync/catalog tests
├── web/                   # React + Vite + TypeScript SPA
│   ├── src/app/           # router.tsx (route table), Layout.tsx (auth gate, shell)
│   ├── src/pages/         # one file per route (PascalCase); settings/ tab cards
│   ├── src/components/    # reusable controls; restore/, recovery/ subfolders
│   ├── src/lib/           # api.ts (only API client), i18n.ts + locales/ (41), hooks, helpers
│   ├── lint-rules/        # custom ESLint plugin (8 bombvault/* house rules)
│   ├── lint-ts/           # TS 6.0.3 side-package for typescript-eslint (DO NOT delete)
│   ├── embed.go           # (at web/ root) //go:embed all:dist → DistFS()
│   └── dist/              # Vite output; only placeholder index.html committed
├── .github/
│   ├── workflows/         # lint.yml, build.yml, docs.yml, dockerhub-description.yml,
│   │                      # repo-watch-instant.yml (kebab-case names)
│   ├── release-notes/     # vX.Y.Z.md per release (v1.0.0 → v8.6.4)
│   └── DOCKERHUB.md       # condensed README for Docker Hub Overview
├── deploy/                # docker-compose.generic.yml (user-facing)
├── templates/             # my-BombVault.xml (Unraid Community Applications)
├── truenas-apps/          # TrueNAS Scale app catalog (app.yaml, questions.yaml, ...)
├── scripts/               # gen_glyphs.py, glyph-paths/, path notes
├── docs/ + overrides/     # MkDocs Material sources (GitHub Pages)
├── go.mod / go.sum        # manifest of record
├── Dockerfile             # 3-stage, digest-pinned, checksum-pinned restic/rclone, tini
├── justfile               # build/test/fmt/check/web/secrets/notes recipes
├── renovate.json / .golangci.yml / .gitleaks.toml / mkdocs.yml
└── CLAUDE.md / README.md / LICENSE (AGPL-3.0-only)
```

## Directory Purposes

**`cmd/bombvault/`:**
- Purpose: the ONLY place that constructs concrete adapters and injects them into `internal/api`; all scheduler wiring.
- Key files: `cmd/bombvault/main.go` (~520 lines; `run()` startup, `healthcheck`, `platformFor`), `main_test.go`, `healthcheck_test.go`.

**`internal/api/`:**
- Purpose: HTTP service + essentially all domain orchestration (backup/restore/retention/off-site/drills/credentials/fleet/receiver).
- Key files: `api.go` (route table), `handlers.go` (4767 lines — handlers + gates), `service.go` (12,026 lines — domain logic), `everything.go`, `foreign.go`, `receiver*.go`, `fleet*.go`/`mesh.go`, `offsite_targets*.go`, `hostshell.go`, `testutil_test.go`.

**`internal/backup/`:**
- Purpose: pure per-domain orchestrators; security-critical guard chain; declares ALL its own ports.
- Key files: `orchestrator.go` (seam — read the package doc + interface block first), `vm_orchestrator.go` (graceful/live/zvol), thin `flash/files/config_orchestrator.go`.

**`internal/restic/`:**
- Purpose: argv builders (pure, table-tested) + engine (exec, JSON parsing, scrubbing, progress).
- Key files: `restic.go` (2118 lines, everything), `restic_args_test.go` (exact-argv pins), `proc_unix.go`/`proc_windows.go` (process-group kill).

**`internal/store/`:**
- Purpose: SQLite settings + state; migrations v1..v99; one accessor file per domain/table.
- Key files: `store.go` (Open), `migrate.go` (migrations slice + Migrate), `settings.go` (Settings + MutateSettings), `runs.go` (StartRun/FinishRun lifecycle), `helpers_test.go` (`OpenMem(t)`).

**`web/src/`:**
- Purpose: entire UI. `lib/api.ts` (~3000 lines) is the only backend contract; `app/router.tsx` the route table; `lib/i18n.ts` + `lib/locales/*.ts` translation tables.
- Key files: `lib/api.ts`, `app/router.tsx`, `app/Layout.tsx`, `lib/pageShell.ts` (PAGE_SHELL), `lib/progress.ts`, `lib/backupWatch.ts`, `index.css` (all design tokens).

## Key File Locations

**Entry Points:**
- `cmd/bombvault/main.go`: binary startup/wiring/shutdown + healthcheck subcommand.
- `web/src/main.tsx`: SPA bootstrap (apply prefs → providers → router).
- `web/embed.go`: `DistFS()` — how the built SPA reaches the Go binary.

**Configuration:**
- `internal/config/config.go`: env loading, `DBPath`.
- `Dockerfile` `ENV` block: runtime defaults; `deploy/docker-compose.generic.yml`: user-facing docs.
- `.golangci.yml`, `.gitleaks.toml`, `renovate.json`, `web/eslint.config.js`, `web/vite.config.ts`, `web/vitest.config.ts`, `web/tsconfig.json`.

**Core Logic:**
- `internal/api/service.go` (domain), `internal/backup/*_orchestrator.go` (orchestration), `internal/restic/restic.go` (engine), `internal/store/migrate.go` (schema), `internal/schedule/schedule.go` (scheduling).

**Testing:**
- Go: co-located `*_test.go` (external) and `*_internal_test.go` (white-box) next to every package; fixtures in `internal/api/testutil_test.go` and `internal/store/helpers_test.go`.
- Web: co-located `*.test.ts` (pure node) and `*.dom.test.tsx` (jsdom) next to sources.

## Naming Conventions

**Files:**
- Go: snake_case (`vm_orchestrator.go`, `offsite_targets_crud.go`); feature-first test names (`everything_singleflight_test.go`, `*_wiring_test.go`, `*_race_internal_test.go`).
- TypeScript: components/pages `PascalCase.tsx`; lib/helpers `camelCase.ts`; hooks `use*.ts`; tests `.test.ts` / `.dom.test.tsx`.
- Workflows: kebab-case. Release notes: exactly `vX.Y.Z.md` in BOTH `.github/release-notes/` and `internal/releasenotes/notes/` (sync test enforced).

**Symbols:**
- Go: exported PascalCase verbs over domain nouns (`UpsertTarget`, `BackupVMGraceful`); unexported lowerCamelCase (`restartStoppedDeps`, `waitHealthy`); deps structs `<X>Deps`; restic tags `<kind>:<stable-identity>`.
- TS: named exports everywhere (default exports ONLY for locale modules and `Recovery.tsx`).

## Where to Add New Code

**New HTTP endpoint (the established path):**
1. Route line in `Handler.Router()` — `internal/api/api.go` (grouped with its domain; comment literal-vs-param ordering).
2. Handler method on `*Handler` — `handlers.go` if it fits an existing domain, else a new `*_handlers.go` file (recent precedent: `fleet_handlers.go`, `receiver_handlers.go`). Respond ONLY via `writeJSON`/`okEnvelope`/`failEnvelope`; validate with `h.nameParam` etc.
3. Domain logic as a `Service` method in `internal/api/service.go` (or a coherent new feature file like `primary_remote.go`).
4. Tests in a new `*_test.go` / `*_internal_test.go`; handler tests through the real router via `newTestRouter` (`internal/api/handlers_test.go`).

**New backup domain (follow flash/files/config as the minimal template):**
1. `internal/backup/<domain>_orchestrator.go`: minimal `XRestic` interface + `XBackupDeps` struct + thin exported `BackupX` doing `Runs.Start` → `Restic.Backup` with the identity tag → `Runs.Finish` (errors via `truncateErr`, `"domain backup: "` prefix).
2. Restore: service-layer (like flash/files) or a full orchestrator with the guard chain (like container/VM).
3. Tests: `internal/backup/<domain>_orchestrator_test.go`, reusing `fakeRuns`; assert the exact tag string and run status on success AND failure.
4. NEVER import `internal/restic`/`dockercli`/`virshcli` here.
5. Wire it: closure in `cmd/bombvault/main.go` `run()` + `Set*Job` call BEFORE `ReloadWithDueChecks`; if everyN cadence, add a `LastRunFunc` due gate.

**New restic subcommand wrapper:**
1. `XyzArgs(repo, ..., m Mode)` in the argv-builders section of `internal/restic/restic.go` — canonical order: `repoFlag` → `storageClassFlags` → `retryLockFlags`/`--no-lock` → subcommand → `insecureFlag` when unencrypted → flags → `--` → positionals.
2. Engine method `(r Restic) Xyz(ctx, ..., m)` in the high-level-operations section via `r.run`.
3. New value-taking GLOBAL flag? Register in `subcommandValueFlags` (`restic.go:1630-1636`) or errors get misparsed.
4. Pin exact argv in `internal/restic/restic_args_test.go` (encrypted + unencrypted + NoLock variants).

**New store table / setting:**
- Table: append migration (next unused number, `CREATE TABLE IF NOT EXISTS`) at the end of `internal/store/migrate.go` + new accessor `internal/store/<name>.go` (template: `fleet_peers.go`).
- Setting: NEW migration `ALTER TABLE settings ADD COLUMN ... NOT NULL DEFAULT <current-behavior>`, then ALL FOUR touchpoints in `internal/store/settings.go` together (struct field, SELECT list, `Scan` args, UPDATE) + the round-trip test.
- Extend `internal/store/migrate_upgrade_internal_test.go` with the new reachable state.

**New React page:**
1. `web/src/pages/Name.tsx` with root `<div className={PAGE_SHELL}>`.
2. Register in `web/src/app/router.tsx`; nav entry in `web/src/components/Sidebar.tsx`.
3. Add `nav.*`/`name.*` keys to `en` AND `de` inline blocks in `web/src/lib/i18n.ts`.
4. The `bombvault/page-uses-page-shell` lint rule and `web/src/app/routedPages.test.ts` enforce this; exceptions go in `web/eslint.config.js`.

**New API call (frontend):** extend `web/src/lib/api.ts` — type mirroring the Go JSON exactly (doc comment per field) + one exported function using `fetchJSON`/`srcParam`.

**New component / hook:** `web/src/components/PascalName.tsx` (feature-scoped in a subfolder like `components/restore/`); hooks `web/src/lib/useName.ts`. Glyphs in `components/glyphs.tsx`.

**New locale:** `web/src/lib/locales/xx.ts` (default-exported `Partial<Translations>`); must pass `i18n.parity/quality/orphans` tests; register in the offered-languages list in `i18n.ts`.

**Utilities:**
- Go shared helpers: only if no adapter imports are needed — precedent is `internal/compose` (pure, stdlib-only, importable by anything).
- Path containment: route user paths through `internal/paths` (`ErrTraversal`).

## Special Directories

**`web/dist/`:**
- Purpose: Vite build output embedded via `web/embed.go`.
- Generated: Yes (Docker stage 1 / `just web`). Committed: only placeholder `web/dist/index.html` via `.gitignore` negation (`!web/dist/index.html`). Do NOT commit hashed assets without an explicit decision; do NOT skip `npm run build` before `go build`.

**`web/lint-ts/` and `web/lint-rules/`:**
- Purpose: TS 6.0.3 side-package for typescript-eslint (TS 7 has no JS API) and the custom house-rule ESLint plugin.
- Generated: No. Committed: Yes. Deleting `lint-ts/` breaks the lint gate.

**`internal/releasenotes/notes/`:**
- Purpose: release notes compiled into the binary ("What's new" dialog).
- Generated: No. Committed: Yes — must be byte-identical to `.github/release-notes/` (embed-sync test).

**`docs/` + `overrides/`:**
- Purpose: MkDocs Material sources for the GitHub Pages site. Generated: No (built in CI). Committed: Yes.

**`scripts/glyph-paths/`:**
- Purpose: generated glyph path data (from `scripts/gen_glyphs.py`). Generated: Yes. Committed: Yes.

---

*Structure analysis: 2026-09-09*
