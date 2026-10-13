# Getting started

This page walks you from a fresh Unraid box to your first backup.

## Requirements

| Requirement | Notes |
|---|---|
| **Unraid 6.12+** | Earlier versions are not tested. Unraid is the main target, but BombVault also runs on a plain Docker host and on TrueNAS Scale (see [Generic Docker host](#generic-docker-host)). |
| **Restic repo location** | A local path (recommended: your array or cache), SMB, NFS, or any rclone backend. |
| **Docker socket** | Mounted by the template automatically (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Mounted whole by the template automatically (`/boot` to `/host/boot`). Powers flash backup and lets a restored container reappear as a normal, editable Unraid app. |
| **KVM VMs** (opt-in) | VM backup talks to libvirt over SSH, no libvirt mount. Set it up under **Settings, Integrations, Host SSH** (see [Configuration](configuration.md)). |
| **ZFS datasets** (opt-in) | The same SSH link as VM backups, `zfs` on the host, and Host Data mapped as `/mnt` with access mode Read/Write - Slave, the template default. See [ZFS datasets](zfs-datasets.md). |
| **Android app** (optional) | Android 10 or later, paired with servers on version 9.7.0 or later. See [Android app](android.md). |

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

## Generic Docker host {#generic-docker-host}

Not on Unraid? BombVault also runs as a plain container on any Docker host (this is also what powers containers-only support on TrueNAS Scale, ahead of its own app-catalog entry).

1. Grab the ready-to-edit [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) from the repo.
2. Set `APP_KEY` (see below) and point the Host Data volume at your real data root. The file's comments walk through both.
3. `docker compose up -d`, then open `https://<host-ip>:3443/`.

What's different from Unraid:

- **No Flash section.** There is no boot USB to capture or restore, so the Flash section has nothing to do here. Instead, **Add** on the Backups page offers **System configuration** as a folder suggestion: a starting `/etc` folder set you review and edit before saving.
- **No Unraid-native notifications.** BombVault's own in-app notification channels (webhook, off-site failure alerts, etc.) work as normal; only the Unraid-specific push to its native notification system is skipped, since there is no such system to push to.
- **VM backup is opt-in and needs a separate libvirtd host reachable over SSH.** See the commented-out block in the compose file. There is no VM manager built into a generic Docker host itself.
- **No dashboard widget.** The BombVault Widget is an Unraid plugin, so that step is skipped as well.
- **Finding a container's data.** Without Unraid's `appdata` convention, a container's data folder is found from the segments in `DATA_ROOT_SEGMENTS`, Docker named volumes, a Compose project's working directory and the `bombvault.data` label (see [Backup source detection](configuration.md#backup-source-detection)). Named volumes and the `/etc` preset only reach paths inside the Host Data mount, so point Host Data at a common ancestor that also covers Docker's data root.
- **`PLATFORM`.** Set it to `generic` or `truenas`. Left unset, BombVault detects Unraid by its own marker on the flash mount and treats anything else as generic, and the Unraid-only steps are skipped instead of tried and failing.

**TrueNAS Scale** takes the same compose route; a catalog entry is prepared in the repository but not submitted yet. VM backup there needs `LIBVIRT_URI`, because TrueNAS's libvirtd listens on a socket of its own (`/run/truenas_libvirt/libvirt-sock`) that the three `LIBVIRT_*` variables cannot express (see [Configuration](configuration.md)). How far this is proven: the zvol backup was run against a real TrueNAS Scale box, on a zvol attached to a running VM, and `zfs snapshot`, `zfs send`, restic and `zfs receive` round-tripped it byte for byte. A complete restore driven by BombVault itself has not been run on TrueNAS hardware yet, and that zvol was sparse, so throughput at many gigabytes is untested. Test a restore there before you rely on it.

## The one required setting

The only variable you must set is `APP_KEY`, a 32-byte hex secret (64 hex characters) used to derive the restic repository password.

Generate one on any machine:

```bash
openssl rand -hex 32
```

Paste the result into the `APP_KEY` field of the template (Unraid), or the `APP_KEY` environment variable in `docker-compose.yml` (generic Docker host).

!!! danger "Do not lose your APP_KEY"
    Losing `APP_KEY` makes your encrypted backups unrecoverable. Store it somewhere safe and separate from the server. Once BombVault is running, set a login password and download the **recovery kit** under **Settings, Restore** (see [Off-site & recovery](offsite-recovery.md#recovery-kit)). It holds the key, every storage location and the restic commands.

The template also mounts the Docker socket, the flash (`/boot`) and the **Host Data** root (`/mnt`) for you. Backup *sources* and *destinations* both live under Host Data. For the full variable reference and the off-site setup, see [Configuration](configuration.md).

## First run

![The overview after a first backup: what needs you, the protection of each section, the storage locations and the history.](assets/screenshots/dashboard.png)

*The overview after a first backup: what needs you, the protection of each section, the storage locations and the history.*

1. Open the web UI at `https://<your-unraid-ip>:3443` (self-signed certificate out of the box). A new install opens on **Welcome to BombVault**, which lists what BombVault found on the server.
2. Choose **Let's go**. The overview then shows the **First steps** card with three steps:
    1. **Choose sections**: switch on what BombVault backs up (Containers, VMs, Flash, Folders, ZFS datasets, Self-backup). **Choose** opens the list of a section's entries, where **Pause schedule** leaves one out. The sections you switch on here run every night, which you change under **Settings, Schedule**.
    2. **Add storage location**: where the backups go. A local folder is enough to start. Safest is a second storage location off the premises, such as a Storage Box or Backblaze B2. Local repositories default to `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` and are created on the first backup.
    3. **First backup**: after that it runs by itself every night.
3. Open **Backups** to see every entry in one list. **Back up everything now** starts a run over all switched-on sections, and **Back up now** on an entry's page backs up just that one.

Used BombVault before? **Restore from a backup** on the welcome page opens the Restore page, where the **Emergency wizard** brings back the settings, containers, VMs and folders of a lost server. See [Off-site & recovery](offsite-recovery.md#emergency).

!!! tip "Optional: pick a backup order"
    If some containers should always be backed up before others (for example a database before the app that uses it), open the **Order** card under **Settings, Schedule** and drag them into the sequence you want. Scheduled runs and the full backup then follow it; anything you leave unordered is backed up most-overdue-first.

!!! note "Host integration check"
    The **Host Integration** card on the overview probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces, so you can confirm the container is wired up correctly before you rely on it. It waits under **Customize** until you add it.

## Finding your way {#navigation}

![Settings as tiles, with the cards of the chosen tile beside them. Every change is written as you make it.](assets/screenshots/settings.png)

*Settings as tiles, with the cards of the chosen tile beside them. Every change is written as you make it.*

The sidebar has five entries: **Overview**, **Anomalies** (while anomaly detection is on), **Backups**, **Restore** and, below **Sign out**, **Settings**. A page's main action floats at the bottom right, and **More** holds the rare ones. On a phone the bar holds Overview, Backups, Restore and Settings, and **More** has the rest.

- **Backups** lists every container, VM, folder set, the flash drive, every ZFS dataset and the self-backup as entries. Tiles above the list pick a kind, and **Filters**, **Sort** and **Search** narrow it down. Every entry has a page of its own with three tabs: **Backups**, **Settings** and **Verify**.
- **Restore** is one page for everything: pick a section, search for an entry and choose a backup. The Emergency wizard and the other sources start here too.
- **Settings** are fourteen tiles: General, Look, Schedule, Storage locations, Restore, Notifications, Verify, Security, Instances, Pairing, Integrations, Apps, System and Info. Press `/` or Ctrl+K to search them all, also by the names pages had in earlier versions. Drag the tiles into your own order under **Look**. Where a column of tiles beside the page would squeeze it, as in a window about 1200 pixels wide, the tiles become a strip above the page that scrolls sideways. A phone shows the same strip.
- **Storage locations** and **Instances** are settings tiles with pages of their own. **Also show in the sidebar** on their tile pins them to the sidebar.

Nothing has a Save button and nothing folds out: rare things have a card of their own or a window, and explanations sit in the (i) bubbles. After an update the release notes open by themselves, and green dots mark what changed. A click hides a dot. **New** in the Version card under **Settings, Info** counts what you have not seen yet and opens the release notes and the list of every change with **Go there**, as two tabs of one window.

## Building from source {#build-from-source}

BombVault is a single static Go binary that serves a JSON API and an embedded React interface. Build the interface first, then run the binary:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

The interface build is needed for `go run` too. The repository tracks only an empty marker under `web/dist`, so without `npm --prefix web run build` the binary embeds nothing and answers `500 SPA index not found`, which is expected. Docker, libvirt and Unraid cannot be tested in CI, so check mounts, restic and the VM SSH link on a real host with the overview's **Host Integration** card (or `GET /api/spike`) before you open a pull request.

## Next steps

- Browse the full **[Features](features.md)**.
- Put every server of your group on your phone with the **[Android app](android.md)**.
- Add one or more storage locations off the premises (each section can copy to several at once) and save your recovery kit, as described under **[Off-site & recovery](offsite-recovery.md)**.
- Cloning a setup or moving to a new box? Carry your whole configuration over with the **Export / import settings** card. See [Configuration](configuration.md#portable-settings-export-and-import).
- Hit a snag? See **[Troubleshooting](troubleshooting.md)**.
