# VM のバックアップ

BombVault は、libvirt のパスを一切マウントせずに KVM/libvirt VM をバックアップします。SSH（`qemu+ssh://`）経由でホスト上の `virsh` を実行し、ホストデータのマウント（ホストの `/mnt` がコンテナの `/host/user`）を通じて VM のディスクを読み書きし、UEFI NVRAM は SSH でコピーします。libvirt に属するものは何もバインドマウントしないので、BombVault が Unraid の VM Manager の邪魔になることは決してありません。

このガイドでは、セットアップ、バックアップの方式、Unraid スナップショットのある VM、TrueNAS SCALE を扱います。コンテナのバックアップには、これらは何も必要ありません。

ZFS データセットのバックアップも、同じ鍵、ホスト、ユーザーを使う同じ SSH 接続を使います。VM バックアップが接続できれば、ZFS データセットの項目も接続できます。[ZFS データセット](zfs-datasets.md)を参照してください。

## 設定する項目 {#what-you-configure}

| 設定 | 場所 | 標準 | 意味 |
|---|---|---|---|
| **Host SSH: Address**（`LIBVIRT_HOST`） | テンプレート変数 | `host.docker.internal` | SSH で接続する Unraid ホスト |
| **Host SSH: Port**（`LIBVIRT_SSH_PORT`） | テンプレート変数 | `22` | Unraid の SSH ポート |
| **Host SSH: User**（`LIBVIRT_SSH_USER`） | テンプレート変数 | `root` | ホスト上の SSH ユーザー |
| 公開鍵 | **設定、連携、ホスト SSH** | 初回起動時に作成 | ホストで承認します |

BombVault は、初回起動時に `/config/ssh/id_ed25519` に SSH 鍵ペアを作り、appdata に保存します。ホストの鍵は、初回の接続時に `/config/ssh/known_hosts` に固定されます。

テンプレートは **Host SSH: Address** にプレースホルダー `192.168.x.x` を入れます。BombVault はそれを未設定として扱い、`host.docker.internal` を試します。これは標準の `bridge` ネットワークで動きます。カスタムネットワークでは、ホストの LAN IP に置き換えてください（手順 3 の B）。

## 手順 1：Unraid ホストで SSH を有効にする {#step-1}

1. Unraid で **Settings, Management Access** を開き、**Use SSH** を **Yes** にします。
2. SSH ポートを変更した場合は、控えておいてください。**Host SSH: Port** と一致している必要があります。

## 手順 2：BombVault の公開鍵を承認する {#step-2}

Unraid の `/root` はメモリ上にあり、起動のたびに消えるので、鍵をフラッシュドライブにも置く必要があります。Unraid のターミナルで次を実行します。

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 以降は、起動のたびに `/boot/config/ssh/root.pubkeys` を root の `authorized_keys` にコピーするので、これは再起動後も残ります。BombVault が使うホスト名やポートに関係なく、あらゆる `root` のログインで有効です。

## 手順 3：ネットワーク {#step-3}

BombVault は、コンテナからホストの SSH ポートへ TCP 接続を開けなければなりません。環境に合うケースを選んでください。

### A. 標準のブリッジネットワーク上の BombVault {#bridge}

- **Host SSH: Address** は `host.docker.internal` かテンプレートのプレースホルダーのままにします。テンプレートは `--add-host=host.docker.internal:host-gateway` を追加し、これが docker0 のゲートウェイ、つまりホストを指します。
- **Host SSH: Port** を、自分の SSH ポートにします。

### B. カスタムネットワーク上の BombVault {#custom-network}

`br0.x`、macvlan、ipvlan 上のコンテナは、`host.docker.internal` ではホストに届きません。172.17.0.1 は docker0 で、`br0.x` からは到達できないからです。代わりに次のようにします。

1. Unraid で **Settings, Docker, Host access to custom networks** を **Enabled** にします。
2. **Host SSH: Address** を、Unraid ホストの LAN IP、つまり Web UI を開くアドレスにします。
3. コンテナのネットワークとホストが別の VLAN にある場合は、ルーターまたはファイアウォールで経路を許可します。コンテナの VLAN からホストの IP の SSH ポートへ、TCP です。
4. ホストの LAN IP にまだ到達できない場合は、コンテナのサブネット上のホストのシム IP を使います。ホストで `ip -4 addr show | grep -B2 <container-subnet>` で探せます。

### ホストが応答するか確認する {#verify-reachability}

Unraid のターミナルで、自分の IP とポートを使って次を実行します。

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` は経路が通っていることを意味します。`UNREACHABLE` は、ほかの何よりも先に、手順 3 のネットワークを直す必要があることを意味します。

## 手順 4：変数を設定してテストする {#step-4}

1. Unraid で **Docker** を開き、BombVault をクリックして **Edit** を選び、**Host SSH: Address**（と、22 でなければ **Host SSH: Port**）を設定して **Apply** を押します。変数がない場合は、テンプレートをもう一度インポートしてください。Unraid は既存のコンテナの保存済みの設定を保持するからです。
2. BombVault で **設定、連携、ホスト SSH** を開き、**接続をテスト**を押します。緑になるはずです。

ターミナルから同じ呼び出しをすると、VM が一覧されます。

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## 手順 5：VM ごとの方式 {#per-vm-method}

各 VM には、設定タブの **VM** カードに**方式**があります（**バックアップ**で VM を開きます）。

- **グレースフル（シャットダウン）**が標準です。VM をシャットダウンし、ディスクをバックアップして、再び起動するので、バックアップは常に整合しています。ゲストがシャットダウンするのを最大 10 分待ち、強制的に電源を切ることはありません。それでも動いているゲストはそのままにされ、そのバックアップは理由とともに失敗します。
- **ライブスナップショット**は、外部スナップショットと `blockcommit` を使って、動作中の VM をバックアップします。静止したスナップショットのために VM に qemu ゲストエージェントが必要で、ディスクは `/mnt/user` ではなく `/mnt/cache` か `/mnt/diskX` にある必要があります。シャットダウン済みの VM では、ライブは自動でグレースフルに切り替わり、スナップショットを作れないときも同様に切り替わります。

## 変更ブロックのみ {#changed-blocks}

qcow2 ディスクを持つ VM は、変わったものだけを読み取ってバックアップできます。同じカードで**変更ブロックのみ**をオンにすると、BombVault は上の方式の代わりに libvirt のバックアップ API を使います。

- プルモードのバックアップジョブ（`virsh backup-begin`）を開始し、それと同時に `bombvault-<time>` という名前のチェックポイントを作って、転送された Unix ソケットを通じて SSH 接続でディスクを読み取ります。ゲストエージェントが応答すれば、ゲストのファイルシステムは、ジョブが始まるあいだだけ凍結されます。失敗した解凍は約 15 秒再試行され、それでもゲストが解凍されなければ、BombVault は通知を送ります。
- 最初の実行は、割り当て済みのすべてのブロックを読み取ります。以降の実行は、前回のバックアップ以降に書き込まれたとチェックポイントのビットマップが示すブロックだけを読み取ります。
- 各ディスクは、ゲストから見える内容を保持する固定サイズのセグメントのフォルダーとして保存されます。変わっていないセグメントは、読み取らずに前のスナップショットから引き継がれるので、すべてのスナップショットが完全で、単独で復元できます。
- 復元では、セグメントからディスクを再構築し、`qemu-img` で qcow2 に変換し直します。ディスクの内容はバックアップ時と同じですが、ファイルは新しい qcow2 ファイルで、バッキングチェーンは平坦化されて戻ります。
- そのような復元の前のチェックは、再構築する各ディスクを一覧し、ゲストの完全なサイズで数えます。qcow2 では 2 倍です。古いファイルが置き換えられるまで、raw イメージと変換後のコピーが並んでいるからです。これがディスクにかかる最大の量なので、この数え方で不足していても、空き容量の行は、ブロックするのではなくグレーのままになります。最初のバックアップのあとの復元チェックと**復元を確認**も、スナップショットがすべてのディスクのすべてのセグメントを、あるべきサイズで保持していることを確かめます。
- 実行と実行のあいだ、VM には libvirt に記録されたチェックポイントがありません。BombVault は、最新の `bombvault-` チェックポイントの定義を `/config/vm-checkpoints` に保持し、libvirt の記録だけを削除します（`virsh checkpoint-delete --metadata`）。そのビットマップは qcow2 ファイルに残って書き込みを数え続け、次の実行は開始前にチェックポイントを戻します（`virsh checkpoint-create --redefine`）。libvirt はチェックポイントのある VM の定義解除を拒否するので、これにより Unraid の **Remove VM** や復元がいつもどおり動きます。
- バックアップの途中で BombVault が停止したために実行中のまま残ったバックアップジョブは、BombVault が再び起動したときに、その実行が記録に残したチェックポイントとともに終了されます。
- スイッチをオフにする、VM をスケジュールから外す、バックアップを削除するといった場合も、そのようなジョブが終了し、VM が動いているあいだはビットマップも削除されます。VM がオフの場合、libvirt はそれを削除できず、使われないままイメージに残ります。
- 他のツールのチェックポイントには手を付けません。それがある VM は、削除するまで復元できず、復元は VM に触れる前に止まります。

このスイッチは自動でフォールバックします。チェックポイントがない、または壊れている、ディスクのサイズが変わった、最新のスナップショットが変更ブロックのものではない、といった場合は、ディスク全体がもう一度読み取られます。VM がオフ、raw ディスクや zvol ディスク、ホストへの SSH 接続がない、バックアップ API のない libvirt、`/config` の空きが少なすぎる、といった場合は、上の方式による通常のバックアップになります。カードに、直近の実行がどれを使ったか、その理由が表示されます。

要件：バックアップ API のある libvirt（Unraid 7.3 の libvirt 12.2 と QEMU 10.2 でテスト済み）、`AllowStreamLocalForwarding yes` の sshd（標準で、Unraid でも同様です。Unraid は TCP フォワーディングをオフにします）、そして qcow2 ディスク。libvirt は raw ディスクのチェックポイントを拒否し、Unraid は標準で raw の vdisk を作ります。VM は、ライブと同じようにバックアップ中も動き続け、restic を待つセグメントは `/config` の下で最大 4 GiB を使います。

## Unraid スナップショットのある VM {#unraid-snapshots}

Unraid の VM Manager が取るスナップショットは、qcow2 ディスクをチェーンに変えます。一番下に元のイメージがあり、その上にスナップショットごとに 1 つのファイルが積まれ、それぞれが、その下のものからの変更だけを保持します。VM は最新のファイルに書き込みます。スナップショットが 2 つあるディスクは、3 つのファイルです。

BombVault は、そのような VM を完全にバックアップします。

- **チェーンのすべてのファイル。** 各ディスクのバッキングチェーンを最後までたどり、その中のすべてのファイルを、Unraid のスナップショットの一覧（`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`、SSH で読み取り）とともにバックアップします。最後までたどれないチェーン（たとえば途中のファイルが消えている）は、バックアップを失敗させ、そのファイルを示します。
- **ライブバックアップはスナップショットを保持します。** **ライブスナップショット**は、ディスクを読み取るあいだ一時的なオーバーレイを上に置き、そのあと、すぐ下のイメージにだけ統合し戻します。バックアップの後も、チェーンはその前と同じです。スナップショットのない VM は、これまでどおりに動きます。
- **復元でチェーンが戻ります。** その場での復元は、すべてのファイルを元の場所に書き戻し、Unraid のスナップショットの一覧も戻すので、VM Manager にスナップショットが再び一覧されます。元の隣への復元は、コピーのフォルダーにファイルを書き込み、復元したディスクを復元したファイルに向けるので、コピーが元のイメージから読み取ることはありません。Unraid のスナップショットの一覧は、バックアップした場所のファイルを指すため、その場での復元でしか戻りません。それがなくても、コピーは同じように起動し、VM Manager にスナップショットが一覧されないだけです。
- **変更ブロックのみ**は、ゲストから見える内容を読み取るため、その復元は、スナップショットのない、平坦化された 1 つの qcow2 ファイルとして戻ります。

### 例：スナップショットが 2 つある VM {#example-unraid-snapshots}

VM `homeassistant` には、ディスク `vdisk1.qcow2` が 1 つあり、2 回のアップデートの前に VM Manager で取った 2 つのスナップショットがあります。そのディスクは今、3 つのファイルのチェーンです。常時稼働しているので、**ライブスナップショット**を使います。

1. **バックアップ**を開き、`homeassistant`、その**設定**タブの順に進みます。**VM** カードで、**方式**が**ライブスナップショット**で、**変更ブロックのみ**がオフになっていることを確認します。
2. ページ上部の**今すぐバックアップ**を押します。
3. 実行が終わると、**検証**タブの**最近の実行**のその行が緑になります。VM はずっと動き続け、バックアップにはディスクの 3 つのファイルすべてと、Unraid のスナップショットの一覧が入っています。
4. VM Manager では、2 つのスナップショットがそのまま残っていて、ディスクも同じ 3 つのファイルを通して読み取ります。

1 か月後、アップデートがうまくいかず、壊れた状態の隣に昨日の状態を戻して比較したいとします。

1. **復元**を開き、**VM** を選んで `homeassistant` をクリックします。
2. 昨夜のおすすめのバックアップのままにして、**復元先**で**元の隣に**を選びます。
3. **復元**を押します。3 つのファイルは、VM の名前に `-restored` を付けたフォルダーに置かれ、復元したディスクは復元したファイルを指します。動いている VM には何も起きません。

VM 全体を巻き戻したい場合は、代わりに、**先に現在の状態をバックアップ**をオンにしたまま**元を置き換える**を選びます。BombVault は壊れた状態をバックアップし、VM をシャットダウンして、3 つのファイルと Unraid のスナップショットの一覧を書き戻し、再び起動します。

## TrueNAS SCALE {#truenas-scale}

上の手順は Unraid 向けに書かれていますが、同じ `qemu+ssh://` 接続は、コンテナが届く任意の libvirtd に対して動きます。25.04.2 から VM が再び libvirt に載った TrueNAS SCALE も含まれます。TrueNAS では、接続文字列に上書きが必要で、鍵は TrueNAS 自身のユーザー設定で root に置きます。さらに 3 つのことが、バックアップに入るものを変えます。zvol 上のディスク、vTPM の状態、TrueNAS が VM に付ける名前です。

### 1. libvirt のソケット {#truenas-socket}

TrueNAS の libvirtd は、**Host SSH** のフィールドから組み立てた接続文字列が想定する場所では待ち受けていません。代わりに、追加の `?socket=` パラメータをつけて `LIBVIRT_URI` を設定してください。

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault は `LIBVIRT_URI` を書かれたとおりに使い、`LIBVIRT_HOST`、`LIBVIRT_SSH_USER`、`LIBVIRT_SSH_PORT` は不要です。未設定のものについては、BombVault は自身の SSH コマンドのホスト、ユーザー、ポートも URI から取るので、NVRAM のコピーと ZFS データセットのバックアップは、`virsh` と同じマシンに届きます。自分で設定した変数が常に優先され、起動ログには URI から取ったターゲットが書かれます。TrueNAS で root 以外のユーザーを使う場合は、ZFS データセットのバックアップに `zfs allow <user> snapshot,destroy,mount <dataset>` も必要です。[ZFS データセット](zfs-datasets.md#truenas)を参照してください。

URI には TrueNAS の LAN アドレスを入れてください。`LIBVIRT_URI` も `LIBVIRT_HOST` も設定していない場合、TrueNAS のアプリでは `host.docker.internal` が名前解決できないため、BombVault はアプリのデフォルトゲートウェイをホストとして使い、起動ログには選んだアドレスが書かれます。ZFS データセットのバックアップにはそれで十分です。VM のバックアップには、ソケットのために URI がやはり必要です。

VM のバックアップには、アプリに `SETPCAP` ケーパビリティが必要で、`CHOWN`、`DAC_OVERRIDE`、`FOWNER`、`SETGID`、`SETUID` と並びます。それがないと、libvirt は "cannot apply process capabilities" で停止します。[TrueNAS SCALE](getting-started.md#truenas) の YAML は、それらをすべて設定し、停止の代わりのスナップショットに必要な `SYS_CHROOT` も設定します。

TrueNAS SCALE 25.10.0 では、実機でソケットのパスを確認しました。標準の `qemu:///system` は `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory` で失敗し、同じ呼び出しに `?socket=/run/truenas_libvirt/libvirt-sock` を付けるとドメインが一覧されます。

### 2. TrueNAS での BombVault の鍵 {#truenas-key}

上の手順 2 は Unraid のフラッシュドライブに書き込むもので、ここには当てはまりません。TrueNAS では次のようにします。

1. **System, Services** で SSH サービスをオンにします。
2. **Credentials, Users** で **root** を編集し、**設定、連携、ホスト SSH** にある BombVault の公開鍵を、SSH 公開鍵として貼り付けます。TrueNAS は、再起動や更新をまたいでそれを保持します。

TrueNAS 24.10 以降の新規インストールでは、root にパスワードがなく、鍵でのログインにパスワードは不要です。root を使いたくない場合は、sudo のできる管理者アカウントに鍵を承認して、`LIBVIRT_URI` の `root` をその名前に置き換えてください。その場合、ZFS データセットのバックアップには、上で説明したように `zfs allow` が必要です。

2 台目のサーバーへの ZFS レプリカは、レプリカのダイアログに表示される専用の鍵でログインします。ホスト SSH の鍵ではレプリカは承認されず、レプリカの鍵では VM バックアップは承認されません。

!!! warning "TrueNAS は virsh による変更を認識しません"
    TrueNAS のミドルウェア、つまり VM 画面の裏側の層は、TrueNAS の VM を変更する唯一の手段であることを前提にしています。この SSH 接続での `virsh` コマンド（BombVault の VM バックアップと復元が実行するもの）はそれを迂回するため、TrueNAS があとで、自分が作っていない状態を調整したり上書きしたりするかもしれません。これは TrueNAS の文書化された動作に基づくもので、実機では確認されていません。Unraid の VM Manager が、外部で行われた変更を調整するのと同じ種類のリスクですが、TrueNAS ではより鋭くなります。

### 3. zvol 上のディスク {#truenas-zvols}

TrueNAS の VM ディスクは、qcow2 ファイルではなく、`/dev/zvol/<pool>/<dataset>` にある ZFS のブロックデバイスである zvol であることがよくあります。restic は、ファイルのようにパスでブロックデバイスをバックアップすることはできないので、BombVault はドメイン XML の中で zvol ディスク（`<source file="...">` ではなく `<source dev="...">`）を認識し、同じ SSH 接続を通じて処理します。BombVault のコンテナに ZFS ツールは不要です。

zvol のバックアップは次のとおりです。

1. `zfs snapshot <dataset>@bombvault-vm-<time>` が、VM が動いているときでも、ある時点を固定します。
2. `zfs clone -o volmode=dev -o readonly=on` が、そのスナップショットをデバイスとして読み取れるようにします。`volmode=dev` により、ホストがゲストのパーティションをスキャンすることはありません。
3. `dd` がデバイスを読み取り、raw イメージが 1 つのファイル `/vm-disks/<dataset>.img` として、タグ `vm:<name>:zvol:<dev>` を付けて restic に入ります。restic は内容に基づいてそれをチャンクに切るので、前回のバックアップから変わっていないブロックは再び保存されません。
4. クローンとスナップショットは、バックアップが成功したかどうかにかかわらず、あとで破棄されます。バックアップの途中で BombVault が止まった場合は、その zvol の次のバックアップが、先に `bombvault-vm-<time>` スナップショットとその `<dataset>-bombvault-vm-<time>` クローンを削除します。ほかのスナップショットには手を付けず、ほかのものがクローン元にしているものは残します。ログには、何を削除し、何を残したかが書かれます。

BombVault はこれに `zfs send` を使いません。send ストリームのすべてのレコードにスナップショットの GUID と連続するチェックサムが含まれるため、restic は前回のバックアップから再利用できるものを何も見つけられず、毎回ディスク全体を保存し直してしまいます。TrueNAS SCALE 25.10 で、200 MiB のデータが入った 512 MiB の zvol で測定した結果は次のとおりです。最初のバックアップで追加されたのは 211 MB、何も変わっていない 2 回目は 52 KB、1 MiB を変更したあとのものは 3.6 MB でした。同じ zvol を `zfs send` で処理すると、毎回 214 MB が追加されました。

各実行は、すべてのディスクが入ったあとに、ドメイン XML を含む `vm:<name>` タグのついた小さなスナップショットも書き込みます。ディスクがすべて zvol の VM にはファイルのスナップショットがなく、これが、その実行を VM の下に一覧し、復元できるようにするものです。

復元では、ファイルのディスクをそのファイルに書き戻すのと同じように、各 zvol をその場に書き戻すので、復元した VM は復元したデータで起動します。

1. 何かに触れる前に、BombVault は、その実行がすべての zvol ディスクのイメージを保持しているかを確認します。欠けているものがある実行は拒否され、VM はそのままにされます。
2. 既存の zvol には、先にスナップショット `<dataset>@bombvault-before-restore-<time>` が取られます。時刻は、BombVault が作るスナップショット名と同じく UTC なので、ローカルの時計と数時間ずれることがあります。ログにその名前が書かれ、それが zvol の最新のスナップショットであるあいだは、`zfs rollback` で復元を取り消せます。次の復元のものなど、より新しいスナップショットがあるときは、`zfs rollback -r` で行え、新しいスナップショットは破棄されます。復元した VM が動いたら、`zfs destroy` で削除するまで残ります。
3. もう存在しない zvol は、バックアップ時のサイズで作成され、別のサイズの既存の zvol は、そのサイズに戻されます。
4. `dd` がイメージを `/dev/zvol/<dataset>` に書き込みます。圧縮がオン（TrueNAS の標準）なら、イメージの空の部分はホールとして戻り、容量を使いません。

**別のインスタンスへの復元には、転送先のプールが必要です。** ファイルのディスクは転送先のフォルダーに移されますが、フォルダーは ZFS プールを指さないので、指定がないと BombVault は復元を開始する前に拒否します。プールは、`POST /api/foreign/restore` の `zvolPool` パラメーターで渡します。各 zvol はプールの下のパスを保ち（`tank/vms/win10/disk0` を `flashpool` に移すと `flashpool/vms/win10/disk0` になります）、ドメイン XML がそれを指します。復元ページの**古いバックアップフォルダーから**のウィンドウには、このためのフィールドがないので、直接の API 呼び出しでしか使えません。同じインスタンスへの復元は、各 zvol を自分自身に書き戻します。

**ファイルとしてのエクスポートは zvol を含みません。** ディスクがすべて zvol の VM は、その理由で拒否されるので、代わりにバックアップから復元してください。ファイルのディスクも持つ VM では、エクスポートにはファイルのディスクと、除外した zvol を示すメモが入ります。

これは ZFS 2.3.4 の TrueNAS SCALE 25.10 で試しました。スクラッチ用の zvol の 3 回のバックアップ、zvol が上書きされて拡張されたあとのその場での復元、削除された zvol への復元です。どの復元も、元と同じ SHA-256 で読み戻され、zvol は元のサイズに戻り、スナップショットもクローンも残りませんでした。唯一のディスクが zvol である VM 全体を、動作中のグレースフルとライブ、そしてシャットダウン中の状態でバックアップし、VM 全体の復元（定義解除、zvol の書き込み、定義、起動）も実行しました。zvol は元のチェックサムで読み戻され、VM は起動しました。

Unraid が使うようなファイルのディスクには、これらのどれも影響しません。

### 4. vTPM の状態 {#truenas-vtpm}

VM には、Secure Boot や Windows 11 のゲストが必要とする仮想 TPM を持たせられます。その状態はディスクと NVRAM の外にあるので、その 2 つだけをバックアップすると、ゲストは新しい空の TPM で戻ってきます。

- **パススルーの TPM**（実際のチップをゲストに渡すもの）は、ドメイン XML にそのパスを書きます。BombVault は、その状態を NVRAM の隣に SSH で保存し、`virsh define` の前に書き戻します。NVRAM と同じようにベストエフォートで、読み取りや書き込みの失敗はログに記録されるだけで、バックアップや復元を失敗させません。
- **エミュレートされた vTPM**（TrueNAS が Windows 11 と Secure Boot のゲスト向けに設定するもの）は、ドメイン XML に状態のパスを公開しません。BombVault は、パスを推測する代わりに "no TPM path found" と報告します。そのようなゲストを復元する前に、BitLocker キーなどの回復キーを手元に用意しておいてください。
- **TrueNAS の VM に自分で TPM を接続することはできません。** TrueNAS SCALE 25.10 の VM デバイス API が受け付けるのは `CDROM`、`DISPLAY`、`NIC`、`PCI`、`RAW`、`DISK`、`USB` だけで、`TPM` は未知の種類として拒否します。

TPM の処理は単体テストのみで、vTPM を持つ実際の TrueNAS のゲストでは動かしていません。Unraid のように TPM のない VM は、これまでとまったく同じようにバックアップと復元ができます。

NVRAM はいつもどおり取得されます。TrueNAS は VM ごとに `/var/db/system/vm/nvram/<id>_<name>_VARS.fd` に保持していて、ドメイン XML にそのパスが書かれています。

### 5. TrueNAS での VM の名前 {#truenas-names}

Unraid では、VM の libvirt 名は自分で付けた名前です。TrueNAS は libvirt 名を自分で作り、その方式を 2 回変えています。

| TrueNAS のバージョン | libvirt 名 | 自分で付けた名前がある場所 |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`（例：`1_debian`） | libvirt 名の中の `{id}_` の後ろ |
| 26 | VM の UUID（例：`550e8400-e29b-41d4-a716-446655440000`） | ドメイン XML の `<title>` のみ |

BombVault が TrueNAS と検出したホスト、または `PLATFORM=truenas` を指定したホストでは、バックアップの一覧に自分で付けた名前が表示されます。25.10 では `1_debian` の代わりに `debian`、26 では UUID の代わりに `<title>`、タイトルがなければ UUID です。これは一覧の表示だけです。すべての `virsh` 呼び出しと VM ごとの設定は引き続き libvirt 名を使うので、バックアップと復元は常に正しい VM に対して動作します。TrueNAS 26 では、TrueNAS が新しい UUID で作り直した VM は、名前が同じに見えても、設定のない新しい項目として表示されるという意味です。Unraid では何も変わらず、`10_Windows` と名付けた VM も同じです。

25.10 の方式は、実際の TrueNAS SCALE 25.10 のマシンで確認しました。TrueNAS 26 の方式は TrueNAS のドキュメントに基づくもので、単体テストのみです。

## トラブルシューティング {#troubleshooting}

| 症状 | 原因と対処 |
|---|---|
| **接続をテスト**が止まり、そのあと失敗する | ホストに到達できません。手順 3（ネットワーク、VLAN、ファイアウォール）と、**Host SSH: Address** および **Port** を確認してください。 |
| 鍵が `authorized_keys` にあるのに `Permission denied (publickey)` になる | 親フォルダーの所有者またはモードが正しくないため、sshd の `StrictModes` が鍵ファイルを拒否しています。`/root/.ssh` が、全員が書き込める FAT のフラッシュドライブ（`/boot/config/ssh/...`）へのリンクである場合によくあります。ホストのログには `Authentication refused: bad ownership or modes for directory ...` と出ます。ホストの sshd 設定のグローバルセクションの、`Match` ブロックの前に `StrictModes no` を追加して、`/etc/rc.d/rc.sshd restart` を実行してください。Unraid は起動のたびに `/etc/ssh/sshd_config` を新しく書くので、下の説明のように永続化してください。 |
| `Permission denied (publickey)` で、鍵が実際にない | 手順 2 をもう一度行い、SSH がオンになっていることを確認してください。 |
| `Host key verification failed` | `docker exec BombVault rm -f /config/ssh/known_hosts` を実行して、もう一度テストしてください。 |
| 手順 3 のチェックが `UNREACHABLE` になる | コンテナがホストの SSH ポートに届きません。手順 3 を参照してください。カスタムネットワークへのホストアクセス、VLAN のルーティング、シムまたは LAN の IP です。 |
| **Edit** で変数が見つからない | テンプレートをもう一度インポートしてください。既存のコンテナは古い保存済みの設定を保持します。 |
| Unraid スナップショットのある VM が失敗し、ファイル名が示される | ディスクのバッキングチェーンを最後までたどれません。VM Manager でそのディスクのすべてのスナップショットがまだあることと、示されたファイルが `/mnt` の下に存在することを確認してください。 |

そのほかの VM の症状は、[トラブルシューティング](troubleshooting.md#vm-backup-will-not-connect)にあります。

## 再起動後も保持する {#persistence}

Unraid は起動のたびに `/etc/ssh/sshd_config` を新しく書くので、ホスト側の変更は起動時にもう一度行う必要があります。

- **SSH ポート**は `/boot/config/ident.cfg` に保持され、Unraid の Web UI で設定します。
- 上で必要になった場合の **`StrictModes no`** は保持されません。`/boot/config/go` から起動時に適用してください。

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

- BombVault 自身の SSH 鍵と `~/.ssh/config` は `/config`（appdata）にあり、起動のたびに作り直されるので、コンテナとホストの再起動をまたいで残ります。

## セキュリティ {#security}

承認された鍵は、ホストの root 権限を与えます。BombVault がすでに使っている Docker ソケットと同じ水準の信頼です。BombVault は信頼できるネットワークに置いてください。鍵は ed25519 で、モード `0600` で appdata に保存され、ホストの鍵は固定されます。ホストのコマンドはすべて、リモートシェルを通さず、別々の引数として渡され、SSH 接続は `BatchMode` と接続タイムアウトを使います。
