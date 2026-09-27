# Storage places

A storage place is somewhere BombVault keeps backups: a folder on this Unraid, a share on a NAS, a bucket at a cloud provider, a rest-server, an SFTP account or a Nextcloud. You connect each place once, on **Settings, Storage**, and its credentials, retention, protection and location belong to it. The domains (containers, VMs, the flash, BombVault's own configuration, folder sets and ZFS datasets) then choose from the places: where each domain is stored and where it is copied. The ZFS datasets have their row on the Domains card while the ZFS domain is switched on.

## Adding a place {#add-a-place}

**Add place** opens a window with one tile per provider, in three groups: cloud storage, self-hosted services, and NAS devices and this server.

1. Pick a tile and fill in its form. The eye button shows a secret you typed.
2. **Test connection** checks the place and creates nothing. For each domain's folder it reports what it found: empty or not there yet, already holding a restic repository, or the error that stopped it.
3. Give the place a name; the provider's name is filled in. For a device you run yourself, answer **Where is the device?**. Cloud providers are always at another site, and a folder on this Unraid is always here.
4. **Add** saves the place.

A new place is not used by any domain yet. Choose it under **Stored in** or **Copied to** on the [Domains card](#domains), or on an item's card for that item alone.

## Folders {#folders}

A place keeps one folder per domain: `container`, `vms`, `flash`, `config`, `files` and `zfs`, the names the default backup locations use. The folders are listed in the place's details and can be renamed there (see [Changing an address](#addresses)). A domain without a folder at a place cannot choose that place.

When a domain uses a place in both roles, the second role gets a suffix and the first keeps its folder. A place that already receives a domain's copies stores the items sent straight to it in `<folder>-direct`; a place that already stores a domain receives its copies in `<folder>-copies`.

Some places are a restic repository themselves: an address that already held a repository when the place was added, a named repository from an existing setup, or a copy target at the root of a bucket. Such a place has no folders, every domain shares its one repository, and it takes no second role. To store more at the same provider, connect another bucket or folder as a place of its own.

## Place details {#details}

Each place is a row with its provider, what it is used for and its last test or copy. **Test** checks every address at the place, and **Details** opens its settings. Every change in the details saves as you make it.

- **General**: the name, the switch that turns the place on and off, the address and, for a device you run yourself, **Where is the device?** (see [Off the premises](#off-the-premises)).
- **Retention**: keep-last, daily, weekly and monthly, for every repository at the place. A new place starts with the default rules; a place with every rule at zero never trims.
- **Protection**: the **Append-only** switch. The far side has to enforce append-only; with the switch on, BombVault never prunes or deletes there. At a rest-server with append-only on, **Test append-only** runs the tamper test against every domain path, switched-on copy and repository at the place and shows one answer for the whole place, *deletes refused* or *deletes accepted* (see [Off-site & recovery](offsite-recovery.md)). Only remote places have this section, because nothing on this box can stop a local repository from being deleted.
- **Access**: the credentials and, for S3, the storage class. A place that uses the shared credentials gets a set of its own at the first change. A direct repository at the place that the new credentials cannot open keeps the old ones, and the answer says so. Folder, SFTP and rclone places have no such section.
- **Limits**: the upload and download rate and the growth budget.
- **Folders**: one switch per domain, with the name of its folder. A domain switched off here cannot choose the place.

Lowering the retention asks first and says how many items that affects; switching append-only off asks first and says how many repositories at the place lose it. Switching a place off switches off every repository at it; a place a domain is stored in cannot be switched off.

## The Domains card {#domains}

The card has one row per domain, with its schedule, where it is stored, where it is copied and its exceptions.

- **Stored in**: while the domain's backup location holds no backups, the chosen place becomes the domain's home and the location moves there. Once it holds backups, the choice for containers, VMs and folder sets becomes the default for new items, which take it at their first backup; items that already have backups stay where they are, because BombVault never moves a backup. For the flash and BombVault's own configuration the home moves, and the backups already written stay at the old place.
- **Copied to**: one chip per place that can take the domain's copies. Ticking a chip makes the place a copy target of the domain; the first time, BombVault says beforehand how many items and snapshots and how much data the first run sends. Unticking stops new copies: the copies already there stay and age by the place's retention, and items with a choice of their own keep copying there. Unticking the last chip stops all copies, also to places added later, until one is ticked again. A switched-off place shows as a dimmed chip and cannot be chosen.
- **Exceptions**: the items with a choice of their own, as a list with links to their cards.
- **Copy now** runs the domain's copies at once.

A domain paused after a rebuild through Discover shows the pause on its row, with **Confirm default** (see [Placement per item](offsite-recovery.md#placement)).

## Changing an address {#addresses}

A domain's folder can be changed in the place's details, and so can the address of a local place, for example after a repository was moved to another disk by hand. BombVault tests every address the change affects and accepts it when each new address is empty and nothing was stored at the old one, or when each new address holds the same restic repository as the old one. Anything else is refused, with the number of backups still at the old address. A remote place keeps its address; to back up somewhere else, connect that as a place of its own.

BombVault builds the list of places from its own database and never lists a remote repository to fill it; the test runs only when you change something.

## Removing a place {#remove}

A place can be removed only while nothing uses it: no domain is stored there, no default points at it, no item is stored there, and no direct repository at it holds items. Otherwise the refusal lists what holds it. Removing it takes its copy targets along, and its own credentials unless a pull source or another place uses them. Nothing is deleted in the storage itself, and the confirmation says how many copies stay behind there.

## Without a place {#without-a-place}

An address that does not fit the form of a place plus a folder keeps working and is listed under **Without a place**, with its address. Native `b2:`, `gs:` and `swift:` addresses are among them. **Assign to place** attaches such a row to a place, after the same test as [changing an address](#addresses). A copy target without a place is also named on its domain's row, next to the chips, and keeps copying. A remote row there has its own **Append-only** switch, and switching it off asks first with the number of items that keep backups at that address. A direct repository follows the switch of its target.

## Off the premises {#off-the-premises}

**Where is the device?** has two answers: **Here in the house** and **At another site**. A copy counts as a site of its own, for the 3-2-1 line on the cards and for the Dashboard's off-site checks, only when its place is at another site. A second disk or a NAS in the same house is a second copy, not a second site. The answer changes no copy. Cloud providers are always at another site and a folder on this Unraid always here, so the form does not ask for them; for every other place, change the answer in its details. A place at another site carries the mark **Other site** on its row.

## Connection kinds

### Folder on this Unraid or a NAS {#kind-local}

The address is a path under `/mnt`, written without `/mnt`, for example `user/bombvault`, and each domain's folder sits below it: `user/bombvault/container`.

- **Folder on this Unraid** picks from the shares, disks and pools.
- **Synology**, **QNAP**, **TrueNAS**, **Another Unraid** and **Other share** pick from `/mnt/remotes`. Mount the share on Unraid first, for example with the Unassigned Devices plugin. Host Data has to be mounted Read/Write - Slave, or a share that mounts after BombVault started stays invisible until a restart (see [Configuration](configuration.md)).

The folder picker creates a folder where it stands with **New folder**. The test checks that the folder is empty or absent and that BombVault can write there.

### S3 {#kind-s3}

The address is `s3:https://<endpoint>/<bucket>/<path>`, for example `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** needs only the key ID and the application key. BombVault asks B2 which bucket, S3 endpoint and folder the key is limited to and builds the address from them. A key that may reach every bucket offers its buckets to choose from.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** and **Vultr** ask for the key and, where the provider needs it, for the region, account ID or endpoint. BombVault fills in the endpoint and lists the buckets when the key may list them; otherwise type the bucket's name.
- **Google Cloud Storage** goes through its S3 interface with an HMAC key, created in the Cloud Storage settings under Interoperability. A service account file does not work here.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** and **Other S3 service** take the service's address and a key.

The storage class is set in the place's details, limited to tiers a restore can read without a thaw.

### rest-server {#kind-rest}

The address is `rest:<url>/<user>`, for example `rest:https://nas.lan:8000/tower`. The form asks for the server's address, a user and a password. With `--private-repos` a user may only reach paths that start with its own name, so BombVault puts the user first unless you type another path. When the server refuses a path outside the user's own, the error says so.

The rest-server form carries a ready-to-paste recipe for a rest-server in append-only mode with one user for this BombVault. **Show recipe** makes a password, shown once, and gives a `docker run` line, a compose file and an Unraid template, each with the `htpasswd` line to put on the server; the user and the password go straight into the form.

**Another BombVault** lists the open offers other instances sent over Fleet above its own fields. Accepting one adds a place that keeps copies of the offered domain only, because an offer carries a user for that one domain. Accepting on the Fleet page adds the same place.

### SFTP {#kind-sftp}

The address is `sftp://<user>@<host>:<port>/<path>`, for example `sftp://bv@backup.lan:22/bombvault`. The form asks for host, port and user and shows BombVault's public key. Add that key to the user's `~/.ssh/authorized_keys` on the server; nothing else has to be installed there. BombVault accepts the server's host key on first contact and checks it from then on.

**Hetzner Storage Box** fills in `<user>.your-storagebox.de` and port 23. Install the key on the box with Hetzner's own command, which asks for the box's password once:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

The form asks for the server's address, the user and an app password. Create the app password in the account's security settings, and enter the user ID rather than an e-mail address. BombVault builds the WebDAV path the product uses and hands the connection to restic through rclone's environment variables, with the password in rclone's obscured form. The address reads `rclone:bvp<id>:<path>`, where `bvp<id>` is a remote that exists only in that environment; nothing is written to the rclone config.

### Azure Blob {#kind-azure}

The address is `azure:<container>:/<path>`. The form asks for the storage account and its access key; after **Test connection** it lists the account's containers to choose from, or you type a container's name. BombVault passes the account and the key to restic as `AZURE_ACCOUNT_NAME` and `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

The address is `rclone:<remote>:<path>`. The form lists the remotes of BombVault's rclone config to choose from. To replace that config, paste a whole `rclone.conf` under **rclone config** and click **Save config**. It is saved at once and serves every rclone place, whether or not the window then adds one.

## Copies between places with different credentials {#different-credentials}

A domain stored in a remote place is the source of its copies. `restic copy` runs with one environment, and BombVault adds the source's credentials to the target's when the two do not set the same variable to different values. A Nextcloud place and a B2 place use different variables, so a domain stored in Nextcloud can be copied to B2. Two S3 accounts or two rest-server users would need the same variables with different values; restic cannot take both, and the chip on the Domains card says the credentials do not fit.
