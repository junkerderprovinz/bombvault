# Features

A short catalogue of what BombVault does. Each line links to the page that explains it.

## Backup scope {#backup-scope}

- **Containers**: the appdata folder and the container definition, with a choice of folders per container. [Backups and entries](backups.md#what-is-saved)
- **VMs**: disk images, XML and UEFI NVRAM over SSH, with a graceful shutdown, a live snapshot or changed blocks only. VMs with Unraid snapshots and disks on zvols are covered in full. [VM backups](vm-backup-ssh-setup.md)
- **Flash**: the whole Unraid USB flash, with a scheduled zip export. [Backups and entries](backups.md#settings-tab)
- **Self-backup**: BombVault's own settings and sign-ins. [Emergency wizard](offsite-recovery.md#emergency)
- **Folders**: any folder on the server as a folder set. [Adding a folder](backups.md#add)
- **ZFS datasets**: a dataset tree read from one snapshot, with single files restorable and an optional replica on a second ZFS server. [ZFS datasets](zfs-datasets.md)
- **Database dumps**: PostgreSQL, MySQL and MariaDB, dumped from the running server into a restore point of their own. [Database dumps](backups.md#database-dumps)

## Backups and entry pages {#backups}

- **One list** with tiles, **Filters**, **Sort** and **Search**, and the last fourteen runs of each row as a strip. [The list](backups.md#list)
- **Select like in a file manager**, and back up or pause a selection on the server. [Picking several entries](backups.md#select)
- **A page per entry** with **Backups**, **Settings** and **Verify**. [The page of an entry](backups.md#entry-page)
- **Tags and compare** for snapshots, and **Export** as tar, zip or a restic repository. [Backups tab](backups.md#backups-tab)
- **Exclude patterns** with a live preview, and recommended excludes for well-known apps. [Exclude patterns](backups.md#excludes)
- **Commands before and after**, and **Stop other containers** with a health-gated restart in dependency order. [Commands and other containers](backups.md#commands)
- **When the app is busy**: a scheduled backup waits for Plex, Jellyfin, Emby, your own check or quiet traffic, and tries again on its own. [When the app is busy](backups.md#app-busy)
- **Snapshot instead of stop**: a container on ZFS is down only for a few seconds. [Schedules](schedules.md#snapshot-instead-of-stop)
- **Plain exports**, optionally sealed with age. [Plain exports](backups.md#exports)

## Restore {#restore}

- **One page and one window** for every kind, with the recommended backup preselected. [Restore](restore.md)
- **Next to the original** by default, and **Replace original** backs up the current state first. [The restore window](restore.md#window)
- **Single files** to a folder or back to their place. [Get one file back](restore.md#example-single-file)
- **Several entries at once**, each from its recommended backup or from one moment. [Restoring several entries](restore.md#several)
- **Whole Compose stacks**, started in dependency order. [Restoring a stack](restore.md#stacks)
- **Containers and VMs are recreated** on the image they ran, with GPU, limits and links. [What comes back](restore.md#kinds)
- **Checks before the restore**, and a list of what it changes. [The restore window](restore.md#window)
- **Flash as a ZIP**, and one plugin at a time back into the running flash. [What comes back](restore.md#kinds)
- **ZFS** into the dataset with a safety snapshot, into a folder or into a new dataset with its properties. [ZFS datasets](zfs-datasets.md#restore)
- **Emergency wizard** for a lost server, with the recovery kit, through the group or with the typed key. [Emergency wizard](offsite-recovery.md#emergency)
- **From an old backup folder**, **Import from the Appdata.Backup plugin**, and **Jobs from Duplicati or Duplicacy**, which turns their backup jobs into folder sets. [Other sources](restore.md#other-sources)

## Storage & scheduling {#storage-scheduling}

- **Storage locations** of every kind: local folders, SMB, WebDAV, SFTP, rest-server, S3, cloud drives and any rclone remote. [Storage locations](offsite-recovery.md#destinations)
- **Sign-ins**: one list with a default. [Sign-ins](offsite-recovery.md#sign-ins)
- **Own folders** on this server or in a storage location. [A section's own folder](offsite-recovery.md#own-folder)
- **3-2-1 as a flow** where every line is a switch, and **placement per entry**. [Placement per entry](offsite-recovery.md#placement)
- **Off-site copies** to several locations per section, with bandwidth limits, compression, cold storage classes and a slowdown while Plex streams. [Off-site copies](offsite-recovery.md#off-site-copies)
- **Direct repositories**, listed under **Backs up straight here**. [Direct repositories](offsite-recovery.md#direct-repositories)
- **Retention as a timeline**: **Short**, **Balanced**, **Long** or **Custom values** per location, with rules per section. [How long to keep](offsite-recovery.md#keep)
- **Move a section's history**, and **Remove storage location** with or without its backups. [Moving and removing](offsite-recovery.md#move-remove)
- **Schedules that read as sentences**, the **Week** band, a full backup, plans per section and an own plan per entry. [Schedules](schedules.md)
- **Catch up missed backups**, a manual order, and updates after a successful backup. [Schedules](schedules.md)

## Insight, verification & monitoring {#insight-verification-monitoring}

- **The overview**: status head, **Needs you**, **Protection status**, storage forecast, history and optional cards. [Overview](overview.md)
- **Anomaly detection** per entry, which also keeps old backups when a source collapses. [Anomalies](anomalies.md)
- **Restore check** after the first backup, and **Start test** in an isolated copy. [Verify tab](backups.md#verify-tab)
- **Scheduled checks and trial restores** from the off-site copy. [DR drills](offsite-recovery.md#dr-drills)
- **Size by folder**, and why a backup was slow. [What a run tells you](backups.md#run-notes)
- **Notifications**: webhook, Gotify, Matrix, email, Apprise, Unraid and Healthchecks, with a weekly digest and an overdue report. [Notifications](notifications.md)
- **Prometheus**, a Home Assistant device, an HTTP API and mDNS. [API and integrations](api.md)
- **Bug report**: a redacted ZIP for a GitHub issue. [Troubleshooting](troubleshooting.md#still-stuck)

## Ransomware protection {#ransomware-protection}

- **Delete protection (append-only)**, enforced by the far side and proven by a tamper test. [Delete protection](offsite-recovery.md#delete-protection-append-only)
- **Accept without delete protection** for a place that cannot refuse a delete. [Delete protection](offsite-recovery.md#delete-protection-append-only)
- **Deleting backups for good takes your own sign-in** once a login password is set, and a paired phone cannot do it. [Security model](configuration.md#security-model)
- **Recovery kit**, optionally sealed with age, and a **key keeper** in your group. [Recovery kit](offsite-recovery.md#recovery-kit), [Key keeper](offsite-recovery.md#key-keeper)

## Several servers {#several-servers}

- **Pairing by twelve words**, end-to-end encrypted, directly or through a relay. [Pairing instances](offsite-recovery.md#pairing)
- **The Instances page** with the group as a net and the roles **Receiver**, **Fetcher**, **ZFS server** and **Key keeper**. [Instances and roles](offsite-recovery.md#roles)
- **A receiving server** set up for you, in append-only mode. [Receiving server](offsite-recovery.md#receiving-server)
- **A shared repository** that several servers copy into, so what they have in common is stored once. Only the server that holds it prunes. [Shared repository](offsite-recovery.md#shared-repository)

## AI assistants (MCP) {#mcp}

A built-in MCP server lets an assistant read backup status, coverage, run history and restore points, and with a key that allows it start a backup. Restores, deletions, prune and settings stay in the web interface. [MCP server](mcp.md)

## Apps and companions {#apps}

- **Android app**: every server of your group on your phone. [Android app](android.md)
- **BombVault Widget**: a tile on the Unraid Dashboard. [Apps](settings.md#apps)
- **Embeddable activity log** for dashboards such as Homepage or Heimdall. [Dashboard widget](api.md#dashboard-widget)

## Other {#other}

- **42 languages**, three themes, and labels, colours, corners and animation of your choice. [Look](settings.md#look)
- **Settings that save themselves**, with a search over all fourteen tiles. [Settings](settings.md)
- **What's new** after an update, with green dots on what changed. [Info](settings.md#info)
- **HTTPS out of the box**, and a healthcheck for auto-heal tools. [Configuration](configuration.md#network)
