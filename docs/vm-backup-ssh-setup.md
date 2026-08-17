# VM Backup over SSH — Setup & Networking

BombVault backs up KVM/libvirt VMs **without mounting any libvirt path**. It runs
`virsh` **on the Unraid host over SSH** (`qemu+ssh://`), reads/writes VM **disks**
through the existing Host Data mount (`/mnt` → `/host/user`), and transfers the
UEFI **NVRAM** over SSH. Because nothing libvirt-owned is bind-mounted, BombVault
can never interfere with the Unraid VM Manager.

This guide is the exact configuration. Container backup needs none of this — it
is only for VM backup.

---

## What you configure

| Setting | Where | Default | Meaning |
|---|---|---|---|
| `VM Backup: Host` (`LIBVIRT_HOST`) | template var | `host.docker.internal` | Unraid host address reached over SSH |
| `VM Backup: SSH Port` (`LIBVIRT_SSH_PORT`) | template var | `22` | Unraid's SSH port |
| `VM Backup: SSH User` (`LIBVIRT_SSH_USER`) | template var | `root` | SSH user on the host |
| Public key | Settings → VM Backup over SSH | (auto-generated) | Authorize on the host |

The SSH keypair is generated automatically on first start at
`/config/ssh/id_ed25519` (persisted in appdata). The host key is pinned in
`/config/ssh/known_hosts` on first connect.

---

## Step 1 — Enable SSH on the Unraid host

1. **Settings → Management Access → Use SSH = Yes.**
2. If you changed the **SSH port** (e.g. to a non-default port), note it — you'll set
   `VM Backup: SSH Port` to match.

## Step 2 — Authorize BombVault's public key (persistent)

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
**host/port independent** — it applies to any `root` login.

## Step 3 — Networking (this is the part that varies)

BombVault must be able to open a TCP connection from the container to the host's
SSH port. Pick the row matching your setup:

### A. BombVault on the default `bridge` network (simplest)
- Leave `VM Backup: Host = host.docker.internal` (the template adds
  `--add-host=host.docker.internal:host-gateway`, which resolves to the docker0
  gateway = the host).
- `VM Backup: SSH Port` = your SSH port.

### B. BombVault on a custom network (`br0.x`, macvlan/ipvlan)
A container on `br0.x` cannot reach the host via `host.docker.internal`
(172.17.0.1 is docker0, unreachable from `br0.x`). Instead:
1. **Settings → Docker → Host access to custom networks = Enabled.**
2. Set `VM Backup: Host` to the **Unraid host's LAN IP** (the IP you open the
   web UI on, e.g. `192.168.x.x`).
3. If the container's network and the host are on **different VLANs**, allow the
   route on your router/firewall: `container VLAN → host-IP : SSH-port (tcp)`.
4. If the host LAN IP is unreachable, use the host's **shim** IP on the
   container's subnet — find it on the host with
   `ip -4 addr show | grep -B2 <container-subnet>`.

### Verify reachability (before anything else)
From the Unraid terminal, replace the IP/port with yours:
```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.x.x/<port>' && echo OPEN || echo UNREACHABLE
```
`OPEN` = the path works. `UNREACHABLE` = fix networking (Step 3) before going on.

## Step 4 — Set the variables & test

1. **Docker → BombVault → Edit** → set `VM Backup: Host` (+ `SSH Port` if not 22)
   → **Apply**. *(If the variables don't appear, re-import the template — Unraid
   keeps an existing container's saved config.)*
2. **Settings → VM Backup over SSH → Test connection** → green.
   Or from the terminal (the exact call BombVault makes):
   ```sh
   docker exec BombVault virsh -c "qemu+ssh://root@192.168.x.x:<port>/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
   ```
   → lists your VMs.

## Step 5 — Per-VM method

In the **VMs** tab each VM has a method:
- **Graceful (shutdown)** — default; shuts the VM down, backs up the disks,
  restarts it. Always consistent.
- **Live snapshot** — backs up a running VM via an external snapshot +
  `blockcommit`. Requirements: the **qemu guest agent** installed in the VM
  (for a quiesced, app-consistent snapshot) and the disk on **`/mnt/cache`** or
  `/mnt/diskX` (not `/mnt/user`). On a shut-off VM, live automatically falls back
  to graceful.

---

## TrueNAS Scale

The steps above (SSH enable, key authorization, networking, per-VM method)
are written for Unraid, but the same `qemu+ssh://` mechanism works against
any reachable libvirtd — including TrueNAS Scale, which has shipped libvirt-
based VMs again since 25.04.2. Two things are different on TrueNAS and need
an explicit override.

### 1. TrueNAS's libvirtd socket is non-standard

TrueNAS Scale's libvirtd does not listen where the default `qemu+ssh://`
connection string built from `VM Backup: Host`/`User`/`Port` expects. Set
`LIBVIRT_URI` directly instead, with the extra `?socket=...` query parameter
the built-string form has no way to express:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

When `LIBVIRT_URI` is set, BombVault uses it **verbatim** — `LIBVIRT_HOST`,
`LIBVIRT_SSH_USER`, and `LIBVIRT_SSH_PORT` are ignored for the connection
string itself (they still don't need to be set at all for VM backup to work,
since the URI already carries the user/host).

### 2. Root SSH is disabled by default since TrueNAS 24.10

Step 2 above (authorizing BombVault's public key for `root`) requires root
SSH login, which TrueNAS Scale turns off out of the box starting with 24.10.
Either:

- **System Settings → General → Login As Root with Password** — enable it,
  then authorize BombVault's public key the same way Step 2 describes, using
  `<user>` = `root` in the `LIBVIRT_URI` above; or
- create a sudo-capable admin account (with an API key, per TrueNAS's own
  account setup) and authorize BombVault's key for that account instead,
  using its username in place of `root` in `LIBVIRT_URI`.

### Caveat: direct `virsh` changes are invisible to TrueNAS's middleware

TrueNAS's own middleware (the layer behind its VM UI) is the intended control
path for its VMs. A `virsh` command run directly over this SSH link — which
is exactly what BombVault's VM backup/restore does — bypasses that
middleware, and TrueNAS may reconcile or overwrite state it didn't originate
(sourced from TrueNAS's publicly documented middleware behavior, not
independently verified against a running instance). This is the same risk
class as Unraid's own VM Manager reconciling changes made outside it, only
sharper on TrueNAS since its middleware is more actively involved in VM
lifecycle management.

**This whole section is reasoned from TrueNAS Scale's public documentation
and the shape of its libvirt setup — there is no TrueNAS Scale test instance
available to verify it against real hardware.** Treat it as a documented
starting point, not a confirmed-working configuration, until it has been
exercised on an actual TrueNAS Scale box.

### 3. VM disks backed by zvols (raw block devices)

TrueNAS Scale VM disks are commonly **zvols** — ZFS-backed block devices
(`/dev/zvol/<pool>/<dataset>`), not qcow2 files. BombVault's existing VM disk
backup (the mechanism described above and throughout this document) assumes
file-based disk images that restic can back up directly by path; a zvol needs
a fundamentally different mechanism, since restic cannot back up a raw block
device the way it backs up a file.

BombVault detects a block-device-backed disk from the domain XML itself
(libvirt's own `<source dev="...">` vs `<source file="...">` distinction —
no guessing), and the backup/restore orchestrators (`BackupVMGraceful`,
`BackupVMLive`, `RestoreVM`) ARE wired to invoke, for such a disk, ZFS's own
tools instead of a plain file backup:

1. `zfs snapshot <dataset>@<name>` — a point-in-time consistency point, taken
   over the same SSH connection as every other command on this page (no local
   ZFS/zvol access inside the BombVault container).
2. `zfs send <dataset>@<name>`, streamed over that SSH connection straight
   into the restic backup (no local staging file, however large the zvol).
3. `zfs destroy <dataset>@<name>` — always run afterward, success or failure;
   the snapshot is a consistency point, not the backup artifact.

Restoring a zvol is a **separate, defensive path** from restoring a
file-based disk: `zfs receive` into an EXISTING dataset can destroy live
data, so a restore always lands on a **freshly-named dataset**
(`<pool>/<dataset>-bombvault-restore-<timestamp>`) — **never** the original.
Promoting that fresh dataset over the live original (so the VM actually boots
from the restored data) is a **deliberate, manual, documented follow-up step
for the operator** — BombVault never automates renaming/overwriting a live
zvol. After a restore completes, decide independently (per your own ZFS
layout and running state of the original VM) whether/how to `zfs rename` the
restored dataset into place, generally after shutting the VM down and
pointing its domain XML at the new dataset (or renaming the restored dataset
to the original's name once the original has been renamed aside).

**⚠ This entire mechanism is REASONED from ZFS's and TrueNAS's public
documentation — the `/dev/zvol/<pool>/<dataset>` device-node convention and
`zfs send`/`zfs receive` as the ZFS-native way to move a point-in-time
dataset byte stream — and is UNIT-TESTED ONLY (argv construction, domain-XML
detection, the snapshot/stream/cleanup control flow, AND the wiring into the
real `BackupVMGraceful`/`BackupVMLive`/`RestoreVM` orchestrators, all
exercised with fakes).** It has **never been exercised against a real
TrueNAS Scale box with a real zvol-backed VM** — no test hardware was
available anywhere in this project's development environment. Treat it as a
documented, unit-tested starting point, not a confirmed-working backup path,
until it has had a real backup → restore-to-a-fresh-dataset → boot-check pass
on actual TrueNAS Scale hardware. File-backed (Unraid) VM disk backup/restore
is completely unaffected by this mechanism — it is a wholly separate code
path.

**Remaining gap beyond hardware verification:** the orchestrator-level wiring
above is real and tested, but BombVault's service layer does not yet call
into it from a live backup/restore — it never reads a domain's block-device
disks or constructs the ZFS-over-SSH adapter this mechanism needs. Closing
that also requires a design decision this project hasn't made yet: because a
VM with both file-backed and block-device disks always produces *multiple*
separate restic snapshots (one per block disk, since restic's stdin-backup
mode can't be combined with a normal file-path backup in one run), and
BombVault's run history and restore-by-"latest" today assume one snapshot per
VM backup, there needs to be an explicit way to track and re-find each block
disk's own snapshot before a restore can rely on it. Until that is decided
and the service layer is wired up, this mechanism cannot be reached from the
BombVault UI at all — track this as a follow-up alongside the hardware
verification pass above.

### 4. vTPM state (Secure-Boot/Windows-11-class guests)

A TrueNAS Scale VM can also have a **vTPM** (virtual TPM) device — required
for Secure Boot and for Windows 11 guests. Its state lives **outside** the
domain's disk and NVRAM, so a backup that only captures those two would leave
a restored Secure-Boot/vTPM guest unable to boot correctly (a fresh, empty
TPM identity, not the guest's real one).

BombVault parses a domain's `<tpm>` element (`internal/virshcli`) the same
way it already parses `<os><nvram>`. libvirt's schema supports several `<tpm>`
backend types, but only the **passthrough** shape
(`<backend type='passthrough'><device path='...'/></backend>`) is documented
to carry an explicit path directly in the domain XML — and that shape is a
real hardware TPM chip, not the kind of vTPM TrueNAS actually provisions. The
**emulated** (software) vTPM TrueNAS uses for Secure Boot/Windows 11 guests
does not, per libvirt's public documentation, expose its swtpm state
directory as a domain-XML attribute; a newer `external`-backend shape can
carry a UNIX socket path and is plausibly how TrueNAS's middleware wires its
vTPM, but its exact attribute layout as rendered by TrueNAS is **not
confirmed against real hardware**. Rather than guess at either shape,
BombVault's parser recognizes only the well-documented passthrough case and
otherwise reports "no TPM path found" — the same clean, non-erroring degrade
a BIOS domain (no `<nvram>`) already gets. TrueNAS's own documented,
fixed-path convention for vTPM state
(`/var/db/system/vm/tpm/{id}_{name}_tpm_state`, mirroring its NVRAM
convention at `/var/db/system/vm/nvram/{id}_{name}_VARS.fd`) exists in code
(`virshcli.TPMFixedPath`) as an explicit, documented fallback — but it is
**not called anywhere automatically**, deliberately: reconstructing it is
only ever correct once a caller has confirmed it's actually talking to
TrueNAS Scale and knows the VM's real numeric id, neither of which the parser
itself can know, and this project would rather degrade to "TPM not captured"
than silently guess a possibly-wrong path.

**Exactly how deep this is wired today (verified by reading the real code,
not assumed):** NVRAM's own *real*, production-reachable capture/restore
mechanism is entirely the SSH-based one described at the top of this
document — read over SSH into the VM's stored definition at backup time,
written back over SSH just before `virsh define` at restore time, both
best-effort and non-fatal (a failed read/write only logs a warning; the
backup/restore itself never fails because of it). That mechanism lives
entirely in `internal/api/service.go`, which is **outside this task's file
scope** (`internal/virshcli` + `internal/backup/vm_orchestrator.go`). Inside
that scope, `internal/backup/vm_orchestrator.go` *also* carries a second,
separate, path-list-based NVRAM mechanism (`VMBackupDeps.NVRAMPath` /
`VMRestoreDeps.NVRAMPath` — the path is simply added to the same restic
backup/restore call as the disk images) — but reading the real
`BackupVM`/`RestoreVM` code in `service.go` shows it **never populates
that field**; it is exercised only by `vm_orchestrator.go`'s own direct unit
tests, not by a real backup/restore today. This is a pre-existing situation,
not something introduced for TPM.

TPM support matches that same layer precisely, introducing no new gap and no
new looseness: `VMBackupDeps.TPMPath` / `VMRestoreDeps.TPMPath` are wired into
the exact same real call sites the existing `NVRAMPath` field already uses
(`runVMGraceful`, `runVMLive`, `runVMRestore` in `vm_orchestrator.go` —
included in the same restic path list, validated by the same restore
path-safety guard), proven by unit tests that assert the actual paths restic
receives. Extending the *real*, SSH-based, best-effort NVRAM mechanism to
also carry TPM bytes is a `service.go` change and is **not done by this
task** — the restore-side write-back hook it would plug into
(`VMRestoreDeps.PreDefine`, a generic caller-supplied closure) needs no
change to support this once that integration happens.

**⚠ Like the zvol mechanism above, this is REASONED from libvirt's and
TrueNAS's public documentation and is UNIT-TESTED ONLY** — it has never been
exercised against a real TrueNAS Scale box with a real vTPM-enabled guest.
NVRAM-only (Unraid) VM backup/restore is completely unaffected: TPM handling
is purely additive, and a domain with no `<tpm>` element parses and
backs up/restores byte-identically to before this feature existed.

---

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Test hangs then fails | Host unreachable — re-check Step 3 (network/VLAN/firewall) and `VM Backup: Host`/`Port`. |
| `Permission denied (publickey)` despite the key being in `authorized_keys` | sshd **StrictModes** rejects the key file because of bad ownership/modes on a parent dir — common when `/root/.ssh` is symlinked to the **FAT flash** (`/boot/config/ssh/...`), which is world-writable. The host log shows `Authentication refused: bad ownership or modes for directory ...`. Fix: add `StrictModes no` to the host's sshd config **in the global section (before any `Match` block)**, then `/etc/rc.d/rc.sshd restart`. Persist it (see below) — Unraid regenerates `/etc/ssh/sshd_config` on boot. |
| `Permission denied (publickey)` (key truly missing) | Key not authorized — redo Step 2; confirm SSH is enabled. |
| `Host key verification failed` | `docker exec BombVault rm -f /config/ssh/known_hosts`, then retry. |
| `/dev/tcp/...` = UNREACHABLE | The container cannot reach the host SSH port — Step 3 (custom-network host access, VLAN routing, or use the shim/LAN IP). |
| Variables missing in Edit | Re-import the template; an existing container keeps its old saved config. |

## Persistence across reboot

Unraid regenerates `/etc/ssh/sshd_config` on every boot, so host-side SSH tweaks
must be persisted:

- **SSH port** persists in `/boot/config/ident.cfg` (set via the Unraid GUI).
- **`StrictModes no`** (if you needed it above) does NOT persist — re-apply it at
  boot from `/boot/config/go`:
  ```sh
  # /boot/config/go — keep SSH key auth working with flash-based authorized_keys
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

The authorized key grants **root on the host** — the same trust level as the
`docker.sock` BombVault already uses. Keep BombVault on a trusted network. The
key is ed25519, stored `0600` in appdata; the host key is pinned. All host
commands are argv-separated (no remote shell) and the SSH connection uses
`BatchMode` + a connect timeout.
