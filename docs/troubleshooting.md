# Troubleshooting

A short FAQ. For the full VM-over-SSH host-side troubleshooting table (permission-denied, host-key verification, missing template variables and more), see the [VM backup over SSH guide](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) on GitHub.

## Something is not wired up correctly

Add the **Host Integration** card to the overview under **Customize**. The host integration check probes every mount and CLI (Docker socket, libvirt, restic, qemu-img, rclone) and reports any missing pieces, and **Check now** runs it again. Start here before assuming a bug: a missing mount or an unreachable host shows up immediately.

## I cannot reach the web UI

BombVault serves HTTPS out of the box on port `3443` (self-signed certificate), so open `https://<your-unraid-ip>:3443`. Accept the self-signed certificate warning, or put BombVault behind a reverse proxy with your own certificate. If you run with `HTTP_ONLY=true`, it serves plain HTTP on port `3000` instead (intended for use behind a TLS-terminating proxy).

## I lost my APP_KEY

`APP_KEY` derives the restic repository password. Without it, without the recovery kit and without a key deposit in your group, encrypted backups cannot be recovered. This is why the overview shows **Save your recovery kit** until you have downloaded it. See [Off-site & recovery](offsite-recovery.md#recovery-kit). Generate a key with `openssl rand -hex 32` and store it off the server before you rely on any backup.

## I forgot the login password

Choose **Forgot password?** on the login page. BombVault writes a code of 12 characters to its container log (on Unraid: Docker tab, BombVault icon, **Logs**). Type it with a new password within 10 minutes. The reset also turns two-factor authentication off, removes every passkey, revokes every API token, MCP key and OAuth sign-in, ends every other session and switches phone pairing off. Add your passkeys again under **Settings, Security**, switch phone pairing back on there, and create new tokens and keys where you still need them. See [Security model](configuration.md#login).

## Backups stopped and say the key can no longer be opened

This server took over another server's backup key in the Emergency wizard, and `APP_KEY` was changed afterwards. The taken-over key is sealed with the old APP_KEY, so BombVault cannot open it and backs nothing up rather than write with the wrong key. **Needs you** on the overview names it. Set APP_KEY back to what it was, or upload the recovery kit again on the Restore page under **Emergency**.

## A phone lands on the login page

A paired phone gets its session over the group only while **Phones can pair with the group's twelve words** is on under **Settings, Security**. A password reset turns it off. An app older than the server also gets no session until it is updated. A server added by its address always asks for the password. A phone removed from the Instances page comes back switched off when its app opens again; switch on **Poll this instance** under **Monitoring** in its details to let it in.

## VM backup will not connect

VM backup talks to libvirt over SSH, never a mount.

- Confirm SSH is enabled on the host and BombVault's public key is authorized in `/root/.ssh/authorized_keys` (Settings, Integrations, Host SSH shows the key and a **Test connection** button).
- On a custom `br0.x` network, set `LIBVIRT_HOST` to your Unraid LAN IP (the container cannot reach the host via `host.docker.internal` there). Enable **Settings, Docker, Host access to custom networks**.
- If you changed Unraid's SSH port, set `LIBVIRT_SSH_PORT` to match.
- On TrueNAS, set `LIBVIRT_URI` with TrueNAS's own libvirt socket, give the app the `SETPCAP` capability and add the key to root under **Credentials, Users** (see [TrueNAS SCALE](getting-started.md#truenas)).
- Full step-by-step diagnosis (reachability test, VLAN routing, `Permission denied (publickey)`, `Host key verification failed`) is in the [VM backup over SSH guide](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## A live VM snapshot did not run

Live snapshots need the qemu guest agent installed in the VM and the disk on `/mnt/cache` (or `/mnt/diskX`), not `/mnt/user`. On a shut-off VM, live automatically falls back to graceful. A graceful backup shuts the VM down, backs up the disks, then restarts it, so it is always consistent. It waits up to ten minutes for the guest to shut down and never forces it off, so a Windows guest installing updates is not cut off halfway. A guest that still runs after that is left alone, and that backup fails with the reason.

## A backup failed with "repository is already locked"

This is usually an orphaned restic lock left behind when the container was updated or restarted mid-operation. BombVault detects a provably orphaned lock, force-clears it and retries once, automatically. If it persists, open the **Check backups** card under **Settings, Verify** and choose **Unlock** under **More** in the affected section's row to clear a stale lock by hand. A genuine problem still surfaces rather than being hidden. After a restart BombVault waits until such a lock has gone ten minutes without a refresh. A restic that is still running, for example in a second BombVault on the same repository, refreshes its lock every five minutes.

## My off-site copy did not happen after a backup

Off-site replication is best-effort by design, so an off-site hiccup never fails the local backup. Check when the storage location copies: its **Copy** card says **After every backup** or names a plan of its own, and **Settings, Schedule, Off-site copies per section** can give a section its own plan. Use **Replicate now** in the section's **Off-site copy** card under **Settings, Storage locations** for an on-demand run, and watch the status head of the overview, which says **Off-site copy running** while it runs. A failed copy shows under **Needs you**.

## A rest-server's certificate is not trusted

restic refuses a certificate it does not know, and the error says where to put it. Copy the rest-server's self-signed certificate, or the one of the authority that signed it, as a PEM file into the `certs` folder of BombVault's data folder (`/config/certs` in the container, `appdata/bombvault/certs` on Unraid) and test again. No restart is needed.

## A backup says data outside Host Data was not backed up {#data-outside-host-data}

The container keeps data in a folder BombVault cannot read through Host Data. The backup saves the definition and every folder it can reach, turns its line in the activity log orange and names the folders it left out. Two cases are common: an ixVolume on TrueNAS under `/mnt/.ix-apps`, and an anonymous Docker volume under `/var/lib/docker/volumes`, which Docker creates by itself for a folder the image declares and nobody mapped, such as the data of a monitoring agent or the cache of a media server.

- **You want that data backed up.** Map the folder to a host path below Host Data, for example a folder in your appdata share, and the next backup includes it.
- **You do not need it.** Add the folder's path inside the container, such as `/var/lib/beszel-agent` or `/cache`, as a line in the **Exclude patterns** card on the container's **Settings** tab. From the next backup on, the warning leaves that folder out, and the preview under the field says why. A line for a folder above it works too, a bare name does not.

## A large backup was killed for running out of memory

Set **Memory limit per backup (MiB, 0 = none)** in the **Performance** card under **Settings, System**, well below what the container may use. Near the limit restic frees memory more often, which costs some speed, so the backup slows down instead of being killed.

## A restore aborted before it started

Before anything is stopped or removed, restore runs a pre-flight conflict check: it verifies the container's static IP and published host ports are free. If another container already holds one, it aborts with a clear, actionable message instead of leaving a half-finished restore. Free the conflicting port or IP, then retry.

If the container's network does not exist, for example because its TrueNAS app was deleted, the restore check names the network and the restore refuses before it changes anything. Reinstall the app in TrueNAS under the same name, which creates the network again, then restore its data.

## A plain export failed instead of writing a file

If **Encrypt exports with age** is on under **Settings, System** but no valid recipient is set, an export fails with a clear error instead of writing plaintext. Add a valid recipient (an age public key or an SSH public key), or turn encryption off if you intend the export to be plaintext. See [Features](features.md).

## A database dump failed

A failed dump never fails the backup around it; it is recorded as its own failed run, and the reason names what to fix.

- **Login refused.** The dump signs in with the container's own password variables (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` or their `_FILE` versions). Check them on the database container. A `_FILE` variable pointing at a secret the container's own user cannot read fails the same way.
- **Missing privileges.** With a random root password the dump can only sign in as the app user, so it holds that one database, and MySQL 8.4 and newer may refuse it outright. Give the container a real root password, or switch the dump off for it.
- **The system tables need an upgrade.** MariaDB refuses to dump when its system tables come from an older version (error 1558). Add the variable `MARIADB_AUTO_UPGRADE=1` and restart the container, or run `mariadb-upgrade` inside it once.
- **No dump tool.** A slim or self-built image without `pg_dump`, `mysqldump` or `mariadb-dump` cannot be dumped. Use the official image, or switch the dump off.
- **A time limit.** A dump gets `DB_DUMP_MAX_HOURS` (6 by default), the backup around it gets `BACKUP_MAX_HOURS`, and a dump that stops making progress is cut after `BACKUP_STALL_HOURS`. The usual cause of the last one is a lock the application holds. Raise the limit that fired, or dump while the application is quiet.
- **The container is paused or restarting.** The dump talks to the running server. If the container keeps restarting, its own log says why.
- **A damaged dump could not be removed.** A dump BombVault could not finish is deleted again. When that delete fails, the dump stays in the list marked as damaged, and you can delete it there.

## An import failed

An import stops the container, moves its data folder aside and lets the image create an empty one in its place. If a step before the import itself fails, the old folder is put back automatically. If the import fails, the container keeps the fresh folder and the old one stays beside it as `<data folder>.bombvault-before-import-<timestamp>`; the run's error message names the exact path.

A data folder that is a dataset of its own, as on TrueNAS, cannot be renamed. The import then leaves the folder in place and moves its contents into the kept folder next to it, so the dataset above needs room for them. The import checks that before it stops the container.

To put it back by hand: stop the container, rename the current data folder out of the way, rename the kept folder back to the original name, and start the container. For a dataset, move the contents instead of renaming the folders. On Unraid, the file manager does this from the Shares tab.

## A ZFS dataset backup failed or skipped a dataset {#zfs-datasets}

Every problem carries a reason code in brackets, and the [ZFS datasets](zfs-datasets.md#reason-codes) page lists all of them with the fix. The three most common:

- **`snapshot-loop`**: the snapshot did not reach BombVault because Host Data does not pass new mounts through. Edit the container, set the Access Mode of Host Data to Read/Write - Slave and restart BombVault.
- **`key-not-loaded`**: an encrypted dataset whose key is not loaded is skipped. Load the key with `zfs load-key` and mount the dataset; the next backup includes it.
- **`ssh-auth`**: the server refused BombVault's key. **ZFS connection to this server**, opened with **View connection** on the Backups page, shows the command that authorizes it; run it once on the server.

## An entry stays at "Learning N/10"

Most anomaly checks start after 10 successful backups of an entry, and the count starts again after **Mark as expected** and after the entry's selection changed. An entry that is not scheduled does not learn, and a container without appdata has nothing to learn from, which its badge says. **Learning progress** on the Anomalies page shows a ring of ten parts per entry.

## Retention stopped deleting old backups of one entry

An open critical anomaly is holding them: the entry's source is almost empty, shrank sharply, or one backup stored most of its data again. Open the anomaly from the badge on the entry. If data is missing or was encrypted, restore from the linked last good backup first. Then acknowledge the anomaly, or mark it as expected if the change was yours, and the next run prunes as usual. The retention preview marks such an entry as kept. For a ZFS entry only the dataset named in the anomaly keeps its old backups, and the other datasets of the tree are pruned as usual.

## Manual prune says some entries were kept

The same cause: prune leaves the old backups of an entry with such an anomaly alone and names the entry in its message. Everything else is pruned as usual.

## History import says a repository could not be read

After the upgrade BombVault reads the sizes of earlier backups from each repository once. A repository it could not reach at that time, such as an off-site target that was down or a share that was not mounted, is counted on the **Anomalies** card under **Settings, Verify** and tried again once a day. Its entries learn from new backups in the meantime.

## The disk-space warning does not match the Unraid dashboard

On the Unraid user share (`/mnt/user`) the free space is that of the whole array, not of one disk. Remote repositories are measured only through rclone remotes that report their free space; S3, B2, REST and SFTP repositories have no figure and are listed as not measured on the **Anomalies** card.

## An AI assistant cannot connect

The [MCP server page](mcp.md#troubleshooting) lists what each status code and each refusal of the MCP endpoint means and what to do about it.

## The container keeps restarting or looks unhealthy

BombVault reports healthy/unhealthy from its own `/api/health`. An auto-heal tool (such as Autoheal) can restart it automatically if the engine ever wedges. Check the container log and the **Host Integration** card for the underlying cause.

## Still stuck?

- Read the full [Configuration](configuration.md) and [Off-site & recovery](offsite-recovery.md) pages.
- Ask on the [Unraid support thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Open a [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues).
