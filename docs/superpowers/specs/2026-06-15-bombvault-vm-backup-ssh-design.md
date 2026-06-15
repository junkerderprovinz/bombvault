# BombVault VM Backup over SSH — Design

**Status:** approved (brainstorming 2026-06-15), pending spec review → writing-plans.

## Goal

Give BombVault reliable, **mount-free** VM backup & restore (graceful + live) on
Unraid by reaching libvirt over **SSH** (`qemu+ssh://`) instead of bind-mounting
any host libvirt path. The container must never again be able to break the host
VM Manager.

## Background — why SSH (the lesson)

BombVault is a **Docker container**; libvirt runs on the **host**. Every attempt
to bridge that gap with a bind mount failed and repeatedly took down the host VM
Manager:

1. socket FILE `/var/run/libvirt/libvirt-sock` → Docker phantom dir → `bind(): Address already in use`.
2. run DIR `/var/run/libvirt` → pins the dir libvirt recreates on toggle → libvirtd won't start.
3. anything under `/etc/libvirt` (NVRAM) → blocks the libvirt.img loopback mount → all VMs vanish.
4. run PARENT `/var/run` → the toggle still failed (later shown to be the box's stale-loop + `php-fpm` instability, not the mount, but trust was gone).

The de-facto reference, **JTok's `unraid.vmbackup`**, never hits this because it
is a **host plugin** (native local `virsh` + direct file access). It backs up
**vdisk + XML + NVRAM**, supports shutdown / pause / live-snapshot (qemu guest
agent recommended; snapshots require the disk on `/mnt/cache` or `/mnt/diskX`,
not `/mnt/user`).

**Key reframing:** BombVault needs libvirt **only for control commands**
(`list`, `dumpxml`, `shutdown`, `start`, `destroy`, `define`, `undefine`,
`autostart`, snapshot/blockcommit). The **vdisks are already reachable** through
the existing Host Data mount (`/mnt` → `/host/user`); restic reads/writes the
`.img` files directly. So the only thing to solve is the control channel — and
`qemu+ssh://` makes the container run `virsh` **on the host**, exactly like the
plugin, but safely from a container. As a bonus, SSH also lets BombVault read/
write the real **NVRAM** file on the host (no `/etc/libvirt` mount) for a perfect
UEFI restore.

## Architecture

```
BombVault container
 ├─ control plane:  virsh -c qemu+ssh://root@<host>/system <cmd>   (over SSH)
 ├─ disk plane:     restic reads/writes /host/user/cache/vms/...   (existing /mnt mount)
 └─ nvram plane:    ssh root@<host> "cat /etc/libvirt/qemu/nvram/<uuid>_VARS.fd"  (over SSH)
```

No libvirt bind mount. The container reaches the host's SSH over the Docker
network (`--add-host=host.docker.internal:host-gateway`).

## Components

### 1. `internal/sshconn` (new) — SSH connection + key management
- On first start, generate an **ed25519** keypair at `<DataDir>/ssh/id_ed25519`
  (+ `.pub`), perms 0600/0700. Idempotent: reuse if present.
- Holds: `Host` (default `host.docker.internal`), `User` (`root`), `KeyPath`,
  `KnownHostsPath` (`<DataDir>/ssh/known_hosts`).
- `PublicKey() (string, error)` — the authorized_keys line to show in the UI.
- `VirshURI() string` →
  `qemu+ssh://root@<host>/system?keyfile=<KeyPath>&known_hosts=<KnownHostsPath>&known_hosts_verify=normal`
  (TOFU: host key pinned on first connect via `accept-new`).
- `Run(ctx, args...) (stdout string, err error)` — raw
  `ssh -i <key> -o UserKnownHostsFile=<kh> -o StrictHostKeyChecking=accept-new
  -o BatchMode=yes root@<host> -- <args>` for NVRAM read/write + host file ops.
- `Test(ctx) error` — `virsh -c <uri> list --all` (or `ssh … true`); maps
  failures to clear hints (host unreachable / key not authorized / host-key
  changed).
- Errors scrub host paths/IPs from messages reaching the API surface.

### 2. `internal/virshcli` — switch to the SSH URI
- `Client` gains a connection URI (from `sshconn.VirshURI()`); every `run` call
  becomes `virsh -c <uri> <args…>` (still separate argv, no shell). When the URI
  is empty (VM backup not configured) the adapter reports not-configured, not a
  crash. Existing parsing (`ParseDomain`, `EnsureNVRAMTemplate`) is unchanged.

### 3. NVRAM over SSH (replaces the dropped `/etc/libvirt` mount)
- **Backup:** if `dumpxml` shows an `<nvram>` path, read its bytes via
  `sshconn.Run(ctx, "cat", "--", <nvramPath>)` and store them in the restic
  snapshot alongside the disks (written to a staging file under `/config`, or
  streamed). The exact NVRAM host path comes from the XML.
- **Restore:** write the bytes back to the host NVRAM path via
  `ssh … "cat > <nvramPath>"` (or scp) **before** `virsh define`. If the NVRAM
  wasn't captured (older backup / BIOS VM), fall back to `EnsureNVRAMTemplate`
  (regenerate from the OVMF master). UEFI boot entries are thus preserved when
  the NVRAM is present, and the VM still boots when it isn't.

### 4. Graceful backup (existing `BackupVMGraceful`, transport swap only)
recordRunStart → IsActive → Shutdown → poll `State` until "shut off"
(timeout → Destroy) → restic Backup(disks [+ nvram]) → **always Start if it was
running** → recordRunFinish. All virsh calls now go over SSH; disks via `/mnt`.

### 5. Live snapshot backup (new, VM-safety-critical)
Per-VM opt-in (the `vms.method` column: `graceful` | `live`):
1. `virsh … snapshot-create-as <vm> bombvault-tmp --disk-only --atomic
   --no-metadata` (+ `--quiesce` when the qemu guest agent is present) → the VM
   writes to a fresh overlay; the base image becomes read-only.
2. restic backs up the now-static **base** image(s) via `/mnt` (+ NVRAM via SSH).
3. `virsh … blockcommit <vm> <target> --active --pivot --wait` → merge the
   overlay back into the base and pivot the running VM onto it → delete the
   overlay file.
- **Safety guarantee (mirrors the always-start rule):** if any step fails, the
  VM is left **running and usable** (on the overlay if the commit didn't
  complete); BombVault aborts, records the run failed, and surfaces an
  actionable message with the overlay path. It NEVER leaves the VM undefined or
  unbootable. Overlay files live next to the disk on `/mnt/cache` (the snapshot
  constraint — never `/mnt/user`); this is validated and surfaced if violated.

### 6. Restore (mount-free)
guard (confirm + hex snapshot id + path containment) → if VM exists: Destroy (if
running) + Undefine → restic RestorePaths(disk parent dirs) via `/mnt` → write
NVRAM back over SSH (or template-fallback) → `EnsureNVRAMTemplate(xml)` →
`virsh … define <xml>` → Autostart → Start.

### 7. UI (Settings → VM Backup)
- Enable VM backup; **Host address** field (default `host.docker.internal`);
  the generated **public key** with a copy button + one-line instruction
  ("append to Unraid `/root/.ssh/authorized_keys`"); a **Test connection**
  button (calls `sshconn.Test`, green/red + reason).
- VMs tab: per-VM **method** selector (Graceful / Live).

## Config / template changes
- **No libvirt mount** (stays removed). VM backup is configured entirely in the
  WebUI (host + authorized key) — no opt-in mount needed.
- Template `ExtraParams` gains `--add-host=host.docker.internal:host-gateway`.
- New env (all optional, overridable): `LIBVIRT_HOST` (default
  `host.docker.internal`), `LIBVIRT_SSH_USER` (default `root`). The disk Host
  Data mount (`/mnt` → `/host/user`) already exists.
- Image: add an `ssh` client + `openssh-keygen` to the Dockerfile.

## Security
- The authorized SSH key grants **root on the host** — the **same trust level as
  the mounted docker.sock** BombVault already has. Documented in README + the
  template Overview.
- ed25519 key, 0600, stored in `/config`. Host key pinned (TOFU) in
  `/config/ssh/known_hosts`; a changed host key fails closed with a clear error.
- `BatchMode=yes` (no interactive prompts); argv-separated (no shell injection);
  snapshot id hex-validated; restore path containment unchanged.

## Error handling
- SSH unreachable / key not authorized / host-key mismatch → clear,
  non-secret messages via the Test button and on every VM operation.
- Live snapshot: defensive at every step (see §5) — the VM is never left broken.

## Testing
- Unit: `sshconn` URI build + key generation/reuse (temp dir; symlink-free);
  `virshcli` argv with the SSH URI; live-snapshot orchestrator sequence with a
  fake VM adapter proving the always-usable guarantee (commit-fails path leaves
  the VM running, run recorded failed). libvirt itself is not testable in CI →
  box-gate.
- Box-gate (real host): authorize the key → Test connection green → graceful
  backup → delete → restore → boots; then live backup of a running VM → restore
  → boots; UEFI VM keeps its boot entries (NVRAM round-tripped).

## Out of scope (separate, later)
- TLS/SASL libvirt auth (TCP) — SSH covers the secure remote case.
- SSH connection multiplexing (ControlMaster) for speed — optimization only.
- Off-site repos + retention — already on the roadmap, independent of transport.
