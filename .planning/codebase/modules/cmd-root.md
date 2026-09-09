# Module Map: `cmd/` + Build & CI Root

**Analysis Date:** 2026-09-09

Scope: `cmd/bombvault/`, Go module manifest (`go.mod`, `go.sum`), `Dockerfile`, `justfile`, `.github/` (workflows + release-notes), `deploy/`, `renovate.json`, `scripts/`, `templates/`, `truenas-apps/`, `web/embed.go` (embed mechanism only), root lint/secret config. Internal packages and `web/` internals are covered by sibling module maps.

## Technology

**Languages & runtime versions:**

- **Go 1.25.0** declared in `go.mod` (`go 1.25.0`, module `github.com/junkerderprovinz/bombvault`). CI (`setup-go` in `.github/workflows/lint.yml`) and the Dockerfile build stage (`golang:1.26-bookworm`) both run **Go 1.26** — the `go` directive is a floor, so this builds fine, but the manifest lags the toolchain.
- **TypeScript / Node 24** — the SPA build stage uses `node:24-slim` (digest-pinned in `Dockerfile`); CI web job pins `node-version: '24'`. Frontend internals are out of scope here.
- **Python 3.x** — only for the MkDocs docs site (`.github/workflows/docs.yml` installs `docs-requirements.txt`).

**Key direct dependencies (from `go.mod` — this file is the manifest of record for the whole repo):**

- `github.com/robfig/cron/v3 v3.0.1` — the scheduler engine (`internal/schedule`).
- `github.com/docker/docker v28.5.2+incompatible` + `github.com/docker/go-connections v0.8.1` + `github.com/containerd/errdefs` — Docker SDK over the mounted `docker.sock` (no docker CLI).
- `modernc.org/sqlite v1.56.0` — CGO-free SQLite (`internal/store`); combined with `CGO_ENABLED=0` builds this is what makes the binary fully static.
- `filippo.io/age v1.3.1` + `golang.org/x/crypto` — secret encryption / crypto.

**Build tooling:**

- **just** — task runner (`justfile`); recipes use sh (Git Bash on Windows).
- **golangci-lint** (config `.golangci.yml`, v2 schema), **gofmt**, **go vet**, **hadolint** (Dockerfile), **gitleaks** (config `.gitleaks.toml`).
- **Docker buildx** — multi-arch amd64+arm64; the web and Go build stages run natively on `BUILDPLATFORM` and cross-compile via `TARGETOS`/`TARGETARCH`, avoiding QEMU emulation for the Go compile.
- **tini** — PID 1 in the image (`-g` flag forwards signals to the whole process group, reaping orphaned restic→rclone grandchildren, issue #35).
- **restic 0.17.3** and **rclone 1.74.2** — downloaded from upstream releases in the Dockerfile runtime stage with pinned SHA256 `ARG`s; Debian's apt packages are too old. `RESTIC_VERSION`/`RCLONE_VERSION` and their SHA256 args must be bumped in the same change or the `sha256sum -c` check fails the build.

**Version stamping:** `Dockerfile` line 51-52 builds with `-ldflags "-s -w -X github.com/junkerderprovinz/bombvault/internal/api.Version=${VERSION}"`; CI computes `v<tag>` for tag builds and `v<lastTag>+<branch>.<sha>` otherwise (branch slashes/underscores become hyphens — SemVer build metadata allows only alphanumerics, hyphens, dots; `internal/releasenotes` parses this string).

**Prescriptive:** to bump Go, change all three places together: `go.mod` (`go 1.25.0`), `.github/workflows/lint.yml` (`go-version`), and the `Dockerfile` build-stage image + digest.

## Integrations

**Container registries (`.github/workflows/build.yml`):**

- **GHCR** — `ghcr.io/${{ github.repository }}`, primary push target, auth via `secrets.GITHUB_TOKEN` (permissions: `packages: write`).
- **Docker Hub mirror** — `docker.io/${{ vars.DOCKERHUB_USERNAME }}/bombvault`, active only when the repo **variable** `DOCKERHUB_USERNAME` and **secret** `DOCKERHUB_TOKEN` are set together; otherwise silently GHCR-only. The README→Docker Hub Overview sync (peter-evans/dockerhub-description) uses `.github/DOCKERHUB.md` (a condensed edition — the full README exceeds Docker Hub's 25000-byte limit).

**Upstream binary sources (`Dockerfile` runtime stage):** restic from `github.com/restic/restic/releases` and rclone from `downloads.rclone.org`, both checksum-verified with pinned SHA256 args. Note: rclone reads `RCLONE_*` env vars as flag overrides, so the `rclone version` build check runs with them explicitly unset.

**GitHub Actions workflows (`.github/workflows/`):**

- `lint.yml` — three jobs: `go` (build/vet/golangci-lint), `web` (npm ci, `tsc --noEmit`, `npm run lint`, `npm test`), `test` (installs restic 0.17.3 from upstream with SHA256 check, then `go test ./...`). Triggers on push to `main`, `feat/v4`, `experiment/**` and PRs to `main`.
- `build.yml` — `smoke` job (amd64 boot test + tini PID 1 check + non-blocking Trivy SARIF scan) gates `build` job (multi-arch build+push with SLSA `provenance: true` and `sbom: true` attestations, GHA cache). Push only on `main`/tags, never from PRs. Also syncs the Docker Hub description.
- `docs.yml` — MkDocs Material → GitHub Pages, strict build, only when `docs/**`, `mkdocs.yml`, or `docs-requirements.txt` change.
- `dockerhub-description.yml` — manual-only credential-isolation probe: syncs the description without building anything, so a red run means "broken Docker Hub credential", not "broken build".
- `repo-watch-instant.yml` — on issues/PRs/comments/reviews, dispatches a `repository_dispatch` to `junkerderprovinz/repo-watch` (Matrix posting lives there). Uses `secrets.REPOWATCH_PAT`. All event fields (attacker-controlled titles on a public repo) pass through **env vars, never `${{ }}` interpolation into `run:`** — documented shell-injection defense.

**Renovate (`renovate.json`):** weekly (Monday before 6am, Europe/Vienna), `config:recommended`, pins GitHub Action digests and Docker base-image digests, semantic commits, auto-merges Actions updates with `ci:` prefix, groups Go minor/patch into one weekly PR (`chore(deps):`). Two deliberate disables: the `web/lint-ts` typescript pin (TS 6.x side-by-side shim, upstream typescript-eslint#10940) and pinning of BombVault's **own** published image in `deploy/docker-compose.generic.yml` (users must keep floating `:latest` — #168).

**Runtime integrations wired in `cmd/bombvault/main.go`:** Docker API over `/var/run/docker.sock`; libvirt/virsh over SSH (`qemu+ssh://`, key generated on first run, `~/.ssh/config` written so the external ssh binary uses it); restic CLI with rclone config for off-site repos; Healthchecks.io lifecycle pings via the notify fan-out (service side).

**Environment variables consumed by the entrypoint:** `PORT` (3000), `HTTPS_PORT` (3443), `TZ` (scheduler timezone, logged loudly at boot — see `logSchedulerTimezone`), plus everything `config.LoadFromEnv` reads (`DATA_DIR=/config`, `APP_KEY`, `PLATFORM`, `HOST_SOURCE_ROOT`/`HOST_MOUNT_ROOT`, `BACKUP_MAX_HOURS`, `HTTP_ONLY`, `LIBVIRT_*` — defaults set in the `Dockerfile` `ENV` block and documented in `deploy/docker-compose.generic.yml`).

## Architecture

**Overall:** single static Go binary; `cmd/bombvault` is the **composition root** — the only place that constructs concrete adapters and injects them into `internal/api`. Everything internal is interface/function-seam driven; `cmd` imports all internals, nothing imports `cmd`.

**Startup sequence (`run()` in `cmd/bombvault/main.go`, order is load-bearing):**

1. `log.SetOutput(os.Stdout)` — one stream with the stdout banner.
2. `config.LoadFromEnv()` (`internal/config`).
3. `ensureDataDirWritable(cfg.DataDir)` — fail fast if `/config` is missing/read-only (otherwise SQLite lands on the ephemeral layer and settings reset every restart).
4. `selfrestore.ApplyPending` — staged config restore **before** the DB opens (only safe moment to swap the WAL-held settings DB); fail-safe: boot continues on the live DB.
5. `store.Open` → `store.Migrate` → `store.New`; `st.ReapInterruptedRuns()` marks runs left `running` by a previous lifetime as failed.
6. `dockercli.New()` — Docker SDK client.
7. `sshconn.New(...)` + `EnsureKey()` + `WriteSSHConfig()`, then `virshcli.New(...)` — both failures are **non-fatal** (VM backup unavailable, not boot-broken).
8. restic engine: `&restic.Restic{Bin: "restic", RcloneConfig: <DataDir>/rclone.conf, CacheDir: <DataDir>/cache/restic}` — the cache under the persistent volume is critical for off-site repos (#95); mkdir failure degrades to restic's default cache dir.
9. `api.NewService(cfg, st, dc, vc, engine)`, then `platform.Detect` → `svc.SetPlatform(platformFor(kind))` (Unraid/TrueNAS/Generic; unknown falls back to Generic with a logged warning), `svc.SetResticCacheDir`, `svc.SetHostSSH`, `svc.SetProgress`.
10. `svc.WriteRcloneConfFile()` — materializes the decrypted rclone config (non-fatal).
11. Scheduler construction — see below — then `restic.SetMaxProcs(settings.BackupCores)` **before** any restic child can start, `scheduler.ReloadWithDueChecks(...)`, `scheduler.Start()`.
12. Background goroutines: catch-up (fixed 2-minute delay, `CatchUpMissed`, one-shot), `handler.WarmSpike()`, `svc.CollectStatsOnStartup()`.
13. `api.NewHandler(...)` → `handler.SetProgress(prog)` (same store the service publishes to → SSE `/api/progress`) → `signal.NotifyContext(SIGINT, SIGTERM)` → `api.NewServer(cfg, web.DistFS(), handler.Router())` → `server.Run(ctx)`.

**Shutdown ordering (reverse of startup, deliberate):** stop serving first (no new work), *then* `svc.BeginShutdown()` cancels backups. `signal.NotifyContext`'s `stop()` is called after the first signal so a **second** Ctrl-C kills immediately instead of waiting behind a slow shutdown.

**Scheduler design (`internal/schedule`, wired entirely from `cmd/bombvault/main.go`):**

- Backed by `robfig/cron/v3`, **one cron schedule per domain**, not one global job. Domains: containers, VMs, flash, config, files, "Backup Everything" (a pseudo-domain looping all five), plus auxiliary jobs: offsite, drills, tamper tests, weekly digest, overdue watchdog, receiver checks, fleet polls.
- Pure dependency injection: `schedule.New(backupFn, listFn)` plus ~18 `Set*Job`/`Set*Store` wiring calls (`SetVMJob`, `SetHealthchecksAggregator`, `SetFlashJob`, `SetConfigJob`, `SetFilesJob`, `SetEverythingJob`, `SetOffsiteJob`, `SetPruneAfterBulkJob`, `SetStacksAfterBulkJob`, `SetOffsiteAfterBulkJob`, `SetDrillJob`, `SetTamperJob`, `SetDigestJob`, `SetJobRunStore`, `SetWatchdogJob`, `SetReceiverJob`, `SetFleetJob`). Every job is a closure over `svc`, so `schedule` never imports `api`.
- **Hard rule:** all `Set*` calls (including `SetJobRunStore`) must precede `ReloadWithDueChecks`/`Start` — the reload reads the wired hooks.
- Context-flag pattern: scheduled multi-item runs wrap the context with `WithBulkReplicateSuppressed(WithMessagesSuppressed(WithHealthchecksSuppressed(...)))` so per-item pings are suppressed and **one aggregate** start/success/fail ping (`SetHealthchecksAggregator`) represents the whole domain run (#49).
- Batched after-bulk hooks: per-item retention forgets without `--prune`, then ONE `PruneAfterBulk` local prune, then stacks backup, then ONE off-site replication + cache trim + one repo-size sample — a 44-container night pays 44 backups but one prune and one off-site index load (#95, #189).
- Skip-not-fail semantics: `backup.ErrContainerNotInstalled` / `ErrVMNotInstalled` / `api.ErrEverythingInFlight` from a scheduled closure are converted to `nil` (a skip, already recorded/logged), not a job failure.
- Cadences (`ParseCadence`): `off`, `daily HH:MM`, `weekly DOW[,DOW] HH:MM`, `everyN <N> HH:MM` (daily trigger + calendar-day due gate backed by the durable `SetJobRunStore` last-run record, #166), or a raw 5-field cron. Every-N gates use per-domain `LastRunFunc`s (`schedule.ContainersDueGate(st)` etc.) that exclude items the pass would not run.

**Web embed mechanism (`web/embed.go` — the only web/ file in scope):** `//go:embed all:dist` must live in the package whose directory contains `dist/` (Go embed cannot reference `..`), so the directive sits at `web/` root and exposes `DistFS()` (an `fs.Sub` rooted at `dist`, panics if malformed). `cmd/bombvault/main.go` line 510 passes `web.DistFS()` into `api.NewServer`. `web/dist/index.html` is a placeholder committed via `.gitignore` negation (`!web/dist/index.html`) so `go build` works before the Vite build; CI builds the SPA into stage 1 of the Dockerfile and copies it to `web/dist` before `go build`.

## Structure

**`cmd/bombvault/` (single command, no subcommand framework):**

- `cmd/bombvault/main.go` (~520 lines) — `main()` (healthcheck dispatch + `run()`), `healthcheck`/`healthcheckAt` (Docker HEALTHCHECK probe, `InsecureSkipVerify` justified for loopback), `platformFor`, `ensureDataDirWritable`, `logSchedulerTimezone`, all wiring.
- `cmd/bombvault/main_test.go` — `platformFor` mapping tests, `logSchedulerTimezone` table test.
- `cmd/bombvault/healthcheck_test.go` — `httptest`-based `healthcheckAt` exit-code test.

**Root manifests & config:**

- `go.mod`, `go.sum` — module manifest of record (see Technology).
- `Dockerfile` — 3-stage build (web → build → runtime), digest-pinned base images, checksum-pinned restic/rclone, `HEALTHCHECK` invoking the binary itself (`bombvault healthcheck`, no shell/curl needed), tini entrypoint.
- `.dockerignore` — build context limited to `go.mod`/`go.sum`, `cmd/`, `internal/`, `web/` source; excludes `.git`, `docs`, `*.md`, `web/dist` (rebuilt in-stage), env/data dirs.
- `justfile` — recipes: `build`, `test`, `fmt`, `check` (the pre-push chain), `web`, `secrets`, `notes <version>` (scaffolds both release-note copies).
- `renovate.json`, `.golangci.yml`, `.gitleaks.toml`, `.gitignore` (note: contains vestigial Next.js `.next/` entries), `.gitattributes`, `mkdocs.yml`, `docs-requirements.txt`, `LICENSE` (AGPL-3.0-only), `README.md`, `CLAUDE.md`.

**`.github/`:** `workflows/` (5 files, kebab-case names), `release-notes/` (`vX.Y.Z.md`, v1.0.0 → v8.6.4, 66 files), `DOCKERHUB.md` (condensed README for Docker Hub), `FUNDING.yml`, `SUPPORT_THREAD.html`, `assets/`.

**Deployment artifacts (not Go code):**

- `deploy/docker-compose.generic.yml` — user-facing generic-host compose file; documents `APP_KEY`, fixed `hostname: bombvault` (stable restic lock hostname so `unlockStale` recognizes its own dead locks), identity-bind Host Data convention, docker.sock root-equivalence warning.
- `templates/my-BombVault.xml` — Unraid Community Applications template.
- `truenas-apps/` — TrueNAS Scale app catalog (`app.yaml`, `item.yaml`, `ix_values.yaml`, `questions.yaml`, `templates/` with a gitleaks-allowlisted `test_values/` fixture dir).
- `scripts/` — `gen_glyphs.py` (glyph path generator feeding `scripts/glyph-paths/`), `docker-path.txt`, `cloud-path.txt` (path notes).
- `docs/` + `overrides/` — MkDocs Material sources (deployed by `docs.yml`).

**Naming conventions:** Go files snake_case, tests co-located (`*_test.go`, internal tests as `*_internal_test.go` in `internal/schedule`); workflows kebab-case; release notes exactly `vX.Y.Z.md` in both `.github/release-notes/` and `internal/releasenotes/notes/`.

## Conventions

**Build/lint gate (run before every push; `just check` runs the chain):**

1. `gofmt -l .` must print nothing (CI and the global pre-push hook fail on output; `just fmt` fixes).
2. `go vet ./...`
3. `golangci-lint run ./...` — enabled linters in `.golangci.yml`: errcheck, govet, staticcheck, ineffassign, unused, **gosec**. Exclusions: `G304|G306` in `internal/(template|paths)/`, govet in `node_modules/`, staticcheck SA5011 in `_test.go` (t.Fatal is not seen as terminating by staticcheck 2.12.x).
4. `go test ./...` — **requires restic >= 0.17 on PATH** (needed for `--insecure-no-password`; apt's is too old — CI installs 0.17.3 from upstream with a SHA256 check, and the comment says to keep that SHA in sync with `RESTIC_SHA256_AMD64` in the Dockerfile).
5. `hadolint Dockerfile`.
6. When `web/` changed: `cd web && npm ci && npm run build` and **commit `web/dist`**... in practice `web/dist` artifacts stay ignored except the placeholder; the Docker build regenerates dist.

**Gitleaks (`.gitleaks.toml`):** extends defaults; a **narrow** allowlist exists so the pre-push hook never gets bypassed with `--no-verify` again — only `truenas-apps/templates/test_values/`, `internal/*_test.go`, `web/src/*.test.tsx?`, plus a shape-scoped regex for `cadence.cronEx*` translation keys. Verified: realistic fake AWS/GitHub credentials outside the allowlist still trip it.

**Commits:** conventional-commit style (`docs:`, `fix(ui):`, `fix(secret):`, `ci:` for Renovate Actions PRs, `chore(deps):` for grouped Go updates). All GitHub Actions and Docker base images are digest-pinned (enforced by Renovate `helpers:pinGitHubActionDigests` / `docker:pinDigests`).

**Release process (NEVER tag without explicit approval):**

1. Write release notes to **both** `.github/release-notes/vX.Y.Z.md` **and** `internal/releasenotes/notes/vX.Y.Z.md` (`just notes X.Y.Z` scaffolds both); an embed-sync test in `internal/releasenotes` fails CI if the copies are missing or differ.
2. Commit, push, wait for Lint + Test + Build Docker Image green.
3. `git tag vX.Y.Z && git push origin vX.Y.Z` → tag build publishes `:X.Y.Z`, `:X.Y`, `:X` to GHCR + Docker Hub. `:latest` is pushed by **branch builds only** (`ref_type == 'branch' && default branch` — a tag build never moves `latest`; see the long comment in `build.yml` after the 2026-09-02 v8.3.0 re-cut incident).
4. `gh release create vX.Y.Z --title "vX.Y.Z" --notes-file .github/release-notes/vX.Y.Z.md` (title is the version only).
5. Verify all Docker Hub tags return 200; reply to the fixed issue **once** and close it.

**Dockerfile conventions:** base images pinned by digest with a `# syntax=docker/dockerfile:1@sha256:...` header; `SHELL ... pipefail` so a failed download cannot slip past `sha256sum -c`; every dependency version bump must update its SHA256 ARG in the same change; heavy justification comments referencing issue numbers on every non-obvious choice.

## Testing

**CI test setup (`.github/workflows/lint.yml` `test` job):** checkout → `setup-go` 1.26 → install restic 0.17.3 (curl from GitHub releases, `sha256sum -c`, `sudo install` to `/usr/local/bin`) → `go test ./...`. The restic install step's SHA comment explicitly requires sync with the Dockerfile's `RESTIC_SHA256_AMD64`.

**Smoke test (`.github/workflows/build.yml` `smoke` job — gates the push):**

- Build amd64 image locally (`load: true`), `docker run` with a random `APP_KEY` and `HTTP_ONLY=true`, poll `http://localhost:3000/api/health` every 2s up to 30 attempts (~60s); failure dumps `docker logs`.
- **tini PID 1 check:** `docker exec bv-smoke cat /proc/1/comm` must be `tini` — the zombie-reaper regression guard for #35.
- Trivy scan of the built image: HIGH/CRITICAL only, `ignore-unfixed`, SARIF to the Security tab, **deliberately non-blocking** (`exit-code: "0"` — most findings live in the upstream base image; this is visibility, not a gate).

**cmd package tests (`go test ./cmd/...`, run in the suite):**

- `cmd/bombvault/healthcheck_test.go` — `httptest.Server` serving `/api/health`; asserts exit 0 on HTTP 200 and exit 1 when nothing listens.
- `cmd/bombvault/main_test.go` — `platformFor` returns concrete `platform.TrueNAS{}`/`Unraid{}`/`Generic{}` and falls back with a logged warning for unknown kinds; `logSchedulerTimezone` table test (unset TZ → "planning in UTC"; misspelled TZ → "did NOT resolve"; resolved zone → named). Note the test sets `time.Local` directly (Go caches `time.Local` from TZ once) and uses `time.FixedZone` so no tzdata is needed on the runner.

**Entry-point smoke on a dev box:** `go run ./cmd/bombvault` (SIGINT is handled, so Ctrl-C shuts down cleanly); `curl localhost:3000/api/health` to confirm the API bound.

**Prescriptive:** a new scheduled domain or job hook needs (a) a closure in `run()`, (b) the corresponding `Set*Job` call **before** `ReloadWithDueChecks`, (c) if it is an every-N-cadence domain, a `LastRunFunc`/due gate passed into `ReloadWithDueChecks`.

## Concerns

**Go version drift (low, hygiene):** `go.mod` declares `go 1.25.0` while CI and the image build on Go 1.26. The directive is a floor so nothing breaks, but language/toolchain features gated on the directive (e.g. newer stdlib APIs available only when the directive is high enough) will not activate. Fix: bump `go.mod` to 1.26 in the same change as the next toolchain bump.

**Docker Hub mirror is silent-failure (medium, operational):** the Docker Hub login/push in `build.yml` is skipped entirely when `vars.DOCKERHUB_USERNAME` is empty, yet `deploy/docker-compose.generic.yml` and `templates/my-BombVault.xml` install from Docker Hub as the canonical tag. If the variable/secret are ever removed or the token expires, GHCR keeps updating while Docker Hub users silently stop receiving updates. Mitigation exists for the description credential (`dockerhub-description.yml` isolates it on demand) but not for the image push itself; a periodic `docker manifest inspect` check would close the gap.

**Semantic-version tag sharp edges (medium, documented-in-code):** `{{major}}` and `{{major}}.{{minor}}` tags are moved by whichever tag built last — re-cutting an old version drags `8`/`8.6` backwards (called out as "STILL SHARP, deliberately" in `build.yml`). The post-folding procedure (re-run the newest tag's build, verify latest/8/8.minor resolve to one digest) is manual. Never re-cut an old tag without following it.

**restic/rclone Dockerfile ARGs are not Renovate-managed (medium, supply-chain freshness):** Renovate updates Go modules, npm, Actions digests and base-image digests, but `RESTIC_VERSION`/`RCLONE_VERSION` and their SHA256 ARGs in the `Dockerfile` are plain ARGs with no custom-manager regex — version bumps are fully manual and easy to miss (and a bump without its checksum fails the build, which is the intended guard). Adding a Renovate custom manager for these ARGs would automate it.

**`cmd/bombvault/main.go` wiring length (low, maintainability):** the `run()` function is ~350 lines with ~18 sequential `Set*` calls; ordering constraints (Set* before Reload, `SetMaxProcs` before any restic child, selfrestore before store open) are documented only in comments. New job types must also be added in `internal/schedule`. The comments are excellent — preserve them when refactoring; consider a struct-literal wiring helper if the list grows further.

**Release-notes dual-copy fragility (low, guarded):** every release requires byte-identical copies in `.github/release-notes/` and `internal/releasenotes/notes/`; the embed-sync test catches divergence at CI time and `just notes` scaffolds both, but nothing prevents editing one copy after scaffolding. Always run the test suite before tagging.

**Vestigial `.gitignore` entries (trivial):** `.next/` and Next.js-era entries predate the Vite SPA; harmless but misleading.

**Security posture in scope (good, keep it):** repo-watch workflow's env-var-only interpolation of attacker-controlled titles; gitleaks allowlist kept narrow and verified against planted credentials; healthcheck's `InsecureSkipVerify` is loopback-only and nolint-justified; compose file warns that the docker.sock mount is root-equivalent and the stack must run on a trusted network; `APP_KEY` loss is called out as unrecoverable. Do not weaken any of these.

---

*Module map: cmd/ + build & CI — 2026-09-09*
