# 虚拟机备份

BombVault 备份 KVM/libvirt 虚拟机时不挂载任何 libvirt 路径。它通过 SSH（`qemu+ssh://`）在主机上运行 `virsh`，通过 Host Data 挂载（主机上的 `/mnt` 在容器里是 `/host/user`）读写虚拟机磁盘，并通过 SSH 复制 UEFI NVRAM。因为属于 libvirt 的东西一概没有被绑定挂载，所以 BombVault 绝不会妨碍 Unraid 的虚拟机管理器。

本指南涵盖设置、备份方式、带 Unraid 快照的虚拟机和 TrueNAS SCALE。容器备份完全不需要这些。

ZFS 数据集备份使用同一条 SSH 链路，用同一把密钥、同一个主机和用户。虚拟机备份连通之后，ZFS 数据集条目也就连通了。参见 [ZFS 数据集](zfs-datasets.md)。

## 你要配置什么 {#what-you-configure}

| 设置 | 位置 | 默认值 | 含义 |
|---|---|---|---|
| **Host SSH: Address**（`LIBVIRT_HOST`） | 模板变量 | `host.docker.internal` | 通过 SSH 连接的 Unraid 主机 |
| **Host SSH: Port**（`LIBVIRT_SSH_PORT`） | 模板变量 | `22` | Unraid 的 SSH 端口 |
| **Host SSH: User**（`LIBVIRT_SSH_USER`） | 模板变量 | `root` | 主机上的 SSH 用户 |
| 公钥 | **设置、集成、主机 SSH** | 首次启动时生成 | 在主机上授权它 |

BombVault 在首次启动时于 `/config/ssh/id_ed25519` 生成它的 SSH 密钥对，保存在 appdata 里。主机的密钥在第一次连接时固定在 `/config/ssh/known_hosts` 里。

模板用占位符 `192.168.x.x` 填写 **Host SSH: Address**。BombVault 把它视为未设置，并尝试 `host.docker.internal`，这在默认的 `bridge` 网络上可行。在自定义网络上，请把它换成主机的局域网 IP（步骤 3 B）。

## 步骤 1：在 Unraid 主机上启用 SSH {#step-1}

1. 在 Unraid 里打开 **Settings, Management Access**，把 **Use SSH** 设为 **Yes**。
2. 如果你更改过 SSH 端口，请记下来。**Host SSH: Port** 必须与它一致。

## 步骤 2：授权 BombVault 的公钥 {#step-2}

Unraid 的 `/root` 在内存里，每次启动都会被清空，所以密钥还必须放在闪存上。在 Unraid 终端里运行：

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 及更新版本在每次启动时会把 `/boot/config/ssh/root.pubkeys` 复制到 root 的 `authorized_keys`，所以这样做重启后依然有效。无论 BombVault 使用哪个主机名或端口，对任何 `root` 登录都有效。

## 步骤 3：网络 {#step-3}

BombVault 必须能从容器向主机的 SSH 端口建立 TCP 连接。请选择与你的设置相符的情况。

### A. BombVault 在默认的 bridge 网络上 {#bridge}

- 把 **Host SSH: Address** 保持为 `host.docker.internal` 或模板的占位符。模板会加上 `--add-host=host.docker.internal:host-gateway`，它指向 docker0 网关，也就是主机。
- 把 **Host SSH: Port** 设为你的 SSH 端口。

### B. BombVault 在自定义网络上 {#custom-network}

位于 `br0.x`、macvlan 或 ipvlan 上的容器无法通过 `host.docker.internal` 访问主机，因为 172.17.0.1 是 docker0，从 `br0.x` 无法访问。请这样做：

1. 在 Unraid 里，把 **Settings, Docker, Host access to custom networks** 设为 **Enabled**。
2. 把 **Host SSH: Address** 设为 Unraid 主机的局域网 IP，也就是你打开网页界面时用的地址。
3. 如果容器的网络和主机在不同的 VLAN 上，请在路由器或防火墙上允许这条路由：从容器 VLAN 到主机 IP 的 SSH 端口，使用 TCP。
4. 如果主机的局域网 IP 仍然无法访问，请使用主机在容器子网上的 shim IP。在主机上用 `ip -4 addr show | grep -B2 <container-subnet>` 找到它。

### 检查主机是否应答 {#verify-reachability}

在 Unraid 终端里运行，用你的 IP 和端口：

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` 表示路径可用。`UNREACHABLE` 表示要先修好步骤 3 里的网络，再做其他事。

## 步骤 4：设置变量并测试 {#step-4}

1. 在 Unraid 里打开 **Docker**，点击 BombVault，选择 **Edit**，设置 **Host SSH: Address**（如果不是 22，还有 **Host SSH: Port**），然后点击 **Apply**。如果缺少这些变量，请重新导入模板，因为 Unraid 会保留已有容器保存的配置。
2. 在 BombVault 里打开**设置、集成、主机 SSH**，点击**测试连接**。它应该变成绿色。

在终端里同样的调用可以列出你的虚拟机：

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## 步骤 5：每台虚拟机的方式 {#per-vm-method}

每台虚拟机在它设置标签页的**虚拟机**卡片里都有一个**方式**（在**备份**下打开这台虚拟机）：

- **优雅（关机）**是默认值。它会关闭虚拟机，备份磁盘，再启动它，所以备份总是一致的。它最多等待客户机关机十分钟，也绝不会强制断电。之后仍在运行的客户机会被放着不动，这次备份会带着原因失败。
- **实时快照**通过外部快照和 `blockcommit` 备份正在运行的虚拟机。它需要虚拟机里有 qemu guest agent（用来做静默快照），并且磁盘在 `/mnt/cache` 或 `/mnt/diskX` 上，而不是 `/mnt/user`。对已关机的虚拟机，实时方式会自动回退到优雅方式，无法创建快照时也会回退。

## 仅变更的块 {#changed-blocks}

使用 qcow2 磁盘的虚拟机，可以只读取发生了变化的部分来备份。在同一张卡片里开启**仅变更的块**，BombVault 就会使用 libvirt 的备份 API，而不是上面的方式：

- 它启动一个拉取模式的备份作业（`virsh backup-begin`），在同一时刻创建一个名为 `bombvault-<time>` 的检查点，并通过转发的 Unix 套接字，经 SSH 链路读取磁盘。当客户机代理应答时，只在作业启动期间冻结客户机的文件系统。解冻失败会重试大约十五秒，如果客户机仍然没有解冻，BombVault 会发送一条通知。
- 第一次运行读取所有已分配的块。之后的运行只读取检查点位图标记为自上次备份以来被写入的块。
- 每个磁盘存储为一个由固定大小的段组成的文件夹，里面是客户机所看到的内容。未变化的段会从上一个快照直接带过来而不被读取，所以每个快照都是完整的，可以单独恢复。
- 恢复时会用这些段重建磁盘，并用 `qemu-img` 把它转换回 qcow2。磁盘的内容与备份时相同，但文件是一个新的 qcow2 文件，后备链会被压平。
- 这样的恢复之前的检查会列出它要重建的每个磁盘，并按客户机的完整大小计算，qcow2 要算两次，因为原始镜像和转换后的副本在旧文件被替换之前并排存在。这是一个磁盘最多可能占用的量，所以按这个计算不够时，空间那一行会变灰，而不是阻止恢复。第一次备份之后的恢复检查以及**检查恢复**，也会确认快照包含每个磁盘的每个段，且大小正确。
- 在两次运行之间，libvirt 里没有这台虚拟机的检查点记录。BombVault 把最新的 `bombvault-` 检查点的定义保存在 `/config/vm-checkpoints` 下，只删除 libvirt 对它的记录（`virsh checkpoint-delete --metadata`）。它的位图仍然保留在 qcow2 文件里并继续记录写入，下一次运行在开始之前会把检查点放回去（`virsh checkpoint-create --redefine`）。libvirt 拒绝取消定义带有检查点的虚拟机，所以这样做才能让 Unraid 的 **Remove VM** 和恢复照常工作。
- BombVault 因为在备份中途停止而留下仍在运行的备份作业，会在 BombVault 再次启动时结束，同时结束那次运行留在记录里的检查点。
- 关闭开关、把虚拟机移出计划或删除它的备份，同样会结束这样的作业，并且在虚拟机运行时也会删除位图。在已关机的虚拟机上，libvirt 无法删除它，它会不被使用地留在镜像里。
- 其他工具的检查点不会被动。带有这类检查点的虚拟机在它们被删除之前无法恢复，恢复会在碰到虚拟机之前就停下。

这个开关会自己回退。检查点缺失或损坏、磁盘大小变了，或最新的快照不是仅变更块的快照，意味着整个磁盘会再读取一次。虚拟机已关机、原始磁盘或 zvol 磁盘、没有主机 SSH 链路、libvirt 没有备份 API，或 `/config` 下可用空间太少，则意味着使用上面那种方式的常规备份。卡片会说明上一次运行用了哪一种以及原因。

要求：带备份 API 的 libvirt（在 Unraid 7.3 上用 libvirt 12.2 和 QEMU 10.2 测试过）、`AllowStreamLocalForwarding yes` 的 sshd（默认值，在 Unraid 上也是，它关闭的是 TCP 转发），以及 qcow2 磁盘。libvirt 拒绝在原始磁盘上建立检查点，而 Unraid 默认创建的是原始 vdisk。备份期间虚拟机保持运行，和实时方式一样，等待 restic 的段最多占用 `/config` 下 4 GiB。

## 带 Unraid 快照的虚拟机 {#unraid-snapshots}

Unraid 虚拟机管理器拍的快照，会把 qcow2 磁盘变成一条链：原始镜像在最底层，每个快照在上面有一个文件，各自只保存相对下面一层发生的变化。虚拟机写入最新的文件。有两个快照的磁盘就是三个文件。

BombVault 会完整备份这样的虚拟机：

- **链上的每个文件。** 它沿着每个磁盘的后备链一直走到末端，并备份其中的每个文件，同时备份 Unraid 的快照列表（`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`，通过 SSH 读取）。无法一路追到末端的链，例如因为中间有一个文件不见了，会让备份失败并点名那个文件。
- **实时备份会保留快照。** **实时快照**在读取磁盘期间在上面加一层临时覆盖，之后再把它合并回去，只合并到紧挨着的下一层镜像里。备份之后的链和之前一样。没有快照的虚拟机和以往的表现相同。
- **恢复会把链带回来。** 原地恢复会把每个文件写回原来的位置，并放回 Unraid 的快照列表，所以虚拟机管理器会再次列出这些快照。放在原件旁边的恢复会把文件写进副本的文件夹，并让恢复的磁盘指向恢复的文件，所以副本绝不会读取原件的镜像。Unraid 的快照列表只会随原地恢复一起回来，因为它记录的是文件备份时所在的位置。没有它，副本的启动没有区别，只是虚拟机管理器不会为它列出快照。
- **仅变更的块**读取的是客户机所看到的内容，所以它的恢复会得到一个压平的、没有快照的 qcow2 文件。

### 示例：有两个快照的虚拟机 {#example-unraid-snapshots}

虚拟机 `homeassistant` 有一个磁盘 `vdisk1.qcow2`，还有你在两次更新之前于虚拟机管理器里拍的两个快照。它的磁盘现在是三个文件的链。它一直在运行，所以使用**实时快照**。

1. 打开**备份**，然后是 `homeassistant`，再到它的**设置**标签页。在**虚拟机**卡片里，确认**方式**是**实时快照**，且**仅变更的块**是关闭的。
2. 点击页面顶部的**立即备份**。
3. 运行完成后，**验证**标签页上**最近运行**里它的那一行是绿色的。虚拟机整个过程一直在运行，备份包含磁盘的全部三个文件和 Unraid 的快照列表。
4. 在虚拟机管理器里，那两个快照仍然在，磁盘仍然通过同样的三个文件读取。

一个月后某次更新出了问题，你想把昨天的状态放在出问题的旁边，用来比较：

1. 打开**恢复**，选择**虚拟机**，点击 `homeassistant`。
2. 保留昨晚推荐的备份，在**目标位置**下选择**放在原件旁边**。
3. 点击**恢复**。三个文件会放进以虚拟机名加 `-restored` 命名的文件夹，恢复的磁盘指向恢复的文件。运行中的虚拟机上什么都不会变。

如果想把整台虚拟机回滚，请改选**替换原件**，并保持**先备份当前状态**开启。BombVault 会备份出问题的状态，关闭虚拟机，写回三个文件和 Unraid 的快照列表，再启动它。

## TrueNAS SCALE {#truenas-scale}

上面的步骤是针对 Unraid 写的，但同样的 `qemu+ssh://` 链路适用于容器能访问到的任何 libvirtd，包括 TrueNAS SCALE，它自 25.04.2 起又用上了 libvirt 虚拟机。在 TrueNAS 上，连接字符串需要覆盖，密钥要放到 TrueNAS 自己用户设置里的 root 上。还有三件事会改变备份的内容：zvol 上的磁盘、vTPM 状态，以及 TrueNAS 给虚拟机起的名字。

### 1. libvirt 套接字 {#truenas-socket}

TrueNAS 的 libvirtd 并不监听由 **Host SSH** 字段拼出的连接字符串所期望的位置。请改为设置 `LIBVIRT_URI`，带上额外的 `?socket=` 参数：

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault 原样使用 `LIBVIRT_URI`，不需要 `LIBVIRT_HOST`、`LIBVIRT_SSH_USER` 和 `LIBVIRT_SSH_PORT`。其中没有设置的，BombVault 也会为它自己的 SSH 命令从 URI 取主机、用户和端口，所以 NVRAM 复制和 ZFS 数据集备份与 `virsh` 到达的是同一台机器。你自己设置的变量总是优先，启动日志会写出它从 URI 取了哪个目标。在 TrueNAS 上，root 之外的用户做 ZFS 数据集备份还需要 `zfs allow <user> snapshot,destroy,mount <dataset>`，参见 [ZFS 数据集](zfs-datasets.md#truenas)。

请把 TrueNAS 的局域网地址写进 URI。`LIBVIRT_URI` 和 `LIBVIRT_HOST` 都没设置时，BombVault 会把应用的默认网关当作主机，因为 `host.docker.internal` 在 TrueNAS 应用里无法解析，启动日志会写出它选了哪个地址。这对 ZFS 数据集备份已经够用。虚拟机备份仍然需要这个 URI 来指明套接字。

虚拟机备份需要应用有 `SETPCAP` 权限，同时还有 `CHOWN`、`DAC_OVERRIDE`、`FOWNER`、`SETGID` 和 `SETUID`。没有它，libvirt 会以"cannot apply process capabilities"停止。[TrueNAS SCALE](getting-started.md#truenas)下的 YAML 把这些全都设置了，还有"用快照代替停止"所需的 `SYS_CHROOT`。

在 TrueNAS SCALE 25.10.0 上，套接字路径在真实硬件上检查过：默认的 `qemu:///system` 会以 `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory` 失败，而带 `?socket=/run/truenas_libvirt/libvirt-sock` 的同一个调用可以列出各个域。

### 2. TrueNAS 上 BombVault 的密钥 {#truenas-key}

上面的步骤 2 写入的是 Unraid 的闪存，这里不适用。在 TrueNAS 上：

1. 在 **System, Services** 下打开 SSH 服务。
2. 在 **Credentials, Users** 下，编辑 **root**，把**设置、集成、主机 SSH** 里 BombVault 的公钥粘贴为它的 SSH 公钥。TrueNAS 在重启和更新后会保留它。

TrueNAS 24.10 及更新版本的全新安装不给 root 设密码，用密钥登录也不需要密码。如果你不想使用 root，请为有 sudo 权限的管理员账户授权这把密钥，并在 `LIBVIRT_URI` 里把 `root` 换成它的名字。ZFS 数据集备份此时需要上面所说的 `zfs allow`。

到第二台服务器的 ZFS 副本使用它自己的密钥，副本对话框里会显示。Host SSH 密钥不能授权副本，副本密钥也不能授权虚拟机备份。

!!! warning "TrueNAS 看不到用 virsh 所做的更改"
    TrueNAS 的中间件，也就是它虚拟机界面背后的那一层，本意是更改它的虚拟机的唯一途径。通过这条 SSH 链路执行的 `virsh` 命令（BombVault 的虚拟机备份和恢复运行的就是这种命令）绕过了它，TrueNAS 之后可能会协调或覆盖它没有创建的状态。这来自 TrueNAS 文档中记载的行为，在真实机器上没有见到过。这和 Unraid 的虚拟机管理器协调在它之外所做更改是同一类风险，只是在 TrueNAS 上更尖锐。

### 3. zvol 上的磁盘 {#truenas-zvols}

TrueNAS 的虚拟机磁盘通常是 zvol，也就是位于 `/dev/zvol/<pool>/<dataset>` 的 ZFS 块设备，而不是 qcow2 文件。restic 无法像备份文件那样按路径备份块设备，所以 BombVault 会在域 XML 里识别 zvol 磁盘（`<source dev="...">`，而不是 `<source file="...">`），并通过同一条 SSH 链路处理它。BombVault 容器不需要任何 ZFS 工具。

zvol 的备份是这样的：

1. `zfs snapshot <dataset>@bombvault-vm-<time>` 固定一个时间点，虚拟机运行时也可以。
2. `zfs clone -o volmode=dev -o readonly=on` 让这个快照可以作为设备读取。`volmode=dev` 让主机不去扫描客户机的分区。
3. `dd` 读取设备，原始镜像作为一个文件 `/vm-disks/<dataset>.img` 放进 restic，标签为 `vm:<name>:zvol:<dev>`。restic 按内容把它切成块，所以自上次备份以来没有变化的块不会再次存储。
4. 无论备份是否成功，克隆和快照之后都会被销毁。如果 BombVault 在备份中途停止，这个 zvol 的下一次备份会先移除 `bombvault-vm-<time>` 快照及其 `<dataset>-bombvault-vm-<time>` 克隆。其他所有快照都不会动，别的东西由此克隆出来的也会保留。日志会写出它移除或保留了什么。

BombVault 不为此使用 `zfs send`。发送流的每条记录都带着快照的 GUID 和一个滚动校验和，所以 restic 找不到上次备份中可重用的部分，每次都会把整个磁盘再存一遍。在 TrueNAS SCALE 25.10 上用一个装有 200 MiB 数据的 512 MiB zvol 测得：第一次备份增加了 211 MB，什么都没变的第二次增加了 52 KB，改动了 1 MiB 之后的一次增加了 3.6 MB。同一个 zvol 用 `zfs send`，每次运行都增加 214 MB。

每次运行在所有磁盘都放入之后，还会写一个带 `vm:<name>` 标签的小快照，里面是域 XML。所有磁盘都是 zvol 的虚拟机没有文件快照，正是这个快照把这次运行列在虚拟机之下并使其可以恢复。

恢复会把每个 zvol 原地写回，就像文件磁盘被写回它的文件一样，所以恢复的虚拟机启动的是恢复的数据：

1. 在碰任何东西之前，BombVault 会检查这次运行里有每个 zvol 磁盘的镜像。缺了一个的运行会被拒绝，虚拟机保持原样。
2. 已有的 zvol 会先拍一个快照 `<dataset>@bombvault-before-restore-<time>`，时间用 UTC，和 BombVault 创建的每个快照名一样，所以可能与本地时钟相差几小时。日志会写出它，在它还是该 zvol 最新的快照期间，对它执行 `zfs rollback` 就能撤销这次恢复。一旦有了更新的快照，例如下一次恢复的，`zfs rollback -r` 也能做到，但会销毁更新的快照。它会一直保留，直到恢复的虚拟机能正常工作后，你用 `zfs destroy` 把它移除。
3. 已经不存在的 zvol 会按备份时的大小重新创建，大小不同的现有 zvol 会被设回那个大小。
4. `dd` 把镜像写到 `/dev/zvol/<dataset>`。开启压缩时（TrueNAS 的默认值），镜像的空白部分会恢复成空洞，不占空间。

**恢复到另一个实例需要一个目标存储池。** 它会把文件磁盘搬进目标文件夹，但文件夹没有指明 ZFS 存储池，所以没有存储池时，BombVault 会在开始之前拒绝这次恢复。把存储池作为 `POST /api/foreign/restore` 的 `zvolPool` 参数传入：每个 zvol 保留它在存储池之下的路径（`tank/vms/win10/disk0` 到 `flashpool` 变成 `flashpool/vms/win10/disk0`），域 XML 会指向它。恢复页面上的**从旧的备份文件夹**窗口没有这个字段，所以只能通过直接调用 API 来使用。同一个实例上的恢复会把每个 zvol 写回它自己。

**导出为文件会略去 zvol。** 所有磁盘都是 zvol 的虚拟机会因此被拒绝，请改为从备份恢复。对于同时有文件磁盘的虚拟机，导出包含文件磁盘，以及一条点名所略去的 zvol 的说明。

这在 TrueNAS SCALE 25.10 和 ZFS 2.3.4 上试过：一个临时 zvol 的三次备份、在 zvol 被覆盖并扩容之后的原地恢复，以及恢复到一个已被删除的 zvol。每次恢复读回来的 SHA-256 都和源相同，zvol 又变回原来的大小，没有留下快照或克隆。唯一磁盘是 zvol 的整台虚拟机，在运行时用优雅方式和实时方式、以及关机时都备份过，整台虚拟机的恢复（取消定义、写入 zvol、定义、启动）也在那里运行过：zvol 读回来的校验和与源相同，虚拟机也启动了。

Unraid 使用的文件磁盘不受以上任何内容影响。

### 4. vTPM 状态 {#truenas-vtpm}

虚拟机可以有虚拟 TPM，Secure Boot 和 Windows 11 客户机需要它。它的状态在磁盘和 NVRAM 之外，所以只备份这两样，会让客户机带着一个新的空 TPM 回来。

- **直通 TPM**，也就是传给客户机的真实芯片，会在域 XML 里写出它的路径。BombVault 通过 SSH 把它的状态与 NVRAM 一起保存，并在 `virsh define` 之前写回去，和 NVRAM 一样是尽力而为：读取或写入失败会被记录，不会让备份或恢复失败。
- **模拟的 vTPM**，也就是 TrueNAS 为 Windows 11 和 Secure Boot 客户机设置的那种，不在域 XML 里发布状态路径。这时 BombVault 会报告"no TPM path found"，而不是猜一个路径。恢复这样的客户机之前，请准备好它的恢复密钥，例如 BitLocker 密钥。
- **你无法自己给 TrueNAS 虚拟机附加 TPM。** TrueNAS SCALE 25.10 的虚拟机设备 API 只接受 `CDROM`、`DISPLAY`、`NIC`、`PCI`、`RAW`、`DISK` 和 `USB`，并把 `TPM` 当作未知类型拒绝。

TPM 的处理只有单元测试，没有在带 vTPM 的真实 TrueNAS 客户机上运行过。没有 TPM 的虚拟机（例如在 Unraid 上）的备份和恢复与以前完全一样。

NVRAM 照常抓取。TrueNAS 把它按虚拟机存放在 `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`，域 XML 里写着这个路径。

### 5. TrueNAS 上的虚拟机名称 {#truenas-names}

在 Unraid 上，虚拟机的 libvirt 名称就是你选的名称。TrueNAS 自己编出 libvirt 名称，并且已经改过两次方式：

| TrueNAS 版本 | libvirt 名称 | 你的名称在哪里 |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`，例如 `1_debian` | 在 libvirt 名称里，`{id}_` 之后 |
| 26 | 虚拟机的 UUID，例如 `550e8400-e29b-41d4-a716-446655440000` | 只在域 XML 的 `<title>` 里 |

在被 BombVault 检测为 TrueNAS，或被 `PLATFORM=truenas` 指定的主机上，备份列表会显示你的名称：在 25.10 上是 `debian` 而不是 `1_debian`，在 26 上是 `<title>` 而不是 UUID，没有标题时则是 UUID。这只是列表显示的内容。每个 `virsh` 调用和每台虚拟机的设置仍然使用 libvirt 名称，所以备份和恢复总是作用于正确的虚拟机。在 TrueNAS 26 上，这意味着被 TrueNAS 用新的 UUID 重新创建的虚拟机，即使名称看起来一样，也会作为没有设置的新条目出现。在 Unraid 上什么都不变，即使你把虚拟机命名为 `10_Windows` 也一样。

25.10 的方案是在一台真实的 TrueNAS SCALE 25.10 机器上看到的。TrueNAS 26 的方案来自 TrueNAS 的文档，只有单元测试。

## 故障排除 {#troubleshooting}

| 症状 | 原因与解决办法 |
|---|---|
| **测试连接**卡住，然后失败 | 访问不到主机。检查步骤 3（网络、VLAN、防火墙）以及 **Host SSH: Address** 和 **Port**。 |
| 密钥明明在 `authorized_keys` 里，却出现 `Permission denied (publickey)` | sshd 的 `StrictModes` 因为某个上级文件夹的所有者或权限不对而拒绝了密钥文件。当 `/root/.ssh` 是指向 FAT 闪存的链接（`/boot/config/ssh/...`）时很常见，那里对所有人可写。主机日志会显示 `Authentication refused: bad ownership or modes for directory ...`。在主机 sshd 配置的全局部分、任何 `Match` 块之前添加 `StrictModes no`，然后运行 `/etc/rc.d/rc.sshd restart`。Unraid 每次启动都会重新写 `/etc/ssh/sshd_config`，所以请按下面所述让它持久化。 |
| `Permission denied (publickey)`，而且密钥确实缺失 | 重复步骤 2，并检查 SSH 是否已开启。 |
| `Host key verification failed` | 运行 `docker exec BombVault rm -f /config/ssh/known_hosts`，然后再测试。 |
| 步骤 3 里的检查显示 `UNREACHABLE` | 容器访问不到主机的 SSH 端口。参见步骤 3：访问自定义网络的主机权限、VLAN 路由，或 shim IP 与局域网 IP。 |
| **Edit** 下缺少这些变量 | 重新导入模板。已有的容器会保留它旧的已保存配置。 |
| 带 Unraid 快照的虚拟机失败，并点名一个文件 | 磁盘的后备链无法一路追到末端。请在虚拟机管理器里检查该磁盘的每个快照是否仍然存在，以及点名的文件是否存在于 `/mnt` 之下。 |

更多虚拟机相关的症状见[故障排除](troubleshooting.md#vm-backup-will-not-connect)。

## 重启后保持有效 {#persistence}

Unraid 每次启动都会重新写 `/etc/ssh/sshd_config`，所以主机端的更改必须在启动时重新应用：

- **SSH 端口**保存在 `/boot/config/ident.cfg` 中，通过 Unraid 网页界面设置。
- **`StrictModes no`**，如果你上面需要它，不会被保留。请在启动时通过 `/boot/config/go` 应用：

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- BombVault 自己的 SSH 密钥和 `~/.ssh/config` 放在 `/config`（appdata）里，每次启动都会重新生成，所以容器和主机重启后它们仍然保留。

## 安全 {#security}

授权的密钥授予的是主机上的 root，和 BombVault 已在使用的 Docker 套接字是同一级别的信任。请让 BombVault 留在受信任的网络上。密钥是 ed25519，以 `0600` 权限保存在 appdata 里，主机的密钥是固定的。每个主机命令都作为单独的参数传递，绝不经过远程 shell，SSH 连接使用 `BatchMode` 和连接超时。
