# BombVault

**将您的 Unraid 数据封存于保险库中。投下一份备份，引爆一次还原。**

BombVault 是一款自托管、Unraid 原生的 Web 应用，用于对 Docker 容器和 KVM/libvirt 虚拟机进行**备份和完整的灾难恢复**。它以单个多架构 Docker 容器的形式运行，为您提供跟随系统浅色/深色偏好的现代化 Web 界面，并覆盖整个生命周期：备份、计划、验证和还原。

还原是自动的。容器会以原样重新出现在 Unraid 的 Docker 标签页中，虚拟机则会在虚拟机管理器中被重新定义，并重新挂接其磁盘和 UEFI NVRAM。无需手动重装，无需重新配置，毫不费力。

由 [restic](https://restic.net) 驱动，因此每份备份都经过去重、增量处理并始终加密。

!!! note "妥善保管您的 APP_KEY"
    BombVault 从一个名为 `APP_KEY` 的 32 字节密钥派生出 restic 仓库密码。丢失它将使加密备份无法恢复。请用 `openssl rand -hex 32` 生成一个，并存放在安全的地方。参见[配置](configuration.md)。

## BombVault 保护什么

| 域 | 保存的内容 |
|---|---|
| **Docker 容器** | Appdata 目录以及容器定义（镜像、环境变量、端口、标签、卷）。 |
| **KVM / libvirt 虚拟机** | 虚拟机磁盘映像、XML 定义和 UEFI NVRAM，通过 SSH 备份（无需挂载 libvirt）。 |
| **Unraid 闪存** | 整个 USB 闪存（`/boot`）：操作系统、许可证、阵列配置、共享、网络和插件配置。 |
| **应用配置** | BombVault 自身的 `/config`：其设置数据库、异地凭据和 libvirt SSH 密钥对。 |
| **文件与文件夹** | 命名的**文件集**，服务器上的任意文件夹，每个文件集可选配各自的排除模式。 |

## 还原才是主角

从 restic 快照复制回数据后，BombVault 会将保存的容器定义对 Docker API 重新执行一遍，因此容器会原样重新出现在 Unraid 的 Docker 标签页中，仿佛它一直都在（相同的镜像、相同的设置、相同的端口映射）。虚拟机的 XML 会通过 SSH 重新定义，其磁盘和 UEFI NVRAM 会被重新挂接，即使虚拟机已被删除也是如此。

当备份停止了依赖的容器时，它们会按正确的顺序恢复：BombVault 会按照 Compose 的 `depends_on` 顺序重启它们，并等待每一个报告健康后再启动依赖它的容器，因此不会有任何容器抢在尚未就绪的数据库或网关之前启动。参见[功能](features.md)。

## 工作原理

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault 是编排和界面层，而非存储引擎。所有实际的数据搬运都通过 restic 完成。

## 快速上手

初次接触？请前往 **[快速上手](getting-started.md)**，通过 Community Applications 在 Unraid 上安装 BombVault 并运行您的第一份备份。然后探索完整的 **[功能](features.md)**，调整您的**[配置](configuration.md)**，并设置 **[异地与恢复](offsite-recovery.md)**。

异地复制可以同时向每个域的多个目标分发，一个只读的**接收方仪表板**在接收这些副本的机器上对其进行监控，而您可以用**导出与导入设置**卡片将您的整套配置迁移到新机器。参见[异地与恢复](offsite-recovery.md)和[配置](configuration.md#portable-settings-export-and-import)。

## 链接

- **源代码：** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid 支持帖：** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **问题反馈：** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "对主机的等同于 root 的控制权"
    通过 Docker 套接字，BombVault 可以停止、删除和重新创建容器并读写 appdata，而对于虚拟机备份，它会通过 SSH 登录主机以运行 `virsh`。任何能访问其 Web 界面的人实际上都拥有主机的 root 权限。请仅在受信任、未对外暴露的网络上运行 BombVault，并在启用异地或不可变备份后打开可选的密码门（设置，安全）。完整的安全模型参见[配置](configuration.md)。
