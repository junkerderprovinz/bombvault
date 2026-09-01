# 快速上手

本页带您从一台全新的 Unraid 机器走到您的第一份备份。

## 要求

| 要求 | 说明 |
|---|---|
| **Unraid 6.12+** | 更早的版本未经测试。 |
| **restic 仓库位置** | 本地路径（推荐：您的阵列或缓存）、SMB、NFS，或任意 rclone 后端。 |
| **Docker 套接字** | 由模板自动挂载（`/var/run/docker.sock`）。 |
| **Unraid 闪存**（`/boot`） | 由模板自动整体挂载（`/boot` 到 `/host/boot`）。它驱动闪存备份，并让已还原的容器以正常、可编辑的 Unraid 应用形式重新出现。 |
| **KVM 虚拟机**（可选启用） | 虚拟机备份通过 SSH 与 libvirt 通信，无需挂载 libvirt。在设置中进行配置（参见[配置](configuration.md)）。 |

## 在 Unraid 上安装

最简单的途径是 **Community Applications**。

1. 打开 Unraid 中的 **Apps** 标签页。
2. 搜索 **BombVault**。
3. 点击 **Install**，设置必需的变量（见下文），然后应用。

!!! tip "手动安装模板"
    如果您更愿意手动添加模板：

    1. 前往 **Docker，Add Container，Template repositories** 并添加：
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. 在 Templates 中搜索 **BombVault**。
    3. 设置必需的变量并点击 **Apply**。

## 通用 Docker 主机

不是 Unraid？BombVault 也能在任意 Docker 主机上以普通容器运行（TrueNAS Scale 上的容器支持，在它拥有自己的应用目录条目之前，也是靠这个跑起来的）。

1. 从仓库取得可直接编辑的 [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml)。
2. 设置 `APP_KEY`（见下文），并把 Host Data 卷指向你真正的数据根目录，文件里的注释会把这两件事都讲清楚。
3. 执行 `docker compose up -d`，然后打开 `https://<host-ip>:3443/`。

与 Unraid 的不同之处：

- **没有 flash/USB 域。** 这里没有要采集或还原的启动 U 盘，所以设置里的 Flash 域无事可做。作为替代，文件域提供了一键建议 **添加预设：主机系统配置**（一份起步的 `/etc` 文件集，保存前由你审阅和修改），作为实用的通用等价物。
- **没有 Unraid 原生通知。** BombVault 自己的通知渠道（Webhook、异地失败告警等）照常工作；略过的只是向 Unraid 自身通知系统的推送，因为这里根本没有那套系统。
- **虚拟机备份是可选的，且需要一台通过 SSH 可达的独立 libvirtd 主机。** 见 compose 文件里被注释掉的那段。通用 Docker 主机本身并不自带虚拟机管理。

## 唯一必需的设置

您唯一必须设置的变量是 `APP_KEY`，一个 32 字节的十六进制密钥（64 个十六进制字符），用于派生 restic 仓库密码。

在任意机器上生成一个：

```bash
openssl rand -hex 32
```

将结果粘贴到模板的 `APP_KEY` 字段。

!!! danger "切勿丢失您的 APP_KEY"
    丢失 `APP_KEY` 将使您的加密备份无法恢复。请将它存放在安全且与服务器分离的地方。BombVault 运行后，使用其一键式的**加密密钥恢复工具包**（参见[异地与恢复](offsite-recovery.md)）保存完整的恢复捆绑包。

模板还会为您挂载 Docker 套接字、闪存（`/boot`）和 **Host Data** 根目录（`/mnt`）。备份的*来源*和*目标*都位于 Host Data 之下。完整的变量参考和异地设置参见[配置](configuration.md)。

## 首次运行

1. 在 `https://<your-unraid-ip>:3443` 打开 Web 界面（开箱即用自签名证书）。
2. 在**设置**中，启用您想要的备份域（容器、虚拟机、闪存、配置、文件）并选择一个强调色。
3. 在**容器**标签页，选择一个容器并点击**备份**以创建您的第一个还原点。仓库路径默认为 `/mnt/user/bombvault/{container,vms,flash,config,files}`，并在首次备份时创建。
4. 在**设置，计划**中设置计划任务。容器和虚拟机都有一键式的*全部加入计划*。

!!! tip "可选：设定备份顺序"
    如果某些容器应始终先于其他容器备份（例如数据库先于使用它的应用），请打开容器页面上的**备份顺序**面板，将它们拖入您想要的顺序。计划任务和多选运行随后会遵循它；任何未排序的项目会像以前一样按最逾期优先备份。

!!! note "主机集成检查"
    容器启动后在 Web 界面打开 `/spike`。它会探测每个挂载和 CLI（Docker 套接字、libvirt、restic、qemu-img、rclone）并报告任何缺失的部分，因此您可以在依赖它之前确认容器已正确接线。

## 简单模式与高级模式

默认情况下界面只显示基本功能（备份、还原、计划）。使用侧边栏中的**简单 / 高级**开关来显示专家控制项：保留、异地复制、备份前/后钩子、文件级还原、通知、Prometheus 指标以及完整性/维护工具。这是每个浏览器各自的偏好，默认关闭，因此新手得到干净的界面，而高级用户拥有一切。

## 后续步骤

- 浏览完整的 **[功能](features.md)**。
- 添加一个或多个 **[异地与恢复](offsite-recovery.md)** 副本（每个域可同时向多个目标发送）并保存您的恢复工具包。
- 克隆一套配置或迁移到新机器？用**导出与导入设置**卡片将您的整套配置迁移过去。参见[配置](configuration.md#portable-settings-export-and-import)。
- 遇到问题？参见 **[疑难解答](troubleshooting.md)**。
