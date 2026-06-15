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
2. If you changed the **SSH port** (e.g. to `1004`), note it — you'll set
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
   web UI on, e.g. `192.168.10.10`).
3. If the container's network and the host are on **different VLANs**, allow the
   route on your router/firewall: `container VLAN → host-IP : SSH-port (tcp)`.
4. If the host LAN IP is unreachable, use the host's **shim** IP on the
   container's subnet — find it on the host with
   `ip -4 addr show | grep -B2 <container-subnet>`.

### Verify reachability (before anything else)
From the Unraid terminal, replace the IP/port with yours:
```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.10.10/1004' && echo OPEN || echo UNREACHABLE
```
`OPEN` = the path works. `UNREACHABLE` = fix networking (Step 3) before going on.

## Step 4 — Set the variables & test

1. **Docker → BombVault → Edit** → set `VM Backup: Host` (+ `SSH Port` if not 22)
   → **Apply**. *(If the variables don't appear, re-import the template — Unraid
   keeps an existing container's saved config.)*
2. **Settings → VM Backup over SSH → Test connection** → green.
   Or from the terminal (the exact call BombVault makes):
   ```sh
   docker exec BombVault virsh -c "qemu+ssh://root@192.168.10.10:1004/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
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

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Test hangs then fails | Host unreachable — re-check Step 3 (network/VLAN/firewall) and `VM Backup: Host`/`Port`. |
| `Permission denied (publickey)` | Key not authorized — redo Step 2; confirm SSH is enabled. |
| `Host key verification failed` | `docker exec BombVault rm -f /config/ssh/known_hosts`, then retry. |
| `/dev/tcp/...` = UNREACHABLE | The container cannot reach the host SSH port — Step 3 (custom-network host access, VLAN routing, or use the shim/LAN IP). |
| Variables missing in Edit | Re-import the template; an existing container keeps its old saved config. |

## Security

The authorized key grants **root on the host** — the same trust level as the
`docker.sock` BombVault already uses. Keep BombVault on a trusted network. The
key is ed25519, stored `0600` in appdata; the host key is pinned. All host
commands are argv-separated (no remote shell) and the SSH connection uses
`BatchMode` + a connect timeout.
