# 配置

本页涵盖容器的环境变量、模板提供的挂载、通过 SSH 进行的虚拟机备份，以及异地设置。备份**仓库路径**在应用内部配置（设置，备份路径），而不是通过环境变量。

## 环境变量

| 变量 | 是否必需 | 描述 |
|---|---|---|
| `APP_KEY` | **是** | 用于派生 restic 仓库密码的 32 字节十六进制密钥（64 个十六进制字符）。用 `openssl rand -hex 32` 生成。请妥善保管：丢失它将使加密备份无法恢复。 |
| `LIBVIRT_HOST` | 虚拟机需要 | 用于虚拟机备份、通过 SSH 连接的 Unraid 主机（默认 `host.docker.internal`；模板会预填一个 LAN-IP 占位符）。请使用您的 Unraid LAN IP，在自定义 `br0.x` 网络上为必需。 |
| `LIBVIRT_SSH_PORT` | 否 | 用于虚拟机备份的主机 SSH 端口（默认 `22`）。 |
| `LIBVIRT_SSH_USER` | 否 | 用于虚拟机备份的主机上的 SSH 用户（默认 `root`）。 |
| `LIBVIRT_URI` | 否 | 完整的 libvirt 连接 URI，将**原样**使用，而不再由上方三个 `LIBVIRT_*` 变量拼接而成（此时这三个变量对连接字符串不再生效）。默认未设置。TrueNAS Scale 上需要用到它，因为其 libvirtd 监听在拼接形式无法表达的非标准套接字上：`qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`。参见 [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) 的 TrueNAS Scale 部分。 |
| `PORT` | 否 | HTTP 端口（默认 `3000`；仅在 `HTTP_ONLY=true` 时使用）。 |
| `HTTPS_PORT` | 否 | HTTPS 端口（默认 `3443`；模板以 1:1 发布它，因此 WebUI 在 `https://<ip>:3443` 上应答）。 |
| `HTTP_ONLY` | 否 | 设为 `true` 以禁用自签名 HTTPS 监听器，仅提供纯 HTTP 服务（用于在一个终止 TLS 的反向代理之后）。 |
| `HOST_SOURCE_ROOT` | 否 | 挂载为 **Host Data** 的主机路径（默认 `/mnt`）。BombVault 会将 Docker 报告的绑定挂载来源转换为此挂载下的路径。仅在您挂载了不同的主机根目录时才更改。 |
| `DATA_ROOT_SEGMENTS` | 否 | 以逗号分隔的路径片段名称，用于将绑定挂载来源标记为备份数据（默认为 `appdata`，对应 Unraid 的 `/mnt/user/appdata/<container>` 约定）。当所列片段中的任意一个作为完整路径片段出现在某容器绑定挂载的主机来源中时，该挂载就会被自动选中用于备份，例如 `DATA_ROOT_SEGMENTS=appdata,config` 也会选中类似 `.../config` 的绑定挂载。关于查找容器数据文件夹的其他常驻方式，参见[备份来源检测](#backup-source-detection)。 |
| `PLATFORM` | 否 | 强制指定 BombVault 认为自己运行在哪个平台上，而不进行自动检测：`unraid`、`generic` 或 `truenas`（默认未设置，会通过在闪存挂载下探测 `dockerMan` 标记来自动检测 Unraid，否则为 `generic`；无法识别的值同样回退为 `generic`，并记录到日志）。请在通用 Docker 主机或 TrueNAS Scale 上显式设置它，而不要依赖仅适用于 Unraid 的自动探测；通用 compose 文件正是这样做的。它会改变 appdata 回退约定、跨实例还原目标的默认值，以及是否会尝试仅适用于 Unraid 的通知/配套插件步骤（参见 `internal/platform`）。 |
| `BOMBVAULT_SELF_CONTAINER` | 否 | BombVault 容器自身的名称，以便它绝不会备份（从而停止）自己（默认 `BombVault`；在桥接网络上通过主机名自动检测）。 |
| `BACKUP_MAX_HOURS` | 否 | 单次备份运行在被强制取消前可持有其域锁的最长挂钟小时数（一道防护，防止卡死的运行永久阻塞该域）。留空（默认）使用 `48`。对于非常大或缓慢的云备份可调高它（在上限处被取消的运行会以 `context deadline exceeded` 失败）。设为 `0` 可完全禁用该上限。 |
| `TZ` | 否 | 计划任务的时区（例如 `Europe/Berlin`）。 **未设置时，所有计划均按 UTC 运行**：设为 02:30 的计划将在 02:30 UTC 启动，而不是本地时间。 在 Unraid 上无需自行设置：系统会将自身时区传递给每个容器。 |

## 挂载

按 CA 模板所示挂载 Docker 套接字、闪存（`/boot`）和 **Host Data** 根目录（`/mnt`）。备份的*来源*和*目标*都位于 Host Data 之下，且它以 **rslave** 方式挂载，因此在容器启动后才挂载的远程共享（例如位于 `/mnt/remotes` 之下）无需重启即可可见。

备份仓库路径默认为 `/mnt/user/bombvault/{container,vms,flash,config,files}`，在首次备份时创建。可随时在**设置，备份路径**中更改位置。

!!! note "主机集成检查"
    容器启动后在 Web 界面打开 `/spike`。它会探测每个挂载和 CLI（Docker 套接字、libvirt、restic、qemu-img、rclone）并报告任何缺失的部分。

## 安全模型

!!! warning "对主机的等同于 root 的控制权"
    通过 Docker 套接字，BombVault 可以停止、删除和重新创建容器并读写 appdata，而对于虚拟机备份，它会通过 SSH 登录主机（`qemu+ssh://`，默认 root）以运行 `virsh`。任何能访问其 Web 界面的人实际上都拥有主机的 root 权限。

- **可选的密码保护**（设置，安全）：设置密码以要求登录，清除它以禁用。默认关闭，供受信任的 LAN 使用。会话经过签名（从 `APP_KEY` 派生的 HMAC），更改密码会使它们失效；登录有速率限制。
- 由于该门是可选启用的，未设置时整个界面和 API（包括异地设置、篡改测试路由和恢复工具包）对任何能访问该端口的人都可访问。在使用异地、不可变备份或加密后就启用该门。
- 请仅在受信任、未对外暴露的网络上运行 BombVault。对于远程访问，请将它置于一个添加了身份验证和 TLS 的反向代理之后。响应携带基线安全头（CSP、`nosniff`、`X-Frame-Options`、`Referrer-Policy`）。
- 使用 `HTTP_ONLY=true` 时，会话 cookie 会失去其 `Secure` 标志（必须如此，才能在纯 HTTP 上工作），因此只有在保密性重要时才在一个终止 TLS 的代理之后启用密码。
- 虚拟机备份的 SSH 连接在首次连接时信任主机密钥（TOFU）并此后固定它。如果您的容器到主机的路径不受信任，请带外验证主机的密钥。
- 启用加密时（设置；默认开启），备份由 restic 加密，密钥从 `APP_KEY` 派生。

## 通过 SSH 进行虚拟机备份

BombVault **不挂载任何 libvirt 路径**即可备份 KVM/libvirt 虚拟机。它通过 SSH（`qemu+ssh://`）在主机上运行 `virsh`，因此它绝不会影响您的主机虚拟机管理器。

快速设置：

1. **设置，系统，通过 SSH 备份虚拟机：** 复制显示的公钥。
2. 将它追加到 Unraid 的 `/root/.ssh/authorized_keys`（也会持久化到闪存，以便重启后仍然有效）。
3. 点击**测试连接**。

模板会添加 `--add-host=host.docker.internal:host-gateway`，以便容器能访问主机。如果该名称无法解析（例如当容器运行在自定义 `br0.x` 网络上时），请将 `LIBVIRT_HOST` 设置为您的 Unraid LAN IP。如果您更改了 Unraid 的 SSH 端口，请相应设置 `LIBVIRT_SSH_PORT`。**实时快照**额外需要虚拟机中的 qemu guest agent，以及位于 `/mnt/cache`（而非 `/mnt/user`）上的磁盘。

!!! important "完整的虚拟机设置与网络指南"
    完整的逐步指南（SSH 启用、持久化密钥授权、自定义网络和 VLAN 路由、每台虚拟机的方式以及主机侧疑难解答）位于 GitHub 上的 [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)。

## 异地设置

在**设置，异地**标签页设置异地副本。完整的工作流程（不可变/append-only、篡改测试和 DR 演练）参见[异地与恢复](offsite-recovery.md)。简而言之：

- **后端：** SMB/CIFS 和 NFS（挂载共享并将备份路径指向它）、无需 rclone 的原生 restic 后端（`s3:...`、`rest:http://host:8000/repo`、`b2:...`、`sftp:user@host:/repo`），或任意 rclone 远程（`rclone:<remote>:<bucket>/path`）。
- **云凭据**以加密方式存储在设置，异地，云凭据之下。
- **SSH 目标无需在对端安装任何东西。** `sftp:` 只需要一个 SSH 服务器。将来自**设置，系统，通过 SSH 备份虚拟机**的公钥（也位于 `/config/ssh/id_ed25519.pub`）添加到目标用户的 `~/.ssh/authorized_keys`。
- **异地复制：** BombVault 以尽力而为的方式用 `restic copy` 复制新快照。本地仓库保持为主。每个域都有各自的异地计划，外加一个**立即复制**按钮。
- **每个域可有多个异地目标：** 每个域都可以同时复制到多个异地目标。在设置，异地添加额外目标，每个目标都有各自的仓库、S3 存储类别、append-only 标志、保留和增长预算；它们全都按该域的异地计划复制。一个现有的单一异地设置会作为第一个目标沿用下来。
- **按来源的保留：** 本地策略位于设置，路径与存储；异地策略位于设置，异地（将它全部留为零则从不自动清理异地快照）。
- **带宽限制：** 在设置，异地之下限制 restic 的上传/下载速率。
- **冷存储与归档存储类别（S3）：** 对于原生 S3 异地仓库，选择一个可还原读取的层级（Standard、Standard-IA、One Zone-IA、Intelligent-Tiering、Glacier Instant Retrieval）。rclone 远程在 rclone 配置中设置其类别。

## 可移植设置（导出与导入） {#portable-settings-export-and-import}

设置页面上的**导出与导入设置**卡片会将您的整套 BombVault 配置（域设置、异地目标、计划、保留、通知）写入一个可移植的 JSON 文件，您可以在另一个实例上导入它，因此迁移到新机器或克隆一套配置不再意味着要手动重新录入一切。导入会显示预览并要求确认，且绝不会触动您的备份数据或历史。

!!! warning "导出文件可能包含凭据"
    您可以选择是否在文件中包含异地和通知凭据。包含凭据时，导出文件与您的恢复工具包一样敏感，因此请将它存放在安全的地方。不包含凭据时，该文件只保存非机密的设置。
