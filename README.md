<h1 align="center">BombVault</h1>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner.png" alt="BombVault" width="100%">
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=githubactions&logoColor=white" alt="Lint" height="36"></a>&nbsp;
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

- **Backs up** appdata directories, container definitions, VM disks and XML, and the Unraid flash config.
- **Restores automatically** — containers are reinstalled and restarted so they reappear in the Docker tab exactly as before. VMs are recreated and show up in the VM tab with no manual steps.
- **Schedules** incremental backups in the background so you never have to think about it.

Inspired by [VolumeVault](https://github.com/Darkdragon14/VolumeVault) (Apache-2.0), built fresh with restic as the engine.

<br>

## 2. Features

### Backup scope

| What | What is saved |
|---|---|
| **Docker containers** | Appdata directory + container definition (image, env vars, ports, labels, volumes) |
| **KVM / libvirt VMs** | VM disk image(s) + XML definition |
| **Unraid flash config** | The USB flash content — complete OS + plugin config |

### Restore (the good part)

- **One-click full restore** — pick a snapshot, click Restore. Done.
- **Containers are automatically reinstalled**: the container definition is replayed against the Docker API so the container reappears in the Unraid Docker tab exactly as it was — same image, same settings, same port mappings.
- **VMs are automatically recreated**: the XML definition is re-imported so the VM reappears in the VM Manager with its disk attached.
- **Individual or group restore** — restore one container/VM without touching the others.
- **File-level restore** — browse a snapshot and restore individual files.

### Storage & scheduling

- Incremental, deduplicated backups via restic — even large VM disks don't balloon the repo.
- Multiple destinations: local path, SMB/CIFS, NFS, rclone (B2, S3, etc.). Per-type destinations (containers, VMs, flash can each go to a different target).
- Configurable retention (keep N snapshots / by age).
- Cron-based scheduling, per backup group.

### Other

- Snapshot browser with restore-point timeline.
- Integrity verification (`restic check`).
- Pre/post backup hooks per container or VM.
- HTTPS out of the box (self-signed, or BYO cert behind a reverse proxy).
- 26-language UI (i18next).

<br>

## 3. How it works

```
Browser ──HTTPS──> BombVault container
                   ├─ Next.js UI + API routes
                   ├─ Background worker (scheduler + job executor)
                   │
                   ├─ /var/run/docker.sock  ─> Docker API (container stop/inspect/recreate)
                   ├─ /var/run/libvirt/     ─> libvirt / KVM (VM snapshot/restore)
                   ├─ /mnt/user/appdata/    ─> container appdata (read/write)
                   ├─ /boot/               ─> Unraid flash (read/write)
                   └─ <repo mount>         ─> restic repository (local or remote)
```

BombVault talks to the Docker socket to stop containers before backup and recreate them after restore. It talks to libvirt to quiesce or snapshot VMs. All actual data movement goes through **restic** — BombVault is the orchestration and UI layer, not the storage engine.

Restore is the star: after copying data back from the restic snapshot, BombVault replays the saved container definition against the Docker API (`docker run` equivalent), so the container reappears in the Unraid Docker tab as if it had always been there. VMs get their XML re-imported into libvirt and their disks reattached.

<br>

## 4. Security / trust model

> [!WARNING]
> **BombVault has no built-in authentication yet.** It is a trusted-LAN tool that holds
> **root-equivalent control of the host** via the Docker and libvirt sockets: it can stop,
> remove and recreate containers and VMs, and reads/writes appdata and the Unraid flash config.
> Anyone who can reach its web UI effectively has root on the host.

Run BombVault **only on a trusted, non-exposed network** — never publish it directly to the internet. If you need remote access, put it **behind a reverse proxy that adds authentication** (and TLS). An optional built-in authentication layer is planned before v1.0.

Backup encryption is always on (restic). You can export the repository key/password from the UI in case you ever need to access the restic repo directly.

<br>

## 5. Requirements

| Requirement | Notes |
|---|---|
| **Unraid 6.12+** | Earlier versions not tested |
| **KVM VMs** | Only for VM backup/restore — not required if you only use containers |
| **Restic repo location** | Local path (recommended: your array or cache), SMB, NFS, or any rclone backend |
| **Docker socket** | Mounted by the template automatically (`/var/run/docker.sock`) |
| **libvirt socket** | Mounted by the template automatically — only needed for VM backup |

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
| `APP_KEY` | **Yes** | 32-byte hex secret used to derive the restic repo password. Generate with `openssl rand -hex 32`. **Keep this safe** — losing it makes backups unrecoverable. |
| `REPO_PATH` | **Yes** | Path to the restic repository inside the container (map your local repo dir here). |
| `PORT` | No | HTTP port (default `3000`). The template exposes HTTPS on `3001` by default. |
| `TZ` | No | Timezone for the scheduler (e.g. `Europe/Berlin`). |

Mount the Docker socket and your appdata directory as shown in the CA template. The template pre-fills all recommended mount points.

> [!NOTE]
> **Host integration check:** open `/spike` in the web UI after the container starts. It probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces.

<br>

## 8. Development

```bash
cp .env.example .env          # set APP_KEY (openssl rand -hex 32)
npm install
npm test                       # unit + integration tests
npm run dev                    # https://localhost:3443 (self-signed cert)
```

Real Docker, libvirt and Unraid behavior cannot be tested in CI (no KVM, no Unraid on runners). Use the **host integration spike** (`/spike` or `npm run spike`) to validate mounts on your actual Unraid host before submitting a PR.

<br>

## 9. Support this project

<a href="https://buymeacoffee.com/junkerderprovinz">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" height="40">
</a>
