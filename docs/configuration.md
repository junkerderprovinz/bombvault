# Configuration

This page covers the container's environment variables, the mounts the template provides, VM backup over SSH, and the off-site setup. Backup **repository paths** are configured inside the app (Settings, Backup paths), not via environment variables.

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `APP_KEY` | **Yes** | 32-byte hex secret (64 hex chars) used to derive the restic repo password. Generate with `openssl rand -hex 32`. Keep this safe: losing it makes encrypted backups unrecoverable. |
| `LIBVIRT_HOST` | For VMs | Unraid host reached over SSH for VM backup (default `host.docker.internal`; the template pre-fills a LAN-IP placeholder). Use your Unraid LAN IP, required on a custom `br0.x` network. |
| `LIBVIRT_SSH_PORT` | No | Host SSH port for VM backup (default `22`). |
| `LIBVIRT_SSH_USER` | No | SSH user on the host for VM backup (default `root`). |
| `LIBVIRT_URI` | No | Full libvirt connection URI, used **verbatim** instead of building one from the three `LIBVIRT_*` variables above (which are then ignored for the connection string). Default unset. Needed on TrueNAS Scale, whose libvirtd listens on a non-standard socket the built-string form cannot express: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. See [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'s TrueNAS Scale section. |
| `PORT` | No | HTTP port (default `3000`; only used with `HTTP_ONLY=true`). |
| `HTTPS_PORT` | No | HTTPS port (default `3443`; the template publishes it 1:1, so the WebUI answers on `https://<ip>:3443`). |
| `HTTP_ONLY` | No | Set `true` to disable the self-signed HTTPS listener and serve plain HTTP only (for use behind a TLS-terminating reverse proxy). |
| `HOST_SOURCE_ROOT` | No | The host path mounted as **Host Data** (default `/mnt`). BombVault translates the bind-mount sources Docker reports into paths under this mount. Change only if you mounted a different host root. |
| `DATA_ROOT_SEGMENTS` | No | Comma-separated path-segment names that mark a bind-mount source as backup data (default `appdata`, matching Unraid's `/mnt/user/appdata/<container>` convention). A container's bind mount is auto-selected for backup when ANY listed segment appears as a full path segment of its host source — for example `DATA_ROOT_SEGMENTS=appdata,config` also picks up a `.../config` bind. See [Backup source detection](#backup-source-detection) for the other, always-on ways a container's data folder is found. |
| `PLATFORM` | No | Forces which platform BombVault treats itself as running on, instead of auto-detecting: `unraid`, `generic`, or `truenas` (default unset — auto-detects Unraid by probing for its `dockerMan` marker under the flash mount, otherwise `generic`; an unrecognized value also falls back to `generic`, logged). Set it explicitly on a generic Docker host or TrueNAS Scale rather than relying on the Unraid-only auto-probe — the generic compose file does this. Changes the appdata-fallback convention, the cross-instance restore-destination defaults, and whether the Unraid-only notification/companion-plugin steps are attempted at all (see `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | No | The name of the BombVault container itself, so it never backs up (and thus stops) itself (default `BombVault`; auto-detected via the hostname on bridge networking). |
| `BACKUP_MAX_HOURS` | No | Maximum wall-clock hours a single backup run may hold its domain lock before it is force-cancelled (a guard so a wedged run cannot block the domain forever). Empty (the default) uses `48`. Raise it for very large or slow cloud backups (a run cancelled at the cap fails with `context deadline exceeded`). Set `0` to disable the cap entirely. |
| `TZ` | No | Timezone for the scheduler (for example `Europe/Berlin`). **Leave it unset and every schedule runs in UTC**: one set to 02:30 then fires at 02:30 UTC rather than on your own wall clock. |

## Mounts

Mount the Docker socket, the flash (`/boot`) and the **Host Data** root (`/mnt`) as shown in the CA template. Backup *sources* and *destinations* both live under Host Data, and it is mounted **rslave** so a remote share that mounts after the container starts (for example under `/mnt/remotes`) becomes visible without a restart.

Backup repository paths default to `/mnt/user/bombvault/{container,vms,flash,config,files}`, created on the first backup. Change the location any time in **Settings, Backup paths**. Each path field also has an inline **Local / Remote** switch — a path can be a restic remote (`s3:...`, `rest:...`, `b2:...`, `sftp:...`, `rclone:...`) instead of a local folder, backing up straight to it with no separate local copy; see [Remote primary repositories](offsite-recovery.md#remote-primary-repositories).

!!! note "Host integration check"
    Open `/spike` in the web UI after the container starts. It probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces.

## Backup source detection {#backup-source-detection}

For each container, BombVault auto-selects which bind mounts and named volumes to back up. A path is picked up when any of the following applies (you can always override the result per container in the container's **Backup paths**):

- **Data-root segment match:** the bind's host source contains one of the `DATA_ROOT_SEGMENTS` segments as a full path component (default `appdata` only).
- **Docker named volumes** are always included — they have no throwaway equivalent, so there is nothing to filter — **but only when the volume's real host storage path is itself reachable through the Host Data mount**, exactly like any other host path BombVault backs up. Docker's default local-volume driver stores a volume under the daemon's own data root — `/var/lib/docker/volumes/<name>/_data` unless you've customized it (check with `docker info -f '{{.DockerRootDir}}'`) — which is NOT covered by the narrow, single-directory Host Data mount the generic `docker-compose.yml` uses by default. An unreachable volume is silently skipped, not an error. To actually back up named volumes on a generic host, point Host Data (and `HOST_SOURCE_ROOT`) at a common ancestor that also covers the Docker data root — see the compose file's Host Data comment for the tradeoff (Unraid sidesteps this by mounting all of `/mnt`, its own universal top-level convention, for the same reason).
- **Docker Compose project directory:** if the container carries the standard `com.docker.compose.project.working_dir` label (set automatically by `docker compose up`), that directory is added too, regardless of whether any bind matched a data-root segment.
- **`bombvault.data` label override:** set the label `bombvault.data=true` on a container to include ALL of its bind mounts, for a layout neither convention above catches (for example a single `/srv/plex/config` bind with no Compose project). Any non-empty value other than `false` counts as truthy; an absent label or `bombvault.data=false` changes nothing.

## Security model

!!! warning "Root-equivalent control of the host"
    Through the Docker socket BombVault can stop, remove and recreate containers and read/write appdata, and for VM backup it logs in to the host over SSH (`qemu+ssh://`, root by default) to run `virsh`. Anyone who can reach its web UI effectively has root on the host.

- **Optional password protection** (Settings, Security): set a password to require login, clear it to disable. Off by default for trusted-LAN use. Sessions are signed (HMAC derived from `APP_KEY`) and changing the password invalidates them; logins are rate-limited.
- Because the gate is opt-in, when unset the whole UI and API (including the off-site setup, tamper-test routes and the recovery kit) are reachable by anyone who can reach the port. Enable the gate once off-site, immutable backups or encryption are in use.
- Run BombVault only on a trusted, non-exposed network. For remote access put it behind a reverse proxy that adds authentication and TLS. Responses carry baseline security headers (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- With `HTTP_ONLY=true` the session cookie loses its `Secure` flag (it has to, to work over plain HTTP), so only enable the password behind a TLS-terminating proxy if confidentiality matters.
- The VM-backup SSH connection trusts the host key on first connect (TOFU) and pins it thereafter. Verify the host's key out-of-band if your container-to-host path is not trusted.
- Backups are encrypted by restic when encryption is enabled (Settings; on by default), with the key derived from `APP_KEY`.

## VM backup over SSH

BombVault backs up KVM/libvirt VMs **without mounting any libvirt path**. It runs `virsh` on the host over SSH (`qemu+ssh://`), so it can never affect your host VM Manager.

Quick setup:

1. **Settings, System, VM Backup over SSH:** copy the shown public key.
2. Append it to Unraid's `/root/.ssh/authorized_keys` (also persisted to the flash so it survives reboots).
3. Click **Test connection**.

The template adds `--add-host=host.docker.internal:host-gateway` so the container can reach the host. Set `LIBVIRT_HOST` to your Unraid LAN IP if that name does not resolve (for example when the container runs on a custom `br0.x` network). If you changed Unraid's SSH port, set `LIBVIRT_SSH_PORT` to match. **Live snapshots** additionally need the qemu guest agent in the VM and the disk on `/mnt/cache` (not `/mnt/user`).

!!! important "Full VM setup and networking guide"
    The complete step-by-step guide (SSH enablement, persistent key authorization, custom-network and VLAN routing, per-VM method and host-side troubleshooting) lives at [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) on GitHub.

## Off-site setup

Set up an off-site replica on the **Settings, Off-site** tab. See [Off-site & recovery](offsite-recovery.md) for the full workflow (immutable/append-only, tamper testing and DR drills). In short:

- **Backends:** SMB/CIFS and NFS (mount the share and point a Backup Path at it), native restic backends without rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), or any rclone remote (`rclone:<remote>:<bucket>/path`).
- **Cloud credentials** are stored encrypted under Settings, Off-site, Cloud credentials.
- **SSH targets need nothing installed on the far side.** `sftp:` only needs an SSH server. Add the public key from **Settings, System, VM Backup over SSH** (also at `/config/ssh/id_ed25519.pub`) to the target user's `~/.ssh/authorized_keys`.
- **Off-site copy:** BombVault replicates new snapshots with `restic copy` on a best-effort basis, on top of a (usually local) primary. Each domain has its own off-site schedule, plus a **Replicate now** button.
- **Multiple off-site targets per domain:** each domain can replicate to several off-site destinations at once. Add extra targets on Settings, Off-site, each with its own repository, S3 storage class, append-only flag, retention and growth budget; they all replicate on that domain's off-site schedule. An existing single off-site setup is carried over as the first target.
- **Retention per source:** the local policy lives on Settings, Paths & Storage; the off-site policy on Settings, Off-site (leave it all-zero to never auto-trim off-site snapshots).
- **Bandwidth limits:** cap the restic upload/download rate under Settings, Off-site.
- **Cold and archival storage class (S3):** for a native S3 off-site repo, pick a restore-readable tier (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone remotes set their class in the rclone config.
- **Remote primary instead of local:** a domain's Backup Path itself can be one of the backends above, with no local copy and no replication step — see [Remote primary repositories](offsite-recovery.md#remote-primary-repositories) for the inline Local/Remote switch and its bandwidth/append-only/growth-budget safety settings.

## Portable settings (export and import) {#portable-settings-export-and-import}

The **Export and import settings** card on the Settings page writes your whole BombVault configuration (domain settings, off-site targets, schedules, retention, notifications) to a portable JSON file you can import on another instance, so moving to a new box or cloning a setup does not mean re-entering everything by hand. Import shows a preview and asks for confirmation, and it never touches your backup data or history.

!!! warning "The export can contain credentials"
    You choose whether to include the off-site and notification credentials in the file. With credentials included, the export is as sensitive as your recovery kit, so store it somewhere safe. Without them, the file holds only non-secret settings.
