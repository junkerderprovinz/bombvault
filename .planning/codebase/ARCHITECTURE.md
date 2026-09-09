<!-- refreshed: 2026-09-09 -->
# Architecture

**Analysis Date:** 2026-09-09

## System Overview

```text
┌────────────────────────────────────────────────────────────────────┐
│                    Browser — React SPA                             │
│   `web/src/pages/` → `web/src/components/` → `web/src/lib/api.ts`  │
│   (SSE progress via `web/src/lib/progress.ts`, one EventSource)    │
└──────────────────────────────┬─────────────────────────────────────┘
                               │ same-origin JSON + SSE /api/progress
                               ▼
┌────────────────────────────────────────────────────────────────────┐
│                 HTTP Layer — internal/api Handler                  │
│   `internal/api/api.go` (route table) · `handlers.go` (gates +     │
│   handlers) · `server.go` (CSP, self-signed TLS) · `spa.go`        │
│   onion: securityHeaders → compress → SPA → csrfGate → authGate    │
└──────────────────────────────┬─────────────────────────────────────┘
                               ▼
┌────────────────────────────────────────────────────────────────────┐
│                Domain Layer — internal/api Service                 │
│   `internal/api/service.go` (369 methods) + feature files:         │
│   everything.go · foreign.go · receiver*.go · fleet*.go ·          │
│   offsite_targets*.go · tamper.go · digest.go · watchdog.go        │
├──────────────────┬──────────────────────────┬──────────────────────┤
│ Scheduler        │ Orchestrators (ports &   │ Cross-cutting        │
│ `internal/       │ adapters, zero I/O)      │ notify · progress ·  │
│  schedule/`      │ `internal/backup/`       │ secret · paths ·     │
│ (robfig/cron,    │ container/vm/flash/      │ platform · template  │
│  one job/domain) │ files/config             │ selfrestore · spike  │
└────────┬─────────┴───────────┬──────────────┴──────────┬───────────┘
         │                     ▼                         │
         │      ┌──────────────────────────────┐         │
         │      │ Adapters / Engines           │         │
         │      │ `internal/restic` (argv+exec)│         │
         │      │ `internal/dockercli` (SDK)   │         │
         │      │ `internal/virshcli` (SSH)    │         │
         │      │ `internal/sshconn` (SSH)     │         │
         │      └──────────────┬───────────────┘         │
         ▼                     ▼                         ▼
┌────────────────────────────────────────────────────────────────────┐
│ Persistence: SQLite `internal/store/` (settings + state, WAL,      │
│ 1 conn) · restic repos (local / rclone / s3 / sftp / …) ·          │
│ restic cache under DataDir                                         │
└────────────────────────────────────────────────────────────────────┘
         ▲
         │  composition root: `cmd/bombvault/main.go` constructs and
         │  injects every concrete adapter (nothing imports cmd/)
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

**Overall:** ports-and-adapters around a two-layer handler/service monolith.

**Key Characteristics:**
- `cmd/bombvault` is the ONLY composition root; everything internal is interface/function-seam driven; nothing imports `cmd`.
- `internal/backup` imports ONLY its own declared interfaces (`orchestrator.go:1-16` package doc) — never `internal/restic`/`dockercli`/`virshcli` — making it fully unit-testable with fakes.
- Adapters bridge concrete packages to orchestrator ports at the service layer (`resticAdapter`, `templatesAdapter`, `runsAdapter`, `sshZFSHost`, `resticZvolAdapter` — `service.go:6737-6965`).
- Always-undo guarantee: backups restart what they stopped even on error (deliberately-ordered defers, `orchestrator.go:386-464`).
- Identity-stable retention: per-item tags, ungrouped (`--group-by ""`). NEVER reintroduce `--group-by paths` (#91); NEVER `--group-by tags` globally (live VM snapshots carry an extra `live` tag).

## Layers

**Presentation (SPA):**
- Purpose: entire UI; client-routed, no SSR.
- Location: `web/src/` (`pages/` → `components/` → `lib/`; nothing imports upward from `lib`).
- Depends on: same-origin `/api` only (`web/src/lib/api.ts`).

**HTTP layer:**
- Purpose: routing, auth/CSRF gates, JSON envelope, boundary validation.
- Location: `internal/api/api.go` (route table `Router()`, `api.go:113-364`), `handlers.go`, `server.go`, `spa.go`, `sse.go`.
- Depends on: Service, store, progress, all engine interfaces.

**Domain layer:**
- Purpose: every domain operation. Per-domain `repoMu` locks + `domainActivity` names, two single-flight guards (`batchActive`, `everythingActive`), two cancel maps (cancel backup safe / cancel restore destructive — `BeginShutdown` cancels only backups).
- Location: `internal/api/service.go` (~369 methods) + feature files.
- Depends on: backup orchestrator ports, store, restic engine, notify, progress, platform.

**Orchestrators:**
- Purpose: pure domain logic with DI deps structs; no receivers, no globals (only test-knob vars).
- Location: `internal/backup/`.
- Depends on: its own interfaces + `internal/compose` (topological restart order) + `internal/model`.

**Adapters/engines:**
- Purpose: subprocess/SDK execution. `internal/restic` (builders + engine), `internal/dockercli`, `internal/virshcli`, `internal/sshconn`.
- Depends on: external binaries/sockets only.

**Persistence:**
- Purpose: settings + state. One `*store.Repo`, one accessor file per domain/table.
- Location: `internal/store/`.

## Data Flow

### Primary Request Path (async backup start, e.g. `POST /api/containers/{name}/backup`)

1. `NewSPAHandler` routes `/api/*` to the API router (`internal/api/spa.go:17`); `csrfGate` refuses cross-site unsafe methods (`handlers.go:305`); `authGate` checks session/pass-through (`handlers.go:3795`).
2. `handleBackup` (`handlers.go:735`): `h.nameParam` validates against `resourceNameRe` (400 on junk) → `h.svc.StartBackup` → 409 `{ok:false}` if domain locked, 200 `{ok:false}` if single-flight held, else 200 `{ok:true,started:true}`.
3. Service: `lockDomainFor` (`service.go:475-531`) → detached goroutine runs `Backup` (`service.go:3957`), publishing progress (`progBegin`/`progEnd`) into `progress.Store` → SSE `/api/progress` (`internal/api/sse.go`).
4. Orchestrator `BackupContainer` (`internal/backup/orchestrator.go`): `Runs.Start` → PreHook (`Docker.Exec`) → stop target (if `WasRunning`) → stop deps → `Restic.Backup(paths, ["container:<ref>","p1"])` → deferred restart of target, then deps in compose `depends_on` order (optional health-gating) → PostHook → `Runs.Finish`.
5. Batch end: per-item retention `applyRetentionPerIdentity`, then ONE `PruneAfterBulk` local prune, then stacks backup, then ONE off-site replication + cache trim (#95, #189).

### Restore Path (the guard chain)

1. Guards in order (`internal/backup/orchestrator.go`, `vm_orchestrator.go`): `Confirmed` flag → strict snapshot-id hex (`snapshotIDRe`) → absolute + traversal-free path validation → wrong-target live re-inspect (containers) → `VerifySnapshot` pre-flight → IP/port conflict pre-flight → pull image before stop/remove → only then destructive steps. Guard failures return sentinel errors (`ErrNotConfirmed`, `ErrInvalidSnapshotID`, `ErrRestoreConflict`) WITHOUT recording a run.
2. Remapped (cross-instance) restores: non-empty `RestoreDirs` switches to per-subtree `RestoreSubtreeTo`; zvol restores land only in FRESH datasets (`<base>-bombvault-restore-<unix-nano>`); rename is a MANUAL operator step.

### Scheduler Path

1. One cron schedule per domain (`internal/schedule`, `robfig/cron/v3`): containers, VMs, flash, config, files, "Backup Everything", plus offsite/drills/tamper/digest/watchdog/receiver/fleet jobs.
2. Every job is a closure over `svc` (schedule never imports api); multi-item runs wrap ctx with `WithBulkReplicateSuppressed(WithMessagesSuppressed(WithHealthchecksSuppressed(...)))` for ONE aggregate ping (#49).
3. everyN cadences gate on the durable `schedule_job_runs` last-fire record via `LastRunFunc`s (`schedule.ContainersDueGate(st)` etc.).

**State Management:**
- Server: SQLite is the single source of truth; orchestrators hold no state (all in deps structs); only package-level mutable state is `restic.maxProcs` atomic and two test-knob vars in `internal/backup`.
- SPA: no state library — Context providers (`web/src/main.tsx`), module-level singleton with subscriber set (`web/src/lib/progress.ts`), window CustomEvents (`bv:settings-changed`, display-prefs adopt), localStorage `bv-*` keys.

## Key Abstractions

**`ResticEngine` interface:**
- Purpose: the storage-engine seam between the service and the restic binary.
- Examples: defined `internal/api/service.go:64-159` (28 methods); real `*restic.Restic` (compile-time check at `service.go:162`); fake `fakeResticEngine` in `internal/api/service_test.go`.
- Pattern: constructor injection; new engine methods require touching this interface.

**Backup orchestrator ports:**
- Purpose: mockable host control (`Docker`, `Restic`, `Templates`, `Runs`, `VM`, `ZFSHost`, `ZvolRestic`, per-domain `*Restic`).
- Examples: `internal/backup/orchestrator.go:62-132`, `vm_orchestrator.go:60-895`.
- Pattern: deps structs (`BackupDeps`, `VMBackupDeps`) bundle interfaces + config; every orchestrator is an exported function `(ctx, XDeps)`.

**`Mode` struct (restic):**
- Purpose: security/behavior context per operation — encryption, lock policy (`--retry-lock 5m` vs `--no-lock`), ambient credentials (`NoAmbientCreds` for foreign sessions), storage class, bandwidth limits.
- Examples: `internal/restic/restic.go:25-265`; threads through every builder and method.

**`MutateSettings` contract:**
- Purpose: the ONLY sanctioned partial write to the settings row (read+mutate+write in one transaction under `settingsMu`; no-op detection).
- Examples: `internal/store/settings.go:454`; enforced by source-scan test `internal/store/settings_writers_test.go`.
- Pattern: mutation fns must be PURE — no DB access inside (deadlocks on the one pooled connection).

**Identity-stable tag scheme:**
- Purpose: retention identity per item. `container:<ref>`+`p1`, `vm:<name>`+`p2` (+`live` for live snapshots, +`vmrun:<runID>` correlation), `flash`, `fileset:<name>`, `config`.
- Examples: `internal/backup/orchestrator.go:378`, `vm_orchestrator.go:461`, `flash_orchestrator.go:39`, `files_orchestrator.go:44`, `config_orchestrator.go:33`.

**Progress pipeline:**
- Purpose: live percentages from restic to the SPA. Context-carried `progress.Sink`/`CopySink` (`internal/progress/progress.go`) + pub/sub `Store`; `RESTIC_PROGRESS_FPS=3` + streaming exec when a sink is in ctx (`internal/restic/restic.go:924-944`); SSE endpoint `GET /api/progress`; SPA ref-counts one EventSource (`web/src/lib/progress.ts`).

## Entry Points

**Binary (`cmd/bombvault/main.go`):**
- Triggers: container start; `bombvault healthcheck` subcommand (Docker HEALTHCHECK).
- Responsibilities: `run()` startup order is load-bearing — log→config→`ensureDataDirWritable`→`selfrestore.ApplyPending` (BEFORE store open; the only safe moment to swap the WAL-held DB)→`store.Open/Migrate/New` + `ReapInterruptedRuns`→docker→sshconn+virsh (non-fatal)→restic engine→`api.NewService`+Setters→rclone conf→scheduler wiring (ALL `Set*` calls BEFORE `ReloadWithDueChecks`/`Start`; `restic.SetMaxProcs` BEFORE any restic child)→background goroutines→`api.NewServer(cfg, web.DistFS(), handler.Router())`.
- Shutdown: stop serving FIRST, then `svc.BeginShutdown()`; a second Ctrl-C kills immediately (`signal.NotifyContext` stop after first signal).

**HTTP server:**
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

**What happens:** Treating `{ok:false}` with HTTP 200 as a bug and returning real 4xx/5xx for domain failures.
**Why it's wrong:** It is the deliberate, consistently-applied SPA contract (`writeJSON`/`okEnvelope`/`failEnvelope`, `internal/api/handlers.go:39`); the frontend checks `res.ok` and surfaces `res.error` (`web/src/lib/errors.ts` `loadErrorMessage`). Real status codes are reserved for transport-level conditions (400 bad path param, 401, 403, 409 busy, 415, 429, 503).
**Do this instead:** Follow the split for new endpoints; only introduce real status codes for genuinely new transport-level consumers, coordinated with the SPA.

### Collapsing `LsStream` into `Ls`

**What happens:** "Simplifying" the streaming listing onto the buffered one.
**Why it's wrong:** `Restic.Ls` buffers entire stdout — 1355 MiB retained on a 672k-node snapshot vs 1–3 MiB for `LsStream` (`internal/restic/restic.go:1108-1120` doc comment).
**Do this instead:** Any new listing feature MUST use `LsStream`/`streamLines`.

### Bypassing a DI seam

**What happens:** Importing `internal/restic`/`dockercli`/`virshcli` from `internal/backup`, or a second production call to `UpdateSettings`.
**Why it's wrong:** Breaks the documented isolation that makes orchestrators fake-testable; Get→edit→Update is a lost-update bug.
**Do this instead:** Extend the port interfaces; write via `MutateSettings` (guard test fails otherwise).

### Editing an applied migration body or renumbering

**What happens:** Changing SQL in a migration that already shipped, or claiming a used number.
**Why it's wrong:** `:latest` publishes on every push to main — a migration is in users' databases the moment it lands (`internal/store/migrate.go:50-67` NUMBERING HAZARD note; the v89/90/91 collision already bit).
**Do this instead:** Append a NEW migration with the next unused number; `alreadySatisfied` probes are reserved for the historical 89/90/91 collision; extend `internal/store/migrate_upgrade_internal_test.go`.

### Disabling a web lint rule instead of declaring an exception

**What happens:** Silencing a `bombvault/*` rule with a disable comment.
**Why it's wrong:** The 8 house rules exist because each was repeatedly broken and user-reported; CI is the enforcement.
**Do this instead:** Declare a stated exception in `web/eslint.config.js` (see the `page-uses-page-shell` options for the form).

## Error Handling

**Strategy:** classify with sentinel errors + `errors.Is`; scrub everything that crosses a boundary; record runs for every operation.

**Patterns:**
- JSON envelope: `writeJSON` with `okEnvelope`/`failEnvelope` (`handlers.go:39`); most domain failures = HTTP 200 `{ok:false, error:<scrubbed>}`.
- Scrubbing: `scrubError`/`scrubSecrets` (`handlers.go:65-131`) — paths→`[path]` FIRST, then `user:pass@`→`[redacted]@` (order is load-bearing). Deliberate twins in `internal/restic/restic.go:1443` and `internal/virshcli`; `internal/backup` duplicates the regexes ON PURPOSE to preserve its seam.
- Wrapping: `fmt.Errorf("domain: step: %w", err)` with domain prefixes (`backup: `, `vm live backup: `, `zvol restore: `, …) so logs identify the failing path; store wraps with the method name as context.
- Persistence: errors written to `runs.error` go through `truncateErr` (scrub + 500-char cap); restic returns ≤300-char scrubbed reasons; per-item causes deduped/bounded (`lastReason`).
- Sentinels: `errDomainBusy`, `errOffsiteAppendOnly`, `ErrBackupPathNotMounted`, `ErrRestoreMetadataOnly` (Unraid FUSE metadata-only restore = success-with-warning), `ErrBackupSourceUnreadable` (restic backup exit 3), `ErrEverythingInFlight`; message-carrying wrappers implement `Is(target)`.
- Cancellation: `restoreOutcome` maps `context.Canceled` → `"cancelled"` (distinct from `"failed"`, no failure alert); async-job SPA states treat cancelled/skipped as NEUTRAL terminals.

## Cross-Cutting Concerns

**Logging:** one stdout stream; full restic stderr server-side only; scrubbed bounded reasons to UI/runs.
**Validation:** always at the handler boundary — `h.nameParam` (`resourceNameRe`), `h.vmNameParam`, `validRunID` (32 hex), `decodeBody` (1 MiB `MaxBytesReader`, `DisallowUnknownFields`, JSON-only + `crossOriginGuard`); batch bodies re-validate every element. Never trust a name past the handler.
**Authentication:** optional login → `csrfGate` (every unsafe method) → `authGate` → `requireAuthForSecrets` for credential-bearing handlers; self-gating public endpoints (`/metrics` bearer, widget, fleet tokens).
**Secrets:** decrypt at point of use via `internal/secret`; write-only secret contract in the UI (blank = keep stored).

---

*Architecture analysis: 2026-09-09*
