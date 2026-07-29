# Troubleshooting

A short FAQ. For the full VM-over-SSH host-side troubleshooting table (permission-denied, host-key verification, missing template variables and more), see the [VM backup over SSH guide](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) on GitHub.

## Something is not wired up correctly

Open `/spike` in the web UI. The host integration check probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces. Start here before assuming a bug: a missing mount or an unreachable host shows up immediately.

## I cannot reach the web UI

BombVault serves HTTPS out of the box on port `3443` (self-signed certificate), so open `https://<your-unraid-ip>:3443`. Accept the self-signed certificate warning, or put BombVault behind a reverse proxy with your own certificate. If you run with `HTTP_ONLY=true`, it serves plain HTTP on port `3000` instead (intended for use behind a TLS-terminating proxy).

## I lost my APP_KEY

`APP_KEY` derives the restic repository password. Without it (and without the encryption-key recovery kit), encrypted backups cannot be recovered. This is why the Dashboard nags you to download the recovery kit. See [Off-site & recovery](offsite-recovery.md). Generate a key with `openssl rand -hex 32` and store it off the server before you rely on any backup.

## VM backup will not connect

VM backup talks to libvirt over SSH, never a mount.

- Confirm SSH is enabled on the host and BombVault's public key is authorized in `/root/.ssh/authorized_keys` (Settings, System, VM Backup over SSH shows the key and a **Test connection** button).
- On a custom `br0.x` network, set `LIBVIRT_HOST` to your Unraid LAN IP (the container cannot reach the host via `host.docker.internal` there). Enable **Settings, Docker, Host access to custom networks**.
- If you changed Unraid's SSH port, set `LIBVIRT_SSH_PORT` to match.
- Full step-by-step diagnosis (reachability test, VLAN routing, `Permission denied (publickey)`, `Host key verification failed`) is in the [VM backup over SSH guide](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## A live VM snapshot did not run

Live snapshots need the qemu guest agent installed in the VM and the disk on `/mnt/cache` (or `/mnt/diskX`), not `/mnt/user`. On a shut-off VM, live automatically falls back to graceful. A graceful backup shuts the VM down, backs up the disks, then restarts it, so it is always consistent.

## A backup failed with "repository is already locked"

This is usually an orphaned restic lock left behind when the container was updated or restarted mid-operation. BombVault detects a provably orphaned lock, force-clears it and retries once, automatically. If it persists, use **Settings, Integrity & maintenance, Unlock** for the affected domain to clear a stale lock by hand. A genuine problem still surfaces rather than being hidden.

## My off-site copy did not happen after a backup

Off-site replication is best-effort by design, so an off-site hiccup never fails the local backup. Check the off-site schedule for that domain (Settings, Schedules): a blank schedule replicates after every local backup, while a cadence ships less often. Use **Replicate now** on the Off-site tab for an on-demand run, and watch the replication indicator on the Dashboard.

## A restore aborted before it started

Before anything is stopped or removed, restore runs a pre-flight conflict check: it verifies the container's static IP and published host ports are free. If another container already holds one, it aborts with a clear, actionable message instead of leaving a half-finished restore. Free the conflicting port or IP, then retry.

## A plain export failed instead of writing a file

If age encryption is on (Settings) but no valid recipient is set, an export fails with a clear error instead of writing plaintext. Add a valid recipient (an age public key or an SSH public key), or turn encryption off if you intend the export to be plaintext. See [Features](features.md).

## The container keeps restarting or looks unhealthy

BombVault reports healthy/unhealthy from its own `/api/health`. An auto-heal tool (such as Autoheal) can restart it automatically if the engine ever wedges. Check the container log and the `/spike` report for the underlying cause.

## Still stuck?

- Read the full [Configuration](configuration.md) and [Off-site & recovery](offsite-recovery.md) pages.
- Ask on the [Unraid support thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Open a [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues).
