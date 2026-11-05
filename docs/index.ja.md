# BombVault

**Unraid のデータを、金庫に封印。バックアップを投げ込み、復元を起爆する。**

![概要画面：ステータス欄、対応が必要なこと、各セクションの保護状況、保存先、履歴。](assets/screenshots/overview/home-dark.png#only-dark)
![概要画面：ステータス欄、対応が必要なこと、各セクションの保護状況、保存先、履歴。](assets/screenshots/overview/home-light.png#only-light)

BombVault は、Unraid をはじめ、TrueNAS SCALE、ZimaOS、通常の Linux マシンなどの Docker ホストで使える、セルフホスト型の Web アプリです。**バックアップと完全な災害復旧**を担当し、Docker コンテナ、KVM/libvirt VM、フォルダー、ZFS データセット、Unraid のフラッシュドライブをバックアップします。単一のマルチアーキテクチャ Docker コンテナとして動作し、ダーク、ライト、システム設定のテーマを選べる Web 画面を備え、バックアップ、スケジュール設定、検証、復元というライフサイクル全体を扱います。

復元は自動です。Unraid ではコンテナが Docker タブに以前とまったく同じ状態で再び現れ、VM はディスクと UEFI NVRAM を再アタッチした状態で VM Manager に再定義されます。手動での再インストールも、再設定も、面倒もありません。TrueNAS では、元のコンテナの隣に復元したコンテナは通常の Docker コンテナとして動き、Apps 画面には表示されません。

[restic](https://restic.net) を基盤としているため、すべてのバックアップは重複排除され、増分方式で、標準で暗号化されています。

!!! note "APP_KEY は安全に保管してください"
    BombVault は、`APP_KEY` という名前の 32 バイトのシークレットから restic リポジトリのパスワードを導出します。これをリカバリーキットとともに失うと、暗号化されたバックアップは復元不能になります。`openssl rand -hex 32` で生成し、安全な場所に保管してください。[設定](configuration.md)を参照してください。

## BombVault が保護するもの {#what-bombvault-protects}

コンテナ、VM、フォルダーセット、フラッシュ、すべての ZFS データセット、BombVault 自身の設定は、それぞれ専用のページを持つ**項目**です。同じ種類の項目をまとめたものが**セクション**で、**設定、一般**でオンにします。

| セクション | 保存される内容 |
|---|---|
| **コンテナ** | Appdata ディレクトリに加え、コンテナ定義（イメージ、環境変数、ポート、ラベル、ボリューム）。 |
| **VM** | VM ディスクイメージ、XML 定義、UEFI NVRAM を、SSH 経由でバックアップ（libvirt のマウントなし）。 |
| **フラッシュ** | USB フラッシュ全体（`/boot`）：OS、ライセンス、アレイ設定、共有、ネットワークおよびプラグイン設定。 |
| **セルフバックアップ** | BombVault 自身の `/config`：設定データベース、保存先の認証情報、libvirt の SSH 鍵ペア、ZFS サーバーの鍵と固定したホスト鍵、証明書。 |
| **フォルダー** | サーバー上の任意のフォルダーをフォルダーセットとして保存します。それぞれに除外パターンを任意で設定できます。 |
| **ZFS データセット** | データセットとその下のすべてのデータセットを 1 つの ZFS スナップショットから読み取り、フォルダーと同じように保存します。[ZFS データセット](zfs-datasets.md)を参照してください。 |

## 復元こそが主役 {#restore-is-the-star}

restic スナップショットからデータを書き戻したあと、BombVault は保存されたコンテナ定義を Docker API に対して再生します。そのため、コンテナは最初からそこにあったかのように Unraid の Docker タブに再び現れます（同じイメージ、同じ設定、同じポートマッピング）。VM は XML が SSH 経由で再定義され、ディスクと UEFI NVRAM が再アタッチされます。VM を削除したあとでも同じです。

1 つの画面で、あらゆる種類を復元できます。標準では**元の隣に**が選ばれていて、バックアップをコピーとして戻し、元のものはそのまま残します。**元を置き換える**は先に現在の状態をバックアップするので、復元を取り消せます。そのほかの復元先として、フォルダーや新しい ZFS データセットがあります。

バックアップで依存するコンテナを止めた場合は、正しい順序で戻ります。BombVault は Compose の `depends_on` の順に再起動し、それぞれが正常と報告するのを待ってから、それに依存するコンテナを起動します。まだ立ち上がっていないデータベースやゲートウェイを追い越して起動することはありません。[復元](restore.md)と[コマンドとほかのコンテナ](backups.md#commands)を参照してください。

## 仕組み {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault は Docker ソケットを使って、バックアップ前にコンテナを停止し、復元後にコンテナを再作成します。VM については、SSH（`qemu+ssh://`）経由でホスト上の `virsh` を実行し、ドメインを正常にシャットダウンするか、ライブスナップショットを取ります。libvirt のパスをバインドマウントすることは一切ないため、ホストの VM Manager の邪魔になることはありません。

BombVault はオーケストレーションと UI のレイヤーであり、ストレージエンジンではありません。実際のデータ移動はすべて restic を通じて行われます。

## クイックスタート {#quick-start}

はじめてですか？ **[はじめに](getting-started.md)** で BombVault をインストールして最初のバックアップを実行し、続けて **[オフサイトと復旧](offsite-recovery.md)** を設定してください。**[実践例](guides.md)** では、よくある作業を最初から最後まで順に説明しています。**[設定](settings.md)** のページにはすべてのタイルが載っていて、**[機能](features.md)** は BombVault ができることの短い一覧です。

各セクションは複数の保存先に同時にコピーでき、ペアリングした BombVault は削除を受け付けないサーバーにオフサイトのコピーを置けます。**緊急ウィザード**は、グループ経由または入力した鍵を使って、失ったサーバーをリカバリーキットから元に戻します。設定を新しいマシンに移すには、**設定のエクスポート / インポート**カードを使います。[オフサイトと復旧](offsite-recovery.md)および[設定](configuration.md#portable-settings-export-and-import)を参照してください。

**[Android アプリ](android.md)** を使えば、グループ内のすべてのサーバーをスマートフォンで扱え、全サーバーのアクティビティログを 1 つの画面で見られます。

ドキュメントは 26 言語、画面は 42 言語に対応しています。まだ翻訳のないページは英語で表示されます。

## この文書で使う言葉 {#glossary}

BombVault とこの文書では、いくつかの言葉が決まった意味で使われます。

| 言葉 | 意味 |
|---|---|
| **項目** | BombVault がバックアップする 1 つの対象。コンテナ、VM、フォルダーセット、フラッシュドライブ、その下のデータセットを含む ZFS データセット、または BombVault 自身の設定です。それぞれに専用のページがあります。 |
| **セクション** | コンテナなど、同じ種類の項目すべて。セクションは **設定、一般** でオンにします。 |
| **保存先** | バックアップの行き先。このサーバー上のフォルダー、共有、自分のサーバー、クラウドサービスなどです。[保存先](offsite-recovery.md#destinations)を参照してください。 |
| **ローカル** | このサーバー上にある、各セクションのフォルダーを置く保存先。 |
| **専用フォルダー** | セクションが最初にバックアップを書き込むリポジトリ。標準ではこのサーバー上にあり、保存先の中に置くこともできます。[セクションの専用フォルダー](offsite-recovery.md#own-folder)を参照してください。 |
| **コピー** | 専用フォルダーから別の保存先へ `restic copy` でコピーしたバックアップ。 |
| **ダイレクトリポジトリ** | ローカルのコピーを挟まず、項目が直接バックアップするオフサイトのリポジトリ。[ダイレクトリポジトリ](offsite-recovery.md#direct-repositories)を参照してください。 |
| **認証情報** | S3 サービスや rest-server 用に保存したキーまたはパスワード。そのうち 1 つが標準になります。[認証情報](offsite-recovery.md#sign-ins)を参照してください。 |
| **配置** | 項目のバックアップをどの保存先に置くか。項目ごと、またはセクションの標準として設定します。[項目ごとの配置](offsite-recovery.md#placement)を参照してください。 |
| **レプリカ** | `zfs send` で作る、2 台目の ZFS サーバー上の ZFS 項目のブロック単位の完全なコピー。バックアップの隣に並ぶもので、バックアップの代わりにはなりません。[レプリカ](zfs-datasets.md#replica)を参照してください。 |
| **ロール** | グループ内のあるインスタンスがほかのインスタンスのために担う役目：レシーバー、取得側、ZFS サーバー、鍵の保管側。[インスタンスとロール](offsite-recovery.md#roles)を参照してください。 |

## クレジット {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**（Apache-2.0、作者は [@Darkdragon14](https://github.com/Darkdragon14)）が、BombVault の出発点となるアイデアをくれました。Docker コンテナのワンクリックバックアップと自動再インストールです。BombVault は Go と restic による別個の実装で、そのアイデアを VM やフラッシュなどへ広げています。
- **[restic](https://restic.net/)** は、BombVault が動かす、高速で安全な重複排除バックアップエンジンです。
- **[rclone](https://rclone.org/)** はクラウドバックエンドを提供します。
- ボタンのグリフの大半は、**[Streamline](https://streamlinehq.com)** の無料の Core Solid セット（[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)、[ソース](https://github.com/webalys-hq/streamline-vectors)）のものです。残りは Font Awesome Free、Material Design Icons、Simple Icons、Tabler Icons のもの、またはこのプロジェクトのために描いたものです。

## ライセンス {#license}

Copyright (C) 2026 Junker der Provinz. BombVault は **GNU Affero General Public License v3.0**（[LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)）のもとで提供されるフリーソフトウェアです。実行、研究、共有、改変を行うことができます。配布する場合、または改変したバージョンをネットワークサービスとして運用する場合は、同じライセンスのもとでソースコードを公開し、既存の著作権表示と帰属表示を保持しなければなりません。

名前とブランドはライセンスの対象外です。AGPL が対象とするのはソースコードだけです。「BombVault」、そのロゴ、そのブランドは権利が留保されたままであり、フォークは独自の名前とブランドを使わなければならず、BombVault を名乗ってはなりません。

## リンク {#links}

- **ソースコード:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid サポートスレッド:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issue:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "ホストに対する root 相当の制御"
    Docker ソケットを通じて、BombVault はコンテナの停止、削除、再作成を行い、appdata の読み書きができます。また VM バックアップのためにホストに SSH でログインして `virsh` を実行します。その Web UI に到達できる者は、実質的にホストの root 権限を持つことになります。BombVault は信頼できる、外部に公開されていないネットワークでのみ実行し、**設定、セキュリティ** でログインパスワードを設定してください。リカバリーキット、鍵の預け入れ、フラッシュ ZIP、フラッシュバックアップやセルフバックアップのエクスポート、保存先のバックアップの削除にはパスワードが必要です。完全なセキュリティモデルについては[設定](configuration.md#security-model)を参照してください。
