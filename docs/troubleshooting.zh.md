# 故障排除

一份简短的常见问题。通过 SSH 的虚拟机备份在主机端的完整故障排除表（permission-denied、主机密钥验证、缺少模板变量等）见[通过 SSH 的虚拟机备份指南](vm-backup-ssh-setup.md)。

## 有东西没接好 {#something-is-not-wired-up-correctly}

在概览的**自定义**下添加**主机集成**卡片。主机集成检查会探测每个挂载和命令行工具（Docker 套接字、libvirt、restic、qemu-img、rclone），并报告缺少的部分，**立即检查**会再运行一次。在怀疑是 bug 之前先从这里看：缺少的挂载或连不上的主机会立刻显示出来。

## 我打不开网页界面 {#i-cannot-reach-the-web-ui}

BombVault 开箱即用地在 `3443` 端口提供 HTTPS（自签名证书），所以请打开 `https://<your-unraid-ip>:3443`。接受自签名证书的警告，或者把 BombVault 放在带有你自己证书的反向代理后面。如果你用 `HTTP_ONLY=true` 运行，它会改为在 `3000` 端口提供纯 HTTP（用于终止 TLS 的代理后面）。

## 我丢了 APP_KEY {#i-lost-my-app_key}

`APP_KEY` 派生出 restic 仓库的密码。没有它，没有恢复工具包，你的组里也没有密钥托管，加密的备份就无法恢复。这就是为什么在你下载之前，概览一直显示**保存你的恢复工具包**。参见[异地与恢复](offsite-recovery.md#recovery-kit)。用 `openssl rand -hex 32` 生成一个密钥，并在依赖任何备份之前把它存放在服务器之外。

## 我忘了登录密码 {#i-forgot-the-login-password}

在登录页面上选择**忘记密码？**。BombVault 会把一个 12 个字符的代码写进它的容器日志（在 Unraid 上：Docker 标签页，BombVault 图标，**Logs**）。请在 10 分钟内连同新密码一起输入。重置还会关闭两步验证、移除所有通行密钥、撤销所有 API 令牌、MCP 密钥和 OAuth 登录、结束所有其他会话，并关闭手机配对。请在**设置、安全**下重新添加你的通行密钥，在那里重新打开手机配对，并在仍然需要的地方创建新的令牌和密钥。参见[安全模型](configuration.md#login)。

## 备份停止了，并说密钥无法再打开 {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

这台服务器在应急向导里接管了另一台服务器的备份密钥，之后 `APP_KEY` 被更改了。接管的密钥是用旧的 APP_KEY 封存的，所以 BombVault 打不开它，宁可什么都不备份，也不用错误的密钥写入。概览上的**需要处理**会点名它。请把 APP_KEY 改回原来的值，或在恢复页面的**紧急**下再次上传恢复工具包。

## 手机停在登录页面 {#a-phone-lands-on-the-login-page}

只有当**设置、安全**下的**允许手机用群组的十二个单词配对**开启时，已配对的手机才会通过组得到会话。重置密码会把它关闭。比服务器旧的应用在更新之前同样得不到会话。通过地址添加的服务器总是要求输入密码。从实例页面移除的手机，在它的应用再次打开时会以关闭状态回来。请在它详情的**监控**下开启**查询此实例**，让它进来。

## 虚拟机备份连不上 {#vm-backup-will-not-connect}

虚拟机备份通过 SSH 与 libvirt 通信，从不使用挂载。

- 确认主机上已启用 SSH，并且 BombVault 的公钥已在 `/root/.ssh/authorized_keys` 里授权（Settings, Integrations, Host SSH 会显示密钥和一个**测试连接**按钮）。
- 在自定义的 `br0.x` 网络上，请把 `LIBVIRT_HOST` 设为你 Unraid 的局域网 IP（在那里容器无法通过 `host.docker.internal` 访问主机）。启用 **Settings, Docker, Host access to custom networks**。
- 如果你更改过 Unraid 的 SSH 端口，请把 `LIBVIRT_SSH_PORT` 设成一致。
- 在 TrueNAS 上，用 TrueNAS 自己的 libvirt 套接字设置 `LIBVIRT_URI`，给应用 `SETPCAP` 权限，并在 **Credentials, Users** 下把密钥添加到 root（参见 [TrueNAS SCALE](getting-started.md#truenas)）。
- 完整的分步诊断（可达性测试、VLAN 路由、`Permission denied (publickey)`、`Host key verification failed`）见[通过 SSH 的虚拟机备份指南](vm-backup-ssh-setup.md)。

## 实时虚拟机快照没有运行 {#a-live-vm-snapshot-did-not-run}

实时快照需要磁盘在 `/mnt/cache`（或 `/mnt/diskX`）上，而不是 `/mnt/user`。虚拟机里装有 qemu guest agent 时，快照是静默的，没有它时，快照是崩溃一致的。对已关机的虚拟机，实时方式会自动回退到正常关机方式。正常关机方式的备份会先关闭虚拟机，备份磁盘，然后再重启它，所以总是一致的。它最多等待客户机关机十分钟，也绝不会强制断电，所以正在安装更新的 Windows 客户机不会被中途切断。之后仍在运行的客户机会被放着不动，这次备份会带着原因失败。

## 备份因"repository is already locked"而失败 {#a-backup-failed-with-repository-is-already-locked}

这通常是容器在操作中途被更新或重启时留下的孤立 restic 锁。BombVault 会检测出确定孤立的锁，自动强制清除它并重试一次。如果问题依旧，请打开**设置、验证**下的**检查备份**卡片，在受影响分区那一行的**更多**下选择**解锁**，手动清除过期的锁。真正的问题仍然会暴露出来，而不会被藏起来。重启之后，BombVault 会等到这样的锁超过十分钟没有刷新。仍在运行的 restic，例如在同一个仓库上的第二个 BombVault 里，会每五分钟刷新一次它的锁。

## 备份之后我的异地副本没有发生 {#my-off-site-copy-did-not-happen-after-a-backup}

异地复制在设计上是尽力而为的，所以异地的小故障绝不会让本地备份失败。请检查存储位置何时复制：它的**复制**卡片会写**每次备份后**，或者写出它自己的计划，**设置、计划、按分区的异地副本**可以给某个分区单独的计划。需要立即运行时，用**设置、目标**下**异地副本**卡片里该分区那一行的**立即复制**，并留意概览的状态栏，它在运行时会显示**正在异地复制**。失败的复制会显示在**需要处理**下。

## rest-server 的证书不受信任 {#a-rest-servers-certificate-is-not-trusted}

restic 会拒绝它不认识的证书，错误信息会说明该放在哪里。把 rest-server 的自签名证书，或签发它的机构的证书，作为 PEM 文件复制到 BombVault 数据文件夹的 `certs` 文件夹里（容器里是 `/config/certs`，Unraid 上是 `appdata/bombvault/certs`），然后再测试一次。不需要重启。

## 备份提示 Host Data 之外的数据没有备份 {#data-outside-host-data}

容器把数据放在 BombVault 无法通过 Host Data 读取的文件夹里。备份会保存定义和它能到达的每个文件夹，把活动日志中它那一行变成橙色，并点名漏掉的文件夹。有两种常见情况：TrueNAS 上 `/mnt/.ix-apps` 下的 ixVolume，以及 `/var/lib/docker/volumes` 下的 Docker 命名卷，例如 compose 堆栈的数据库卷。容器**设置**标签页上的**备份文件夹**卡片会列出每个这样的文件夹，带它在容器内的路径和主机路径，并标成红色。

匿名 Docker 卷不会被报告。Docker 会为镜像声明了但没有人映射的文件夹自己创建一个，用 64 个十六进制数字命名，放在 `/var/lib/docker/volumes` 下。它通常存放的是缓存，所以备份保持绿色，卡片只是列出它，并注明它是匿名的。

- **你想备份那份数据。** 把文件夹映射到 Host Data 之下的主机路径，例如你的 appdata 共享里的一个文件夹，下一次备份就会包含它。
- **你不需要它。** 在同一标签页的**排除模式**卡片里，把它的两个路径之一添加为一行：容器内的路径，例如 `/cache`，或警告里点名的主机路径，例如 `/var/lib/docker/volumes/jellyfin_cache/_data`。从下一次备份起，警告会把那个文件夹略去，字段下方的预览会说明原因。写上它上一级文件夹的一行也可以，单独的名称或 `/` 则不行。

## 备份显示已推迟 {#deferred-backups}

容器的应用很忙，所以它的备份在等待，并会自己再试。活动日志里那一行会说明原因以及下次尝试的时间。容器**设置**标签页上的**应用繁忙时**卡片说的是同样的内容，以及等待何时结束。不需要做任何事。要立即备份，请用**立即备份**：你自己启动的备份从不等待，退出码 75 也不会让它等。

- **你自己的检查。** 让**备份前命令**在应用繁忙时以退出码 75 退出，例如 `[ "$(cat /tmp/players)" = 0 ] || exit 75`。任何其他非 0 的退出码仍然会像以前一样让备份失败，所以有问题的脚本依然会显示在**需要处理**下。
- **从未询问 Plex、Jellyfin 或 Emby。** BombVault 先在容器自己的地址上询问服务器，如果没有应答，就在容器内运行 curl 或 wget。两者都没有的镜像无法通过自身被询问，日志里会这样说明。开启**服务器无响应时仍然备份**时，这样的备份仍然会运行，关闭则会等待。
- **Jellyfin 或 Emby 拒绝。** 检查 **API 密钥**。它在服务器控制台的 API 密钥下创建。
- **Plex 拒绝。** BombVault 从挂载在 `/config` 的文件夹里的 `Preferences.xml` 读取令牌。从未登录过的服务器没有令牌，只在允许无令牌访问的地方应答。
- **暂停的电影让备份整晚都在等。** 设置**忽略暂停超过此时长的会话（分钟）**，例如设为 30。
- **流量检查始终不放行备份。** 把**繁忙阈值（KiB/s）**调到高于没人使用时容器所发送的流量。BombVault 刚启动后，检查会等到它观察了容器**保持安静（分钟）**所设的分钟数为止。
- **备份在夜里从不运行。** 设置了时间窗口时，只在窗口内尝试。到截止时间仍在窗口之外的等待，仍然在截止时间结束。

## 大型备份因内存不足被杀掉 {#a-large-backup-was-killed-for-running-out-of-memory}

在**设置、系统**下的**性能**卡片里设置**每次备份的内存上限 (MiB, 0 = 无)**，要远低于容器可以使用的量。接近上限时，restic 会更频繁地释放内存，这会损失一些速度，所以备份会变慢，而不是被杀掉。

## 恢复在开始之前就中止了 {#a-restore-aborted-before-it-started}

在停止或删除任何东西之前，恢复会先运行一次预检冲突检查：它验证容器的静态 IP 和已发布的主机端口是否空闲。如果另一个容器已经占用了其中一个，它会中止并给出清楚、可操作的信息，而不是留下做了一半的恢复。请释放冲突的端口或 IP，然后重试。

如果容器的网络不存在，例如因为它的 TrueNAS 应用被删除了，恢复检查会点名这个网络，恢复会在改动任何东西之前拒绝执行。请在 TrueNAS 里用同样的名称重新安装这个应用，这会重新创建网络，然后再恢复它的数据。

## 普通导出失败了，没有写出文件 {#a-plain-export-failed-instead-of-writing-a-file}

如果**设置、系统**下的**使用 age 加密导出**是开启的，但没有设置有效的收件人，导出会带着清楚的错误失败，而不是写出明文。请添加有效的收件人（age 公钥或 SSH 公钥），或者如果你本来就想导出明文，就关闭加密。参见[普通导出](backups.md#exports)。

## 数据库转储失败了 {#a-database-dump-failed}

失败的转储绝不会让它周围的备份失败。它会被记录为单独的失败运行，原因会点明该修复什么。

- **登录被拒绝。** 转储用容器自己的密码变量登录（`POSTGRES_PASSWORD`、`MARIADB_ROOT_PASSWORD`、`MYSQL_ROOT_PASSWORD` 或它们的 `_FILE` 版本）。请在数据库容器上检查它们。指向容器自己的用户无法读取的密钥的 `_FILE` 变量，会以同样的方式失败。
- **权限不足。** root 密码是随机的时，转储只能以应用用户登录，所以只能转储那一个数据库，MySQL 8.4 及更新版本可能会直接拒绝。请给容器设置一个真正的 root 密码，或者为它关闭转储。
- **系统表需要升级。** 当 MariaDB 的系统表来自较旧的版本时，它会拒绝转储（错误 1558）。添加变量 `MARIADB_AUTO_UPGRADE=1` 并重启容器，或者在容器里运行一次 `mariadb-upgrade`。
- **没有转储工具。** 没有 `pg_dump`、`mysqldump` 或 `mariadb-dump` 的精简或自制镜像无法转储。请使用官方镜像，或者关闭转储。
- **时间限制。** 一次转储有 `DB_DUMP_MAX_HOURS`（默认 6），它周围的备份有 `BACKUP_MAX_HOURS`，而不再有进展的转储会在 `BACKUP_STALL_HOURS` 之后被切断。最后一种的常见原因是应用持有的锁。请调高触发的那个限制，或在应用安静时转储。
- **容器已暂停或正在重启。** 转储与运行中的服务器通信。如果容器一直重启，它自己的日志会说明原因。
- **损坏的转储无法删除。** BombVault 无法完成的转储会被再次删除。当这次删除失败时，转储会作为损坏的留在列表里，你可以在那里删除它。

## 导入失败了 {#an-import-failed}

导入会停止容器，把它的数据文件夹挪到一边，让镜像在原位置创建一个空的。如果导入本身之前的某一步失败，旧文件夹会自动放回去。如果导入失败，容器保留新的文件夹，旧的则作为 `<data folder>.bombvault-before-import-<timestamp>` 留在旁边。运行的错误信息会写出确切的路径。

本身就是一个数据集的数据文件夹（例如在 TrueNAS 上）无法重命名。这时导入会让文件夹留在原处，把它的内容挪到旁边保留的文件夹里，所以上一级数据集需要有足够的空间。导入会在停止容器之前检查这一点。

要手动放回去：停止容器，把当前的数据文件夹重命名挪开，把保留的文件夹改回原来的名字，然后启动容器。对于数据集，请移动内容，而不是重命名文件夹。在 Unraid 上，可以在 Shares 标签页用文件管理器完成。

## ZFS 数据集备份失败，或跳过了某个数据集 {#zfs-datasets}

每个问题都带有一个放在括号里的原因代码，[ZFS 数据集](zfs-datasets.md#reason-codes)页面列出了全部代码和修复办法。最常见的三个：

- **`snapshot-loop`**：快照没有到达 BombVault，因为 Host Data 不传递新的挂载。编辑容器，把 Host Data 的 Access Mode 设为 Read/Write - Slave，然后重启 BombVault。
- **`key-not-loaded`**：密钥没有加载的加密数据集会被跳过。用 `zfs load-key` 加载密钥并挂载数据集。下一次备份就会包含它。
- **`ssh-auth`**：服务器拒绝了 BombVault 的密钥。备份页面上用**查看连接**打开的**与这台服务器的 ZFS 连接**会显示授权它的命令。在服务器上运行一次。

## 条目一直停在"学习中 N/10" {#an-entry-stays-at-learning-n10}

大多数异常检查在一个条目成功备份 10 次之后才开始，在**标记为预期之内**之后以及条目的选择改变之后，计数会重新开始。没有计划的条目不会学习，没有 appdata 的容器无从学习，它的徽章会这样说明。异常页面上的**学习进度**为每个条目显示一个分成十份的圆环。

## 保留规则不再删除某个条目的旧备份 {#retention-stopped-deleting-old-backups-of-one-entry}

有一个未处理的严重异常拦住了它们：这个条目的源几乎为空、急剧缩小，或者一次备份把大部分数据重新保存了一遍。请从条目上的徽章打开这个异常。如果数据丢了或被加密了，请先从链接的最后一份好备份恢复。然后确认这个异常，或者如果变化是你做的，就把它标记为预期之内，下一次运行就会照常清理。保留预览会把这样的条目标为保留。对于 ZFS 条目，只有异常里点名的数据集会保留它的旧备份，树里的其他数据集照常清理。

## 手动清理说有些条目被保留了 {#manual-prune-says-some-entries-were-kept}

原因相同：清理不会动带有这种异常的条目的旧备份，并在它的信息里点名这个条目。其他一切照常清理。

## 备份因"on no mounted pool or share"而失败 {#no-mounted-pool}

在 Unraid 上，`/mnt` 位于内存中，所以那里没有挂载池的文件夹会在下次重启时消失，而池恢复后又会被遮住。BombVault 拒绝把备份或闪存 ZIP 写到那里，即使那个文件夹里已经有一个早先写入的仓库也是如此。请挂载池，然后再次运行备份。

## 历史导入说某个仓库无法读取 {#history-import-says-a-repository-could-not-be-read}

升级之后，BombVault 会从每个仓库读取一次早先备份的大小。当时无法访问的仓库，例如宕机的异地目标或没有挂载的共享，会计入**设置、验证**下**异常**卡片，并每天重试一次。在此期间，它的条目会从新的备份中学习。

## 磁盘空间警告与 Unraid 仪表盘不一致 {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

在 Unraid 用户共享（`/mnt/user`）上，可用空间是整个阵列的，而不是某一块磁盘的。BombVault 通过 rclone 访问的远程存储位置，例如 SFTP、Hetzner Storage Box、SMB、WebDAV 或云盘，会在对方报告可用空间时被测量，手动输入的 `sftp:` 地址也一样。S3、B2 和 REST 仓库没有这个数字：它们的存储位置会显示**大小未知**，**异常**卡片会把它们列为未测量。

## AI 助手连不上 {#an-ai-assistant-cannot-connect}

[MCP 服务器页面](mcp.md#troubleshooting)列出了 MCP 端点的每个状态码和每种拒绝的含义，以及该怎么办。

## 容器一直重启，或看起来不健康 {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault 根据自己的 `/api/health` 报告健康或不健康。自动修复工具（例如 Autoheal）可以在引擎卡住时自动重启它。请检查容器日志和**主机集成**卡片，找出根本原因。

## 存储位置拒绝保存 {#location-refused}

BombVault 在保存位置之前会先检查它，拒绝时会用一句话说明原因。常见的有：

- **"This location lies inside another repository or target, or contains one."** 仓库里再放仓库会让两者都出问题。信息会点名另一个位置。请选另一个文件夹，或先移除那一个。
- **它是你设置的一个仓库。** 这个位置是**存储库**下的某一行，或位于其内部或周围。请选择另一个地方，或先移除那个仓库。
- **与某个分区的专属文件夹是同一个地方。** 在分区的专属文件夹上做复制，会在每次运行时把它的备份复制到它们自己上，所以会被拒绝。请选另一个地方，或参见[分区的专属文件夹](offsite-recovery.md#own-folder)。
- **"That is not an off-site target of this domain"** 来自指向一个已被移除的位置的链接。当前的版本在启动时和导入时会清除这些。如果你仍然看到它，请更新 BombVault。
- **完整的主机路径**，例如 `/mnt/user/backups`。BombVault 在容器里运行，所以它需要 Host Data 之下的路径 `user/backups`，信息会点出该输入的那一个。

## 远程存储位置说它的仓库不见了 {#repository-gone}

曾经存有备份、突然没有仓库的位置，不会自己得到一个新的空仓库，因为一旦磁盘恢复，那个新的会盖住真正的仓库。常见的原因是 rest-server 的磁盘没有挂载，所以请重新挂载磁盘，再运行一次备份或复制。只有当你有意清空或删除了旧仓库时，才在位置的页面上点击**在这里新建存储库**。它会对该位置的所有分区一次性重新开始。

## 云端登录过期了 {#cloud-sign-in-expired}

测试或下一次复制会说登录已过期。位置页面上的**更改连接**会重新登录。

- **Box、Jottacloud 以及其他使用一次性刷新令牌的服务：** BombVault 会保留 rclone 刷新过的令牌，重启之后以及在恢复工具包里也一样。从较旧的设置备份恢复之后，里面的令牌可能已经用完，所以请再登录一次。仓库保持原样。
- **iCloud Drive：** Apple 会在 30 天后结束登录。它需要来自 `rclone config` 的新信任令牌和 cookie。
- **OneDrive：** 有多个驱动器的登录会询问用哪一个，所以没有做选择的位置在你选择之前一直无法使用。关于 OneDrive Business 和 Google Drive，请看[存储位置](offsite-recovery.md#destinations)下关于被清理数据的说明。

## "上一次备份的异地副本仍在运行" {#copy-still-running}

前一个备份已经完成，它的副本还在路上，这会阻止该分区的新备份。请等到状态栏不再显示**正在异地复制**为止。启动备份的按钮在等待时也会这样说明。到新位置的第一次复制可能要花几个小时，参见 [S3 示例](offsite-recovery.md#example-s3)。

## ZFS 副本停止，因为目标上有更新的快照 {#replica-target-newer}

目标上有比它与这台服务器共有的最新快照更新的快照，例如因为这里的数据集被回滚了，或目标上的副本被改动了。副本卡片和运行历史会在**将目标回滚到**下点出那个快照。请对运行点名的每个数据集，在目标上运行 `zfs rollback -r` 回到那个快照，然后点击**立即复制**。参见[快照保留多久](zfs-datasets.md#replica-retention)。

## 有 Unraid 快照的虚拟机失败，并点名一个文件 {#vm-chain}

BombVault 会沿着每个磁盘的后备链，备份其中的每个文件。无法一路追到末端的链会让备份失败，而不是漏掉磁盘的一部分，信息会点名那个文件。请检查文件是否存在于 `/mnt` 之下，以及快照是否仍然列在虚拟机管理器里。Host Data 之外的文件根本无法读取。参见[带 Unraid 快照的虚拟机](vm-backup-ssh-setup.md#unraid-snapshots)。

## 重建之后异地副本在等你做出决定 {#copies-paused}

在**发现备份**或没有恢复设置的应急向导之后，或当某个分区找到了这个数据库之前它自己的备份时，该分区的异地副本会等待，因为 BombVault 不再知道你排除了哪些条目。**需要处理**和该存储位置的页面会询问这件事。点击**接收为此服务器自己的备份**继续，或者点击**先检查规则**，先看看该分区的规则再决定。导入设置文件不会替你做决定。参见[重建之后](offsite-recovery.md#after-a-rebuild)。

## 收不到通知 {#notifications-do-not-arrive}

先从**设置、通知**下的**发送测试**开始。卡片下面的那一行会说明发生了什么。

- **什么都没有发送。** 检查**通知**。设为**从不**时不会发出任何消息，只有 Healthchecks 网址的 ping，设为**仅在失败时**时，成功的运行保持安静。开启**汇总计划运行**时，计划运行会在结束时发送一份摘要。
- **新的地址。** 更改 Matrix、Gotify 或邮件的服务器之后，字段会再次要求输入令牌或密码，新地址只有在你输入之后才会保存。BombVault 不会把旧的密钥发到新的地方。
- **Gotify** 说服务器拒绝了应用令牌：请从 Gotify 里的应用中重新复制。**服务器 URL** 是基础地址，不带 `/message`。
- **Webhook：** 检查 **Webhook 格式**是否适合该服务。Discord 地址需要 **Discord**，ntfy 需要 **ntfy**。
- **Unraid 通知**通过主机 SSH 连接运行，所以请先在**设置、集成**下测试它。在其他平台上它保持关闭，卡片会这样说明。
- **通知邮件始终收不到。** 对照端口检查**加密**，通常 587 端口用 **STARTTLS**，465 端口用 **TLS（隐式）**。

Healthchecks 的 ping 不依赖以上任何一项，参见 [Healthchecks](notifications.md#healthchecks)。

## Android 应用打不开某台服务器 {#android-cannot-open}

起始屏幕会在服务器的卡片上说明原因：

- **未连接：** 服务器的地址和中继都没有应答。使用**不用中继**时，应用只能在服务器自己的地址上访问它。
- **检查证书：** 服务器显示的证书与你信任过的不同。只有在你信任新证书之后，应用才会打开它，所以请确认这个变化是你做的。
- **需要登录：** 通过地址添加的服务器要求输入密码。另见[手机停在登录页面](#a-phone-lands-on-the-login-page)。
- **密钥变了：** 起始屏幕会显示新密钥的指纹，你信任它之后页面才会打开。参见[离家在外](android.md#away)。

## 其他用户读不到本地仓库 {#repository-permissions}

本地仓库对其他用户保持可读，例如通过 SMB 的同步用户。在 10.4.1 之前，清理、标签、拉取、分区移动、删除或堆栈备份所写的文件，可能要到下一个夜晚之前一直不可读。更新 BombVault 之后，它们立即可读。自我备份的仓库有意只让 root 可读，因为里面有 BombVault 的设置。

## 还是解决不了？ {#still-stuck}

- 阅读完整的[配置](configuration.md)和[异地与恢复](offsite-recovery.md)页面。
- **设置、信息**下**帮助**卡片里的**错误报告**会下载一个用于 GitHub issue 的 ZIP：主机检查、去除了所有密钥的配置、最近的运行、接下来计划的内容，以及容器上次启动以来的最近日志。里面还有每个数据库的最后一次转储、带挂载的 ZFS 条目、未处理的异常，以及有多少个 MCP 密钥。密码、令牌和 rclone 配置都已去除，压缩包在它自己的清单里这样说明。它需要登录密码。对于导致容器重启的崩溃，请查看 `docker logs`。
- 到 [Unraid 支持帖](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)里提问。
- 提交一个 [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues)。
