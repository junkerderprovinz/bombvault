# Off-site & recovery

Local backups protect you from a lost container or a bad update. Off-site replication and a tested recovery kit protect you from the whole box, ransomware, or a fire. This page covers replicating off-site, making that copy tamper-proof, proving you can restore, and recovering when BombVault itself is gone.

## Off-site replication

Keep the fast local backup and add one or more off-site replicas. Set a repo per domain on the **Settings, Off-site** tab. BombVault replicates new snapshots there with `restic copy` on a best-effort basis, so an off-site hiccup never fails the local backup. In this shape the local repo stays primary and the off-site repo is a replica — but a domain's primary repo does not have to be local at all; see [Remote primary repositories](#remote-primary-repositories) below for backing up straight to S3/rest-server/etc. instead of replicating to it.

- **Multiple off-site targets per domain.** Each domain (containers, VMs, flash, config and file sets) can replicate to several off-site destinations at once, not just one, so you can keep, for example, a rest-server on a friend's box and an S3 bucket in parallel. Add extra targets on Settings, Off-site, each with its own repository, S3 storage class, append-only flag, retention and growth budget. An existing single off-site setup is carried over untouched as the first target, and every target of a domain replicates on that domain's off-site schedule.
- **Per-domain off-site schedule** (edited alongside every other schedule on Settings, Schedules): leave it blank to replicate after every local backup, or set a cadence (for example `weekly Sun 03:00`) to ship off-site less often than you back up locally. A **Replicate now** button covers on-demand runs.
- **Off-site retention** lives on Settings, Off-site so you can keep off-site copies longer as an archive. Leave the policy all-zero to never auto-trim off-site snapshots.
- **Bandwidth limits** (Settings, Off-site) cap the restic upload/download rate so replication does not saturate your WAN.
- A **replication indicator** shows which domain is replicating while it runs (on its page and the Dashboard). It is an active indicator, not a percentage bar, because `restic copy` exposes no machine-readable progress.

!!! note "Restore straight from off-site"
    Every backup browser has a **Local / Off-site** switch, so if a local repo is lost or corrupt you can list and restore directly from the off-site replica. Delete is per-source: removing a backup only affects the copy you are viewing.

## Remote primary repositories {#remote-primary-repositories}

A domain's Backup Path (Settings, Paths & Storage) is not limited to a local folder — point it straight at a restic remote (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) and BombVault backs up to it directly, with no separate local copy and no replication step. This is a genuinely different shape from off-site replication above: there the local repo is primary and the off-site repo is a best-effort archive of it; here the remote repo **is** the primary, and it is the only copy unless you also configure off-site replication (or a second remote) for that domain.

Each of the five path fields (Containers, VMs, Flash, Config, Files) has an inline **Local / Remote** switch right next to it:

- **Local** shows the familiar folder browser.
- **Remote** swaps it for a plain URL field, plus a button that opens the same connection-test/credentials dialog off-site destinations use, configured for this primary instead. From there you get:
    - **A connection test** against the live path, before you rely on it.
    - **Bandwidth limits** (upload/download) so a scheduled backup to a remote primary does not saturate your WAN — the same `--limit-upload`/`--limit-download` restic flags off-site replication uses, applied to the backup itself.
    - **Append-only (immutable) protection**, verified with the same active tamper test (a real DELETE probe against the far side) off-site destinations get. With it on, BombVault refuses to prune the repo itself — since there is no separate local copy behind it, the credentials on this box must not be able to delete the only copy of the backup.
    - **A growth-budget alarm**, sampled from the same repo-size trend the Storage card already tracks.

None of this is required: a hand-typed remote path with no saved safety settings backs up exactly as it always has (unlimited bandwidth, prunable, no budget alarm) — the safety dialog is there for when you want the same protections an off-site copy gets, without needing a separate off-site destination just to get them.

!!! note "Cloud/REST credentials are shared"
    A remote primary authenticates with the same S3/REST credentials configured under Settings, Off-site, Cloud credentials — there is no separate credential store for primary repos.

## Immutable (append-only) off-site

Flag an off-site repo append-only so ransomware, or a compromised host, cannot delete or rewrite your backups. The far side (a `restic/rest-server` running in `--append-only` mode) **enforces** it. BombVault only ever **verifies** it and never shows green on a configuration claim alone.

The **guided off-site setup** wizard walks you from backend choice (rest-server / rclone / S3) through a ready-to-paste rest-server deploy snippet, a connection test, the immutable toggle (which runs the tamper test immediately) and a retention strategy, so append-only off-site is reachable without hand-editing configs.

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

## DR drills

BombVault offers two levels of proof that your backups are actually restorable, not just present.

- **Restore-verification drills (local).** BombVault periodically runs `restic check --read-data-subset` (bounded, never a disk-filling full restore) and shows a *last verified restorable* badge per domain. The cadence lives on Settings, Schedules; the badge on Settings, Integrity.
- **DR drills (off-site).** BombVault restores a real target from the off-site repo into a throwaway sandbox, verifies it file-for-file and byte-for-byte, then cleans up. This proves you can recover from off-site, not just that the repo answers.

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

**1. On VAULT, stand up the append-only server.** In BombVault on TOWER, go to *Settings → Off-site → guided setup*, pick **rest-server**, and generate the deploy recipe. Copy the **Unraid template (XML)** tab, save it on VAULT as `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, then *Docker → Add Container* and pick **rest-server** from the template dropdown. Before starting it, write the shown `htpasswd` line into `/mnt/user/appdata/rest-server/.htpasswd` on VAULT. The one-time password is displayed once and never stored, so copy it now.

    Leave `--append-only` in the OPTIONS field. It is the whole point: without it VAULT is an ordinary share again.

**2. On TOWER, point the off-site repo at it.** The repo URL follows the pattern the recipe prints:

    rest:http://VAULT:8000/bombvault-containers/containers

The first path segment is the htpasswd user, the second is the repository. Enter the generated user and password as the destination's REST credentials, then run the **connection test**.

**3. On TOWER, turn on Immutable.** The tamper test runs immediately and must say *protected*. What the answers mean:

| Result | What happened |
| --- | --- |
| **protected** | VAULT refused the delete. This is the only passing state. |
| **NOT protected** | VAULT accepted a delete. `--append-only` is missing or was removed. |
| **inconclusive** | Neither. Usually the URL is not the one restic itself uses, or the credentials changed. Nothing is recorded and no alert fires. |

**4. On VAULT, watch what arrives.** Turn on *Settings → Receiver*, open the **Receiver** tab and register the repository read-only.

!!! warning "The location is a path **inside** the container, written relative to the host mount"
    Enter `user/appdata/rest-server/bombvault-containers/containers`, **not** `/mnt/user/appdata/...`. BombVault runs in a container, where the host's `/mnt` is mounted elsewhere; an absolute host path does not exist inside it. If you paste one, BombVault now tells you the relative path to use instead.

    The **Sending APP_KEY** is TOWER's key, not VAULT's. Find it on TOWER under *Settings → System*.

**5. Make it mutual, if you want.** Repeat the same five steps in the other direction: a rest-server on TOWER receiving VAULT's copy. Each box then enforces immutability for the other, and neither can delete the other's backups.

## Guided recovery

A dedicated **Recovery** tab walks a fresh or rebuilt install through the disaster case, in one place:

1. **Restores BombVault's own settings first**, so the backup paths, off-site targets and credentials the rest of the flow needs come pre-filled (applied via a self-restart over the Docker socket, so the live settings database is never overwritten under an open handle).
2. **Checks BombVault can read your backups** (the encryption-key gotcha up front).
3. Lets you **point at your existing repo** (local or off-site).
4. **Discovers** the containers, VMs and file sets stored in it.
5. **Restores them all** (left stopped, so you start them deliberately), with your recovery kit one click away.

!!! tip "Planned migration versus disaster"
    Guided recovery restores BombVault's own settings from a backup. For a *planned* move to a new box, you can instead carry your configuration over directly with the **Export and import settings** card (a portable JSON file). See [Configuration](configuration.md#portable-settings-export-and-import).

### Restore from another BombVault repo

A separate card on the **Recovery** tab opens a *different* BombVault instance's repo (a share mounted under `/mnt`, or a remote URL) with **that instance's `APP_KEY`**, in a one-time, read-only session. Browse the containers, VMs and file sets stored there, pick a snapshot and restore it, and the restored object becomes a normal local container, VM or file set. Nothing is ever written to the other repo, and your own backup settings stay untouched (the session lives in memory and expires by itself). Moving a container from server A to server B no longer means repointing your repo settings and reverting them afterwards. Live server-to-server federation is explicitly out of scope; this is a deliberate one-shot pull.

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
