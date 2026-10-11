# Configuration

This page covers the container's environment variables, the mounts the template provides, the security model, VM backup over SSH, and the off-site setup. Where backups go is set up inside the app as **storage locations**, not via environment variables.

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `APP_KEY` | **Yes** | 32-byte hex secret (64 hex chars) used to derive the restic repo password. Generate with `openssl rand -hex 32`. Keep this safe: losing it makes encrypted backups unrecoverable. |
| `LIBVIRT_HOST` | For VMs and ZFS datasets | Unraid host reached over SSH for VM and ZFS dataset backups (template field **Host SSH: Address**, default `host.docker.internal`). The template pre-fills the placeholder `192.168.x.x`, which counts as unset. Use your Unraid LAN IP, required on a custom `br0.x` network. |
| `LIBVIRT_SSH_PORT` | No | Host SSH port for VM and ZFS dataset backups (template field **Host SSH: Port**, default `22`). |
| `LIBVIRT_SSH_USER` | No | SSH user on the host for VM and ZFS dataset backups (template field **Host SSH: User**, default `root`). |
| `LIBVIRT_URI` | No | Full libvirt connection URI, used **verbatim** instead of building one from the three `LIBVIRT_*` variables above (which are then ignored for the connection string). Default unset. Needed on TrueNAS Scale, whose libvirtd listens on a non-standard socket the built-string form cannot express: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. See [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)'s TrueNAS Scale section. When it is a `qemu+ssh://` URI, each of `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` and `LIBVIRT_SSH_PORT` that is not set is taken from it, also for BombVault's own SSH commands (NVRAM transfer, ZFS datasets). |
| `PORT` | No | HTTP port (default `3000`; only used with `HTTP_ONLY=true`). |
| `HTTPS_PORT` | No | HTTPS port (default `3443`; the template publishes it 1:1, so the WebUI answers on `https://<ip>:3443`). |
| `HTTP_ONLY` | No | Set `true` to disable the self-signed HTTPS listener and serve plain HTTP only (for use behind a TLS-terminating reverse proxy). |
| `BIND_HOST` | No | Address the WebUI listens on (default `0.0.0.0`, every interface). Leave it unset in the container, whose published ports need every interface; `127.0.0.1` suits a run outside Docker. The healthcheck asks the same address. |
| `TRUSTED_PROXY` | No | Comma-separated addresses or CIDR ranges of the reverse proxy in front of BombVault (for example `192.168.20.11` or `10.0.0.0/8`). Only from those hops is `X-Forwarded-For` believed, and the login throttle then counts failures per real client instead of putting everyone behind the proxy in one bucket. Unset (the default) trusts nobody: a header believed unconditionally would let any caller choose their own bucket. |
| `HOST_SOURCE_ROOT` | No | The host path mounted as **Host Data** (default `/mnt`). BombVault translates the bind-mount sources Docker reports into paths under this mount. Change only if you mounted a different host root. |
| `DATA_ROOT_SEGMENTS` | No | Comma-separated path-segment names that mark a bind-mount source as backup data (default `appdata`, matching Unraid's `/mnt/user/appdata/<container>` convention). A container's bind mount is auto-selected for backup when ANY listed segment appears as a full path segment of its host source; for example `DATA_ROOT_SEGMENTS=appdata,config` also picks up a `.../config` bind. See [Backup source detection](#backup-source-detection) for the other, always-on ways a container's data folder is found. |
| `PLATFORM` | No | Forces which platform BombVault treats itself as running on, instead of auto-detecting: `unraid`, `generic`, or `truenas` (default unset: auto-detects Unraid by probing for its `dockerMan` marker under the flash mount, otherwise `generic`; an unrecognized value also falls back to `generic`, logged). Set it explicitly on a generic Docker host or TrueNAS Scale rather than relying on the Unraid-only auto-probe; the generic compose file does this. Changes the appdata-fallback convention, the cross-instance restore-destination defaults, and whether the Unraid-only notification/companion-plugin steps are attempted at all (see `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | No | The name of the BombVault container itself, so it never backs up (and thus stops) itself. |
| `BACKUP_MAX_HOURS` | No | Maximum wall-clock hours a single backup run may hold its domain lock before it is force-cancelled (a guard so a wedged run cannot block the domain forever). Empty (the default) uses `48`. Raise it for very large or slow cloud backups (a run cancelled at the cap fails with `context deadline exceeded`). Set `0` to disable the cap entirely. |
| `BACKUP_STALL_HOURS` | No | Hours a backup may make **no progress at all** before it is cancelled. Empty (the default) uses `2`; set `0` to never cancel on a stall. This is the finer of the two guards and usually the one that fires: it watches whether anything is still happening rather than how long the run has taken, so a slow but healthy multi-terabyte backup is left alone while one wedged on an unresponsive share is stopped in hours instead of days. A warning is logged after 30 minutes of silence, before anything is cancelled. Scanning counts as progress: restic writes no bytes while it walks a large tree, and that phase is watched through its file and byte totals rather than through the bytes written. The two variables are independent, and `BACKUP_MAX_HOURS` still bounds the phases after the backup itself (retention, statistics, off-site copy), where there are no counters to watch. |
| `DB_DUMP_MAX_HOURS` | No | Hours one automatic database dump may run before it is stopped. Empty (the default) uses `6`; allowed values are `1` to `48`, and the limit is kept an hour below `BACKUP_MAX_HOURS` (at half of it when that is under two hours), so a long dump is cut by its own limit and reported as such rather than taking the backup down with it. A dump that makes no progress is stopped earlier, after `BACKUP_STALL_HOURS`. A stopped dump fails on its own and the container backup carries on. On Unraid, add it to the BombVault container with **Add another Path, Port, Variable**. |
| `TZ` | No | Timezone for the scheduler (for example `Europe/Berlin`). **Leave it unset and every schedule runs in UTC**: one set to 02:30 then fires at 02:30 UTC rather than on your own wall clock. On Unraid you never set this yourself: the system passes its own timezone into every container. The boot log says which zone it resolved to. A zone with daylight saving skips one run in spring and runs one twice in autumn, while UTC does neither but drifts an hour against your clock twice a year. |

## Mounts

Mount the Docker socket, the flash (`/boot`) and the **Host Data** root (`/mnt`) as shown in the CA template. Backup *sources* and *destinations* both live under Host Data, and it is mounted **slave** so a remote share that mounts after the container starts (for example under `/mnt/remotes`) becomes visible without a restart.

ZFS dataset backups need this mode too: the host mounts a dataset's snapshot only after the container has started. See [ZFS datasets](zfs-datasets.md).

Local repositories default to `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, created on the first backup. They form the **Local folder** storage location. On its page under **Storage locations**, the **Connection** card changes the folder of each section, and the window it opens has a **Local / Remote** switch: a section's path can be a restic remote (`s3:...`, `rest:...`, `sftp:...`, `rclone:...`) instead of a local folder, backing up straight to it with no separate local copy; see [Remote primary repositories](offsite-recovery.md#remote-primary-repositories).

!!! note "Host integration check"
    The **Host Integration** card on the overview (add it under **Customize**) probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces.

## Backup source detection {#backup-source-detection}

For each container, BombVault auto-selects which bind mounts and named volumes to back up. A path is picked up when any of the following applies (you can always override the result in the **Backup folders** card on the container's Settings tab):

- **Data-root segment match:** the bind's host source contains one of the `DATA_ROOT_SEGMENTS` segments as a full path component (default `appdata` only).
- **Docker named volumes** are always included, since they have no throwaway equivalent and there is nothing to filter, **but only when the volume's real host storage path is itself reachable through the Host Data mount**, exactly like any other host path BombVault backs up. Docker's default local-volume driver stores a volume under the daemon's own data root (`/var/lib/docker/volumes/<name>/_data` unless you've customized it; check with `docker info -f '{{.DockerRootDir}}'`), which is not covered by the narrow, single-directory Host Data mount the generic `docker-compose.yml` uses by default. An unreachable volume is silently skipped, not an error. To actually back up named volumes on a generic host, point Host Data (and `HOST_SOURCE_ROOT`) at a common ancestor that also covers the Docker data root. The compose file's Host Data comment explains the tradeoff (Unraid sidesteps this by mounting all of `/mnt`, its own universal top-level convention, for the same reason).
- **Docker Compose project directory:** if the container carries the standard `com.docker.compose.project.working_dir` label (set automatically by `docker compose up`), that directory is added too, regardless of whether any bind matched a data-root segment.
- **`bombvault.data` label override:** set the label `bombvault.data=true` on a container to include all of its bind mounts, for a layout neither convention above catches (for example a single `/srv/plex/config` bind with no Compose project). Any non-empty value other than `false` counts as truthy; an absent label or `bombvault.data=false` changes nothing.
- **`bombvault.dbdump` label:** set `bombvault.dbdump=false` on a container to switch its automatic database dump off (`0`, `no` and `off` do the same), or name the engine (`postgres`, `mysql`, `mariadb`) to dump a container BombVault does not recognise by itself. The label wins over the switch in the **Database** card on the container's Settings tab, which is the way to do it on Unraid.

## Security model {#security-model}

!!! warning "Root-equivalent control of the host"
    Through the Docker socket BombVault can stop, remove and recreate containers and read/write appdata, and for VM backup it logs in to the host over SSH (`qemu+ssh://`, root by default) to run `virsh`. Anyone who can reach its web UI effectively has root on the host.

### Login {#login}

- **Optional password protection** (**Password** card under **Settings, Security**): set a password to require login, clear it to disable. Off by default for trusted-LAN use. The password is stored with Argon2id over an `APP_KEY`-keyed pepper, so a copied `/config` is worthless without the key and slow to attack with it. A new password needs at least 12 characters; an existing shorter one keeps working until it is changed. Sessions are signed (HMAC derived from `APP_KEY`) and changing the password invalidates them; logins are limited to five failures a minute per client.
- **Some things need a password.** Downloading the recovery kit and the bug report, depositing the key with a key keeper, allowing a deposit to be handed out, making a new recovery code, deleting the backups at a storage location, exporting settings with credentials, two-factor authentication and passkeys are refused until a login password is set, and they need a sign-in at the instance itself.
- **Two-factor authentication** (**Settings, Security**): a time code from an authenticator app on top of the password, plus eight single-use recovery codes handed out once when it is switched on. The shared secret is stored encrypted with `APP_KEY`, and turning the factor off again requires a current code.
- **Passkeys** (WebAuthn) sit on their own card once a password is set, alongside the password and never instead of it, so removing every passkey locks nobody out. They need a real domain name and a certificate the browser trusts. The default `https://<ip>:3443` is exactly what WebAuthn refuses, and the card says so instead of offering a button that fails.
- **A forgotten password** is reset on the login page. **Forgot password?** has BombVault write a code of 12 characters, in three groups of four, to its container log (on Unraid: Docker tab, BombVault icon, **Logs**). That code and a new password sign you in. The reset also switches two-factor authentication off, removes every passkey, revokes every API token, MCP key and OAuth sign-in, ends every other session, signs every paired phone out and switches phone pairing off; the page afterwards lists what went. A code is valid for 10 minutes, and wrong guesses do not use it up. Widget and metrics tokens stay, since they only read.
- **Whoever can read the container log can reset the password.** That is root on the host, but also a log viewer such as Dozzle, any container given the Docker socket (read-only is enough) and a log shipper that forwards container output. Give those only to people you would trust with the password. Somebody who only reaches the web interface never sees the code.

### Phones and group members {#phones-and-members}

- **Paired phones reach an allow-list.** A phone that knows the group's twelve words signs in without the password, as long as **Phones can pair with the group's twelve words** is on in the **Android app** card under **Settings, Security**. Its session reaches only what the app shows: the overview, the runs, the backups to look at, start and restore, the anomalies to read and the Instances page to look at. Storage locations and commands reach it with their addresses and secrets removed, and a restore that replaces the original backs up the current state first. Everything else answers that it needs a sign-in at the instance itself: keys, the recovery kit, key deposits, settings, deleting backups. A password reset switches phone pairing off until you switch it on again.
- **Members sign what they ask for.** Role requests and their answers, the handing out of a restic password to a receiver or fetcher, receiver logins and key deposits are signed with a key of each member, which an instance pins the first time it sees that member. A call signed with another key is refused. **Forget key** in an instance's details drops the pinned key, for the case where the one known here is not the member's own; the member is asked for its key again at its next signed call. Members on BombVault 9.x do not sign and can be impersonated by anyone with the words, as in 9.9.
- **The twelve words are a password.** Anyone who has them joins the group, sees the members' protection status and can ask for roles, which a person on the other instance still has to allow. Without a login password anyone who can open the web interface can read them, and the pairing card says so until a password is set.

### Keys {#keys}

- **APP_KEY does two things.** It seals what this install stores (passwords, sign-ins, tokens) and it is the key of the backups: the restic password is derived from it, and container and VM definitions are sealed with it.
- **A taken-over key is kept apart.** When the Emergency wizard takes over another server's backups, this install keeps sealing with its own APP_KEY and stores the other server's key as its **backup key**, sealed like every other secret. The recovery kit then prints both. If APP_KEY is changed afterwards, the backup key can no longer be opened: BombVault stops backing up and says so under **Needs you**, instead of writing with the wrong key. Upload the recovery kit again under **Restore, Emergency**, or set APP_KEY back. See [Emergency wizard](offsite-recovery.md#emergency).
- **Encryption** is on by default (**Encrypt backups** in the **Encryption** card under **Settings, Security**). Backups are encrypted by restic with the password derived from the backup key.
- **Key deposits.** A key keeper in your group holds a sealed copy of this server's recovery kit that it cannot open itself. Only the 20-character recovery code opens it, and the keeper hands it out only after someone there presses **Allow**. See [Key keeper](offsite-recovery.md#key-keeper).

### Network {#network}

- **Changes need JSON.** A request that changes something must send `Content-Type: application/json` and must not be marked cross-site by the browser, so a page on another site cannot make your browser change settings on a LAN address. A script that drives the API sends that header; anything else is refused with `415`.
- Because the gate is opt-in, when unset the whole UI and API (including the storage locations, tamper-test routes and the restore routes) are reachable by anyone who can reach the port. Set a login password once off-site, append-only backups or encryption are in use.
- Run BombVault only on a trusted, non-exposed network. For remote access put it behind a reverse proxy that adds authentication and TLS. Responses carry baseline security headers (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Behind a reverse proxy every request carries the proxy's address, so without `TRUSTED_PROXY` the login throttle counts every client in one bucket and an attacker's failures lock you out as well. Name the proxy in `TRUSTED_PROXY` to get per-client counting back.
- A reverse proxy in front of BombVault has to pass the `Authorization` or `X-API-Key` header through to `/mcp` and must not buffer its responses, or assistants cannot connect. See [MCP server](mcp.md#tls).
- The MCP endpoint `/mcp` answers `404` until a key exists or sign-in through OAuth is switched on, and it asks every client for its key or token even with the login password off; no address is exempt, not even `localhost`. It has no restore or delete tools, and restoring a configuration backup revokes every key. See [MCP server](mcp.md#security).
- With `HTTP_ONLY=true` the session cookie loses its `Secure` flag (it has to, to work over plain HTTP), so only enable the password behind a TLS-terminating proxy if confidentiality matters.
- The VM-backup SSH connection trusts the host key on first connect (TOFU) and pins it thereafter. Verify the host's key out-of-band if your container-to-host path is not trusted.

## MCP server {#mcp-server}

The MCP server needs no environment variable. You switch it on by creating a key under **Settings, Integrations, MCP server**, and it answers at `/mcp` on the same port as the web interface (for example `https://192.168.1.10:3443/mcp`). Without an active key that path answers `404`. Clients, certificates and limits are described on [MCP server](mcp.md).

## VM backup over SSH

BombVault backs up KVM/libvirt VMs **without mounting any libvirt path**. It runs `virsh` on the host over SSH (`qemu+ssh://`), so it can never affect your host VM Manager.

Mounting the host's libvirt socket into a container is fragile on Unraid: the VM Manager owns those paths, and switching "Enable VMs" can leave libvirt unable to start. The SSH key grants root on the host, the same level of trust as the Docker socket BombVault already uses.

Quick setup:

1. **Settings, Integrations, Host SSH:** copy the shown public key.
2. Append it to Unraid's `/root/.ssh/authorized_keys` (also persisted to the flash so it survives reboots).
3. Click **Test connection**.

The template adds `--add-host=host.docker.internal:host-gateway` so the container can reach the host. Set `LIBVIRT_HOST` to your Unraid LAN IP if that name does not resolve (for example when the container runs on a custom `br0.x` network). If you changed Unraid's SSH port, set `LIBVIRT_SSH_PORT` to match. **Live snapshots** additionally need the qemu guest agent in the VM and the disk on `/mnt/cache` (not `/mnt/user`).

!!! important "Full VM setup and networking guide"
    The complete step-by-step guide (SSH enablement, persistent key authorization, custom-network and VLAN routing, per-VM method and host-side troubleshooting) lives at [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) on GitHub.

## Off-site setup

Off-site copies go to **storage locations**. Open **Storage locations** from its settings tile (or pin it to the sidebar) and choose **Add storage location**. See [Off-site & recovery](offsite-recovery.md) for the full workflow (delete protection, tamper testing and DR drills). In short:

- **Backends:** SMB/CIFS and WebDAV through rclone without a host mount, NFS (mount the export on the host and use it as a local folder), native restic backends without rclone (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`), or any rclone remote (`rclone:<remote>:<bucket>/path`). Backblaze B2 has no native backend here: reach it through its S3 endpoint (`s3:https://s3.<region>.backblazeb2.com/<bucket>/<path>`) with the key ID and application key as the S3 credentials.
- **Credentials** are stored encrypted, either as the shared cloud credentials or as a named set. Both are cards under **Settings, Storage locations**, and a location's own page picks the set it uses under **Credentials**.
- **SSH targets need nothing installed on the far side.** `sftp:` only needs an SSH server. Add the public key from **Settings, Integrations, Host SSH** (also at `/config/ssh/id_ed25519.pub`) to the target user's `~/.ssh/authorized_keys`.
- **Off-site copy:** BombVault copies new snapshots with `restic copy` on a best-effort basis, on top of a (usually local) primary. A location's **Copy** card sets whether it copies **After every backup** or on a plan of its own, and **Settings, Schedule, Off-site copies per section** gives one section its own copy plan. **Replicate now** sits in each section's **Off-site copy** card under **Settings, Storage locations**.
- **Several storage locations per section:** the **Storage locations per section** card on the Storage locations page switches each line from a section to a location on or off. A location's **Rules per section** give one section its own repository, retention or compression there.
- **Placement per entry:** each container, VM, folder set and ZFS dataset lights **Local** and the locations that get its backups, in the **Schedule and storage location** card on its Settings tab. Entries without a choice of their own follow their section. See [Placement per entry](offsite-recovery.md#placement).
- **Retention:** each storage location keeps backups by its **How long to keep** card, and a section can keep its own values there under **Rules per section**. The copy in a section's off-site field keeps the rules of the **Off-site retention** card under **Settings, Storage locations**, which has **Keep rules per source** for single sections (leave it all-zero to never auto-trim off-site snapshots).
- **Bandwidth limits:** cap the restic upload/download rate in the **Off-site bandwidth** card under **Settings, Storage locations**, or per location in its **Copy** card.
- **Streaming first:** in the **Streaming** card on the Storage locations page, pick the media servers (Plex, Jellyfin and Emby are preselected by image name), the send rate from which one counts as streaming, the upload limit while it streams and how long after a stream the normal limit comes back.
- **Cold and archival storage class (S3):** for an S3 storage location, pick a restore-readable tier (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval) on its page. rclone remotes set their class in the rclone config.
- **Remote primary instead of local:** a section's own path can itself be one of the backends above, with no local copy and no replication step. See [Remote primary repositories](offsite-recovery.md#remote-primary-repositories) for the **Local / Remote** switch and its bandwidth, append-only and growth-budget settings.

## Anomalies {#anomalies}

Anomaly detection is set up in the **Anomalies** card under **Settings, Verify**. Each control saves as soon as you change it, and the three under the switch are hidden while detection is off.

| Setting | Default | What it does |
|---|---|---|
| **Detect anomalies** | On | Compares every backup with the entry's own history. Switched off, nothing new is checked and the **Anomalies** entry leaves the sidebar; the card still links to earlier findings. |
| **Sensitivity** | Balanced | Strict reports smaller changes, Permissive only large ones. |
| **Send a notification for** | Critical findings only | The lowest severity that sends a message through the channels set up under Notifications. Repeated backup and dump failures and failed scheduled restore checks already send their own message and are not sent twice. |
| **Keep old backups when a source shrinks sharply or is rewritten** | On | While an entry has an open finding for an almost empty source, a sharp shrink or most of its data stored again, retention and prune leave that entry's old backups alone. Acknowledge the finding or mark it as expected to let them go. |

Each entry can use its own sensitivity and notification minimum. Set them on the entry's **Verify** tab, or on the **Anomalies** page under **Monitoring**. For a ZFS entry they apply to every dataset of its tree.

## Portable settings (export and import) {#portable-settings-export-and-import}

The **Export / import settings** card under **Settings, System** writes your whole BombVault configuration (section settings, storage locations, schedules, retention, notifications) to a portable JSON file you can import on another instance, so moving to a new box or cloning a setup does not mean re-entering everything by hand. Import shows a preview and asks for confirmation, and it never touches your backup data or history.

!!! warning "The export can contain credentials"
    You choose whether to include the off-site, notification and MQTT broker credentials in the file. With credentials included, the export is as sensitive as your recovery kit, so store it somewhere safe. Without them, the file holds only non-secret settings.
