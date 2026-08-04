# Features

BombVault is simple by default and deep when you need it. The interface shows only the essentials until you flip the **Simple / Advanced** switch. This page groups the full feature set.

## Backup scope

| What | What is saved |
|---|---|
| **Docker containers** | Appdata directory plus the container definition (image, env vars, ports, labels, volumes). |
| **KVM / libvirt VMs** | VM disk image(s), XML definition and UEFI NVRAM (graceful shutdown or live snapshot, over SSH). Live snapshots fall back to a graceful backup automatically if the snapshot cannot be created, so a VM backup never just errors out. |
| **Unraid flash** | The whole USB flash (`/boot`): OS, license, array config, shares, network and plugin config. Restore is a one-click `.zip` download and never overwrites the live flash. |
| **App configuration** | BombVault's own `/config` (settings database, off-site credentials, libvirt SSH keypair), snapshotted with SQLite `VACUUM INTO` so a WAL-mode database is never captured mid-write. Restored via a self-restart, so the live database is never overwritten under an open handle. |
| **Files & folders** | Named **file sets**: any folder on the server (a share, your documents, a photo library), each with optional per-set exclude patterns. Full parity with the other domains (schedules, retention, off-site copy, integrity checks and restore drills). |

## Restore

- **One-click full restore.** Pick a snapshot, click Restore. Done.
- **Restore from local or off-site.** Every backup browser has a **Local / Off-site** switch, so if a local repo is lost or corrupt you can list and restore straight from the off-site replica. Delete is per-source: removing a backup only affects the copy you are viewing.
- **Containers are automatically reinstalled.** The container definition is replayed against the Docker API, so the container reappears in the Unraid Docker tab exactly as it was.
- **VMs are automatically recreated.** The XML is re-imported over SSH so the VM reappears in the VM Manager with its disk and UEFI NVRAM reattached, even after the VM was deleted. **Discover backups** rebuilds an entry that is gone entirely (for example after a fresh install).
- **Individual restore.** Restore one container, one VM or one file set without touching the others.
- **Flash restore is a `.zip` download.** It streams to your browser as `flash-<id>.zip`, ready to drop into the Unraid USB creator. The live `/boot` is never touched.
- **Scheduled flash zip export.** After every flash backup, optionally write the snapshot out as a plain `.zip` to a folder you pick (a single overwritten `flash-latest.zip` or a rolling history). Point it at a Syncthing or rclone folder so your bootable-USB backup leaves the server automatically.
- **Pre-flight conflict check.** Before anything is stopped or removed, restore verifies the container's static IP and published host ports are free, and aborts with a clear message instead of leaving a half-finished restore.
- **File-level restore.** Expand a container snapshot's **Files**, filter, tick any number of files and folders, then restore the selection in place or into a folder you pick.
- **File-set restore.** Restore a file-set snapshot in place (after an explicit confirmation) or into a folder you pick, never silently. Selective restore works here too.
- **Restore keeps the run-state.** A container or VM that was running when backed up comes back running; one that was stopped stays stopped. Tick **Leave stopped after restore** to recreate without starting.
- **Restore a whole stack.** Containers from the same Docker Compose project are grouped into a **Stacks** panel. **Restore stack** rebuilds every member from its latest backup left stopped, then optionally starts them in `depends_on` order.
- **Live progress, cancel and busy feedback.** A long restore shows a live percentage bar and can be cancelled with a type-aware confirmation. A cancelled restore is recorded as *cancelled*, not failed.
- **Guided recovery.** A dedicated **Recovery** tab walks a fresh install through the disaster case. See [Off-site & recovery](offsite-recovery.md).
- **Restore from another BombVault repo.** A one-time, read-only session opens a different BombVault instance's repo with that instance's `APP_KEY`, so you can pull a container from server A to server B without touching your own settings. See [Off-site & recovery](offsite-recovery.md).

## Storage & scheduling

- Incremental, deduplicated backups via restic, so even large VM disks do not balloon the repo.
- **Destinations:** a local path, or off-site. SMB/CIFS and NFS (mount the share on Unraid and point a Backup Path at it), native restic backends without rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), or any rclone remote via `rclone:<remote>:<bucket>/path`. All credentials are stored encrypted.
- **SSH targets need nothing installed on the far side.** `sftp:` only requires an SSH server, so a bare Raspberry Pi (no Docker, no restic) works as an off-site destination. Host keys are pinned automatically on first contact.
- **Off-site copy (local + remote).** Keep the fast local backup and add one or more off-site replicas, replicated with `restic copy` on a best-effort basis (an off-site hiccup never fails the local backup). Each domain has its own off-site schedule, plus a **Replicate now** button.
- **Multiple off-site targets per domain.** Each domain (containers, VMs, flash, config and file sets) can replicate to several off-site destinations at once, not just one. Add extra targets on the Off-site tab, each with its own repository, S3 storage class, append-only flag, retention and growth budget. Your existing off-site copy is carried over as the first target, so nothing changes until you add a second one, and every target of a domain replicates on that domain's off-site schedule.
- **Manual backup order.** Set the exact order your containers are backed up in from the backup-order panel on the Containers page. Scheduled and multi-select runs follow it; any container you leave unordered keeps the previous most-overdue-first behaviour, and a single container backup is unchanged.
- **Configurable retention:** keep-last / daily / weekly / monthly, pruned automatically after each backup, set **per source** (local next to the backup paths, off-site on the Off-site tab so you can keep off-site copies longer as an archive).
- Per-domain scheduling (daily / weekly including multi-day sets / every-N-days / raw cron), all edited in one place on **Settings, Schedules**.
- **Off-site bandwidth limits.** Cap the restic upload/download rate so replication does not saturate your WAN.
- **Cold and archival storage class (S3).** For a native S3 off-site repo you can pick the storage class, restricted to restore-readable tiers (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval) so archival pricing never silently breaks a restore. The deep-archive tiers that first need an async thaw (Glacier Flexible, Deep Archive) are intentionally left out. Native S3 backends only; rclone remotes set their class in the rclone config.
- **Backup folders stay copyable off-box.** After every backup BombVault relaxes the local repo tree to dirs `0755` / files `0644` (repos are encrypted, so nothing is exposed) so a non-root sync user over SMB is not locked out. Recovery definitions live inside each repo, so a copied repo folder is fully self-contained.

## Insight, verification & monitoring

- **Protection status (RPO).** The Dashboard shows a green / amber / red indicator per domain, comparing the last successful backup against its schedule, so an overdue backup turns red instead of hiding in a log.
- **Backup-health heatmap.** A GitHub-contributions-style calendar of per-day backup outcomes per domain, with a Containers / VMs / Flash / Config / Files toggle.
- **Run timing everywhere.** Every run-history entry reads `start, end (duration)`, and each container and VM carries its own **Recent runs** list on its page.
- **A dashboard you can rearrange.** Toggle customize mode to drag cards into your order and hide the ones you do not need. The layout is saved per browser.
- **Repository size & dedup trend.** Current repo size, deduplication ratio and snapshot count per domain, with a sparkline of storage growth.
- **Restore-verification drills.** BombVault periodically proves your backups are restorable (`restic check --read-data-subset`, bounded) and shows a *last verified restorable* badge per domain.
- **Self-healing operations.** A provably orphaned restic lock (left by a mid-operation restart) is force-cleared and retried once, automatically. Retention is identity-stable (pruned per item, immune to path or host changes) and a retention failure sends a notification.
- **Encryption-key recovery kit.** One-click download of the master key, the derived restic password and the exact repo locations and commands, so you can restore without a running BombVault. See [Off-site & recovery](offsite-recovery.md).
- **Export and import your settings.** An *Export and import settings* card on the Settings page writes your whole configuration (domain settings, off-site targets, schedules, retention, notifications) to a portable JSON file, so moving to a new box or cloning a setup does not mean re-entering everything by hand. You choose whether to include the off-site and notification credentials; with them the file is as sensitive as your recovery kit. Import shows a preview and asks for confirmation, and it never touches your backup data or history.
- **Notifications.** Webhook (Discord / Slack / Gotify / ntfy), Matrix, Healthchecks.io, email (SMTP), a self-hosted [Apprise API](https://github.com/caronc/apprise-api) server, and Unraid's native notification system. Policy per backup: never / on failure / always. A scheduled run of many items can send one *N of M succeeded* summary. Healthchecks gets the full lifecycle (`/start`, then success or `/fail`) whenever a URL is set.
- **Prometheus `/metrics`.** Opt-in (default off, optional bearer token) for Grafana or Uptime Kuma. Exposes backup status, sizes and timestamps, with no secrets or paths in the labels.

## Ransomware protection

- **Immutable (append-only) off-site.** Flag an off-site repo append-only so ransomware or a compromised host cannot delete or rewrite your backups. The far side (a `restic/rest-server` in `--append-only` mode) enforces it; BombVault only ever verifies it and never shows green on a configuration claim alone.
- **Tamper test.** BombVault periodically proves the append-only guarantee by actually attempting a delete against the off-site repo (aimed at a non-existent object): refused means protected, accepted means not protected. An inconclusive result never flips the stored verdict.
- **Guided off-site setup.** A wizard walks you from backend choice through a ready-to-paste rest-server deploy snippet, a connection test, the immutable toggle and a retention strategy.
- **DR drills (off-site).** Restore a real target from the off-site repo into a throwaway sandbox, verify it file-for-file and byte-for-byte, then clean up. See [Off-site & recovery](offsite-recovery.md).
- **Ransomware-protection scorecard.** A Dashboard card with a green / amber / red posture per domain and an age-stamped checklist; every red row deep-links to the fix. It only goes green on verified facts.
- **Growth-budget alarm.** For an immutable off-site (where old snapshots are deliberately never pruned), set a size budget and get alerted before it runs away.
- **Receiver dashboard (receiving side).** On the box that receives immutable off-site copies from another BombVault, turn on the **Receiver** toggle (Settings) to reveal a **Receiver** tab. Register a received repository read-only (opened with the sending instance's key) to see its snapshot inventory grouped by source, when each source last arrived, and run an independent `restic check` on the receiving hardware. It alerts you when a source stops sending within a window you set (a dead-man's switch) or when an integrity check fails. Strictly read-only, so it never writes to the received repository, and off by default. See [Off-site & recovery](offsite-recovery.md).

## Plain exports

- **Container plain export.** A per-container **Export** button writes a browsable, tool-free copy next to the repo: `<name>.tar.gz` of the backup folders plus the Unraid `<name>.xml` template. Restic stays the engine; this is an extra convenience copy.
- **VM plain export.** VMs have the same **Export (plain tar)**: `<name>.tar.gz` of the disk image(s) plus `<name>.xml`, restorable with `virsh define` plus the disk, no BombVault or restic needed.
- **Encrypt the plain exports (age).** The exports sit outside restic, so they are plaintext by default. Turn on age encryption under Settings and add one or more recipients (an age public key or an SSH public key). Each export (container and VM `.tar.gz`, their `.xml` sidecars, and the flash ZIP) is then sealed for those recipients, and you decrypt it later off the box with the matching private key. As a safety rule, with encryption on and no valid recipient set, an export fails with a clear error instead of ever writing plaintext.

## Other

- **Back up many at once.** Multi-select containers and hit **Back up selected**. The batch runs server-side, so it keeps going even if you close the tab or lose the connection. BombVault never backs up (and so never stops) its own container.
- **Snapshot browser** with a restore-point list, per-snapshot delete, and a collapsible folder tree for file-level restore.
- **Repository maintenance per domain:** **Verify** (`restic check`), **Unlock** (clear a stale lock), and **Prune** (applies the retention policy on demand when one is set, otherwise a plain space-reclaim).
- **Pre/post-backup hooks per container.** Shell commands run inside the container (for example `mysqldump` into appdata before backup); a failing pre-hook aborts the backup.
- **Stop other containers during backup, with a health-gated restart.** Name dependent containers (for example a database) to stop while this one is backed up. Afterwards BombVault brings them back in their Compose `depends_on` order and, by default, waits for each one to report healthy (or running, if it has no healthcheck) before starting the containers that depend on it, so a dependency like Pi-hole, a database or a VPN gateway is actually up before the services that need it, instead of those returning to a *connection refused*. The wait is bounded by a per-container timeout (120 seconds by default) so a slow or never-healthy container can never hang the run; both the wait and the timeout live on Settings, Schedules (turn the wait off for the previous all-at-once restart). The same ordered, health-gated restart also wraps the post-backup image update, so on a day an update lands the dependents are held down through the recreate and only brought back, health-gated, once it is done.
- **Exclude patterns per container.** List subdirectories to skip inside a backed-up volume, one per line. Type the paths as you see them inside the container; a live preview shows what each line resolves to and warns when a line would exclude nothing.
- **Update after successful backup (advanced, off by default).** Flip this on a container and BombVault pulls the newest image and recreates it, but only when there is actually a newer image, so a fresh restore point always exists first. Optional extras: a notification per updated container and image cleanup (a base image shared by other containers is never deleted). After the update BombVault also asks Unraid to re-check that one container's update status, so the Docker tab's stale *update available* banner clears itself instead of lingering (Unraid updates go straight through the Docker API, so its cached status, and on some versions a cached digest, would otherwise keep showing the banner). It is best-effort, never affects the backup, on by default and has a toggle in Settings.
- **Restore to an alternate folder** for cloning or inspection.
- **Snapshot diff & tags.** Compare two snapshots to see what changed, and tag snapshots to filter them.
- **What's new after an update.** Release notes pop up once per new version, served from notes embedded in the binary, so the dialog works offline.
- **HTTPS out of the box** (self-signed, or bring your own cert behind a reverse proxy).
- **Docker healthcheck.** The container reports healthy/unhealthy from its own `/api/health`, so an auto-heal tool can restart it if the engine ever wedges.
- **Dark/light UI in 26 languages** with a flag picker.
