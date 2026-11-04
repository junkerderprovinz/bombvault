# はじめに

このページでは、まっさらな Unraid マシンから、最初のバックアップまでを順に説明します。

## 動作要件 {#requirements}

| 要件 | 備考 |
|---|---|
| **Unraid 6.12 以降** | それより前のバージョンはテストしていません。主な対象は Unraid ですが、BombVault は通常の Docker ホストや TrueNAS Scale でも動きます（[汎用 Docker ホスト](#generic-docker-host)と [TrueNAS SCALE](#truenas) を参照）。 |
| **restic リポジトリの場所** | ローカルパス（おすすめはアレイまたはキャッシュ）、SMB、NFS、または任意の rclone バックエンド。 |
| **Docker ソケット** | テンプレートが自動でマウントします（`/var/run/docker.sock`）。 |
| **Unraid フラッシュ**（`/boot`） | テンプレートが全体を自動でマウントします（`/boot` を `/host/boot` へ）。フラッシュのバックアップを支え、復元したコンテナを通常の編集可能な Unraid アプリとして再び表示させます。 |
| **KVM VM**（任意） | VM のバックアップは SSH 経由で libvirt と通信し、libvirt のマウントは不要です。**設定、連携、ホスト SSH** で設定します（[設定](configuration.md)を参照）。 |
| **ZFS データセット**（任意） | VM バックアップと同じ SSH 接続、ホスト上の `zfs`、そしてホストデータを `/mnt` として、アクセスモード Read/Write - Slave（テンプレートの標準）でマップすること。[ZFS データセット](zfs-datasets.md)を参照してください。 |
| **Android アプリ**（任意） | Android 10 以降で、バージョン 9.7.0 以降のサーバーとペアリングします。[Android アプリ](android.md)を参照してください。 |

## Unraid へのインストール {#install-on-unraid}

いちばん簡単なのは **Community Applications** を使う方法です。

1. Unraid の **Apps** タブを開きます。
2. **BombVault** を検索します。
3. **Install** をクリックし、必須の変数（下記）を設定して適用します。

!!! tip "テンプレートを手動でインストールする"
    テンプレートを手動で追加したい場合は、次のようにします。

    1. **Docker, Add Container, Template repositories** に移動して、次を追加します。
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. テンプレートで **BombVault** を検索します。
    3. 必須の変数を設定して、**Apply** をクリックします。

## 汎用 Docker ホスト {#generic-docker-host}

Unraid ではない場合でも、BombVault は任意の Docker ホスト上で通常のコンテナとして動きます。TrueNAS SCALE には、[TrueNAS SCALE](#truenas) に専用の手順があります。

1. リポジトリから、そのまま編集できる [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) を取得します。
2. `APP_KEY`（下記参照）を設定し、ホストデータのボリュームを実際のデータのルートに向けます。ファイル内のコメントに両方の手順があります。
3. `docker compose up -d` を実行して、`https://<host-ip>:3443/` を開きます。

Unraid との違いは次のとおりです。

- **フラッシュセクションなし。** バックアップや復元の対象になる起動用 USB がないので、ここではフラッシュセクションにやることがありません。代わりに、ホストデータがホストの `/etc` に届いている場合、バックアップページの**追加**が、フォルダーの候補として**システム構成**を提案します。保存前に確認して編集する、`/etc` の最初のフォルダーセットです。
- **Unraid 専用の通知なし。** BombVault 自身のアプリ内通知チャンネル（Webhook、オフサイトの失敗アラートなど）は通常どおり動きます。Unraid 固有の、ネイティブな通知システムへのプッシュだけは、送る先がないのでスキップされます。
- **VM バックアップは任意で、SSH で到達できる別の libvirtd ホストが必要です。** compose ファイルのコメントアウトされたブロックを参照してください。汎用の Docker ホストには VM マネージャーが組み込まれていません。
- **ダッシュボードウィジェットなし。** BombVault Widget は Unraid のプラグインなので、その手順もスキップされます。
- **コンテナのデータの見つけ方。** Unraid の `appdata` の慣習がないため、コンテナのデータフォルダーは、`DATA_ROOT_SEGMENTS` のセグメント、Docker の名前付きボリューム、Compose プロジェクトの作業ディレクトリ、`bombvault.data` ラベルから見つけます（[バックアップ元の検出](configuration.md#backup-source-detection)を参照）。名前付きボリュームと `/etc` のプリセットが届くのは、ホストデータのマウント内のパスだけなので、Docker のデータルートも含む共通の親フォルダーにホストデータを向けてください。
- **`PLATFORM`。** `generic` または `truenas` に設定します。未設定の場合、BombVault はフラッシュのマウント上の独自のマーカーで Unraid を判定し、それ以外はすべて generic として扱い、Unraid 専用の手順は試して失敗するのではなくスキップされます。

### TrueNAS SCALE {#truenas}

BombVault は TrueNAS のアプリカタログにまだ入っていないので、カスタムアプリとしてインストールします。**Apps, Discover Apps** を開き、**Custom App** の横のメニューから **Install via YAML** を選びます。下の YAML は、リポジトリの `truenas-apps` フォルダーにあるカタログ項目と同じ設定をします。`tank` を自分のプールに置き換え、次のセクションの説明に従って `APP_KEY` を入力してください。

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

そのあと `https://<truenas-ip>:30528/` を開きます。[Unraid との違い](#generic-docker-host)に挙げたことはここでも当てはまり、TrueNAS では次の点も加わります。

- **ケーパビリティ。** BombVault には上の 7 つが必要です。`SETPCAP` がないと VM のバックアップが失敗し、`SYS_CHROOT` がないと、すべてのコンテナがスナップショットから読み取られる代わりにバックアップの間ずっと停止されます。
- **ホストデータ**は、データセット上のホストのパス（ここでは `/mnt/tank`）で、アプリ内の同じパスにマウントされます。そのため `HOST_SOURCE_ROOT` と `HOST_MOUNT_ROOT` の両方がそれを指しています。`rslave` は、あとからホストが行うマウントを引き継ぐもので、ZFS データセットのバックアップに必要です。
- **ixVolume にあるデータはバックアップされません。** ストレージが ixVolume のアプリは、データをホストデータの外の `/mnt/.ix-apps` に置いています。BombVault が保存するのはアプリの定義だけで、実行時にどのフォルダーを除外したかが書かれます。データをバックアップしたいアプリでは、ホストデータの下のデータセット上の **Host Path** ストレージを選んでください。
- **VM と ZFS データセットのバックアップ**は、root として SSH で TrueNAS に届きます。**System, Services** で SSH サービスをオンにし、**設定、連携、ホスト SSH** の公開鍵を、**Credentials, Users** の root ユーザーに追加してください。VM のバックアップには、上のように `LIBVIRT_URI` も必要です。TrueNAS の libvirtd は専用のソケットで待ち受けているからです。`LIBVIRT_URI` も `LIBVIRT_HOST` も設定していない場合、TrueNAS のアプリでは `host.docker.internal` が名前解決できないため、BombVault はアプリのデフォルトゲートウェイをホストとして使います。詳細は [VM バックアップガイド](vm-backup-ssh-setup.md#truenas-scale)にあり、zvol 上の VM も扱っています。
- **ZFS レプリカは別の鍵を使います。** 2 台目の ZFS サーバーへのレプリカは、ホスト SSH の鍵ではなく、レプリカのダイアログに表示される鍵でログインします。その鍵を、受信側のサーバーに追加してください。
- **削除したアプリは 2 段階で戻ります。** まず TrueNAS で同じ名前でアプリを再インストールしてネットワークを作り直し、そのあと BombVault でデータを復元します。それまでは、復元チェックが足りないネットワークを報告し、復元は何かを変更する前に拒否します。
- **元の隣に**を使うと、通常の Docker コンテナが作られます。動きますが、TrueNAS の Apps 画面には表示されません。
- **データベースダンプ**はデータセットのマウント上で動きます。それ自体がデータセットであるデータフォルダーへのインポートでは、古い内容がその隣のフォルダーに移動されるので、上位のデータセットにその分の空きが必要です。

### ZimaOS、CasaOS、そのほかのホスト {#other-hosts}

- **ZimaOS と CasaOS：** [コミュニティストア](https://github.com/junkerderprovinz/zimaos-apps#install)を追加して、そこから BombVault をインストールします。そこでは `PLATFORM=generic` で、ホストデータは `/DATA` で動きます。ZimaOS はアプリのデータを `/DATA/AppData/<app>` に置いていて、`DATA_ROOT_SEGMENTS` が大文字小文字を問わず `appdata` に一致するので、BombVault は自動で見つけます。
- **MOS：** [MOS Hub](https://mos-official.net/apps.html?search=BombVault) から BombVault をインストールします。そこでは `PLATFORM=generic` で、ホストデータは `/mnt/cache` で動きます。
- **Synology：** Container Manager で、compose ファイルからプロジェクトを作成します（DSM 7.2 以降）。
- **Proxmox：** Docker が入った VM または LXC コンテナで動かします。

コメントを除くと、汎用の compose ファイルは短いものです。`./data` を、コンテナのデータが入っているホストのフォルダーに置き換え、`HOST_SOURCE_ROOT` を同じパスにして、`APP_KEY` を入力してください。

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Docker の名前付きボリュームもバックアップしたい場合は、Unraid のテンプレートが `/mnt` 全体をマウントするように、データと Docker のデータルート（`docker info -f '{{.DockerRootDir}}'`、通常は `/var/lib/docker`）の両方を含むフォルダーをマウントしてください。

## 唯一の必須設定 {#the-one-required-setting}

必ず設定しなければならない変数は `APP_KEY` だけです。これは restic リポジトリのパスワードを導出するための、32 バイトの 16 進数のシークレット（16 進数 64 文字）です。

どのマシンでも生成できます。

```bash
openssl rand -hex 32
```

結果を、テンプレート（Unraid）の `APP_KEY` フィールド、または `docker-compose.yml`（汎用 Docker ホスト）の `APP_KEY` 環境変数に貼り付けます。

!!! danger "APP_KEY をなくさないでください"
    `APP_KEY` をなくすと、暗号化されたバックアップは復元できなくなります。サーバーとは別の、安全な場所に保管してください。BombVault が動いたら、ログインパスワードを設定し、**設定、復元** で**リカバリーキット**をダウンロードしてください（[オフサイトと復旧](offsite-recovery.md#recovery-kit)を参照）。キットには、鍵、すべての保存先、restic のコマンドが入っています。

テンプレートは、Docker ソケット、フラッシュ（`/boot`）、**ホストデータ**のルート（`/mnt`）のマウントも行います。バックアップの*ソース*と*保存先*は、どちらもホストデータの下にあります。変数の完全な一覧とオフサイトの設定は、[設定](configuration.md)を参照してください。

## 初回起動 {#first-run}

![新規インストールのウェルカムページ。BombVault がサーバーで見つけたものが表示されている。](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![新規インストールのウェルカムページ。BombVault がサーバーで見つけたものが表示されている。](assets/screenshots/getting-started/welcome-light.png#only-light)

1. `https://<your-unraid-ip>:3443` で Web 画面を開きます（標準では自己署名証明書です）。新規インストールでは **BombVault へようこそ**が開き、BombVault がサーバーで見つけたものが一覧されます。
2. **始める**を選びます。すると概要に、3 つのステップを持つ**最初のステップ**カードが表示されます。
    1. **セクションを選ぶ**：BombVault がバックアップする対象（コンテナ、VM、フラッシュ、フォルダー、ZFS データセット、セルフバックアップ）をオンにします。**選ぶ**を押すとセクションの項目の一覧が開き、**スケジュールを一時停止**で 1 つを対象から外せます。ここでオンにしたセクションは毎晩実行されます。時刻は **設定、スケジュール** で変えられます。
    2. **保存先を追加**：バックアップの 2 つ目の置き場所です。どのセクションにも、すでにこのサーバー上のフォルダー `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` があり、最初のバックアップで作られます。いちばん安全なのは、Storage Box や Backblaze B2 のような、設置場所の外にある 2 つ目の保存先です。
    3. **最初のバックアップ**：そのあとは毎晩自動で実行されます。最初のバックアップが完了するまで、カードは残ります。
3. **バックアップ**を開くと、すべての項目が 1 つの一覧に並びます。**今すぐすべてバックアップ**は、オンになっているすべてのセクションの実行を開始し、項目のページの**今すぐバックアップ**は、その 1 つだけをバックアップします。

![3 つのステップを持つ「最初のステップ」カード。2 つ目が開いている。](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![3 つのステップを持つ「最初のステップ」カード。2 つ目が開いている。](assets/screenshots/getting-started/first-steps-light.png#only-light)

以前に BombVault を使っていましたか？ ウェルカムページの**バックアップから復元**は復元ページを開き、そこの**緊急ウィザード**が、失われたサーバーの設定、コンテナ、VM、フォルダーを取り戻します。[オフサイトと復旧](offsite-recovery.md#emergency)を参照してください。

!!! tip "任意：バックアップの順序を決める"
    あるコンテナを必ずほかより先にバックアップしたい場合（たとえば、データベースをそれを使うアプリより先に）は、**設定、スケジュール** の**順序**カードを開き、希望の並びにドラッグしてください。スケジュール実行と全体バックアップは、その順序に従います。順序を付けなかったものは、最も遅れているものから順にバックアップされます。

## 例：最初のコンテナバックアップを最初から最後まで {#example-first-backup}

ここでは、`plex` を含むいくつかのコンテナがあるまっさらな Unraid マシンで、最初のバックアップが終わり、その保存場所が分かるまでを順に追います。約 10 分と、最初のバックアップにかかる時間が必要です。

1. **インストール。** Unraid の **Apps** タブで **BombVault** を検索し、**Install** を押します。
2. **鍵を設定。** パソコンで `openssl rand -hex 32` を実行し、64 文字をテンプレートの `APP_KEY` フィールドに貼り付けます。何よりも先に、今のうちに、コピーをパスワードマネージャーに保管してください。**Apply** を押します。
3. **開く。** `https://<your-unraid-ip>:3443` に移動して、自己署名証明書を受け入れます。**BombVault へようこそ**に、見つかったもの（たとえば 12 個のコンテナ、2 つの VM、フラッシュドライブ）が一覧されます。
4. **始める。** 概要が開き、**最初のステップ**カードが表示されます。
5. **セクションを選ぶ。** **コンテナ**は最初からオンです。ほかにバックアップしたいもの（たとえば**フラッシュ**）をオンにします。**選ぶ**を押すと、見つかったコンテナの一覧が開き、**スケジュールを一時停止**で、気にしないテスト用コンテナなどを対象から外せます。オンにしたセクションは毎晩の全体バックアップで実行されます。
6. **保存先を追加。** どのセクションにも、すでにこのサーバー上の `/mnt/user/bombvault` の下に専用のフォルダーがあるので、最初のバックアップの行き先はあります。この手順では 2 つ目の場所を追加します。いちばん安全なのは、設置場所の外です。今すぐ [S3 の例](offsite-recovery.md#example-s3)で行うことも、最初のバックアップが終わってから戻ってくることもできます。それまでカードは残ります。
7. **最初のバックアップ。** **バックアップ**を開いて**今すぐすべてバックアップ**を押すか、夜を待ちます。ステータス欄のリングは進捗リングに変わり、項目数を数えます（たとえば **バックアップ中: 12 件中 3 件**）。各コンテナは、データを読み取るあいだ停止し、その直後に再び起動します。データが ZFS 上にあるコンテナは、スナップショットの数秒だけ停止します。最初のバックアップはすべてを読み取るので、いちばん時間がかかります。次回からは、変わった分だけが保存されます。
8. **確認する。** リングに**すべてバックアップ済み**と表示されたら、概要の**履歴**を開きます。**実行**で、すべてのコンテナに緑の行があります。**バックアップ**を開いて `plex` をクリックし、その**バックアップ**タブを見ます。最初のスナップショットがあり、それを持つ場所として**ローカル**が表示されています。数分後には、その**検証**タブに復元チェックが表示され、バックアップの標本を復元して読み戻したことが分かります。
9. **鍵を守る。** **設定、セキュリティ** を開いて、ログインパスワードを設定します。次に **設定、復元** を開いて、**緊急時用**の**リカバリーキットをダウンロード**を押します。ファイルは、手順 2 でコピーした `APP_KEY` の隣に、サーバーの外に保管してください。概要の**リカバリーキットを保存してください**というリマインダーは、**安全に保管しました**を押すと消えます。

これで、コンテナは毎晩バックアップされます。このあとの良い進め方は次の順です。まず、サーバーが壊れてもバックアップが一緒に失われないよう、[オフサイトの保存先](offsite-recovery.md#example-s3)。次に、失敗が届くようにする[通知](notifications.md)。そして、03:00 が都合に合わなければ[スケジュール](schedules.md)を見直します。

![最初のバックアップの実行中のステータス欄。進捗リングとそのステーションが見える。](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![最初のバックアップの実行中のステータス欄。進捗リングとそのステーションが見える。](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "ホスト統合のチェック"
    概要の**ホスト統合**カードは、すべてのマウントと CLI（Docker ソケット、libvirt、restic、qemu-img、rclone）を調べて、足りないものを報告します。頼りにする前に、コンテナが正しくつながっているかを確認できます。このカードは、追加するまで**カスタマイズ**の下に控えています。

## 画面の見つけ方 {#navigation}

![タイルとして表示された設定と、選んだタイルのカード。変更はすべてその場で書き込まれる。](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![タイルとして表示された設定と、選んだタイルのカード。変更はすべてその場で書き込まれる。](assets/screenshots/getting-started/settings-rail-light.png#only-light)

サイドバーには 5 つの項目があります。**概要**、**異常**（異常検出がオンのあいだ）、**バックアップ**、**復元**、そして**サインアウト**の下の**設定**です。ページの主な操作は右下に浮かんでいて、あまり使わないものは**その他**にあります。スマートフォンでは、バーに概要、バックアップ、復元、設定が並び、残りは**その他**にあります。

ページと設定は、ウィンドウの幅いっぱいを使います。ウィンドウのボタンは足元にまとまり、先へ進むものが右側にあります。スマートフォンでは、ボタンが 4 つ以上ある足元は、戻る手段と主な操作を残し、ほかを**その他**の下に入れます。

![スマートフォンで、下部のバーと開いた「その他」。](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![スマートフォンで、下部のバーと開いた「その他」。](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **概要**は、すべてバックアップされているか、何に対応が必要かを示します。[概要](overview.md)を参照してください。
- **バックアップ**は、すべてのコンテナ、VM、フォルダーセット、フラッシュドライブ、すべての ZFS データセット、セルフバックアップを項目として一覧にします。一覧の上のタイルで種類を選び、**フィルター**、**並べ替え**、**検索**で絞り込みます。どの項目にも、**バックアップ**、**設定**、**検証**の 3 つのタブを持つ専用ページがあります。項目のアイコンの左にある戻るボタンで一覧に戻れます。[バックアップと項目](backups.md)を参照してください。
- **復元**は、すべてのための 1 つのページです。セクションを選び、項目を検索し、バックアップを選びます。緊急ウィザードやほかの取得元もここから始まります。[復元](restore.md)を参照してください。
- **設定**は 14 個のタイルです。一般、外観、スケジュール、保存先、復元、通知、検証、セキュリティ、インスタンス、ペアリング、連携、アプリ、システム、情報。[設定](settings.md)を参照してください。`/` か Ctrl+K を押すと、すべてを検索できます。以前のバージョンでページが持っていた名前でも見つかります。タイルは**外観**で好きな順にドラッグできます。ページの横にタイルの列を置くと窮屈になる場合（幅 1200 ピクセル程度のウィンドウなど）は、タイルがページの上の、横にスクロールできる帯になります。スマートフォンでも同じ帯が表示されます。
- **保存先**と**インスタンス**は、専用のページを持つ設定タイルです。タイルの**サイドバーにも表示**で、サイドバーに固定できます。

保存ボタンはどこにもなく、折りたたまれて開くものもありません。あまり使わないものは専用のカードかウィンドウにあり、説明は (i) のバブルにあります。アップデートのあとは、前のバージョン以降のリリースごとのカードを持つウィンドウが自動で開き、変わった箇所に緑のドットが付きます。クリックするとドットは消えます。**設定、情報** のバージョンカードの**新規**は、まだ見ていないものを数え、そのウィンドウを、各リリースのノートと変更点（**そこへ移動**付き）とともにもう一度開きます。

## ソースからビルドする {#build-from-source}

BombVault は、JSON API と埋め込みの React 画面を提供する、単一の静的 Go バイナリです。先に画面をビルドしてから、バイナリを実行します。

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

`go run` でも画面のビルドが必要です。リポジトリで管理しているのは `web/dist` の下の空のマーカーだけなので、`npm --prefix web run build` をしないと、バイナリには何も埋め込まれず、`500 SPA index not found` を返します。これは想定どおりです。Docker、libvirt、Unraid は CI でテストできないので、プルリクエストを出す前に、概要の**ホスト統合**カード（または `GET /api/spike`）を使って、実際のホストでマウント、restic、VM の SSH 接続を確認してください。

## 次のステップ {#next-steps}

- 設置場所の外にある保存先を 1 つ以上追加し（各セクションは複数に同時にコピーできます）、**[オフサイトと復旧](offsite-recovery.md)** の説明に従ってリカバリーキットを保存します。[S3 の例](offsite-recovery.md#example-s3)で一つ順に説明しています。
- **[通知](notifications.md)** で、失敗が確実に届くようにします。
- **[スケジュール](schedules.md)** で、いつ実行されるかを確認します。
- **[Android アプリ](android.md)** で、グループ内のすべてのサーバーをスマートフォンに表示します。
- **[実践例](guides.md)** ではよくある作業を順に説明し、**[機能](features.md)** には BombVault ができることがすべて載っています。
- セットアップの複製や、新しいマシンへの移行ですか？ **設定のエクスポート / インポート**カードで、設定全体を引き継げます。[設定](configuration.md#portable-settings-export-and-import)を参照してください。
- 困ったときは **[トラブルシューティング](troubleshooting.md)** を参照してください。
