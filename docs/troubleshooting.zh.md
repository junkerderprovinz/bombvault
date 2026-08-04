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

## 容器不断重启或看起来不健康

BombVault 从其自身的 `/api/health` 报告健康/不健康。如果引擎卡死，一个自愈工具（例如 Autoheal）可以自动重启它。检查容器日志和 `/spike` 报告以找出根本原因。

## 仍然卡住？

- 阅读完整的[配置](configuration.md)和[异地与恢复](offsite-recovery.md)页面。
- 在 [Unraid 支持帖](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)上提问。
- 提交一个 [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues)。
