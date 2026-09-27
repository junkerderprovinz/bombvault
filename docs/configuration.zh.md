# 配置

本页涵盖容器的环境变量、模板提供的挂载、通过 SSH 进行的虚拟机备份，以及异地设置。备份存放到哪里，在应用内的**设置，存储**中设置，而不是通过环境变量。

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
| `TRUSTED_PROXY` | 否 | BombVault 前面反向代理的地址或 CIDR 网段，以逗号分隔（例如 `192.168.20.11` 或 `10.0.0.0/8`）。只有来自这些跳点的 `X-Forwarded-For` 才会被采信，登录限流随后按真实客户端分别计数，而不是把代理后面的所有人放进同一个桶。未设置（默认）表示谁都不信任：无条件采信的标头会让任何调用方自行挑选计数桶。 |
| `HOST_SOURCE_ROOT` | 否 | 挂载为 **Host Data** 的主机路径（默认 `/mnt`）。BombVault 会将 Docker 报告的绑定挂载来源转换为此挂载下的路径。仅在您挂载了不同的主机根目录时才更改。 |
| `DATA_ROOT_SEGMENTS` | 否 | 以逗号分隔的路径片段名称，用于将绑定挂载来源标记为备份数据（默认为 `appdata`，对应 Unraid 的 `/mnt/user/appdata/<container>` 约定）。当所列片段中的任意一个作为完整路径片段出现在某容器绑定挂载的主机来源中时，该挂载就会被自动选中用于备份，例如 `DATA_ROOT_SEGMENTS=appdata,config` 也会选中类似 `.../config` 的绑定挂载。关于查找容器数据文件夹的其他常驻方式，参见[备份来源检测](#backup-source-detection)。 |
| `PLATFORM` | 否 | 强制指定 BombVault 认为自己运行在哪个平台上，而不进行自动检测：`unraid`、`generic` 或 `truenas`（默认未设置，会通过在闪存挂载下探测 `dockerMan` 标记来自动检测 Unraid，否则为 `generic`；无法识别的值同样回退为 `generic`，并记录到日志）。请在通用 Docker 主机或 TrueNAS Scale 上显式设置它，而不要依赖仅适用于 Unraid 的自动探测；通用 compose 文件正是这样做的。它会改变 appdata 回退约定、跨实例还原目标的默认值，以及是否会尝试仅适用于 Unraid 的通知/配套插件步骤（参见 `internal/platform`）。 |
| `BOMBVAULT_SELF_CONTAINER` | 否 | BombVault 容器自身的名称，以便它绝不会备份（从而停止）自己。 |
| `BACKUP_MAX_HOURS` | 否 | 单次备份运行在被强制取消前可持有其域锁的最长挂钟小时数（一道防护，防止卡死的运行永久阻塞该域）。留空（默认）使用 `48`。对于非常大或缓慢的云备份可调高它（在上限处被取消的运行会以 `context deadline exceeded` 失败）。设为 `0` 可完全禁用该上限。 |
| `TZ` | 否 | 计划任务的时区（例如 `Europe/Berlin`）。 **未设置时，所有计划均按 UTC 运行**：设为 02:30 的计划将在 02:30 UTC 启动，而不是本地时间。 在 Unraid 上无需自行设置：系统会将自身时区传递给每个容器。 |

## 挂载

按 CA 模板所示挂载 Docker 套接字、闪存（`/boot`）和 **Host Data** 根目录（`/mnt`）。备份的*来源*和*目标*都位于 Host Data 之下，且它以 **slave** 方式挂载，因此在容器启动后才挂载的远程共享（例如位于 `/mnt/remotes` 之下）无需重启即可可见。

全新安装会把每个域都存放在存储位置 **Unraid** 中，位于 `/mnt/user/bombvault`，每个域一个文件夹（`container`、`vms`、`flash`、`config`、`files`），在首次备份时创建。您可以在**设置，存储**中添加更多本地或远程的存储位置；参见[存储位置](storage-places.md)。

!!! note "主机集成检查"
    容器启动后在 Web 界面打开 `/spike`。它会探测每个挂载和 CLI（Docker 套接字、libvirt、restic、qemu-img、rclone）并报告任何缺失的部分。

## 备份来源的自动识别 {#backup-source-detection}

对每个容器，BombVault 会自行挑选要备份哪些绑定挂载和具名卷。只要符合下列任意一条，路径就会被采纳（结果随时可在该容器的 **备份路径** 中逐个覆盖）：

- **命中数据根目录片段：** 绑定在宿主机上的来源，把 `DATA_ROOT_SEGMENTS` 里的某个片段作为完整的一级路径包含在内（默认只有 `appdata`）。
- **Docker 具名卷** 一律纳入，因为它们没有可丢弃的对应物，也就没什么可过滤的，**但仅当该卷在宿主机上的真实存储路径本身能通过 Host Data 挂载抵达时才算**，这和 BombVault 备份的其他宿主机路径条件完全一致。默认的本地卷驱动会把卷放在守护进程自己的数据根目录下，即未做修改时的 `/var/lib/docker/volumes/<name>/_data`（可用 `docker info -f '{{.DockerRootDir}}'` 查看）。这个位置并不在通用 `docker-compose.yml` 默认使用的那个只含单个目录的窄 Host Data 挂载里。抵达不到的卷会被静默跳过，这不算错误。要在通用主机上真正备份具名卷，请把 Host Data（以及 `HOST_SOURCE_ROOT`）指向一个同时覆盖 Docker 数据根目录的共同上级目录，取舍见 compose 文件里的 Host Data 注释（Unraid 出于同样的原因，直接挂载整个 `/mnt`，也就是它自己的顶层通用约定，从而绕开了这个问题）。
- **Docker Compose 项目目录：** 若容器带有标准标签 `com.docker.compose.project.working_dir`（由 `docker compose up` 自动设置），该目录也会一并加入，无论是否有绑定命中了数据根目录片段。
- **用标签 `bombvault.data` 覆盖：** 给容器设上标签 `bombvault.data=true`，即可纳入它的全部绑定挂载，适用于上面两条约定都抓不到的布局（例如没有 Compose 项目、只有单个 `/srv/plex/config` 绑定）。除 `false` 之外任何非空值都算作真；没有该标签或 `bombvault.data=false` 则什么也不改变。

## 安全模型

!!! warning "对主机的等同于 root 的控制权"
    通过 Docker 套接字，BombVault 可以停止、删除和重新创建容器并读写 appdata，而对于虚拟机备份，它会通过 SSH 登录主机（`qemu+ssh://`，默认 root）以运行 `virsh`。任何能访问其 Web 界面的人实际上都拥有主机的 root 权限。

- **可选的密码保护**（设置、安全）：设置密码即要求登录，清空即关闭。默认关闭，面向可信局域网使用。密码以 Argon2id 存储在一个用 `APP_KEY` 加过胡椒的值之上，因此被复制的 `/config` 没有密钥毫无价值，有密钥也很难快速破解。新密码至少需要 12 个字符；已有的较短密码在更改之前仍然可用。会话经过签名（由 `APP_KEY` 派生的 HMAC），更改密码会使其失效；登录被限制为每个客户端每分钟五次失败。
- **两步验证**（设置）：在密码之外再用身份验证应用的时间码，并在开启时一次性发放八个一次性恢复码。共享密钥用 `APP_KEY` 加密存储，关闭时需要一个当前有效的验证码。
- 由于该门是可选启用的，未设置时整个界面和 API（包括异地设置、篡改测试路由和恢复工具包）对任何能访问该端口的人都可访问。在使用异地、不可变备份或加密后就启用该门。
- 请仅在受信任、未对外暴露的网络上运行 BombVault。对于远程访问，请将它置于一个添加了身份验证和 TLS 的反向代理之后。响应携带基线安全头（CSP、`nosniff`、`X-Frame-Options`、`Referrer-Policy`）。
- 在反向代理后面，每个请求携带的都是代理的地址，因此不设置 `TRUSTED_PROXY` 时登录限流会把所有客户端算作一个，攻击者的失败也会把你锁在门外。在 `TRUSTED_PROXY` 中写明代理，即可恢复按客户端计数。
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

异地副本会发送到存储位置。在**设置，存储**中用**添加存储位置**添加存储位置，然后在该域所在行的**复制到**下勾选它。[存储位置](storage-places.md)涵盖每种连接类型，[异地与恢复](offsite-recovery.md)涵盖 append-only、篡改测试和 DR 演练。简而言之：

- **连接类型：** 此 Unraid 上的文件夹或挂载在 `/mnt/remotes` 下的 NAS 共享、S3 存储（Backblaze B2 和其他云提供商，或 MinIO、Garage 等自托管服务）、rest-server、SFTP（包括 Hetzner Storage Box）、用于 Nextcloud、ownCloud 和 OpenCloud 的 WebDAV、Azure Blob，以及任意 rclone 远程。Backblaze B2 只需要密钥：BombVault 会从中读取存储桶和 S3 端点。
- **凭据**以加密方式与其所属的存储位置一起存储。拉取来源所用的凭据集位于实例页面的**拉取**标签页上。
- **SSH 目标无需在对端安装任何东西。** SFTP 存储位置只需要一个 SSH 服务器。将 SFTP 表单中显示的公钥（也可在**设置，系统，通过 SSH 备份 VM**中以及 `/config/ssh/id_ed25519.pub` 找到）添加到目标用户的 `~/.ssh/authorized_keys`。
- **异地复制：** BombVault 以尽力而为的方式用 `restic copy` 复制新快照，作为域所存放的存储位置之外的额外副本。每个域在设置，计划中都有各自的复制计划，其所在行上还有**立即复制**。
- **每个域可有多个复制存储位置：** 在**复制到**下勾选任意多个存储位置；每个都按该域的计划进行复制。
- **保留策略、限制、存储类别和增长预算属于存储位置**，在其详情中设置。存储位置的保留策略适用于其上的每个仓库，因此异地存储位置可以把副本作为归档保留更久；所有规则都为零的存储位置从不精简。
- **冷存储与归档存储类别（S3）：** 对于 S3 存储位置，选择一个可还原读取的层级（Standard、Standard-IA、One Zone-IA、Intelligent-Tiering、Glacier Instant Retrieval）。rclone 远程在 rclone 配置中设置其类别。
- **存放在远程存储位置的域：** 参见[存放在远程存储位置的域](offsite-recovery.md#remote-primary-repositories)。

## 可移植设置（导出与导入） {#portable-settings-export-and-import}

设置页面上的**导出与导入设置**卡片会将您的整套 BombVault 配置（域设置、存储位置、计划、通知）写入一个可移植的 JSON 文件，您可以在另一个实例上导入它，因此迁移到新机器或克隆一套配置不再意味着要手动重新录入一切。导入会显示预览并要求确认，且绝不会触动您的备份数据或历史。预览会统计文件中的存储位置，在包含凭据时还会统计凭据集；对于没有存储位置的旧文件，BombVault 会根据导入的设置构建存储位置。

!!! warning "导出文件可能包含凭据"
    您可以选择是否在文件中包含存储位置和通知的凭据。包含凭据时，导出文件与您的恢复工具包一样敏感，因此请将它存放在安全的地方。不包含凭据时，该文件只保存非机密的设置。
