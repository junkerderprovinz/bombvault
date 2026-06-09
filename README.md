<h1 align="center">BombVault</h1>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner.png" alt="BombVault" width="100%">
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=githubactions&logoColor=white" alt="Lint" height="36"></a>&nbsp;
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault"><img src="https://img.shields.io/badge/Image-GHCR-24292e?style=for-the-badge&logo=github&logoColor=white" alt="GHCR" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://restic.net"><img src="https://img.shields.io/badge/Engine-restic-CE4844?style=for-the-badge&logoColor=white" alt="restic" height="36"></a>&nbsp;
  <a href="https://unraid.net"><img src="https://img.shields.io/badge/Unraid-Template-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<br>

<p align="center">
Your Unraid data, <b>locked in a vault</b>. Armed with a fuse.<br>
BombVault backs up Docker containers, KVM VMs, appdata and the Unraid flash config —
and restores everything with a single click. Containers <b>automatically reappear in the
Docker tab</b>, VMs <b>automatically in the VM tab</b> — no manual reinstall, no
reconfiguration, no drama.<br>
<br>
Drop a backup. Detonate a restore. <b>Data loss doesn't stand a chance.</b><br>
Powered by <a href="https://restic.net">restic</a> — deduplicated, incremental, always encrypted.
</p>

<br>

<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz">
    <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" height="40">
  </a>
</p>

<br>

## Table of Contents

1. [What is this?](#1-what-is-this)
2. [Status](#2-status)
3. [How it works](#3-how-it-works)
4. [Security / trust model](#4-security--trust-model)
5. [Host integration spike](#5-host-integration-spike)
6. [Requirements](#6-requirements)
7. [Install on Unraid](#7-install-on-unraid)
8. [Configuration](#8-configuration)
9. [Development](#9-development)
10. [Support this project](#10-support-this-project)

<br>

## 1. What is this?

BombVault is a self-hosted, **Unraid-native** web app for **backup and full disaster recovery**
of your Docker containers — built as a **single static Go binary with an embedded React UI** and
**[restic](https://restic.net)** as the storage engine.

You run one container, open a modern dark web UI, point it at the paths you want backed up, and
it handles the lifecycle: back up a container's appdata + definition with one click, then restore
it just as easily — the container is reinstalled and restarted so it **reappears in the Unraid
Docker tab exactly as before**. No manual reinstall, no reconfiguration.

Everything — backup paths, encryption, schedule — is configured **in the app**, not in the
container template. The template only needs the master key and the mounts.

The architecture (Go API + embedded React SPA, restic engine, SQLite state, Docker SDK over the
mounted socket) is documented in detail in
[`docs/superpowers/specs/2026-06-08-bombvault-go-rewrite-design.md`](docs/superpowers/specs/2026-06-08-bombvault-go-rewrite-design.md).

Inspired by [VolumeVault](https://github.com/Darkdragon14/VolumeVault) (Apache-2.0), built fresh
with restic as the engine.

<br>

## 2. Status

**Phase 1 — one-click container backup & restore.**

- A single static **Go binary** serves the JSON API **and** the embedded **React** UI; **restic**
  is the backup engine and **SQLite** holds state.
- Backup paths, **encryption on/off**, and a **per-domain schedule** are set **in the app**
  (Settings), not in the template.
- A built-in **host integration spike** ("Check now") probes the host for the pieces a backup
  needs (see below).
- **VMs and the Unraid flash config are later phases** — their toggles exist in Settings but show
  a "coming in a later phase" placeholder for now.

> [!IMPORTANT]
> **No authentication yet.** BombVault is a **trusted-LAN** tool. Optional built-in auth (and a
> non-root runtime) is on the backlog before any public-facing release. See
> [Security / trust model](#4-security--trust-model).

<br>

## 3. How it works

```
Browser ──HTTPS──> BombVault container (single Go binary)
                   ├─ Go HTTP server: JSON API + embedded React SPA (embed.FS)
                   ├─ SQLite state (settings, targets, run history)
                   ├─ In-process scheduler (per-domain cadence)
                   │
                   ├─ /var/run/docker.sock  ─> Docker API via the official Go SDK
                   │                            (inspect / stop / remove / recreate)
                   ├─ /host/user            ─> your Unraid share root: backup SOURCES
                   │                            and DESTINATIONS (subpaths chosen in the UI)
                   └─ restic                ─> the bundled engine (deduplicated, incremental)
```

BombVault is the **orchestration and UI layer**, not the storage engine. To back up a container
it talks to the Docker socket (via the official Go SDK — no `docker` CLI) to stop the container,
runs **restic** against the appdata path you chose, captures the container definition, and
**always restarts the container** afterwards even if the backup failed.

Restore is the star: BombVault restores the appdata back to its origin, then replays the saved
container definition against the Docker API so the container reappears in the Unraid Docker tab as
if it had always been there — same image, env, ports, volumes, and the security-relevant fields
(user, caps, privileged, network mode, devices) preserved.

All data movement goes through restic. With encryption **on**, the repo password is derived from
your `APP_KEY` (HMAC-SHA256, domain-separated) — nothing to type, recoverable as long as you keep
the key. With encryption **off**, restic runs `--insecure-no-password`. The encryption mode is
fixed per repo at init time.

<br>

## 4. Security / trust model

> [!WARNING]
> **BombVault has no built-in authentication yet.** It is a trusted-LAN tool that holds
> **root-equivalent control of the host**: through the mounted `docker.sock` it can stop, remove
> and recreate containers, and through the broad `/mnt/user` mount it reads appdata for backup and
> writes it back on restore. Anyone who can reach its web UI effectively has root on the host.

Run BombVault **only on a trusted, non-exposed network** — never publish it directly to the
internet. If you need remote access, put it **behind a reverse proxy that adds authentication**
(and TLS). The broad host mount needed for in-app path selection adds little marginal risk because
`docker.sock` already grants root-equivalent access.

An optional built-in authentication layer and a non-root runtime are planned **before any
public-facing release**. API error messages are scrubbed of secrets and filesystem paths.

<br>

## 5. Host integration spike

Real Docker and Unraid behavior can't be tested in CI (no Docker socket, no Unraid on the
runners). Instead, BombVault ships a **host integration spike**: a panel under
**Settings → "Check now"** (also `POST /api/spike`) that probes the host the container is actually
running on.

It checks:

- **docker.sock** reachable, **restic** present (version ≥ 0.17), and the chosen **backup path**
  readable/writable — these **gate** backups.
- **libvirt/qemu**, **rclone** — best-effort probes for later phases.

Each probe reports **OK / FAIL** with detail. Failures are findings, never crashes — run it after
first start to confirm your mounts are wired correctly.

<br>

## 6. Requirements

| Requirement | Notes |
|---|---|
| **Unraid 6.12+** | Earlier versions not tested |
| **Docker socket** | Mounted by the template (`/var/run/docker.sock`) — root-equivalent host control |
| **Backup storage** | A subpath under your share root (chosen in the UI); local array/cache recommended |
| **APP_KEY** | 64 hex chars (`openssl rand -hex 32`) — master key for secrets + the derived restic password |

<br>

## 7. Install on Unraid

Install via **Community Applications** — search for **BombVault**.

Or add the template manually:

1. In Unraid, go to **Docker → Add Container → Template repositories** and add:
   ```
   https://github.com/junkerderprovinz/unraid-docker-templates
   ```
2. Search for **BombVault** in Templates.
3. Set `APP_KEY` (generate with `openssl rand -hex 32`), confirm the mounts, and click **Apply**.
4. Open the WebUI (HTTPS on `3443`), go to **Settings**, pick your backup paths + encryption +
   schedule, and run **Check now**.

<br>

## 8. Configuration

Only the master key and the mounts are template-level. **Paths, encryption and schedule are set
in the WebUI** (Settings) — they are no longer container variables.

| Setting | Where | Description |
|---|---|---|
| `APP_KEY` | Template (required) | 64 lowercase hex chars (`openssl rand -hex 32`). Master key for secrets and the derived restic password. **Keep it safe** — losing it makes encrypted backups unrecoverable. |
| Config volume | Template (`/config`) | BombVault's own state: SQLite DB, TLS cert, restic cache. |
| Host data mount | Template (`/host/user` ← `/mnt/user`) | Your share root. Backup sources **and** destinations live here; pick the exact subpaths in the WebUI. |
| Docker socket | Template (`/var/run/docker.sock`) | Container control around backup/restore. |
| `HTTP_ONLY` | Template (advanced) | Set `true` to serve plain HTTP behind a TLS-terminating reverse proxy. |
| Backup paths | **WebUI → Settings** | Per-domain subpath under the host mount. |
| Encryption on/off | **WebUI → Settings** | Fixed per repo at init time. On → password derived from `APP_KEY`. |
| Schedule | **WebUI → Settings** | Per-domain cadence (off / daily / weekly / cron). |

Ports: **HTTPS on `3443`** (primary, self-signed cert out of the box) and **HTTP on `3000`**
(only used with `HTTP_ONLY` behind a reverse proxy).

<br>

## 9. Development

**Go (API + binary):**

```bash
go test ./...                              # unit tests (mocked Docker + restic, real restic roundtrip)
APP_KEY=$(openssl rand -hex 32) go run ./cmd/bombvault
```

**Web (React UI):**

```bash
npm --prefix web install
npm --prefix web run dev                   # Vite dev server
npm --prefix web run build                 # build web/dist (embedded into the binary)
```

The Go binary embeds `web/dist`, so for a production-like run build the web bundle first, then
`go run`/`go build`. Real Docker and Unraid behavior can't be exercised in CI — use the
**[host integration spike](#5-host-integration-spike)** (Settings → "Check now") on your actual
Unraid host to validate mounts before submitting a PR.

<br>

## 10. Support this project

<a href="https://buymeacoffee.com/junkerderprovinz">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" height="40">
</a>
