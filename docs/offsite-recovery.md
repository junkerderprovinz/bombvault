# Off-site & recovery

!!! note "Off-site copies wait after a rebuild"
    When the Emergency wizard or **Discover backups** rebuilds entries without the old settings, off-site copies of those sections pause until their default is confirmed. See [After a rebuild](#after-a-rebuild).

Local backups protect you from a lost container or a bad update. Off-site copies and a tested recovery kit protect you from the whole box, ransomware, or a fire. This page covers storage locations and off-site copies, making a copy tamper-proof, proving you can restore, pairing several servers, and getting everything back when BombVault itself is gone.

## Off-site copies {#off-site-copies}

Keep the fast local backup and add one or more storage locations off the premises. BombVault copies new snapshots there with `restic copy` on a best-effort basis, so an off-site hiccup never fails the local backup. In this shape the local repo stays primary and the off-site repo is a copy, but a section's primary repo does not have to be local at all; see [Remote primary repositories](#remote-primary-repositories) below for backing up straight to S3, a rest-server and the like instead of copying to it.

- **Several storage locations per section.** Each section (containers, VMs, flash, self-backup, folders and ZFS datasets) can copy to several off-site storage locations at once, so you can keep, for example, a rest-server on a friend's box and an S3 bucket in parallel. The **Storage locations per section** card on the Storage locations page switches every line from a section to a location on or off.
- **When copies run.** A location's **Copy** card sets **When to copy**: **After every backup**, or on a plan of its own that collects the snapshots and copies at fixed times. Under **Settings, Schedule, Off-site copies per section**, **Exception** gives one section its own copy plan. **Replicate now** in a section's **Off-site copy** card under **Settings, Storage locations** starts a copy on demand.
- **Retention** is set per storage location under **How long to keep**, so you can keep off-site copies longer as an archive. The copy in a section's off-site field follows the **Off-site retention** card under **Settings, Storage locations**; leave it all-zero to never auto-trim off-site snapshots.
- **Bandwidth limits** (**Off-site bandwidth** under **Settings, Storage locations**, or a location's **Copy** card) cap the restic upload/download rate so copies do not saturate your WAN.
- While a copy runs, the overview's status head says **Off-site copy running**. It is an active indicator, not a percentage bar, because `restic copy` exposes no machine-readable progress.

!!! note "Restore from any place"
    Every entry lists its backups as one timeline across all places a backup lies, on its **Backups** tab. A backup copied to B2 appears once, marked with each place that holds it. A restore takes the first place it can reach, starting with the repository the entry is written to, and you can pick another place per snapshot. Off-site storage locations are read only when you click **Check**, because listing cloud storage takes a while. Deleting at one place first checks the others and says whether it was the last copy.

## Storage locations {#destinations}

A storage location is a place backups go: a local folder, a share, a server of your own or a cloud service. It is set up once and then serves every section. Open **Storage locations** from its settings tile, or switch on **Also show in the sidebar** there to pin it.

**Add storage location** opens a window in four steps:

1. **Provider.** Every service is listed with its logo, with a search at the top right of the window, in groups: storage services with S3 buckets (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 and others), your own S3 server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), your own server and shares (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, a mounted path), cloud storage (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud and the rest rclone supports) and the receiving servers of your group. Each one says how well it suits backups: cloud drives slow down under many requests, so the first backup and pruning take longer there.
2. **Sign in.** The fields depend on the service: an access key for S3, a user and password for WebDAV and SMB, an app password where two-factor sign-in blocks the normal one, BombVault's public SSH key for SFTP and the Storage Box, or a token for services that sign in through a browser. For those, the window shows an `rclone authorize` command to run on a computer with a browser; the token it prints goes into the field. **Test** checks the sign-in before anything is saved. The window then lists the folders on the storage location, with **New folder** to create one and the free space where the service reports it. An empty folder is safest; BombVault puts one folder per section in it.
3. **Protection.** The window says plainly what the service can do. A rest-server in append-only mode refuses deletion, and the tamper test checks that. An S3 bucket can keep old versions with versioning and object lock, which BombVault cannot check yet. A cloud drive cannot refuse deletion at all: whoever gets into the server gets into that copy too. Switch **Delete protection (append-only)** on only where the far side really refuses deletion; BombVault then never prunes there. **For an emergency** says what the recovery kit holds for this location.
4. **For which sections.** Pick the sections that back up here. The first copy brings the whole history along. Entries that chose their own storage locations do not get the new one on their own, and the window names them.

S3 services run through restic's own S3 backend, which is what lets a storage class and object lock apply. Every other service runs through the rclone BombVault ships, and its remote then appears in the **Off-site (rclone)** card under **Settings, Storage locations**. A settings export carries the storage locations; with credentials included it carries their sign-in as well.

The folder on this server is called **Local** everywhere: on the storage pages, in the flow, on an entry and on Restore. With several folders on this server, each goes by its own name. A storage location that a section's off-site field sets up is named after its place, the last folder of its address or, without one, its server, and gets a number when another location already goes by that name. A name you give it on its page stays. Where two locations with the same name stand side by side, each carries its server or folder.

Every storage location has a page of its own:

- **Connection** with its name, the switch **Available** (off means BombVault no longer writes here, and entries that back up here fail with a clear message instead of landing somewhere else), the folder, compression, storage class and **Off the premises**. **Change connection** tests and saves a new address or sign-in.
- **Credentials**: the shared set or a set of its own.
- **Delete protection**, with **Test delete protection now** and who prunes this copy.
- **Copy**: when to copy, and the upload and download limits.
- **How long to keep**: **Short**, **Balanced**, **Long** or **Custom values**, with a timeline of which backups the rule keeps. **Ask before keeping less** asks before a shorter rule deletes backups whose only copy is here.
- **Rules per section**: one section's own repository, retention or compression at this location. **Change rules** opens them, and **Reset to the storage location** brings a section back.
- **Already under this storage location**: sections whose hand-typed address lies in a folder of this location. Taking one over leaves the repository where it is; sign-in, storage class and delete protection then come from the location. BombVault first checks that the location's sign-in opens the repository, and it refuses to put an append-only repository under a location that is not append-only.
- A vessel that shows how full the location is, with the mark where it is full and how many weeks are left at the growth of the last weeks. A location that does not report its size says **Size unknown**.

A receiving server that another instance of your group runs appears among the providers as **From your group**; see [Receiving server](#receiving-server).

### Moving and removing {#move-remove}

- **Move** takes the copy of one section to another storage location, with its whole history. Everything stays in the old place; BombVault deletes nothing there. The move runs in the background and can take long: about 9 minutes per 100 GB between local disks, hours over a slow link. The overview shows it while it runs.
- **Remove storage location** offers two ways. **Remove storage location, keep the backups** stops writing there and leaves the backups in place; you can read them later under **Restore, From an old backup folder**. **Remove storage location and delete the backups there** first reads what is stored there and lists it: which backups go, which of them are nowhere else, and which repositories stay because delete protection refuses, entries still back up straight to them, another location or section keeps its copies there, or they could not be read. Entries that would lose their only backup are listed separately and are kept unless you switch on **Delete these as well**. You confirm by typing the location's name, and the deletion runs in the background.
- Deleting the backups at a storage location needs a login password and a sign-in at the instance. Without a password the location can still be removed, only its backups cannot be deleted from here.

## Placement per entry {#placement}

The **Schedule and storage location** card on an entry's Settings tab has a row of buttons: **Local** and one button per off-site storage location of the section. Lit buttons get the entry's backups.

- With **Local** lit, the entry is written to the repository shown under **Stored on** and copied to every other lit location. Darken a location and it gets nothing new from this entry. Local alone copies nowhere, which suits data that already has a second copy, for example a share that lives on a NAS.
- With **Local** dark, the entry is written straight to the direct repository of the first lit location and copied from there to the other lit ones. The first time, a dialog creates that direct repository.
- One button stays lit, because a backup needs a place to go. To leave an entry out of backups, use **Don't back up**.

BombVault never moves backups between repositories. An entry that follows its section goes along when the section's default changes: its next backup lands in the new place, and the backups it has stay where they are. A location chosen on the entry itself stays fixed from its first backup on; the **Repository** card on the Settings tab names it. The copies can change at any time. A location that no longer gets an entry keeps the copies it has and trims them to its own retention at the next off-site run of the section. Append-only locations cannot be deleted from.

The place an entry left keeps its backups. On the entry's **Backups** tab and on **Restore** it stands as **Earlier backups only**, a restore can still read from it, and a prune there leaves those backups alone.

**Follow the default** brings an entry with a choice of its own back to its section. It asks first, says where new backups go then, and names what each location stops getting and what the next run would upload.

Under the row the card says where the entry goes and what is actually there: how many sites hold it, when each location was last seen, and whether 3-2-1 is met. A site is the server with the original data, each off-site location and each repository marked **Off the premises**. BombVault checks copies and sites; it does not check the "two media" part of 3-2-1.

### Defaults per section {#placement-defaults}

The **Storage locations per section** card on the Storage locations page draws the default of every section as a 3-2-1 flow. Every line from a section to a storage location is a switch, **Local** included, for every section, the flash drive, ZFS datasets and the self-backup as well. A click on a line or a dot switches it at once, without a question; dashed lines are off. **Graph / Switches** shows the same as rows of buttons. A switch decides where new backups go. Entries without a choice of their own, and the project folders of Compose stacks, follow at once, so their next backup goes to the new places. Backups already stored stay where they are and can still be restored from there. Flash, ZFS datasets and the self-backup back up to their local folder and copy off-site from that backup. Their line to **Local** can be switched off too; their backups then go straight to their off-site locations, and a ZFS entry with a replica and no location at all only sends its replica.

With **Local** off and only off-site locations on, a section backs up straight to them, and BombVault picks the repository there itself. Containers, VMs and folder sets go to the direct repository of the first location that is on, described below, and are copied from there to the others. The flash drive, ZFS datasets and the self-backup write each backup to every location that is on, and each location keeps them by its own retention.

A section with every line off says **Not backed up**. Its entries get no backups and count as not protected.

Entries that chose their own storage locations on their page keep them. The flow says how many there are in a section, for example **2 entries differ**, and leads to them: to the entry's Settings tab when there is one, to a list with where each backs up when there are several. **Follow the default** there brings an entry back.

A new off-site location receives every entry that is not set to Local. The window that adds it says how many entries and, where known, how much history that is.

### Direct repositories

A direct repository is where entries back up straight to an off-site location. BombVault puts it beside the location's folder with `-direct` added to the name, or, when the location sits at the root of its bucket, its remote or a rest-server user, in a folder inside it named after the section, such as `containers-direct`. In the flow on the Storage locations page it is created without a question. Turning Local off for a single entry, so that a location without a direct repository becomes its home, opens a dialog with the suggested location, for example `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, and a connection test that creates nothing. **Create and use** creates the repository and points the entry at it. A direct repository takes the location's key, storage class, limits, append-only setting and retention, and changes with them; the Repositories card shows it read-only. When a new key for the location cannot open it, the direct repository keeps the key it has and the save says so. An entry on a direct repository is copied from there to the other lit locations, never to the location the repository belongs to. Its snapshots carry the tag `bv:direct`, and every other retention pass keeps them, so a direct repository that lost its link to its location never ages by the local rules. B2 is reached through its S3 endpoint, with the key ID and application key entered as the S3 credentials; a key limited to the location's own folder cannot reach the folder next to it, so limit the key to the folder above instead.

### Off the premises

A storage location or a named repository can be marked **Off the premises**. Remote ones start marked; switch it off for a rest-server in the same building. The mark only counts sites and 3-2-1 on the cards. It changes no copy.

A ZFS replica is a copy off the premises, but not a backup with history. It keeps only the snapshots you set, and old versions and single files still come from the backups. It goes from one ZFS server straight to the other, over SSH to a ZFS server or to the receive endpoint of a paired instance. The pairing group only carries the request. See [Replica](zfs-datasets.md#replica).

### After a rebuild {#after-a-rebuild}

Copy choices live in BombVault's own settings. After a rebuild through **Discover backups** or the Emergency wizard without restored settings they are gone, and copying everything would send the entries you had left out to B2 again. Off-site copies of every rebuilt section therefore pause. The overview says so, and the **Storage locations per section** card offers **Confirm default** with a preview of what the next run copies and the names in the backups that have no entry, which you can leave out there. Only the confirmation ends the pause; importing a settings file brings back rules and defaults but does not end it.

## Remote primary repositories {#remote-primary-repositories}

A section's own path is not limited to a local folder: point it straight at a restic remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) and BombVault backs up to it directly, with no separate local copy and no copy step. This is a different shape from the off-site copies above: there the local repo is primary and the off-site repo is a best-effort archive of it; here the remote repo **is** the primary, and it is the only copy unless you also copy that section to another storage location.

The path of each section (Containers, VMs, Flash, Self-backup, Folders, ZFS datasets) is set on the page of the **Local** storage location: **Change** beside the section in its **Connection** card opens a window with a **Local / Remote** switch:

- **Local** shows the familiar folder browser.
- **Remote** swaps it for a plain URL field and the connection settings an off-site location uses, configured for this primary instead. From there you get:
    - **A connection test** against the live path, before you rely on it.
    - **Bandwidth limits** (upload/download) so a scheduled backup to a remote primary does not saturate your WAN: the same `--limit-upload`/`--limit-download` restic flags off-site copies use, applied to the backup itself.
    - **Append-only protection**, verified with the same active tamper test (a real DELETE probe against the far side) off-site locations get. With it on, BombVault refuses to prune the repo itself, since there is no separate local copy behind it and the credentials on this box must not be able to delete the only copy of the backup.
    - **A growth-budget alarm**, sampled from the same repo-size trend the Storage card already tracks.

None of this is required: a hand-typed remote path with no saved safety settings backs up unlimited, prunable and without a budget alarm. The safety settings are there for when you want the protections an off-site copy gets, without a separate storage location just to get them.

!!! note "Cloud/REST credentials are shared"
    A remote primary authenticates with the shared cloud credentials under **Settings, Storage locations**; there is no separate credential store for primary repos.

### SMB and WebDAV without a host mount {#smb-webdav}

The **Off-site (rclone)** card under **Settings, Storage locations** has a form for a Windows or Samba share and for a WebDAV server (Nextcloud, ownCloud, SharePoint or any other). Fill in a short name, the host and share (SMB) or the URL and server type (WebDAV), the user and the password, and BombVault writes the rclone section for you. rclone obscures the password itself before it is stored; adding one with a name that already exists replaces that section instead of adding a second one. **Add storage location** offers SMB and WebDAV as providers with the same fields.

The form answers with the finished location, for example `rclone:nas:backups`. Use it as a section's path or a storage location and add a sub-folder if you want one (`rclone:nas:backups/bombvault`). The share is the first path segment, not part of the name.

This is the better route than mounting the share on Unraid: restic advises against keeping a repository on a mounted CIFS share, and here nothing is mounted. NFS is not in the form because neither restic nor rclone has an NFS backend; for NFS, mount the export on the host and use it as a local folder.

## Delete protection (append-only)

Switch on delete protection for an off-site storage location so ransomware, or a compromised host, cannot delete or rewrite your backups. The far side (a `restic/rest-server` running in `--append-only` mode) **enforces** it. BombVault only ever **verifies** it and never shows green on a configuration claim alone. A folder on this server cannot refuse deletion, because whoever controls BombVault controls the folder too, so local locations do not offer the switch.

The switch belongs to the storage location: a section that copies there takes it from the location, so turn it on from the location's page under **Storage locations**. The scheduled **Check delete protection** runs only while a section copies to a location with the switch on. The far side has to be started with `--append-only`; with the `restic/rest-server` image, on TrueNAS as on any other Docker host, that goes into the `OPTIONS` variable.

The fastest way to an append-only copy is a paired BombVault with a [receiving server](#receiving-server): it runs the rest-server for you, in append-only mode.

!!! note "A successful delete under `/locks/` is expected"
    Append-only does not mean nothing can ever be removed. restic has to take and release its own locks, so `/locks/` stays writable and deletable by design. Snapshots and the data behind them, which is what ransomware would go after, cannot be removed. If you probe the far side yourself, a delete that succeeds under `/locks/` is correct behaviour and not a hole in the protection.

!!! warning "Append-only repos are never pruned from this box"
    An append-only location never prunes old snapshots from here; pruning is the far side's job. Set a **growth-budget alarm** for it so you are alerted before the repo size runs away.

## Tamper test

BombVault periodically proves the append-only guarantee by actually attempting a delete against the off-site repo, aimed at a non-existent object. **Test delete protection now** on a location's page runs it on demand, and **Check delete protection** under **Settings, Verify, Automatic checks** runs it on a schedule.

A copy that is switched off is not tested, and the location says so. The connection test still runs on it, because it only reads, so a copy can be checked before it is switched on.

- **Refused** means protected.
- **Accepted** means not protected.
- An **inconclusive** result (server unreachable, auth error) never flips the stored verdict.

A real protected-to-unprotected flip fires a single alert.

## DR drills

BombVault offers two levels of proof that your backups are actually restorable, not just present.

- **Restore-verification drills (local).** BombVault periodically runs `restic check --read-data-subset` (bounded, never a disk-filling full restore). The cadence lives under **Settings, Verify, Automatic checks**, and the **Check backups** card on the same tile shows the result.
- **Trial restore (off-site).** BombVault restores a real target from the off-site repo into a sealed-off sandbox, verifies it file-for-file and byte-for-byte, then cleans up. This proves you can recover from off-site, not just that the repo answers.

The **Protection status** card on the overview rolls this up per section into four checks: a current backup on this server, a copy off-site, delete protection there and a trial restore from there. A click on a section shows the details, and the card only goes green on verified facts.

## Pairing instances {#pairing}

Several BombVaults work together as one group: they show each other's protection status, take on roles for each other and can keep each other's key. An instance joins the group with twelve words.

On the first instance open **Settings, Pairing** and press **Generate phrase**. Three step cards show how it goes. Twelve words appear in a window with a **Copy** button. On every other instance open the same place, press **Enter phrase** and paste or type them, or press **Paste** in that window. A word that is not on the list is named with its position as you type, and the last word carries a checksum, so a mistyped or swapped word is caught before anything pairs. Generate the phrase on one instance only: two instances that both generate one form two separate groups. If nobody shows up for a minute, the page offers two ways out: show the words again to enter them over there, or enter the other instance's words and join its group in one step. Pairing works without a login password, but set one: without it anyone who can open the web interface can read the words and, through the group, ask for roles that hand out the restic password of the instances in it. The pairing card says so until a password is set. With a password, showing the phrase again asks for it. **Leave group** takes an instance out again. The **Instances** card on the pairing page lists the members of the group; the picture of who does what for whom is on the Instances page.

Anyone who knows the words can join the group, so treat them like a password.

**How members reach each other.** Each instance learns its own address on the network from your browser the moment you sign in, listed under **Known addresses** in the **This instance** card on the pairing page together with the ones the instance found itself; correct it there if a reverse proxy or an unusual port sits in front. On the same network members announce that address by multicast and talk directly, and where multicast cannot cross a container network, such as Docker's default bridge, an instance instead searches its own subnet for the others with a signed call only a group member can answer, so pairing still finishes in seconds without a relay. If nothing turns up, **Can't find it?** takes one address by hand, for another subnet or a non-standard port. Instances on different networks go through a relay, picked in one row of four in the **Relay** card on the same page; each form comes with a picture:

- **Project relay** (the default): `parleyport.halleluja.design`, the relay KnightLoader uses as well. Nothing to set up.
- **Own relay**: the [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) container from the Unraid Community Apps (also under **Settings, Apps**). Enter its address on every instance that should use it.
- **BombVault instance**: one of your instances that is already reachable from outside, with **Serve as relay** switched on. That instance then answers at `/relay/connect` on its own address, behind the reverse proxy and certificate it already has, and lets in your group only. Enter its address on every instance that should use it.
- **No relay**: members find each other automatically on the same network, and nowhere else.

**What the relay sees.** A button of the same name beside the four forms shows this for the form you picked. Every call between members is sealed with AES-256-GCM under a key derived from the twelve words, and that key never leaves your instances. The relay learns a hash that groups the connections, which instance a message is for, how big it is and when it passes. A direct call on the local network is sealed the same way and signed as well, so nothing depends on the self-signed certificate an instance serves. Calls that hand something out (role answers, a restic password, receiver logins, key deposits) are also signed with a key of the member that sends them, pinned on first sight; see [Security model](configuration.md#security-model).

**What travels over the group.** The protection status on the Instances page, a request to check one section now, role requests and their answers, storage offers, and what a receiver or fetcher needs once its role is allowed: the other instance's repository locations and its restic password. Backup data never does; it still goes straight to the restic backends. Nor does the APP_KEY: the restic password opens that instance's repositories and nothing else, not its stored secrets, sessions or recovery codes.

**Entries from before pairing.** Instances added with a fleet token, and received repositories and pull sources set up with the other instance's APP_KEY, stay after the update and are marked **Pair again**. They keep working: at its first start BombVault replaces each stored APP_KEY with the restic password derived from it. Pair both instances, then edit the entry and choose its instance. Until then such entries stand in the **Without an instance** card on the Instances page.

The one place that still takes an APP_KEY by hand is [From an old backup folder](#restore-from-another-bombvault-repo), for the case where the other instance is gone and cannot answer in a group.

## Instances and roles {#roles}

The **Instances** page (a settings tile that can be pinned to the sidebar) shows the group as a net of who does what for whom, a card for this instance and a card for every other member, the Android phones and the ZFS servers. **Pairing** at the bottom right leads to the pairing page. Which parts are on is set under **Settings, Instances, Multiple servers**: **Instances**, **Receiver** and **Pull**.

Each card shows the member's protection status and its roles as tiles. A role is something one instance does for another:

- **Receiver** keeps this server's off-site copies in a rest-server in append-only mode. This server sends after every backup and cannot delete anything there, even if someone takes it over.
- **Fetcher** fetches this server's backups on its own schedule. This server does not have to be awake for it.
- **ZFS server** takes this server's ZFS replicas. Each ZFS entry picks it under **Replica** in its settings.
- **Key keeper** keeps a sealed copy of this server's recovery kit. See [Key keeper](#key-keeper).

Switching a role on sends a request, and nothing starts until someone on the other instance presses **Allow**. The request shows on that instance's card as a badge, and its **Details** list it with **Allow** and **Decline**; an allowed role can be revoked there at any time. A dashed line in the net is a role still waiting. An instance on BombVault 9.x cannot answer a request, so a role is active there as soon as you switch it on, and a request from a 9.x instance waits for **Allow** here.

**Details** on a card opens the rest: the received repositories and pull sources of that member, storage offers, monitoring, **Remove** and **Forget key**.

## Receiver and fetcher {#receiver}

![A received off-site copy in an instance's details, watched read-only, with an integrity check run on this hardware.](assets/screenshots/receiver.png)

*A received off-site copy in an instance's details, watched read-only, with an integrity check run on this hardware.*

On the box that **receives** append-only copies from another BombVault, the received repositories give you independent, read-only monitoring on the receiving hardware, so a silent failure at the far end does not go unnoticed. Switch on **Receiver** under **Settings, Instances**, then open the sending instance's **Details** on the Instances page and add the received repository (read-only, opened with the sending instance's restic password, which it gets over the [pairing group](#pairing)). You get:

- **An inventory by source**, so you can see exactly which containers, VMs and folder sets have landed.
- **Last received** per source, so you know how fresh each one is.
- **An independent `restic check`** run on the receiving hardware, so integrity is verified where the data actually sits, not only on the sender.
- **A dead-man's switch:** an alert when a source stops sending within a window you set.
- **Integrity alerts:** an alert when a check on the receiving side fails.

The receiver is strictly read-only. It never writes to the received repository, so it can never break the append-only guarantee the sender relies on.

A fetcher works the other way round: with **Pull** switched on, an instance reads another member's backups into its own repository on its own schedule. Its sources stand in that member's details as well.

### Receiving server {#receiving-server}

The receiving box can also run the rest-server the others copy to. It is set up the first time you allow a **Receiver** request that asks for it: the window says the server needs a receiving server and asks for a folder on a share, with **New folder** to create one, and a port (8000 unless another container uses it). BombVault then:

1. refuses if a container called `rest-server` already exists or another container holds the port;
2. pulls `restic/rest-server` and starts it through the Docker socket in append-only mode with private repositories and a login file in the folder;
3. writes its Unraid template to the flash drive, so the container stays editable in the Docker tab, or offers the template as a download when the flash drive is out of reach;
4. runs the tamper test against it and shows whether it refuses deletes.

The instances in your group then find the server among the providers under **From your group**, named after the receiving box, and the **Receiver** role offers it as the safe choice. Each instance gets a login of its own the first time it uses the server, and writes only into its own folder there. With **Receiver** switched on under **Settings, Instances**, **Receiving server** on this instance's card opens the server again. Its window lists the logins under **Partners with a login**, and **Revoke login** takes one away; what that instance already copied stays in the folder. Setup also creates one **Login outside the group**, whose password the window shows once.

An instance that reaches the receiving box only through the relay cannot use the server, because the relay carries no backups. Add the receiving box's address on the pairing page first. When BombVault runs on an IP address of its own (on br0, for example), fill in **Address for partners**, because the server listens on the host's address.

## Worked example: two Unraid boxes, end to end

Everything above describes the parts. This is one complete setup, because the parts are easier to assemble when you have seen them assembled once.

Two boxes: **TOWER** runs the containers and pushes backups; **VAULT** receives them and enforces append-only. Substitute your own names, addresses and share paths.

**1. Pair the two boxes.** On TOWER open **Settings, Pairing**, press **Generate phrase**, and enter the twelve words on VAULT under **Enter phrase**. Set a login password on both first.

**2. On TOWER, ask VAULT to be the receiver.** On the Instances page, switch on **Receiver** on VAULT's card, pick the sections and **VAULT's receiving server** as the place, and send the request.

**3. On VAULT, allow it.** VAULT's Instances page shows the request on TOWER's card. Press **Allow**. The first time, VAULT asks for a folder for its receiving server, such as `user/appdata/rest-server`, starts the rest-server in append-only mode and runs the tamper test against it.

**4. Back on TOWER.** From then on TOWER copies to VAULT after every backup, and the new storage location appears on TOWER's Storage locations page with its delete protection.

What the tamper test answers:

| Result | What happened |
| --- | --- |
| **protected** | VAULT refused the delete. This is the only passing state. |
| **not protected** | VAULT accepted a delete. `--append-only` is missing or was removed. |
| **inconclusive** | Neither. Usually the URL is not the one restic itself uses, or the credentials changed. Nothing is recorded and no alert fires. |

**5. Watch what arrives, if you want.** Switch on **Receiver** under **Settings, Instances** on VAULT, open TOWER's **Details** and add the received repository. VAULT gets TOWER's restic password over the group; nobody types a key.

**6. Make it mutual.** Repeat the steps in the other direction: VAULT asks TOWER to be its receiver. Each box then enforces append-only for the other, and neither can delete the other's backups.

A rest-server you run by hand works too: add it with **Add storage location** as a rest-server, with the user and password from its `.htpasswd`, and switch on delete protection. Keep `--append-only` in its options; without it the far side is an ordinary share again.

!!! warning "A local path is a path inside the container"
    Where BombVault asks for a folder on this server, enter it relative to the host mount, such as `user/appdata/rest-server/bombvault-containers`, not `/mnt/user/appdata/...`. BombVault runs in a container, where the host's `/mnt` is mounted elsewhere; an absolute host path does not exist inside it. If you paste one, BombVault tells you the relative path to use instead.

## Emergency wizard {#emergency}

The **Emergency wizard** brings back a server that is gone or was set up again. It starts on the **Restore** page under **Emergency**, and from **Restore from a backup** on the welcome page of a new install. It runs in four steps: **Access**, **Settings**, **Selection** and **Restore**.

1. **Access.** Show BombVault the key of the old backups, in one of three ways:
    - **Upload recovery kit**: the file you downloaded during setup. It holds the key and every storage location, and BombVault fills in the rest from it. A kit sealed with age asks for the identity it was sealed for; it is used once and not stored.
    - **Through the group**: an instance of your group keeps the key as a deposit. You need the group's twelve words, your recovery code and someone at that instance. See [Key keeper](#key-keeper).
    - **Type the key**: the old server's `APP_KEY`, and where its backups are.

    BombVault then tries the key on every storage location and says for each one whether it is readable, not found, locked with another key, not reachable or still missing its sign-in. Nothing has been saved yet, and the key stays in memory for a limited time. **Take the key over** makes this server write its backups with that key from now on; **Undo the takeover** goes back to its own key until the first backup has run. A server that already has backups of its own refuses the takeover and points to [From an old backup folder](#restore-from-another-bombvault-repo) instead. **Save sign-ins** stores the sign-ins the kit carries for remote storage locations, and asks before it replaces sign-ins this server already has.
2. **Settings.** **Bring back settings** restores BombVault's own settings from the newest self-backup: storage locations, credentials, schedules and rules. BombVault restarts to apply them, through the Docker socket, so the live settings database is never overwritten under an open handle, and the wizard carries on by itself afterwards. This server's login stays: you keep signing in with the password you use now. Secrets the old server had stored that cannot be taken over are listed, so you can enter them again.
3. **Selection.** BombVault looks through your backups and lists the containers, VMs, folder sets and ZFS datasets it finds. The newest snapshot is preselected everywhere. What this server still has starts switched off, because a restore would replace it. Folder sets no longer know where they came from after a rebuild, so they go into a folder you choose.
4. **Restore.** Everything selected comes back in one go, left stopped if you want to start each container and VM yourself, which is safest after a total loss. Afterwards download a new recovery kit: the old one belongs to the previous install.

!!! tip "Planned migration versus disaster"
    The Emergency wizard restores BombVault's own settings from a backup. For a *planned* move to a new box, you can instead carry your configuration over directly with the **Export / import settings** card (a portable JSON file). See [Configuration](configuration.md#portable-settings-export-and-import).

### From an old backup folder {#restore-from-another-bombvault-repo}

**From an old backup folder** under **Other sources** on the Restore page opens a *different* BombVault instance's repository (a share mounted under `/mnt`, or a remote address) with that instance's recovery kit or its typed `APP_KEY`, in a one-time, read-only session. Browse the containers, VMs, folder sets and ZFS datasets stored there, pick a snapshot and restore it, and the restored object becomes a normal local container, VM or folder set. Nothing is ever written to the other repository, and your own backup settings stay untouched (the session lives in memory and expires by itself). Moving a container from server A to server B does not mean repointing your settings and reverting them afterwards. This window is a one-shot: it opens a session, restores what you pick, and forgets the other instance. If you want a standing arrangement instead, where this box fetches another instance's snapshots into its own repository on a schedule, that is the **Fetcher** role on the Instances page.

A container whose network does not exist on this server, such as an Unraid `br0` network on a plain Docker host, shows a network picker under its row. BombVault creates it on the network you pick, together with its other networks. The fixed IP and MAC address belonged to the old network and are dropped, so the new network assigns them.

## Key keeper {#key-keeper}

A key keeper is an instance of your group that holds a sealed copy of this server's recovery kit. It cannot open the copy itself. If this server is ever lost, a new install that joins your group asks the keeper for the copy and opens it with your recovery code.

1. On the Instances page, switch on **Key keeper** on the card of the instance that should keep it, and choose **Make deposit**. Making a deposit needs a login password.
2. This server shows the **Recovery code** once: twenty letters and digits in four groups of five. Write it down and confirm with **I wrote the code down**. This server does not store the code, only what it needs to seal a deposit for it.
3. The keeper now holds the deposit, and this server renews it when the kit changes. **Key deposit** on this instance's card lists **This server's deposit** with each keeper and its state, and **Deposits this server keeps** for the others. **Make a new code** replaces the code; each keeper switches to the new one when it gets the new deposit.

On that day you need three things: the twelve words of your group, the recovery code, and access to the keeper's interface, where someone has to press **Allow**. In the Emergency wizard choose **Through the group**, pick the deposit, ask for it and type the recovery code. The code is used on the new server and is not sent to the keeper. On the keeper, the request shows under **Key deposit** with who asks, whether that instance was ever seen there before and how often it asked; **Allow** hands out the sealed copy, and allowing needs a login password. The new install has 30 minutes to collect it after the Allow, a request nobody answers runs out after a day, and an instance that asks too often has to wait an hour.

Without the code nobody can open the copy, the keeper included. If the code is lost, make a new one while this server still runs; if the server is already gone, the recovery kit and the typed APP_KEY still work. The keeper can delete a deposit at any time, and the owner then has to ask again.

## Recovery kit {#recovery-kit}

This is the piece that makes disaster recovery possible even when there is no running BombVault.

**Download recovery kit** in the **For emergencies** card under **Settings, Restore** gives you the **key**, the **derived restic password**, every **storage location** with the repository of each section, the sign-ins a new install needs to reach them, and the **exact restic commands**, so you can restore straight with the restic CLI on any machine. It needs a login password. The kit prints the key also with encryption switched off, because the container and VM definitions are always sealed with it. After a takeover it prints the taken-over backup key and this server's APP_KEY, which a new install needs only to bring back the settings. A **Save your recovery kit** card on the overview reminds you until you have stored it.

!!! danger "Store the recovery kit off the server"
    The kit contains the secret that decrypts your backups. Keep it somewhere safe and separate from the server (a password manager, a printed copy in a safe). If you lose both BombVault and `APP_KEY` with no recovery kit and no key deposit, your encrypted backups cannot be recovered.

!!! warning "The newest snapshot is not always the one to restore"
    Since restic 0.17, `restic snapshots` shows each snapshot's size. After data loss the newest snapshot can be the emptied one, so do not restore a snapshot that is far smaller than the ones before it. After ransomware it can be the encrypted one at the usual size. If BombVault still runs, check its **Anomalies** page first: it names the last good backup, and the restore window preselects it. A restore does not need any of BombVault's anomaly data, and the retention pause only ever keeps more snapshots.

### Sealing the kit

If you have switched on **Encrypt exports with age** under **Settings, System**, the kit is sealed with it too and downloads as `bombvault-recovery-kit.md.age`. It is ASCII-armored rather than binary, so it is still plain text: pasting it into a password manager or printing it works exactly as before, the contents are simply unreadable without your key.

!!! warning "Do not store the age key inside the kit"
    You need your age **private** key to open a sealed kit. Keep it somewhere that does not depend on the kit itself, or you will have two things to recover instead of one. Sealing is worth it when the kit is stored somewhere you do not fully control (a shared password manager, cloud notes, a printout in an office); a kit in your own safe is already protected by the safe.

    With encryption on and no usable recipient configured, the download is refused outright. BombVault never falls back to handing out the master key in the clear.

### If you do not have the kit to hand

The password is not stored anywhere, it is **computed** from the key of the backups, so you can reproduce it yourself with nothing but the key and a shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

That is HMAC-SHA256 over the fixed string `bombvault:restic-repo`, keyed with the raw bytes of the hex `APP_KEY`, printed as 64 lowercase hex characters. The same value is in the kit, listed as the derived restic password; this is for the day the kit is somewhere you are not. On a server that took over another one's backups, use the taken-over backup key, which is that server's old APP_KEY.

!!! warning "For a received repository, use the SENDING instance's key"
    A repository that arrived here through an off-site copy was created by the machine that sent it, with **its** key. Deriving from the receiving box's key produces a password restic will reject, which reads exactly like a corrupt repository and is not. This is the usual reason `restic check` on a received repo asks for a password over and over.

Because recovery definitions live **inside** each repo (`<repo>/def`, `<repo>/vm-def`), a copied repo folder is fully self-contained, so the kit plus the repo is everything a bare-metal restore needs.

## Getting a database dump back {#database-dumps}

A database dump is a restore point of its own in the containers repository, tagged `dbdump:<container>`, holding the single file `/dbdump/<container>.sql`. BombVault lists, downloads and imports them on the container's **Backups** tab; below are the same steps with restic alone, for the day BombVault is not there.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

The tags `dbversion:` and `dbname:` on each dump say which server version it came from and which databases it holds. A complete file ends with `-- PostgreSQL database cluster dump complete` or `-- Dump completed`.

Import it into a container of the same or a newer version (PostgreSQL), or the same major version (MySQL and MariaDB), started once with an empty data folder so it initialises. The host needs no database client, the container has one:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

For a single database out of a full dump, MySQL and MariaDB take `--one-database <name>` on the client command. A PostgreSQL dump has one section per database, each starting with a `\connect <name>` line: copy that section into its own file and import it with `-d <name>` after creating the database.

!!! warning "A root dump carries the server's users"
    A full MySQL or MariaDB dump taken as root contains the `mysql` system database, so importing it replaces the new server's accounts, including root's password, with the ones from the dump. On PostgreSQL, `role ... already exists` for the user the container created is expected and harmless.
