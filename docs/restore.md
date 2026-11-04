# Restore

Every restore starts on the **Restore** page: pick a section, search for what you want back and choose a backup. One window restores every kind of entry and asks in steps. By default nothing changes on the original, because the backup comes back next to it.

![The Restore page with the sections as tiles, the search, and the Stacks card.](assets/screenshots/restore/page-dark.png#only-dark)
![The Restore page with the sections as tiles, the search, and the Stacks card.](assets/screenshots/restore/page-light.png#only-light)

## The Restore page {#page}

- **Sections** lists the sections as tiles, each with how many entries have snapshots.
- **What do you want to bring back?** searches the entries of the picked section. Each row shows its newest snapshot, and **Open entry** leads to the entry's page.
- **Stacks** lists the Compose projects with at least two backed-up containers, such as `immich` with its database and Redis. The search finds a stack by its name or by any of its containers.
- **Running now** shows a restore while it runs, also after you closed its window.
- **Emergency** starts the Emergency wizard for a server that is gone or was set up again, see [Emergency wizard](offsite-recovery.md#emergency).
- **Other sources** holds **From an old backup folder**, for the backups of another BombVault, and **Import from the Appdata.Backup plugin**.

![The Restore page on a phone.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![The Restore page on a phone.](assets/screenshots/restore/page-phone-light.png#only-light)

## The restore window {#window}

A click on an entry opens **Restore** with its name. It asks in steps, and each step can be changed before you start:

1. **Backup**: the recommended snapshot is preselected. That is the newest one, or after an open anomaly the last one before it. Older ones are in the list, and **Compare** shows what differs between two of them.
2. **Source**: where the files come from, when a snapshot lies in several places. **Local** is fastest.
3. **What**: **All**, or **Individual files**, where you tick the files and folders you want.
4. **Where to**: the target. The choices depend on the kind of entry:

    | Target | What it does |
    |---|---|
    | **Next to the original** | The default. A container comes back as a second container named `<name>-restored` with ports of its own, a VM's disks and NVRAM land in a folder named `<folder>-restored`, and a folder set comes back as `<path>-restored`. Nothing changes on the original. |
    | **Replace original** | Overwrites what is there now. With **Back up the current state first**, which is on by default, BombVault first backs up today's state so you can go back. If that backup fails, the restore does not start. |
    | **To a folder** | Just the files, for browsing. |
    | **Download as ZIP** | The flash drive, for a new stick. BombVault never writes into the running flash drive. |
    | **Into a new dataset** and **Restore into the dataset** | ZFS datasets, see [ZFS datasets](zfs-datasets.md#restore). |

5. **What happens** says in words what the restore will do, and a scene shows the target you picked.

Before **Restore** unlocks, the window checks that the repository answers, that the key opens it, that the snapshot is there and that the target has room. While a check fails the button stays locked, and the (i) in it says which one.

The window also shows what the restore changes compared with what is there now: new, replaced and unchanged files, with the list on request, and the files at the target that are not in the backup and stay where they are. restic works this out as a dry run from size and modification time, without reading the files, and a very large tree stops after 30 seconds and says so. For containers and VMs it compares the settings the restore recreates with the running ones: image and tag, ports, variable names and volumes, or memory, vCPUs, disks and network.

Some things can stop a restore before it changes anything. A container's static IP and published ports have to be free, and its network has to exist. A restore in place names every other container whose bind mount reaches into a folder it writes to, for example "this path is also used by nextcloud-db", but that only warns.

A few options appear where they apply: **Leave stopped afterwards**, to look at a container or VM before it runs again, and **Restore without GPU and runtime** on a host that lacks the GPU or runtime the backup was made with.

The restore runs in the background. You can close the window, and the progress also shows under **Running now**. A long restore shows a percentage and can be cancelled, and a cancelled restore is recorded as cancelled, not as failed.

![The restore window at the What step, with single files ticked.](assets/screenshots/restore/window-what-dark.png#only-dark)
![The restore window at the What step, with single files ticked.](assets/screenshots/restore/window-what-light.png#only-light)

### Where a restore into a folder goes {#folders}

**Restore folders** under **Settings, Restore** sets the **Default restore folder**, with a **Subfolder per section** if you like, and an **Exception** for one section. Picked files land straight in the target folder: one file comes back as just that file, a folder as the folder with its contents, and picks from different folders keep only their path below the folder they share. BombVault refuses a target that is not on mounted storage, because on Unraid such a folder lives in memory and is gone after a reboot, and it never restores into its own data folder.

## Example: get one file back {#example-single-file}

Someone edited Nextcloud's `config.php` last night and Nextcloud no longer starts. The nightly backup from 03:00 still has the good file. You want that one file, not the whole container.

1. Open **Restore**, pick the **Containers** tile and type `nextcloud` into **What do you want to bring back?**.
2. Click `nextcloud`. The window opens with the 03:00 backup preselected as the recommended one.
3. Leave **Source** at **Local**.
4. Under **What**, choose **Individual files**. Open the folders down to `config` and tick `config.php`.
5. Under **Where to**, choose **To a folder** and pick a folder, for example `user/restore`, which is `/mnt/user/restore` on the host.
6. Read **What happens**: the file lands in the target folder, and nothing changes on the container. Press **Restore**.

The window ends with **Done: the files are in /mnt/user/restore**. You find the file as `/mnt/user/restore/config.php`, without the folders above it. Compare it with the broken one, copy it back into the appdata folder and restart Nextcloud.

To put the file straight back instead, choose **Replace original** in step 5. Keep **Back up the current state first** on, so BombVault backs up the broken state before it writes, type `nextcloud` to confirm and press **Restore**. Nextcloud is stopped while the picked file is written back to its place, and only the files you ticked are written.

![The restore window at What happens, with Replace original picked.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![The restore window at What happens, with Replace original picked.](assets/screenshots/restore/window-replace-light.png#only-light)

## What comes back, by kind {#kinds}

**Containers** are recreated from their saved definition through the Docker API, with their entrypoint, hostname, stop settings, volumes and mounts, resource limits, log driver, DNS settings and legacy links. A container comes back on the image it ran when the backup was taken, not on a newer one pulled later, and an image that is missing is pulled first, with the sign-in from **Container registries** under **Settings, Restore**. Its GPU or runtime (`--gpus`, `--runtime=nvidia`) comes back too. On a host without that driver or runtime the restore says so and offers **Restore without GPU and runtime**. A link to a container that is missing, or stopped while the restored one starts, is left out, and the run history says so. A restore that fails after the container was removed puts it back, stopped.

**VMs** get their XML defined again over SSH and their disks and UEFI NVRAM reattached, even after the VM was deleted. See [VM backups](vm-backup-ssh-setup.md) for VMs with Unraid snapshots and disks on zvols.

**Folder sets** come back with their ownership, modes and timestamps as restic stores them.

**The flash drive** never touches the live `/boot`. The restore is a download called `flash-<id>.zip`, ready to drop into the Unraid USB Creator. The window also lists the plugins in each flash backup with their version and size, and can put a single one back into the running flash: its `.plg` file, its folder under `config/plugins` and the package files the backup holds. Unraid installs it on the next boot, or right away under **Plugins, Install Plugin**.

**ZFS datasets** have targets of their own and keep their ZFS properties, see [Restoring](zfs-datasets.md#restore).

**The self-backup** comes back only through the Emergency wizard, which restarts BombVault, so the live database is never overwritten under an open handle.

**Database dumps** are never loaded back on their own. A row under **Database dumps** can be downloaded, saved to a folder, or imported into a freshly started database. See [Database dumps](backups.md#database-dumps).

## Restoring several entries at once {#several}

On the Restore page, pick entries the way you pick rows in the Backups list: Ctrl+click, Shift+click, Ctrl+A, a long press, or **Select**. **Restore** then opens one window for all of them.

- **Which backup**: **Recommended** takes for each entry the backup its own window would recommend. **One moment for all** takes for each entry its newest backup from a **Moment** you pick or earlier, for example the evening before something went wrong.
- **Where to**: **Next to the original** or **Replace original**, with **Back up the current state first**.

Each entry is checked the way a single restore is, and then they run one after the other. Every row shows how its own went: **Waiting**, **Running**, **Restored**, **Failed** or **Left out**. **Stop after this one** lets the running restore finish and starts no more. The flash drive comes as a ZIP to download, and ZFS datasets and the self-backup are restored one at a time.

![Restoring several entries: one row per entry with its progress.](assets/screenshots/restore/several-dark.png#only-dark)
![Restoring several entries: one row per entry with its progress.](assets/screenshots/restore/several-light.png#only-light)

## Restoring a stack {#stacks}

A click on a stack in the **Stacks** card opens **Restore stack**. It restores every member from its latest backup at the same time, and with **Start in dependency order after restore** it starts them in their `depends_on` order: databases and caches first, then the app. When a member's project folder is missing at the place you picked, the window offers the state from Local instead.

## Other sources {#other-sources}

- **From an old backup folder** opens the repository of another BombVault that no longer runs, on a disk or at an off-site storage location, with its recovery kit or its typed `APP_KEY`. The session is read-only and lives only in memory. See [From an old backup folder](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Import from the Appdata.Backup plugin** reads the plugin's backup folder. Each container archive becomes a restore point of its container, dated when the plugin made it. Archives imported before are skipped, and the archives themselves are only read. The container needs one backup in BombVault first, so the restore has its definition. Retention leaves imported restore points alone, so delete one yourself when you no longer need it.

## After a restore {#after}

A copy restored next to the original is listed under **Backups** as excluded, so it does not count as an open task. Add it to the schedule if you want to keep it. A container or VM that was running when it was backed up comes back running, one that was stopped stays stopped. On Unraid a restored container reappears in the Docker tab as a normal, editable app, and a VM in the VM Manager with its disks and NVRAM attached.
