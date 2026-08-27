# 設定

このページでは、コンテナの環境変数、テンプレートが提供するマウント、SSH 経由の VM バックアップ、そしてオフサイトのセットアップを扱います。バックアップの**リポジトリパス**は、環境変数ではなくアプリ内（Settings, Backup paths）で設定します。

## 環境変数

| 変数 | 必須 | 説明 |
|---|---|---|
| `APP_KEY` | **はい** | restic リポジトリのパスワードを導出するために使う 32 バイトの 16 進シークレット（16 進数 64 文字）。`openssl rand -hex 32` で生成します。これを安全に保管してください: 失うと暗号化されたバックアップは復元不能になります。 |
| `LIBVIRT_HOST` | VM に必要 | VM バックアップのために SSH で到達する Unraid ホスト（デフォルト `host.docker.internal`。テンプレートは LAN-IP のプレースホルダーをあらかじめ入力します）。Unraid の LAN IP を使ってください。カスタムの `br0.x` ネットワークでは必須です。 |
| `LIBVIRT_SSH_PORT` | いいえ | VM バックアップのためのホスト SSH ポート（デフォルト `22`）。 |
| `LIBVIRT_SSH_USER` | いいえ | VM バックアップのためのホスト上の SSH ユーザー（デフォルト `root`）。 |
| `LIBVIRT_URI` | いいえ | 完全な libvirt 接続 URI。上記 3 つの `LIBVIRT_*` 変数から組み立てる代わりに、これを**そのまま**使用します（設定するとそれらは接続文字列には使われなくなります）。デフォルトは未設定です。TrueNAS Scale では必須です。TrueNAS の libvirtd は非標準のソケットで待ち受けており、組み立て式の URI ではそれを表現できません: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`。詳細は [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) の TrueNAS Scale の節を参照してください。 |
| `PORT` | いいえ | HTTP ポート（デフォルト `3000`。`HTTP_ONLY=true` の場合にのみ使用）。 |
| `HTTPS_PORT` | いいえ | HTTPS ポート（デフォルト `3443`。テンプレートは 1:1 で公開するため、WebUI は `https://<ip>:3443` で応答します）。 |
| `HTTP_ONLY` | いいえ | `true` に設定すると、自己署名 HTTPS リスナーを無効にし、プレーン HTTP のみを提供します（TLS を終端するリバースプロキシの背後での使用向け）。 |
| `HOST_SOURCE_ROOT` | いいえ | **Host Data** としてマウントされるホストパス（デフォルト `/mnt`）。BombVault は Docker が報告するバインドマウントのソースを、このマウント配下のパスに変換します。別のホストルートをマウントした場合のみ変更してください。 |
| `DATA_ROOT_SEGMENTS` | いいえ | バインドマウントのソースをバックアップ対象データとして扱うためのパスセグメント名のカンマ区切りリスト（デフォルト `appdata`。Unraid の `/mnt/user/appdata/<container>` という慣習に一致します）。列挙したセグメントのいずれかが、コンテナのバインドマウントのホストソースの完全なパスセグメントとして現れる場合、そのバインドマウントは自動的にバックアップ対象として選択されます。たとえば `DATA_ROOT_SEGMENTS=appdata,config` と設定すると、`.../config` のバインドも対象になります。コンテナのデータフォルダーを見つけるための、その他の常時有効な方法については[バックアップソースの検出](#backup-source-detection)を参照してください。 |
| `PLATFORM` | いいえ | BombVault が自動検出する代わりに、自身がどのプラットフォームで動作しているとみなすかを強制します: `unraid`、`generic`、`truenas` のいずれか（デフォルトは未設定。フラッシュマウント配下の `dockerMan` マーカーを探して Unraid を自動検出し、見つからなければ `generic` になります。認識できない値を指定した場合も `generic` にフォールバックし、ログに記録されます）。汎用の Docker ホストや TrueNAS Scale では、Unraid 専用の自動検出に頼らず明示的に設定してください（汎用の compose ファイルはこれを行っています）。appdata フォールバックの慣習、インスタンス間の復元先のデフォルト、そして Unraid 専用の通知・コンパニオンプラグイン処理を試みるかどうかが変わります（`internal/platform` を参照）。 |
| `BOMBVAULT_SELF_CONTAINER` | いいえ | BombVault コンテナ自身の名前。これにより自分自身をバックアップ（したがって停止）しません（デフォルト `BombVault`。ブリッジネットワークではホスト名から自動検出）。 |
| `BACKUP_MAX_HOURS` | いいえ | 単一のバックアップ実行が強制キャンセルされる前に、そのドメインロックを保持できる最大の実時間（実行が動かなくなってもドメインを永遠にブロックできないようにするガード）。空（デフォルト）は `48` を使います。非常に大きい、または遅いクラウドバックアップでは引き上げてください（上限でキャンセルされた実行は `context deadline exceeded` で失敗します）。`0` に設定すると上限を完全に無効にします。 |
| `TZ` | いいえ | スケジューラーのタイムゾーン（たとえば `Europe/Berlin`）。 **設定しない場合、すべてのスケジュールは UTC で実行されます**。02:30 に設定したスケジュールは現地時間ではなく 02:30 UTC に開始します。 |

## マウント

CA テンプレートに示されているとおり、Docker ソケット、フラッシュ（`/boot`）、そして **Host Data** ルート（`/mnt`）をマウントします。バックアップの*ソース*と*デスティネーション*はどちらも Host Data の配下に存在し、それは **rslave** でマウントされます。そのため、コンテナ起動後にマウントされるリモート共有（たとえば `/mnt/remotes` の配下）が、再起動なしで見えるようになります。

バックアップのリポジトリパスはデフォルトで `/mnt/user/bombvault/{container,vms,flash,config,files}` になり、初回バックアップ時に作成されます。場所はいつでも **Settings, Backup paths** で変更できます。

!!! note "Host integration check"
    コンテナが起動したあと、Web UI で `/spike` を開いてください。これはすべてのマウントと CLI（Docker ソケット、libvirt、restic、qemu-img、rclone）をプローブし、欠けている部分を報告します。

## セキュリティモデル

!!! warning "Root-equivalent control of the host"
    Docker ソケットを通じて、BombVault はコンテナの停止、削除、再作成を行い、appdata の読み書きができます。また VM バックアップのためにホストに SSH（`qemu+ssh://`、デフォルトは root）でログインして `virsh` を実行します。その Web UI に到達できる者は、実質的にホストの root 権限を持つことになります。

- **任意のパスワード保護**（Settings, Security）: パスワードを設定するとログインが必須になり、クリアすると無効になります。信頼できる LAN での使用のため、デフォルトはオフです。セッションは署名されており（`APP_KEY` から導出された HMAC）、パスワードを変更すると無効になります。ログインはレート制限されます。
- ゲートはオプトインのため、未設定のときは UI と API 全体（オフサイトのセットアップ、改ざんテストのルート、リカバリーキットを含む）が、ポートに到達できる誰からでもアクセス可能になります。オフサイト、イミュータブルなバックアップ、または暗号化を使い始めたら、ゲートを有効にしてください。
- BombVault は信頼できる、外部に公開されていないネットワークでのみ実行してください。リモートアクセスには、認証と TLS を追加するリバースプロキシの背後に置いてください。レスポンスにはベースラインのセキュリティヘッダー（CSP、`nosniff`、`X-Frame-Options`、`Referrer-Policy`）が付きます。
- `HTTP_ONLY=true` では、セッション Cookie は `Secure` フラグを失います（プレーン HTTP で機能させるためにそうせざるを得ません）。そのため、機密性が重要な場合は、TLS を終端するプロキシの背後でのみパスワードを有効にしてください。
- VM バックアップの SSH 接続は、初回接続時にホスト鍵を信頼し（TOFU）、それ以降ピン留めします。コンテナからホストへの経路が信頼できない場合は、ホストの鍵を帯域外で検証してください。
- 暗号化が有効な場合（設定。デフォルトはオン）、バックアップは restic によって暗号化され、鍵は `APP_KEY` から導出されます。

## SSH 経由の VM バックアップ

BombVault は **libvirt のパスを一切マウントすることなく** KVM/libvirt VM をバックアップします。SSH（`qemu+ssh://`）経由でホスト上の `virsh` を実行するため、ホストの VM Manager に影響を与えることは決してありません。

クイックセットアップ:

1. **Settings, System, VM Backup over SSH:** 表示された公開鍵をコピーします。
2. それを Unraid の `/root/.ssh/authorized_keys` に追記します（再起動後も維持されるようフラッシュにも保存されます）。
3. **Test connection** をクリックします。

テンプレートは `--add-host=host.docker.internal:host-gateway` を追加するため、コンテナはホストに到達できます。その名前が解決されない場合（たとえばコンテナがカスタムの `br0.x` ネットワークで実行されている場合）は、`LIBVIRT_HOST` を Unraid の LAN IP に設定してください。Unraid の SSH ポートを変更した場合は、`LIBVIRT_SSH_PORT` を一致させてください。**ライブスナップショット**には、加えて VM 内の qemu ゲストエージェントと、ディスクが `/mnt/cache`（`/mnt/user` ではなく）にあることが必要です。

!!! important "Full VM setup and networking guide"
    完全なステップバイステップガイド（SSH の有効化、永続的な鍵の承認、カスタムネットワークと VLAN のルーティング、VM ごとの方式、ホスト側のトラブルシューティング）は、GitHub の [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) にあります。

## オフサイトのセットアップ

**Settings, Off-site** タブでオフサイトのレプリカをセットアップします。完全なワークフロー（イミュータブル/追記専用、改ざんテスト、DR ドリル）については[オフサイトと復旧](offsite-recovery.md)を参照してください。要点は以下のとおりです:

- **バックエンド:** SMB/CIFS と NFS（共有をマウントして Backup Path をそこに向ける）、rclone なしのネイティブ restic バックエンド（`s3:...`、`rest:http://host:8000/repo`、`b2:...`、`sftp:user@host:/repo`）、または任意の rclone リモート（`rclone:<remote>:<bucket>/path`）。
- **クラウド認証情報**は、Settings, Off-site, Cloud credentials で暗号化して保存されます。
- **SSH ターゲットは相手側に何もインストールする必要がありません。** `sftp:` は SSH サーバーだけを必要とします。**Settings, System, VM Backup over SSH** の公開鍵（`/config/ssh/id_ed25519.pub` にもあります）を、ターゲットユーザーの `~/.ssh/authorized_keys` に追加します。
- **オフサイトコピー:** BombVault はベストエフォート方式で `restic copy` により新しいスナップショットを複製します。ローカルリポジトリが主のままです。各ドメインには独自のオフサイトスケジュールがあり、**今すぐ複製**ボタンも備わっています。
- **ドメインごとに複数のオフサイトターゲット:** 各ドメインは複数のオフサイトデスティネーションへ同時に複製できます。Settings, Off-site で追加のターゲットを加え、それぞれに独自のリポジトリ、S3 ストレージクラス、追記専用フラグ、保持、成長予算を設定します。それらはすべてそのドメインのオフサイトスケジュールで複製されます。既存の単一オフサイトセットアップは最初のターゲットとして引き継がれます。
- **ソースごとの保持:** ローカルのポリシーは Settings, Paths & Storage にあり、オフサイトのポリシーは Settings, Off-site にあります（すべてをゼロのままにするとオフサイトのスナップショットを自動整理しません）。
- **帯域幅制限:** Settings, Off-site で restic のアップロード/ダウンロードレートに上限を設けます。
- **コールドおよびアーカイブのストレージクラス（S3）:** ネイティブ S3 のオフサイトリポジトリでは、復元可能な階層（Standard、Standard-IA、One Zone-IA、Intelligent-Tiering、Glacier Instant Retrieval）を選びます。rclone リモートはそのクラスを rclone 設定で指定します。

## 持ち運び可能な設定（エクスポートとインポート） {#portable-settings-export-and-import}

Settings ページの**設定のエクスポートとインポート**カードは、BombVault の設定一式（ドメイン設定、オフサイトターゲット、スケジュール、保持、通知）を、別のインスタンスでインポートできる持ち運び可能な JSON ファイルに書き出します。これにより、新しいマシンへの移行やセットアップの複製で、すべてを手作業で入力し直す必要がなくなります。インポートはプレビューを表示して確認を求め、バックアップデータや履歴に触れることは決してありません。

!!! warning "The export can contain credentials"
    オフサイトと通知の認証情報をファイルに含めるかどうかは選べます。認証情報を含めた場合、エクスポートはリカバリーキットと同じくらい機密性が高くなるため、安全な場所に保管してください。含めない場合、ファイルには機密でない設定のみが含まれます。
