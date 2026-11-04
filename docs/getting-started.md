# Getting started

This page walks you from a fresh Unraid box to your first backup.

## Requirements {#requirements}

| Requirement | Notes |
|---|---|
| **Unraid 6.12+** | Earlier versions are not tested. Unraid is the main target, but BombVault also runs on a plain Docker host and on TrueNAS Scale (see [Generic Docker host](#generic-docker-host) and [TrueNAS SCALE](#truenas)). |
| **Restic repo location** | A local path (recommended: your array or cache), SMB, NFS, or any rclone backend. |
| **Docker socket** | Mounted by the template automatically (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Mounted whole by the template automatically (`/boot` to `/host/boot`). Powers flash backup and lets a restored container reappear as a normal, editable Unraid app. |
| **KVM VMs** (opt-in) | VM backup talks to libvirt over SSH, no libvirt mount. Set it up under **Settings, Integrations, Host SSH** (see [Configuration](configuration.md)). |
| **ZFS datasets** (opt-in) | The same SSH link as VM backups, `zfs` on the host, and Host Data mapped as `/mnt` with access mode Read/Write - Slave, the template default. See [ZFS datasets](zfs-datasets.md). |
| **Android app** (optional) | Android 10 or later, paired with servers on version 9.7.0 or later. See [Android app](android.md). |

## Install on Unraid {#install-on-unraid}

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

Not on Unraid? BombVault also runs as a plain container on any Docker host. TrueNAS SCALE has its own steps under [TrueNAS SCALE](#truenas).

1. Grab the ready-to-edit [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) from the repo.
2. Set `APP_KEY` (see below) and point the Host Data volume at your real data root. The file's comments walk through both.
3. `docker compose up -d`, then open `https://<host-ip>:3443/`.

What's different from Unraid:

- **No Flash section.** There is no boot USB to capture or restore, so the Flash section has nothing to do here. Instead, when Host Data reaches the host's `/etc`, **Add** on the Backups page offers **System configuration** as a folder suggestion: a starting `/etc` folder set you review and edit before saving.
- **No Unraid-native notifications.** BombVault's own in-app notification channels (webhook, off-site failure alerts, etc.) work as normal. Only the Unraid-specific push to its native notification system is skipped, since there is no such system to push to.
- **VM backup is opt-in and needs a separate libvirtd host reachable over SSH.** See the commented-out block in the compose file. There is no VM manager built into a generic Docker host itself.
- **No dashboard widget.** The BombVault Widget is an Unraid plugin, so that step is skipped as well.
- **Finding a container's data.** Without Unraid's `appdata` convention, a container's data folder is found from the segments in `DATA_ROOT_SEGMENTS`, Docker named volumes, a Compose project's working directory and the `bombvault.data` label (see [Backup source detection](configuration.md#backup-source-detection)). Named volumes and the `/etc` preset only reach paths inside the Host Data mount, so point Host Data at a common ancestor that also covers Docker's data root.
- **`PLATFORM`.** Set it to `generic` or `truenas`. Left unset, BombVault detects Unraid by its own marker on the flash mount and treats anything else as generic, and the Unraid-only steps are skipped instead of tried and failing.

### TrueNAS SCALE {#truenas}

BombVault is not in the TrueNAS app catalog yet, so install it as a custom app: **Apps, Discover Apps**, then **Install via YAML** in the menu beside **Custom App**. The YAML below sets the same things as the catalog entry in the repository's `truenas-apps` folder. Replace `tank` with your pool and fill in `APP_KEY` as the next section describes.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Then open `https://<truenas-ip>:30528/`. What is listed under [What's different from Unraid](#generic-docker-host) applies here too, and on TrueNAS:

- **Capabilities.** BombVault needs the seven above. VM backups fail without `SETPCAP`, and without `SYS_CHROOT` every container is stopped for its whole backup instead of being read from a snapshot.
- **Host Data** is a host path on a dataset, here `/mnt/tank`, mounted at the same path inside the app. That is why `HOST_SOURCE_ROOT` and `HOST_MOUNT_ROOT` both name it. `rslave` passes on the mounts the host makes later, which ZFS dataset backups need.
- **Data in an ixVolume is not backed up.** An app whose storage is an ixVolume keeps its data under `/mnt/.ix-apps`, outside Host Data. BombVault saves only the app's definition, and the run names the folders it left out. For every app whose data you want backed up, choose **Host Path** storage on a dataset below Host Data.
- **VM and ZFS dataset backups** reach TrueNAS over SSH as root. Turn on the SSH service under **System, Services**, and add the public key from **Settings, Integrations, Host SSH** to the root user under **Credentials, Users**. VM backups also need `LIBVIRT_URI` as above, because TrueNAS's libvirtd listens on a socket of its own. With neither `LIBVIRT_URI` nor `LIBVIRT_HOST` set, BombVault uses the app's default gateway as the host, since `host.docker.internal` does not resolve in a TrueNAS app. The [VM backup guide](vm-backup-ssh-setup.md#truenas-scale) covers the details, VMs on zvols included.
- **A ZFS replica uses a different key.** A replica on a second ZFS server logs in with the key the replica dialog shows, not with the Host SSH key. Add that one on the receiving server.
- **A deleted app comes back in two steps.** Reinstall it in TrueNAS under the same name first, which creates its network again, then restore its data in BombVault. Until then the restore check reports the missing network, and the restore refuses before it changes anything.
- **Next to the original** creates a plain Docker container. It runs, but the TrueNAS Apps screen does not list it.
- **Database dumps** work on dataset mounts. An import into a data folder that is a dataset of its own moves the old contents into a folder next to it, so the dataset above needs room for them.

### ZimaOS, CasaOS and other hosts {#other-hosts}

- **ZimaOS and CasaOS:** add the [community store](https://github.com/junkerderprovinz/zimaos-apps#install) and install BombVault from it. It runs there as `PLATFORM=generic` with Host Data on `/DATA`. ZimaOS keeps app data in `/DATA/AppData/<app>`, which BombVault finds on its own, since `DATA_ROOT_SEGMENTS` matches `appdata` in any case.
- **MOS:** install BombVault from the [MOS Hub](https://mos-official.net/apps.html?search=BombVault). It runs there as `PLATFORM=generic` with Host Data on `/mnt/cache`.
- **Synology:** in Container Manager, create a project from the compose file (DSM 7.2 or newer).
- **Proxmox:** run it in a VM or an LXC container that has Docker.

Without comments, the generic compose file is short. Replace `./data` with the host folder that holds your containers' data, set `HOST_SOURCE_ROOT` to that same path, and fill in `APP_KEY`:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

To back up Docker named volumes as well, mount a folder that holds both your data and Docker's data root (`docker info -f '{{.DockerRootDir}}'`, usually `/var/lib/docker`), the way the Unraid template mounts all of `/mnt`.

## The one required setting {#the-one-required-setting}

The only variable you must set is `APP_KEY`, a 32-byte hex secret (64 hex characters) used to derive the restic repository password.

Generate one on any machine:

```bash
openssl rand -hex 32
```

Paste the result into the `APP_KEY` field of the template (Unraid), or the `APP_KEY` environment variable in `docker-compose.yml` (generic Docker host).

!!! danger "Do not lose your APP_KEY"
    Losing `APP_KEY` makes your encrypted backups unrecoverable. Store it somewhere safe and separate from the server. Once BombVault is running, set a login password and download the **recovery kit** under **Settings, Restore** (see [Off-site & recovery](offsite-recovery.md#recovery-kit)). It holds the key, every storage location and the restic commands.

The template also mounts the Docker socket, the flash (`/boot`) and the **Host Data** root (`/mnt`) for you. Backup *sources* and *destinations* both live under Host Data. For the full variable reference and the off-site setup, see [Configuration](configuration.md).

## First run {#first-run}

![The welcome page of a new install, with what BombVault found on the server.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![The welcome page of a new install, with what BombVault found on the server.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Open the web UI at `https://<your-unraid-ip>:3443` (self-signed certificate out of the box). A new install opens on **Welcome to BombVault**, which lists what BombVault found on the server.
2. Choose **Let's go**. The overview then shows the **First steps** card with three steps:
    1. **Choose sections**: switch on what BombVault backs up (Containers, VMs, Flash, Folders, ZFS datasets, Self-backup). **Choose** opens the list of a section's entries, where **Pause schedule** leaves one out. The sections you switch on here run every night, which you change under **Settings, Schedule**.
    2. **Add storage location**: a second place for the backups. Every section already has a folder on this server, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, created on the first backup. Safest is a second storage location off the premises, such as a Storage Box or Backblaze B2.
    3. **First backup**: after that it runs by itself every night. The card stays until the first backup has finished.
3. Open **Backups** to see every entry in one list. **Back up everything now** starts a run over all switched-on sections, and **Back up now** on an entry's page backs up just that one.

![The First steps card with its three steps, the second one open.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![The First steps card with its three steps, the second one open.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Used BombVault before? **Restore from a backup** on the welcome page opens the Restore page, where the **Emergency wizard** brings back the settings, containers, VMs and folders of a lost server. See [Off-site & recovery](offsite-recovery.md#emergency).

!!! tip "Optional: pick a backup order"
    If some containers should always be backed up before others (for example a database before the app that uses it), open the **Order** card under **Settings, Schedule** and drag them into the sequence you want. Scheduled runs and the full backup then follow it. Anything you leave unordered is backed up most overdue first.

## Example: your first container backup, end to end {#example-first-backup}

This walks through a fresh Unraid box with a few containers, among them `plex`, until the first backup is done and you know where to find it. It takes about ten minutes plus the time of the first backup.

1. **Install.** In the Unraid **Apps** tab, search for **BombVault** and press **Install**.
2. **Set the key.** On a computer, run `openssl rand -hex 32` and paste the 64 characters into the template's `APP_KEY` field. Keep a copy in your password manager now, before anything else. Press **Apply**.
3. **Open it.** Go to `https://<your-unraid-ip>:3443` and accept the self-signed certificate. **Welcome to BombVault** lists what it found, such as 12 containers, 2 VMs and the flash drive.
4. **Let's go.** The overview opens with the **First steps** card.
5. **Choose sections.** **Containers** is on from the start. Switch on what else you want, such as **Flash**. **Choose** opens the list of containers it found, where **Pause schedule** leaves one out, for example a test container you do not care about. Switched-on sections run every night in the full backup.
6. **Add storage location.** Every section already has its own folder on this server, below `/mnt/user/bombvault`, so the first backup has somewhere to go. This step adds a second place, and the safest one is off the premises. You can do it now with the [S3 example](offsite-recovery.md#example-s3), or come back to it once the first backup is done. The card stays until then.
7. **First backup.** Open **Backups** and press **Back up everything now**, or wait for the night. The ring in the status head turns into a progress ring and counts the entries, such as **Backing up now: 3 of 12**. Each container is stopped while its data is read and started again right after. A container whose data lies on ZFS is down only for the few seconds of a snapshot. The first backup reads everything, so it takes longest. Later ones store only what changed.
8. **Check it.** When the ring says **Everything backed up**, open **History** on the overview: every container has a green line in **Runs**. Open **Backups**, click `plex` and look at its **Backups** tab: the first snapshot is there, with **Local** as the place that holds it. A few minutes later its **Verify** tab shows the restore check, which restored a sample of the backup and read it back.
9. **Protect the key.** Open **Settings, Security** and set a login password. Then open **Settings, Restore** and press **Download recovery kit** in **For emergencies**. Store the file off the server, next to the `APP_KEY` copy from step 2. The **Save your recovery kit** reminder on the overview goes away once you press **I've stored it safely**.

From now on the containers are backed up every night. Good next steps, in this order: an [off-site storage location](offsite-recovery.md#example-s3), so a dead server does not take the backups with it, then [notifications](notifications.md), so a failure reaches you, then a look at [Schedules](schedules.md) if 03:00 does not suit you.

![The status head while the first backup runs, with the progress ring and its stations.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![The status head while the first backup runs, with the progress ring and its stations.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Host integration check"
    The **Host Integration** card on the overview probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces, so you can confirm the container is wired up correctly before you rely on it. It waits under **Customize** until you add it.

## Finding your way {#navigation}

![Settings as tiles, with the cards of the chosen tile beside them. Every change is written as you make it.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Settings as tiles, with the cards of the chosen tile beside them. Every change is written as you make it.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

The sidebar has five entries: **Overview**, **Anomalies** (while anomaly detection is on), **Backups**, **Restore** and, below **Sign out**, **Settings**. A page's main action floats at the bottom right, and **More** holds the rare ones. On a phone the bar holds Overview, Backups, Restore and Settings, and **More** has the rest.

Pages and settings use the whole width of the window. A window keeps its buttons in its foot, with the one that goes forward on the right. On a phone, a foot with more than three buttons keeps the way out and the main action and puts the others under **More**.

![On a phone, the bar at the bottom with More opened.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![On a phone, the bar at the bottom with More opened.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Overview** shows whether everything is backed up and what needs you. See [Overview](overview.md).
- **Backups** lists every container, VM, folder set, the flash drive, every ZFS dataset and the self-backup as entries. Tiles above the list pick a kind, and **Filters**, **Sort** and **Search** narrow it down. Every entry has a page of its own with three tabs: **Backups**, **Settings** and **Verify**. A back button left of the entry's icon leads to the list. See [Backups and entries](backups.md).
- **Restore** is one page for everything: pick a section, search for an entry and choose a backup. The Emergency wizard and the other sources start here too. See [Restore](restore.md).
- **Settings** are fourteen tiles: General, Look, Schedule, Storage locations, Restore, Notifications, Verify, Security, Instances, Pairing, Integrations, Apps, System and Info. See [Settings](settings.md). Press `/` or Ctrl+K to search them all, also by the names pages had in earlier versions. Drag the tiles into your own order under **Look**. Where a column of tiles beside the page would squeeze it, as in a window about 1200 pixels wide, the tiles become a strip above the page that scrolls sideways. A phone shows the same strip.
- **Storage locations** and **Instances** are settings tiles with pages of their own. **Also show in the sidebar** on their tile pins them to the sidebar.

Nothing has a Save button and nothing folds out: rare things have a card of their own or a window, and explanations sit in the (i) bubbles. After an update a window opens by itself with a card for every release since your last version, and green dots mark what changed. A click hides a dot. **New** in the Version card under **Settings, Info** counts what you have not seen yet and opens that window again, with the notes of each release and its changes with **Go there**.

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

## Next steps {#next-steps}

- Add one or more storage locations off the premises (each section can copy to several at once) and save your recovery kit, as described under **[Off-site & recovery](offsite-recovery.md)**. The [S3 example](offsite-recovery.md#example-s3) walks through one.
- Make sure a failure reaches you with **[Notifications](notifications.md)**.
- Check when things run under **[Schedules](schedules.md)**.
- Put every server of your group on your phone with the **[Android app](android.md)**.
- The **[worked examples](guides.md)** walk through the common jobs, and **[Features](features.md)** lists everything BombVault does.
- Cloning a setup or moving to a new box? Carry your whole configuration over with the **Export / import settings** card. See [Configuration](configuration.md#portable-settings-export-and-import).
- Hit a snag? See **[Troubleshooting](troubleshooting.md)**.
