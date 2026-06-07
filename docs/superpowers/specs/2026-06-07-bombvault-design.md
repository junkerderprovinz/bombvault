# BombVault — Design Spec

**Date:** 2026-06-07
**Status:** Draft for review
**Author:** junkerderprovinz (with Claude, via superpowers:brainstorming)

---

## 1. Overview

BombVault is a self-hosted, **Unraid-native (but cross-platform)** web application for
**backup and full disaster-recovery of Docker containers and KVM/libvirt VMs**. It runs as
a single Docker container, presents a modern dark web UI, and can back up *and* restore
container app-data, container definitions, VM disks/definitions and the Unraid flash config —
restoring them so that containers reappear in the **Docker tab** and VMs in the **VM tab**
with **no manual steps** (same-host disaster recovery).

It is inspired by [VolumeVault](https://github.com/Darkdragon14/VolumeVault) (Apache-2.0) but
is a fresh build in the author's stack, with the actual backup work delegated to
[**restic**](https://restic.net).

### Goals

- Do everything Unraid's *Appdata Backup* plugin does (app-data backup, container stop/start,
  flash config, VM meta) **and more** (full VM disk backup, one-click full restore).
- **Prettier, modern UI** with first-class restore UX (snapshot browser, file-level restore).
- **Individual, group, or full** backup/restore for containers and VMs.
- **Per-type destinations** (container / VM / flash can each go to a different place).
- Space-efficient (incremental + dedup), encrypted, multi-backend off-site backups.
- Distributed as an own image + CA template via the existing `unraid-docker-templates` feed.

### Non-goals (v1)

- Backing up the whole Unraid array / user shares (this is app/VM/flash DR, not bulk media).
- Cross-architecture or cross-host *VM* migration with hardware passthrough remapping
  (we detect and assist, but full auto cross-host passthrough restore is out of scope).
- A general restic UI (Backrest already exists); our value is the Unraid-aware DR layer.

---

## 2. Background

- **Unraid "Appdata Backup" plugin:** native plugin, stops containers, tars `appdata`,
  optionally backs up VM XML + flash. Does **not** back up VM vdisks. Tightly integrated but
  PHP/Slackware, constrained UI, Unraid-only.
- **VolumeVault:** Laravel/Vue web app that orchestrates `offen/docker-volume-backup`.
  Good UX/feature blueprint, but PHP stack and a full-tar engine (no dedup).
- **restic:** content-addressed, deduplicated, incremental, always-encrypted backup program
  with many backends and first-class restore. Chosen as BombVault's engine.

---

## 3. Key decisions (locked during brainstorming)

| Topic | Decision | Why |
|---|---|---|
| Platform | **Docker container** (not Unraid plugin) | Same capabilities via mounts, full UI freedom, CA-template distribution, cross-platform |
| Stack | **Next.js / TypeScript** (fresh build) | Author's stack, long-term maintainability; engine is external anyway |
| Engine | **restic**, always-encrypted, **key exportable** | Dedup/incremental (vital for vdisks), backends, first-class restore; encryption is transparent + safe |
| Encryption | Always on (restic), with **key escrow** (export/show repo password) | "Optional" gains little, loses dedup/restore; off-site needs it |
| Scope (v1) | **Maximal** | Containers + VMs (incl. live-snapshot), flash/USB, multi off-site backends, scheduling/retention, dashboard + one-click restore, notifications |
| VM quiesce | **Per-VM choice**: graceful shutdown (default) **or** live snapshot | Reliability now, near-zero downtime later (qemu-guest-agent) |
| Granularity | **Individual / group / full** for both containers and VMs | Restore one app/VM without touching others |
| Destinations | **Per-type** (container/VM/flash each its own target, schedule, retention) | User requirement |
| Extras in v1 | File-level restore, integrity verify (`restic check`), generic pre/post hooks | High value, low cost on restic |
| Extras deferred | Automated sandbox test-restore (v1.1), DB-dump presets (v2), ransomware/immutable backends (v2) | Bigger effort / backend-specific |
| Name | **BombVault** | User choice |
| License | MIT; credits VolumeVault (Apache-2.0) + restic | — |

---

## 4. Architecture

A single container running a Next.js/TS app plus a background worker. It reaches the host
through explicitly mounted sockets/paths.

```
Browser ──HTTPS──> BombVault container
                   ├─ Next.js (UI + API routes)
                   ├─ Worker (scheduler + job executor)     ← same image, separate process
                   ├─ SQLite (targets, jobs, runs, secrets, settings)
                   └─ bundled CLIs: restic, docker, virsh, qemu-img, rclone

Host access (mounts):
  /var/run/docker.sock              → list/stop/start/create containers (dockerode)
  /var/run/libvirt/libvirt-sock     → manage VMs (virsh)
  /mnt/user/appdata                 → container data
  /mnt/user/domains (configurable)  → VM vdisks
  /etc/libvirt (ro) or virsh dumpxml→ VM XML + NVRAM
  /boot                             → flash config + user templates

Destinations (restic repos, encrypted):
  local path | S3 / Backblaze B2 | SFTP | WebDAV | rclone-anything
```

### Components

- **UI + API** — Next.js App Router (route handlers / server actions). Pages: Dashboard,
  Targets, Schedules, Restore, Destinations, Settings, Onboarding.
- **Worker** — Node process: a `node-cron`-style scheduler reads `Schedule` rows and enqueues
  jobs into a SQLite-backed queue; an executor runs them serially (configurable concurrency).
  No Redis (keep the single-file ethos like featherdrop).
- **DB** — SQLite via `better-sqlite3` (migrations on boot).
- **Engine adapter** — typed wrapper that shells out to `restic` and parses `--json` output
  (backup, snapshots, ls, restore, check, forget/prune).
- **Docker adapter** — `dockerode` over the socket (inspect, stop, start, create, pull).
- **VM adapter** — `virsh`/`qemu-img` over the libvirt socket (dumpxml, shutdown, snapshot,
  define, start, autostart) + NVRAM handling.

### Image

Base `node:22-alpine` (or debian-slim if alpine libvirt/qemu packages are awkward), adding
`restic`, `docker-cli`, `libvirt-clients` (`virsh`), `qemu-img`, `rclone`. **Multi-arch**
amd64 + arm64. Loud "BombVault IS READY" log banner + ASCII init banner (house standard).

---

## 5. Backup targets & capture

### 5.1 Container
1. (optional) run **pre-hook**; **stop** the container (Docker socket), configurable timeout.
2. restic-backup the container's `appdata` path(s).
3. Capture the **definition**: copy the Unraid template `my-<App>.xml` from
   `/boot/config/plugins/dockerMan/templates-user/`; also store `docker inspect` JSON as a
   fallback for non-template containers.
4. **start** the container; run **post-hook**.
- The container **image** is *not* backed up (re-pulled from the registry on restore).

### 5.2 VM (per-VM mode)
- **Graceful shutdown (default):** `virsh shutdown` → wait → restic-backup vdisk(s) + NVRAM +
  `virsh dumpxml` → `virsh start`. 100 % consistent; downtime = backup duration.
- **Live snapshot:** `virsh snapshot-create-as --disk-only [--quiesce]` (quiesce needs
  `qemu-guest-agent` in the guest) → restic-backup the base image → `blockcommit` to merge →
  near-zero downtime. More moving parts; offered per-VM.
- Always capture: domain XML, NVRAM/UEFI vars (`*_VARS.fd`), vdisk image(s).

### 5.3 Flash / USB
- restic-backup `/boot/config` (templates, plugins config, Unraid config). Small; the basis
  for rebuilding the box.

### 5.4 Generic Docker volume / arbitrary path
- restic-backup named-volume mountpoints or any allow-listed host path (non-Unraid use).

---

## 6. Restore flows (the heart)

All restores support **dry-run / preview** and never overwrite without explicit confirm.

### 6.1 Container (→ Docker tab)
1. Restore `appdata` from the chosen snapshot.
2. Write the template XML back to `/boot/.../templates-user/`.
3. Re-create the container via the Docker socket from the template/inspect (image pulled from
   registry); start it.
4. Result: container appears in the **Docker tab**, running, with icon + update integration.

### 6.2 VM (→ VM tab)
1. Restore vdisk(s) + NVRAM to their paths.
2. `virsh define domain.xml` → VM re-registered; optional `virsh autostart` + `virsh start`.
3. Result: VM appears in the **VM tab** and runs.
4. **Passthrough guard:** before define, compare host PCI/USB IDs to those referenced in the
   XML; if they differ, warn and offer to strip/remap rather than silently failing.

### 6.3 File-level restore
- Browse a snapshot (`restic ls`), select files/folders, restore to original or a chosen path.

### 6.4 Full disaster recovery
- One action restores a selected set (all containers + all VMs + flash), in dependency-aware
  order, with a live progress view.

---

## 7. Scheduling, retention, granularity

- **Granularity:** every container and every VM is its own `BackupTarget` with its own
  schedule, destination and retention. Targets can be grouped (tags) and run together; an
  "all" job enables full-DR backups.
- **Schedules:** cron expression per target/group; optional time windows; manual "run now".
- **Retention:** restic `forget`/`prune` policies (keep last N / hourly / daily / weekly /
  monthly) per target.
- **Per-type destinations:** container, VM and flash targets may each point to different
  destinations (e.g., appdata→local+B2, VMs→big SFTP, flash→local).

---

## 8. Storage backends (destinations)

restic repos on: **local path**, **S3 / Backblaze B2**, **SFTP**, **WebDAV**, or
**rclone** (anything rclone supports). Each `Destination` stores its restic repo URL +
credentials (encrypted) + the repo password (encrypted, exportable). A destination can host
many targets (restic dedups across them).

---

## 9. Extras (v1)

- **File-level restore / snapshot browser** (≈ free with restic).
- **Integrity verify** — scheduled `restic check`; dashboard shows "last verified" and
  "age of last good backup" per target, with alerts on staleness/failure.
- **Pre/Post hooks** — arbitrary command (or `docker exec` into the target) before/after a
  backup, e.g. `pg_dump`/`mysqldump` for app-consistent DB dumps. Curated presets later.

---

## 10. Notifications

Pluggable channels; v1 ships **Unraid notifications** (write to the Unraid notify API/agent)
and **Matrix** (the author already runs Matrix). Events: success / failure / missed /
verify-failed / staleness. Optional per-target digest.

---

## 11. Data model (SQLite)

- **BackupTarget**: id, type(`container|vm|flash|volume|path`), ref, displayName, tags[],
  options(JSON: container stopMode/timeout; vm mode `shutdown|snapshot`, quiesce; include/exclude),
  hookPre, hookPost, enabled.
- **Destination**: id, name, kind(`local|s3|b2|sftp|webdav|rclone`), config(JSON), repoUrl,
  secretRef(creds + repo password).
- **Schedule**: id, targetId|group, cron, window, retention(JSON), destinationId, enabled.
- **Run**: id, targetId, destinationId, kind(`backup|restore|verify`), status, startedAt,
  finishedAt, bytes, snapshotId, logRef, error.
- **Secret**: id, ciphertext (AES-GCM with app master key from `APP_KEY` env).
- **Setting**: notifications config, defaults, onboarding state.
- **User**: single admin (argon2 password hash), session.

---

## 12. Security model

- **Powerful by necessity:** the Docker and libvirt sockets grant root-equivalent control.
  This is made **explicit and opt-in** in the CA template (each mount is a labelled field with
  a warning), mirroring how the Unraid plugin already has full host access.
- **Secrets at rest:** all credentials + restic repo passwords encrypted with `APP_KEY`.
- **Key escrow:** the UI can display/export every restic repo password + a printable recovery
  sheet so the user is never locked out if BombVault itself is lost.
- **Auth:** mandatory admin password (onboarding), session cookies, **HTTPS by default**
  (self-signed; works behind a reverse proxy) — consistent with the house WebUI-HTTPS rule.
- Least privilege where possible (read-only mounts for XML/flash where feasible).

---

## 13. UI

Modern dark dashboard, house palette `#161616` (IBM-Carbon mono). Screens:
- **Dashboard** — health tiles (last good backup age per target, failures, next runs), recent runs.
- **Targets** — containers/VMs/flash auto-discovered; per-target settings (mode, schedule,
  destination, retention, hooks).
- **Restore** — wizard: pick target → pick snapshot → preview (dry-run) → restore (whole /
  file-level / full-DR), live progress.
- **Destinations** — add/test backends; show/export repo keys.
- **Settings** — notifications, defaults, security.
- **Onboarding** — set admin password, confirm mounts, first destination.

(UI layout/mockups to be iterated with the visual companion during implementation.)

---

## 14. Deployment & distribution

- Own image on **GHCR + Docker Hub**, multi-arch, CI build/lint.
- **CA template** added to the **`unraid-docker-templates`** feed (folder `bombvault/`), with
  pre-filled, clearly-labelled mounts (docker.sock, libvirt-sock, appdata, domains, /boot) and
  HTTPS WebUI default.
- README + white banner + badges per house conventions; support via the shared
  `unraid-docker-templates` thread (or its own, TBD).
- Loud READY log banner + ASCII init banner.

---

## 15. Error handling

- **Container won't stop** (timeout) → configurable: force-kill, skip, or fail the run.
- **VM won't shut down gracefully** (timeout) → configurable: fall back to live-snapshot, or abort.
- **restic repo locked / stale lock** → detect and unlock stale locks safely; never two writers.
- **Destination unreachable** → bounded retries + alert; previous good snapshot preserved.
- **Partial failure** → run marked failed with per-step log; last good backup untouched.
- **Restore safety** → dry-run preview, explicit overwrite confirm, passthrough-mismatch warning,
  surfaced image-pull errors.

---

## 16. Testing strategy

- **Roundtrip characterization (CI):** create a throwaway container + volume with known data →
  backup → wipe → restore → assert data identical and container running. Same for a path/volume.
- **restic adapter:** unit tests parsing `--json` output; integration test against a temp local repo.
- **Orchestration logic:** unit-test command building (virsh define args, template handling,
  stop/start sequencing) with mocked adapters.
- **VM path:** logic unit-tested with mocked `virsh`; a real backup→restore is validated
  **manually on the Unraid box** (CI runners lack KVM) and documented as a release gate.
- **Scheduled `restic check`** as an ongoing integrity test in production.

---

## 17. Implementation phasing (build order; v1 still ships all)

- **P0 Skeleton** — Next.js app, SQLite + migrations, auth/onboarding, image with
  restic/docker/virsh/qemu/rclone, READY+ASCII banners, HTTPS.
- **P1 Containers** — discover, backup (stop/appdata/template), local destination, manual run,
  container restore → Docker tab.
- **P2 Scheduling + dashboard** — scheduler/worker/queue, run history, retention, `restic check`,
  health tiles, file-level restore.
- **P3 Remote destinations** — S3/B2/SFTP/WebDAV/rclone, encryption + key escrow, notifications
  (Unraid + Matrix).
- **P4 VMs** — backup/restore graceful-shutdown; then live-snapshot; passthrough guard; flash/USB.
- **P5 Extras + polish** — pre/post hooks, full-DR orchestration, UI polish.
- **P6 Distribution** — image CI (GHCR+Docker Hub), CA template in the feed, README/banner/support.

---

## 18. Open questions / risks

- **libvirt access from a container** on Unraid (socket path/permissions) — validate early on
  the real box; this is the riskiest integration.
- **Live-snapshot** correctness (blockcommit, quiesce without guest agent) — treat as the
  advanced path; ship shutdown mode first.
- **Unraid template re-create** edge cases (containers created without a template) — fall back
  to `docker inspect`, optionally synthesise a template.
- **Full-DR ordering / dependencies** (e.g., a DB container before its app) — needs a simple
  dependency/priority field.
- Own support thread vs. shared `unraid-docker-templates` thread — decide at distribution.

---

## 19. Out of scope (v2+)

Automated sandbox test-restore; DB-dump presets; ransomware/immutable (object-lock) backends;
multi-host management; array/share bulk backup; cross-host passthrough remap.

---

## 20. Credits & licenses

BombVault: MIT. Design blueprint: **VolumeVault** (Apache-2.0). Engine: **restic** (BSD-2).
Also uses dockerode, rclone, libvirt/qemu tooling.
