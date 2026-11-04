# 功能

BombVault 功能的简短目录。每一行都链接到讲解它的页面。

## 备份范围 {#backup-scope}

- **容器**：appdata 文件夹和容器定义，每个容器可以选择文件夹。[备份与条目](backups.md#what-is-saved)
- **虚拟机**：磁盘映像、XML 和 UEFI NVRAM，通过 SSH，支持正常关机、实时快照或仅变更的块。带 Unraid 快照的虚拟机和放在 zvol 上的磁盘有完整的说明。[虚拟机备份](vm-backup-ssh-setup.md)
- **闪存**：整个 Unraid USB 闪存，可以按计划导出 zip。[备份与条目](backups.md#settings-tab)
- **自我备份**：BombVault 自己的设置和登录凭据。[应急向导](offsite-recovery.md#emergency)
- **文件夹**：服务器上的任意文件夹，作为文件夹集。[添加文件夹](backups.md#add)
- **ZFS 数据集**：从一个快照读取的数据集树，可以恢复单个文件，并可选地在第二台 ZFS 服务器上做副本。[ZFS 数据集](zfs-datasets.md)
- **数据库转储**：PostgreSQL、MySQL 和 MariaDB，从运行中的服务器转储到它们自己的恢复点。[数据库转储](backups.md#database-dumps)

## 备份和条目页面 {#backups}

- **一个列表**，带磁贴、**筛选**、**排序**和**搜索**，每一行用一条小色带显示最近十四次运行。[列表](backups.md#list)
- **像在文件管理器里一样选择**，并在服务器上备份或暂停所选内容。[选择多个条目](backups.md#select)
- **每个条目一个页面**，带**备份**、**设置**和**验证**。[条目的页面](backups.md#entry-page)
- 快照的**标签和比较**，以及以 tar、zip 或 restic 仓库形式**导出**。[备份标签页](backups.md#backups-tab)
- 带实时预览的**排除模式**，以及知名应用的推荐排除项。[排除模式](backups.md#excludes)
- **前后命令**，以及**停止其他容器**，按依赖顺序、等健康后再重启。[命令与其他容器](backups.md#commands)
- **应用繁忙时**：计划备份会等 Plex、Jellyfin、Emby、你自己的检查或安静的流量，并自动重试。[应用繁忙时](backups.md#app-busy)
- **用快照代替停止**：ZFS 上的容器只停机几秒钟。[计划](schedules.md#snapshot-instead-of-stop)
- **普通导出**，可选地用 age 封存。[普通导出](backups.md#exports)

## 恢复 {#restore}

- 所有种类共用**一个页面和一个窗口**，预先选中推荐的备份。[恢复](restore.md)
- 默认**放在原件旁边**，**替换原件**会先备份当前状态。[恢复窗口](restore.md#window)
- **单个文件**恢复到文件夹或放回原位。[找回一个文件](restore.md#example-single-file)
- **一次恢复多个条目**，各用它推荐的备份，或用同一个时刻。[一次恢复多个条目](restore.md#several)
- **整个 Compose 堆栈**，按依赖顺序启动。[恢复堆栈](restore.md#stacks)
- **容器和虚拟机会重新创建**，使用它们运行时的镜像，带 GPU、限制和链接。[按种类恢复什么](restore.md#kinds)
- **恢复前的检查**，以及它会改变什么的列表。[恢复窗口](restore.md#window)
- **闪存以 ZIP 形式恢复**，也可以一次把一个插件放回正在运行的闪存。[按种类恢复什么](restore.md#kinds)
- **ZFS** 恢复到数据集（带安全快照）、恢复到文件夹，或恢复到带属性的新数据集。[ZFS 数据集](zfs-datasets.md#restore)
- **应急向导**用于丢失的服务器，使用恢复工具包、通过组，或输入密钥。[应急向导](offsite-recovery.md#emergency)
- **从旧的备份文件夹**，以及**从 Appdata.Backup 插件导入**。[其他来源](restore.md#other-sources)

## 存储与计划 {#storage-scheduling}

- 各种**存储位置**：本地文件夹、SMB、WebDAV、SFTP、rest-server、S3、云盘和任意 rclone 远端。[存储位置](offsite-recovery.md#destinations)
- **登录凭据**：一个带默认项的列表。[登录凭据](offsite-recovery.md#sign-ins)
- 这台服务器上或存储位置里的**专属文件夹**。[分区的专属文件夹](offsite-recovery.md#own-folder)
- **3-2-1 流程**，每一行都是一个开关，以及**按条目放置**。[按条目放置](offsite-recovery.md#placement)
- 每个分区可以**异地复制**到多个位置，带带宽限制、压缩、冷存储类别，以及 Plex 串流时的减速。[异地副本](offsite-recovery.md#off-site-copies)
- **直接仓库**，列在**直接备份到这里**下。[直接仓库](offsite-recovery.md#direct-repositories)
- **以时间线呈现的保留**：每个位置选**短**、**均衡**、**长**或**自定义值**，并可按分区设置规则。[保留多久](offsite-recovery.md#keep)
- **移动分区的历史**，以及带或不带备份的**移除目标**。[移动与移除](offsite-recovery.md#move-remove)
- **读起来像句子的计划**、**一周**带、全部备份、按分区的计划和每个条目的独立计划。[计划](schedules.md)
- **补做错过的备份**、手动顺序，以及备份成功后的更新。[计划](schedules.md)

## 洞察、验证与监控 {#insight-verification-monitoring}

- **概览**：状态栏、**需要处理**、**保护状态**、存储预测、历史和可选的卡片。[概览](overview.md)
- 按条目的**异常检测**，在源崩溃时也会保留旧备份。[异常](anomalies.md)
- 第一次备份之后的**恢复检查**，以及在隔离副本中的**启动测试**。[验证标签页](backups.md#verify-tab)
- 来自异地副本的**计划检查和测试恢复**。[DR 演练](offsite-recovery.md#dr-drills)
- **按文件夹统计大小**，以及备份为什么慢。[运行会告诉你什么](backups.md#run-notes)
- **通知**：webhook、Gotify、Matrix、邮件、Apprise、Unraid 和 Healthchecks，带每周摘要和逾期报告。[通知](notifications.md)
- **Prometheus**、Home Assistant 设备、HTTP API 和 mDNS。[API 与集成](api.md)
- **错误报告**：去除敏感信息的 ZIP，用于 GitHub issue。[故障排除](troubleshooting.md#still-stuck)

## 勒索软件防护 {#ransomware-protection}

- **防删除保护（append-only）**，由对方强制执行，并通过防篡改测试证明。[防删除保护](offsite-recovery.md#delete-protection-append-only)
- 对无法拒绝删除的位置，可以**在没有防删除保护的情况下接受**。[防删除保护](offsite-recovery.md#delete-protection-append-only)
- **永久删除备份需要登录密码。**[安全模型](configuration.md#security-model)
- **恢复工具包**，可选地用 age 封存，以及你组里的**密钥保管端**。[恢复工具包](offsite-recovery.md#recovery-kit)、[密钥保管端](offsite-recovery.md#key-keeper)

## 多台服务器 {#several-servers}

- **用十二个单词配对**，端到端加密，直接连接或通过中继。[配对实例](offsite-recovery.md#pairing)
- **实例页面**，把组画成一张网，带**接收端**、**拉取端**、**ZFS 服务器**和**密钥保管端**这几个角色。[实例和角色](offsite-recovery.md#roles)
- 为你**设置好的接收服务器**，处于 append-only 模式。[接收服务器](offsite-recovery.md#receiving-server)

## AI 助手 (MCP) {#mcp}

内置的 MCP 服务器让助手可以读取备份状态、覆盖情况、运行历史和恢复点，并且在密钥允许时启动备份。恢复、删除、清理和设置仍然只在网页界面里进行。[MCP 服务器](mcp.md)

## 应用和配套工具 {#apps}

- **Android 应用**：你组里的每台服务器都在手机上。[Android 应用](android.md)
- **BombVault Widget**：Unraid 仪表盘上的一个磁贴。[应用](settings.md#apps)
- 可嵌入的**活动日志**，用于 Homepage 或 Heimdall 这类仪表盘。[仪表盘小组件](api.md#dashboard-widget)

## 其他 {#other}

- **42 种语言**、三种主题，以及你自己选择的标签、颜色、圆角和动画。[外观](settings.md#look)
- **自动保存的设置**，可以搜索全部十四个磁贴。[设置](settings.md)
- 更新后的**新增内容**，用绿点标出有变化的地方。[信息](settings.md#info)
- 开箱即用的 **HTTPS**，以及给自动修复工具用的健康检查。[配置](configuration.md#network)
