# Codebase Concerns

**Analysis Date:** 2026-09-09

Consolidated from the six module maps, deduplicated, ordered by severity. Items marked **accepted** are documented-in-code deliberate tradeoffs — do not "fix" blindly.

## Tech Debt

**`internal/api/service.go` god object (the headline debt) — HIGH:**
- Issue: 12,026 lines, 369 methods on one `Service` struct covering containers, VMs, flash, config, files, retention, off-site, budgets, drills, credentials, notifications, repo stats, discovery, self-restart.
- Files: `internal/api/service.go` (same story one layer up: `internal/api/handlers.go`, 4,767 lines holding auth/TOTP/metrics/containers/VMs/browse).
- Impact: high merge-conflict surface, hard navigation, mutex soup in the struct header, unrelated features' tests collide on shared fakes.
- Fix approach: extract per-domain services behind the existing interfaces incrementally per domain (precedent: `everything.go`, `foreign.go`, `receiver.go` are service-adjacent files; next step is real sub-structs, e.g. an `offsiteService` holding targets + replication + budgets). Keep moving new handlers to `*_handlers.go`; consider relocating the auth/TOTP block (`handlers.go:2940-3846`) to an `auth.go`. Do NOT big-bang — the 35k-line test suite is the safety net.

**`internal/restic/restic.go` monolith — HIGH:**
- Issue: 2,118 lines / 95KB carrying six concerns (types, argv builders, exec, error classification/scrubbing, high-level ops, JSON parsing); banner comments are the only navigation.
- Files: `internal/restic/restic.go`.
- Impact: cognitive load, merge-conflict surface.
- Fix approach: mechanical split into `args.go`/`exec.go`/`errors.go`/`json.go`/`progress.go` within the package — pure code motion, tests already pin behavior, no API change. Low urgency, opportunistic.

**HTTP 200 on domain failures (deliberate debt) — MEDIUM, accepted:**
- Issue: most domain failures return HTTP 200 with `{ok:false, error}` (`internal/api/handlers.go:39` envelope); status-code-based monitoring is blind to them.
- Files: `internal/api/handlers.go`, consumed by `web/src/lib/api.ts` / `web/src/lib/errors.ts`.
- Impact: unusual for a JSON API; non-SPA consumers must know the contract.
- Fix approach: none piecemeal — consistently-applied SPA contract; coordinate any change with the frontend.

**Frontend monolith files — MEDIUM:**
- Issue: `web/src/lib/api.ts` (3,007 lines, every type + endpoint), `web/src/pages/Settings.tsx` (5,149; card extraction to `web/src/pages/settings/*Card.tsx` in progress — continue it), `web/src/pages/Containers.tsx` (3,298), `web/src/pages/Dashboard.tsx` (2,765).
- Impact: routine merge conflicts on `api.ts`; unbounded growth.
- Fix approach: split `api.ts` by domain into `lib/api/` modules re-exported from one index (coordinate — many direct importers); keep extracting Settings cards; route-level `React.lazy` if bundle size becomes a reported problem.

**Go toolchain version drift — MEDIUM:**
- Issue: `go.mod` declares `go 1.25.0`; CI (`setup-go`) and the Dockerfile build on Go 1.26. The directive is a floor, so nothing breaks, but newer stdlib/toolchain features gated on the directive stay inactive.
- Files: `go.mod`, `.github/workflows/lint.yml`, `Dockerfile`.
- Fix approach: bump all three together in the next toolchain change.

**Orchestrator file size / comment weight — LOW:**
- Issue: `internal/backup/vm_orchestrator.go` (1,226 lines) and `orchestrator.go` (946) mix shared interfaces, five domains, and multi-hundred-word dated "UPDATE (Task N)" commentary.
- Fix approach: extract zvol orchestrators to `zvol_orchestrator.go`; collapse UPDATE chains to a single current-state statement per contract. Preserve the safety-property comments verbatim.

**Deliberate cross-package duplication — LOW, accepted:**
- Issue: path/credential scrub regexes exist in three documented copies (`internal/backup/orchestrator.go:869-872`, `internal/restic/restic.go`, `internal/api/handlers.go`; plus `internal/virshcli`); zvol snapshot/dataset naming mirrored in `internal/virshcli`. A fix must be applied N times or behavior silently diverges.
- Fix approach: consolidate only if a shared leaf package with no adapter imports can host them without breaking the DI seam.

**`cmd/bombvault/main.go` wiring length — LOW:**
- Issue: `run()` is ~350 lines with ~18 sequential `Set*` calls; ordering constraints documented only in comments.
- Fix approach: preserve the comments when refactoring; struct-literal wiring helper if the list grows.

**Vestigial `.gitignore` entries — TRIVIAL:** Next.js-era `.next/` entries predate the Vite SPA; harmless but misleading.

## Known Bugs

**Password-with-`/` scrub false negative — accepted tradeoff (see Security):**
- Symptoms: a URL password containing an unencoded `/` is partially consumed by the path-scrub pass first, leaving the front half of the secret in the clear in surfaced error reasons.
- Files: `internal/api/handlers.go:96-109` (`scrubSecrets`), `internal/restic/restic.go:1443` + `restic.go:1402-1423` (documented residual risk).
- Workaround: none local; the in-code doc explains why no provably-safe tighter regex was found. Tighten ONLY with a fix that provably cannot introduce a new false negative.

**Accepted race: `FailRunningRun` vs `DownloadFlashZip` — LOW, accepted:**
- Symptoms: a concurrent panic-driven fail could transiently mark the flash-zip download's run failed in the Activity Log; self-heals (the download's own `FinishRun` overwrites by id).
- Files: `internal/store/runs.go:95-104`. Do NOT fix by broadening `FailRunningRun` to global scope.

**Pre-feature multi-snapshot VM backups fail restore loudly — by design:**
- Mixed file+zvol VM backups correlate per-disk snapshots via the `vmrun:<runID>` tag; backups from before that feature lack it and restore fails loudly per-disk (empty `SnapshotID` reaches restic).
- Files: `internal/backup/vm_orchestrator.go:166-211`, `1008-1064`. Keep the fail-loud fallback; never add a silent skip.

## Security Considerations

**Trusted-LAN default (auth off) exposure — HIGH vigilance:**
- Risk: with no password set the whole API is open to the LAN; safety relies on every future credential-bearing handler remembering the second `requireAuthForSecrets` gate (403 when auth disabled).
- Files: `internal/api/handlers.go:2940-3846` (`authGate`, `requireAuthForSecrets` at 2985).
- Current mitigation: `crossOriginGuard`, self-gating tokens for `/metrics`/widget/fleet, tests in `settings_export_gate_test.go`.
- Recommendation: keep `requireAuthForSecrets` a review-checklist item for any handler that decrypts stored credentials.

**`hostshell.go` is the sharpest edge — HIGH vigilance:**
- Risk: runs `sh -c` strings inside BombVault's own container (Docker socket + `/mnt` mounted).
- Files: `internal/api/hostshell.go` (+ `_proc_unix`/`_windows`).
- Current mitigation: settings-gated, `crossOriginGuard`, session protection, process-group kill on cancel; `hostshell_cap_internal_test.go`.
- Recommendation: any change there deserves the existing paranoid comment-and-test treatment.

**Login throttle collapses behind a reverse proxy — accepted:** one bucket per proxy IP (`internal/api/api.go:48-58`). `POST /api/logout` is client-side only; real revocation is epoch rotation via `/api/logout-all`.

**Keep the existing posture:** env-only restic credentials, argv `--` guards, gitleaks narrow allowlist (never bypass the pre-push hook with `--no-verify`), `APP_KEY` loss = unrecoverable, docker.sock root-equivalence warnings in `deploy/docker-compose.generic.yml`.

## Performance Bottlenecks

**`Restic.Ls` memory hazard — HIGH:**
- Problem: buffered `Ls` retained 1355 MiB / churned 1610 MiB on a 672k-node snapshot vs 1–3 MiB for `LsStream`.
- Files: `internal/restic/restic.go:1108-1120` (doc comment).
- Improvement path: any new listing feature MUST use `LsStream`/`streamLines`; consider deprecating buffered `Ls` for large snapshots. The type system will happily let someone "simplify" `LsStream` onto `Ls` — the doc comment forbids it.

**`authGate` hits SQLite on every request — MEDIUM:**
- Problem: `h.store.GetSettings()` per request (fail-closed by design) makes the DB a request-path dependency.
- Files: `internal/api/handlers.go:3795`.
- Improvement path: cached read with invalidation on `MutateSettings` — only if the one-operator assumption ever changes.

**Scanner 16 MiB line ceiling — accepted:** `scanLines` swallows (logged) over-long lines; `streamLines` fails the run by design (#175, all-or-nothing listing integrity). Files: `internal/restic/restic.go:1018-1106`.

**SPA ships every route in one bundle — LOW:** no route-level code splitting (only locales are lazy). Files: `web/src/app/router.tsx`, `web/src/lib/i18n.ts`.

## Fragile Areas

**SQLite migration renumbering hazard (v89/90/91) — HIGH:**
- Why fragile: two branches once claimed the same migration numbers and BOTH reached users (`:latest` publishes on every push to main). Defense is structural: `alreadySatisfied` probes (`columnPresent`, `internal/store/migrate.go:37`) + fresh-numbered idempotent recovery migrations v92/v94 — reserved for that collision only.
- Files: `internal/store/migrate.go:10-67`, `internal/store/migrate_upgrade_internal_test.go`.
- Safe modification: re-read the NUMBERING HAZARD note before renumbering anything; take the NEXT unused number; never edit a body that has been on main; extend the convergence tests.

**`web/dist` commit-vs-.gitignore conflict (stale embed) — HIGH:**
- Why fragile: the repo guide says "commit web/dist", but `.gitignore` ignores `web/dist/*` except a placeholder `web/dist/index.html` whose hashed asset references are from an old build. Running only `go build` after a `web/` change embeds the stale/placeholder SPA.
- Files: `.gitignore`, `web/dist/`, `web/embed.go`, `justfile` (`just web`).
- Safe modification: always `cd web && npm ci && npm run build` before building the Go binary after frontend changes; do NOT "fix" the gitignore to commit hashed assets without an explicit decision.

**Defer ordering in `BackupContainer` — HIGH:**
- Why fragile: correctness depends on the two defers' registration order inside the closure (dependents-restart registered FIRST so it runs LAST — "never leave a dep stopped").
- Files: `internal/backup/orchestrator.go:386-440`.
- Safe modification: change behavior INSIDE the closure body; if the defers must move, re-read the block comment at 394-417 and run `restart_health_test.go` + `update_window_test.go`.

**`subcommand()` coupling — MEDIUM:**
- Why fragile: adding a global flag without registering it in `subcommandValueFlags` (`internal/restic/restic.go:1630-1636`) makes every failure of that subcommand log the flag's VALUE as the subcommand name.
- Mitigation: `TestSubcommandSkipsGlobalFlagValues` breaks when a new flag is added — keep that discipline.

**Settings row positional scan — MEDIUM:** four positional lists (struct, SELECT, Scan, UPDATE) in `internal/store/settings.go` must stay in lockstep; the compiler catches only the struct; a mismatch is a runtime scan error. Always add new settings to the round-trip tests.

**String-matching backfills v93/v95 — MEDIUM:** migrations infer history by matching error-message text (`internal/store/migrate.go`); do not reword the marker strings they match.

**`isFreezeErr` string matching — MEDIUM:** matches virsh error substrings ("freeze", "quiesce", "guest agent"); a libvirt wording change silently disables the crash-consistent retry. Files: `internal/backup/vm_orchestrator.go:776-786`; pinned by `vm_freeze_internal_test.go`.

**`restic copy` progress text-scrape — MEDIUM:** depends on restic 0.17.x plain-text output (no JSON mode); loose anchors limit blast radius to a missing percentage. Check the copy-progress tests after ANY restic version bump. Files: `internal/restic/restic.go:1169-1216`.

**Fragile web pairs — MEDIUM (must change together, test/comment-enforced only):** `web/index.html` FOUC script ↔ `web/src/lib/theme.ts`; `--color-statusOffsite` in `web/src/index.css` ↔ hard-coded copy in `internal/api/widget.html` (Go test guards it); `en`/`de` tables ↔ 40 locale files (parity tests catch missing keys, not bad wording).

**Convention enforcement is load-bearing — MEDIUM:** the 8 `bombvault/*` lint rules exist because each was repeatedly broken and shipped regressions. Exceptions go in `web/eslint.config.js`, never disable comments.

## Scaling Limits

**Single-connection SQLite pool:**
- Current capacity: `SetMaxOpenConns(1)` (`internal/store/store.go:18`) — fine for one operator.
- Limit: any store call inside a `MutateSettings` fn (or any open transaction callback) deadlocks until the HTTP request times out; constraint is comment-enforced only (`internal/store/settings.go:449-453`).
- Scaling path: cached settings reads with invalidation; keep mutation fns pure.

**One-operator assumptions:** per-IP (not global) login throttle, per-request auth store reads, no coverage of many concurrent operators.

## Dependencies at Risk

**TS 7 / typescript-eslint shim — MEDIUM:**
- Risk: `web/lint-ts/` (TS 6.0.3 side-package + module-resolve hook in `web/eslint.config.js`) is load-bearing; deleting it breaks the lint gate. Renovate deliberately does not pin it (upstream typescript-eslint#10940).
- Migration plan: revisit when typescript-eslint gains a TS 7-compatible API.

**restic/rclone Dockerfile ARGs — MEDIUM:**
- Risk: `RESTIC_VERSION`/`RCLONE_VERSION` + SHA256 ARGs are plain ARGs with no Renovate custom manager — bumps are manual; a bump without its checksum fails the build (intended guard).
- Files: `Dockerfile`, `.github/workflows/lint.yml` test-job SHA (kept in sync).
- Migration plan: add a Renovate custom manager for these ARGs.

**No version gate on the restic binary — LOW:** the engine assumes restic >= 0.17 (`--insecure-no-password`, `--retry-lock`, `--read-data-subset`) but never checks `restic version`; an old binary surfaces only as scrubbed runtime failures. Files: `internal/restic/restic.go:269-270`, `cmd/bombvault/main.go`. A startup-time assert would make misconfiguration obvious.

**Docker Hub mirror silent-failure — MEDIUM:** the push is skipped when `vars.DOCKERHUB_USERNAME` is empty, while `deploy/docker-compose.generic.yml` and `templates/my-BombVault.xml` install canonically from Docker Hub — if the credential dies, Docker Hub users silently stop receiving updates. Mitigation exists for the description credential (`dockerhub-description.yml`) but not the push; a periodic `docker manifest inspect` check would close the gap.

**SemVer tag sharp edges — MEDIUM, documented:** re-cutting an old tag drags `:8`/`:8.6` backwards (called out "STILL SHARP, deliberately" in `.github/workflows/build.yml`); the post-folding verification procedure is manual. NEVER tag without explicit approval; never re-cut an old tag without re-running the newest tag's build.

## Missing Critical Features

**No e2e suite / no route-level error boundary (web) — MEDIUM:**
- Problem: no Playwright/Cypress; no `ErrorBoundary` — a render crash in any page takes down the SPA; long flows (DR restore, Recovery polling) are covered only by unit tests on pure helpers.
- Files: `web/src/app/router.tsx`, `web/src/app/Layout.tsx`.

**NVRAM/TPM wiring gap — LOW (documented):**
- Problem: `VMBackupDeps.NVRAMPath`/`TPMPath` are exercised only by unit tests; the service layer's `BackupVM` does not set `NVRAMPath` (live NVRAM capture rides a separate SSH mechanism there).
- Files: `internal/backup/vm_orchestrator.go:131-154` (⚠ comment), `internal/api/service.go`.

## Test Coverage Gaps

**Async cleanup CI flakiness — HIGH discipline gap:**
- What's not tested: tests that do not WAIT for detached goroutines flake on Linux CI (repo rule).
- Files: all of `internal/api` async tests; patterns to copy in `panic_recovery_test.go`, `run_group_test.go`, `newTestRouterSvc` (`internal/api/handlers_test.go`).

**Windows dev sandbox runs a reduced suite — MEDIUM:** POSIX-only tests (`internal/restic/proc_test.go`, cancel/stream self-exec tests, group-kill proofs) skip on Windows; do not assume the full cancel suite ran locally.

**`internal/backup` gaps — MEDIUM:** `parentDirs` (`vm_orchestrator.go:22-34`) has no direct unit test; `Runs.Start` failure path untested for flash/files/config thin orchestrators; zvol sequencing is fake-tested only (commands hardware-verified on TrueNAS SCALE 25.10.0, 2026-08-27, but no end-to-end hardware test of ordering/cleanup — documented at `vm_orchestrator.go:789-806`).

**`internal/restic` gaps — LOW:** `authEnv`'s `RcloneConfig`-missing and `NoAmbientCreds` branches untested; no fake produces exit-3-with-valid-summary; `parseDiffStatistics` has no dedicated table; `isPermCause` breadth untabled.

**`internal/api` gaps — LOW:** rclone conf parsing edge cases, `DownloadFlashZip` streaming, and some `vmRestorePlan` zvol branches covered only indirectly.

**Embed/catalog guards are checkout-relative — LOW:** `internal/releasenotes` tests `t.Skip` when `../../.github/release-notes` is absent — a vendored/module-only run silently loses both guards; don't assume they ran in odd environments.

**No coverage thresholds (Go or web) — LOW:** inspect with `go test -cover`; no target enforced.

---

*Concerns audit: 2026-09-09*
