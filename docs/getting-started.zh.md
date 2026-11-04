# 快速上手

本页带你从一台全新的 Unraid 机器走到第一份备份。

## 要求 {#requirements}

| 要求 | 说明 |
|---|---|
| **Unraid 6.12+** | 更早的版本没有测试过。Unraid 是主要目标，但 BombVault 也能在普通 Docker 主机和 TrueNAS Scale 上运行（参见[通用 Docker 主机](#generic-docker-host)和 [TrueNAS SCALE](#truenas)）。 |
| **restic 仓库位置** | 本地路径（推荐放在阵列或缓存上）、SMB、NFS，或任意 rclone 后端。 |
| **Docker 套接字** | 由模板自动挂载（`/var/run/docker.sock`）。 |
| **Unraid 闪存**（`/boot`） | 由模板自动整个挂载（`/boot` 到 `/host/boot`）。它用于闪存备份，也能让恢复的容器重新成为正常、可编辑的 Unraid 应用。 |
| **KVM 虚拟机**（可选） | 虚拟机备份通过 SSH 与 libvirt 通信，不挂载 libvirt。在**设置、集成、主机 SSH** 中设置（参见[配置](configuration.md)）。 |
| **ZFS 数据集**（可选） | 和虚拟机备份相同的 SSH 连接，主机上要有 `zfs`，Host Data 要映射为 `/mnt`，访问模式为 Read/Write - Slave（模板默认值）。参见 [ZFS 数据集](zfs-datasets.md)。 |
| **Android 应用**（可选） | Android 10 或更高版本，与 9.7.0 或更高版本的服务器配对。参见 [Android 应用](android.md)。 |

## 在 Unraid 上安装 {#install-on-unraid}

最简单的方式是通过 **Community Applications**。

1. 打开 Unraid 的 **Apps** 标签页。
2. 搜索 **BombVault**。
3. 点击 **Install**，填写必需的变量（见下文），然后应用。

!!! tip "手动安装模板"
    如果你想手动添加模板：

    1. 前往 **Docker, Add Container, Template repositories** 并添加：
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. 在 Templates 里搜索 **BombVault**。
    3. 填写必需的变量，点击 **Apply**。

## 通用 Docker 主机 {#generic-docker-host}

不用 Unraid？BombVault 也可以在任何 Docker 主机上作为普通容器运行。TrueNAS SCALE 的步骤见 [TrueNAS SCALE](#truenas)。

1. 从仓库取得可以直接编辑的 [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml)。
2. 设置 `APP_KEY`（见下文），并让 Host Data 卷指向你真正的数据根目录。文件里的注释把这两步都讲清楚了。
3. 运行 `docker compose up -d`，然后打开 `https://<host-ip>:3443/`。

和 Unraid 的不同之处：

- **没有闪存分区。** 这里没有可以采集或恢复的启动 U 盘，所以闪存分区无事可做。不过，当 Host Data 能到达主机的 `/etc` 时，备份页面上的**添加**会提供**系统配置**作为文件夹建议：一个以 `/etc` 为起点的文件夹集，你可以在保存前检查和修改。
- **没有 Unraid 原生通知。** BombVault 自己的应用内通知渠道（Webhook、异地失败提醒等）照常工作。只有推送到 Unraid 原生通知系统这一步会跳过，因为这里没有这样的系统可推送。
- **虚拟机备份是可选的，需要另一台可通过 SSH 访问的 libvirtd 主机。** 参见 compose 文件里被注释掉的那一段。通用 Docker 主机本身没有内置的虚拟机管理器。
- **没有仪表盘小组件。** BombVault Widget 是 Unraid 插件，所以这一步同样跳过。
- **查找容器的数据。** 没有 Unraid 的 `appdata` 约定时，容器的数据文件夹会根据 `DATA_ROOT_SEGMENTS` 中的片段、Docker 命名卷、Compose 项目的工作目录和 `bombvault.data` 标签来找（参见[备份源检测](configuration.md#backup-source-detection)）。命名卷和 `/etc` 预设只能到达 Host Data 挂载内的路径，所以请让 Host Data 指向一个同时包含 Docker 数据根目录的共同上级目录。
- **`PLATFORM`。** 设为 `generic` 或 `truenas`。不设置时，BombVault 通过闪存挂载上自己的标记识别 Unraid，其他情况都按通用主机处理，只属于 Unraid 的步骤会直接跳过，而不是尝试后失败。

### TrueNAS SCALE {#truenas}

BombVault 还不在 TrueNAS 的应用目录里，所以请把它装成自定义应用：**Apps, Discover Apps**，然后在 **Custom App** 旁边的菜单里选 **Install via YAML**。下面的 YAML 和仓库 `truenas-apps` 文件夹里的目录条目设置的内容一样。把 `tank` 换成你的存储池，`APP_KEY` 按下一节的说明填写。

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

然后打开 `https://<truenas-ip>:30528/`。[和 Unraid 的不同之处](#generic-docker-host)下列出的内容在这里同样适用，在 TrueNAS 上还有这些：

- **权限（capabilities）。** BombVault 需要上面这七个。没有 `SETPCAP`，虚拟机备份会失败。没有 `SYS_CHROOT`，每个容器在整个备份期间都会被停止，而不是从快照读取。
- **Host Data** 是数据集上的一个主机路径，这里是 `/mnt/tank`，在应用里挂载到同一个路径。所以 `HOST_SOURCE_ROOT` 和 `HOST_MOUNT_ROOT` 写的都是它。`rslave` 会把主机之后新建的挂载传进来，ZFS 数据集备份需要这一点。
- **ixVolume 里的数据不会被备份。** 存储用 ixVolume 的应用把数据放在 `/mnt/.ix-apps` 下，在 Host Data 之外。BombVault 只保存应用的定义，运行记录里会写明它漏掉了哪些文件夹。对每个需要备份数据的应用，请选择 Host Data 之下某个数据集上的 **Host Path** 存储。
- **虚拟机和 ZFS 数据集备份**以 root 身份通过 SSH 连接 TrueNAS。在 **System, Services** 里打开 SSH 服务，并把 **设置、集成、主机 SSH** 里的公钥加到 **Credentials, Users** 下的 root 用户。虚拟机备份还需要像上面那样设置 `LIBVIRT_URI`，因为 TrueNAS 的 libvirtd 监听的是它自己的套接字。`LIBVIRT_URI` 和 `LIBVIRT_HOST` 都没设置时，BombVault 会把应用的默认网关当作主机，因为 `host.docker.internal` 在 TrueNAS 应用里无法解析。细节见[虚拟机备份指南](vm-backup-ssh-setup.md#truenas-scale)，包括放在 zvol 上的虚拟机。
- **ZFS 副本使用另一把密钥。** 到第二台 ZFS 服务器的副本使用副本对话框里显示的密钥登录，而不是 Host SSH 的密钥。请把那一把加到接收端服务器上。
- **删除的应用分两步找回。** 先在 TrueNAS 里用同样的名称重新安装，这会重新创建它的网络，然后在 BombVault 里恢复它的数据。在此之前，恢复检查会报告网络缺失，恢复会在改动任何东西之前拒绝执行。
- **放在原件旁边**会创建一个普通的 Docker 容器。它能运行，但 TrueNAS 的 Apps 页面不会列出它。
- **数据库转储**在数据集挂载上可以使用。导入到本身就是一个数据集的数据文件夹时，旧内容会被移到它旁边的一个文件夹里，所以上一级数据集需要有足够的空间。

### ZimaOS、CasaOS 和其他主机 {#other-hosts}

- **ZimaOS 和 CasaOS：** 添加[社区商店](https://github.com/junkerderprovinz/zimaos-apps#install)，从那里安装 BombVault。它在那里以 `PLATFORM=generic` 运行，Host Data 在 `/DATA`。ZimaOS 把应用数据放在 `/DATA/AppData/<app>`，BombVault 能自己找到，因为 `DATA_ROOT_SEGMENTS` 不区分大小写地匹配 `appdata`。
- **MOS：** 从 [MOS Hub](https://mos-official.net/apps.html?search=BombVault) 安装 BombVault。它在那里以 `PLATFORM=generic` 运行，Host Data 在 `/mnt/cache`。
- **Synology：** 在 Container Manager 里用 compose 文件创建项目（DSM 7.2 或更新版本）。
- **Proxmox：** 在装有 Docker 的虚拟机或 LXC 容器里运行。

去掉注释后，通用的 compose 文件很短。把 `./data` 换成存放你容器数据的主机文件夹，把 `HOST_SOURCE_ROOT` 设成同一个路径，然后填写 `APP_KEY`：

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

如果还想备份 Docker 命名卷，请挂载一个同时包含你的数据和 Docker 数据根目录（`docker info -f '{{.DockerRootDir}}'`，通常是 `/var/lib/docker`）的文件夹，就像 Unraid 模板挂载整个 `/mnt` 那样。

## 唯一必填的设置 {#the-one-required-setting}

唯一必须设置的变量是 `APP_KEY`，一个 32 字节的十六进制密钥（64 个十六进制字符），用来派生 restic 仓库的密码。

在任意一台机器上生成一个：

```bash
openssl rand -hex 32
```

把结果粘贴到模板的 `APP_KEY` 字段（Unraid），或 `docker-compose.yml` 里的 `APP_KEY` 环境变量（通用 Docker 主机）。

!!! danger "不要丢失你的 APP_KEY"
    丢失 `APP_KEY` 会让加密的备份无法恢复。把它存放在安全的、和服务器分开的地方。BombVault 运行起来后，请设置登录密码，并在**设置、恢复**中下载**恢复工具包**（参见[异地与恢复](offsite-recovery.md#recovery-kit)）。里面有密钥、每个存储位置和 restic 命令。

模板还会替你挂载 Docker 套接字、闪存（`/boot`）和 **Host Data** 根目录（`/mnt`）。备份的*来源*和*目标*都在 Host Data 之下。完整的变量参考和异地设置请看[配置](configuration.md)。

## 第一次运行 {#first-run}

![新安装的欢迎页，显示 BombVault 在服务器上找到的内容。](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![新安装的欢迎页，显示 BombVault 在服务器上找到的内容。](assets/screenshots/getting-started/welcome-light.png#only-light)

1. 在 `https://<your-unraid-ip>:3443` 打开网页界面（默认使用自签名证书）。新安装会打开**欢迎使用 BombVault**，列出 BombVault 在服务器上找到的内容。
2. 选择**开始吧**。概览里随后会出现**入门步骤**卡片，共三步：
    1. **选择分区**：开启 BombVault 要备份的内容（容器、虚拟机、闪存、文件夹、ZFS 数据集、自我备份）。**选择**会打开该分区的条目列表，用**暂停计划**可以把某一个排除在外。你在这里开启的分区每晚都会运行，可以在**设置、计划**中更改。
    2. **添加目标**：给备份找第二个存放的地方。每个分区在这台服务器上已经有一个文件夹，`/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`，在第一次备份时创建。最安全的是场所之外的第二个存储位置，例如 Storage Box 或 Backblaze B2。
    3. **首次备份**：之后它每晚自动运行。这张卡片会一直保留，直到第一次备份完成。
3. 打开**备份**，在一个列表里查看所有条目。**立即备份全部**会对所有开启的分区执行一次运行，条目页面上的**立即备份**只备份那一个条目。

![入门步骤卡片和它的三个步骤，第二步已展开。](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![入门步骤卡片和它的三个步骤，第二步已展开。](assets/screenshots/getting-started/first-steps-light.png#only-light)

以前用过 BombVault？欢迎页上的**从备份恢复**会打开恢复页面，在那里**应急向导**可以找回丢失服务器的设置、容器、虚拟机和文件夹。参见[异地与恢复](offsite-recovery.md#emergency)。

!!! tip "可选：设定备份顺序"
    如果有些容器应该总是先于其他容器备份（例如数据库先于使用它的应用），请打开**设置、计划**下的**顺序**卡片，把它们拖成你想要的顺序。计划运行和全部备份随后都会遵循这个顺序。没有排序的条目按逾期最久的优先备份。

## 示例：第一次容器备份，从头到尾 {#example-first-backup}

这个示例用一台全新的 Unraid 机器，上面有几个容器，其中有 `plex`，一直走到第一次备份完成，你也知道去哪里找它。大约需要十分钟，外加第一次备份本身的时间。

1. **安装。** 在 Unraid 的 **Apps** 标签页里搜索 **BombVault**，点击 **Install**。
2. **设置密钥。** 在一台电脑上运行 `openssl rand -hex 32`，把这 64 个字符粘贴到模板的 `APP_KEY` 字段。先在你的密码管理器里存一份，其他事情都往后放。点击 **Apply**。
3. **打开它。** 前往 `https://<your-unraid-ip>:3443` 并接受自签名证书。**欢迎使用 BombVault** 会列出它找到的内容，例如 12 个容器、2 台虚拟机和闪存。
4. **开始吧。** 概览打开，上面有**入门步骤**卡片。
5. **选择分区。** **容器**一开始就是开启的。把你还想备份的也开启，例如**闪存**。**选择**会打开它找到的容器列表，用**暂停计划**可以排除其中一个，例如你不在乎的测试容器。开启的分区每晚都在全部备份里运行。
6. **添加目标。** 每个分区在这台服务器上已经有自己的文件夹，位于 `/mnt/user/bombvault` 之下，所以第一次备份有地方可去。这一步添加第二个地方，最安全的是场所之外。你可以现在就按 [S3 示例](offsite-recovery.md#example-s3)来做，也可以等第一次备份完成后再回来。在那之前这张卡片会一直保留。
7. **首次备份。** 打开**备份**，点击**立即备份全部**，或者等到夜里。状态栏里的圆环会变成进度环并统计条目数，例如**正在备份：3 / 12**。每个容器在读取数据时会被停止，读完立即重新启动。数据在 ZFS 上的容器只在拍快照的那几秒钟停机。第一次备份要读取全部内容，所以最久。之后的备份只保存变化的部分。
8. **检查。** 当圆环显示**全部已备份**时，打开概览上的**历史**：每个容器在**运行**里都有一行绿色记录。打开**备份**，点击 `plex`，看它的**备份**标签页：第一个快照就在那里，存放位置是**本地**。几分钟后，它的**验证**标签页会显示恢复检查，它恢复了备份的一个样本并读了回来。
9. **保护密钥。** 打开**设置、安全**，设置登录密码。然后打开**设置、恢复**，在**应对紧急情况**里点击**下载恢复工具包**。把文件存放在服务器之外，和第 2 步里 `APP_KEY` 的副本放在一起。点击**我已妥善保存**之后，概览上的**保存你的恢复工具包**提醒就会消失。

从现在起，容器每晚都会备份。接下来建议按这个顺序做：一个[异地存储位置](offsite-recovery.md#example-s3)，这样服务器坏了备份不会一起丢，然后是[通知](notifications.md)，让失败的消息能到达你，如果 03:00 对你不合适，再看看[计划](schedules.md)。

![第一次备份运行时的状态栏，带有进度环和它的各个站点。](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![第一次备份运行时的状态栏，带有进度环和它的各个站点。](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "主机集成检查"
    概览上的**主机集成**卡片会探测每个挂载和命令行工具（Docker 套接字、libvirt、restic、qemu-img、rclone），并报告缺少的部分，这样你在依赖它之前就能确认容器配置正确。在你添加它之前，它放在**自定义**里。

## 找到你要的页面 {#navigation}

![设置以磁贴呈现，所选磁贴的卡片显示在旁边。每次更改都会立即写入。](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![设置以磁贴呈现，所选磁贴的卡片显示在旁边。每次更改都会立即写入。](assets/screenshots/getting-started/settings-rail-light.png#only-light)

侧边栏有五项：**概览**、**异常**（异常检测开启时）、**备份**、**恢复**，以及在**退出登录**下面的**设置**。页面的主要操作浮在右下角，不常用的放在**更多**里。在手机上，底栏有概览、备份、恢复和设置，其余的在**更多**里。

页面和设置使用窗口的整个宽度。窗口把按钮放在底部，向前的那个在右边。在手机上，底部超过三个按钮时，只保留退出和主要操作，其他的放到**更多**下面。

![在手机上，底栏和已打开的更多菜单。](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![在手机上，底栏和已打开的更多菜单。](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **概览**显示是否一切都已备份，以及什么需要你处理。参见[概览](overview.md)。
- **备份**把每个容器、虚拟机、文件夹集、闪存、每个 ZFS 数据集和自我备份都列为条目。列表上方的磁贴用来选择种类，**筛选**、**排序**和**搜索**可以缩小范围。每个条目都有自己的页面，带三个标签页：**备份**、**设置**和**验证**。条目图标左边的返回按钮回到列表。参见[备份与条目](backups.md)。
- **恢复**是一个统一的页面：选一个分区，搜索条目，再选一份备份。应急向导和其他来源也从这里开始。参见[恢复](restore.md)。
- **设置**有十四个磁贴：常规、外观、计划、目标、恢复、通知、验证、安全、实例、配对、集成、应用、系统和信息。参见[设置](settings.md)。按 `/` 或 Ctrl+K 可以搜索所有设置，页面在旧版本里用过的名称也能搜到。在**外观**下把磁贴拖成你自己的顺序。当页面旁边的一列磁贴会把页面挤得太窄时，例如窗口约 1200 像素宽，磁贴会变成页面上方一条可以横向滚动的带子。手机上显示的也是同一条带子。
- **目标**和**实例**是设置磁贴，各有自己的页面。它们磁贴上的**同时显示在侧边栏**可以把它们固定到侧边栏。

没有任何保存按钮，也没有任何要展开的内容：不常用的东西有自己的卡片或窗口，说明放在 (i) 气泡里。更新之后会自动弹出一个窗口，里面有自你上个版本以来每个版本的卡片，绿点标出有变化的地方。点一下就会隐藏圆点。**设置、信息**里版本卡片上的**新增**会统计你还没看过的内容，并再次打开那个窗口，里面有每个版本的说明，以及带**前往**的变更。

## 从源码构建 {#build-from-source}

BombVault 是单个静态 Go 二进制文件，提供 JSON API 和内嵌的 React 界面。先构建界面，再运行二进制文件：

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

`go run` 同样需要先构建界面。仓库只跟踪 `web/dist` 下一个空的标记文件，所以不执行 `npm --prefix web run build` 的话，二进制文件什么都没嵌入，会返回 `500 SPA index not found`，这是正常的。Docker、libvirt 和 Unraid 无法在 CI 里测试，所以在提交拉取请求之前，请在真实主机上用概览的**主机集成**卡片（或 `GET /api/spike`）检查挂载、restic 和虚拟机的 SSH 连接。

## 下一步 {#next-steps}

- 添加一个或多个场所之外的存储位置（每个分区可以同时复制到多个位置），并保存你的恢复工具包，详见**[异地与恢复](offsite-recovery.md)**。[S3 示例](offsite-recovery.md#example-s3)带你走了一遍。
- 用**[通知](notifications.md)**确保失败的消息能到达你。
- 在**[计划](schedules.md)**里查看各项何时运行。
- 用 **[Android 应用](android.md)**把你组里的每台服务器都放到手机上。
- **[示例](guides.md)**把常见的工作走了一遍，**[功能](features.md)**列出了 BombVault 的全部功能。
- 要复制一套配置，或者搬到新机器？用**导出 / 导入设置**卡片带上你的全部配置。参见[配置](configuration.md#portable-settings-export-and-import)。
- 遇到问题？参见**[故障排除](troubleshooting.md)**。
