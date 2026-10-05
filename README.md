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
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue?style=for-the-badge&logo=gnu&logoColor=white" alt="License: AGPL-3.0" height="36"></a>&nbsp;
  <a href="https://junkerderprovinz.github.io/bombvault/"><img src="https://img.shields.io/badge/Docs-online-526CFE?style=for-the-badge&logo=materialformkdocs&logoColor=white" alt="Documentation" height="36"></a>
</p>

<br>

<p align="center">
Your Unraid server, <b>sealed in a vault</b>. Drop a backup. Detonate a restore.<br>
<br>
Containers, VMs, appdata, the flash drive and any folder you point it at. BombVault also backs up
<b>itself</b>, because a backup tool that cannot save its own skin is a hobby project. One click puts
it all back: containers reappear in the <b>Docker tab</b>, VMs in the <b>VM tab</b>, already configured.
No reinstall, no rebuild, no evening lost.<br>
<br>
Built on <a href="https://restic.net">restic</a>, so every snapshot is deduplicated, incremental and
encrypted before it leaves the box. Off-site copies can be <b>append-only</b>, which is a polite way of
saying ransomware is welcome to knock.
</p>

<br>

<p align="center">
  <a href="https://play.google.com/apps/testing/bombvault.halleluja.design"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/testers.png" alt="Android testers wanted: join the Google Play closed test" width="100%"></a>
</p>

> [!IMPORTANT]
> **Android testers wanted.** Google Play only lists an app from a new developer account after at least 12 testers have kept it installed for 14 days. If you have an Android phone:
>
> 1. Join the [tester group](https://groups.google.com/g/arrowloop-testers).
> 2. Open the [test page](https://play.google.com/apps/testing/bombvault.halleluja.design) and tap **Become a tester**.
> 3. Install BombVault from Google Play and keep it for 14 days. Using it for real helps most, and anything that goes wrong is welcome as an [issue](https://github.com/junkerderprovinz/bombvault/issues).

<!-- download-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <a href="https://unraid.net/community/apps?q=bombvault"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(0,0,841.9,245.3))" alt="Install from Unraid&#x27;s Community Applications" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(866,0,841.9,245.3))" alt="Run it with Docker" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/bombvault/"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(1732,0,841.9,245.3))" alt="Read the documentation" width="160" height="46.618"></a>
</p>
<br>
<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(2598,0,841.9,245.3))" alt="On Google Play soon" width="160" height="46.618">
  &nbsp;
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(3464,0,841.9,245.3))" alt="On F-Droid soon" width="160" height="46.618">
  &nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(4330,0,841.9,245.3))" alt="Download the Android app" width="160" height="46.618"></a>
  <br><sub>Always downloads the latest build</sub>
</p>
<br>
<p align="center">
  <a href="https://github.com/junkerderprovinz/parleyport"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(5196,0,841.9,245.3))" alt="Get ParleyPort, the relay for KnightLoader and BombVault" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault-widget"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(6062,0,841.9,245.3))" alt="Get the BombVault Widget for the Unraid dashboard" width="160" height="46.618"></a>
</p>
<!-- /download-buttons -->

<br>

<p align="center">
A one-knight job: I build it, keep it running, work through the issues and add what people ask for, until nothing is missing. It is free, with no accounts, no telemetry, no ads and no paid tier. No asterisk anywhere. Nothing readable ever leaves your own walls. Forged on evenings and weekends, with heart and stubbornness.
</p>

<p align="center">
If it has earned a place on your server or computer, toss a coin to your knight: it helps cover the costs and keeps the project alive. It also makes this knight's heart beat a little faster. Three ways below, whichever suits you.
</p>

<br>

<!-- give-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(6928,0,841.9,245.3))" alt="Buy me a coffee" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(7794,0,841.9,245.3))" alt="PayPal" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/junkerderprovinz/"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(8660,0,841.9,245.3))" alt="Donate with crypto" width="160" height="46.618"></a>
</p>
<!-- /give-buttons -->

<br>

## Table of Contents

1. [Screenshots](#1-screenshots)
2. [What it does](#2-what-it-does)
3. [How it compares](#3-how-it-compares)
4. [Getting started](#4-getting-started)
5. [Documentation](#5-documentation)
6. [How AI is used here](#6-how-ai-is-used-here)
7. [Support this project](#7-support-this-project)

<br>

## 1. Screenshots

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/dashboard.png" alt="BombVault dashboard: recovery point, next backup, last result and the live activity log" width="100%">
  <br><em>Dashboard: the recovery point, the next backup and the last result sit above a live activity log. The log also shows the off-site copy and the tamper test that proves the far side refuses a delete.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/recovery.png" alt="BombVault Recovery: the guided disaster-recovery flow onto a fresh install" width="100%">
  <br><em>Recovery: a guided flow for a fresh install. Check that BombVault can read your backups, restore its own settings, then attach your container, VM and flash backups and restore them.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/containers.png" alt="BombVault Containers: each container with its schedule switch, placement and restore check" width="100%">
  <br><em>Containers: each container has its own schedule switch, a choice of local, off-site or both, a one-click backup and a restore check. Filters and bulk include or exclude sit above the list.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/settings.png" alt="BombVault Settings: configuration organised into pages for domains, paths, schedules, off-site, notifications and integrity" width="100%">
  <br><em>Settings, organised into pages (General · Look · Storage · Retention · Schedules · Containers · Off-site · Cloud access · Notifications · Integrity · Security · Pairing · Integrations · System). General turns each backup domain on or off and holds the language and quiet toasts; Look holds the theme, colours, corners and animation. Nothing here has a Save button; every change is written as you make it.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/receiver.png" alt="BombVault Receiver: a received off-site copy with its snapshots by source and an independent check" width="100%">
  <br><em>Receiver: the other end of an off-site copy, read-only. It lists what arrived from each source and when, and runs its own integrity check on this hardware instead of trusting the sender.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/android.png" alt="The Android app with its list of servers, the activity log of all of them and the pairing screen" width="100%">
  <br><em>The Android app: every server of your group on one list, their activity in one log, paired by twelve words.</em>
</p>

<br>

## 2. What it does

- **Backs up the whole server.** Containers with their appdata and definition, VMs with their disks, XML and NVRAM, the Unraid flash, any folder, ZFS datasets with their children, and BombVault's own settings. PostgreSQL, MySQL and MariaDB containers are dumped first. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Restores to a running state.** A restored container comes back in the Docker tab with its image, settings and data, a VM in the VM tab with its disks and NVRAM. Every restore shows what it will change before it starts. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Copies off site, item by item.** Encrypted, to one or more targets that can be append-only. A destination is set up once, with a wizard that knows S3 storage services, your own servers and every cloud drive rclone supports. Each container, VM and folder set then lights the places that get its backups, and its card says how many sites hold it and whether 3-2-1 is met. [Off-site & recovery](https://junkerderprovinz.github.io/bombvault/offsite-recovery/)
- **Proves that restores work.** A restore check after each item's first backup, scheduled drills, and a start test that runs a restored container in an isolated network. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Notices when a backup looks wrong.** Much more new data than usual, a source that shrank, a run that took far longer. When a source shrinks sharply, its old backups are kept until you acknowledge the finding. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Fits into the rest of your setup.** Several servers pair by twelve words, an Android app shows them all, and AI assistants, scripts and Home Assistant read the status over MCP, an HTTP API and MQTT. [Android app](https://junkerderprovinz.github.io/bombvault/android/), [MCP server](https://junkerderprovinz.github.io/bombvault/mcp/), [API and integrations](https://junkerderprovinz.github.io/bombvault/api/)

It runs as one Docker container on Unraid, TrueNAS Scale or a plain Docker host and stores everything with [restic](https://restic.net). The idea of one-click backup with automatic reinstall comes from [**VolumeVault**](https://github.com/Darkdragon14/VolumeVault) by [@Darkdragon14](https://github.com/Darkdragon14); BombVault is a separate implementation (see [Credits](https://junkerderprovinz.github.io/bombvault/#credits)).

<br>

## 3. How it compares

On Unraid, backups usually run through [**Appdata.Backup**](https://github.com/Commifreak/unraid-appdata.backup), a CA plugin that archives appdata folders, or through a general engine such as [Duplicati](https://duplicati.com), [Kopia](https://kopia.io) or [BorgBackup](https://borgbackup.readthedocs.io). They save files well, but a restore gives you files back, not a running container or VM.

The closest counterpart is [**Vault**](https://github.com/ruaan-deysel/vault) by [@ruaan-deysel](https://github.com/ruaan-deysel), a native Unraid plugin built on the same idea: it recreates containers and re-defines VMs on restore, and it has a good deal of what BombVault has, down to changed-block VM backups and Home Assistant. BombVault is ahead on getting data back: restic reads its backups without BombVault, the off-site copy can be append-only, and restores are tested for real, up to starting a restored container in isolation. Worth a look.

| | **BombVault** | Vault (plugin) | Appdata.Backup (CA) | Duplicati | Kopia | BorgBackup |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| Restore brings a container back whole (image, env, ports, labels) | ✅ | ✅ | ⚠️ files and XML | ❌ | ❌ | ❌ |
| Restore re-defines a VM, not only its disks | ✅ | ✅ | ⚠️ XML only | ❌ | ❌ | ❌ |
| VM backups read only changed blocks | ✅ qcow2 | ✅ qcow2 | ❌ | ❌ | ❌ | ❌ |
| Database dumps for recognised database containers | ✅ | ✅ | ❌ | ❌ | ❌ | ⚠️ via Borgmatic |
| Installed Unraid plugins | ✅ one by one from the flash backup | ✅ | ⚠️ in flash backup | ❌ | ❌ | ❌ |
| ZFS datasets as a source | ✅ | ✅ | ❌ | ❌ | ⚠️ via action scripts | ⚠️ via Borgmatic |
| Deduplication | ✅ | ✅ opt-in | ❌ | ✅ fixed blocks | ✅ | ✅ |
| Client-side encryption | ✅ on by default | ✅ opt-in | ❌ | ✅ | ✅ | ✅ |
| Backups readable with a standard open-source CLI | ✅ restic | ⚠️ not with dedup | ✅ tar | ⚠️ Python script | ✅ kopia | ✅ borg |
| Append-only or immutable off-site copy | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ |
| Scheduled backup waits until the app is idle | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Scheduled test restores, not only a checksum read | ✅ | ❌ | ❌ | ❌ | ❌ | ⚠️ via Borgmatic |
| Start test: a restored container is started in isolation and checked | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Off-site upload slows down while a media server streams | ✅ | ✅ any outside traffic | ❌ | ❌ | ❌ | ❌ |
| Several off-site targets, each with its own credentials | ✅ | ⚠️ one per job | ❌ | ✅ | ⚠️ CLI sync | ⚠️ via Borgmatic |
| Each item chooses which off-site targets get a copy | ✅ | ⚠️ a second job per target | ❌ | ⚠️ a job per destination | ❌ | ⚠️ via Borgmatic |
| Each item chooses local only, local and off-site, or off-site only | ✅ | ⚠️ by the jobs it is in | ❌ | ⚠️ a job per destination | ❌ | ⚠️ via Borgmatic |
| Shows how many sites hold each item and whether 3-2-1 is met | ✅ | ⚠️ for the whole server, not per item | ❌ | ❌ | ❌ | ❌ |
| Pre/post-backup hooks | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ via Borgmatic |
| Live progress and cancel, backup and restore | ✅ | ⚠️ no restore cancel | ⚠️ log, no percentage | ✅ | ⚠️ [no restore percentage](https://github.com/kopia/kopia/issues/3609) | ⚠️ CLI or Vorta |
| Notifications | ✅ SMTP, Matrix, Apprise, more | ✅ Discord, Unraid | ✅ Unraid's agents | ✅ email, Telegram, HTTP | ✅ email, Pushover, webhook | ⚠️ via Borgmatic |
| Anomaly detection (size, duration, shrink) | ✅ | ✅ | ❌ | ⚠️ paid Console | ❌ | ❌ |
| AI assistant access (MCP) | ✅ | ✅ | ❌ | ⚠️ third party | ❌ | ❌ |
| Documented HTTP API for scripts, with its own tokens | ✅ | ✅ | ❌ | ⚠️ undocumented | ⚠️ undocumented | ❌ |
| Home Assistant integration | ✅ over MQTT | ✅ | ❌ | ⚠️ third party | ⚠️ third party | ⚠️ third party |
| Announces itself on the network (mDNS) | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
| Backs up desktops and laptops | ❌ | ❌ | ❌ | ✅ | ✅ | ⚠️ Windows experimental |
| Runs outside Unraid | ✅ | ⚠️ replica only | ❌ | ✅ | ✅ | ✅ |
| In Unraid Community Applications | ✅ | ✅ | ✅ | ✅ community template | ✅ community template | ✅ community template |
| Android app | ✅ APK, Google Play in closed test | ❌ | ❌ | ❌ | ❌ | ❌ |
| Track record | ⚠️ since 2026, one maintainer | ⚠️ since 2026, one maintainer | ⚠️ since 2023, feature-frozen | ✅ since 2008 | ✅ since 2019 | ✅ since 2015 |

✅ yes · ⚠️ partly · ❌ no. The BombVault column is v9.8.0. The other tools were checked against their code and docs on 25 September 2026, the start-test row and Vault's cells for idle waiting, mDNS, Home Assistant and changed-block VM backups again on 28 September 2026 against Vault v2026.09.01, and the rows for placement, 3-2-1 and the Android app on 4 October 2026.

<br>

## 4. Getting started

On Unraid, install **BombVault** from [Community Applications](https://unraid.net/community/apps?q=bombvault). The one setting it needs is `APP_KEY`, a secret you make with `openssl rand -hex 32`. Keep a copy somewhere other than the server, because without it nobody can read the encrypted backups. Then open `https://<server-ip>:3443`, switch on the kinds of backup you want under Settings, and press **Back up now** on a container.

On any other Docker host or on TrueNAS Scale, take [`deploy/docker-compose.generic.yml`](deploy/docker-compose.generic.yml), set `APP_KEY` and the Host Data volume, and run `docker compose up -d`. VM and ZFS backups reach the host over SSH, so BombVault's public key has to be added there once. [Getting started](https://junkerderprovinz.github.io/bombvault/getting-started/) and [Configuration](https://junkerderprovinz.github.io/bombvault/configuration/) cover the template, the mounts, every variable and the SSH setup.

### Android app

<!-- app-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(2598,0,841.9,245.3))" alt="On Google Play soon" width="160" height="46.618">
  &nbsp;
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(3464,0,841.9,245.3))" alt="On F-Droid soon" width="160" height="46.618">
  &nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(4330,0,841.9,245.3))" alt="Download the Android app" width="160" height="46.618"></a>
  <br><sub>Always downloads the latest build</sub>
</p>
<!-- /app-buttons -->

Every release has the app as `bombvault-android.apk`. On any server, open **Settings, Pairing, Show phrase** and scan the QR code with the app: it adds every server of your group, shows what runs on all of them, also away from home over the relay, and opens each one already signed in. Google Play has it in a closed test for now, and F-Droid follows. The [Android app](https://junkerderprovinz.github.io/bombvault/android/) page has the details, and the [privacy policy](android/PRIVACY.md) lists what the app stores and sends.

<br>

## 5. Documentation

The [documentation](https://junkerderprovinz.github.io/bombvault/), in 26 languages, has what this page leaves out:

- [Getting started](https://junkerderprovinz.github.io/bombvault/getting-started/): requirements, the Unraid template, other Docker hosts and TrueNAS Scale, the first backup and building from source
- [Android app](https://junkerderprovinz.github.io/bombvault/android/): pairing by QR code, servers outside a group, settings and downloads
- [Features](https://junkerderprovinz.github.io/bombvault/features/): everything BombVault backs up, restores, checks and reports, and the companion apps
- [Configuration](https://junkerderprovinz.github.io/bombvault/configuration/): environment variables, mounts, the security model, VM backup over SSH and the off-site setup
- [Off-site & recovery](https://junkerderprovinz.github.io/bombvault/offsite-recovery/): destinations, placement per item, append-only copies, tamper tests, pairing, the recovery kit and guided recovery
- [ZFS datasets](https://junkerderprovinz.github.io/bombvault/zfs-datasets/): items and child datasets, restores and the safety snapshot
- [MCP server](https://junkerderprovinz.github.io/bombvault/mcp/): connecting AI assistants, keys and limits
- [API and integrations](https://junkerderprovinz.github.io/bombvault/api/): the HTTP API, Home Assistant and mDNS
- [Troubleshooting](https://junkerderprovinz.github.io/bombvault/troubleshooting/): failed backups, locks, VM connections and a container that restarts
- [VM backup over SSH](docs/vm-backup-ssh-setup.md): the full SSH and networking guide, including TrueNAS Scale

<br>

## 6. How AI is used here

One knight builds this, and AI is one of the tools I work with, the same way I work with an editor or a compiler. It helps me write code and documentation and it checks my work, and that saves me a good many evenings. It does not make the decisions, though. I read and understand everything before it ships, and if something here breaks, that is on me and not on the tool.

You do not have to take my word for it. The code is open and every release note is written by hand. The issue tracker shows how problems actually get handled, including the ones I got wrong the first time. If you find something that is not right, open an issue and I will look at it.

<br>

## 7. Support this project

Questions? Ask in [Discussions](https://github.com/junkerderprovinz/bombvault/discussions/categories/q-a) or check the [support thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/). Bugs, ideas or feature requests? Please [open a GitHub issue](https://github.com/junkerderprovinz/bombvault/issues).

A one-knight job: I build it, keep it running, work through the issues and add what people ask for, until nothing is missing. It is free, with no accounts, no telemetry, no ads and no paid tier. No asterisk anywhere. Nothing readable ever leaves your own walls. Forged on evenings and weekends, with heart and stubbornness.

If it has earned a place on your server or computer, toss a coin to your knight: it helps cover the costs and keeps the project alive. It also makes this knight's heart beat a little faster. Three ways below, whichever suits you.

<!-- give-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <a href="https://buymeacoffee.com/junkerderprovinz"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(6928,0,841.9,245.3))" alt="Buy me a coffee" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(7794,0,841.9,245.3))" alt="PayPal" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/junkerderprovinz/"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=339b197223f4#svgView(viewBox(8660,0,841.9,245.3))" alt="Donate with crypto" width="160" height="46.618"></a>
</p>
<!-- /give-buttons -->
