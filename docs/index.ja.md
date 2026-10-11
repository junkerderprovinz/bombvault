# BombVault

**Unraid のデータを、金庫に封印。バックアップを投げ込み、復元を起爆する。**

BombVault は、Docker コンテナと KVM/libvirt VM の**バックアップと完全な災害復旧**のための、セルフホスト型で Unraid ネイティブな Web アプリです。単一のマルチアーキテクチャ Docker コンテナとして動作し、ダーク、ライト、システム設定のテーマを選べる Web 画面を備え、バックアップ、スケジュール設定、検証、復元というライフサイクル全体を扱います。

復元は自動です。コンテナは Unraid の Docker タブに以前とまったく同じ状態で再び現れ、VM はディスクと UEFI NVRAM を再アタッチした状態で VM Manager に再定義されます。手動での再インストールも、再設定も、面倒もありません。

[restic](https://restic.net) を基盤としているため、すべてのバックアップは重複排除され、増分方式で、常に暗号化されています。

!!! note "APP_KEY は安全に保管してください"
    BombVault は、`APP_KEY` という名前の 32 バイトのシークレットから restic リポジトリのパスワードを導出します。これをリカバリーキットとともに失うと、暗号化されたバックアップは復元不能になります。`openssl rand -hex 32` で生成し、安全な場所に保管してください。[設定](configuration.md)を参照してください。

## BombVault が保護するもの

コンテナ、VM、フォルダーセット、フラッシュ、すべての ZFS データセット、BombVault 自身の設定は、それぞれ専用のページを持つ**項目**です。同じ種類の項目をまとめたものが**セクション**で、**設定、一般** でオンにします。

| セクション | 保存される内容 |
|---|---|
| **コンテナ** | Appdata ディレクトリに加え、コンテナ定義（イメージ、環境変数、ポート、ラベル、ボリューム）。 |
| **VM** | VM ディスクイメージ、XML 定義、UEFI NVRAM を、SSH 経由でバックアップ（libvirt のマウントなし）。 |
| **フラッシュ** | USB フラッシュ全体（`/boot`）：OS、ライセンス、アレイ設定、共有、ネットワークおよびプラグイン設定。 |
| **セルフバックアップ** | BombVault 自身の `/config`：設定データベース、保存先のサインイン情報、libvirt の SSH 鍵ペア。 |
| **フォルダー** | サーバー上の任意のフォルダーをフォルダーセットとして保存します。それぞれに除外パターンを任意で設定できます。 |
| **ZFS データセット** | データセットとその下のすべてのデータセットを 1 つの ZFS スナップショットから読み取り、フォルダーと同じように保存します。[ZFS データセット](zfs-datasets.md)を参照してください。 |

## 復元こそが主役

restic スナップショットからデータを書き戻したあと、BombVault は保存されたコンテナ定義を Docker API に対して再生します。そのため、コンテナは最初からそこにあったかのように Unraid の Docker タブに再び現れます（同じイメージ、同じ設定、同じポートマッピング）。VM は XML が SSH 経由で再定義され、ディスクと UEFI NVRAM が再アタッチされます。VM が削除された後でも同様です。

1 つのウィンドウで、すべての種類を復元できます。初期状態では **元の隣に** が選ばれ、バックアップをコピーとして戻して元はそのままにします。**元を置き換える** は先に現在の状態をバックアップするので、復元を取り消せます。ほかの復元先はフォルダーと新しい ZFS データセットです。

バックアップが依存するコンテナを停止した場合でも、正しい順序で復帰します。BombVault はそれらを Compose の `depends_on` 順で再起動し、それに依存するコンテナを起動する前に、各コンテナが healthy を報告するまで待機します。そのため、まだ立ち上がっていないデータベースやゲートウェイを追い越して起動してしまうことはありません。[機能](features.md)を参照してください。

## 仕組み

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault は Docker ソケットを使って、バックアップ前にコンテナを停止し、復元後にコンテナを再作成します。VM については、SSH（`qemu+ssh://`）経由でホスト上の `virsh` を実行し、ドメインを正常にシャットダウンするか、ライブスナップショットを取ります。libvirt のパスをバインドマウントすることは一切ないため、ホストの VM Manager の邪魔になることはありません。

BombVault はオーケストレーションと UI のレイヤーであり、ストレージエンジンではありません。実際のデータ移動はすべて restic を通じて行われます。

## クイックスタート

はじめてですか？ **[はじめに](getting-started.md)** に進んで、Community Applications 経由で Unraid に BombVault をインストールし、最初のバックアップを実行しましょう。その後、**[機能](features.md)**の全体を探索し、**[設定](configuration.md)**を調整し、**[オフサイトと復旧](offsite-recovery.md)**をセットアップしてください。

各セクションは複数の保存先に同時にコピーでき、ペアリングした BombVault は削除を受け付けないサーバーにオフサイトのコピーを置けます。**緊急ウィザード**は、グループ経由または入力した鍵を使って、失ったサーバーをリカバリーキットから元に戻します。設定を新しいマシンに移すには、**設定のエクスポート / インポート**カードを使います。[オフサイトと復旧](offsite-recovery.md)および[設定](configuration.md#portable-settings-export-and-import)を参照してください。

**[Android アプリ](android.md)** を使えば、グループ内のすべてのサーバーをスマートフォンで扱え、全サーバーのアクティビティログを 1 つの画面で見られます。

## クレジット {#credits}

- [@Darkdragon14](https://github.com/Darkdragon14) による **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)**（Apache-2.0）が、BombVault の出発点となるアイデアをくれました。Docker コンテナのワンクリックバックアップと自動再インストールです。BombVault は Go と restic による別個の実装で、そのアイデアを VM やフラッシュなどへ広げています。
- **[restic](https://restic.net/)** は、BombVault が動かす、高速で安全な重複排除バックアップエンジンです。
- **[rclone](https://rclone.org/)** はクラウドバックエンドを提供します。
- ボタンのグリフの大半は、**[Streamline](https://streamlinehq.com)** の無料の Core Solid セット（[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)、[ソース](https://github.com/webalys-hq/streamline-vectors)）のものです。残りは Font Awesome Free、Material Design Icons、Simple Icons、Tabler Icons のもの、またはこのプロジェクトのために描いたものです。

## ライセンス {#license}

Copyright (C) 2026 Junker der Provinz. BombVault は **GNU Affero General Public License v3.0**（[LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)）のもとで提供されるフリーソフトウェアです。実行、研究、共有、改変を行うことができます。配布する場合、または改変したバージョンをネットワークサービスとして運用する場合は、同じライセンスのもとでソースコードを公開し、既存の著作権表示と帰属表示を保持しなければなりません。

名前とブランドはライセンスの対象外です。AGPL が対象とするのはソースコードだけです。「BombVault」、そのロゴ、そのブランドは権利が留保されたままであり、フォークは独自の名前とブランドを使わなければならず、BombVault を名乗ってはなりません。

## リンク

- **ソースコード:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid サポートスレッド:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issue:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "ホストに対する root 相当の制御"
    Docker ソケットを通じて、BombVault はコンテナの停止、削除、再作成を行い、appdata の読み書きができます。また VM バックアップのためにホストに SSH でログインして `virsh` を実行します。その Web UI に到達できる者は、実質的にホストの root 権限を持つことになります。BombVault は信頼できる、外部に公開されていないネットワークでのみ実行し、オフサイトまたはイミュータブルなバックアップを使い始めたら、**設定、セキュリティ** でログインパスワードを設定してください。リカバリーキット、鍵の預け入れ、保存先のバックアップの削除にはパスワードが必要です。完全なセキュリティモデルについては[設定](configuration.md#security-model)を参照してください。
