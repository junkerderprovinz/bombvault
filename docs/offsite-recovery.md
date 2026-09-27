# Off-site & recovery

Local backups protect you from a lost container or a bad update. Off-site replication and a tested recovery kit protect you from the whole box, ransomware, or a fire. This page covers replicating off-site, making that copy tamper-proof, proving you can restore, and recovering when BombVault itself is gone.

## Off-site replication

Keep the fast local backup and copy it to one or more other places. You choose the places a domain is copied to on the **Domains** card under **Settings, Storage**, one chip per place (see [Storage places](storage-places.md#domains)). BombVault copies new snapshots there with `restic copy` on a best-effort basis, so a failed copy never fails the local backup. The place a domain is stored in does not have to be local; see [A domain stored in a remote place](#remote-primary-repositories).

- **Several copy places per domain.** A domain can be copied to several places at once, for example a rest-server at a friend's house and a B2 bucket. Retention, storage class, append-only, limits and growth budget belong to the place, so each copy follows the rules of the place it lands in.
- **Per-domain copy schedule** (edited alongside every other schedule on Settings, Schedules): leave it blank to copy after every local backup, or set a cadence (for example `weekly Sun 03:00`) to copy less often than you back up. **Copy now** on the domain's row runs it on demand.
- **Retention per place.** Each place keeps its own rules, so an off-site place can keep copies longer as an archive. A place with every rule at zero never trims.
- **Bandwidth limits** per place cap the restic upload and download rate so copying does not saturate your WAN.
- A **replication indicator** shows which domain is copying while it runs (on its page and the Dashboard). It is an active indicator, not a percentage bar, because `restic copy` exposes no machine-readable progress.

!!! note "Restore from any place"
    Every container, VM, folder set, the flash and the app configuration list their backups as one timeline across all places a backup lies. A backup copied to B2 appears once, marked with each place that holds it. A restore takes the first place it can reach, starting with the repository the item is written to, and you can pick another place per row. Off-site places are read only when you open them. Deleting at one place first checks the others and says whether it was the last copy.

## Placement per item {#placement}

Each container, VM and folder set card has a **Placement** row with three segments:

- **Local** writes the item to the repository shown under **Stored on** and copies it nowhere. Use it for data that already has a second copy, for example a share that lives on a NAS.
- **Local + off-site** writes it there as well and copies it to the targets ticked under **Copy to**, one chip per off-site target of the domain. Untick a chip and that target gets nothing new from this item.
- **Off-site only** writes the item straight to the place under **Send to**, any place other than the domain's home. Where the domain is already copied to that place, the item gets a direct repository beside the copies; otherwise BombVault creates a repository for the domain there.

The location is fixed from the item's first backup on, because BombVault never moves backups between repositories. The copies can change at any time. A target that no longer gets an item keeps the copies it has and trims them to its own retention at the next off-site run of the domain; **Delete in B2** on the card removes them at once. When some of those copies exist nowhere else, the confirmation lists them by date and asks for the item's name. Append-only targets cannot be deleted from.

Under the row the card says where the item goes and what is actually there: how many sites hold it, when each target was last seen, and whether 3-2-1 is met. A site is the server with the original data and each place at another site (see [Off the premises](#off-the-premises-mark)). BombVault checks copies and sites; it does not check the "two media" part of 3-2-1.

### Defaults per domain

The **Domains** card under Settings, Storage has one row per domain. **Copied to** applies at once to every item without a choice of its own, and to the project folders of Compose stacks. Once a domain has backups, **Stored in** applies to a new item at its first backup, and changing it moves no backups. Before saving, the row names every place that gains or loses items and how many snapshots that means, and the question carries the switch **Apply to items without backups**, which also puts every item without a backup yet on the new default. **Exceptions** lists the items with a choice of their own.

Ticking a new place under **Copied to** makes it receive every item that is not set to Local. The confirmation says how many items and, where known, how much history that is.

### Direct repositories

Choosing a place under Off-site only where the domain is already copied asks once, then creates a direct repository beside the copies, for example `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, and points the item at it. For a copy target without a place, the choice opens a dialog with a suggested address and a connection test that creates nothing, and **Create and use** creates the repository. A direct repository takes the place's key, storage class, limits, append-only setting and retention, and changes with them. When a new key for the place cannot open it, the direct repository keeps the key it has and the save says so. Its snapshots carry the tag `bv:direct`, and every other retention pass keeps them, so a direct repository that lost its link to its place never ages by the local rules. A B2 key limited to one folder has to cover the place's address, not only the domain's folder, or the folder beside it is out of reach.

### Off the premises {#off-the-premises-mark}

A copy counts as a site of its own only when its place is at another site. A cloud place always counts and a folder on this Unraid never does; for a NAS, a rest-server or an SFTP server, answer **Where is the device?** in the place's details with **Here in the house** or **At another site**. The answer only counts sites and 3-2-1 on the cards and the Dashboard. It changes no copy.

### After a rebuild

Copy choices live in BombVault's own settings. After a rebuild through Discover without a restored `/config` they are gone, and copying everything would send the items you had left out to B2 again. Off-site replication of every rebuilt domain therefore pauses. The Dashboard shows it in amber, and the domain's row on the Domains card offers **Confirm default** with a preview of what the next run copies and the names in the backups that have no entry, which you can leave out there. Only the confirmation ends the pause; importing a settings file brings back rules and defaults but does not end it.

## A domain stored in a remote place {#remote-primary-repositories}

A domain does not have to be stored locally. While its backup location holds no backups, choose a remote place under **Stored in** on the Domains card and the domain backs up straight to it, with no local copy and no copy step. The remote repository is then the only copy unless the domain is also copied to another place. Every remote place comes with the same safeguards:

- **A connection test** before anything is written.
- **Bandwidth limits** for the backup itself, the same `--limit-upload` and `--limit-download` flags a copy uses.
- **Append-only protection**, verified with the same active tamper test. With it on, BombVault never prunes the repository, because the credentials on this box must not be able to delete the only copy of the backup.
- **A growth budget**, sampled from the same size trend the Storage card tracks.

A domain stored in a remote place is the source of its copies like a local one; see [Copies between places with different credentials](storage-places.md#different-credentials).

!!! note "Credentials belong to the place"
    A remote place keeps its own credentials. A place set up with the shared cloud credentials keeps using them until its access is changed in its details.

## Immutable (append-only) off-site

Flag an off-site repo append-only so ransomware, or a compromised host, cannot delete or rewrite your backups. The far side (a `restic/rest-server` running in `--append-only` mode) **enforces** it. BombVault only ever **verifies** it and never shows green on a configuration claim alone.

The **Add place** window carries a ready-to-paste recipe for a rest-server in append-only mode, with one user for this BombVault. At a rest-server place with **Append-only** on, **Test append-only** in the place's details runs the tamper test against every domain path, switched-on copy and repository at the place and gives one answer for the place, so append-only off-site is reachable without hand-editing configs.

!!! note "A successful delete under `/locks/` is expected"
    Append-only does not mean nothing can ever be removed. restic has to take and release its own locks, so `/locks/` stays writable and deletable by design. Snapshots and the data behind them, which is what ransomware would go after, cannot be removed. If you probe the far side yourself, a delete that succeeds under `/locks/` is correct behaviour and not a hole in the protection.

!!! warning "Immutable repos are never pruned from this box"
    An immutable off-site deliberately never prunes old snapshots. Set a **growth-budget alarm** for it so you are alerted before the repo size runs away.

## Tamper test

BombVault periodically proves the append-only guarantee by actually attempting a delete against the off-site repo, aimed at a non-existent object:

- **Refused** means protected.
- **Accepted** means not protected.
- An **inconclusive** result (server unreachable, auth error) never flips the stored verdict.

A real protected-to-unprotected flip fires a single alert.

At a place, **Test append-only** probes each domain path, switched-on copy and repository there with its own credentials and folds the verdicts into one answer: a single repository that accepts a delete makes the whole place *deletes accepted*.

## DR drills

BombVault offers two levels of proof that your backups are actually restorable, not just present.

- **Restore-verification drills (local).** BombVault periodically runs `restic check --read-data-subset` (bounded, never a disk-filling full restore) and shows a *last verified restorable* badge per domain. The cadence lives on Settings, Schedules; the badge on Settings, Integrity.
- **DR drills (off-site).** BombVault restores a real target from the off-site repo into a throwaway sandbox, verifies it file-for-file and byte-for-byte, then cleans up. This proves you can recover from off-site, not just that the repo answers. Only places at another site are drilled, since a copy in the same house proves nothing about losing the house. A domain copied to several of them is drilled against one per scheduled run, in turn, and the Dashboard names the place of the last drill.

The **ransomware-protection scorecard** on the Dashboard rolls this up into a green / amber / red posture per domain, with an age-stamped checklist (off-site configured, append-only verified, replication current, restore drill passed, encryption on, prune strategy set). Every red row deep-links to the fix, and the card only ever goes green on verified facts.

## Receiver dashboard (the receiving side)

![The receiving side, watched read-only, with an integrity check run on this hardware.](assets/screenshots/receiver.png)

*The receiving side, watched read-only, with an integrity check run on this hardware.*

Everything above is the *sending* side. On the box that **receives** immutable off-site copies from another BombVault, the Receiver dashboard gives you independent, read-only monitoring of those repositories on the receiving hardware, so a silent failure at the far end does not go unnoticed.

Turn on the **Receiver** toggle in Settings to reveal a **Receiver** tab. It is off by default; enable it only on a box that actually receives immutable off-site backups. Then register a received repository (read-only, opened with the sending instance's key) to get:

- **A snapshot inventory grouped by source**, so you can see exactly which containers, VMs and file sets have landed.
- **Last-received** per source, so you know how fresh each one is.
- **An independent `restic check`** run on the receiving hardware, so integrity is verified where the data actually sits, not only on the sender.
- **A dead-man's switch:** an alert when a source stops sending within a window you set.
- **Integrity alerts:** an alert when a check on the receiving side fails.

The Receiver is strictly read-only. It never writes to the received repository, so it can never break the append-only guarantee the sender relies on.

## Worked example: two Unraid boxes, end to end

Everything above describes the parts. This is one complete setup with real values, because the parts are easier to assemble when you have seen them assembled once.

Two boxes: **TOWER** runs the containers and pushes backups; **VAULT** receives them and enforces immutability. Substitute your own names, addresses and share paths.

**1. On VAULT, stand up the append-only server.** In BombVault on TOWER, open *Settings → Storage*, click **Add place**, pick **rest-server** and click **Show recipe**. Copy the **Unraid template** block, save it on VAULT as `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, then *Docker → Add Container* and pick **rest-server** from the template dropdown. Before starting it, write the shown `htpasswd` line into `/mnt/user/appdata/rest-server/.htpasswd` on VAULT. The password is shown once and never stored; the recipe has already put it and the user into the form on TOWER, so leave that window open. The `htpasswd` line carries the same password, bcrypt-hashed for you, so there is nothing for you to hash yourself.

    Leave `--append-only` in the OPTIONS field. Without it VAULT is just an ordinary share again.

**2. On TOWER, add the place.** Enter VAULT's address, `http://VAULT:8000`, next to the user and password the recipe filled in, then click **Test connection**. BombVault builds the address from them:

    rest:http://VAULT:8000/tower

The first path segment is the htpasswd user, here `tower`, and each domain gets its folder below it, for example `rest:http://VAULT:8000/tower/container`. Answer **Where is the device?** with **At another site**, click **Add**, and tick the place under **Copied to** for the domains that should go there.

**3. On TOWER, switch Append-only on** under **Protection** in the place's details, then click **Test append-only**. The test probes every domain path, copy and repository at the place and gives one answer for the place, which must be *deletes refused*. What the answers mean:

| Result | What happened |
| --- | --- |
| **deletes refused** | VAULT refused the delete. This is the only passing state. |
| **deletes accepted** | VAULT accepted a delete. `--append-only` is missing or was removed. |
| a message instead of a result | The test could not run. Usually the address is not the one restic itself uses, or the credentials changed. Nothing is recorded and no alert fires. |

**4. On VAULT, watch what arrives.** Turn on *Settings → Receiver*, open the **Receiver** tab and register the repository read-only.

!!! warning "The location is a path **inside** the container, written relative to the host mount"
    Enter `user/appdata/rest-server/tower/container`, **not** `/mnt/user/appdata/...`. BombVault runs in a container, where the host's `/mnt` is mounted elsewhere; an absolute host path does not exist inside it. If you paste one, BombVault tells you the relative path to use instead.

    The **Sending APP_KEY** is TOWER's key, not VAULT's. Find it on TOWER under *Settings → System*.

**5. Make it mutual, if you want.** Repeat the same five steps in the other direction: a rest-server on TOWER receiving VAULT's copy. Each box then enforces immutability for the other, and neither can delete the other's backups.

## Guided recovery

A dedicated **Recovery** tab walks a fresh or rebuilt install through the disaster case, in one place:

1. **Checks BombVault can read your backups** (the encryption-key gotcha up front).
2. **Restores BombVault's own settings**, so the backup paths, off-site targets and credentials the rest of the flow needs come pre-filled. It reads the settings backup from the place the Self-Backup row names under **Stored in**, or from the Self-Backup's copy under **Copied to**, and shows that place with its address; to read from another place, change the Self-Backup row in step 3 first. The restore is applied via a self-restart over the Docker socket, so the live settings database is never overwritten under an open handle.
3. **Attaches your existing backups** through the rows of the Domains card: on each domain's row, choose the place its backups lie on under **Stored in** and the places holding its copies under **Copied to**. A place no row offers yet, such as a share, a server or a cloud bucket, is connected with **Add place**, the same window as on Settings, Storage. **Connect & preview** then checks that the backups can be read.
4. **Discovers** the containers, VMs and file sets stored in it.
5. **Restores them all** (left stopped, so you start them deliberately), with your recovery kit one click away.

!!! note "Off-site copies wait after a rebuild"
    When step 4 rebuilds entries without the old settings, off-site replication of those domains pauses until the placement default is confirmed. See [Placement per item](#placement).

!!! tip "Planned migration versus disaster"
    Guided recovery restores BombVault's own settings from a backup. For a *planned* move to a new box, you can instead carry your configuration over directly with the **Export and import settings** card (a portable JSON file). See [Configuration](configuration.md#portable-settings-export-and-import).

### Restore from another BombVault repo

A separate card on the **Recovery** tab opens a *different* BombVault instance's repo (a share mounted under `/mnt`, or a remote URL) with **that instance's `APP_KEY`**, in a one-time, read-only session. Browse the containers, VMs and file sets stored there, pick a snapshot and restore it, and the restored object becomes a normal local container, VM or file set. Nothing is ever written to the other repo, and your own backup settings stay untouched (the session lives in memory and expires by itself). Moving a container from server A to server B no longer means repointing your repo settings and reverting them afterwards. This card is a deliberate one-shot: it opens a session, restores what you pick, and forgets the other instance. If you want a standing arrangement instead, where this box fetches another instance's snapshots into its own repository on a schedule, that is the **Pull** tab of the **Instances** page.

## Encryption-key recovery kit

This is the piece that makes disaster recovery possible even when there is no running BombVault.

One click downloads the **master key**, the **derived restic password**, and the **exact repo locations and commands**, so you can restore straight with the restic CLI on any machine. A Dashboard reminder nags until you have stored it.

!!! danger "Store the recovery kit off the server"
    The kit contains the secret that decrypts your backups. Keep it somewhere safe and separate from the server (a password manager, a printed copy in a safe). If you lose both BombVault and `APP_KEY` with no recovery kit, your encrypted backups cannot be recovered.

### If you do not have the kit to hand

The password is not stored anywhere, it is **computed** from `APP_KEY`, so you can reproduce it yourself with nothing but the key and a shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

That is HMAC-SHA256 over the fixed string `bombvault:restic-repo`, keyed with the raw bytes of the hex `APP_KEY`, printed as 64 lowercase hex characters. The same value is in the kit, listed as the derived restic password; this is for the day the kit is somewhere you are not.

!!! warning "For a received repository, use the SENDING instance's key"
    A repository that arrived here through off-site replication was created by the machine that sent it, with **its** `APP_KEY`. Deriving from the receiving box's key produces a password restic will reject, which reads exactly like a corrupt repository and is not. This is the usual reason `restic check` on a received repo asks for a password over and over.

Because recovery definitions live **inside** each repo (`<repo>/def`, `<repo>/vm-def`), a copied repo folder is fully self-contained, so the kit plus the repo is everything a bare-metal restore needs.
