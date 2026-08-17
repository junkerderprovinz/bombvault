# Getting started

This page walks you from a fresh Unraid box to your first backup.

## Requirements

| Requirement | Notes |
|---|---|
| **Unraid 6.12+** | Earlier versions are not tested. |
| **Restic repo location** | A local path (recommended: your array or cache), SMB, NFS, or any rclone backend. |
| **Docker socket** | Mounted by the template automatically (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Mounted whole by the template automatically (`/boot` to `/host/boot`). Powers flash backup and lets a restored container reappear as a normal, editable Unraid app. |
| **KVM VMs** (opt-in) | VM backup talks to libvirt over SSH, no libvirt mount. Set it up in Settings (see [Configuration](configuration.md)). |

## Install on Unraid

The easiest path is **Community Applications**.

1. Open the **Apps** tab in Unraid.
2. Search for **BombVault**.
3. Click **Install**, set the required variables (below), and apply.

!!! tip "Manual template install"
    If you prefer to add the template by hand:

    1. Go to **Docker, Add Container, Template repositories** and add:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Search for **BombVault** in Templates.
    3. Set the required variables and click **Apply**.

## Generic Docker host

Not on Unraid? BombVault also runs as a plain container on any Docker host (this is also what powers containers-only support on TrueNAS Scale, ahead of its own app-catalog entry).

1. Grab the ready-to-edit [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) from the repo.
2. Set `APP_KEY` (see below) and point the Host Data volume at your real data root — the file's comments walk through both.
3. `docker compose up -d`, then open `https://<host-ip>:3443/`.

What's different from Unraid:

- **No flash/USB domain.** There is no boot USB to capture or restore, so the Flash domain in Settings has nothing to do here. Instead, the Files domain offers a one-click **Add preset: Host system config** suggestion (a starting `/etc` file set you review and edit before saving) as a practical, generic equivalent.
- **No Unraid-native notifications.** BombVault's own in-app notification channels (webhook, off-site failure alerts, etc.) work as normal; only the Unraid-specific push to its native notification system is skipped, since there is no such system to push to.
- **VM backup is opt-in and needs a separate libvirtd host reachable over SSH** — see the commented-out block in the compose file. There is no VM manager built into a generic Docker host itself.

## The one required setting

The only variable you must set is `APP_KEY`, a 32-byte hex secret (64 hex characters) used to derive the restic repository password.

Generate one on any machine:

```bash
openssl rand -hex 32
```

Paste the result into the `APP_KEY` field of the template (Unraid), or the `APP_KEY` environment variable in `docker-compose.yml` (generic Docker host).

!!! danger "Do not lose your APP_KEY"
    Losing `APP_KEY` makes your encrypted backups unrecoverable. Store it somewhere safe and separate from the server. Once BombVault is running, use its one-click **encryption-key recovery kit** (see [Off-site & recovery](offsite-recovery.md)) to save the full recovery bundle.

The template also mounts the Docker socket, the flash (`/boot`) and the **Host Data** root (`/mnt`) for you. Backup *sources* and *destinations* both live under Host Data. For the full variable reference and the off-site setup, see [Configuration](configuration.md).

## First run

1. Open the web UI at `https://<your-unraid-ip>:3443` (self-signed certificate out of the box).
2. In **Settings**, enable the backup domains you want (Containers, VMs, Flash, Config, Files) and pick an accent colour.
3. On the **Containers** tab, pick a container and click **Back up** to make your first restore point. Repository paths default to `/mnt/user/bombvault/{container,vms,flash,config,files}` and are created on the first backup.
4. Set up scheduling from **Settings, Schedules**. There is a one-click *include all in schedule* for containers and VMs.

!!! tip "Optional: pick a backup order"
    If some containers should always be backed up before others (for example a database before the app that uses it), open the **backup-order** panel on the Containers page and drag them into the sequence you want. Scheduled and multi-select runs then follow it; anything you leave unordered is backed up most-overdue-first, as before.

!!! note "Host integration check"
    Open `/spike` in the web UI after the container starts. It probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces, so you can confirm the container is wired up correctly before you rely on it.

## Simple vs Advanced

By default the interface shows only the essentials (back up, restore, schedule). Use the **Simple / Advanced** switch in the sidebar to reveal the expert controls: retention, off-site copy, pre/post hooks, file-level restore, notifications, Prometheus metrics and the integrity/maintenance tools. It is a per-browser preference and off by default, so newcomers get a clean UI and power users get everything.

## Next steps

- Browse the full **[Features](features.md)**.
- Add one or more **[Off-site & recovery](offsite-recovery.md)** replicas (each domain can ship to several destinations at once) and save your recovery kit.
- Cloning a setup or moving to a new box? Carry your whole configuration over with the **Export and import settings** card. See [Configuration](configuration.md#portable-settings-export-and-import).
- Hit a snag? See **[Troubleshooting](troubleshooting.md)**.
