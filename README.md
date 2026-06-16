<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner.png" alt="BombVault" width="100%">
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=githubactions&logoColor=white" alt="Lint" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/pulls/junkerderprovinz/bombvault?style=for-the-badge&logo=docker&logoColor=white&label=Pulls&color=1d99f3" alt="Docker Pulls" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/image-size/junkerderprovinz/bombvault/latest?style=for-the-badge&logo=docker&logoColor=white&label=Size&color=1d99f3" alt="Image Size" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://restic.net"><img src="https://img.shields.io/badge/Engine-restic-CE4844?style=for-the-badge&logoColor=white" alt="restic" height="36"></a>&nbsp;
  <a href="https://unraid.net"><img src="https://img.shields.io/badge/Unraid-Template-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<br>

<p align="center">
Your Unraid data, <b>sealed in a vault</b>. Armed with a fuse.<br>
BombVault backs up Docker containers, KVM VMs, appdata and the Unraid flash config —
and restores everything with a single click. Containers <b>automatically reappear in the
Docker tab</b>, VMs <b>automatically in the VM tab</b> — no manual reinstall, no
reconfiguration, no drama.<br>
<br>
<b>Your data, locked in. Loss, locked out.</b> Data loss doesn't stand a chance.<br>
Powered by <a href="https://restic.net">restic</a> — deduplicated, incremental, always encrypted.
</p>

<br>

<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz">
    <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" width="220">
  </a>
</p>

<br>

<p align="center">
  <b>Status:</b> one-click <b>Docker container</b>, <b>KVM/libvirt VM</b> and <b>Unraid flash</b> backup &amp; restore are all live (VMs over SSH — no libvirt mount). Off-site repos (SMB/NFS/rclone), retention, file-level restore, integrity verification and hooks are on the <b>roadmap</b> — marked <i>(planned)</i> below.
</p>

<br>

## Table of Contents

1. [What is this?](#1-what-is-this)
2. [Features](#2-features)
3. [How it works](#3-how-it-works)
4. [Security / trust model](#4-security--trust-model)
5. [Requirements](#5-requirements)
6. [Install on Unraid](#6-install-on-unraid)
7. [Configuration](#7-configuration)
8. [Development](#8-development)
9. [Support this project](#9-support-this-project)

<br>

## 1. What is this?

BombVault is a self-hosted, **Unraid-native** web app for **backup and full disaster recovery** of your Docker containers and KVM/libvirt VMs. It runs as a single Docker container, gives you a modern dark web UI, and handles the whole lifecycle:

- **Backs up** Docker appdata + container definitions, KVM/libvirt VM disks + XML (incl. UEFI NVRAM), and the whole Unraid flash (`/boot`).
- **Restores automatically** — containers are reinstalled and restarted so they reappear in the Docker tab exactly as before, and VMs are re-defined in the VM Manager with their disks + NVRAM reattached.
- **Schedules** incremental backups in the background (per domain), so you never have to think about it.

Inspired by [VolumeVault](https://github.com/Darkdragon14/VolumeVault) (Apache-2.0), built fresh with restic as the engine.

<br>

## 2. Features

### Backup scope

| What | What is saved |
|---|---|
| **Docker containers** | Appdata directory + container definition (image, env vars, ports, labels, volumes) |
| **KVM / libvirt VMs** | VM disk image(s) + XML definition + UEFI NVRAM (graceful-shutdown or live-snapshot, over SSH) |
| **Unraid flash** | The whole USB flash (`/boot`) — OS, license, array config, shares, network + plugin config. Restore extracts to a folder (never overwrites the live flash) |

### Restore (the good part)

- **One-click full restore** — pick a snapshot, click Restore. Done.
- **Containers are automatically reinstalled**: the container definition is replayed against the Docker API so the container reappears in the Unraid Docker tab exactly as it was — same image, same settings, same port mappings.
- **VMs are automatically recreated**: the XML definition is re-imported over SSH so the VM reappears in the VM Manager with its disk + UEFI NVRAM reattached, even after the VM was deleted.
- **Individual restore** — restore one container or one VM without touching the others.
- **Flash restore is safe** — a flash snapshot is *extracted to a folder* you then copy onto a fresh USB; the live, running `/boot` is never overwritten (which could leave the server unbootable).
- **Pre-flight conflict check** — before anything is stopped or removed, restore verifies the container's static IP and published host ports are free; if another container already holds one, it aborts with a clear, actionable message instead of leaving you with a half-finished restore.
- **File-level restore** *(planned)* — browse a snapshot and restore individual files.

### Storage & scheduling

- Incremental, deduplicated backups via restic — even large VM disks don't balloon the repo.
- Destinations: a **local path** today; SMB/CIFS, NFS and rclone (B2, S3, …) plus per-type destinations are *(planned)*.
- Configurable retention (keep N snapshots / by age) *(planned)*.
- Per-domain scheduling (daily / weekly / cron); per-backup-group scheduling is *(planned)*.

### Other

- Snapshot browser with a restore-point list.
- Integrity verification (`restic check`) *(planned)*.
- Pre/post backup hooks per container or VM *(planned)*.
- HTTPS out of the box (self-signed, or BYO cert behind a reverse proxy).
- Dark/light UI in English + German today; more locales *(planned)*.

<br>

## 3. How it works

```
Browser ──HTTPS──> BombVault container
                   ├─ Go binary: JSON API + embedded React UI
                   ├─ Background worker (per-domain scheduler + job executor)
                   │
                   ├─ /var/run/docker.sock  ─> Docker API (container stop/inspect/recreate)
                   ├─ qemu+ssh://host       ─> libvirt / KVM on the HOST over SSH (no mount)
                   ├─ /mnt/user/            ─> appdata, VM disks + restic repos (read/write)
                   ├─ /boot/ ─> /host/boot  ─> Unraid flash backup (whole USB)
                   └─ <repo path>           ─> restic repository (local; remote planned)
```

BombVault talks to the Docker socket to stop containers before backup and recreate them after restore. For VMs it runs `virsh` **on the host over SSH** (`qemu+ssh://`) to gracefully shut down or live-snapshot a domain — it never bind-mounts any libvirt path, so it can never interfere with the host VM Manager. All actual data movement goes through **restic** — BombVault is the orchestration and UI layer, not the storage engine.

Restore is the star: after copying data back from the restic snapshot, BombVault replays the saved container definition against the Docker API (`docker run` equivalent), so the container reappears in the Unraid Docker tab as if it had always been there. VMs get their XML re-defined over SSH and their disks + UEFI NVRAM reattached.

<br>

## 4. Security / trust model

> [!WARNING]
> **BombVault holds root-equivalent control of the host**: via the Docker socket it can
> stop, remove and recreate containers and reads/writes appdata, and for VM backup it logs
> in to the host over SSH (`qemu+ssh://`, root by default) to run `virsh`. Anyone who can
> reach its web UI effectively has root on the host.

BombVault has **optional built-in password protection** (Settings → Security): set a password
to require login, clear it to disable. It is **off by default** for trusted-LAN use. Sessions are
signed (HMAC, derived from `APP_KEY`) and changing the password invalidates them. Regardless,
run BombVault **only on a trusted, non-exposed network** — never publish it directly to the
internet; for remote access put it behind a reverse proxy that adds authentication and TLS.
Responses carry baseline security headers (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).

Backups are encrypted by restic when encryption is enabled (Settings; on by default), with the
key derived from `APP_KEY`.

<br>

## 5. Requirements

| Requirement | Notes |
|---|---|
| **Unraid 6.12+** | Earlier versions not tested |
| **Restic repo location** | Local path (recommended: your array or cache), SMB, NFS, or any rclone backend |
| **Docker socket** | Mounted by the template automatically (`/var/run/docker.sock`) |
| **Unraid flash** (`/boot`) | Mounted whole by the template automatically (`/boot` → `/host/boot`). Powers Flash backup of the entire USB, and lets a restored container reappear as a normal, editable Unraid app (via the templates folder under `/boot`) instead of a "third-party" container |
| **KVM VMs** *(opt-in)* | VM backup talks to libvirt **over SSH** — no libvirt mount. Set it up in Settings → see below |

> [!IMPORTANT]
> **VM backup uses SSH, not a libvirt mount.** BombVault never bind-mounts any
> libvirt path (mounting the host's libvirt socket/runtime on Unraid is fragile —
> the VM Manager owns those paths and toggling "Enable VMs" can leave libvirt
> unable to start). Instead it runs `virsh` **on the host over SSH**
> (`qemu+ssh://`), so it can never affect your host VM Manager. Setup:
> **Settings → VM Backup over SSH** → copy the shown public key → append it to
> Unraid's `/root/.ssh/authorized_keys` → click **Test connection**. The template
> adds `--add-host=host.docker.internal:host-gateway` so the container reaches the
> host; set `LIBVIRT_HOST` to your Unraid LAN IP if that name doesn't resolve (e.g.
> when the container runs on a custom `br0.x` network). If you changed Unraid's SSH
> port, set `LIBVIRT_SSH_PORT` to match (default `22`). The
> SSH key grants root on the host — the same trust level as the docker.sock
> BombVault already uses. **Live snapshots** additionally need the qemu guest
> agent in the VM and the disk on `/mnt/cache` (not `/mnt/user`).
>
> **Full setup + networking guide:** [docs/vm-backup-ssh-setup.md](docs/vm-backup-ssh-setup.md).

<br>

## 6. Install on Unraid

Install via **Community Applications** — search for **BombVault**.

Or add the template manually:

1. In Unraid, go to **Docker → Add Container → Template repositories** and add:
   ```
   https://github.com/junkerderprovinz/unraid-docker-templates
   ```
2. Search for **BombVault** in Templates.
3. Set the required variables (see [Configuration](#7-configuration)) and click **Apply**.

<br>

## 7. Configuration

| Variable | Required | Description |
|---|---|---|
| `APP_KEY` | **Yes** | 32-byte hex secret (64 hex chars) used to derive the restic repo password. Generate with `openssl rand -hex 32`. **Keep this safe** — losing it makes encrypted backups unrecoverable. |
| `LIBVIRT_HOST` | For VMs | Unraid host reached over SSH for VM backup (default `host.docker.internal`; set to the host LAN IP on a custom `br0.x` network). |
| `LIBVIRT_SSH_PORT` | No | Host SSH port for VM backup (default `22`). |
| `LIBVIRT_SSH_USER` | No | SSH user on the host for VM backup (default `root`). |
| `PORT` | No | HTTP port (default `3000`). |
| `HTTPS_PORT` | No | HTTPS port (default `3443`; the template maps it to `3001`). |
| `HTTP_ONLY` | No | Set `true` to disable the self-signed HTTPS listener and serve plain HTTP only. |
| `TZ` | No | Timezone for the scheduler (e.g. `Europe/Berlin`). |

Mount the Docker socket, your appdata and a backup directory as shown in the CA template. **Backup repository paths are configured in the app** (Settings → Backup paths) — not via env — and default to `/mnt/user/bombvault/{container,vms,flash}`. VM backup needs no mount: see [§5](#5-requirements) and [docs/vm-backup-ssh-setup.md](docs/vm-backup-ssh-setup.md).

> [!NOTE]
> **Host integration check:** open `/spike` in the web UI after the container starts. It probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces.

<br>

## 8. Development

BombVault is a single static **Go** binary that serves a JSON API and an embedded
React/Vite SPA (`go:embed`). Build the SPA first, then run the binary:

```bash
npm --prefix web ci
npm --prefix web run build     # outputs web/dist (embedded into the binary)
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # Go unit + integration tests (real restic roundtrip)
golangci-lint run ./...        # lint
go run ./cmd/bombvault         # serves https://localhost:3443 (self-signed cert)
```

Real Docker, libvirt and Unraid behavior cannot be tested in CI (no KVM, no Unraid on runners). Use the **host integration check** (`/spike` in the web UI) to validate mounts, restic and the VM SSH connection on your actual Unraid host before submitting a PR.

<br>

## 9. Support this project

<a href="https://buymeacoffee.com/junkerderprovinz">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" height="40">
</a>
