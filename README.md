<h1 align="center">BombVault</h1>

<!-- Banner placeholder — .github/assets/bombvault-banner.png does not exist yet (P0 stub). -->
<p align="center">
  <img src="https://raw.githubusercontent.com/junkerderprovinz/bombvault/main/.github/assets/bombvault-banner.png" alt="BombVault" width="100%">
</p>

<p align="center">
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/build.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/build.yml?branch=main&label=Build&style=for-the-badge&logo=githubactions&logoColor=white" alt="Build" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/actions/workflows/lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/junkerderprovinz/bombvault/lint.yml?branch=main&label=Lint&style=for-the-badge&logo=githubactions&logoColor=white" alt="Lint" height="36"></a>&nbsp;
  <a href="https://github.com/junkerderprovinz/bombvault/pkgs/container/bombvault"><img src="https://img.shields.io/badge/Arch-amd64%20%7C%20arm64-success?style=for-the-badge&logo=linux&logoColor=white" alt="Arch" height="36"></a>&nbsp;
  <a href="https://nextjs.org"><img src="https://img.shields.io/badge/Next.js-000000?style=for-the-badge&logo=nextdotjs&logoColor=white" alt="Next.js" height="36"></a>&nbsp;
  <a href="https://restic.net"><img src="https://img.shields.io/badge/Engine-restic-CE4844?style=for-the-badge&logo=restic&logoColor=white" alt="restic" height="36"></a>&nbsp;
  <a href="https://unraid.net"><img src="https://img.shields.io/badge/Unraid-Template-f15a2c?style=for-the-badge&logo=unraid&logoColor=white" alt="Unraid" height="36"></a>&nbsp;
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License" height="36"></a>
</p>

<br>

<p align="center">
BombVault is a self-hosted, Unraid-native web app for <b>backup and full disaster recovery</b>
of Docker containers and KVM/libvirt VMs — app-data, container/VM definitions, vdisks and the
Unraid flash config — restoring them so containers reappear in the Docker tab and VMs in the
VM tab with no manual steps. Powered by <a href="https://restic.net">restic</a>.
</p>

<br>

## Table of Contents

1. [Status](#1-status)
2. [What is this?](#2-what-is-this)
3. [Security / trust model](#3-security--trust-model)
4. [Host integration spike](#4-host-integration-spike)
5. [Development](#5-development)

<br>

## 1. Status

**P1 — Container backup & restore (one-click).** On top of the Next.js + SQLite
foundation (thin restic adapter, AES-256-GCM secrets util, host-integration spike),
BombVault now backs up and restores Docker containers with a single click: the backup
destination is the template-mounted repo and the restic password is **derived from
`APP_KEY`** — no destination to configure, no password to type. There is **no
authentication yet** — see [Security / trust model](#3-security--trust-model).
VMs, flash config, off-site backends and scheduling are the next phases.

<br>

## 2. What is this?

A single Docker container that backs up and restores Docker containers and KVM/libvirt VMs,
delegating the heavy lifting to restic (deduplicated, incremental, always-encrypted). See
`docs/superpowers/specs/2026-06-07-bombvault-design.md` for the full design.

<br>

## 3. Security / trust model

> [!WARNING]
> **BombVault has no built-in authentication.** It is a trusted-LAN tool that holds
> **root-equivalent control of the host** via the Docker and libvirt sockets: it can
> stop, remove and recreate containers and VMs, and it reads/writes appdata and the
> Unraid flash config. Anyone who can reach its web UI effectively has root on the host.

Run BombVault **only on a trusted, non-exposed network** — never publish it directly to
the internet. If you need remote access, put it **behind a reverse proxy that adds
authentication** (and TLS). An optional built-in authentication layer is planned before
any public release.

<br>

## 4. Host integration spike

Real Docker, libvirt and Unraid behavior cannot be tested in CI (no KVM, no Unraid on runners).
The **spike is the real-host validation step**: run it inside the container on your actual host to
confirm every mount and CLI is reachable. It degrades gracefully — a failed check is a finding,
never a crash.

- Web: open **`/spike`**.
- CLI: `npm run spike` (or `docker exec bombvault npm run spike`).

It probes: docker socket, libvirt (`virsh`), appdata readability, `restic`, `qemu-img`, `rclone`.

<br>

## 5. Development

```bash
cp .env.example .env        # set APP_KEY (openssl rand -hex 32)
npm install
npm test                    # unit + restic integration tests
npm run dev                 # https://localhost:3443 (self-signed)
```
