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

## The one required setting

The only variable you must set is `APP_KEY`, a 32-byte hex secret (64 hex characters) used to derive the restic repository password.

Generate one on any machine:

```bash
openssl rand -hex 32
```

Paste the result into the `APP_KEY` field of the template.

!!! danger "Do not lose your APP_KEY"
    Losing `APP_KEY` makes your encrypted backups unrecoverable. Store it somewhere safe and separate from the server. Once BombVault is running, use its one-click **encryption-key recovery kit** (see [Off-site & recovery](offsite-recovery.md)) to save the full recovery bundle.

The template also mounts the Docker socket, the flash (`/boot`) and the **Host Data** root (`/mnt`) for you. Backup *sources* and *destinations* both live under Host Data. For the full variable reference and the off-site setup, see [Configuration](configuration.md).

## First run

1. Open the web UI at `https://<your-unraid-ip>:3443` (self-signed certificate out of the box).
2. In **Settings**, enable the backup domains you want (Containers, VMs, Flash, Config, Files) and pick an accent colour.
3. On the **Containers** tab, pick a container and click **Back up** to make your first restore point. Repository paths default to `/mnt/user/bombvault/{container,vms,flash,config,files}` and are created on the first backup.
4. Set up scheduling from **Settings, Schedules**. There is a one-click *include all in schedule* for containers and VMs.

!!! note "Host integration check"
    Open `/spike` in the web UI after the container starts. It probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces, so you can confirm the container is wired up correctly before you rely on it.

## Simple vs Advanced

By default the interface shows only the essentials (back up, restore, schedule). Use the **Simple / Advanced** switch in the sidebar to reveal the expert controls: retention, off-site copy, pre/post hooks, file-level restore, notifications, Prometheus metrics and the integrity/maintenance tools. It is a per-browser preference and off by default, so newcomers get a clean UI and power users get everything.

## Next steps

- Browse the full **[Features](features.md)**.
- Add an **[Off-site & recovery](offsite-recovery.md)** replica and save your recovery kit.
- Hit a snag? See **[Troubleshooting](troubleshooting.md)**.
