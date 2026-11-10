# Restore

Every restore starts on the **Restore** page: pick a section, search for what you want back and choose a backup. One window restores every kind of entry and asks in steps. By default nothing changes on the original, because the backup comes back next to it.

![The Restore page with the sections as tiles, the search, and the Stacks card.](assets/screenshots/restore/page-dark.png#only-dark)
![The Restore page with the sections as tiles, the search, and the Stacks card.](assets/screenshots/restore/page-light.png#only-light)

## The Restore page {#page}

- **Sections** lists the sections as tiles, each with how many entries have snapshots.
- **What do you want to bring back?** searches the entries of the picked section. Each row shows its newest snapshot, and **Open entry** leads to the entry's page.
- **Find a file** looks for a file or folder by its name in every backup, see [Finding a file](#find-a-file).
- **Stacks** lists the Compose projects with at least two backed-up containers, such as `immich` with its database and Redis. The search finds a stack by its name or by any of its containers.
- **Running now** shows a restore while it runs, also after you closed its window.
- **Emergency** starts the Emergency wizard for a server that is gone or was set up again, see [Emergency wizard](offsite-recovery.md#emergency).
- **Other sources** holds **From an old backup folder**, for the backups of another BombVault, **Import from the Appdata.Backup plugin** and **Jobs from Duplicati or Duplicacy**.
- **Other servers** lists the other servers of a shared repository this server is in, see [Other servers](#other-servers).

![The Restore page on a phone.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![The Restore page on a phone.](assets/screenshots/restore/page-phone-light.png#only-light)

## The restore window {#window}

A click on an entry opens **Restore** with its name. It asks in steps, and each step can be changed before you start:

1. **Backup**: the recommended snapshot is preselected. That is the newest one, or after an open anomaly the last one before it. The backup taken before an earlier restore holds the state that restore replaced, so it is passed over unless there is no other. Older ones are in the list, and **Compare** shows what differs between two of them.
2. **Source**: where the files come from, when a snapshot lies in several places. **Local** is fastest.
3. **What**: **All**, or **Individual files**, where you tick the files and folders you want.
4. **Where to**: the target. The choices depend on the kind of entry:

    | Target | What it does |
    |---|---|
    | **Next to the original** | The default. A container comes back as a second container named `<name>-restored` with ports of its own, a VM's disks and NVRAM land in a folder named `<folder>-restored`, and a folder set comes back as `<path>-restored`. Nothing changes on the original. |
    | **Replace original** | Overwrites what is there now. With **Back up the current state first**, which is on by default, BombVault first backs up today's state so you can go back. If that backup fails, the restore does not start. That also works for a container or VM that is gone from the server: the folders the restore writes over are backed up on their own. |
    | **To a folder** | Just the files, for browsing. |
    | **Download as ZIP** | The flash drive, for a new stick. BombVault never writes into the running flash drive. |
    | **Into a new dataset** and **Restore into the dataset** | ZFS datasets, see [ZFS datasets](zfs-datasets.md#restore). |

5. **What happens** says in words what the restore will do, and a scene shows the target you picked.

Before **Restore** unlocks, the window checks that the repository answers, that the key opens it, that the snapshot is there and that the target has room. While a check fails the button stays locked, and the (i) in it says which one.

The window also shows what the restore changes compared with what is there now: new, replaced and unchanged files, with the list on request, and the files at the target that are not in the backup and stay where they are. restic works this out as a dry run from size and modification time, without reading the files, and a very large tree stops after 30 seconds and says so. For containers and VMs it compares the settings the restore recreates with the running ones: image and tag, ports, variable names and volumes, or memory, vCPUs, disks and network.

Some things can stop a restore before it changes anything. A container's static IP and published ports have to be free, and its network has to exist. On Unraid every restore in place refuses a folder that is not on a mounted pool or share, because the files would land in memory and be gone after a reboot. A restore in place names every other container whose bind mount reaches into a folder it writes to, for example "this path is also used by nextcloud-db", but that only warns.

A few options appear where they apply: **Leave stopped afterwards**, to look at a container or VM before it runs again, and **Restore without GPU and runtime** on a host that lacks the GPU or runtime the backup was made with.

The restore runs in the background. You can close the window, and the progress also shows under **Running now**. A long restore shows a percentage and can be cancelled, and a cancelled restore is recorded as cancelled, not as failed. A restore cancelled while the container is stopping, or one that cannot remove the container, has restored nothing yet, so the container starts again. A container that takes its volumes from another with `--volumes-from` keeps them, and the restore refuses to begin while that other container does not exist.

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

To put the file straight back instead, choose **Replace original** in step 5. Keep **Back up the current state first** on, so BombVault backs up the broken state before it writes, type `nextcloud` to confirm and press **Restore**. Nextcloud is stopped while the picked file is written back to its place and started again afterwards, and only the files you ticked are written. A restore that fails partway leaves Nextcloud stopped, since the file it reads may be half written, and the run says so.

![The restore window at What happens, with Replace original picked.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![The restore window at What happens, with Replace original picked.](assets/screenshots/restore/window-replace-light.png#only-light)

## Finding a file {#find-a-file}

When you know the name of a file but not which entry or backup still has it, type the name into **Find a file** and press **Search**. Plain text matches anywhere in the name and ignores case, so `invoice` finds `Invoice-2024.pdf`. With `*` or `?` the pattern has to match the whole name, so `*.pdf` finds every PDF. Type just the name, without a folder.

The search reads every backup of the containers, folder sets, ZFS datasets and the flash drive. Each hit shows the entry, the path and every backup that holds the file, newest first, with the date, the size and the place it was read at. A backup in which the file has the same size and modification time as in the one before it says **unchanged**, so you can tell where the file last changed. A click on a backup opens the restore window on it, with **Individual files** picked and the file ticked. A file from the flash drive comes back as part of the flash drive ZIP, because the flash drive is only restored as a whole.

The search takes a while, because restic walks through every backup. **Stop** ends it, and so does leaving the page. Under the field it says which places it read:

- Places on this server and on your network are always searched.
- A backup that lies at several places is read only once, at the first one. A place whose backups were all read elsewhere says **already searched at another place**.
- Off-site places such as S3, B2 or SFTP are left out unless you switch on **Also search off-site places**. Then only the backups that no place here holds are read there, usually the older ones a longer retention keeps off-site. The first search at such a place can take long, and some providers charge for the data it downloads.
- A place that could not be opened says **could not be read**, and the (i) beside it tells why. The other places are still searched.

Virtual machines are saved as whole disks, so the files inside them are not searched. The self-backup, the backups of another BombVault under **Other sources** and those of the servers under **Other servers** are not searched either. A search lists the first 200 files, so a short name such as `log` may need more letters.

## What comes back, by kind {#kinds}

**Containers** are recreated from their saved definition through the Docker API, with their entrypoint, hostname, stop settings, volumes and mounts, resource limits, log driver, DNS settings and legacy links. A container comes back on the image it ran when the backup was taken, not on a newer one pulled later, and an image that is missing is pulled first, with the sign-in from **Container registries** under **Settings, Restore**. Its GPU or runtime (`--gpus`, `--runtime=nvidia`) comes back too. On a host without that driver or runtime the restore says so and offers **Restore without GPU and runtime**. A link to a container that is missing, or stopped while the restored one starts, is left out, and the run history says so. A restore that fails after the container was removed puts it back, stopped.

A backup can hold folders the container no longer has selected, for example an older one taken before a folder vanished. A folder that is gone or empty comes back. One that holds files now stays as it is, because you may have unticked it on purpose, and the check before the restore and the run history name it. Get it back with **Individual files** or **To a folder**.

**VMs** get their XML defined again over SSH and their disks and UEFI NVRAM reattached, even after the VM was deleted. See [VM backups](vm-backup-ssh-setup.md) for VMs with Unraid snapshots and disks on zvols.

**Folder sets** come back with their ownership, modes and timestamps as restic stores them. Where the target only refuses the ownership, as `/mnt/user` does, the run ends with a note that says so. BombVault then compares the files with the backup, and a file that did not arrive makes the restore fail.

**The flash drive** never touches the live `/boot`. The restore is a download called `flash-<id>.zip`, ready to drop into the Unraid USB Creator. The ZIP holds `APP_KEY`, so downloading it needs a login password and a sign-in at the instance. The window also lists the plugins in each flash backup with their version and size, and can put a single one back into the running flash: its `.plg` file, its folder under `config/plugins` and the package files the backup holds. Unraid installs it on the next boot, or right away under **Plugins, Install Plugin**.

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

BombVault checks the members and the project folder first, and a restore that could not work changes nothing. Otherwise it stops the running members, the apps before their databases, then writes the project folder, then restores the members. If the project folder does not come back, the members are still restored but stay stopped, because the folder may be half written and it is where a member's bind mounts usually lie. Each member's run says so.

A database that has only database dumps and no files backup, and whose container and named volume are gone, comes back without its data. The check says so before the restore. It stays stopped, and so do the containers that depend on it, so no app runs its setup on an empty database. Start it, import a dump and start the others. See [Database dumps](backups.md#database-dumps).

## Other sources {#other-sources}

- **From an old backup folder** opens the repository of another BombVault that no longer runs, on a disk or at an off-site storage location, with its recovery kit or its typed `APP_KEY`. The session is read-only and lives only in memory. See [From an old backup folder](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Import from the Appdata.Backup plugin** reads the plugin's backup folder. Each container archive becomes a restore point of its container, dated when the plugin made it. Archives imported before are skipped, and the archives themselves are only read. The container needs one backup in BombVault first, so the restore has its definition. Retention leaves imported restore points alone, so delete one yourself when you no longer need it.
- **Jobs from Duplicati or Duplicacy** turns the backup jobs of those tools into folder sets. It reads their settings only. The old backups stay with the old tool, and BombVault starts with a first backup of its own. Neither tool has to be installed here.
    - **From Duplicati**: export each job in Duplicati to a file without a password, and pick the `.json` files. An export that includes the passwords brings the destination's sign-in along.
    - **From Duplicacy**: pick the Duplicacy Web folder that holds `duplicacy.json`, or a folder you back up with the Duplicacy command line, which holds `.duplicacy`. Keys the command line saved in `.duplicacy/preferences` come along. Duplicacy Web keeps its sign-ins encrypted, so the window asks for them.

    Nothing is created before you have seen the preview. Each source folder becomes a folder set, with the folder on this server it was found at, the exclusions that could be translated and the schedule. When the old tool ran in a container of its own and a folder was not found, you pick it. The backups go to a new storage location in a folder `bombvault` inside the old destination, for a local folder, S3, SFTP, an rclone remote, or Backblaze B2 by its S3 address. A sign-in from the file is saved with your other sign-ins, and one the file lacks is asked for. Notes under each job say what did not come along, such as include rules, regular expressions, how long backups were kept, and the old encryption password, which BombVault does not need.

## Other servers {#other-servers}

In a [shared repository](offsite-recovery.md#shared-repository) every key opens the backups of all servers, so the Restore page lists the other members under **Other servers**, each with its name, its host, how many entries it has and when its newest backup ran. A host that belongs to no server this one knows shows as **A server this one does not know**. A click opens **Backups of** and the server's name, read-only and with nothing to enter. Folder sets and ZFS datasets go into a folder you choose. Containers and VMs come back as plain files in a folder, because the other server seals their definitions with its own key. This server's own backups in the shared repository are on the **Backups** tab of each entry.

## After a restore {#after}

A copy restored next to the original is listed under **Backups** as excluded, so it does not count as an open task. Add it to the schedule if you want to keep it. A container or VM that was running when it was backed up comes back running, one that was stopped stays stopped. On Unraid a restored container reappears in the Docker tab as a normal, editable app, and a VM in the VM Manager with its disks and NVRAM attached.
