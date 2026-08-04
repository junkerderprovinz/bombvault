# BombVault

**Your Unraid data, sealed in a vault. Drop a backup. Detonate a restore.**

BombVault is a self-hosted, Unraid-native web app for **backup and full disaster recovery** of your Docker containers and KVM/libvirt VMs. It runs as a single multi-arch Docker container, gives you a modern dark web UI, and handles the whole lifecycle: back up, schedule, verify and restore.

Restores are automatic. Containers reappear in the Unraid Docker tab exactly as before, and VMs are re-defined in the VM Manager with their disks and UEFI NVRAM reattached. No manual reinstall, no reconfiguration, no drama.

Powered by [restic](https://restic.net), so every backup is deduplicated, incremental and always encrypted.

!!! note "Keep your APP_KEY safe"
    BombVault derives the restic repository password from a 32-byte secret named `APP_KEY`. Losing it makes encrypted backups unrecoverable. Generate one with `openssl rand -hex 32` and store it somewhere safe. See [Configuration](configuration.md).

## What BombVault protects

| Domain | What is saved |
|---|---|
| **Docker containers** | Appdata directory plus the container definition (image, env vars, ports, labels, volumes). |
| **KVM / libvirt VMs** | VM disk image(s), the XML definition and UEFI NVRAM, backed up over SSH (no libvirt mount). |
| **Unraid flash** | The whole USB flash (`/boot`): OS, license, array config, shares, network and plugin config. |
| **App configuration** | BombVault's own `/config`: its settings database, off-site credentials and the libvirt SSH keypair. |
| **Files & folders** | Named **file sets**, any folder on the server, each with optional per-set exclude patterns. |

## Restore is the star

After copying data back from the restic snapshot, BombVault replays the saved container definition against the Docker API, so the container reappears in the Unraid Docker tab as if it had always been there (same image, same settings, same port mappings). VMs get their XML re-defined over SSH and their disks and UEFI NVRAM reattached, even after the VM was deleted.

When a backup stops dependent containers, they come back in the right order: BombVault restarts them in their Compose `depends_on` order and waits for each to report healthy before starting the ones that depend on it, so nothing races ahead of a database or a gateway that is not up yet. See [Features](features.md).

## How it works

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault is the orchestration and UI layer, not the storage engine. All actual data movement goes through restic.

## Quick start

New here? Head to **[Getting started](getting-started.md)** to install BombVault on Unraid via Community Applications and run your first backup. Then explore the full **[Features](features.md)**, tune your **[Configuration](configuration.md)**, and set up **[Off-site & recovery](offsite-recovery.md)**.

Off-site can fan out to several targets per domain at once, a read-only **receiver dashboard** monitors those copies on the box that receives them, and you can carry your whole configuration to a new box with the **Export and import settings** card. See [Off-site & recovery](offsite-recovery.md) and [Configuration](configuration.md#portable-settings-export-and-import).

## Links

- **Source code:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid support thread:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-equivalent control of the host"
    Through the Docker socket BombVault can stop, remove and recreate containers and read/write appdata, and for VM backup it logs in to the host over SSH to run `virsh`. Anyone who can reach its web UI effectively has root on the host. Run BombVault only on a trusted, non-exposed network, and enable the optional password gate (Settings, Security) once off-site or immutable backups are in use. See [Configuration](configuration.md) for the full security model.
