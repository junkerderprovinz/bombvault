# Worked examples

Each of these walks through one real job from start to end, with names and numbers filled in. They stand on the page that explains the feature, so the details are right beside them.

## Getting started {#start}

- [Your first container backup, end to end](getting-started.md#example-first-backup): from the install to the recovery kit.

## Storage and off-site {#storage}

- [An off-site copy on Backblaze B2](offsite-recovery.md#example-s3): a bucket, a key for that bucket, and the first copy. Cloudflare R2 works the same way.
- [A rest-server you run yourself](offsite-recovery.md#example-rest-server): an off-site copy that refuses deletes, with the `docker run` line and the tamper test.
- [What Balanced keeps](offsite-recovery.md#example-retention): retention over two years of nightly backups, backup by backup.
- [Containers move to a NAS](offsite-recovery.md#example-own-folder): a section's own folder changes place, and what happens to the backups already made.

## Backups and schedules {#backups}

- [Three plans that fit together](schedules.md#example): a full backup at night, VMs once a week, and an own plan for one entry every 6 hours.
- [Plex waits while someone watches](backups.md#example-plex): a scheduled backup that does not interrupt a film.
- [A VM with two Unraid snapshots](vm-backup-ssh-setup.md#example-unraid-snapshots): a live backup that keeps the snapshots, and a restore next to the original.
- [A replica to a second Unraid box](zfs-datasets.md#example-replica): a ZFS copy you can read at any time, and three ways to get one file from it.

## Getting things back {#restore}

- [Get one file back](restore.md#example-single-file): a single file from last night's container backup.
- [One file from a ZFS backup](zfs-datasets.md#example-zfs-file): the same for a dataset.
- [A new server from the recovery kit](offsite-recovery.md#example-emergency): a dead server comes back on a new box, without its old `APP_KEY` in the template.

## Staying informed {#staying-informed}

- [Gotify on your phone](notifications.md#example-gotify): failures ring and successes arrive quietly.
- [ntfy without an account](notifications.md#example-ntfy): messages to a topic of your own.

## Several servers {#several-servers}

- [Pairing two Unraid boxes](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): two servers that keep each other's copies, and neither can delete the other's backups.
- [A card and an alert in Home Assistant](api.md#home-assistant-example): BombVault on a dashboard, with a message when a backup fails.
