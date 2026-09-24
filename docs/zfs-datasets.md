# ZFS datasets

The **ZFS** page backs up ZFS datasets. An item is one dataset together with every dataset below it. For each backup BombVault takes one ZFS snapshot of the whole tree, so every dataset in it is captured at the same instant. It then reads each dataset's files from that snapshot and stores them with restic, the same way it stores a folder, and removes the snapshot right after. The backups are deduplicated, you can browse every one of them, and single files can be restored.

BombVault never uses `zfs send` for datasets, never rolls a dataset back and never destroys one.

## Requirements {#requirements}

- **The SSH link to this server.** ZFS datasets use the same key, host and user as VM backups. If VM backups already work, so does this. Otherwise follow the [VM backup over SSH guide](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) on GitHub. The template fields are called **Host SSH: Address**, **Host SSH: Port** and **Host SSH: User**.
- **The `zfs` command on that host.** Unraid 6.12 and newer and TrueNAS SCALE have it.
- **Host Data mapped as `/mnt` with Access Mode Read/Write - Slave.** That is the template default. The snapshot of a dataset shows up inside the dataset's `.zfs/snapshot` folder only after BombVault has started, so the container has to receive mounts the host makes later.
- **The datasets mounted below `/mnt`.** On Unraid pools live at `/mnt/<pool>`, so that is already the case.

Switch the domain on under **Settings, General** (ZFS datasets). The ZFS page then shows a **Connection to this server** card. It tests the SSH link, names the user and host it connects to, and says what is missing when something is. The host-integration check (`/spike`) shows the same result.

## Items and child datasets {#items-and-children}

Open **Add datasets** on the ZFS page. The list comes from the server. Pick the dataset at the top of what you want backed up, for example `cache/appdata`, and the item covers it and every dataset below it.

- **New child datasets join on their own.** A dataset created below the item later is backed up with the next run, and that run names it as new. Its first backup reads it in full once; after that only changes are read.
- **You can leave single children out.** Switch a child off in the item's settings and it is left out together with everything below it. A left-out child that no longer exists on the server is marked as such and can be removed from the list.
- **Children that cannot be read are skipped, never silently.** The run lists them, the item shows how many were skipped, and the dashboard's coverage card counts each one as not protected. The run still backs up everything else and does not fail because of a skipped child. The reasons are in the [table of reason codes](#reason-codes): a dataset that is not mounted, has `canmount=off`, a `legacy` or no mountpoint, an encryption key that is not loaded, snapshot access switched off, or a mountpoint BombVault cannot see.
- **A skipped dataset does not take its children with it.** A `canmount=off` dataset that only holds other datasets is skipped (shown as "structure only") and its mounted children are backed up. An encrypted dataset whose key is not loaded is skipped together with the children that share its key.
- **Children that are VM disks or system data start switched off** in the add dialog, with the reason next to the switch. Adding a whole pool asks for a confirmation that lists what it contains.

### Volumes {#volumes}

A volume (zvol) holds a virtual disk instead of files, and the ZFS page never backs one up.

- A volume that a VM uses is backed up with that VM on the **VMs** page.
- A volume that no VM uses (an iSCSI extent, a disk you detached) is **not backed up by BombVault**. The add dialog and the ZFS page count these volumes and say so. A later version will back them up.

Volumes inside an item's tree are skipped and named on every run.

### Docker's storage {#docker-storage}

With Docker's ZFS storage driver, every image layer is a dataset with a `legacy` mountpoint. The add dialog folds these into one line per parent. A tree that holds more than 20 such datasets cannot become an item: while a snapshot of it exists, Docker cannot remove image layers. Add the datasets below it instead, for example `appdata`.

### Items never overlap {#overlap}

A dataset can belong to one item only. BombVault refuses a new item that lies inside an existing one, or that would contain one. To merge several child items into one parent item, delete the child items first and choose to keep their backups, then add the parent. Every dataset keeps its history under its own name, so the next backup continues where the old items left off and does not read everything again.

## Stopping containers and commands around the snapshot {#consistency}

A snapshot of a running database is like a sudden power cut: the database will usually recover, but it has to. Each item can do two things about that, and both only cover the instant of the snapshot, not the whole backup.

- **Stop these containers for the snapshot.** BombVault stops the listed containers, takes the snapshot and starts them again right away. Containers that write into the item's datasets are suggested first. Containers of one level of dependencies stop in parallel, dependents first, so the whole window is usually a few seconds; the run shows how long it was. The backup then reads the frozen snapshot while the apps are already running again. Only containers that were running are stopped.
- **A command before and after the snapshot.** It runs inside a container you choose, for example to dump a database into the dataset right before the snapshot, without stopping anything. If the command before the snapshot fails, the backup fails and no snapshot is taken. A failing command after the snapshot is shown on the run but does not fail the backup.

What happens when something goes wrong:

- If a container cannot be stopped, BombVault starts the ones it already stopped and the backup fails, naming the container. It never falls back to a snapshot of running apps.
- The stop waits for a running container backup to finish (up to 30 minutes for a manual run, up to the backup time limit for a scheduled one), so the two never stop and start the same container at once.
- Before the first container stops, BombVault writes down which ones it stops. If BombVault is killed inside the window, it starts those containers again the next time it starts, sends a notification, and the item shows a red note for any container it could not start.

A container can be on this list and on the **Containers** page at the same time. Its data is then stored twice, in two repositories, and **Backup Everything** stops it twice. The item says so.

## Restoring {#restore}

Open **Backups** on the item, pick the backup, then the dataset. By default that is the item's top dataset.

- **Into the dataset.** Files from the backup are written into the dataset's mountpoint. Files with the same name are overwritten, other files stay. The dataset is never rolled back or replaced. BombVault checks that the dataset is mounted, visible and writable, once before it starts and again right before it writes. Where a child dataset is mounted inside it, nothing is written: the child keeps its files, owner and permissions, and is restored from its own backup.
- **To a folder.** Pick a folder below `/mnt`. BombVault checks that the folder is on a mounted pool or share and that there is enough free space. This works without the SSH link and for datasets that no longer exist.
- **Select files** (Advanced): write only the files and folders you pick back into the dataset.
- **All datasets of this backup** (Advanced): every dataset of the tree into its own subfolder of the folder you pick. Datasets that were skipped in that backup are named.
- **From another server:** the **Recovery** page restores a dataset from another BombVault's repository, always into a folder.

The item's list of containers to stop is offered for an in-place restore as well. Those containers stay stopped for the whole restore, and container backups wait meanwhile.

### The safety snapshot {#safety-snapshot}

Before it writes into a dataset, BombVault takes a ZFS snapshot of that one dataset, named `bombvault-prerestore-<time>`. It is on by default; switching it off needs a second confirmation. If the snapshot cannot be taken, nothing is restored.

BombVault never deletes a safety snapshot by itself. The item lists them with their age and size, each with a **Delete** action, and warns when the oldest one is more than 30 days old, because it keeps deleted and changed data on the pool.

To go back after a restore, copy single files from `.zfs/snapshot/bombvault-prerestore-<time>` inside the dataset. `zfs rollback <dataset>@bombvault-prerestore-<time>` only works while it is the newest snapshot of that dataset. `zfs rollback -r` deletes every newer snapshot, automatic ones included.

### Restoring as a new dataset {#new-dataset}

BombVault does not create datasets. Create it on the server with the properties you want, then restore to a folder that is its mountpoint:

```
zfs create -o compression=lz4 cache/appdata-restored
```

and in BombVault restore to the folder `cache/appdata-restored` below `/mnt`.

## What is in the backup {#contents}

In the backup: the files and folders of every backed-up dataset, with their ownership, modes, timestamps and extended attributes as restic stores them.

Not in the backup:

- the datasets' ZFS properties (compression, recordsize, quota, mountpoint and the rest);
- the owner and mode of each dataset's top folder itself (everything below it is included). A restore into the dataset leaves the existing top folder as it is, a restore to a folder creates it with mode `0755`;
- existing ZFS snapshots;
- children that were skipped or left out;
- volumes.

To restore onto a new pool, create the datasets first with the properties you want. Whether NFSv4 ACLs, as TrueNAS uses them on SMB datasets, come back the way you expect has not been verified yet, so check a restore on your own data before you rely on them.

## Encrypted datasets {#encryption}

An encrypted dataset is backed up only while its key is loaded. Otherwise it is skipped with a warning; load the key with `zfs load-key` and mount the dataset. BombVault reads the data decrypted and stores it in restic's repository, which is encrypted. If you switched encryption off in BombVault, that repository is not.

## Leftover snapshots {#leftover-snapshots}

The snapshot of a backup is named `<dataset>@bombvault-<14 digits>`, for example `cache/appdata@bombvault-20260924021500` (UTC). BombVault removes it right after the backup. If that fails, for example because the dataset is busy or BombVault was stopped, BombVault removes it:

- before the next backup of that item,
- when BombVault starts, for every item, also with the domain switched off,
- when you delete the item,
- when you press **Remove now** on the item, which shows how many are left.

Only names that match exactly `bombvault-` plus 14 digits are removed. Safety snapshots, your own snapshots and automatic snapshots are never touched. To remove one by hand:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Reason codes {#reason-codes}

The page, the run history and the notifications name a problem with one of these codes. Most have the fix next to them on the page as well.

| Code | Meaning | What to do |
|---|---|---|
| `ssh-missing` | The SSH connection is not set up in this container. | Set up the SSH link as for VM backups. |
| `host-placeholder` | Host SSH: Address is still the placeholder, and `host.docker.internal` did not answer either. | Set Host SSH: Address to this server's LAN IP. |
| `host-fallback` | Host SSH: Address is still the placeholder, and `host.docker.internal` works. | Nothing, or set the LAN IP. |
| `ssh-unreachable` | The server cannot be reached over SSH. | Check address and port, and that SSH is switched on. |
| `ssh-auth` | The server refused BombVault's key. | Run the command shown on the connection card once on the server. |
| `zfs-not-found` | The SSH host has no `zfs` command. | Point Host SSH: Address at the machine that owns the pools. |
| `zfs-permission` | The SSH user may not run this zfs command. | Use root, or see [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` names a different host or user than the SSH fields. | Make them agree, or clear the SSH fields so both come from the URI. |
| `zfs-error` | zfs reported another error. | The details show its message. |
| `propagation-missing` | New mounts on the host do not reach the container. | Set the Access Mode of Host Data to Read/Write - Slave and restart BombVault. |
| `invalid-name` | A dataset name BombVault does not accept. | Rename the dataset. |
| `name-too-long` | A dataset in the tree is too long for a snapshot name. | Rename it, or add a dataset below it as the item. |
| `invalid-exclude` | An exclude pattern or a left-out child does not fit the item. | Fix the entry the message names. To leave out a whole child dataset, switch it off instead of writing a pattern. |
| `not-found` | The dataset does not exist on the server. | Remove the item, or recreate the dataset. Its backups stay restorable. |
| `not-filesystem` | This is a volume, not a filesystem. | See [Volumes](#volumes). |
| `overlaps-item` | The dataset overlaps an existing item. | See [Items never overlap](#overlap). |
| `docker-storage` | The tree holds Docker's image storage. | See [Docker's storage](#docker-storage). |
| `nothing-readable` | No dataset in the item can be read right now. | Look at the skipped datasets' codes. |
| `snapshot-failed` | The snapshot could not be created. | The details show zfs's message. |
| `containers-busy` | A container backup was still running when the containers had to stop. | Start again later. Scheduled runs wait on their own. |
| `consistency-stop-failed` | A container could not be stopped, so no snapshot was taken. | Check the container, or take it off the list. |
| `pre-snapshot-failed` | The command before the snapshot failed. | The run details show its output. |
| `container-unknown` | A listed container does not exist. | Remove it from the list. |
| `container-is-self` | BombVault cannot stop its own container. | Remove it from the list. |
| `leftover-snapshots` | Snapshots BombVault could not remove are still on the server. | Press **Remove now**, see [Leftover snapshots](#leftover-snapshots). |
| `zvol` | A volume in the tree, skipped. | See [Volumes](#volumes). |
| `canmount-off` | Never mounted (`canmount=off`), skipped. | If it holds data, mount it or move the data into a child dataset. |
| `legacy-mount` | Legacy mountpoint, skipped. | Give it a mountpoint below `/mnt`. |
| `no-mountpoint` | No mountpoint, skipped. | Give it a mountpoint below `/mnt`. |
| `not-mounted` | Not mounted on the server, skipped. | `zfs mount` it, or set `canmount=on`. |
| `key-not-loaded` | Encrypted and the key is not loaded, skipped. | `zfs load-key`, then mount it. |
| `snapdir-disabled` | Snapshot access is switched off, skipped. | `zfs set snapdir=hidden <dataset>`. The `.zfs` folder stays hidden. |
| `not-visible` | BombVault cannot see the dataset's mountpoint. | Move the mountpoint below the Host Data path, or map it into the container at the same path with Read/Write - Slave. |
| `shfs-only` | The dataset is only visible through `/mnt/user`, which hides snapshots. | Map `/mnt`, not `/mnt/user`, as Host Data. |
| `snapshot-not-visible` | The snapshot was created but did not appear inside BombVault. | Run **Test snapshot access**; see below. |
| `snapshot-loop` | The snapshot did not reach BombVault because Host Data does not pass new mounts through. | Set the Access Mode of Host Data to Read/Write - Slave and restart BombVault. |
| `backup-failed` | restic failed for this dataset. | The run details show why. |
| `not-reached` | The run ended before this dataset. | Run the backup again. |
| `gone` | The dataset is no longer on the server. | Nothing. Its backups stay restorable. |
| `read-only-mount` | BombVault can only read the dataset, so it cannot restore into it. | Set the mapping to Read/Write - Slave, or restore to a folder. |
| `destination-not-mounted` | The folder is not on a mounted pool or share. | Choose a folder on a pool or share. |
| `not-enough-space` | Not enough free space at the destination. | Free space or choose another folder. |
| `safety-snapshot-failed` | The safety snapshot could not be taken, so nothing was restored. | The details show zfs's message. |
| `safety-name-too-long` | The dataset name is too long for a safety snapshot. | Switch the safety snapshot off, or restore to a folder. |

### Checking what the container sees {#mountinfo}

**Test snapshot access** on an item takes a real snapshot of its tree, looks for it inside BombVault for every dataset, and removes it again. It is the quickest way to prove the whole path before the first scheduled run.

To look yourself, run this on the server:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Each line is one mount inside the container. The line of a dataset shows its path inside the container (below `/host/user`) and the dataset name. A `master:N` field on that line means the mount receives the mounts the host makes later, which is what snapshot access needs. When it is missing, set the Access Mode of Host Data to Read/Write - Slave and restart BombVault.

## TrueNAS SCALE {#truenas}

- When `LIBVIRT_URI` is set (as for VM backups on TrueNAS), BombVault takes the SSH host, user and port for its zfs commands from the URI, each one that is not set on its own. Without VM backups, set `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` and `LIBVIRT_SSH_PORT` instead. Add the variables under **Additional Environment Variables**.
- A user other than root needs permission on the item's top dataset, which then covers every dataset below it:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  A non-root SSH session on TrueNAS does not have `/usr/sbin` in its path; BombVault then calls `/usr/sbin/zfs` directly.
- The app's **Host Data** must be a host path above the datasets, for example `/mnt/tank`, not an ixVolume. With a host path the app passes the host's new mounts on to BombVault (`rslave`), which snapshot access needs.
