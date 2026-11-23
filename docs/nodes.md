# Nodes

A node is a BombVault without a web interface, for example on a VPS or on a small server at a friend's place. It backs up the containers and folders on its own server, and the BombVault you add it on, called **home** on this page, tells it when to back up and where the backups go. You look after it from home's web interface, next to your own entries.

![The card of the node vps-falkenstein on the Instances page: Connected, its version, its backups on the receiving server, when each section backs up, and Edit placement, Replace and Release.](assets/screenshots/nodes/card-dark.png#only-dark)
![The card of the node vps-falkenstein on the Instances page: Connected, its version, its backups on the receiving server, when each section backs up, and Edit placement, Replace and Release.](assets/screenshots/nodes/card-light.png#only-light)

## How it works {#how-it-works}

- **Home holds the node's key.** When you add a node, home makes its `APP_KEY`, keeps a sealed copy and hands it to you once, in the node's compose file. With that copy home reaches the node and opens its backups.
- **The node needs no open port.** Home and the node talk over the relay through a private link of their own, apart from your pairing group. Home starts every call and signs it, and the node asks home for nothing but its hello. The only placement that needs an open port is [keeping the backups on the node](#keep-on-node).
- **Backups never cross the relay.** The node sends them straight to the place you chose: home's receiving server, one of home's storage locations, or a rest-server on the node itself.
- **The node plans nothing itself.** It runs no schedules, sends no notifications and joins no group or fortress. Home starts its backups, watches how they go and sends the alerts.
- **A node backs up containers and folder sets.** VMs, the flash drive, ZFS datasets and the self-backup stay with full installs.

The link uses home's own relay when its group has one, and the project relay otherwise, even when home pairs without a relay.

## Before you start {#before-you-start}

- **A login password on home.** Adding a node, showing its compose file again and opening its entries need a sign-in with the password, see [Login](configuration.md#login).
- **Docker with Compose on the node's server.**
- **A place for the backups**, one of the three under [Where the backups go](#placement). The receiving server is the safest choice, because the node cannot delete anything there.

## Adding a node {#add}

1. On home, open **Instances** and choose **Add node** at the bottom right.
2. **Name.** Type how the node shows up here, such as the server's name, and choose **Next**.

    ![The Add node window at its first step, with the name vps-helsinki typed in.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![The Add node window at its first step, with the name vps-helsinki typed in.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Backups.** Pick where the backups go and fill in what that place asks for. **Backs up daily at** is when both sections start. Home suggests half an hour after its own container backups, so the two do not reach for the same disks at once. You can give each section its own plan on the card later. Choose **Next**.

    ![The second step: the receiving server picked, Address for nodes filled in with an https address, and Backs up daily at 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![The second step: the receiving server picked, Address for nodes filled in with an https address, and Backs up daily at 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Start.** The window shows the node's compose file with `MODE`, `APP_KEY`, `NODE_HOME` and `NODE_NAME` filled in. Copy it and keep the `APP_KEY` somewhere safe. On the node's server:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Before you start it, point the Host Data volume at the data you want backed up and set `HOST_SOURCE_ROOT` to the same path, as the example below shows.

5. The window waits for the node. Once the node answers, home tells it where its backups go and checks that it reaches them, and the window says **vps-helsinki is online and takes its orders from this instance.**

    ![The last step: the compose file, then the node online and its backups placed on the receiving server.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![The last step: the compose file, then the node online and its backups placed on the receiving server.](assets/screenshots/nodes/add-start-light.png#only-light)

You can close the window before the node answers. Its card then says **Waiting for the node**, and home tells it where its backups go once it shows up.

### Example: a VPS that backs up to the receiving server {#example-vps}

Home is an Unraid box called `tower` that runs the [receiving server](offsite-recovery.md#receiving-server). A reverse proxy publishes it as `https://backup.tower.example.org`. The VPS `vps-helsinki` runs Nextcloud and Caddy with their data under `/srv/appdata`.

1. On `tower`, **Instances**, **Add node**, name `vps-helsinki`, **Next**.
2. Pick **This instance's receiving server** and type `https://backup.tower.example.org` under **Address for nodes**. Home's containers back up at 02:30, so the window offers 03:00. Keep it and choose **Next**.
3. On the VPS, save the file from the window with the volumes and `HOST_SOURCE_ROOT` changed for `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. A few seconds later the window on `tower` says the node is online and **Backups go to https://backup.tower.example.org.**
5. At 03:00 home starts the node's backups. The next morning the **Nodes** card on the overview shows the last backup of `vps-helsinki`, and **Backups** lists its containers in a section of their own.

How a container's data is found on a plain Docker host is described under [Backup source detection](configuration.md#backup-source-detection).

## Where the backups go {#placement}

Each node has one placement for both its sections. **Edit placement** on its card changes it once the node is online. The new place gets new repositories, and the backups at the old place stay where they are.

### This instance's receiving server {#to-receiver}

The node writes to home's receiving server in append-only mode: it can add backups but cannot delete any. It gets a login of its own, named after the node, and one repository per section in its own folder there. Home adds its own key to each repository and tidies them up itself, since the node cannot.

If home runs no receiving server yet, the step says so and offers **Set up receiving server**. Home needs no group for it. Pick a folder on a share and a port, choose **Set up and continue**, then **Done** once the window shows the server, and the step goes on with it. The details are under [Receiving server](offsite-recovery.md#receiving-server).

The node reaches the server at **Address for nodes**, which you set in the second step or in the **Receiving server** window. A node usually sends from the internet, so the address has to be one of these:

- `https://`, for example behind a reverse proxy, Pangolin or a tunnel. When the server shows a certificate of its own instead of one from a public authority, paste it under **Certificate** in PEM form.
- `http://` with a private IP address, such as `10.0.0.5` on a WireGuard link or `100.101.102.103` on Tailscale.

Plain `http://` to a public address is refused, because the node would send its backups across the internet unencrypted. When you change the address, home tells every node on the receiver, and a node that is away hears it once it is back.

### A storage location {#to-location}

The node writes to one of home's storage locations that it can reach itself: an S3 service or a REST server, over `https://` or at a private address. Its repositories lie in `bombvault-node-<node id>-containers` and `bombvault-node-<node id>-files` there.

The node signs in with a [saved sign-in](offsite-recovery.md#sign-ins) made for it, which has to differ from the one home writes to the location with. Add one first if the window says there is none. With that sign-in the node can usually delete what lies at the location, unlike on the receiving server, and the window says so.

### Keep them on the node {#keep-on-node}

The node keeps its backups itself, behind a rest-server of its own in append-only mode, and home fetches a copy of each section into its own storage. This suits a node that home cannot offer a place to. It is the one placement that needs an open port.

![The second step with Keep them on the node picked, the address https://203.0.113.7:8443 and the note about the open port.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![The second step with Keep them on the node picked, the address https://203.0.113.7:8443 and the note about the open port.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Under **Address of the node**, type where home reaches the node: its public address and a free port, such as `https://203.0.113.7:8443`, with no folder after it. Use `http://` only with a private address, as above.
2. The node starts the container `bombvault-node-rest-server` on that port, with its repositories in the folder `bombvault-node` under Host Data. With `https://` it makes a certificate for that address, and home pins it.
3. Let the port through the node's firewall, so home can reach it. A port that another container on the node holds is refused with its name.

Home fetches a section right after the node's backups of it end, and every hour at a quarter past as a fallback, also with **Pull** switched off. What home fetched stays with home like any [fetched copy](offsite-recovery.md#receiver), even after the node is gone. The node tidies up its own repositories when home asks, since its rest-server takes no deletes, and its folder sets leave the `bombvault-node` folder out, so it never backs up its own backups.

### Off-site copies with home's {#copy-onward}

Switch on **Copy off site with this instance's backups** in the **Backups** step or under **Edit placement**, and home copies the node's containers and folder sets to the storage locations its own Containers and Folders copy to, on the same plan. The node needs no folder and no copy plan of its own for this.

- The copies lie in the same repositories as home's copies, under the node's host. Home's rules at the location leave them alone, and home keeps them by the node's rule, the same one it [tidies up the node's backups](#schedule) by.
- Home reads the node's repositories with its own key and writes with its own sign-in for the location. The node gets no new credentials, and the receiving server stays append-only. For a node that keeps its backups itself, home copies what it fetched.
- The node's repositories use chunker settings of their own, so their copies share little data with home's at the location.
- At an append-only location nothing ages, as with home's own copies. A [shared repository](offsite-recovery.md#shared-repository) gets no copies of a node, because its owner ages it by the rules of the group's servers.
- The card shows **Copies go to** and the locations. A copy that fails is an error on the card and sends an alert, see [Watching your nodes](#watch).
- Switched off, home copies nothing more. The copies already there stay, and home's rules still leave them alone.

#### Example: a VPS copied to B2 with home's containers {#example-copy-onward}

`tower` keeps 7 backups of each container under **How long to keep** and copies its Containers to a B2 bucket every night at 04:00. The bucket keeps the last 14. `vps-helsinki` backs up to tower's receiving server, where its backups age by the same 7. With the switch on, tower copies the VPS's containers to the bucket at 04:00 as well. The bucket then holds 14 of tower's backups of each container and 7 of each of the VPS's.

## When nodes back up {#schedule}

Home owns every node's schedule. The card shows a plan per section, such as **Containers** *Daily at 2:00* and **Folders** *Weekly (Sun) at 4:00*. A click opens the same editor as under [Schedules](schedules.md#plans), and a change is saved at once.

![The plan of a node's containers open from its card, with Off, Daily, Weekly, Every N days and Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![The plan of a node's containers open from its card, with Off, Daily, Weekly, Every N days and Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **A node that was away** starts the backups it missed as soon as it is back. Home looks every five minutes.
- **A node that is busy** gets asked again five minutes later.
- **While home tidies up** a node's old backups, the node starts nothing until home is done. On the receiving server and on the node itself this window opens every Sunday at 03:00 and lasts up to six hours. At a storage location it follows home's off-site copy plan for that location. Home keeps the backups by its own rule for the section under **How long to keep**, or by the location's rule where it has one.
- **In the node's run list**, the backups and restores home started show as started by home.

## Watching your nodes {#watch}

The **Nodes** card on the overview lists every node with its state, its last backup, its errors and its open anomalies. It shows up once you have a node.

![The Nodes card on the overview with three nodes: one connected, one inactive with two errors, and one still waiting for the node.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![The Nodes card on the overview with three nodes: one connected, one inactive with two errors, and one still waiting for the node.](assets/screenshots/nodes/overview-card-light.png#only-light)

Home sends one alert per episode through its [notification channels](notifications.md), and the next only after that episode is over:

- **A backup failed**, such as *BombVault: vps-helsinki: backup failed*, with the entry and the reason. A later backup of that entry that works ends the episode.
- **The node is out of reach**, such as *BombVault: vps-helsinki is out of reach*. Home waits for the shortest dead man's switch on the node's repositories, 26 hours unless you change it, or 6 hours while the node has no placement yet.
- **An off-site copy failed**, such as *BombVault: vps-helsinki: off-site copy failed*, when home could not copy the node's backups to one of its storage locations. The next copy that goes through ends the episode.

Home's receiver watch also looks after each repository of a node, with a dead man's switch and a daily `restic check` at 04:00, see [Receiver and fetcher](offsite-recovery.md#receiver).

## A node's entries {#entries}

Under **Backups**, the Containers and Folders lists have a section for each node below your own entries. The node's own container is left out. **Back up now** in an entry's menu starts its backup on the node, and **Open** opens its page.

In a node's section of the Folders list, **Add folder set** adds a folder set on the node, and **Edit** in a set's menu changes one. Both open the same window as for your own sets. Its folder picker browses the node's folders but cannot make new ones there.

![The section of the node vps-falkenstein in the Containers list: nextcloud, nextcloud-db, caddy and uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![The section of the node vps-falkenstein in the Containers list: nextcloud, nextcloud-db, caddy and uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

The page of a node's entry says **On** and the node's name, and lists its backups. **Restore** on one of them restores the entry on the node and replaces what is there now. A container is stopped for it, as at home.

![The page of nextcloud on vps-falkenstein with its backups and a Restore button beside each.](assets/screenshots/nodes/entry-dark.png#only-dark)
![The page of nextcloud on vps-falkenstein with its backups and a Restore button beside each.](assets/screenshots/nodes/entry-light.png#only-light)

Through home you reach a fixed list of the node's own routes: its containers, folder sets, runs and anomalies, and backing up and restoring them. The node's settings, keys and storage stay out of reach, and nothing on that list deletes. See [Nodes in the API](api.md#nodes).

## Replacing and releasing {#replace-release}

**Replace** shows the node's compose file again, after a sign-in with the password. Use it to move the node to another server or to rebuild it: start the file there and shut the old server down. The same `APP_KEY` and `NODE_HOME` make it the same node again, with its backups and its history. Home keeps each node's folder sets, its containers with what it takes to recreate them, and its runs. When the node starts with an empty `config` folder, home takes it on again, tells it where its backups go and hands all of that back before the next backup, so the node can restore a container that is gone. On a new server, a node that keeps its backups itself starts with empty repositories. Its earlier backups stay in the copies home fetched. Two servers with the same `APP_KEY` count as one node, so never run both.

**Release** lets a node go. It stops taking orders from home and backs nothing up any more. Its backups stay where they are, and so does every copy home fetched. Its login on the receiving server goes, and home stops watching its repositories. A node that is away when you release it keeps its card as **Being released** until it hears it. A released node takes no home again with the same `APP_KEY`. To use the server again, add it as a new node and start it with the new compose file and an empty `config` folder.

## Security {#security}

If home is taken over, the attacker controls its nodes too, and a node holds Docker on its server, which is root there. Protect home with a password and keep it off the internet. If a node is taken over, the attacker gets no pairing group, no fortress, no other node and no other repository, and cannot delete history on the receiving server. The details are under [Nodes in the security model](configuration.md#nodes).

The [recovery kit](offsite-recovery.md#recovery-kit) has a chapter **Nodes** with each node's `APP_KEY`, its restic password and its repositories, so you can bring a node back or restore its backups with plain restic.
