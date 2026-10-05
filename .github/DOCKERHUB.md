<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner.png" alt="BombVault" width="100%">
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/pulls/junkerderprovinz/bombvault?style=for-the-badge&logo=docker&logoColor=white&label=Pulls&color=1d99f3" alt="Docker Pulls" height="36"></a>&nbsp;
  <a href="https://hub.docker.com/r/junkerderprovinz/bombvault"><img src="https://img.shields.io/docker/image-size/junkerderprovinz/bombvault/latest?style=for-the-badge&logo=docker&logoColor=white&label=Size&color=1d99f3" alt="Image Size" height="36"></a>&nbsp;
  <img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36">&nbsp;
  <a href="https://restic.net"><img src="https://img.shields.io/badge/Engine-restic-CE4844?style=for-the-badge&logoColor=white" alt="restic" height="36"></a>&nbsp;
  <a href="https://unraid.net/community/apps?q=bombvault"><img src="https://img.shields.io/badge/Unraid-Template-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue?style=for-the-badge&logo=gnu&logoColor=white" alt="License: AGPL-3.0" height="36"></a>&nbsp;
  <a href="https://junkerderprovinz.github.io/bombvault/"><img src="https://img.shields.io/badge/Docs-online-526CFE?style=for-the-badge&logo=materialformkdocs&logoColor=white" alt="Documentation" height="36"></a>
</p>

<p align="center">
Your Unraid server, <b>sealed in a vault</b>. Drop a backup. Detonate a restore.<br>
<br>
Containers, VMs, appdata, the flash drive and any folder you point it at. BombVault also backs up
<b>itself</b>. One click puts it all back: containers reappear in the <b>Docker tab</b>, VMs in the
<b>VM tab</b>, already configured. Built on <a href="https://restic.net">restic</a>, so every snapshot
is deduplicated, incremental and encrypted before it leaves the box.
</p>

## What it does

- Backs up containers with their appdata and definition, VMs with their disks, XML and NVRAM, the Unraid flash, any folder, ZFS datasets and its own settings. PostgreSQL, MySQL and MariaDB containers are dumped first.
- Restores a container to the Docker tab and a VM to the VM tab, running as before, and shows what a restore will change before it starts.
- Copies each item off site to one or more encrypted targets, which can be append-only, and says how many sites hold it and whether 3-2-1 is met.
- Proves that restores work, with a check after each item's first backup, scheduled drills and a start test that runs a restored container in an isolated network.
- Warns when a backup looks wrong, such as much more new data than usual or a source that shrank. When a source shrinks sharply, its old backups stay until you acknowledge the finding.
- Pairs several servers by twelve words and talks to AI assistants, scripts and Home Assistant over MCP, an HTTP API and MQTT.

The [Android app](https://junkerderprovinz.github.io/bombvault/android/) shows every server of your group on one list, paired by a QR code.

## Getting started

On Unraid, install **BombVault** from [Community Applications](https://unraid.net/community/apps?q=bombvault). The one setting it needs is `APP_KEY`, a secret you make with `openssl rand -hex 32`. Keep a copy somewhere other than the server, because without it nobody can read the encrypted backups. Then open `https://<server-ip>:3443`.

On any other Docker host:

```sh
docker run -d --name bombvault \
  --hostname bombvault \
  --restart unless-stopped \
  -p 3443:3443 \
  -v /path/to/config:/config \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/appdata:/host/user \
  -e APP_KEY=<output of openssl rand -hex 32> \
  -e PLATFORM=generic \
  -e HOST_SOURCE_ROOT=/srv/appdata \
  junkerderprovinz/bombvault:latest
```

Replace `/srv/appdata` in both places with the host folder that holds your containers' data. The same setup as a Compose file is [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml). BombVault has root-level control of the host through the Docker socket, so keep it on a trusted network.

`latest` is the newest release. Each release is also tagged with its version, such as `9.7.0`, and with `9.7` and `9`. `edge` follows the main branch.

## More

- [Documentation](https://junkerderprovinz.github.io/bombvault/): getting started, configuration, VM and ZFS backups over SSH, off-site copies and recovery
- [Source and Android app](https://github.com/junkerderprovinz/bombvault)

Questions? Ask in [Discussions](https://github.com/junkerderprovinz/bombvault/discussions/categories/q-a) or the [support thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/). Bugs and ideas go to [GitHub issues](https://github.com/junkerderprovinz/bombvault/issues).

BombVault is free, with no accounts, no telemetry and no ads. If it has earned a place on your server, a coffee helps keep it going.

<a href="https://buymeacoffee.com/junkerderprovinz"><img src="https://raw.githubusercontent.com/junkerderprovinz/junkerderprovinz/main/donate/buttons/button-buy-me-a-coffee-live.svg" alt="Buy me a coffee" width="160"></a>
