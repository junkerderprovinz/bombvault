<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner-dark.png">
    <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner.png" alt="BombVault" width="100%">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=githubactions&logoColor=white" alt="Lint" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/pulls/junkerderprovinz/bombvault?style=for-the-badge&logo=docker&logoColor=white&label=Pulls&color=1d99f3" alt="Docker Pulls" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/image-size/junkerderprovinz/bombvault/latest?style=for-the-badge&logo=docker&logoColor=white&label=Size&color=1d99f3" alt="Image Size" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://restic.net"><img src="https://img.shields.io/badge/Engine-restic-CE4844?style=for-the-badge&logoColor=white" alt="restic" height="36"></a>&nbsp;
  <a href="https://unraid.net"><img src="https://img.shields.io/badge/Unraid-Template-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<br>

<p align="center">
Your Unraid data, <b>sealed in a vault</b>. Drop a backup. Detonate a restore.<br>
BombVault backs up Docker containers, KVM VMs, appdata, the Unraid flash config — and even itself —
and restores everything with a single click. Containers <b>automatically reappear in the
Docker tab</b>, VMs <b>automatically in the VM tab</b> — no manual reinstall, no
reconfiguration, no drama.<br>
<br>
<b>Your data, locked in. Loss, locked out.</b> Data loss doesn't stand a chance.<br>
Powered by <a href="https://restic.net">restic</a> — deduplicated, incremental, always encrypted.
</p>

<br>

<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz">
    <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" width="220">
  </a>
</p>

<br>

<p align="center">
  <b>Status:</b> one-click <b>Docker container</b>, <b>KVM/libvirt VM</b>, <b>Unraid flash</b>, <b>app configuration</b> and <b>files/folders</b> backup &amp; restore are all live (VMs over SSH — no libvirt mount), with <b>off-site repos</b> (SMB/NFS/rclone/SSH-sftp), <b>per-source retention</b>, <b>file-level restore</b>, <b>integrity checks</b>, <b>pre/post-backup hooks</b>, a <b>protection-status dashboard</b> with <b>restore-verification drills</b>, <b>immutable/append-only off-site</b> with <b>tamper verification</b> (ransomware-resistant), <b>live restore progress + cancel</b>, <b>restore from another BombVault repo</b> (one-time, read-only), <b>self-healing maintenance</b> (orphaned-lock auto-recovery), and <b>notifications</b> (webhook / Matrix / email / Healthchecks / Unraid-native / Prometheus). A few niceties remain on the <b>roadmap</b> — marked <i>(planned)</i> below.
</p>

<br>

## Table of Contents

1. [What is this?](#1-what-is-this)
2. [Screenshots](#2-screenshots)
3. [Features](#3-features)
4. [How it works](#4-how-it-works)
5. [Security / trust model](#5-security--trust-model)
6. [Requirements](#6-requirements)
7. [Install on Unraid](#7-install-on-unraid)
8. [Configuration](#8-configuration)
9. [Development](#9-development)
10. [Support this project](#10-support-this-project)
11. [Credits](#11-credits)

<br>

## 1. What is this?

BombVault is a self-hosted, **Unraid-native** web app for **backup and full disaster recovery** of your Docker containers and KVM/libvirt VMs. It runs as a single Docker container, gives you a modern dark web UI, and handles the whole lifecycle:

- **Backs up** Docker appdata + container definitions, KVM/libvirt VM disks + XML (incl. UEFI NVRAM), the whole Unraid flash (`/boot`), any folders you point it at (named **file sets** with per-set excludes), and its own `/config` (settings database + off-site credentials).
- **Restores automatically** — containers are reinstalled and restarted so they reappear in the Docker tab exactly as before, and VMs are re-defined in the VM Manager with their disks + NVRAM reattached.
- **Schedules** incremental backups in the background (per domain) from one place — the **Schedules** tab under Settings — with one-click *"include all in schedule"* for containers and VMs, so you never have to think about it.
- **Optionally updates a container right after its backup** — flip on *Update after successful backup* on a container (advanced, off by default) and BombVault pulls the newest image and recreates it, but only when there's actually a newer image. A fresh restore point always exists first, so a bad update is one restore away. Optional extras: a **notification per updated container** so you know to check it, and **image cleanup** (Settings → Paths & Storage) that removes the superseded image afterwards — a base image shared by other containers is never deleted.

The core idea — one-click backup *and* automatic re-install of Docker containers — comes from [**VolumeVault**](https://github.com/Darkdragon14/VolumeVault) by [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0). BombVault is a fresh, independent implementation with restic as the engine; see [Credits](#11-credits).

<br>

## 2. Screenshots

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/dashboard.png" alt="BombVault Dashboard — health summary, protection status per domain, run history and backup-health heatmap" width="90%">
  <br><em>Dashboard — a compact health summary (overall status, next backup, last result) above protection status per domain, last backups, run history and a backup-health heatmap.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/recovery.png" alt="BombVault Recovery — guided disaster-recovery flow onto a fresh install" width="90%">
  <br><em>Recovery — a guided disaster-recovery flow: confirm your backups are readable, restore BombVault's own settings, then attach and restore your container, VM and flash backups onto a fresh install.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/containers.png" alt="BombVault Containers tab — per-container backup with schedule toggle, filters and bulk include/exclude" width="90%">
  <br><em>Containers — per-container backup with an include-in-schedule toggle, collapsible filters, bulk include/exclude, one-click backup and an expandable per-container history.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/settings.png" alt="BombVault Settings — tabbed configuration for domains, paths, schedules, off-site, notifications and integrity" width="90%">
  <br><em>Settings — organised into tabs (General · Paths &amp; Storage · Schedules · Off-site · Notifications · Integrity · System); enable each backup domain and pick an accent colour.</em>
</p>

<br>

## 3. Features

> **Simple by default.** The interface shows only the essentials (back up, restore, schedule). Use the **Simple / Advanced** switch in the sidebar to reveal the expert controls — retention, off-site copy, pre/post hooks, file-level restore, notifications, Prometheus metrics, integrity/maintenance tools and more. It's a per-browser preference, off by default, so newcomers get a clean UI and power users get everything.

### Backup scope

| What | What is saved |
|---|---|
| **Docker containers** | Appdata directory + container definition (image, env vars, ports, labels, volumes) |
| **KVM / libvirt VMs** | VM disk image(s) + XML definition + UEFI NVRAM (graceful-shutdown or live-snapshot, over SSH). Live snapshots **fall back to a graceful backup automatically** if the snapshot can't be created, so a VM backup never just errors out |
| **Unraid flash** | The whole USB flash (`/boot`) — OS, license, array config, shares, network + plugin config. Restore is a one-click **`.zip` download** (never overwrites the live flash) |
| **App configuration** | BombVault's own `/config` — its settings database, off-site credentials (`rclone.conf`) and libvirt SSH keypair, snapshotted with SQLite `VACUUM INTO` so a WAL-mode database is never captured mid-write. No container stop. Restore is from the **Recovery** tab, staged and applied by a self-restart so the live database is never overwritten under an open handle |
| **Files & folders** | Named **file sets** — any folder on the server (a share, your documents, a photo library), each with optional per-set **exclude patterns**. Full parity with the other domains: schedules, retention, off-site copy, integrity checks and restore drills. Sources just need to be visible under the container's `/mnt` mapping — the Unraid template's default Host Data mount (all of `/mnt`) already covers shares, cache and pool paths |

### Restore (the good part)

- **One-click full restore** — pick a snapshot, click Restore. Done.
- **Restore from local or off-site** — every backup browser (and the integrity/maintenance card) has a **Local | Off-site** switch, so if a local repo is lost or corrupt you can list and restore straight from the off-site replica. Delete is per-source: removing a backup only affects the copy you're viewing, never both.
- **Containers are automatically reinstalled**: the container definition is replayed against the Docker API so the container reappears in the Unraid Docker tab exactly as it was — same image, same settings, same port mappings.
- **VMs are automatically recreated**: the XML definition is re-imported over SSH so the VM reappears in the VM Manager with its disk + UEFI NVRAM reattached, even after the VM was deleted. A VM deleted from the host shows under **Not installed** in the VM tab; if its entry is gone too (e.g. after a fresh install), **Discover backups** rebuilds it from storage — same as for containers.
- **Individual restore** — restore one container, one VM or one file set without touching the others.
- **Flash restore is a `.zip` download** — pick a snapshot and it streams straight to your browser as `flash-<id>.zip`, ready to drop into the Unraid USB creator (or unzip onto a fresh USB). The live, running `/boot` is never touched, and because a zip carries no filesystem metadata there are no permission errors on the way out.
- **Scheduled flash zip export** — turn it on and, after every flash backup, BombVault also writes the snapshot out as a plain `.zip` to a folder you pick — either a single `flash-latest.zip` that's overwritten each time or a rolling history of timestamped zips. Point it at a Syncthing or rclone folder and your bootable-USB backup leaves the server automatically, so it's reachable even when the server itself won't boot.
- **Pre-flight conflict check** — before anything is stopped or removed, restore verifies the container's static IP and published host ports are free; if another container already holds one, it aborts with a clear, actionable message instead of leaving you with a half-finished restore.
- **File-level restore** — expand a container snapshot's **Files**, filter, **tick any number of files and folders**, then restore the whole selection **in place** (original locations) or **into a folder** you pick.
- **File-set restore** — restore a file-set snapshot **in place** (back to its original folder, after an explicit confirmation) or **into a folder** you pick — never silently. **Selective restore** works here too: list the snapshot's contents, tick only the files and folders you want, and restore just those — same file-tree picker as the container file-level restore.
- **Restore keeps the run-state** — a container (or VM) that was running when backed up comes back running; one that was stopped stays stopped. Tick **Leave stopped after restore** to recreate it without starting it, so you can rebuild a group of dependent containers one by one and start them yourself afterwards.
- **Restore a whole stack** — containers from the same Docker Compose project (via the `com.docker.compose.project` label) are grouped into a **Stacks** panel. **Restore stack** rebuilds every member from its latest backup **left stopped**, then optionally **starts them in `depends_on` order** — so a compose stack (e.g. managed with Dockhand) comes back without members racing ahead of their dependencies.
- **Live progress, cancel & busy feedback** — a long restore shows a live percentage bar ("Restoring… NN%") instead of a bare spinner, and can be **cancelled** with a type-aware confirmation (a restore-to-a-folder cancels cleanly; an in-place restore warns it leaves the target partial). A cancelled restore is recorded as *cancelled*, not failed. And starting a backup while a restore (or a scheduled/maintenance op) holds a repository now shows a clear "a restore is running" hint instead of silently doing nothing.
- **Guided recovery** — a dedicated **Recovery** tab walks a fresh or rebuilt install through the disaster case: it **restores BombVault's own settings first** (so the backup paths, off-site targets and credentials the rest of the flow needs come pre-filled — applied via a self-restart over the Docker socket, so the live settings database is never overwritten under an open handle), checks BombVault can read your backups (the encryption-key gotcha up front), lets you point at your existing repo (local or off-site), **discovers** the containers, VMs and file sets stored in it, and **restores them all** (left stopped, so you start them deliberately) — with your recovery kit one click away. Everything a disaster recovery needs, in one place.
- **Restore from another BombVault repo** — a separate card on the **Recovery** tab opens a *different* BombVault instance's repo (a share mounted under `/mnt`, or a remote URL) with **that instance's `APP_KEY`**, in a **one-time, read-only session**: browse the containers, VMs and file sets stored there, pick a snapshot and restore it — the restored object becomes a normal local container / VM / file set. Nothing is ever written to the other repo, and **your own backup settings stay untouched** (the session lives in memory and expires by itself), so moving a container from server A to server B no longer means repointing your repo settings and reverting them afterwards. Live server-to-server federation (two instances talking to each other) is explicitly out of scope — this is a deliberate one-shot pull.

### Storage & scheduling

- Incremental, deduplicated backups via restic — even large VM disks don't balloon the repo.
- Destinations: a **local path**, or **off-site** — SMB/CIFS & NFS (mount the share on Unraid and point a Backup Path at it), **native restic backends** without rclone (`s3:…`, `rest:http://host:8000/repo`, `b2:…`, `sftp:user@host:/repo`) with their credentials stored encrypted under Settings → Off-site → Cloud credentials, or **rclone** (any of its remotes) via Settings → Off-site (`rclone:<remote>:<bucket>/path`). All credentials are stored encrypted.
- **SSH targets need nothing installed on the far side** — `sftp:` only requires an SSH server, so a bare Raspberry Pi (no Docker, no restic) works as an off-site destination. BombVault connects with its own persistent SSH keypair: add the public key shown under **Settings → System → VM Backup over SSH** (also at `/config/ssh/id_ed25519.pub`) to the target user's `~/.ssh/authorized_keys`, then use `sftp:user@host:/path/to/repo`. Host keys are pinned automatically on first contact.
- **Off-site copy (local + remote):** keep the fast local backup *and* add an off-site replica. Set a second repo per domain on the **Settings → Off-site** tab; BombVault replicates new snapshots there with `restic copy` (best-effort — an off-site hiccup never fails the local backup). The local repo stays primary. Each domain has its own **off-site schedule** (edited alongside every other schedule on the **Settings → Schedules** tab): leave it blank to replicate after every local backup, or set a cadence (e.g. `weekly Sun 03:00`) to ship off-site less often than you back up locally — plus a **Replicate now** button for on-demand runs. While a replication is in flight, an **off-site replication indicator** shows which domain is running (on its page and the Dashboard); it is an active indicator, not a percentage bar, since `restic copy` exposes no machine-readable progress.
- Configurable **retention**: keep-last / daily / weekly / monthly, pruned automatically after each backup. Set it **per source** — the **local** policy sits on Settings → Paths & Storage next to the backup paths it prunes, the **off-site** policy on Settings → Off-site, so you can keep off-site copies longer as an archive. Leave the off-site policy all-zero to never auto-trim off-site snapshots.
- Per-domain scheduling (daily / weekly incl. multi-day sets / every-N-days / raw cron), all edited in one place on Settings → Schedules; per-backup-group scheduling is *(planned)*.
- **Off-site bandwidth limits** (Settings → Off-site) — cap the `restic` upload/download rate so replication doesn't saturate your WAN.
- **Backup folders stay copyable off-box** — restic writes local repos owner-only (`0700`/`0600`), which over an SMB share can lock a non-root sync user out of the whole folder. After every backup BombVault relaxes the local repo tree to dirs `0755` / files `0644` (repos are encrypted, so nothing is exposed) and heals folders an older version locked down. Recovery definitions live **inside** each repo (`<repo>/def`, `<repo>/vm-def`), so a copied repo folder is fully self-contained.

### Insight, verification & monitoring

- **Protection status (RPO)** — the Dashboard shows a green / amber / red indicator per domain comparing the last successful backup against its schedule, so an overdue backup turns red instead of hiding in a log.
- **Backup-health heatmap** — a GitHub-contributions-style calendar of per-day backup outcomes per domain (green = all OK, red = a failure), with a Containers / VMs / Flash / Config / Files toggle.
- **Run timing everywhere** — every run-history entry reads `start → end (duration)`, and each container and VM carries its own **Recent runs** list right on its page, so per-item timing never hides in a log.
- **A dashboard you can rearrange** — the pencil icon in the top-right toggles customize mode: drag the cards into your order (or nudge them up/down) and hide the ones you don't need; the layout is saved per browser.
- **Repository size & dedup trend** — current repo size, deduplication ratio and snapshot count per domain, with a sparkline of how storage grows over time.
- **Restore-verification drills** — BombVault periodically *proves* your backups are restorable (`restic check --read-data-subset`, bounded — never a disk-filling full restore) and shows a **"last verified restorable"** badge per domain (Settings → Integrity; the drill cadence lives with every other schedule on Settings → Schedules).
- **Self-healing operations** — an orphaned restic lock (left behind when the container is updated or restarted mid-operation) used to fail the next verify or retention prune with "repository is already locked". Both now detect the provably orphaned lock, force-clear it and retry once, automatically; a real problem still surfaces. Retention itself is **identity-stable** — snapshots are pruned per item, immune to path or host changes — and a retention failure sends a **notification** instead of hiding in the container log.
- **Encryption-key recovery kit** — one-click download of the master key, the derived restic password and the exact repo locations + commands, so you can restore **without a running BombVault**. A Dashboard reminder nags until you've stored it.
- **Notifications** — webhook (Discord / Slack / Gotify / ntfy), Matrix, Healthchecks.io, **email (SMTP)** and **Unraid's native notification system** (over the SSH link); policy per backup: never / on failure / always. A scheduled run of many containers/VMs can send **one "N of M succeeded" summary** per run instead of a message per item (off by default — 45 containers no longer means 45 emails); manual backups still notify per item. **Healthchecks** gets the full lifecycle — a `/start` ping when a backup begins, then success / `/fail` on done — whenever a URL is set, independent of that policy, so it measures duration, catches a run that started but never finished, and stays green on success even with failure-only notifications. You can also give each domain (containers / VMs / flash / config / files) its own Healthchecks check for per-domain runtime and history, or leave them blank to share one global check.
- **Prometheus `/metrics`** — opt-in (default off, optional bearer token) for Grafana or Uptime Kuma; exposes backup status, sizes and timestamps, with no secrets or paths in the labels.

### Ransomware protection

- **Immutable (append-only) off-site** — flag an off-site repo append-only so ransomware (or a compromised host) can't delete or rewrite your backups. The far side (a `restic/rest-server` in `--append-only` mode) *enforces* it; BombVault only ever *verifies* it and never shows green on a configuration claim alone.
- **Tamper test** — BombVault periodically *proves* the append-only guarantee by actually attempting a delete against the off-site repo (aimed at a non-existent object): refused = protected, accepted = not protected. An inconclusive result (server unreachable, auth error) never flips the stored verdict, and a real protected → unprotected flip fires a single alert.
- **Guided off-site setup** — a wizard walks you from backend choice (rest-server / rclone / S3) through a ready-to-paste rest-server deploy snippet, a connection test, the immutable toggle (which runs the tamper test immediately) and a retention strategy — so append-only off-site is reachable without hand-editing configs.
- **DR drills (off-site)** — beyond the local integrity drill, BombVault can restore a real target from the *off-site* repo into a throwaway sandbox, verify it file-for-file and byte-for-byte, then clean up — proving you can actually recover from off-site, not just that the repo answers.
- **Ransomware-protection scorecard** — a Dashboard card with a green / amber / red posture per domain and an age-stamped checklist (off-site configured, append-only verified, replication current, restore drill passed, encryption on, prune strategy set); every red row deep-links to the fix. The card and its chip only ever go green on *verified* facts, never on intent.
- **Growth-budget alarm** — for an immutable off-site (where old snapshots are deliberately never pruned), set a size budget and get alerted before it runs away.

### Other

- **Back up many at once** — multi-select containers and hit **Back up selected**. The batch runs **server-side**, so it keeps going even if you close the tab, lose the connection, or back up the very container your browser is running in. Each container shows its own progress bar plus an overall batch indicator. BombVault never backs up (and so never stops) its own container.
- Snapshot browser with a restore-point list; **delete individual backups** you no longer want, and a **collapsible folder tree** for file-level restore.
- Repository maintenance per domain (Settings → Integrity & maintenance): **Verify** (`restic check`), **Unlock** (clear a stale lock left by an interrupted run), and **Prune** — when a retention policy is set, Prune **applies it** (collapses snapshots per your keep-last/daily/weekly/monthly rules and reclaims space), so you can enforce a newly-changed policy on demand instead of waiting for the next backup; with no policy set it stays a plain space-reclaim.
- Pre/post-backup hooks per container — shell commands run inside the container (e.g. `mysqldump` into appdata before backup); a failing pre-hook aborts the backup.
- **Stop other containers during backup** — name dependent containers (e.g. a database) to stop while this one is backed up and start again afterwards.
- **Exclude patterns per container** — list subdirectories to skip inside a backed-up volume, one per line (e.g. Plex's regenerable `.../Plex Media Server/Cache`), to shrink the backup while keeping the important data. Type the paths as you see them **inside the container** (`/config/…`); BombVault translates them to what restic stores so they match exactly, and a **live preview** shows what each line resolves to and warns when a line would exclude nothing.
- **Plain export** — a per-container **Export** button writes a browsable, tool-free copy next to the repo: `<name>.tar.gz` of the backup folders plus the Unraid `<name>.xml` template (like the Appdata Backup plugin). Restic stays the engine; the export is an extra, *unencrypted* convenience copy.
- **VM plain export** — VMs have the same **Export (plain tar)**: `<name>.tar.gz` of the disk image(s) plus `<name>.xml` (the persistent domain definition), restorable with `virsh define` + the disk, no BombVault or restic needed.
- **Restore to an alternate folder** — restore a container snapshot (or individual files) to a different path instead of in place, for cloning or inspection.
- **Snapshot diff & tags** — compare two snapshots to see what changed (files added / changed / removed and the size delta), and tag snapshots to filter them.
- **What's new after an update** — the release notes pop up once per new version, served from notes embedded in the binary, so the dialog works offline and without GitHub rate limits.
- HTTPS out of the box (self-signed, or BYO cert behind a reverse proxy).
- **Docker healthcheck** — the container reports healthy/unhealthy from its own `/api/health`, so an auto-heal tool (Autoheal and the like) can restart it automatically if the engine ever wedges.
- Dark/light UI in **26 languages** with a flag picker.

<br>

## 4. How it works

```
Browser ──HTTPS──> BombVault container
                   ├─ Go binary: JSON API + embedded React UI
                   ├─ Background worker (per-domain scheduler + job executor)
                   │
                   ├─ /var/run/docker.sock  ─> Docker API (container stop/inspect/recreate)
                   ├─ qemu+ssh://host       ─> libvirt / KVM on the HOST over SSH (no mount)
                   ├─ /mnt/ ─> /host/user   ─> appdata, VM disks + restic repos (read/write)
                   ├─ /boot/ ─> /host/boot  ─> Unraid flash backup (whole USB)
                   ├─ /config               ─> BombVault's own settings + credentials (self-backup)
                   └─ <repo path>           ─> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault talks to the Docker socket to stop containers before backup and recreate them after restore. For VMs it runs `virsh` **on the host over SSH** (`qemu+ssh://`) to gracefully shut down or live-snapshot a domain — it never bind-mounts any libvirt path, so it can never interfere with the host VM Manager. All actual data movement goes through **restic** — BombVault is the orchestration and UI layer, not the storage engine.

Restore is the star: after copying data back from the restic snapshot, BombVault replays the saved container definition against the Docker API (`docker run` equivalent), so the container reappears in the Unraid Docker tab as if it had always been there. VMs get their XML re-defined over SSH and their disks + UEFI NVRAM reattached.

<br>

## 5. Security / trust model

> [!WARNING]
> **BombVault holds root-equivalent control of the host**: via the Docker socket it can
> stop, remove and recreate containers and reads/writes appdata, and for VM backup it logs
> in to the host over SSH (`qemu+ssh://`, root by default) to run `virsh`. Anyone who can
> reach its web UI effectively has root on the host.

BombVault has **optional built-in password protection** (Settings → Security): set a password
to require login, clear it to disable. It is **off by default** for trusted-LAN use. Sessions are
signed (HMAC, derived from `APP_KEY`) and changing the password invalidates them; logins are
rate-limited to slow guessing. Regardless,
run BombVault **only on a trusted, non-exposed network** — never publish it directly to the
internet; for remote access put it behind a reverse proxy that adds authentication and TLS.
Responses carry baseline security headers (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).

Because the password gate is **opt-in**, when it is unset the whole UI and API are reachable by
anyone who can reach the port — including the off-site setup and tamper-test routes that mint or
use append-only credentials, and the encryption-key recovery kit. Enable the password gate
(Settings → Security), especially once off-site/immutable backups or encryption are in use, and
never expose the port directly to the internet — reach it over a VPN or a reverse proxy that adds
authentication.

Two caveats for the security-conscious: with `HTTP_ONLY=true` the session cookie loses its
`Secure` flag (it has to, to work over plain HTTP), so only enable the password behind a
TLS-terminating proxy if confidentiality matters. And the VM-backup SSH connection trusts the
host key on first connect (TOFU) and pins it thereafter — fine on a trusted LAN, but verify the
host's key out-of-band if your container↔host path isn't trusted.

Backups are encrypted by restic when encryption is enabled (Settings; on by default), with the
key derived from `APP_KEY`.

<br>

## 6. Requirements

| Requirement | Notes |
|---|---|
| **Unraid 6.12+** | Earlier versions not tested |
| **Restic repo location** | Local path (recommended: your array or cache), SMB, NFS, or any rclone backend |
| **Docker socket** | Mounted by the template automatically (`/var/run/docker.sock`) |
| **Unraid flash** (`/boot`) | Mounted whole by the template automatically (`/boot` → `/host/boot`). Powers Flash backup of the entire USB, and lets a restored container reappear as a normal, editable Unraid app (via the templates folder under `/boot`) instead of a "third-party" container |
| **KVM VMs** *(opt-in)* | VM backup talks to libvirt **over SSH** — no libvirt mount. Set it up in Settings → see below |

> [!IMPORTANT]
> **VM backup uses SSH, not a libvirt mount.** BombVault never bind-mounts any
> libvirt path (mounting the host's libvirt socket/runtime on Unraid is fragile —
> the VM Manager owns those paths and toggling "Enable VMs" can leave libvirt
> unable to start). Instead it runs `virsh` **on the host over SSH**
> (`qemu+ssh://`), so it can never affect your host VM Manager. Setup:
> **Settings → System → VM Backup over SSH** → copy the shown public key → append it to
> Unraid's `/root/.ssh/authorized_keys` → click **Test connection**. The template
> adds `--add-host=host.docker.internal:host-gateway` so the container reaches the
> host; set `LIBVIRT_HOST` to your Unraid LAN IP if that name doesn't resolve (e.g.
> when the container runs on a custom `br0.x` network). If you changed Unraid's SSH
> port, set `LIBVIRT_SSH_PORT` to match (default `22`). The
> SSH key grants root on the host — the same trust level as the docker.sock
> BombVault already uses. **Live snapshots** additionally need the qemu guest
> agent in the VM and the disk on `/mnt/cache` (not `/mnt/user`).
>
> **Full setup + networking guide:** [docs/vm-backup-ssh-setup.md](docs/vm-backup-ssh-setup.md).

<br>

## 7. Install on Unraid

Install via **Community Applications** — search for **BombVault**.

Or add the template manually:

1. In Unraid, go to **Docker → Add Container → Template repositories** and add:
   ```
   https://github.com/junkerderprovinz/unraid-apps
   ```
2. Search for **BombVault** in Templates.
3. Set the required variables (see [Configuration](#8-configuration)) and click **Apply**.

<br>

## 8. Configuration

| Variable | Required | Description |
|---|---|---|
| `APP_KEY` | **Yes** | 32-byte hex secret (64 hex chars) used to derive the restic repo password. Generate with `openssl rand -hex 32`. **Keep this safe** — losing it makes encrypted backups unrecoverable. |
| `LIBVIRT_HOST` | For VMs | Unraid host reached over SSH for VM backup (default `host.docker.internal`; the template pre-fills a LAN-IP placeholder — use your Unraid LAN IP, required on a custom `br0.x` network). |
| `LIBVIRT_SSH_PORT` | No | Host SSH port for VM backup (default `22`). |
| `LIBVIRT_SSH_USER` | No | SSH user on the host for VM backup (default `root`). |
| `PORT` | No | HTTP port (default `3000`; only used with `HTTP_ONLY=true`). |
| `HTTPS_PORT` | No | HTTPS port (default `3443`; the template publishes it 1:1, so the WebUI answers on `https://<ip>:3443`). |
| `HTTP_ONLY` | No | Set `true` to disable the self-signed HTTPS listener and serve plain HTTP only (for use behind a TLS-terminating reverse proxy). |
| `HOST_SOURCE_ROOT` | No | The host path mounted as **Host Data** (default `/mnt`). BombVault translates the bind-mount sources Docker reports (e.g. `/mnt/user/appdata/x`) into paths under this mount — change only if you mounted a different host root. |
| `BOMBVAULT_SELF_CONTAINER` | No | The name of the BombVault container itself, so it never backs up (and thus stops) itself (default `BombVault`; auto-detected via the hostname on bridge networking). |
| `TZ` | No | Timezone for the scheduler (e.g. `Europe/Berlin`). |

Mount the Docker socket, the flash (`/boot`) and the **Host Data** root (`/mnt`) as shown in the CA template — backup *sources* and *destinations* both live under Host Data, and it is mounted **rslave** so a remote share that mounts after the container starts (e.g. under `/mnt/remotes`) becomes visible without a restart. **Backup repository paths are configured in the app** (Settings → Backup paths) — not via env — and default to `/mnt/user/bombvault/{container,vms,flash,config,files}`, created on the first backup (change the location any time in Settings). VM backup needs no mount: see [§6](#6-requirements) and [docs/vm-backup-ssh-setup.md](docs/vm-backup-ssh-setup.md).

> [!NOTE]
> **Host integration check:** open `/spike` in the web UI after the container starts. It probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces.

<br>

## 9. Development

BombVault is a single static **Go** binary that serves a JSON API and an embedded
React/Vite SPA (`go:embed`). Build the SPA first, then run the binary:

```bash
npm --prefix web ci
npm --prefix web run build     # outputs web/dist (embedded into the binary)
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # Go unit + integration tests (real restic roundtrip)
golangci-lint run ./...        # lint
go run ./cmd/bombvault         # serves https://localhost:3443 (self-signed cert)
```

Real Docker, libvirt and Unraid behavior cannot be tested in CI (no KVM, no Unraid on runners). Use the **host integration check** (`/spike` in the web UI) to validate mounts, restic and the VM SSH connection on your actual Unraid host before submitting a PR.

<br>

## 10. Support this project

Questions, bugs, ideas? **[Unraid support thread →](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)** (or open a [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues)).

<a href="https://buymeacoffee.com/junkerderprovinz">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/button-buy-me-a-coffee.svg" alt="Buy me a coffee" height="40">
</a>

<br>

## 11. Credits

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** by [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) — the original idea that sparked BombVault: one-click backup and automatic re-install of Docker containers. Thank you. BombVault is an independent rewrite (Go + restic) that extends the concept to VMs and the Unraid flash.
- **[restic](https://restic.net/)** — the fast, secure, deduplicating backup engine BombVault orchestrates.
- **[rclone](https://rclone.org/)** — off-site cloud backends.

---

<sub>Part of a family of self-hosted Unraid apps + plugins by <b>junkerderprovinz</b> — see them all at <a href="https://github.com/junkerderprovinz">github.com/junkerderprovinz</a>, or install from <a href="https://unraid.net/community/apps">Community Applications</a>.</sub>
