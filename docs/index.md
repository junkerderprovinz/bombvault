# BombVault

**Your Unraid data, sealed in a vault. Drop a backup. Detonate a restore.**

BombVault is a self-hosted, Unraid-native web app for **backup and full disaster recovery** of your Docker containers and KVM/libvirt VMs. It runs as a single multi-arch Docker container, gives you a web interface in a dark, light or system theme, and handles the whole lifecycle: back up, schedule, verify and restore.

Restores are automatic. Containers reappear in the Unraid Docker tab exactly as before, and VMs are re-defined in the VM Manager with their disks and UEFI NVRAM reattached. No manual reinstall, no reconfiguration, no drama.

Powered by [restic](https://restic.net), so every backup is deduplicated, incremental and always encrypted.

!!! note "Keep your APP_KEY safe"
    BombVault derives the restic repository password from a 32-byte secret named `APP_KEY`. Losing it, together with the recovery kit, makes encrypted backups unrecoverable. Generate one with `openssl rand -hex 32` and store it somewhere safe. See [Configuration](configuration.md).

## What BombVault protects

Every container, VM, folder set, the flash drive, every ZFS dataset and BombVault's own settings is an **entry** with a page of its own. Entries of one kind form a **section**, which you switch on under **Settings, General**.

| Section | What is saved |
|---|---|
| **Containers** | Appdata directory plus the container definition (image, env vars, ports, labels, volumes). |
| **VMs** | VM disk image(s), the XML definition and UEFI NVRAM, backed up over SSH (no libvirt mount). |
| **Flash** | The whole USB flash (`/boot`): OS, license, array config, shares, network and plugin config. |
| **Self-backup** | BombVault's own `/config`: its settings database, the sign-ins of its storage locations and the libvirt SSH keypair. |
| **Folders** | Any folder on the server as a folder set, each with optional exclude patterns. |
| **ZFS datasets** | A dataset and every dataset below it, read from one ZFS snapshot and stored like a folder. See [ZFS datasets](zfs-datasets.md). |

## Restore is the star

After copying data back from the restic snapshot, BombVault replays the saved container definition against the Docker API, so the container reappears in the Unraid Docker tab as if it had always been there (same image, same settings, same port mappings). VMs get their XML re-defined over SSH and their disks and UEFI NVRAM reattached, even after the VM was deleted.

One window restores every kind. By default it picks **Next to the original**, which brings the backup back as a copy and leaves the original as it is. **Replace original** backs up the current state first, so a restore can be undone. A folder or a new ZFS dataset are the other targets.

When a backup stops dependent containers, they come back in the right order: BombVault restarts them in their Compose `depends_on` order and waits for each to report healthy before starting the ones that depend on it, so nothing races ahead of a database or a gateway that is not up yet. See [Features](features.md).

## How it works

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault uses the Docker socket to stop containers before a backup and to recreate them after a restore. For VMs it runs `virsh` on the host over SSH (`qemu+ssh://`) to shut a domain down gracefully or take a live snapshot. It never bind-mounts a libvirt path, so it cannot get in the way of the VM Manager on the host.

BombVault is the orchestration and UI layer, not the storage engine. All actual data movement goes through restic.

## Quick start

New here? Head to **[Getting started](getting-started.md)** to install BombVault on Unraid via Community Applications and run your first backup. Then explore the full **[Features](features.md)**, tune your **[Configuration](configuration.md)**, and set up **[Off-site & recovery](offsite-recovery.md)**.

Each section can copy to several storage locations at once, a paired BombVault can keep your off-site copies in a server that refuses deletes, and the **Emergency wizard** brings a lost server back from its recovery kit, through your group or with its typed key. To carry your configuration to a new box, use the **Export / import settings** card. See [Off-site & recovery](offsite-recovery.md) and [Configuration](configuration.md#portable-settings-export-and-import).

The **[Android app](android.md)** puts every server of your group on your phone, with the activity log of all of them on one screen.

## Credits {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** by [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) gave BombVault its starting idea: one-click backup and automatic reinstall of Docker containers. BombVault is a separate implementation on Go and restic that carries the idea over to VMs, the flash and more.
- **[restic](https://restic.net/)** is the fast, secure, deduplicating backup engine BombVault drives.
- **[rclone](https://rclone.org/)** provides the cloud backends.
- Most glyphs on buttons come from the free Core Solid set by **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [source](https://github.com/webalys-hq/streamline-vectors)). The others come from Font Awesome Free, Material Design Icons, Simple Icons and Tabler Icons, or were drawn for the project.

## License {#license}

Copyright (C) 2026 Junker der Provinz. BombVault is free software under the **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). You may run, study, share and change it. If you distribute it, or run a changed version as a network service, you must publish your source under the same licence and keep the existing copyright and attribution notices.

The name and branding are not licensed. The AGPL covers the source code only: "BombVault", its logo and its branding stay reserved, so a fork has to use a name and branding of its own and may not present itself as BombVault.

## Links

- **Source code:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid support thread:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-equivalent control of the host"
    Through the Docker socket BombVault can stop, remove and recreate containers and read/write appdata, and for VM backup it logs in to the host over SSH to run `virsh`. Anyone who can reach its web UI effectively has root on the host. Run BombVault only on a trusted, non-exposed network, and set a login password under **Settings, Security**. The recovery kit, key deposits and deleting the backups at a storage location need one. See [Configuration](configuration.md#security-model) for the full security model.
