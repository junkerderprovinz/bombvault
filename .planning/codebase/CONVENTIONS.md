# Coding Conventions

**Analysis Date:** 2026-09-09

## Naming Patterns

**Files:**
- Go: snake_case, 1:1 with the domain/table/accessor (`internal/store/fleet_peers.go`); feature-first test names (`everything_singleflight_test.go`, `*_wiring_test.go` for constructor wiring, `*_race_internal_test.go` for concurrency, `*_internal_test.go` for white-box).
- TypeScript: `PascalCase.tsx` for components/pages, `camelCase.ts` for lib/helpers, `use*.ts` for hooks, `.test.ts` / `.dom.test.tsx` for tests.

**Functions:**
- Go: exported verbs over domain nouns (`BackupVMGraceful`, `RestoreContainer`, `UpsertTarget`, `LastEverythingPass`); unexported lowerCamelCase verbs (`restartStoppedDeps`, `waitHealthy`, `runVMGraceful`). Orchestrator deps structs: `<X>Deps` / `Backup<X>Deps`. Per-domain restic interfaces: `<X>Restic`.
- TypeScript: named exports everywhere; default exports ONLY for locale modules and `Recovery.tsx`. Standard alias for the translate fn: `type T = ReturnType<typeof useT>["t"]`.

**Variables:**
- Go: `ctx` always the first parameter of every function and interface method; `logPrefix` parameters keep helper log lines carrying the CALLER's method name.
- IDs are 32-hex crypto-random strings (`newID()`, `internal/store/repo.go:31`); timestamps Unix seconds; booleans INTEGER in SQL, `bool` in Go (converted at the scan boundary only).

**Types:**
- restic tags are always `<kind>:<stable-identity>` (`container:plex`, `vm:win10:zvol:vdb`).
- TS wire types mirror the Go JSON shape field-for-field (camelCase) with a doc comment per field, kept in sync by hand (`web/src/lib/api.ts`).

## Code Style

**Formatting:**
- `gofmt` — `gofmt -l .` must print nothing (CI + pre-push hook fail on output; `just fmt` fixes).
- TypeScript: `strict` + `noUnusedLocals` + `noUnusedParameters` (`web/tsconfig.json`); Tailwind utility classes only, built on semantic tokens from `web/src/index.css` (`carbon-*`, `accent*`, `status*`) — never raw hex or hard-coded radii on controls; shared controls carry the `glim-*` engine classes.

**Linting:**
- Go: golangci-lint (`.golangci.yml`, v2) — errcheck, govet, staticcheck, ineffassign, unused, **gosec**. Exclusions: `G304|G306` in `internal/(template|paths)/`, staticcheck SA5011 in `_test.go`. Every `nolint` carries a justification comment (`//nolint:gosec // G204` — "argv is constructed by typed builders; no user input reaches here").
- Web: eslint flat config (`web/eslint.config.js`) + 8 custom `bombvault/*` rules from `web/lint-rules/` (icon badges need tooltips, one badge size, no status color on controls, controls read engine tokens, user text is translated, no em dashes in user text, pages use PAGE_SHELL) — each rule itself tested in `web/src/lib/uiConventions.test.ts`. When a rule blocks legitimate work, declare an exception in `web/eslint.config.js`, never a disable comment.
- `hadolint Dockerfile` and gitleaks (narrow allowlist in `.gitleaks.toml`) gate pushes.

## Import Organization

**Order (Go):**
1. Stdlib
2. Sibling `internal/` packages
3. Third-party (rare — `internal/api` and `internal/backup` use stdlib + internal only; `internal/backup` imports ZERO third-party and ZERO concrete adapters, by documented rule)

**Path Aliases:**
- `@/*` → `./src/*` exists in `web/tsconfig.json` but the codebase overwhelmingly uses relative imports (`../lib/api`) — follow the relative style.

## Error Handling

**Patterns:**
- Wrap, never mask: `fmt.Errorf("Context: %w", err)` with domain prefixes that identify the failing path from the log line alone (`"backup: "`, `"vm live backup: "`, `"zvol restore: "`; store wraps with the method name).
- Sentinel errors + `errors.Is` for classified outcomes (`errDomainBusy`, `ErrNotConfirmed`, `ErrRestoreConflict`, `ErrBackupSourceUnreadable`, `ErrRestoreMetadataOnly`); message-carrying wrappers implement `Is(target)`.
- Errors crossing to the UI/runs are ALWAYS scrubbed and capped: `scrubError`/`scrubSecrets` (paths→`[path]` first, then credentials→`[redacted]@`; order is load-bearing), `truncateErr` (500 chars, runs), ≤300-char restic reasons.
- Cancellation: remap exec `*ExitError` via `ctx.Err()`; `"cancelled"` is distinct from `"failed"`.
- Best-effort steps log via `log.Printf` and continue; fatal steps return; `Runs.Finish` errors ignored with `_ =` on the failure path (original error wins).
- `RowsAffected` checked where zero-rows matters (`FinishRun`); best-effort rollbacks `tx.Rollback() //nolint:errcheck,gosec` with the original error taking priority.

**API envelope (mandatory):** every endpoint responds through `writeJSON` with `okEnvelope(extra)` or `failEnvelope(err)`. Most domain failures = HTTP 200 `{ok:false, error:<scrubbed>}`; real status codes only for transport-level conditions (400 bad path param, 401, 403 secrets/cross-site, 409 busy, 415 non-JSON, 429 throttle, 503 store down). The SPA checks `res.ok` and surfaces `res.error` via `loadErrorMessage` (`web/src/lib/errors.ts`).

## Validation

- At the handler boundary, always: `h.nameParam` (`resourceNameRe`: alnum + `._-`, no `..`, ≤128), `h.vmNameParam`, `validRunID` (32 hex); batch bodies re-validate every element; `decodeBody` enforces 1 MiB `MaxBytesReader` + `DisallowUnknownFields` + JSON-only + cross-site refusal. Never trust a name past the handler. User paths go through `internal/paths` (`ErrTraversal`).

## Settings & Data Access

- ALL settings writes via `h.store.MutateSettings(func(s *store.Settings) error {...})` — mutation fns must be pure (no DB calls inside; deadlocks on the single pooled connection). `UpdateSettings` is full-row REPLACE and production-forbidden (source-scan guard test).
- Adding a setting touches FOUR positional lists together (struct, SELECT, Scan, UPDATE in `internal/store/settings.go`) + the round-trip test.

## Logging

**Framework:** stdlib `log` to stdout (single stream, `cmd/bombvault/main.go`). No structured logging library.

**Patterns:**
- Full restic stderr is logged server-side only; scrubbed, bounded reasons go to callers/UI.
- Scheduler timezone is logged loudly at boot (`logSchedulerTimezone`).

## Comments

**When to Comment:**
- Comments are load-bearing "why" documents — the signature house convention across Go AND TS. Every non-obvious choice carries a paragraph citing issue numbers (`#91`, `#152`), decision references (`[375]`), and the reason a simpler alternative was rejected (e.g. the SIGTERM-not-SIGKILL comment at `internal/restic/proc_unix.go:26-41`, the `LsStream` "do not simplify" comment). A tricky choice without a paragraph is off-style.
- Intentionally-unwired fields say so explicitly (`VMBackupDeps.TPMPath` ⚠ comment).
- Web: long narrative block comments at the top of every nontrivial file; exceptions to rules are written down at the site (model: `web/src/lib/pageShell.ts`).
- SQL migration etiquette comments: `internal/store/migrate.go:50-67` (NUMBERING HAZARD). Preserve all of these when editing; they are the memory that prevents regressions.

## Function Design

**Size:** no enforced limit, but the codebase treats god-file growth as debt in progress: new API features go to `*_handlers.go` files and service-adjacent feature files rather than growing `handlers.go`/`service.go`; Settings tab cards are being extracted to `web/src/pages/settings/*Card.tsx`.

**Parameters:** deps structs bundling interfaces + config for orchestrators (`BackupDeps`); `ctx` first everywhere; options as separate typed args elsewhere.

**Return Values:** `(T, error)` with `%w` wrapping; guards return sentinels; success-with-warning classified by sentinels (restic backup exit 3 → `ErrBackupSourceUnreadable`).

## Module Design

**Exports:** export the minimum; white-box details stay unexported and are tested via `*_internal_test.go`.

**Barrel Files:** none in Go (direct imports); none in TS except re-export pattern planned for a future `lib/api/` split.

**Concurrency:** per-domain mutexes with activity names (busy instead of blocking); single-flight guards; detached goroutines recovered by `recoverOperation`; tests must WAIT for goroutines (CI rule).

## TypeScript-Specific

- Async jobs: POSTs return `{ok:true,started:true}`; outcomes NEVER read from the POST response — use `useBackupWatch` (`web/src/lib/backupWatch.ts`) correlating new run by id (baseline ids before firing, never client clock); bulk loops use `fireAndWaitRun`.
- Secrets in forms: write-only contract (GET returns `""` + `*Set` flag; blank on save keeps stored value; Clear flag/DELETE removes).
- Every user-visible string through `t()` from `useT()` (lint-enforced); em dashes banned in user text (lint-enforced, non-configurable); backend error text shown verbatim.
- Hook dependency workaround: `xRef.current = x` ref-mirroring when a callback must read fresh state (`useBackupWatch`/`progress.ts` pattern) — `exhaustive-deps` is warn-only.
- Status colors belong on Badges/chips, never on interactive controls; page roots use `PAGE_SHELL` (`web/src/lib/pageShell.ts`).

---

*Convention analysis: 2026-09-09*
