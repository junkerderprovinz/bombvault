# 機能

BombVault ができることの短い一覧です。各行から、その説明があるページにリンクしています。

## バックアップの範囲 {#backup-scope}

- **コンテナ**：appdata フォルダーとコンテナ定義。コンテナごとにフォルダーを選べます。[バックアップと項目](backups.md#what-is-saved)
- **VM**：ディスクイメージ、XML、UEFI NVRAM を SSH 経由で。正常なシャットダウン、ライブスナップショット、変更ブロックのみから選べます。Unraid スナップショットのある VM や、zvol 上のディスクにも完全に対応しています。[VM のバックアップ](vm-backup-ssh-setup.md)
- **フラッシュ**：Unraid の USB フラッシュ全体と、スケジュールされた zip エクスポート。[バックアップと項目](backups.md#settings-tab)
- **セルフバックアップ**：BombVault 自身の設定と認証情報。[緊急ウィザード](offsite-recovery.md#emergency)
- **フォルダー**：サーバー上の任意のフォルダーをフォルダーセットとして。[フォルダーを追加する](backups.md#add)
- **ZFS データセット**：1 つのスナップショットから読み取るデータセットのツリー。単一ファイルを復元でき、2 台目の ZFS サーバーへのレプリカも任意で持てます。[ZFS データセット](zfs-datasets.md)
- **データベースダンプ**：PostgreSQL、MySQL、MariaDB を、実行中のサーバーから専用のリストアポイントへダンプします。[データベースダンプ](backups.md#database-dumps)

## バックアップと項目のページ {#backups}

- **1 つの一覧**：タイル、**フィルター**、**並べ替え**、**検索**があり、各行に直近 14 回の実行が帯状に並びます。[一覧](backups.md#list)
- **ファイルマネージャーのように選択**して、選択したものをサーバー上でバックアップまたは一時停止できます。[複数の項目を選ぶ](backups.md#select)
- **項目ごとに 1 ページ**：**バックアップ**、**設定**、**検証**。[項目のページ](backups.md#entry-page)
- スナップショットの**タグと比較**、そして tar、zip、restic リポジトリとしての**エクスポート**。[バックアップタブ](backups.md#backups-tab)
- ライブプレビュー付きの**除外パターン**と、よく知られたアプリ向けのおすすめの除外。[除外パターン](backups.md#excludes)
- **前後のコマンド**と、ヘルス確認を挟んで依存関係の順に再起動する**他のコンテナを停止**。[コマンドとほかのコンテナ](backups.md#commands)
- **アプリが使用中のとき**：スケジュールされたバックアップが Plex、Jellyfin、Emby、独自のチェック、または静かなトラフィックを待ち、自動で再試行します。[アプリが使用中のとき](backups.md#app-busy)
- **停止の代わりにスナップショット**：ZFS 上のコンテナは、停止するのが数秒だけです。[スケジュール](schedules.md#snapshot-instead-of-stop)
- **通常のエクスポート**。age による暗号化も選べます。[通常のエクスポート](backups.md#exports)

## 復元 {#restore}

- すべての種類に**1 つのページと 1 つのウィンドウ**。おすすめのバックアップが最初から選ばれています。[復元](restore.md)
- 標準は**元の隣に**。**元を置き換える**は先に現在の状態をバックアップします。[復元ウィンドウ](restore.md#window)
- **単一のファイル**をフォルダーへ、または元の場所へ。[ファイルを 1 つ取り戻す](restore.md#example-single-file)
- **複数の項目を同時に**。それぞれおすすめのバックアップから、または同じ時点から。[複数の項目をまとめて復元する](restore.md#several)
- **Compose スタック全体**を、依存関係の順に起動して。[スタックを復元する](restore.md#stacks)
- **コンテナと VM の再作成**は、動いていたイメージ、GPU、制限、リンクごと。[種類ごとに戻るもの](restore.md#kinds)
- **復元前のチェック**と、何が変わるかの一覧。[復元ウィンドウ](restore.md#window)
- **ZIP としてのフラッシュ**と、動作中のフラッシュへ 1 つずつ戻せるプラグイン。[種類ごとに戻るもの](restore.md#kinds)
- **ZFS** は、安全のためのスナップショットを取ってデータセットの中へ、フォルダーへ、またはプロパティつきの新しいデータセットへ。[ZFS データセット](zfs-datasets.md#restore)
- 失われたサーバーのための**緊急ウィザード**。リカバリーキット、グループ経由、または入力した鍵で。[緊急ウィザード](offsite-recovery.md#emergency)
- **古いバックアップフォルダーから**と、**Appdata.Backup プラグインから取り込む**。[ほかの取得元](restore.md#other-sources)

## ストレージとスケジュール {#storage-scheduling}

- あらゆる種類の**保存先**：ローカルフォルダー、SMB、WebDAV、SFTP、rest-server、S3、クラウドドライブ、任意の rclone リモート。[保存先](offsite-recovery.md#destinations)
- **認証情報**：標準のものがある 1 つの一覧。[認証情報](offsite-recovery.md#sign-ins)
- このサーバー上、または保存先の中の**専用フォルダー**。[セクションの専用フォルダー](offsite-recovery.md#own-folder)
- すべての行がスイッチになった**流れとしての 3-2-1**と、**項目ごとの配置**。[項目ごとの配置](offsite-recovery.md#placement)
- セクションごとに複数の場所への**オフサイトコピー**。帯域幅の制限、圧縮、コールドストレージクラス、Plex のストリーミング中の速度低下に対応します。[オフサイトコピー](offsite-recovery.md#off-site-copies)
- **ダイレクトリポジトリ**は、**ここに直接バックアップ**の下に一覧されます。[ダイレクトリポジトリ](offsite-recovery.md#direct-repositories)
- **タイムラインとしての保持**：保存先ごとに **短め**、**標準**、**長め**、**カスタム値**、そしてセクションごとのルール。[保持期間](offsite-recovery.md#keep)
- **セクションの履歴を移す**ことと、バックアップごと、またはバックアップなしでの**保存先を削除**。[移動と削除](offsite-recovery.md#move-remove)
- **文章として読めるスケジュール**、**週**の帯、全体バックアップ、セクションごとのプラン、項目ごとの専用プラン。[スケジュール](schedules.md)
- **実行されなかったバックアップの追い上げ**、手動の順序、バックアップ成功後の更新。[スケジュール](schedules.md)

## 洞察、検証、監視 {#insight-verification-monitoring}

- **概要**：ステータス欄、**要対応**、**保護ステータス**、ストレージの予測、履歴、任意のカード。[概要](overview.md)
- 項目ごとの**異常検出**。ソースが崩れたときは古いバックアップも保持します。[異常](anomalies.md)
- 最初のバックアップのあとの**復元チェック**と、隔離したコピーでの**起動テスト**。[検証タブ](backups.md#verify-tab)
- オフサイトのコピーからの**スケジュールされたチェックとテスト復元**。[DR ドリル](offsite-recovery.md#dr-drills)
- **フォルダー別のサイズ**と、バックアップが遅かった理由。[実行から分かること](backups.md#run-notes)
- **通知**：Webhook、Gotify、Matrix、メール、Apprise、Unraid、Healthchecks。週次ダイジェストと遅延の報告つき。[通知](notifications.md)
- **Prometheus**、Home Assistant デバイス、HTTP API、mDNS。[API と連携](api.md)
- **バグ報告**：GitHub の issue 用に情報を伏せた ZIP。[トラブルシューティング](troubleshooting.md#still-stuck)

## ランサムウェア対策 {#ransomware-protection}

- **削除に対する保護（append-only）**：相手側が強制し、改ざんテストで証明されます。[削除に対する保護](offsite-recovery.md#delete-protection-append-only)
- 削除を拒否できない場所のための**削除に対する保護なしで承認**。[削除に対する保護](offsite-recovery.md#delete-protection-append-only)
- **ログインパスワードを設定している場合、バックアップの完全な削除には本人のサインインが必要です。** ペアリングしたスマートフォンからは削除できません。[セキュリティモデル](configuration.md#security-model)
- **リカバリーキット**（age による暗号化も選べます）と、グループ内の**鍵の保管側**。[リカバリーキット](offsite-recovery.md#recovery-kit)、[鍵の保管側](offsite-recovery.md#key-keeper)

## 複数のサーバー {#several-servers}

- **12 個の単語によるペアリング**：エンドツーエンドで暗号化され、直接またはリレー経由。[インスタンスのペアリング](offsite-recovery.md#pairing)
- **インスタンスのページ**：グループをネットワークとして表示し、ロールは**レシーバー**、**取得側**、**ZFS サーバー**、**鍵の保管側**。[インスタンスとロール](offsite-recovery.md#roles)
- append-only モードで用意される**受信側サーバー**。[受信サーバー](offsite-recovery.md#receiving-server)

## AI アシスタント（MCP） {#mcp}

組み込みの MCP サーバーにより、アシスタントはバックアップの状態、カバレッジ、実行履歴、リストアポイントを読み取れます。許可するキーがあればバックアップの開始もできます。復元、削除、プルーン、設定は Web 画面にとどまります。[MCP サーバー](mcp.md)

## アプリと関連ツール {#apps}

- **Android アプリ**：グループ内のすべてのサーバーをスマートフォンで。[Android アプリ](android.md)
- **BombVault Widget**：Unraid ダッシュボードのタイル。[アプリ](settings.md#apps)
- Homepage や Heimdall などのダッシュボード向けの**埋め込めるアクティビティログ**。[ダッシュボードウィジェット](api.md#dashboard-widget)

## その他 {#other}

- **42 言語**、3 つのテーマ、そして自由に選べるラベル、色、角、アニメーション。[外観](settings.md#look)
- **自動で保存される設定**と、14 個すべてのタイルを横断する検索。[設定](settings.md)
- アップデートのあとの**新着情報**。変わった箇所には緑のドットが付きます。[情報](settings.md#info)
- **標準で使える HTTPS**と、自動復旧ツール向けのヘルスチェック。[設定](configuration.md#network)
