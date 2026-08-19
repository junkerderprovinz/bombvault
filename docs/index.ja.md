# BombVault

**Unraid のデータを、金庫に封印。バックアップを投げ込み、復元を起爆する。**

BombVault は、Docker コンテナと KVM/libvirt VM の**バックアップと完全な災害復旧**のための、セルフホスト型で Unraid ネイティブな Web アプリです。単一のマルチアーキテクチャ Docker コンテナとして動作し、システムのライト/ダークの設定に自動で追従するモダンな Web UI を備え、バックアップ、スケジュール設定、検証、復元というライフサイクル全体を扱います。

復元は自動です。コンテナは Unraid の Docker タブに以前とまったく同じ状態で再び現れ、VM はディスクと UEFI NVRAM を再アタッチした状態で VM Manager に再定義されます。手動での再インストールも、再設定も、面倒もありません。

[restic](https://restic.net) を基盤としているため、すべてのバックアップは重複排除され、増分方式で、常に暗号化されています。

!!! note "Keep your APP_KEY safe"
    BombVault は、`APP_KEY` という名前の 32 バイトのシークレットから restic リポジトリのパスワードを導出します。これを失うと、暗号化されたバックアップは復元不能になります。`openssl rand -hex 32` で生成し、安全な場所に保管してください。[設定](configuration.md)を参照してください。

## BombVault が保護するもの

| ドメイン | 保存される内容 |
|---|---|
| **Docker コンテナ** | Appdata ディレクトリに加え、コンテナ定義（イメージ、環境変数、ポート、ラベル、ボリューム）。 |
| **KVM / libvirt VM** | VM ディスクイメージ、XML 定義、UEFI NVRAM を、SSH 経由でバックアップ（libvirt のマウントなし）。 |
| **Unraid フラッシュ** | USB フラッシュ全体（`/boot`）：OS、ライセンス、アレイ設定、共有、ネットワークおよびプラグイン設定。 |
| **アプリ設定** | BombVault 自身の `/config`：設定データベース、オフサイトの認証情報、libvirt の SSH 鍵ペア。 |
| **ファイルとフォルダー** | 名前付きの**ファイルセット**。サーバー上の任意のフォルダーで、それぞれにセットごとの除外パターンを任意で設定できます。 |

## 復元こそが主役

restic スナップショットからデータを書き戻したあと、BombVault は保存されたコンテナ定義を Docker API に対して再生します。そのため、コンテナは最初からそこにあったかのように Unraid の Docker タブに再び現れます（同じイメージ、同じ設定、同じポートマッピング）。VM は XML が SSH 経由で再定義され、ディスクと UEFI NVRAM が再アタッチされます。VM が削除された後でも同様です。

バックアップが依存するコンテナを停止した場合でも、正しい順序で復帰します。BombVault はそれらを Compose の `depends_on` 順で再起動し、それに依存するコンテナを起動する前に、各コンテナが healthy を報告するまで待機します。そのため、まだ立ち上がっていないデータベースやゲートウェイを追い越して起動してしまうことはありません。[機能](features.md)を参照してください。

## 仕組み

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault はオーケストレーションと UI のレイヤーであり、ストレージエンジンではありません。実際のデータ移動はすべて restic を通じて行われます。

## クイックスタート

はじめてですか？ **[はじめに](getting-started.md)** に進んで、Community Applications 経由で Unraid に BombVault をインストールし、最初のバックアップを実行しましょう。その後、**[機能](features.md)**の全体を探索し、**[設定](configuration.md)**を調整し、**[オフサイトと復旧](offsite-recovery.md)**をセットアップしてください。

オフサイトはドメインごとに複数のターゲットへ同時にファンアウトでき、読み取り専用の**受信側ダッシュボード**がコピーを受け取る側のマシンでそれらのコピーを監視します。また、**設定のエクスポートとインポート**カードを使えば、設定一式を新しいマシンに持ち運べます。[オフサイトと復旧](offsite-recovery.md)および[設定](configuration.md#portable-settings-export-and-import)を参照してください。

## リンク

- **ソースコード:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid サポートスレッド:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issue:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-equivalent control of the host"
    Docker ソケットを通じて、BombVault はコンテナの停止、削除、再作成を行い、appdata の読み書きができます。また VM バックアップのためにホストに SSH でログインして `virsh` を実行します。その Web UI に到達できる者は、実質的にホストの root 権限を持つことになります。BombVault は信頼できる、外部に公開されていないネットワークでのみ実行し、オフサイトまたはイミュータブルなバックアップを使い始めたら、任意のパスワードゲート（設定、セキュリティ）を有効にしてください。完全なセキュリティモデルについては[設定](configuration.md)を参照してください。
