# 疑难解答

一份简短的常见问题。完整的虚拟机通过 SSH 主机侧疑难解答表（permission-denied、host-key verification、缺失模板变量等等），参见 GitHub 上的[通过 SSH 备份虚拟机指南](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)。

## 有些东西没有正确接线

在 Web 界面打开 `/spike`。主机集成检查会探测每个挂载和 CLI（Docker 套接字、libvirt、restic、qemu-img、rclone）并报告任何缺失的部分。在假定这是一个 bug 之前先从这里开始：缺失的挂载或不可达的主机会立即显现。

## 我无法访问 Web 界面

BombVault 开箱即用地在端口 `3443` 上提供 HTTPS（自签名证书），因此请打开 `https://<your-unraid-ip>:3443`。接受自签名证书警告，或将 BombVault 置于一个带有您自己证书的反向代理之后。如果您以 `HTTP_ONLY=true` 运行，它会改为在端口 `3000` 上提供纯 HTTP（意在用于一个终止 TLS 的代理之后）。

## 我丢失了我的 APP_KEY

`APP_KEY` 派生 restic 仓库密码。没有它（且没有加密密钥恢复工具包），加密备份将无法恢复。这正是仪表板催促您下载恢复工具包的原因。参见[异地与恢复](offsite-recovery.md)。用 `openssl rand -hex 32` 生成一个密钥，并在依赖任何备份之前将它存放在服务器之外。

## 虚拟机备份无法连接

虚拟机备份通过 SSH 与 libvirt 通信，绝不是通过挂载。

- 确认主机上已启用 SSH，且 BombVault 的公钥已在 `/root/.ssh/authorized_keys` 中获得授权（设置，系统，通过 SSH 备份虚拟机会显示该密钥和一个**测试连接**按钮）。
- 在自定义 `br0.x` 网络上，将 `LIBVIRT_HOST` 设置为您的 Unraid LAN IP（在那里容器无法通过 `host.docker.internal` 访问主机）。启用**设置，Docker，主机访问自定义网络**。
- 如果您更改了 Unraid 的 SSH 端口，请相应设置 `LIBVIRT_SSH_PORT`。
- 完整的逐步诊断（可达性测试、VLAN 路由、`Permission denied (publickey)`、`Host key verification failed`）位于[通过 SSH 备份虚拟机指南](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md)。

## 一次实时虚拟机快照没有运行

实时快照需要虚拟机中安装了 qemu guest agent，以及位于 `/mnt/cache`（或 `/mnt/diskX`）而非 `/mnt/user` 上的磁盘。在一台已关机的虚拟机上，实时会自动回退到优雅。优雅备份会将虚拟机关机、备份磁盘、然后重启它，因此它总是一致的。

## 一份备份以 "repository is already locked" 失败

这通常是容器在操作中途被更新或重启时留下的孤立 restic 锁。BombVault 会检测一个可证明为孤立的锁，将其强制清除并自动重试一次。如果它持续存在，请对受影响的域使用**设置，完整性与维护，解锁**来手动清除卡住的锁。真正的问题仍会浮现，而不会被隐藏。

## 我的异地副本在备份后没有发生

异地复制在设计上是尽力而为的，因此异地出现问题绝不会导致本地备份失败。检查该域的异地计划（设置，计划）：空白计划会在每次本地备份后复制，而设置了周期则会更不频繁地发送。使用异地标签页上的**立即复制**进行按需运行，并在仪表板上观察复制指示器。

## 一次还原在开始前就中止了

在停止或移除任何东西之前，还原会运行一次预检冲突检查：它会验证容器的静态 IP 和已发布的主机端口是否空闲。如果另一个容器已占用其中之一，它会以一条清晰、可操作的消息中止，而不是留下一个只完成一半的还原。释放冲突的端口或 IP，然后重试。

## 一次普通导出失败了，没有写出文件

如果 age 加密已开启（设置）但未设置有效收件人，导出会以一条清晰的错误失败，而不是写出明文。添加一个有效收件人（一个 age 公钥或一个 SSH 公钥），或者如果您本就打算让导出为明文，则关闭加密。参见[功能](features.md)。

## 数据库转储失败了

转储失败绝不会让它所在的备份失败；它会作为一次独立的失败运行记录下来，原因会说明要修什么。

- **登录被拒绝。** 转储用容器自己的密码变量登录（`POSTGRES_PASSWORD`、`MARIADB_ROOT_PASSWORD`、`MYSQL_ROOT_PASSWORD` 或它们的 `_FILE` 版本）。请在数据库容器上检查这些变量。若 `_FILE` 变量指向的密钥容器自身的用户读不了，结果也一样。
- **权限不足。** 用随机 root 密码时，转储只能以应用用户身份登录，因此只含那一个数据库；MySQL 8.4 及以上甚至可能直接拒绝。给容器设一个真正的 root 密码，或者关掉它的转储。
- **系统表需要升级。** 当 MariaDB 的系统表来自更旧的版本时，它会拒绝转储（错误 1558）。添加变量 `MARIADB_AUTO_UPGRADE=1` 并重启容器，或在容器内运行一次 `mariadb-upgrade`。
- **没有转储工具。** 精简镜像或自制镜像若没有 `pg_dump`、`mysqldump` 或 `mariadb-dump`，就无法转储。请改用官方镜像，或关掉转储。
- **时间上限。** 一次转储有 `DB_DUMP_MAX_HOURS`（默认 6），它所在的备份有 `BACKUP_MAX_HOURS`，而不再有进展的转储会在 `BACKUP_STALL_HOURS` 之后被切断。最后这种情况多半是应用持有的锁造成的。调高触发的那个上限，或者在应用空闲时再转储。
- **容器处于暂停或正在重启。** 转储要和运行中的服务器对话。如果容器不停重启，原因在它自己的日志里。
- **损坏的转储无法删除。** BombVault 会删除没能完成的转储。若那次删除失败，转储会留在列表里并标记为损坏，你可以在那里删掉它。

## 导入失败了

导入会停止容器，把它的数据目录挪到一边，让镜像在原位创建一个空目录。如果失败发生在导入本身之前的某一步，旧目录会自动放回去。如果是导入本身失败，容器会保留新目录，旧的则留在旁边，名为 `<数据目录>.bombvault-before-import-<时间戳>`；该次运行的错误信息会给出确切路径。

手动放回的做法：停止容器，把当前的数据目录改名挪开，把保留的目录改回原名，然后启动容器。在 Unraid 上，Shares 标签页的文件管理器就能做到。

## AI 助手无法连接

[MCP 服务器](mcp.md#troubleshooting) 页面列出了 MCP 连接地址每个状态码和每种拒绝的含义以及应对方法。

## 容器不断重启或看起来不健康

BombVault 从其自身的 `/api/health` 报告健康/不健康。如果引擎卡死，一个自愈工具（例如 Autoheal）可以自动重启它。检查容器日志和 `/spike` 报告以找出根本原因。

## 仍然卡住？

- 阅读完整的[配置](configuration.md)和[异地与恢复](offsite-recovery.md)页面。
- 在 [Unraid 支持帖](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)上提问。
- 提交一个 [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues)。
