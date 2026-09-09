# Module Map: `internal/api` — HTTP Service + Domain Logic

**Analysis Date:** 2026-09-09

The API package is the largest package in the repo: **45 non-test Go files (~28.5k lines)** plus **129 test files (~35k lines)**, **~63.6k lines total**. It contains both the HTTP layer and essentially all domain orchestration (backup/restore/retention/off-site/drills/credentials). `service.go` alone is **12,026 lines / 369 functions** — the "service.go is large" reputation is accurate and then some.

## Technology

**HTTP stack: standard library only.** No chi/gin/echo/httprouter anywhere in this package.

- Router: `http.NewServeMux()` with Go 1.22+ method+path patterns (`"POST /api/containers/{name}/backup"`), built in `Handler.Router()` at `internal/api/api.go:113-364`. Path params read via `r.PathValue("name")`.
- Middleware: plain `http.Handler` wrapper functions, composed manually (no alice/wrapper library). Onion order, outermost first: `securityHeaders` → `withCompression` → `NewSPAHandler(spaFS, apiRouter)` → `csrfGate` → `authGate` → mux (wired at `internal/api/server.go:53` and `internal/api/api.go:363`).
- SSE: hand-rolled via `http.Flusher` + ticker keepalive in `internal/api/sse.go` (`GET /api/progress`).
- HTTPS: self-signed ECDSA P-256 cert generated in pure Go (`EnsureSelfSigned`, `internal/api/server.go:194`); HTTP-only mode via `cfg.HTTPOnly`.
- Response compression: content-type-sniffing gzip wrapper in `internal/api/compress.go`.
- Prometheus text format written by hand in `internal/api/metrics.go` (no prometheus client dep).
- Go 1.25 (`go.mod`); zero new external dependencies for this package — everything it needs is stdlib + sibling `internal/` packages.

## Integrations

**Internal packages imported (non-test files, by breadth):**
`internal/store` (24 files — SQLite settings/state via `*store.Repo`), `internal/restic` (13 — types + real engine), `internal/secret` (9 — password hashing, session tokens, TOTP, AES-GCM decrypt), `internal/schedule` (8 — due-gate `LastRunFunc`s), `internal/notify` (8), `internal/paths` (5), `internal/backup` (5 — orchestrator interfaces), `internal/restickey` (4), `internal/progress` (3 — SSE event store), `internal/platform` (3), `internal/model`, `internal/dockercli`, `internal/config` (3 each), `internal/virshcli`, `internal/template`, `internal/spike`, `internal/ageseal` (2 each), `internal/selfrestore`, `internal/releasenotes`, `internal/compose` (1 each).

**External systems reached:**

- **restic binary** — through the `ResticEngine` interface defined at `internal/api/service.go:64-159` (28 methods: Init/Backup/Restore/Copy/Stats/Diff/Tag/Check/Prune/Unlock/Ls…). The real `*restic.Restic` satisfies it (compile-time check at line 162). This is the storage engine for all six domains.
- **Docker daemon** — `dockercli.Docker` interface: List/Inspect/Stop/Start/Remove/Pull/PullWithAuth/CreateAndStart/Exec/Self/Allocations. Used for container backup (stop/start orchestration), post-backup image update pulls (with `registryauth.go` credentials), and self-container detection.
- **libvirt/qemu** — `virshcli.Virsh` interface: DumpXML/Shutdown/Start/SnapshotCreateDiskOnly/BlockCommitActivePivot etc. for VM backup (live snapshot + blockcommit) and restore.
- **Host over SSH** — the `HostSSH` interface (`service.go:168-187`): NVRAM/TPM file transfer, `zfs send|receive` streaming (`StreamCommand`/`RunWithStdin`), Unraid notification script, known-host pinning.
- **Unraid host** — platform detection/gates via `internal/platform`; dashboard-tile plugin install over SSH (`internal/api/dashplugin.go`); appdata path conventions.
- **SQLite** — all settings, targets, runs, drills, repo stats via `store.Repo` (`GetSettings`/`MutateSettings` pattern).
- **Peer BombVault instances** — Fleet view status polls and mesh off-site offers, HTTP JSON self-gated by shared fleet token (`internal/api/fleet.go`, `internal/api/mesh.go`).
- **Notifications** — Apprise-style webhook config (`/api/notify`), Unraid notify over SSH, Healthchecks.io start/result pings (`service.go:11867-11950`).

**Adapter structs** bridging concrete packages to the `internal/backup` orchestrator's interfaces (defined `service.go:6737-6965`): `resticAdapter`, `templatesAdapter`, `runsAdapter`, `startedRunsAdapter`, `sshZFSHost`, `resticZvolAdapter`.

## Architecture

**Overall: two-layer handler/service with constructor injection and interface seams.**

- `Handler` (`internal/api/api.go:17-66`) — HTTP layer. Holds `cfg`, `store`, `docker`, `svc *Service`, `scheduler`, `probes`, optional `progress`, the six schedule due-gate funcs, cached spike checks, and the login-throttle map. All 160+ routes are methods on `*Handler`.
- `Service` (`internal/api/service.go:190-440`) — domain layer. Holds the same config/store/docker plus `virsh`, `engine ResticEngine`, optional `ssh`/`progress`/`hostShell`/`platform`, per-domain `repoMu` locks, activity names, two cancel maps, single-flight atomics, exclusion-suggest cache. ~369 methods spanning every domain.

**Request flow for a typical async-start endpoint** (`POST /api/containers/{name}/backup`):

1. SPA/`/metrics`/`/widget` split: `NewSPAHandler` routes `/api/*` to the API router (`internal/api/spa.go:17`).
2. `csrfGate` refuses `Sec-Fetch-Site: cross-site` on every unsafe method (`internal/api/handlers.go:305`).
3. `authGate` checks session (or passes through in trusted-LAN mode; fails closed 503 on store error) (`internal/api/handlers.go:3795`).
4. `handleBackup` (`internal/api/handlers.go:735`): `h.nameParam(w, r)` validates the path value against `resourceNameRe` (400 on junk) → `h.svc.StartBackup(...)` → 409 `{ok:false,error:<busy reason>}` if the domain is locked, 200 `{ok:false,error:"a backup is already running"}` if the single-flight is held, else 200 `{ok:true,started:true}`.
5. Service: `lockDomainFor` acquires the per-domain `repoMu`, records `domainActivity`, spawns a detached goroutine running `Backup` (`service.go:3957`), publishing progress via `progBegin`/`progEnd` into `progress.Store`; the SPA watches over SSE.
6. On finish: `runsAdapter.Finish` records the run row; retention (`applyRetentionPerIdentity`, identity-stable per item tag — issue #91) and off-site replication (`ReplicateOffsiteAfterBulk`) run after the batch.

**Auth model (all in `internal/api/handlers.go:2940-3846`):**

- Optional login (trusted-LAN mode = auth off by default, `authGate` is a pass-through).
- Stateless HMAC session token in cookie `bv_session`, TTL 7 days, bound to the stored password hash **and** a random `SessionEpoch`; `POST /api/logout-all` rotates the epoch to revoke every session (`handleLogoutAll`, line 3541). HttpOnly, SameSite=Lax, Secure unless HTTP-only.
- Optional TOTP second factor with single-use recovery codes (`handleLogin`, `secondFactorOK` line 3441); legacy hashes upgraded at login time.
- Login brute-force throttle keyed per client IP (`loginThrottled`, `loginClientKey`) — deliberately not shared globally; documented limitation behind a reverse proxy.
- `requireAuthForSecrets` (line 2985): second, fail-closed gate for handlers that decrypt stored credentials (recovery kit, credentialed settings export, TOTP setup) — refused with 403 when auth is disabled.
- Self-gating public endpoints (allow-listed in `authGate`, each enforces its own stored token, 403 when unset): `GET /metrics` (bearer token), `GET /widget` + `GET /api/widget/data` (widget token), `GET /api/fleet/status` + `POST /api/fleet/mesh-offer` (fleet token).
- `decodeBody` (line 365): 1 MiB `MaxBytesReader`, `DisallowUnknownFields`, JSON-only content type + cross-site refusal (`crossOriginGuard`, line 347) — this is the CSRF defense for trusted-LAN mode where `authGate` is a pass-through.

**Concurrency model:**

- Per-domain repo `sync.Mutex`es (`repoMu map[string]*sync.Mutex`) with human-readable `domainActivity` names so starters can return "busy" instead of blocking (`lockDomainFor`/`tryLockDomainFor`, `service.go:475-531`).
- Two single-flight guards: `batchActive` (any backup/restore) and `everythingActive` (Backup Everything pass) — deliberately separate (`service.go:280-298`).
- Two cancel maps with a documented asymmetry: cancelling a backup is safe, cancelling a restore is destructive, so `BeginShutdown` cancels only backups (`service.go:244-266`, `internal/api/shutdown.go`).
- Panics in run goroutines are recovered by `recoverOperation` (`service.go:4735`) and recorded as failed runs.

## Structure

```
internal/api/
├── api.go               (364)   Handler struct + full route table (Router())
├── server.go            (315)   Server, securityHeaders/CSP, self-signed certs, banner
├── spa.go               (104)   SPA static serving + /api delegation, cache headers
├── handlers.go          (4767)  ALL HTTP handlers + JSON envelope + auth/CSRF gates
├── service.go           (12026) Service struct + domain logic for everything
├── everything.go        (557)   "Backup Everything" 6th pseudo-domain pass
├── excludes_suggest.go  (1212)  exclusion assistant (snapshot aggregation, streamed)
├── foreign.go           (936)   read-only foreign-repo sessions (other instance's repo)
├── settings_portable.go (734)   settings export/import, recovery kit
├── encryption_detect.go (585)   repo encryption-mode detection
├── stacks.go            (323)   compose-stack restore, dependency ordering
├── stack_backup.go      (207)   stack-aware backup ordering
├── receiver*.go         (~1400) received-repo dashboard engine + handlers + watch
├── fleet*.go / mesh.go  (~1350) peer status, mesh off-site offers
├── offsite_targets*.go  (~590)  multi-target off-site CRUD + whitelist
├── primary_remote.go    (439)   remote-primary safety settings (role="primary")
├── tamper.go / digest.go / watchdog.go / forecast.go / metrics.go
├── deploy.go / export.go / cache.go / displayprefs.go / widget.go / dashplugin.go
├── hostshell.go (+ _proc_unix/_windows)  global pre/post hooks in own container
├── registryauth.go / files_preset.go / flash_export_compat.go / mountinfo.go
├── diskfree_linux.go / diskfree_other.go  build-tagged statfs probe
├── sse.go / shutdown.go / compress.go / banner.txt / widget.html
└── *_test.go            (129 files, ~35k lines)
```

**Where to add a new endpoint (the established path):**

1. Route line in `Handler.Router()` at `internal/api/api.go` (grouped with its domain's routes, with a comment on anything non-obvious like literal-vs-param segment ordering).
2. Handler method on `*Handler` — either in `handlers.go` if it fits an existing domain or in a new feature file (recent precedent: `fleet_handlers.go`, `receiver_handlers.go`, `offsite_targets_crud.go` — a `*_handlers.go` file per feature beats growing `handlers.go`).
3. Domain logic as a `Service` method in `service.go` (or a new `service`-adjacent file like `primary_remote.go` when it's a coherent feature).
4. Tests in a new `*_test.go` / `*_internal_test.go` named for the feature.

## Conventions

**JSON envelope (mandatory):** every endpoint responds through `writeJSON` (`handlers.go:39`) with either `okEnvelope(extra)` → `{ok:true, ...}` or `failEnvelope(err)` → `{ok:false, error:"<scrubbed>"}`. **Most domain failures return HTTP 200 with `ok:false`** — only genuine transport-level conditions use real status codes: 400 invalid path param, 401 unauthenticated, 403 secrets/cross-site, 409 domain busy, 415 non-JSON body, 429 login throttle, 503 auth store down. Follow this split; the SPA relies on it.

**Error scrubbing:** errors crossing the API boundary go through `scrubError`/`scrubSecrets` (`handlers.go:65-131`) — absolute paths → `[path]`, `user:pass@` userinfo → `[redacted]@` (paths pass first, order is load-bearing; see the long comment). Restore-destination refusals deliberately bypass path scrubbing via `restoreDestErr` (`errors.Is` classification).

**Validation at the boundary, always:** every `{name}` path value goes through `h.nameParam` (`resourceNameRe`: alnum + `._-`, no `..`, ≤128) or `h.vmNameParam` (libvirt names allow spaces but block separators/leading `-`/control chars). Run ids use `validRunID` (32 hex). Batch bodies re-validate every element (see `handleBackupAll`, `handlers.go:756`). Never trust a name past the handler.

**Sentinel errors + `errors.Is` carriers:** package-level `errDomainBusy`, `errOffsiteAppendOnly`, `ErrBackupPathNotMounted`, `ErrSelfBackup`, plus message-carrying wrappers (`platformMismatchErr`, `zvolRebaseErr`, `restoreDestErr`) that implement `Is(target)` for classification. Follow this shape for new classified errors.

**Settings mutation:** never write `store.Settings` directly — use `h.store.MutateSettings(func(s *store.Settings) error {...})` with re-checks inside the mutation when racing matters (see the password rehash guard, `handlers.go:3417`).

**Secrets:** decrypted only where needed via `internal/secret` (`secret.Decrypt(h.cfg.AppKey, ...)`); TOTP secrets stored hex-encoded + encrypted; cloud cred sets decode lazily (`decodeCloudCredSets`). Any handler returning stored credentials must call `requireAuthForSecrets` first.

**Comments are load-bearing "why" documents:** long doc comments explaining the reasoning, with bracketed decision references (`[375]`, `[364]`) and issue numbers (`#91`, `#152`, `#62`). New code is expected to keep this up — a tricky non-obvious choice without a paragraph is off-style. `nolint` directives always carry a justification comment.

**Route naming:** resource-per-domain nouns (`/api/containers`, `/api/vms`, `/api/files/sets`, `/api/offsite/targets`), literal segments (`preset`, `targets`) take precedence over `{id}`/`{domain}` params (mux specificity handles it — call it out in a comment anyway). Query params: `?source=local|offsite[:id]` via `sourceParam`/`normalizeSource` (`handlers.go:818-842`).

## Testing

**129 test files, 782 `Test*` functions, ~35k lines.** No testify/ginkgo — pure stdlib `testing` with plain `if` + `t.Fatalf`/`t.Fatal(err)`, and heavy use of `t.Run` subtests and `t.Helper()`.

**Two package modes, deliberately split:**

- `package api` (internal, 95 files — `*_internal_test.go`): white-box tests of unexported helpers (`csrf_internal_test.go`, `login_throttle_internal_test.go`, `cache_trim_internal_test.go`).
- `package api_test` (external, 34 files — `*_test.go`): tests through the public API — service methods and full HTTP routes.

**Fixtures (`internal/api/testutil_test.go`):** `newMemStore(t)` (in-memory SQLite + Migrate + Cleanup), `fakeServiceDocker` (configurable fake that records `calls` labels for ordering assertions, satisfies `dockercli.Docker`), `fakeVirsh`. `fakeResticEngine` lives in `service_test.go` and is the standard `ResticEngine` fake.

**Handler tests go through the real router:** `newTestRouter(t, d, eng)` / `newTestRouterSvc` (`handlers_test.go:23-63`) wire `Handler` + `Service` over fakes and return the real mux, so middleware (`authGate`, `csrfGate`) is exercised too. Assert with `httptest.NewRecorder` + JSON decode of the `{ok, ...}` envelope.

**Async discipline:** backups/restores run in detached goroutines; tests must wait for them (`newTestRouterSvc` exists specifically so a test can block until the goroutine touches nothing after the closed store — see `panic_recovery_test.go`, `run_group_test.go`). When adding async behavior, mirror the "wait for the goroutine" pattern; CI flakes on Linux otherwise (per `CLAUDE.md`).

**Naming:** feature-first files (`everything_singleflight_test.go`, `offsite_progress_heartbeat_internal_test.go`), `*_wiring_test.go` for constructor/wiring assertions (`vm_restore_vmrun_wiring_test.go`), `*_race_internal_test.go` for race-prone concurrency (`encryption_detect_race_internal_test.go`).

**Coverage gaps:** the exclude-assistant and foreign-restore edges are tested but a few service.go regions (rclone conf parsing edge cases, `DownloadFlashZip` streaming, some `vmRestorePlan` zvol branches) are only covered indirectly. There is no coverage target enforced; run `go test ./... -cover ./internal/api/` locally to inspect.

## Concerns

**`service.go` god object (the headline debt):**
- Issue: 12,026 lines, 369 methods on one `Service` struct covering containers, VMs, flash, config, files, retention, off-site replication, budget latches, DR drills, rclone/cloud credentials, notifications, repo stats, discovery, self-restart.
- Files: `internal/api/service.go`
- Impact: high merge-conflict surface, hard navigation, every domain's state shares one struct (mutex soup in the struct header), unit tests for unrelated features can collide on shared fakes.
- Fix approach: extract per-domain services behind the existing interfaces (the codebase already shows the pattern — `everything.go`, `foreign.go`, `receiver.go` are service-adjacent files; the next step is real sub-structs, e.g. an `offsiteService` holding targets + replication + budgets). Do it incrementally per domain; don't big-bang.

**`handlers.go` is the same story one layer up:** 4,767 lines holding auth, TOTP, metrics, containers, VMs, browse, and more. Recent history already moves new features to `*_handlers.go` files — keep doing that, and consider relocating the auth/TOTP block (`handlers.go:2940-3846`) to an `auth.go`.

**HTTP 200 for domain failures:** `{ok:false}`-with-200 makes status-code-based monitoring/health proxies blind to failures and is unusual for a JSON API. It is a deliberate, consistently-applied SPA contract — do not "fix" piecemeal (that would break the frontend), but be aware when adding non-SPA consumers.

**Trusted-LAN default (auth off) exposure:** with no password set, the whole API is open to the LAN; the mitigations (`requireAuthForSecrets`, `crossOriginGuard`, self-gating tokens for `/metrics`, widget, fleet) are well-designed but rely on every future credential-bearing handler remembering the second gate. Risk: a new handler decrypting secrets without `requireAuthForSecrets`. Recommendation: keep that call a review checklist item; the tests in `settings_export_gate_test.go` show the expected pattern.

**Documented, accepted limitations (do not "fix" blindly):** login throttle collapses to one bucket behind a reverse proxy (`api.go:48-58`); `POST /api/logout` is client-side only — real revocation is epoch rotation via `/api/logout-all`; `scrubSecrets` has a known false-negative for passwords containing `/` (`handlers.go:96-109`).

**`authGate` hits SQLite on every request:** `h.store.GetSettings()` per request (fail-closed by design). Fine for one operator on SQLite, but it makes the DB a request-path dependency; a cached read with invalidation on `MutateSettings` is the scaling path if that ever changes.

**Hostshell is the sharpest security edge:** `hostshell.go` runs `sh -c` strings in BombVault's own container (Docker socket + `/mnt` mounted). Guards: settings-gated, `crossOriginGuard`, session protection, process-group kill on cancel. Any change there deserves the same paranoid comment-and-test treatment the existing code has (`hostshell_cap_internal_test.go`).

**Hygiene is otherwise excellent:** one `TODO` in non-test code (`service.go:7353`), all `nolint`s justified, forbidden-file hygiene respected (secrets live encrypted in SQLite, never in the tree), and the test suite (35k lines, 782 tests) is a real safety net for refactoring the god object.

---

*Module analysis: 2026-09-09*
