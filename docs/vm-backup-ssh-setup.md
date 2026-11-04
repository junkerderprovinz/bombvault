# VM backups

BombVault backs up KVM/libvirt VMs without mounting any libvirt path. It runs `virsh` on the host over SSH (`qemu+ssh://`), reads and writes the VM disks through the Host Data mount (`/mnt` on the host is `/host/user` in the container), and copies the UEFI NVRAM over SSH. Because nothing that belongs to libvirt is bind-mounted, BombVault can never get in the way of the Unraid VM Manager.

This guide covers the setup, the backup methods, VMs with Unraid snapshots and TrueNAS SCALE. Container backups need none of this.

ZFS dataset backups use the same SSH link, with the same key, host and user. Once VM backups connect, ZFS dataset entries connect too. See [ZFS datasets](zfs-datasets.md).

## What you configure {#what-you-configure}

| Setting | Where | Default | Meaning |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | template variable | `host.docker.internal` | The Unraid host, reached over SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | template variable | `22` | Unraid's SSH port |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | template variable | `root` | The SSH user on the host |
| Public key | **Settings, Integrations, Host SSH** | made on first start | Authorize it on the host |

BombVault makes its SSH key pair on the first start at `/config/ssh/id_ed25519`, kept in appdata. The host's key is pinned in `/config/ssh/known_hosts` on the first connection.

The template fills **Host SSH: Address** with the placeholder `192.168.x.x`. BombVault treats that as unset and tries `host.docker.internal`, which works on the default `bridge` network. On a custom network, replace it with the host's LAN IP (step 3 B).

## Step 1: Enable SSH on the Unraid host {#step-1}

1. In Unraid, open **Settings, Management Access** and set **Use SSH** to **Yes**.
2. If you changed the SSH port, note it. **Host SSH: Port** has to match.

## Step 2: Authorize BombVault's public key {#step-2}

Unraid's `/root` lives in memory and is wiped at every boot, so the key has to be on the flash drive as well. Run this in the Unraid terminal:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 and newer copy `/boot/config/ssh/root.pubkeys` into root's `authorized_keys` at every boot, so this survives reboots. It holds for any `root` login, whatever host name or port BombVault uses.

## Step 3: Networking {#step-3}

BombVault has to open a TCP connection from the container to the host's SSH port. Pick the case that matches your setup.

### A. BombVault on the default bridge network {#bridge}

- Leave **Host SSH: Address** at `host.docker.internal` or the template placeholder. The template adds `--add-host=host.docker.internal:host-gateway`, which points at the docker0 gateway, which is the host.
- Set **Host SSH: Port** to your SSH port.

### B. BombVault on a custom network {#custom-network}

A container on `br0.x`, macvlan or ipvlan cannot reach the host through `host.docker.internal`, because 172.17.0.1 is docker0 and not reachable from `br0.x`. Instead:

1. In Unraid, set **Settings, Docker, Host access to custom networks** to **Enabled**.
2. Set **Host SSH: Address** to the Unraid host's LAN IP, the address you open the web UI on.
3. If the container's network and the host are on different VLANs, allow the route on your router or firewall: from the container VLAN to the host IP on the SSH port, over TCP.
4. If the host's LAN IP still cannot be reached, use the host's shim IP on the container's subnet. Find it on the host with `ip -4 addr show | grep -B2 <container-subnet>`.

### Check that the host answers {#verify-reachability}

Run this in the Unraid terminal, with your IP and port:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` means the path works. `UNREACHABLE` means the networking in step 3 needs fixing before anything else.

## Step 4: Set the variables and test {#step-4}

1. In Unraid, open **Docker**, click BombVault, choose **Edit**, set **Host SSH: Address** (and **Host SSH: Port** if it is not 22) and press **Apply**. If the variables are missing, import the template again, because Unraid keeps the saved configuration of an existing container.
2. In BombVault, open **Settings, Integrations, Host SSH** and press **Test connection**. It should turn green.

The same call from the terminal lists your VMs:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Step 5: The method per VM {#per-vm-method}

Each VM has a **Method** in the **VM** card on its Settings tab (open the VM under **Backups**):

- **Graceful (shutdown)** is the default. It shuts the VM down, backs up the disks and starts it again, so the backup is always consistent. It waits up to ten minutes for the guest to shut down and never forces it off. A guest that still runs after that is left alone, and that backup fails with the reason.
- **Live snapshot** backs up a running VM through an external snapshot and `blockcommit`. It needs the qemu guest agent in the VM, for a quiesced snapshot, and the disk on `/mnt/cache` or `/mnt/diskX`, not `/mnt/user`. On a VM that is shut off, Live falls back to Graceful by itself, and when the snapshot cannot be made, it falls back as well.

## Changed blocks only {#changed-blocks}

A VM with qcow2 disks can be backed up by reading only what changed. Switch on **Changed blocks only** in the same card, and BombVault uses libvirt's backup API instead of the method above:

- It starts a pull-mode backup job (`virsh backup-begin`) that creates a checkpoint named `bombvault-<time>` at the same instant, and reads the disks over the SSH link through a forwarded Unix socket. When the guest agent answers, the guest's filesystems are frozen only while the job starts. A thaw that fails is tried again for about fifteen seconds, and if the guest still does not thaw, BombVault sends a notification.
- The first run reads every allocated block. Later runs read only the blocks the checkpoint's bitmap marks as written since the previous backup.
- Each disk is stored as a folder of fixed-size segments holding what the guest sees. Unchanged segments are taken over from the previous snapshot without being read, so every snapshot is complete and restores on its own.
- A restore rebuilds the disk from its segments and converts it back to qcow2 with `qemu-img`. The disk has the same content as at backup time, but the file is a new qcow2 file, and a backing chain comes back flattened.
- The check before such a restore lists each disk it rebuilds and counts it at the guest's full size, twice for qcow2, since the raw image and its converted copy sit side by side until the old file is replaced. That is the most a disk can take, so a shortfall by this count leaves the space line grey instead of blocking. The restore check after the first backup, and **Check restore**, also make sure the snapshot holds every segment of every disk at the size it should have.
- Between runs the VM has no checkpoint on record in libvirt. BombVault keeps the definition of the newest `bombvault-` checkpoint under `/config/vm-checkpoints` and removes only libvirt's record of it (`virsh checkpoint-delete --metadata`). Its bitmap stays in the qcow2 file and keeps counting writes, and the next run puts the checkpoint back (`virsh checkpoint-create --redefine`) before it starts. libvirt refuses to undefine a VM that has checkpoints, so this is what lets Unraid's **Remove VM** and a restore work as usual.
- A backup job that BombVault left running, because it stopped in the middle of a backup, is ended when BombVault starts again, together with the checkpoints that run left on record.
- Turning the switch off, taking the VM out of the schedule or deleting its backups also ends such a job, and deletes the bitmap too while the VM runs. On a VM that is off, libvirt cannot delete it, and it stays in the image unused.
- Checkpoints of other tools are left alone. A VM that has some cannot be restored until they are deleted, and the restore stops before it touches the VM.

The switch falls back on its own. A missing or broken checkpoint, a disk that changed size, or a newest snapshot that is not a changed-block one means the whole disk is read once more. A VM that is off, a raw or zvol disk, no host SSH link, a libvirt without the backup API, or too little free space under `/config` means the usual backup with the method above. The card says which of these the last run used and why.

Requirements: libvirt with the backup API (tested with libvirt 12.2 and QEMU 10.2 on Unraid 7.3), sshd with `AllowStreamLocalForwarding yes` (the default, also on Unraid, which turns TCP forwarding off), and qcow2 disks. libvirt refuses checkpoints on raw disks, and Unraid creates raw vdisks by default. The VM keeps running during the backup, as with Live, and the segments waiting for restic take up to 4 GiB under `/config`.

## VMs with Unraid snapshots {#unraid-snapshots}

A snapshot that the Unraid VM Manager takes turns a qcow2 disk into a chain: the original image at the bottom, and on top of it one file per snapshot, each holding only what changed since the one below. The VM writes into the newest file. A disk with two snapshots is three files.

BombVault backs up such a VM in full:

- **Every file of the chain.** It follows each disk's backing chain to its end and backs up every file in it, together with Unraid's snapshot list (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, read over SSH). A chain that cannot be followed to its end, for example because one file in the middle is gone, fails the backup and names the file.
- **A live backup keeps the snapshots.** **Live snapshot** puts a temporary overlay on top while it reads the disk and merges it back afterwards, only into the image right below. The chain is the same after the backup as before it. A VM without snapshots behaves as it always has.
- **A restore brings the chain back.** A restore in place writes every file back where it was and puts Unraid's snapshot list back, so the VM Manager lists the snapshots again. A restore next to the original writes the files into the folder of the copy and points the restored disks at the restored files, so the copy never reads from the original's images. Unraid's snapshot list goes back only with a restore in place, because it names the files where they were backed up. Without it the copy boots the same, and the VM Manager just lists no snapshots for it.
- **Changed blocks only** reads what the guest sees, so its restore comes back as one flattened qcow2 file without the snapshots.

### Example: a VM with two snapshots {#example-unraid-snapshots}

The VM `homeassistant` has one disk, `vdisk1.qcow2`, and two snapshots you took in the VM Manager before two updates. Its disk is now a chain of three files. It runs all the time, so it uses **Live snapshot**.

1. Open **Backups**, then `homeassistant`, then its **Settings** tab. In the **VM** card, check that **Method** is **Live snapshot** and **Changed blocks only** is off.
2. Press **Back up now** in the head of the page.
3. When the run is done, its line in **Recent runs** on the **Verify** tab is green. The VM kept running the whole time, and the backup holds all three files of the disk and Unraid's snapshot list.
4. In the VM Manager, the two snapshots are still there, and the disk still reads through the same three files.

A month later an update goes wrong and you want yesterday's state back next to the broken one, to compare:

1. Open **Restore**, pick **VMs** and click `homeassistant`.
2. Keep the recommended backup from last night and choose **Next to the original** under **Where to**.
3. Press **Restore**. The three files land in a folder named after the VM with `-restored`, and the restored disk points at the restored files. Nothing changes on the running VM.

To roll the whole VM back instead, choose **Replace original** with **Back up the current state first** on. BombVault backs up the broken state, shuts the VM down, writes the three files and Unraid's snapshot list back, and starts it again.

## TrueNAS SCALE {#truenas-scale}

The steps above are written for Unraid, but the same `qemu+ssh://` link works against any libvirtd the container can reach, TrueNAS SCALE included, which has VMs on libvirt again since 25.04.2. On TrueNAS the connection string needs an override and the key goes onto root in TrueNAS's own user settings. Three more things change what a backup holds: disks on zvols, vTPM state, and the names TrueNAS gives its VMs.

### 1. The libvirt socket {#truenas-socket}

TrueNAS's libvirtd does not listen where the connection string built from the **Host SSH** fields expects. Set `LIBVIRT_URI` instead, with the extra `?socket=` parameter:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault uses `LIBVIRT_URI` exactly as written, and `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` and `LIBVIRT_SSH_PORT` are not needed. For each of them that is unset, BombVault takes the host, user and port from the URI for its own SSH commands too, so the NVRAM copy and ZFS dataset backups reach the same machine as `virsh`. A variable you set yourself always wins, and the boot log names the target it took from the URI. On TrueNAS a user other than root also needs `zfs allow <user> snapshot,destroy,mount <dataset>` for ZFS dataset backups, see [ZFS datasets](zfs-datasets.md#truenas).

Put the TrueNAS LAN address into the URI. With neither `LIBVIRT_URI` nor `LIBVIRT_HOST` set, BombVault takes the app's default gateway as the host, because `host.docker.internal` does not resolve in a TrueNAS app, and the boot log names the address it picked. That is enough for ZFS dataset backups. VM backups still need the URI for the socket.

The app needs the `SETPCAP` capability for VM backups, next to `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` and `SETUID`. Without it libvirt stops with "cannot apply process capabilities". The YAML under [TrueNAS SCALE](getting-started.md#truenas) sets all of them, together with the `SYS_CHROOT` that snapshot instead of stop needs.

On TrueNAS SCALE 25.10.0 the socket path was checked on real hardware: the stock `qemu:///system` fails with `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, and the same call with `?socket=/run/truenas_libvirt/libvirt-sock` lists the domains.

### 2. BombVault's key on TrueNAS {#truenas-key}

Step 2 above writes to Unraid's flash drive and does not apply here. On TrueNAS:

1. Turn on the SSH service under **System, Services**.
2. Under **Credentials, Users**, edit **root** and paste BombVault's public key from **Settings, Integrations, Host SSH** as its SSH public key. TrueNAS keeps it across reboots and updates.

Fresh installs of TrueNAS 24.10 and newer give root no password, and the key login needs none. If you would rather not use root, authorize the key for an admin account with sudo and put its name in place of `root` in `LIBVIRT_URI`. ZFS dataset backups then need `zfs allow` as described above.

A ZFS replica to a second server logs in with a key of its own, which the replica dialog shows. The Host SSH key does not authorize a replica, and the replica key does not authorize VM backups.

!!! warning "TrueNAS does not see changes made with virsh"
    TrueNAS's middleware, the layer behind its VM screens, is meant to be the only way its VMs are changed. A `virsh` command over this SSH link, which is what BombVault's VM backup and restore run, goes around it, and TrueNAS may later reconcile or overwrite state it did not create. This comes from TrueNAS's documented behaviour and has not been seen on a real box. It is the same kind of risk as Unraid's VM Manager reconciling changes made outside it, only sharper on TrueNAS.

### 3. Disks on zvols {#truenas-zvols}

TrueNAS VM disks are often zvols, ZFS block devices at `/dev/zvol/<pool>/<dataset>`, not qcow2 files. restic cannot back up a block device by its path the way it backs up a file, so BombVault recognises a zvol disk in the domain XML (`<source dev="...">` instead of `<source file="...">`) and handles it over the same SSH link. The BombVault container needs no ZFS tools.

A backup of a zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fixes a point in time, also while the VM runs.
2. `zfs clone -o volmode=dev -o readonly=on` makes that snapshot readable as a device. `volmode=dev` keeps the host from scanning the guest's partitions.
3. `dd` reads the device, and the raw image goes into restic as one file, `/vm-disks/<dataset>.img`, tagged `vm:<name>:zvol:<dev>`. restic cuts it into chunks by content, so blocks that did not change since the last backup are not stored again.
4. The clone and the snapshot are destroyed afterwards, whether the backup worked or not. If BombVault stopped in the middle of a backup, the next backup of that zvol removes the `bombvault-vm-<time>` snapshot and its `<dataset>-bombvault-vm-<time>` clone first. It leaves every other snapshot alone, and keeps one that something else was cloned from. The log names what it removed or kept.

BombVault does not use `zfs send` for this. Every record of a send stream carries the snapshot's GUID and a running checksum, so restic would find nothing of the last backup to reuse and store the whole disk again each time. Measured on TrueNAS SCALE 25.10 with a 512 MiB zvol holding 200 MiB of data: the first backup added 211 MB, a second one with nothing changed 52 KB, and one after 1 MiB had changed 3.6 MB. The same zvol through `zfs send` added 214 MB on every run.

Each run also writes a small snapshot tagged `vm:<name>` with the domain XML, once all disks are in. A VM whose disks are all zvols has no file snapshot, and this is the one that lists the run under the VM and makes it restorable.

A restore writes each zvol back in place, as a file disk is written back to its file, so the restored VM boots the restored data:

1. Before anything is touched, BombVault checks that the run holds an image of every zvol disk. A run that lacks one is refused and the VM is left as it is.
2. An existing zvol gets a snapshot first, `<dataset>@bombvault-before-restore-<time>`, with the time in UTC like every snapshot name BombVault makes, so it can be hours off the local clock. The log names it, and `zfs rollback` to it undoes the restore while it is the newest snapshot of the zvol. Once a newer one exists, such as the next restore's, `zfs rollback -r` does it and destroys the newer snapshots. It stays until you remove it with `zfs destroy` once the restored VM works.
3. A zvol that no longer exists is created at the size it had when it was backed up, and an existing one of another size is set back to that size.
4. `dd` writes the image onto `/dev/zvol/<dataset>`. With compression on, the TrueNAS default, the empty parts of the image come back as holes and take no space.

**A restore onto another instance needs a destination pool.** It moves file disks into a destination folder, but a folder names no ZFS pool, so without one BombVault refuses the restore before it starts. Pass the pool as the `zvolPool` parameter of `POST /api/foreign/restore`: each zvol keeps its path below the pool (`tank/vms/win10/disk0` onto `flashpool` becomes `flashpool/vms/win10/disk0`), and the domain XML points at it. The **From an old backup folder** window on the Restore page has no field for this, so it is reachable through a direct API call only. A restore on the same instance writes each zvol back onto itself.

**Export as files leaves zvols out.** A VM whose disks are all zvols is refused with that reason, so restore it from a backup instead. For a VM with file disks as well, the export holds the file disks and a note that names the zvols it left out.

This was tried on TrueNAS SCALE 25.10 with ZFS 2.3.4: three backups of a scratch zvol, a restore in place after the zvol had been overwritten and grown, and a restore onto a zvol that had been deleted. Each restore read back with the same SHA-256 as the source, the zvol had its old size again, and no snapshot or clone was left behind. A whole VM whose only disk is a zvol was backed up running, with Graceful and with Live, and shut off, and a whole VM restore (undefine, write the zvols, define, boot) ran there as well: the zvol read back with the source's checksum and the VM started.

File disks, as Unraid uses them, are not affected by any of this.

### 4. vTPM state {#truenas-vtpm}

A VM can have a virtual TPM, which Secure Boot and Windows 11 guests need. Its state lives outside the disk and the NVRAM, so a backup of only those two would bring a guest back with a new, empty TPM.

- **A passthrough TPM**, a real chip passed to the guest, names its path in the domain XML. BombVault saves its state next to the NVRAM over SSH and writes it back before `virsh define`, best effort like the NVRAM: a failed read or write is logged and does not fail the backup or restore.
- **An emulated vTPM**, the kind TrueNAS sets up for Windows 11 and Secure Boot guests, publishes no state path in the domain XML. BombVault then reports "no TPM path found" instead of guessing a path. Keep such a guest's recovery key, such as its BitLocker key, at hand before you restore it.
- **You cannot attach a TPM to a TrueNAS VM yourself.** TrueNAS SCALE 25.10's VM device API accepts only `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` and `USB`, and refuses `TPM` as an unknown type.

The TPM handling is unit-tested only and has not run against a real TrueNAS guest with a vTPM. A VM without a TPM, as on Unraid, backs up and restores exactly as before.

NVRAM is captured as usual. TrueNAS keeps it per VM at `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, and the domain XML names that path.

### 5. VM names on TrueNAS {#truenas-names}

On Unraid a VM's libvirt name is the name you chose. TrueNAS makes up the libvirt name, and has changed how twice:

| TrueNAS version | libvirt name | Where your name lives |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, such as `1_debian` | in the libvirt name, behind the `{id}_` |
| 26 | the VM's UUID, such as `550e8400-e29b-41d4-a716-446655440000` | only in the domain XML's `<title>` |

On a host BombVault detects as TrueNAS, or that `PLATFORM=truenas` names, the Backups list shows your name: `debian` instead of `1_debian` on 25.10, and the `<title>` instead of the UUID on 26, or the UUID when there is no title. This is only what the list shows. Every `virsh` call and the per-VM settings still use the libvirt name, so backup and restore always act on the right VM. On TrueNAS 26 that means a VM that TrueNAS creates again with a new UUID shows up as a new entry without its settings, even though its name looks the same. On Unraid nothing changes, also for a VM you named `10_Windows`.

The 25.10 scheme was seen on a real TrueNAS SCALE 25.10 box. The TrueNAS 26 scheme comes from TrueNAS's documentation and is unit-tested only.

## Troubleshooting {#troubleshooting}

| Symptom | Cause and fix |
|---|---|
| **Test connection** hangs, then fails | The host cannot be reached. Check step 3 (network, VLAN, firewall) and **Host SSH: Address** and **Port**. |
| `Permission denied (publickey)` although the key is in `authorized_keys` | sshd's `StrictModes` rejects the key file because a parent folder has the wrong owner or mode. That is common when `/root/.ssh` is a link to the FAT flash drive (`/boot/config/ssh/...`), which is writable for everyone. The host log shows `Authentication refused: bad ownership or modes for directory ...`. Add `StrictModes no` to the host's sshd config in the global section, before any `Match` block, then run `/etc/rc.d/rc.sshd restart`. Unraid writes `/etc/ssh/sshd_config` anew at every boot, so persist it as described below. |
| `Permission denied (publickey)` and the key really is missing | Repeat step 2, and check that SSH is on. |
| `Host key verification failed` | Run `docker exec BombVault rm -f /config/ssh/known_hosts` and test again. |
| The check in step 3 says `UNREACHABLE` | The container cannot reach the host's SSH port. See step 3: host access to custom networks, VLAN routing, or the shim or LAN IP. |
| The variables are missing under **Edit** | Import the template again. An existing container keeps its old saved configuration. |
| A VM with Unraid snapshots fails and names a file | The disk's backing chain cannot be followed to its end. Check in the VM Manager that every snapshot of that disk is still there, and that the named file exists below `/mnt`. |

More VM symptoms are in [Troubleshooting](troubleshooting.md#vm-backup-will-not-connect).

## Keeping it across reboots {#persistence}

Unraid writes `/etc/ssh/sshd_config` anew at every boot, so changes on the host side have to be made again at boot:

- The **SSH port** is kept in `/boot/config/ident.cfg`, set through the Unraid web UI.
- **`StrictModes no`**, if you needed it above, is not kept. Apply it at boot from `/boot/config/go`:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- BombVault's own SSH key and `~/.ssh/config` live in `/config` (appdata) and are made again at every start, so they survive restarts of the container and the host.

## Security {#security}

The authorized key grants root on the host, the same level of trust as the Docker socket BombVault already uses. Keep BombVault on a trusted network. The key is ed25519, stored with mode `0600` in appdata, and the host's key is pinned. Every host command is passed as separate arguments, never through a remote shell, and the SSH connection uses `BatchMode` and a connect timeout.
