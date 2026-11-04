# 示例

下面每个示例都把一项真实的工作从头走到尾，名称和数字都已填好。它们依托于讲解对应功能的页面，所以细节就在旁边。

## 快速上手 {#start}

- [第一次容器备份，从头到尾](getting-started.md#example-first-backup)：从安装到恢复工具包。

## 存储与异地 {#storage}

- [在 Backblaze B2 上做异地副本](offsite-recovery.md#example-s3)：一个存储桶、一把只用于该桶的密钥，以及第一份副本。Cloudflare R2 的做法相同。
- [自己运行的 rest-server](offsite-recovery.md#example-rest-server)：一份拒绝删除的异地副本，附 `docker run` 命令和防篡改测试。
- [均衡保留什么](offsite-recovery.md#example-retention)：两年每晚备份的保留规则，一份一份地看。
- [容器搬到 NAS](offsite-recovery.md#example-own-folder)：分区的专属文件夹换了位置，以及已有的备份会怎样。

## 备份与计划 {#backups}

- [三个互相配合的计划](schedules.md#example)：夜里做全部备份，虚拟机每周一次，再给一个条目单独设置每 6 小时一次的计划。
- [有人在看时 Plex 先等一等](backups.md#example-plex)：计划备份不会打断正在放的电影。
- [有两个 Unraid 快照的虚拟机](vm-backup-ssh-setup.md#example-unraid-snapshots)：保留快照的实时备份，以及放在原件旁边的恢复。
- [复制到第二台 Unraid 机器的副本](zfs-datasets.md#example-replica)：随时可以读取的 ZFS 副本，以及从中取出单个文件的三种方式。

## 把数据找回来 {#restore}

- [找回一个文件](restore.md#example-single-file)：从昨晚的容器备份里取出单个文件。
- [从 ZFS 备份找回一个文件](zfs-datasets.md#example-zfs-file)：数据集也是同样的做法。
- [用恢复工具包装好一台新服务器](offsite-recovery.md#example-emergency)：一台坏掉的服务器在新机器上重生，模板里不需要旧的 `APP_KEY`。

## 保持知情 {#staying-informed}

- [手机上的 Gotify](notifications.md#example-gotify)：失败时响铃，成功时静静到达。
- [把 ntfy 当作 webhook](notifications.md#example-ntfy)：不需要账号的消息。

## 多台服务器 {#several-servers}

- [配对两台 Unraid 机器](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end)：两台服务器互相保存对方的副本，谁也删不掉对方的备份。
- [Home Assistant 里的卡片和提醒](api.md#home-assistant-example)：BombVault 显示在仪表盘上，备份失败时发出消息。
