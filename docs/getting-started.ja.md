# はじめに

このページでは、まっさらな Unraid マシンから最初のバックアップまでを案内します。

## 要件

| 要件 | 備考 |
|---|---|
| **Unraid 6.12 以上** | それより古いバージョンはテストされていません。Unraid が主な対象ですが、BombVault は通常の Docker ホストや TrueNAS Scale でも動きます（[一般的な Docker ホスト](#generic-docker-host)を参照）。 |
| **restic リポジトリの場所** | ローカルパス（推奨: アレイまたはキャッシュ）、SMB、NFS、または任意の rclone バックエンド。 |
| **Docker ソケット** | テンプレートによって自動的にマウントされます（`/var/run/docker.sock`）。 |
| **Unraid フラッシュ**（`/boot`） | テンプレートによって全体が自動的にマウントされます（`/boot` を `/host/boot` へ）。フラッシュバックアップを支え、復元されたコンテナが通常の編集可能な Unraid アプリとして再び現れるようにします。 |
| **KVM VM**（オプトイン） | VM バックアップは SSH 経由で libvirt と通信し、libvirt のマウントは不要です。設定で構成します（[設定](configuration.md)を参照）。 |
| **ZFS データセット**（オプトイン） | VM バックアップと同じ SSH 接続、ホスト上の `zfs`、そしてアクセスモード Read/Write - Slave で `/mnt` にマッピングした Host Data（テンプレートの既定値）が必要です。[ZFS データセット](zfs-datasets.md)を参照してください。 |
| **Android アプリ**（任意） | Android 10 以降で、ペアリングするサーバーはバージョン 9.7.0 以降が必要です。[Android アプリ](android.md)を参照してください。 |

## Unraid へのインストール

最も簡単な方法は **Community Applications** です。

1. Unraid の **Apps** タブを開きます。
2. **BombVault** を検索します。
3. **Install** をクリックし、必要な変数（下記）を設定して適用します。

!!! tip "テンプレートの手動インストール"
    テンプレートを手動で追加したい場合は:

    1. **Docker, Add Container, Template repositories** に移動して、以下を追加します:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Templates で **BombVault** を検索します。
    3. 必要な変数を設定して **Apply** をクリックします。

## 一般的な Docker ホスト {#generic-docker-host}

Unraid ではない場合。BombVault はどの Docker ホストでも通常のコンテナとして動きます（TrueNAS Scale でのコンテナ対応も、独自のアプリカタログ掲載に先立ってこれで動いています）。

1. リポジトリからそのまま編集できる [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) を取得します。
2. `APP_KEY`（下記参照）を設定し、Host Data ボリュームを実際のデータルートに向けます。どちらもファイル内のコメントが案内します。
3. `docker compose up -d` を実行し、`https://<host-ip>:3443/` を開きます。

Unraid との違い:

- **flash/USB ドメインがありません。** 取り込んだり復元したりする起動 USB が存在しないため、設定のフラッシュドメインはここでは出番がありません。代わりにフォルダードメインが、ワンクリックの提案 **プリセットを追加: ホストのシステム設定**（保存前に確認して編集する `/etc` の初期ファイルセット）を、実用的な汎用の代替として用意しています。
- **Unraid 独自の通知がありません。** BombVault 自身の通知チャネル（Webhook、オフサイト失敗の警告など）は通常どおり動きます。省かれるのは Unraid 固有の通知システムへの送信だけで、ここにはその仕組みがないからです。
- **VM のバックアップは任意で、SSH で届く別の libvirtd ホストが必要です。** compose ファイル内のコメントアウトされたブロックを参照してください。一般的な Docker ホスト自体には VM 管理機能はありません。
- **ダッシュボードウィジェットがありません。** BombVault Widget は Unraid のプラグインなので、その手順も省かれます。
- **コンテナのデータの見つけ方。** Unraid の `appdata` の慣習がないため、コンテナのデータフォルダーは `DATA_ROOT_SEGMENTS` の区切り、Docker の名前付きボリューム、Compose プロジェクトの作業ディレクトリ、`bombvault.data` ラベルから見つけます（[バックアップ元の自動判定](configuration.md#backup-source-detection)を参照）。名前付きボリュームと `/etc` プリセットが届くのは Host Data マウント内のパスだけなので、Host Data は Docker のデータルートも含む共通の上位ディレクトリに向けてください。
- **`PLATFORM`。** `generic` または `truenas` に設定します。未設定の場合、BombVault はフラッシュのマウントにある Unraid 独自の目印で Unraid を判別し、それ以外はすべて汎用として扱います。Unraid 専用の手順は、試して失敗する代わりに省かれます。

**TrueNAS Scale** でも同じ compose の方法を使います。カタログのエントリはリポジトリに用意してありますが、まだ提出していません。TrueNAS の libvirtd は独自のソケット（`/run/truenas_libvirt/libvirt-sock`）で待ち受けており、3 つの `LIBVIRT_*` 変数では表せないため、そこでの VM バックアップには `LIBVIRT_URI` が必要です（[設定](configuration.md)を参照）。検証の範囲は次のとおりです。zvol のバックアップは実際の TrueNAS Scale マシンで、稼働中の VM に接続された zvol に対して実行し、`zfs snapshot`、`zfs send`、restic、`zfs receive` の往復でバイト単位まで一致しました。BombVault 自身による完全な復元は TrueNAS のハードウェア上ではまだ実行しておらず、その zvol はスパースだったため、数ギガバイト規模でのスループットは未検証です。頼りにする前に、そこで復元を試してください。

## 必須設定はひとつだけ

設定が必須の変数は `APP_KEY` だけです。これは restic リポジトリのパスワードを導出するために使われる 32 バイトの 16 進シークレット（16 進数 64 文字）です。

任意のマシンで生成します:

```bash
openssl rand -hex 32
```

生成された結果をテンプレートの `APP_KEY` フィールド（Unraid）、または `docker-compose.yml` の環境変数 `APP_KEY`（汎用の Docker ホスト）に貼り付けます。

!!! danger "APP_KEY をなくさないでください"
    `APP_KEY` を失うと、暗号化されたバックアップは復元不能になります。サーバーとは別の安全な場所に保管してください。BombVault が起動したら、ワンクリックの**暗号化キー・リカバリーキット**（[オフサイトと復旧](offsite-recovery.md)を参照）を使って、完全なリカバリーバンドルを保存してください。

テンプレートは Docker ソケット、フラッシュ（`/boot`）、そして **Host Data** ルート（`/mnt`）もマウントしてくれます。バックアップの*ソース*と*デスティネーション*はどちらも Host Data の配下に存在します。変数の完全なリファレンスとオフサイトのセットアップについては、[設定](configuration.md)を参照してください。

## 初回起動

![最初のバックアップ後のダッシュボード。何が守られ、次に何が動き、いま何が起きているか。](assets/screenshots/dashboard.png)

*最初のバックアップ後のダッシュボード。何が守られ、次に何が動き、いま何が起きているか。*

1. Web UI を `https://<your-unraid-ip>:3443` で開きます（デフォルトで自己署名証明書）。
2. **設定** で、使いたいバックアップドメイン（コンテナ、VM、フラッシュ、セルフバックアップ、フォルダー、ZFS データセット）を有効にし、アクセントカラーを選びます。
3. **コンテナ** タブでコンテナを選び、**今すぐバックアップ** をクリックして最初の復元ポイントを作成します。リポジトリパスはデフォルトで `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` になり、初回バックアップ時に作成されます。
4. **設定、スケジュール** からスケジュールを設定します。コンテナと VM には*すべてスケジュールに含める*ワンクリック操作があります。

!!! tip "任意: バックアップ順序を決める"
    一部のコンテナを常に他のコンテナより先にバックアップしたい場合（たとえばデータベースをそれを使うアプリより先に）は、コンテナページの**バックアップ順序**パネルを開き、希望する順序にドラッグします。以降、スケジュール実行や複数選択実行はその順序に従います。順序を指定しなかったものは、これまでどおり最も期限超過のものから順にバックアップされます。

!!! note "ホスト統合チェック"
    コンテナが起動したあと、Web UI で `/spike` を開いてください。これはすべてのマウントと CLI（Docker ソケット、libvirt、restic、qemu-img、rclone）をプローブし、欠けている部分を報告します。これにより、コンテナに頼る前に、正しく配線されていることを確認できます。

## シンプル vs 詳細

![設定に保存ボタンはありません。変更したその場で書き込まれます。](assets/screenshots/settings.png)

*設定に保存ボタンはありません。変更したその場で書き込まれます。*

デフォルトでは、インターフェースは基本操作（バックアップ、復元、スケジュール）だけを表示します。サイドバーの **シンプル表示 / 詳細表示** スイッチを使うと、エキスパート向けの操作が表示されます: 保持、オフサイトコピー、pre/post フック、ファイルレベルの復元、通知、Prometheus メトリクス、そして整合性/メンテナンスツールです。これはブラウザごとの設定で、デフォルトはオフです。そのため、初心者はすっきりした UI を、パワーユーザーはすべての機能を手にできます。

## ソースからのビルド {#build-from-source}

BombVault は、JSON API と埋め込みの React インターフェースを提供する単一の静的 Go バイナリです。まずインターフェースをビルドしてから、バイナリを実行します:

```bash
npm --prefix web ci
npm --prefix web run build     # web/dist に書き出し、バイナリがこれを埋め込む
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # ユニットテストと統合テスト（実際の restic の往復を含む）
golangci-lint run ./...
go run ./cmd/bombvault         # 自己署名証明書で https://localhost:3443 を提供
```

`go run` にもインターフェースのビルドが必要です。リポジトリが `web/dist` に置いているのは空の目印だけなので、`npm --prefix web run build` を実行しないとバイナリには何も埋め込まれず、`500 SPA index not found` と応答します。これは想定どおりの動作です。Docker、libvirt、Unraid は CI でテストできないため、プルリクエストを出す前に、実際のホストでホスト統合チェック（`/spike`）を使ってマウント、restic、VM の SSH 接続を確認してください。

## 次のステップ

- **[機能](features.md)**の全体を見てみましょう。
- **[Android アプリ](android.md)** で、グループ内のすべてのサーバーをスマートフォンに入れましょう。
- ひとつ以上の**[オフサイトと復旧](offsite-recovery.md)**レプリカを追加し（各ドメインは複数のデスティネーションへ同時に送信できます）、リカバリーキットを保存しましょう。
- セットアップを複製したり、新しいマシンに移行したりしますか？ **設定のエクスポート / インポート**カードで設定一式を持ち運べます。[設定](configuration.md#portable-settings-export-and-import)を参照してください。
- 問題にぶつかりましたか？ **[トラブルシューティング](troubleshooting.md)**を参照してください。
