# BombVault

**你的 Unraid 数据，封存在保险库里。投下一份备份，引爆一次恢复。**

![概览：状态栏、需要你处理的事项、各分区的保护情况、存储位置和历史记录。](assets/screenshots/overview/home-dark.png#only-dark)
![概览：状态栏、需要你处理的事项、各分区的保护情况、存储位置和历史记录。](assets/screenshots/overview/home-light.png#only-light)

BombVault 是一款自托管的 Web 应用，用于在 Unraid 和其他 Docker 主机（例如 TrueNAS SCALE、ZimaOS 或普通 Linux 机器）上做**备份和完整的灾难恢复**。它可以备份 Docker 容器、KVM/libvirt 虚拟机、文件夹、ZFS 数据集和 Unraid 闪存。它以单个多架构 Docker 容器运行，提供深色、浅色或跟随系统的 Web 界面，并覆盖整个生命周期：备份、计划、验证和恢复。

恢复是自动的。在 Unraid 上，容器会和原来一样重新出现在 Docker 标签页中，虚拟机会在虚拟机管理器中重新定义，磁盘和 UEFI NVRAM 也会重新挂上。不用手动重装，不用重新配置，也不会手忙脚乱。在 TrueNAS 上，恢复到原容器旁边的容器会作为普通 Docker 容器运行，Apps 页面不会列出它。

由 [restic](https://restic.net) 驱动，所以每份备份默认都是去重、增量和加密的。

!!! note "请保管好你的 APP_KEY"
    BombVault 用一个名为 `APP_KEY` 的 32 字节密钥派生出 restic 仓库的密码。如果它和恢复工具包一起丢失，加密的备份就无法恢复。用 `openssl rand -hex 32` 生成一个，并放在安全的地方。参见[配置](configuration.md)。

## BombVault 保护什么 {#what-bombvault-protects}

每个容器、虚拟机、文件夹集、闪存、每个 ZFS 数据集以及 BombVault 自己的设置都是一个**条目**，有自己的页面。同一种类的条目组成一个**分区**，你可以在**设置、常规**中开启分区。

| 分区 | 保存的内容 |
|---|---|
| **容器** | Appdata 目录，加上容器定义（镜像、环境变量、端口、标签、卷）。 |
| **虚拟机** | 虚拟机磁盘映像、XML 定义和 UEFI NVRAM，通过 SSH 备份（不挂载 libvirt）。 |
| **闪存** | 整个 USB 闪存（`/boot`）：系统、许可证、阵列配置、共享、网络和插件配置。 |
| **自我备份** | BombVault 自己的 `/config`：设置数据库、各存储位置的登录凭据、libvirt SSH 密钥对、ZFS 服务器的密钥和固定的主机密钥以及证书。 |
| **文件夹** | 服务器上的任意文件夹，作为一个文件夹集，每个集可以设置排除模式。 |
| **ZFS 数据集** | 一个数据集及其下的所有数据集，从一个 ZFS 快照读取，像文件夹一样保存。参见 [ZFS 数据集](zfs-datasets.md)。 |

## 恢复才是主角 {#restore-is-the-star}

从 restic 快照把数据复制回来之后，BombVault 会把保存的容器定义对 Docker API 重放一遍，容器就会重新出现在 Unraid 的 Docker 标签页中，仿佛一直都在那里（相同的镜像、相同的设置、相同的端口映射）。虚拟机的 XML 会通过 SSH 重新定义，磁盘和 UEFI NVRAM 也会重新挂上，即使虚拟机已经被删除。

一个窗口就能恢复所有类型。默认选中**放在原件旁边**，把备份恢复成一个副本，原件保持不动。**替换原件**会先备份当前状态，所以恢复可以撤销。另外的目标是一个文件夹，或一个新的 ZFS 数据集。

当备份停止了依赖它的容器时，它们会按正确的顺序回来：BombVault 按 Compose 的 `depends_on` 顺序重启它们，并等每个容器报告健康后再启动依赖它的容器，所以不会有容器抢在还没起来的数据库或网关前面。参见[恢复](restore.md)和[命令与其他容器](backups.md#commands)。

## 工作原理 {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault 通过 Docker 套接字在备份前停止容器，并在恢复后重新创建容器。对于虚拟机，它通过 SSH（`qemu+ssh://`）在主机上运行 `virsh`，以便正常关闭虚拟机或做实时快照。它从不绑定挂载任何 libvirt 路径，所以不会妨碍主机上的虚拟机管理器。

BombVault 是编排和界面层，不是存储引擎。所有实际的数据搬运都由 restic 完成。

## 快速上手 {#quick-start}

刚开始用？请先看**[快速上手](getting-started.md)**，安装 BombVault 并运行第一次备份，然后设置**[异地与恢复](offsite-recovery.md)**。**[示例](guides.md)**会把常见的工作从头到尾走一遍，**[设置](settings.md)**页面列出了每一个磁贴，**[功能](features.md)**则是 BombVault 全部功能的简短目录。

每个分区可以同时复制到多个存储位置。配对的 BombVault 可以把你的异地副本放在一台拒绝删除的服务器上。**应急向导**能凭恢复工具包，通过你的组或输入密钥，把丢失的服务器找回来。要把配置带到新机器上，请使用**导出 / 导入设置**卡片。参见[异地与恢复](offsite-recovery.md)和[配置](configuration.md#portable-settings-export-and-import)。

**[Android 应用](android.md)**把你组里的每台服务器都带到手机上，所有服务器的活动日志显示在同一个屏幕上。

文档有 26 种语言，界面有 42 种。还没有翻译的页面会显示英文。

## 本文用到的词 {#glossary}

有几个词在 BombVault 和这些页面里有固定的含义。

| 词 | 含义 |
|---|---|
| **条目** | BombVault 备份的一样东西：一个容器、一台虚拟机、一个文件夹集、闪存、一个 ZFS 数据集及其下的数据集，或它自己的设置。每个条目都有自己的页面。 |
| **分区** | 同一种类的所有条目，例如容器。你在**设置、常规**中开启分区。 |
| **存储位置** | 备份存放的地方：这台服务器上的文件夹、一个共享、你自己的服务器或云服务。参见[存储位置](offsite-recovery.md#destinations)。 |
| **本地** | 这台服务器上存放各分区文件夹的存储位置。 |
| **专属文件夹** | 分区首先写入备份的仓库。默认在这台服务器上，也可以放在某个存储位置。参见[分区的专属文件夹](offsite-recovery.md#own-folder)。 |
| **复制** | 用 `restic copy` 从专属文件夹复制到另一个存储位置的备份。 |
| **直接仓库** | 条目直接备份到的异地仓库，中间没有本地副本。参见[直接仓库](offsite-recovery.md#direct-repositories)。 |
| **登录凭据** | 为 S3 服务或 rest-server 保存的密钥或密码，其中一个是默认的。参见[登录凭据](offsite-recovery.md#sign-ins)。 |
| **放置** | 条目的备份存到哪些存储位置，可以在条目上设置，也可以作为分区的默认值。参见[按条目放置](offsite-recovery.md#placement)。 |
| **副本** | 用 `zfs send` 在第二台 ZFS 服务器上做的 ZFS 条目的逐块精确副本。它和备份并存，不会取代备份。参见[副本](zfs-datasets.md#replica)。 |
| **角色** | 你的组里一个实例为另一个实例做的事：接收端、拉取端、ZFS 服务器或密钥保管端。参见[实例和角色](offsite-recovery.md#roles)。 |

## 致谢 {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**，作者 [@Darkdragon14](https://github.com/Darkdragon14)（Apache-2.0），给了 BombVault 最初的想法：一键备份并自动重装 Docker 容器。BombVault 是基于 Go 和 restic 的独立实现，把这个想法延伸到了虚拟机、闪存等更多方面。
- **[restic](https://restic.net/)** 是 BombVault 所驱动的快速、安全、会去重的备份引擎。
- **[rclone](https://rclone.org/)** 提供云后端。
- 按钮上的大部分图标来自 **[Streamline](https://streamlinehq.com)** 的免费 Core Solid 图标集（[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)，[来源](https://github.com/webalys-hq/streamline-vectors)）。其余来自 Font Awesome Free、Material Design Icons、Simple Icons 和 Tabler Icons，或是为本项目专门绘制的。

## 许可证 {#license}

Copyright (C) 2026 Junker der Provinz. BombVault 是自由软件，采用 **GNU Affero General Public License v3.0**（[LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)）。你可以运行、研究、分享和修改它。如果你分发它，或把修改后的版本作为网络服务运行，就必须以相同的许可证公开你的源代码，并保留现有的版权和署名声明。

名称和品牌不在许可范围内。AGPL 只涵盖源代码："BombVault"、它的标志和品牌仍然保留，所以分支必须使用自己的名称和品牌，不得自称是 BombVault。

## 链接 {#links}

- **源代码：** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid 支持帖：** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **问题反馈：** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "等同于主机 root 的控制权"
    通过 Docker 套接字，BombVault 可以停止、删除和重新创建容器，并读写 appdata。做虚拟机备份时，它还会通过 SSH 登录主机来运行 `virsh`。任何能访问它的 Web 界面的人，实际上都拥有主机的 root 权限。请只在受信任、不对外暴露的网络上运行 BombVault，并在**设置、安全**中设置登录密码。恢复工具包、密钥托管、闪存 ZIP 以及导出闪存备份或自我备份都需要密码。完整的安全模型参见[配置](configuration.md#security-model)。
