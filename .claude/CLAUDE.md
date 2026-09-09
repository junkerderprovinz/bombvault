<!-- GSD:project-start source:PROJECT.md -->

## Project

**BombVault**

Backup and full disaster recovery for Unraid servers (Docker containers, KVM VMs, USB flash, appdata/config), also targeting TrueNAS Scale and generic Docker hosts. A Go backend drives restic as the storage engine behind a React/Vite SPA: discovery, one-click and scheduled backups, guided restores, retention, and off-site replication, shipped as a multi-arch Docker image.

**Core Value:** Every container, VM, and config on the host can be backed up consistently and restored completely — a dead server is rebuilt from the restic repo alone.

### Constraints

- **Tech stack**: Go stdlib `net/http` only (no router/framework); SPA with no state library, no UI kit — the tree UI must be hand-rolled in line with existing components.
- **Storage engine**: restic only; `internal/backup` stays isolated from adapters (ports-and-adapters seam is enforced by review tests).
- **Retention identity**: per-item tags, ungrouped — NEVER reintroduce `--group-by paths` (#91) or global `--group-by tags` (live VM snapshots).
- **SQLite migrations**: append-only numbering; never edit a shipped migration (`:latest` publishes on every push to main).
- **Security discipline**: boundary validation (`resourceNameRe`, 1 MiB bodies, DisallowUnknownFields), error scrubbing (paths → `[path]` first), argv discipline (user-influenced positionals after `--`), credentials via env only.
- **Process**: `go build/vet/gofmt/golangci-lint/test` before every push; web build (`tsc --noEmit && vite build`, commit `web/dist`) required after any `web/` change since the SPA is embedded; async tests must wait for detached goroutines.
- **Releases**: SemVer 3-digit; never tag without explicit approval.

<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->

## Technology Stack

## Languages

- **Go 1.25.0** declared in `go.mod` (`go 1.25.0`, module `github.com/junkerderprovinz/bombvault`) — backend, entrypoint `cmd/bombvault/main.go`. CI (`setup-go` in `.github/workflows/lint.yml`) and the Docker build stage (`golang:1.26-bookworm`) both run **Go 1.26**; the `go` directive is a floor, so this builds, but the manifest lags the toolchain (see CONCERNS.md). To bump Go, change all three places together: `go.mod`, `.github/workflows/lint.yml` `go-version`, and the `Dockerfile` build-stage image + digest.
- **TypeScript ^7.0.2** (native TS 7 compiler, tsgo) — the SPA in `web/`. `npm run build` = `tsc --noEmit && vite build`. typescript-eslint runs against a side-by-side TS 6.0.3 copy in `web/lint-ts/` (TS 7 has no JS compiler API); do not delete `web/lint-ts/`.
- **Node 24** — SPA build stage (`node:24-slim`, digest-pinned in `Dockerfile`); CI web job pins `node-version: '24'`.
- **Python 3.x** — MkDocs docs site only (`.github/workflows/docs.yml` installs `docs-requirements.txt`).

## Runtime

- Single static Go binary (`CGO_ENABLED=0`, enabled by the pure-Go `modernc.org/sqlite` driver). `tini` is PID 1 with `-g` (forwards signals to the whole process group, reaping orphaned restic→rclone grandchildren — issue #35).
- External binaries shipped in the image: **restic 0.17.3** and **rclone 1.74.2**, downloaded from upstream releases with pinned SHA256 `ARG`s (Debian's apt packages are too old). `RESTIC_VERSION`/`RCLONE_VERSION` and their SHA256 args must be bumped in the same change or `sha256sum -c` fails the build.
- The Go test suite requires **restic >= 0.17 on PATH** (`--insecure-no-password`); apt's is too old — CI installs 0.17.3.
- Go modules (`go.mod`, `go.sum`).
- npm for the SPA (`web/package-lock.json` committed).

## Frameworks

- **No web framework** — `net/http` stdlib only. `http.NewServeMux()` with Go 1.22+ method+path patterns, middleware as plain `http.Handler` wrappers, hand-rolled SSE (`internal/api/sse.go`), Prometheus text format written by hand (`internal/api/metrics.go`). No chi/gin/echo/httprouter, no prometheus client dep.
- **React ^19.2.7** + react-dom ^19.2.7 — SPA (`web/src/main.tsx`, StrictMode, `createRoot`).
- **react-router-dom ^7.18.1** — classic `<BrowserRouter>/<Routes>/<Route>` API only (`web/src/app/router.tsx`); data-router/loader APIs are NOT used.
- **Tailwind CSS ^4.3.3** via `@tailwindcss/postcss` + postcss ^8.5.19 — design tokens as CSS variables in `web/src/index.css` (`carbon-*`, `accent*`, `statusOk/Fail/Warn/Neutral`), dark mode via `[data-theme="dark"]` custom variant.
- Go: stdlib `go test` (no testify/ginkgo).
- Web: vitest ^4.1.10, @testing-library/react ^16.3.2, jsdom ^30.0.1.
- Vite ^8.1.5 + @vitejs/plugin-react ^6.0.3 (`web/vite.config.ts`; dev proxy `/api` → `https://localhost:3443`).
- **just** — task runner (`justfile`; `just check` = the pre-push Go chain).
- golangci-lint (`.golangci.yml`, v2 schema: errcheck, govet, staticcheck, ineffassign, unused, gosec), gofmt, go vet, hadolint, gitleaks (`.gitleaks.toml`).
- eslint ^10.8.1 + typescript-eslint ^8.65.0 + eslint-plugin-react-hooks ^7.1.1 + local plugin `bombvault-lint-ts` (`web/lint-rules/`, 8 `bombvault/*` house rules).
- Docker buildx — multi-arch amd64+arm64; Go/web stages cross-compile via `TARGETOS`/`TARGETARCH`, no QEMU for the compile.
- Renovate (`renovate.json`) — weekly; pins Action and Docker base-image digests; groups Go minor/patch.

## Key Dependencies

- `modernc.org/sqlite v1.56.0` — CGO-free SQLite (`internal/store`); what makes the binary fully static and the multi-arch build toolchain-free.
- `github.com/robfig/cron/v3 v3.0.1` — scheduler engine (`internal/schedule`), one cron per domain.
- `github.com/docker/docker v28.5.2+incompatible` + `github.com/docker/go-connections v0.8.1` + `github.com/containerd/errdefs` — Docker SDK over the mounted `docker.sock` (no docker CLI).
- `filippo.io/age v1.3.1` + `golang.org/x/crypto` — export encryption (`internal/ageseal`) and crypto (`internal/secret`: AES-256-GCM, Argon2id, hand-written TOTP).
- `flag-icons` (language switcher), `qrcode-generator` (TOTP QR in `web/src/components/QRCode.tsx`). No UI kit, no react-query/SWR, no Redux/Zustand, no CSS-in-JS.
- restic (external process) — the storage engine; the ONLY code touching it is `internal/restic`.
- rclone — spawned by restic as a grandchild for `rclone:` off-site repos; no direct API use.

## Configuration

- Process config from env vars via `config.LoadFromEnv()` (`internal/config/config.go`): `PORT` (3000), `HTTPS_PORT` (3443), `TZ` (scheduler timezone), `DATA_DIR` (=/config), `APP_KEY`, `PLATFORM`, `HOST_SOURCE_ROOT`/`HOST_MOUNT_ROOT`, `BACKUP_MAX_HOURS`, `HTTP_ONLY`, `LIBVIRT_*`.
- Defaults set in the `Dockerfile` `ENV` block; user-facing docs in `deploy/docker-compose.generic.yml`.
- Database file: `<DataDir>/bombvault.sqlite` (derived at `internal/config/config.go:104`).
- Secrets are NOT env-var-borne at runtime: `APP_KEY` unlocks AES-256-GCM blobs stored in SQLite (see INTEGRATIONS.md).
- `Dockerfile` — 3 stages (web → build → runtime), digest-pinned bases, `# syntax=docker/dockerfile:1@sha256:...` header, `SHELL ... pipefail`, checksum-pinned restic/rclone, `HEALTHCHECK` invoking `bombvault healthcheck` itself, tini entrypoint.
- Version stamping: `-ldflags "-s -w -X ...internal/api.Version=${VERSION}"`; CI computes `v<tag>` for tags and `v<lastTag>+<branch>.<sha>` otherwise (`internal/releasenotes` parses this string).

## Platform Requirements

- Go 1.26, Node 24, restic >= 0.17 on PATH for `go test ./...`, golangci-lint, hadolint, just (recipes use sh — Git Bash on Windows).
- Windows dev caveat: POSIX-only tests (`internal/restic/proc_test.go`, cancel/stream self-exec tests) skip on Windows — a reduced suite runs locally.
- Frontend: `cd web && npm ci && npm run build` before `go build` after any `web/` change (the binary embeds `web/dist`; a stale build embeds the stale SPA).
- Multi-arch (amd64+arm64) Docker image published to GHCR (+ Docker Hub mirror); targets Unraid, TrueNAS Scale, and generic Docker hosts (`internal/platform` detection, `PLATFORM` env override).
- Requires: writable `/config` volume (fail-fast check in `cmd/bombvault/main.go`), Docker socket (root-equivalent — documented warning), optional SSH to the libvirt host (`qemu+ssh://`, key generated on first run).
- restic cache must live under the persistent DataDir (critical for off-site repos, issue #95).

<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->

## Conventions

## Naming Patterns

- Go: snake_case, 1:1 with the domain/table/accessor (`internal/store/fleet_peers.go`); feature-first test names (`everything_singleflight_test.go`, `*_wiring_test.go` for constructor wiring, `*_race_internal_test.go` for concurrency, `*_internal_test.go` for white-box).
- TypeScript: `PascalCase.tsx` for components/pages, `camelCase.ts` for lib/helpers, `use*.ts` for hooks, `.test.ts` / `.dom.test.tsx` for tests.
- Go: exported verbs over domain nouns (`BackupVMGraceful`, `RestoreContainer`, `UpsertTarget`, `LastEverythingPass`); unexported lowerCamelCase verbs (`restartStoppedDeps`, `waitHealthy`, `runVMGraceful`). Orchestrator deps structs: `<X>Deps` / `Backup<X>Deps`. Per-domain restic interfaces: `<X>Restic`.
- TypeScript: named exports everywhere; default exports ONLY for locale modules and `Recovery.tsx`. Standard alias for the translate fn: `type T = ReturnType<typeof useT>["t"]`.
- Go: `ctx` always the first parameter of every function and interface method; `logPrefix` parameters keep helper log lines carrying the CALLER's method name.
- IDs are 32-hex crypto-random strings (`newID()`, `internal/store/repo.go:31`); timestamps Unix seconds; booleans INTEGER in SQL, `bool` in Go (converted at the scan boundary only).
- restic tags are always `<kind>:<stable-identity>` (`container:plex`, `vm:win10:zvol:vdb`).
- TS wire types mirror the Go JSON shape field-for-field (camelCase) with a doc comment per field, kept in sync by hand (`web/src/lib/api.ts`).

## Code Style

- `gofmt` — `gofmt -l .` must print nothing (CI + pre-push hook fail on output; `just fmt` fixes).
- TypeScript: `strict` + `noUnusedLocals` + `noUnusedParameters` (`web/tsconfig.json`); Tailwind utility classes only, built on semantic tokens from `web/src/index.css` (`carbon-*`, `accent*`, `status*`) — never raw hex or hard-coded radii on controls; shared controls carry the `glim-*` engine classes.
- Go: golangci-lint (`.golangci.yml`, v2) — errcheck, govet, staticcheck, ineffassign, unused, **gosec**. Exclusions: `G304|G306` in `internal/(template|paths)/`, staticcheck SA5011 in `_test.go`. Every `nolint` carries a justification comment (`//nolint:gosec // G204` — "argv is constructed by typed builders; no user input reaches here").
- Web: eslint flat config (`web/eslint.config.js`) + 8 custom `bombvault/*` rules from `web/lint-rules/` (icon badges need tooltips, one badge size, no status color on controls, controls read engine tokens, user text is translated, no em dashes in user text, pages use PAGE_SHELL) — each rule itself tested in `web/src/lib/uiConventions.test.ts`. When a rule blocks legitimate work, declare an exception in `web/eslint.config.js`, never a disable comment.
- `hadolint Dockerfile` and gitleaks (narrow allowlist in `.gitleaks.toml`) gate pushes.

## Import Organization

- `@/*` → `./src/*` exists in `web/tsconfig.json` but the codebase overwhelmingly uses relative imports (`../lib/api`) — follow the relative style.

## Error Handling

- Wrap, never mask: `fmt.Errorf("Context: %w", err)` with domain prefixes that identify the failing path from the log line alone (`"backup: "`, `"vm live backup: "`, `"zvol restore: "`; store wraps with the method name).
- Sentinel errors + `errors.Is` for classified outcomes (`errDomainBusy`, `ErrNotConfirmed`, `ErrRestoreConflict`, `ErrBackupSourceUnreadable`, `ErrRestoreMetadataOnly`); message-carrying wrappers implement `Is(target)`.
- Errors crossing to the UI/runs are ALWAYS scrubbed and capped: `scrubError`/`scrubSecrets` (paths→`[path]` first, then credentials→`[redacted]@`; order is load-bearing), `truncateErr` (500 chars, runs), ≤300-char restic reasons.
- Cancellation: remap exec `*ExitError` via `ctx.Err()`; `"cancelled"` is distinct from `"failed"`.
- Best-effort steps log via `log.Printf` and continue; fatal steps return; `Runs.Finish` errors ignored with `_ =` on the failure path (original error wins).
- `RowsAffected` checked where zero-rows matters (`FinishRun`); best-effort rollbacks `tx.Rollback() //nolint:errcheck,gosec` with the original error taking priority.

## Validation

- At the handler boundary, always: `h.nameParam` (`resourceNameRe`: alnum + `._-`, no `..`, ≤128), `h.vmNameParam`, `validRunID` (32 hex); batch bodies re-validate every element; `decodeBody` enforces 1 MiB `MaxBytesReader` + `DisallowUnknownFields` + JSON-only + cross-site refusal. Never trust a name past the handler. User paths go through `internal/paths` (`ErrTraversal`).

## Settings & Data Access

- ALL settings writes via `h.store.MutateSettings(func(s *store.Settings) error {...})` — mutation fns must be pure (no DB calls inside; deadlocks on the single pooled connection). `UpdateSettings` is full-row REPLACE and production-forbidden (source-scan guard test).
- Adding a setting touches FOUR positional lists together (struct, SELECT, Scan, UPDATE in `internal/store/settings.go`) + the round-trip test.

## Logging

- Full restic stderr is logged server-side only; scrubbed, bounded reasons go to callers/UI.
- Scheduler timezone is logged loudly at boot (`logSchedulerTimezone`).

## Comments

- Comments are load-bearing "why" documents — the signature house convention across Go AND TS. Every non-obvious choice carries a paragraph citing issue numbers (`#91`, `#152`), decision references (`[375]`), and the reason a simpler alternative was rejected (e.g. the SIGTERM-not-SIGKILL comment at `internal/restic/proc_unix.go:26-41`, the `LsStream` "do not simplify" comment). A tricky choice without a paragraph is off-style.
- Intentionally-unwired fields say so explicitly (`VMBackupDeps.TPMPath` ⚠ comment).
- Web: long narrative block comments at the top of every nontrivial file; exceptions to rules are written down at the site (model: `web/src/lib/pageShell.ts`).
- SQL migration etiquette comments: `internal/store/migrate.go:50-67` (NUMBERING HAZARD). Preserve all of these when editing; they are the memory that prevents regressions.

## Function Design

## Module Design

## TypeScript-Specific

- Async jobs: POSTs return `{ok:true,started:true}`; outcomes NEVER read from the POST response — use `useBackupWatch` (`web/src/lib/backupWatch.ts`) correlating new run by id (baseline ids before firing, never client clock); bulk loops use `fireAndWaitRun`.
- Secrets in forms: write-only contract (GET returns `""` + `*Set` flag; blank on save keeps stored value; Clear flag/DELETE removes).
- Every user-visible string through `t()` from `useT()` (lint-enforced); em dashes banned in user text (lint-enforced, non-configurable); backend error text shown verbatim.
- Hook dependency workaround: `xRef.current = x` ref-mirroring when a callback must read fresh state (`useBackupWatch`/`progress.ts` pattern) — `exhaustive-deps` is warn-only.
- Status colors belong on Badges/chips, never on interactive controls; page roots use `PAGE_SHELL` (`web/src/lib/pageShell.ts`).

<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->

## Architecture

## System Overview

```text

```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Composition root | Constructs all adapters, wires scheduler + server, startup/shutdown order | `cmd/bombvault/main.go` |
| HTTP layer | Routing, middleware (CSRF/auth), JSON envelope, validation at boundary | `internal/api/api.go`, `internal/api/handlers.go`, `internal/api/server.go`, `internal/api/spa.go` |
| Domain service | All domain orchestration: backup/restore starts, retention, off-site, drills, credentials, fleet, receiver | `internal/api/service.go` + feature files (`everything.go`, `foreign.go`, `receiver*.go`, `fleet*.go`, `offsite_targets*.go`) |
| Backup orchestrators | Pure per-domain orchestration (stop→snapshot→restart, restore guard chain); zero third-party/adapter imports | `internal/backup/orchestrator.go`, `internal/backup/vm_orchestrator.go`, `flash/files/config_orchestrator.go` |
| restic engine | Only code invoking the restic binary: argv builders + exec + JSON parsing + scrubbing | `internal/restic/restic.go` |
| Scheduler | One cron schedule per domain, catch-up, everyN due-gates, after-bulk batching | `internal/schedule/schedule.go`, wired from `cmd/bombvault/main.go` |
| Persistence | SQLite settings/state; the settings read/write contract | `internal/store/store.go`, `settings.go`, `migrate.go`, per-table accessor files |
| SPA | Entire UI; only API client is `web/src/lib/api.ts` | `web/src/` |

## Pattern Overview

- `cmd/bombvault` is the ONLY composition root; everything internal is interface/function-seam driven; nothing imports `cmd`.
- `internal/backup` imports ONLY its own declared interfaces (`orchestrator.go:1-16` package doc) — never `internal/restic`/`dockercli`/`virshcli` — making it fully unit-testable with fakes.
- Adapters bridge concrete packages to orchestrator ports at the service layer (`resticAdapter`, `templatesAdapter`, `runsAdapter`, `sshZFSHost`, `resticZvolAdapter` — `service.go:6737-6965`).
- Always-undo guarantee: backups restart what they stopped even on error (deliberately-ordered defers, `orchestrator.go:386-464`).
- Identity-stable retention: per-item tags, ungrouped (`--group-by ""`). NEVER reintroduce `--group-by paths` (#91); NEVER `--group-by tags` globally (live VM snapshots carry an extra `live` tag).

## Layers

- Purpose: entire UI; client-routed, no SSR.
- Location: `web/src/` (`pages/` → `components/` → `lib/`; nothing imports upward from `lib`).
- Depends on: same-origin `/api` only (`web/src/lib/api.ts`).
- Purpose: routing, auth/CSRF gates, JSON envelope, boundary validation.
- Location: `internal/api/api.go` (route table `Router()`, `api.go:113-364`), `handlers.go`, `server.go`, `spa.go`, `sse.go`.
- Depends on: Service, store, progress, all engine interfaces.
- Purpose: every domain operation. Per-domain `repoMu` locks + `domainActivity` names, two single-flight guards (`batchActive`, `everythingActive`), two cancel maps (cancel backup safe / cancel restore destructive — `BeginShutdown` cancels only backups).
- Location: `internal/api/service.go` (~369 methods) + feature files.
- Depends on: backup orchestrator ports, store, restic engine, notify, progress, platform.
- Purpose: pure domain logic with DI deps structs; no receivers, no globals (only test-knob vars).
- Location: `internal/backup/`.
- Depends on: its own interfaces + `internal/compose` (topological restart order) + `internal/model`.
- Purpose: subprocess/SDK execution. `internal/restic` (builders + engine), `internal/dockercli`, `internal/virshcli`, `internal/sshconn`.
- Depends on: external binaries/sockets only.
- Purpose: settings + state. One `*store.Repo`, one accessor file per domain/table.
- Location: `internal/store/`.

## Data Flow

### Primary Request Path (async backup start, e.g. `POST /api/containers/{name}/backup`)

### Restore Path (the guard chain)

### Scheduler Path

- Server: SQLite is the single source of truth; orchestrators hold no state (all in deps structs); only package-level mutable state is `restic.maxProcs` atomic and two test-knob vars in `internal/backup`.
- SPA: no state library — Context providers (`web/src/main.tsx`), module-level singleton with subscriber set (`web/src/lib/progress.ts`), window CustomEvents (`bv:settings-changed`, display-prefs adopt), localStorage `bv-*` keys.

## Key Abstractions

- Purpose: the storage-engine seam between the service and the restic binary.
- Examples: defined `internal/api/service.go:64-159` (28 methods); real `*restic.Restic` (compile-time check at `service.go:162`); fake `fakeResticEngine` in `internal/api/service_test.go`.
- Pattern: constructor injection; new engine methods require touching this interface.
- Purpose: mockable host control (`Docker`, `Restic`, `Templates`, `Runs`, `VM`, `ZFSHost`, `ZvolRestic`, per-domain `*Restic`).
- Examples: `internal/backup/orchestrator.go:62-132`, `vm_orchestrator.go:60-895`.
- Pattern: deps structs (`BackupDeps`, `VMBackupDeps`) bundle interfaces + config; every orchestrator is an exported function `(ctx, XDeps)`.
- Purpose: security/behavior context per operation — encryption, lock policy (`--retry-lock 5m` vs `--no-lock`), ambient credentials (`NoAmbientCreds` for foreign sessions), storage class, bandwidth limits.
- Examples: `internal/restic/restic.go:25-265`; threads through every builder and method.
- Purpose: the ONLY sanctioned partial write to the settings row (read+mutate+write in one transaction under `settingsMu`; no-op detection).
- Examples: `internal/store/settings.go:454`; enforced by source-scan test `internal/store/settings_writers_test.go`.
- Pattern: mutation fns must be PURE — no DB access inside (deadlocks on the one pooled connection).
- Purpose: retention identity per item. `container:<ref>`+`p1`, `vm:<name>`+`p2` (+`live` for live snapshots, +`vmrun:<runID>` correlation), `flash`, `fileset:<name>`, `config`.
- Examples: `internal/backup/orchestrator.go:378`, `vm_orchestrator.go:461`, `flash_orchestrator.go:39`, `files_orchestrator.go:44`, `config_orchestrator.go:33`.
- Purpose: live percentages from restic to the SPA. Context-carried `progress.Sink`/`CopySink` (`internal/progress/progress.go`) + pub/sub `Store`; `RESTIC_PROGRESS_FPS=3` + streaming exec when a sink is in ctx (`internal/restic/restic.go:924-944`); SSE endpoint `GET /api/progress`; SPA ref-counts one EventSource (`web/src/lib/progress.ts`).

## Entry Points

- Triggers: container start; `bombvault healthcheck` subcommand (Docker HEALTHCHECK).
- Responsibilities: `run()` startup order is load-bearing — log→config→`ensureDataDirWritable`→`selfrestore.ApplyPending` (BEFORE store open; the only safe moment to swap the WAL-held DB)→`store.Open/Migrate/New` + `ReapInterruptedRuns`→docker→sshconn+virsh (non-fatal)→restic engine→`api.NewService`+Setters→rclone conf→scheduler wiring (ALL `Set*` calls BEFORE `ReloadWithDueChecks`/`Start`; `restic.SetMaxProcs` BEFORE any restic child)→background goroutines→`api.NewServer(cfg, web.DistFS(), handler.Router())`.
- Shutdown: stop serving FIRST, then `svc.BeginShutdown()`; a second Ctrl-C kills immediately (`signal.NotifyContext` stop after first signal).
- Location: `internal/api/server.go`; SPA + `/api` mux, self-signed ECDSA cert (`EnsureSelfSigned`) unless `HTTP_ONLY`.

## Architectural Constraints

- **Threading:** single process; detached goroutines per run with `recoverOperation` panic capture (`service.go:4735`); per-domain mutexes; SQLite pooled to ONE connection (`internal/store/store.go`) — any store call inside a `MutateSettings` fn deadlocks.
- **Global state:** `restic.maxProcs` atomic (`internal/restic/restic.go:878-891`); `progress.Store` singleton; SPA module singletons (`progress.ts`, display-prefs). Nothing else.
- **Process discipline:** restic children run in their own process group (`Setpgid`) and get SIGTERM-to-group (never SIGKILL — restic treats SIGTERM as clean-abort, no half-written snapshots; `internal/restic/proc_unix.go:26-41`). Cancelled exec surfaces as `*ExitError`; remap via `ctx.Err()` (`ctxCancelErr`, `restic.go:957-962`).
- **argv discipline:** global flags before the subcommand; user-influenced positionals always after `--`; credentials only via env.
- **Embedding:** `//go:embed all:dist` must live at `web/` root (`web/embed.go` — Go embed cannot reference `..`); `web/dist/index.html` placeholder is committed via `.gitignore` negation so `go build` works pre-Vite.
- **Async tests:** tests must WAIT for detached goroutines (Linux CI flakes otherwise — repo rule).

## Anti-Patterns

### "Fixing" the HTTP-200 envelope piecemeal

### Collapsing `LsStream` into `Ls`

### Bypassing a DI seam

### Editing an applied migration body or renumbering

### Disabling a web lint rule instead of declaring an exception

## Error Handling

- JSON envelope: `writeJSON` with `okEnvelope`/`failEnvelope` (`handlers.go:39`); most domain failures = HTTP 200 `{ok:false, error:<scrubbed>}`.
- Scrubbing: `scrubError`/`scrubSecrets` (`handlers.go:65-131`) — paths→`[path]` FIRST, then `user:pass@`→`[redacted]@` (order is load-bearing). Deliberate twins in `internal/restic/restic.go:1443` and `internal/virshcli`; `internal/backup` duplicates the regexes ON PURPOSE to preserve its seam.
- Wrapping: `fmt.Errorf("domain: step: %w", err)` with domain prefixes (`backup: `, `vm live backup: `, `zvol restore: `, …) so logs identify the failing path; store wraps with the method name as context.
- Persistence: errors written to `runs.error` go through `truncateErr` (scrub + 500-char cap); restic returns ≤300-char scrubbed reasons; per-item causes deduped/bounded (`lastReason`).
- Sentinels: `errDomainBusy`, `errOffsiteAppendOnly`, `ErrBackupPathNotMounted`, `ErrRestoreMetadataOnly` (Unraid FUSE metadata-only restore = success-with-warning), `ErrBackupSourceUnreadable` (restic backup exit 3), `ErrEverythingInFlight`; message-carrying wrappers implement `Is(target)`.
- Cancellation: `restoreOutcome` maps `context.Canceled` → `"cancelled"` (distinct from `"failed"`, no failure alert); async-job SPA states treat cancelled/skipped as NEUTRAL terminals.

## Cross-Cutting Concerns

<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->

## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:

- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->

## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
