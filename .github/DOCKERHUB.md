<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner.png" alt="BombVault" width="100%">
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/pulls/junkerderprovinz/bombvault?style=for-the-badge&logo=docker&logoColor=white&label=Pulls&color=1d99f3" alt="Docker Pulls" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/image-size/junkerderprovinz/bombvault/latest?style=for-the-badge&logo=docker&logoColor=white&label=Size&color=1d99f3" alt="Image Size" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://restic.net"><img src="https://img.shields.io/badge/Engine-restic-CE4844?style=for-the-badge&logoColor=white" alt="restic" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<p align="center">
Your Unraid data, <b>sealed in a vault</b>. Armed with a fuse.<br>
BombVault backs up Docker containers, KVM VMs, appdata, the Unraid flash config — and even itself —
and restores everything with a single click. Containers <b>automatically reappear in the
Docker tab</b>, VMs <b>automatically in the VM tab</b> — no manual reinstall, no
reconfiguration, no drama.<br>
<br>
<b>Your data, locked in. Loss, locked out.</b><br>
Powered by <a href="https://restic.net">restic</a> — deduplicated, incremental, always encrypted.
</p>

## What is this?

BombVault is a self-hosted, **Unraid-native** web app for **backup and full disaster recovery**. One container, a modern dark web UI, and the whole lifecycle:

- **Backs up** Docker appdata + container definitions, KVM/libvirt VM disks + XML (incl. UEFI NVRAM), the whole Unraid flash (`/boot`), any folders you point it at (named **file sets** with per-set excludes), and its own `/config`.
- **Restores automatically** — containers are reinstalled and restarted so they reappear in the Docker tab exactly as before; VMs are re-defined in the VM Manager with their disks + NVRAM reattached.
- **Schedules** incremental backups per domain from one place, with one-click *"include all in schedule"*.
- **Optionally updates a container right after its backup** (advanced, off by default) — a fresh restore point always exists first, so a bad update is one restore away.

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/dashboard.png" alt="BombVault Dashboard — health summary, protection status per domain, run history and backup-health heatmap" width="90%">
  <br><em>Dashboard — health summary, protection status per domain, run history and a backup-health heatmap.</em>
</p>

## Feature highlights

**Simple by default** — the UI shows only the essentials; a Simple/Advanced switch reveals the expert controls.

- **One-click full restore** of containers, VMs, flash, config and file sets — individually (keeping each item's run-state) or all at once via guided recovery (left stopped, so you start things deliberately), from **local or off-site**.
- **Guided disaster recovery** — a dedicated Recovery tab restores BombVault's own settings first, discovers everything stored in your repos and restores it onto a fresh install; plus a one-time, read-only **restore from another BombVault instance's repo**.
- **Flash restore as a `.zip` download** (the live `/boot` is never touched) and a **scheduled flash zip export** so a bootable-USB copy leaves the server automatically.
- **File-level restore** — tick any files/folders inside a snapshot and restore them in place or into a folder; **stack restore** rebuilds a Docker Compose project, then starts its members in `depends_on` order.
- **Storage anywhere** — local path, SMB/NFS, native restic backends (`s3:` / `rest:` / `b2:` / `sftp:`) or any **rclone** remote; **off-site replication** (`restic copy`) with its own schedule and bandwidth caps; **per-source retention** pruned automatically.
- **Proof, not hope** — protection-status (RPO) dashboard, backup-health heatmap, **restore-verification drills** with a "last verified restorable" badge, repository integrity checks, and an **encryption-key recovery kit** for restoring without a running BombVault.
- **Ransomware protection** — append-only (immutable) off-site repos with a periodic **tamper test** that *proves* deletes are refused, off-site DR drills into a throwaway sandbox, a posture scorecard and a growth-budget alarm.
- **Notifications** — webhook (Discord/Slack/Gotify/ntfy), Matrix, Healthchecks.io (full start/success/fail lifecycle), email (SMTP) and Unraid-native alerts; opt-in **Prometheus `/metrics`**.
- **Ops niceties** — pre/post-backup hooks, stop-dependent-containers during backup, per-container exclude patterns with live preview, plain `tar.gz` exports (containers *and* VMs), snapshot diff & tags, server-side batch backups, Docker healthcheck, HTTPS out of the box, dark/light UI in **26 languages**.

## Install on Unraid

Requires **Unraid 6.12+** (earlier versions untested). Install via **Community Applications** — search for **BombVault**. Or add the template repo manually under **Docker → Template repositories**:

```
https://github.com/junkerderprovinz/unraid-apps
```

### Configuration

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

Mount the Docker socket, your appdata and a backup directory as shown in the CA template. **Backup repository paths are configured in the app** (Settings → Backup paths), not via env. **VM backup needs no libvirt mount** — it runs `virsh` on the host over SSH (`qemu+ssh://`): copy the key shown under *Settings → VM Backup over SSH* into the host's `authorized_keys` and click *Test connection*. After the first start, open `/spike` in the web UI — it probes every mount and CLI and reports any missing pieces.

## Security

**⚠️ BombVault holds root-equivalent control of the host** (Docker socket + SSH for VMs). Run it **only on a trusted, non-exposed network** — never publish it directly to the internet; for remote access use a VPN or a reverse proxy that adds authentication and TLS. Optional built-in password protection is available under Settings → Security (off by default for trusted-LAN use); backups are encrypted by restic (on by default), with the key derived from `APP_KEY`.

## Full documentation & support

The complete README — features in depth, screenshots, the security/trust model, the **VM-backup-over-SSH setup + networking guide**, and development docs — lives on GitHub:

**[github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)**

Questions, bugs, ideas? **[Unraid support thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)** · [GitHub issues](https://github.com/junkerderprovinz/bombvault/issues)

<a href="https://buymeacoffee.com/junkerderprovinz">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" height="40">
</a>

## Credits

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** by [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) — the original idea. BombVault is an independent rewrite (Go + restic) that extends the concept to VMs and the Unraid flash.
- **[restic](https://restic.net/)** — the backup engine. **[rclone](https://rclone.org/)** — off-site cloud backends.

---

<sub>Part of a family of self-hosted Unraid apps + plugins by <b>junkerderprovinz</b> — see them all at <a href="https://github.com/junkerderprovinz">github.com/junkerderprovinz</a>, or install from <a href="https://unraid.net/community/apps">Community Applications</a>.</sub>
