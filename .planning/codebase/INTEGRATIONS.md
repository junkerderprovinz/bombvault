# External Integrations

**Analysis Date:** 2026-09-09

## APIs & External Services

**restic storage engine (the core integration):**
- Only `internal/restic` touches the restic binary (`exec.CommandContext`, argv builders + engine in `internal/restic/restic.go`).
- Repo location types recognized by `remoteRepoRe` (`restic.go:28`): local paths, `rclone:Remote:bucket/path` (primary off-site), and native backends `s3:`, `sftp:`, `rest:`, `b2:`, `azure:`, `gs:`, `swift:`.
- Credential transport rule: **env only, never argv** — `RESTIC_PASSWORD`, `RESTIC_INSECURE_NO_PASSWORD=true`, `RESTIC_FROM_PASSWORD` (for `restic copy` source repo), backend creds (`AWS_*`, `RESTIC_REST_*`) in `Mode.Env`, `RCLONE_CONFIG` — all set in one place, `authEnv` (`restic.go:893-918`).
- `rclone` is spawned by restic as a grandchild (hence the process-group SIGTERM kill, `internal/restic/proc_unix.go`); no direct rclone API use.
- S3 storage classes restricted to a whitelist (`STANDARD`, `STANDARD_IA`, `ONEZONE_IA`, `INTELLIGENT_TIERING`, `GLACIER_IR`); GLACIER/DEEP_ARCHIVE deliberately excluded (async thaw would break restore).

**Docker daemon:**
- SDK over the mounted `/var/run/docker.sock` via `internal/dockercli` (interface `dockercli.Docker`: List/Inspect/Stop/Start/Remove/Pull/CreateAndStart/Exec/Allocations). Used for container stop/start orchestration, post-backup image pulls (credentials via `internal/api/registryauth.go`), self-container detection.

**libvirt/qemu (VMs):**
- `internal/virshcli` over SSH (`qemu+ssh://`; no libvirt socket is ever bind-mounted — virsh runs ON the host). SSH managed by `internal/sshconn` (key generated on first run, `~/.ssh/config` written so the external ssh binary uses it). VM backup = live disk-only snapshot + blockcommit; graceful path shuts down/starts.

**Host over SSH (Unraid/TrueNAS):**
- `HostSSH` interface (`internal/api/service.go:168-187`): NVRAM/TPM file transfer, `zfs send|receive` streaming for zvol backup/restore (`internal/backup/vm_orchestrator.go` `ZFSHost` port), Unraid notification script, dashboard-tile plugin install (`internal/api/dashplugin.go`).

**Healthchecks.io:**
- Lifecycle pings (start/success/fail) via `internal/notify`; scheduled domain runs suppress per-item pings with context flags and send ONE aggregate ping per pass (`SetHealthchecksAggregator`, issue #49).

**Notifications (`internal/notify`):**
- Generic webhook JSON (Discord/Slack/Gotify/ntfy), Matrix, SMTP email, Apprise API. Best-effort, time-bounded — a notify failure never affects a backup. Config arrives as the encrypted `NotifyConf` settings blob.

**Peer BombVault instances (Fleet/Mesh):**
- Status polls and mesh off-site offers, HTTP JSON self-gated by a shared fleet token (`internal/api/fleet.go`, `internal/api/mesh.go`; endpoints `GET /api/fleet/status`, `POST /api/fleet/mesh-offer`).

**GitHub (CI/CD):**
- GHCR push (`ghcr.io/${{ github.repository }}`, `secrets.GITHUB_TOKEN` with `packages: write`) — primary target (`.github/workflows/build.yml`).
- Docker Hub mirror (`docker.io/${{ vars.DOCKERHUB_USERNAME }}/bombvault`, `secrets.DOCKERHUB_TOKEN`) — active only when BOTH the repo variable and secret are set; otherwise silently GHCR-only.
- `repo-watch-instant.yml` dispatches to `junkerderprovinz/repo-watch` (Matrix posting) using `secrets.REPOWATCH_PAT`; attacker-controlled event fields pass through **env vars, never `${{ }}` interpolation into `run:`**.
- Renovate app token flows are `config:recommended`; two deliberate disables: the `web/lint-ts` typescript pin and pinning BombVault's own image in `deploy/docker-compose.generic.yml` (users must keep floating `:latest`, #168).

## Data Storage

**Databases:**
- **SQLite** via `modernc.org/sqlite` (pure Go, driver name `"sqlite"`), file at `<DataDir>/bombvault.sqlite`.
  - Client: `internal/store` (`store.Open` → `store.Migrate` → `store.New` in `cmd/bombvault/main.go:192-200`; the only production open site).
  - Tuning: `SetMaxOpenConns(1)` (writes can never race; `MutateSettings` relies on it), `PRAGMA journal_mode=WAL`, `PRAGMA foreign_keys=ON` (`internal/store/store.go`).
  - Schema: 18 tables created by hand-rolled forward-only migrations v1..v99 (`internal/store/migrate.go`); ledger table `schema_migrations`.
  - Settings: ONE row (`id = 1`), ~90 columns, single Go struct `store.Settings` (`internal/store/settings.go`). All partial writes via `MutateSettings` (transaction + mutex); `UpdateSettings` is full-row REPLACE and test-only (source-scan guard `internal/store/settings_writers_test.go`).
  - Snapshots: `Repo.VacuumInto(dst)` (`internal/store/vacuum.go`) stages config backup/self-restore.

**File Storage:**
- restic repos (local paths or the remote types above) hold all backup data.
- Persistent Docker volume at `/config` holds: the SQLite DB, `rclone.conf` (materialized by `svc.WriteRcloneConfFile()`), the restic cache (`<DataDir>/cache/restic` — must be on the persistent volume, #95), staged config snapshots.

**Caching:**
- restic cache dir under the persistent volume (set in `cmd/bombvault/main.go`); mkdir failure degrades to restic's default.
- `repo_stats` SQLite table caches restic repo statistics (`internal/store/repostats.go`).

## Authentication & Identity

**Auth Provider:** Custom, in-process (`internal/secret` — no external IdP).
- Optional login (trusted-LAN mode = auth off; `authGate` pass-through, `internal/api/handlers.go:3795`).
- Stateless HMAC session token in cookie `bv_session` (TTL 7 days, bound to password hash + `SessionEpoch`; `/api/logout-all` rotates the epoch). HttpOnly, SameSite=Lax.
- Optional TOTP second factor with single-use recovery codes (hand-written RFC 6238 in `internal/secret/totp.go`); Argon2id password hashing with legacy-hash upgrade at login.
- Per-IP login throttle (`loginThrottled`); second fail-closed `requireAuthForSecrets` gate for any handler decrypting stored credentials.
- restic repo passwords derived from `APP_KEY` via domain-separated HMAC-SHA256 (`internal/restickey`); exports sealed to age public keys (`internal/ageseal`).

## Monitoring & Observability

**Error Tracking:** None external. Run history in SQLite (`runs` table; `StartRun`/`FinishRun` in `internal/store/runs.go`), panics recovered and recorded as failed runs (`recoverOperation`, `internal/api/service.go:4735`).

**Logs:**
- Single stdout stream (`log.SetOutput(os.Stdout)` in `cmd/bombvault/main.go`). Full restic stderr logged server-side; only scrubbed, bounded reasons reach the UI (`runError`, ≤300 chars, `internal/restic/restic.go`; `truncateErr`, 500 chars, `internal/backup/orchestrator.go`).

**Metrics:**
- `GET /metrics` — Prometheus text format written by hand (`internal/api/metrics.go`), self-gated by a bearer token.

**Watchdog:** overdue-backup watchdog (`internal/api/watchdog.go`, `watchdog_state` table); DR drills and tamper tests recorded in `restore_drills`/`tamper_tests`.

## CI/CD & Deployment

**Hosting:** Self-hosted on Unraid/TrueNAS/generic Docker via the published image. Not a cloud deployment.

**CI Pipeline (`.github/workflows/`):**
- `lint.yml` — jobs: `go` (build/vet/golangci-lint), `web` (npm ci, tsc, eslint, vitest), `test` (installs restic 0.17.3 from upstream with SHA256 check, then `go test ./...`).
- `build.yml` — `smoke` job (amd64 boot test: must serve `/api/health`; tini must be PID 1; non-blocking Trivy SARIF scan) gates the multi-arch build+push with SLSA provenance + SBOM attestations. Push only on `main`/tags. `:latest` is moved by branch builds only, never tag builds (2026-09-02 v8.3.0 re-cut incident).
- `docs.yml` — MkDocs Material → GitHub Pages. `dockerhub-description.yml` — manual credential-isolation probe. `repo-watch-instant.yml` — Matrix dispatch.

## Environment Configuration

**Required env vars (runtime):**
- `APP_KEY` — master key; loss is unrecoverable (encrypted blobs + derived repo passwords).
- `DATA_DIR` (=/config), `PORT`, `HTTPS_PORT`, `TZ`, `PLATFORM`, `HTTP_ONLY`, `HOST_SOURCE_ROOT`/`HOST_MOUNT_ROOT`, `BACKUP_MAX_HOURS`, `LIBVIRT_*`.

**CI secrets/vars:** `GITHUB_TOKEN` (implicit), `DOCKERHUB_TOKEN` + repo variable `DOCKERHUB_USERNAME` (both required for the mirror), `REPOWATCH_PAT`.

**Secrets location:**
- Runtime secrets live **encrypted in SQLite** (AES-256-GCM via `internal/secret`, keyed by `APP_KEY`): `RcloneConf`, `NotifyConf`, `CloudConf`, `CloudCredSets`, `RegistryAuths`, `TOTPSecret` blobs on the settings row. Never in the repo tree (gitleaks-enforced; narrow allowlist in `.gitleaks.toml`).
- UI contract: settings GETs return secrets as `""` plus a `*Set` flag; blank field on save keeps the stored value.

## Webhooks & Callbacks

**Incoming:**
- Peer fleet/mesh calls: `GET /api/fleet/status`, `POST /api/fleet/mesh-offer` (fleet-token gated, `internal/api/fleet.go`, `internal/api/mesh.go`).
- No third-party webhook receivers otherwise.

**Outgoing:**
- Notification channels above (webhook/Matrix/SMTP/Apprise), Healthchecks.io pings, Unraid notify over SSH, repo-watch `repository_dispatch` from CI.

---

*Integration audit: 2026-09-09*
