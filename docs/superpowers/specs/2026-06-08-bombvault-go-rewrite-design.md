# BombVault 2.0 — Go Rewrite Design

> Status: **approved** (2026-06-08). Supersedes the Next.js/TypeScript implementation
> (tagged `ts-final`). This document is the source of truth for the implementation plan.

## 1. Goal

Rewrite BombVault from scratch in **Go** as a single static binary, keeping **restic** as
the backup engine. Deliver a modern, tidy web UI where backup **storage paths, encryption,
schedule, and which domains (Container / VM / Flash) are active** are all configured **in the
app** — not baked into the Unraid template. Same-host disaster recovery: a restored container
reappears in the Docker tab with no manual steps.

### Why Go
The backend is Go's home turf: the **official Docker SDK** is Go, **restic** is Go, libvirt has
Go bindings, and the result is a tiny, robust single-binary image with none of the Next.js/
Turbopack/Edge boot fragility that the TS version repeatedly hit. The modern UI is preserved by
embedding a **React SPA** into the binary (`embed.FS`).

### Non-goals (this spec = Phase 1)
VM backup, Flash backup, off-site backends (S3/SFTP/WebDAV), retention/pruning policy,
file-level restore, `restic check` verify, pre/post hooks, notifications, and authentication
are **later phases** (the UI reserves their toggles/tabs but Phase 1 implements **containers
only**). No authentication yet (trusted-LAN; optional auth is a pre-public backlog item).

## 2. Architecture

- **One static binary.** Go HTTP server serves a JSON API **and** the embedded React SPA
  (`embed.FS`). Built `CGO_ENABLED=0`; image is `scratch`/distroless + the bundled host CLIs.
  Multi-arch amd64 + arm64.
- **Engine: restic CLI**, pinned **≥ 0.17** (needs `--insecure-no-password` for the
  encryption-off mode). Password (encryption-on) passed via `RESTIC_PASSWORD` env, never argv.
- **State: SQLite** via `modernc.org/sqlite` (pure-Go driver → no cgo, no native-build pain).
- **Host control:** Docker via the official SDK (`github.com/docker/docker/client`) over the
  mounted `docker.sock`; restic/qemu-img/rclone as bundled CLIs; libvirt later.
- **Frontend:** React + Vite + TypeScript, Tailwind CSS, IBM-Carbon monochrome dark/light
  theme, i18n (ported keys). Built to static assets, embedded.

## 3. Module structure

### Go (`/`)
- `cmd/bombvault/main.go` — wiring: load config, open DB, run migrations, start scheduler,
  start HTTP server (API + embedded SPA), print ASCII init banner + "READY" log banner.
- `internal/store` — SQLite open/migrate; typed repositories (`settings`, `targets`, `runs`).
  Forward-only migrations.
- `internal/config` — process config (APP_KEY, data dir, host-mount root, ports) from env +
  the runtime **settings** persisted in the DB (encryption flag, paths, schedule, domain
  toggles, default language).
- `internal/dockercli` — thin wrapper over the Docker SDK behind a `Docker` interface
  (list/inspect/stop/start/remove/pull/create) for mockable tests.
- `internal/restic` — argv builders + run helpers: `Init`, `Backup`, `Restore`, `Snapshots`,
  `Stats`; supports password and `--insecure-no-password`; `--` end-of-options guard;
  errors scrubbed of paths/secrets.
- `internal/restickey` — derive the restic password from APP_KEY (HMAC-SHA256, domain-separated).
- `internal/backup` — orchestrator: `BackupContainer` (stop → backup → **always** start →
  record run; re-throw on failure) and `RestoreContainer` (confirm → pull → stop → remove →
  `restore --target /` → write template → recreate → record run). Dependency-injected.
- `internal/template` — read/write Unraid container template XML (`my-<Name>.xml`).
- `internal/schedule` — in-process scheduler (cron/ticker) reading the schedule from settings.
- `internal/spike` — host-integration probes (graceful degradation).
- `internal/api` — HTTP handlers (JSON), request validation, error mapping.
- `web/` — the React SPA (built + embedded).

### React (`web/`)
- `src/app` — router + layout (sidebar shell, top bar with theme + language).
- `src/pages` — Dashboard, Containers, (VMs, Flash — later), Settings.
- `src/components` — table, toggle, button-with-inline-result, spike panel.
- `src/lib` — API client, i18n, theme.

## 4. Data model (SQLite, forward-only migrations)

- **`settings`** — single-row (or key/value) app settings:
  `encryption_enabled` (bool),
  `containers_schedule` / `vms_schedule` / `flash_schedule` (each: off | daily HH:MM | weekly DOW HH:MM | cron expr),
  `containers_enabled` / `vms_enabled` / `flash_enabled` (bool),
  `containers_path` / `vms_path` / `flash_path` (subpaths under the host-mount root),
  `default_language`.
- **`targets`** — one row per container that has been backed up or marked for scheduling:
  `id`, `container_name` (UNIQUE), `appdata_paths` (JSON), `include_in_schedule` (bool),
  `created_at`.
- **`runs`** — backup/restore history: `id`, `target_id`, `kind`, `status`, `started_at`,
  `finished_at`, `snapshot_id`, `bytes`, `error`.

`APP_KEY` remains the master secret (drives the encryption-on key derivation and any future
stored credentials). Lose it and encrypted backups are unrecoverable (documented).

## 5. HTTP API (JSON)

- `GET /api/containers` → list with status + last-backup + include flag.
- `POST /api/containers/{name}/backup` → run a backup now; returns `{ok, snapshotId|error}`.
- `GET /api/containers/{name}/snapshots` → snapshots for the container.
- `POST /api/containers/{name}/restore` → `{snapshotId, confirm}`; returns `{ok|error}`.
- `PATCH /api/containers/{name}` → set `include_in_schedule`.
- `GET /api/settings` / `PUT /api/settings` → read/update settings.
- `POST /api/spike` → run the host checks; returns results.
- `GET /api/runs` → run history (dashboard).
- `GET /api/health` → liveness.

All mutating endpoints return a structured `{ok:false, error}` on failure; the SPA renders
errors **inline** (never a full-page crash). Errors are scrubbed of secrets/paths.

## 6. Configuration semantics

- **Encryption toggle** (`encryption_enabled`):
  - **On** → restic repo initialised/used with a password **derived from APP_KEY** (transparent;
    no passphrase to type; recoverable while APP_KEY is kept).
  - **Off** → restic `--insecure-no-password` (repo not password-protected).
  - The toggle is fixed **per repo at init time**: changing it after a repo exists requires a
    new/empty path (we detect and warn — restic can't switch a repo's encryption in place).
- **In-app paths:** the container mounts the host **once** at a broad root (default host
  `/mnt/user` → container `/host/user`). In Settings you pick a **subpath per domain**
  (e.g. `backups/bombvault/containers`). BombVault creates the folder and `restic init`s it.
  Paths are validated to stay **within** the mount root (no traversal).
- **Schedule (per domain):** each domain has its **own** schedule, configured in that domain's
  tab (not a single global cadence). The scheduler runs each enabled domain on its own cadence;
  for containers it backs up every container with `include_in_schedule = true`. (Per-*container*
  overrides within a domain = later.)
- **Domain toggles:** `containers/vms/flash_enabled`; enabling one reveals its sidebar tab.
  Phase 1 implements **containers**; VM/Flash toggles exist but their tabs show a
  "coming in a later phase" placeholder until built.

## 7. Backup flow (ported)

`recordRunStart → (stop → restic backup [in-app path, auto-init, password per toggle] →
capture template) → finally always-start → recordRunFinish(success|failed) → re-throw on fail.`
The container is **guaranteed** to restart even if the backup throws. Tags: `container:<name>`.

## 8. Restore flow (ported + SEC fixes)

`confirm gate → pull image → stop → remove → restic restore --target / → write template to
flash → recreate+start via Docker SDK → record run.` Ported safeguards: `--`+hex snapshot-id
guard (arg injection), template-path containment (traversal), recreate **preserves**
security-relevant fields (User/Cap*/Privileged/SecurityOpt/ReadonlyRootfs/NetworkMode/Devices),
and a **live container re-check** before the destructive stop/remove (wrong-target guard).
`--target /` is mandatory so absolute appdata paths land back at origin (the TS SEC-102 lesson).

## 9. Scheduler

In-process (Go `time` ticker evaluating **each domain's** cadence every minute). On a domain's
trigger: back up that domain's due items sequentially (one restic run at a time) — for
containers, every `include_in_schedule` container — and record each run. Survives restarts
(per-domain schedules live in the DB; next due times recomputed on boot).

## 10. Host-integration spike

Settings → **"Check now"** runs probes: docker.sock reachable, libvirt reachable, `restic`
present (+version ≥0.17), `qemu-img`, `rclone`, and the chosen backup path readable/writable.
Each returns OK / FAIL + detail; failures are findings, never crashes. A short paragraph
explains what the spike is and why it matters.

## 11. Security / trust model

Trusted-LAN tool with **root-equivalent host control** via docker.sock (stop/remove/recreate
containers, read/write appdata, write the flash template). **No authentication yet** — run only
on a non-exposed network or behind an authenticating reverse proxy; optional built-in auth is a
pre-public backlog item. The broad host mount needed for in-app paths adds little marginal risk
because docker.sock already grants root-equivalent access. Clear warnings in UI + README +
template. Secrets/paths are scrubbed from API error messages.

## 12. Deployment / Unraid template

Single-binary image to **GHCR + Docker Hub**, multi-arch. The template gets **simpler**:
- `APP_KEY` (required, 64 hex).
- **One broad mount**: host `/mnt/user` → container `/host/user` (rw) — backup storage.
- `docker.sock` (rw), `libvirt-sock` (later).
- A small **config volume** (`/config`: SQLite DB, restic cache, self-signed cert).
- Ports: HTTPS (3443) primary, HTTP (3000) fallback (`HTTP_ONLY`).
- Boot (appdata read for backup) is covered by the broad mount.
Paths / encryption / schedule are **no longer template fields** — set them in the UI.
The image prints the ASCII init banner and a loud **"BOMBVAULT IS READY"** log banner.

## 13. Testing / CI

- **Go unit tests:** restic argv builders, `restickey` derivation, orchestrator with mocked
  Docker + restic (via interfaces), scheduler timing, path-containment validation, settings
  store, template read/write.
- **Real restic roundtrip** in CI (install restic on the runner): init → backup → restore.
- **React:** typecheck + build.
- **CI jobs:** `golangci-lint`, `go test`, React build, multi-arch image build (GHCR push on
  `main`). `paths-ignore` docs on the build job only.
- **Box-gate (user):** the spike + a real container backup→restore on the actual Unraid host
  (CI has no Docker/KVM).

## 14. Repo strategy & phasing

- **Same repo** `bombvault`. The TS final state is tagged **`ts-final`**. Go is built on
  branch **`feat/go-rewrite`**; once container backup/restore reaches parity and the box-gate
  passes, Go is merged to `main` (the TS tree is replaced; history + the tag remain).
- **Phase 1 (this spec):** Go foundation, embedded React UI shell (sidebar, theme, i18n),
  Settings (encryption toggle, in-app paths, schedule, domain toggles, spike), **container
  backup + restore**, scheduler, CI, template, README.
- **Phase 2:** VM backup/restore (graceful shutdown / live snapshot).
- **Phase 3:** Flash backup.
- **Later:** off-site backends, retention/prune, file-level restore, `restic check`, hooks,
  notifications, optional authentication, non-root container.

## 15. Decisions locked

- **Scheduling is per domain** — each domain tab has its own cadence (`containers_schedule`,
  `vms_schedule`, `flash_schedule`). Per-*container* overrides within a domain = later.
- **HTTPS by default** — self-signed cert generated at first boot (openssl bundled); HTTP-only
  is the `HTTP_ONLY` fallback for running behind a TLS-terminating reverse proxy.
- Styling: Tailwind with the IBM-Carbon monochrome palette.
