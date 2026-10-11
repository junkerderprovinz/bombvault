# BombVault

**将你的 Unraid 数据封存于保险库中。投下一份备份，引爆一次还原。**

BombVault 是一款自托管、Unraid 原生的 Web 应用，用于对 Docker 容器和 KVM/libvirt 虚拟机进行**备份和完整的灾难恢复**。它以单个多架构 Docker 容器的形式运行，为你提供深色、浅色或跟随系统主题的 Web 界面，并覆盖整个生命周期：备份、计划、验证和还原。

还原是自动的。容器会以原样重新出现在 Unraid 的 Docker 标签页中，虚拟机则会在虚拟机管理器中被重新定义，并重新挂接其磁盘和 UEFI NVRAM。无需手动重装，无需重新配置，毫不费力。

由 [restic](https://restic.net) 驱动，因此每份备份都经过去重、增量处理并始终加密。

!!! note "妥善保管你的 APP_KEY"
    BombVault 从一个名为 `APP_KEY` 的 32 字节密钥派生出 restic 仓库密码。同时丢失它和恢复工具包，加密备份将无法恢复。请用 `openssl rand -hex 32` 生成一个，并存放在安全的地方。参见[配置](configuration.md)。

## BombVault 保护什么

每个容器、虚拟机、文件夹集、闪存、每个 ZFS 数据集以及 BombVault 自身的设置，都是一个有独立页面的**条目**。同一类条目构成一个**分区**，你可以在**设置、常规**中开启它。

| 分区 | 保存的内容 |
|---|---|
| **容器** | Appdata 目录以及容器定义（镜像、环境变量、端口、标签、卷）。 |
| **虚拟机** | 虚拟机磁盘映像、XML 定义和 UEFI NVRAM，通过 SSH 备份（无需挂载 libvirt）。 |
| **闪存** | 整个 USB 闪存（`/boot`）：操作系统、许可证、阵列配置、共享、网络和插件配置。 |
| **自我备份** | BombVault 自身的 `/config`：其设置数据库、各目标的登录信息和 libvirt SSH 密钥对。 |
| **文件夹** | 服务器上的任意文件夹，作为一个文件夹集，每个集可选配各自的排除模式。 |
| **ZFS 数据集** | 一个数据集及其下的所有数据集，从一个 ZFS 快照读取，像文件夹一样保存。参见 [ZFS 数据集](zfs-datasets.md)。 |

## 还原才是主角

从 restic 快照复制回数据后，BombVault 会将保存的容器定义对 Docker API 重新执行一遍，因此容器会原样重新出现在 Unraid 的 Docker 标签页中，仿佛它一直都在（相同的镜像、相同的设置、相同的端口映射）。虚拟机的 XML 会通过 SSH 重新定义，其磁盘和 UEFI NVRAM 会被重新挂接，即使虚拟机已被删除也是如此。

一个窗口即可还原所有类型。默认选择**放在原件旁边**，它会把备份还原为一份副本，原件保持不变。**替换原件**会先备份当前状态，因此还原可以撤销。其他目标是文件夹或新的 ZFS 数据集。

当备份停止了依赖的容器时，它们会按正确的顺序恢复：BombVault 会按照 Compose 的 `depends_on` 顺序重启它们，并等待每一个报告健康后再启动依赖它的容器，因此不会有任何容器抢在尚未就绪的数据库或网关之前启动。参见[功能](features.md)。

## 工作原理

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault 使用 Docker 套接字在备份前停止容器，并在还原后重新创建容器。对于虚拟机，它通过 SSH（`qemu+ssh://`）在主机上运行 `virsh`，以正常关闭域或创建实时快照。它从不绑定挂载任何 libvirt 路径，因此不会妨碍主机上的虚拟机管理器。

BombVault 是编排和界面层，而非存储引擎。所有实际的数据搬运都通过 restic 完成。

## 快速上手

初次接触？请前往 **[快速上手](getting-started.md)**，通过 Community Applications 在 Unraid 上安装 BombVault 并运行你的第一份备份。然后探索完整的 **[功能](features.md)**，调整你的**[配置](configuration.md)**，并设置 **[异地与恢复](offsite-recovery.md)**。

每个分区可以同时复制到多个目标，配对的 BombVault 可以把你的异地副本保存在拒绝删除的服务器上，**应急向导**则能凭恢复工具包，通过你的组或输入的密钥找回丢失的服务器。要把配置迁移到新机器，请使用**导出 / 导入设置**卡片。参见[异地与恢复](offsite-recovery.md)和[配置](configuration.md#portable-settings-export-and-import)。

**[Android 应用](android.md)** 把群组中的每台服务器都放到你的手机上，并在一个屏幕上显示所有服务器的活动日志。

## 致谢 {#credits}

- [@Darkdragon14](https://github.com/Darkdragon14) 的 **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**（Apache-2.0）为 BombVault 提供了最初的构想：一键备份并自动重装 Docker 容器。BombVault 是基于 Go 和 restic 的独立实现，把这一构想扩展到了虚拟机、闪存等更多方面。
- **[restic](https://restic.net/)** 是 BombVault 所驱动的快速、安全、去重的备份引擎。
- **[rclone](https://rclone.org/)** 提供云后端。
- 按钮上的大部分图标来自 **[Streamline](https://streamlinehq.com)** 的免费 Core Solid 图标集（[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)，[来源](https://github.com/webalys-hq/streamline-vectors)）。其余图标来自 Font Awesome Free、Material Design Icons、Simple Icons 和 Tabler Icons，或是为本项目绘制的。

## 许可证 {#license}

Copyright (C) 2026 Junker der Provinz. BombVault 是依据 **GNU Affero General Public License v3.0**（[LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)）发布的自由软件。你可以运行、研究、分享和修改它。如果你分发它，或将修改后的版本作为网络服务运行，则必须以相同的许可证公开你的源代码，并保留现有的版权声明和署名声明。

名称和品牌不在许可范围内。AGPL 只涵盖源代码："BombVault"、其徽标和品牌均保留权利，因此分支必须使用自己的名称和品牌，且不得将自己呈现为 BombVault。

## 链接

- **源代码：** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid 支持帖：** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **问题反馈：** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "对主机的等同于 root 的控制权"
    通过 Docker 套接字，BombVault 可以停止、删除和重新创建容器并读写 appdata，而对于虚拟机备份，它会通过 SSH 登录主机以运行 `virsh`。任何能访问其 Web 界面的人实际上都拥有主机的 root 权限。请仅在受信任、未对外暴露的网络上运行 BombVault，并在**设置、安全**中设置登录密码。恢复工具包、密钥托管副本以及删除某个目标上的备份都需要它。完整的安全模型参见[配置](configuration.md#security-model)。
