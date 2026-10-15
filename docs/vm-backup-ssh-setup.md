# VM Backup over SSH: Setup & Networking

BombVault backs up KVM/libvirt VMs **without mounting any libvirt path**. It runs
`virsh` **on the Unraid host over SSH** (`qemu+ssh://`), reads/writes VM **disks**
through the existing Host Data mount (`/mnt` → `/host/user`), and transfers the
UEFI **NVRAM** over SSH. Because nothing libvirt-owned is bind-mounted, BombVault
can never interfere with the Unraid VM Manager.

This guide is the exact configuration. Container backup needs none of this; it
is only for VM backup.

**Also used by ZFS datasets.** ZFS dataset backups run their `zfs` commands over
the same SSH link, with the same key, host and user. Once VM backup connects,
ZFS dataset entries connect too; nothing else needs setting up for them. See
[ZFS datasets](https://junkerderprovinz.github.io/bombvault/zfs-datasets/).

---

## What you configure

| Setting | Where | Default | Meaning |
|---|---|---|---|
| `Host SSH: Address` (`LIBVIRT_HOST`) | template var | `host.docker.internal` | Unraid host address reached over SSH |
| `Host SSH: Port` (`LIBVIRT_SSH_PORT`) | template var | `22` | Unraid's SSH port |
| `Host SSH: User` (`LIBVIRT_SSH_USER`) | template var | `root` | SSH user on the host |
| Public key | Settings → Integrations → Host SSH | (auto-generated) | Authorize on the host |

The SSH keypair is generated automatically on first start at
`/config/ssh/id_ed25519` (persisted in appdata). The host key is pinned in
`/config/ssh/known_hosts` on first connect.

The template pre-fills `Host SSH: Address` with the placeholder `192.168.x.x`.
BombVault treats that value as unset and tries `host.docker.internal`, which
works on the default `bridge` network. On a custom network, replace it with the
host's LAN IP (Step 3 B).

---

## Step 1: Enable SSH on the Unraid host

1. **Settings → Management Access → Use SSH = Yes.**
2. If you changed the **SSH port** (e.g. to a non-default port), note it; you'll set
   `Host SSH: Port` to match.

## Step 2: Authorize BombVault's public key (persistent)

Unraid's `/root` is tmpfs (wiped on reboot), so the key must also live on the
flash. From the **Unraid terminal**:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9+ restores `/boot/config/ssh/root.pubkeys` into root's
`authorized_keys` at every boot, so this survives reboots. The authorization is
**host/port independent**: it applies to any `root` login.

## Step 3: Networking (this is the part that varies)

BombVault must be able to open a TCP connection from the container to the host's
SSH port. Pick the row matching your setup:

### A. BombVault on the default `bridge` network (simplest)
- Leave `Host SSH: Address = host.docker.internal`, or the template placeholder (the template adds
  `--add-host=host.docker.internal:host-gateway`, which resolves to the docker0
  gateway = the host).
- `Host SSH: Port` = your SSH port.

### B. BombVault on a custom network (`br0.x`, macvlan/ipvlan)
A container on `br0.x` cannot reach the host via `host.docker.internal`
(172.17.0.1 is docker0, unreachable from `br0.x`). Instead:
1. **Settings → Docker → Host access to custom networks = Enabled.**
2. Set `Host SSH: Address` to the **Unraid host's LAN IP** (the IP you open the
   web UI on, e.g. `192.168.x.x`).
3. If the container's network and the host are on **different VLANs**, allow the
   route on your router/firewall: `container VLAN → host-IP : SSH-port (tcp)`.
4. If the host LAN IP is unreachable, use the host's **shim** IP on the
   container's subnet; find it on the host with
   `ip -4 addr show | grep -B2 <container-subnet>`.

### Verify reachability (before anything else)
From the Unraid terminal, replace the IP/port with yours:
```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.x.x/<port>' && echo OPEN || echo UNREACHABLE
```
`OPEN` = the path works. `UNREACHABLE` = fix networking (Step 3) before going on.

## Step 4: Set the variables & test

1. **Docker → BombVault → Edit** → set `Host SSH: Address` (+ `Host SSH: Port` if not 22)
   → **Apply**. *(If the variables don't appear, re-import the template, because Unraid
   keeps an existing container's saved config.)*
2. **Settings → Integrations → Host SSH → Test connection** → green.
   Or from the terminal (the exact call BombVault makes):
   ```sh
   docker exec BombVault virsh -c "qemu+ssh://root@192.168.x.x:<port>/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
   ```
   → lists your VMs.

## Step 5: Per-VM method

Each VM has a **Method** in the **VM** card on its page's Settings tab (open the VM
under **Backups**):
- **Graceful (shutdown)**: default; shuts the VM down, backs up the disks,
  restarts it. Always consistent.
- **Live snapshot**: backs up a running VM via an external snapshot +
  `blockcommit`. Requirements: the **qemu guest agent** installed in the VM
  (for a quiesced, app-consistent snapshot) and the disk on **`/mnt/cache`** or
  `/mnt/diskX` (not `/mnt/user`). On a shut-off VM, live automatically falls back
  to graceful.

## Changed blocks only

A VM with qcow2 disks can be backed up by reading only what changed. Switch on
**Changed blocks only** in the same card, and BombVault uses libvirt's backup API
instead of the method above:

- It starts a pull-mode backup job (`virsh backup-begin`) that creates a
  checkpoint named `bombvault-<time>` at the same instant, and reads the disks
  over the SSH link through a forwarded Unix socket. When the guest agent
  answers, the guest's filesystems are frozen only while the job starts. A
  thaw that fails is tried again for about fifteen seconds, and if the guest
  still does not thaw, BombVault sends a notification.
- The first run reads every allocated block. Later runs read only the blocks
  the checkpoint's bitmap marks as written since the previous backup.
- Each disk is stored as a folder of fixed-size segments holding what the guest
  sees. Unchanged segments are taken over from the previous snapshot without
  being read, so every snapshot is complete and restores on its own.
- A restore rebuilds the disk from its segments and converts it back to qcow2
  with `qemu-img`. The disk has the same content as at backup time, but the file
  is a new qcow2 file, and a backing chain comes back flattened.
- The check before such a restore lists each disk it rebuilds and counts it at
  the guest's full size, twice for qcow2, since the raw image and its converted
  copy sit side by side until the old file is replaced. That is the most a disk
  can take, so a shortfall by this count leaves the space line grey instead of
  blocking. The restore check after the first backup, and **Check restore**,
  also make sure the snapshot holds every segment of every disk at the size it
  should have.
- Between runs the VM has no checkpoint on record in libvirt. BombVault keeps
  the definition of the newest `bombvault-` checkpoint under
  `/config/vm-checkpoints` and removes only libvirt's record of it
  (`virsh checkpoint-delete --metadata`). Its bitmap stays in the qcow2 file
  and keeps counting writes, and the next run puts the checkpoint back
  (`virsh checkpoint-create --redefine`) before it starts. libvirt refuses to
  undefine a VM that has checkpoints, so this is what lets Unraid's **Remove
  VM** and a restore work as usual.
- A backup job that BombVault left running because it stopped in the middle
  of a backup is ended when BombVault starts again, and so are the
  checkpoints that run left on record.
- Turning the switch off, taking the VM out of the schedule or deleting its
  backups also ends such a job and deletes the bitmap too while the VM runs. On a VM that is off,
  libvirt cannot delete it, and it stays in the image unused.
- Checkpoints of other tools are left alone. A VM that has some cannot be
  restored until they are deleted, and the restore stops before it touches
  the VM.

The switch falls back on its own. A missing or broken checkpoint, a disk that
changed size, or a newest snapshot that is not a changed-block one means the
whole disk is read once more. A VM that is off, a raw or zvol disk, no host SSH
link, a libvirt without the backup API, or too little free space under
`/config` means the usual backup with the method above. The card says which of
these the last run used and why.

Requirements: libvirt with the backup API (tested with libvirt 12.2 and QEMU
10.2 on Unraid 7.3), sshd with `AllowStreamLocalForwarding yes` (the default,
also on Unraid, which turns TCP forwarding off), and qcow2 disks. libvirt refuses
checkpoints on raw disks, and Unraid creates raw vdisks by default. The VM keeps
running during the backup, as with Live, and the segments waiting for restic
take up to 4 GiB under `/config`.

---

## TrueNAS Scale

The steps above (SSH enable, key authorization, networking, per-VM method)
are written for Unraid, but the same `qemu+ssh://` mechanism works against
any reachable libvirtd, including TrueNAS Scale, which has shipped libvirt-
based VMs again since 25.04.2. Five things differ on TrueNAS: the connection
string needs an explicit override (1) and BombVault's key goes onto root in
TrueNAS's own user settings (2) before anything works at all, and three further differences affect what a backup
actually captures: zvol-backed disks (3), vTPM state (4), and how TrueNAS
names its libvirt domains (5).

### 1. TrueNAS's libvirtd socket is non-standard

TrueNAS Scale's libvirtd does not listen where the default `qemu+ssh://`
connection string built from `Host SSH: Address`/`User`/`Port` expects. Set
`LIBVIRT_URI` directly instead, with the extra `?socket=...` query parameter
the built-string form has no way to express:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

When `LIBVIRT_URI` is set, BombVault uses it **verbatim**: `LIBVIRT_HOST`,
`LIBVIRT_SSH_USER`, and `LIBVIRT_SSH_PORT` are ignored for the connection
string itself. They don't need to be set at all: for every one of them that is
unset, BombVault takes the host, user and port from the URI for its own SSH
commands too, so the NVRAM transfer and ZFS dataset backups reach the same
machine as `virsh`. An explicitly set variable always wins, and the boot log
names the target taken from the URI. On TrueNAS a user other than root also
needs `zfs allow <user> snapshot,destroy,mount <dataset>` for ZFS dataset
backups; see [ZFS datasets](https://junkerderprovinz.github.io/bombvault/zfs-datasets/#truenas).

Put the TrueNAS LAN address into the URI. With neither `LIBVIRT_URI` nor
`LIBVIRT_HOST` set, BombVault takes the app's default gateway as the host,
because `host.docker.internal` does not resolve in a TrueNAS app, and the boot
log names the address it picked. That is enough for ZFS dataset backups; VM
backups still need the URI for the socket.

The app needs the `SETPCAP` capability for VM backups, next to `CHOWN`,
`DAC_OVERRIDE`, `FOWNER`, `SETGID` and `SETUID`. Without it libvirt stops with
"cannot apply process capabilities". The YAML under
[TrueNAS SCALE](https://junkerderprovinz.github.io/bombvault/getting-started/#truenas)
sets all six.

### 2. BombVault's key on TrueNAS

Step 2 above writes to Unraid's flash and does not apply here. On TrueNAS:

1. Turn on the SSH service under **System, Services**.
2. Under **Credentials, Users**, edit **root** and paste BombVault's public key
   (from **Settings, Integrations, Host SSH**) as its SSH public key. TrueNAS
   keeps it across reboots and updates.

Fresh installs of TrueNAS 24.10 and newer give root no password, and the key
login needs none. If you would rather not use root, authorize the key for an admin
account with sudo and put its name in place of `root` in `LIBVIRT_URI`; ZFS
dataset backups then need `zfs allow` as described above.

A ZFS replica to a second server logs in with a key of its own, which the
replica dialog shows. The Host SSH key does not authorize a replica, and the
replica key does not authorize VM backups.

### Caveat: direct `virsh` changes are invisible to TrueNAS's middleware

TrueNAS's own middleware (the layer behind its VM UI) is the intended control
path for its VMs. A `virsh` command run directly over this SSH link (which
is exactly what BombVault's VM backup/restore does) bypasses that
middleware, and TrueNAS may reconcile or overwrite state it didn't originate
(sourced from TrueNAS's publicly documented middleware behavior, not
independently verified against a running instance). This is the same risk
class as Unraid's own VM Manager reconciling changes made outside it, only
sharper on TrueNAS since its middleware is more actively involved in VM
lifecycle management.

**Verified on real hardware (TrueNAS SCALE 25.10.0, 2026-08-27):** the
non-default socket path above is correct and required. Connecting with the
stock `qemu:///system` URI fails on TrueNAS with `Failed to connect socket to
'/var/run/libvirt/libvirt-sock': No such file or directory`, while the same
call with `?socket=/run/truenas_libvirt/libvirt-sock` lists domains normally.
Also confirmed on that box: TrueNAS names each libvirt domain `<id>_<name>`
(for example `1_bvzvol`) and puts the display name you typed in the UI into
the domain's `<title>`, which is the pairing BombVault's VM list relies on.

What is still *not* verified here is the middleware-reconciliation risk
described just above: it remains sourced from TrueNAS's documented behaviour,
not from an observed incident.

### 3. VM disks backed by zvols (raw block devices)

TrueNAS Scale VM disks are commonly **zvols**: ZFS block devices at
`/dev/zvol/<pool>/<dataset>`, not qcow2 files. restic cannot back up a block
device by path the way it backs up a file, so BombVault recognises a zvol disk
from the domain XML (`<source dev="...">` rather than `<source file="...">`)
and handles it on its own, over the same SSH connection as every other command
on this page. The BombVault container needs no ZFS tools.

A backup of a zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fixes a point in time, also
   while the VM runs.
2. `zfs clone -o volmode=dev -o readonly=on` makes that snapshot readable as a
   device. `volmode=dev` keeps the host from scanning the guest's partitions.
3. `dd` reads the device, and the raw image goes into restic as one file,
   `/vm-disks/<dataset>.img`, tagged `vm:<name>:zvol:<dev>`. restic cuts it
   into chunks by content, so blocks that did not change since the last backup
   are not stored again.
4. The clone and the snapshot are destroyed afterwards, whether the backup
   worked or not. If BombVault stopped in the middle of a backup, the next
   backup of that zvol removes the `bombvault-vm-<time>` snapshot and its
   `<dataset>-bombvault-vm-<time>` clone first. It leaves every other snapshot
   alone, and keeps one that something else was cloned from; the log names
   what it removed or kept.

BombVault does not use `zfs send` for this. Every record of a send stream
carries the snapshot's GUID and a running checksum, so restic would find
nothing of the last backup to reuse and store the whole disk again each time.
Measured on TrueNAS SCALE 25.10 with a 512 MiB zvol holding 200 MiB of data:
the first backup added 211 MB, a second one with nothing changed 52 KB, and one
after 1 MiB had changed 3.6 MB. The same zvol through `zfs send` added 214 MB
on every run.

Each run also writes a small snapshot tagged `vm:<name>` with the domain XML,
once all disks are in. A VM whose disks are all zvols has no file snapshot, and
this is the one that lists the run under the VM and makes it restorable.

A restore writes each zvol back in place, as a file disk is written back to its
file, so the restored VM boots the restored data:

1. Before anything is touched, BombVault checks that the run holds an image of
   every zvol disk. A run that lacks one is refused and the VM is left as it is.
2. An existing zvol gets a snapshot first, `<dataset>@bombvault-before-restore-<time>`.
   The log names it, and `zfs rollback` to it undoes the restore. It stays
   until you remove it with `zfs destroy` once the restored VM works.
3. A zvol that no longer exists is created at the size it had when it was
   backed up, and an existing one of another size is set back to that size.
4. `dd` writes the image onto `/dev/zvol/<dataset>`. With compression on (the
   TrueNAS default), the empty parts of the image come back as holes and take
   no space.

**A restore onto another instance needs a destination pool.** It moves file
disks into a destination folder, but a folder names no ZFS pool, so without one
BombVault refuses the restore before it starts. Pass the pool as the `zvolPool`
parameter of `POST /api/foreign/restore`: each zvol keeps its path below the
pool (`tank/vms/win10/disk0` onto `flashpool` becomes
`flashpool/vms/win10/disk0`), and the domain XML points at it. **The From an
old backup folder window on the Restore page has no field for this, so it is
reachable through a direct API call only.** A restore on the same instance
writes each zvol back onto itself.

**Export as files leaves zvols out.** A VM whose disks are all zvols is
refused with that reason; restore it from a backup instead. For a VM with
file disks as well, the export holds the file disks and a note that names the
zvols it left out.

**Tried on real hardware (TrueNAS SCALE 25.10, ZFS 2.3.4).** The backup and
restore code ran over SSH against a scratch zvol: three backups, a restore in
place after the zvol had been overwritten and grown, and a restore onto a zvol
that had been deleted. Each restore read back with the same SHA-256 as the
source, the zvol had its old size again, and no snapshot or clone was left
behind. On TrueNAS SCALE 25.10 BombVault also backed up a whole VM whose only
disk is a zvol, running (with the graceful and the live method) and shut off.
A whole VM restore on TrueNAS (undefine, write the zvols, define, boot) is
covered by tests with fakes only.

File-backed (Unraid) VM disks are not affected by any of this.

**You cannot attach a TPM to a TrueNAS VM yourself.** TrueNAS SCALE 25.10's VM
device API accepts only `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` and
`USB`; asking it for a `TPM` device is rejected outright as an unknown type. A
VM created through the API therefore has no TPM in its domain XML, which is
what was measured here.

Read that narrowly. It does **not** prove a TrueNAS domain can never carry TPM
state: TrueNAS provisions an emulated vTPM of its own for Windows 11 and
Secure-Boot guests, and that case was not exercised on the test box. The
practical guidance for such a guest is unchanged and is the same one that
applies on any platform: an emulated vTPM does not publish a state path in the
domain XML, so BombVault reports "no TPM path found" rather than guessing, and
you should keep that guest's recovery key to hand before restoring it.

NVRAM is unaffected and is captured normally. TrueNAS stores it per domain at
`/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, and the path is named directly
in the domain XML.

### 4. vTPM state (Secure-Boot/Windows-11-class guests)

A TrueNAS Scale VM can also have a **vTPM** (virtual TPM) device, required
for Secure Boot and for Windows 11 guests. Its state lives **outside** the
domain's disk and NVRAM, so a backup that only captures those two would leave
a restored Secure-Boot/vTPM guest unable to boot correctly (a fresh, empty
TPM identity, not the guest's real one).

BombVault parses a domain's `<tpm>` element (`internal/virshcli`) the same
way it already parses `<os><nvram>`. libvirt's schema supports several `<tpm>`
backend types, but only the **passthrough** shape
(`<backend type='passthrough'><device path='...'/></backend>`) is documented
to carry an explicit path directly in the domain XML, and that shape is a
real hardware TPM chip, not the kind of vTPM TrueNAS actually provisions. The
**emulated** (software) vTPM TrueNAS uses for Secure Boot/Windows 11 guests
does not, per libvirt's public documentation, expose its swtpm state
directory as a domain-XML attribute; a newer `external`-backend shape can
carry a UNIX socket path and is plausibly how TrueNAS's middleware wires its
vTPM, but its exact attribute layout as rendered by TrueNAS is **not
confirmed against real hardware**. Rather than guess at either shape,
BombVault's parser recognizes only the well-documented passthrough case and
otherwise reports "no TPM path found": the same clean, non-erroring degrade
a BIOS domain (no `<nvram>`) already gets. TrueNAS's own documented,
fixed-path convention for vTPM state
(`/var/db/system/vm/tpm/{id}_{name}_tpm_state`, mirroring its NVRAM
convention at `/var/db/system/vm/nvram/{id}_{name}_VARS.fd`) exists in code
(`virshcli.TPMFixedPath`) as an explicit, documented fallback, but it is
**not called anywhere automatically**, deliberately: reconstructing it is
only ever correct once a caller has confirmed it's actually talking to
TrueNAS Scale and knows the VM's real numeric id, neither of which the parser
itself can know, and this project would rather degrade to "TPM not captured"
than silently guess a possibly-wrong path.

**Exactly how deep this is wired today (verified by reading the real code,
not assumed):** NVRAM's own *real*, production-reachable capture/restore
mechanism is entirely the SSH-based one described at the top of this
document: read over SSH into the VM's stored definition at backup time,
written back over SSH just before `virsh define` at restore time, both
best-effort and non-fatal (a failed read/write only logs a warning; the
backup/restore itself never fails because of it). That mechanism lives
entirely in the service layer (`internal/api/service.go`). One layer below
it, `internal/backup/vm_orchestrator.go` *also* carries a second,
separate, path-list-based NVRAM mechanism (`VMBackupDeps.NVRAMPath` /
`VMRestoreDeps.NVRAMPath`; the path is simply added to the same restic
backup/restore call as the disk images), but reading the real
`BackupVM`/`RestoreVM` code in `service.go` shows it **never populates
that field**; it is exercised only by `vm_orchestrator.go`'s own direct unit
tests, not by a real backup/restore today. This is a pre-existing situation,
not something introduced for TPM.

TPM support matches that same layer precisely, introducing no new gap and no
new looseness: `VMBackupDeps.TPMPath` / `VMRestoreDeps.TPMPath` are wired into
the exact same real call sites the existing `NVRAMPath` field already uses
(`runVMGraceful`, `runVMLive`, `runVMRestore` in `vm_orchestrator.go`,
included in the same restic path list, validated by the same restore
path-safety guard), proven by unit tests that assert the actual paths restic
receives, though, same as NVRAM's own path-list mechanism, this
orchestrator-level field is exercised only by that package's own tests, not
by a real backup/restore.

**The *real*, SSH-based, best-effort NVRAM mechanism now also carries TPM
bytes:** `service.go`'s existing inline SSH read/write, described at the top
of this document, reads `domain.TPMPath` into a `TPMBytes []byte` field on
the VM's stored definition alongside `NVRAMBytes`, and writes it back over
SSH just before `virsh define` at restore time, the same best-effort,
non-fatal treatment NVRAM already gets (a failed read/write only logs a
warning). Empty-safe: skipped entirely when the domain has no `<tpm>`
element (`TPMPath == ""`) or SSH capture failed, exactly like NVRAM.

**⚠ Unlike the zvol mechanism above, this is reasoned from libvirt's and
TrueNAS's public documentation and is unit-tested only**: it has never been
exercised against a real TrueNAS Scale box with a real vTPM-enabled guest.
NVRAM-only (Unraid) VM backup/restore is completely unaffected: TPM handling
is purely additive, and a domain with no `<tpm>` element parses and
backs up/restores byte-identically to before this feature existed.

### 5. TrueNAS names its libvirt domains differently, so VMs may show up under an unfamiliar name

Unraid's libvirt domain name is simply the VM's name, so the name you see in
BombVault's **Backups** list is the name you chose. TrueNAS derives the domain
name instead, and has changed how twice:

| TrueNAS version | libvirt domain name | Where the name you chose lives |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, e.g. `1_debian` | in the domain name, behind an `{id}_` prefix |
| 26 | the VM's bare **UUID**, e.g. `550e8400-e29b-41d4-a716-446655440000` | only in the domain XML's `<title>` element |

BombVault's libvirt wrapper (`internal/virshcli`) recognizes both shapes and
resolves a friendly name for each: it strips the `{id}_` prefix on 25.10, and
on 26 it makes one extra `virsh dumpxml` call per UUID-named domain to read
`<title>`, falling back to the UUID itself if there is no `<title>` or the
call fails.

**What you will actually see today:** on a host BombVault detects (or is
configured, via `PLATFORM=truenas`) as TrueNAS, the VM list **displays** the
resolved friendly name, so on TrueNAS 25.10 your VMs show as `debian`
rather than `1_debian`, and on TrueNAS 26 they show their domain XML
`<title>` rather than the bare UUID (falling back to the UUID itself if
there is no `<title>`). This is presentation-only: internally, every lookup
(matching your saved per-VM settings, every `virsh` call site) still keys on
the **raw libvirt domain name**, never the friendly name. On any other
platform, display is unchanged (the raw name is shown, as before).

This was, and remains, a **display** feature only, not a correctness one:
the raw domain name is the identifier every `virsh` command legitimately
takes, so backup and restore operate on the right VM either way. The one
practical consequence beyond cosmetics is on TrueNAS 26, where a VM's saved
BombVault settings (backup method, schedule membership) are still keyed to
the raw UUID: if TrueNAS ever re-creates the VM and issues a new UUID, the
saved settings will not follow it and the VM will reappear as a new,
unconfigured entry, even though its displayed name looks unchanged. That
matching gap is unaffected by this display feature and remains open.

A related note on how the UI wiring above stays safe: the classifier works
purely on the **shape** of the domain name and is deliberately not
platform-gated, so an ordinary Unraid VM named literally `10_Windows` is
classified exactly like a real TrueNAS 25.10 domain. `ListVMs` guards against
that by checking the detected platform explicitly (`Kind() ==
platform.KindTrueNAS`) before ever preferring the resolved friendly name over
the raw one. On Unraid the shape-classifier's output is never consulted, so
a coincidentally-TrueNAS-shaped Unraid VM name displays unchanged.

**The 25.10 scheme was seen on a real TrueNAS SCALE 25.10 box** (see the
caveat above). The TrueNAS 26 scheme is reasoned from TrueNAS's public
documentation and is unit-tested only. Unraid is unaffected:
an ordinary hand-chosen domain name matches neither shape and passes through
unchanged.

---

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Test hangs then fails | Host unreachable: re-check Step 3 (network/VLAN/firewall) and `Host SSH: Address`/`Port`. |
| `Permission denied (publickey)` despite the key being in `authorized_keys` | sshd **StrictModes** rejects the key file because of bad ownership/modes on a parent dir, common when `/root/.ssh` is symlinked to the **FAT flash** (`/boot/config/ssh/...`), which is world-writable. The host log shows `Authentication refused: bad ownership or modes for directory ...`. Fix: add `StrictModes no` to the host's sshd config **in the global section (before any `Match` block)**, then `/etc/rc.d/rc.sshd restart`. Persist it (see below), because Unraid regenerates `/etc/ssh/sshd_config` on boot. |
| `Permission denied (publickey)` (key truly missing) | Key not authorized: redo Step 2; confirm SSH is enabled. |
| `Host key verification failed` | `docker exec BombVault rm -f /config/ssh/known_hosts`, then retry. |
| `/dev/tcp/...` = UNREACHABLE | The container cannot reach the host SSH port; see Step 3 (custom-network host access, VLAN routing, or use the shim/LAN IP). |
| Variables missing in Edit | Re-import the template; an existing container keeps its old saved config. |

## Persistence across reboot

Unraid regenerates `/etc/ssh/sshd_config` on every boot, so host-side SSH tweaks
must be persisted:

- **SSH port** persists in `/boot/config/ident.cfg` (set via the Unraid GUI).
- **`StrictModes no`** (if you needed it above) does not persist; re-apply it at
  boot from `/boot/config/go`:
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
- BombVault's own SSH key + `~/.ssh/config` are written to `/config` (appdata) and
  re-created at startup, so they survive container/host restarts automatically.

## Security

The authorized key grants **root on the host**, the same trust level as the
`docker.sock` BombVault already uses. Keep BombVault on a trusted network. The
key is ed25519, stored `0600` in appdata; the host key is pinned. All host
commands are argv-separated (no remote shell) and the SSH connection uses
`BatchMode` + a connect timeout.
