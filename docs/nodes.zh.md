# 节点

节点是没有网页界面的 BombVault，例如运行在 VPS 上，或者朋友家里的一台小服务器上。它备份自己服务器上的容器和文件夹，而什么时候备份、备份发往哪里，由你添加它的那个 BombVault 决定，本页把它称为**主实例**。你在主实例的网页界面里照看节点，就在你自己的条目旁边。

![实例页面上节点 vps-falkenstein 的卡片：已连接、它的版本、发往接收服务器的备份、每个分区何时备份，以及更改目标、替换和释放。](assets/screenshots/nodes/card-dark.png#only-dark)
![实例页面上节点 vps-falkenstein 的卡片：已连接、它的版本、发往接收服务器的备份、每个分区何时备份，以及更改目标、替换和释放。](assets/screenshots/nodes/card-light.png#only-light)

## 工作原理 {#how-it-works}

- **主实例持有节点的密钥。** 添加节点时，主实例为它生成 `APP_KEY`，保留一份封存的副本，并通过节点的 compose 文件把它交给你一次。主实例凭这份副本访问节点并打开它的备份。
- **节点无需开放端口。** 主实例和节点通过中继，用一条独立于你的配对组的专用私有连接通信。每次调用都由主实例发起并签名，节点只向主实例发送它的问候。唯一需要开放端口的去向是[把备份保留在节点上](#keep-on-node)。
- **备份从不经过中继。** 节点把备份直接发往你选择的地方：主实例的接收服务器、主实例的某个存储位置，或者节点自己的 rest-server。
- **节点自己不做任何计划。** 它不运行计划，不发送通知，也不加入任何组或堡垒。由主实例启动它的备份、关注备份的进展并发出提醒。
- **节点备份容器和文件夹集。** 虚拟机、闪存、ZFS 数据集和自我备份只由完整安装负责。

这条连接在主实例的组有自己的中继时使用它，否则使用项目的中继，即使主实例配对时不用中继也是如此。

## 开始之前 {#before-you-start}

- **主实例上的登录密码。** 添加节点、再次显示它的 compose 文件以及打开它的条目，都需要用密码登录，参见[登录](configuration.md#login)。
- **节点的服务器上装有带 Compose 的 Docker。**
- **一个存放备份的地方**，即[备份发往哪里](#placement)下的三种之一。接收服务器最安全，因为节点在那里什么也删不掉。

## 添加节点 {#add}

1. 在主实例上打开**实例**，选择右下角的**添加节点**。
2. **名称。** 输入节点在这里显示的名称，例如服务器的名字，然后选择**下一步**。

    ![添加节点窗口的第一步，已输入名称 vps-helsinki。](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![添加节点窗口的第一步，已输入名称 vps-helsinki。](assets/screenshots/nodes/add-name-light.png#only-light)

3. **备份。** 选择备份发往哪里，并填写那个地方需要的信息。**每天备份时间**是两个分区开始的时间。主实例建议在它自己的容器备份半小时之后，这样两者不会同时去抢同一批磁盘。之后你可以在卡片上为每个分区单独设置计划。选择**下一步**。

    ![第二步：选中了接收服务器，节点使用的地址填了一个 https 地址，每天备份时间为 03:00。](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![第二步：选中了接收服务器，节点使用的地址填了一个 https 地址，每天备份时间为 03:00。](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **启动。** 窗口显示节点的 compose 文件，其中已填好 `MODE`、`APP_KEY`、`NODE_HOME` 和 `NODE_NAME`。把它复制下来，并把 `APP_KEY` 妥善保存。在节点的服务器上执行：

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    启动之前，把 Host Data 卷指向你要备份的数据，并把 `HOST_SOURCE_ROOT` 设为同一路径，如下面的示例所示。

5. 窗口会等待节点。节点一应答，主实例就告诉它备份发往哪里，并检查它能否访问到那里，然后窗口显示 **vps-helsinki 已上线，并接受此实例的指令。**

    ![最后一步：compose 文件，然后是已上线的节点，它的备份放在接收服务器上。](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![最后一步：compose 文件，然后是已上线的节点，它的备份放在接收服务器上。](assets/screenshots/nodes/add-start-light.png#only-light)

你可以在节点应答之前关闭窗口。这时它的卡片显示**正在等待节点**，等节点出现后，主实例再告诉它备份发往哪里。

### 示例：备份到接收服务器的 VPS {#example-vps}

主实例是一台叫 `tower` 的 Unraid 机器，运行着[接收服务器](offsite-recovery.md#receiving-server)。一个反向代理把它发布为 `https://backup.tower.example.org`。VPS `vps-helsinki` 运行着 Nextcloud 和 Caddy，数据在 `/srv/appdata` 下。

1. 在 `tower` 上依次选择**实例**、**添加节点**，名称填 `vps-helsinki`，然后**下一步**。
2. 选择**此实例的接收服务器**，在**节点使用的地址**里输入 `https://backup.tower.example.org`。主实例的容器在 02:30 备份，所以窗口建议 03:00。保持不变，选择**下一步**。
3. 在 VPS 上保存窗口里的文件，把卷和 `HOST_SOURCE_ROOT` 改成 `/srv/appdata`：

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

4. 执行 `docker compose up -d`。几秒钟后，`tower` 上的窗口显示节点已上线，以及 **备份发往 https://backup.tower.example.org。**
5. 03:00 主实例启动节点的备份。第二天早上，概览上的**节点**卡片显示 `vps-helsinki` 的上一次备份，**备份**页面把它的容器列在单独的一个分区里。

在普通的 Docker 主机上如何找到容器的数据，见[备份源检测](configuration.md#backup-source-detection)。

## 备份发往哪里 {#placement}

每个节点的两个分区共用一个去向。节点上线后，可以用它卡片上的**更改目标**来改。新的地方会有新的仓库，旧地方的备份留在原处。

### 此实例的接收服务器 {#to-receiver}

节点以 append-only 模式写入主实例的接收服务器：它可以添加备份，但删不掉任何备份。它在那里有一个以节点命名的专属登录，并在自己的文件夹里每个分区各有一个仓库。主实例把自己的密钥加到每个仓库里，并亲自整理它们，因为节点做不到。

如果主实例还没有运行接收服务器，这一步会说明并提供**设置接收服务器**。主实例不需要组也能设置。选择共享上的一个文件夹和端口，点**设置并继续**，窗口显示服务器后再点**完成**，这一步就会用它继续。详情见[接收服务器](offsite-recovery.md#receiving-server)。

节点通过**节点使用的地址**访问服务器，这个地址在第二步或**接收服务器**窗口里设置。节点通常从互联网发送数据，所以地址必须是下面两种之一：

- `https://`，例如放在反向代理、Pangolin 或隧道后面。如果服务器出示的是自己的证书而不是公共证书机构签发的证书，请把它以 PEM 格式粘贴到**证书**里。
- 带私有 IP 地址的 `http://`，例如 WireGuard 链路上的 `10.0.0.5` 或 Tailscale 上的 `100.101.102.103`。

指向公网地址的普通 `http://` 会被拒绝，因为节点会以未加密的方式把备份发过互联网。你更改地址后，主实例会通知使用这个接收服务器的每个节点，不在线的节点会在回来后得知。

### 存储位置 {#to-location}

节点写入主实例的某个它自己能访问到的存储位置：通过 `https://` 或私有地址访问的 S3 服务或 REST 服务器。它的仓库位于那里的 `bombvault-node-<node id>-containers` 和 `bombvault-node-<node id>-files`。

节点用一组为它创建的[已保存的凭据](offsite-recovery.md#sign-ins)登录，这组凭据必须不同于主实例写入该位置所用的凭据。如果窗口说还没有，请先添加一组。与接收服务器不同，凭这组凭据，节点通常可以删除该位置上的内容，窗口也会这样提示。

### 保留在节点上 {#keep-on-node}

节点把备份保存在自己的一个 append-only 模式的 rest-server 后面，主实例把每个分区的副本拉取到自己的存储里。这适合主实例无法为它提供存放位置的节点。这是唯一需要开放端口的去向。

![第二步中选择了保留在节点上，地址为 https://203.0.113.7:8443，并附有关于开放端口的说明。](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![第二步中选择了保留在节点上，地址为 https://203.0.113.7:8443，并附有关于开放端口的说明。](assets/screenshots/nodes/add-keep-light.png#only-light)

1. 在**节点地址**里输入主实例访问节点的地址：节点的公网地址加一个空闲端口，例如 `https://203.0.113.7:8443`，后面不要加文件夹。和上面一样，只有私有地址才使用 `http://`。
2. 节点在那个端口上启动容器 `bombvault-node-rest-server`，仓库放在 Host Data 下的 `bombvault-node` 文件夹里。使用 `https://` 时，它会为这个地址生成证书，主实例会固定这张证书。
3. 在节点的防火墙里放行这个端口，让主实例能访问到。节点上另一个容器已占用的端口会被拒绝，并提示那个容器的名字。

节点某个分区的备份一结束，主实例就拉取这个分区，另外每小时的 15 分也会拉取一次作为后备，即使**拉取**关闭也是如此。主实例拉取到的内容像任何[拉取的副本](offsite-recovery.md#receiver)一样留在主实例上，即使节点已经没了。节点的 rest-server 不接受删除，所以节点会在主实例要求时自己整理仓库。节点的文件夹集会排除 `bombvault-node` 文件夹，所以它绝不会备份自己的备份。

### 随主实例的副本一起异地复制 {#copy-onward}

在**备份**步骤里或在**更改目标**下打开**随本实例的备份一起异地复制**，主实例就会把节点的容器和文件夹集复制到它自己的容器和文件夹分区所复制到的存储位置，使用同一个计划。为此节点不需要自己的文件夹，也不需要自己的复制计划。

- 这些副本和主实例的副本在同一批仓库里，位于节点的主机下。主实例在该位置的规则不会动它们，主实例按节点的规则保留它们，也就是它[整理节点备份](#schedule)时所用的那条规则。
- 主实例用自己的密钥读取节点的仓库，用自己在该位置的凭据写入。节点不会得到新的凭据，接收服务器保持 append-only。对于自己保存备份的节点，主实例复制它拉取到的内容。
- 节点的仓库使用自己的 chunker 设置，所以它们的副本在该位置与主实例的副本几乎不共享数据。
- 在 append-only 的位置上，什么都不会按期清理，主实例自己的副本也是这样。[共享仓库](offsite-recovery.md#shared-repository)不会收到节点的副本，因为它的所有者按组内服务器的规则清理它。
- 卡片显示**副本发往**和这些位置。复制失败会在卡片上算作错误，并发出提醒，参见[关注你的节点](#watch)。
- 关闭后，主实例不再复制任何内容。已有的副本会留着，主实例的规则也仍然不会动它们。

#### 示例：一台 VPS 随主实例的容器复制到 B2 {#example-copy-onward}

`tower` 在**保留多久**下为每个容器保留 7 份备份，并在每晚 04:00 把它的容器分区复制到一个 B2 存储桶。存储桶保留最近 14 份。`vps-helsinki` 备份到 tower 的接收服务器，它的备份在那里按同样的 7 份清理。打开开关后，tower 在 04:00 也把 VPS 的容器复制到存储桶。存储桶里于是有 tower 每个容器的 14 份备份，以及 VPS 每个容器的 7 份。

## 节点何时备份 {#schedule}

每个节点的计划都归主实例管。卡片为每个分区显示一个计划，例如**容器** *每天 2:00* 和**文件夹** *每周（日）4:00*。点击会打开与[计划](schedules.md#plans)里相同的编辑器，更改会立即保存。

![从卡片打开的节点容器计划，有关闭、每天、每周、每 N 天和 Cron。](assets/screenshots/nodes/schedule-dark.png#only-dark)
![从卡片打开的节点容器计划，有关闭、每天、每周、每 N 天和 Cron。](assets/screenshots/nodes/schedule-light.png#only-light)

- **曾经不在线的节点**一回来就启动它错过的备份。主实例每五分钟检查一次。
- **正忙的节点**会在五分钟后被再次询问。
- **主实例整理节点的旧备份期间**，节点在整理完成前不会启动任何东西。在接收服务器和节点本身上，这个时间窗口每周日 03:00 开始，最长持续六小时。在存储位置上，它遵循主实例对该位置的异地复制计划。主实例按它自己在**保留多久**下为该分区设定的规则保留备份，该位置有自己的规则时则按该位置的规则。
- **在节点的运行列表里**，主实例启动的备份和还原显示为由主实例启动。

## 关注你的节点 {#watch}

概览上的**节点**卡片列出每个节点及其状态、上一次备份、错误和未处理的异常。有了节点之后它才会出现。

![概览上的节点卡片，有三个节点：一个已连接，一个不活跃并有两个错误，一个仍在等待节点。](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![概览上的节点卡片，有三个节点：一个已连接，一个不活跃并有两个错误，一个仍在等待节点。](assets/screenshots/nodes/overview-card-light.png#only-light)

主实例通过它的[通知渠道](notifications.md)每次事件只发一条提醒，等这次事件结束后才会发下一条：

- **备份失败**，例如 *BombVault: vps-helsinki: backup failed*，附带条目和原因。这个条目之后有一次备份成功，事件就结束。
- **节点无法访问**，例如 *BombVault: vps-helsinki is out of reach*。主实例会等待节点各仓库上最短的那个失联开关，默认 26 小时，节点还没有去向时为 6 小时。
- **异地复制失败**，例如 *BombVault: vps-helsinki: off-site copy failed*，表示主实例无法把节点的备份复制到它的某个存储位置。下一次复制成功，事件就结束。

主实例的接收端监视也会照看节点的每个仓库，带有失联开关和每天 04:00 的 `restic check`，参见[接收端和拉取端](offsite-recovery.md#receiver)。

## 节点的条目 {#entries}

在**备份**下，容器和文件夹列表在你自己的条目下方为每个节点设有一个分区。节点自己的容器不会列出。条目菜单里的**立即备份**会在节点上启动它的备份，**打开**会打开它的页面。

在文件夹列表里节点的分区中，**添加文件夹集**会在节点上添加一个文件夹集，文件夹集菜单里的**编辑**会修改它。两者打开的窗口和你自己的文件夹集一样。其中的文件夹选择器浏览节点的文件夹，但不会在那里新建文件夹。

![容器列表中节点 vps-falkenstein 的分区：nextcloud、nextcloud-db、caddy 和 uptime-kuma。](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![容器列表中节点 vps-falkenstein 的分区：nextcloud、nextcloud-db、caddy 和 uptime-kuma。](assets/screenshots/nodes/backups-section-light.png#only-light)

节点条目的页面会显示**在 vps-falkenstein 上**这样的节点名称，并列出它的备份。在其中一个备份上选择**还原**，会在节点上还原这个条目，并替换那里现有的内容。和在主实例上一样，容器会为此停止。

![vps-falkenstein 上 nextcloud 的页面，列出它的备份，每个旁边都有一个还原按钮。](assets/screenshots/nodes/entry-dark.png#only-dark)
![vps-falkenstein 上 nextcloud 的页面，列出它的备份，每个旁边都有一个还原按钮。](assets/screenshots/nodes/entry-light.png#only-light)

通过主实例，你能访问节点自身路由的一份固定列表：它的容器、文件夹集、运行和异常，以及对它们的备份和还原。节点的设置、密钥和存储都访问不到，列表上也没有任何删除操作。参见[API 中的节点](api.md#nodes)。

## 替换和释放 {#replace-release}

**替换**在你用密码登录后再次显示节点的 compose 文件。用它把节点迁移到另一台服务器或重建它：在新服务器上启动这个文件，并关闭旧服务器。同样的 `APP_KEY` 和 `NODE_HOME` 会让它重新成为同一个节点，带着它的备份和历史。主实例为每个节点保存它的文件夹集、容器以及重建容器所需的一切，还有它的运行记录。节点用一个空的 `config` 文件夹启动时，主实例会重新接管它，告诉它备份发往哪里，并在下一次备份之前把这些全部交还给它，所以节点也能恢复一个已经不在的容器。在新服务器上，自己保存备份的节点会从空仓库开始。它之前的备份留在主实例拉取的副本里。使用同一 `APP_KEY` 的两台服务器算作同一个节点，所以千万不要同时运行两台。

**释放**让节点离开。它不再接受主实例的指令，也不再备份任何东西。它的备份留在原处，主实例拉取的每份副本也都留着。它在接收服务器上的登录会被删除，主实例也不再关注它的仓库。释放时不在线的节点，卡片会一直显示**正在释放**，直到它得知此事。被释放的节点不能再用同一个 `APP_KEY` 归属任何主实例。要再次使用这台服务器，请把它作为新节点添加，并用新的 compose 文件和一个空的 `config` 文件夹启动它。

## 安全 {#security}

如果主实例被攻破，攻击者也能控制它的节点，而节点掌握着它所在服务器上的 Docker，在那里就等于 root。请用密码保护主实例，并且不要把它暴露在互联网上。如果某个节点被攻破，攻击者拿不到配对组、堡垒、其他节点或其他仓库，也删不掉接收服务器上的历史。详情见[安全模型中的节点](configuration.md#nodes)。

[恢复工具包](offsite-recovery.md#recovery-kit)里有一个**节点**章节，列出每个节点的 `APP_KEY`、restic 密码和仓库，这样你可以找回节点，或用普通的 restic 还原它的备份。
