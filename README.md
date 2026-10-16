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
Containers, VMs, appdata, ZFS datasets, the flash drive and any folder you point it at. BombVault also backs up
<b>itself</b>, because a backup tool that cannot save its own skin is a hobby project. A restore puts
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
> 1. Open the [tester group](https://groups.google.com/g/arrowloop-testers), sign in with the Google account your phone uses and click **Join group**.
> 2. Open the [test page](https://play.google.com/apps/testing/bombvault.halleluja.design) and tap **Become a tester**.
> 3. Install BombVault from Google Play and keep it for 14 days. Using it for real helps most, and anything that goes wrong is welcome as an [issue](https://github.com/junkerderprovinz/bombvault/issues).

<!-- download-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <a href="https://unraid.net/community/apps?q=bombvault"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(0,0,841.9,245.3))" alt="Install from Unraid&#x27;s Community Applications" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/deploy/docker-compose.generic.yml"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(866,0,841.9,245.3))" alt="Download the docker-compose file" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/bombvault/"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(1732,0,841.9,245.3))" alt="Read the documentation" width="160" height="46.618"></a>
</p>
<br>
<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(2598,0,841.9,245.3))" alt="On Google Play soon" width="160" height="46.618">
  &nbsp;
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(3464,0,841.9,245.3))" alt="On F-Droid soon" width="160" height="46.618">
  &nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(4330,0,841.9,245.3))" alt="Download the Android app" width="160" height="46.618"></a>
  <br><sub>Always downloads the latest build</sub>
</p>
<br>
<p align="center">
  <a href="https://github.com/junkerderprovinz/parleyport"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(5196,0,841.9,245.3))" alt="Get ParleyPort, the relay for KnightLoader and BombVault" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault-widget"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(6062,0,841.9,245.3))" alt="Get the BombVault Widget for the Unraid dashboard" width="160" height="46.618"></a>
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
  <a href="https://buymeacoffee.com/junkerderprovinz"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(6928,0,841.9,245.3))" alt="Buy me a coffee" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(7794,0,841.9,245.3))" alt="PayPal" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/junkerderprovinz/"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(8660,0,841.9,245.3))" alt="Donate with crypto" width="160" height="46.618"></a>
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
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/dashboard.png" alt="BombVault overview: what needs you, the protection of each section, the storage locations and the history" width="100%">
  <br><em>Overview: a ring shows how much is protected, then what needs you, the protection of each section, how full each storage location is and the history with every run and the log. More cards wait under Customize.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/recovery.png" alt="BombVault Restore: one page to pick a section, an entry and a backup" width="100%">
  <br><em>Restore: one page for everything. Pick a section, search for an entry and choose a backup. One window restores every kind, next to the original by default. The Emergency wizard brings back a lost server from its recovery kit, through your group or with its typed key.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/containers.png" alt="BombVault Backups: one list of every container, VM, folder set, flash drive and ZFS dataset" width="100%">
  <br><em>Backups: everything that is backed up stands in one list, each row with its last fourteen runs and one status. Tiles pick a kind, Filters, Sort and Search sit above the list, and every entry has a page with its backups, its settings and its checks.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/settings.png" alt="BombVault Settings: fourteen tiles from General to Info" width="100%">
  <br><em>Settings as fourteen tiles (General · Look · Schedule · Storage locations · Restore · Notifications · Verify · Security · Instances · Pairing · Integrations · Apps · System · Info), with a search over all of them on / or Ctrl+K. Nothing here has a Save button; every change is written as you make it.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/receiver.png" alt="BombVault Instances: a paired server as receiver, with a received off-site copy and an independent check" width="100%">
  <br><em>Instances: paired servers take on roles for each other: receiver, fetcher, ZFS server and key keeper. A receiver keeps the other's off-site copy in an append-only server, lists what arrived from each source and when, and runs its own integrity check on this hardware instead of trusting the sender.</em>
</p>

<br>

<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/screenshots/android.png" alt="The Android app with its list of servers, the activity log of all of them and the pairing screen" width="100%">
  <br><em>The Android app: every server of your group on one list, their activity in one log, paired by twelve words.</em>
</p>

<br>

## 2. What it does

- **Backs up the whole server.** Containers with their appdata and definition, VMs with their disks, XML and NVRAM, the Unraid flash, any folder, ZFS datasets with their children, and BombVault's own settings. PostgreSQL, MySQL and MariaDB containers are dumped first. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Restores to a running state.** A container comes back in the Docker tab with its image, settings and data, a VM in the VM tab with its disks and NVRAM. By default the copy lands next to the original and the original stays as it is: a second, stopped container named `<name>-restored`, a VM's disks in a folder of their own, a folder beside the old one. When a restore replaces the original, BombVault first backs up what is there now unless you switch that off, and if that backup fails, the restore does not start. Every restore shows what it will change before it starts. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Copies off site, entry by entry.** Encrypted, to one or more storage locations that can be append-only. A storage location is set up once, from tiles that know S3 storage services, your own servers and every cloud drive rclone supports. A picture shows 3-2-1 for every section, and each entry says how many sites hold it and whether 3-2-1 is met. A ZFS dataset can also keep a replica as real datasets on a second ZFS host or a paired BombVault, sent incrementally with `zfs send`. [Off-site & recovery](https://junkerderprovinz.github.io/bombvault/offsite-recovery/), [ZFS replica](https://junkerderprovinz.github.io/bombvault/zfs-datasets/#replica)
- **Proves that restores work.** A restore check after each entry's first backup, scheduled drills, and a start test that runs a restored container in an isolated network. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Notices when a backup looks wrong.** Much more new data than usual, a source that shrank, a run that took far longer. When a source shrinks sharply, its old backups are kept until you acknowledge the finding. [Features](https://junkerderprovinz.github.io/bombvault/features/)
- **Survives losing the server.** A new install reads the old backups without the old APP_KEY in its template. The Emergency wizard takes the old key from the recovery kit, from a key keeper in your group or as typed text, tries it on every storage location, and restores settings, containers, VMs and folders. A key keeper is another server of your group that holds a sealed copy of the recovery kit. It cannot open the copy itself, only your recovery code does, and it hands the copy out only after someone there presses **Allow**. [Emergency wizard](https://junkerderprovinz.github.io/bombvault/offsite-recovery/#emergency), [Key keeper](https://junkerderprovinz.github.io/bombvault/offsite-recovery/#key-keeper)
- **Lets you back in without the password.** **Forgot password?** on the login page writes a code to the container log, and that code sets a new password. The reset also switches two-factor authentication off, removes every passkey, revokes API tokens, MCP keys and OAuth sign-ins and signs out every other session and paired phone. Anyone who can read the container log can do this, so give log access only to people you would trust with the password. [Login](https://junkerderprovinz.github.io/bombvault/configuration/#login)
- **Keeps everything in one list.** The sidebar has five entries: Overview, Anomalies, Backups, Restore and Settings. Backups shows every container, VM, folder set, the flash drive, every ZFS dataset and the self-backup in one list, and each entry has a page with its backups, its settings and its checks. Restore is one page for all of them, and Settings are fourteen tiles with a search. [Finding your way](https://junkerderprovinz.github.io/bombvault/getting-started/#navigation)
- **Fits into the rest of your setup.** Servers pair by twelve words and take on roles for each other: receiver, fetcher, ZFS server and key keeper. Receiver, fetcher and ZFS server start only after someone at the other server presses **Allow**. The Android app shows every server of the group and opens each one from any network, at home at its own address and elsewhere through the relay. AI assistants, scripts and Home Assistant read the status over MCP, an HTTP API and MQTT. [Instances and roles](https://junkerderprovinz.github.io/bombvault/offsite-recovery/#roles), [Android app](https://junkerderprovinz.github.io/bombvault/android/), [MCP server](https://junkerderprovinz.github.io/bombvault/mcp/), [API and integrations](https://junkerderprovinz.github.io/bombvault/api/)

It runs as one Docker container on Unraid, TrueNAS Scale or a plain Docker host and stores everything with [restic](https://restic.net). The interface comes in 42 languages and the documentation in 26. The idea of one-click backup with automatic reinstall comes from [**VolumeVault**](https://github.com/Darkdragon14/VolumeVault) by [@Darkdragon14](https://github.com/Darkdragon14); BombVault is a separate implementation (see [Credits](https://junkerderprovinz.github.io/bombvault/#credits)).

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

✅ yes · ⚠️ partly · ❌ no. The BombVault column is v10.0.0. The other tools were checked against their code and docs on 25 September 2026, the start-test row and Vault's cells for idle waiting, mDNS, Home Assistant and changed-block VM backups again on 28 September 2026 against Vault v2026.09.01, and the rows for placement, 3-2-1 and the Android app on 4 October 2026.

<br>

## 4. Getting started

On Unraid, install **BombVault** from [Community Applications](https://unraid.net/community/apps?q=bombvault). The one setting it needs is `APP_KEY`, a secret you make with `openssl rand -hex 32`. Keep a copy somewhere other than the server, because without it nobody can read the encrypted backups. Then open `https://<server-ip>:3443`. A new install opens on a welcome page with what BombVault found, and **Let's go** leads to three first steps: choose sections, add a storage location, run the first backup. After that it runs every night by itself. Then set a login password and download the recovery kit under **Settings, Restore**, or let another server of your group keep it as key keeper. If the install replaces a lost server, **Restore from a backup** on the welcome page opens the Restore page with the Emergency wizard.

On any other Docker host or on TrueNAS Scale, take [`deploy/docker-compose.generic.yml`](deploy/docker-compose.generic.yml), set `APP_KEY` and the Host Data volume, and run `docker compose up -d`. VM and ZFS backups reach the host over SSH, so BombVault's public key has to be added there once. [Getting started](https://junkerderprovinz.github.io/bombvault/getting-started/) and [Configuration](https://junkerderprovinz.github.io/bombvault/configuration/) cover the template, the mounts, every variable and the SSH setup.

- **Synology:** in Container Manager, create a project from the compose file (DSM 7.2 or newer).
- **Proxmox:** run it in a VM or an LXC container that has Docker.
- **MOS:** install BombVault from the [MOS Hub](https://mos-official.net/apps.html?search=BombVault). It runs there as `PLATFORM=generic` with Host Data on `/mnt/cache`.

### Android app

<!-- app-buttons: written by scripts/gen_download_buttons.py -->
<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(2598,0,841.9,245.3))" alt="On Google Play soon" width="160" height="46.618">
  &nbsp;
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(3464,0,841.9,245.3))" alt="On F-Droid soon" width="160" height="46.618">
  &nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(4330,0,841.9,245.3))" alt="Download the Android app" width="160" height="46.618"></a>
  <br><sub>Always downloads the latest build</sub>
</p>
<!-- /app-buttons -->

Every release has the app as `bombvault-android.apk`. On any server, open **Settings, Pairing**, choose **Show phrase** and scan the QR code with the app. It joins through the group's relay, adds every server of your group and shows what runs on all of them. Each server opens already signed in, at home at its own address and from any other network through the relay, where the server signs every answer. A paired phone reaches what the app shows; keys, settings and deleting backups still need the password at the server itself. Google Play has it in a closed test for now, and F-Droid follows. The [Android app](https://junkerderprovinz.github.io/bombvault/android/) page has the details, and the [privacy policy](android/PRIVACY.md) lists what the app stores and sends.

<br>

## 5. Documentation

The [documentation](https://junkerderprovinz.github.io/bombvault/), in 26 languages, has what this page leaves out:

- [Getting started](https://junkerderprovinz.github.io/bombvault/getting-started/): requirements, the Unraid template, other Docker hosts and TrueNAS Scale, the welcome page and the first backup, the sidebar and the settings tiles, and building from source
- [Android app](https://junkerderprovinz.github.io/bombvault/android/): pairing by QR code, opening a server from any network through the relay, what a paired phone may do, servers outside a group, settings and downloads
- [Features](https://junkerderprovinz.github.io/bombvault/features/): everything BombVault backs up, restores, checks and reports
- [Configuration](https://junkerderprovinz.github.io/bombvault/configuration/): environment variables, mounts, the security model with the login and the password reset, phones and group members, the APP_KEY and a taken-over backup key, VM backup over SSH and the off-site setup
- [Off-site & recovery](https://junkerderprovinz.github.io/bombvault/offsite-recovery/): storage locations and moving or removing them, placement per entry, append-only copies, tamper tests, pairing and the four roles, the Emergency wizard, the key keeper and the recovery kit
- [ZFS datasets](https://junkerderprovinz.github.io/bombvault/zfs-datasets/): entries and child datasets, restores, the safety snapshot and the replica on a second ZFS host or a paired BombVault
- [MCP server](https://junkerderprovinz.github.io/bombvault/mcp/): connecting AI assistants, keys and limits
- [API and integrations](https://junkerderprovinz.github.io/bombvault/api/): the HTTP API, Home Assistant and mDNS
- [Troubleshooting](https://junkerderprovinz.github.io/bombvault/troubleshooting/): failed backups, locks, a lost APP_KEY or password, a phone on the login page, VM connections and a container that restarts
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
  <a href="https://buymeacoffee.com/junkerderprovinz"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(6928,0,841.9,245.3))" alt="Buy me a coffee" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://www.paypal.com/donate/?hosted_button_id=76FVV52TKXTUS"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(7794,0,841.9,245.3))" alt="PayPal" width="160" height="46.618"></a>
  &nbsp;
  <a href="https://junkerderprovinz.github.io/junkerderprovinz/"><img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/download-buttons/buttons.svg?v=fe07e1465559#svgView(viewBox(8660,0,841.9,245.3))" alt="Donate with crypto" width="160" height="46.618"></a>
</p>
<!-- /give-buttons -->
